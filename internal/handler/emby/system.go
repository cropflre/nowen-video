package emby

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

// 当前实现版本（声明我们兼容到 Emby 4.x 客户端）
const (
	embyServerVersion = "4.8.0.0"
	embyProductName   = "Emby Server"
)

// PingHandler 对应 /System/Ping 和 /emby/System/Ping。
// Emby 官方端这个接口返回单纯的字符串 "Emby Server"。
// Infuse 会用这个接口 (或 /System/Info/Public) 做服务器可达性探测。
func (h *Handler) PingHandler(c *gin.Context) {
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, embyProductName)
}

// SystemInfoPublicHandler 对应 /System/Info/Public，无需认证。
// 重要：Infuse 在添加服务器时首先打这个端点来识别服务。
func (h *Handler) SystemInfoPublicHandler(c *gin.Context) {
	info := SystemInfoPublic{
		LocalAddress:           resolveLocalAddress(c),
		ServerName:             h.resolveServerName(),
		Version:                embyServerVersion,
		ProductName:            embyProductName,
		OperatingSystem:        runtime.GOOS,
		Id:                     h.serverID,
		StartupWizardCompleted: true,
	}
	c.JSON(http.StatusOK, info)
}

// SystemInfoHandler 对应 /System/Info，需要认证。
func (h *Handler) SystemInfoHandler(c *gin.Context) {
	pub := SystemInfoPublic{
		LocalAddress:           resolveLocalAddress(c),
		ServerName:             h.resolveServerName(),
		Version:                embyServerVersion,
		ProductName:            embyProductName,
		OperatingSystem:        runtime.GOOS,
		Id:                     h.serverID,
		StartupWizardCompleted: true,
	}
	info := SystemInfo{
		SystemInfoPublic:           pub,
		OperatingSystemDisplayName: runtime.GOOS + "/" + runtime.GOARCH,
		HttpServerPortNumber:       h.cfg.App.Port,
		WebSocketPortNumber:        h.cfg.App.Port,
		SupportsLibraryMonitor:     true,
		CanSelfRestart:             false,
		CanSelfUpdate:              false,
		SystemArchitecture:         runtime.GOARCH,
		EncoderLocation:            "System",
		CompletedInstallations:     []string{},
	}
	c.JSON(http.StatusOK, info)
}

// SystemEndpointHandler 对应 /System/Endpoint，返回客户端的网络信息。
// Infuse 有时用这个端点判断是本地还是远程连接。
func (h *Handler) SystemEndpointHandler(c *gin.Context) {
	remote := c.ClientIP()
	isLocal := strings.HasPrefix(remote, "127.") ||
		strings.HasPrefix(remote, "10.") ||
		strings.HasPrefix(remote, "192.168.") ||
		remote == "::1"
	c.JSON(http.StatusOK, gin.H{
		"IsLocal":     isLocal,
		"IsInNetwork": isLocal,
	})
}

// WakeOnLanInfoHandler 对应 /System/WakeOnLanInfo。
// 官方 Emby Android/TV 客户端会把该接口按数组解析；返回 HTML 或字符串会触发
// Expected BEGIN_ARRAY but was STRING。没有 WOL 设备时返回空数组即可。
func (h *Handler) WakeOnLanInfoHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []gin.H{})
}

// BitrateTestHandler 对应 /Playback/BitrateTest?Size=...。
// 官方客户端用它做带宽探测，期望收到指定长度的二进制数据，而不是 HTML。
func (h *Handler) BitrateTestHandler(c *gin.Context) {
	size := atoiSafe(c.Query("Size"))
	if size <= 0 {
		size = atoiSafe(c.Query("size"))
	}
	if size <= 0 {
		size = 1024
	}
	// 防止异常参数导致内存分配过大；官方常见 5000000，这里给到 10MB 上限。
	if size > 10*1024*1024 {
		size = 10 * 1024 * 1024
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/octet-stream", make([]byte, size))
}

// WebManifestHandler 对应 /web/manifest.json。
// 官方客户端/内置 WebView 会请求此资源；返回最小 manifest JSON，避免落到 nowen 前端 HTML。
func (h *Handler) WebManifestHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":             embyProductName,
		"short_name":       "Emby",
		"start_url":        "/emby/web/index.html",
		"display":          "standalone",
		"background_color": "#101010",
		"theme_color":      "#52b54b",
		"icons":            []gin.H{},
	})
}

