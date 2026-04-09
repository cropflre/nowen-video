package service

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// 支持的视频文件扩展名
var supportedExts = map[string]bool{
	".mkv":  true,
	".mp4":  true,
	".avi":  true,
	".mov":  true,
	".wmv":  true,
	".flv":  true,
	".webm": true,
	".m4v":  true,
	".ts":   true,
	".strm": true, // STRM 远程流文件
}

// extrasExcludeDirs Emby/Kodi 标准的非正片内容目录名（小写）
var extrasExcludeDirs = map[string]bool{
	"extras":            true,
	"extra":             true,
	"featurettes":       true,
	"behind the scenes": true,
	"deleted scenes":    true,
	"interviews":        true,
	"trailers":          true,
	"trailer":           true,
	"samples":           true,
	"sample":            true,
	"shorts":            true,
	"scenes":            true,
	"bonus":             true,
	"bonus features":    true,
}

// extrasSuffixes Emby 标准的特典文件命名后缀（小写）
var extrasSuffixes = []string{
	"-behindthescenes", "-deleted", "-featurette",
	"-interview", "-scene", "-short", "-trailer", "-sample",
}

// isExtrasPath 判断文件路径是否在非正片目录下
func isExtrasPath(filePath string) bool {
	parts := strings.Split(filepath.ToSlash(filePath), "/")
	for _, part := range parts {
		if extrasExcludeDirs[strings.ToLower(part)] {
			return true
		}
	}
	return false
}

// isExtrasFile 判断文件名是否含有非正片后缀
func isExtrasFile(filename string) bool {
	lower := strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
	for _, suffix := range extrasSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

// idTagPatterns 从文件名/文件夹名中提取元数据 ID 的正则
var idTagPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[\[\{](tmdbid|tmdb)[=\-](\d+)[\]\}]`),
	regexp.MustCompile(`(?i)[\[\{](imdbid|imdb)[=\-](tt\d+)[\]\}]`),
	regexp.MustCompile(`(?i)[\[\{](tvdbid|tvdb)[=\-](\d+)[\]\}]`),
}

// yearInNamePattern 从文件名/文件夹名中提取年份 (2009) 或 [2009]
var yearInNamePattern = regexp.MustCompile(`[\(\[]((?:19|20)\d{2})[\)\]]`)

// parseIDFromName 从文件名/文件夹名中提取元数据 ID
func parseIDFromName(name string) (idType string, idValue string) {
	for _, pattern := range idTagPatterns {
		if m := pattern.FindStringSubmatch(name); len(m) >= 3 {
			return strings.ToLower(m[1]), m[2]
		}
	}
	return "", ""
}

// stackingPatterns 多 CD/多版本堆叠检测正则（P2）
var stackingPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[_\-\.\s](cd|disc|disk|part|pt|dvd)\s*(\d+)`),
	regexp.MustCompile(`(?i)[_\-\.\s](cd|disc|disk|part|pt|dvd)\s*([a-d])`),
}

// versionPatterns 多版本检测正则（P2: Director's Cut, Extended, Remastered 等）
var versionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(director'?s?\s*cut|extended|unrated|remastered|theatrical|imax|criterion|special\s*edition)`),
	regexp.MustCompile(`(?i)\b(remux|2160p|1080p|720p|4k|uhd|hdr|sdr|3d)\b`),
}

// extractYearFromName 从文件名/文件夹名中提取年份
func extractYearFromName(name string) int {
	if m := yearInNamePattern.FindStringSubmatch(name); len(m) >= 2 {
		year, _ := strconv.Atoi(m[1])
		if year >= 1900 && year <= 2099 {
			return year
		}
	}
	return 0
}

// FFprobeResult FFprobe输出结构
type FFprobeResult struct {
	Streams []FFprobeStream `json:"streams"`
	Format  FFprobeFormat   `json:"format"`
}

// FFprobeStream 流信息
type FFprobeStream struct {
	Index         int    `json:"index"`
	CodecType     string `json:"codec_type"` // video, audio, subtitle
	CodecName     string `json:"codec_name"` // h264, hevc, aac, srt, ass
	CodecLongName string `json:"codec_long_name"`
	Width         int    `json:"width"`
	Height        int    `json:"height"`
	Duration      string `json:"duration"`
	BitRate       string `json:"bit_rate"`
	// 字幕相关
	Tags        map[string]string  `json:"tags"`
	Disposition FFprobeDisposition `json:"disposition"`
}

// FFprobeDisposition 流标志
type FFprobeDisposition struct {
	Default int `json:"default"`
	Forced  int `json:"forced"`
}

// FFprobeFormat 格式信息
type FFprobeFormat struct {
	Filename       string `json:"filename"`
	Duration       string `json:"duration"`
	Size           string `json:"size"`
	BitRate        string `json:"bit_rate"`
	FormatName     string `json:"format_name"`
	FormatLongName string `json:"format_long_name"`
}

