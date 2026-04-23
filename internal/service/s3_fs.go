package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== S3 兼容对象存储文件系统（V2.3）====================
//
// 直接对接 S3 REST API（AWS Signature V4），兼容 AWS S3 / MinIO / Cloudflare R2 /
// 阿里云 OSS / 腾讯云 COS 等。
//
// 之所以不引入 AWS SDK V2：
//   - 避免增加数 MB 的依赖体积
//   - S3 API 本身是简单的 HTTP REST，签名算法标准化
//   - 本服务仅需 GET/HEAD/LIST（只读），实现成本低
//
// 核心操作：
//   - Stat:    HEAD /{key}
//   - Open + ReadAt: GET /{key} + Range
//   - ReadDir: GET /?list-type=2&prefix=...&delimiter=/
//   - Walk:    ReadDir 递归

// ==================== S3 ListObjectsV2 响应 ====================

type s3ListBucketResult struct {
	XMLName        xml.Name       `xml:"ListBucketResult"`
	Name           string         `xml:"Name"`
	Prefix         string         `xml:"Prefix"`
	Delimiter      string         `xml:"Delimiter"`
	IsTruncated    bool           `xml:"IsTruncated"`
	NextToken      string         `xml:"NextContinuationToken"`
	KeyCount       int            `xml:"KeyCount"`
	Contents       []s3Object     `xml:"Contents"`
	CommonPrefixes []s3CommonPref `xml:"CommonPrefixes"`
}

type s3Object struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type s3CommonPref struct {
	Prefix string `xml:"Prefix"`
}

// ==================== S3FS ====================

// S3FS S3 兼容对象存储文件系统
type S3FS struct {
	endpoint  string // https://s3.amazonaws.com 或 https://minio.example.com:9000
	region    string
	accessKey string
	secretKey string
	bucket    string
	basePath  string // 可选前缀，形如 "media/"
	pathStyle bool   // MinIO 必开

	httpClient *http.Client
	logger     *zap.SugaredLogger

	// 元数据缓存
	cacheEnabled bool
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	statCache    map[string]*cacheEntry
	dirCache     map[string]*cacheEntry

	// ReadAt 块缓存
	blockSize  int64
	blockCount int
}

// NewS3FS 创建 S3 文件系统实例
func NewS3FS(endpoint, region, accessKey, secretKey, bucket, basePath string,
	pathStyle bool, timeoutSec, cacheTTLHours int, cacheEnabled bool,
	logger *zap.SugaredLogger) *S3FS {
	if timeoutSec <= 0 {
		timeoutSec = 30
	}
	if region == "" {
		region = "us-east-1"
	}
	ttl := time.Duration(cacheTTLHours) * time.Hour
	if cacheTTLHours <= 0 {
		ttl = 0
		cacheEnabled = false
	}
	// 规范化 basePath：确保不以 / 开头，以 / 结尾（S3 前缀约定）
	bp := strings.Trim(basePath, "/")
	if bp != "" {
		bp += "/"
	}
	return &S3FS{
		endpoint:   strings.TrimRight(endpoint, "/"),
		region:     region,
		accessKey:  accessKey,
		secretKey:  secretKey,
		bucket:     bucket,
		basePath:   bp,
		pathStyle:  pathStyle,
		httpClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		logger:     logger,

		cacheEnabled: cacheEnabled,
		cacheTTL:     ttl,
		statCache:    make(map[string]*cacheEntry),
		dirCache:     make(map[string]*cacheEntry),

		blockSize:  defaultWebDAVBlockSize,
		blockCount: defaultWebDAVBlockCount,
	}
}

// SetBlockCacheConfig 调整 ReadAt 块缓存参数
func (s *S3FS) SetBlockCacheConfig(blockSize int64, blockCount int) {
	s.blockSize = blockSize
	s.blockCount = blockCount
}

// Type 返回类型标识
func (s *S3FS) Type() string { return "s3" }

// ==================== 路径 / Key 转换 ====================

// keyForPath 将 VFS 路径转为 S3 对象 Key
//
//	"/" -> ""（根）
//	"/movies/a.mkv" -> "{basePath}movies/a.mkv"
func (s *S3FS) keyForPath(p string) string {
	p = strings.TrimLeft(strings.ReplaceAll(p, "\\", "/"), "/")
	return s.basePath + p
}

