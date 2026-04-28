// Package service 增强番号识别引擎
// 借鉴自 mdcx-master 项目（https://github.com/sqzw-x/mdcx）的 number.py 模块
// 将 40+ 种番号识别规则从 Python 移植到 Go，显著提升番号识别准确率
package service

import (
	"path/filepath"
	"regexp"
	"strings"
)

// ==================== 番号识别常量表 ====================

// suRenPrefixes 素人系列番号前缀（需要前置 3-6 位数字）
// 例如：259LUXU-1456、200GANA-2300、300MIUM-400
var suRenPrefixes = map[string]string{
	"LUXU":   "259",
	"GANA":   "200",
	"MIUM":   "300",
	"MAAN":   "300",
	"NTK":    "300",
	"KIRAY":  "300",
	"NMCH":   "300",
	"MAMA":   "300",
	"AKID":   "300",
	"WAKID":  "302",
	"SEMM":   "277",
	"DCV":    "277",
	"SCUTE":  "229",
	"JAC":    "261",
	"NANX":   "326",
	"GEKI":   "326",
	"KNB":    "326",
}

// oumeiNameMap 欧美片商名称映射（缩写 -> 完整名）
var oumeiNameMap = map[string]string{
	"bigtitsatwork":      "BigTitsatWork",
	"brazzersexxtra":     "BrazzersExxtra",
	"dirtymasseur":       "DirtyMasseur",
	"realwifestories":    "RealWifeStories",
	"teenslovehugecocks": "TeensLoveHugeCocks",
	"sexart":             "SexArt",
	"milehighmedia":      "MileHighMedia",
	"twistys":            "Twistys",
	"blacked":            "Blacked",
	"blackedraw":         "BlackedRaw",
	"tushy":              "Tushy",
	"tushyraw":           "TushyRaw",
	"vixen":              "Vixen",
	"deeper":             "Deeper",
	"slayed":             "Slayed",
	"milfy":              "Milfy",
}

// 无码番号前缀（用于快速判定 mosaic）
var uncensoredPrefixesSet = map[string]struct{}{
	"BT": {}, "CT": {}, "EMP": {}, "CCDV": {}, "CWP": {}, "CWPBD": {},
	"DSAM": {}, "DRC": {}, "DRG": {}, "GACHI": {}, "HEYZO": {}, "JAV": {},
	"KTG": {}, "KP": {}, "KG": {}, "LAF": {}, "LAFBD": {}, "LLDV": {},
	"MCDV": {}, "MKD": {}, "MKBD": {}, "MMDV": {}, "NIP": {}, "PB": {},
	"PT": {}, "QE": {}, "RED": {}, "RHJ": {}, "S2M": {}, "SKY": {},
	"SKYHD": {}, "SMD": {}, "SSDV": {}, "SSKP": {}, "TRG": {}, "TS": {},
	"XXX-AV": {}, "YKB": {},
}

// 国产番号关键词
var guochanKeywords = []string{
	"MD", "MDX", "MKY", "MSD", "MSM", "MKYT", "MTVQ", "MSIN",
	"91CM", "91TM", "GDCM", "PMC", "PMS", "PME", "JVID",
	"GDCM", "XKG", "XSJ",
}

// 常见编码/分辨率标签（用于清理文件名）
var cleanNoiseTags = []string{
	"4K", "4KS", "8K", "HD", "LR", "VR", "DVD", "FULL",
	"HEVC", "H264", "H265", "X264", "X265", "AAC", "AC3",
	"XXX", "PRT", "FHD", "UHD", "BLURAY", "WEBRIP", "WEB-DL",
	"HDRIP", "BDRIP", "REMUX", "REPACK", "PROPER", "UNCUT",
	"DIRECTORS", "EXTENDED", "SUBBED", "DUBBED",
}

// 发布组/字幕组常见标签（方括号中的内容）
var cleanBracketTags = []string{
	"中字", "无码", "有码", "破解", "加勒比", "一本道", "东京热",
	"修正", "流出", "无修正", "Carib", "Tokyo-Hot", "Heyzo",
	"FHD", "1080P", "720P", "2160P",
}

// ==================== 番号识别 Regex ====================

