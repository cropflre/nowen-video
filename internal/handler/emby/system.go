package emby

import (
	"net/http"
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
	hostname, _ := os.Hostname()
	info := SystemInfoPublic{
		LocalAddress:           resolveLocalAddress(c),
		ServerName:             nvl(hostname, "nowen-video"),
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
	hostname, _ := os.Hostname()
	pub := SystemInfoPublic{
		LocalAddress:           resolveLocalAddress(c),
		ServerName:             nvl(hostname, "nowen-video"),
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

// ==================== 用户与认证 ====================

// PublicUsersHandler 对应 /Users/Public，返回可选用户列表（无需认证）。
// 为了防止暴露所有用户名，默认返回空数组——Emby 客户端会自动走 "手动输入用户名密码" 流程。
func (h *Handler) PublicUsersHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []EmbyUser{})
}

// AuthenticateByNameRequest 对应 Emby 登录请求体。
type AuthenticateByNameRequest struct {
	Username string `json:"Username"`
	Pw       string `json:"Pw"`       // 新版字段
	Password string `json:"Password"` // 旧版 MD5 字段（大部分客户端仍发送 Pw）
}

// AuthenticateByNameHandler 对应 /Users/AuthenticateByName。
// 这是 Infuse 登录入口。成功后返回 AuthenticationResult，其中 AccessToken 即为 JWT。
func (h *Handler) AuthenticateByNameHandler(c *gin.Context) {
	var req AuthenticateByNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

// LogoutHandler 对应 /Sessions/Logout，Emby 客户端退出时调用。
// nowen 的 JWT 本身无状态，只需返回 204。
func (h *Handler) LogoutHandler(c *gin.Context) {
	c.Status(http.StatusNoContent)
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
