package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	"unicode/utf8"

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

// ==================== xiaoya / 小雅多级分类目录适配 ====================
//
// 适配以下典型目录结构（媒体库根直接选到 /media/xiaoya 即可，无需用户配置）：
//   xiaoya/
//     ├── 115/
//     │   ├── 电视剧/【我推的孩子】(2024)/Season 1/*.mkv
//     │   ├── 电影/...
//     │   └── 动漫/...
//     ├── 电视剧/...
//     ├── 电影/...
//     └── ISO/                 ← 直接跳过
//
// 策略：
//   1. 扫描入口会先调用 expandCategoryRoots 把"分类目录"穿透展开成真实媒体根列表；
//   2. extrasExcludeDirs + xiaoyaSkipDirs 在 Walk 过程中直接 SkipDir；
//   3. 标题提取同时兼容中文/全角括号与【】《》等装饰符号。

// xiaoyaCategoryDirs 已知的"分类目录"名（需要穿透递归，向下一层找真正的剧集/电影目录）
// key 使用原样（含中文）的目录名；比较时会忽略大小写
var xiaoyaCategoryDirs = map[string]bool{
	"电视剧": true, "电影": true, "动漫": true, "短剧": true,
	"纪录片": true, "纪录片（已刮削）": true, "纪录片(已刮削)": true,
	"综艺": true, "演唱会": true, "音乐": true, "每日更新": true,
	// xiaoya 常见的"来源分组"目录
	"115": true, "115盘": true, "阿里云盘": true, "夸克": true, "夸克网盘": true,
	"每日更新夸克": true, "xiaoya": true, "小雅": true,
}

// xiaoyaSkipDirs 完全跳过（不扫描内部）的特殊目录
// 比较时会忽略大小写
var xiaoyaSkipDirs = map[string]bool{
	"iso":    true,
	"json":   true,
	"画质演示":   true,
	"画质演示测试": true,
	"画质演示测试（4k，8k，hdr，dolby）": true,
	"bdmv":        true, // 完整蓝光结构，当前不支持解析
	"certificate": true,
	"backup":      true,
}

// isCategoryDirName 按名字判断是否为已知的"分类目录"
func isCategoryDirName(name string) bool {
	return xiaoyaCategoryDirs[strings.ToLower(strings.TrimSpace(name))] ||
		xiaoyaCategoryDirs[strings.TrimSpace(name)]
}

// isXiaoyaSkipDir 按名字判断是否为需要完全跳过的特殊目录
func isXiaoyaSkipDir(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if xiaoyaSkipDirs[lower] {
		return true
	}
	// 简单兜底：目录名以 "画质演示" 开头的一律跳过
	if strings.HasPrefix(strings.TrimSpace(name), "画质演示") {
		return true
	}
	return false
}

// yearInNameAnyBracketPattern 兼容中文/全角括号的年份正则
//
//	支持: (2024) [2024] （2024） 【2024】
var yearInNameAnyBracketPattern = regexp.MustCompile(`[\(\[（【]\s*((?:19|20)\d{2})\s*[\)\]）】]`)

