// Package service 番号多数据源聚合器
// 借鉴自 mdcx-master 的 core_main.py get_data 函数
// 思路：不同数据源各有优劣，通过并发调用 + 字段优先级合并，获取最完整元数据
//
// 策略：
//   1. 按各字段优先级从不同数据源取值（如封面优先 Fanza，简介优先 JAV321）
//   2. 支持并发调用（大幅降低总耗时）
//   3. 失败源自动跳过，不影响其他源
package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ==================== 字段优先级配置 ====================

// fieldPriority 各字段的数据源优先级配置
// 借鉴自 mdcx 的 config.ini field_priority 设计
var fieldPriority = map[string][]string{
	"title":         {"fanza", "javdb", "javbus", "jav321", "freejavbt", "mgstage", "fc2hub"},
	"original_title":{"fanza", "javdb", "javbus", "mgstage"},
	"plot":          {"jav321", "fanza", "javdb", "javbus", "freejavbt", "mgstage", "fc2hub"},
	"cover":         {"fanza", "javbus", "mgstage", "freejavbt", "javdb", "jav321", "fc2hub"},
	"thumb":         {"fanza", "freejavbt", "javbus", "javdb"},
	"trailer":       {"fanza", "mgstage", "javdb", "freejavbt", "javbus", "jav321"},
	"release_date":  {"fanza", "javbus", "javdb", "mgstage", "freejavbt", "jav321"},
	"duration":      {"fanza", "javbus", "mgstage", "javdb", "freejavbt", "jav321"},
	"studio":        {"fanza", "javbus", "mgstage", "freejavbt", "javdb", "jav321"},
	"label":         {"fanza", "mgstage", "javbus", "freejavbt"},
	"series":        {"fanza", "javbus", "javdb", "mgstage", "freejavbt", "jav321"},
	"director":      {"fanza", "mgstage", "freejavbt", "javbus", "javdb"},
	"rating":        {"javdb", "fanza", "jav321", "freejavbt"},
	"actresses":     {"fanza", "javbus", "javdb", "mgstage", "freejavbt", "jav321"},
	"actor_photos":  {"javbus", "javdb", "freejavbt"},
	"genres":        {"javbus", "fanza", "javdb", "freejavbt", "jav321", "mgstage"},
	"extra_fanart":  {"fanza", "javbus", "javdb", "freejavbt", "jav321", "mgstage"},
}

// ==================== 聚合刮削 ====================

// ScrapeByCodeAggregated 多数据源并发刮削并聚合
// 与 ScrapeByCode 的区别：
//   - ScrapeByCode 是按顺序串行，第一个成功就返回
//   - ScrapeByCodeAggregated 是并发所有源，按字段优先级合并得到最完整结果
//
// aggregated 模式更慢但数据更完整，适合精刮场景
func (s *AdultScraperService) ScrapeByCodeAggregated(code string) (*AdultMetadata, map[string]*AdultMetadata, error) {
	if !s.IsEnabled() {
		return nil, nil, fmtErrf("番号刮削功能未启用")
	}
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, nil, fmtErrf("番号不能为空")
	}

	s.logger.Infof("开始聚合刮削: %s", code)

	// 组装待执行的数据源列表
	type scrapeFn func(string) (*AdultMetadata, error)
	sources := map[string]scrapeFn{}
	if s.cfg.AdultScraper.EnableJavBus {
		sources["javbus"] = s.scrapeJavBus
	}
	if s.cfg.AdultScraper.EnableJavDB {
		sources["javdb"] = s.scrapeJavDB
	}
	if s.cfg.AdultScraper.EnableFreejavbt {
		sources["freejavbt"] = s.scrapeFreejavbt
	}
	if s.cfg.AdultScraper.EnableJav321 {
		sources["jav321"] = s.scrapeJav321
	}
	if s.cfg.AdultScraper.EnableFanza {
		sources["fanza"] = s.scrapeFanza
	}
	if s.cfg.AdultScraper.EnableMGStage {
		sources["mgstage"] = s.scrapeMGStage
	}
	if s.cfg.AdultScraper.EnableFC2Hub {
		sources["fc2hub"] = s.scrapeFC2Hub
	}

	if len(sources) == 0 {
		return nil, nil, fmtErrf("未启用任何数据源")
	}

	// 并发刮削
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results = make(map[string]*AdultMetadata, len(sources))
		start   = time.Now()
	)
	for name, fn := range sources {
		name, fn := name, fn
		wg.Add(1)
		go func() {
			defer wg.Done()
			// 每个源独立超时保护（防止某个源拖慢整体）
			done := make(chan struct{})
			var (
				meta *AdultMetadata
				err  error
			)
			go func() {
				meta, err = fn(code)
				close(done)
			}()
			select {
			case <-done:
				if err == nil && meta != nil {
					mu.Lock()
					results[name] = meta
					mu.Unlock()
					s.logger.Debugf("聚合刮削 %s 成功: %s", name, code)
				} else if err != nil {
					s.logger.Debugf("聚合刮削 %s 失败: %s - %v", name, code, err)
				}
			case <-time.After(20 * time.Second):
				s.logger.Debugf("聚合刮削 %s 超时: %s", name, code)
			}
		}()
	}
	wg.Wait()

	s.logger.Infof("聚合刮削完成: %s, 成功源数 %d/%d, 耗时 %v",
		code, len(results), len(sources), time.Since(start))

	if len(results) == 0 {
		return nil, nil, fmtErrf("所有数据源均未找到番号 %s", code)
	}

	// 按字段优先级合并
	merged := mergeAdultMetadata(code, results)

	// 统一执行规范化和翻译
	NormalizeMetadata(merged)
	s.TranslateAdultMetadata(merged)

	return merged, results, nil
}

