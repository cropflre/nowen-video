package service

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== EMBY 兼容服务 ====================

// EmbyCompatService EMBY 媒体库格式兼容服务
// 支持自动识别 EMBY 标准文件夹结构、解析 NFO/XML 元数据、批量导入
type EmbyCompatService struct {
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	nfoService *NFOService
	scanner    *ScannerService
	logger     *zap.SugaredLogger
	wsHub      *WSHub
}

func NewEmbyCompatService(
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	nfoService *NFOService,
	scanner *ScannerService,
	logger *zap.SugaredLogger,
) *EmbyCompatService {
	return &EmbyCompatService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		nfoService: nfoService,
		scanner:    scanner,
		logger:     logger,
	}
}

func (s *EmbyCompatService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== EMBY NFO 增强结构体 ====================

// EmbyMovieNFO EMBY 电影 NFO（扩展字段）
type EmbyMovieNFO struct {
	XMLName     xml.Name       `xml:"movie"`
	Title       string         `xml:"title"`
	OrigTitle   string         `xml:"originaltitle"`
	SortTitle   string         `xml:"sorttitle"`
	Year        int            `xml:"year"`
	Plot        string         `xml:"plot"`
	Outline     string         `xml:"outline"`
	Tagline     string         `xml:"tagline"`
	Rating      float64        `xml:"rating"`
	UserRating  float64        `xml:"userrating"`
	Runtime     int            `xml:"runtime"`
	Studio      string         `xml:"studio"`
	Country     string         `xml:"country"`
	Language    string         `xml:"language"`
	TMDbID      int            `xml:"tmdbid"`
	IMDBID      string         `xml:"imdbid"`
	DoubanID    string         `xml:"doubanid"`
	Genres      []string       `xml:"genre"`
	Tags        []string       `xml:"tag"`
	Directors   []string       `xml:"director"`
	Credits     []string       `xml:"credits"` // 编剧
	Actors      []EmbyNFOActor `xml:"actor"`
	Premiered   string         `xml:"premiered"`       // 首映日期 YYYY-MM-DD
	DateAdded   string         `xml:"dateadded"`       // 添加日期
	MPAA        string         `xml:"mpaa"`            // 分级 PG-13 等
	Set         *EmbyNFOSet    `xml:"set"`             // 电影合集
	Thumb       []EmbyNFOArt   `xml:"thumb"`           // 缩略图/海报
	Fanart      *EmbyNFOFanart `xml:"fanart"`          // 背景图
	FileInfo    *EmbyFileInfo  `xml:"fileinfo"`        // 文件信息
	PlayCount   int            `xml:"playcount"`       // 播放次数
	LastPlayed  string         `xml:"lastplayed"`      // 最后播放时间
	ResumePos   float64        `xml:"resume>position"` // 播放进度（秒）
	ResumeTotal float64        `xml:"resume>total"`    // 总时长（秒）
}

// EmbyEpisodeNFO EMBY 单集 NFO
type EmbyEpisodeNFO struct {
	XMLName     xml.Name       `xml:"episodedetails"`
	Title       string         `xml:"title"`
	OrigTitle   string         `xml:"originaltitle"`
	Season      int            `xml:"season"`
	Episode     int            `xml:"episode"`
	Plot        string         `xml:"plot"`
	Rating      float64        `xml:"rating"`
	Runtime     int            `xml:"runtime"`
	Aired       string         `xml:"aired"` // 播出日期
	Premiered   string         `xml:"premiered"`
	Directors   []string       `xml:"director"`
	Credits     []string       `xml:"credits"`
	Actors      []EmbyNFOActor `xml:"actor"`
	Thumb       []EmbyNFOArt   `xml:"thumb"`
	PlayCount   int            `xml:"playcount"`
	LastPlayed  string         `xml:"lastplayed"`
	ResumePos   float64        `xml:"resume>position"`
	ResumeTotal float64        `xml:"resume>total"`
	FileInfo    *EmbyFileInfo  `xml:"fileinfo"`
}

// EmbyNFOActor EMBY 演员信息（扩展）
type EmbyNFOActor struct {
	Name      string `xml:"name"`
	Role      string `xml:"role"`
	Type      string `xml:"type"`    // Actor / Director / Writer
	Thumb     string `xml:"thumb"`   // 头像URL
	Profile   string `xml:"profile"` // 个人资料URL
	SortOrder int    `xml:"sortorder"`
	TMDbID    int    `xml:"tmdbid"`
}

// EmbyNFOSet 电影合集
type EmbyNFOSet struct {
	Name     string `xml:"name"`
	Overview string `xml:"overview"`
}

// EmbyNFOArt 图片资源
type EmbyNFOArt struct {
	Aspect string `xml:"aspect,attr"` // poster / banner / thumb
	URL    string `xml:",chardata"`
}

// EmbyNFOFanart 背景图
type EmbyNFOFanart struct {
	Thumbs []EmbyNFOArt `xml:"thumb"`
}

// EmbyFileInfo 文件信息
type EmbyFileInfo struct {
	StreamDetails *EmbyStreamDetails `xml:"streamdetails"`
}

// EmbyStreamDetails 流信息
type EmbyStreamDetails struct {
	Video    *EmbyVideoStream     `xml:"video"`
	Audio    []EmbyAudioStream    `xml:"audio"`
	Subtitle []EmbySubtitleStream `xml:"subtitle"`
}

// EmbyVideoStream 视频流
type EmbyVideoStream struct {
	Codec             string  `xml:"codec"`
	Aspect            string  `xml:"aspect"`
	Width             int     `xml:"width"`
	Height            int     `xml:"height"`
	DurationInSeconds float64 `xml:"durationinseconds"`
}

// EmbyAudioStream 音频流
type EmbyAudioStream struct {
	Codec    string `xml:"codec"`
	Language string `xml:"language"`
	Channels int    `xml:"channels"`
}

// EmbySubtitleStream 字幕流
type EmbySubtitleStream struct {
	Language string `xml:"language"`
}

// ==================== EMBY 文件夹结构识别 ====================

// EmbyFolderType EMBY 文件夹类型
type EmbyFolderType string

const (
	EmbyFolderMovies  EmbyFolderType = "movies"
	EmbyFolderTVShows EmbyFolderType = "tvshows"
	EmbyFolderMixed   EmbyFolderType = "mixed"
	EmbyFolderUnknown EmbyFolderType = "unknown"
)

// EmbyDetectResult EMBY 格式检测结果
type EmbyDetectResult struct {
	IsEmbyFormat    bool              `json:"is_emby_format"`
	FolderType      EmbyFolderType    `json:"folder_type"`
	TotalFiles      int               `json:"total_files"`
	VideoFiles      int               `json:"video_files"`
	NFOFiles        int               `json:"nfo_files"`
	ImageFiles      int               `json:"image_files"`
	SubtitleFiles   int               `json:"subtitle_files"`
	HasMetadata     bool              `json:"has_metadata"`
	FolderStructure string            `json:"folder_structure"` // 文件夹结构描述
	Movies          []EmbyScannedItem `json:"movies,omitempty"`
	TVShows         []EmbyScannedItem `json:"tvshows,omitempty"`
	Confidence      float64           `json:"confidence"` // 置信度 0-1
}

// EmbyScannedItem 扫描到的 EMBY 媒体项
type EmbyScannedItem struct {
	Path          string              `json:"path"`
	Title         string              `json:"title"`
	Year          int                 `json:"year"`
	MediaType     string              `json:"media_type"` // movie / tvshow
	VideoFiles    []string            `json:"video_files"`
	NFOFile       string              `json:"nfo_file"`
	PosterFile    string              `json:"poster_file"`
	BackdropFile  string              `json:"backdrop_file"`
	SubtitleFiles []string            `json:"subtitle_files"`
	Seasons       []EmbyScannedSeason `json:"seasons,omitempty"`
	HasNFO        bool                `json:"has_nfo"`
	Imported      bool                `json:"imported"` // 是否已导入
}

// EmbyScannedSeason 扫描到的季
type EmbyScannedSeason struct {
	SeasonNum  int      `json:"season_num"`
	Path       string   `json:"path"`
	Episodes   int      `json:"episodes"`
	VideoFiles []string `json:"video_files"`
}

// DetectEmbyFormat 检测指定目录是否为 EMBY 格式的媒体库
func (s *EmbyCompatService) DetectEmbyFormat(rootPath string) (*EmbyDetectResult, error) {
	result := &EmbyDetectResult{
		Movies:  make([]EmbyScannedItem, 0),
		TVShows: make([]EmbyScannedItem, 0),
	}

	// 检查目录是否存在
	info, err := os.Stat(rootPath)
	if err != nil {
		return nil, fmt.Errorf("目录不存在: %s", rootPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", rootPath)
	}

	// 遍历根目录
	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var nfoCount, videoCount, imageCount, subtitleCount, totalCount int
	var hasMovieFolder, hasTVShowFolder bool

	// EMBY 标准文件夹名称（不区分大小写）
	embyMovieFolders := map[string]bool{
		"movies": true, "电影": true, "movie": true, "films": true, "film": true,
	}
	embyTVFolders := map[string]bool{
		"tv shows": true, "tv": true, "tvshows": true, "电视剧": true, "series": true,
		"shows": true, "tv series": true, "剧集": true, "动漫": true, "anime": true,
	}

	for _, entry := range entries {
		totalCount++
		name := entry.Name()
		nameLower := strings.ToLower(name)

		if entry.IsDir() {
			// 检查是否为 EMBY 标准文件夹
			if embyMovieFolders[nameLower] {
				hasMovieFolder = true
				items := s.scanEmbyMovieFolder(filepath.Join(rootPath, name))
				result.Movies = append(result.Movies, items...)
			} else if embyTVFolders[nameLower] {
				hasTVShowFolder = true
				items := s.scanEmbyTVShowFolder(filepath.Join(rootPath, name))
				result.TVShows = append(result.TVShows, items...)
			} else {
				// 尝试智能判断子目录类型
				subPath := filepath.Join(rootPath, name)
				itemType := s.detectEmbyItemType(subPath, name)
				switch itemType {
				case "movie":
					item := s.scanEmbyMovieItem(subPath, name)
					if item != nil {
						result.Movies = append(result.Movies, *item)
					}
				case "tvshow":
					item := s.scanEmbyTVShowItem(subPath, name)
					if item != nil {
						result.TVShows = append(result.TVShows, *item)
					}
				}
			}
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			switch {
			case ext == ".nfo":
				nfoCount++
			case isVideoExt(ext):
				videoCount++
			case isImageExt(ext):
				imageCount++
			case isSubtitleExt(ext):
				subtitleCount++
			}
		}
	}

	// 统计子目录中的文件
	for _, item := range result.Movies {
		videoCount += len(item.VideoFiles)
		if item.HasNFO {
			nfoCount++
		}
		if item.PosterFile != "" {
			imageCount++
		}
		if item.BackdropFile != "" {
			imageCount++
		}
		subtitleCount += len(item.SubtitleFiles)
	}
	for _, item := range result.TVShows {
		videoCount += len(item.VideoFiles)
		if item.HasNFO {
			nfoCount++
		}
		for _, season := range item.Seasons {
			videoCount += len(season.VideoFiles)
		}
	}

	result.TotalFiles = totalCount
	result.VideoFiles = videoCount
	result.NFOFiles = nfoCount
	result.ImageFiles = imageCount
	result.SubtitleFiles = subtitleCount
	result.HasMetadata = nfoCount > 0

	// 判断是否为 EMBY 格式
	confidence := 0.0
	if hasMovieFolder || hasTVShowFolder {
		confidence += 0.4
	}
	if nfoCount > 0 {
		confidence += 0.3
	}
	if imageCount > 0 {
		confidence += 0.1
	}
	if len(result.Movies) > 0 || len(result.TVShows) > 0 {
		confidence += 0.2
	}

	result.IsEmbyFormat = confidence >= 0.3
	result.Confidence = confidence

	// 确定文件夹类型
	if hasMovieFolder && hasTVShowFolder {
		result.FolderType = EmbyFolderMixed
	} else if hasMovieFolder || (len(result.Movies) > 0 && len(result.TVShows) == 0) {
		result.FolderType = EmbyFolderMovies
	} else if hasTVShowFolder || (len(result.TVShows) > 0 && len(result.Movies) == 0) {
		result.FolderType = EmbyFolderTVShows
	} else if len(result.Movies) > 0 && len(result.TVShows) > 0 {
		result.FolderType = EmbyFolderMixed
	} else {
		result.FolderType = EmbyFolderUnknown
	}

	// 生成结构描述
	result.FolderStructure = fmt.Sprintf("电影: %d, 剧集: %d, NFO: %d, 图片: %d",
		len(result.Movies), len(result.TVShows), nfoCount, imageCount)

	// 标记已导入的项
	s.markImportedItems(result)

	return result, nil
}

// ==================== EMBY 批量导入 ====================

// EmbyImportRequest EMBY 导入请求
type EmbyImportRequest struct {
	RootPath        string   `json:"root_path"`
	TargetLibraryID string   `json:"target_library_id"`
	ImportMode      string   `json:"import_mode"`     // full（全量）/ incremental（增量）
	ImportNFO       bool     `json:"import_nfo"`      // 是否导入 NFO 元数据
	ImportImages    bool     `json:"import_images"`   // 是否导入图片
	ImportProgress  bool     `json:"import_progress"` // 是否导入播放进度
	SelectedPaths   []string `json:"selected_paths"`  // 选择性导入的路径列表（空则全部导入）
}

// EmbyImportProgress 导入进度
type EmbyImportProgress struct {
	Phase       string `json:"phase"` // detecting / importing / metadata / done
	Current     int    `json:"current"`
	Total       int    `json:"total"`
	CurrentItem string `json:"current_item"`
	Imported    int    `json:"imported"`
	Skipped     int    `json:"skipped"`
	Failed      int    `json:"failed"`
	Message     string `json:"message"`
}

// EmbyImportResult EMBY 导入结果
type EmbyImportResult struct {
	Total          int      `json:"total"`
	Imported       int      `json:"imported"`
	Skipped        int      `json:"skipped"`
	Failed         int      `json:"failed"`
	MoviesImported int      `json:"movies_imported"`
	SeriesImported int      `json:"series_imported"`
	NFOParsed      int      `json:"nfo_parsed"`
	ImagesMapped   int      `json:"images_mapped"`
	Errors         []string `json:"errors"`
}

// ImportEmbyLibrary 从 EMBY 格式的文件夹导入媒体库
func (s *EmbyCompatService) ImportEmbyLibrary(req EmbyImportRequest) (*EmbyImportResult, error) {
	result := &EmbyImportResult{
		Errors: make([]string, 0),
	}

	// 检测 EMBY 格式
	s.broadcastProgress("detecting", 0, 0, "", "正在检测 EMBY 媒体库格式...")
	detect, err := s.DetectEmbyFormat(req.RootPath)
	if err != nil {
		return nil, fmt.Errorf("检测 EMBY 格式失败: %w", err)
	}

	if !detect.IsEmbyFormat {
		return nil, fmt.Errorf("指定目录不是有效的 EMBY 媒体库格式（置信度: %.0f%%）", detect.Confidence*100)
	}

	// 计算总数
	allMovies := detect.Movies
	allTVShows := detect.TVShows

	// 如果指定了选择性导入路径，过滤
	if len(req.SelectedPaths) > 0 {
		selectedSet := make(map[string]bool)
		for _, p := range req.SelectedPaths {
			selectedSet[p] = true
		}
		filteredMovies := make([]EmbyScannedItem, 0)
		for _, m := range allMovies {
			if selectedSet[m.Path] {
				filteredMovies = append(filteredMovies, m)
			}
		}
		filteredTVShows := make([]EmbyScannedItem, 0)
		for _, t := range allTVShows {
			if selectedSet[t.Path] {
				filteredTVShows = append(filteredTVShows, t)
			}
		}
		allMovies = filteredMovies
		allTVShows = filteredTVShows
	}

	result.Total = len(allMovies) + len(allTVShows)
	current := 0

	// 导入电影
	for _, movie := range allMovies {
		current++
		s.broadcastProgress("importing", current, result.Total, movie.Title,
			fmt.Sprintf("导入电影: %s", movie.Title))

		if movie.Imported && req.ImportMode == "incremental" {
			result.Skipped++
			continue
		}

		imported, err := s.importEmbyMovie(movie, req)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("电影 %s: %v", movie.Title, err))
			continue
		}
		if imported {
			result.Imported++
			result.MoviesImported++
		} else {
			result.Skipped++
		}
	}

	// 导入剧集
	for _, tvshow := range allTVShows {
		current++
		s.broadcastProgress("importing", current, result.Total, tvshow.Title,
			fmt.Sprintf("导入剧集: %s", tvshow.Title))

		if tvshow.Imported && req.ImportMode == "incremental" {
			result.Skipped++
			continue
		}

		importedCount, err := s.importEmbyTVShow(tvshow, req)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("剧集 %s: %v", tvshow.Title, err))
			continue
		}
		if importedCount > 0 {
			result.Imported++
			result.SeriesImported++
		} else {
			result.Skipped++
		}
	}

	s.broadcastProgress("done", result.Total, result.Total, "",
		fmt.Sprintf("导入完成: 成功 %d, 跳过 %d, 失败 %d", result.Imported, result.Skipped, result.Failed))

	s.logger.Infof("EMBY 导入完成: 总计 %d, 导入 %d (电影 %d, 剧集 %d), 跳过 %d, 失败 %d",
		result.Total, result.Imported, result.MoviesImported, result.SeriesImported,
		result.Skipped, result.Failed)

	return result, nil
}