// normalizeXiaoyaTitle 清洗 xiaoya/小雅风格的标题，返回去除装饰符号后的干净标题
// 例如：
//
//	"【我推的孩子】"     → "我推的孩子"
//	"#居酒屋新干线"     → "居酒屋新干线"
//	"《三体》"           → "三体"
//	"3 Body Problem"  → "3 Body Problem"（保持不变）
func normalizeXiaoyaTitle(raw string) string {
	if raw == "" {
		return raw
	}
	s := raw

	// 1. 移除首尾常见装饰前缀/后缀字符（# ＃ ★ ☆ ♥ ♡ 以及未配对的半边括号）
	s = regexp.MustCompile(`^[#＃★☆♥♡・·\s]+`).ReplaceAllString(s, "")

	// 2. 成对装饰括号内容保留：【xxx】→ xxx，《xxx》→ xxx，「xxx」→ xxx，『xxx』→ xxx
	pairPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[【](.*?)[】]`),
		regexp.MustCompile(`[《](.*?)[》]`),
		regexp.MustCompile(`[「](.*?)[」]`),
		regexp.MustCompile(`[『](.*?)[』]`),
		regexp.MustCompile(`[〈](.*?)[〉]`),
	}
	for _, p := range pairPatterns {
		// 反复替换直到稳定（处理嵌套情况）
		for {
			next := p.ReplaceAllString(s, "$1")
			if next == s {
				break
			}
			s = next
		}
	}

	// 3. 全角空格 → 半角空格，全角破折号 → 半角
	s = strings.ReplaceAll(s, "　", " ")

	// 4. 清理多余空格
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
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
// 优先匹配标准 ASCII 括号 (2024)/[2024]；失败时再尝试中文/全角括号 （2024）/【2024】（xiaoya 常见）
func extractYearFromName(name string) int {
	if m := yearInNamePattern.FindStringSubmatch(name); len(m) >= 2 {
		year, _ := strconv.Atoi(m[1])
		if year >= 1900 && year <= 2099 {
			return year
		}
	}
	if m := yearInNameAnyBracketPattern.FindStringSubmatch(name); len(m) >= 2 {
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
	vfsMgr         *VFSManager            // V2.1: VFS 管理器（支持 webdav:// 路径）
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

// SetVFSManager 设置 VFS 管理器（V2.1: 用于 webdav:// 路径支持）
func (s *ScannerService) SetVFSManager(vfsMgr *VFSManager) {
	s.vfsMgr = vfsMgr
}

// walkLibraryPath 根据媒体库路径前缀自动选择 VFS 遍历
// 返回的 path 是完整路径（LocalFS 返回原生路径；WebDAVFS 返回 webdav:// 前缀路径）
func (s *ScannerService) walkLibraryPath(root string, fn filepath.WalkFunc) error {
	if s.vfsMgr != nil && IsWebDAVPath(root) {
		return s.vfsMgr.Walk(root, fn)
	}
	return filepath.Walk(root, fn)
}

// statLibraryPath 根据路径前缀选择合适的 Stat 实现
func (s *ScannerService) statLibraryPath(p string) (os.FileInfo, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		return s.vfsMgr.Stat(p)
	}
	return os.Stat(p)
}

// readDirLibraryPath 根据路径前缀选择合适的 ReadDir 实现
func (s *ScannerService) readDirLibraryPath(p string) ([]os.DirEntry, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		entries, err := s.vfsMgr.ReadDir(p)
		if err != nil {
			return nil, err
		}
		// fs.DirEntry 与 os.DirEntry 在 Go 1.16+ 其实是同一接口别名
		result := make([]os.DirEntry, len(entries))
		copy(result, entries)
		return result, nil
	}
	return os.ReadDir(p)
}

// vfsJoin 拼接路径：对 webdav:// 前缀的路径使用正斜杠，避免 filepath.Join 在 Windows 下把前缀破坏为 webdav:\
func vfsJoin(base, name string) string {
	if IsWebDAVPath(base) {
		base = strings.TrimRight(base, "/")
		return base + "/" + strings.TrimLeft(name, "/")
	}
	return filepath.Join(base, name)
}

// collectMediaRoots 将媒体库根路径展开为一个或多个"真实媒体根目录"列表
//
// 适配 xiaoya/小雅 这种多级分类结构（如 xiaoya/115/电视剧/xxx）：
//   - 若 root 本身或其子目录是已知分类目录（xiaoyaCategoryDirs）且不包含直接视频文件，
//     则递归穿透；
//   - 若目录名命中 xiaoyaSkipDirs 或 extrasExcludeDirs，则直接跳过；
//   - 最多穿透 maxDepth 层，防止无限递归；
//   - 如果没有任何分类目录命中，直接返回 [root]（完全向后兼容）。
//
// kind 仅影响日志打印，不影响展开规则。
func (s *ScannerService) collectMediaRoots(root string, kind string) []string {
	const maxDepth = 4
	var results []string
	seen := make(map[string]bool)

	var walk func(path string, depth int)
	walk = func(path string, depth int) {
		if seen[path] {
			return
		}
		seen[path] = true

		base := filepath.Base(path)
		if isXiaoyaSkipDir(base) || extrasExcludeDirs[strings.ToLower(base)] {
			s.logger.Debugf("[xiaoya] 跳过特殊目录: %s", path)
			return
		}

		// depth==0 即 root 本身，不论是否命中分类名都要尝试穿透；
		// depth>0 时只有命中分类白名单 或 目录内没视频才穿透
		entries, err := s.readDirLibraryPath(path)
		if err != nil {
			// 无法读取的目录保守当做普通媒体根加入
			results = append(results, path)
			return
		}

		var hasVideoFile bool
		var subDirs []os.DirEntry
		for _, e := range entries {
			if e.IsDir() {
				subDirs = append(subDirs, e)
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if supportedExts[ext] {
				hasVideoFile = true
			}
			// 已有 NFO 也视为"当前目录已是媒体根"
			lower := strings.ToLower(e.Name())
			if lower == "tvshow.nfo" || lower == "movie.nfo" {
				hasVideoFile = true
			}
		}

		// 已是真实媒体目录 —— 不再下钻
		if hasVideoFile {
			results = append(results, path)
			return
		}

		// 分类目录识别：
		// - depth == 0：总是尝试穿透（用户把库根设在 xiaoya 上层也能工作）
		// - depth >  0：仅当目录名命中分类白名单 或 子目录数 >= 3（无视频+多子目录特征）
		shouldExpand := depth == 0
		if !shouldExpand {
			if isCategoryDirName(base) {
				shouldExpand = true
			} else if len(subDirs) >= 3 && !hasVideoFile {
				// 兜底启发式：无视频 + 多子目录，很可能也是分类目录
				shouldExpand = true
			}
		}

		if !shouldExpand || depth >= maxDepth {
			results = append(results, path)
			return
		}

		// 穿透：把每个子目录递归展开
		expandedCount := 0
		for _, sd := range subDirs {
			if isXiaoyaSkipDir(sd.Name()) || extrasExcludeDirs[strings.ToLower(sd.Name())] {
				continue
			}
			childPath := vfsJoin(path, sd.Name())
			before := len(results)
			walk(childPath, depth+1)
			if len(results) > before {
				expandedCount++
			}
		}

		// 如果穿透后一个结果都没有（极端情况），保底把当前目录加入
		if expandedCount == 0 {
			results = append(results, path)
		}
	}

	walk(root, 0)

	// 去重并保持顺序
	uniq := make([]string, 0, len(results))
	dedup := make(map[string]bool)
	for _, r := range results {
		if dedup[r] {
			continue
		}
		dedup[r] = true
		uniq = append(uniq, r)
	}

	if len(uniq) > 1 {
		s.logger.Infof("[xiaoya] %s 库多级分类展开: %s → 共 %d 个媒体根目录", kind, root, len(uniq))
	}
	return uniq
}

// SetOnScanComplete 设置扫描完成回调（用于触发视频预处理）
func (s *ScannerService) SetOnScanComplete(fn func(libraryID string)) {
	s.onScanComplete = fn
}

// ScanLibrary 扫描媒体库目录
func (s *ScannerService) ScanLibrary(library *model.Library) (int, error) {
	s.logger.Infof("开始扫描媒体库: %s (路径数: %d)", library.Name, len(library.AllPaths()))

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
	if s.onScanComplete != nil {
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

	allPaths := library.AllPaths()
	s.logger.Infof("电影库扫描开始: %s, 路径: %v, 上次扫描: %v", library.Name, allPaths, lastScanTime)

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
	// 【火力全开 A】收集"已存在但需要更新"的文件，后续统一走 parallelProbe 并行探测
	// 避免 walkFn 内同步调用 FFprobe 拖慢整个遍历过程。
	var updateList []pendingMedia
	// 【火力全开 B】细粒度锁：walkFn 原先整体加锁串行，现在仅对共享容器写入加锁，
	// 把磁盘 IO（readdir）和 CPU 操作（标题解析/正则匹配）放在锁外并行。
	var collectMu sync.Mutex
	// existingPaths 并发读写也需要保护（来自多路径并行遍历场景）
	var existingMu sync.Mutex

	walkFn := func(path string, info os.FileInfo, err error) error {
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
		// 计数类字段放到最后统一用细粒度锁保护，中间先做无锁过滤
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			collectMu.Lock()
			totalFiles++
			collectMu.Unlock()
			return nil
		}

		// P0: 文件大小过滤（启用 MinFileSize 配置）
		// 注意：.strm 是纯文本的远程流索引文件（通常仅几百字节），
		// 其"大小"与实际媒体内容无关，必须豁免此过滤，否则会被误判为小样片而跳过
		if library.EnableFileFilter && library.MinFileSize > 0 && ext != ".strm" {
			minBytes := int64(library.MinFileSize) * 1024 * 1024
			if info.Size() < minBytes {
				s.logger.Debugf("跳过过小文件(%dMB < %dMB): %s",
					info.Size()/(1024*1024), library.MinFileSize, path)
				collectMu.Lock()
				totalFiles++
				collectMu.Unlock()
				return nil
			}
		}

		// P0: 排除 extras 路径和 Emby 特典后缀文件
		if isExtrasPath(path) || isExtrasFile(filepath.Base(path)) {
			s.logger.Debugf("跳过非正片内容: %s", path)
			collectMu.Lock()
			totalFiles++
			collectMu.Unlock()
			return nil
		}

		// P2: 内存查重（替代逐个 DB 查询）
		// 【火力全开 A】已存在文件的 probe 不再在 walkFn 里同步执行，
		// 而是收集到 updateList，待 walk 结束后与 pendingList 合并走 parallelProbe。
		if existingPaths != nil {
			existingMu.Lock()
			hit := existingPaths[path]
			if hit {
				delete(existingPaths, path)
			}
			existingMu.Unlock()
			if hit {
				// 文件已存在：增量扫描模式下，如果文件未修改则跳过
				if !lastScanTime.IsZero() && info.ModTime().Before(lastScanTime) {
					collectMu.Lock()
					totalFiles++
					videoFiles++
					skippedExist++
					collectMu.Unlock()
					return nil
				}
				// 文件已变更 → 先查 DB 记录，但不在这里 probe
				existing, findErr := s.mediaRepo.FindByFilePath(path)
				if findErr == nil && existing != nil {
					existing.FileSize = info.Size()
					collectMu.Lock()
					totalFiles++
					videoFiles++
					skippedUpdated++
					updateList = append(updateList, pendingMedia{media: existing, path: path, info: info})
					collectMu.Unlock()
				} else {
					collectMu.Lock()
					totalFiles++
					videoFiles++
					collectMu.Unlock()
				}
				return nil
			}
		} else {
			// 回退：逐个查询
			existing, findErr := s.mediaRepo.FindByFilePath(path)
			if findErr == nil && existing != nil {
				if !lastScanTime.IsZero() && info.ModTime().Before(lastScanTime) {
					collectMu.Lock()
					totalFiles++
					videoFiles++
					skippedExist++
					collectMu.Unlock()
					return nil
				}
				existing.FileSize = info.Size()
				collectMu.Lock()
				totalFiles++
				videoFiles++
				skippedUpdated++
				updateList = append(updateList, pendingMedia{media: existing, path: path, info: info})
				collectMu.Unlock()
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
		collectMu.Lock()
		totalFiles++
		videoFiles++
		pendingList = append(pendingList, pendingMedia{media: media, path: path, info: info})
		collectMu.Unlock()
		return nil
	}

	// 【火力全开】多路径并行遍历：
	// 之前串行 for 导致多媒体库 / 多根目录只能一个个扫，
	// 遇到慢盘（网络挂载、WebDAV、外置 USB）会拖死整体进度。
	// 改为每条根路径一个 goroutine，IO 并行打满。
	if len(allPaths) <= 1 {
		for _, root := range allPaths {
			if walkErr := s.walkLibraryPath(root, walkFn); walkErr != nil {
				s.logger.Warnf("扫描路径失败: %s, 错误: %v", root, walkErr)
				err = walkErr
			}
		}
	} else {
		var (
			walkWg   sync.WaitGroup
			errMu    sync.Mutex // 仅保护 firstErr 写入（walkFn 自身已线程安全）
			firstErr error
		)
		// 【火力全开 B】walkFn 内部已使用细粒度锁（collectMu / existingMu）保护共享变量，
		// 这里不再用大锁包裹整个回调，磁盘 readdir 与文件处理在不同根路径间完全并行。
		for _, root := range allPaths {
			root := root
			walkWg.Add(1)
			go func() {
				defer walkWg.Done()
				if walkErr := s.walkLibraryPath(root, walkFn); walkErr != nil {
					s.logger.Warnf("扫描路径失败: %s, 错误: %v", root, walkErr)
					errMu.Lock()
					if firstErr == nil {
						firstErr = walkErr
					}
					errMu.Unlock()
				}
			}()
		}
		walkWg.Wait()
		if firstErr != nil {
			err = firstErr
		}
	}

	// P2: 并行 FFprobe 探测 + 批量入库
	// 【火力全开 A】已存在但需更新的文件(updateList) 也一并走并行 probe，
	// 与 pendingList 合并后只跑一次 Worker Pool，把 CPU 全部吃满。
	if len(pendingList) > 0 || len(updateList) > 0 {
		combined := make([]pendingMedia, 0, len(pendingList)+len(updateList))
		combined = append(combined, pendingList...)
		combined = append(combined, updateList...)
		s.logger.Infof("开始并行 FFprobe 探测 %d 个文件 (新增: %d, 更新: %d)",
			len(combined), len(pendingList), len(updateList))
		s.parallelProbe(combined)
	}

	// 【火力全开 A】处理已更新文件：probe 已在上面并行完成，这里只做字幕扫描 + DB 更新
	if len(updateList) > 0 {
		for _, pm := range updateList {
			s.scanExternalSubtitles(pm.media)
			if err := s.mediaRepo.Update(pm.media); err != nil {
				s.logger.Warnf("更新媒体失败: %s, 错误: %v", pm.path, err)
				continue
			}
			s.logger.Debugf("更新已有媒体: %s", pm.path)
		}
	}

	if len(pendingList) > 0 {
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

			// 识别本地海报封面图片（使用按文件名匹配的方法，避免同目录多视频共用封面）
			if poster, backdrop := s.nfoService.FindLocalImagesForMedia(pm.path); poster != "" || backdrop != "" {
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

	// 清理失效记录：walk 正常完成后，existingPaths 里剩余的路径即为"DB 有但磁盘已不存在"的记录
	// （如用户把文件拖拽到其他位置/删除后的残留）。walk 出错时保守不删，避免误清理。
	staleRemoved := 0
	if err == nil && existingPaths != nil && len(existingPaths) > 0 {
		for stalePath := range existingPaths {
			if m, findErr := s.mediaRepo.FindByFilePath(stalePath); findErr == nil && m != nil {
				if delErr := s.mediaRepo.DeleteByID(m.ID); delErr != nil {
					s.logger.Warnf("删除失效媒体记录失败: %s, 错误: %v", stalePath, delErr)
					continue
				}
				staleRemoved++
				s.logger.Infof("清理失效媒体记录（磁盘已不存在）: %s", stalePath)
			}
		}
	}

	s.logger.Infof("电影库扫描统计: %s — 遍历文件: %d, 视频文件: %d, 新增: %d, 已存在跳过: %d, 已更新: %d, 清理失效: %d",
		library.Name, totalFiles, videoFiles, count, skippedExist, skippedUpdated, staleRemoved)

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
	// 【火力全开】并发数 = NumCPU，让 FFprobe 用满所有 CPU 核心。
	// FFprobe 主要瓶颈是磁盘 IO 与容器解析，单实例占用很低，
	// 多开不会压爆 CPU，反而能把 NVMe/SSD 的 IO 并发打满。
	workers := runtime.NumCPU()
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
	allPaths := library.AllPaths()
	s.logger.Infof("混合媒体库扫描: %s (路径数: %d)", library.Name, len(allPaths))

	// [xiaoya 适配] 将所有媒体库根展开为平铺的真实媒体根集合
	var mediaRoots []string
	for _, p := range allPaths {
		mediaRoots = append(mediaRoots, s.collectMediaRoots(p, "mixed")...)
	}

	var totalCount int
	// === 阶段一：收集子目录，按标准化系列名分组（用于多季合并检测） ===
	seriesDirGroups := make(map[string][]seriesFolder) // 标准化系列名 -> 目录列表
	type movieDirEntry struct {
		entry    os.DirEntry
		rootPath string
	}
	type looseEntry struct {
		entry    os.DirEntry
		rootPath string
	}
	var movieDirs []movieDirEntry    // 被判定为电影的目录
	var looseVideoFiles []looseEntry // 根目录散落的视频文件

	for _, root := range mediaRoots {
		entries, err := s.readDirLibraryPath(root)
		if err != nil {
			s.logger.Warnf("读取混合库根目录失败: %s, 错误: %v", root, err)
			continue
		}
		s.logger.Infof("混合库根 %s 包含 %d 个条目", root, len(entries))

		for _, entry := range entries {
			if !entry.IsDir() {
				// 根目录下的散落视频文件
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if supportedExts[ext] {
					looseVideoFiles = append(looseVideoFiles, looseEntry{entry: entry, rootPath: root})
				}
				continue
			}

			dirName := entry.Name()
			// [xiaoya 适配] 跳过特殊目录（ISO/json/画质演示/extras 等）
			if isXiaoyaSkipDir(dirName) || extrasExcludeDirs[strings.ToLower(dirName)] {
				s.logger.Debugf("[xiaoya] 混合库扫描跳过特殊目录: %s", dirName)
				continue
			}

			folderPath := vfsJoin(root, dirName)

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
				movieDirs = append(movieDirs, movieDirEntry{entry: entry, rootPath: root})
			}
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
		folderPath := vfsJoin(entry.rootPath, entry.entry.Name())
		err := s.walkLibraryPath(folderPath, func(path string, info os.FileInfo, err error) error {
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
		filePath := filepath.Join(entry.rootPath, entry.entry.Name())
		if _, err := s.mediaRepo.FindByFilePath(filePath); err == nil {
			continue // 已存在
		}
		info, err := entry.entry.Info()
		if err != nil {
			continue
		}
		title := s.extractTitle(entry.entry.Name())
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

	// 收尾清理：遍历该库 DB 中所有文件路径，用 os.Stat 检查磁盘是否真的存在，
	// 不存在的直接删除。这样无论文件原来属于电影目录还是剧集目录，都能正确清理。
	// 之前的"限定范围"策略会遗漏被判为剧集目录的电影文件夹中的失效记录。
	dbPathSet, cleanupErr := s.mediaRepo.GetAllFilePathsByLibrary(library.ID)
	if cleanupErr == nil && dbPathSet != nil {
		staleRemoved := 0
		for dbPath := range dbPathSet {
			if _, statErr := s.statLibraryPath(dbPath); os.IsNotExist(statErr) {
				if m, findErr := s.mediaRepo.FindByFilePath(dbPath); findErr == nil && m != nil {
					if delErr := s.mediaRepo.DeleteByID(m.ID); delErr != nil {
						s.logger.Warnf("删除失效媒体记录失败: %s, 错误: %v", dbPath, delErr)
						continue
					}
					staleRemoved++
					s.logger.Infof("清理失效媒体记录（磁盘已不存在）: %s", dbPath)
				}
			}
		}
		if staleRemoved > 0 {
			s.logger.Infof("混合库 %s 清理失效媒体记录: %d 条", library.Name, staleRemoved)
		}
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
	entries, err := s.readDirLibraryPath(folderPath)
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
			subEntries, err := s.readDirLibraryPath(vfsJoin(folderPath, entry.Name()))
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

	allPaths := library.AllPaths()
	s.logger.Infof("剧集库扫描开始: %s, 路径数: %d", library.Name, len(allPaths))

	// [xiaoya 适配] 将所有媒体库根展开为多个"真实剧集根"
	// 普通用户的平铺目录会返回 [library.AllPaths()]（完全向后兼容）
	var mediaRoots []string
	for _, p := range allPaths {
		mediaRoots = append(mediaRoots, s.collectMediaRoots(p, "tvshow")...)
	}

	// 收集根目录下的散落视频文件，按系列名分组（跨所有 roots）
	type looseFile struct {
		entry    os.DirEntry
		info     os.FileInfo
		rootPath string
	}
	seriesGroups := make(map[string][]looseFile) // 系列名 -> 文件列表

	// 标准化系列名 -> 目录列表（跨所有 roots）
	seriesDirGroups := make(map[string][]seriesFolder)

	// === 阶段一：对每个媒体根分别收集剧集目录和散落视频 ===
	for _, root := range mediaRoots {
		entries, err := s.readDirLibraryPath(root)
		if err != nil {
			s.logger.Warnf("读取剧集根目录失败: %s, 错误: %v", root, err)
			continue
		}
		s.logger.Infof("剧集根 %s 包含 %d 个条目", root, len(entries))

		for _, entry := range entries {
			if !entry.IsDir() {
				// 根目录下的视频文件
				ext := strings.ToLower(filepath.Ext(entry.Name()))
				if supportedExts[ext] {
					filePath := vfsJoin(root, entry.Name())
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
					seriesGroups[seriesName] = append(seriesGroups[seriesName], looseFile{entry: entry, info: info, rootPath: root})
				}
				continue
			}

			dirName := entry.Name()
			// [xiaoya 适配] 跳过特殊目录（ISO/json/画质演示/extras 等）
			if isXiaoyaSkipDir(dirName) || extrasExcludeDirs[strings.ToLower(dirName)] {
				s.logger.Debugf("[xiaoya] 剧集扫描跳过特殊目录: %s", dirName)
				continue
			}

			folderPath := vfsJoin(root, dirName)

			// 从目录名提取标准化系列名（去掉季号标识）和季号
			normalizedName := s.normalizeSeriesName(dirName)
			seasonNum := s.extractSeasonFromDirName(dirName)

			seriesDirGroups[normalizedName] = append(seriesDirGroups[normalizedName], seriesFolder{
				path:      folderPath,
				dirName:   dirName,
				seasonNum: seasonNum,
			})
		}
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
				filePath := vfsJoin(f.rootPath, f.entry.Name())
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
				filePath := vfsJoin(f.rootPath, f.entry.Name())
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
			filePath := vfsJoin(f.rootPath, f.entry.Name())
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

	// 收尾清理：检查该库 DB 中所有文件路径，磁盘上不存在的视为失效记录删除
	dbPathSet, ppErr := s.mediaRepo.GetAllFilePathsByLibrary(library.ID)
	if ppErr == nil && dbPathSet != nil {
		staleRemoved := 0
		for dbPath := range dbPathSet {
			if _, statErr := s.statLibraryPath(dbPath); os.IsNotExist(statErr) {
				if m, findErr := s.mediaRepo.FindByFilePath(dbPath); findErr == nil && m != nil {
					if delErr := s.mediaRepo.DeleteByID(m.ID); delErr != nil {
						s.logger.Warnf("删除失效媒体记录失败: %s, 错误: %v", dbPath, delErr)
						continue
					}
					staleRemoved++
					s.logger.Infof("清理失效媒体记录（磁盘已不存在）: %s", dbPath)
				}
			}
		}
		if staleRemoved > 0 {
			s.logger.Infof("剧集库 %s 清理失效媒体记录: %d 条", library.Name, staleRemoved)
		}
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

	s.walkLibraryPath(folderPath, func(path string, info os.FileInfo, err error) error {
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
	meta, err := ParseSTRMFileEnhanced(filePath)
	if err != nil {
		return "", err
	}
	return meta.URL, nil
}

// parseSTRMFileMeta 返回 .strm 的完整元数据（URL + 自定义 Header 等）
func (s *ScannerService) parseSTRMFileMeta(filePath string) (*STRMMeta, error) {
	return ParseSTRMFileEnhanced(filePath)
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
		meta, err := s.parseSTRMFileMeta(media.FilePath)
		if err != nil {
			s.logger.Warnf("解析 STRM 文件失败: %s, 错误: %v", media.FilePath, err)
			return
		}
		ApplySTRMMetaToMedia(media, meta)
		s.probeSTRMMedia(media, meta.URL)

		// 可选：对直链型 STRM 进行远程 FFprobe 探测（慢但能拿到真实时长/编码/分辨率）
		// 仅对 http(s) + 明显的视频直链启用（排除 HLS/磁力/rtmp 等）
		if s.cfg != nil && s.cfg.STRM.RemoteProbe && isDirectVideoLink(meta.URL) {
			// 构造请求头（FFprobe 需要透传 UA / Referer / Cookie）
			ua, referer, cookie, extra := ResolveSTRMHeaders(&s.cfg.STRM, media)
			hdr := http.Header{}
			if ua != "" {
				hdr.Set("User-Agent", ua)
			}
			if referer != "" {
				hdr.Set("Referer", referer)
			}
			if cookie != "" {
				hdr.Set("Cookie", cookie)
			}
			for k, v := range extra {
				hdr.Set(k, v)
			}
			timeout := s.cfg.STRM.RemoteProbeTimeout
			if timeout <= 0 {
				timeout = 8
			}
			if ok := RemoteProbeSTRM(context.Background(), s.cfg.App.FFprobePath, media, hdr, timeout); ok {
				s.logger.Debugf("STRM 远程 probe 成功: %s", media.FilePath)
			}
		}
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
// 以及国内资源站的脏命名：
//   - "01届.《翼》-《Wings》-1927-1929。【十万度Q裙 319940383】.mkv"
//   - "[yyh3d.com]采花和尚.Satyr Monks.1994.LD_D9.x264.AAC.480P.YYH3D.xt.mkv"
func (s *ScannerService) extractTitleEnhanced(filename string) (title string, year int, tmdbID int) {
	// 优先走统一增强解析器：它能处理《》、【广告】、[站点]、XX届、115chrome 等脏命名
	if parsed := ParseMovieFilename(filename); parsed.Title != "" {
		title = parsed.Title
		year = parsed.Year
		tmdbID = parsed.TMDbID
		return
	}

	// —— 兜底：保留原本的简单清洗逻辑，避免极端边界下丢名 ——
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

// ==================== P0: 批量字幕提取导出 ====================

// ExtractedSubtitleFile 提取后的字幕文件信息
type ExtractedSubtitleFile struct {
	TrackIndex int    `json:"track_index"`
	Language   string `json:"language"`
	Title      string `json:"title"`
	Codec      string `json:"codec"`
	Format     string `json:"format"`
	Path       string `json:"path"`
	Bitmap     bool   `json:"bitmap"`
	Error      string `json:"error,omitempty"`
}

// ExtractAllSubtitles 批量提取视频中所有文本字幕轨道
// format: 输出格式 "srt" | "vtt"
// trackIndices: 指定轨道索引列表，为空则提取所有文本字幕
func (s *ScannerService) ExtractAllSubtitles(filePath string, format string, trackIndices []int) ([]ExtractedSubtitleFile, error) {
	if format == "" {
		format = "srt"
	}

	// 获取所有字幕轨道
	tracks, err := s.GetSubtitleTracks(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取字幕轨道失败: %w", err)
	}

	if len(tracks) == 0 {
		return nil, fmt.Errorf("该视频文件不包含任何字幕轨道")
	}

	// 过滤要提取的轨道
	var targetTracks []SubtitleTrack
	if len(trackIndices) > 0 {
		indexSet := make(map[int]bool)
		for _, idx := range trackIndices {
			indexSet[idx] = true
		}
		for _, t := range tracks {
			if indexSet[t.Index] {
				targetTracks = append(targetTracks, t)
			}
		}
	} else {
		targetTracks = tracks
	}

	var results []ExtractedSubtitleFile

	for _, track := range targetTracks {
		result := ExtractedSubtitleFile{
			TrackIndex: track.Index,
			Language:   track.Language,
			Title:      track.Title,
			Codec:      track.Codec,
			Bitmap:     track.Bitmap,
			Format:     format,
		}

		if track.Bitmap {
			result.Error = "图形字幕（" + track.Codec + "）无法提取为文本格式"
			results = append(results, result)
			continue
		}

		outputPath, err := s.ExtractSubtitle(filePath, track.Index, format)
		if err != nil {
			result.Error = err.Error()
			s.logger.Warnf("提取字幕轨道 #%d 失败: %v", track.Index, err)
		} else {
			result.Path = outputPath
		}

		results = append(results, result)
	}

	return results, nil
}

// ==================== P1: 编码自动检测与转换 ====================

// ConvertSubtitleToVTTWithEncoding 带编码检测的字幕转换
// 先检测文件编码，如果非 UTF-8 则先转码为 UTF-8 临时文件，再交给 FFmpeg 转换
func (s *ScannerService) ConvertSubtitleToVTTWithEncoding(subtitlePath string) (string, error) {
	// 确定输出文件路径
	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "subtitles")
	os.MkdirAll(cacheDir, 0755)

	baseName := strings.TrimSuffix(filepath.Base(subtitlePath), filepath.Ext(subtitlePath))
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("%s_ext.vtt", baseName))

	// 检查缓存
	if outInfo, err := os.Stat(outputPath); err == nil {
		if srcInfo, err := os.Stat(subtitlePath); err == nil {
			if outInfo.ModTime().After(srcInfo.ModTime()) {
				return outputPath, nil
			}
		}
	}

	// 编码检测：读取原始文件
	raw, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", fmt.Errorf("读取字幕文件失败: %w", err)
	}

	// 检测是否为 UTF-8
	inputPath := subtitlePath
	needCleanup := false

	if !isValidUTF8(raw) {
		// 非 UTF-8，使用 SubtitleCleaner 的编码检测逻辑
		cleaner := NewSubtitleCleaner(SubtitleCleanConfig{AutoDetectEncoding: true}, s.logger)
		content, encoding, converted := cleaner.detectAndConvertEncoding(subtitlePath)

		if converted && content != "" {
			s.logger.Infof("字幕编码转换: %s -> UTF-8 (检测到: %s)", filepath.Base(subtitlePath), encoding)

			// 写入 UTF-8 临时文件
			tmpPath := filepath.Join(cacheDir, fmt.Sprintf("%s_utf8%s", baseName, filepath.Ext(subtitlePath)))
			if err := os.WriteFile(tmpPath, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("写入 UTF-8 临时文件失败: %w", err)
			}
			inputPath = tmpPath
			needCleanup = true
		}
	}

	// 使用 FFmpeg 转换为 WebVTT
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-y",
		"-i", inputPath,
		"-c:s", "webvtt",
		outputPath,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		// 清理临时文件
		if needCleanup {
			os.Remove(inputPath)
		}
		return "", fmt.Errorf("FFmpeg字幕转换失败: %w, 输出: %s", err, string(output))
	}

	// 清理临时文件
	if needCleanup {
		os.Remove(inputPath)
	}

	s.logger.Debugf("字幕转换完成（含编码检测）: %s -> %s", subtitlePath, outputPath)
	return outputPath, nil
}

// isValidUTF8 检查字节数据是否为有效的 UTF-8 编码
func isValidUTF8(data []byte) bool {
	// 跳过 BOM
	if len(data) >= 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		data = data[3:]
	}
	return utf8.Valid(data)
}

// EnsureUTF8Subtitle 确保字幕文件为 UTF-8 编码（保持原始格式不变）
// 用于 Android 端 ExoPlayer 直接解析 ASS/SRT 等格式时，确保编码正确
func (s *ScannerService) EnsureUTF8Subtitle(subtitlePath string) (string, error) {
	// 读取原始文件
	raw, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", fmt.Errorf("读取字幕文件失败: %w", err)
	}

	// 如果已经是 UTF-8，直接返回原始路径
	if isValidUTF8(raw) {
		return subtitlePath, nil
	}

	// 非 UTF-8，进行编码转换
	cacheDir := filepath.Join(s.cfg.Cache.CacheDir, "subtitles")
	os.MkdirAll(cacheDir, 0755)

	baseName := strings.TrimSuffix(filepath.Base(subtitlePath), filepath.Ext(subtitlePath))
	ext := filepath.Ext(subtitlePath)
	outputPath := filepath.Join(cacheDir, fmt.Sprintf("%s_utf8%s", baseName, ext))

	// 检查缓存
	if outInfo, err := os.Stat(outputPath); err == nil {
		if srcInfo, err := os.Stat(subtitlePath); err == nil {
			if outInfo.ModTime().After(srcInfo.ModTime()) {
				return outputPath, nil
			}
		}
	}

	// 使用编码检测逻辑转换
	cleaner := NewSubtitleCleaner(SubtitleCleanConfig{AutoDetectEncoding: true}, s.logger)
	content, encoding, converted := cleaner.detectAndConvertEncoding(subtitlePath)

	if converted && content != "" {
		s.logger.Infof("字幕编码转换（保持原格式）: %s -> UTF-8 (检测到: %s)", filepath.Base(subtitlePath), encoding)
		if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
			return "", fmt.Errorf("写入 UTF-8 字幕文件失败: %w", err)
		}
		return outputPath, nil
	}

	// 转换失败，返回原始文件
	return subtitlePath, nil
}

// ==================== P2: 异步字幕提取 + 进度反馈 ====================

// 字幕提取事件常量
const (
	EventSubExtractStarted   = "sub_extract_started"
	EventSubExtractProgress  = "sub_extract_progress"
	EventSubExtractCompleted = "sub_extract_completed"
	EventSubExtractFailed    = "sub_extract_failed"
)

// SubExtractProgressData 字幕提取进度事件数据
type SubExtractProgressData struct {
	MediaID    string                  `json:"media_id"`
	MediaTitle string                  `json:"media_title"`
	Format     string                  `json:"format"`
	Total      int                     `json:"total"`
	Current    int                     `json:"current"`
	Progress   float64                 `json:"progress"`
	Message    string                  `json:"message"`
	Results    []ExtractedSubtitleFile `json:"results,omitempty"`
	Error      string                  `json:"error,omitempty"`
}

// ExtractAllSubtitlesAsync 异步批量提取字幕（适用于大文件，通过 WebSocket 推送进度）
func (s *ScannerService) ExtractAllSubtitlesAsync(mediaID, mediaTitle, filePath, format string, trackIndices []int) {
	go func() {
		if format == "" {
			format = "srt"
		}

		// 广播开始事件
		s.broadcastSubExtractEvent(EventSubExtractStarted, &SubExtractProgressData{
			MediaID:    mediaID,
			MediaTitle: mediaTitle,
			Format:     format,
			Message:    "开始提取字幕...",
		})

		// 获取所有字幕轨道
		tracks, err := s.GetSubtitleTracks(filePath)
		if err != nil {
			s.broadcastSubExtractEvent(EventSubExtractFailed, &SubExtractProgressData{
				MediaID:    mediaID,
				MediaTitle: mediaTitle,
				Error:      fmt.Sprintf("获取字幕轨道失败: %v", err),
				Message:    "提取失败",
			})
			return
		}

		// 过滤目标轨道
		var targetTracks []SubtitleTrack
		if len(trackIndices) > 0 {
			indexSet := make(map[int]bool)
			for _, idx := range trackIndices {
				indexSet[idx] = true
			}
			for _, t := range tracks {
				if indexSet[t.Index] {
					targetTracks = append(targetTracks, t)
				}
			}
		} else {
			targetTracks = tracks
		}

		total := len(targetTracks)
		var results []ExtractedSubtitleFile

		for i, track := range targetTracks {
			result := ExtractedSubtitleFile{
				TrackIndex: track.Index,
				Language:   track.Language,
				Title:      track.Title,
				Codec:      track.Codec,
				Bitmap:     track.Bitmap,
				Format:     format,
			}

			// 广播进度
			progress := float64(i) / float64(total) * 100
			s.broadcastSubExtractEvent(EventSubExtractProgress, &SubExtractProgressData{
				MediaID:    mediaID,
				MediaTitle: mediaTitle,
				Format:     format,
				Total:      total,
				Current:    i + 1,
				Progress:   progress,
				Message:    fmt.Sprintf("正在提取轨道 #%d (%d/%d)...", track.Index, i+1, total),
			})

			if track.Bitmap {
				result.Error = "图形字幕无法提取为文本格式"
			} else {
				outputPath, err := s.ExtractSubtitle(filePath, track.Index, format)
				if err != nil {
					result.Error = err.Error()
					s.logger.Warnf("异步提取字幕轨道 #%d 失败: %v", track.Index, err)
				} else {
					result.Path = outputPath
				}
			}

			results = append(results, result)
		}

		// 广播完成事件
		successCount := 0
		for _, r := range results {
			if r.Error == "" {
				successCount++
			}
		}

		s.broadcastSubExtractEvent(EventSubExtractCompleted, &SubExtractProgressData{
			MediaID:    mediaID,
			MediaTitle: mediaTitle,
			Format:     format,
			Total:      total,
			Current:    total,
			Progress:   100,
			Message:    fmt.Sprintf("提取完成: %d/%d 个轨道成功", successCount, total),
			Results:    results,
		})

		s.logger.Infof("异步字幕提取完成: %s, %d/%d 成功", mediaTitle, successCount, total)
	}()
}

// broadcastSubExtractEvent 广播字幕提取事件
func (s *ScannerService) broadcastSubExtractEvent(eventType string, data *SubExtractProgressData) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
}

// ==================== 重复媒体检测 ====================

// DuplicateGroup 重复媒体组
type DuplicateGroup struct {
	GroupKey   string          `json:"group_key"`   // 分组键（标题+年份）
	Title      string          `json:"title"`       // 标题
	Year       int             `json:"year"`        // 年份
	MediaCount int             `json:"media_count"` // 重复数量
	Media      []DuplicateItem `json:"media"`       // 重复的媒体列表
	Suggestion string          `json:"suggestion"`  // 处理建议
}

// DuplicateItem 重复媒体项
type DuplicateItem struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	FilePath   string  `json:"file_path"`
	FileSize   int64   `json:"file_size"`
	Resolution string  `json:"resolution"`
	VideoCodec string  `json:"video_codec"`
	AudioCodec string  `json:"audio_codec"`
	Duration   float64 `json:"duration"`
	LibraryID  string  `json:"library_id"`
	IsPrimary  bool    `json:"is_primary"` // 是否为推荐保留的版本
}

// DetectDuplicates 检测媒体库中的重复媒体
// 检测策略：
// 1. 基于标题+年份的精确匹配
// 2. 基于文件大小+时长的近似匹配
// 3. 基于 TMDb ID 的精确匹配
func (s *ScannerService) DetectDuplicates(libraryID string) ([]DuplicateGroup, error) {
	var media []model.Media
	query := s.mediaRepo.DB().Where("media_type = ? AND deleted_at IS NULL", "movie")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if err := query.Find(&media).Error; err != nil {
		return nil, fmt.Errorf("查询媒体列表失败: %w", err)
	}

	// 按标题+年份分组
	titleYearGroups := make(map[string][]model.Media)
	// 按 TMDb ID 分组
	tmdbGroups := make(map[int][]model.Media)

	for _, m := range media {
		// 标题+年份分组
		normalizedTitle := strings.ToLower(strings.TrimSpace(m.Title))
		key := fmt.Sprintf("%s|%d", normalizedTitle, m.Year)
		titleYearGroups[key] = append(titleYearGroups[key], m)

		// TMDb ID 分组
		if m.TMDbID > 0 {
			tmdbGroups[m.TMDbID] = append(tmdbGroups[m.TMDbID], m)
		}
	}

	// 合并检测结果（去重）
	seen := make(map[string]bool) // 已处理的媒体 ID
	var groups []DuplicateGroup

	// 优先使用 TMDb ID 匹配（最精确）
	for tmdbID, mediaList := range tmdbGroups {
		if len(mediaList) < 2 {
			continue
		}
		group := s.buildDuplicateGroup(mediaList, fmt.Sprintf("tmdb:%d", tmdbID))
		groups = append(groups, group)
		for _, m := range mediaList {
			seen[m.ID] = true
		}
	}

	// 标题+年份匹配（排除已被 TMDb ID 匹配的）
	for key, mediaList := range titleYearGroups {
		if len(mediaList) < 2 {
			continue
		}
		// 过滤已处理的
		var filtered []model.Media
		for _, m := range mediaList {
			if !seen[m.ID] {
				filtered = append(filtered, m)
			}
		}
		if len(filtered) < 2 {
			continue
		}
		group := s.buildDuplicateGroup(filtered, key)
		groups = append(groups, group)
	}

	// 按重复数量降序排列
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].MediaCount > groups[j].MediaCount
	})

	s.logger.Infof("重复媒体检测完成: 发现 %d 组重复", len(groups))
	return groups, nil
}

// buildDuplicateGroup 构建重复组信息
func (s *ScannerService) buildDuplicateGroup(mediaList []model.Media, groupKey string) DuplicateGroup {
	group := DuplicateGroup{
		GroupKey:   groupKey,
		Title:      mediaList[0].Title,
		Year:       mediaList[0].Year,
		MediaCount: len(mediaList),
	}

	// 选择推荐保留的版本（优先级：4K > 1080p > 720p，同分辨率选文件最大的）
	resolutionPriority := map[string]int{
		"4K": 5, "2K": 4, "1080p": 3, "720p": 2, "480p": 1,
	}

	bestIdx := 0
	bestScore := 0
	for i, m := range mediaList {
		score := resolutionPriority[m.Resolution] * 1000000
		score += int(m.FileSize / (1024 * 1024)) // 加上文件大小（MB）作为次要排序
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	for i, m := range mediaList {
		item := DuplicateItem{
			ID:         m.ID,
			Title:      m.Title,
			FilePath:   m.FilePath,
			FileSize:   m.FileSize,
			Resolution: m.Resolution,
			VideoCodec: m.VideoCodec,
			AudioCodec: m.AudioCodec,
			Duration:   m.Duration,
			LibraryID:  m.LibraryID,
			IsPrimary:  i == bestIdx,
		}
		group.Media = append(group.Media, item)
	}

	// 生成处理建议
	if len(mediaList) == 2 {
		best := mediaList[bestIdx]
		otherIdx := 1 - bestIdx
		other := mediaList[otherIdx]
		group.Suggestion = fmt.Sprintf("建议保留 %s 版本（%s, %.1fGB），可删除 %s 版本（%s, %.1fGB）",
			best.Resolution, best.VideoCodec, float64(best.FileSize)/(1024*1024*1024),
			other.Resolution, other.VideoCodec, float64(other.FileSize)/(1024*1024*1024))
	} else {
		group.Suggestion = fmt.Sprintf("发现 %d 个重复版本，建议保留最高质量版本", len(mediaList))
	}

	return group
}

// MarkDuplicates 标记重复媒体（在扫描完成后调用）
func (s *ScannerService) MarkDuplicates(libraryID string) (int, error) {
	groups, err := s.DetectDuplicates(libraryID)
	if err != nil {
		return 0, err
	}

	marked := 0
	for _, group := range groups {
		for _, item := range group.Media {
			if item.IsPrimary {
				// 主版本：设置 DuplicateGroup 但不设置 DuplicateOf
				s.mediaRepo.UpdateFields(item.ID, map[string]interface{}{
					"duplicate_group": group.GroupKey,
					"duplicate_of":    "",
				})
			} else {
				// 重复版本：设置 DuplicateOf 指向主版本
				primaryID := ""
				for _, m := range group.Media {
					if m.IsPrimary {
						primaryID = m.ID
						break
					}
				}
				s.mediaRepo.UpdateFields(item.ID, map[string]interface{}{
					"duplicate_group": group.GroupKey,
					"duplicate_of":    primaryID,
				})
				marked++
			}
		}
	}

	s.logger.Infof("重复媒体标记完成: 标记 %d 个重复文件", marked)
	return marked, nil
}