// SubtitleTrack 字幕轨道信息
type SubtitleTrack struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`    // srt, ass, subrip, hdmv_pgs_subtitle
	Language string `json:"language"` // chi, eng, jpn等
	Title    string `json:"title"`    // 字幕标题
	Default  bool   `json:"default"`  // 是否默认
	Forced   bool   `json:"forced"`   // 是否强制
	Bitmap   bool   `json:"bitmap"`   // 是否为图形字幕（PGS/VobSub等，不可提取为文本）
}

// isBitmapSubtitle 判断字幕编解码器是否为图形字幕
func isBitmapSubtitle(codec string) bool {
	switch strings.ToLower(codec) {
	case "hdmv_pgs_subtitle", "pgssub", "dvd_subtitle", "dvdsub", "dvb_subtitle", "xsub":
		return true
	default:
		return false
	}
}

// ScannerService 媒体文件扫描服务
type ScannerService struct {
	mediaRepo      *repository.MediaRepo
	seriesRepo     *repository.SeriesRepo
	cfg            *config.Config
	logger         *zap.SugaredLogger
	wsHub          *WSHub                 // WebSocket事件广播
	nfoService     *NFOService            // NFO 本地元数据解析服务
	onScanComplete func(libraryID string) // 扫描完成回调（用于触发预处理）
}

func NewScannerService(mediaRepo *repository.MediaRepo, seriesRepo *repository.SeriesRepo, cfg *config.Config, logger *zap.SugaredLogger) *ScannerService {
	return &ScannerService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		cfg:        cfg,
		logger:     logger,
		nfoService: NewNFOService(logger),
	}
}

// SetWSHub 设置WebSocket Hub（延迟注入，避免循环依赖）
func (s *ScannerService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// SetOnScanComplete 设置扫描完成回调（用于触发视频预处理）
func (s *ScannerService) SetOnScanComplete(fn func(libraryID string)) {
	s.onScanComplete = fn
}

// ScanLibrary 扫描媒体库目录
func (s *ScannerService) ScanLibrary(library *model.Library) (int, error) {
	s.logger.Infof("开始扫描媒体库: %s (%s)", library.Name, library.Path)

	// 发送扫描开始事件
	s.broadcastScanEvent(EventScanStarted, &ScanProgressData{
		LibraryID:   library.ID,
		LibraryName: library.Name,
		Phase:       "scanning",
		Message:     fmt.Sprintf("开始扫描媒体库: %s", library.Name),
	})

	// 根据媒体库类型采用不同的扫描策略
	var count int
	var err error

	switch library.Type {
	case "tvshow":
		count, err = s.scanTVShowLibrary(library)
	case "mixed":
		count, err = s.scanMixedLibrary(library)
	default:
		count, err = s.scanMovieLibrary(library)
	}

	if err != nil {
		s.broadcastScanEvent(EventScanFailed, &ScanProgressData{
			LibraryID:   library.ID,
			LibraryName: library.Name,
			Phase:       "scanning",
			NewFound:    count,
			Message:     fmt.Sprintf("扫描出错: %v", err),
		})
	} else {
		s.broadcastScanEvent(EventScanCompleted, &ScanProgressData{
			LibraryID:   library.ID,
			LibraryName: library.Name,
			Phase:       "scanning",
			NewFound:    count,
			Message:     fmt.Sprintf("扫描完成: %s, 新增 %d 个媒体", library.Name, count),
		})
	}

	s.logger.Infof("扫描完成: %s, 新增 %d 个媒体", library.Name, count)

	// 触发预处理回调（如果已配置）
	if s.onScanComplete != nil && count > 0 {
		go s.onScanComplete(library.ID)
	}

	return count, err
}

// scanMovieLibrary 扫描电影库（支持增量扫描 + P2 性能优化）
func (s *ScannerService) scanMovieLibrary(library *model.Library) (int, error) {
	var count int
	var totalFiles int     // 遍历到的总文件数
	var videoFiles int     // 识别到的视频文件数
	var skippedExist int   // 已存在且未变更跳过的文件数
	var skippedUpdated int // 已存在但已更新的文件数

	// 增量扫描：获取上次扫描时间，仅处理新增/变更的文件
	lastScanTime := time.Time{}
	if library.LastScan != nil {
		lastScanTime = *library.LastScan
	}

	s.logger.Infof("电影库扫描开始: %s, 路径: %s, 上次扫描: %v", library.Name, library.Path, lastScanTime)

	// P2: 文件路径预加载到内存 Set（避免 N+1 查询）
	existingPaths, err := s.mediaRepo.GetAllFilePathsByLibrary(library.ID)
	if err != nil {
		s.logger.Warnf("预加载文件路径失败，回退到逐个查询: %v", err)
		existingPaths = nil
	} else {
		s.logger.Infof("预加载 %d 个已有文件路径到内存", len(existingPaths))
	}

	// P2: 收集新发现的媒体文件，用于后续批量处理 FFprobe 和堆叠检测
	var pendingList []pendingMedia

	err = filepath.Walk(library.Path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			s.logger.Warnf("访问文件失败: %s, 错误: %v", path, err)
			return nil
		}
		if info.IsDir() {
			// 跳过 extras/trailers 等非正片目录（P0: 兼容 Emby 标准）
			if extrasExcludeDirs[strings.ToLower(filepath.Base(path))] {
				return filepath.SkipDir
			}
			return nil
		}
		totalFiles++
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			return nil
		}
		videoFiles++

		// P0: 文件大小过滤（启用 MinFileSize 配置）
		if library.EnableFileFilter && library.MinFileSize > 0 {
			minBytes := int64(library.MinFileSize) * 1024 * 1024
			if info.Size() < minBytes {
				s.logger.Debugf("跳过过小文件(%dMB < %dMB): %s",
					info.Size()/(1024*1024), library.MinFileSize, path)
				return nil
			}
		}

		// P0: 排除 extras 路径和 Emby 特典后缀文件
		if isExtrasPath(path) || isExtrasFile(filepath.Base(path)) {
			s.logger.Debugf("跳过非正片内容: %s", path)
			return nil
		}

		// P2: 内存查重（替代逐个 DB 查询）
		if existingPaths != nil {
			if existingPaths[path] {
				// 文件已存在：增量扫描模式下，如果文件未修改则跳过
				if !lastScanTime.IsZero() && info.ModTime().Before(lastScanTime) {
					skippedExist++
					return nil
				}
				// 文件已变更，更新文件大小和媒体信息
				skippedUpdated++
				existing, findErr := s.mediaRepo.FindByFilePath(path)
				if findErr == nil {
					existing.FileSize = info.Size()
					s.probeMediaInfo(existing)
					s.scanExternalSubtitles(existing)
					s.mediaRepo.Update(existing)
					s.logger.Debugf("更新已有媒体: %s", path)
				}
				return nil
			}
		} else {
			// 回退：逐个查询
			existing, findErr := s.mediaRepo.FindByFilePath(path)
			if findErr == nil {
				if !lastScanTime.IsZero() && info.ModTime().Before(lastScanTime) {
					skippedExist++
					return nil
				}
				skippedUpdated++
				existing.FileSize = info.Size()
				s.probeMediaInfo(existing)
				s.scanExternalSubtitles(existing)
				s.mediaRepo.Update(existing)
				s.logger.Debugf("更新已有媒体: %s", path)
				return nil
			}
		}

		// P0: 增强的标题提取（含年份 + ID 标签解析）
		filename := filepath.Base(path)
		title, year, tmdbID := s.extractTitleEnhanced(filename)

		// 提取 IMDB ID 标签（如 [imdbid=tt1234567]）
		imdbID := ""
		idType, idValue := parseIDFromName(filepath.Base(path))
		if idType == "imdbid" || idType == "imdb" {
			imdbID = idValue
		}

		media := &model.Media{
			LibraryID:    library.ID,
			Title:        title,
			FilePath:     path,
			FileSize:     info.Size(),
			MediaType:    "movie",
			Year:         year,
			TMDbID:       tmdbID,
			IMDbID:       imdbID,
			ScrapeStatus: "pending",
		}

		// P2: 检测多 CD 堆叠
		stackBase, stackOrder := detectStacking(filename)
		if stackOrder > 0 {
			media.StackGroup = stackBase
			media.StackOrder = stackOrder
			s.logger.Debugf("检测到堆叠文件: %s (组=%s, 序号=%d)", filename, stackBase, stackOrder)
		}

		// P2: 检测多版本标识
		if versionTag := detectVersionTag(filename); versionTag != "" {
			media.VersionTag = versionTag
			s.logger.Debugf("检测到版本标识: %s -> %s", filename, versionTag)
		}

		// 收集到待处理列表（FFprobe 后续并行处理）
		pendingList = append(pendingList, pendingMedia{media: media, path: path, info: info})
		return nil
	})

	// P2: 并行 FFprobe 探测 + 批量入库
	if len(pendingList) > 0 {
		s.logger.Infof("开始并行 FFprobe 探测 %d 个新文件", len(pendingList))
		s.parallelProbe(pendingList)

		// P2: 堆叠分组 — 为同一 StackGroup 的文件分配相同的 VersionGroup
		stackGroups := make(map[string][]*pendingMedia)
		for i := range pendingList {
			if pendingList[i].media.StackGroup != "" {
				stackGroups[pendingList[i].media.StackGroup] = append(stackGroups[pendingList[i].media.StackGroup], &pendingList[i])
			}
		}
		for _, group := range stackGroups {
			if len(group) > 1 {
				// 使用第一个文件的标题作为组标识
				groupID := group[0].media.Title
				for _, pm := range group {
					pm.media.VersionGroup = groupID
				}
			}
		}

		// 逐个入库（保留 NFO/图片扫描逻辑 + 事件广播）
		for _, pm := range pendingList {
			s.scanExternalSubtitles(pm.media)

			// 识别本地 NFO 信息文件并解析元数据
			if nfoPath := s.nfoService.FindNFOForMedia(pm.path); nfoPath != "" {
				if err := s.nfoService.ParseMovieNFO(nfoPath, pm.media); err != nil {
					s.logger.Debugf("解析NFO失败: %s, 错误: %v", nfoPath, err)
				} else {
					s.logger.Debugf("从NFO读取元数据: %s -> %s", nfoPath, pm.media.Title)
				}
			}

			// 识别本地海报封面图片
			mediaDir := filepath.Dir(pm.path)
			if poster, backdrop := s.nfoService.FindLocalImages(mediaDir); poster != "" || backdrop != "" {
				if poster != "" && pm.media.PosterPath == "" {
					pm.media.PosterPath = poster
					s.logger.Debugf("发现本地海报: %s", poster)
				}
				if backdrop != "" && pm.media.BackdropPath == "" {
					pm.media.BackdropPath = backdrop
					s.logger.Debugf("发现本地背景图: %s", backdrop)
				}
			}

			if err := s.mediaRepo.Create(pm.media); err != nil {
				s.logger.Warnf("保存媒体失败: %s, 错误: %v", pm.path, err)
				continue
			}
			count++
			s.logger.Infof("发现电影: %s [%s | %s | %s]", pm.media.Title, pm.media.Resolution, pm.media.VideoCodec, pm.media.AudioCodec)
			s.broadcastScanEvent(EventScanProgress, &ScanProgressData{
				LibraryID:   library.ID,
				LibraryName: library.Name,
				Phase:       "scanning",
				NewFound:    count,
				Message:     fmt.Sprintf("发现: %s [%s]", pm.media.Title, pm.media.Resolution),
			})
		}
	}

	s.logger.Infof("电影库扫描统计: %s — 遍历文件: %d, 视频文件: %d, 新增: %d, 已存在跳过: %d, 已更新: %d",
		library.Name, totalFiles, videoFiles, count, skippedExist, skippedUpdated)

	return count, err
}

// ==================== P2: 并行 FFprobe 探测 ====================

// pendingMedia 待处理的媒体文件信息（P2: 用于并行 FFprobe 和批量入库）
type pendingMedia struct {
	media *model.Media
	path  string
	info  os.FileInfo
}

// parallelProbe 使用 Worker Pool 并行执行 FFprobe 探测
func (s *ScannerService) parallelProbe(items []pendingMedia) {
	// 并发数 = min(CPU核数, 4)，避免 FFprobe 进程过多导致系统负载过高
	workers := runtime.NumCPU()
	if workers > 4 {
		workers = 4
	}
	if workers < 1 {
		workers = 1
	}

	type probeJob struct {
		index int
	}

	jobs := make(chan probeJob, len(items))
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				s.probeMediaInfo(items[job.index].media)
			}
		}()
	}

	for i := range items {
		jobs <- probeJob{index: i}
	}
	close(jobs)
	wg.Wait()
}

// ==================== P2: 多 CD 堆叠检测 ====================

// detectStacking 检测文件名中的多 CD/多分卷标识
// 返回: (去除堆叠后缀的基础名, 堆叠序号)，序号为 0 表示非堆叠文件
func detectStacking(filename string) (baseName string, order int) {
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	for _, pattern := range stackingPatterns {
		if m := pattern.FindStringSubmatchIndex(nameWithoutExt); m != nil {
			// 提取序号
			orderStr := nameWithoutExt[m[4]:m[5]]
			// 字母序号转数字: a=1, b=2, c=3, d=4
			if len(orderStr) == 1 && orderStr[0] >= 'a' && orderStr[0] <= 'd' {
				order = int(orderStr[0]-'a') + 1
			} else {
				order, _ = strconv.Atoi(orderStr)
			}
			if order > 0 {
				// 基础名 = 去除堆叠标识的部分
				baseName = strings.TrimSpace(nameWithoutExt[:m[0]])
				return baseName, order
			}
		}
	}
	return "", 0
}

// detectVersionTag 检测文件名中的版本标识（Director's Cut, Extended 等）
func detectVersionTag(filename string) string {
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))
	if m := versionPatterns[0].FindStringSubmatch(nameWithoutExt); len(m) >= 2 {
		return m[1]
	}
	return ""
}

// scanMixedLibrary
// scanMixedLibrary 扫描混合媒体库（智能区分电影和电视剧）
// 策略：遍历根目录第一层，对每个子目录判断是电影还是电视剧文件夹
// - 如果子目录内包含多个视频文件，或文件名匹配剧集命名模式，则视为电视剧
// - 如果子目录内只有单个视频文件且不匹配剧集模式，则视为电影
// - 根目录下的散落视频文件按电影处理
func (s *ScannerService) scanMixedLibrary(library *model.Library) (int, error) {
	s.logger.Infof("混合媒体库扫描: %s (%s)", library.Name, library.Path)

	entries, err := os.ReadDir(library.Path)
	if err != nil {
		return 0, fmt.Errorf("读取媒体库目录失败: %w", err)
	}

	s.logger.Infof("混合库根目录包含 %d 个条目", len(entries))

	var totalCount int
	// === 阶段一：收集子目录，按标准化系列名分组（用于多季合并检测） ===
	seriesDirGroups := make(map[string][]seriesFolder) // 标准化系列名 -> 目录列表
	var movieDirs []os.DirEntry                        // 被判定为电影的目录
	var looseVideoFiles []os.DirEntry                  // 根目录散落的视频文件

	for _, entry := range entries {
		if !entry.IsDir() {
			// 根目录下的散落视频文件
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if supportedExts[ext] {
				looseVideoFiles = append(looseVideoFiles, entry)
			}
			continue
		}

		dirName := entry.Name()
		folderPath := filepath.Join(library.Path, dirName)

		// 智能判断：该目录是电视剧还是电影
		if s.isTVShowFolder(folderPath) {
			// 电视剧目录：按标准化系列名分组（支持多季合并）
			normalizedName := s.normalizeSeriesName(dirName)
			seasonNum := s.extractSeasonFromDirName(dirName)
			seriesDirGroups[normalizedName] = append(seriesDirGroups[normalizedName], seriesFolder{
				path:      folderPath,
				dirName:   dirName,
				seasonNum: seasonNum,
			})
		} else {
			// 电影目录
			movieDirs = append(movieDirs, entry)
		}
	}

	// === 阶段二：处理电视剧目录（复用 scanTVShowLibrary 的分组逻辑） ===
	for normalizedName, folders := range seriesDirGroups {
		if len(folders) == 1 && folders[0].seasonNum == 0 {
			// 单个目录且未识别到季号 → 独立处理
			f := folders[0]
			seriesTitle := s.extractSeriesTitle(f.dirName)
			newCount, err := s.scanSeriesFolder(library, f.path, seriesTitle)
			if err != nil {
				s.logger.Warnf("混合库-扫描剧集文件夹失败: %s, 错误: %v", f.path, err)
				continue
			}
			totalCount += newCount
		} else {
			// 多季合并
			newCount, err := s.scanMultiSeasonSeries(library, normalizedName, folders)
			if err != nil {
				s.logger.Warnf("混合库-扫描多季合集失败: %s, 错误: %v", normalizedName, err)
				continue
			}
			totalCount += newCount
		}
	}

	// === 阶段三：处理电影目录（扫描目录内的视频文件作为电影） ===
	for _, entry := range movieDirs {
		folderPath := filepath.Join(library.Path, entry.Name())
		err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if !supportedExts[ext] {
				return nil
			}
			if _, err := s.mediaRepo.FindByFilePath(path); err == nil {
				return nil // 已存在
			}
			title := s.extractTitle(filepath.Base(path))
			media := &model.Media{
				LibraryID: library.ID,
				Title:     title,
				FilePath:  path,
				FileSize:  info.Size(),
				MediaType: "movie",
			}
			s.probeMediaInfo(media)
			s.scanExternalSubtitles(media)
			if err := s.mediaRepo.Create(media); err != nil {
				s.logger.Warnf("保存媒体失败: %s, 错误: %v", path, err)
				return nil
			}
			totalCount++
			s.logger.Debugf("发现电影(混合库): %s [%s | %s | %s]", title, media.Resolution, media.VideoCodec, media.AudioCodec)
			s.broadcastScanEvent(EventScanProgress, &ScanProgressData{
				LibraryID:   library.ID,
				LibraryName: library.Name,
				Phase:       "scanning",
				NewFound:    totalCount,
				Message:     fmt.Sprintf("发现电影: %s [%s]", title, media.Resolution),
			})
			return nil
		})
		if err != nil {
			s.logger.Warnf("混合库-扫描电影目录失败: %s, 错误: %v", folderPath, err)
		}
	}

	// === 阶段四：处理根目录散落的视频文件（作为电影） ===
	for _, entry := range looseVideoFiles {
		filePath := filepath.Join(library.Path, entry.Name())
		if _, err := s.mediaRepo.FindByFilePath(filePath); err == nil {
			continue // 已存在
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		title := s.extractTitle(entry.Name())
		media := &model.Media{
			LibraryID: library.ID,
			Title:     title,
			FilePath:  filePath,
			FileSize:  info.Size(),
			MediaType: "movie",
		}
		s.probeMediaInfo(media)
		s.scanExternalSubtitles(media)
		if err := s.mediaRepo.Create(media); err != nil {
			s.logger.Warnf("保存媒体失败: %s, 错误: %v", filePath, err)
			continue
		}
		totalCount++
		s.logger.Debugf("发现电影(散落): %s [%s]", title, media.Resolution)
		s.broadcastScanEvent(EventScanProgress, &ScanProgressData{
			LibraryID:   library.ID,
			LibraryName: library.Name,
			Phase:       "scanning",
			NewFound:    totalCount,
			Message:     fmt.Sprintf("发现电影: %s [%s]", title, media.Resolution),
		})
	}

	s.logger.Infof("混合媒体库扫描完成: %s, 新增 %d 个媒体", library.Name, totalCount)
	return totalCount, nil
}

// isTVShowFolder 智能判断一个目录是否为电视剧文件夹
// 判断依据（满足任一即认定为电视剧）：
// 1. 目录名包含季号标识（如 S1、Season 1、第一季）
// 2. 目录内包含 Season 子目录
// 3. 目录内有多个视频文件且文件名匹配剧集命名模式（S01E01、EP01、第N集等）
// 4. 目录内有多个视频文件且文件名包含连续编号
func (s *ScannerService) isTVShowFolder(folderPath string) bool {
	dirName := filepath.Base(folderPath)

	// 规则1: 目录名包含季号标识
	if s.extractSeasonFromDirName(dirName) > 0 {
		return true
	}

	// 读取目录内容
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return false
	}

	// 规则2: 包含 Season 子目录
	var videoFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			for _, pattern := range seasonDirPatterns {
				if pattern.MatchString(entry.Name()) {
					return true
				}
			}
			// 递归检查子目录中的视频文件（只深入一层）
			subEntries, err := os.ReadDir(filepath.Join(folderPath, entry.Name()))
			if err == nil {
				for _, subEntry := range subEntries {
					if !subEntry.IsDir() {
						ext := strings.ToLower(filepath.Ext(subEntry.Name()))
						if supportedExts[ext] {
							videoFiles = append(videoFiles, subEntry.Name())
						}
					}
				}
			}
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if supportedExts[ext] {
				videoFiles = append(videoFiles, entry.Name())
			}
		}
	}

	// 只有0或1个视频文件 → 大概率是电影
	if len(videoFiles) <= 1 {
		return false
	}

	// 规则3: 多个视频文件中有匹配剧集命名模式的
	episodeMatchCount := 0
	for _, vf := range videoFiles {
		ep := s.parseEpisodeInfo(vf)
		if ep.EpisodeNum > 0 {
			episodeMatchCount++
		}
	}

	// 如果超过一半的视频文件匹配剧集模式，认定为电视剧
	if episodeMatchCount > 0 && episodeMatchCount >= len(videoFiles)/2 {
		return true
	}

	// 规则4: 有3个及以上视频文件（即使无法解析集号，多文件目录更可能是剧集）
	if len(videoFiles) >= 3 {
		return true
	}

	return false
}

// ==================== 剧集扫描逻辑 ====================

// 常见分辨率数字，用于排除误匹配
var resolutionNums = map[int]bool{
	240: true, 360: true, 480: true, 540: true,
	720: true, 1080: true, 1440: true, 2160: true, 4320: true,
}

// isResolutionContext 检查匹配位置前后是否有分辨率标志（如 p, P, i, I）
func isResolutionContext(filename string, matchEnd int) bool {
	if matchEnd < len(filename) {
		nextChar := filename[matchEnd]
		if nextChar == 'p' || nextChar == 'P' || nextChar == 'i' || nextChar == 'I' {
			return true
		}
	}
	return false
}

// 剧集命名模式正则
var episodePatterns = []*regexp.Regexp{
	// 模式0: S01E01 / S1E1 / s01e01
	regexp.MustCompile(`(?i)S(\d{1,2})\s*E(\d{1,4})`),
	// 模式1: S01.E01
	regexp.MustCompile(`(?i)S(\d{1,2})\.E(\d{1,4})`),
	// 模式2: 1x01 / 01x01
	regexp.MustCompile(`(?i)(\d{1,2})x(\d{1,4})`),
	// 模式3: 第01集 / 第1集
	regexp.MustCompile(`第\s*(\d{1,4})\s*集`),
	// 模式4: EP01 / EP.01 / Episode 01
	regexp.MustCompile(`(?i)(?:EP|Episode)\s*\.?\s*(\d{1,4})`),
	// 模式5: OVA01 / OVA 01 / SP01 / SP 01（特殊剧集类型+数字）
	regexp.MustCompile(`(?i)(?:OVA|OAD|SP|SPECIAL|NCOP|NCED)\s*(\d{1,4})`),
	// 模式6: E01（单独的E+数字）
	regexp.MustCompile(`(?i)\bE(\d{1,4})\b`),
	// 模式7: [01] / [001] / [12END] / [24END] — 方括号内的数字（可能带END/FINAL/完等后缀）
	regexp.MustCompile(`(?i)\[(\d{2,4})(?:END|FINAL|完)?\]`),
	// 模式8: - 01 - / .01. / 空格01空格
	regexp.MustCompile(`[\-\.\s](\d{2,4})[\]\-\.\s]`),
}

// multiEpPatterns 多集连播文件正则（优先于单集模式匹配）
var multiEpPatterns = []*regexp.Regexp{
	// S01E02-E03 / S01E02-E05 / S01E02-e03
	regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,4})\s*[-–~]\s*E(\d{1,4})`),
	// S01E02-03 (无前缀 E 的范围)
	regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,4})\s*[-–~]\s*(\d{1,4})`),
}

// dateEpisodePattern 日期格式集号正则（用于脱口秀/日播剧等）
// 匹配: 2024.01.15 / 2024-01-15 / 2024_01_15
var dateEpisodePattern = regexp.MustCompile(`((?:19|20)\d{2})[\.\-_](\d{2})[\.\-_](\d{2})`)

// 独立季号正则：从文件名中提取 S2、Season 2 等季号（不依赖集号）
var seasonInFilenamePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bS(\d{1,2})\b`),
	regexp.MustCompile(`(?i)\bSeason\s*(\d{1,2})\b`),
	regexp.MustCompile(`第\s*(\d{1,2})\s*季`),
}

