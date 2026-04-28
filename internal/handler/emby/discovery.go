package emby

// discovery.go 实现 Emby / Jellyfin 的 UDP 局域网服务器自动发现协议。
//
// 工作原理：
//   客户端（Emby/Jellyfin iOS/Android/Web）在"添加服务器"时，
//   会向同网段的 UDP 255.255.255.255:7359 广播一条文本探测包，
//   常见内容为 "who is EmbyServer?" 或 "why are you here?"。
//
//   服务器收到后回一段 JSON：
//     {"Address":"http://<ip>:<port>","Id":"<serverId>","Name":"<serverName>","EndpointAddress":null}
//
//   客户端据此在"添加服务器"列表中展示本机。
//
// 这是让移动端"无需手动输入 IP"的关键。

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
)

// DiscoveryService 负责在 UDP 端口上响应客户端发现探测。
type DiscoveryService struct {
	port       int
	serverID   string
	serverName string
	httpPort   int
	logger     *zap.SugaredLogger

	conn   *net.UDPConn
	cancel context.CancelFunc
}

// NewDiscoveryService 构造服务发现对象。
//   - port:        UDP 监听端口（默认 7359）
//   - serverID:    稳定的 Emby ServerId（deriveServerID 结果）
//   - serverName:  回包中的 ServerName（留空则使用主机名）
//   - httpPort:    HTTP 服务端口，用于回包中的 Address
func NewDiscoveryService(port int, serverID, serverName string, httpPort int, logger *zap.SugaredLogger) *DiscoveryService {
	if serverName == "" {
		if h, err := os.Hostname(); err == nil && h != "" {
			serverName = h
		} else {
			serverName = "nowen-video"
		}
	}
	if port <= 0 {
		port = 7359
	}
	return &DiscoveryService{
		port:       port,
		serverID:   serverID,
		serverName: serverName,
		httpPort:   httpPort,
		logger:     logger,
	}
}

// Start 启动 UDP 监听。返回 nil 表示成功启动。
// 已启动时重复调用会被忽略。
func (d *DiscoveryService) Start() error {
	if d.conn != nil {
		return nil
	}
	addr := &net.UDPAddr{IP: net.IPv4zero, Port: d.port}
	conn, err := net.ListenUDP("udp4", addr)
	if err != nil {
		return fmt.Errorf("UDP %d 监听失败: %w", d.port, err)
	}
	d.conn = conn

	ctx, cancel := context.WithCancel(context.Background())
	d.cancel = cancel

	go d.serve(ctx)
	if d.logger != nil {
		d.logger.Infof("[emby] 服务器自动发现已启动 (UDP :%d)", d.port)
	}
	return nil
}

// Stop 关闭 UDP 监听。
func (d *DiscoveryService) Stop() {
	if d.cancel != nil {
		d.cancel()
	}
	if d.conn != nil {
		_ = d.conn.Close()
		d.conn = nil
	}
}

// serve 循环接收探测包并回复。
func (d *DiscoveryService) serve(ctx context.Context) {
	buf := make([]byte, 1024)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		// 设置读超时以便周期性检查 ctx
		_ = d.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remote, err := d.conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			// 连接被关闭或其他错误
			if ctx.Err() != nil {
				return
			}
			if d.logger != nil {
				d.logger.Debugf("[emby] 服务发现读取失败: %v", err)
			}
			continue
		}
		msg := strings.ToLower(strings.TrimSpace(string(buf[:n])))
		if !isDiscoveryProbe(msg) {
			// 非发现探测包，忽略（避免被其他 UDP 流量干扰）
			continue
		}
		d.respond(remote)
	}
}

// isDiscoveryProbe 判断是否为 Emby/Jellyfin 发现探测。
// Emby 官方:    "who is EmbyServer?"
// Jellyfin:    "why are you here?"
// Emby Web 老: "who is JellyfinServer?"
// 这里采用宽松匹配策略，只要包含任一关键字段即响应。
func isDiscoveryProbe(msg string) bool {
	keywords := []string{
		"who is embyserver",
		"who is jellyfinserver",
		"why are you here",
		"emby server",
		"jellyfin server",
	}
	for _, k := range keywords {
		if strings.Contains(msg, k) {
			return true
		}
	}
	return false
}

// respond 向客户端回发一段 JSON 标识。
func (d *DiscoveryService) respond(remote *net.UDPAddr) {
	// 选一个能被客户端访问的本机 IP（优先与 remote 同网段）
	localIP := preferredLocalIP(remote)
	address := fmt.Sprintf("http://%s:%d", localIP, d.httpPort)

	payload := map[string]interface{}{
		"Address":         address,
		"Id":              d.serverID,
		"Name":            d.serverName,
		"EndpointAddress": nil,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if _, err := d.conn.WriteToUDP(data, remote); err != nil {
		if d.logger != nil {
			d.logger.Debugf("[emby] 服务发现回包失败 remote=%s: %v", remote, err)
		}
		return
	}
	if d.logger != nil {
		d.logger.Debugf("[emby] 服务发现已响应 remote=%s address=%s", remote, address)
	}
}

// preferredLocalIP 选择最适合回给 remote 的本机 IPv4 地址。
// 策略：优先同一 /24 子网；退而求其次选第一个非回环 IPv4。
func preferredLocalIP(remote *net.UDPAddr) string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "127.0.0.1"
	}
	var fallback string
	remoteIP := ""
	if remote != nil && remote.IP != nil {
		remoteIP = remote.IP.To4().String()
	}
	remotePrefix := ipClassCPrefix(remoteIP)

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
			ipStr := ipNet.IP.To4().String()
			if fallback == "" {
				fallback = ipStr
			}
			// 同 /24 子网最优
			if remotePrefix != "" && ipClassCPrefix(ipStr) == remotePrefix {
				return ipStr
			}
		}
	}
	if fallback != "" {
		return fallback
	}
	return "127.0.0.1"
}

// ipClassCPrefix 取 IPv4 的前三段（a.b.c），非法返回空串。
func ipClassCPrefix(ip string) string {
	parts := strings.Split(ip, ".")
	if len(parts) != 4 {
		return ""
	}
	return parts[0] + "." + parts[1] + "." + parts[2]
}
