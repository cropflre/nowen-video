package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== Alist 聚合网盘文件系统（V2.3）====================
//
// Alist 是一个支持多种网盘的统一文件列表程序（https://alist.nn.ci/）。
// 通过对接 Alist HTTP API，Nowen 可以一站式访问 20+ 网盘（阿里云盘/115/夸克/
// 百度网盘/OneDrive/Google Drive/FTP/SMB 等），无需为每家单独适配 SDK。
//
// 核心 API：
//   POST /api/auth/login           登录换 Token
//   POST /api/fs/list              列目录
//   POST /api/fs/get               获取文件信息（包含直链）
//
// 播放策略：
//   通过 /api/fs/get 拿到 raw_url（直链）后，使用 HTTP Range 请求访问，
//   直链通常支持 Range，seek/转码均可正常工作。

// ==================== Alist API 响应结构 ====================

type alistResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type alistLoginData struct {
	Token string `json:"token"`
}

type alistFileInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	IsDir    bool   `json:"is_dir"`
	Modified string `json:"modified"`
	Sign     string `json:"sign"`
	Thumb    string `json:"thumb"`
	Type     int    `json:"type"`
	RawURL   string `json:"raw_url"` // 仅 /api/fs/get 返回
}

type alistListData struct {
	Content []alistFileInfo `json:"content"`
	Total   int             `json:"total"`
}

// ==================== AlistFS 实现 ====================

// AlistFS Alist 聚合网盘文件系统
type AlistFS struct {
	serverURL string
	basePath  string
	username  string
	password  string
	logger    *zap.SugaredLogger

	httpClient *http.Client

	// Token 管理
	tokenMu   sync.RWMutex
	token     string
	tokenFrom string // "config" | "login"

	// 元数据缓存
	cacheEnabled bool
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	statCache    map[string]*cacheEntry // path -> fs.FileInfo
	dirCache     map[string]*cacheEntry // path -> []fs.DirEntry
	urlCache     map[string]*cacheEntry // path -> string (raw_url)

	// ReadAt 块缓存
	blockSize  int64
	blockCount int
}

// NewAlistFS 创建 Alist 文件系统实例
func NewAlistFS(serverURL, username, password, token, basePath string,
	timeoutSec, cacheTTLHours int, cacheEnabled bool,
	logger *zap.SugaredLogger) *AlistFS {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	ttl := time.Duration(cacheTTLHours) * time.Hour
	if cacheTTLHours <= 0 {
		ttl = 0
		cacheEnabled = false
	}
	f := &AlistFS{
		serverURL: strings.TrimRight(serverURL, "/"),
		basePath:  normalizeAlistPath(basePath),
		username:  username,
		password:  password,
		logger:    logger,
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSec) * time.Second,
		},
		cacheEnabled: cacheEnabled,
		cacheTTL:     ttl,
		statCache:    make(map[string]*cacheEntry),
		dirCache:     make(map[string]*cacheEntry),
		urlCache:     make(map[string]*cacheEntry),
		blockSize:    defaultWebDAVBlockSize,
		blockCount:   defaultWebDAVBlockCount,
	}
	if token != "" {
		f.token = token
		f.tokenFrom = "config"
	}
	return f
}

// SetBlockCacheConfig 调整 ReadAt 块缓存参数
func (a *AlistFS) SetBlockCacheConfig(blockSize int64, blockCount int) {
	a.blockSize = blockSize
	a.blockCount = blockCount
}

// Type 返回类型标识
func (a *AlistFS) Type() string { return "alist" }

// ==================== 路径规范化 ====================

// normalizeAlistPath 规范化 Alist 路径（统一为 /aaa/bbb 形式）
func normalizeAlistPath(p string) string {
	if p == "" {
		return "/"
	}
	p = strings.ReplaceAll(p, "\\", "/")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)
	return p
}

// buildFullPath 拼接 basePath 与请求路径
func (a *AlistFS) buildFullPath(p string) string {
	if a.basePath == "" || a.basePath == "/" {
		return normalizeAlistPath(p)
	}
	p = normalizeAlistPath(p)
	if p == "/" {
		return a.basePath
	}
	return normalizeAlistPath(a.basePath + p)
}

// ==================== Token 管理 ====================

