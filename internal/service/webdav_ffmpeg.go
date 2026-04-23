package service

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/nowen-video/nowen-video/internal/config"
)

// ==================== WebDAV → FFmpeg HTTP URL 转换 ====================
//
// FFmpeg 原生支持 HTTP(S) 输入，包括 Basic Auth（通过 URL 内嵌凭证）。
// 对于 webdav:// 前缀的路径，转换为完整 HTTP URL，供 ffmpeg -i 直接使用，
// 避免把整个视频文件先下载到本地。
//
// 注意：
//   - ffmpeg 不支持 webdav:// 协议，必须转为 http(s)://
//   - URL 内嵌的用户名/密码会自动 URL 编码
//   - ffmpeg 会根据 URL 里的用户名/密码自动发送 Basic Auth 头

// ResolveWebDAVFFmpegURL 把 webdav:// 路径翻译为 ffmpeg 可直接读取的 HTTP URL。
//
// 若输入不是 webdav:// 前缀，或 WebDAV 未启用/配置不完整，原样返回。
// 这保证本地路径完全不受影响。
func ResolveWebDAVFFmpegURL(cfg *config.Config, p string) string {
	if !strings.HasPrefix(p, WebDAVScheme) {
		return p
	}
	if cfg == nil || !cfg.Storage.WebDAV.Enabled || cfg.Storage.WebDAV.ServerURL == "" {
		return p
	}

	webdavCfg := cfg.Storage.WebDAV

	// 1) 解析服务器 URL
	base, err := url.Parse(webdavCfg.ServerURL)
	if err != nil {
		return p
	}

	// 2) 拼接相对路径（webdav:// 剥离 + basePath + 相对路径）
	rel := strings.TrimPrefix(p, WebDAVScheme)
	rel = "/" + strings.TrimLeft(rel, "/")

	basePath := strings.Trim(webdavCfg.BasePath, "/")

	// base.Path 可能已经含 /dav/ 等前缀，需要保留
	serverBase := strings.TrimRight(base.Path, "/")
	var fullPath string
	if basePath == "" {
		fullPath = serverBase + rel
	} else {
		fullPath = serverBase + "/" + basePath + rel
	}
	base.Path = fullPath
	base.RawPath = "" // 重置 raw，让 Go 自动转义

	// 3) 内嵌 Basic Auth 凭证
	if webdavCfg.Username != "" {
		base.User = url.UserPassword(webdavCfg.Username, webdavCfg.Password)
	}

	return base.String()
}

// BuildFFmpegInputArgs 为 ffmpeg 输入构造前置参数（含 HTTP 协议超时与重连）。
// 对 HTTP(S) 输入自动注入：
//   - -reconnect 1 -reconnect_streamed 1 -reconnect_delay_max 5: 支持断线重连
//   - -timeout 30000000: 30 秒连接/读取超时
//   - -rw_timeout: 读写超时
//
// 返回值：放在 "-i" 之前的参数切片。
// 对于非 HTTP 源（本地文件）返回空切片。
func BuildFFmpegInputArgs(inputURL string) []string {
	if !strings.HasPrefix(inputURL, "http://") && !strings.HasPrefix(inputURL, "https://") {
		return nil
	}
	return []string{
		"-reconnect", "1",
		"-reconnect_streamed", "1",
		"-reconnect_delay_max", "5",
		"-rw_timeout", "30000000", // 30s in microseconds
		"-user_agent", "nowen-video/1.0",
	}
}

// SprintSafeFFmpegURL 日志友好：打印 URL 时屏蔽密码（避免密钥泄漏）
func SprintSafeFFmpegURL(u string) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return u
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		parsed.User = url.UserPassword(username, "***")
	}
	// V2.3: S3 预签名 URL 的 X-Amz-Signature 也要脱敏
	q := parsed.Query()
	if sig := q.Get("X-Amz-Signature"); sig != "" {
		q.Set("X-Amz-Signature", "***")
		parsed.RawQuery = q.Encode()
	}
	return parsed.String()
}

// ==================== V2.3: 统一远程存储 FFmpeg URL 解析 ====================
//
// 支持的前缀：
//   - webdav://  → ResolveWebDAVFFmpegURL（直接 HTTP + Basic Auth）
//   - alist://   → 调用 Alist API 拿 raw_url
//   - s3://      → 生成 2 小时预签名 URL
//   - 其他       → 原样返回（本地文件）
//
// 注入方式：由 service.go 初始化时调用 SetRemoteStorageService 注入。
var globalRemoteStorageSvc *RemoteStorageService

// SetGlobalRemoteStorageService 设置全局远程存储服务（用于 URL 解析）
func SetGlobalRemoteStorageService(svc *RemoteStorageService) {
	globalRemoteStorageSvc = svc
}

// ResolveRemoteFFmpegURL 解析任意远程路径为 ffmpeg 可用的 HTTP URL。
//
// 这是所有调用 ResolveWebDAVFFmpegURL 的推荐替代方案（向后兼容）。
func ResolveRemoteFFmpegURL(cfg *config.Config, p string) string {
	switch {
	case strings.HasPrefix(p, WebDAVScheme):
		return ResolveWebDAVFFmpegURL(cfg, p)
	case strings.HasPrefix(p, AlistScheme):
		return resolveAlistFFmpegURL(p)
	case strings.HasPrefix(p, S3Scheme):
		return resolveS3FFmpegURL(p)
	}
	return p
}

// resolveAlistFFmpegURL 通过 Alist API 获取 raw_url
func resolveAlistFFmpegURL(p string) string {
	if globalRemoteStorageSvc == nil {
		return p
	}
	alistFS := globalRemoteStorageSvc.GetAlistFS()
	if alistFS == nil {
		return p
	}
	rel := "/" + strings.TrimLeft(strings.TrimPrefix(p, AlistScheme), "/")
	rawURL, err := alistFS.GetHTTPURL(rel)
	if err != nil {
		return p
	}
	return rawURL
}

// resolveS3FFmpegURL 生成 S3 预签名 URL
func resolveS3FFmpegURL(p string) string {
	if globalRemoteStorageSvc == nil {
		return p
	}
	s3fs := globalRemoteStorageSvc.GetS3FS()
	if s3fs == nil {
		return p
	}
	rel := "/" + strings.TrimLeft(strings.TrimPrefix(p, S3Scheme), "/")
	signedURL, err := s3fs.GetHTTPURL(rel)
	if err != nil {
		return p
	}
	return signedURL
}

// _ 防止 fmt 被 IDE 移除（保留未来使用）
var _ = fmt.Sprintf