// ==================== 内部方法：文件夹扫描 ====================

// scanEmbyMovieFolder 扫描 EMBY 电影文件夹
func (s *EmbyCompatService) scanEmbyMovieFolder(folderPath string) []EmbyScannedItem {
	var items []EmbyScannedItem

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		s.logger.Warnf("读取电影目录失败: %s, %v", folderPath, err)
		return items
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item := s.scanEmbyMovieItem(filepath.Join(folderPath, entry.Name()), entry.Name())
		if item != nil {
			items = append(items, *item)
		}
	}

	// 也扫描根目录下的散落视频文件
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isVideoExt(ext) {
			title, year := parseEmbyMovieName(strings.TrimSuffix(entry.Name(), ext))
			items = append(items, EmbyScannedItem{
				Path:       filepath.Join(folderPath, entry.Name()),
				Title:      title,
				Year:       year,
				MediaType:  "movie",
				VideoFiles: []string{filepath.Join(folderPath, entry.Name())},
				NFOFile:    s.findNFOForFile(filepath.Join(folderPath, entry.Name())),
				HasNFO:     s.findNFOForFile(filepath.Join(folderPath, entry.Name())) != "",
			})
		}
	}

	return items
}

// scanEmbyMovieItem 扫描单个 EMBY 电影项
func (s *EmbyCompatService) scanEmbyMovieItem(itemPath, dirName string) *EmbyScannedItem {
	entries, err := os.ReadDir(itemPath)
	if err != nil {
		return nil
	}

	item := &EmbyScannedItem{
		Path:          itemPath,
		MediaType:     "movie",
		VideoFiles:    make([]string, 0),
		SubtitleFiles: make([]string, 0),
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		fullPath := filepath.Join(itemPath, name)

		switch {
		case isVideoExt(ext):
			item.VideoFiles = append(item.VideoFiles, fullPath)
		case ext == ".nfo":
			item.NFOFile = fullPath
			item.HasNFO = true
		case isSubtitleExt(ext):
			item.SubtitleFiles = append(item.SubtitleFiles, fullPath)
		case isImageExt(ext):
			nameLower := strings.ToLower(strings.TrimSuffix(name, ext))
			if strings.Contains(nameLower, "poster") || strings.Contains(nameLower, "cover") ||
				strings.Contains(nameLower, "folder") || strings.Contains(nameLower, "thumb") {
				item.PosterFile = fullPath
			} else if strings.Contains(nameLower, "fanart") || strings.Contains(nameLower, "backdrop") ||
				strings.Contains(nameLower, "banner") || strings.Contains(nameLower, "background") {
				item.BackdropFile = fullPath
			} else if item.PosterFile == "" {
				item.PosterFile = fullPath
			}
		}
	}

	if len(item.VideoFiles) == 0 {
		return nil
	}

	// 解析标题和年份
	item.Title, item.Year = parseEmbyMovieName(dirName)

	return item
}

