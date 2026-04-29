// Package service - 文件夹扫描 & 自定义文件夹刮削（参考 mdcx 项目设计）
//
// 功能概览：
//   1. ScanFolder：递归扫描指定文件夹，识别视频文件并解析番号
//   2. StartFolderBatch：针对扫出的视频列表启动批量刮削任务
//      - 封面图落地到视频旁边（xxx-fanart.jpg + xxx-poster.jpg），符合 Emby/Jellyfin/Infuse 约定
//      - NFO 文件同样落在视频旁，供媒体中心识别
//      - 整个流程不依赖数据库 Media 记录，适合用户自选目录刮削
package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
)

// 常见视频后缀名（大小写不敏感）
var adultVideoExtensions = map[string]struct{}{
	".mp4": {}, ".mkv": {}, ".avi": {}, ".wmv": {}, ".mov": {},
	".ts": {}, ".m2ts": {}, ".m4v": {}, ".webm": {}, ".flv": {},
	".iso": {}, ".vob": {}, ".rmvb": {}, ".rm": {}, ".3gp": {},
	".mpg": {}, ".mpeg": {}, ".mts": {}, ".f4v": {}, ".divx": {},
}

// FolderScanEntry 单个扫描到的视频条目
type FolderScanEntry struct {
	Path         string `json:"path"`          // 完整路径
	Filename     string `json:"filename"`      // 文件名（不含路径）
	RelPath      string `json:"rel_path"`      // 相对于扫描根的相对路径
	SizeMB       int64  `json:"size_mb"`       // 文件大小（MB）
	DetectedCode string `json:"detected_code"` // 识别出的番号（无则空）
	HasCode      bool   `json:"has_code"`      // 是否成功识别番号
	HasNFO       bool   `json:"has_nfo"`       // 旁边是否已有 .nfo 文件（判断是否已刮削过）
	HasPoster    bool   `json:"has_poster"`    // 旁边是否已有 poster 图片
}

// FolderScanResult 扫描结果
type FolderScanResult struct {
	Root         string            `json:"root"`           // 扫描根路径
	Total        int               `json:"total"`          // 扫到的视频总数
	WithCode     int               `json:"with_code"`      // 识别到番号的数量
	WithoutCode  int               `json:"without_code"`   // 未识别番号的数量
	AlreadyDone  int               `json:"already_done"`   // 旁边已有 NFO 的数量
	Entries      []FolderScanEntry `json:"entries"`        // 条目列表
	ScannedAt    time.Time         `json:"scanned_at"`     // 扫描完成时间
}

// ScanFolder 递归扫描指定目录下所有视频文件并解析番号
// recursive=true 时会深入子目录；false 只看当前层
// maxDepth>0 时限制递归深度（0=不限）
func (s *AdultScraperService) ScanFolder(root string, recursive bool, maxDepth int) (*FolderScanResult, error) {
	root = filepath.Clean(root)
	fi, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("无法访问路径: %w", err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", root)
	}

	result := &FolderScanResult{
		Root:      root,
		Entries:   make([]FolderScanEntry, 0, 64),
		ScannedAt: time.Now(),
	}

	rootDepth := strings.Count(root, string(os.PathSeparator))
	walkErr := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// 某个目录/文件不可读，跳过继续
			return nil
		}
		// 深度控制
		if d.IsDir() {
			if !recursive && p != root {
				return filepath.SkipDir
			}
			if maxDepth > 0 {
				cur := strings.Count(p, string(os.PathSeparator)) - rootDepth
				if cur > maxDepth {
					return filepath.SkipDir
				}
			}
			// 跳过常见特殊目录
			base := strings.ToLower(d.Name())
			if base == "$recycle.bin" || base == "system volume information" ||
				strings.HasPrefix(base, ".") {
				if p != root {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(p))
		if _, ok := adultVideoExtensions[ext]; !ok {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return nil
		}
		// 过滤过小文件（<50MB 视为样片/片头，非正片）
		if info.Size() < 50*1024*1024 {
			return nil
		}

		rel, _ := filepath.Rel(root, p)
		entry := FolderScanEntry{
			Path:     p,
			Filename: filepath.Base(p),
			RelPath:  rel,
			SizeMB:   info.Size() / (1024 * 1024),
		}
		// 番号识别：优先用增强版，其次兜底
		codeInfo := ParseCodeEnhanced(filepath.Base(p))
		if codeInfo.Number != "" {
			entry.DetectedCode = codeInfo.Number
			entry.HasCode = true
		} else if code, _ := ParseCode(filepath.Base(p)); code != "" {
			entry.DetectedCode = code
			entry.HasCode = true
		}

		// 检查旁边是否已有 NFO / Poster
		stem := strings.TrimSuffix(p, filepath.Ext(p))
		if _, err := os.Stat(stem + ".nfo"); err == nil {
			entry.HasNFO = true
		}
		// 检查多种常见 poster 命名
		dir := filepath.Dir(p)
		base := filepath.Base(stem)
		for _, suffix := range []string{"-poster.jpg", "-poster.png", "-thumb.jpg", ".jpg"} {
			if _, err := os.Stat(filepath.Join(dir, base+suffix)); err == nil {
				entry.HasPoster = true
				break
			}
		}

		result.Entries = append(result.Entries, entry)
		if entry.HasCode {
			result.WithCode++
		} else {
			result.WithoutCode++
		}
		if entry.HasNFO {
			result.AlreadyDone++
		}
		return nil
	})

	result.Total = len(result.Entries)
	if walkErr != nil {
		return result, fmt.Errorf("扫描过程出错: %w", walkErr)
	}
	return result, nil
}