// prefixForDir 目录前缀（保证以 / 结尾，除了根）
func (s *S3FS) prefixForDir(p string) string {
	p = strings.TrimLeft(strings.ReplaceAll(p, "\\", "/"), "/")
	if p == "" {
		return s.basePath
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return s.basePath + p
}

// buildURL 构造完整 URL（Virtual-Host 或 Path 风格）
func (s *S3FS) buildURL(objectKey string, query url.Values) (*url.URL, error) {
	base, err := url.Parse(s.endpoint)
	if err != nil {
		return nil, err
	}
	if s.pathStyle {
		base.Path = "/" + s.bucket
		if objectKey != "" {
			base.Path += "/" + objectKey
		}
	} else {
		base.Host = s.bucket + "." + base.Host
		if objectKey != "" {
			base.Path = "/" + objectKey
		}
	}
	if query != nil {
		base.RawQuery = query.Encode()
	}
	return base, nil
}

// ==================== AWS Signature V4 ====================

const s3EmptyPayloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
const s3UnsignedPayload = "UNSIGNED-PAYLOAD"

// signRequest 为请求附加 AWS V4 签名
//
// 参考：
//
//	https://docs.aws.amazon.com/general/latest/gr/sigv4_signing.html
//
// payloadHash 为请求体的 SHA256（GET/HEAD 为 emptyHash；Range 请求也需要 emptyHash）
func (s *S3FS) signRequest(req *http.Request, payloadHash string) {
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("host", req.URL.Host)

	// 1. Canonical Request
	canonicalURI := req.URL.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQueryString(req.URL.Query())

	// 签名头：按字母序，包括 host, x-amz-content-sha256, x-amz-date（如有 range 也签）
	signedHeadersList := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	if rng := req.Header.Get("Range"); rng != "" {
		signedHeadersList = append(signedHeadersList, "range")
	}
	sort.Strings(signedHeadersList)

	var canonicalHeaders strings.Builder
	for _, h := range signedHeadersList {
		var v string
		switch h {
		case "host":
			v = req.URL.Host
		case "range":
			v = req.Header.Get("Range")
		default:
			v = req.Header.Get(h)
		}
		canonicalHeaders.WriteString(h)
		canonicalHeaders.WriteString(":")
		canonicalHeaders.WriteString(strings.TrimSpace(v))
		canonicalHeaders.WriteString("\n")
	}
	signedHeaders := strings.Join(signedHeadersList, ";")

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders.String(),
		signedHeaders,
		payloadHash,
	}, "\n")

	// 2. String to Sign
	credentialScope := strings.Join([]string{dateStamp, s.region, "s3", "aws4_request"}, "/")
	hash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hex.EncodeToString(hash[:]),
	}, "\n")

	// 3. Signing Key
	kDate := hmacSHA256([]byte("AWS4"+s.secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, s.region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")

	// 4. Signature
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	// 5. Authorization Header
	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKey, credentialScope, signedHeaders, signature)
	req.Header.Set("Authorization", authHeader)
}

func hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// canonicalQueryString 按 AWS 规范生成
func canonicalQueryString(q url.Values) string {
	if len(q) == 0 {
		return ""
	}
	keys := make([]string, 0, len(q))
	for k := range q {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		vals := q[k]
		sort.Strings(vals)
		for j, v := range vals {
			if i > 0 || j > 0 {
				b.WriteString("&")
			}
			b.WriteString(awsURIEncode(k, true))
			b.WriteString("=")
			b.WriteString(awsURIEncode(v, true))
		}
	}
	return b.String()
}

// awsURIEncode 按 AWS 规范编码
// 与 url.QueryEscape 差异：斜杠的处理 + 大小写
func awsURIEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		case c == '/' && !encodeSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// ==================== HTTP 请求 ====================