var (
	// 欧美日期格式番号：SexArt.22.11.26 / Brazzers.21.02.01
	reOumeiDate = regexp.MustCompile(`(?i)([A-Z0-9]{3,})[-_.]2?0?(\d{2}[-_.]\d{2}[-_.]\d{2})`)
	// MYWIFE No.1234 格式
	reMywife = regexp.MustCompile(`(?i)MYWIFE[^\d]*(\d{3,})`)
	// CW3D2D(BD)-11
	reCw3d2d = regexp.MustCompile(`(?i)CW3D2D?BD?-?\d{2,}`)
	// MMR-AK089SP 特殊
	reMmrSpecial = regexp.MustCompile(`(?i)MMR-?[A-Z]{2,}-?\d+[A-Z]*`)
	// MD-0165-1 国产番号
	reMdProduced = regexp.MustCompile(`(?i)(^|[^A-Z])(MD[A-Z-]*\d{4,}(-\d)?)`)
	// XXX-AV-11111
	reXxxAv = regexp.MustCompile(`(?i)XXX-AV-\d{4,}`)
	// MKY-A-12345
	reMkyA = regexp.MustCompile(`(?i)MKY-[A-Z]+-\d{3,}`)
	// FC2-1234567 / FC2-PPV-1234567 / FC2PPV-1234567
	reFc2All = regexp.MustCompile(`(?i)FC2-?(?:PPV-?)?(\d{5,})`)
	// HEYZO-1234
	reHeyzoAll = regexp.MustCompile(`(?i)HEYZO[^\d]*?(\d{3,})`)
	// H4610-ki111111 / C0930-ki221218 / H0930-ori1665
	reHxxxx = regexp.MustCompile(`(?i)(H4610|C0930|H0930)-?[A-Z]+\d{4,}`)
	// KIN8TENGOKU-1234 / KIN8-1234
	reKin8 = regexp.MustCompile(`(?i)KIN8(?:TENGOKU)?-?\d{3,}`)
	// S2MBD-002 / MCB3DBD-33
	reS2m    = regexp.MustCompile(`(?i)S2M[BD]*-\d{3,}`)
	reMcb3d  = regexp.MustCompile(`(?i)MCB3D[BD]*-\d{2,}`)
	// T28-223
	reT28 = regexp.MustCompile(`(?i)T28-?\d{3,}`)
	// TH101-140-112594
	reTh101 = regexp.MustCompile(`(?i)TH101-\d{3,}-\d{5,}`)
	// SSNI00644 -> SSNI-644
	reNoHyphen = regexp.MustCompile(`(?i)([A-Z]{2,})00(\d{3,})`)
	// 259LUXU-1456 素人系列
	reSuRen = regexp.MustCompile(`(?i)\d{2,}[A-Z]{2,}-\d{2,}[A-Z]?`)
	// MKBD-120 标准番号
	reStandard = regexp.MustCompile(`(?i)[A-Z]{2,}-\d{2,}[Z]?`)
	// MKBD-S120
	reStdWithLetter = regexp.MustCompile(`(?i)[A-Z]+-[A-Z]\d+`)
	// 111111-000 无码格式
	reUncensoredNum = regexp.MustCompile(`\d{6}[-_]\d{3,}`)
	// N1234 格式
	reN1234 = regexp.MustCompile(`(?i)(^|[^A-Z])(N\d{4})(\D|$)`)
	// H_173MEGA05 格式
	reHMega = regexp.MustCompile(`(?i)H_\d{3,}([A-Z]{2,})(\d{2,})`)
	// 3+字母 2+数字
	reLoose1 = regexp.MustCompile(`(?i)([A-Z]{3,}).*?(\d{2,})`)
	// 2+字母 3+数字
	reLoose2 = regexp.MustCompile(`(?i)([A-Z]{2,}).*?(\d{3,})`)

	// CD/PART 分集标识
	reCdPart = regexp.MustCompile(`(?i)[-_ .](CD|PART|PT|DISC|DISK)\s*(\d{1,2})`)
	// 中文字幕标识：-C / _C / .C
	reChineseSub = regexp.MustCompile(`(?i)[-_. ]C([-_. ]|$)`)
	// 时间戳：2022-03-15 / [22-11-26]
	reTimestamp = regexp.MustCompile(`\d{4}[-_.]\d{1,2}[-_.]\d{1,2}|[-\[]\d{2}[-_.]\d{2}[-_.]\d{2}\]?`)
	// 中括号/圆括号/日文括号内容
	reBrackets = regexp.MustCompile(`[【(（\[][^]）)】]*[]）)】]`)
)

// ==================== 数据结构 ====================

