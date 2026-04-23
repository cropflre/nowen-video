package service

import (
	"fmt"
	"sync"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// ==================== 远程存储统一服务（V2.3）====================
//
// RemoteStorageService 统一管理 Alist 与 S3 的初始化、连接测试、状态查询，
// 与已有的 WebDAVService 形成并列关系：
//
//   WebDAVService    -> webdav://
//   RemoteStorageService.alist -> alist://
//   RemoteStorageService.s3    -> s3://
//
// 所有实例通过 VFSManager.SetGlobalXxxFS 注入到全局 VFS 路由表。

type RemoteStorageService struct {
	config *config.Config
	logger *zap.SugaredLogger
	vfsMgr *VFSManager

	mu      sync.RWMutex
	alistFS *AlistFS
	s3FS    *S3FS
}

// NewRemoteStorageService 创建统一远程存储服务
func NewRemoteStorageService(cfg *config.Config, logger *zap.SugaredLogger, vfsMgr *VFSManager) *RemoteStorageService {
	return &RemoteStorageService{
		config: cfg,
		logger: logger,
		vfsMgr: vfsMgr,
	}
}

// Initialize 初始化所有已启用的远程存储
func (s *RemoteStorageService) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清理旧注入
	s.vfsMgr.SetGlobalAlistFS(nil)
	s.vfsMgr.SetGlobalS3FS(nil)

	// ---- Alist ----
	if s.config.Storage.Alist.Enabled {
		if err := s.initAlistLocked(); err != nil {
			s.logger.Warnf("Alist 初始化失败: %v", err)
		}
	} else {
		s.logger.Info("Alist 存储未启用")
	}

	// ---- S3 ----
	if s.config.Storage.S3.Enabled {
		if err := s.initS3Locked(); err != nil {
			s.logger.Warnf("S3 初始化失败: %v", err)
		}
	} else {
		s.logger.Info("S3 存储未启用")
	}

	return nil
}

// initAlistLocked 必须持有 s.mu
func (s *RemoteStorageService) initAlistLocked() error {
	cfg := s.config.Storage.Alist
	if cfg.ServerURL == "" {
		return fmt.Errorf("Alist 服务器地址不能为空")
	}

	fs := NewAlistFS(
		cfg.ServerURL, cfg.Username, cfg.Password, cfg.Token,
		cfg.BasePath, cfg.Timeout, cfg.CacheTTLHours, cfg.EnableCache,
		s.logger,
	)
	blockSize := int64(cfg.ReadBlockSizeMB) * 1024 * 1024
	fs.SetBlockCacheConfig(blockSize, cfg.ReadBlockCount)

	// 连接测试：列根目录
	if _, err := fs.ReadDir("/"); err != nil {
		return fmt.Errorf("Alist 连接测试失败: %w", err)
	}

	s.alistFS = fs
	s.vfsMgr.SetGlobalAlistFS(fs)
	s.logger.Infof("Alist 服务初始化成功: %s (BasePath=%s)", cfg.ServerURL, cfg.BasePath)
	return nil
}

// initS3Locked 必须持有 s.mu
func (s *RemoteStorageService) initS3Locked() error {
	cfg := s.config.Storage.S3
	if cfg.Endpoint == "" {
		return fmt.Errorf("S3 Endpoint 不能为空")
	}
	if cfg.Bucket == "" {
		return fmt.Errorf("S3 Bucket 不能为空")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return fmt.Errorf("S3 AccessKey / SecretKey 不能为空")
	}

	fs := NewS3FS(
		cfg.Endpoint, cfg.Region, cfg.AccessKey, cfg.SecretKey,
		cfg.Bucket, cfg.BasePath, cfg.PathStyle,
		cfg.Timeout, cfg.CacheTTLHours, cfg.EnableCache,
		s.logger,
	)
	blockSize := int64(cfg.ReadBlockSizeMB) * 1024 * 1024
	fs.SetBlockCacheConfig(blockSize, cfg.ReadBlockCount)

	// 连接测试：列根目录（前缀为 basePath）
	if _, err := fs.ReadDir("/"); err != nil {
		return fmt.Errorf("S3 连接测试失败: %w", err)
	}

	s.s3FS = fs
	s.vfsMgr.SetGlobalS3FS(fs)
	s.logger.Infof("S3 服务初始化成功: %s/%s", cfg.Endpoint, cfg.Bucket)
	return nil
}

