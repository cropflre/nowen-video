package service

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

// NFOService NFO 本地元数据解析服务
// 支持 Kodi / Emby / Jellyfin 风格的 NFO XML 文件
//
// V2.1: 支持 webdav:// 前缀路径，通过 VFSManager 读取远程 NFO 与图片
type NFOService struct {
	logger *zap.SugaredLogger

	// V2.1: VFS 管理器（可选，nil 时纯本地模式）
	vfsMgr *VFSManager
}

func NewNFOService(logger *zap.SugaredLogger) *NFOService {
	return &NFOService{logger: logger}
}

// SetVFSManager 注入 VFS 管理器（V2.1，用于 webdav:// NFO 支持）
func (s *NFOService) SetVFSManager(vfsMgr *VFSManager) {
	s.vfsMgr = vfsMgr
}

// ==================== VFS 辅助方法 ====================

// readFile 读取文件（支持 webdav://）
func (s *NFOService) readFile(p string) ([]byte, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		f, err := s.vfsMgr.Open(p)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return io.ReadAll(f)
	}
	return os.ReadFile(p)
}

// statPath 获取文件信息（支持 webdav://）
func (s *NFOService) statPath(p string) (os.FileInfo, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		return s.vfsMgr.Stat(p)
	}
	return os.Stat(p)
}

// readDir 读取目录（支持 webdav://）
func (s *NFOService) readDir(p string) ([]os.DirEntry, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		entries, err := s.vfsMgr.ReadDir(p)
		if err != nil {
			return nil, err
		}
		result := make([]os.DirEntry, len(entries))
		copy(result, entries)
		return result, nil
	}
	return os.ReadDir(p)
}

// joinPath 路径拼接（webdav:// 用正斜杠）
func (s *NFOService) joinPath(base, name string) string {
	if IsWebDAVPath(base) {
		base = strings.TrimRight(base, "/")
		return base + "/" + strings.TrimLeft(name, "/")
	}
	return filepath.Join(base, name)
}

// dirOf 提取目录（webdav:// 用正斜杠规则）
func (s *NFOService) dirOf(p string) string {
	if IsWebDAVPath(p) {
		i := strings.LastIndex(p, "/")
		if i <= len("webdav:/") {
			return p
		}
		return p[:i]
	}
	return filepath.Dir(p)
}

// ==================== NFO XML 结构体 ====================

// NFOMovie 电影 NFO XML 根元素
type NFOMovie struct {
	XMLName   xml.Name   `xml:"movie"`
	Title     string     `xml:"title"`
	OrigTitle string     `xml:"originaltitle"`
	Year      int        `xml:"year"`
	Premiered string     `xml:"premiered"`
	Plot      string     `xml:"plot"`
	Tagline   string     `xml:"tagline"`
	Rating    float64    `xml:"rating"`
	Runtime   int        `xml:"runtime"`
	Studio    string     `xml:"studio"`
	Country   string     `xml:"country"`
	TMDbID    int        `xml:"tmdbid"`
	DoubanID  string     `xml:"doubanid"`
	Genres    []string   `xml:"genre"`
	Directors []string   `xml:"director"`
	Actors    []NFOActor `xml:"actor"`
}

// NFOTVShow 剧集 NFO XML 根元素
type NFOTVShow struct {
	XMLName   xml.Name   `xml:"tvshow"`
	Title     string     `xml:"title"`
	OrigTitle string     `xml:"originaltitle"`
	Year      int        `xml:"year"`
	Plot      string     `xml:"plot"`
	Rating    float64    `xml:"rating"`
	Studio    string     `xml:"studio"`
	Country   string     `xml:"country"`
	TMDbID    int        `xml:"tmdbid"`
	DoubanID  string     `xml:"doubanid"`
	Genres    []string   `xml:"genre"`
	Directors []string   `xml:"director"`
	Actors    []NFOActor `xml:"actor"`
}

// NFOActor NFO 演员信息
type NFOActor struct {
	Name      string `xml:"name"`
	Role      string `xml:"role"`
	Thumb     string `xml:"thumb"`
	SortOrder int    `xml:"sortorder"`
}

// ==================== 解析方法 ====================

// ParseMovieNFO 解析电影 NFO 文件并将数据应用到 Media 对象
func (s *NFOService) ParseMovieNFO(nfoPath string, media *model.Media) error {
	data, err := s.readFile(nfoPath)
	if err != nil {
		return fmt.Errorf("读取NFO文件失败: %w", err)
	}

	var nfo NFOMovie
	if err := xml.Unmarshal(data, &nfo); err != nil {
		// 尝试作为 tvshow 解析
		var tvNFO NFOTVShow
		if err2 := xml.Unmarshal(data, &tvNFO); err2 != nil {
			return fmt.Errorf("解析NFO XML失败: %w", err)
		}
		// 如果是 tvshow 格式，转换后应用
		s.applyTVShowNFOToMedia(media, &tvNFO)
		return nil
	}

	s.applyMovieNFOToMedia(media, &nfo)
	return nil
}

