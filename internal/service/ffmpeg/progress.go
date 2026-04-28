package ffmpeg

import (
	"bufio"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// 预编译正则（避免每次调用都重新编译）
var (
	timeRegex  = regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2})\.(\d{2})`)
	speedRegex = regexp.MustCompile(`speed=\s*([\d.]+)x`)
)

// ProgressEvent 一次 FFmpeg 进度回调的数据。
type ProgressEvent struct {
	// CurrentSec 当前已转码位置（秒）
	CurrentSec float64
	// Progress 百分比（0~100）
	Progress float64
	// Speed FFmpeg 输出的 speed 值，形如 "2.50x"；为空表示未解析到
	Speed string
}

// ProgressOptions 进度解析行为参数。
type ProgressOptions struct {
	// MinDeltaPct 两次回调间 Progress 最小变化量（百分比）。小于它的更新会被丢弃。
	// 例如 1.0 表示每 1% 回调一次。<=0 时默认 1.0。
	MinDeltaPct float64
	// CollectStderrLines 是否收集 stderr 最后 N 行用于错误诊断。<=0 时不收集。
	CollectStderrLines int
}

// ParseProgress 读取 FFmpeg stderr 并解析进度，按 MinDeltaPct 节流后回调 onProgress。
//
//   - totalDuration 总时长（秒），<=0 时不做回调（与历史 transcode/preprocess 行为一致）；
//   - onProgress 为 nil 时只消费 stderr 但不回调；
//   - 返回值为 stderr 最后 N 行（N=opts.CollectStderrLines），用于错误诊断。
//
// 说明：FFmpeg stderr 行分隔既有 \n 也有 \r，这里使用自定义 SplitFunc 处理两者。
func ParseProgress(
	stderr io.Reader,
	totalDuration float64,
	opts ProgressOptions,
	onProgress func(ProgressEvent),
) []string {
	if opts.MinDeltaPct <= 0 {
		opts.MinDeltaPct = 1.0
	}

	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	scanner.Split(splitFFmpegLine)

	var stderrRing []string
	if opts.CollectStderrLines > 0 {
		stderrRing = make([]string, 0, opts.CollectStderrLines)
	}

	var lastProgress float64

	for scanner.Scan() {
		line := scanner.Text()

		// 收集 stderr 最后 N 行
		if opts.CollectStderrLines > 0 {
			if trimmed := strings.TrimSpace(line); trimmed != "" {
				if len(stderrRing) >= opts.CollectStderrLines {
					stderrRing = stderrRing[1:]
				}
				stderrRing = append(stderrRing, trimmed)
			}
		}

		// 解析时间戳
		timeMatches := timeRegex.FindStringSubmatch(line)
		if len(timeMatches) < 5 || totalDuration <= 0 {
			continue
		}

		hours, _ := strconv.ParseFloat(timeMatches[1], 64)
		minutes, _ := strconv.ParseFloat(timeMatches[2], 64)
		seconds, _ := strconv.ParseFloat(timeMatches[3], 64)
		centis, _ := strconv.ParseFloat(timeMatches[4], 64)
		currentTime := hours*3600 + minutes*60 + seconds + centis/100

		progress := (currentTime / totalDuration) * 100
		if progress > 100 {
			progress = 100
		}

		// 节流：进度变化不足阈值则跳过
		if progress-lastProgress < opts.MinDeltaPct {
			continue
		}
		lastProgress = progress

		speed := ""
		if m := speedRegex.FindStringSubmatch(line); len(m) >= 2 {
			speed = m[1] + "x"
		}

		if onProgress != nil {
			onProgress(ProgressEvent{
				CurrentSec: currentTime,
				Progress:   progress,
				Speed:      speed,
			})
		}
	}

	return stderrRing
}

// splitFFmpegLine 以 \n 或 \r 作为行分隔符，用于 bufio.Scanner。
func splitFFmpegLine(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			return i + 1, data[:i], nil
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
