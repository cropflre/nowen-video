package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

const (
	// fastIndexBatchSize 第一阶段流式入库的单批大小：
	// 每发现 200 条即在一个事务内落库并推送一次进度，让前端在 1~2 秒内即可展示首屏。
	fastIndexBatchSize = 200

	// scanProgressThrottleInterval WebSocket 进度事件最小广播间隔（首批立即发送）。
	scanProgressThrottleInterval = 400 * time.Millisecond

	// asyncEnrichQueueCapacity 后台异步探测队列容量；超过时由库级补扫接管，绝不阻塞快扫。
	asyncEnrichQueueCapacity = 8192
)

var fastSubtitleExtSet = map[string]bool{
	".srt": true,
	".ass": true,
	".ssa": true,
	".vtt": true,
	".sub": true,
	".idx": true,
}

var fastPosterSuffixes = []string{
	"-poster.jpg", "-poster.png", "-poster.webp",
	"-cover.jpg", "-cover.png", "-cover.webp",
	"-thumb.jpg", "-thumb.png", "-thumb.webp",
	".jpg", ".png", ".webp",
}

var fastBackdropSuffixes = []string{
	"-fanart.jpg", "-fanart.png", "-fanart.webp",
	"-backdrop.jpg", "-backdrop.png", "-backdrop.webp",
	"-banner.jpg", "-banner.png", "-banner.webp",
}

// ==================== 目录快照缓存 ====================

type dirSnapshot struct {
	files         []string          // 非目录项的原始文件名（按 ReadDir 顺序）
	lowerToOrigin map[string]string // 小写文件名 -> 原始文件名
	videoCount    int               // 目录内视频文件数（用于判断是否允许使用通用封面）
	firstNFO      string            // 目录内首个 .nfo 文件名
	imageFiles    []string          // 目录内所有图片文件名
	subtitleFiles []string          // 目录内所有外挂字幕文件名
}

// scanDirCache 单次扫描生命周期内的目录快照缓存。
// 每个目录最多调用一次 ReadDir，将原本每个视频 15~25 次 os.Stat/ReadDir 降为纯内存查表。
type scanDirCache struct {
	scanner *ScannerService
	mu      sync.RWMutex
	dirs    map[string]*dirSnapshot
}

func newScanDirCache(scanner *ScannerService) *scanDirCache {
	return &scanDirCache{
		scanner: scanner,
		dirs:    make(map[string]*dirSnapshot),
	}
}

func (c *scanDirCache) get(dir string) *dirSnapshot {
	if c == nil || dir == "" {
		return nil
	}
	c.mu.RLock()
	snap, ok := c.dirs[dir]
	c.mu.RUnlock()
	if ok {
		return snap
	}

	var (
		entries []os.DirEntry
		err     error
	)
	if c.scanner != nil {
		entries, err = c.scanner.readDirLibraryPath(dir)
	} else {
		entries, err = os.ReadDir(dir)
	}
	if err != nil {
		snap = &dirSnapshot{}
	} else {
		snap = &dirSnapshot{
			lowerToOrigin: make(map[string]string, len(entries)),
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			lower := strings.ToLower(name)
			snap.files = append(snap.files, name)
			snap.lowerToOrigin[lower] = name

			ext := strings.ToLower(filepath.Ext(name))
			if nfoVideoExts[ext] {
				snap.videoCount++
			}
			if ext == ".nfo" && snap.firstNFO == "" {
				snap.firstNFO = name
			}
			if nfoImageExts[ext] {
				snap.imageFiles = append(snap.imageFiles, name)
			}
			if fastSubtitleExtSet[ext] {
				snap.subtitleFiles = append(snap.subtitleFiles, name)
			}
		}
	}

	c.mu.Lock()
	c.dirs[dir] = snap
	c.mu.Unlock()
	return snap
}

func dirOfMediaPath(p string) string {
	if IsWebDAVPath(p) {
		i := strings.LastIndex(p, "/")
		if i <= len("webdav:/") {
			return p
		}
		return p[:i]
	}
	return filepath.Dir(p)
}

