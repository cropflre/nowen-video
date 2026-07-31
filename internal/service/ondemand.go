// Package service implements request-driven video and audio segments as part of
// the shared media execution runtime.
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

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
	"go.uber.org/zap"
)

const onDemandSegmentSeconds = hlsTargetSegmentSeconds

type onDemandKeyLock struct {
	mu   sync.Mutex
	refs int
}

type onDemandLimiter struct {
	mu   sync.Mutex
	keys map[string]*onDemandKeyLock
}

var defaultOnDemandLimiter = &onDemandLimiter{keys: make(map[string]*onDemandKeyLock)}

// acquire serializes duplicate work for one artifact and removes the lock entry
// after the last waiter leaves, preventing unbounded growth across long-lived
// servers and large seek histories.
func (l *onDemandLimiter) acquire(key string) func() {
	l.mu.Lock()
	entry := l.keys[key]
	if entry == nil {
		entry = &onDemandKeyLock{}
		l.keys[key] = entry
	}
	entry.refs++
	l.mu.Unlock()

	entry.mu.Lock()
	var once sync.Once
	return func() {
		once.Do(func() {
			entry.mu.Unlock()
			l.mu.Lock()
			entry.refs--
			if entry.refs == 0 {
				delete(l.keys, key)
			}
			l.mu.Unlock()
		})
	}
}

func (l *onDemandLimiter) size() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.keys)
}

func (s *StreamService) ServeOnDemandSegment(mediaID, quality, segName string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return fmt.Errorf("STRM 远程流不支持按需分段")
	}
	if s.transcoder == nil || s.transcoder.ExecutionRuntime() == nil {
		return fmt.Errorf("媒体执行 Runtime 不可用")
	}

	segIndex, err := parseSegmentIndex(segName)
	if err != nil {
		return fmt.Errorf("无效的分片名 %s: %w", segName, err)
	}
	outputDir := filepath.Join(s.transcoder.GetOutputDir(mediaID, quality), "ondemand")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	segPath := filepath.Join(outputDir, segName)

	release := defaultOnDemandLimiter.acquire(mediaID + "/" + quality + "/" + segName)
	defer release()
	if fi, err := os.Stat(segPath); err == nil && fi.Size() > 0 {
		http.ServeFile(w, r, segPath)
		return nil
	}

	startSec := float64(segIndex) * float64(onDemandSegmentSeconds)
	qc, ok := qualityPresets[quality]
	if !ok {
		qc = qualityPresets["720p"]
	}
	inputPath := media.FilePath
	if IsWebDAVPath(inputPath) {
		inputPath = ResolveRemoteFFmpegURL(s.cfg, inputPath)
	}
	tempPath := fmt.Sprintf("%s.part-%d", segPath, time.Now().UnixNano())
	defer os.Remove(tempPath)

	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", inputPath,
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
		tempPath,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	result := s.transcoder.ExecutionRuntime().Run(ctx, transcodegovernor.KindOnDemand, transcodeexecutor.Command{
		Path:       s.cfg.App.FFmpegPath,
		Args:       args,
		StderrTail: 60,
		Prepare: func(cmd *exec.Cmd) {
			setLowPriority(cmd)
		},
	}, transcodeexecutor.Callbacks{})
	if result.Err != nil {
		s.logger.Warnf("on-demand 切片失败 media=%s quality=%s seg=%d: %s", mediaID, quality, segIndex, result.ErrorText())
		return fmt.Errorf("切片失败: %s", result.ErrorText())
	}
	if fi, err := os.Stat(tempPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("切片输出为空")
	}
	if err := os.Rename(tempPath, segPath); err != nil {
		return fmt.Errorf("发布切片失败: %w", err)
	}
	s.transcoder.InvalidateCacheDiskUsage()
	http.ServeFile(w, r, segPath)
	return nil
}

