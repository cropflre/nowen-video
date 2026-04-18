package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// ==================== 诊断报告结构 ====================

// DiagnoseItem 单个媒体的诊断结果
type DiagnoseItem struct {
	MediaID       string `json:"media_id"`
	Title         string `json:"title"`
	OrigTitle     string `json:"orig_title"`
	FilePath      string `json:"file_path"`
	CollectionID  string `json:"collection_id"`
	CollectionName string `json:"collection_name,omitempty"`
	Year          int    `json:"year"`
	MediaType     string `json:"media_type"`

	// 诊断结果
	ExtractedBaseName    string   `json:"extracted_base_name,omitempty"`
	MatchLayer          string   `json:"match_layer,omitempty"` // L1序号/L2连接词/L2人名后缀/未匹配
	FailureReason       string   `json:"failure_reason,omitempty"`
	SuggestedBaseName   string   `json:"suggested_base_name,omitempty"`
	SuggestedCollection string   `json:"suggested_collection,omitempty"`
	FixActions          []string `json:"fix_actions,omitempty"`
}

// DiagnoseReport 诊断报告
type DiagnoseReport struct {
	TotalMovies    int                    `json:"total_movies"`
	MatchedCount   int                    `json:"matched_count"`
	UnmatchedCount int                    `json:"unmatched_count"`
	FixableCount   int                    `json:"fixable_count"`
	Items          []DiagnoseItem         `json:"items"`
	PatternStats   map[string]int         `json:"pattern_stats"`
	GroupPreview   map[string][]string    `json:"group_preview"` // baseName -> []title
	Suggestions    []string               `json:"suggestions"`
}

// ==================== 匹配算法（与 collection.go 同步） ====================

var (
	reChineseSequel    = regexp.MustCompile(`^(.{2,})\s*[0-9０-９一二三四五六七八九十百]+\s*$`)
	reEnglishSequel    = regexp.MustCompile(`(?i)^(.{2,})\s+(\d+|[IVX]+|Part\s+\d+|Chapter\s+\d+)\s*$`)
	reColonSequel      = regexp.MustCompile(`^(.{2,})\s*[:：]\s*.+$`)
	reParenSuffix      = regexp.MustCompile(`^(.{2,})\s*[（(]\s*(?:\d{4}|\d+)\s*[）)]\s*$`)
	reRomanSuffix      = regexp.MustCompile(`(?i)^(.{2,})\s+(?:II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)\s*$`)
	reChineseDelimiter = regexp.MustCompile(`^(.{2,}?)(之|[·•]|\s*[—–-]\s*)(.{2,})$`)
	reChineseDelimiterDe = regexp.MustCompile(`^(.{2,}?)的(.{3,})$`)
	reEnglishDelimiter = regexp.MustCompile(`(?i)^(.{2,}?)\s*[-–—:：]\s+(.{2,})$`)
	rePersonSuffix     = regexp.MustCompile(`^(.{2,}?)\s+(.{2,}(?:编|篇|版|章|辑|卷|期|作|風|style|edition))\s*$`)
	reSpaceSplit       = regexp.MustCompile(`^(.{2,}?)\s+(.{2,})\s*$`)
)

// extractBaseNameFull 完整提取基础名（返回 baseName 和匹配层级描述）
func extractBaseNameFull(title string) (baseName string, layer string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", ""
	}

	// 第一层：精确模式匹配
	patterns := []struct {
		re    *regexp.Regexp
		label string
	}{
		{reChineseSequel, "L1-中文序号"},
		{reEnglishSequel, "L1-英文续集"},
		{reRomanSuffix, "L1-罗马数字"},
		{reParenSuffix, "L1-括号编号"},
		{reColonSequel, "L1-冒号分隔"},
	}
	for _, p := range patterns {
		if matches := p.re.FindStringSubmatch(title); len(matches) >= 2 {
			bn := strings.TrimSpace(matches[1])
			if len([]rune(bn)) >= 2 {
				return normalizeBaseName(bn), p.label
			}
		}
	}

	// 第二层：连接词分割
	if matches := reChineseDelimiter.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix), "L2-中文连接词"
		}
	}
	if matches := reChineseDelimiterDe.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix), "L2-的连接词"
		}
	}
	if matches := reEnglishDelimiter.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix), "L2-英文分隔符"
		}
	}

	// 第二层补充：人名/副标题后缀
	if matches := rePersonSuffix.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix), "L2-人名后缀"
		}
	}

	// 第三层：通用空格分割
	if matches := reSpaceSplit.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix), "L3-空格分割"
		}
	}

	return "", "未匹配"
}