// ==================== 文件夹批量刮削 ====================

// FolderBatchOptions 文件夹批量刮削选项
type FolderBatchOptions struct {
	Paths          []string // 待刮削的视频文件路径列表
	Aggregated     bool     // 是否使用聚合模式（更完整但更慢）
	Concurrency    int      // 并发数（默认 2）
	SkipIfHasNFO   bool     // 已有 NFO 的文件自动跳过
	OverrideCode   string   // 若所有文件共用同一番号可指定（一般不用）
}

// FolderBatchItemResult 单个条目结果
type FolderBatchItemResult struct {
	Path    string    `json:"path"`
	Code    string    `json:"code"`
	Status  string    `json:"status"` // success / failed / skipped
	Message string    `json:"message,omitempty"`
	Source  string    `json:"source,omitempty"` // 刮削源（javbus/javdb/...)
	At      time.Time `json:"at"`
}

// FolderBatchTask 文件夹批量刮削任务
type FolderBatchTask struct {
	ID         string                  `json:"id"`
	Status     string                  `json:"status"` // running / paused / cancelled / completed / failed
	Total      int                     `json:"total"`
	Current    int32                   `json:"current"`
	Success    int32                   `json:"success"`
	Failed     int32                   `json:"failed"`
	Skipped    int32                   `json:"skipped"`
	StartedAt  time.Time               `json:"started_at"`
	FinishedAt *time.Time              `json:"finished_at,omitempty"`
	Results    []FolderBatchItemResult `json:"results"`
	Aggregated bool                    `json:"aggregated"`
	Concurrency int                    `json:"concurrency"`

	cancelCtx context.Context
	cancelFn  context.CancelFunc
	mu        sync.Mutex
}

// AdultFolderBatchService 自定义文件夹批量刮削服务
type AdultFolderBatchService struct {
	scraper *AdultScraperService

	mu      sync.Mutex
	tasks   map[string]*FolderBatchTask
	history []*FolderBatchTask // 最多保存 50 条
}

// NewAdultFolderBatchService 构造服务
func NewAdultFolderBatchService(scraper *AdultScraperService) *AdultFolderBatchService {
	return &AdultFolderBatchService{
		scraper: scraper,
		tasks:   make(map[string]*FolderBatchTask),
		history: make([]*FolderBatchTask, 0, 50),
	}
}

// Start 启动一个文件夹批量刮削任务
func (fs *AdultFolderBatchService) Start(opts FolderBatchOptions) (string, error) {
	if fs.scraper == nil || !fs.scraper.IsEnabled() {
		return "", fmt.Errorf("番号刮削服务未启用")
	}
	if len(opts.Paths) == 0 {
		return "", fmt.Errorf("路径列表为空")
	}
	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 2
	}
	if concurrency > 8 {
		concurrency = 8
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &FolderBatchTask{
		ID:          uuid.NewString(),
		Status:      "running",
		Total:       len(opts.Paths),
		StartedAt:   time.Now(),
		Results:     make([]FolderBatchItemResult, 0, len(opts.Paths)),
		Aggregated:  opts.Aggregated,
		Concurrency: concurrency,
		cancelCtx:   ctx,
		cancelFn:    cancel,
	}

	fs.mu.Lock()
	fs.tasks[task.ID] = task
	fs.mu.Unlock()

	go fs.run(task, opts)
	return task.ID, nil
}

// Cancel 取消任务
func (fs *AdultFolderBatchService) Cancel(id string) error {
	fs.mu.Lock()
	t, ok := fs.tasks[id]
	fs.mu.Unlock()
	if !ok {
		return fmt.Errorf("任务不存在")
	}
	t.cancelFn()
	t.mu.Lock()
	t.Status = "cancelled"
	t.mu.Unlock()
	return nil
}

// Get 查询单任务
func (fs *AdultFolderBatchService) Get(id string) *FolderBatchTask {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.tasks[id]
}

