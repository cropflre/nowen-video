package service

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SubtitleProviderSearchRequest 是在线字幕 Provider 的统一搜索请求。
// Queries 已由 Nowen 的文件名解析器生成，Provider 不需要理解媒体库模型。
type SubtitleProviderSearchRequest struct {
	Queries   []string
	FileName  string
	Title     string
	Year      int
	MediaType string
	Languages []string
}

// SubtitleProviderDetails 表示一个字幕详情页可下载的语言集合。
type SubtitleProviderDetails struct {
	ID        string
	Title     string
	SourceURL string
	Languages []SubtitleProviderLanguage
}

// SubtitleProviderLanguage 是 Provider 内部的可下载语言条目。
// DownloadID 必须是 Provider 自己生成的 opaque ref，不能直接暴露任意外部 URL。
type SubtitleProviderLanguage struct {
	Code       string
	Name       string
	DownloadID string
}

// SubtitleProviderDownload 是 Provider 下载并完成基础 HTTP 校验后的原始字幕负载。
type SubtitleProviderDownload struct {
	Content  []byte
	FileName string
	Language string
}

// SubtitleProvider 为 HTML/API 字幕源提供最小可替换抽象。
// HTML selector、URL pattern、语言映射必须留在具体 Provider 内部。
type SubtitleProvider interface {
	Name() string
	Search(context.Context, SubtitleProviderSearchRequest) ([]SubtitleSearchResult, error)
	GetDetails(context.Context, string) (*SubtitleProviderDetails, error)
	Download(context.Context, string) (*SubtitleProviderDownload, error)
}

// SubtitleProviderUnavailableError 表示上游暂时不可用（限流、反爬、超时、5xx、页面挑战等）。
// Handler 可将它作为普通搜索失败展示，不应该导致播放器崩溃。
type SubtitleProviderUnavailableError struct {
	Provider string
	Reason   string
}

func (e *SubtitleProviderUnavailableError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("字幕源 %s 暂时不可用", e.Provider)
	}
	return fmt.Sprintf("字幕源 %s 暂时不可用: %s", e.Provider, e.Reason)
}

func IsSubtitleProviderUnavailable(err error) bool {
	var target *SubtitleProviderUnavailableError
	return errors.As(err, &target)
}

var subtitleSearchTechToken = regexp.MustCompile(`(?i)(?:^|[. _-])(?:4320p|2160p|1440p|1080p|720p|480p|8k|4k|uhd|web[ ._-]?dl|webrip|bluray|blu[ ._-]?ray|bdremux|bdrip|remux|hdtv|dvdrip|x26[45]|h[ .]?26[45]|hevc|avc|av1|hdr10\+?|hdr|dovi|dolby[ ._-]?vision|sdr|ddp\d?(?:\.\d)?|truehd|dts(?:[ ._-]?hd)?|aac|flac|atmos)(?:$|[. _-])`)
var subtitleEpisodeTag = regexp.MustCompile(`(?i)S(\d{1,2})[. _-]*E(\d{1,4})`)
var subtitleYearTag = regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(?:[^0-9]|$)`)

// BuildSubtitleSearchQueries 使用项目已有的电影/剧集解析器生成搜索词。
// 第一项尽量保留 release 的点分隔写法，后续项逐步放宽到“标题 + 年份/季集”。
func BuildSubtitleSearchQueries(filePath, fallbackTitle string, fallbackYear int, mediaType string) []string {
	fileName := filepath.Base(filePath)
	base := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	queries := make([]string, 0, 5)
	add := func(v string) {
		v = strings.TrimSpace(strings.Trim(v, ".-_ "))
		if v == "" {
			return
		}
		for _, existing := range queries {
			if strings.EqualFold(existing, v) {
				return
			}
		}
		queries = append(queries, v)
	}

	// 精确 release 关键词：从首个技术标签开始截断，但保留年份和 SxxExx。
	if loc := subtitleSearchTechToken.FindStringIndex(base); loc != nil {
		add(base[:loc[0]])
	} else {
		add(base)
	}

	ep := ParseEpisodeFilename(fileName)
	isEpisode := mediaType == "episode" || ep.EpisodeNum > 0
	if isEpisode {
		seriesTitle := strings.TrimSpace(ep.SeriesTitle)
		if seriesTitle == "" {
			seriesTitle = strings.TrimSpace(fallbackTitle)
		}
		if seriesTitle != "" && ep.EpisodeNum > 0 {
			season := ep.SeasonNum
			if season <= 0 {
				season = 1
			}
			tag := fmt.Sprintf("S%02dE%02d", season, ep.EpisodeNum)
			add(fmt.Sprintf("%s %s", seriesTitle, tag))
			add(strings.ReplaceAll(fmt.Sprintf("%s.%s", seriesTitle, tag), " ", "."))
		}
		add(seriesTitle)
		return queries
	}

	parsed := ParseMovieFilename(fileName)
	title := strings.TrimSpace(parsed.Title)
	if title == "" {
		title = strings.TrimSpace(fallbackTitle)
	}
	year := parsed.Year
	if year <= 0 {
		year = fallbackYear
	}
	if title != "" && year > 0 {
		add(fmt.Sprintf("%s %d", title, year))
		add(strings.ReplaceAll(fmt.Sprintf("%s.%d", title, year), " ", "."))
	}
	add(title)
	if parsed.TitleAlt != "" && year > 0 {
		add(fmt.Sprintf("%s %d", parsed.TitleAlt, year))
	}
	add(parsed.TitleAlt)
	return queries
}

func parseSubtitleLanguages(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"zh-CN", "zh-TW", "en"}
	}
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, item := range strings.Split(raw, ",") {
		code := normalizeSubtitleLanguageCode(item)
		if code == "" || seen[strings.ToLower(code)] {
			continue
		}
		seen[strings.ToLower(code)] = true
		out = append(out, code)
	}
	if len(out) == 0 {
		return []string{"zh-CN", "zh-TW", "en"}
	}
	return out
}

func normalizeSubtitleLanguageCode(code string) string {
	code = strings.TrimSpace(strings.ReplaceAll(code, "_", "-"))
	switch strings.ToLower(code) {
	case "zh", "zh-cn", "zh-hans", "chs", "sc", "chi-sim":
		return "zh-CN"
	case "zh-tw", "zh-hant", "cht", "tc", "chi-tra":
		return "zh-TW"
	case "eng", "english", "en":
		return "en"
	case "jpn", "jp", "japanese", "ja":
		return "ja"
	case "kor", "korean", "ko":
		return "ko"
	default:
		parts := strings.Split(code, "-")
		if len(parts) == 2 {
			return strings.ToLower(parts[0]) + "-" + strings.ToUpper(parts[1])
		}
		return strings.ToLower(code)
	}
}

func subtitleLanguagePriority(code string) int {
	switch normalizeSubtitleLanguageCode(code) {
	case "zh-CN":
		return 0
	case "zh-TW":
		return 1
	case "en":
		return 2
	default:
		return 10
	}
}

func sortSubtitleSearchResults(results []SubtitleSearchResult) {
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].MatchScore != results[j].MatchScore {
			return results[i].MatchScore > results[j].MatchScore
		}
		pi := subtitleLanguagePriority(results[i].Language)
		pj := subtitleLanguagePriority(results[j].Language)
		if pi != pj {
			return pi < pj
		}
		if results[i].DownloadCount != results[j].DownloadCount {
			return results[i].DownloadCount > results[j].DownloadCount
		}
		return results[i].FileName < results[j].FileName
	})
}