func normalizeBaseName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimRight(name, " -_·.、，,")
	name = strings.TrimRight(name, ":：")
	name = strings.TrimSpace(name)
	allPunct := true
	for _, r := range name {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			allPunct = false
			break
		}
	}
	if allPunct {
		return ""
	}
	return name
}

// normalizeForMatch 标准化名称用于模糊匹配
func normalizeForMatch(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		if unicode.IsSpace(r) || unicode.IsPunct(r) {
			continue
		}
		switch {
		case r >= 'Ａ' && r <= 'Ｚ':
			r = r - 'Ａ' + 'A'
		case r >= 'ａ' && r <= 'ｚ':
			r = r - 'ａ' + 'a'
		case r >= '０' && r <= '９':
			r = r - '０' + '0'
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// ==================== 诊断逻辑 ====================

func diagnose(db *gorm.DB, fixMode bool) *DiagnoseReport {
	// 查询所有无合集的电影
	var movies []model.Media
	db.Where("media_type = 'movie' AND (collection_id = '' OR collection_id IS NULL)").
		Where("title != ''").
		Order("title ASC").
		Find(&movies)

	// 查询所有已有合集
	var collections []model.MovieCollection
	db.Find(&collections)
	collMap := make(map[string]string) // id -> name
	for _, c := range collections {
		collMap[c.ID] = c.Name
	}

	report := &DiagnoseReport{
		TotalMovies:  len(movies),
		PatternStats: make(map[string]int),
		GroupPreview: make(map[string][]string),
	}

	// 基础名分组
	baseNameGroups := make(map[string][]DiagnoseItem)
	var unmatchedItems []DiagnoseItem

	for _, m := range movies {
		baseName, layer := extractBaseNameFull(m.Title)
		report.PatternStats[layer]++

		item := DiagnoseItem{
			MediaID:     m.ID,
			Title:       m.Title,
			OrigTitle:   m.OrigTitle,
			FilePath:    m.FilePath,
			CollectionID: m.CollectionID,
			Year:        m.Year,
			MediaType:   m.MediaType,
		}

		if baseName != "" {
			item.ExtractedBaseName = baseName
			item.MatchLayer = layer

			// 检查是否已有同名合集
			normBase := normalizeForMatch(baseName)
			foundColl := false
			for _, c := range collections {
				if normalizeForMatch(c.Name) == normBase {
					item.SuggestedCollection = c.Name
					foundColl = true
					break
				}
			}
			if !foundColl {
				item.SuggestedCollection = baseName
			}

			baseNameGroups[baseName] = append(baseNameGroups[baseName], item)
		} else {
			item.MatchLayer = "未匹配"
			item.FailureReason = analyzeFailureReason(m)
			item.SuggestedBaseName = suggestBaseName(m)
			if item.SuggestedBaseName != "" {
				item.FixActions = []string{fmt.Sprintf("重命名标题为 '%s' 或添加到合集 '%s'", m.Title, item.SuggestedBaseName)}
			}
			unmatchedItems = append(unmatchedItems, item)
		}

		report.Items = append(report.Items, item)
	}

	// 统计匹配/未匹配
	for _, item := range report.Items {
		if item.ExtractedBaseName != "" {
			report.MatchedCount++
		} else {
			report.UnmatchedCount++
		}
	}

	// 生成分组预览
	for baseName, items := range baseNameGroups {
		titles := make([]string, 0, len(items))
		for _, it := range items {
			titles = append(titles, it.Title)
		}
		report.GroupPreview[baseName] = titles
	}

	// 可修复数量 = 有建议基础名的未匹配数 + 可归入新合集的匹配数
	report.FixableCount = 0
	for _, item := range unmatchedItems {
		if item.SuggestedBaseName != "" {
			report.FixableCount++
		}
	}
	for _, items := range baseNameGroups {
		if len(items) >= 2 {
			report.FixableCount += len(items)
		}
	}

	// 生成建议
	report.Suggestions = generateSuggestions(report, baseNameGroups, unmatchedItems)

	// 自动修复模式
	if fixMode {
		applyFixes(db, baseNameGroups, unmatchedItems)
	}

	return report
}

// analyzeFailureReason 分析匹配失败的具体原因
func analyzeFailureReason(m model.Media) string {
	title := strings.TrimSpace(m.Title)

	// 检查标题是否太短
	if len([]rune(title)) < 2 {
		return "标题太短（少于2个字符），无法提取系列基础名"
	}

	// 检查是否包含特殊字符导致匹配失败
	hasSpecial := false
	for _, r := range title {
		if unicode.IsPunct(r) && r != '·' && r != '-' && r != '—' && r != ':' && r != '：' {
			hasSpecial = true
			break
		}
	}
	if hasSpecial {
		return "标题包含特殊标点符号，无法被现有模式匹配"
	}

	// 检查是否缺少空格分隔
	if !strings.Contains(title, " ") && !strings.Contains(title, "　") {
		// 无空格，可能是连续文字标题
		return "标题为连续文字，无分隔符或序号后缀，无法自动提取系列基础名"
	}

	return "标题格式不符合现有匹配模式（无序号、无连接词、无分隔符）"
}

// suggestBaseName 为未匹配的标题建议基础名
func suggestBaseName(m model.Media) string {
	title := strings.TrimSpace(m.Title)

	// 尝试提取空格前的部分作为基础名
	if idx := strings.IndexAny(title, " 　"); idx > 0 {
		prefix := title[:idx]
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix)
		}
	}

	// 尝试移除括号内的内容
	re := regexp.MustCompile(`^(.+?)[（(].+[）)]$`)
	if matches := re.FindStringSubmatch(title); len(matches) >= 2 {
		return normalizeBaseName(matches[1])
	}

	return ""
}

