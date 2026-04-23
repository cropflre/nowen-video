package service

import (
	"fmt"
	"sync"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// ==================== WebDAV 服务 ====================

// WebDAVService WebDAV 存储服务
// 负责管理 WebDAV 存储的初始化、连接池和错误处理
type WebDAVService struct {
	config *config.Config
	logger *zap.SugaredLogger
	vfsMgr *VFSManager
	mu     sync.RWMutex

	// WebDAV 客户端缓存（按配置哈希缓存）
	clients map[string]*WebDAVFS
}

// NewWebDAVService 创建 WebDAV 服务实例
func NewWebDAVService(cfg *config.Config, logger *zap.SugaredLogger, vfsMgr *VFSManager) *WebDAVService {
	return &WebDAVService{
		config:  cfg,
		logger:  logger,
		vfsMgr:  vfsMgr,
		clients: make(map[string]*WebDAVFS),
	}
}

// Initialize 初始化 WebDAV 服务
func (s *WebDAVService) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	webdavCfg := s.config.Storage.WebDAV

	// 检查是否启用 WebDAV
	if !webdavCfg.Enabled {
		s.logger.Info("WebDAV 存储未启用")
		// 确保全局 VFS 被清空
		s.vfsMgr.SetGlobalWebDAVFS(nil)
		return nil
	}

	// 验证必要配置
	if webdavCfg.ServerURL == "" {
		return fmt.Errorf("WebDAV 服务器地址不能为空")
	}

	// 创建 WebDAV 文件系统实例（使用配置中的超时和缓存选项）
	webdavFS := NewWebDAVFSWithOptions(
		webdavCfg.ServerURL,
		webdavCfg.Username,
		webdavCfg.Password,
		webdavCfg.BasePath,
		webdavCfg.Timeout,
		webdavCfg.CacheTTLHours,
		webdavCfg.EnableCache,
		s.logger,
	)

	// V2.1: 应用 ReadAt 块缓存配置（播放器 seek 加速）
	blockSize := int64(webdavCfg.ReadBlockSizeMB) * 1024 * 1024
	webdavFS.SetBlockCacheConfig(blockSize, webdavCfg.ReadBlockCount)

	// 缓存客户端
	s.clients[s.getConfigHash(webdavCfg)] = webdavFS

	// 注入到 VFSManager 作为全局 WebDAV FS（webdav:// 路径会自动路由到它）
	s.vfsMgr.SetGlobalWebDAVFS(webdavFS)

	// 测试连接
	s.logger.Infof("正在测试 WebDAV 连接: %s", webdavCfg.ServerURL)
	if err := s.testConnection(webdavFS); err != nil {
		s.logger.Warnf("WebDAV 连接测试失败: %v", err)
		return fmt.Errorf("WebDAV 连接测试失败: %w", err)
	}

	s.logger.Infof("WebDAV 服务初始化成功（TTL=%dh, Cache=%v）: %s",
		webdavCfg.CacheTTLHours, webdavCfg.EnableCache, webdavCfg.ServerURL)
	return nil
}

// GetWebDAVFS 获取 WebDAV 文件系统实例
func (s *WebDAVService) GetWebDAVFS() (VFS, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	webdavCfg := s.config.Storage.WebDAV
	if !webdavCfg.Enabled {
		return nil, fmt.Errorf("WebDAV 存储未启用")
	}

	configHash := s.getConfigHash(webdavCfg)
	if client, exists := s.clients[configHash]; exists {
		return client, nil
	}

	return nil, fmt.Errorf("未找到对应的 WebDAV 客户端")
}

// RegisterWebDAVLibrary 为指定媒体库注册 WebDAV 文件系统
func (s *WebDAVService) RegisterWebDAVLibrary(libraryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	webdavCfg := s.config.Storage.WebDAV
	if !webdavCfg.Enabled {
		return fmt.Errorf("WebDAV 存储未启用")
	}

	configHash := s.getConfigHash(webdavCfg)
	webdavFS, exists := s.clients[configHash]
	if !exists {
		return fmt.Errorf("WebDAV 客户端未初始化")
	}

	// 注册到 VFS 管理器
	s.vfsMgr.RegisterFS(libraryID, webdavFS)
	s.logger.Infof("已为媒体库 %s 注册 WebDAV 存储", libraryID)

	return nil
}

// TestConnection 测试 WebDAV 连接
func (s *WebDAVService) TestConnection(cfg config.WebDAVConfig) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("WebDAV 服务器地址不能为空")
	}

	// 创建临时客户端进行测试（测试时不启用缓存）
	testClient := NewWebDAVFSWithOptions(
		cfg.ServerURL,
		cfg.Username,
		cfg.Password,
		cfg.BasePath,
		cfg.Timeout,
		0,     // 禁用缓存
		false, // 禁用缓存
		s.logger,
	)

	return s.testConnection(testClient)
}

// testConnection 内部连接测试方法
func (s *WebDAVService) testConnection(webdavFS *WebDAVFS) error {
	// 尝试获取根目录信息
	_, err := webdavFS.Stat("/")
	if err != nil {
		return fmt.Errorf("无法访问 WebDAV 根目录: %w", err)
	}

	// 尝试列出根目录内容
	_, err = webdavFS.ReadDir("/")
	if err != nil {
		return fmt.Errorf("无法列出 WebDAV 根目录内容: %w", err)
	}

	return nil
}

// getConfigHash 生成配置哈希，用于客户端缓存
func (s *WebDAVService) getConfigHash(cfg config.WebDAVConfig) string {
	// 使用配置的关键字段生成哈希
	hash := fmt.Sprintf("%s:%s:%s", cfg.ServerURL, cfg.Username, cfg.BasePath)
	return hash
}

// UpdateConfig 更新 WebDAV 配置
func (s *WebDAVService) UpdateConfig(newCfg config.WebDAVConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldCfg := s.config.Storage.WebDAV

	// 如果配置未改变，直接返回
	if s.getConfigHash(oldCfg) == s.getConfigHash(newCfg) {
		return nil
	}

	// 更新配置
	s.config.Storage.WebDAV = newCfg

	// 清除旧的客户端缓存
	s.clients = make(map[string]*WebDAVFS)

	// 重新初始化
	if newCfg.Enabled {
		return s.Initialize()
	}

	s.logger.Info("WebDAV 配置已更新")
	return nil
}

// GetStatus 获取 WebDAV 服务状态
func (s *WebDAVService) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	webdavCfg := s.config.Storage.WebDAV

	status := map[string]interface{}{
		"enabled":      webdavCfg.Enabled,
		"server_url":   webdavCfg.ServerURL,
		"base_path":    webdavCfg.BasePath,
		"client_count": len(s.clients),
	}

	// 如果启用，添加连接状态
	if webdavCfg.Enabled && len(s.clients) > 0 {
		// 测试第一个客户端的状态
		configHash := s.getConfigHash(webdavCfg)
		if client, exists := s.clients[configHash]; exists {
			if err := s.testConnection(client); err == nil {
				status["connected"] = true
			} else {
				status["connected"] = false
				status["error"] = err.Error()
			}
		}
	}

	return status
}

// Close 关闭 WebDAV 服务
func (s *WebDAVService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清除全局注入
	s.vfsMgr.SetGlobalWebDAVFS(nil)

	// 清理所有客户端
	s.clients = make(map[string]*WebDAVFS)
	s.logger.Info("WebDAV 服务已关闭")

	return nil
}