// Season目录模式
var seasonDirPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^Season\s*(\d{1,2})$`),
	regexp.MustCompile(`(?i)^S(\d{1,2})$`),
	regexp.MustCompile(`^第\s*(\d{1,2})\s*季$`),
	regexp.MustCompile(`(?i)^Specials?$`),   // 特别篇
	regexp.MustCompile(`(?i)^Season\s*0+$`), // Season 0 / Season 00（Emby 特别篇格式）
}

// seriesFolder 多季合并时使用的目录信息
type seriesFolder struct {
	path      string // 完整路径
	dirName   string // 原始目录名
	seasonNum int    // 从目录名提取的季号（0表示未识别到季号）
}

// EpisodeInfo 解析出的剧集信息
type EpisodeInfo struct {
	SeasonNum     int
	EpisodeNum    int
	EpisodeNumEnd int // 多集连播结束集号（0=单集），如 S01E02-E05 → Start=2, End=5
	EpisodeTitle  string
	AirDate       string // 日期格式集号：2024-01-15（脱口秀/日播剧）
	FilePath      string
	FileInfo      os.FileInfo
}

// scanTVShowLibrary 扫描剧集库（基于文件夹的合集识别 + 根目录散落文件智能归类）
func (s *ScannerService) scanTVShowLibrary(library *model.Library) (int, error) {
	var totalNewEpisodes int

	s.logger.Infof("剧集库扫描开始: %s, 路径: %s", library.Name, library.Path)

	// 遍历媒体库根目录的第一层子目录，每个子目录视为一个剧集
	entries, err := os.ReadDir(library.Path)
	if err != nil {
		return 0, fmt.Errorf("读取媒体库目录失败: %w", err)
	}

	s.logger.Infof("剧集库根目录包含 %d 个条目", len(entries))

	// 收集根目录下的散落视频文件，按系列名分组
	type looseFile struct {
		entry os.DirEntry
		info  os.FileInfo
	}
	seriesGroups := make(map[string][]looseFile) // 系列名 -> 文件列表

	// === 阶段一：收集所有子目录，按标准化系列名分组 ===
	// 标准化系列名 -> 目录列表
	seriesDirGroups := make(map[string][]seriesFolder)

	for _, entry := range entries {
		if !entry.IsDir() {
			// 根目录下的视频文件
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if supportedExts[ext] {
				filePath := filepath.Join(library.Path, entry.Name())
				if _, err := s.mediaRepo.FindByFilePath(filePath); err == nil {
					continue // 已存在
				}
				info, _ := entry.Info()
				if info == nil {
					continue
				}
				// 从文件名提取系列名称用于智能归类
				seriesName := s.extractSeriesNameFromFile(entry.Name())
				if seriesName == "" {
					seriesName = "__ungrouped__"
				}
				seriesGroups[seriesName] = append(seriesGroups[seriesName], looseFile{entry: entry, info: info})
			}
			continue
		}

		dirName := entry.Name()
		folderPath := filepath.Join(library.Path, dirName)

		// 从目录名提取标准化系列名（去掉季号标识）和季号
		normalizedName := s.normalizeSeriesName(dirName)
		seasonNum := s.extractSeasonFromDirName(dirName)

		seriesDirGroups[normalizedName] = append(seriesDirGroups[normalizedName], seriesFolder{
			path:      folderPath,
			dirName:   dirName,
			seasonNum: seasonNum,
		})
	}

	// === 阶段二：处理分组后的目录 ===
	for normalizedName, folders := range seriesDirGroups {
		if len(folders) == 1 && folders[0].seasonNum == 0 {
			// 单个目录且未识别到季号 → 按原有逻辑独立处理
			f := folders[0]
			seriesTitle := s.extractSeriesTitle(f.dirName)
			newCount, err := s.scanSeriesFolder(library, f.path, seriesTitle)
			if err != nil {
				s.logger.Warnf("扫描剧集文件夹失败: %s, 错误: %v", f.path, err)
				continue
			}
			totalNewEpisodes += newCount
		} else {
			// 多个目录属于同一系列（如"一拳超人 S1"和"一拳超人 S2"）
			// 或单个目录但明确包含季号标识 → 合并到同一个 Series
			newCount, err := s.scanMultiSeasonSeries(library, normalizedName, folders)
			if err != nil {
				s.logger.Warnf("扫描多季合集失败: %s, 错误: %v", normalizedName, err)
				continue
			}
			totalNewEpisodes += newCount
		}
	}

	// 处理根目录散落文件的智能归类
	for seriesName, files := range seriesGroups {
		if len(files) <= 1 && seriesName == "__ungrouped__" {
			// 单个无法识别系列名的文件，作为独立媒体处理
			for _, f := range files {
				filePath := filepath.Join(library.Path, f.entry.Name())
				title := s.extractTitle(f.entry.Name())
				media := &model.Media{
					LibraryID: library.ID,
					Title:     title,
					FilePath:  filePath,
					FileSize:  f.info.Size(),
					MediaType: "episode",
				}
				s.probeMediaInfo(media)
				s.scanExternalSubtitles(media)
				ep := s.parseEpisodeInfo(f.entry.Name())
				media.SeasonNum = ep.SeasonNum
				media.EpisodeNum = ep.EpisodeNum
				media.EpisodeTitle = ep.EpisodeTitle
				if err := s.mediaRepo.Create(media); err != nil {
					s.logger.Warnf("保存媒体失败: %s, 错误: %v", filePath, err)
				}
				totalNewEpisodes++
			}
			continue
		}

		// 有多个同名系列的文件或者能识别系列名的文件，自动创建合集
		actualSeriesName := seriesName
		if seriesName == "__ungrouped__" {
			// 多个无法识别系列名的文件，使用文件名作为标题独立存储
			for _, f := range files {
				filePath := filepath.Join(library.Path, f.entry.Name())
				title := s.extractTitle(f.entry.Name())
				media := &model.Media{
					LibraryID: library.ID,
					Title:     title,
					FilePath:  filePath,
					FileSize:  f.info.Size(),
					MediaType: "episode",
				}
				s.probeMediaInfo(media)
				s.scanExternalSubtitles(media)
				ep := s.parseEpisodeInfo(f.entry.Name())
				media.SeasonNum = ep.SeasonNum
				media.EpisodeNum = ep.EpisodeNum
				media.EpisodeTitle = ep.EpisodeTitle
				if err := s.mediaRepo.Create(media); err != nil {
					s.logger.Warnf("保存媒体失败: %s, 错误: %v", filePath, err)
				}
				totalNewEpisodes++
			}
			continue
		}

		// 为同系列的散落文件创建虚拟合集
		// 使用"__loose__:系列名"作为虚拟文件夹路径来区分
		virtualFolderPath := filepath.Join(library.Path, "__loose__:"+actualSeriesName)

		series, err := s.seriesRepo.FindByFolderPath(virtualFolderPath)
		if err != nil {
			series = &model.Series{
				LibraryID:  library.ID,
				Title:      actualSeriesName,
				FolderPath: virtualFolderPath,
			}
			if err := s.seriesRepo.Create(series); err != nil {
				s.logger.Warnf("创建散落剧集合集失败: %s, 错误: %v", actualSeriesName, err)
				continue
			}
			s.logger.Infof("创建散落剧集合集: %s (ID=%s)", actualSeriesName, series.ID)
		}

		seasonSet := make(map[int]bool)
		var newCount int

		for _, f := range files {
			filePath := filepath.Join(library.Path, f.entry.Name())
			ep := s.parseEpisodeInfo(f.entry.Name())
			if ep.SeasonNum == 0 {
				ep.SeasonNum = 1
			}

			media := &model.Media{
				LibraryID:    library.ID,
				SeriesID:     series.ID,
				Title:        actualSeriesName,
				FilePath:     filePath,
				FileSize:     f.info.Size(),
				MediaType:    "episode",
				SeasonNum:    ep.SeasonNum,
				EpisodeNum:   ep.EpisodeNum,
				EpisodeTitle: ep.EpisodeTitle,
			}
			s.probeMediaInfo(media)
			s.scanExternalSubtitles(media)

			if err := s.mediaRepo.Create(media); err != nil {
				s.logger.Warnf("保存剧集失败: %s, 错误: %v", filePath, err)
				continue
			}

			seasonSet[ep.SeasonNum] = true
			newCount++

			s.logger.Debugf("发现散落剧集: %s S%02dE%02d [%s]", actualSeriesName, ep.SeasonNum, ep.EpisodeNum, media.Resolution)
			s.broadcastScanEvent(EventScanProgress, &ScanProgressData{
				LibraryID:   library.ID,
				LibraryName: library.Name,
				Phase:       "scanning",
				NewFound:    newCount,
				Message:     fmt.Sprintf("发现: %s S%02dE%02d", actualSeriesName, ep.SeasonNum, ep.EpisodeNum),
			})
		}

		// 更新合集统计
		allEpisodes, _ := s.mediaRepo.ListBySeriesID(series.ID)
		series.EpisodeCount = len(allEpisodes)
		series.SeasonCount = len(seasonSet)
		s.seriesRepo.Update(series)

		s.logger.Infof("散落剧集归类完成: %s, 新增 %d 集, 共 %d 季 %d 集",
			actualSeriesName, newCount, series.SeasonCount, series.EpisodeCount)

		totalNewEpisodes += newCount
	}

	return totalNewEpisodes, nil
}

// normalizeSeriesName 标准化系列名：从目录名中去掉季号标识，返回纯系列名
// 例如: "一拳超人 S1" → "一拳超人", "Breaking Bad Season 2" → "Breaking Bad", "一拳超人 第二季" → "一拳超人"
func (s *ScannerService) normalizeSeriesName(dirName string) string {
	title := s.extractSeriesTitle(dirName) // 先清理年份、编码等标记

	// 移除季号标识
	seasonPatterns := []string{
		`(?i)\s*S\d{1,2}\s*$`,            // 末尾 S1, S02
		`(?i)\s*Season\s*\d{1,2}\s*$`,    // 末尾 Season 1
		`\s*第\s*[一二三四五六七八九十\d]+\s*季\s*$`, // 末尾 第一季, 第2季
	}
	for _, p := range seasonPatterns {
		re := regexp.MustCompile(p)
		title = re.ReplaceAllString(title, "")
	}

	title = strings.TrimSpace(title)
	if title == "" {
		// 如果标准化后为空（极端情况），回退使用原始清理标题
		return s.extractSeriesTitle(dirName)
	}
	return title
}

// extractSeasonFromDirName 从目录名中提取季号
// 例如: "一拳超人 S2" → 2, "Breaking Bad Season 1" → 1, "一拳超人 第二季" → 2
func (s *ScannerService) extractSeasonFromDirName(dirName string) int {
	// 支持 S1, S02 格式
	if m := regexp.MustCompile(`(?i)\bS(\d{1,2})\b`).FindStringSubmatch(dirName); len(m) >= 2 {
		num, _ := strconv.Atoi(m[1])
		if num > 0 && num <= 30 {
			return num
		}
	}
	// 支持 Season 1, Season 02 格式
	if m := regexp.MustCompile(`(?i)\bSeason\s*(\d{1,2})\b`).FindStringSubmatch(dirName); len(m) >= 2 {
		num, _ := strconv.Atoi(m[1])
		if num > 0 && num <= 30 {
			return num
		}
	}
	// 支持中文 "第1季", "第二季"
	if m := regexp.MustCompile(`第\s*(\d{1,2})\s*季`).FindStringSubmatch(dirName); len(m) >= 2 {
		num, _ := strconv.Atoi(m[1])
		if num > 0 && num <= 30 {
			return num
		}
	}
	// 支持中文数字 "第一季" ~ "第十季"
	cnNumMap := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
	}
	if m := regexp.MustCompile(`第\s*([一二三四五六七八九十]+)\s*季`).FindStringSubmatch(dirName); len(m) >= 2 {
		if num, ok := cnNumMap[m[1]]; ok {
			return num
		}
	}
	return 0
}

// scanMultiSeasonSeries 扫描属于同一系列的多季目录，将其合并到一个 Series 中
// folders 中的 seriesFolder 包含各个季目录的路径、目录名和从目录名提取的季号
func (s *ScannerService) scanMultiSeasonSeries(library *model.Library, seriesTitle string, folders []seriesFolder) (int, error) {
	s.logger.Infof("扫描多季合集: %s (%d 个目录)", seriesTitle, len(folders))

	// 查找或创建统一的 Series 合集
	// 优先按第一个目录的 FolderPath 查找（兼容旧数据），
	// 然后按标题+媒体库查找，最后创建新的
	var series *model.Series

	// 1. 尝试按任意一个目录的 FolderPath 查找已有 Series
	for _, f := range folders {
		if existing, err := s.seriesRepo.FindByFolderPath(f.path); err == nil {
			series = existing
			break
		}
	}

	// 2. 按标题+媒体库查找（可能之前已经合并过）
	if series == nil {
		if existing, err := s.seriesRepo.FindByTitleAndLibrary(seriesTitle, library.ID); err == nil {
			series = existing
		}
	}

	// 3. 创建新合集，FolderPath 使用第一个目录（或虚拟路径）
	if series == nil {
		// 使用"__multi__:系列名"作为虚拟路径，标识这是一个多季合并的合集
		virtualPath := filepath.Join(library.Path, "__multi__:"+seriesTitle)
		series = &model.Series{
			LibraryID:  library.ID,
			Title:      seriesTitle,
			FolderPath: virtualPath,
		}
		if err := s.seriesRepo.Create(series); err != nil {
			return 0, fmt.Errorf("创建多季合集失败: %w", err)
		}
		s.logger.Infof("创建多季合集: %s (ID=%s, %d 个季目录)", seriesTitle, series.ID, len(folders))
	}

	// 识别本地 NFO 信息文件（从各季目录中查找）
	for _, f := range folders {
		if nfoPath := s.nfoService.FindNFOFile(f.path); nfoPath != "" {
			if err := s.nfoService.ParseTVShowNFO(nfoPath, series); err != nil {
				s.logger.Debugf("解析多季合集NFO失败: %s, 错误: %v", nfoPath, err)
			} else {
				s.logger.Debugf("从NFO读取多季合集元数据: %s -> %s", nfoPath, series.Title)
			}
			break // 只用第一个找到的NFO
		}
	}

	// 识别本地海报封面图片（从各季目录中查找）
	for _, f := range folders {
		if poster, backdrop := s.nfoService.FindLocalImages(f.path); poster != "" || backdrop != "" {
			if poster != "" && series.PosterPath == "" {
				series.PosterPath = poster
				s.logger.Debugf("发现多季合集本地海报: %s", poster)
			}
			if backdrop != "" && series.BackdropPath == "" {
				series.BackdropPath = backdrop
				s.logger.Debugf("发现多季合集本地背景图: %s", backdrop)
			}
			if series.PosterPath != "" && series.BackdropPath != "" {
				break
			}
		}
	}

	// 保存NFO和图片更新
	s.seriesRepo.Update(series)

	var totalNewCount int
	seasonSet := make(map[int]bool)

	// 扫描每个季目录
	for _, f := range folders {
		episodes := s.collectEpisodes(f.path)
		if len(episodes) == 0 {
			s.logger.Debugf("多季合集目录无视频文件: %s", f.path)
			continue
		}

		// 如果目录名带有明确的季号，且剧集文件未识别出季号，则使用目录季号
		dirSeasonNum := f.seasonNum
		if dirSeasonNum == 0 {
			// 尝试用 parseSeasonFromDir 再识别一次
			dirSeasonNum = s.parseSeasonFromDir(f.dirName)
		}

		// === 集号重编逻辑 ===
		// 当检测到同一季目录下的集号是全局连续编号（延续上一季），而非从1开始时，
		// 自动重新编为季内相对编号。
		// 例如：第二季目录下文件名编号 [13][14]...[24]，应重编为 1,2,...,12
		if dirSeasonNum > 1 && len(episodes) > 0 {
			// 收集本目录下属于相同季号的"普通"剧集（排除OVA/SP等特殊类型的集号）
			var normalEpNums []int
			for _, ep := range episodes {
				// 判断是否为特殊剧集类型（OVA/SP等），它们的集号不参与重编判断
				isSpecial := false
				if m := episodePatterns[5].FindStringSubmatch(filepath.Base(ep.FilePath)); len(m) >= 2 {
					isSpecial = true
				}
				if !isSpecial && ep.EpisodeNum > 0 {
					normalEpNums = append(normalEpNums, ep.EpisodeNum)
				}
			}

			// 如果普通集号的最小值大于1，且集号是连续的，说明是全局编号需要重编
			if len(normalEpNums) > 0 {
				sort.Ints(normalEpNums)
				minEp := normalEpNums[0]

				if minEp > 1 {
					// 检查集号是否大致连续（允许少量缺失）
					isSequential := true
					for i := 1; i < len(normalEpNums); i++ {
						gap := normalEpNums[i] - normalEpNums[i-1]
						if gap > 2 { // 允许最多跳1集
							isSequential = false
							break
						}
					}

					if isSequential {
						// 计算偏移量，将集号重编为从1开始
						offset := minEp - 1
						s.logger.Infof("多季合集集号重编: %s 第%d季, 集号偏移 -%d (原始范围: %d~%d → 重编为 1~%d)",
							seriesTitle, dirSeasonNum, offset, minEp, normalEpNums[len(normalEpNums)-1], len(normalEpNums))

						for i := range episodes {
							// 只重编普通剧集，不重编OVA/SP等
							isSpecial := false
							if m := episodePatterns[5].FindStringSubmatch(filepath.Base(episodes[i].FilePath)); len(m) >= 2 {
								isSpecial = true
							}
							if !isSpecial && episodes[i].EpisodeNum > offset {
								episodes[i].EpisodeNum -= offset
							}
						}
					}
				}
			}
		}

		for _, ep := range episodes {
			// 季号分配：
			// 当目录名有明确季号时，优先使用目录季号（除非文件名中有不同的、合理的季号如S2标识的OVA）
			seasonNum := ep.SeasonNum
			if dirSeasonNum > 0 {
				// 如果文件名中的季号与目录季号不同且>1，说明文件自带了明确季号（如OVA标S2），保留它
				// 否则一律使用目录季号
				if seasonNum <= 1 || seasonNum == dirSeasonNum {
					seasonNum = dirSeasonNum
				}
			}
			if seasonNum == 0 {
				seasonNum = 1
			}

			// 检查是否已存在，如果存在则修正可能的脏数据（如 episode_title、season_num、episode_num）
			if existing, err := s.mediaRepo.FindByFilePath(ep.FilePath); err == nil {
				seasonSet[seasonNum] = true
				needUpdate := false
				if existing.EpisodeTitle != ep.EpisodeTitle {
					existing.EpisodeTitle = ep.EpisodeTitle
					needUpdate = true
				}
				if existing.SeasonNum != seasonNum {
					existing.SeasonNum = seasonNum
					needUpdate = true
				}
				if existing.EpisodeNum != ep.EpisodeNum {
					existing.EpisodeNum = ep.EpisodeNum
					needUpdate = true
				}
				if needUpdate {
					s.mediaRepo.Update(existing)
				}
				continue
			}

			media := &model.Media{
				LibraryID:    library.ID,
				SeriesID:     series.ID,
				Title:        seriesTitle,
				FilePath:     ep.FilePath,
				FileSize:     ep.FileInfo.Size(),
				MediaType:    "episode",
				SeasonNum:    seasonNum,
				EpisodeNum:   ep.EpisodeNum,
				EpisodeTitle: ep.EpisodeTitle,
			}

			s.probeMediaInfo(media)
			s.scanExternalSubtitles(media)

			if err := s.mediaRepo.Create(media); err != nil {
				s.logger.Warnf("保存剧集失败: %s, 错误: %v", ep.FilePath, err)
				continue
			}

			seasonSet[seasonNum] = true
			totalNewCount++

			s.logger.Debugf("发现剧集(多季): %s S%02dE%02d [%s | %s]",
				seriesTitle, seasonNum, ep.EpisodeNum, media.Resolution, media.VideoCodec)
			s.broadcastScanEvent(EventScanProgress, &ScanProgressData{
				LibraryID:   library.ID,
				LibraryName: library.Name,
				Phase:       "scanning",
				NewFound:    totalNewCount,
				Message:     fmt.Sprintf("发现: %s S%02dE%02d", seriesTitle, seasonNum, ep.EpisodeNum),
			})
		}
	}

	// 更新合集统计信息
	allEpisodes, _ := s.mediaRepo.ListBySeriesID(series.ID)
	series.EpisodeCount = len(allEpisodes)
	series.SeasonCount = len(seasonSet)
	s.seriesRepo.Update(series)

	if totalNewCount > 0 {
		s.logger.Infof("多季合集扫描完成: %s, 新增 %d 集, 共 %d 季 %d 集",
			seriesTitle, totalNewCount, series.SeasonCount, series.EpisodeCount)
	}

	return totalNewCount, nil
}

// scanSeriesFolder 扫描单个剧集文件夹
func (s *ScannerService) scanSeriesFolder(library *model.Library, folderPath, seriesTitle string) (int, error) {
	s.logger.Infof("扫描剧集: %s (%s)", seriesTitle, folderPath)

	// 查找或创建剧集合集条目
	series, err := s.seriesRepo.FindByFolderPath(folderPath)
	if err != nil {
		// 新剧集，创建合集条目
		series = &model.Series{
			LibraryID:  library.ID,
			Title:      seriesTitle,
			FolderPath: folderPath,
		}
		if err := s.seriesRepo.Create(series); err != nil {
			return 0, fmt.Errorf("创建剧集合集失败: %w", err)
		}
		s.logger.Infof("创建剧集合集: %s (ID=%s)", seriesTitle, series.ID)
	}

	// 识别本地 NFO 信息文件并解析剧集元数据
	if nfoPath := s.nfoService.FindNFOFile(folderPath); nfoPath != "" {
		if err := s.nfoService.ParseTVShowNFO(nfoPath, series); err != nil {
			s.logger.Debugf("解析剧集NFO失败: %s, 错误: %v", nfoPath, err)
		} else {
			s.logger.Debugf("从NFO读取剧集元数据: %s -> %s", nfoPath, series.Title)
			// 如果NFO中有标题，更新seriesTitle用于后续剧集
			if series.Title != "" {
				seriesTitle = series.Title
			}
		}
	}

	// 识别本地海报封面图片
	if poster, backdrop := s.nfoService.FindLocalImages(folderPath); poster != "" || backdrop != "" {
		if poster != "" && series.PosterPath == "" {
			series.PosterPath = poster
			s.logger.Debugf("发现剧集本地海报: %s", poster)
		}
		if backdrop != "" && series.BackdropPath == "" {
			series.BackdropPath = backdrop
			s.logger.Debugf("发现剧集本地背景图: %s", backdrop)
		}
	}

	// 保存NFO和图片更新
	s.seriesRepo.Update(series)

	// 收集所有剧集文件
	episodes := s.collectEpisodes(folderPath)

	if len(episodes) == 0 {
		s.logger.Debugf("剧集文件夹无视频文件: %s", folderPath)
		// 如果该合集下已经没有任何剧集，清理这个空合集
		existingEpisodes, _ := s.mediaRepo.ListBySeriesID(series.ID)
		if len(existingEpisodes) == 0 {
			s.seriesRepo.Delete(series.ID)
			s.logger.Infof("清理空合集: %s (ID=%s)", seriesTitle, series.ID)
		}
		return 0, nil
	}

	// 导入剧集
	var newCount int
	seasonSet := make(map[int]bool)

	for _, ep := range episodes {
		// 检查是否已存在，如果存在则修正可能的脏数据
		if existing, err := s.mediaRepo.FindByFilePath(ep.FilePath); err == nil {
			seasonSet[ep.SeasonNum] = true
			needUpdate := false
			if existing.EpisodeTitle != ep.EpisodeTitle {
				existing.EpisodeTitle = ep.EpisodeTitle
				needUpdate = true
			}
			if existing.SeasonNum != ep.SeasonNum {
				existing.SeasonNum = ep.SeasonNum
				needUpdate = true
			}
			if existing.EpisodeNum != ep.EpisodeNum {
				existing.EpisodeNum = ep.EpisodeNum
				needUpdate = true
			}
			if needUpdate {
				s.mediaRepo.Update(existing)
			}
			continue
		}

		media := &model.Media{
			LibraryID:    library.ID,
			SeriesID:     series.ID,
			Title:        seriesTitle,
			FilePath:     ep.FilePath,
			FileSize:     ep.FileInfo.Size(),
			MediaType:    "episode",
			SeasonNum:    ep.SeasonNum,
			EpisodeNum:   ep.EpisodeNum,
			EpisodeTitle: ep.EpisodeTitle,
		}

		s.probeMediaInfo(media)
		s.scanExternalSubtitles(media)

		if err := s.mediaRepo.Create(media); err != nil {
			s.logger.Warnf("保存剧集失败: %s, 错误: %v", ep.FilePath, err)
			continue
		}

		seasonSet[ep.SeasonNum] = true
		newCount++

		s.logger.Debugf("发现剧集: %s S%02dE%02d [%s | %s]", seriesTitle, ep.SeasonNum, ep.EpisodeNum, media.Resolution, media.VideoCodec)
		s.broadcastScanEvent(EventScanProgress, &ScanProgressData{
			LibraryID:   library.ID,
			LibraryName: library.Name,
			Phase:       "scanning",
			NewFound:    newCount,
			Message:     fmt.Sprintf("发现: %s S%02dE%02d", seriesTitle, ep.SeasonNum, ep.EpisodeNum),
		})
	}

	// 更新合集统计信息
	allEpisodes, _ := s.mediaRepo.ListBySeriesID(series.ID)
	series.EpisodeCount = len(allEpisodes)
	series.SeasonCount = len(seasonSet)
	s.seriesRepo.Update(series)

	s.logger.Infof("剧集扫描完成: %s, 新增 %d 集, 共 %d 季 %d 集",
		seriesTitle, newCount, series.SeasonCount, series.EpisodeCount)

	return newCount, nil
}

// collectEpisodes 递归收集剧集文件夹下的所有视频文件
func (s *ScannerService) collectEpisodes(folderPath string) []EpisodeInfo {
	var episodes []EpisodeInfo

	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			return nil
		}

		fileName := filepath.Base(path)
		ep := s.parseEpisodeInfo(fileName)

		// 尝试从Season目录名获取季号（如果文件名中没有季号）
		if ep.SeasonNum == 0 {
			parentDir := filepath.Base(filepath.Dir(path))
			if seasonNum := s.parseSeasonFromDir(parentDir); seasonNum > 0 {
				ep.SeasonNum = seasonNum
			}
		}

		// 默认季号为1
		if ep.SeasonNum == 0 {
			ep.SeasonNum = 1
		}

		ep.FilePath = path
		ep.FileInfo = info

		episodes = append(episodes, ep)
		return nil
	})

	// 按季号+集号排序
	sort.Slice(episodes, func(i, j int) bool {
		if episodes[i].SeasonNum != episodes[j].SeasonNum {
			return episodes[i].SeasonNum < episodes[j].SeasonNum
		}
		return episodes[i].EpisodeNum < episodes[j].EpisodeNum
	})

	// 如果所有集号都是0，按文件名排序后自动编号
	allZero := true
	for _, ep := range episodes {
		if ep.EpisodeNum > 0 {
			allZero = false
			break
		}
	}
	if allZero {
		sort.Slice(episodes, func(i, j int) bool {
			return episodes[i].FilePath < episodes[j].FilePath
		})
		for i := range episodes {
			episodes[i].EpisodeNum = i + 1
		}
	}

	return episodes
}

// parseEpisodeInfo 从文件名解析剧集信息
// 支持的命名格式：
//
//	标准格式: [字幕组][剧名][One-Punch Man][01][1280x720][简体]
//	季集格式: [HYSUB][ONE PUNCH MAN S2][OVA01][GB_MP4][1280X720].mp4
//	通用格式: S01E01, 1x01, 第1集, EP01, OVA01 等
func (s *ScannerService) parseEpisodeInfo(filename string) EpisodeInfo {
	var ep EpisodeInfo

	// 预处理：移除文件扩展名，方便后续解析
	nameWithoutExt := strings.TrimSuffix(filename, filepath.Ext(filename))

	// === 阶段零：多集连播检测（优先于单集匹配） ===

	// 多集模式0: S01E02-E03 / S01E02-E05
	if m := multiEpPatterns[0].FindStringSubmatch(filename); len(m) >= 4 {
		sNum, _ := strconv.Atoi(m[1])
		eStart, _ := strconv.Atoi(m[2])
		eEnd, _ := strconv.Atoi(m[3])
		if eEnd > eStart && sNum <= 30 {
			ep.SeasonNum = sNum
			ep.EpisodeNum = eStart
			ep.EpisodeNumEnd = eEnd
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
			return ep
		}
	}

	// 多集模式1: S01E02-03 (无前缀E的范围)
	if m := multiEpPatterns[1].FindStringSubmatch(filename); len(m) >= 4 {
		sNum, _ := strconv.Atoi(m[1])
		eStart, _ := strconv.Atoi(m[2])
		eEnd, _ := strconv.Atoi(m[3])
		if eEnd > eStart && sNum <= 30 && !resolutionNums[eEnd] {
			ep.SeasonNum = sNum
			ep.EpisodeNum = eStart
			ep.EpisodeNumEnd = eEnd
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
			return ep
		}
	}

	// === 阶段零-B：日期格式集号检测（日播剧/脱口秀） ===
	if m := dateEpisodePattern.FindStringSubmatch(filename); len(m) >= 4 {
		year, _ := strconv.Atoi(m[1])
		month, _ := strconv.Atoi(m[2])
		day, _ := strconv.Atoi(m[3])
		// 验证日期合理性
		if year >= 1990 && year <= 2099 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			// 不与 SxxExx 冲突：如果同时有 S01E01 格式，优先使用 SxxExx
			if !episodePatterns[0].MatchString(filename) && !episodePatterns[1].MatchString(filename) {
				ep.AirDate = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
				// 将日期编码为集号: MMDD (方便排序)
				ep.EpisodeNum = month*100 + day
				ep.SeasonNum = year - 2000 // 年份作为季号标识（如 2024 → 24）
				ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
				return ep
			}
		}
	}

	// === 阶段一：提取集号（原有逻辑） ===

	// 模式 0: S01E01 — 最精确的格式，同时包含季号和集号
	if m := episodePatterns[0].FindStringSubmatch(filename); len(m) >= 3 {
		sNum, _ := strconv.Atoi(m[1])
		eNum, _ := strconv.Atoi(m[2])
		// 排除明显不合理的值：集号恰好是分辨率
		if !resolutionNums[eNum] || sNum <= 30 {
			ep.SeasonNum = sNum
			ep.EpisodeNum = eNum
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
			return ep
		}
	}

	// 模式 1: S01.E01
	if m := episodePatterns[1].FindStringSubmatch(filename); len(m) >= 3 {
		sNum, _ := strconv.Atoi(m[1])
		eNum, _ := strconv.Atoi(m[2])
		if !resolutionNums[eNum] || sNum <= 30 {
			ep.SeasonNum = sNum
			ep.EpisodeNum = eNum
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
			return ep
		}
	}

	// 模式 2: 1x01 — 排除分辨率如 "1920x1080" "1280x720"
	if m := episodePatterns[2].FindStringSubmatch(filename); len(m) >= 3 {
		sNum, _ := strconv.Atoi(m[1])
		eNum, _ := strconv.Atoi(m[2])
		if !resolutionNums[eNum] && !resolutionNums[sNum] && sNum < 100 {
			ep.SeasonNum = sNum
			ep.EpisodeNum = eNum
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
			return ep
		}
	}

	// 模式 3: 第01集
	if m := episodePatterns[3].FindStringSubmatch(filename); len(m) >= 2 {
		ep.EpisodeNum, _ = strconv.Atoi(m[1])
		ep.SeasonNum = s.extractSeasonFromFilename(filename)
		ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
		return ep
	}

	// 模式 4: EP01 / Episode 01
	if m := episodePatterns[4].FindStringSubmatch(filename); len(m) >= 2 {
		ep.EpisodeNum, _ = strconv.Atoi(m[1])
		ep.SeasonNum = s.extractSeasonFromFilename(filename)
		ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
		return ep
	}

	// 模式 5: OVA01 / SP01 / SPECIAL01 等特殊剧集类型
	if m := episodePatterns[5].FindStringSubmatch(filename); len(m) >= 2 {
		ep.EpisodeNum, _ = strconv.Atoi(m[1])
		ep.SeasonNum = s.extractSeasonFromFilename(filename)
		ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
		return ep
	}

	// 模式 6: E01（单独的E+数字）— 需排除分辨率上下文
	if m := episodePatterns[6].FindStringSubmatchIndex(filename); m != nil {
		full := filename[m[0]:m[1]]
		sub := filename[m[2]:m[3]]
		eNum, _ := strconv.Atoi(sub)
		if !resolutionNums[eNum] && !isResolutionContext(filename, m[1]) {
			ep.EpisodeNum = eNum
			ep.SeasonNum = s.extractSeasonFromFilename(filename)
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, full)
			return ep
		}
	}

	// 模式 7: [01] / [001] — 方括号内的纯数字（字幕组常用格式）
	if m := episodePatterns[7].FindStringSubmatch(filename); len(m) >= 2 {
		num, _ := strconv.Atoi(m[1])
		// 排除年份和分辨率数字
		if num > 0 && num < 1900 && !resolutionNums[num] {
			ep.EpisodeNum = num
			ep.SeasonNum = s.extractSeasonFromFilename(filename)
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, m[0])
			return ep
		}
	}

	// 模式 8: - 01 - / .01. — 最宽松的匹配，需要严格过滤
	if m := episodePatterns[8].FindStringSubmatchIndex(filename); m != nil {
		sub := filename[m[2]:m[3]]
		num, _ := strconv.Atoi(sub)
		if num > 0 && num < 1900 && !resolutionNums[num] && !isResolutionContext(filename, m[1]) {
			ep.EpisodeNum = num
			ep.SeasonNum = s.extractSeasonFromFilename(filename)
			ep.EpisodeTitle = s.extractEpisodeTitle(nameWithoutExt, filename[m[0]:m[1]])
			return ep
		}
	}

	return ep
}

// extractSeasonFromFilename 从文件名中独立提取季号
// 处理文件名中包含 S2、Season 2、第2季 等情况（不依赖集号格式）
func (s *ScannerService) extractSeasonFromFilename(filename string) int {
	for _, pattern := range seasonInFilenamePatterns {
		if m := pattern.FindStringSubmatch(filename); len(m) >= 2 {
			num, _ := strconv.Atoi(m[1])
			if num > 0 && num <= 30 {
				return num
			}
		}
	}
	return 0
}

// extractEpisodeTitle 从文件名中提取集标题（集号模式之后的部分）
func (s *ScannerService) extractEpisodeTitle(nameWithoutExt string, matchedPattern string) string {
	idx := strings.Index(nameWithoutExt, matchedPattern)
	if idx < 0 {
		return ""
	}
	after := nameWithoutExt[idx+len(matchedPattern):]
	// 清理开头的分隔符和空格
	after = strings.TrimLeft(after, " .-_")
	if after == "" {
		return ""
	}
	// 去除尾部常见的元信息标记（分辨率/编码/组名等括号内容）
	// 例如 "[1080p]" "(BDRip)" "[FLAC]" 等
	metaPattern := regexp.MustCompile(`[\[\(].*[\]\)]`)
	after = metaPattern.ReplaceAllString(after, "")
	after = strings.TrimRight(after, " .-_")
	// 如果剩余内容太短或全是数字，则不作为标题
	if len(after) <= 1 {
		return ""
	}
	// 排除纯数字（可能是分辨率等残留）
	if _, err := strconv.Atoi(after); err == nil {
		return ""
	}
	// 排除分辨率字符串（如 720p、1080p、4K 等）
	resPattern := regexp.MustCompile(`(?i)^\d{3,4}[pi]$|^[248]K$`)
	if resPattern.MatchString(after) {
		return ""
	}
	// 排除纯技术性标记（编码/混流/来源等），这些不是有意义的剧集标题
	// 例如：remux, remux nvl, x264, HEVC, BDRip, WEB-DL 等
	techPattern := regexp.MustCompile(`(?i)^[\s\-\.]*(?:remux|re-?mux|nvl|x26[45]|h\.?26[45]|hevc|avc|aac|flac|dts|bdri?p|dvdri?p|web-?dl|web-?rip|blu-?ray|hdr|10bit|ma[25]\.?[01]|truehd|atmos|opus)(?:[\s\-\.]+(?:remux|nvl|x26[45]|h\.?26[45]|hevc|avc|aac|flac|dts|bdri?p|dvdri?p|web-?dl|web-?rip|blu-?ray|hdr|10bit|ma[25]\.?[01]|truehd|atmos|opus))*[\s\-\.]*$`)
	if techPattern.MatchString(after) {
		return ""
	}
	return after
}

// parseSeasonFromDir 从Season目录名解析季号
func (s *ScannerService) parseSeasonFromDir(dirName string) int {
	for _, pattern := range seasonDirPatterns {
		if m := pattern.FindStringSubmatch(dirName); len(m) >= 2 {
			num, _ := strconv.Atoi(m[1])
			return num
		}
		// Specials特别篇 -> 季号 0
		if pattern.MatchString(dirName) && strings.Contains(strings.ToLower(dirName), "special") {
			return 0
		}
	}
	return 0
}

// extractSeriesNameFromFile 从视频文件名中提取系列名称
// 适用于根目录下散落的剧集文件，如 [HYSUB][ONE PUNCH MAN][01].mkv -> ONE PUNCH MAN
func (s *ScannerService) extractSeriesNameFromFile(filename string) string {
	// 去掉扩展名
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 模式1: [字幕组][系列名][集号] 格式
	// 匹配方括号中的内容，提取第二个方括号的内容作为系列名
	bracketPattern := regexp.MustCompile(`\[([^\[\]]+)\]`)
	matches := bracketPattern.FindAllStringSubmatch(name, -1)
	if len(matches) >= 2 {
		// 遍历方括号内容，找到最可能是系列名的部分
		// 跳过: 纯数字（集号）、分辨率（720P/1080P）、编码格式等
		skipPatterns := []*regexp.Regexp{
			regexp.MustCompile(`(?i)^\d+$`),                                                          // 纯数字
			regexp.MustCompile(`(?i)^\d{3,4}[PpKk]$`),                                                // 分辨率如720P
			regexp.MustCompile(`(?i)^\d+[Xx]\d+$`),                                                   // 分辨率如1280X720
			regexp.MustCompile(`(?i)^(BIG5|GB|UTF-?8|MP4|MKV|AVI|HEVC|H\.?26[45]|AAC|FLAC|x26[45])`), // 编码/格式
			regexp.MustCompile(`(?i)^(BIG5_MP4|GB_MP4|CHS|CHT|JPN|ENG)`),                             // 字幕/编码组合
			regexp.MustCompile(`(?i)^S\d+E\d+$`),                                                     // 剧集号 S01E01
			regexp.MustCompile(`(?i)^EP?\s*\d+$`),                                                    // EP01
			regexp.MustCompile(`(?i)^V\d+$`),                                                         // 版本号 V2
			regexp.MustCompile(`(?i)^(WebRip|BDRip|DVDRip|WEB-DL|BluRay|HDTV)$`),                     // 来源
		}

		// 通常第一个方括号是字幕组，第二个是系列名
		// 但也可能系列名在其他位置，需要智能判断
		candidates := []string{}
		for _, m := range matches {
			content := strings.TrimSpace(m[1])
			if content == "" {
				continue
			}
			skip := false
			for _, sp := range skipPatterns {
				if sp.MatchString(content) {
					skip = true
					break
				}
			}
			if !skip {
				candidates = append(candidates, content)
			}
		}

		// 如果有多个候选项，选择第二个（通常第一个是字幕组名）
		if len(candidates) >= 2 {
			return candidates[1]
		}
		if len(candidates) == 1 {
			return candidates[0]
		}
	}

	// 模式2: 尝试从文件名中移除集号信息后得到系列名
	// 先去掉所有方括号内容和常见标记
	cleanName := name
	cleanName = bracketPattern.ReplaceAllString(cleanName, " ")

	// 移除集号模式 S01E01, EP01, E01, 第N集
	epPatterns := []string{
		`(?i)S\d{1,2}\s*E\d{1,4}`,
		`(?i)S\d{1,2}\.\s*E\d{1,4}`,
		`(?i)\d{1,2}x\d{1,4}`,
		`第\s*\d{1,4}\s*集`,
		`(?i)(?:EP|Episode)\s*\.?\s*\d{1,4}`,
		`(?i)\bE\d{1,4}\b`,
	}
	for _, p := range epPatterns {
		re := regexp.MustCompile(p)
		cleanName = re.ReplaceAllString(cleanName, " ")
	}

	// 移除分辨率、编码等常见标记
	cleanPatterns := []string{
		`(?i)\b(BluRay|BDRip|HDRip|WEB-?DL|WEBRip|HDTV|COMPLETE)\b`,
		`(?i)\b(1080p|720p|2160p|4K)\b`,
		`(?i)\b(x264|x265|HEVC|AAC|FLAC)\b`,
	}
	for _, p := range cleanPatterns {
		re := regexp.MustCompile(p)
		cleanName = re.ReplaceAllString(cleanName, " ")
	}

	// 清理分隔符和多余空格
	cleanName = strings.ReplaceAll(cleanName, ".", " ")
	cleanName = strings.ReplaceAll(cleanName, "_", " ")
	cleanName = strings.ReplaceAll(cleanName, "-", " ")
	cleanName = regexp.MustCompile(`\s+`).ReplaceAllString(cleanName, " ")
	cleanName = strings.TrimSpace(cleanName)

	// 移除末尾的纯数字（可能是集号）
	cleanName = regexp.MustCompile(`\s+\d{1,4}\s*$`).ReplaceAllString(cleanName, "")
	cleanName = strings.TrimSpace(cleanName)

	if len(cleanName) > 0 {
		return cleanName
	}

	return ""
}

// extractSeriesTitle 从文件夹名提取剧集标题
func (s *ScannerService) extractSeriesTitle(folderName string) string {
	title := folderName

	// 移除年份信息，如 "Breaking Bad (2008)"
	yearRegex := regexp.MustCompile(`\s*[\(\[]\.?(\d{4})[\)\]]\.?\s*$`)
	title = yearRegex.ReplaceAllString(title, "")

	// 清理常见标记
	cleanPatterns := []string{
		`(?i)\b(BluRay|BDRip|HDRip|WEB-?DL|WEBRip|HDTV|COMPLETE)\b`,
		`(?i)\b(1080p|720p|2160p|4K)\b`,
		`(?i)\b(x264|x265|HEVC)\b`,
	}
	for _, p := range cleanPatterns {
		re := regexp.MustCompile(p)
		title = re.ReplaceAllString(title, "")
	}

	// 替换常见分隔符
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")

	// 清理多余空格
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	return strings.TrimSpace(title)
}

// broadcastScanEvent 广播扫描事件
func (s *ScannerService) broadcastScanEvent(eventType string, data *ScanProgressData) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
}

// ProbeMediaInfo 公开的 FFprobe 媒体信息探测方法（供外部服务调用）
func (s *ScannerService) ProbeMediaInfo(media *model.Media) {
	s.probeMediaInfo(media)
}

// parseSTRMFile 解析 .strm 文件，提取远程流 URL
// .strm 文件格式：纯文本文件，第一行为可播放的远程 URL
func (s *ScannerService) parseSTRMFile(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("读取 .strm 文件失败: %w", err)
	}

	// 逐行读取，取第一个非空、非注释行作为 URL
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 验证是否为有效的 URL
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line, nil
		}
	}

	return "", fmt.Errorf(".strm 文件中未找到有效的 URL: %s", filePath)
}

// isSTRMFile 判断是否为 .strm 文件
func isSTRMFile(filePath string) bool {
	return strings.ToLower(filepath.Ext(filePath)) == ".strm"
}

// probeSTRMMedia 处理 .strm 文件的媒体信息
// 对于 .strm 文件，不使用 FFprobe 探测（远程 URL 可能很慢或不支持），
// 而是设置默认值，后续播放时由前端/后端动态处理
func (s *ScannerService) probeSTRMMedia(media *model.Media, streamURL string) {
	media.StreamURL = streamURL
	// 根据远程 URL 的扩展名推断基本信息
	urlLower := strings.ToLower(streamURL)
	if strings.Contains(urlLower, ".m3u8") {
		media.VideoCodec = "strm_hls"
	} else if strings.HasSuffix(urlLower, ".mp4") || strings.Contains(urlLower, ".mp4?") {
		media.VideoCodec = "strm_mp4"
	} else if strings.HasSuffix(urlLower, ".mkv") || strings.Contains(urlLower, ".mkv?") {
		media.VideoCodec = "strm_mkv"
	} else {
		media.VideoCodec = "strm_unknown"
	}
	s.logger.Debugf("STRM 文件: %s -> %s", media.FilePath, streamURL)
}

// probeMediaInfo 使用FFprobe提取视频元数据（.strm 文件走特殊逻辑）
func (s *ScannerService) probeMediaInfo(media *model.Media) {
	// .strm 文件：解析远程 URL，不使用 FFprobe
	if isSTRMFile(media.FilePath) {
		streamURL, err := s.parseSTRMFile(media.FilePath)
		if err != nil {
			s.logger.Warnf("解析 STRM 文件失败: %s, 错误: %v", media.FilePath, err)
			return
		}
		s.probeSTRMMedia(media, streamURL)
		return
	}

	cmd := exec.Command(s.cfg.App.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		media.FilePath,
	)

	output, err := cmd.Output()
	if err != nil {
		s.logger.Warnf("FFprobe分析失败: %s, 错误: %v", media.FilePath, err)
		return
	}

	var result FFprobeResult
	if err := json.Unmarshal(output, &result); err != nil {
		s.logger.Warnf("解析FFprobe输出失败: %s, 错误: %v", media.FilePath, err)
		return
	}

	// 提取视频流信息
	for _, stream := range result.Streams {
		switch stream.CodecType {
		case "video":
			media.VideoCodec = stream.CodecName
			if stream.Width > 0 && stream.Height > 0 {
				media.Resolution = s.classifyResolution(stream.Width, stream.Height)
			}
		case "audio":
			if media.AudioCodec == "" {
				media.AudioCodec = stream.CodecName
			}
		}
	}

	// 提取时长
	if result.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			media.Duration = dur
		}
	}
}

// GetSubtitleTracks 获取媒体文件的内嵌字幕轨道列表
func (s *ScannerService) GetSubtitleTracks(filePath string) ([]SubtitleTrack, error) {
	cmd := exec.Command(s.cfg.App.FFprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s", filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("FFprobe获取字幕失败: %w", err)
	}

	var result struct {
		Streams []FFprobeStream `json:"streams"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析字幕信息失败: %w", err)
	}

	var tracks []SubtitleTrack
	for _, stream := range result.Streams {
		track := SubtitleTrack{
			Index:   stream.Index,
			Codec:   stream.CodecName,
			Default: stream.Disposition.Default == 1,
			Forced:  stream.Disposition.Forced == 1,
			Bitmap:  isBitmapSubtitle(stream.CodecName),
		}
		if lang, ok := stream.Tags["language"]; ok {
			track.Language = lang
		}
		if title, ok := stream.Tags["title"]; ok {
			track.Title = title
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// ExtractSubtitle 提取内嵌字幕到文件
func (s *ScannerService) ExtractSubtitle(filePath string, streamIndex int, outputFormat string) (string, error) {
	// 确定输出文件路径
	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "subtitles")
	os.MkdirAll(cacheDir, 0755)

	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("%s_%d.%s", baseName, streamIndex, outputFormat))

	// 检查缓存
	if _, err := os.Stat(outputPath); err == nil {
		return outputPath, nil
	}

	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", s.getSubtitleCodec(outputFormat),
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("提取字幕失败: %w", err)
	}

	return outputPath, nil
}