func (c *scanDirCache) findExternalSubtitles(mediaPath string) string {
	if IsWebDAVPath(mediaPath) {
		return ""
	}
	dir := filepath.Dir(mediaPath)
	snap := c.get(dir)
	if snap == nil || len(snap.subtitleFiles) == 0 {
		return ""
	}
	baseLower := strings.ToLower(strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath)))
	var found []string
	for _, name := range snap.subtitleFiles {
		ext := filepath.Ext(name)
		nameWithoutExt := strings.ToLower(strings.TrimSuffix(name, ext))
		if strings.HasPrefix(nameWithoutExt, baseLower) {
			found = append(found, filepath.Join(dir, name))
		}
	}
	return strings.Join(found, "|")
}

func (c *scanDirCache) findNFOForMedia(mediaPath string) string {
	dir := dirOfMediaPath(mediaPath)
	snap := c.get(dir)
	if snap == nil {
		return ""
	}
	baseName := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	if orig, ok := snap.lowerToOrigin[strings.ToLower(baseName+".nfo")]; ok {
		return vfsJoin(dir, orig)
	}
	if snap.firstNFO != "" {
		return vfsJoin(dir, snap.firstNFO)
	}
	return ""
}

func (c *scanDirCache) findLocalImagesForMedia(mediaPath string) (poster, backdrop string) {
	dir := dirOfMediaPath(mediaPath)
	snap := c.get(dir)
	if snap == nil || len(snap.imageFiles) == 0 {
		return "", ""
	}
	baseLower := strings.ToLower(strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath)))

	// 阶段1：优先匹配与视频文件同名的图片
	for _, suffix := range fastPosterSuffixes {
		if orig, ok := snap.lowerToOrigin[baseLower+suffix]; ok {
			poster = vfsJoin(dir, orig)
			break
		}
	}
	for _, suffix := range fastBackdropSuffixes {
		if orig, ok := snap.lowerToOrigin[baseLower+suffix]; ok {
			backdrop = vfsJoin(dir, orig)
			break
		}
	}
	if poster != "" {
		return poster, backdrop
	}

	// 阶段2：仅当目录下只有一个视频文件时才使用通用封面
	if snap.videoCount <= 1 {
		for _, name := range standardPosterNames {
			if orig, ok := snap.lowerToOrigin[name]; ok {
				poster = vfsJoin(dir, orig)
				break
			}
		}
		for _, name := range standardBackdropNames {
			if orig, ok := snap.lowerToOrigin[name]; ok {
				backdrop = vfsJoin(dir, orig)
				break
			}
		}
		if poster == "" {
			for _, img := range snap.imageFiles {
				candidate := vfsJoin(dir, img)
				if candidate != backdrop {
					poster = candidate
					break
				}
			}
		}
	}
	return poster, backdrop
}

// ==================== 进度广播节流器 ====================

// scanProgressThrottler 对 EventScanProgress 做首包即发 + 时间窗节流，
// 避免万级文件扫描时向 WebSocket 狂发数万条消息拖垮前端。
type scanProgressThrottler struct {
	mu       sync.Mutex
	lastSent time.Time
	interval time.Duration
}

func newScanProgressThrottler() *scanProgressThrottler {
	return &scanProgressThrottler{interval: scanProgressThrottleInterval}
}

func (t *scanProgressThrottler) Emit(s *ScannerService, force bool, data *ScanProgressData) {
	if s == nil || s.wsHub == nil || data == nil {
		return
	}
	t.mu.Lock()
	now := time.Now()
	if !force && !t.lastSent.IsZero() && now.Sub(t.lastSent) < t.interval {
		t.mu.Unlock()
		return
	}
	t.lastSent = now
	t.mu.Unlock()
	s.broadcastScanEvent(EventScanProgress, data)
}

// ==================== Phase 1：流式快速入库 ====================

// prepareSTRMFastMeta 快速解析本地 .strm 文件 URL 与 Header（纯本地小文件读取，不做远程 FFprobe），
// 确保 STRM 条目在第一阶段秒级入库后即可直接播放。
func (s *ScannerService) prepareSTRMFastMeta(media *model.Media) {
	if media == nil || !isSTRMFile(media.FilePath) {
		return
	}
	meta, err := s.parseSTRMFileMeta(media.FilePath)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("解析 STRM 文件失败: %s, 错误: %v", media.FilePath, err)
		}
		return
	}
	ApplySTRMMetaToMedia(media, meta)
	s.probeSTRMMedia(media, meta.URL)
}

