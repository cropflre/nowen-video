package service

// mdns.go 实现 NowenVideo 的 mDNS/DNS-SD 局域网服务器自动发现广播。
//
// 工作原理：
//   使用标准 mDNS 协议（RFC 6762 + RFC 6763），在局域网内广播服务信息。
//   安卓端通过 NsdManager 发现 "_nowen-video._tcp" 类型的服务，
//   自动获取服务器 IP、端口和元信息（版本号、服务器名称等）。
//
// 纯 Go 标准库实现，无需第三方 mDNS 依赖。

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	// mDNS 标准组播地址和端口
	mdnsIPv4Addr = "224.0.0.251"
	mdnsPort     = 5353

	// NowenVideo 服务类型
	mdnsServiceType = "_nowen-video._tcp"
	mdnsDomain      = "local"

	// DNS 记录类型
	dnsTypePTR = 12
	dnsTypeSRV = 33
	dnsTypeTXT = 16
	dnsTypeA   = 1

	// DNS 类
	dnsClassIN    = 1
	dnsClassFlush = 0x8001 // Cache-Flush + IN

	// 默认 TTL（秒）
	defaultTTL = 120
)

// MdnsService 负责在局域网内通过 mDNS 广播 NowenVideo 服务器信息。
type MdnsService struct {
	instanceName string
	serviceType  string
	httpPort     int
	txtRecords   []string
	logger       *zap.SugaredLogger

	conn   *net.UDPConn
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMdnsService 构造 mDNS 广播服务。
//   - serverName: 服务器显示名称（留空则使用主机名）
//   - httpPort:   HTTP 服务端口
//   - version:    服务器版本号
func NewMdnsService(serverName string, httpPort int, version string, logger *zap.SugaredLogger) *MdnsService {
	if serverName == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			serverName = h
		} else {
			serverName = "NowenVideo"
		}
	}

	// mDNS 实例名 = 服务器名称
	instanceName := serverName

	// TXT 记录：携带版本号和服务器名称
	txtRecords := []string{
		"version=" + version,
		"server_name=" + serverName,
		"server_type=nowen-video",
	}

	return &MdnsService{
		instanceName: instanceName,
		serviceType:  mdnsServiceType,
		httpPort:     httpPort,
		txtRecords:   txtRecords,
		logger:       logger,
	}
}

// Start 启动 mDNS 广播服务。
func (m *MdnsService) Start() error {
	if m.conn != nil {
		return nil
	}

	// 加入 mDNS 组播组
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: mdnsPort}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("mDNS UDP %d 监听失败: %w", mdnsPort, err)
	}
	m.conn = conn

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// 启动查询响应协程
	m.wg.Add(1)
	go m.listenAndRespond(ctx)

	// 启动定期公告协程
	m.wg.Add(1)
	go m.periodicAnnounce(ctx)

	// 立即发送一次公告
	m.sendAnnouncement()

	if m.logger != nil {
		m.logger.Infof("[mdns] NowenVideo 服务发现已启动 (mDNS :%d, 服务类型: %s)", mdnsPort, m.serviceType)
	}
	return nil
}

// Stop 停止 mDNS 广播服务。
func (m *MdnsService) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
	m.wg.Wait()
	if m.logger != nil {
		m.logger.Info("[mdns] NowenVideo 服务发现已停止")
	}
}

// listenAndRespond 监听 mDNS 查询并响应。
func (m *MdnsService) listenAndRespond(ctx context.Context) {
	defer m.wg.Done()
	buf := make([]byte, 4096)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = m.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remote, err := m.conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			continue
		}

		// 解析 DNS 查询，检查是否在查询我们的服务类型
		if m.isQueryForOurService(buf[:n]) {
			m.respondTo(remote)
		}
	}
}

// periodicAnnounce 定期发送 mDNS 公告（每 60 秒一次）。
func (m *MdnsService) periodicAnnounce(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.sendAnnouncement()
		}
	}
}

// sendAnnouncement 向 mDNS 组播地址发送服务公告。
func (m *MdnsService) sendAnnouncement() {
	packet := m.buildResponsePacket()
	if packet == nil {
		return
	}

	dst := &net.UDPAddr{
		IP:   net.ParseIP(mdnsIPv4Addr),
		Port: mdnsPort,
	}

	if _, err := m.conn.WriteToUDP(packet, dst); err != nil {
		if m.logger != nil {
			m.logger.Debugf("[mdns] 公告发送失败: %v", err)
		}
	}
}

// respondTo 向特定客户端发送响应。
func (m *MdnsService) respondTo(remote *net.UDPAddr) {
	packet := m.buildResponsePacket()
	if packet == nil {
		return
	}

	// mDNS 响应发送到组播地址
	dst := &net.UDPAddr{
		IP:   net.ParseIP(mdnsIPv4Addr),
		Port: mdnsPort,
	}

	if _, err := m.conn.WriteToUDP(packet, dst); err != nil {
		if m.logger != nil {
			m.logger.Debugf("[mdns] 响应发送失败 remote=%s: %v", remote, err)
		}
		return
	}

	if m.logger != nil {
		m.logger.Debugf("[mdns] 已响应服务发现查询 remote=%s", remote)
	}
}

