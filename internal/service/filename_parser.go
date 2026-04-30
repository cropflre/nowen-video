package service

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ParsedFilename 统一的文件名解析结果。
//
// 面向电影/剧集通用命名场景，尤其处理国内资源站常见的脏命名：
//   - 01届.《翼》-《Wings》-1927-1929。【十万度Q裙 319940383】.mkv
//   - [yyh3d.com]采花和尚.Satyr Monks.1994.LD_D9.x264.AAC.480P.YYH3D.xt.mkv
//   - Movie.Name.2020.1080p.BluRay.x264.mkv（仍兼容）
//   - Title (2020) [tmdbid=12345].mkv（仍兼容）
type ParsedFilename struct {
	// Title 清洗后的主标题（通常为中文名；若无中文则是英文名）
	Title string
	// TitleAlt 英文别名（当存在《英文名》或中英文并列时填充），便于作为备选搜索词
	TitleAlt string
	// Year 上映年份；0 表示未识别到
	Year int
	// TMDbID / IMDbID 从 [tmdbid=xxx]/{imdb-ttxxx} 标签中识别到的 ID
	TMDbID int
	IMDbID string
}

// ---- 通用正则（文件作用域复用） ----

var (
	// siteTagPattern 匹配文件名开头 / 结尾的站点域名前缀标签，例如 [yyh3d.com]、[nyaa.si]
	siteTagPattern = regexp.MustCompile(`(?i)\[[a-z0-9][a-z0-9\-]*\.[a-z]{2,}(?:\.[a-z]{2,})?\]`)

	// leadingAwardPrefixPattern 匹配开头的"XX届"之类中文前缀（一到三位数字 + 届/集/期），后面可能跟 . / - / 空格
	leadingAwardPrefixPattern = regexp.MustCompile(`^\s*\d{1,3}\s*[届集期]\s*[\.\-_\s]*`)

	// chineseAdPattern 匹配【xxx】或（xxx）中包含推广/联系方式关键词的广告段
	chineseAdPattern = regexp.MustCompile(`[【（\[(][^【】()\[\]（）]*(?:Q裙|Q群|V信|微信|微 信|QQ|薇信|订阅|公众号|关注|联系|淘宝|加入|群号|扫码|小\s*红\s*书|十万度|推广)[^【】()\[\]（）]*[】）\])]`)

	// chineseNameInBookPattern 匹配《中文名》
	chineseNameInBookPattern = regexp.MustCompile(`《([^《》]+)》`)

	// trailingJunkPattern 匹配尾部垃圾后缀，例如 .115chrome_5_17、.cfg 已被 filepath.Ext 处理
	//   115chrome/115/chrome 之类是 115 网盘标记，非文件名内容
	trailingJunkPattern = regexp.MustCompile(`(?i)\.(?:115chrome|115)[_\-\.\w]*$`)

	// embeddedTrailingJunkPattern 文件名中间的 115chrome 标记（刮削前再兜底清理一次）
	embeddedTrailingJunkPattern = regexp.MustCompile(`(?i)[\-\._\s]*(?:115chrome|115)[_\-\.\w]*`)

	// embeddedMediaExtPattern 文件名内嵌的媒体扩展名残留（常见于错误的双扩展名场景，如 ...mkv.115chrome_4_28）
	embeddedMediaExtPattern = regexp.MustCompile(`(?i)\b(?:mkv|mp4|avi|ts|m2ts|mov|wmv|flv|webm|rmvb|mpg|mpeg)\b`)

	// yyh3dTagPattern 匹配常见的私有发布站技术标签组合，如 LD_D9.x264.AAC.480P.YYH3D.xt
	yyh3dTagPattern = regexp.MustCompile(`(?i)\b(?:YYH3D|xt|LD_D\d|LD_D9|D9|D5|D1)\b`)

	// codecNoisePattern 一次性吃掉常见编码/来源/分辨率噪声标签
	codecNoisePattern = regexp.MustCompile(`(?i)\b(BluRay|BDRip|HDRip|WEB-?DL|WEBRip|DVDRip|HDTV|HDCam|REMUX|PROPER|REPACK|EXTENDED|UNRATED|DIRECTORS\.?CUT|REMASTERED|x264|x265|h\.?264|h\.?265|HEVC|AVC|AAC|DTS|AC3|FLAC|OPUS|1080p|720p|480p|2160p|4K|UHD|HDR|SDR|3D)\b`)

	// spaceSquashPattern 多空格合一
	spaceSquashPattern = regexp.MustCompile(`\s+`)

	// chineseYearRangePattern 匹配 1927-1929 / 1971-1972 这样的年份区间（颁奖届使用跨年），第一个年份即为电影年份
	chineseYearRangePattern = regexp.MustCompile(`((?:19|20)\d{2})\s*[\-–—~～]\s*((?:19|20)\d{2})`)

	// latinTitleRunPattern 连续 ASCII 片段：用于识别并抽取英文别名
	latinTitleRunPattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9 '&:,\.\-]*[A-Za-z0-9]`)
)