func (a *AlistFS) getToken(ctx context.Context) (string, error) {
	a.tokenMu.RLock()
	tk := a.token
	a.tokenMu.RUnlock()
	if tk != "" {
		return tk, nil
	}

	if a.username == "" {
		return "", fmt.Errorf("Alist 未配置 Token 且未配置用户名，无法登录")
	}

	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	// 双检
	if a.token != "" {
		return a.token, nil
	}

	body := map[string]string{
		"username": a.username,
		"password": a.password,
	}
	var data alistLoginData
	if err := a.callAPI(ctx, "/api/auth/login", body, &data, false); err != nil {
		return "", fmt.Errorf("Alist 登录失败: %w", err)
	}
	if data.Token == "" {
		return "", fmt.Errorf("Alist 登录返回空 Token")
	}
	a.token = data.Token
	a.tokenFrom = "login"
	a.logger.Infof("Alist 登录成功，已获取 Token (用户: %s)", a.username)
	return a.token, nil
}

// clearTokenOnAuthFail 清除 Token（遇到 401/403 时调用，下次重新登录）
func (a *AlistFS) clearTokenOnAuthFail() {
	a.tokenMu.Lock()
	defer a.tokenMu.Unlock()
	// 仅清除 login 获取的 Token，配置 Token 保留
	if a.tokenFrom == "login" {
		a.token = ""
	}
}

// ==================== HTTP API 调用 ====================

// callAPI 调用 Alist JSON API
func (a *AlistFS) callAPI(ctx context.Context, apiPath string, body interface{}, out interface{}, needAuth bool) error {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("序列化请求体失败: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	fullURL := a.serverURL + apiPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, reqBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nowen-video/1.0")

	if needAuth {
		token, err := a.getToken(ctx)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", token)
	}

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		a.clearTokenOnAuthFail()
		return fmt.Errorf("Alist 认证失败 (状态码 %d)", resp.StatusCode)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Alist API 状态码错误 %d: %s", resp.StatusCode, string(respBody))
	}

	var ar alistResp
	if err := json.NewDecoder(resp.Body).Decode(&ar); err != nil {
		return fmt.Errorf("解析响应失败: %w", err)
	}
	if ar.Code != 200 {
		// Alist 的 401 会以 Code=401 返回
		if ar.Code == 401 || ar.Code == 403 {
			a.clearTokenOnAuthFail()
		}
		return fmt.Errorf("Alist API 业务错误 (code=%d): %s", ar.Code, ar.Message)
	}
	if out != nil && len(ar.Data) > 0 {
		if err := json.Unmarshal(ar.Data, out); err != nil {
			return fmt.Errorf("解析 data 字段失败: %w", err)
		}
	}
	return nil
}

// ==================== 目录 / 文件信息 ====================

type alistDirEntry struct {
	info alistFileInfo
}

func (e *alistDirEntry) Name() string { return e.info.Name }
func (e *alistDirEntry) IsDir() bool  { return e.info.IsDir }
func (e *alistDirEntry) Type() fs.FileMode {
	if e.info.IsDir {
		return fs.ModeDir
	}
	return 0
}
func (e *alistDirEntry) Info() (fs.FileInfo, error) {
	return newAlistFileInfo(e.info), nil
}

// alistFileInfoImpl 实现 os.FileInfo
type alistFileInfoImpl struct {
	name    string
	size    int64
	isDir   bool
	modTime time.Time
}