// scanEmbyTVShowFolder 扫描 EMBY 电视剧文件夹
func (s *EmbyCompatService) scanEmbyTVShowFolder(folderPath string) []EmbyScannedItem {
	var items []EmbyScannedItem

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		s.logger.Warnf("读取电视剧目录失败: %s, %v", folderPath, err)
		return items
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item := s.scanEmbyTVShowItem(filepath.Join(folderPath, entry.Name()), entry.Name())
		if item != nil {
			items = append(items, *item)
		}
	}

	return items
}

// scanEmbyTVShowItem 扫描单个 EMBY 电视剧项
func (s *EmbyCompatService) scanEmbyTVShowItem(itemPath, dirName string) *EmbyScannedItem {
	entries, err := os.ReadDir(itemPath)
	if err != nil {
		return nil
	}

	item := &EmbyScannedItem{
		Path:          itemPath,
		MediaType:     "tvshow",
		VideoFiles:    make([]string, 0),
		SubtitleFiles: make([]string, 0),
		Seasons:       make([]EmbyScannedSeason, 0),
	}

	// 查找 tvshow.nfo
	tvshowNFO := filepath.Join(itemPath, "tvshow.nfo")
	if _, err := os.Stat(tvshowNFO); err == nil {
		item.NFOFile = tvshowNFO
		item.HasNFO = true
	}

	// 查找海报和背景图
	poster, backdrop := s.nfoService.FindLocalImages(itemPath)
	item.PosterFile = poster
	item.BackdropFile = backdrop

	seasonRegex := regexp.MustCompile(`(?i)^season\s*(\d+)$|^第(\d+)季$|^S(\d+)$|^Specials$`)

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() {
			// 检查是否为季目录
			matches := seasonRegex.FindStringSubmatch(name)
			if matches != nil {
				seasonNum := 0
				for _, m := range matches[1:] {
					if m != "" {
						seasonNum, _ = strconv.Atoi(m)
						break
					}
				}
				if strings.EqualFold(name, "Specials") {
					seasonNum = 0
				}

				season := s.scanEmbySeasonFolder(filepath.Join(itemPath, name), seasonNum)
				item.Seasons = append(item.Seasons, season)
			} else {
				// 非标准季目录，可能直接包含视频文件
				subEntries, _ := os.ReadDir(filepath.Join(itemPath, name))
				for _, se := range subEntries {
					ext := strings.ToLower(filepath.Ext(se.Name()))
					if isVideoExt(ext) {
						item.VideoFiles = append(item.VideoFiles, filepath.Join(itemPath, name, se.Name()))
					}
				}
			}
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if isVideoExt(ext) {
				item.VideoFiles = append(item.VideoFiles, filepath.Join(itemPath, name))
			} else if isSubtitleExt(ext) {
				item.SubtitleFiles = append(item.SubtitleFiles, filepath.Join(itemPath, name))
			}
		}
	}

	// 排序季
	sort.Slice(item.Seasons, func(i, j int) bool {
		return item.Seasons[i].SeasonNum < item.Seasons[j].SeasonNum
	})

	// 如果没有视频文件也没有季，则不是有效的电视剧
	if len(item.VideoFiles) == 0 && len(item.Seasons) == 0 {
		return nil
	}

	// 解析标题和年份
	item.Title, item.Year = parseEmbyMovieName(dirName)

	return item
}

