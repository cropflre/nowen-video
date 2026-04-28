// Package matcher 提供电影系列合集标题匹配算法。
//
// 该包将标题匹配的正则表达式与核心函数集中维护，供合集自动匹配服务
// 和诊断脚本共同使用，避免在多处维护相同的算法（一处修改、多处同步的风险）。
package matcher

import (
	"regexp"
	"strings"
	"unicode"
)

// 用于匹配标题中的数字序号和分隔模式
var (
	// ===== 第一层：精确续集模式 =====
	// 匹配中文数字序号：逃学威龙1、逃学威龙2、速度与激情3
	ReChineseSequel = regexp.MustCompile(`^(.{2,})\s*[0-9０-９一二三四五六七八九十百]+\s*$`)
	// 匹配英文续集模式：Toy Story 2, Iron Man 3, Fast & Furious 7
	ReEnglishSequel = regexp.MustCompile(`(?i)^(.{2,})\s+(\d+|[IVX]+|Part\s+\d+|Chapter\s+\d+)\s*$`)
	// 匹配带冒号的续集：Alien: Resurrection, Batman: The Dark Knight
	ReColonSequel = regexp.MustCompile(`^(.{2,})\s*[:：]\s*.+$`)
	// 匹配括号中的年份或编号：电影名 (2020)
	ReParenSuffix = regexp.MustCompile(`^(.{2,})\s*[（(]\s*(?:\d{4}|\d+)\s*[）)]\s*$`)
	// 匹配罗马数字后缀
	ReRomanSuffix = regexp.MustCompile(`(?i)^(.{2,})\s+(?:II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)\s*$`)

	// ===== 第二层：连接词分割模式 =====
	// 匹配中文连接词模式："哈哈哈之我真是醉了"、"名侦探柯南之xxx"、"熊出没·原始时代"
	// 支持的连接词：之、·、—、-
	ReChineseDelimiter = regexp.MustCompile(`^(.{2,}?)(之|[·•]|\s*[—–-]\s*)(.{2,})$`)
	// "的"作为连接词时，要求后半部分至少3字（避免把"我的家"拆成"我"+"家"）
	ReChineseDelimiterDe = regexp.MustCompile(`^(.{2,}?)的(.{3,})$`)
	// 匹配英文分隔符模式："Harry Potter - The Chamber of Secrets"
	ReEnglishDelimiter = regexp.MustCompile(`(?i)^(.{2,}?)\s*[-–—:：]\s+(.{2,})$`)

	// ===== 第二层补充：人名/副标题后缀模式 =====
	// 匹配"基础名 + 空格 + XX编/篇/版/章/辑/卷/期"模式
	// 例如："少女教育 稻垣纱衣编" -> "少女教育"
	RePersonSuffix = regexp.MustCompile(`^(.{2,}?)\s+(.{2,}(?:编|篇|版|章|辑|卷|期|作|風|style|edition))\s*$`)

	// ===== 第三层：通用空格分割模式 =====
	// 匹配"基础名 + 空格 + 任意后缀"的通用模式
	// 这是最后一道防线，只要标题中包含空格且基础名合法，就提取空格前的基础名
	ReSpaceSplit = regexp.MustCompile(`^(.{2,}?)\s+(.{2,})\s*$`)
)

// ExtractSeriesBaseName 从电影标题中提取系列基础名（第一层：精确模式匹配）。
// 例如：
//
//	"逃学威龙1"              -> "逃学威龙"
//	"速度与激情7"            -> "速度与激情"
//	"Toy Story 2"            -> "Toy Story"
//	"Iron Man 3"             -> "Iron Man"
//	"The Godfather Part II"  -> "The Godfather"
func ExtractSeriesBaseName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	patterns := []*regexp.Regexp{
		ReChineseSequel,
		ReEnglishSequel,
		ReRomanSuffix,
		ReParenSuffix,
		ReColonSequel,
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(title); len(matches) >= 2 {
			baseName := strings.TrimSpace(matches[1])
			if len([]rune(baseName)) >= 2 {
				return NormalizeBaseName(baseName)
			}
		}
	}

	return ""
}

// ExtractBaseNameDeep 组合式基础名提取：先 L2（连接词分割）再 L1（精确续集）。
//
// 设计目的：处理诸如 "逃学威龙3之龙过鸡年" 的复合标题。
//   - 只走 L1：失败（尾部不是数字）
//   - 只走 L2：得到 "逃学威龙3"（未去掉序号 3）
//   - 组合：L2 先拆出 "逃学威龙3"，再由 L1 剥离尾部数字 -> "逃学威龙" ✅
//
// 同时也处理英文的等价场景，例如：
//
//	"Iron Man 3: Rise of Ultron" -> "Iron Man"
//	"Harry Potter 7 - Part 1"    -> "Harry Potter"
//
// 返回空字符串表示组合规则未命中。
func ExtractBaseNameDeep(title string) string {
	prefix := ExtractPrefixByDelimiter(title)
	if prefix == "" {
		return ""
	}
	// 如果前缀本身仍含数字/罗马数字/Part 尾缀，再剥一层
	if deeper := ExtractSeriesBaseName(prefix); deeper != "" {
		return deeper
	}
	return ""
}

// NormalizeForCompare 归一化标题用于分组 key 匹配（忽略全半角、空白、常见标点差异）。
//
// 用途：
//   - 合集名 / 分组 key 的稳定比较
//   - 避免 "逃学威龙" 与 "逃学威龙 " / "逃学 威龙" / "逃学威龙：" 被误判为不同
func NormalizeForCompare(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// 跳过所有空白
		if unicode.IsSpace(r) {
			continue
		}
		// 跳过常见标点（中英冒号、顿号、逗号、破折号等），只保留文字和数字
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			continue
		}
		// 全角 → 半角（字母数字）
		if r >= 0xFF01 && r <= 0xFF5E {
			r = r - 0xFEE0
		}
		// 大小写不敏感
		if r >= 'A' && r <= 'Z' {
			r = r + ('a' - 'A')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// ExtractPrefixByDelimiter 从电影标题中提取前缀（第二层：连接词分割法）。
// 通过识别中文连接词（之、的、·、—）和英文分隔符、人名后缀来分割标题。
func ExtractPrefixByDelimiter(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	if matches := ReChineseDelimiter.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return NormalizeBaseName(prefix)
		}
	}

	if matches := ReChineseDelimiterDe.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return NormalizeBaseName(prefix)
		}
	}

	if matches := ReEnglishDelimiter.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return NormalizeBaseName(prefix)
		}
	}

	if matches := RePersonSuffix.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return NormalizeBaseName(prefix)
		}
	}

	return ""
}

// ExtractBaseNameBySpaceSplit 第三层：通用空格分割。
// 只要标题中包含空格，就提取空格前的基础名，作为前两层都未命中时的兜底。
func ExtractBaseNameBySpaceSplit(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	if matches := ReSpaceSplit.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return NormalizeBaseName(prefix)
		}
	}

	return ""
}

// NormalizeBaseName 标准化基础名（去除尾部标点、空格等）。
func NormalizeBaseName(name string) string {
	name = strings.TrimSpace(name)
	// 去除尾部的常见分隔符
	name = strings.TrimRight(name, " -_·.、，,")
	// 去除尾部的冒号
	name = strings.TrimRight(name, ":：")
	name = strings.TrimSpace(name)

	// 如果全是标点或空白，返回空
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
