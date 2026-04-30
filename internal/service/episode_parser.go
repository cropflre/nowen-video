package service

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ParsedEpisode 电视剧/动漫文件名统一解析结果。
//
// 面向电视剧/动漫场景处理以下脏命名：
//  1. [字幕组][One Punch Man][01][720p].mkv
//  2. 报春鸟【傲仔压制】.mkv
//  3. 03A.mkv / 02A.mkv / 01B.mkv（数字+字母的多版本命名）
//  4. 特别篇1.mkv / SP01.mkv / OVA02.mkv（特别篇 → Season 0）
//  5. The.Series.Name.S02E05.1080p.WEB-DL.mkv
//  6. 翔ぶが如くⅡ 大久保の決断.mkv（日文连续命名）
//  7. [yyh3d.com]Series.Name.S01E01.mkv（站点标签前缀）
type ParsedEpisode struct {
	// SeriesTitle 清洗后的系列标题（用于聚合、搜索、展示）
	SeriesTitle string
	// SeriesTitleAlt 英文别名（中日剧名并列时填充）
	SeriesTitleAlt string
	// SeasonNum 季号（特别篇 / OVA / SP 为 0）
	SeasonNum int
	// EpisodeNum 集号（0 表示未识别）
	EpisodeNum int
	// EpisodeNumEnd 多集合并的终止集号（单集为 0）
	EpisodeNumEnd int
	// VersionTag 版本标识（如 03A 中的 A 表示 v2 版本；无则为空）
	VersionTag string
	// IsSpecial 是否为特别篇/SP/OVA（true 则强制 SeasonNum=0）
	IsSpecial bool
	// Year 年份（0 表示未识别）
	Year int
}

