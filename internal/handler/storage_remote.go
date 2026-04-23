package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/config"
)

// ==================== V2.3: Alist / S3 存储管理 API ====================
//
// 复用 StorageHandler 的 remoteStorageService 与 cfg，提供：
//   GET  /api/admin/storage/alist
//   PUT  /api/admin/storage/alist
//   POST /api/admin/storage/alist/test
//   GET  /api/admin/storage/s3
//   PUT  /api/admin/storage/s3
//   POST /api/admin/storage/s3/test

// ---------- Alist ----------

// GetAlistConfig 获取 Alist 配置（密码 / Token 脱敏）
func (h *StorageHandler) GetAlistConfig(c *gin.Context) {
	cfg := h.cfg.Storage.Alist
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"enabled":            cfg.Enabled,
		"server_url":         cfg.ServerURL,
		"username":           cfg.Username,
		"password":           h.maskPassword(cfg.Password),
		"token":              h.maskPassword(cfg.Token),
		"base_path":          cfg.BasePath,
		"timeout":            cfg.Timeout,
		"enable_cache":       cfg.EnableCache,
		"cache_ttl_hours":    cfg.CacheTTLHours,
		"read_block_size_mb": cfg.ReadBlockSizeMB,
		"read_block_count":   cfg.ReadBlockCount,
	}})
}

// UpdateAlistConfigRequest 更新 Alist 配置
type UpdateAlistConfigRequest struct {
	Enabled         bool   `json:"enabled"`
	ServerURL       string `json:"server_url"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	Token           string `json:"token"`
	BasePath        string `json:"base_path"`
	Timeout         int    `json:"timeout"`
	EnableCache     bool   `json:"enable_cache"`
	CacheTTLHours   int    `json:"cache_ttl_hours"`
	ReadBlockSizeMB int    `json:"read_block_size_mb"`
	ReadBlockCount  int    `json:"read_block_count"`
}

// UpdateAlistConfig 更新 Alist 配置
func (h *StorageHandler) UpdateAlistConfig(c *gin.Context) {
	var req UpdateAlistConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if req.Enabled && req.ServerURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "启用 Alist 时必须提供服务器地址"})
		return
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	newCfg := config.AlistConfig{
		Enabled:         req.Enabled,
		ServerURL:       strings.TrimRight(req.ServerURL, "/"),
		Username:        req.Username,
		Password:        req.Password,
		Token:           req.Token,
		BasePath:        req.BasePath,
		Timeout:         req.Timeout,
		EnableCache:     req.EnableCache,
		CacheTTLHours:   req.CacheTTLHours,
		ReadBlockSizeMB: req.ReadBlockSizeMB,
		ReadBlockCount:  req.ReadBlockCount,
	}
	// 空字符串沿用原值
	old := h.cfg.Storage.Alist
	if newCfg.Password == "" {
		newCfg.Password = old.Password
	}
	if newCfg.Token == "" {
		newCfg.Token = old.Token
	}

	if err := h.remoteStorageService.UpdateAlistConfig(newCfg); err != nil {
		h.logger.Errorf("更新 Alist 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	h.cfg.Storage.Alist = newCfg
	c.JSON(http.StatusOK, gin.H{"message": "Alist 配置已更新"})
}

// TestAlistRequest 测试 Alist 连接
type TestAlistRequest struct {
	ServerURL string `json:"server_url" binding:"required"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Token     string `json:"token"`
	BasePath  string `json:"base_path"`
}