// NumberInfo 完整的番号信息
type NumberInfo struct {
	Number      string // 最终识别的番号（如 SSIS-001）
	Letters     string // 字母部分（如 SSIS）
	Number2     string // 数字部分（如 001）
	CodeType    string // 番号类型：standard/fc2/heyzo/uncensored/suren/guochan/oumei/mywife/kin8/t28/s2m/mcb3d/hxxxx/cw3d2d/mkya/xxxav/th101/mdseries/mmr
	Mosaic      string // 有码/无码/国产/欧美
	CDPart      string // 分集：CD1/PART2 等，空表示非分集
	HasChnSub   bool   // 是否为中文字幕版
	ShortNumber string // 去除 letters 后的短番号
	IsAdult     bool   // 是否为成人内容番号
}

// ==================== 对外主函数 ====================

// ParseCodeEnhanced 增强版番号识别（兼容旧接口同时返回更多信息）
// 相比旧 ParseCode：
//   - 支持 30+ 种番号格式
//   - 返回 mosaic（有码/无码/国产/欧美）
//   - 返回 CD 分集信息
//   - 返回中文字幕识别
//   - 返回 letters/short 分解
func ParseCodeEnhanced(input string) *NumberInfo {
	info := &NumberInfo{}
	if input == "" {
		return info
	}

	// 1) 取文件名部分（不含扩展名），大写
	realName := strings.TrimSpace(strings.TrimSuffix(filepath.Base(input), filepath.Ext(input)))
	if realName == "" {
		realName = input
	}

	// 2) CD/PART 分集识别（保留原文件名去匹配）
	if m := reCdPart.FindStringSubmatch(realName); len(m) >= 3 {
		info.CDPart = strings.ToUpper(m[1]) + m[2]
	}

	// 3) 中文字幕识别
	if reChineseSub.MatchString(realName + ".") {
		info.HasChnSub = true
	}

	// 4) 清理文件名
	cleaned := cleanFilename(realName)

	// 5) 提取番号（按优先级）
	number, codeType := extractNumber(cleaned)
	if number == "" {
		// 退化到旧的简单模式
		if old, oldType := ParseCode(realName); old != "" {
			info.Number = old
			info.CodeType = oldType
			info.IsAdult = true
			info.Letters = getLetters(old)
			info.ShortNumber = getShortNumber(old, info.Letters)
			info.Mosaic = inferMosaic(old, info.Letters)
			return info
		}
		return info
	}

	info.Number = number
	info.CodeType = codeType
	info.IsAdult = true
	info.Letters = getLetters(number)
	info.ShortNumber = getShortNumber(number, info.Letters)
	info.Mosaic = inferMosaic(number, info.Letters)

	return info
}

// ==================== 内部实现 ====================

// cleanFilename 清理文件名，移除发布组/分辨率/编码等噪声标签
func cleanFilename(filename string) string {
	name := strings.ToUpper(filename) + "."

	// 去除括号中的常见发布组/字幕组标签
	for _, tag := range cleanBracketTags {
		tagUpper := strings.ToUpper(tag)
		name = strings.ReplaceAll(name, tagUpper, "")
	}
	// 去除 [], (), 【】, （）
	name = reBrackets.ReplaceAllString(name, "")

	// 去除常见编码/分辨率标签
	for _, noise := range cleanNoiseTags {
		re := regexp.MustCompile(`[-_ .\[]` + regexp.QuoteMeta(noise) + `[-_ .\]]`)
		name = re.ReplaceAllString(name, "-")
	}

	// 替换 CD/PART/EP 分集
	name = strings.ReplaceAll(name, "-C.", ".")
	name = strings.ReplaceAll(name, ".PART", "-CD")
	name = strings.ReplaceAll(name, "-PART", "-CD")
	name = strings.ReplaceAll(name, " EP.", ".EP")
	name = strings.ReplaceAll(name, "-CD-", "")

	// 去除 -CD1/-CD2/...
	name = regexp.MustCompile(`[-_ .]CD\d{1,2}`).ReplaceAllString(name, "")
	// 去除结尾的单字母/数字分集
	name = regexp.MustCompile(`[-_ .][A-Z0-9]\.$`).ReplaceAllString(name, "")

	// 去除时间戳
	name = reTimestamp.ReplaceAllString(name, "")

	// 规范化分隔符
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.Trim(name, "-_. ")

	// FC2 变体统一
	name = strings.ReplaceAll(name, "FC2PPV", "FC2-")
	name = strings.ReplaceAll(name, "FC2-PPV", "FC2-")
	name = strings.ReplaceAll(name, "GACHIPPV", "GACHI")
	name = strings.ReplaceAll(name, "--", "-")

	return name
}

