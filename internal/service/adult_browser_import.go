package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
)

// AdultBrowserImportResult describes the files produced from browser-captured metadata.
type AdultBrowserImportResult struct {
	VideoPath  string `json:"video_path"`
	NFOPath    string `json:"nfo_path"`
	PosterPath string `json:"poster_path,omitempty"`
	FanartPath string `json:"fanart_path,omitempty"`
	Source     string `json:"source"`
	Code       string `json:"code"`
}

// AdultBrowserMediaStatus describes whether a local media path already has sidecar metadata.
type AdultBrowserMediaStatus struct {
	Path          string   `json:"path"`
	IsDir         bool     `json:"is_dir"`
	Exists        bool     `json:"exists"`
	HasNFO        bool     `json:"has_nfo"`
	NeedsScrape   bool     `json:"needs_scrape"`
	SameNameNFO   string   `json:"same_name_nfo,omitempty"`
	DirectoryNFOs []string `json:"directory_nfos,omitempty"`
	HasPoster     bool     `json:"has_poster"`
	Message       string   `json:"message"`
}

// CheckBrowserMediaStatus checks whether a local media path appears to be scraped.
func (s *AdultScraperService) CheckBrowserMediaStatus(path string) (*AdultBrowserMediaStatus, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		return nil, fmt.Errorf("路径为空")
	}
	if IsWebDAVPath(path) {
		return nil, fmt.Errorf("浏览器状态检查暂不支持 webdav:// 路径")
	}

	status := &AdultBrowserMediaStatus{Path: path}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			status.Message = "路径不存在"
			status.NeedsScrape = true
			return status, nil
		}
		return nil, fmt.Errorf("无法访问路径: %w", err)
	}
	status.Exists = true
	status.IsDir = info.IsDir()

	dir := path
	if !info.IsDir() {
		dir = filepath.Dir(path)
		stem := strings.TrimSuffix(path, filepath.Ext(path))
		sameName := stem + ".nfo"
		if _, err := os.Stat(sameName); err == nil {
			status.SameNameNFO = sameName
			status.HasNFO = true
		}
		status.HasPoster = hasMediaPoster(path)
	}

	nfos, _ := filepath.Glob(filepath.Join(dir, "*.nfo"))
	status.DirectoryNFOs = nfos
	if len(nfos) > 0 {
		status.HasNFO = true
	}

	status.NeedsScrape = !status.HasNFO
	if status.NeedsScrape {
		status.Message = "未刮削，需要刮削"
	} else {
		status.Message = "已刮削"
	}
	return status, nil
}

// ImportBrowserMetadataToPath applies metadata extracted by the Chrome extension to a local media file.
func (s *AdultScraperService) ImportBrowserMetadataToPath(videoPath string, meta *AdultMetadata) (*AdultBrowserImportResult, error) {
	if s == nil || !s.IsEnabled() {
		return nil, fmt.Errorf("番号刮削功能未启用")
	}
	if meta == nil {
		return nil, fmt.Errorf("元数据为空")
	}
	videoPath = filepath.Clean(strings.TrimSpace(videoPath))
	if videoPath == "" {
		return nil, fmt.Errorf("视频路径为空")
	}
	if IsWebDAVPath(videoPath) {
		return nil, fmt.Errorf("浏览器导入暂不支持 webdav:// 路径")
	}
	info, err := os.Stat(videoPath)
	if err != nil {
		return nil, fmt.Errorf("无法访问视频文件: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("视频路径不能是目录: %s", videoPath)
	}

	normalizeBrowserMetadata(videoPath, meta)
	NormalizeMetadata(meta)
	s.TranslateAdultMetadata(meta)

	media := &model.Media{
		ID:        "browser-" + uuid.NewString()[:8],
		FilePath:  videoPath,
		Title:     meta.Title,
		OrigTitle: meta.Code,
	}
	if err := s.ApplyToMedia(media, meta, "primary"); err != nil {
		return nil, err
	}

	result := &AdultBrowserImportResult{
		VideoPath: videoPath,
		NFOPath:   strings.TrimSuffix(videoPath, filepath.Ext(videoPath)) + ".nfo",
		Source:    meta.Source,
		Code:      meta.Code,
	}

	poster, fanart := s.writeBrowserSidecarImages(videoPath, media.PosterPath, meta)
	result.PosterPath = poster
	result.FanartPath = fanart
	return result, nil
}

func hasMediaPoster(videoPath string) bool {
	stem := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))
	dir := filepath.Dir(videoPath)
	base := filepath.Base(stem)
	candidates := []string{
		stem + "-poster.jpg",
		stem + "-poster.png",
		stem + "-poster.webp",
		stem + ".jpg",
		stem + ".png",
		stem + ".webp",
		filepath.Join(dir, base+"-cover.jpg"),
		filepath.Join(dir, base+"-thumb.jpg"),
		filepath.Join(dir, "poster.jpg"),
		filepath.Join(dir, "cover.jpg"),
		filepath.Join(dir, "folder.jpg"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return true
		}
	}
	return false
}

func normalizeBrowserMetadata(videoPath string, meta *AdultMetadata) {
	meta.Code = strings.ToUpper(strings.TrimSpace(meta.Code))
	if meta.Code == "" {
		if info := ParseCodeEnhanced(filepath.Base(videoPath)); info.Number != "" {
			meta.Code = info.Number
		} else if code, _ := ParseCode(filepath.Base(videoPath)); code != "" {
			meta.Code = code
		}
	}
	if meta.Source == "" {
		meta.Source = "browser"
	}
	meta.Source = strings.ToLower(strings.TrimSpace(meta.Source))
	if meta.ActorPhotos == nil {
		meta.ActorPhotos = make(map[string]string)
	}
	if meta.Genres == nil {
		meta.Genres = []string{}
	}
	if meta.Actresses == nil {
		meta.Actresses = []string{}
	}
	if meta.ExtraFanart == nil {
		meta.ExtraFanart = []string{}
	}
}

func (s *AdultScraperService) writeBrowserSidecarImages(videoPath, localPosterPath string, meta *AdultMetadata) (posterPath, fanartPath string) {
	stem := strings.TrimSuffix(videoPath, filepath.Ext(videoPath))

	if localPosterPath != "" {
		posterPath = stem + "-poster.jpg"
		if err := copyFileForFolderScrape(localPosterPath, posterPath); err != nil {
			s.logger.Debugf("浏览器导入 poster 写入失败: %v", err)
			posterPath = ""
		}
	} else if meta.Cover != "" {
		posterPath = stem + "-poster.jpg"
		if err := s.downloadImageTo(meta.Cover, posterPath); err != nil {
			s.logger.Debugf("浏览器导入 poster 下载失败: %v", err)
			posterPath = ""
		}
	}

	fanartURL := meta.Thumb
	if fanartURL == "" && len(meta.ExtraFanart) > 0 {
		fanartURL = meta.ExtraFanart[0]
	}
	if fanartURL != "" {
		fanartPath = stem + "-fanart.jpg"
		if err := s.downloadImageTo(fanartURL, fanartPath); err != nil {
			s.logger.Debugf("浏览器导入 fanart 下载失败: %v", err)
			fanartPath = ""
		}
	}
	return posterPath, fanartPath
}