// do 执行带签名的 HTTP 请求
func (s *S3FS) do(method string, u *url.URL, rangeHeader string) (*http.Response, error) {
	req, err := http.NewRequest(method, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	s.signRequest(req, s3EmptyPayloadHash)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// ==================== VFS 接口实现 ====================

// Stat 获取对象/目录信息
func (s *S3FS) Stat(p string) (fs.FileInfo, error) {
	cp := normalizeAlistPath(p)

	if s.cacheEnabled {
		s.cacheMu.RLock()
		if e, ok := s.statCache[cp]; ok && time.Now().Before(e.expiresAt) {
			if info, ok := e.data.(os.FileInfo); ok {
				s.cacheMu.RUnlock()
				return info, nil
			}
		}
		s.cacheMu.RUnlock()
	}

	key := s.keyForPath(cp)

	// 根 / 空 key → 视为目录
	if key == "" || strings.HasSuffix(key, "/") || cp == "/" {
		info := &alistFileInfoImpl{name: path.Base(cp), isDir: true, modTime: time.Now()}
		return info, nil
	}

	// 先 HEAD 判定对象
	u, err := s.buildURL(key, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(http.MethodHead, u, "")
	if err != nil {
		return nil, fmt.Errorf("S3 HEAD 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		size := resp.ContentLength
		modTime, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
		info := &alistFileInfoImpl{
			name:    path.Base(cp),
			size:    size,
			isDir:   false,
			modTime: modTime,
		}
		if s.cacheEnabled {
			s.cacheMu.Lock()
			s.statCache[cp] = &cacheEntry{data: os.FileInfo(info), expiresAt: time.Now().Add(s.cacheTTL)}
			s.cacheMu.Unlock()
		}
		return info, nil
	}

	// 404 → 尝试作为目录（ListObjectsV2 有内容 = 存在）
	if resp.StatusCode == http.StatusNotFound {
		prefix := s.prefixForDir(cp)
		result, err := s.listObjects(prefix, "/", "", 1)
		if err != nil {
			return nil, err
		}
		if result.KeyCount > 0 || len(result.CommonPrefixes) > 0 {
			info := &alistFileInfoImpl{name: path.Base(cp), isDir: true, modTime: time.Now()}
			if s.cacheEnabled {
				s.cacheMu.Lock()
				s.statCache[cp] = &cacheEntry{data: os.FileInfo(info), expiresAt: time.Now().Add(s.cacheTTL)}
				s.cacheMu.Unlock()
			}
			return info, nil
		}
		return nil, os.ErrNotExist
	}

	return nil, fmt.Errorf("S3 HEAD 状态码 %d", resp.StatusCode)
}

// ReadDir 列出目录下一层对象
func (s *S3FS) ReadDir(p string) ([]fs.DirEntry, error) {
	cp := normalizeAlistPath(p)

	if s.cacheEnabled {
		s.cacheMu.RLock()
		if e, ok := s.dirCache[cp]; ok && time.Now().Before(e.expiresAt) {
			if entries, ok := e.data.([]fs.DirEntry); ok {
				s.cacheMu.RUnlock()
				return entries, nil
			}
		}
		s.cacheMu.RUnlock()
	}

	prefix := s.prefixForDir(cp)
	basePfxLen := len(prefix)

	var entries []fs.DirEntry
	token := ""
	for {
		result, err := s.listObjects(prefix, "/", token, 1000)
		if err != nil {
			return nil, err
		}

		// 子目录（CommonPrefixes）
		for _, cp := range result.CommonPrefixes {
			rel := strings.TrimSuffix(strings.TrimPrefix(cp.Prefix, prefix), "/")
			if rel == "" {
				continue
			}
			// 只取最后一段
			if idx := strings.Index(rel, "/"); idx >= 0 {
				rel = rel[:idx]
			}
			entries = append(entries, &alistDirEntry{info: alistFileInfo{
				Name: rel, IsDir: true,
			}})
		}

		// 文件（Contents）
		for _, obj := range result.Contents {
			name := obj.Key
			if len(name) >= basePfxLen {
				name = name[basePfxLen:]
			}
			if name == "" || strings.HasSuffix(name, "/") {
				continue // 跳过目录占位对象
			}
			t, _ := time.Parse(time.RFC3339Nano, obj.LastModified)
			entries = append(entries, &alistDirEntry{info: alistFileInfo{
				Name:     name,
				Size:     obj.Size,
				IsDir:    false,
				Modified: t.Format(time.RFC3339),
			}})
		}

		if !result.IsTruncated {
			break
		}
		token = result.NextToken
	}

	if s.cacheEnabled {
		s.cacheMu.Lock()
		s.dirCache[cp] = &cacheEntry{data: entries, expiresAt: time.Now().Add(s.cacheTTL)}
		s.cacheMu.Unlock()
	}
	return entries, nil
}

// listObjects 调用 ListObjectsV2
func (s *S3FS) listObjects(prefix, delimiter, continuationToken string, maxKeys int) (*s3ListBucketResult, error) {
	query := url.Values{}
	query.Set("list-type", "2")
	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if delimiter != "" {
		query.Set("delimiter", delimiter)
	}
	if continuationToken != "" {
		query.Set("continuation-token", continuationToken)
	}
	if maxKeys > 0 {
		query.Set("max-keys", fmt.Sprintf("%d", maxKeys))
	}

	u, err := s.buildURL("", query)
	if err != nil {
		return nil, err
	}
	resp, err := s.do(http.MethodGet, u, "")
	if err != nil {
		return nil, fmt.Errorf("S3 ListObjectsV2 失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("S3 ListObjectsV2 状态码 %d: %s", resp.StatusCode, string(body))
	}

	var result s3ListBucketResult
	if err := xml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("解析 S3 列目录响应失败: %w", err)
	}
	return &result, nil
}

// Open 打开对象
func (s *S3FS) Open(p string) (VFSFile, error) {
	info, err := s.Stat(p)
	if err != nil {
		return nil, err
	}
	f := &s3File{
		fs:         s,
		remotePath: normalizeAlistPath(p),
		stat:       info,
	}
	if !info.IsDir() && s.blockSize > 0 && s.blockCount > 0 {
		f.blockCache = newWebDAVBlockCache(s.blockSize, s.blockCount)
	}
	return f, nil
}

// Walk 递归遍历
func (s *S3FS) Walk(root string, fn filepath.WalkFunc) error {
	root = normalizeAlistPath(root)
	return s.walkInternal(root, fn)
}

func (s *S3FS) walkInternal(p string, fn filepath.WalkFunc) error {
	info, err := s.Stat(p)
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
	entries, err := s.ReadDir(p)
	if err != nil {
		return fn(p, info, err)
	}
	for _, e := range entries {
		child := strings.TrimRight(p, "/") + "/" + e.Name()
		if err := s.walkInternal(child, fn); err != nil {
			if err == filepath.SkipDir {
				continue
			}
			return err
		}
	}
	return nil
}

// GetHTTPURL 返回预签名 URL（V2.3，供 FFmpeg 使用）
// 说明：对象存储直接使用 Authorization 头会被 FFmpeg 忽略；改用预签名 URL 是标准做法。
func (s *S3FS) GetHTTPURL(p string) (string, error) {
	key := s.keyForPath(normalizeAlistPath(p))
	return s.PresignGet(key, 2*time.Hour)
}

// PresignGet 生成一个 GET 对象的预签名 URL（AWS V4 查询字符串签名）
func (s *S3FS) PresignGet(key string, expires time.Duration) (string, error) {
	if expires <= 0 {
		expires = time.Hour
	}
	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStamp := now.Format("20060102")
	credentialScope := strings.Join([]string{dateStamp, s.region, "s3", "aws4_request"}, "/")

	q := url.Values{}
	q.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
	q.Set("X-Amz-Credential", s.accessKey+"/"+credentialScope)
	q.Set("X-Amz-Date", amzDate)
	q.Set("X-Amz-Expires", fmt.Sprintf("%d", int(expires.Seconds())))
	q.Set("X-Amz-SignedHeaders", "host")

	u, err := s.buildURL(key, q)
	if err != nil {
		return "", err
	}

	canonicalURI := u.EscapedPath()
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := canonicalQueryString(u.Query())
	canonicalHeaders := "host:" + u.Host + "\n"
	signedHeaders := "host"

	canonicalRequest := strings.Join([]string{
		http.MethodGet,
		canonicalURI,
		canonicalQuery,
		canonicalHeaders,
		signedHeaders,
		s3UnsignedPayload,
	}, "\n")

	hash := sha256.Sum256([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		hex.EncodeToString(hash[:]),
	}, "\n")

	kDate := hmacSHA256([]byte("AWS4"+s.secretKey), dateStamp)
	kRegion := hmacSHA256(kDate, s.region)
	kService := hmacSHA256(kRegion, "s3")
	kSigning := hmacSHA256(kService, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(kSigning, stringToSign))

	q.Set("X-Amz-Signature", signature)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// ==================== s3File ====================

type s3File struct {
	fs         *S3FS
	remotePath string
	stat       fs.FileInfo

	mu     sync.Mutex
	reader io.ReadCloser
	offset int64

	blockCache *webdavBlockCache
}

func (f *s3File) Stat() (fs.FileInfo, error) { return f.stat, nil }

func (f *s3File) Read(p []byte) (int, error) {
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

func (f *s3File) ReadAt(p []byte, off int64) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if off < 0 {
		return 0, fmt.Errorf("ReadAt: negative offset")
	}
	if f.blockCache != nil {
		return f.readAtCachedS3(p, off)
	}
	return f.readAtDirect(p, off)
}

func (f *s3File) readAtDirect(p []byte, off int64) (int, error) {
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

// openRangeReader 发起 Range 请求
func (f *s3File) openRangeReader(off, length int64) (io.ReadCloser, error) {
	key := f.fs.keyForPath(f.remotePath)
	u, err := f.fs.buildURL(key, nil)
	if err != nil {
		return nil, err
	}
	rangeHdr := ""
	if length > 0 {
		rangeHdr = fmt.Sprintf("bytes=%d-%d", off, off+length-1)
	} else if off > 0 {
		rangeHdr = fmt.Sprintf("bytes=%d-", off)
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	if rangeHdr != "" {
		req.Header.Set("Range", rangeHdr)
	}
	f.fs.signRequest(req, s3EmptyPayloadHash)

	// 使用长超时 client 专用于流读
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		resp.Body.Close()
		return nil, fmt.Errorf("S3 GET 状态码 %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	return resp.Body, nil
}

// readAtCachedS3 带块缓存的 ReadAt（与 alist 版逻辑相同）
func (f *s3File) readAtCachedS3(p []byte, off int64) (int, error) {
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

func (f *s3File) ensureBlock(cache *webdavBlockCache, blockOff, fileSize int64) (*webdavCacheBlock, error) {
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

func (f *s3File) Close() error {
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
