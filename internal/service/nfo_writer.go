// Package service NFO 写入器
// 借鉴自 mdcx-master 项目的 nfo.py 模块
// 生成符合 Emby/Jellyfin/Kodi 规范的 .nfo XML 文件
// 使移动端 Emby 客户端（Infuse/Emby Mobile/Jellyfin）能正确识别刮削的元数据
package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== NFO 写入器 ====================

// WriteMovieNFO 为媒体文件生成 .nfo 文件（电影格式）
// 符合 Emby/Jellyfin/Kodi 的 <movie> 规范
// 参数：
//   - mediaFilePath: 媒体文件绝对路径（如 /media/SSIS-001.mp4）
//   - media: Media 模型数据
//
// 返回：生成的 .nfo 文件路径
func (s *NFOService) WriteMovieNFO(mediaFilePath string, media *model.Media) (string, error) {
	if media == nil {
		return "", fmt.Errorf("media 对象为空")
	}

	// NFO 文件路径：媒体文件同名 + .nfo
	ext := filepath.Ext(mediaFilePath)
	nfoPath := strings.TrimSuffix(mediaFilePath, ext) + ".nfo"

	// 构造 XML 内容
	content := buildMovieNFOXML(media)

	// 写入文件（不支持 webdav:// 写入，仅本地）
	if IsWebDAVPath(nfoPath) {
		return "", fmt.Errorf("不支持向 webdav:// 路径写入 NFO 文件")
	}

	// 确保目录存在
	dir := filepath.Dir(nfoPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建 NFO 目录失败: %w", err)
	}

	if err := os.WriteFile(nfoPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("写入 NFO 文件失败: %w", err)
	}

	s.logger.Debugf("NFO 写入成功: %s", nfoPath)
	return nfoPath, nil
}

// WriteAdultNFO 为成人内容生成专用 NFO（包含番号、演员、片商等专用字段）
// 支持额外字段：originaltitle（番号）、mosaic、actresses、series、label、trailer
func (s *NFOService) WriteAdultNFO(mediaFilePath string, media *model.Media, meta *AdultMetadata) (string, error) {
	if media == nil || meta == nil {
		return "", fmt.Errorf("media 或 meta 对象为空")
	}

	ext := filepath.Ext(mediaFilePath)
	nfoPath := strings.TrimSuffix(mediaFilePath, ext) + ".nfo"

	if IsWebDAVPath(nfoPath) {
		return "", fmt.Errorf("不支持向 webdav:// 路径写入 NFO 文件")
	}

	content := buildAdultNFOXML(media, meta)

	dir := filepath.Dir(nfoPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建 NFO 目录失败: %w", err)
	}

	if err := os.WriteFile(nfoPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("写入 NFO 文件失败: %w", err)
	}

	s.logger.Infof("番号 NFO 写入成功: %s (%s)", nfoPath, meta.Code)
	return nfoPath, nil
}

// WriteTVShowNFO 为剧集生成 tvshow.nfo 文件
func (s *NFOService) WriteTVShowNFO(dir string, series *model.Series) (string, error) {
	if series == nil {
		return "", fmt.Errorf("series 对象为空")
	}

	nfoPath := filepath.Join(dir, "tvshow.nfo")

	if IsWebDAVPath(nfoPath) {
		return "", fmt.Errorf("不支持向 webdav:// 路径写入 NFO 文件")
	}

	content := buildTVShowNFOXML(series)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建 NFO 目录失败: %w", err)
	}

	if err := os.WriteFile(nfoPath, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("写入 NFO 文件失败: %w", err)
	}

	s.logger.Debugf("剧集 NFO 写入成功: %s", nfoPath)
	return nfoPath, nil
}

// ==================== XML 构造函数 ====================

// escapeXML 转义 XML 特殊字符
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

