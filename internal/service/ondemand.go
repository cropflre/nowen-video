// Package service
//
// 按需分段切片（on-demand segment transcoding）与多音轨支持。
//
// 背景
// ----
// 原有主转码流程（transcode.go）走的是"顺序 HLS 流水线"：FFmpeg 从 -ss 位置开始
// 持续推流，依赖 m3u8 的 EVENT 模式边写边播。这在顺序观看时非常高效，
// 但面对以下场景会变差：
//
//  1. 用户大范围 seek 到一个从未转码过的位置。原流程需要杀掉旧进程重启 FFmpeg，
//     首片延迟 2~5s；且旧进程刚写入的缓冲会被废弃。
//  2. 视频库支持多音轨（例如国语/日语/英语），hls.js 切换音轨时需要独立的
//     audio playlist，而当前主转码只输出混流的 .ts。
//
// 本文件实现"按需分段切片"，作为主流程的补充：
//   - 视频分片：/api/stream/:id/:quality/ondemand/seg_N.ts
//       收到请求时若 seg_N.ts 不存在，就独立 ffmpeg 切一片再返回
//   - 音频分片：/api/stream/:id/audio/:track/seg_N.aac
//       为指定音轨号按需切片
//   - 音频 playlist：/api/stream/:id/audio/:track.m3u8
//       基于 media.Duration 静态生成（每 2s 一片），客户端按需拉取
//
// 这样 Master Playlist 可以通过 #EXT-X-MEDIA:TYPE=AUDIO 声明多音轨，
// 客户端切换音轨时无需触发整流重启。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 按需分段的目标时长（秒），必须与主转码的 hlsTargetSegmentSeconds 一致，
// 否则 hls.js 拼接不同来源的分片时会错位。
const onDemandSegmentSeconds = hlsTargetSegmentSeconds

// 按需切片的并发上限：避免客户端乱序请求 seg_10/20/30 时同时起 10 个 FFmpeg
// 具体到单个 media+quality，限制为每次只允许一个进程占用同一输出目录
type onDemandLimiter struct {
	mu   sync.Mutex
	keys map[string]*sync.Mutex
}

var defaultOnDemandLimiter = &onDemandLimiter{keys: make(map[string]*sync.Mutex)}

// acquire 获取某个 key 的互斥锁；释放时调用返回的函数
func (l *onDemandLimiter) acquire(key string) func() {
	l.mu.Lock()
	m, ok := l.keys[key]
	if !ok {
		m = &sync.Mutex{}
		l.keys[key] = m
	}
	l.mu.Unlock()
	m.Lock()
	return m.Unlock
}

// ==================== 视频按需分段 ====================

// ServeOnDemandSegment 供 handler 层调用：当主转码流程还没产出指定 seg_N.ts 时，
// 独立起一个 FFmpeg 从 N*2 秒位置切 2 秒片段，直接写到 HTTP 响应。
//
// 与主转码的关系：
//   - 切出来的分片也会写入 outputDir，下次直接命中 http.ServeFile
//   - 不干扰主转码的 FFmpeg 进程（在独立临时目录 ondemand/ 下切）
//
// 调用路径：
//   GET /api/stream/:id/:quality/ondemand/seg_0003.ts
func (s *StreamService) ServeOnDemandSegment(mediaID, quality, segName string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return fmt.Errorf("STRM 远程流不支持按需分段")
	}

	// 解析段号：seg_0003.ts -> 3
	segIndex, err := parseSegmentIndex(segName)
	if err != nil {
		return fmt.Errorf("无效的分片名 %s: %w", segName, err)
	}

	outputDir := filepath.Join(s.transcoder.GetOutputDir(mediaID, quality), "ondemand")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	segPath := filepath.Join(outputDir, segName)

	// 加锁：同一 media+quality+seg 只允许一个请求在切
	// 后续请求可在前序完成后直接读文件
	release := defaultOnDemandLimiter.acquire(mediaID + "/" + quality + "/" + segName)
	defer release()

	// 双重检查：锁内再查一次（前一个请求可能刚切完）
	if fi, err := os.Stat(segPath); err == nil && fi.Size() > 0 {
		http.ServeFile(w, r, segPath)
		return nil
	}

	// 构建切片命令：从 N * 2 秒开始，切 2 秒
	startSec := float64(segIndex) * float64(onDemandSegmentSeconds)
	qc, ok := qualityPresets[quality]
	if !ok {
		qc = qualityPresets["720p"]
	}

	// 独立 FFmpeg 命令，只切一片，直接落盘
	// 关键点：
	//   -ss 放在 -i 前，走 demux 层快速跳转（精度略差但速度 ~50x）
	//   -t 2         仅编码 2 秒
	//   -copyts      保留原始时间戳（避免多片拼接时出现时间戳跳变）
	//   -avoid_negative_ts make_zero  防止负时间戳导致的切片异常
	//   -reset_timestamps 1          每个输出重置 PTS（使 hls.js 认为是独立片段）
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", media.FilePath,
		"-t", strconv.Itoa(onDemandSegmentSeconds),
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "libx264",
		"-preset", "ultrafast",
		"-tune", "zerolatency",
		"-b:v", qc.VideoBitrate,
		"-maxrate", qc.VideoBitrate,
		"-bufsize", qc.VideoBitrate,
		"-vf", fmt.Sprintf("scale=%d:%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2",
			qc.Width, qc.Height, qc.Width, qc.Height),
		"-c:a", "aac",
		"-b:a", qc.AudioBitrate,
		"-ac", "2",
		"-copyts",
		"-avoid_negative_ts", "make_zero",
		"-f", "mpegts",
		"-muxdelay", "0",
		"-muxpreload", "0",
		segPath,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath, args...)
	// stderr 只在失败时读取（避免大量正常日志）
	stderrOut, err := cmd.CombinedOutput()
	if err != nil {
		// 切片失败时清理可能残留的空文件
		_ = os.Remove(segPath)
		tail := string(stderrOut)
		if len(tail) > 400 {
			tail = tail[len(tail)-400:]
		}
		s.logger.Warnf("on-demand 切片失败 media=%s quality=%s seg=%d: %v\nstderr(tail): %s",
			mediaID, quality, segIndex, err, tail)
		return fmt.Errorf("切片失败: %w", err)
	}

	if fi, err := os.Stat(segPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("切片输出为空")
	}

	http.ServeFile(w, r, segPath)
	return nil
}