// scanEmbySeasonFolder 扫描季目录
func (s *EmbyCompatService) scanEmbySeasonFolder(seasonPath string, seasonNum int) EmbyScannedSeason {
	season := EmbyScannedSeason{
		SeasonNum:  seasonNum,
		Path:       seasonPath,
		VideoFiles: make([]string, 0),
	}

	entries, err := os.ReadDir(seasonPath)
	if err != nil {
		return season
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if isVideoExt(ext) {
			season.VideoFiles = append(season.VideoFiles, filepath.Join(seasonPath, entry.Name()))
		}
	}

	season.Episodes = len(season.VideoFiles)
	return season
}

// detectEmbyItemType 智能检测子目录是电影还是电视剧
func (s *EmbyCompatService) detectEmbyItemType(itemPath, dirName string) string {
	entries, err := os.ReadDir(itemPath)
	if err != nil {
		return ""
	}

	var videoCount int
	var hasSeasonDir bool
	var hasTVShowNFO bool
	var hasEpisodePattern bool

	seasonRegex := regexp.MustCompile(`(?i)^season\s*\d+$|^第\d+季$|^S\d+$|^Specials$`)
	episodeRegex := regexp.MustCompile(`(?i)S\d{1,2}E\d{1,2}|第\d+集|EP?\d{2,}|\d+x\d+`)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			if seasonRegex.MatchString(name) {
				hasSeasonDir = true
			}
		} else {
			ext := strings.ToLower(filepath.Ext(name))
			if isVideoExt(ext) {
				videoCount++
				if episodeRegex.MatchString(name) {
					hasEpisodePattern = true
				}
			}
			if strings.EqualFold(name, "tvshow.nfo") {
				hasTVShowNFO = true
			}
		}
	}

	if hasTVShowNFO || hasSeasonDir {
		return "tvshow"
	}
	if hasEpisodePattern || videoCount > 3 {
		return "tvshow"
	}
	if videoCount > 0 {
		return "movie"
	}
	return ""
}