func (f *alistFileInfoImpl) Name() string { return f.name }
func (f *alistFileInfoImpl) Size() int64  { return f.size }
func (f *alistFileInfoImpl) Mode() fs.FileMode {
	if f.isDir {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (f *alistFileInfoImpl) ModTime() time.Time { return f.modTime }
func (f *alistFileInfoImpl) IsDir() bool        { return f.isDir }
func (f *alistFileInfoImpl) Sys() interface{}   { return nil }

func newAlistFileInfo(info alistFileInfo) os.FileInfo {
	t, _ := time.Parse(time.RFC3339, info.Modified)
	return &alistFileInfoImpl{
		name:    info.Name,
		size:    info.Size,
		isDir:   info.IsDir,
		modTime: t,
	}
}

// ==================== VFS 接口实现 ====================

// ReadDir 列出目录
func (a *AlistFS) ReadDir(p string) ([]fs.DirEntry, error) {
	p = normalizeAlistPath(p)

	// 缓存命中？
	if a.cacheEnabled {
		a.cacheMu.RLock()
		if e, ok := a.dirCache[p]; ok && time.Now().Before(e.expiresAt) {
			if entries, ok := e.data.([]fs.DirEntry); ok {
				a.cacheMu.RUnlock()
				return entries, nil
			}
		}
		a.cacheMu.RUnlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.httpClient.Timeout)
	defer cancel()

	body := map[string]interface{}{
		"path":     a.buildFullPath(p),
		"password": "",
		"page":     1,
		"per_page": 0, // 0 = 不分页
		"refresh":  false,
	}
	var ld alistListData
	if err := a.callAPI(ctx, "/api/fs/list", body, &ld, true); err != nil {
		return nil, err
	}

	entries := make([]fs.DirEntry, 0, len(ld.Content))
	for _, f := range ld.Content {
		entries = append(entries, &alistDirEntry{info: f})
	}

	if a.cacheEnabled {
		a.cacheMu.Lock()
		a.dirCache[p] = &cacheEntry{data: entries, expiresAt: time.Now().Add(a.cacheTTL)}
		a.cacheMu.Unlock()
	}
	return entries, nil
}

// Stat 获取文件信息
func (a *AlistFS) Stat(p string) (fs.FileInfo, error) {
	p = normalizeAlistPath(p)

	if a.cacheEnabled {
		a.cacheMu.RLock()
		if e, ok := a.statCache[p]; ok && time.Now().Before(e.expiresAt) {
			if info, ok := e.data.(os.FileInfo); ok {
				a.cacheMu.RUnlock()
				return info, nil
			}
		}
		a.cacheMu.RUnlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.httpClient.Timeout)
	defer cancel()

	body := map[string]interface{}{
		"path":     a.buildFullPath(p),
		"password": "",
	}
	var af alistFileInfo
	if err := a.callAPI(ctx, "/api/fs/get", body, &af, true); err != nil {
		return nil, err
	}
	info := newAlistFileInfo(af)

	if a.cacheEnabled {
		a.cacheMu.Lock()
		a.statCache[p] = &cacheEntry{data: info, expiresAt: time.Now().Add(a.cacheTTL)}
		if af.RawURL != "" {
			a.urlCache[p] = &cacheEntry{data: af.RawURL, expiresAt: time.Now().Add(a.urlCacheTTL())}
		}
		a.cacheMu.Unlock()
	}
	return info, nil
}

// getRawURL 获取文件直链（带缓存）
func (a *AlistFS) getRawURL(p string) (string, error) {
	p = normalizeAlistPath(p)

	if a.cacheEnabled {
		a.cacheMu.RLock()
		if e, ok := a.urlCache[p]; ok && time.Now().Before(e.expiresAt) {
			if s, ok := e.data.(string); ok {
				a.cacheMu.RUnlock()
				return s, nil
			}
		}
		a.cacheMu.RUnlock()
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.httpClient.Timeout)
	defer cancel()

	body := map[string]interface{}{
		"path":     a.buildFullPath(p),
		"password": "",
	}
	var af alistFileInfo
	if err := a.callAPI(ctx, "/api/fs/get", body, &af, true); err != nil {
		return "", err
	}
	if af.RawURL == "" {
		return "", fmt.Errorf("Alist 未返回直链 (path=%s)", p)
	}

	if a.cacheEnabled {
		a.cacheMu.Lock()
		a.urlCache[p] = &cacheEntry{data: af.RawURL, expiresAt: time.Now().Add(a.urlCacheTTL())}
		a.cacheMu.Unlock()
	}
	return af.RawURL, nil
}

// urlCacheTTL 直链缓存 TTL（限最长 30 分钟，避免过期签名）
func (a *AlistFS) urlCacheTTL() time.Duration {
	if a.cacheTTL > 30*time.Minute {
		return 30 * time.Minute
	}
	return a.cacheTTL
}

// Open 打开文件（返回支持 ReadAt 的 VFSFile）
func (a *AlistFS) Open(p string) (VFSFile, error) {
	p = normalizeAlistPath(p)
	info, err := a.Stat(p)
	if err != nil {
		return nil, err
	}

	f := &alistFile{
		fs:         a,
		remotePath: p,
		stat:       info,
	}
	if !info.IsDir() && a.blockSize > 0 && a.blockCount > 0 {
		f.blockCache = newWebDAVBlockCache(a.blockSize, a.blockCount)
	}
	return f, nil
}

// Walk 递归遍历
func (a *AlistFS) Walk(root string, fn filepath.WalkFunc) error {
	root = normalizeAlistPath(root)
	return a.walkInternal(root, fn)
}

func (a *AlistFS) walkInternal(p string, fn filepath.WalkFunc) error {
	info, err := a.Stat(p)
	if err != nil {
		return fn(p, nil, err)
	}
	if err := fn(p, info, nil); err != nil {
		if err == filepath.SkipDir {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	entries, err := a.ReadDir(p)
	if err != nil {
		return fn(p, info, err)
	}
	for _, e := range entries {
		child := strings.TrimRight(p, "/") + "/" + e.Name()
		if err := a.walkInternal(child, fn); err != nil {
			if err == filepath.SkipDir {
				continue
			}
			return err
		}
	}
	return nil
}

// GetHTTPURL 返回可供 FFmpeg 等外部工具使用的 HTTP 直链（V2.3）
func (a *AlistFS) GetHTTPURL(p string) (string, error) {
	return a.getRawURL(p)
}

// ==================== alistFile 文件实现 ====================

type alistFile struct {
	fs         *AlistFS
	remotePath string
	stat       fs.FileInfo

	mu     sync.Mutex
	reader io.ReadCloser
	offset int64

	blockCache *webdavBlockCache
}

func (f *alistFile) Stat() (fs.FileInfo, error) {
	return f.stat, nil
}

// Read 顺序读取
func (f *alistFile) Read(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.reader == nil {
		rc, err := f.openRangeReader(0, -1)
		if err != nil {
			return 0, err
		}
		f.reader = rc
	}
	n, err := f.reader.Read(p)
	f.offset += int64(n)
	return n, err
}

// ReadAt 随机读取（优先走缓存）
func (f *alistFile) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off < 0 {
		return 0, fmt.Errorf("ReadAt: negative offset")
	}
	if f.blockCache != nil {
		return f.readAtCachedAlist(p, off)
	}
	return f.readAtDirect(p, off)
}

// readAtDirect 直连 Range 请求
func (f *alistFile) readAtDirect(p []byte, off int64) (int, error) {
	rc, err := f.openRangeReader(off, int64(len(p)))
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	total := 0
	for total < len(p) {
		n, rerr := rc.Read(p[total:])
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

// openRangeReader 发起 Range HTTP 请求，返回 ReadCloser
// length < 0 表示读到文件末尾
func (f *alistFile) openRangeReader(off, length int64) (io.ReadCloser, error) {
	rawURL, err := f.fs.getRawURL(f.remotePath)
	if err != nil {
		return nil, err
	}
	// 使用 HEAD 一致的 HTTP client
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("解析直链失败: %w", err)
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if length > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", off, off+length-1))
	} else if off > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", off))
	}
	req.Header.Set("User-Agent", "nowen-video/1.0")

	// 使用更长超时的 client 专用于流读取
	client := &http.Client{
		Timeout: 0, // 流式读取不设总超时
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求直链失败: %w", err)
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		// 403 可能是签名过期 → 清缓存并重试一次
		if resp.StatusCode == 403 {
			f.fs.cacheMu.Lock()
			delete(f.fs.urlCache, f.remotePath)
			f.fs.cacheMu.Unlock()
		}
		return nil, fmt.Errorf("直链请求状态码 %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// readAtCachedAlist 带块缓存的 ReadAt
//
// 逻辑与 webdavFile.readAtCached 相同，但底层下载器复用 openRangeReader
func (f *alistFile) readAtCachedAlist(p []byte, off int64) (int, error) {
	cache := f.blockCache
	fileSize := int64(-1)
	if f.stat != nil {
		fileSize = f.stat.Size()
	}
	blockSize := cache.blockSize
	total := 0
	cur := off
	end := off + int64(len(p))

	for cur < end {
		blockOff := (cur / blockSize) * blockSize

		block, err := f.ensureBlock(cache, blockOff, fileSize)
		if err != nil {
			if total > 0 {
				if err == io.EOF {
					return total, io.EOF
				}
				return total, err
			}
			return 0, err
		}
		inBlock := cur - block.offset
		if inBlock >= int64(len(block.data)) {
			if total > 0 {
				return total, io.EOF
			}
			return 0, io.EOF
		}
		n := copy(p[total:], block.data[inBlock:])
		total += n
		cur += int64(n)
		if int64(len(block.data)) < blockSize && cur >= block.offset+int64(len(block.data)) {
			if total < len(p) {
				return total, io.EOF
			}
		}
		if n == 0 {
			break
		}
	}
	if total < len(p) {
		return total, io.EOF
	}
	return total, nil
}

func (f *alistFile) ensureBlock(cache *webdavBlockCache, blockOff, fileSize int64) (*webdavCacheBlock, error) {
	if b, ok := cache.get(blockOff); ok {
		return b, nil
	}
	if wait := cache.acquireFetch(blockOff); wait != nil {
		<-wait
		if b, ok := cache.get(blockOff); ok {
			return b, nil
		}
	}
	defer cache.releaseFetch(blockOff)

	readLen := cache.blockSize
	if fileSize >= 0 {
		if blockOff >= fileSize {
			return nil, io.EOF
		}
		if blockOff+readLen > fileSize {
			readLen = fileSize - blockOff
		}
	}
	data := make([]byte, readLen)
	n, err := f.readAtDirect(data, blockOff)
	if err != nil && err != io.EOF {
		return nil, err
	}
	block := &webdavCacheBlock{offset: blockOff, data: data[:n]}
	cache.put(block)
	return block, nil
}

// Close 关闭文件
func (f *alistFile) Close() error {
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