// generateSuggestions 生成优化建议
func generateSuggestions(report *DiagnoseReport, baseNameGroups map[string][]DiagnoseItem, _ []DiagnoseItem) []string {
	var suggestions []string

	// 可自动归入合集的分组
	autoGroupCount := 0
	for _, items := range baseNameGroups {
		if len(items) >= 2 {
			autoGroupCount++
		}
	}
	if autoGroupCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("发现 %d 组可自动归入合集的电影系列，建议执行 rematch 操作", autoGroupCount))
	}

	// 单独匹配到基础名但只有1部的
	singleCount := 0
	for _, items := range baseNameGroups {
		if len(items) == 1 {
			singleCount++
		}
	}
	if singleCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("有 %d 部电影单独匹配到系列基础名但无同伴，可能需要补充文件或手动关联", singleCount))
	}

	// 完全未匹配的
	if report.UnmatchedCount > 0 {
		suggestions = append(suggestions, fmt.Sprintf("有 %d 部电影完全无法匹配，建议检查标题格式或手动指定合集", report.UnmatchedCount))
	}

	// 匹配模式统计
	suggestions = append(suggestions, "\n=== 匹配模式统计 ===")
	for layer, count := range report.PatternStats {
		suggestions = append(suggestions, fmt.Sprintf("  %s: %d 部", layer, count))
	}

	return suggestions
}

