package service

import (
	"encoding/json"
	"strings"
)

// ScrapeMode 刮削刷新档位。
type ScrapeMode string

const (
	// ScrapeModeReplace 全部替换：现有行为，全量覆盖（被锁定字段除外）。
	ScrapeModeReplace ScrapeMode = "replace"
	// ScrapeModeFillMissing 只补缺失：只写当前为空的字段，绝不覆盖已有值。
	ScrapeModeFillMissing ScrapeMode = "fill-missing"
)

// ParseScrapeMode 解析 handler 传入的档位字符串。
// 空值与无法识别的值一律按只补缺失处理（安全默认：不覆盖已有数据）。
func ParseScrapeMode(raw string) ScrapeMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "replace", "all", "overwrite":
		return ScrapeModeReplace
	default:
		return ScrapeModeFillMissing
	}
}

// LockableScrapeFields 可锁定字段清单（locked_fields JSON 数组里的字段名）。
// 只取刮削会写的字段；演职人员不在锁定范围内（由“缺失时才保存”规则保护）。
var LockableScrapeFields = []string{
	"title",
	"orig_title",
	"year",
	"overview",
	"genres",
	"poster",
	"backdrop",
	"rating",
}

var lockableScrapeFieldSet = func() map[string]bool {
	set := make(map[string]bool, len(LockableScrapeFields))
	for _, f := range LockableScrapeFields {
		set[f] = true
	}
	return set
}()

// ParseLockedFields 解析 locked_fields 列（JSON 数组）为字段集合。
// 空值、非法 JSON 一律视为空集合（不锁定）；同时容忍逗号分隔的手写值。
func ParseLockedFields(raw string) map[string]bool {
	set := make(map[string]bool)
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return set
	}
	var names []string
	if err := json.Unmarshal([]byte(trimmed), &names); err != nil {
		names = strings.Split(trimmed, ",")
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if lockableScrapeFieldSet[name] {
			set[name] = true
		}
	}
	return set
}

// NormalizeLockedFields 规范化待写入的 locked_fields：只保留可锁定字段，
// 去重并按 LockableScrapeFields 的顺序输出；空集合存为空字符串。
func NormalizeLockedFields(raw string) string {
	set := ParseLockedFields(raw)
	if len(set) == 0 {
		return ""
	}
	ordered := make([]string, 0, len(set))
	for _, f := range LockableScrapeFields {
		if set[f] {
			ordered = append(ordered, f)
		}
	}
	data, err := json.Marshal(ordered)
	if err != nil {
		return ""
	}
	return string(data)
}

// scrapeGuard 刮削写入守卫：一个 helper 统一查锁定与档位，
// 不在每个赋值点各写一遍。两档都跳过锁定字段；
// 只补缺失档 additionally 跳过已有值的字段。
type scrapeGuard struct {
	fillMissing bool
	lockedSet   map[string]bool
}

func newScrapeGuard(mode ScrapeMode, lockedRaw string) scrapeGuard {
	return scrapeGuard{
		fillMissing: mode == ScrapeModeFillMissing,
		lockedSet:   ParseLockedFields(lockedRaw),
	}
}

// skip 该字段是否跳过写入。empty 为字段当前是否为空
//（字符串判空、数值判 0、海报/背景图判路径为空）。
func (g scrapeGuard) skip(field string, empty bool) bool {
	if g.lockedSet[field] {
		return true
	}
	return g.fillMissing && !empty
}

// locked 该字段是否被锁定（不论档位）。
func (g scrapeGuard) locked(field string) bool {
	return g.lockedSet[field]
}

// providerModeToScrapeMode 把 ProviderChain 的 primary/supplement
// 口径翻译为刷新档位：primary=全部替换，其余=只补缺失。
func providerModeToScrapeMode(mode string) ScrapeMode {
	if mode == "primary" {
		return ScrapeModeReplace
	}
	return ScrapeModeFillMissing
}