// List 活跃 + 历史任务
func (fs *AdultFolderBatchService) List() ([]*FolderBatchTask, []*FolderBatchTask) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	active := make([]*FolderBatchTask, 0, len(fs.tasks))
	for _, t := range fs.tasks {
		active = append(active, t)
	}
	hist := make([]*FolderBatchTask, len(fs.history))
	copy(hist, fs.history)
	return active, hist
}

// ==================== 执行主循环 ====================

func (fs *AdultFolderBatchService) run(task *FolderBatchTask, opts FolderBatchOptions) {
	defer func() {
		if r := recover(); r != nil {
			fs.scraper.logger.Errorf("文件夹批量刮削 panic: %v", r)
			task.mu.Lock()
			task.Status = "failed"
			task.mu.Unlock()
		}
		finished := time.Now()
		task.mu.Lock()
		task.FinishedAt = &finished
		if task.Status == "running" {
			task.Status = "completed"
		}
		task.mu.Unlock()
		fs.archive(task)
	}()

	sem := make(chan struct{}, task.Concurrency)
	var wg sync.WaitGroup

	for _, p := range opts.Paths {
		if task.cancelCtx.Err() != nil {
			return
		}
		path := p
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			fs.processOne(task, path, opts)
		}()
	}
	wg.Wait()
}

// processOne 处理单个视频文件
func (fs *AdultFolderBatchService) processOne(task *FolderBatchTask, videoPath string, opts FolderBatchOptions) {
	result := FolderBatchItemResult{Path: videoPath, At: time.Now()}
	defer func() {
		atomic.AddInt32(&task.Current, 1)
		task.mu.Lock()
		task.Results = append(task.Results, result)
		task.mu.Unlock()
	}()

	// 1. 跳过已有 NFO 的文件
	if opts.SkipIfHasNFO {
		nfoPath := strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo"
		if _, err := os.Stat(nfoPath); err == nil {
			result.Status = "skipped"
			result.Message = "已有 NFO，跳过"
			atomic.AddInt32(&task.Skipped, 1)
			return
		}
	}

	// 2. 识别番号
	code := opts.OverrideCode
	if code == "" {
		info := ParseCodeEnhanced(filepath.Base(videoPath))
		if info.Number != "" {
			code = info.Number
		} else if c, _ := ParseCode(filepath.Base(videoPath)); c != "" {
			code = c
		}
	}
	if code == "" {
		result.Status = "failed"
		result.Message = "未识别出番号"
		atomic.AddInt32(&task.Failed, 1)
		return
	}
	result.Code = code

	// 3. 刮削元数据
	var meta *AdultMetadata
	var err error
	if opts.Aggregated {
		meta, _, err = fs.scraper.ScrapeByCodeAggregated(code)
	} else {
		meta, err = fs.scraper.ScrapeByCode(code)
	}
	if err != nil || meta == nil {
		result.Status = "failed"
		result.Message = fmt.Sprintf("刮削失败: %v", err)
		atomic.AddInt32(&task.Failed, 1)
		return
	}
	result.Source = meta.Source

	// 4. 构造临时 Media 对象（只需要 FilePath + ID）
	media := &model.Media{
		ID:       "folder-" + uuid.NewString()[:8],
		FilePath: videoPath,
		Title:    filepath.Base(videoPath),
	}
	// 5. 应用元数据（会自动下载封面到 cache_dir）
	if err := fs.scraper.ApplyToMedia(media, meta, "primary"); err != nil {
		result.Status = "failed"
		result.Message = "应用元数据失败: " + err.Error()
		atomic.AddInt32(&task.Failed, 1)
		return
	}

	// 6. 将封面复制到视频旁边（Emby/Jellyfin 识别所必须）
	if err := fs.saveSidecarPoster(videoPath, media.PosterPath, meta); err != nil {
		fs.scraper.logger.Debugf("封面旁路写入失败（不致命）: %v", err)
	}

	result.Status = "success"
	result.Message = "刮削完成"
	atomic.AddInt32(&task.Success, 1)
}

