package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// ==================== WebDAV 存储管理 ====================

// StorageHandler 存储管理处理器
type StorageHandler struct {
	webdavService        *service.WebDAVService
	remoteStorageService *service.RemoteStorageService // V2.3: Alist / S3
	cfg                  *config.Config
	logger               *zap.SugaredLogger
}

// NewStorageHandler 创建存储管理处理器
func NewStorageHandler(webdavService *service.WebDAVService, remoteStorageService *service.RemoteStorageService, cfg *config.Config, logger *zap.SugaredLogger) *StorageHandler {
	return &StorageHandler{
		webdavService:        webdavService,
		remoteStorageService: remoteStorageService,
		cfg:                  cfg,
		logger:               logger,
	}
}

// GetWebDAVConfig 获取 WebDAV 配置
func (h *StorageHandler) GetWebDAVConfig(c *gin.Context) {
	webdavCfg := h.cfg.Storage.WebDAV

	// 返回配置（密码字段需要掩码处理）
	response := gin.H{
		"enabled":         webdavCfg.Enabled,
		"server_url":      webdavCfg.ServerURL,
		"username":        webdavCfg.Username,
		"password":        h.maskPassword(webdavCfg.Password),
		"base_path":       webdavCfg.BasePath,
		"timeout":         webdavCfg.Timeout,
		"enable_pool":     webdavCfg.EnablePool,
		"pool_size":       webdavCfg.PoolSize,
		"enable_cache":    webdavCfg.EnableCache,
		"cache_ttl_hours": webdavCfg.CacheTTLHours,
		"max_retries":     webdavCfg.MaxRetries,
		"retry_interval":  webdavCfg.RetryInterval,
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// UpdateWebDAVConfigRequest 更新 WebDAV 配置请求
type UpdateWebDAVConfigRequest struct {
	Enabled       bool   `json:"enabled"`
	ServerURL     string `json:"server_url" binding:"required_if=Enabled true"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	BasePath      string `json:"base_path"`
	Timeout       int    `json:"timeout"`
	EnablePool    bool   `json:"enable_pool"`
	PoolSize      int    `json:"pool_size"`
	EnableCache   bool   `json:"enable_cache"`
	CacheTTLHours int    `json:"cache_ttl_hours"`
	MaxRetries    int    `json:"max_retries"`
	RetryInterval int    `json:"retry_interval"`
}

// UpdateWebDAVConfig 更新 WebDAV 配置
func (h *StorageHandler) UpdateWebDAVConfig(c *gin.Context) {
	var req UpdateWebDAVConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// 验证配置
	if req.Enabled {
		if req.ServerURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "启用 WebDAV 时必须提供服务器地址"})
			return
		}
		if req.Timeout <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "超时时间必须大于0"})
			return
		}
	}

	// 更新配置
	newCfg := config.WebDAVConfig{
		Enabled:       req.Enabled,
		ServerURL:     req.ServerURL,
		Username:      req.Username,
		Password:      req.Password,
		BasePath:      req.BasePath,
		Timeout:       req.Timeout,
		EnablePool:    req.EnablePool,
		PoolSize:      req.PoolSize,
		EnableCache:   req.EnableCache,
		CacheTTLHours: req.CacheTTLHours,
		MaxRetries:    req.MaxRetries,
		RetryInterval: req.RetryInterval,
	}

	// 如果密码为空，保持原密码
	if req.Password == "" {
		newCfg.Password = h.cfg.Storage.WebDAV.Password
	}

	// 更新服务配置
	if err := h.webdavService.UpdateConfig(newCfg); err != nil {
		h.logger.Errorf("更新 WebDAV 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新配置失败: " + err.Error()})
		return
	}

	// 更新主配置
	h.cfg.Storage.WebDAV = newCfg

	c.JSON(http.StatusOK, gin.H{"message": "WebDAV 配置已更新"})
}

// TestWebDAVConnectionRequest 测试 WebDAV 连接请求
type TestWebDAVConnectionRequest struct {
	ServerURL string `json:"server_url" binding:"required"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	BasePath  string `json:"base_path"`
}

// TestWebDAVConnection 测试 WebDAV 连接
func (h *StorageHandler) TestWebDAVConnection(c *gin.Context) {
	var req TestWebDAVConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	testCfg := config.WebDAVConfig{
		Enabled:   true,
		ServerURL: req.ServerURL,
		Username:  req.Username,
		Password:  req.Password,
		BasePath:  req.BasePath,
		Timeout:   30,
	}

	if err := h.webdavService.TestConnection(testCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接测试失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "WebDAV 连接测试成功"})
}

// GetWebDAVStatus 获取 WebDAV 服务状态
func (h *StorageHandler) GetWebDAVStatus(c *gin.Context) {
	status := h.webdavService.GetStatus()
	c.JSON(http.StatusOK, gin.H{"data": status})
}

// RegisterWebDAVLibraryRequest 为媒体库注册 WebDAV 存储请求
type RegisterWebDAVLibraryRequest struct {
	LibraryID string `json:"library_id" binding:"required"`
}

// RegisterWebDAVLibrary 为指定媒体库注册 WebDAV 存储
func (h *StorageHandler) RegisterWebDAVLibrary(c *gin.Context) {
	var req RegisterWebDAVLibraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.webdavService.RegisterWebDAVLibrary(req.LibraryID); err != nil {
		h.logger.Errorf("为媒体库注册 WebDAV 存储失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "注册失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "媒体库已成功注册 WebDAV 存储"})
}

// GetStorageStatus 获取所有存储状态
func (h *StorageHandler) GetStorageStatus(c *gin.Context) {
	webdavStatus := h.webdavService.GetStatus()
	var remoteStatus map[string]interface{}
	if h.remoteStorageService != nil {
		remoteStatus = h.remoteStorageService.GetStatus()
	}

	response := gin.H{
		"webdav": webdavStatus,
		"local": gin.H{
			"enabled": true,
			"type":    "local",
		},
	}
	if remoteStatus != nil {
		response["alist"] = remoteStatus["alist"]
		response["s3"] = remoteStatus["s3"]
	}

	c.JSON(http.StatusOK, gin.H{"data": response})
}

// maskPassword 密码掩码处理
func (h *StorageHandler) maskPassword(password string) string {
	if password == "" {
		return ""
	}
	if len(password) <= 8 {
		return strings.Repeat("*", len(password))
	}
	return password[:4] + strings.Repeat("*", len(password)-8) + password[len(password)-4:]
}