// ---- 电视剧专用正则 ----
var (
	// epReleaseGroupAdPattern 匹配【xxx压制】【xxx字幕组】【Q群xxx】等发行组/广告标签
	//
	// 覆盖：
	//   【傲仔压制】 【诸神字幕组】 【百度云】 【字幕组】 【压制】 【Q群123】 【V信xxx】
	//   (压制) （字幕组） (Q群xxx)
	epReleaseGroupAdPattern = regexp.MustCompile(`[【（\[(][^【】()\[\]（）]*(?:压制|字幕|字幕组|字幕社|发布|出品|转载|联合|汉化|制作|字幕君|Q群|Q裙|V信|微信|微 信|QQ|薇信|订阅|公众号|关注|联系|淘宝|加入|群号|扫码|小\s*红\s*书|十万度|推广|百度云|百度网盘|网盘|倍镜|蓝光|原盘)[^【】()\[\]（）]*[】）\])]`)

	// epBracketedTagPattern 匹配一般方括号内的短标签（字幕组/分辨率/编码标签）
	// 后续在 parse 内部按白名单过滤，只保留系列名所在的那段
	epBracketedTagPattern = regexp.MustCompile(`\[([^\[\]]{1,40})\]`)

	// epSingleLetterVersionPattern  形如 "03A" "02B" "12C"，末尾单字母版本
	epSingleLetterVersionPattern = regexp.MustCompile(`^(\d{1,3})([A-Za-z])$`)

	// epSpecialPatterns 特别篇 / OVA / SP / Specials 标记
	epSpecialPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)特别篇\s*(\d{1,3})?`),
		regexp.MustCompile(`(?i)\bSP(\d{1,3})\b`),
		regexp.MustCompile(`(?i)\bOVA(\d{1,3})\b`),
		regexp.MustCompile(`(?i)\bOAD(\d{1,3})\b`),
		regexp.MustCompile(`(?i)\bSpecial[s]?\s*(\d{1,3})?\b`),
	}

	// epSeasonStripPattern 从系列名末尾剥离 S1/S02/Season 2/第X季/第X部
	epSeasonStripPattern = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s*S\d{1,2}\s*$`),
		regexp.MustCompile(`(?i)\s*Season\s*\d{1,2}\s*$`),
		regexp.MustCompile(`\s*第\s*[一二三四五六七八九十\d]+\s*季\s*$`),
		regexp.MustCompile(`\s*第\s*[一二三四五六七八九十\d]+\s*部\s*$`),
		regexp.MustCompile(`(?i)\s*\(\s*Season\s*\d{1,2}\s*\)\s*$`),
	}

	// epCodecNoisePattern 电视剧命名中的分辨率/编码/来源噪声
	epCodecNoisePattern = regexp.MustCompile(`(?i)\b(BluRay|BDRip|HDRip|WEB-?DL|WEBRip|DVDRip|HDTV|HDCam|REMUX|PROPER|REPACK|EXTENDED|UNRATED|REMASTERED|COMPLETE|x264|x265|h\.?264|h\.?265|HEVC|AVC|AAC|DTS|AC3|FLAC|OPUS|Hi10P|MP4|MKV|FLV|AVI|TS|1080p|720p|480p|2160p|4K|UHD|HDR|SDR|3D|10bit|8bit|BIG5|GB|JPTC|CHS|CHT|JPN|ENG|GB_MP4|BIG5_MP4|CHS_SRT|CHT_SRT)\b`)

	// epBracketedNoise 方括号内的"纯噪声"列表（按出现原文判定要不要丢掉）
	epBracketedNoisePattern = regexp.MustCompile(`(?i)^(BIG5|GB|UTF-?8|MP4|MKV|AVI|HEVC|H\.?26[45]|AAC|FLAC|x26[45]|BIG5_MP4|GB_MP4|CHS|CHT|JPN|ENG|\d+[Pp]|\d+[xX]\d+|\d+|S\d+E\d+|E\d+|EP\d+|V\d+|WebRip|BDRip|DVDRip|WEB-DL|BluRay|HDTV|OVA\d*|SP\d*|2160p|1080p|720p|480p|4K|HDR|SDR|10bit|8bit|Hi10P|COMPLETE)$`)

	// epEpisodePatterns 集号匹配规则（优先级从高到低）
	// 注意：这里不消费 capture 之外的内容，只用来确认集号位置
	epEpisodePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)S(\d{1,2})E(\d{1,4})`),             // 0: S01E01
		regexp.MustCompile(`(?i)S(\d{1,2})\.E(\d{1,4})`),           // 1: S01.E01
		regexp.MustCompile(`(?i)(\d{1,2})x(\d{1,4})`),              // 2: 1x01
		regexp.MustCompile(`第\s*(\d{1,4})\s*集`),                    // 3: 第01集
		regexp.MustCompile(`(?i)(?:EP|Episode)\s*\.?\s*(\d{1,4})`), // 4: EP01
		regexp.MustCompile(`(?i)\[(\d{1,4})\]`),                    // 5: [01]
		regexp.MustCompile(`(?i)\bE(\d{1,4})\b`),                   // 6: E01
	}

	// epSpaceSquash 多空格合一
	epSpaceSquash = regexp.MustCompile(`\s+`)

	// epYearPattern 提取年份
	epYearPattern = regexp.MustCompile(`(?:^|[^0-9])((?:19|20)\d{2})(?:[^0-9]|$)`)
)

// ParseEpisodeFilename 统一解析电视剧/动漫单个剧集文件名。
// 入参可以是文件名（带扩展名）或目录名（无扩展名）。
func ParseEpisodeFilename(filename string) ParsedEpisode {
	out := ParsedEpisode{}
	if filename == "" {
		return out
	}

	// 1) 去扩展名
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// 2) 去站点标签 [yyh3d.com] / [nyaa.si]
	name = siteTagPattern.ReplaceAllString(name, " ")

	// 3) 去广告/发行组 【xxx压制】
	name = epReleaseGroupAdPattern.ReplaceAllString(name, " ")

	// 4) 统一中文标点 / 全角空格
	name = strings.ReplaceAll(name, "。", ".")
	name = strings.ReplaceAll(name, "　", " ")

	// 5) 检测特别篇 / OVA / SP — 必须在集号匹配之前，因为它们会把 EpisodeNum 归零后重新解析
	for _, p := range epSpecialPatterns {
		if m := p.FindStringSubmatch(name); len(m) >= 1 {
			out.IsSpecial = true
			if len(m) >= 2 && m[1] != "" {
				if n, _ := strconv.Atoi(m[1]); n > 0 {
					out.EpisodeNum = n
				}
			}
			break
		}
	}

	// 6) 单字母版本号检测："03A" "02B"
	//    仅当整个文件主体恰好是 "数字+字母"（前后最多有空格/标点）时才触发
	trimmed := strings.Trim(name, " -_.·・【】()（）[]")
	if m := epSingleLetterVersionPattern.FindStringSubmatch(trimmed); len(m) >= 3 {
		if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
			out.EpisodeNum = n
			out.VersionTag = strings.ToUpper(m[2])
			// 单独的 "03A" 没有系列名；SeriesTitle 留给上层用父目录名填充
			return out
		}
	}

	// 7) 集号识别（如果前面没从 SP/OVA 里拿到）
	if out.EpisodeNum == 0 {
		for idx, p := range epEpisodePatterns {
			if m := p.FindStringSubmatch(name); len(m) >= 2 {
				switch idx {
				case 0, 1, 2:
					s, _ := strconv.Atoi(m[1])
					e, _ := strconv.Atoi(m[2])
					if s > 0 {
						out.SeasonNum = s
					}
					out.EpisodeNum = e
				default:
					n, _ := strconv.Atoi(m[1])
					if n > 0 && n < 1900 {
						out.EpisodeNum = n
					}
				}
				if out.EpisodeNum > 0 {
					// 把命中的片段及其左边的分隔符从 name 中移除，剩下的作为标题线索
					name = p.ReplaceAllString(name, " ")
					break
				}
			}
		}
	}

	// 8) 再清一次广告/特殊标记残留
	for _, p := range epSpecialPatterns {
		name = p.ReplaceAllString(name, " ")
	}

	// 9) 方括号标签按白名单过滤：保留"像系列名的"，去掉"纯噪声的"
	name = epBracketedTagPattern.ReplaceAllStringFunc(name, func(m string) string {
		inner := strings.Trim(m, "[]")
		inner = strings.TrimSpace(inner)
		if inner == "" {
			return " "
		}
		if epBracketedNoisePattern.MatchString(inner) {
			return " "
		}
		// 看起来是系列名或附加信息，保留（去掉括号）
		return " " + inner + " "
	})

	// 10) 去编码/分辨率/来源噪声
	name = epCodecNoisePattern.ReplaceAllString(name, " ")

	// 11) 提取并剥离年份
	if m := epYearPattern.FindStringSubmatch(name); len(m) >= 2 {
		if y, err := strconv.Atoi(m[1]); err == nil && y >= 1900 && y <= 2099 {
			out.Year = y
		}
		name = epYearPattern.ReplaceAllString(name, " ")
	}

	// 12) 规范分隔符
	replacer := strings.NewReplacer(".", " ", "_", " ", "·", " ", "・", " ")
	name = replacer.Replace(name)
	name = epSpaceSquash.ReplaceAllString(name, " ")
	name = strings.Trim(name, " -·・")

	// 13) 剥离末尾季号（"一拳超人 S1" → "一拳超人"）
	for _, p := range epSeasonStripPattern {
		if m := p.FindStringSubmatch(name); len(m) >= 1 {
			// 顺便尝试提取季号
			if out.SeasonNum == 0 {
				if numMatch := regexp.MustCompile(`\d{1,2}`).FindString(m[0]); numMatch != "" {
					if s, _ := strconv.Atoi(numMatch); s > 0 && s <= 50 {
						out.SeasonNum = s
					}
				}
			}
			name = p.ReplaceAllString(name, "")
		}
	}
	name = strings.TrimSpace(name)

	// 14) 尝试分出中文 / 英文（复用 filename_parser.go 里的 picker）
	if cn := pickFirstChineseSegment(name); cn != "" {
		out.SeriesTitle = cn
		if en := pickLongestLatinSegment(name); en != "" && en != cn {
			out.SeriesTitleAlt = en
		}
	} else {
		out.SeriesTitle = name
	}
	out.SeriesTitle = strings.Trim(out.SeriesTitle, " -·・")
	out.SeriesTitleAlt = strings.Trim(out.SeriesTitleAlt, " -·・")

	// 15) 如果是特别篇，强制 SeasonNum=0
	if out.IsSpecial {
		out.SeasonNum = 0
	}

	return out
}

// NormalizeSeriesTitle 对任意来源的 Series 标题做最后一轮清洗，
// 用于：入库前 / 搜索前 / 合并前的归一化。
//
// 规则：
//   - 去发行组/广告【xxx压制】【Q群xxx】
//   - 去站点标签 [yyh3d.com]
//   - 去编码/来源噪声
//   - 剥离末尾季号（S1 / 第一季 / Season 2）
//   - 去首尾分隔符
func NormalizeSeriesTitle(raw string) string {
	if raw == "" {
		return ""
	}
	t := raw
	t = siteTagPattern.ReplaceAllString(t, " ")
	t = epReleaseGroupAdPattern.ReplaceAllString(t, " ")
	t = chineseAdPattern.ReplaceAllString(t, " ")
	t = strings.ReplaceAll(t, "。", ".")
	t = strings.ReplaceAll(t, "　", " ")
	t = epCodecNoisePattern.ReplaceAllString(t, " ")
	// 去年份
	t = epYearPattern.ReplaceAllString(t, " ")
	// 剥离季号
	for _, p := range epSeasonStripPattern {
		t = p.ReplaceAllString(t, "")
	}
	// 规范分隔符
	t = strings.NewReplacer(".", " ", "_", " ").Replace(t)
	t = epSpaceSquash.ReplaceAllString(t, " ")
	t = strings.Trim(t, " -·・【】()（）[]")
	return t
}