// ==================== 内部方法：导入逻辑 ====================

// importEmbyMovie 导入单个 EMBY 电影
func (s *EmbyCompatService) importEmbyMovie(item EmbyScannedItem, req EmbyImportRequest) (bool, error) {
	if len(item.VideoFiles) == 0 {
		return false, fmt.Errorf("没有视频文件")
	}

	// 取第一个视频文件作为主文件
	videoFile := item.VideoFiles[0]

	// 检查是否已导入
	if _, err := s.mediaRepo.FindByFilePath(videoFile); err == nil {
		return false, nil // 已存在，跳过
	}

	// 获取文件信息
	fileInfo, err := os.Stat(videoFile)
	if err != nil {
		return false, fmt.Errorf("无法读取文件: %w", err)
	}

	media := &model.Media{
		LibraryID: req.TargetLibraryID,
		Title:     item.Title,
		Year:      item.Year,
		FilePath:  videoFile,
		FileSize:  fileInfo.Size(),
		MediaType: "movie",
	}

	// 解析 NFO 元数据
	if req.ImportNFO && item.HasNFO && item.NFOFile != "" {
		if err := s.parseEmbyMovieNFO(item.NFOFile, media); err != nil {
			s.logger.Debugf("解析 EMBY NFO 失败: %s, %v", item.NFOFile, err)
		}
	}

	// 映射图片
	if req.ImportImages {
		if item.PosterFile != "" {
			media.PosterPath = item.PosterFile
		}
		if item.BackdropFile != "" {
			media.BackdropPath = item.BackdropFile
		}
	}

	// 映射字幕
	if len(item.SubtitleFiles) > 0 {
		media.SubtitlePaths = strings.Join(item.SubtitleFiles, "|")
	}

	// 使用 FFprobe 探测媒体信息
	s.scanner.ProbeMediaInfo(media)

	if err := s.mediaRepo.Create(media); err != nil {
		return false, fmt.Errorf("保存媒体失败: %w", err)
	}

	return true, nil
}