func parseSegmentIndex(name string) (int, error) {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	digitStart := -1
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

type AudioTrackInfo struct {
	Index    int    `json:"index"`
	AudioIdx int    `json:"audio_idx"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Channels int    `json:"channels"`
	Default  bool   `json:"default"`
}

func probeAudioTracks(media *model.Media, ffprobePath string, logger *zap.SugaredLogger) []AudioTrackInfo {
	if media == nil || media.StreamURL != "" || IsWebDAVPath(media.FilePath) {
		return nil
	}
	if _, err := os.Stat(media.FilePath); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index,codec_name,channels:stream_tags=language,title:stream_disposition=default",
		"-of", "json",
		media.FilePath,
	)
	out, err := cmd.Output()
	if err != nil {
		if logger != nil {
			logger.Debugf("probeAudioTracks ffprobe failed: %v", err)
		}
		return nil
	}

	var probe struct {
		Streams []struct {
			Index       int               `json:"index"`
			CodecName   string            `json:"codec_name"`
			Channels    int               `json:"channels"`
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
	for i, stream := range probe.Streams {
		track := AudioTrackInfo{
			Index:    stream.Index,
			AudioIdx: i,
			Codec:    stream.CodecName,
			Channels: stream.Channels,
			Default:  stream.Disposition.Default == 1,
		}
		if stream.Tags != nil {
			track.Language = stream.Tags["language"]
			track.Title = stream.Tags["title"]
		}
		tracks = append(tracks, track)
	}
	return tracks
}

func (s *StreamService) GetAudioTracks(mediaID string) []AudioTrackInfo {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil
	}
	return probeAudioTracks(media, s.cfg.App.FFprobePath, s.logger)
}

func buildAudioMediaEntries(mediaID string, tracks []AudioTrackInfo) string {
	if len(tracks) <= 1 {
		return ""
	}
	var builder strings.Builder
	defaultPicked := false
	for index, track := range tracks {
		name := track.Title
		if name == "" {
			name = track.Language
		}
		if name == "" {
			name = fmt.Sprintf("Track %d", index+1)
		}
		if track.Codec != "" {
			name = fmt.Sprintf("%s [%s]", name, strings.ToUpper(track.Codec))
		}
		language := track.Language
		if language == "" {
			language = "und"
		}
		isDefault := "NO"
		if !defaultPicked && (track.Default || index == 0) {
			isDefault = "YES"
			defaultPicked = true
		}
		builder.WriteString(fmt.Sprintf(
			"#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"%s\",LANGUAGE=\"%s\",DEFAULT=%s,AUTOSELECT=YES,URI=\"/api/audio-track/%s/%d.m3u8\"\n",
			escapeAttr(name), escapeAttr(language), isDefault, mediaID, track.AudioIdx,
		))
	}
	return builder.String()
}

func (s *StreamService) BuildAudioMediaEntries(mediaID string, tracks []AudioTrackInfo) string {
	return buildAudioMediaEntries(mediaID, tracks)
}

func escapeAttr(value string) string {
	return strings.ReplaceAll(value, `"`, `'`)
}

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

	totalSegments := int(media.Duration) / onDemandSegmentSeconds
	if int(media.Duration)%onDemandSegmentSeconds != 0 {
		totalSegments++
	}
	if totalSegments <= 0 {
		totalSegments = 1
	}

	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-VERSION:3\n")
	builder.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", onDemandSegmentSeconds))
	builder.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	builder.WriteString("#EXT-X-PLAYLIST-TYPE:VOD\n")
	for index := 0; index < totalSegments; index++ {
		duration := float64(onDemandSegmentSeconds)
		if index == totalSegments-1 {
			remaining := media.Duration - float64(index*onDemandSegmentSeconds)
			if remaining > 0 && remaining < duration {
				duration = remaining
			}
		}
		builder.WriteString(fmt.Sprintf("#EXTINF:%.3f,\n", duration))
		builder.WriteString(fmt.Sprintf("/api/audio-track/%s/%d/seg_%04d.aac\n", mediaID, trackIdx, index))
	}
	builder.WriteString("#EXT-X-ENDLIST\n")
	return builder.String(), nil
}

func (s *StreamService) ServeAudioSegment(mediaID string, trackIdx int, segName string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return fmt.Errorf("STRM 远程流不支持音轨切片")
	}
	if s.transcoder == nil || s.transcoder.ExecutionRuntime() == nil {
		return fmt.Errorf("媒体执行 Runtime 不可用")
	}

	segIndex, err := parseSegmentIndex(segName)
	if err != nil {
		return fmt.Errorf("无效的分片名 %s: %w", segName, err)
	}
	outputDir := filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, "audio", strconv.Itoa(trackIdx))
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	segPath := filepath.Join(outputDir, segName)

	release := defaultOnDemandLimiter.acquire(mediaID + "/audio/" + strconv.Itoa(trackIdx) + "/" + segName)
	defer release()
	if fi, err := os.Stat(segPath); err == nil && fi.Size() > 0 {
		http.ServeFile(w, r, segPath)
		return nil
	}

	inputPath := media.FilePath
	if IsWebDAVPath(inputPath) {
		inputPath = ResolveRemoteFFmpegURL(s.cfg, inputPath)
	}
	startSec := float64(segIndex) * float64(onDemandSegmentSeconds)
	tempPath := fmt.Sprintf("%s.part-%d", segPath, time.Now().UnixNano())
	defer os.Remove(tempPath)
	args := []string{
		"-y",
		"-ss", fmt.Sprintf("%.3f", startSec),
		"-i", inputPath,
		"-t", strconv.Itoa(onDemandSegmentSeconds),
		"-map", fmt.Sprintf("0:a:%d", trackIdx),
		"-vn",
		"-c:a", "aac",
		"-profile:a", "aac_low",
		"-b:a", "128k",
		"-ac", "2",
		"-f", "adts",
		tempPath,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	result := s.transcoder.ExecutionRuntime().Run(ctx, transcodegovernor.KindOnDemand, transcodeexecutor.Command{
		Path:       s.cfg.App.FFmpegPath,
		Args:       args,
		StderrTail: 60,
		Prepare: func(cmd *exec.Cmd) {
			setLowPriority(cmd)
		},
	}, transcodeexecutor.Callbacks{})
	if result.Err != nil {
		s.logger.Warnf("audio on-demand 切片失败 media=%s track=%d seg=%d: %s", mediaID, trackIdx, segIndex, result.ErrorText())
		return fmt.Errorf("音轨切片失败: %s", result.ErrorText())
	}
	if fi, err := os.Stat(tempPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("音轨切片输出为空")
	}
	if err := os.Rename(tempPath, segPath); err != nil {
		return fmt.Errorf("发布音轨切片失败: %w", err)
	}
	s.transcoder.InvalidateCacheDiskUsage()
	http.ServeFile(w, r, segPath)
	return nil
}