// ==================== 用户与认证 ====================

// PublicUsersHandler 对应 /Users/Public，返回可选用户列表（无需认证）。
//
// 行为取决于配置 Emby.PublicUserListEnabled：
//   - false（默认）：返回空数组，客户端走"手动输入用户名密码"流程，保护用户名隐私；
//   - true：         返回所有非隐藏、未禁用的用户，登录页可点击头像直接进入。
//
// 注意：仅返回用户标识信息，不含 Policy / Configuration 等详情。
func (h *Handler) PublicUsersHandler(c *gin.Context) {
	if !h.cfg.Emby.PublicUserListEnabled {
		c.JSON(http.StatusOK, []EmbyUser{})
		return
	}
	users, err := h.userRepo.List()
	if err != nil {
		c.JSON(http.StatusOK, []EmbyUser{})
		return
	}
	out := make([]EmbyUser, 0, len(users))
	for i := range users {
		u := &users[i]
		// 跳过已禁用的用户
		if u.Disabled {
			continue
		}
		out = append(out, EmbyUser{
			Name:                  u.Username,
			ServerId:              h.serverID,
			Id:                    h.idMap.ToEmbyID(u.ID),
			HasPassword:           true,
			HasConfiguredPassword: true,
		})
	}
	c.JSON(http.StatusOK, out)
}

// AuthenticateByNameRequest 对应 Emby 登录请求体。
type AuthenticateByNameRequest struct {
	Username string `json:"Username" form:"Username"`
	Pw       string `json:"Pw" form:"Pw"`             // 新版字段
	Password string `json:"Password" form:"Password"` // 旧版 MD5 字段（大部分客户端仍发送 Pw）
}

// AuthenticateByNameHandler 对应 /Users/AuthenticateByName。
// 这是 Infuse 登录入口。成功后返回 AuthenticationResult，其中 AccessToken 即为 JWT。
func (h *Handler) AuthenticateByNameHandler(c *gin.Context) {
	req, ok := parseAuthenticateByNameRequest(c)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Invalid request body"})
		return
	}
	password := req.Pw
	if password == "" {
		password = req.Password
	}
	if req.Username == "" || password == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"Error": "Username or password missing"})
		return
	}

	loginReq := &service.LoginRequest{Username: req.Username, Password: password}
	tok, err := h.auth.Login(loginReq, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		h.logger.Infof("[emby] 登录失败 user=%s err=%v", req.Username, err)
		c.JSON(http.StatusUnauthorized, gin.H{"Error": "Invalid username or password"})
		return
	}

	authHdr := parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
	embyUser := h.buildEmbyUser(tok.User.ID, tok.User.Username, tok.User.Role == "admin")
	sess := SessionInfo{
		Id:                 newSessionID(tok.User.ID, authHdr.DeviceId),
		UserId:             embyUser.Id,
		UserName:           embyUser.Name,
		Client:             authHdr.Client,
		DeviceName:         authHdr.Device,
		DeviceId:           authHdr.DeviceId,
		ApplicationVersion: authHdr.Version,
		ServerId:           h.serverID,
		IsActive:           true,
		PlayableMediaTypes: []string{"Video", "Audio"},
		SupportedCommands:  []string{},
		LastActivityDate:   formatEmbyTime(nowUTC()),
	}
	c.JSON(http.StatusOK, AuthenticationResult{
		User:        embyUser,
		AccessToken: tok.Token,
		ServerId:    h.serverID,
		SessionInfo: sess,
	})
}