// importEmbyTVShow 导入单个 EMBY 电视剧
func (s *EmbyCompatService) importEmbyTVShow(item EmbyScannedItem, req EmbyImportRequest) (int, error) {
	// 查找或创建 Series
	series, err := s.seriesRepo.FindByFolderPath(item.Path)
	if err != nil {
		// 创建新的 Series
		series = &model.Series{
			LibraryID:  req.TargetLibraryID,
			Title:      item.Title,
			Year:       item.Year,
			FolderPath: item.Path,
		}

		// 解析 tvshow.nfo
		if req.ImportNFO && item.HasNFO && item.NFOFile != "" {
			if err := s.nfoService.ParseTVShowNFO(item.NFOFile, series); err != nil {
				s.logger.Debugf("解析 tvshow.nfo 失败: %s, %v", item.NFOFile, err)
			}
		}

		// 映射图片
		if req.ImportImages {
			if item.PosterFile != "" {
				series.PosterPath = item.PosterFile
			}
			if item.BackdropFile != "" {
				series.BackdropPath = item.BackdropFile
			}
		}

		if err := s.seriesRepo.Create(series); err != nil {
			return 0, fmt.Errorf("创建剧集合集失败: %w", err)
		}
	}

	importedCount := 0

	// 导入各季的剧集
	for _, season := range item.Seasons {
		for _, videoFile := range season.VideoFiles {
			if _, err := s.mediaRepo.FindByFilePath(videoFile); err == nil {
				continue // 已存在
			}

			fileInfo, err := os.Stat(videoFile)
			if err != nil {
				continue
			}

			episodeTitle, seasonNum, episodeNum := parseEmbyEpisodeName(filepath.Base(videoFile))
			if seasonNum == 0 {
				seasonNum = season.SeasonNum
			}

			media := &model.Media{
				LibraryID:    req.TargetLibraryID,
				SeriesID:     series.ID,
				Title:        series.Title,
				EpisodeTitle: episodeTitle,
				SeasonNum:    seasonNum,
				EpisodeNum:   episodeNum,
				FilePath:     videoFile,
				FileSize:     fileInfo.Size(),
				MediaType:    "episode",
			}

			// 解析单集 NFO
			if req.ImportNFO {
				episodeNFO := s.findNFOForFile(videoFile)
				if episodeNFO != "" {
					s.parseEmbyEpisodeNFO(episodeNFO, media)
				}
			}

			// 扫描外挂字幕
			s.scanSubtitlesForFile(media, videoFile)

			s.scanner.ProbeMediaInfo(media)

			if err := s.mediaRepo.Create(media); err != nil {
				s.logger.Warnf("保存剧集失败: %s, %v", videoFile, err)
				continue
			}
			importedCount++
		}
	}

	// 导入根目录下的散落视频文件（作为未分季的剧集）
	for _, videoFile := range item.VideoFiles {
		if _, err := s.mediaRepo.FindByFilePath(videoFile); err == nil {
			continue
		}

		fileInfo, err := os.Stat(videoFile)
		if err != nil {
			continue
		}

		episodeTitle, seasonNum, episodeNum := parseEmbyEpisodeName(filepath.Base(videoFile))

		media := &model.Media{
			LibraryID:    req.TargetLibraryID,
			SeriesID:     series.ID,
			Title:        series.Title,
			EpisodeTitle: episodeTitle,
			SeasonNum:    seasonNum,
			EpisodeNum:   episodeNum,
			FilePath:     videoFile,
			FileSize:     fileInfo.Size(),
			MediaType:    "episode",
		}

		if req.ImportNFO {
			episodeNFO := s.findNFOForFile(videoFile)
			if episodeNFO != "" {
				s.parseEmbyEpisodeNFO(episodeNFO, media)
			}
		}

		s.scanSubtitlesForFile(media, videoFile)
		s.scanner.ProbeMediaInfo(media)

		if err := s.mediaRepo.Create(media); err != nil {
			s.logger.Warnf("保存剧集失败: %s, %v", videoFile, err)
			continue
		}
		importedCount++
	}

	// 更新 Series 的季数和集数
	if importedCount > 0 {
		s.updateSeriesCounts(series.ID)
	}

	return importedCount, nil
}

// ==================== 内部方法：NFO 解析 ====================

// parseEmbyMovieNFO 解析 EMBY 电影 NFO（增强版）
func (s *EmbyCompatService) parseEmbyMovieNFO(nfoPath string, media *model.Media) error {
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return fmt.Errorf("读取 NFO 失败: %w", err)
	}

	// 先尝试 EMBY 增强格式
	var embyNFO EmbyMovieNFO
	if err := xml.Unmarshal(data, &embyNFO); err == nil && embyNFO.Title != "" {
		s.applyEmbyMovieNFO(media, &embyNFO)
		return nil
	}

	// 回退到标准 NFO 格式
	return s.nfoService.ParseMovieNFO(nfoPath, media)
}

// applyEmbyMovieNFO 应用 EMBY 电影 NFO 数据
func (s *EmbyCompatService) applyEmbyMovieNFO(media *model.Media, nfo *EmbyMovieNFO) {
	if nfo.Title != "" {
		media.Title = nfo.Title
	}
	if nfo.OrigTitle != "" {
		media.OrigTitle = nfo.OrigTitle
	}
	if nfo.Year > 0 {
		media.Year = nfo.Year
	}
	if nfo.Plot != "" {
		media.Overview = nfo.Plot
	} else if nfo.Outline != "" {
		media.Overview = nfo.Outline
	}
	if nfo.Rating > 0 {
		media.Rating = nfo.Rating
	}
	if nfo.Runtime > 0 {
		media.Runtime = nfo.Runtime
	}
	if len(nfo.Genres) > 0 {
		media.Genres = strings.Join(nfo.Genres, ",")
	}
	if nfo.Tagline != "" {
		media.Tagline = nfo.Tagline
	}
	if nfo.Studio != "" {
		media.Studio = nfo.Studio
	}
	if nfo.Country != "" {
		media.Country = nfo.Country
	}
	if nfo.Language != "" {
		media.Language = nfo.Language
	}
	if nfo.TMDbID > 0 {
		media.TMDbID = nfo.TMDbID
	}
	if nfo.DoubanID != "" {
		media.DoubanID = nfo.DoubanID
	}

	// EMBY 扩展：从 fileinfo 获取视频信息
	if nfo.FileInfo != nil && nfo.FileInfo.StreamDetails != nil {
		sd := nfo.FileInfo.StreamDetails
		if sd.Video != nil {
			if sd.Video.Codec != "" {
				media.VideoCodec = sd.Video.Codec
			}
			if sd.Video.Width > 0 && sd.Video.Height > 0 {
				media.Resolution = detectResolutionFromDimensions(sd.Video.Width, sd.Video.Height)
			}
			if sd.Video.DurationInSeconds > 0 {
				media.Duration = sd.Video.DurationInSeconds
			}
		}
		if len(sd.Audio) > 0 {
			media.AudioCodec = sd.Audio[0].Codec
		}
	}

	// EMBY 扩展：从 thumb 获取海报
	for _, thumb := range nfo.Thumb {
		if thumb.Aspect == "poster" && thumb.URL != "" && media.PosterPath == "" {
			media.PosterPath = thumb.URL
		}
	}
	if nfo.Fanart != nil {
		for _, thumb := range nfo.Fanart.Thumbs {
			if thumb.URL != "" && media.BackdropPath == "" {
				media.BackdropPath = thumb.URL
			}
		}
	}
}

