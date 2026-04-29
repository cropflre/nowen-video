// Package service 番号刮削失败分析与数据源评分
// 基于 TaskStore 历史数据，为每个数据源生成评分
// 帮助用户了解：
//   - 哪个数据源命中率最高
//   - 哪些番号前缀最常失败
//   - 哪个时段成功率最好
package service

import (
	"sort"
	"strings"
	"time"
)

// SourceStats 单个数据源的统计
type SourceStats struct {
	Source       string  `json:"source"`
	Total        int     `json:"total"`
	Success      int     `json:"success"`
	Failed       int     `json:"failed"`
	SuccessRate  float64 `json:"success_rate"`  // 0.0-1.0
	AvgLatencyMS int64   `json:"avg_latency_ms,omitempty"`
}

// PrefixStats 番号前缀统计（SSIS、MIDV 等）
type PrefixStats struct {
	Prefix      string  `json:"prefix"`
	Total       int     `json:"total"`
	Success     int     `json:"success"`
	Failed      int     `json:"failed"`
	SuccessRate float64 `json:"success_rate"`
}

// AdultScrapeReport 刮削总体报表
type AdultScrapeReport struct {
	Period         string        `json:"period"` // "7d" / "30d" / "all"
	TotalProcessed int           `json:"total_processed"`
	TotalSuccess   int           `json:"total_success"`
	TotalFailed    int           `json:"total_failed"`
	OverallRate    float64       `json:"overall_rate"`

	BySource      []SourceStats `json:"by_source"`
	ByPrefix      []PrefixStats `json:"by_prefix"`       // 按番号前缀统计
	TopFailures   []string      `json:"top_failures"`     // 最常失败的番号
	BestHours     []int         `json:"best_hours"`       // 成功率最高的时段

	GeneratedAt time.Time `json:"generated_at"`
}

// BuildReport 从 TaskStore 历史数据生成报表
func BuildReport(store *AdultTaskStore, days int) AdultScrapeReport {
	if store == nil {
		return emptyReport(days)
	}
	cutoff := time.Time{}
	if days > 0 {
		cutoff = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	}

	var (
		allResults    []AdultBatchItemResult
		bySrcTotal    = map[string]int{}
		bySrcSuccess  = map[string]int{}
		byPrefixTotal = map[string]int{}
		byPrefixSucc  = map[string]int{}
		byHourTotal   = map[int]int{}
		byHourSucc    = map[int]int{}
		failCount     = map[string]int{}
	)

	for _, t := range store.List() {
		for _, r := range t.Results {
			if !cutoff.IsZero() && r.FinishedAt.Before(cutoff) {
				continue
			}
			allResults = append(allResults, r)

			// 数据源统计
			if r.Source != "" {
				bySrcTotal[r.Source]++
				if r.Status == "success" {
					bySrcSuccess[r.Source]++
				}
			}
			// 番号前缀统计（去除数字部分）
			prefix := extractCodePrefix(r.Code)
			if prefix != "" {
				byPrefixTotal[prefix]++
				if r.Status == "success" {
					byPrefixSucc[prefix]++
				}
			}
			// 时段统计
			h := r.FinishedAt.Hour()
			byHourTotal[h]++
			if r.Status == "success" {
				byHourSucc[h]++
			}
			// 失败番号计数
			if r.Status == "failed" && r.Code != "" {
				failCount[r.Code]++
			}
		}
	}

	report := AdultScrapeReport{
		Period:      periodLabel(days),
		GeneratedAt: time.Now(),
	}
	// 总览
	for _, r := range allResults {
		switch r.Status {
		case "success":
			report.TotalSuccess++
		case "failed":
			report.TotalFailed++
		}
		report.TotalProcessed++
	}
	if report.TotalProcessed > 0 {
		report.OverallRate = float64(report.TotalSuccess) / float64(report.TotalProcessed)
	}

	// 数据源
	for src, total := range bySrcTotal {
		succ := bySrcSuccess[src]
		rate := 0.0
		if total > 0 {
			rate = float64(succ) / float64(total)
		}
		report.BySource = append(report.BySource, SourceStats{
			Source:      src,
			Total:       total,
			Success:     succ,
			Failed:      total - succ,
			SuccessRate: rate,
		})
	}
	sort.Slice(report.BySource, func(i, j int) bool {
		return report.BySource[i].SuccessRate > report.BySource[j].SuccessRate
	})

	// 前缀
	for p, total := range byPrefixTotal {
		succ := byPrefixSucc[p]
		rate := 0.0
		if total > 0 {
			rate = float64(succ) / float64(total)
		}
		report.ByPrefix = append(report.ByPrefix, PrefixStats{
			Prefix:      p,
			Total:       total,
			Success:     succ,
			Failed:      total - succ,
			SuccessRate: rate,
		})
	}
	sort.Slice(report.ByPrefix, func(i, j int) bool {
		return report.ByPrefix[i].Total > report.ByPrefix[j].Total
	})
	if len(report.ByPrefix) > 20 {
		report.ByPrefix = report.ByPrefix[:20]
	}

	// Top 10 失败番号
	type kv struct {
		code  string
		count int
	}
	pairs := []kv{}
	for c, n := range failCount {
		pairs = append(pairs, kv{c, n})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
	for i, p := range pairs {
		if i >= 10 {
			break
		}
		report.TopFailures = append(report.TopFailures, p.code)
	}

	// 最佳时段（成功率最高的 3 个小时）
	type hr struct {
		hour int
		rate float64
	}
	hrs := []hr{}
	for h, total := range byHourTotal {
		if total < 3 {
			continue
		}
		succ := byHourSucc[h]
		rate := float64(succ) / float64(total)
		hrs = append(hrs, hr{h, rate})
	}
	sort.Slice(hrs, func(i, j int) bool { return hrs[i].rate > hrs[j].rate })
	for i, h := range hrs {
		if i >= 3 {
			break
		}
		report.BestHours = append(report.BestHours, h.hour)
	}

	// 确保切片字段为非 nil（JSON 序列化为 []，避免前端 .map/.length 崩溃）
	if report.BySource == nil {
		report.BySource = []SourceStats{}
	}
	if report.ByPrefix == nil {
		report.ByPrefix = []PrefixStats{}
	}
	if report.TopFailures == nil {
		report.TopFailures = []string{}
	}
	if report.BestHours == nil {
		report.BestHours = []int{}
	}

	return report
}

// emptyReport 返回一份字段完整（切片非 nil）的空报表
func emptyReport(days int) AdultScrapeReport {
	return AdultScrapeReport{
		Period:      periodLabel(days),
		GeneratedAt: time.Now(),
		BySource:    []SourceStats{},
		ByPrefix:    []PrefixStats{},
		TopFailures: []string{},
		BestHours:   []int{},
	}
}

// extractCodePrefix 从番号中提取字母前缀（SSIS-001 -> SSIS）
func extractCodePrefix(code string) string {
	if code == "" {
		return ""
	}
	code = strings.ToUpper(code)
	// 找到第一个分隔符或数字
	for i, ch := range code {
		if ch == '-' || (ch >= '0' && ch <= '9') {
			if i == 0 {
				return ""
			}
			return code[:i]
		}
	}
	return code
}

func periodLabel(days int) string {
	switch days {
	case 0:
		return "all"
	case 7:
		return "7d"
	case 30:
		return "30d"
	default:
		return "custom"
	}
}