// parseAuthenticateByNameRequest 兼容不同 Emby/Jellyfin 客户端的登录请求格式。
//
// 官方/Infuse 常见为 JSON：
//
//	{"Username":"admin","Pw":"password"}
//
// Kodi EmbyCon 等插件可能使用 application/x-www-form-urlencoded，甚至把字段放在 query 中。
// 这里不再只依赖 ShouldBindJSON，而是显式兼容：
//   - JSON body: Username/Pw/Password 及小写变体
//   - form body: Username/Pw/Password 及小写变体
//   - query:     Username/Pw/Password 及小写变体
func parseAuthenticateByNameRequest(c *gin.Context) (AuthenticateByNameRequest, bool) {
	var req AuthenticateByNameRequest
	body, _ := c.GetRawData()
	trimmed := strings.TrimSpace(string(body))

	if trimmed != "" && (strings.HasPrefix(trimmed, "{") || strings.Contains(strings.ToLower(c.GetHeader("Content-Type")), "json")) {
		var raw map[string]interface{}
		if err := json.Unmarshal(body, &raw); err == nil {
			req.Username = firstAuthValue(raw, "Username", "username", "UserName", "name")
			req.Pw = firstAuthValue(raw, "Pw", "pw", "Password", "password")
			req.Password = firstAuthValue(raw, "Password", "password", "Pw", "pw")
			if req.Username != "" || req.Pw != "" || req.Password != "" {
				return req, true
			}
		}
	}

	values := url.Values{}
	if trimmed != "" {
		if formValues, err := url.ParseQuery(trimmed); err == nil {
			for k, vals := range formValues {
				for _, v := range vals {
					values.Add(k, v)
				}
			}
		}
	}
	for k, vals := range c.Request.URL.Query() {
		for _, v := range vals {
			values.Add(k, v)
		}
	}

	req.Username = firstFormValue(values, "Username", "username", "UserName", "name")
	req.Pw = firstFormValue(values, "Pw", "pw", "Password", "password")
	req.Password = firstFormValue(values, "Password", "password", "Pw", "pw")
	return req, req.Username != "" || req.Pw != "" || req.Password != ""
}

func firstAuthValue(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func firstFormValue(values url.Values, keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(values.Get(key)); v != "" {
			return v
		}
	}
	return ""
}

// LogoutHandler 对应 /Sessions/Logout，Emby 客户端退出时调用。
// nowen 的 JWT 本身无状态，只需返回 204。
func (h *Handler) LogoutHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// SessionsHandler 对应 /Sessions。
// 官方 Emby 客户端在登录成功后通常会请求 /Sessions?DeviceId=... 来确认当前设备会话。
// 旧实现返回空数组会让官方客户端认为会话未建立，从而在登录后仍提示连接/添加失败。
func (h *Handler) SessionsHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	username := c.GetString("username")
	if userID == "" {
		abortEmbyUnauthorized(c, "No user")
		return
	}

	authHdr := parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
	if authHdr.Client == "" && c.GetHeader("Authorization") != "" {
		authHdr = parseEmbyAuthHeader(c.GetHeader("Authorization"))
	}

	deviceID := strings.TrimSpace(c.Query("DeviceId"))
	if deviceID == "" {
		deviceID = strings.TrimSpace(c.Query("deviceId"))
	}
	if deviceID == "" {
		deviceID = authHdr.DeviceId
	}
	if deviceID == "" {
		deviceID = "nowen-device"
	}

	client := authHdr.Client
	if client == "" {
		client = "Emby"
	}
	device := authHdr.Device
	if device == "" {
		device = deviceID
	}
	version := authHdr.Version
	if version == "" {
		version = "4.8.0.0"
	}

	embyUserID := h.idMap.ToEmbyID(userID)
	sess := SessionInfo{
		Id:                    newSessionID(userID, deviceID),
		UserId:                embyUserID,
		UserName:              username,
		Client:                client,
		DeviceName:            device,
		DeviceId:              deviceID,
		ApplicationVersion:    version,
		ServerId:              h.serverID,
		IsActive:              true,
		SupportsRemoteControl: true,
		PlayableMediaTypes:    []string{"Video", "Audio"},
		SupportedCommands:     []string{},
		LastActivityDate:      formatEmbyTime(nowUTC()),
	}
	c.JSON(http.StatusOK, []SessionInfo{sess})
}

// GetCurrentUserHandler 对应 /Users/Me 和 /Users/{id}。
// Infuse 登录后立即调用 /Users/Me 拉取当前用户详情，必须返回完整的 Policy。
func (h *Handler) GetCurrentUserHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		abortEmbyUnauthorized(c, "No user")
		return
	}
	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, h.buildEmbyUser(user.ID, user.Username, user.Role == "admin"))
}