// extractNumber 从清理后的文件名中提取番号（按优先级匹配）
func extractNumber(s string) (string, string) {
	upper := strings.ToUpper(s)

	// MYWIFE No.1234
	if strings.Contains(upper, "MYWIFE") {
		if m := reMywife.FindStringSubmatch(upper); len(m) > 1 {
			return "Mywife No." + m[1], "mywife"
		}
	}

	// CW3D2DBD-11
	if m := reCw3d2d.FindString(upper); m != "" {
		return strings.ToUpper(m), "cw3d2d"
	}

	// MMR-AK089SP
	if m := reMmrSpecial.FindString(upper); m != "" {
		return strings.ReplaceAll(strings.ToUpper(m), "MMR-", "MMR"), "mmr"
	}

	// MD-0165-1 国产
	if m := reMdProduced.FindStringSubmatch(upper); len(m) > 2 && !strings.Contains(upper, "MDVR") {
		return strings.ToUpper(m[2]), "guochan"
	}

	// 欧美日期格式
	if m := reOumeiDate.FindStringSubmatch(s); len(m) > 2 {
		prefix := strings.Trim(strings.ToLower(m[1]), "-")
		if longName, ok := oumeiNameMap[prefix]; ok {
			return longName + "." + strings.ReplaceAll(m[2], "-", "."), "oumei"
		}
		return strings.Title(prefix) + "." + strings.ReplaceAll(m[2], "-", "."), "oumei"
	}

	// XXX-AV-11111
	if m := reXxxAv.FindString(upper); m != "" {
		return strings.ToUpper(m), "xxxav"
	}

	// MKY-A-12345
	if m := reMkyA.FindString(upper); m != "" {
		return strings.ToUpper(m), "mkya"
	}

	// FC2
	if strings.Contains(upper, "FC2") {
		if m := reFc2All.FindStringSubmatch(upper); len(m) > 1 {
			return "FC2-" + m[1], "fc2"
		}
	}

	// HEYZO
	if strings.Contains(upper, "HEYZO") {
		if m := reHeyzoAll.FindStringSubmatch(upper); len(m) > 1 {
			return "HEYZO-" + m[1], "heyzo"
		}
	}

	// H4610/C0930/H0930
	if m := reHxxxx.FindString(upper); m != "" {
		return strings.ToUpper(m), "hxxxx"
	}

	// KIN8
	if m := reKin8.FindString(upper); m != "" {
		normalized := strings.ToUpper(m)
		normalized = strings.ReplaceAll(normalized, "TENGOKU", "-")
		normalized = strings.ReplaceAll(normalized, "--", "-")
		return normalized, "kin8"
	}

	// S2M
	if m := reS2m.FindString(upper); m != "" {
		return strings.ToUpper(m), "s2m"
	}

	// MCB3D
	if m := reMcb3d.FindString(upper); m != "" {
		return strings.ToUpper(m), "mcb3d"
	}

	// T28
	if m := reT28.FindString(upper); m != "" {
		return strings.ReplaceAll(strings.ToUpper(m), "T2800", "T28-"), "t28"
	}

	// TH101
	if m := reTh101.FindString(upper); m != "" {
		return strings.ToLower(m), "th101"
	}

	// SSNI00644 -> SSNI-644
	if m := reNoHyphen.FindStringSubmatch(upper); len(m) > 2 {
		return m[1] + "-" + m[2], "standard"
	}

	// 素人系列 259LUXU-1456
	if m := reSuRen.FindString(upper); m != "" {
		return strings.ToUpper(m), "suren"
	}

	// 无码格式 111111-000
	if m := reUncensoredNum.FindString(upper); m != "" {
		return m, "uncensored"
	}

	// 标准番号 MKBD-120
	if m := reStandard.FindString(upper); m != "" {
		num := strings.ToUpper(m)
		// 检查是否需要加前缀（素人系列）
		for prefix, numPrefix := range suRenPrefixes {
			if strings.HasPrefix(num, prefix+"-") {
				num = numPrefix + num
				break
			}
		}
		if isExcludedPrefix(num) {
			// 退化到宽松匹配
		} else {
			return num, "standard"
		}
	}

	// MKBD-S120
	if m := reStdWithLetter.FindString(upper); m != "" {
		return strings.ToUpper(m), "standard"
	}

	// N1234
	if m := reN1234.FindStringSubmatch(upper); len(m) > 2 {
		return strings.ToLower(m[2]), "uncensored"
	}

	// H_173MEGA05
	if m := reHMega.FindStringSubmatch(upper); len(m) > 2 {
		return m[1] + "-" + m[2], "standard"
	}

	// 宽松匹配：3+字母 2+数字
	if m := reLoose1.FindStringSubmatch(upper); len(m) > 2 && !isExcludedPrefix(m[1]) {
		return m[1] + "-" + m[2], "standard"
	}
	// 宽松匹配：2+字母 3+数字
	if m := reLoose2.FindStringSubmatch(upper); len(m) > 2 && !isExcludedPrefix(m[1]) {
		return m[1] + "-" + m[2], "standard"
	}

	return "", ""
}