// parseEmbyEpisodeNFO 解析 EMBY 单集 NFO
func (s *EmbyCompatService) parseEmbyEpisodeNFO(nfoPath string, media *model.Media) {
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return
	}

	var nfo EmbyEpisodeNFO
	if err := xml.Unmarshal(data, &nfo); err != nil {
		// 回退到标准解析
		s.nfoService.ParseMovieNFO(nfoPath, media)
		return
	}

	if nfo.Title != "" {
		media.EpisodeTitle = nfo.Title
	}
	if nfo.Season > 0 {
		media.SeasonNum = nfo.Season
	}
	if nfo.Episode > 0 {
		media.EpisodeNum = nfo.Episode
	}
	if nfo.Plot != "" {
		media.Overview = nfo.Plot
	}
	if nfo.Rating > 0 {
		media.Rating = nfo.Rating
	}
}

// ==================== 内部方法：辅助函数 ====================

// findNFOForFile 查找视频文件对应的 NFO
func (s *EmbyCompatService) findNFOForFile(videoPath string) string {
	return s.nfoService.FindNFOForMedia(videoPath)
}

// scanSubtitlesForFile 扫描视频文件的外挂字幕
func (s *EmbyCompatService) scanSubtitlesForFile(media *model.Media, videoPath string) {
	dir := filepath.Dir(videoPath)
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var subtitles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !isSubtitleExt(ext) {
			continue
		}
		// 匹配同名字幕
		if strings.HasPrefix(strings.TrimSuffix(name, ext), baseName) {
			subtitles = append(subtitles, filepath.Join(dir, name))
		}
	}

	if len(subtitles) > 0 {
		media.SubtitlePaths = strings.Join(subtitles, "|")
	}
}

// markImportedItems 标记已导入的项
func (s *EmbyCompatService) markImportedItems(result *EmbyDetectResult) {
	for i, movie := range result.Movies {
		for _, vf := range movie.VideoFiles {
			if _, err := s.mediaRepo.FindByFilePath(vf); err == nil {
				result.Movies[i].Imported = true
				break
			}
		}
	}
	for i, tvshow := range result.TVShows {
		if _, err := s.seriesRepo.FindByFolderPath(tvshow.Path); err == nil {
			result.TVShows[i].Imported = true
		}
	}
}

// updateSeriesCounts 更新剧集合集的季数和集数
func (s *EmbyCompatService) updateSeriesCounts(seriesID string) {
	series, err := s.seriesRepo.FindByIDOnly(seriesID)
	if err != nil {
		return
	}

	// 查询该合集下的所有剧集
	episodes, err := s.mediaRepo.ListBySeriesID(seriesID)
	if err != nil {
		return
	}

	seasonSet := make(map[int]bool)
	for _, ep := range episodes {
		if ep.SeasonNum > 0 {
			seasonSet[ep.SeasonNum] = true
		}
	}

	series.SeasonCount = len(seasonSet)
	series.EpisodeCount = len(episodes)
	s.seriesRepo.Update(series)
}

// broadcastProgress 广播导入进度
func (s *EmbyCompatService) broadcastProgress(phase string, current, total int, currentItem, message string) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent("emby_import_progress", &EmbyImportProgress{
			Phase:       phase,
			Current:     current,
			Total:       total,
			CurrentItem: currentItem,
			Message:     message,
		})
	}
}

// ==================== 工具函数 ====================