// ParseTVShowNFO 解析剧集 NFO 文件并将数据应用到 Series 对象
func (s *NFOService) ParseTVShowNFO(nfoPath string, series *model.Series) error {
	data, err := s.readFile(nfoPath)
	if err != nil {
		return fmt.Errorf("读取NFO文件失败: %w", err)
	}

	var nfo NFOTVShow
	if err := xml.Unmarshal(data, &nfo); err != nil {
		return fmt.Errorf("解析NFO XML失败: %w", err)
	}

	s.applyTVShowNFOToSeries(series, &nfo)
	return nil
}

// GetActorsFromNFO 从 NFO 文件中提取演员列表
func (s *NFOService) GetActorsFromNFO(nfoPath string) ([]NFOActor, []string, error) {
	data, err := s.readFile(nfoPath)
	if err != nil {
		return nil, nil, err
	}

	// 先尝试 movie
	var movie NFOMovie
	if err := xml.Unmarshal(data, &movie); err == nil && movie.Title != "" {
		return movie.Actors, movie.Directors, nil
	}

	// 再尝试 tvshow
	var tvshow NFOTVShow
	if err := xml.Unmarshal(data, &tvshow); err == nil && tvshow.Title != "" {
		return tvshow.Actors, tvshow.Directors, nil
	}

	return nil, nil, fmt.Errorf("无法解析NFO文件")
}

// ==================== 本地图片扫描 ====================

// 常见视频文件扩展名
var nfoVideoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
	".wmv": true, ".flv": true, ".webm": true, ".m4v": true, ".ts": true,
	".rmvb": true, ".rm": true, ".3gp": true, ".mpg": true, ".mpeg": true,
	".strm": true,
}

// 常见图片文件扩展名
var nfoImageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// 常见本地海报文件名（按优先级排序）
var standardPosterNames = []string{
	"poster.jpg", "poster.png", "poster.webp",
	"cover.jpg", "cover.png", "cover.webp",
	"folder.jpg", "folder.png", "folder.webp",
	"thumb.jpg", "thumb.png", "thumb.webp",
	"movie.jpg", "movie.png",
	"show.jpg", "show.png",
}

// 常见本地背景图文件名
var standardBackdropNames = []string{
	"fanart.jpg", "fanart.png", "fanart.webp",
	"backdrop.jpg", "backdrop.png", "backdrop.webp",
	"banner.jpg", "banner.png", "banner.webp",
	"background.jpg", "background.png", "background.webp",
	"clearart.jpg", "clearart.png",
	"landscape.jpg", "landscape.png",
}

// FindLocalImages 在指定目录下查找本地图片（poster/fanart/banner 等）
// 适用于剧集/合集等场景，不区分具体视频文件
// 支持 jpg、png、webp 等常见图片格式
func (s *NFOService) FindLocalImages(dir string) (poster, backdrop string) {
	for _, name := range standardPosterNames {
		path := s.joinPath(dir, name)
		if _, err := s.statPath(path); err == nil {
			poster = path
			break
		}
	}

	for _, name := range standardBackdropNames {
		path := s.joinPath(dir, name)
		if _, err := s.statPath(path); err == nil {
			backdrop = path
			break
		}
	}

	// 如果没有找到标准命名的海报，尝试查找目录中的第一张图片作为海报
	if poster == "" {
		entries, err := s.readDir(dir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() {
					ext := strings.ToLower(filepath.Ext(entry.Name()))
					if nfoImageExts[ext] {
						// 排除已识别为backdrop的文件
						candidate := s.joinPath(dir, entry.Name())
						if candidate != backdrop {
							poster = candidate
							break
						}
					}
				}
			}
		}
	}

	return poster, backdrop
}