// getLetters 提取番号字母部分
func getLetters(number string) string {
	upper := strings.ToUpper(number)
	if strings.HasPrefix(upper, "FC2") {
		return "FC2"
	}
	if strings.HasPrefix(upper, "MYWIFE") {
		return "MYWIFE"
	}
	if strings.HasPrefix(upper, "KIN8") {
		return "KIN8"
	}
	if strings.HasPrefix(upper, "S2M") {
		return "S2M"
	}
	if strings.HasPrefix(upper, "T28") {
		return "T28"
	}
	if strings.HasPrefix(upper, "TH101") {
		return "TH101"
	}
	if strings.HasPrefix(upper, "XXX-AV") {
		return "XXX-AV"
	}
	if strings.HasPrefix(upper, "HEYZO") {
		return "HEYZO"
	}
	// 一般字母部分
	re := regexp.MustCompile(`(\d*[A-Z]+)`)
	if m := re.FindStringSubmatch(upper); len(m) > 1 {
		return m[1]
	}
	return ""
}

// getShortNumber 提取短番号（不含 letters）
func getShortNumber(number, letters string) string {
	upper := strings.ToUpper(number)
	if letters == "" {
		return ""
	}
	rest := strings.TrimPrefix(upper, letters)
	return strings.Trim(rest, "-_. ")
}

// inferMosaic 根据番号推断 mosaic 类型
func inferMosaic(number, letters string) string {
	upper := strings.ToUpper(number)

	// 欧美：含小数点或日期格式
	if strings.Contains(upper, ".") || regexp.MustCompile(`\d{2}\.\d{2}\.\d{2}`).MatchString(upper) {
		return "欧美"
	}

	// 国产番号
	for _, kw := range guochanKeywords {
		if strings.HasPrefix(upper, kw) {
			return "国产"
		}
	}

	// 无码前缀
	if _, ok := uncensoredPrefixesSet[letters]; ok {
		return "无码"
	}

	// N1234 格式
	if regexp.MustCompile(`^N\d{4}$`).MatchString(upper) {
		return "无码"
	}

	// 111111-000 无码格式
	if reUncensoredNum.MatchString(upper) {
		return "无码"
	}

	return "有码"
}

// isExcludedPrefix 判断前缀是否为无意义的编码/分辨率标签
func isExcludedPrefix(prefix string) bool {
	excludedPrefixes := map[string]bool{
		"MP": true, "AVC": true, "AAC": true, "DTS": true, "AC3": true,
		"HD": true, "SD": true, "BD": true, "CD": true, "DVD": true,
		"WEB": true, "MKV": true, "AVI": true, "HEVC": true, "FHD": true,
		"UHD": true, "H264": true, "H265": true, "X264": true, "X265": true,
		"REPACK": true, "PROPER": true, "SUBBED": true, "DUBBED": true,
	}
	return excludedPrefixes[strings.ToUpper(prefix)]
}

// ==================== 分集信息附加方法 ====================

// HasMultipleCDs 判断文件名中是否包含 CD 分集
func HasMultipleCDs(filename string) bool {
	return reCdPart.MatchString(filename)
}

// ExtractCDPart 提取 CD 分集编号（返回 CD1/PART2 等）
func ExtractCDPart(filename string) string {
	if m := reCdPart.FindStringSubmatch(filename); len(m) >= 3 {
		return strings.ToUpper(m[1]) + m[2]
	}
	return ""
}
