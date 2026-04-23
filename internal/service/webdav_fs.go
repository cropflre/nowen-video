package service

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/studio-b12/gowebdav"
	"go.uber.org/zap"
)

// ==================== WebDAV 文件系统实现 ====================

// cacheEntry 缓存条目
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// WebDAVFS WebDAV 远程文件系统
//
// 特性：
//  1. Stat/ReadDir 本地 TTL 缓存（加速扫描 & 重复请求）
//  2. ReadAt 使用真实 HTTP Range 请求（支持随机读取 / 播放器 seek）
//  3. ReadAt LRU 块缓存（减少结对请求并加速相邻播放 seek）
//  4. 失败自动重试（由 gowebdav 客户端层处理）
type WebDAVFS struct {
	client   *gowebdav.Client
	basePath string
	logger   *zap.SugaredLogger

	// 元数据缓存
	cacheEnabled bool
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	statCache    map[string]*cacheEntry // path -> fs.FileInfo
	dirCache     map[string]*cacheEntry // path -> []fs.DirEntry

	// V2.1: ReadAt 块缓存配置（0 表示禁用）
	blockSize  int64
	blockCount int
}

// webdavFile WebDAV 文件包装器
//
// 通过 HTTP Range 请求实现 ReadAt，避免一次性下载整个文件。
// 内部保持一个顺序读取流用于 Read()；ReadAt() 独立使用 Range 请求。
// V2.1：可选的 LRU 块缓存加速相邻区间访问。
type webdavFile struct {
	fs         *WebDAVFS
	remotePath string
	stat       fs.FileInfo

	// 顺序读取流（懒加载）
	mu     sync.Mutex
	reader io.ReadCloser
	offset int64

	// V2.1: 随机读 LRU 块缓存（按文件独立，Close 时释放）
	blockCache *webdavBlockCache
}

// ensureReader 懒加载顺序读取流
func (f *webdavFile) ensureReader() error {
	if f.reader != nil {
		return nil
	}
	r, err := f.fs.client.ReadStream(f.remotePath)
	if err != nil {
		return fmt.Errorf("WebDAV 打开流失败: %w", err)
	}
	f.reader = r
	f.offset = 0
	return nil
}

// Read 顺序读取
func (f *webdavFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.ensureReader(); err != nil {
		return 0, err
	}
	n, err := f.reader.Read(p)
	f.offset += int64(n)
	return n, err
}

// ReadAt 通过 HTTP Range 请求随机读取
//
// V2.1: 如果启用了块缓存（blockCache != nil），优先走缓存路径；否则退到直连 Range。
// 大块并发读取（如 HLS 分片、播放器 seek）由 HTTP/1.1 Keep-Alive 或 HTTP/2 多路复用支撑。
func (f *webdavFile) ReadAt(p []byte, off int64) (int, error) {
	if f.blockCache != nil {
		return f.readAtCached(p, off)
	}
	return f.readAtDirect(p, off)
}

// readAtDirect 直连 HTTP Range 的 ReadAt。供缓存层与禁用缓存时调用。
func (f *webdavFile) readAtDirect(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off < 0 {
		return 0, fmt.Errorf("ReadAt: negative offset")
	}
	// 使用 ReadStreamRange 发起 Range 请求
	reader, err := f.fs.client.ReadStreamRange(f.remotePath, off, int64(len(p)))
	if err != nil {
		return 0, fmt.Errorf("WebDAV Range 读取失败 (off=%d, len=%d): %w", off, len(p), err)
	}
	defer reader.Close()

	// 读满 p 或到 EOF
	total := 0
	for total < len(p) {
		n, rerr := reader.Read(p[total:])
		total += n
		if rerr == io.EOF {
			if total == 0 {
				return 0, io.EOF
			}
			return total, io.EOF
		}
		if rerr != nil {
			return total, rerr
		}
	}
	return total, nil
}

// Close 关闭文件
func (f *webdavFile) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.blockCache != nil {
		f.blockCache.clear()
		f.blockCache = nil
	}
	if f.reader != nil {
		err := f.reader.Close()
		f.reader = nil
		return err
	}
	return nil
}