// scanExternalSubtitles 扫描外挂字幕文件
func (s *ScannerService) scanExternalSubtitles(media *model.Media) {
	dir := filepath.Dir(media.FilePath)
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))

	subtitleExts := []string{".srt", ".ass", ".ssa", ".vtt", ".sub", ".idx"}

	var found []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		// 检查是否为字幕文件且与视频同名前缀
		isSubtitle := false
		for _, subExt := range subtitleExts {
			if ext == subExt {
				isSubtitle = true
				break
			}
		}
		if !isSubtitle {
			continue
		}

		// 检查文件名前缀匹配
		nameWithoutExt := strings.TrimSuffix(name, ext)
		if strings.HasPrefix(strings.ToLower(nameWithoutExt), strings.ToLower(baseName)) {
			found = append(found, filepath.Join(dir, name))
		}
	}

	if len(found) > 0 {
		media.SubtitlePaths = strings.Join(found, "|")
		s.logger.Debugf("发现外挂字幕: %s -> %d 个", baseName, len(found))
	}
}

// getSubtitleCodec 根据输出格式获取字幕编解码器
func (s *ScannerService) getSubtitleCodec(format string) string {
	switch format {
	case "srt":
		return "srt"
	case "ass", "ssa":
		return "ass"
	case "vtt", "webvtt":
		return "webvtt"
	default:
		return "srt"
	}
}

