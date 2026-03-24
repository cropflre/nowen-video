package service

import (
	"encoding/xml"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// CastDevice 投屏设备信息
type CastDevice struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"` // dlna / chromecast
	Location     string `json:"location"`
	Manufacturer string `json:"manufacturer"`
	ModelName    string `json:"model_name"`
	LastSeen     int64  `json:"last_seen"` // unix timestamp
}

// CastSession 投屏会话
type CastSession struct {
	ID       string      `json:"id"`
	DeviceID string      `json:"device_id"`
	MediaID  string      `json:"media_id"`
	Status   string      `json:"status"` // idle / playing / paused / stopped
	Position float64     `json:"position"`
	Duration float64     `json:"duration"`
	Volume   float64     `json:"volume"`
	Device   *CastDevice `json:"device,omitempty"`
}

// CastService 投屏服务
type CastService struct {
	mediaRepo *repository.MediaRepo
	cfg       *config.Config
	logger    *zap.SugaredLogger

	mu       sync.RWMutex
	devices  map[string]*CastDevice  // 已发现的设备
	sessions map[string]*CastSession // 活跃投屏会话

	stopChan chan struct{} // 用于停止设备发现
}

func NewCastService(
	mediaRepo *repository.MediaRepo,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) *CastService {
	cs := &CastService{
		mediaRepo: mediaRepo,
		cfg:       cfg,
		logger:    logger,
		devices:   make(map[string]*CastDevice),
		sessions:  make(map[string]*CastSession),
		stopChan:  make(chan struct{}),
	}

	// 启动后台SSDP设备发现
	go cs.startDiscovery()

	return cs
}

// ==================== SSDP 设备发现 ====================

const (
	ssdpMulticastAddr = "239.255.255.250:1900"
	ssdpSearchTarget  = "urn:schemas-upnp-org:device:MediaRenderer:1"
	ssdpSearchMX      = 3
)

// startDiscovery 后台定期发现DLNA设备
func (s *CastService) startDiscovery() {
	// 首次立即发现
	s.discoverDevices()

	// 每30秒扫描一次
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.discoverDevices()
			// 清理60秒未响应的设备
			s.cleanStaleDevices(60 * time.Second)
		case <-s.stopChan:
			return
		}
	}
}

// discoverDevices 通过SSDP M-SEARCH发现DLNA设备
func (s *CastService) discoverDevices() {
	// 构造SSDP搜索请求
	searchMsg := fmt.Sprintf(
		"M-SEARCH * HTTP/1.1\r\n"+
			"HOST: %s\r\n"+
			"MAN: \"ssdp:discover\"\r\n"+
			"ST: %s\r\n"+
			"MX: %d\r\n"+
			"\r\n",
		ssdpMulticastAddr, ssdpSearchTarget, ssdpSearchMX,
	)

	// 解析多播地址
	addr, err := net.ResolveUDPAddr("udp4", ssdpMulticastAddr)
	if err != nil {
		s.logger.Debugf("解析SSDP多播地址失败: %v", err)
		return
	}

	// 创建UDP连接
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		s.logger.Debugf("创建UDP连接失败: %v", err)
		return
	}
	defer conn.Close()

	// 设置超时
	conn.SetDeadline(time.Now().Add(time.Duration(ssdpSearchMX+1) * time.Second))

	// 发送搜索请求
	_, err = conn.WriteToUDP([]byte(searchMsg), addr)
	if err != nil {
		s.logger.Debugf("发送SSDP搜索请求失败: %v", err)
		return
	}

	// 接收响应
	buf := make([]byte, 4096)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // 超时或错误
		}

		response := string(buf[:n])
		device := s.parseSSDPResponse(response, remoteAddr)
		if device != nil {
			// 异步获取设备详细信息
			go s.fetchDeviceDescription(device)
		}
	}
}

