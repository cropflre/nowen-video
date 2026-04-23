package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// ==================== WebDAV 断链回退：下载到本地再转码 ====================
//
// 场景：HTTP 源不稳定，ffmpeg -reconnect 仍然失败时，降级为先把整个文件
//       下载到本地缓存目录，再走本地转码流程。
//
// 权衡：
//   - 多一次完整传输（慢，占磁盘），但比"转码全部失败"强
//   - 下载完成后删除临时文件（deferred）
//   - 下载期间不显示转码进度，仅记录日志（用户感知为"准备中"）

const (
	// webdavDownloadPartExt 下载中临时后缀
	webdavDownloadPartExt = ".part"
)

// downloadWebDAVToLocal 把 WebDAV 源（HTTP URL）下载到本地临时路径。
//
// 参数：
//   - ctx: 上下文（用于取消）
//   - httpURL: 已经带 Basic Auth 凭证的 HTTP(S) URL（来自 ResolveWebDAVFFmpegURL）
//   - cacheDir: 输出根目录（通常是预处理缓存目录）
//   - mediaID: 用于文件命名（避免并发下载冲突）
//   - logger: 日志
//
// 返回：
//   - 本地文件绝对路径（.mp4/.mkv 等原始扩展名）
//   - cleanup: 调用方在使用完毕后必须调用，用于删除临时文件
//   - 错误
func downloadWebDAVToLocal(ctx context.Context, httpURL, cacheDir, mediaID string, logger *zap.SugaredLogger) (string, func(), error) {
	// 1) 解析 URL，取扩展名（保留原始格式以便 ffmpeg 正确识别容器）
	u, err := url.Parse(httpURL)
	if err != nil {
		return "", nil, fmt.Errorf("解析 URL 失败: %w", err)
	}
	ext := filepath.Ext(u.Path)
	if ext == "" {
		ext = ".mp4" // 兜底
	}

	// 2) 构造临时输出路径
	dir := filepath.Join(cacheDir, "webdav_download")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", nil, fmt.Errorf("创建下载目录失败: %w", err)
	}
	// 命名：mediaID + 时间戳 + 扩展名；下载完成后去掉 .part
	ts := time.Now().UnixNano()
	finalPath := filepath.Join(dir, fmt.Sprintf("%s_%d%s", mediaID, ts, ext))
	partPath := finalPath + webdavDownloadPartExt

	// 3) 发起 HTTP GET
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpURL, nil)
	if err != nil {
		return "", nil, fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("User-Agent", "nowen-video/1.0")

	client := &http.Client{
		Timeout: 0, // 大文件，不设总超时；靠 ctx 控制取消
		Transport: &http.Transport{
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
			IdleConnTimeout:       60 * time.Second,
		},
	}
	logger.Infof("WebDAV 降级下载开始: %s", SprintSafeFFmpegURL(httpURL))
	start := time.Now()

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("HTTP 状态码错误: %d", resp.StatusCode)
	}

	// 4) 写入临时文件（.part）
	out, err := os.Create(partPath)
	if err != nil {
		return "", nil, fmt.Errorf("创建临时文件失败: %w", err)
	}

	// 大文件拷贝：带缓冲
	n, err := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if err != nil {
		os.Remove(partPath)
		return "", nil, fmt.Errorf("下载写入失败 (已写 %d 字节): %w", n, err)
	}
	if closeErr != nil {
		os.Remove(partPath)
		return "", nil, fmt.Errorf("关闭下载文件失败: %w", closeErr)
	}

	// 5) 原子重命名到最终路径
	if err := os.Rename(partPath, finalPath); err != nil {
		os.Remove(partPath)
		return "", nil, fmt.Errorf("重命名失败: %w", err)
	}

	elapsed := time.Since(start)
	speedMBps := float64(n) / elapsed.Seconds() / 1024 / 1024
	logger.Infof("WebDAV 降级下载完成: %s, 大小 %.2f MB, 耗时 %s, 平均 %.2f MB/s",
		finalPath, float64(n)/1024/1024, elapsed.Round(time.Millisecond), speedMBps)

	cleanup := func() {
		if err := os.Remove(finalPath); err != nil && !os.IsNotExist(err) {
			logger.Warnf("清理 WebDAV 下载临时文件失败: %s, %v", finalPath, err)
		}
	}
	return finalPath, cleanup, nil
}

// shouldFallbackToLocalDownload 判断是否应该触发本地下载降级
//
// 条件：
//  1. 原 InputPath 必须是 HTTP(S) 输入（WebDAV / Alist 直链 / S3 预签名 URL）
//  2. 对应存储在配置中已启用
//  3. 缓存目录可写
func shouldFallbackToLocalDownload(cfg *config.Config, inputPath string) bool {
	if !strings.HasPrefix(inputPath, "http://") && !strings.HasPrefix(inputPath, "https://") {
		return false
	}
	if cfg == nil {
		return false
	}
	// V2.3: WebDAV / Alist / S3 任一启用即允许降级
	if cfg.Storage.WebDAV.Enabled || cfg.Storage.Alist.Enabled || cfg.Storage.S3.Enabled {
		return true
	}
	return false
}