// classifyResolution 根据分辨率分类
func (s *ScannerService) classifyResolution(width, height int) string {
	// 以高度为主要判断标准
	maxDim := height
	if width > height {
		// 正常横向视频
		maxDim = height
	} else {
		// 竖向视频
		maxDim = width
	}

	switch {
	case maxDim >= 2160:
		return "4K"
	case maxDim >= 1440:
		return "2K"
	case maxDim >= 1080:
		return "1080p"
	case maxDim >= 720:
		return "720p"
	case maxDim >= 480:
		return "480p"
	default:
		return fmt.Sprintf("%dp", maxDim)
	}
}

// extractTitle 从文件名提取标题（保持向后兼容的简单版本）
func (s *ScannerService) extractTitle(filename string) string {
	title, _, _ := s.extractTitleEnhanced(filename)
	return title
}

// extractTitleEnhanced 从文件名增强提取标题、年份和 TMDb ID
// 支持 Emby 标准命名格式：Title (Year) [tmdbid=xxx]
func (s *ScannerService) extractTitleEnhanced(filename string) (title string, year int, tmdbID int) {
	// 去掉扩展名
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 步骤1：提取 ID 标签 [tmdbid=xxx]、{imdb-ttxxx} 等
	idType, idValue := parseIDFromName(name)
	if idType == "tmdbid" || idType == "tmdb" {
		tmdbID, _ = strconv.Atoi(idValue)
	}
	// 注意：IMDB ID (imdbid/imdb) 标签在此处仅被识别和移除，
	// 实际的 IMDB ID → TMDb ID 转换在刮削阶段（ScrapeMedia）中通过网络请求完成
	// 从名称中移除 ID 标签
	for _, pattern := range idTagPatterns {
		name = pattern.ReplaceAllString(name, "")
	}

	// 步骤2：提取年份 (2009) 或 [2009]
	year = extractYearFromName(name)
	// 移除年份标记
	name = yearInNamePattern.ReplaceAllString(name, "")

	// 步骤3：清理常见编码/来源/分辨率标记
	cleanPatterns := []string{
		`(?i)\b(BluRay|BDRip|HDRip|WEB-?DL|WEBRip|DVDRip|HDTV|HDCam|REMUX)\b`,
		`(?i)\b(x264|x265|h\.?264|h\.?265|HEVC|AVC|AAC|DTS|AC3|FLAC|OPUS)\b`,
		`(?i)\b(1080p|720p|480p|2160p|4K|UHD)\b`,
		`(?i)\b(PROPER|REPACK|EXTENDED|UNRATED|DIRECTORS\.?CUT|REMASTERED)\b`,
	}
	for _, p := range cleanPatterns {
		re := regexp.MustCompile(p)
		name = re.ReplaceAllString(name, " ")
	}

	// 步骤4：替换常见分隔符为空格
	replacer := strings.NewReplacer(
		".", " ",
		"_", " ",
	)
	name = replacer.Replace(name)

	// 步骤5：清理多余空格和首尾的分隔符
	name = regexp.MustCompile(`\s+`).ReplaceAllString(name, " ")
	name = strings.Trim(name, " -")

	title = strings.TrimSpace(name)
	return
}