// TestAlistConnection 测试 Alist 连接
func (h *StorageHandler) TestAlistConnection(c *gin.Context) {
	var req TestAlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.remoteStorageService.TestAlist(config.AlistConfig{
		Enabled:   true,
		ServerURL: strings.TrimRight(req.ServerURL, "/"),
		Username:  req.Username,
		Password:  req.Password,
		Token:     req.Token,
		BasePath:  req.BasePath,
		Timeout:   30,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接测试失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Alist 连接测试成功"})
}

// ---------- S3 ----------

// GetS3Config 获取 S3 配置（密钥脱敏）
func (h *StorageHandler) GetS3Config(c *gin.Context) {
	cfg := h.cfg.Storage.S3
	c.JSON(http.StatusOK, gin.H{"data": gin.H{
		"enabled":            cfg.Enabled,
		"endpoint":           cfg.Endpoint,
		"region":             cfg.Region,
		"access_key":         cfg.AccessKey,
		"secret_key":         h.maskPassword(cfg.SecretKey),
		"bucket":             cfg.Bucket,
		"base_path":          cfg.BasePath,
		"path_style":         cfg.PathStyle,
		"timeout":            cfg.Timeout,
		"enable_cache":       cfg.EnableCache,
		"cache_ttl_hours":    cfg.CacheTTLHours,
		"read_block_size_mb": cfg.ReadBlockSizeMB,
		"read_block_count":   cfg.ReadBlockCount,
	}})
}

// UpdateS3ConfigRequest 更新 S3 配置
type UpdateS3ConfigRequest struct {
	Enabled         bool   `json:"enabled"`
	Endpoint        string `json:"endpoint"`
	Region          string `json:"region"`
	AccessKey       string `json:"access_key"`
	SecretKey       string `json:"secret_key"`
	Bucket          string `json:"bucket"`
	BasePath        string `json:"base_path"`
	PathStyle       bool   `json:"path_style"`
	Timeout         int    `json:"timeout"`
	EnableCache     bool   `json:"enable_cache"`
	CacheTTLHours   int    `json:"cache_ttl_hours"`
	ReadBlockSizeMB int    `json:"read_block_size_mb"`
	ReadBlockCount  int    `json:"read_block_count"`
}

// UpdateS3Config 更新 S3 配置
func (h *StorageHandler) UpdateS3Config(c *gin.Context) {
	var req UpdateS3ConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if req.Enabled {
		if req.Endpoint == "" || req.Bucket == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "启用 S3 时必须提供 Endpoint 和 Bucket"})
			return
		}
	}
	if req.Timeout <= 0 {
		req.Timeout = 30
	}

	newCfg := config.S3Config{
		Enabled:         req.Enabled,
		Endpoint:        strings.TrimRight(req.Endpoint, "/"),
		Region:          req.Region,
		AccessKey:       req.AccessKey,
		SecretKey:       req.SecretKey,
		Bucket:          req.Bucket,
		BasePath:        req.BasePath,
		PathStyle:       req.PathStyle,
		Timeout:         req.Timeout,
		EnableCache:     req.EnableCache,
		CacheTTLHours:   req.CacheTTLHours,
		ReadBlockSizeMB: req.ReadBlockSizeMB,
		ReadBlockCount:  req.ReadBlockCount,
	}
	old := h.cfg.Storage.S3
	if newCfg.SecretKey == "" {
		newCfg.SecretKey = old.SecretKey
	}

	if err := h.remoteStorageService.UpdateS3Config(newCfg); err != nil {
		h.logger.Errorf("更新 S3 配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新失败: " + err.Error()})
		return
	}
	h.cfg.Storage.S3 = newCfg
	c.JSON(http.StatusOK, gin.H{"message": "S3 配置已更新"})
}

// TestS3Request 测试 S3 连接
type TestS3Request struct {
	Endpoint  string `json:"endpoint" binding:"required"`
	Region    string `json:"region"`
	AccessKey string `json:"access_key" binding:"required"`
	SecretKey string `json:"secret_key" binding:"required"`
	Bucket    string `json:"bucket" binding:"required"`
	BasePath  string `json:"base_path"`
	PathStyle bool   `json:"path_style"`
}

// TestS3Connection 测试 S3 连接
func (h *StorageHandler) TestS3Connection(c *gin.Context) {
	var req TestS3Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	if err := h.remoteStorageService.TestS3(config.S3Config{
		Enabled:   true,
		Endpoint:  strings.TrimRight(req.Endpoint, "/"),
		Region:    req.Region,
		AccessKey: req.AccessKey,
		SecretKey: req.SecretKey,
		Bucket:    req.Bucket,
		BasePath:  req.BasePath,
		PathStyle: req.PathStyle,
		Timeout:   30,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "连接测试失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "S3 连接测试成功"})
}
