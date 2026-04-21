package emby

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
)

// splitGenres 把逗号分隔的 genres 字符串切片，过滤空白。
func splitGenres(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '|' || r == '/' || r == '、'
	})
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// genresToNameIdPairs 将类型名称列表转成 Emby GenreItems。
// 这里 Id 用 "genre:<name>" 哈希，便于以后做类型筛选时客户端回传识别。
func genresToNameIdPairs(genres []string) []NameIdPair {
	if len(genres) == 0 {
		return nil
	}
	m := NewIDMapper() // 临时映射，只为生成稳定数字 Id；全局 IDMapper 不需记住
	out := make([]NameIdPair, 0, len(genres))
	for _, g := range genres {
		out = append(out, NameIdPair{Name: g, Id: m.ToEmbyID("genre:" + g)})
	}
	return out
}

// buildProviderIds 基于 Media 的 TMDb/IMDb/Douban/Bangumi 字段生成 Emby ProviderIds。
// 这是 Infuse 判断是否 "已识别" 的关键依据，务必填充。
func buildProviderIds(m *model.Media) map[string]string {
	ids := make(map[string]string, 4)
	if m.TMDbID > 0 {
		ids["Tmdb"] = strconv.Itoa(m.TMDbID)
	}
	if m.IMDbID != "" {
		ids["Imdb"] = m.IMDbID
	}
	if m.DoubanID != "" {
		ids["Douban"] = m.DoubanID
	}
	if m.BangumiID > 0 {
		ids["Bangumi"] = strconv.Itoa(m.BangumiID)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// buildSeriesProviderIds 同上，针对 Series。
func buildSeriesProviderIds(s *model.Series) map[string]string {
	ids := make(map[string]string, 4)
	if s.TMDbID > 0 {
		ids["Tmdb"] = strconv.Itoa(s.TMDbID)
	}
	if s.IMDbID != "" {
		ids["Imdb"] = s.IMDbID
	}
	if s.DoubanID != "" {
		ids["Douban"] = s.DoubanID
	}
	if s.BangumiID > 0 {
		ids["Bangumi"] = strconv.Itoa(s.BangumiID)
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// buildImageTags 为 Media 生成 ImageTags（ Primary/Thumb ）和 BackdropImageTags。
// 值是 Emby 客户端用来做缓存校验的 "Tag"，任何稳定字符串都行——这里用 ID + "_primary"。
func buildImageTags(m *model.Media) map[string]string {
	tags := make(map[string]string, 2)
	if m.PosterPath != "" || m.ID != "" {
		tags["Primary"] = shortTag(m.ID + "|" + m.PosterPath)
	}
	if m.BackdropPath != "" {
		tags["Backdrop"] = shortTag(m.ID + "|" + m.BackdropPath)
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

// buildSeriesImageTags 同上，针对 Series。
func buildSeriesImageTags(s *model.Series) map[string]string {
	tags := make(map[string]string, 2)
	if s.PosterPath != "" || s.ID != "" {
		tags["Primary"] = shortTag(s.ID + "|" + s.PosterPath)
	}
	if s.BackdropPath != "" {
		tags["Backdrop"] = shortTag(s.ID + "|" + s.BackdropPath)
	}
	if len(tags) == 0 {
		return nil
	}
	return tags
}

// shortTag 生成稳定的短 tag（用于 ImageTag）。
func shortTag(input string) string {
	// 简单的 djb2 hash，生成 8 位十六进制字符串足够做缓存 key
	var h uint32 = 5381
	for i := 0; i < len(input); i++ {
		h = ((h << 5) + h) + uint32(input[i])
	}
	hex := "0123456789abcdef"
	out := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		out[i] = hex[h&0xF]
		h >>= 4
	}
	return string(out)
}

// parseResolutionWidth 从 "1920x1080" / "1080p" / "4K" 等字符串解析宽度。
// 无法解析时返回 0。
func parseResolutionWidth(res string) int {
	if res == "" {
		return 0
	}
	if idx := strings.IndexByte(res, 'x'); idx > 0 {
		if w, err := strconv.Atoi(strings.TrimSpace(res[:idx])); err == nil {
			return w
		}
	}
	switch strings.ToLower(strings.TrimSpace(res)) {
	case "4k", "2160p":
		return 3840
	case "1440p", "2k":
		return 2560
	case "1080p":
		return 1920
	case "720p":
		return 1280
	case "480p":
		return 854
	case "360p":
		return 640
	}
	return 0
}

// parseResolutionHeight 从分辨率字符串解析高度。
func parseResolutionHeight(res string) int {
	if res == "" {
		return 0
	}
	if idx := strings.IndexByte(res, 'x'); idx > 0 && idx < len(res)-1 {
		if h, err := strconv.Atoi(strings.TrimSpace(res[idx+1:])); err == nil {
			return h
		}
	}
	switch strings.ToLower(strings.TrimSpace(res)) {
	case "4k", "2160p":
		return 2160
	case "1440p", "2k":
		return 1440
	case "1080p":
		return 1080
	case "720p":
		return 720
	case "480p":
		return 480
	case "360p":
		return 360
	}
	return 0
}

// containerFromPath 从文件路径提取容器格式（不含点，emby 约定如 "mkv"、"mp4"）。
func containerFromPath(p string) string {
	if p == "" {
		return ""
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p), "."))
	return ext
}

// mimeFromContainer 由容器名推断 Content-Type。
func mimeFromContainer(c string) string {
	switch strings.ToLower(c) {
	case "mp4", "m4v":
		return "video/mp4"
	case "mkv":
		return "video/x-matroska"
	case "webm":
		return "video/webm"
	case "avi":
		return "video/x-msvideo"
	case "mov":
		return "video/quicktime"
	case "ts":
		return "video/mp2t"
	case "flv":
		return "video/x-flv"
	default:
		return "application/octet-stream"
	}
}