// GetExternalSubtitles 获取媒体文件的外挂字幕列表
func (s *ScannerService) GetExternalSubtitles(filePath string) []ExternalSubtitle {
	dir := filepath.Dir(filePath)
	baseName := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	subtitleExts := []string{".srt", ".ass", ".ssa", ".vtt", ".sub"}

	var subs []ExternalSubtitle
	entries, err := os.ReadDir(dir)
	if err != nil {
		return subs
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))

		isSubtitle := false
		for _, subExt := range subtitleExts {
			if ext == subExt {
				isSubtitle = true
				break
			}
		}
		if !isSubtitle {
			continue
		}

		nameWithoutExt := strings.TrimSuffix(name, ext)
		if strings.HasPrefix(strings.ToLower(nameWithoutExt), strings.ToLower(baseName)) {
			// 尝试从文件名提取语言信息，如 movie.zh.srt, movie.eng.srt
			langs := strings.TrimPrefix(strings.ToLower(nameWithoutExt), strings.ToLower(baseName))
			langs = strings.Trim(langs, "._ ")
			lang := s.detectSubtitleLanguage(langs)

			subs = append(subs, ExternalSubtitle{
				Path:     filepath.Join(dir, name),
				Filename: name,
				Format:   strings.TrimPrefix(ext, "."),
				Language: lang,
			})
		}
	}

	return subs
}