// parseSegmentIndex 从 "seg_0003.ts" / "seg3.ts" / "segment_12.aac" 中提取序号
func parseSegmentIndex(name string) (int, error) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	// 找最后一个下划线/非数字边界
	var digitStart = -1
	for i := len(base) - 1; i >= 0; i-- {
		ch := base[i]
		if ch >= '0' && ch <= '9' {
			digitStart = i
		} else {
			break
		}
	}
	if digitStart < 0 {
		return 0, fmt.Errorf("no digits in %s", name)
	}
	return strconv.Atoi(base[digitStart:])
}

// ==================== 多音轨支持 ====================

// AudioTrackInfo 描述视频中的一条音频轨
type AudioTrackInfo struct {
	Index    int    `json:"index"`     // 在文件内的流索引
	AudioIdx int    `json:"audio_idx"` // 仅音频流中的序号（0,1,2...）
	Codec    string `json:"codec"`     // aac/ac3/eac3/dts...
	Language string `json:"language"`  // zh/en/ja...
	Title    string `json:"title"`
	Channels int    `json:"channels"`
	Default  bool   `json:"default"`
}

// GetAudioTracks 探测媒体的所有音频轨。
// 使用 ffprobe，结果用于 Master Playlist 注入 #EXT-X-MEDIA:TYPE=AUDIO。
//
// 失败时（ffprobe 调用错误/解析失败）返回空切片，让 master playlist 回退到
// 单音轨模式，保持向后兼容。
func (s *StreamService) GetAudioTracks(mediaID string) []AudioTrackInfo {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil || media.StreamURL != "" {
		return nil
	}
	// WebDAV / 远程路径不做探测（ffprobe 读远程开销过大）
	if IsWebDAVPath(media.FilePath) {
		return nil
	}
	if _, err := os.Stat(media.FilePath); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.cfg.App.FFprobePath,
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index,codec_name,channels:stream_tags=language,title:stream_disposition=default",
		"-of", "json",
		media.FilePath,
	)
	out, err := cmd.Output()
	if err != nil {
		s.logger.Debugf("GetAudioTracks ffprobe failed: %v", err)
		return nil
	}

	var probe struct {
		Streams []struct {
			Index       int    `json:"index"`
			CodecName   string `json:"codec_name"`
			Channels    int    `json:"channels"`
			Tags        map[string]string `json:"tags"`
			Disposition struct {
				Default int `json:"default"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil
	}
	tracks := make([]AudioTrackInfo, 0, len(probe.Streams))
	for i, st := range probe.Streams {
		t := AudioTrackInfo{
			Index:    st.Index,
			AudioIdx: i,
			Codec:    st.CodecName,
			Channels: st.Channels,
			Default:  st.Disposition.Default == 1,
		}
		if st.Tags != nil {
			t.Language = st.Tags["language"]
			t.Title = st.Tags["title"]
		}
		tracks = append(tracks, t)
	}
	return tracks
}

// BuildAudioMediaEntries 为 Master Playlist 构造 #EXT-X-MEDIA:TYPE=AUDIO 行。
// 当且仅当返回 GROUP-ID="audio" 引用了实际轨道时才应该被注入到 EXT-X-STREAM-INF。
func (s *StreamService) BuildAudioMediaEntries(mediaID string, tracks []AudioTrackInfo) string {
	if len(tracks) <= 1 {
		return ""
	}
	var sb strings.Builder
	defaultPicked := false
	for i, t := range tracks {
		name := t.Title
		if name == "" {
			name = t.Language
		}
		if name == "" {
			name = fmt.Sprintf("Track %d", i+1)
		}
		lang := t.Language
		if lang == "" {
			lang = "und"
		}
		isDefault := "NO"
		// 优先选 disposition=default，否则选第一条
		if (!defaultPicked && (t.Default || i == 0)) {
			isDefault = "YES"
			defaultPicked = true
		}
		sb.WriteString(fmt.Sprintf(
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",LANGUAGE=\"%s\",DEFAULT=%s,AUTOSELECT=YES,URI=\"/api/audio-track/%s/%d.m3u8\"\n",
			escapeAttr(name), escapeAttr(lang), isDefault, mediaID, t.AudioIdx,
		))
	}
	return sb.String()
}

// escapeAttr 转义 HLS 属性字符串中的双引号（规范要求）
func escapeAttr(s string) string {
	return strings.ReplaceAll(s, `"`, `'`)
}

// GetAudioPlaylist 为指定音轨按需构造 playlist。
// 基于 media.Duration 静态生成每 2s 一片的 m3u8，前端请求 seg_N.aac 时再按需切。
// 这样不需要预转码整个音轨，节省 I/O 和 CPU。
func (s *StreamService) GetAudioPlaylist(mediaID string, trackIdx int) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return "", fmt.Errorf("STRM 远程流不支持独立音轨")
	}
	if media.Duration <= 0 {
		return "", fmt.Errorf("未知的媒体时长")
	}

	// 计算分片总数（向上取整）
	totalSegs := int(media.Duration) / onDemandSegmentSeconds
	if int(media.Duration)%onDemandSegmentSeconds != 0 {
		totalSegs++
	}
	if totalSegs <= 0 {
		totalSegs = 1
	}

	var sb strings.Builder
	sb.WriteString("#EXTM3U\n")
	sb.WriteString("#EXT-X-VERSION:3\n")
	sb.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", onDemandSegmentSeconds))
	sb.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	sb.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")

	for i := 0; i < totalSegs; i++ {
		segDur := float64(onDemandSegmentSeconds)
		// 最后一片可能不足 2s
		if i == totalSegs-1 {
			remain := media.Duration - float64(i*onDemandSegmentSeconds)
			if remain > 0 && remain < segDur {
				segDur = remain
			}
		}
		sb.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", segDur))
		sb.WriteString(fmt.Sprintf("/api/audio-track/%s/%d/seg_%04d.aac\n", mediaID, trackIdx, i))
	}
	sb.WriteString("#EXT-X-ENDLIST\n")
	return sb.String(), nil
}

