// Package service 番号元数据映射表（演员/片商/系列别名统一）
// 借鉴自 mdcx-master 项目的 mapping 设计
// 目的：消除同一主体在不同数据源下的名称差异（简繁日/缩写等）
package service

import (
	"strings"
	"sync"
)

// ==================== 内置常用别名映射表 ====================

// defaultActressAliases 常见演员别名 -> 规范中文名
// 注：仅列举高频 Top 名字作为内置种子，可通过 SetActressAliases 动态扩展
var defaultActressAliases = map[string]string{
	// 三上悠亚
	"三上悠亜":         "三上悠亚",
	"三上悠亞":         "三上悠亚",
	"mikami yua":   "三上悠亚",
	"Mikami Yua":   "三上悠亚",
	"MIKAMI YUA":   "三上悠亚",
	"三上 悠亜":        "三上悠亚",
	// 桥本有菜
	"橋本ありな":        "桥本有菜",
	"橋本有菜":         "桥本有菜",
	"hashimoto arina": "桥本有菜",
	// 明里䌷
	"明里つむぎ":        "明里䌷",
	"明里紬":          "明里䌷",
	"akari tsumugi": "明里䌷",
	// 深田咏美
	"深田えいみ":        "深田咏美",
	"深田詠美":         "深田咏美",
	// 相泽南
	"相沢みなみ":        "相泽南",
	"相澤みなみ":        "相泽南",
	// 天使萌
	"天使もえ":         "天使萌",
	"天使 萌":         "天使萌",
	// 七泽美亚
	"七沢みあ":         "七泽美亚",
	"七澤みあ":         "七泽美亚",
	// 高桥圣子
	"高橋しょう子":       "高桥圣子",
	"高橋聖子":         "高桥圣子",
	// 枫富爱
	"楓ふうあ":         "枫富爱",
	"楓カレン":         "枫花恋",
	// 水卜樱
	"水卜さくら":        "水卜樱",
	// 葵司
	"葵つかさ":         "葵司",
	// 河北彩花
	"河北彩花":         "河北彩花",
	// 乃木坂步美
	"乃木坂あゆみ":       "乃木坂步美",
}

// defaultStudioAliases 片商/制作商别名 -> 规范名
var defaultStudioAliases = map[string]string{
	// S1
	"s1":              "S1 NO.1 STYLE",
	"S1":              "S1 NO.1 STYLE",
	"s1 no.1 style":   "S1 NO.1 STYLE",
	"エスワン ナンバーワンスタイル": "S1 NO.1 STYLE",
	// MOODYZ
	"moodyz":  "MOODYZ",
	"ムーディーズ": "MOODYZ",
	// IdeaPocket
	"idea pocket": "IdeaPocket",
	"IDEA POCKET": "IdeaPocket",
	"アイデアポケット":   "IdeaPocket",
	// Madonna
	"madonna": "Madonna",
	"マドンナ":    "Madonna",
	// Premium
	"premium": "Premium",
	"プレミアム":   "Premium",
	// Attackers
	"attackers": "Attackers",
	"アタッカーズ":    "Attackers",
	// Prestige
	"prestige": "Prestige",
	"プレステージ":   "Prestige",
	// kawaii*
	"kawaii*":  "kawaii*",
	"kawaii":   "kawaii*",
	// SOD Create
	"sod create":   "SOD Create",
	"soft on demand": "SOD Create",
	// FALENO
	"faleno": "FALENO",
	"FALENO": "FALENO",
}

// defaultSeriesAliases 常见系列别名 -> 规范名
var defaultSeriesAliases = map[string]string{
	"新・絶頂" : "新・绝顶",
	"極上泡姫物語": "极上泡姫物语",
	"中出し":    "中出",
}

// defaultGenreAliases 标签别名 -> 规范标签
var defaultGenreAliases = map[string]string{
	"単体作品":    "单体作品",
	"美少女":     "美少女",
	"巨乳":      "巨乳",
	"美乳":      "美乳",
	"中出し":     "中出",
	"騎乗位":     "骑乘位",
	"フェラ":     "口交",
	"イラマチオ":   "深喉",
	"デジモ":     "高清",
	"ハイビジョン":  "高清",
	"独占配信":    "独家",
	"HD":      "高清",
	"4K":      "4K",
}

