package service

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// ==================== VFS 路径前缀 ====================

const (
	// WebDAVScheme WebDAV 统一路径前缀
	WebDAVScheme = "webdav://"
	// AlistScheme Alist 聚合网盘统一路径前缀（V2.3）
	AlistScheme = "alist://"
	// S3Scheme S3 兼容对象存储统一路径前缀（V2.3）
	S3Scheme = "s3://"
)

// RemoteSchemes 所有远程存储前缀
var RemoteSchemes = []string{WebDAVScheme, AlistScheme, S3Scheme}

// IsRemotePath 判断路径是否为任一远程存储路径
func IsRemotePath(path string) bool {
	for _, scheme := range RemoteSchemes {
		if strings.HasPrefix(path, scheme) {
			return true
		}
	}
	return false
}

// ==================== VFS 虚拟文件系统接口 ====================
// 抽象文件系统访问，为未来支持 WebDAV / 网盘 / SMB 等远程存储预留扩展接口

// VFSFile 虚拟文件接口
type VFSFile interface {
	io.ReadCloser
	io.ReaderAt
	Stat() (fs.FileInfo, error)
}

// VFS 虚拟文件系统接口
type VFS interface {
	// Open 打开文件
	Open(path string) (VFSFile, error)
	// ReadDir 读取目录内容
	ReadDir(path string) ([]fs.DirEntry, error)
	// Stat 获取文件信息
	Stat(path string) (fs.FileInfo, error)
	// Walk 递归遍历目录
	Walk(root string, fn filepath.WalkFunc) error
	// Type 返回文件系统类型标识
	Type() string
}

// ==================== 本地文件系统实现 ====================

// LocalFS 本地文件系统
type LocalFS struct {
	logger *zap.SugaredLogger
}

// localFile 本地文件（实现 VFSFile 接口）
type localFile struct {
	*os.File
}

func (f *localFile) ReadAt(p []byte, off int64) (n int, err error) {
	return f.File.ReadAt(p, off)
}

func NewLocalFS(logger *zap.SugaredLogger) *LocalFS {
	return &LocalFS{logger: logger}
}

func (lfs *LocalFS) Open(path string) (VFSFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f}, nil
}

func (lfs *LocalFS) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (lfs *LocalFS) Stat(path string) (fs.FileInfo, error) {
	return os.Stat(path)
}

func (lfs *LocalFS) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

func (lfs *LocalFS) Type() string {
	return "local"
}

// ==================== VFS 管理器 ====================
// 注意：WebDAVFS 的完整实现位于 webdav_fs.go 文件中

// VFSManager 管理多个文件系统实例
type VFSManager struct {
	mu          sync.RWMutex
	filesystems map[string]VFS // libraryID -> VFS
	defaultFS   VFS            // 默认本地文件系统
	webdavFS    VFS            // 全局 WebDAV 文件系统（从 WebDAVService 注入）
	alistFS     VFS            // V2.3: 全局 Alist 文件系统
	s3FS        VFS            // V2.3: 全局 S3 兼容对象存储文件系统
	logger      *zap.SugaredLogger
}

func NewVFSManager(logger *zap.SugaredLogger) *VFSManager {
	return &VFSManager{
		filesystems: make(map[string]VFS),
		defaultFS:   NewLocalFS(logger),
		logger:      logger,
	}
}

// GetFS 获取指定媒体库的文件系统
func (m *VFSManager) GetFS(libraryID string) VFS {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if vfs, ok := m.filesystems[libraryID]; ok {
		return vfs
	}
	return m.defaultFS
}

// SetGlobalWebDAVFS 设置全局 WebDAV 文件系统（由 WebDAVService 调用）
func (m *VFSManager) SetGlobalWebDAVFS(vfs VFS) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webdavFS = vfs
	if vfs != nil {
		m.logger.Info("已注入全局 WebDAV VFS")
	}
}

// SetGlobalAlistFS 设置全局 Alist 文件系统（V2.3）
func (m *VFSManager) SetGlobalAlistFS(vfs VFS) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alistFS = vfs
	if vfs != nil {
		m.logger.Info("已注入全局 Alist VFS")
	}
}

// SetGlobalS3FS 设置全局 S3 文件系统（V2.3）
func (m *VFSManager) SetGlobalS3FS(vfs VFS) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.s3FS = vfs
	if vfs != nil {
		m.logger.Info("已注入全局 S3 VFS")
	}
}

// ResolvePath 解析路径前缀并返回对应的 VFS 和相对路径。
// 支持的前缀：
//   - "webdav://xxx" → 全局 WebDAV VFS
//   - "alist://xxx"  → 全局 Alist VFS（V2.3）
//   - "s3://xxx"     → 全局 S3 VFS（V2.3）
//   - 其他（任意本地路径） → 返回默认 LocalFS
func (m *VFSManager) ResolvePath(path string) (VFS, string) {
	switch {
	case strings.HasPrefix(path, WebDAVScheme):
		m.mu.RLock()
		fs := m.webdavFS
		m.mu.RUnlock()
		if fs == nil {
			return m.defaultFS, path
		}
		rel := "/" + strings.TrimLeft(strings.TrimPrefix(path, WebDAVScheme), "/")
		return fs, rel
	case strings.HasPrefix(path, AlistScheme):
		m.mu.RLock()
		fs := m.alistFS
		m.mu.RUnlock()
		if fs == nil {
			return m.defaultFS, path
		}
		rel := "/" + strings.TrimLeft(strings.TrimPrefix(path, AlistScheme), "/")
		return fs, rel
	case strings.HasPrefix(path, S3Scheme):
		m.mu.RLock()
		fs := m.s3FS
		m.mu.RUnlock()
		if fs == nil {
			return m.defaultFS, path
		}
		rel := "/" + strings.TrimLeft(strings.TrimPrefix(path, S3Scheme), "/")
		return fs, rel
	}
	return m.defaultFS, path
}