// enrichFastMovieItem 使用目录快照缓存为电影条目补齐外挂字幕、本地 NFO 和本地海报（零 FFprobe）。
func (s *ScannerService) enrichFastMovieItem(pm *pendingMedia, dirCache *scanDirCache) {
	if pm == nil || pm.media == nil {
		return
	}
	// .strm 先做本地快速解析（纯文本读取，微秒级）
	if isSTRMFile(pm.path) {
		s.prepareSTRMFastMeta(pm.media)
	}

	if subs := dirCache.findExternalSubtitles(pm.path); subs != "" {
		pm.media.SubtitlePaths = subs
	}

	if s.nfoService != nil {
		if nfoPath := dirCache.findNFOForMedia(pm.path); nfoPath != "" {
			if err := s.nfoService.ParseMovieNFO(nfoPath, pm.media); err != nil && s.logger != nil {
				s.logger.Debugf("解析NFO失败: %s, 错误: %v", nfoPath, err)
			}
		}
	}

	if poster, backdrop := dirCache.findLocalImagesForMedia(pm.path); poster != "" || backdrop != "" {
		if poster != "" && pm.media.PosterPath == "" {
			pm.media.PosterPath = poster
		}
		if backdrop != "" && pm.media.BackdropPath == "" {
			pm.media.BackdropPath = backdrop
		}
	}
}

// createMediaBatchInDB 批量写入媒体记录；若批量事务失败（如偶发唯一约束冲突），自动降级为逐条写入以最大化入库成功率。
func (s *ScannerService) createMediaBatchInDB(items []*model.Media) []*model.Media {
	if len(items) == 0 || s.mediaRepo == nil {
		return nil
	}
	if err := s.mediaRepo.CreateInBatches(items, 15); err == nil {
		return items
	}
	created := make([]*model.Media, 0, len(items))
	for _, m := range items {
		if err := s.mediaRepo.Create(m); err != nil {
			if s.logger != nil {
				s.logger.Warnf("保存媒体失败: %s, 错误: %v", m.FilePath, err)
			}
			continue
		}
		created = append(created, m)
	}
	return created
}

// flushMovieBatch 刷新一批新发现的电影记录到数据库，立即广播进度，并投递到后台异步探测队列。
func (s *ScannerService) flushMovieBatch(
	library *model.Library,
	batch []pendingMedia,
	dirCache *scanDirCache,
	throttler *scanProgressThrottler,
	totalNewCount *int,
	isFirstBatch bool,
) (overflowed bool) {
	if len(batch) == 0 {
		return false
	}

	// 同批次内处理多 CD 堆叠分组
	stackGroups := make(map[string][]*pendingMedia)
	for i := range batch {
		if batch[i].media.StackGroup != "" {
			stackGroups[batch[i].media.StackGroup] = append(stackGroups[batch[i].media.StackGroup], &batch[i])
		}
	}
	for _, group := range stackGroups {
		if len(group) > 1 {
			groupID := group[0].media.Title
			for _, pm := range group {
				pm.media.VersionGroup = groupID
			}
		}
	}

	models := make([]*model.Media, 0, len(batch))
	for i := range batch {
		s.enrichFastMovieItem(&batch[i], dirCache)
		models = append(models, batch[i].media)
	}

	created := s.createMediaBatchInDB(models)
	if len(created) == 0 {
		return false
	}

	*totalNewCount += len(created)
	lastTitle := created[len(created)-1].Title
	throttler.Emit(s, isFirstBatch, &ScanProgressData{
		LibraryID:   library.ID,
		LibraryName: library.Name,
		Phase:       "scanning",
		NewFound:    *totalNewCount,
		Message:     fmt.Sprintf("已快速索引 %d 部影片（最新: %s）", *totalNewCount, lastTitle),
	})

	if !s.enqueueAsyncEnrich(created...) {
		overflowed = true
	}
	return overflowed
}