// isQueryForOurService 简单解析 DNS 包，判断是否在查询我们的服务类型。
func (m *MdnsService) isQueryForOurService(data []byte) bool {
	if len(data) < 12 {
		return false
	}

	// DNS Header: flags 字段（第2-3字节），QR=0 表示查询
	flags := binary.BigEndian.Uint16(data[2:4])
	if flags&0x8000 != 0 {
		// 这是一个响应包，不是查询
		return false
	}

	// 查询数量
	qdCount := binary.BigEndian.Uint16(data[4:6])
	if qdCount == 0 {
		return false
	}

	// 简单检查：将整个包转为小写字符串，看是否包含我们的服务类型
	// 这是一种简化的匹配方式，对于 mDNS 场景足够可靠
	lowerData := strings.ToLower(string(data))
	serviceCheck := strings.ToLower(m.serviceType + "." + mdnsDomain)
	if strings.Contains(lowerData, strings.ToLower(m.serviceType)) ||
		strings.Contains(lowerData, serviceCheck) {
		return true
	}

	// 也响应 _services._dns-sd._udp.local 的服务枚举查询
	if strings.Contains(lowerData, "_services._dns-sd._udp") {
		return true
	}

	return false
}

// buildResponsePacket 构建 mDNS 响应包。
// 包含 PTR、SRV、TXT、A 记录。
func (m *MdnsService) buildResponsePacket() []byte {
	localIPs := getLocalIPv4Addrs()
	if len(localIPs) == 0 {
		return nil
	}

	// 完整服务名
	serviceFQDN := m.serviceType + "." + mdnsDomain           // _nowen-video._tcp.local
	instanceFQDN := m.instanceName + "." + serviceFQDN        // NowenVideo._nowen-video._tcp.local
	hostFQDN := strings.ReplaceAll(m.instanceName, " ", "-") + "." + mdnsDomain // NowenVideo.local

	var buf []byte

	// DNS Header（响应包）
	header := make([]byte, 12)
	// Transaction ID = 0（mDNS 响应）
	// Flags: QR=1 (Response), AA=1 (Authoritative)
	binary.BigEndian.PutUint16(header[2:4], 0x8400)
	// Answer count = PTR + SRV + TXT + A记录数
	answerCount := uint16(3 + len(localIPs))
	binary.BigEndian.PutUint16(header[6:8], answerCount)
	buf = append(buf, header...)

	// PTR 记录：_nowen-video._tcp.local -> instanceName._nowen-video._tcp.local
	buf = append(buf, encodeDNSRecord(serviceFQDN, dnsTypePTR, dnsClassIN, defaultTTL, encodeDNSName(instanceFQDN))...)

	// SRV 记录：instanceName._nowen-video._tcp.local -> hostFQDN:port
	srvData := make([]byte, 6)
	binary.BigEndian.PutUint16(srvData[0:2], 0)                  // Priority
	binary.BigEndian.PutUint16(srvData[2:4], 0)                  // Weight
	binary.BigEndian.PutUint16(srvData[4:6], uint16(m.httpPort)) // Port
	srvData = append(srvData, encodeDNSName(hostFQDN)...)
	buf = append(buf, encodeDNSRecord(instanceFQDN, dnsTypeSRV, dnsClassFlush, defaultTTL, srvData)...)

	// TXT 记录
	var txtData []byte
	for _, txt := range m.txtRecords {
		txtBytes := []byte(txt)
		txtData = append(txtData, byte(len(txtBytes)))
		txtData = append(txtData, txtBytes...)
	}
	buf = append(buf, encodeDNSRecord(instanceFQDN, dnsTypeTXT, dnsClassFlush, defaultTTL, txtData)...)

	// A 记录：每个本机 IP 一条
	for _, ip := range localIPs {
		ipBytes := ip.To4()
		if ipBytes == nil {
			continue
		}
		buf = append(buf, encodeDNSRecord(hostFQDN, dnsTypeA, dnsClassFlush, defaultTTL, ipBytes)...)
	}

	return buf
}

// encodeDNSName 将域名编码为 DNS 名称格式（长度前缀标签序列）。
func encodeDNSName(name string) []byte {
	var buf []byte
	parts := strings.Split(name, ".")
	for _, part := range parts {
		if part == "" {
			continue
		}
		buf = append(buf, byte(len(part)))
		buf = append(buf, []byte(part)...)
	}
	buf = append(buf, 0) // 终止符
	return buf
}

// encodeDNSRecord 编码一条 DNS 资源记录。
func encodeDNSRecord(name string, rrType, rrClass uint16, ttl uint32, rdata []byte) []byte {
	var buf []byte
	buf = append(buf, encodeDNSName(name)...)

	// Type
	typeBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(typeBuf, rrType)
	buf = append(buf, typeBuf...)

	// Class
	classBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(classBuf, rrClass)
	buf = append(buf, classBuf...)

	// TTL
	ttlBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(ttlBuf, ttl)
	buf = append(buf, ttlBuf...)

	// RDLENGTH + RDATA
	rdlenBuf := make([]byte, 2)
	binary.BigEndian.PutUint16(rdlenBuf, uint16(len(rdata)))
	buf = append(buf, rdlenBuf...)
	buf = append(buf, rdata...)

	return buf
}

// getLocalIPv4Addrs 获取所有非回环的本机 IPv4 地址。
func getLocalIPv4Addrs() []net.IP {
	var ips []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ipNet, ok := a.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ips = append(ips, ipNet.IP.To4())
		}
	}
	return ips
}