// ParseMovieFilename 统一的电影文件名解析入口。
//
// 兼容以下脏命名模式：
//  1. "01届.《翼》-《Wings》-1927-1929。【十万度Q裙 319940383】.mkv"
//  2. "[yyh3d.com]采花和尚.Satyr Monks.1994.LD_D9.x264.AAC.480P.YYH3D.xt.mkv"
//  3. "The.Matrix.1999.BluRay.1080p.x264.mkv"
//  4. "Avatar (2009) [tmdbid=19995].mkv"
//  5. 尾部 .115chrome_5_17 垃圾后缀
//
// 返回值字段含义见 ParsedFilename。对于无法识别年份的，Year 为 0；上层可再调用
// extractYearFromName 对整条路径做兜底。
func ParseMovieFilename(filename string) ParsedFilename {
	out := ParsedFilename{}
	if filename == "" {
		return out
	}

	// 1) 剥离扩展名，并清理 .115chrome_xxx 这类紧贴扩展名前的垃圾后缀
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = trailingJunkPattern.ReplaceAllString(name, "")

	// 2) 提取并移除 tmdb/imdb 标签
	if idType, idValue := parseIDFromName(name); idValue != "" {
		switch idType {
		case "tmdbid", "tmdb":
			out.TMDbID, _ = strconv.Atoi(idValue)
		case "imdbid", "imdb":
			out.IMDbID = idValue
		}
	}
	for _, p := range idTagPatterns {
		name = p.ReplaceAllString(name, "")
	}

	// 3) 去掉站点标签 [xxx.com]（无论在头在尾都去）
	name = siteTagPattern.ReplaceAllString(name, "")

	// 4) 去掉开头的"XX届"前缀
	name = leadingAwardPrefixPattern.ReplaceAllString(name, "")

	// 5) 去掉【十万度Q裙 xxx】这种中文广告段
	name = chineseAdPattern.ReplaceAllString(name, "")

	// 6) 把"中文句号。"统一成半角点，便于后续分隔符统一
	name = strings.ReplaceAll(name, "。", ".")
	name = strings.ReplaceAll(name, "　", " ") // 全角空格

	// 7) 年份区间（1927-1929）优先取前者作为电影年份
	if m := chineseYearRangePattern.FindStringSubmatch(name); len(m) >= 2 {
		if y, _ := strconv.Atoi(m[1]); y >= 1900 && y <= 2099 {
			out.Year = y
		}
		name = chineseYearRangePattern.ReplaceAllString(name, " ")
	}

	// 8) 尝试提取《中文名》和《英文名》，优先作为 Title / TitleAlt
	if ms := chineseNameInBookPattern.FindAllStringSubmatch(name, -1); len(ms) > 0 {
		// 第一个书名号视为中文主标题，后续若包含英文字母视为英文别名
		for _, m := range ms {
			inner := strings.TrimSpace(m[1])
			if inner == "" {
				continue
			}
			if out.Title == "" {
				out.Title = inner
				continue
			}
			if out.TitleAlt == "" && containsLatin(inner) {
				out.TitleAlt = inner
			}
		}
		name = chineseNameInBookPattern.ReplaceAllString(name, " ")
	}

	// 9) 去掉内嵌 115chrome 垃圾
	name = embeddedTrailingJunkPattern.ReplaceAllString(name, " ")

	// 9.5) 去掉内嵌媒体扩展名（双扩展名场景，如 xxx.mkv.115chrome_4_28 在第1步被截掉 .115chrome_4_28 后仍残留 .mkv）
	name = embeddedMediaExtPattern.ReplaceAllString(name, " ")

	// 10) 去掉 YYH3D / LD_D9 / xt / D9 等私有站点技术标签
	name = yyh3dTagPattern.ReplaceAllString(name, " ")

	// 11) 编码/来源/分辨率噪声
	name = codecNoisePattern.ReplaceAllString(name, " ")

	// 12) 如果之前没拿到年份，再尝试常规括号年份 / 普通年份
	if out.Year == 0 {
		if y := extractYearFromName(name); y > 0 {
			out.Year = y
		}
	}
	if out.Year == 0 {
		if m := regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(?:[^0-9]|$)`).FindStringSubmatch(name); len(m) >= 2 {
			if y, _ := strconv.Atoi(m[1]); y >= 1900 && y <= 2099 {
				out.Year = y
			}
		}
	}
	// 无论什么括号，移除年份
	name = yearInNamePattern.ReplaceAllString(name, " ")
	name = yearInNameAnyBracketPattern.ReplaceAllString(name, " ")
	name = regexp.MustCompile(`\b((?:19|20)\d{2})\b`).ReplaceAllString(name, " ")

	// 13) 使用通用括号清洗器去掉【】《》「」『』〈〉等装饰
	name = normalizeXiaoyaTitle(name)

	// 14) 统一分隔符，规范空格
	replacer := strings.NewReplacer(".", " ", "_", " ")
	clean := replacer.Replace(name)
	clean = spaceSquashPattern.ReplaceAllString(clean, " ")
	clean = strings.Trim(clean, " -·・")

	// 15) 如果还没有 Title，就按"中文段优先、否则整串"决定；同时提取英文别名
	if out.Title == "" {
		if cn := pickFirstChineseSegment(clean); cn != "" {
			out.Title = cn
			if out.TitleAlt == "" {
				if en := pickLongestLatinSegment(clean); en != "" && en != cn {
					out.TitleAlt = en
				}
			}
		} else {
			out.Title = clean
		}
	} else if out.TitleAlt == "" {
		// 已有中文 Title，但尾部可能还残留英文名（例如"采花和尚 Satyr Monks"）
		if en := pickLongestLatinSegment(clean); en != "" {
			out.TitleAlt = en
		}
	}

	// 保险：再次去掉首尾分隔
	out.Title = strings.Trim(out.Title, " -·・")
	out.TitleAlt = strings.Trim(out.TitleAlt, " -·・")
	return out
}

// containsLatin 判断字符串是否包含 ASCII 拉丁字母
func containsLatin(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

// containsHan 判断字符串是否包含 CJK 汉字
func containsHan(s string) bool {
	for _, r := range s {
		if r >= 0x4E00 && r <= 0x9FFF {
			return true
		}
		if r >= 0x3400 && r <= 0x4DBF {
			return true
		}
	}
	return false
}

// pickFirstChineseSegment 从一串空格分隔的文本中挑出第一个包含汉字的片段（允许相邻汉字连起来）
func pickFirstChineseSegment(s string) string {
	// 优先把紧邻的汉字串合并：用一个简单 rune 扫描
	var buf strings.Builder
	var result string
	for _, r := range s {
		isHan := (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF)
		isPunctAttached := r == '·' || r == '・' || r == '-'
		if isHan || (buf.Len() > 0 && isPunctAttached) {
			buf.WriteRune(r)
			continue
		}
		if buf.Len() > 0 {
			result = strings.Trim(buf.String(), "-·・")
			if containsHan(result) {
				return result
			}
			buf.Reset()
		}
	}
	if buf.Len() > 0 {
		result = strings.Trim(buf.String(), "-·・")
		if containsHan(result) {
			return result
		}
	}
	return ""
}

// pickLongestLatinSegment 从字符串中挑出最长的拉丁（英文）片段，常见于"中文名 English Title"并列
func pickLongestLatinSegment(s string) string {
	matches := latinTitleRunPattern.FindAllString(s, -1)
	best := ""
	for _, m := range matches {
		t := strings.TrimSpace(m)
		// 过滤纯 1~2 字母噪声（例如 "D9" "xt"）
		if len([]rune(t)) < 3 {
			continue
		}
		if len(t) > len(best) {
			best = t
		}
	}
	return best
}