// applyFixes 执行自动修复
func applyFixes(db *gorm.DB, baseNameGroups map[string][]DiagnoseItem, unmatchedItems []DiagnoseItem) {
	fmt.Println("\n========== 执行自动修复 ==========")

	fixedCount := 0

	// 处理可归入合集的分组（>=2 部电影共享基础名）
	for baseName, items := range baseNameGroups {
		if len(items) < 2 {
			continue
		}

		// 查找或创建合集
		var coll model.MovieCollection
		result := db.Where("name = ?", baseName).First(&coll)
		if result.Error != nil {
			// 尝试模糊匹配
			var allColls []model.MovieCollection
			db.Find(&allColls)
			normBase := normalizeForMatch(baseName)
			for _, c := range allColls {
				if normalizeForMatch(c.Name) == normBase {
					coll = c
					break
				}
			}
		}

		if coll.ID == "" {
			// 创建新合集
			coll = model.MovieCollection{
				Name:        baseName,
				PosterPath:  "",
				MediaCount:  len(items),
				AutoMatched: true,
			}
			if err := db.Create(&coll).Error; err != nil {
				fmt.Printf("  [错误] 创建合集 '%s' 失败: %v\n", baseName, err)
				continue
			}
			fmt.Printf("  [新建] 合集 '%s' (ID: %s)\n", baseName, coll.ID)
		}

		// 关联电影
		for _, item := range items {
			if item.CollectionID != "" {
				continue
			}
			if err := db.Model(&model.Media{}).Where("id = ?", item.MediaID).
				Update("collection_id", coll.ID).Error; err != nil {
				fmt.Printf("  [错误] 关联 '%s' -> '%s' 失败: %v\n", item.Title, baseName, err)
			} else {
				fmt.Printf("  [关联] '%s' -> 合集 '%s'\n", item.Title, baseName)
				fixedCount++
			}
		}

		// 更新合集计数
		var count int64
		db.Model(&model.Media{}).Where("collection_id = ?", coll.ID).Count(&count)
		db.Model(&model.MovieCollection{}).Where("id = ?", coll.ID).Update("media_count", count)
	}

	// 处理未匹配但可修复的（建议基础名匹配到已有合集的）
	for _, item := range unmatchedItems {
		if item.SuggestedBaseName == "" {
			continue
		}

		// 尝试匹配已有合集
		normSugg := normalizeForMatch(item.SuggestedBaseName)
		var allColls []model.MovieCollection
		db.Find(&allColls)

		var matchedColl *model.MovieCollection
		for i := range allColls {
			if normalizeForMatch(allColls[i].Name) == normSugg {
				matchedColl = &allColls[i]
				break
			}
		}

		if matchedColl != nil {
			if err := db.Model(&model.Media{}).Where("id = ?", item.MediaID).
				Update("collection_id", matchedColl.ID).Error; err != nil {
				fmt.Printf("  [错误] 修复关联 '%s' -> '%s' 失败: %v\n", item.Title, matchedColl.Name, err)
			} else {
				fmt.Printf("  [修复] '%s' -> 合集 '%s' (建议基础名: %s)\n", item.Title, matchedColl.Name, item.SuggestedBaseName)
				fixedCount++
			}
		}
	}

	fmt.Printf("\n========== 修复完成：共修复 %d 条关联 ==========\n", fixedCount)
}

// ==================== 报告输出 ====================

func printReport(report *DiagnoseReport) {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║          媒体文件合集匹配诊断报告                            ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")

	// 总览
	fmt.Println("\n── 总览 ──")
	fmt.Printf("  无合集的电影总数: %d\n", report.TotalMovies)
	fmt.Printf("  可自动匹配:       %d\n", report.MatchedCount)
	fmt.Printf("  无法匹配:         %d\n", report.UnmatchedCount)
	fmt.Printf("  可修复数量:       %d\n", report.FixableCount)

	// 匹配模式统计
	fmt.Println("\n── 匹配模式统计 ──")
	keys := make([]string, 0, len(report.PatternStats))
	for k := range report.PatternStats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-16s %d 部\n", k+":", report.PatternStats[k])
	}

	// 可归入合集的分组
	fmt.Println("\n── 可自动归入合集的分组 ──")
	sortedGroups := make([]string, 0, len(report.GroupPreview))
	for k := range report.GroupPreview {
		sortedGroups = append(sortedGroups, k)
	}
	sort.Strings(sortedGroups)

	groupIdx := 0
	for _, baseName := range sortedGroups {
		titles := report.GroupPreview[baseName]
		if len(titles) < 2 {
			continue
		}
		groupIdx++
		fmt.Printf("\n  [%d] 合集名: %s (%d 部)\n", groupIdx, baseName, len(titles))
		for i, t := range titles {
			fmt.Printf("      ├─ %s\n", t)
			_ = i
		}
	}

	if groupIdx == 0 {
		fmt.Println("  (无)")
	}

	// 单独匹配的电影
	fmt.Println("\n── 单独匹配（仅1部，无法自动建合集）──")
	singleCount := 0
	for _, baseName := range sortedGroups {
		titles := report.GroupPreview[baseName]
		if len(titles) == 1 {
			singleCount++
			fmt.Printf("  %-30s -> %s\n", titles[0], baseName)
		}
	}
	if singleCount == 0 {
		fmt.Println("  (无)")
	}

	// 无法匹配的电影
	fmt.Println("\n── 无法匹配的电影 ──")
	unmatchedCount := 0
	for _, item := range report.Items {
		if item.ExtractedBaseName == "" {
			unmatchedCount++
			fmt.Printf("  %-40s  原因: %s\n", item.Title, item.FailureReason)
			if item.SuggestedBaseName != "" {
				fmt.Printf("    └─ 建议: 基础名 '%s'\n", item.SuggestedBaseName)
			}
		}
	}
	if unmatchedCount == 0 {
		fmt.Println("  (无)")
	}

	// 建议
	fmt.Println("\n── 优化建议 ──")
	for _, s := range report.Suggestions {
		fmt.Printf("  %s\n", s)
	}

	// 具体案例：《少女教育》系列
	fmt.Println("\n── 具体案例：《少女教育》系列 ──")
	for _, item := range report.Items {
		if strings.Contains(item.Title, "少女教育") {
			status := "未匹配"
			if item.ExtractedBaseName != "" {
				status = fmt.Sprintf("已匹配 -> '%s' (%s)", item.ExtractedBaseName, item.MatchLayer)
			}
			fmt.Printf("  %-40s  %s\n", item.Title, status)
		}
	}
}