// FindLocalImagesForMedia 根据媒体文件路径查找对应的本地图片
// 方案 C：优先匹配与视频同名的图片，当目录下只有一个视频文件时才使用通用封面
// 解决多部影片在同一目录下共用同一张封面的问题
func (s *NFOService) FindLocalImagesForMedia(mediaFilePath string) (poster, backdrop string) {
	dir := s.dirOf(mediaFilePath)
	baseName := strings.TrimSuffix(filepath.Base(mediaFilePath), filepath.Ext(mediaFilePath))

	// === 阶段1：优先查找与视频文件同名的图片 ===
	posterSuffixes := []string{
		"-poster.jpg", "-poster.png", "-poster.webp",
		"-cover.jpg", "-cover.png", "-cover.webp",
		"-thumb.jpg", "-thumb.png", "-thumb.webp",
		".jpg", ".png", ".webp",
	}
	backdropSuffixes := []string{
		"-fanart.jpg", "-fanart.png", "-fanart.webp",
		"-backdrop.jpg", "-backdrop.png", "-backdrop.webp",
		"-banner.jpg", "-banner.png", "-banner.webp",
	}

	for _, suffix := range posterSuffixes {
		path := s.joinPath(dir, baseName+suffix)
		if _, err := s.statPath(path); err == nil {
			poster = path
			break
		}
	}

	for _, suffix := range backdropSuffixes {
		path := s.joinPath(dir, baseName+suffix)
		if _, err := s.statPath(path); err == nil {
			backdrop = path
			break
		}
	}

	// 如果已经找到同名图片，直接返回
	if poster != "" {
		return poster, backdrop
	}

	// === 阶段2：统计目录中的视频文件数量 ===
	entries, err := s.readDir(dir)
	if err != nil {
		return poster, backdrop
	}

	videoCount := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if nfoVideoExts[ext] {
				videoCount++
				if videoCount > 1 {
					break
				}
			}
		}
	}

	// === 阶段3：根据视频文件数量决定是否使用通用封面 ===
	if videoCount <= 1 {
		for _, name := range standardPosterNames {
			path := s.joinPath(dir, name)
			if _, err := s.statPath(path); err == nil {
				poster = path
				break
			}
		}

		for _, name := range standardBackdropNames {
			path := s.joinPath(dir, name)
			if _, err := s.statPath(path); err == nil {
				backdrop = path
				break
			}
		}

		// 兜底：目录中第一张图片
		if poster == "" {
			for _, entry := range entries {
				if !entry.IsDir() {
					ext := strings.ToLower(filepath.Ext(entry.Name()))
					if nfoImageExts[ext] {
						candidate := s.joinPath(dir, entry.Name())
						if candidate != backdrop {
							poster = candidate
							break
						}
					}
				}
			}
		}
	} else {
		s.logger.Debugf("目录 %s 下有 %d 个视频文件，跳过通用封面分配", dir, videoCount)
	}

	return poster, backdrop
}

// FindNFOFile 在指定目录下查找 NFO 文件
func (s *NFOService) FindNFOFile(dir string) string {
	entries, err := s.readDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".nfo") {
			return s.joinPath(dir, entry.Name())
		}
	}
	return ""
}

// FindNFOForMedia 根据媒体文件路径查找关联的 NFO 文件
func (s *NFOService) FindNFOForMedia(mediaFilePath string) string {
	// 策略1: 同名 .nfo 文件
	ext := filepath.Ext(mediaFilePath)
	nfoPath := strings.TrimSuffix(mediaFilePath, ext) + ".nfo"
	if _, err := s.statPath(nfoPath); err == nil {
		return nfoPath
	}

	// 策略2: 目录下任意 .nfo 文件
	dir := s.dirOf(mediaFilePath)
	return s.FindNFOFile(dir)
}

// ==================== 应用 NFO 数据 ====================

func (s *NFOService) applyMovieNFOToMedia(media *model.Media, nfo *NFOMovie) {
	if nfo.Title != "" {
		media.Title = nfo.Title
	}
	if nfo.OrigTitle != "" {
		media.OrigTitle = nfo.OrigTitle
	}
	if nfo.Year > 0 {
		media.Year = nfo.Year
	}
	if nfo.Premiered != "" {
		media.Premiered = nfo.Premiered
	}
	if nfo.Plot != "" {
		media.Overview = nfo.Plot
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
	if nfo.TMDbID > 0 {
		media.TMDbID = nfo.TMDbID
	}
	if nfo.DoubanID != "" {
		media.DoubanID = nfo.DoubanID
	}
}

func (s *NFOService) applyTVShowNFOToMedia(media *model.Media, nfo *NFOTVShow) {
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
	}
	if nfo.Rating > 0 {
		media.Rating = nfo.Rating
	}
	if len(nfo.Genres) > 0 {
		media.Genres = strings.Join(nfo.Genres, ",")
	}
	if nfo.Country != "" {
		media.Country = nfo.Country
	}
}

func (s *NFOService) applyTVShowNFOToSeries(series *model.Series, nfo *NFOTVShow) {
	if nfo.Title != "" {
		series.Title = nfo.Title
	}
	if nfo.OrigTitle != "" {
		series.OrigTitle = nfo.OrigTitle
	}
	if nfo.Year > 0 {
		series.Year = nfo.Year
	}
	if nfo.Plot != "" {
		series.Overview = nfo.Plot
	}
	if nfo.Rating > 0 {
		series.Rating = nfo.Rating
	}
	if len(nfo.Genres) > 0 {
		series.Genres = strings.Join(nfo.Genres, ",")
	}
	if nfo.Studio != "" {
		series.Studio = nfo.Studio
	}
	if nfo.Country != "" {
		series.Country = nfo.Country
	}
	if nfo.TMDbID > 0 {
		series.TMDbID = nfo.TMDbID
	}
	if nfo.DoubanID != "" {
		series.DoubanID = nfo.DoubanID
	}
}