// parseSSDPResponse 解析SSDP响应
func (s *CastService) parseSSDPResponse(response string, addr *net.UDPAddr) *CastDevice {
	lines := strings.Split(response, "\r\n")
	headers := make(map[string]string)

	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			headers[strings.ToUpper(strings.TrimSpace(parts[0]))] = strings.TrimSpace(parts[1])
		}
	}

	location := headers["LOCATION"]
	usn := headers["USN"]

	if location == "" || usn == "" {
		return nil
	}

	// 使用USN作为设备唯一标识
	deviceID := usn
	if idx := strings.Index(usn, "::"); idx > 0 {
		deviceID = usn[:idx]
	}

	return &CastDevice{
		ID:       deviceID,
		Location: location,
		Type:     "dlna",
		LastSeen: time.Now().Unix(),
	}
}

// deviceDescription UPnP设备描述XML结构
type deviceDescription struct {
	XMLName xml.Name `xml:"root"`
	Device  struct {
		FriendlyName string `xml:"friendlyName"`
		Manufacturer string `xml:"manufacturer"`
		ModelName    string `xml:"modelName"`
		UDN          string `xml:"UDN"`
	} `xml:"device"`
}

// fetchDeviceDescription 获取设备详细描述
func (s *CastService) fetchDeviceDescription(device *CastDevice) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(device.Location)
	if err != nil {
		s.logger.Debugf("获取设备描述失败 [%s]: %v", device.Location, err)
		return
	}
	defer resp.Body.Close()

	var desc deviceDescription
	if err := xml.NewDecoder(resp.Body).Decode(&desc); err != nil {
		s.logger.Debugf("解析设备描述XML失败: %v", err)
		return
	}

	device.Name = desc.Device.FriendlyName
	device.Manufacturer = desc.Device.Manufacturer
	device.ModelName = desc.Device.ModelName
	if desc.Device.UDN != "" {
		device.ID = desc.Device.UDN
	}

	s.mu.Lock()
	s.devices[device.ID] = device
	s.mu.Unlock()

	s.logger.Infof("发现DLNA设备: %s (%s)", device.Name, device.Manufacturer)
}

// cleanStaleDevices 清理长时间未响应的设备
func (s *CastService) cleanStaleDevices(maxAge time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-maxAge).Unix()
	for id, device := range s.devices {
		if device.LastSeen < cutoff {
			delete(s.devices, id)
			s.logger.Debugf("移除过期DLNA设备: %s", device.Name)
		}
	}
}

// ==================== 设备与会话管理 ====================

// ListDevices 列出已发现的投屏设备
func (s *CastService) ListDevices() []CastDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var devices []CastDevice
	for _, d := range s.devices {
		devices = append(devices, *d)
	}
	return devices
}

// RefreshDevices 手动触发设备发现
func (s *CastService) RefreshDevices() {
	go s.discoverDevices()
}

// GetDevice 获取指定设备
func (s *CastService) GetDevice(deviceID string) (*CastDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, ok := s.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在或已离线: %s", deviceID)
	}
	return device, nil
}

// ==================== DLNA 投屏控制 ====================