// ExternalSubtitle 外挂字幕信息
type ExternalSubtitle struct {
	Path     string `json:"path"`
	Filename string `json:"filename"`
	Format   string `json:"format"`   // srt, ass, vtt等
	Language string `json:"language"` // 语言编码
}

// detectSubtitleLanguage 从文件名中检测字幕语言
func (s *ScannerService) detectSubtitleLanguage(namePart string) string {
	// 按优先级排序的语言映射（长匹配优先，避免短码误匹配）
	type langEntry struct {
		code string
		lang string
	}
	langEntries := []langEntry{
		// 长匹配优先
		{"chinese", "中文"},
		{"english", "English"},
		{"japanese", "日本語"},
		{"korean", "한국어"},
		{"简体", "简体中文"},
		{"繁体", "繁体中文"},
		{"简中", "简体中文"},
		{"繁中", "繁体中文"},
		// 三字母ISO 639-2
		{"chi", "中文"},
		{"chs", "简体中文"},
		{"cht", "繁体中文"},
		{"eng", "English"},
		{"jpn", "日本語"},
		{"kor", "한국어"},
		// 两字母ISO 639-1（使用分隔符精确匹配）
		{"zh", "中文"},
		{"en", "English"},
		{"ja", "日本語"},
		{"jp", "日本語"},
		{"ko", "한국어"},
		{"sc", "简体中文"},
		{"tc", "繁体中文"},
	}

	namePart = strings.ToLower(namePart)
	// 将分隔符统一为点号，方便精确匹配
	normalized := strings.NewReplacer("_", ".", "-", ".", " ", ".").Replace(namePart)
	parts := strings.Split(normalized, ".")

	// 先尝试精确匹配各段
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		for _, entry := range langEntries {
			if part == entry.code {
				return entry.lang
			}
		}
	}

	// 再尝试包含匹配（仅对长码，避免短码误匹配）
	for _, entry := range langEntries {
		if len(entry.code) >= 3 && strings.Contains(namePart, entry.code) {
			return entry.lang
		}
	}

	if namePart != "" {
		return namePart
	}
	return "未知"
}