// buildMovieNFOXML 构造标准电影 NFO XML
func buildMovieNFOXML(media *model.Media) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString("<movie>\n")

	writeXMLField(&sb, "title", media.Title)
	writeXMLField(&sb, "originaltitle", media.OrigTitle)
	// 排序标题：优先使用 SortTitle，否则回退到 Title
	sortTitle := media.SortTitle
	if sortTitle == "" {
		sortTitle = media.Title
	}
	writeXMLField(&sb, "sorttitle", sortTitle)

	// 番号（Emby 扩展字段）
	writeXMLField(&sb, "num", media.Num)

	if media.Year > 0 {
		writeXMLField(&sb, "year", fmt.Sprintf("%d", media.Year))
	}
	if media.Premiered != "" {
		writeXMLField(&sb, "premiered", media.Premiered)
	}
	// 发行日期：优先 ReleaseDate，否则 Premiered
	releaseDate := media.ReleaseDate
	if releaseDate == "" {
		releaseDate = media.Premiered
	}
	if releaseDate != "" {
		writeXMLField(&sb, "releasedate", releaseDate)
		writeXMLField(&sb, "release", releaseDate)
	}

	if media.Overview != "" {
		// Plot 使用 CDATA 封装，避免 HTML 标签被转义
		sb.WriteString("  <plot><![CDATA[" + media.Overview + "]]></plot>\n")
	}
	// outline：优先用独立的 Outline，否则回退到 Overview
	outline := media.Outline
	if outline == "" {
		outline = media.Overview
	}
	if outline != "" {
		sb.WriteString("  <outline><![CDATA[" + outline + "]]></outline>\n")
	}
	if media.OriginalPlot != "" {
		sb.WriteString("  <originalplot><![CDATA[" + media.OriginalPlot + "]]></originalplot>\n")
	}

	if media.Tagline != "" {
		writeXMLField(&sb, "tagline", media.Tagline)
	}
	if media.Rating > 0 {
		writeXMLField(&sb, "rating", fmt.Sprintf("%.1f", media.Rating))
	}
	if media.Runtime > 0 {
		writeXMLField(&sb, "runtime", fmt.Sprintf("%d", media.Runtime))
	}
	// 分级：mpaa 与 customrating 同步写入
	if media.MPAA != "" {
		writeXMLField(&sb, "mpaa", media.MPAA)
		writeXMLField(&sb, "customrating", media.MPAA)
	}
	if media.CountryCode != "" {
		writeXMLField(&sb, "countrycode", media.CountryCode)
	}
	if media.Studio != "" {
		writeXMLField(&sb, "studio", media.Studio)
	}
	if media.Maker != "" {
		writeXMLField(&sb, "maker", media.Maker)
	}
	if media.Publisher != "" {
		writeXMLField(&sb, "publisher", media.Publisher)
	}
	if media.Label != "" {
		writeXMLField(&sb, "label", media.Label)
	}
	if media.Country != "" {
		writeXMLField(&sb, "country", media.Country)
	}

	// 海报/背景图
	if media.PosterPath != "" {
		writeXMLField(&sb, "poster", media.PosterPath)
		sb.WriteString(fmt.Sprintf("  <thumb aspect=\"poster\">%s</thumb>\n", escapeXML(media.PosterPath)))
	}
	if media.BackdropPath != "" {
		writeXMLField(&sb, "fanart", media.BackdropPath)
		sb.WriteString("  <fanart>\n")
		sb.WriteString(fmt.Sprintf("    <thumb>%s</thumb>\n", escapeXML(media.BackdropPath)))
		sb.WriteString("  </fanart>\n")
	}

	// 类型（逗号分隔 -> 多个 <genre>）
	if media.Genres != "" {
		for _, g := range strings.Split(media.Genres, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				writeXMLField(&sb, "genre", g)
			}
		}
	}

	// 标签（与 genres 分开）：优先使用独立的 Tags 字段，若为空则回退到 Genres
	tagSource := media.Tags
	if tagSource == "" {
		tagSource = media.Genres
	}
	if tagSource != "" {
		for _, tg := range strings.Split(tagSource, ",") {
			tg = strings.TrimSpace(tg)
			if tg != "" {
				writeXMLField(&sb, "tag", tg)
			}
		}
	}

	// 官方网站
	if media.Website != "" {
		writeXMLField(&sb, "website", media.Website)
	}

	// 外部 ID
	if media.TMDbID > 0 {
		sb.WriteString(fmt.Sprintf("  <uniqueid type=\"tmdb\" default=\"true\">%d</uniqueid>\n", media.TMDbID))
		writeXMLField(&sb, "tmdbid", fmt.Sprintf("%d", media.TMDbID))
	}
	if media.DoubanID != "" {
		sb.WriteString(fmt.Sprintf("  <uniqueid type=\"douban\">%s</uniqueid>\n", escapeXML(media.DoubanID)))
		writeXMLField(&sb, "doubanid", media.DoubanID)
	}
	if media.IMDbID != "" {
		sb.WriteString(fmt.Sprintf("  <uniqueid type=\"imdb\">%s</uniqueid>\n", escapeXML(media.IMDbID)))
		writeXMLField(&sb, "imdbid", media.IMDbID)
	}

	// 生成时间戳
	sb.WriteString(fmt.Sprintf("  <dateadded>%s</dateadded>\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("</movie>\n")
	return sb.String()
}

// buildAdultNFOXML 构造番号专用 NFO XML（包含番号、演员、片商等完整字段）
func buildAdultNFOXML(media *model.Media, meta *AdultMetadata) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString("<movie>\n")

	// 标题：优先使用番号 + 标题格式（Emby 展示友好）
	displayTitle := media.Title
	if displayTitle == "" {
		displayTitle = meta.Title
	}
	if meta.Code != "" && !strings.Contains(displayTitle, meta.Code) {
		displayTitle = meta.Code + " " + displayTitle
	}
	writeXMLField(&sb, "title", displayTitle)

	// 原始标题（日文）— P1 新增：优先用刮削到的日文原标题，否则用番号
	originalTitle := meta.OriginalTitle
	if originalTitle == "" {
		originalTitle = meta.Code
	}
	writeXMLField(&sb, "originaltitle", originalTitle)
	writeXMLField(&sb, "sorttitle", meta.Code) // 排序用番号

	// 番号单独字段（Emby 扩展）
	writeXMLField(&sb, "num", meta.Code)

	if media.Year > 0 {
		writeXMLField(&sb, "year", fmt.Sprintf("%d", media.Year))
	}
	if meta.ReleaseDate != "" {
		writeXMLField(&sb, "premiered", meta.ReleaseDate)
		writeXMLField(&sb, "releasedate", meta.ReleaseDate)
	}

	// 简介（P1：优先使用 meta.Plot，fallback 到 media.Overview 和 meta.Title）
	plot := meta.Plot
	if plot == "" {
		plot = media.Overview
	}
	if plot == "" {
		plot = meta.Title
	}
	if plot != "" {
		sb.WriteString("  <plot><![CDATA[" + plot + "]]></plot>\n")
		sb.WriteString("  <outline><![CDATA[" + plot + "]]></outline>\n")
	}

	// 评分
	if meta.Rating > 0 {
		rating := meta.Rating
		// JavDB 是 5 分制，转为 10 分制
		if meta.Source == "javdb" && rating <= 5 {
			rating *= 2
		}
		writeXMLField(&sb, "rating", fmt.Sprintf("%.1f", rating))
		writeXMLField(&sb, "criticrating", fmt.Sprintf("%.0f", rating*10))
	}

	// 时长
	if meta.Duration > 0 {
		writeXMLField(&sb, "runtime", fmt.Sprintf("%d", meta.Duration))
	}

	// 片商
	if meta.Studio != "" {
		writeXMLField(&sb, "studio", meta.Studio)
		writeXMLField(&sb, "maker", meta.Studio) // Emby 番号插件扩展字段
	}
	if meta.Label != "" {
		writeXMLField(&sb, "publisher", meta.Label)
		writeXMLField(&sb, "label", meta.Label)
	}

	// 系列
	if meta.Series != "" {
		writeXMLField(&sb, "set", meta.Series)
		writeXMLField(&sb, "series", meta.Series)
	}

	// Mosaic 标签（便于 Emby 过滤）
	numInfo := ParseCodeEnhanced(meta.Code)
	if numInfo.Mosaic != "" {
		writeXMLField(&sb, "mpaa", numInfo.Mosaic)
		writeXMLField(&sb, "tag", numInfo.Mosaic)
	}

	// 国家
	country := "JP"
	if numInfo.Mosaic == "国产" {
		country = "CN"
	} else if numInfo.Mosaic == "欧美" {
		country = "US"
	}
	writeXMLField(&sb, "country", country)

	// 海报
	if media.PosterPath != "" {
		writeXMLField(&sb, "poster", media.PosterPath)
		sb.WriteString(fmt.Sprintf("  <thumb aspect=\"poster\">%s</thumb>\n", escapeXML(media.PosterPath)))
	} else if meta.Cover != "" {
		writeXMLField(&sb, "poster", meta.Cover)
		sb.WriteString(fmt.Sprintf("  <thumb aspect=\"poster\">%s</thumb>\n", escapeXML(meta.Cover)))
	}
	if media.BackdropPath != "" {
		writeXMLField(&sb, "fanart", media.BackdropPath)
		sb.WriteString("  <fanart>\n")
		sb.WriteString(fmt.Sprintf("    <thumb>%s</thumb>\n", escapeXML(media.BackdropPath)))
		sb.WriteString("  </fanart>\n")
	}

	// 类型标签
	for _, g := range meta.Genres {
		g = strings.TrimSpace(g)
		if g != "" {
			writeXMLField(&sb, "genre", g)
			writeXMLField(&sb, "tag", g)
		}
	}

	// 演员（Emby 格式）— P1：加入 thumb 支持演员头像展示
	for i, actress := range meta.Actresses {
		actress = strings.TrimSpace(actress)
		if actress == "" {
			continue
		}
		sb.WriteString("  <actor>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", escapeXML(actress)))
		sb.WriteString("    <role>演员</role>\n")
		sb.WriteString(fmt.Sprintf("    <sortorder>%d</sortorder>\n", i))
		sb.WriteString("    <type>Actor</type>\n")
		// P1：演员头像（Emby/Jellyfin 读取 thumb 作为 actor 图片）
		if photo, ok := meta.ActorPhotos[actress]; ok && photo != "" {
			sb.WriteString(fmt.Sprintf("    <thumb>%s</thumb>\n", escapeXML(photo)))
		}
		sb.WriteString("  </actor>\n")
	}

	// P1：导演
	if meta.Director != "" {
		writeXMLField(&sb, "director", meta.Director)
	}

	// P1：预告片 Trailer
	if meta.Trailer != "" {
		writeXMLField(&sb, "trailer", meta.Trailer)
	}

	// 唯一 ID（使用番号）
	sb.WriteString(fmt.Sprintf("  <uniqueid type=\"num\" default=\"true\">%s</uniqueid>\n", escapeXML(meta.Code)))

	// 数据来源
	if meta.Source != "" {
		sb.WriteString(fmt.Sprintf("  <!-- scraped from %s -->\n", meta.Source))
	}

	sb.WriteString(fmt.Sprintf("  <dateadded>%s</dateadded>\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("</movie>\n")
	return sb.String()
}

// buildTVShowNFOXML 构造剧集 NFO XML
func buildTVShowNFOXML(series *model.Series) string {
	var sb strings.Builder
	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	sb.WriteString("<tvshow>\n")

	writeXMLField(&sb, "title", series.Title)
	writeXMLField(&sb, "originaltitle", series.OrigTitle)
	writeXMLField(&sb, "sorttitle", series.Title)

	if series.Year > 0 {
		writeXMLField(&sb, "year", fmt.Sprintf("%d", series.Year))
	}

	if series.Overview != "" {
		sb.WriteString("  <plot><![CDATA[" + series.Overview + "]]></plot>\n")
		sb.WriteString("  <outline><![CDATA[" + series.Overview + "]]></outline>\n")
	}

	if series.Rating > 0 {
		writeXMLField(&sb, "rating", fmt.Sprintf("%.1f", series.Rating))
	}
	if series.Studio != "" {
		writeXMLField(&sb, "studio", series.Studio)
	}
	if series.Country != "" {
		writeXMLField(&sb, "country", series.Country)
	}

	if series.Genres != "" {
		for _, g := range strings.Split(series.Genres, ",") {
			g = strings.TrimSpace(g)
			if g != "" {
				writeXMLField(&sb, "genre", g)
			}
		}
	}

	// 外部 ID
	if series.TMDbID > 0 {
		sb.WriteString(fmt.Sprintf("  <uniqueid type=\"tmdb\" default=\"true\">%d</uniqueid>\n", series.TMDbID))
		writeXMLField(&sb, "tmdbid", fmt.Sprintf("%d", series.TMDbID))
	}
	if series.DoubanID != "" {
		sb.WriteString(fmt.Sprintf("  <uniqueid type=\"douban\">%s</uniqueid>\n", escapeXML(series.DoubanID)))
		writeXMLField(&sb, "doubanid", series.DoubanID)
	}

	sb.WriteString(fmt.Sprintf("  <dateadded>%s</dateadded>\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("</tvshow>\n")
	return sb.String()
}

// writeXMLField 写入一个 XML 字段（自动跳过空值）
func writeXMLField(sb *strings.Builder, name, value string) {
	if value == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("  <%s>%s</%s>\n", name, escapeXML(value), name))
}