// GetUserByIDHandler 对应 /Users/{id}。
// 对于非本人查询，非管理员返回 403。
func (h *Handler) GetUserByIDHandler(c *gin.Context) {
	targetID := h.idMap.Resolve(c.Param("id"))
	selfID := c.GetString("user_id")
	role := c.GetString("role")
	if targetID != selfID && role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"Error": "Forbidden"})
		return
	}
	user, err := h.userRepo.FindByID(targetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, h.buildEmbyUser(user.ID, user.Username, user.Role == "admin"))
}

// ListUsersHandler 对应 /Users，只允许管理员。
func (h *Handler) ListUsersHandler(c *gin.Context) {
	role := c.GetString("role")
	if role != "admin" {
		// 非管理员只能看到自己
		userID := c.GetString("user_id")
		u, err := h.userRepo.FindByID(userID)
		if err != nil {
			c.JSON(http.StatusOK, []EmbyUser{})
			return
		}
		c.JSON(http.StatusOK, []EmbyUser{h.buildEmbyUser(u.ID, u.Username, u.Role == "admin")})
		return
	}
	users, err := h.userRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to list users"})
		return
	}
	out := make([]EmbyUser, 0, len(users))
	for i := range users {
		out = append(out, h.buildEmbyUser(users[i].ID, users[i].Username, users[i].Role == "admin"))
	}
	c.JSON(http.StatusOK, out)
}

// buildEmbyUser 把 nowen User 构造成完整的 Emby User（含 Policy + Configuration）。
func (h *Handler) buildEmbyUser(userID, username string, isAdmin bool) EmbyUser {
	embyID := h.idMap.ToEmbyID(userID)
	return EmbyUser{
		Name:                      username,
		ServerId:                  h.serverID,
		Id:                        embyID,
		HasPassword:               true,
		HasConfiguredPassword:     true,
		HasConfiguredEasyPassword: false,
		EnableAutoLogin:           false,
		LastLoginDate:             formatEmbyTime(nowUTC()),
		LastActivityDate:          formatEmbyTime(nowUTC()),
		Configuration: UserConfig{
			AudioLanguagePreference:    "",
			PlayDefaultAudioTrack:      true,
			SubtitleLanguagePreference: "",
			DisplayMissingEpisodes:     false,
			SubtitleMode:               "Default",
			EnableNextEpisodeAutoPlay:  true,
			RememberAudioSelections:    true,
			RememberSubtitleSelections: true,
			GroupedFolders:             []string{},
			OrderedViews:               []string{},
			LatestItemsExcludes:        []string{},
			MyMediaExcludes:            []string{},
		},
		Policy: UserPolicy{
			IsAdministrator:                isAdmin,
			IsHidden:                       false,
			IsDisabled:                     false,
			EnableUserPreferenceAccess:     true,
			EnableRemoteAccess:             true,
			EnableMediaPlayback:            true,
			EnableAudioPlaybackTranscoding: true,
			EnableVideoPlaybackTranscoding: true,
			EnablePlaybackRemuxing:         true,
			EnableContentDeletion:          isAdmin,
			EnableContentDownloading:       true,
			EnableSubtitleDownloading:      true,
			EnableSubtitleManagement:       isAdmin,
			EnableAllDevices:               true,
			EnableAllChannels:              true,
			EnableAllFolders:               true,
			EnablePublicSharing:            false,
			EnableLiveTvAccess:             false,
			EnableLiveTvManagement:         false,
			InvalidLoginAttemptCount:       0,
			AuthenticationProviderId:       "Emby.Server.Implementations.Library.DefaultAuthenticationProvider",
			BlockedTags:                    []string{},
			EnabledChannels:                []string{},
			EnabledFolders:                 []string{},
			EnabledDevices:                 []string{},
		},
	}
}

// resolveLocalAddress 构造给客户端回写的 LocalAddress（Emby 用它做跨网段切换）。
func resolveLocalAddress(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	} else if xf := c.GetHeader("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}
	return scheme + "://" + host
}

func nvl(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// resolveServerName 计算对外展示的 ServerName。
// 优先级：配置 Emby.ServerName > 主机名 > "nowen-video"。
func (h *Handler) resolveServerName() string {
	if name := strings.TrimSpace(h.cfg.Emby.ServerName); name != "" {
		return name
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		return hostname
	}
	return "nowen-video"
}