// parseEmbyMovieName 解析 EMBY 电影文件夹名称，提取标题和年份
// 支持格式: "Movie Name (2023)", "Movie.Name.2023", "Movie Name [2023]"
func parseEmbyMovieName(name string) (title string, year int) {
	// 尝试匹配 "Title (Year)" 或 "Title [Year]"
	yearRegex := regexp.MustCompile(`^(.+?)\s*[\(\[](\d{4})[\)\]]\s*$`)
	if matches := yearRegex.FindStringSubmatch(name); matches != nil {
		title = strings.TrimSpace(matches[1])
		year, _ = strconv.Atoi(matches[2])
		return
	}

	// 尝试匹配末尾的年份 "Title.2023" 或 "Title 2023"
	yearSuffixRegex := regexp.MustCompile(`^(.+?)[.\s](\d{4})$`)
	if matches := yearSuffixRegex.FindStringSubmatch(name); matches != nil {
		title = strings.TrimSpace(matches[1])
		year, _ = strconv.Atoi(matches[2])
		// 清理标题中的点号
		title = strings.ReplaceAll(title, ".", " ")
		title = strings.TrimSpace(title)
		return
	}

	// 无年份，直接使用名称
	title = strings.ReplaceAll(name, ".", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = regexp.MustCompile(`\s+`).ReplaceAllString(title, " ")
	title = strings.TrimSpace(title)
	return
}

// parseEmbyEpisodeName 解析 EMBY 剧集文件名，提取标题、季号、集号
// 支持格式: "S01E02 - Title.mkv", "1x02.mkv", "EP02.mkv", "第02集.mkv"
func parseEmbyEpisodeName(filename string) (title string, seasonNum, episodeNum int) {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// S01E02 格式
	seRegex := regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,3})`)
	if matches := seRegex.FindStringSubmatch(name); matches != nil {
		seasonNum, _ = strconv.Atoi(matches[1])
		episodeNum, _ = strconv.Atoi(matches[2])
		// 提取标题（S01E02 后面的部分）
		idx := seRegex.FindStringIndex(name)
		if idx != nil && idx[1] < len(name) {
			title = strings.TrimSpace(name[idx[1]:])
			title = strings.TrimLeft(title, " -._")
		}
		return
	}

	// 1x02 格式
	xRegex := regexp.MustCompile(`(\d{1,2})x(\d{1,3})`)
	if matches := xRegex.FindStringSubmatch(name); matches != nil {
		seasonNum, _ = strconv.Atoi(matches[1])
		episodeNum, _ = strconv.Atoi(matches[2])
		return
	}

	// EP02 格式
	epRegex := regexp.MustCompile(`(?i)EP?(\d{2,3})`)
	if matches := epRegex.FindStringSubmatch(name); matches != nil {
		episodeNum, _ = strconv.Atoi(matches[1])
		seasonNum = 1
		return
	}

	// 第02集 格式
	cnRegex := regexp.MustCompile(`第(\d+)集`)
	if matches := cnRegex.FindStringSubmatch(name); matches != nil {
		episodeNum, _ = strconv.Atoi(matches[1])
		seasonNum = 1
		return
	}

	// 纯数字集号
	numRegex := regexp.MustCompile(`\b(\d{2,3})\b`)
	if matches := numRegex.FindStringSubmatch(name); matches != nil {
		episodeNum, _ = strconv.Atoi(matches[1])
		seasonNum = 1
	}

	return
}

// detectResolutionFromDimensions 根据宽高判断分辨率
func detectResolutionFromDimensions(width, height int) string {
	switch {
	case width >= 3840 || height >= 2160:
		return "4K"
	case width >= 1920 || height >= 1080:
		return "1080p"
	case width >= 1280 || height >= 720:
		return "720p"
	case width >= 720 || height >= 480:
		return "480p"
	default:
		return fmt.Sprintf("%dx%d", width, height)
	}
}

// isVideoExt 判断是否为视频扩展名
func isVideoExt(ext string) bool {
	videoExts := map[string]bool{
		".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true, ".ts": true,
		".rmvb": true, ".rm": true, ".3gp": true, ".mpg": true, ".mpeg": true,
		".strm": true, // STRM 远程流文件
	}
	return videoExts[ext]
}

// isImageExt 判断是否为图片扩展名
func isImageExt(ext string) bool {
	imageExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
		".bmp": true, ".gif": true, ".tiff": true,
	}
	return imageExts[ext]
}

// isSubtitleExt 判断是否为字幕扩展名
func isSubtitleExt(ext string) bool {
	subtitleExts := map[string]bool{
		".srt": true, ".ass": true, ".ssa": true, ".vtt": true,
		".sub": true, ".idx": true, ".sup": true,
	}
	return subtitleExts[ext]
}

// GenerateEmbyNFO
// GenerateEmbyNFO 生成 EMBY 兼容的 NFO 文件（用于导出）
func (s *EmbyCompatService) GenerateEmbyNFO(media *model.Media) ([]byte, error) {
	nfo := EmbyMovieNFO{
		Title:     media.Title,
		OrigTitle: media.OrigTitle,
		Year:      media.Year,
		Plot:      media.Overview,
		Tagline:   media.Tagline,
		Rating:    media.Rating,
		Runtime:   media.Runtime,
		Studio:    media.Studio,
		Country:   media.Country,
		Language:  media.Language,
		TMDbID:    media.TMDbID,
		DoubanID:  media.DoubanID,
	}

	if media.Genres != "" {
		nfo.Genres = strings.Split(media.Genres, ",")
	}

	output, err := xml.MarshalIndent(nfo, "", "  ")
	if err != nil {
		return nil, err
	}

	return append([]byte(xml.Header), output...), nil
}

// ExportToEmbyFormat 将媒体库导出为 EMBY 兼容格式（生成 NFO + 图片结构）
func (s *EmbyCompatService) ExportToEmbyFormat(libraryID, outputPath string) error {
	// 此功能为未来扩展预留
	return fmt.Errorf("导出功能暂未实现")
}

// GetEmbyCompatInfo 获取 EMBY 兼容性信息
func (s *EmbyCompatService) GetEmbyCompatInfo() map[string]interface{} {
	return map[string]interface{}{
		"supported_nfo_formats": []string{"movie", "tvshow", "episodedetails"},
		"supported_image_names": []string{
			"poster.jpg", "cover.jpg", "folder.jpg", "thumb.jpg",
			"fanart.jpg", "backdrop.jpg", "banner.jpg", "background.jpg",
		},
		"supported_folder_structures": []string{
			"Movies/<Movie Name> (Year)/movie.mkv",
			"TV Shows/<Show Name>/Season 01/S01E01.mkv",
			"TV Shows/<Show Name>/tvshow.nfo",
		},
		"supported_naming_conventions": []string{
			"<Title> (Year)", "<Title> [Year]", "<Title>.Year",
			"S01E02", "1x02", "EP02", "第02集",
		},
		"version": "1.0",
	}
}