// IsWebDAVPath 判断路径是否为 WebDAV 路径
func IsWebDAVPath(path string) bool {
	return strings.HasPrefix(path, WebDAVScheme)
}

// Open 通过路径前缀自动路由到对应 VFS 打开文件
func (m *VFSManager) Open(path string) (VFSFile, error) {
	vfs, realPath := m.ResolvePath(path)
	return vfs.Open(realPath)
}

// Stat 通过路径前缀自动路由 Stat
func (m *VFSManager) Stat(path string) (fs.FileInfo, error) {
	vfs, realPath := m.ResolvePath(path)
	return vfs.Stat(realPath)
}

// ReadDir 通过路径前缀自动路由 ReadDir
func (m *VFSManager) ReadDir(path string) ([]fs.DirEntry, error) {
	vfs, realPath := m.ResolvePath(path)
	return vfs.ReadDir(realPath)
}

// Walk 通过路径前缀自动路由 Walk，并自动带回带前缀的路径（以便下游持久化到 DB）
func (m *VFSManager) Walk(root string, fn filepath.WalkFunc) error {
	vfs, realRoot := m.ResolvePath(root)

	// 对于远程路径：VFS Walk 返回的是相对路径，需要拼接回完整的前缀形式
	switch vfs.Type() {
	case "webdav", "alist", "s3":
		base := strings.TrimSuffix(root, "/")
		return vfs.Walk(realRoot, func(p string, info os.FileInfo, err error) error {
			fullPath := base + "/" + strings.TrimLeft(p, "/")
			if p == "" || p == "." {
				fullPath = base
			}
			return fn(fullPath, info, err)
		})
	}
	return vfs.Walk(realRoot, fn)
}

// RegisterFS 为指定媒体库注册文件系统
func (m *VFSManager) RegisterFS(libraryID string, vfs VFS) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filesystems[libraryID] = vfs
	m.logger.Infof("已注册 VFS: 媒体库=%s, 类型=%s", libraryID, vfs.Type())
}

// UnregisterFS 取消注册
func (m *VFSManager) UnregisterFS(libraryID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.filesystems, libraryID)
}

// GetDefault 获取默认文件系统（本地）
func (m *VFSManager) GetDefault() VFS {
	return m.defaultFS
}

// HasCustomFS 检查指定媒体库是否有自定义文件系统
func (m *VFSManager) HasCustomFS(libraryID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.filesystems[libraryID]
	return exists
}

// GetFSType 获取指定媒体库的文件系统类型
func (m *VFSManager) GetFSType(libraryID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if vfs, exists := m.filesystems[libraryID]; exists {
		return vfs.Type()
	}
	return m.defaultFS.Type()
}

// GetAllFS 获取所有已注册的文件系统信息
func (m *VFSManager) GetAllFS() map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]string)
	result["default"] = m.defaultFS.Type()
	for libraryID, vfs := range m.filesystems {
		result[libraryID] = vfs.Type()
	}
	return result
}

// ClearAllFS 清除所有自定义文件系统
func (m *VFSManager) ClearAllFS() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.filesystems = make(map[string]VFS)
	m.logger.Info("已清除所有自定义文件系统")
}

// TestFSConnection 测试指定媒体库的文件系统连接
func (m *VFSManager) TestFSConnection(libraryID string) error {
	vfs := m.GetFS(libraryID)

	// 尝试访问根目录来测试连接
	_, err := vfs.Stat("/")
	if err != nil {
		return fmt.Errorf("文件系统连接测试失败: %w", err)
	}

	return nil
}

// ==================== VFS ReadSeeker 适配器 ====================

// vfsReadSeeker 将 VFSFile (ReaderAt) 适配为 io.ReadSeeker
//
// 主要用途：http.ServeContent 需要 io.ReadSeeker 来处理 HTTP Range 请求。
// 实现上利用 VFSFile.ReadAt 实现真正的随机读取，而非顺序读取+Discard 模拟。
type vfsReadSeeker struct {
	file VFSFile
	size int64
	pos  int64
}

// NewVFSReadSeeker 创建基于 VFSFile 的 io.ReadSeeker
func NewVFSReadSeeker(file VFSFile, size int64) io.ReadSeeker {
	return &vfsReadSeeker{file: file, size: size}
}

// Read 实现 io.Reader
func (r *vfsReadSeeker) Read(p []byte) (int, error) {
	if r.pos >= r.size {
		return 0, io.EOF
	}
	n, err := r.file.ReadAt(p, r.pos)
	r.pos += int64(n)
	// ReadAt 的 EOF 在读满前返回时是符合预期的
	if err == io.EOF && n > 0 {
		// http.ServeContent 对 EOF 的处理依赖 n > 0 时不返回 EOF
		if r.pos < r.size {
			err = nil
		}
	}
	return n, err
}

// Seek 实现 io.Seeker
func (r *vfsReadSeeker) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = r.pos + offset
	case io.SeekEnd:
		newPos = r.size + offset
	default:
		return 0, fmt.Errorf("invalid whence: %d", whence)
	}
	if newPos < 0 {
		return 0, fmt.Errorf("negative position")
	}
	r.pos = newPos
	return r.pos, nil
}