// ==================== 动态映射注册表 ====================

// mappingRegistry 运行时可扩展的映射表（线程安全）
type mappingRegistry struct {
	mu       sync.RWMutex
	actress  map[string]string
	studio   map[string]string
	series   map[string]string
	genre    map[string]string
}

var globalMapping = &mappingRegistry{
	actress: copyStringMap(defaultActressAliases),
	studio:  copyStringMap(defaultStudioAliases),
	series:  copyStringMap(defaultSeriesAliases),
	genre:   copyStringMap(defaultGenreAliases),
}

// copyStringMap 复制 map（避免修改默认表）
func copyStringMap(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ==================== 对外 API ====================

// NormalizeActress 规范化演员名（将别名统一为规范名）
func NormalizeActress(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	globalMapping.mu.RLock()
	defer globalMapping.mu.RUnlock()
	// 精确匹配
	if canonical, ok := globalMapping.actress[name]; ok {
		return canonical
	}
	// 小写匹配（英文名）
	if canonical, ok := globalMapping.actress[strings.ToLower(name)]; ok {
		return canonical
	}
	return name
}

// NormalizeStudio 规范化片商名
func NormalizeStudio(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	globalMapping.mu.RLock()
	defer globalMapping.mu.RUnlock()
	if canonical, ok := globalMapping.studio[name]; ok {
		return canonical
	}
	if canonical, ok := globalMapping.studio[strings.ToLower(name)]; ok {
		return canonical
	}
	return name
}

// NormalizeSeries 规范化系列名
func NormalizeSeries(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	globalMapping.mu.RLock()
	defer globalMapping.mu.RUnlock()
	if canonical, ok := globalMapping.series[name]; ok {
		return canonical
	}
	return name
}

// NormalizeGenre 规范化单个标签
func NormalizeGenre(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	globalMapping.mu.RLock()
	defer globalMapping.mu.RUnlock()
	if canonical, ok := globalMapping.genre[name]; ok {
		return canonical
	}
	return name
}

// NormalizeActresses 批量规范化演员名列表，并去重
func NormalizeActresses(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, raw := range names {
		n := NormalizeActress(raw)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		result = append(result, n)
	}
	return result
}

// NormalizeGenres 批量规范化标签并去重
func NormalizeGenres(names []string) []string {
	seen := make(map[string]struct{}, len(names))
	result := make([]string, 0, len(names))
	for _, raw := range names {
		n := NormalizeGenre(raw)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		result = append(result, n)
	}
	return result
}

// ==================== 运行时扩展 API ====================

// SetActressAliases 批量注入演员别名（别名 -> 规范名）
func SetActressAliases(aliases map[string]string) {
	globalMapping.mu.Lock()
	defer globalMapping.mu.Unlock()
	for k, v := range aliases {
		globalMapping.actress[k] = v
	}
}

// SetStudioAliases 批量注入片商别名
func SetStudioAliases(aliases map[string]string) {
	globalMapping.mu.Lock()
	defer globalMapping.mu.Unlock()
	for k, v := range aliases {
		globalMapping.studio[k] = v
	}
}

// SetSeriesAliases 批量注入系列别名
func SetSeriesAliases(aliases map[string]string) {
	globalMapping.mu.Lock()
	defer globalMapping.mu.Unlock()
	for k, v := range aliases {
		globalMapping.series[k] = v
	}
}

// SetGenreAliases 批量注入标签别名
func SetGenreAliases(aliases map[string]string) {
	globalMapping.mu.Lock()
	defer globalMapping.mu.Unlock()
	for k, v := range aliases {
		globalMapping.genre[k] = v
	}
}

// NormalizeMetadata 对 AdultMetadata 执行全面规范化（就地修改）
// 会同时规范化 actresses/studio/label/series/genres
func NormalizeMetadata(meta *AdultMetadata) {
	if meta == nil {
		return
	}
	meta.Actresses = NormalizeActresses(meta.Actresses)
	meta.Studio = NormalizeStudio(meta.Studio)
	meta.Label = NormalizeStudio(meta.Label) // Label 与 Studio 共用同一映射
	meta.Series = NormalizeSeries(meta.Series)
	meta.Genres = NormalizeGenres(meta.Genres)
}