// CastMedia 投屏媒体到指定设备
func (s *CastService) CastMedia(deviceID, mediaID string, serverAddr string) (*CastSession, error) {
	// 验证设备
	device, err := s.GetDevice(deviceID)
	if err != nil {
		return nil, err
	}

	// 验证媒体
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("媒体不存在: %s", mediaID)
	}

	// 构造媒体播放URL（设备需要能直接访问这个URL）
	mediaURL := fmt.Sprintf("http://%s/api/stream/%s/direct?cast=1", serverAddr, mediaID)

	// 获取AVTransport服务控制URL
	controlURL, err := s.getAVTransportControlURL(device.Location)
	if err != nil {
		return nil, fmt.Errorf("获取设备控制地址失败: %w", err)
	}

	// 发送SetAVTransportURI指令
	err = s.sendDLNAAction(controlURL, "SetAVTransportURI", map[string]string{
		"InstanceID":         "0",
		"CurrentURI":         mediaURL,
		"CurrentURIMetaData": s.buildDIDLMetadata(media),
	})
	if err != nil {
		return nil, fmt.Errorf("设置播放源失败: %w", err)
	}

	// 发送Play指令
	err = s.sendDLNAAction(controlURL, "Play", map[string]string{
		"InstanceID": "0",
		"Speed":      "1",
	})
	if err != nil {
		return nil, fmt.Errorf("发送播放指令失败: %w", err)
	}

	// 创建投屏会话
	session := &CastSession{
		ID:       fmt.Sprintf("cast-%s-%d", deviceID, time.Now().UnixMilli()),
		DeviceID: deviceID,
		MediaID:  mediaID,
		Status:   "playing",
		Volume:   1.0,
		Device:   device,
	}

	s.mu.Lock()
	s.sessions[session.ID] = session
	s.mu.Unlock()

	s.logger.Infof("开始投屏: %s -> %s", media.Title, device.Name)
	return session, nil
}

// ControlCast 控制投屏播放
func (s *CastService) ControlCast(sessionID, action string, value float64) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.RUnlock()
		return fmt.Errorf("投屏会话不存在: %s", sessionID)
	}
	device := s.devices[session.DeviceID]
	s.mu.RUnlock()

	if device == nil {
		return fmt.Errorf("设备已离线")
	}

	controlURL, err := s.getAVTransportControlURL(device.Location)
	if err != nil {
		return err
	}

	switch action {
	case "play":
		err = s.sendDLNAAction(controlURL, "Play", map[string]string{
			"InstanceID": "0",
			"Speed":      "1",
		})
		if err == nil {
			s.mu.Lock()
			session.Status = "playing"
			s.mu.Unlock()
		}

	case "pause":
		err = s.sendDLNAAction(controlURL, "Pause", map[string]string{
			"InstanceID": "0",
		})
		if err == nil {
			s.mu.Lock()
			session.Status = "paused"
			s.mu.Unlock()
		}

	case "stop":
		err = s.sendDLNAAction(controlURL, "Stop", map[string]string{
			"InstanceID": "0",
		})
		if err == nil {
			s.mu.Lock()
			session.Status = "stopped"
			delete(s.sessions, sessionID)
			s.mu.Unlock()
		}

	case "seek":
		// value为秒数
		h := int(value) / 3600
		m := (int(value) % 3600) / 60
		sec := int(value) % 60
		target := fmt.Sprintf("%02d:%02d:%02d", h, m, sec)

		err = s.sendDLNAAction(controlURL, "Seek", map[string]string{
			"InstanceID": "0",
			"Unit":       "REL_TIME",
			"Target":     target,
		})

	case "volume":
		// value为0-100
		renderControlURL, err2 := s.getRenderingControlURL(device.Location)
		if err2 != nil {
			return err2
		}
		err = s.sendDLNAAction(renderControlURL, "SetVolume", map[string]string{
			"InstanceID":    "0",
			"Channel":       "Master",
			"DesiredVolume": fmt.Sprintf("%d", int(value)),
		})
		if err == nil {
			s.mu.Lock()
			session.Volume = value / 100
			s.mu.Unlock()
		}

	default:
		return fmt.Errorf("不支持的操作: %s", action)
	}

	return err
}

// GetSession 获取投屏会话状态
func (s *CastService) GetSession(sessionID string) (*CastSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("投屏会话不存在")
	}

	return session, nil
}

// ListSessions 列出所有活跃投屏会话
func (s *CastService) ListSessions() []CastSession {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var sessions []CastSession
	for _, sess := range s.sessions {
		sessions = append(sessions, *sess)
	}
	return sessions
}