// TestAlist 测试 Alist 连接（不修改全局状态）
func (s *RemoteStorageService) TestAlist(cfg config.AlistConfig) error {
	if cfg.ServerURL == "" {
		return fmt.Errorf("Alist 服务器地址不能为空")
	}
	fs := NewAlistFS(
		cfg.ServerURL, cfg.Username, cfg.Password, cfg.Token,
		cfg.BasePath, cfg.Timeout, 0, false, // 测试时不启用缓存
		s.logger,
	)
	if _, err := fs.ReadDir("/"); err != nil {
		return fmt.Errorf("Alist 连接测试失败: %w", err)
	}
	return nil
}

// TestS3 测试 S3 连接
func (s *RemoteStorageService) TestS3(cfg config.S3Config) error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("S3 Endpoint 不能为空")
	}
	fs := NewS3FS(
		cfg.Endpoint, cfg.Region, cfg.AccessKey, cfg.SecretKey,
		cfg.Bucket, cfg.BasePath, cfg.PathStyle,
		cfg.Timeout, 0, false,
		s.logger,
	)
	if _, err := fs.ReadDir("/"); err != nil {
		return fmt.Errorf("S3 连接测试失败: %w", err)
	}
	return nil
}

// UpdateAlistConfig 更新 Alist 配置并重初始化
func (s *RemoteStorageService) UpdateAlistConfig(cfg config.AlistConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Storage.Alist = cfg
	s.alistFS = nil
	s.vfsMgr.SetGlobalAlistFS(nil)
	if cfg.Enabled {
		return s.initAlistLocked()
	}
	return nil
}

// UpdateS3Config 更新 S3 配置并重初始化
func (s *RemoteStorageService) UpdateS3Config(cfg config.S3Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.Storage.S3 = cfg
	s.s3FS = nil
	s.vfsMgr.SetGlobalS3FS(nil)
	if cfg.Enabled {
		return s.initS3Locked()
	}
	return nil
}

// GetStatus 获取 Alist + S3 状态
func (s *RemoteStorageService) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	alistCfg := s.config.Storage.Alist
	s3Cfg := s.config.Storage.S3

	alistStatus := map[string]interface{}{
		"enabled":    alistCfg.Enabled,
		"server_url": alistCfg.ServerURL,
		"base_path":  alistCfg.BasePath,
		"connected":  s.alistFS != nil,
	}
	s3Status := map[string]interface{}{
		"enabled":    s3Cfg.Enabled,
		"endpoint":   s3Cfg.Endpoint,
		"bucket":     s3Cfg.Bucket,
		"region":     s3Cfg.Region,
		"path_style": s3Cfg.PathStyle,
		"connected":  s.s3FS != nil,
	}
	return map[string]interface{}{
		"alist": alistStatus,
		"s3":    s3Status,
	}
}

// GetAlistFS 获取已初始化的 AlistFS（可能为 nil）
func (s *RemoteStorageService) GetAlistFS() *AlistFS {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.alistFS
}

// GetS3FS 获取已初始化的 S3FS（可能为 nil）
func (s *RemoteStorageService) GetS3FS() *S3FS {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.s3FS
}

// Close 关闭并清理所有远程存储
func (s *RemoteStorageService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.vfsMgr.SetGlobalAlistFS(nil)
	s.vfsMgr.SetGlobalS3FS(nil)
	s.alistFS = nil
	s.s3FS = nil
	s.logger.Info("远程存储服务已关闭")
	return nil
}