// saveSidecarPoster 把封面图复制一份到视频旁边（典型命名：xxx-poster.jpg + xxx-fanart.jpg）
// - localPosterPath：ApplyToMedia 已下载到 cache_dir 下的本地封面
// - meta.Thumb：fanart 缩略图（若不同于 cover 则单独下载）
func (fs *AdultFolderBatchService) saveSidecarPoster(videoPath, localPosterPath string, meta *AdultMetadata) error {
	if videoPath == "" {
		return fmt.Errorf("视频路径为空")
	}
	stem := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))

	// 1. poster 图（若 ApplyToMedia 已下载到 cache 中则直接复制）
	if localPosterPath != "" {
		if err := copyFileForFolderScrape(localPosterPath, stem+"-poster.jpg"); err != nil {
			return err
		}
	} else if meta.Cover != "" {
		if err := fs.scraper.downloadImageTo(meta.Cover, stem+"-poster.jpg"); err != nil {
			return err
		}
	}

	// 2. fanart 大图（若 meta.Thumb 存在且不同于 cover 则下载）
	if meta.Thumb != "" && meta.Thumb != meta.Cover {
		_ = fs.scraper.downloadImageTo(meta.Thumb, stem+"-fanart.jpg")
	} else if localPosterPath != "" {
		// 兜底：直接复用 poster 当 fanart
		_ = copyFileForFolderScrape(localPosterPath, stem+"-fanart.jpg")
	}
	return nil
}

// downloadImageTo 直接下载图片到指定路径（用于文件夹刮削的旁路封面写入）
// 与 downloadCover 区别：不走 cache_dir，而是写到调用方指定的绝对路径
func (s *AdultScraperService) downloadImageTo(imageURL, destPath string) error {
	if imageURL == "" {
		return fmt.Errorf("图片 URL 为空")
	}
	if !strings.HasPrefix(imageURL, "http") {
		imageURL = "https:" + imageURL
	}
	req, err := http.NewRequest("GET", imageURL, nil)
	if err != nil {
		return err
	}
	s.applyBrowserHeaders(req)
	// 设置 Referer
	if strings.Contains(imageURL, "javbus") {
		req.Header.Set("Referer", "https://www.javbus.com/")
	} else if strings.Contains(imageURL, "javdb") {
		req.Header.Set("Referer", "https://javdb.com/")
	} else if strings.Contains(imageURL, "dmm") || strings.Contains(imageURL, "fanza") {
		req.Header.Set("Referer", "https://www.dmm.co.jp/")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载图片返回异常状态码: %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}
	f, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

// copyFileForFolderScrape 简单文件复制（避免依赖额外库）
func copyFileForFolderScrape(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer df.Close()
	_, err = io.Copy(df, sf)
	return err
}

// archive 完成后归档到历史列表（最多保留 50 条）
func (fs *AdultFolderBatchService) archive(task *FolderBatchTask) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	delete(fs.tasks, task.ID)
	fs.history = append([]*FolderBatchTask{task}, fs.history...)
	if len(fs.history) > 50 {
		fs.history = fs.history[:50]
	}
}

// ==================== Cookie 连通性测试（辅助功能） ====================

// TestCookieLogin 用已配置 Cookie 请求目标站点首页，判断登录态是否有效
// site 枚举：javbus / javdb / freejavbt / jav321 / fanza / mgstage / fc2hub
func (s *AdultScraperService) TestCookieLogin(site string) (ok bool, message string, statusCode int) {
	var (
		testURL       string
		loggedKeyword string // 登录后页面通常会出现的关键字
	)
	switch strings.ToLower(site) {
	case "javbus":
		testURL = "https://www.javbus.com/"
		loggedKeyword = "" // JavBus 的登录态主要影响无码区访问，这里只探活
	case "javdb":
		testURL = "https://javdb.com/"
		loggedKeyword = "user_panel" // 登录后顶部会有 user_panel 元素
	case "freejavbt":
		testURL = "https://freejavbt.com/"
	case "jav321":
		testURL = "https://www.jav321.com/"
	case "fanza", "dmm":
		testURL = "https://www.dmm.co.jp/"
	case "mgstage":
		testURL = "https://www.mgstage.com/"
	case "fc2hub":
		testURL = "https://fc2hub.com/"
	default:
		return false, "未知站点: " + site, 0
	}

	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false, "构造请求失败: " + err.Error(), 0
	}
	s.applyBrowserHeaders(req)
	resp, err := s.client.Do(req)
	if err != nil {
		return false, "请求失败: " + err.Error(), 0
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 最多读 1MB
	statusCode = resp.StatusCode

	if resp.StatusCode >= 400 {
		return false, fmt.Sprintf("HTTP 状态码异常: %d", resp.StatusCode), statusCode
	}

	cookie := s.lookupSiteCookie(req.URL.Host)
	if cookie == "" {
		return true, fmt.Sprintf("未配置该站点 Cookie，仅完成基本连通性测试（HTTP %d）", statusCode), statusCode
	}

	// 若能识别登录关键字则视为登录成功
	if loggedKeyword != "" && strings.Contains(string(body), loggedKeyword) {
		return true, fmt.Sprintf("登录态有效（HTTP %d，检测到关键字 %q）", statusCode, loggedKeyword), statusCode
	}
	return true, fmt.Sprintf("站点连通（HTTP %d），Cookie 已注入但未检测到强登录标志；建议实际刮削一个番号验证", statusCode), statusCode
}