// Stat 获取文件信息
func (f *webdavFile) Stat() (fs.FileInfo, error) {
	return f.stat, nil
}

// NewWebDAVFS 创建 WebDAV 文件系统实例
func NewWebDAVFS(serverURL, username, password, basePath string, logger *zap.SugaredLogger) *WebDAVFS {
	return NewWebDAVFSWithOptions(serverURL, username, password, basePath, 30, 24, true, logger)
}

// NewWebDAVFSWithOptions 创建 WebDAV 文件系统实例（带自定义参数）
//
//	timeoutSec: HTTP 请求超时（秒）
//	cacheTTLHours: 元数据缓存 TTL（小时），<=0 则禁用
//	cacheEnabled: 是否启用元数据缓存
func NewWebDAVFSWithOptions(
	serverURL, username, password, basePath string,
	timeoutSec int, cacheTTLHours int, cacheEnabled bool,
	logger *zap.SugaredLogger,
) *WebDAVFS {
	client := gowebdav.NewClient(serverURL, username, password)

	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	client.SetTimeout(time.Duration(timeoutSec) * time.Second)

	ttl := time.Duration(cacheTTLHours) * time.Hour
	if cacheTTLHours <= 0 {
		ttl = 0
		cacheEnabled = false
	}

	return &WebDAVFS{
		client:       client,
		basePath:     strings.Trim(basePath, "/"),
		logger:       logger,
		cacheEnabled: cacheEnabled,
		cacheTTL:     ttl,
		statCache:    make(map[string]*cacheEntry),
		dirCache:     make(map[string]*cacheEntry),
		// V2.1: 默认启用块缓存（可通过 SetBlockCacheConfig 调整）
		blockSize:  defaultWebDAVBlockSize,
		blockCount: defaultWebDAVBlockCount,
	}
}

// SetBlockCacheConfig 调整 ReadAt 块缓存参数（V2.1）
//
//	blockSize: 单块字节数，<=0 禁用块缓存
//	blockCount: 每文件最大块数，<=0 禁用块缓存
func (wfs *WebDAVFS) SetBlockCacheConfig(blockSize int64, blockCount int) {
	wfs.blockSize = blockSize
	wfs.blockCount = blockCount
}

// buildPath 构建完整路径
func (wfs *WebDAVFS) buildPath(p string) string {
	p = strings.TrimLeft(p, "/")
	if wfs.basePath == "" {
		return "/" + p
	}
	return "/" + path.Join(wfs.basePath, p)
}

// ==================== 缓存辅助 ====================

func (wfs *WebDAVFS) cacheGetStat(key string) (fs.FileInfo, bool) {
	if !wfs.cacheEnabled {
		return nil, false
	}
	wfs.cacheMu.RLock()
	defer wfs.cacheMu.RUnlock()
	if entry, ok := wfs.statCache[key]; ok && time.Now().Before(entry.expiresAt) {
		if info, ok := entry.data.(fs.FileInfo); ok {
			return info, true
		}
	}
	return nil, false
}

func (wfs *WebDAVFS) cacheSetStat(key string, info fs.FileInfo) {
	if !wfs.cacheEnabled {
		return
	}
	wfs.cacheMu.Lock()
	defer wfs.cacheMu.Unlock()
	wfs.statCache[key] = &cacheEntry{data: info, expiresAt: time.Now().Add(wfs.cacheTTL)}
}

func (wfs *WebDAVFS) cacheGetDir(key string) ([]fs.DirEntry, bool) {
	if !wfs.cacheEnabled {
		return nil, false
	}
	wfs.cacheMu.RLock()
	defer wfs.cacheMu.RUnlock()
	if entry, ok := wfs.dirCache[key]; ok && time.Now().Before(entry.expiresAt) {
		if entries, ok := entry.data.([]fs.DirEntry); ok {
			return entries, true
		}
	}
	return nil, false
}

func (wfs *WebDAVFS) cacheSetDir(key string, entries []fs.DirEntry) {
	if !wfs.cacheEnabled {
		return
	}
	wfs.cacheMu.Lock()
	defer wfs.cacheMu.Unlock()
	wfs.dirCache[key] = &cacheEntry{data: entries, expiresAt: time.Now().Add(wfs.cacheTTL)}
}

