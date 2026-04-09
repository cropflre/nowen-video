package service

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.uber.org/zap"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/transform"

	"github.com/saintfish/chardet"
)

// ==================== 字幕清洗配置 ====================

// SubtitleCleanConfig 字幕清洗配置
type SubtitleCleanConfig struct {
	// 编码检测与转换
	AutoDetectEncoding bool   // 自动检测编码并转为 UTF-8
	FallbackEncoding   string // 检测失败时的回退编码（如 "gbk"）

	// 文本清洗
	RemoveHTMLTags       bool // 去除 HTML 标签（<i>, <b> 等）
	RemoveASSStyles      bool // 去除 ASS 样式标签（{\an8} 等）
	NormalizePunctuation bool // 统一标点符号（全角→半角等）
	RemoveSDH            bool // 去除 SDH 标注（[音乐], (笑声) 等）
	RemoveAds            bool // 去除广告水印字幕

	// 时间轴处理
	TimeOffsetMs  int64 // 全局时间偏移（毫秒）
	MinDurationMs int64 // 最小显示时长（毫秒），过短的合并
	MaxDurationMs int64 // 最大显示时长（毫秒），过长的拆分
	MinGapMs      int64 // 最小间隔（毫秒），间隔过小的合并

	// 分段与合并
	MergeShortCues  bool // 合并过短的字幕条目
	SplitLongCues   bool // 拆分过长的字幕条目
	MaxCharsPerLine int  // 每行最大字符数
	MaxLinesPerCue  int  // 每条字幕最大行数

	// 备份
	BackupOriginal bool // 处理前备份原始文件
}

// CleanReport 字幕清洗报告
type CleanReport struct {
	SourcePath        string   `json:"source_path"`
	BackupPath        string   `json:"backup_path,omitempty"`
	DetectedEncoding  string   `json:"detected_encoding"`
	OriginalCueCount  int      `json:"original_cue_count"`
	ProcessedCueCount int      `json:"processed_cue_count"`
	RemovedAds        int      `json:"removed_ads"`
	RemovedSDH        int      `json:"removed_sdh"`
	RemovedEmpty      int      `json:"removed_empty"`
	MergedCues        int      `json:"merged_cues"`
	SplitCues         int      `json:"split_cues"`
	EncodingConverted bool     `json:"encoding_converted"`
	Warnings          []string `json:"warnings,omitempty"`
}

// SubtitleCleaner 字幕内容清洗器
type SubtitleCleaner struct {
	config SubtitleCleanConfig
	logger *zap.SugaredLogger
}

// NewSubtitleCleaner 创建字幕清洗器
func NewSubtitleCleaner(config SubtitleCleanConfig, logger *zap.SugaredLogger) *SubtitleCleaner {
	return &SubtitleCleaner{
		config: config,
		logger: logger,
	}
}

// ==================== 正则表达式（编译一次复用） ====================