// ConvertSubtitleToVTT 将外挂字幕文件转换为WebVTT格式（浏览器原生支持）
func (s *ScannerService) ConvertSubtitleToVTT(subtitlePath string) (string, error) {
	// 确定输出文件路径
	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "subtitles")
	os.MkdirAll(cacheDir, 0755)

	// 使用原始文件名+哈希避免冲突
	baseName := strings.TrimSuffix(filepath.Base(subtitlePath), filepath.Ext(subtitlePath))
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("%s_ext.vtt", baseName))

	// 检查缓存：如果转换后的文件已存在且比源文件新，直接返回
	if outInfo, err := os.Stat(outputPath); err == nil {
		if srcInfo, err := os.Stat(subtitlePath); err == nil {
			if outInfo.ModTime().After(srcInfo.ModTime()) {
				return outputPath, nil
			}
		}
	}

	// 使用FFmpeg将字幕转换为WebVTT
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-i", subtitlePath,
		"-c:s", "webvtt",
		outputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("FFmpeg字幕转换失败: %w, 输出: %s", err, string(output))
	}

	s.logger.Debugf("字幕转换完成: %s -> %s", subtitlePath, outputPath)
	return outputPath, nil
}

// GetFileExt 获取文件扩展名（小写）
func GetFileExt(path string) string {
	return strings.ToLower(filepath.Ext(path))
}