// InvalidateCache 清除所有缓存
func (wfs *WebDAVFS) InvalidateCache() {
	wfs.cacheMu.Lock()
	defer wfs.cacheMu.Unlock()
	wfs.statCache = make(map[string]*cacheEntry)
	wfs.dirCache = make(map[string]*cacheEntry)
}

// ==================== VFS 接口实现 ====================

// Open 打开 WebDAV 文件
func (wfs *WebDAVFS) Open(p string) (VFSFile, error) {
	fullPath := wfs.buildPath(p)

	// 先获取文件信息（走缓存）
	stat, err := wfs.Stat(p)
	if err != nil {
		return nil, err
	}

	f := &webdavFile{
		fs:         wfs,
		remotePath: fullPath,
		stat:       stat,
	}
	// V2.1: 为非目录文件挂载块缓存
	if !stat.IsDir() && wfs.blockSize > 0 && wfs.blockCount > 0 {
		f.blockCache = newWebDAVBlockCache(wfs.blockSize, wfs.blockCount)
	}
	return f, nil
}

// ReadDir 读取 WebDAV 目录内容
func (wfs *WebDAVFS) ReadDir(p string) ([]fs.DirEntry, error) {
	fullPath := wfs.buildPath(p)

	// 查缓存
	if entries, ok := wfs.cacheGetDir(fullPath); ok {
		return entries, nil
	}

	files, err := wfs.client.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("WebDAV 目录列表失败 %s: %w", fullPath, err)
	}

	entries := make([]fs.DirEntry, len(files))
	for i, file := range files {
		entries[i] = &webdavDirEntry{file: file}
	}

	wfs.cacheSetDir(fullPath, entries)
	return entries, nil
}

// Stat 获取 WebDAV 文件信息
func (wfs *WebDAVFS) Stat(p string) (fs.FileInfo, error) {
	fullPath := wfs.buildPath(p)

	// 查缓存
	if info, ok := wfs.cacheGetStat(fullPath); ok {
		return info, nil
	}

	stat, err := wfs.client.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("WebDAV Stat 失败 %s: %w", fullPath, err)
	}

	wfs.cacheSetStat(fullPath, stat)
	return stat, nil
}

// Walk 递归遍历 WebDAV 目录
func (wfs *WebDAVFS) Walk(root string, fn filepath.WalkFunc) error {
	return wfs.walkRecursive(root, fn)
}

// walkRecursive 递归遍历实现
// 注意：回调 fn 拿到的 path 是相对 root 的逻辑路径（以 / 开头，不含 basePath / webdav:// 前缀）
func (wfs *WebDAVFS) walkRecursive(relPath string, fn filepath.WalkFunc) error {
	stat, err := wfs.Stat(relPath)
	if err != nil {
		return fn(relPath, nil, err)
	}

	err = fn(relPath, stat, nil)
	if err != nil {
		if err == filepath.SkipDir {
			return nil
		}
		return err
	}

	if stat.IsDir() {
		entries, err := wfs.ReadDir(relPath)
		if err != nil {
			return fn(relPath, stat, err)
		}

		for _, entry := range entries {
			childRel := path.Join(relPath, entry.Name())
			if !strings.HasPrefix(childRel, "/") {
				childRel = "/" + childRel
			}
			if walkErr := wfs.walkRecursive(childRel, fn); walkErr != nil && walkErr != filepath.SkipDir {
				return walkErr
			}
		}
	}

	return nil
}

// Type 返回文件系统类型标识
func (wfs *WebDAVFS) Type() string {
	return "webdav"
}

// ==================== WebDAV DirEntry 实现 ====================

type webdavDirEntry struct {
	file fs.FileInfo
}

func (e *webdavDirEntry) Name() string {
	return e.file.Name()
}

func (e *webdavDirEntry) IsDir() bool {
	return e.file.IsDir()
}

func (e *webdavDirEntry) Type() fs.FileMode {
	return e.file.Mode().Type()
}

func (e *webdavDirEntry) Info() (fs.FileInfo, error) {
	return e.file, nil
}
