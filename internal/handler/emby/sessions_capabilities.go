package emby

import (
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

// ==================== Sessions/Capabilities ====================
//
// Hills（Android / Windows）与 Yamby（Android）这类 Emby 第三方客户端，
// 在 /Users/AuthenticateByName 登录成功后会**立即** POST
// /Sessions/Capabilities/Full，把自身的 DeviceProfile（支持的容器 / 视频 / 音频 /
// 字幕编码）与 SupportedCommands 上报给服务端。官方 Emby 对该请求返回 204。
//
// 本兼容层此前没有实现该端点，请求会落空（最终被前端静态资源兜底成 200 HTML），
// 客户端解析失败后可能判定服务器不兼容，表现为"能登录、但进不去或播放异常"。
//
// 这里实现为：解析并记录上报内容（进程内内存态，供排障与后续能力判定复用），
// 返回 204。未调用该端点的客户端（如 Infuse）行为完全不受影响。

// DeviceProfileRequest 是 Emby DeviceProfile 的最小可用子集。
// 只解析我们真正关心的字段，其余字段直接忽略（客户端不依赖服务端回显）。
type DeviceProfileRequest struct {
	MaxStreamingBitrate int `json:"MaxStreamingBitrate"`
	DirectPlayProfiles  []struct {
		Container  string `json:"Container"`
		VideoCodec string `json:"VideoCodec"`
		AudioCodec string `json:"AudioCodec"`
		Type       string `json:"Type"`
	} `json:"DirectPlayProfiles"`
	TranscodingProfiles []struct {
		Container string `json:"Container"`
		Type      string `json:"Type"`
	} `json:"TranscodingProfiles"`
}

// SessionCapabilitiesRequest 对应 POST /Sessions/Capabilities[/Full] 的请求体。
type SessionCapabilitiesRequest struct {
	PlayableMediaTypes           []string              `json:"PlayableMediaTypes"`
	SupportedCommands            []string              `json:"SupportedCommands"`
	SupportsMediaControl         bool                  `json:"SupportsMediaControl"`
	SupportsPersistentIdentifier bool                  `json:"SupportsPersistentIdentifier"`
	DeviceProfile                *DeviceProfileRequest `json:"DeviceProfile"`
}

// clientCapabilityStore 记录各 DeviceId 上报的播放能力。
// 客户端每次启动都会重新上报，因此进程内内存态即可，无需持久化。
type clientCapabilityStore struct {
	mu   sync.RWMutex
	data map[string]SessionCapabilitiesRequest
}

func newClientCapabilityStore() *clientCapabilityStore {
	return &clientCapabilityStore{data: make(map[string]SessionCapabilitiesRequest)}
}

func (s *clientCapabilityStore) set(deviceID string, caps SessionCapabilitiesRequest) {
	if s == nil || deviceID == "" {
		return
	}
	s.mu.Lock()
	s.data[deviceID] = caps
	s.mu.Unlock()
}

func (s *clientCapabilityStore) get(deviceID string) (SessionCapabilitiesRequest, bool) {
	if s == nil {
		return SessionCapabilitiesRequest{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	caps, ok := s.data[deviceID]
	return caps, ok
}

// SessionsCapabilitiesHandler 对应 POST /Sessions/Capabilities。
func (h *Handler) SessionsCapabilitiesHandler(c *gin.Context) {
	h.recordClientCapabilities(c, "Capabilities")
	c.Status(http.StatusNoContent)
}

// SessionsCapabilitiesFullHandler 对应 POST /Sessions/Capabilities/Full。
func (h *Handler) SessionsCapabilitiesFullHandler(c *gin.Context) {
	h.recordClientCapabilities(c, "Capabilities/Full")
	c.Status(http.StatusNoContent)
}

// recordClientCapabilities 解析并保存本次能力上报，同时记录客户端标识便于排障。
// 无论解析是否成功都返回 nil —— 能力上报失败不应阻断客户端初始化。
func (h *Handler) recordClientCapabilities(c *gin.Context, endpoint string) {
	deviceID := c.GetString("emby_device_id")
	client := c.GetString("emby_client")
	if deviceID == "" || client == "" {
		ah := parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
		if deviceID == "" {
			deviceID = ah.DeviceId
		}
		if client == "" {
			client = ah.Client
		}
	}

	var req SessionCapabilitiesRequest
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			h.logger.Debugf("[emby] %s 能力上报解析失败 device=%s err=%v", endpoint, deviceID, err)
		}
	}
	h.caps.set(deviceID, req)

	directPlayCount, container := 0, ""
	if req.DeviceProfile != nil {
		directPlayCount = len(req.DeviceProfile.DirectPlayProfiles)
		if len(req.DeviceProfile.DirectPlayProfiles) > 0 {
			container = req.DeviceProfile.DirectPlayProfiles[0].Container
		}
	}
	h.logger.Infof("[emby] %s 客户端能力上报 client=%q deviceId=%q directPlayProfiles=%d firstContainer=%q",
		endpoint, client, deviceID, directPlayCount, container)
}