// StopAllSessions 停止所有投屏会话
func (s *CastService) StopAllSessions() {
	s.mu.Lock()
	sessionIDs := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		sessionIDs = append(sessionIDs, id)
	}
	s.mu.Unlock()

	for _, id := range sessionIDs {
		_ = s.ControlCast(id, "stop", 0)
	}
}

// Stop 停止投屏服务
func (s *CastService) Stop() {
	close(s.stopChan)
	s.StopAllSessions()
}

// ==================== DLNA SOAP 通信 ====================

// getAVTransportControlURL 从设备描述中获取AVTransport服务控制URL
func (s *CastService) getAVTransportControlURL(deviceLocation string) (string, error) {
	return s.getServiceControlURL(deviceLocation, "AVTransport")
}

// getRenderingControlURL 从设备描述中获取RenderingControl服务控制URL
func (s *CastService) getRenderingControlURL(deviceLocation string) (string, error) {
	return s.getServiceControlURL(deviceLocation, "RenderingControl")
}

// serviceList 解析设备服务列表的XML结构
type serviceList struct {
	XMLName xml.Name `xml:"root"`
	Device  struct {
		ServiceList struct {
			Services []struct {
				ServiceType string `xml:"serviceType"`
				ControlURL  string `xml:"controlURL"`
			} `xml:"service"`
		} `xml:"serviceList"`
	} `xml:"device"`
}

func (s *CastService) getServiceControlURL(deviceLocation, serviceKeyword string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(deviceLocation)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var svcList serviceList
	if err := xml.NewDecoder(resp.Body).Decode(&svcList); err != nil {
		return "", err
	}

	for _, svc := range svcList.Device.ServiceList.Services {
		if strings.Contains(svc.ServiceType, serviceKeyword) {
			// 拼接完整的控制URL
			if strings.HasPrefix(svc.ControlURL, "http") {
				return svc.ControlURL, nil
			}
			// 相对路径，基于设备Location的base URL
			baseURL := deviceLocation
			if idx := strings.LastIndex(baseURL, "/"); idx > 8 { // 跳过 "http://"
				baseURL = baseURL[:idx]
			}
			return baseURL + svc.ControlURL, nil
		}
	}

	return "", fmt.Errorf("设备不支持 %s 服务", serviceKeyword)
}

// sendDLNAAction 发送DLNA SOAP控制指令
func (s *CastService) sendDLNAAction(controlURL, action string, args map[string]string) error {
	// 确定服务类型
	serviceType := "urn:schemas-upnp-org:service:AVTransport:1"
	if action == "SetVolume" || action == "GetVolume" {
		serviceType = "urn:schemas-upnp-org:service:RenderingControl:1"
	}

	// 构造SOAP XML body
	var argsXML string
	for key, value := range args {
		argsXML += fmt.Sprintf("<%s>%s</%s>", key, xmlEscape(value), key)
	}

	soapBody := fmt.Sprintf(
		`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%s xmlns:u="%s">
      %s
    </u:%s>
  </s:Body>
</s:Envelope>`,
		action, serviceType, argsXML, action,
	)

	req, err := http.NewRequest("POST", controlURL, strings.NewReader(soapBody))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPAction", fmt.Sprintf(`"%s#%s"`, serviceType, action))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("DLNA请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DLNA响应错误: HTTP %d", resp.StatusCode)
	}

	return nil
}

// buildDIDLMetadata 构造DIDL-Lite元数据
func (s *CastService) buildDIDLMetadata(media *model.Media) string {
	return fmt.Sprintf(
		`&lt;DIDL-Lite xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"&gt;`+
			`&lt;item id="0" parentID="-1" restricted="1"&gt;`+
			`&lt;dc:title&gt;%s&lt;/dc:title&gt;`+
			`&lt;upnp:class&gt;object.item.videoItem.movie&lt;/upnp:class&gt;`+
			`&lt;/item&gt;`+
			`&lt;/DIDL-Lite&gt;`,
		xmlEscape(media.Title),
	)
}

// xmlEscape 转义XML特殊字符
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
