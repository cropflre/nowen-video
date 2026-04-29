// Package service 番号刮削的扩展资源下载（剧照/演员头像/POST 请求辅助）
// 借鉴自 mdcx-master 的 extrafanart/actor_photo 下载流程
// 下载目录规范（Emby/Jellyfin 标准）：
//   - 剧照：<media_dir>/extrafanart/fanart1.jpg, fanart2.jpg ...
//   - 演员头像：<media_dir>/.actors/<name>.jpg
package service

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== Extra Fanart 下载 ====================

// DownloadExtraFanart 为媒体下载额外剧照到 extrafanart/ 子目录
// 若配置未启用或无剧照则直接返回
// 返回已下载的本地文件路径列表
func (s *AdultScraperService) DownloadExtraFanart(media *model.Media, meta *AdultMetadata) []string {
	if !s.cfg.AdultScraper.DownloadExtraFanart || len(meta.ExtraFanart) == 0 {
		return nil
	}
	if media.FilePath == "" || IsWebDAVPath(media.FilePath) {
		// WebDAV 路径不支持本地写入
		return nil
	}

	dir := filepath.Join(filepath.Dir(media.FilePath), "extrafanart")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.logger.Debugf("创建 extrafanart 目录失败: %v", err)
		return nil
	}

	maxCount := s.cfg.AdultScraper.MaxExtraFanart
	if maxCount <= 0 {
		maxCount = 10
	}
	if len(meta.ExtraFanart) < maxCount {
		maxCount = len(meta.ExtraFanart)
	}

	// 并发下载（限制并发度 3）
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		results  []string
		sem      = make(chan struct{}, 3)
		refererH = inferRefererFromSource(meta.Source)
	)

	for i := 0; i < maxCount; i++ {
		i := i
		fanartURL := meta.ExtraFanart[i]
		if fanartURL == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			ext := filepath.Ext(fanartURL)
			if ext == "" || len(ext) > 5 {
				ext = ".jpg"
			}
			localPath := filepath.Join(dir, fmt.Sprintf("fanart%d%s", i+1, ext))
			if err := s.downloadFileForAdult(fanartURL, localPath, refererH); err != nil {
				s.logger.Debugf("剧照下载失败 %s: %v", fanartURL, err)
				return
			}
			mu.Lock()
			results = append(results, localPath)
			mu.Unlock()
		}()
	}
	wg.Wait()

	if len(results) > 0 {
		s.logger.Infof("下载剧照成功: %s -> %d 张", meta.Code, len(results))
	}
	return results
}

// ==================== 演员头像下载 ====================

// DownloadActorPhotos 为演员下载头像到媒体同目录的 .actors/ 下
// Emby/Jellyfin 标准：<media_dir>/.actors/<actor_name>.jpg
func (s *AdultScraperService) DownloadActorPhotos(media *model.Media, meta *AdultMetadata) int {
	if !s.cfg.AdultScraper.DownloadActorPhoto || len(meta.ActorPhotos) == 0 {
		return 0
	}
	if media.FilePath == "" || IsWebDAVPath(media.FilePath) {
		return 0
	}

	dir := filepath.Join(filepath.Dir(media.FilePath), ".actors")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.logger.Debugf("创建 .actors 目录失败: %v", err)
		return 0
	}

	referer := inferRefererFromSource(meta.Source)
	count := 0
	for name, photoURL := range meta.ActorPhotos {
		if name == "" || photoURL == "" {
			continue
		}
		// 文件名清理（去除路径分隔符和特殊字符）
		safeName := sanitizeFilename(name)
		if safeName == "" {
			continue
		}
		ext := filepath.Ext(photoURL)
		if ext == "" || len(ext) > 5 {
			ext = ".jpg"
		}
		localPath := filepath.Join(dir, safeName+ext)
		// 已存在则跳过（避免重复下载）
		if _, err := os.Stat(localPath); err == nil {
			continue
		}
		if err := s.downloadFileForAdult(photoURL, localPath, referer); err != nil {
			s.logger.Debugf("演员头像下载失败 %s (%s): %v", name, photoURL, err)
			continue
		}
		count++
	}

	if count > 0 {
		s.logger.Infof("下载演员头像成功: %s -> %d 张", meta.Code, count)
	}
	return count
}

// ==================== 通用文件下载 ====================

// downloadFileForAdult 下载文件到本地（带 Referer 和浏览器头部）
func (s *AdultScraperService) downloadFileForAdult(fileURL, localPath, referer string) error {
	if !strings.HasPrefix(fileURL, "http") {
		if strings.HasPrefix(fileURL, "//") {
			fileURL = "https:" + fileURL
		} else {
			fileURL = "https://" + fileURL
		}
	}

	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}
	s.applyBrowserHeaders(req)
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return nil
}

// postFormForAdult 发送 POST 表单请求（JAV321 等站点使用）
func (s *AdultScraperService) postFormForAdult(targetURL, referer string, form url.Values) (string, error) {
	req, err := http.NewRequest("POST", targetURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	s.applyBrowserHeaders(req)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if referer != "" {
		req.Header.Set("Referer", referer)
	}

	client := s.client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// ==================== 工具函数 ====================

// inferRefererFromSource 根据数据源推断 Referer（防盗链）
func inferRefererFromSource(source string) string {
	switch source {
	case "javbus":
		return "https://www.javbus.com/"
	case "javdb":
		return "https://javdb.com/"
	case "freejavbt":
		return "https://freejavbt.com/"
	case "jav321":
		return "https://www.jav321.com/"
	default:
		return ""
	}
}

// sanitizeFilename 清理文件名中不合法的字符
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	// Windows/Linux 不合法字符
	badChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\n", "\r", "\t"}
	for _, c := range badChars {
		name = strings.ReplaceAll(name, c, "_")
	}
	// 限制长度
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

// doRequestReadString 执行 HTTP 请求并读取完整响应体为字符串
// 注：调用方需自行设置 headers、cookies 等
func doRequestReadString(client *http.Client, req *http.Request) (string, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