// ServeAudioSegment 按需切指定音轨的单个 AAC 分片
//
// 路径：GET /api/stream/:id/audio/:trackIdx/seg_NNNN.aac
//
// 与视频按需切片一样，用 limiter 防止同一分片的并发切片重复启动。
func (s *StreamService) ServeAudioSegment(mediaID string, trackIdx int, segName string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return fmt.Errorf("STRM 远程流不支持音轨切片")
	}

	segIndex, err := parseSegmentIndex(segName)
	if err != nil {
		return fmt.Errorf("无效的分片名 %s: %w", segName, err)
	}

	outputDir := filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, "audio", strconv.Itoa(trackIdx))
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	segPath := filepath.Join(outputDir, segName)

	release := defaultOnDemandLimiter.acquire(mediaID + "/audio/" + strconv.Itoa(trackIdx) + "/" + segName)
	defer release()

	if fi, err := os.Stat(segPath); err == nil && fi.Size() > 0 {
		http.ServeFile(w, r, segPath)
		return nil
	}

	startSec := float64(segIndex) * float64(onDemandSegmentSeconds)
	// 选第 trackIdx 条音频流并转码为 aac
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", media.FilePath,
		"-t", strconv.Itoa(onDemandSegmentSeconds),
		"-map", fmt.Sprintf("0:a:%d", trackIdx),
		"-vn",
		"-c:a", "aac",
		"-b:a", "128k",
		"-ac", "2",
		"-f", "adts", // 裸 AAC 容器，hls.js 对 .aac playlist 直接支持
		segPath,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(segPath)
		tail := string(out)
		if len(tail) > 400 {
			tail = tail[len(tail)-400:]
		}
		s.logger.Warnf("audio on-demand 切片失败 media=%s track=%d seg=%d: %v\nstderr(tail): %s",
			mediaID, trackIdx, segIndex, err, tail)
		return err
	}

	http.ServeFile(w, r, segPath)
	return nil
}

// 保证 io 未使用告警消失（根据实际使用情况移除）

