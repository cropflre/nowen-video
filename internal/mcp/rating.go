package mcp

import (
	"encoding/json"
	"regexp"
	"strings"
)

// 家庭分级档位与前端保持一致（见 web/src/utils/familyRating.ts）。
// 后端字段值只能是 '' | 'G' | 'PG' | 'PG-13' | 'R' | 'NC-17'，'' 表示未确认。
var validFamilyLevels = map[string]bool{
	"": true, "G": true, "PG": true, "PG-13": true, "R": true, "NC-17": true,
}

// RatingSuggestion 分级建议：只给建议，不写库，不放行儿童。
type RatingSuggestion struct {
	Level    string `json:"level"`    // G / PG / PG-13 / R / NC-17
	Source   string `json:"source"`   // "TMDb · 美国" 或 "本地 NFO"
	Original string `json:"original"` // 原始分级字符串，如 PG-13
}

var usRatings = map[string]string{
	"G": "G", "PG": "PG", "PG-13": "PG-13", "R": "R", "NC-17": "NC-17",
}

// 电视分级到家庭分级的映射（与前端 suggestFamilyRating 的 tvRatings 一致）。
var tvRatings = map[string]string{
	"TV-Y": "G", "TV-G": "G", "TV-Y7": "PG", "TV-Y7-FV": "PG",
	"TV-PG": "PG", "TV-14": "PG-13", "TV-MA": "NC-17",
}

var localRatingPattern = regexp.MustCompile(`^(?:US\s*:\s*)?(G|PG|PG-13|R|NC-17)$`)

type tmdbRatingRecord struct {
	country string
	rating  string
}

// parseTMDbRatings 解析 tmdb_rating_data 中的原始分级 JSON。
// 与前端实现同样严格：任何结构异常都视为无法判断（返回 ok=false），
// 而不是猜一个结果。
func parseTMDbRatings(tmdbData string, series bool) ([]tmdbRatingRecord, bool) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(tmdbData), &parsed); err != nil {
		return nil, false
	}
	results, ok := parsed["results"].([]any)
	if !ok {
		return nil, false
	}
	var records []tmdbRatingRecord
	for _, item := range results {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		country, ok := entry["iso_3166_1"].(string)
		if !ok {
			return nil, false
		}
		country = strings.ToUpper(country)
		if series {
			rating, ok := entry["rating"].(string)
			if !ok {
				return nil, false
			}
			if strings.TrimSpace(rating) != "" {
				records = append(records, tmdbRatingRecord{country: country, rating: strings.ToUpper(strings.TrimSpace(rating))})
			}
			continue
		}
		releases, ok := entry["release_dates"].([]any)
		if !ok {
			return nil, false
		}
		for _, rel := range releases {
			release, ok := rel.(map[string]any)
			if !ok {
				return nil, false
			}
			cert, ok := release["certification"].(string)
			if !ok {
				return nil, false
			}
			if strings.TrimSpace(cert) != "" {
				records = append(records, tmdbRatingRecord{country: country, rating: strings.ToUpper(strings.TrimSpace(cert))})
			}
		}
	}
	return records, true
}

// suggestFamilyRating 是 web/src/utils/familyRating.ts 中 suggestFamilyRating
// 的 Go 移植：仅美国明确分级才给建议，冲突/混合/未知一律不建议。
// 返回 (建议, 原因)：建议为 nil 时，原因说明为什么不建议。
func suggestFamilyRating(mpaa, tmdbData string, series bool) (*RatingSuggestion, string) {
	local := strings.ToUpper(strings.TrimSpace(mpaa))
	var localLevel string
	if local != "" {
		if m := localRatingPattern.FindStringSubmatch(local); m != nil {
			localLevel = usRatings[m[1]]
		}
	}

	var records []tmdbRatingRecord
	if strings.TrimSpace(tmdbData) != "" {
		var ok bool
		records, ok = parseTMDbRatings(tmdbData, series)
		if !ok {
			return nil, "TMDb 分级数据无法解析"
		}
	}

	usSeen := map[string]bool{}
	var usRatingsList []string
	for _, r := range records {
		if r.country == "US" && !usSeen[r.rating] {
			usSeen[r.rating] = true
			usRatingsList = append(usRatingsList, r.rating)
		}
	}
	// 混合地区体系无法安全比较。
	for _, r := range records {
		if r.country != "US" {
			return nil, "存在非美国分级，无法安全比较"
		}
	}
	if len(usRatingsList) > 1 {
		return nil, "存在多个不同的美国分级，无法安全比较"
	}

	lookup := usRatings
	if series {
		lookup = tvRatings
	}
	var tmdbLevel string
	if len(usRatingsList) == 1 {
		tmdbLevel = lookup[usRatingsList[0]]
	}
	if len(usRatingsList) > 0 && tmdbLevel == "" {
		return nil, "美国分级无法映射到家庭分级"
	}
	if local != "" && localLevel == "" {
		return nil, "本地 MPAA 不是明确的美国分级"
	}
	if localLevel != "" && tmdbLevel != "" && localLevel != tmdbLevel {
		return nil, "本地分级与 TMDb 美国分级冲突"
	}
	if tmdbLevel != "" {
		return &RatingSuggestion{Level: tmdbLevel, Source: "TMDb · 美国", Original: usRatingsList[0]}, ""
	}
	if localLevel != "" {
		return &RatingSuggestion{Level: localLevel, Source: "本地 NFO", Original: localRatingPattern.FindStringSubmatch(local)[1]}, ""
	}
	return nil, "未找到明确的美国分级"
}