// ==================== Phase 2：后台异步技术探测 + 演职人员补全 ====================

func (s *ScannerService) ensureAsyncEnrichStarted() {
	s.enrichOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.enrichCtx = ctx
		s.enrichCancel = cancel
		s.enrichQueue = make(chan *model.Media, asyncEnrichQueueCapacity)
		s.enrichPending = make(map[string]struct{})

		workers := runtime.NumCPU() / 2
		if workers < 2 {
			workers = 2
		}
		if workers > 4 {
			workers = 4
		}
		for i := 0; i < workers; i++ {
			s.enrichWorkerWg.Add(1)
			go s.asyncEnrichWorker()
		}
	})
}

func (s *ScannerService) asyncEnrichWorker() {
	defer s.enrichWorkerWg.Done()
	for {
		select {
		case <-s.enrichCtx.Done():
			return
		case media, ok := <-s.enrichQueue:
			if !ok {
				return
			}
			s.processAsyncEnrichItem(media)
		}
	}
}

func (s *ScannerService) processAsyncEnrichItem(media *model.Media) {
	defer func() {
		s.enrichMu.Lock()
		if media != nil {
			delete(s.enrichPending, media.ID)
		}
		s.enrichMu.Unlock()
		s.enrichInFlight.Done()
	}()

	if media == nil || media.ID == "" || media.FilePath == "" || s.mediaRepo == nil {
		return
	}

	// 1. FFprobe 技术信息探测（通过单飞保护，避免与按需即探重复探测同一文件）
	needProbe := media.VideoCodec == "" && media.Duration == 0
	if !needProbe && isSTRMFile(media.FilePath) && s.cfg != nil && s.cfg.STRM.RemoteProbe && media.Duration == 0 {
		needProbe = true
	}
	if needProbe {
		s.probeMediaOnce(media)
	}

	// 2. 静态图片防御：若后台探测发现是伪装成 .mp4/.mkv 的单帧缩略图，自动清理该幽灵记录
	if isStillImageMedia(media) {
		if s.logger != nil {
			s.logger.Warnf("后台探测识别到静态图片伪视频，自动移除记录: %s [%s]", media.FilePath, media.VideoCodec)
		}
		_ = s.mediaRepo.DeleteByID(media.ID)
		return
	}

	// 3. 演职人员 NFO 导入（延后至后台执行，避免第一阶段争抢 SQLite 写锁）
	if s.castImporter != nil {
		s.castImporter(media)
	}
}

// enqueueAsyncEnrich 将新入库或变更的媒体投递到后台异步探测队列（非阻塞）。
// 返回 false 表示队列已满（部分条目未入队，调用方可在扫描结束时触发库级补扫）。
func (s *ScannerService) enqueueAsyncEnrich(items ...*model.Media) bool {
	if s == nil || len(items) == 0 {
		return true
	}
	s.ensureAsyncEnrichStarted()

	allQueued := true
	for _, media := range items {
		if media == nil || media.ID == "" {
			continue
		}
		// 复制一份快照，避免调用方并发修改指针
		snapshot := *media

		s.enrichMu.Lock()
		if s.enrichClosed {
			s.enrichMu.Unlock()
			return false
		}
		if _, exists := s.enrichPending[snapshot.ID]; exists {
			s.enrichMu.Unlock()
			continue
		}
		s.enrichPending[snapshot.ID] = struct{}{}
		s.enrichInFlight.Add(1)
		s.enrichMu.Unlock()

		select {
		case s.enrichQueue <- &snapshot:
		default:
			// 队列已满：不阻塞第一阶段快扫，由扫描结束后的库级补扫接管
			s.enrichMu.Lock()
			delete(s.enrichPending, snapshot.ID)
			s.enrichMu.Unlock()
			s.enrichInFlight.Done()
			allQueued = false
		}
	}
	return allQueued
}