var (
	// HTML 标签匹配
	htmlTagRegex = regexp.MustCompile(`<[^>]+>`)
	// ASS 样式标签匹配（如 {\an8}, {\pos(320,50)}, {\fad(500,500)} 等）
	assStyleRegex = regexp.MustCompile(`\{\\[^}]*\}`)
	// SDH 标注匹配（方括号、圆括号、中文括号等）
	sdhBracketRegex = regexp.MustCompile(`[\[\(（【《][^\]\)）】》]*[\]\)）】》]`)
	// 广告/水印常见模式
	adPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:字幕|翻译|校对|时间轴|压制|后期|特效)[:：\s].*`),
		regexp.MustCompile(`(?i)(?:www\.|http|\.com|\.cn|\.org|\.net|\.tv)`),
		regexp.MustCompile(`(?i)(?:subtitle[sd]?|translated|synced|ripped|encoded)\s*(?:by|from).*`),
		regexp.MustCompile(`(?i)(?:sub\.?(?:team|group|studio)|fansub|字幕组|字幕社)`),
		regexp.MustCompile(`(?i)(?:opensubtitles|subscene|addic7ed|yifysubtitles)`),
		regexp.MustCompile(`(?i)(?:@|＠)\s*\w+`), // @用户名 水印
	}
	// VTT 时间戳格式匹配
	vttTimeRegex = regexp.MustCompile(`^(\d{1,2}:)?\d{2}:\d{2}[.,]\d{3}$`)
)

// ==================== 核心清洗方法 ====================

// CleanVTT 对 VTT 文件执行全套清洗流程
func (c *SubtitleCleaner) CleanVTT(vttPath string) (*CleanReport, error) {
	report := &CleanReport{SourcePath: vttPath}

	// 1. 备份原始文件
	if c.config.BackupOriginal {
		backupPath := vttPath + ".bak"
		if err := copyFile(vttPath, backupPath); err != nil {
			c.logger.Warnf("备份字幕文件失败: %v", err)
			report.Warnings = append(report.Warnings, fmt.Sprintf("备份失败: %v", err))
		} else {
			report.BackupPath = backupPath
			c.logger.Debugf("已备份字幕文件: %s -> %s", vttPath, backupPath)
		}
	}

	// 2. 编码检测与转换
	content, encoding, converted := c.detectAndConvertEncoding(vttPath)
	report.DetectedEncoding = encoding
	report.EncodingConverted = converted

	// 3. 解析 VTT cues
	cues := parseVTTCues(content)
	report.OriginalCueCount = len(cues)

	if len(cues) == 0 {
		report.Warnings = append(report.Warnings, "未解析到任何字幕条目")
		return report, nil
	}

	// 4. 文本清洗
	cues, removedAds, removedSDH, removedEmpty := c.cleanTexts(cues)
	report.RemovedAds = removedAds
	report.RemovedSDH = removedSDH
	report.RemovedEmpty = removedEmpty

	// 5. 时间轴标准化
	cues = c.normalizeTimeline(cues)

	// 6. 分段与合并优化
	cues, mergedCount, splitCount := c.optimizeSegments(cues)
	report.MergedCues = mergedCount
	report.SplitCues = splitCount

	report.ProcessedCueCount = len(cues)

	// 7. 写回文件
	if err := writeVTTToFile(vttPath, cues); err != nil {
		return report, fmt.Errorf("写回 VTT 文件失败: %w", err)
	}

	c.logger.Infof("字幕清洗完成: %s (编码: %s, %d→%d 条, 去广告: %d, 去SDH: %d, 合并: %d, 拆分: %d)",
		filepath.Base(vttPath), encoding,
		report.OriginalCueCount, report.ProcessedCueCount,
		removedAds, removedSDH, mergedCount, splitCount)

	return report, nil
}

// ==================== 编码检测与转换 ====================

// detectAndConvertEncoding 检测文件编码并转换为 UTF-8
// 返回值: (内容字符串, 检测到的编码名, 是否进行了转换)
func (c *SubtitleCleaner) detectAndConvertEncoding(filePath string) (string, string, bool) {
	raw, err := os.ReadFile(filePath)
	if err != nil {
		c.logger.Warnf("读取字幕文件失败: %v", err)
		return "", "unknown", false
	}

	// 1. 检查 BOM（Byte Order Mark）
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		// UTF-8 BOM
		return string(raw[3:]), "UTF-8 (BOM)", false
	}
	if bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) {
		// UTF-16 LE BOM — 需要转换
		decoded := decodeUTF16LE(raw[2:])
		return decoded, "UTF-16 LE", true
	}
	if bytes.HasPrefix(raw, []byte{0xFE, 0xFF}) {
		// UTF-16 BE BOM — 需要转换
		decoded := decodeUTF16BE(raw[2:])
		return decoded, "UTF-16 BE", true
	}

	// 2. 尝试 UTF-8 验证
	if utf8.Valid(raw) {
		return string(raw), "UTF-8", false
	}

	// 3. chardet 自动检测
	detector := chardet.NewTextDetector()
	result, err := detector.DetectBest(raw)
	if err == nil && result.Confidence > 30 {
		if decoded, ok := decodeByCharset(raw, result.Charset); ok {
			return decoded, result.Charset, true
		}
	}

	// 4. 回退编码
	if c.config.FallbackEncoding != "" {
		if decoded, ok := decodeByCharset(raw, c.config.FallbackEncoding); ok {
			return decoded, c.config.FallbackEncoding + " (fallback)", true
		}
	}

	// 5. 最终回退：强制当作 UTF-8 处理（可能有乱码）
	c.logger.Warnf("无法检测字幕文件编码，强制使用 UTF-8: %s", filePath)
	return string(raw), "unknown (forced UTF-8)", false
}

// decodeByCharset 根据 charset 名称解码字节数据为 UTF-8 字符串
func decodeByCharset(data []byte, charset string) (string, bool) {
	charset = strings.ToLower(strings.TrimSpace(charset))

	var decoder *transform.Reader

	switch charset {
	case "gb2312", "gbk", "gb18030", "gb-2312", "gb-18030":
		decoder = transform.NewReader(bytes.NewReader(data), simplifiedchinese.GBK.NewDecoder())
	case "big5", "big-5":
		decoder = transform.NewReader(bytes.NewReader(data), traditionalchinese.Big5.NewDecoder())
	case "shift_jis", "shift-jis", "sjis", "shiftjis":
		decoder = transform.NewReader(bytes.NewReader(data), japanese.ShiftJIS.NewDecoder())
	case "euc-jp", "eucjp":
		decoder = transform.NewReader(bytes.NewReader(data), japanese.EUCJP.NewDecoder())
	case "iso-2022-jp":
		decoder = transform.NewReader(bytes.NewReader(data), japanese.ISO2022JP.NewDecoder())
	case "euc-kr", "euckr":
		decoder = transform.NewReader(bytes.NewReader(data), korean.EUCKR.NewDecoder())
	case "iso-8859-1", "latin1", "latin-1":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.ISO8859_1.NewDecoder())
	case "iso-8859-2", "latin2", "latin-2":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.ISO8859_2.NewDecoder())
	case "iso-8859-15", "latin9", "latin-9":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.ISO8859_15.NewDecoder())
	case "windows-1250", "cp1250":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.Windows1250.NewDecoder())
	case "windows-1251", "cp1251":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.Windows1251.NewDecoder())
	case "windows-1252", "cp1252":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.Windows1252.NewDecoder())
	case "windows-1256", "cp1256":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.Windows1256.NewDecoder())
	case "koi8-r", "koi8r":
		decoder = transform.NewReader(bytes.NewReader(data), charmap.KOI8R.NewDecoder())
	default:
		return "", false
	}

	decoded, err := readAll(decoder)
	if err != nil {
		return "", false
	}
	return string(decoded), true
}

// decodeUTF16LE 解码 UTF-16 LE 字节为 UTF-8 字符串
func decodeUTF16LE(data []byte) string {
	var result strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		ch := rune(data[i]) | rune(data[i+1])<<8
		result.WriteRune(ch)
	}
	return result.String()
}

// decodeUTF16BE 解码 UTF-16 BE 字节为 UTF-8 字符串
func decodeUTF16BE(data []byte) string {
	var result strings.Builder
	for i := 0; i+1 < len(data); i += 2 {
		ch := rune(data[i])<<8 | rune(data[i+1])
		result.WriteRune(ch)
	}
	return result.String()
}

// readAll 从 transform.Reader 读取所有数据
func readAll(r *transform.Reader) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	return buf.Bytes(), err
}

// ==================== 文本内容清洗 ====================

// cleanTexts 清洗字幕文本内容
// 返回值: (清洗后的cues, 去除的广告数, 去除的SDH数, 去除的空条目数)
func (c *SubtitleCleaner) cleanTexts(cues []vttCue) ([]vttCue, int, int, int) {
	var cleaned []vttCue
	removedAds := 0
	removedSDH := 0
	removedEmpty := 0

	for _, cue := range cues {
		text := cue.text

		// 去除 HTML 标签
		if c.config.RemoveHTMLTags {
			text = htmlTagRegex.ReplaceAllString(text, "")
		}

		// 去除 ASS 样式标签
		if c.config.RemoveASSStyles {
			text = assStyleRegex.ReplaceAllString(text, "")
		}

		// 去除广告水印
		if c.config.RemoveAds {
			isAd := false
			for _, p := range adPatterns {
				if p.MatchString(text) {
					isAd = true
					break
				}
			}
			if isAd {
				removedAds++
				continue
			}
		}

		// 去除 SDH 标注
		if c.config.RemoveSDH {
			originalLen := len(text)
			text = sdhBracketRegex.ReplaceAllString(text, "")
			if len(text) < originalLen {
				removedSDH++
			}
		}

		// 统一标点符号
		if c.config.NormalizePunctuation {
			text = normalizePunctuation(text)
		}

		// 清理多余空白
		text = cleanWhitespace(text)

		if text == "" {
			removedEmpty++
			continue
		}

		cue.text = text
		cleaned = append(cleaned, cue)
	}

	return cleaned, removedAds, removedSDH, removedEmpty
}

// normalizePunctuation 统一标点符号
// 将全角标点转为半角（适用于英文字幕），保留中日韩字幕的全角标点
func normalizePunctuation(text string) string {
	// 检测文本是否主要为 CJK 字符
	cjkCount := 0
	totalCount := 0
	for _, r := range text {
		if !unicode.IsSpace(r) {
			totalCount++
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Katakana, r) ||
				unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Hangul, r) {
				cjkCount++
			}
		}
	}

	// 如果 CJK 字符占比超过 30%，保留全角标点
	if totalCount > 0 && float64(cjkCount)/float64(totalCount) > 0.3 {
		// 仅做基本清理：去除多余空格
		return text
	}

	// 非 CJK 文本：全角→半角转换
	replacer := strings.NewReplacer(
		"，", ",",
		"。", ".",
		"！", "!",
		"？", "?",
		"；", ";",
		"：", ":",
		"（", "(",
		"）", ")",
		"【", "[",
		"】", "]",
		"「", "\"",
		"」", "\"",
		"『", "'",
		"』", "'",
		"—", "-",
		"…", "...",
		"\u3000", " ", // 全角空格
	)
	return replacer.Replace(text)
}

// cleanWhitespace 清理多余空白字符
func cleanWhitespace(text string) string {
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.Join(cleanLines, "\n")
}

// ==================== 时间轴标准化 ====================

// normalizeTimeline 标准化时间轴
func (c *SubtitleCleaner) normalizeTimeline(cues []vttCue) []vttCue {
	if len(cues) == 0 {
		return cues
	}

	for i := range cues {
		startMs := parseVTTTimeMs(cues[i].startTime)
		endMs := parseVTTTimeMs(cues[i].endTime)

		// 应用全局时间偏移
		if c.config.TimeOffsetMs != 0 {
			startMs += c.config.TimeOffsetMs
			endMs += c.config.TimeOffsetMs
			// 确保不为负数
			if startMs < 0 {
				startMs = 0
			}
			if endMs < 0 {
				endMs = 0
			}
		}

		// 确保结束时间 > 开始时间
		if endMs <= startMs {
			endMs = startMs + 2000 // 默认 2 秒显示时长
		}

		// 确保不超过最大显示时长
		if c.config.MaxDurationMs > 0 && (endMs-startMs) > c.config.MaxDurationMs {
			endMs = startMs + c.config.MaxDurationMs
		}

		cues[i].startTime = formatVTTTimeFromMs(startMs)
		cues[i].endTime = formatVTTTimeFromMs(endMs)
	}

	return cues
}

// ==================== 分段与合并优化 ====================

// optimizeSegments 优化字幕分段（合并过短、拆分过长）
// 返回值: (优化后的cues, 合并数, 拆分数)
func (c *SubtitleCleaner) optimizeSegments(cues []vttCue) ([]vttCue, int, int) {
	if len(cues) == 0 {
		return cues, 0, 0
	}

	mergedCount := 0
	splitCount := 0

	// 第一步：合并过短的字幕
	if c.config.MergeShortCues && c.config.MinDurationMs > 0 {
		var merged []vttCue
		i := 0
		for i < len(cues) {
			current := cues[i]
			startMs := parseVTTTimeMs(current.startTime)
			endMs := parseVTTTimeMs(current.endTime)
			durationMs := endMs - startMs

			// 如果当前条目过短，尝试与下一条合并
			if durationMs < c.config.MinDurationMs && i+1 < len(cues) {
				nextStartMs := parseVTTTimeMs(cues[i+1].startTime)
				gap := nextStartMs - endMs

				// 间隔足够小才合并
				if c.config.MinGapMs > 0 && gap < c.config.MinGapMs {
					// 合并：扩展当前条目到下一条的结束时间
					nextEndMs := parseVTTTimeMs(cues[i+1].endTime)
					current.endTime = formatVTTTimeFromMs(nextEndMs)
					current.text = current.text + "\n" + cues[i+1].text
					mergedCount++
					i += 2 // 跳过下一条
					merged = append(merged, current)
					continue
				}
			}

			merged = append(merged, current)
			i++
		}
		cues = merged
	}

	// 第二步：拆分过长的字幕（按字符数）
	if c.config.SplitLongCues && c.config.MaxCharsPerLine > 0 {
		var split []vttCue
		for _, cue := range cues {
			textLen := utf8.RuneCountInString(cue.text)
			maxLines := c.config.MaxLinesPerCue
			if maxLines <= 0 {
				maxLines = 2
			}
			maxChars := c.config.MaxCharsPerLine * maxLines

			if textLen > maxChars {
				// 需要拆分
				subCues := c.splitCue(cue, maxChars)
				split = append(split, subCues...)
				splitCount += len(subCues) - 1
			} else {
				split = append(split, cue)
			}
		}
		cues = split
	}

	return cues, mergedCount, splitCount
}

// splitCue 将过长的字幕条目拆分为多个
func (c *SubtitleCleaner) splitCue(cue vttCue, maxChars int) []vttCue {
	runes := []rune(cue.text)
	totalRunes := len(runes)
	if totalRunes <= maxChars {
		return []vttCue{cue}
	}

	startMs := parseVTTTimeMs(cue.startTime)
	endMs := parseVTTTimeMs(cue.endTime)
	totalDuration := endMs - startMs

	// 计算需要拆分为几段
	parts := (totalRunes + maxChars - 1) / maxChars
	if parts <= 1 {
		return []vttCue{cue}
	}

	durationPerPart := totalDuration / int64(parts)
	var result []vttCue

	for i := 0; i < parts; i++ {
		start := i * maxChars
		end := start + maxChars
		if end > totalRunes {
			end = totalRunes
		}

		partStartMs := startMs + int64(i)*durationPerPart
		partEndMs := partStartMs + durationPerPart
		if i == parts-1 {
			partEndMs = endMs // 最后一段用原始结束时间
		}

		result = append(result, vttCue{
			startTime: formatVTTTimeFromMs(partStartMs),
			endTime:   formatVTTTimeFromMs(partEndMs),
			text:      string(runes[start:end]),
		})
	}

	return result
}

// ==================== 时间工具函数 ====================

// parseVTTTimeMs 解析 VTT 时间戳为毫秒数
// 支持格式: "HH:MM:SS.mmm" 或 "MM:SS.mmm"
func parseVTTTimeMs(timeStr string) int64 {
	timeStr = strings.TrimSpace(timeStr)
	// 移除可能的位置信息（如 "00:01:23.456 position:10%"）
	if idx := strings.Index(timeStr, " "); idx > 0 {
		timeStr = timeStr[:idx]
	}

	// 统一分隔符
	timeStr = strings.Replace(timeStr, ",", ".", 1)

	parts := strings.Split(timeStr, ":")
	var hours, minutes, seconds int64
	var milliseconds int64

	switch len(parts) {
	case 3:
		// HH:MM:SS.mmm
		fmt.Sscanf(parts[0], "%d", &hours)
		fmt.Sscanf(parts[1], "%d", &minutes)
		secParts := strings.Split(parts[2], ".")
		fmt.Sscanf(secParts[0], "%d", &seconds)
		if len(secParts) > 1 {
			msStr := secParts[1]
			// 补齐到 3 位
			for len(msStr) < 3 {
				msStr += "0"
			}
			if len(msStr) > 3 {
				msStr = msStr[:3]
			}
			fmt.Sscanf(msStr, "%d", &milliseconds)
		}
	case 2:
		// MM:SS.mmm
		fmt.Sscanf(parts[0], "%d", &minutes)
		secParts := strings.Split(parts[1], ".")
		fmt.Sscanf(secParts[0], "%d", &seconds)
		if len(secParts) > 1 {
			msStr := secParts[1]
			for len(msStr) < 3 {
				msStr += "0"
			}
			if len(msStr) > 3 {
				msStr = msStr[:3]
			}
			fmt.Sscanf(msStr, "%d", &milliseconds)
		}
	default:
		return 0
	}

	return hours*3600000 + minutes*60000 + seconds*1000 + milliseconds
}

// formatVTTTimeFromMs 将毫秒数格式化为 VTT 时间戳 "HH:MM:SS.mmm"
func formatVTTTimeFromMs(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	ms %= 3600000
	minutes := ms / 60000
	ms %= 60000
	seconds := ms / 1000
	milliseconds := ms % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, milliseconds)
}

// ==================== 文件工具函数 ====================

// copyFile 复制文件
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("读取源文件失败: %w", err)
	}
	return os.WriteFile(dst, data, 0644)
}

// writeVTTToFile 将 cues 写入 VTT 文件
func writeVTTToFile(vttPath string, cues []vttCue) error {
	var buf strings.Builder
	buf.WriteString("WEBVTT\n\n")

	for i, cue := range cues {
		fmt.Fprintf(&buf, "%d\n", i+1)
		fmt.Fprintf(&buf, "%s --> %s\n", cue.startTime, cue.endTime)
		fmt.Fprintf(&buf, "%s\n\n", cue.text)
	}

	return os.WriteFile(vttPath, []byte(buf.String()), 0644)
}

// ==================== 辅助：确保 math 包被使用 ====================

var _ = math.MaxInt64