// mergeAdultMetadata 按字段优先级合并多个数据源的结果
func mergeAdultMetadata(code string, results map[string]*AdultMetadata) *AdultMetadata {
	merged := &AdultMetadata{
		Code:        code,
		Genres:      []string{},
		Actresses:   []string{},
		ActorPhotos: make(map[string]string),
		ExtraFanart: []string{},
		Source:      "aggregated",
	}

	// 单值字段按优先级填充
	pickString := func(field string, getter func(*AdultMetadata) string, setter func(string)) {
		prio := fieldPriority[field]
		for _, src := range prio {
			if meta, ok := results[src]; ok && meta != nil {
				if v := strings.TrimSpace(getter(meta)); v != "" {
					setter(v)
					return
				}
			}
		}
		// fallback：用任一非空值
		for _, meta := range results {
			if v := strings.TrimSpace(getter(meta)); v != "" {
				setter(v)
				return
			}
		}
	}
	pickString("title", func(m *AdultMetadata) string { return m.Title }, func(v string) { merged.Title = v })
	pickString("original_title", func(m *AdultMetadata) string { return m.OriginalTitle }, func(v string) { merged.OriginalTitle = v })
	pickString("plot", func(m *AdultMetadata) string { return m.Plot }, func(v string) { merged.Plot = v })
	pickString("cover", func(m *AdultMetadata) string { return m.Cover }, func(v string) { merged.Cover = v })
	pickString("thumb", func(m *AdultMetadata) string { return m.Thumb }, func(v string) { merged.Thumb = v })
	pickString("trailer", func(m *AdultMetadata) string { return m.Trailer }, func(v string) { merged.Trailer = v })
	pickString("release_date", func(m *AdultMetadata) string { return m.ReleaseDate }, func(v string) { merged.ReleaseDate = v })
	pickString("studio", func(m *AdultMetadata) string { return m.Studio }, func(v string) { merged.Studio = v })
	pickString("label", func(m *AdultMetadata) string { return m.Label }, func(v string) { merged.Label = v })
	pickString("series", func(m *AdultMetadata) string { return m.Series }, func(v string) { merged.Series = v })
	pickString("director", func(m *AdultMetadata) string { return m.Director }, func(v string) { merged.Director = v })

	// Duration：取非零最大
	{
		prio := fieldPriority["duration"]
		for _, src := range prio {
			if meta, ok := results[src]; ok && meta != nil && meta.Duration > 0 {
				merged.Duration = meta.Duration
				break
			}
		}
	}
	// Rating：按优先级取第一个非零值
	{
		prio := fieldPriority["rating"]
		for _, src := range prio {
			if meta, ok := results[src]; ok && meta != nil && meta.Rating > 0 {
				merged.Rating = meta.Rating
				break
			}
		}
	}

	// 演员：合并去重（按顺序：fanza > javbus > ...）
	{
		prio := fieldPriority["actresses"]
		seen := make(map[string]struct{})
		for _, src := range prio {
			if meta, ok := results[src]; ok && meta != nil {
				for _, a := range meta.Actresses {
					a = strings.TrimSpace(a)
					if a == "" {
						continue
					}
					if _, exist := seen[a]; !exist {
						seen[a] = struct{}{}
						merged.Actresses = append(merged.Actresses, a)
					}
				}
			}
		}
	}

	// 演员头像：合并（按优先级覆盖）
	{
		prio := fieldPriority["actor_photos"]
		// 倒序遍历：低优先级先写入，高优先级覆盖
		for i := len(prio) - 1; i >= 0; i-- {
			src := prio[i]
			if meta, ok := results[src]; ok && meta != nil {
				for k, v := range meta.ActorPhotos {
					if v != "" {
						merged.ActorPhotos[k] = v
					}
				}
			}
		}
	}

	// 类型标签：合并去重
	{
		prio := fieldPriority["genres"]
		seen := make(map[string]struct{})
		for _, src := range prio {
			if meta, ok := results[src]; ok && meta != nil {
				for _, g := range meta.Genres {
					g = strings.TrimSpace(g)
					if g == "" {
						continue
					}
					if _, exist := seen[g]; !exist {
						seen[g] = struct{}{}
						merged.Genres = append(merged.Genres, g)
					}
				}
			}
		}
	}

	// 剧照：合并去重（最多保留 30 张）
	{
		prio := fieldPriority["extra_fanart"]
		seen := make(map[string]struct{})
		for _, src := range prio {
			if meta, ok := results[src]; ok && meta != nil {
				for _, f := range meta.ExtraFanart {
					if f == "" {
						continue
					}
					if _, exist := seen[f]; !exist {
						seen[f] = struct{}{}
						merged.ExtraFanart = append(merged.ExtraFanart, f)
						if len(merged.ExtraFanart) >= 30 {
							break
						}
					}
				}
			}
			if len(merged.ExtraFanart) >= 30 {
				break
			}
		}
	}

	// 记录参与的数据源列表到 Source 字段
	srcNames := make([]string, 0, len(results))
	for k := range results {
		srcNames = append(srcNames, k)
	}
	sort.Strings(srcNames)
	merged.Source = "aggregated:" + strings.Join(srcNames, "+")

	return merged
}

// fmtErrf 使用 fmt.Errorf 的快捷别名
func fmtErrf(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