// scheduleLibraryUnprobedSweep 当扫描文件数超过内存队列容量时，在后台分页扫描该库剩余未探测记录并补全。
func (s *ScannerService) scheduleLibraryUnprobedSweep(libraryID string) {
	if s == nil || libraryID == "" || s.mediaRepo == nil {
		return
	}
	s.ensureAsyncEnrichStarted()
	go func() {
		afterID := ""
		for {
			if s.enrichCtx != nil && s.enrichCtx.Err() != nil {
				return
			}
			rows, err := s.mediaRepo.ListUnprobedByLibrary(libraryID, afterID, 200)
			if err != nil || len(rows) == 0 {
				return
			}
			for i := range rows {
				if s.enrichCtx != nil && s.enrichCtx.Err() != nil {
					return
				}
				m := rows[i]
				s.enrichMu.Lock()
				if s.enrichClosed {
					s.enrichMu.Unlock()
					return
				}
				if _, exists := s.enrichPending[m.ID]; exists {
					s.enrichMu.Unlock()
					continue
				}
				s.enrichPending[m.ID] = struct{}{}
				s.enrichInFlight.Add(1)
				s.enrichMu.Unlock()

				select {
				case <-s.enrichCtx.Done():
					s.enrichMu.Lock()
					delete(s.enrichPending, m.ID)
					s.enrichMu.Unlock()
					s.enrichInFlight.Done()
					return
				case s.enrichQueue <- &m:
				}
			}
			afterID = rows[len(rows)-1].ID
			if len(rows) < 200 {
				return
			}
		}
	}()
}

// WaitAsyncEnrich 等待当前已入队的后台异步探测任务全部完成（主要用于测试验证）。
func (s *ScannerService) WaitAsyncEnrich() {
	if s == nil {
		return
	}
	s.enrichInFlight.Wait()
}

// ShutdownAsyncEnrich 优雅停止后台异步探测 Worker。
func (s *ScannerService) ShutdownAsyncEnrich() {
	if s == nil {
		return
	}
	s.enrichMu.Lock()
	if s.enrichClosed {
		s.enrichMu.Unlock()
		return
	}
	s.enrichClosed = true
	cancel := s.enrichCancel
	s.enrichMu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.enrichWorkerWg.Wait()
}

// ==================== 按需即探（On-Demand Probe） ====================

// EnsureMediaProbed 按需探测单条媒体：当用户打开详情或点击播放、而后台探测尚未轮到该片时，
// 通过 singleflight 立即执行一次 FFprobe 并持久化技术字段，避免播放决策缺失编码信息。
func (s *ScannerService) EnsureMediaProbed(media *model.Media) bool {
	if s == nil || media == nil || media.FilePath == "" {
		return false
	}
	if media.VideoCodec != "" || media.Duration > 0 {
		return false
	}
	return s.probeMediaOnce(media)
}

func (s *ScannerService) probeMediaOnce(media *model.Media) bool {
	if s == nil || media == nil || media.FilePath == "" {
		return false
	}
	key := media.ID
	if key == "" {
		key = media.FilePath
	}
	res, _, _ := s.probeFlights.Do(key, func() (any, error) {
		snapshot := *media
		s.probeMediaInfo(&snapshot)
		if s.mediaRepo != nil && snapshot.ID != "" && !isStillImageMedia(&snapshot) {
			if snapshot.VideoCodec != "" || snapshot.AudioCodec != "" || snapshot.Resolution != "" || snapshot.Duration > 0 || snapshot.StreamURL != "" {
				_ = s.mediaRepo.UpdateTechnicalFields(&snapshot)
			}
		}
		return &snapshot, nil
	})
	probed, ok := res.(*model.Media)
	if !ok || probed == nil {
		return false
	}
	media.VideoCodec = probed.VideoCodec
	media.AudioCodec = probed.AudioCodec
	media.Resolution = probed.Resolution
	media.Duration = probed.Duration
	if probed.StreamURL != "" {
		media.StreamURL = probed.StreamURL
		media.StreamUA = probed.StreamUA
		media.StreamReferer = probed.StreamReferer
		media.StreamCookie = probed.StreamCookie
		media.StreamHeaders = probed.StreamHeaders
		media.StreamRefreshURL = probed.StreamRefreshURL
	}
	return media.VideoCodec != "" || media.Duration > 0 || media.StreamURL != ""
}