func printJSONReport(report *DiagnoseReport) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Printf("JSON 序列化失败: %v\n", err)
		return
	}
	fmt.Println(string(data))
}

// ==================== main ====================

func main() {
	// 解析命令行参数
	fixMode := false
	jsonOutput := false
	filterKeyword := ""

	for _, arg := range os.Args[1:] {
		switch arg {
		case "--fix":
			fixMode = true
		case "--json":
			jsonOutput = true
		case "--help", "-h":
			fmt.Println("用法: diagnose [选项]")
			fmt.Println()
			fmt.Println("诊断媒体文件合集匹配失败的问题")
			fmt.Println()
			fmt.Println("选项:")
			fmt.Println("  --fix     执行自动修复（创建合集并关联电影）")
			fmt.Println("  --json    以 JSON 格式输出报告")
			fmt.Println("  --filter  按关键词过滤")
			fmt.Println("  --help    显示帮助信息")
			return
		default:
			if !strings.HasPrefix(arg, "--") {
				filterKeyword = arg
			}
		}
	}

	if fixMode {
		fmt.Println("⚠️  自动修复模式已启用，将修改数据库！")
		fmt.Print("确认继续？(y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("已取消")
			return
		}
	}

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("加载配置失败: %v\n", err)
		fmt.Println("尝试使用默认数据库路径 ./data/nowen.db")
		cfg = &config.Config{}
		cfg.Database.DBPath = "./data/nowen.db"
	}

	// 连接数据库
	db, err := gorm.Open(sqlite.Open(cfg.GetDBDSN()), &gorm.Config{})
	if err != nil {
		fmt.Printf("连接数据库失败: %v\n", err)
		os.Exit(1)
	}

	// 运行诊断
	report := diagnose(db, fixMode)

	// 按关键词过滤
	if filterKeyword != "" {
		var filtered []DiagnoseItem
		for _, item := range report.Items {
			if strings.Contains(item.Title, filterKeyword) ||
				strings.Contains(item.ExtractedBaseName, filterKeyword) ||
				strings.Contains(item.SuggestedBaseName, filterKeyword) {
				filtered = append(filtered, item)
			}
		}
		report.Items = filtered

		filteredGroups := make(map[string][]string)
		for k, titles := range report.GroupPreview {
			if strings.Contains(k, filterKeyword) {
				filteredGroups[k] = titles
			}
		}
		report.GroupPreview = filteredGroups
	}

	// 输出报告
	if jsonOutput {
		printJSONReport(report)
	} else {
		printReport(report)
	}

	// 表格输出修正前后对比
	if fixMode {
		fmt.Println("\n── 修正前后对比 ──")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "标题\t修正前\t修正后")
		fmt.Fprintln(w, "────\t──────\t──────")
		for _, item := range report.Items {
			if item.ExtractedBaseName != "" || item.SuggestedBaseName != "" {
				before := "无合集"
				after := item.SuggestedCollection
				if after == "" {
					after = item.SuggestedBaseName
				}
				if after == "" {
					after = "（仍无匹配）"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\n", item.Title, before, after)
			}
		}
		w.Flush()
	}
}
