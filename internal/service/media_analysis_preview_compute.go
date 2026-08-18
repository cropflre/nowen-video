package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

const (
	MediaComputeJobPreviewThumbnailV1         = "preview_thumbnail_v1"
	MediaComputeCapabilityPreviewThumbnailV1 = "preview_thumbnail_v1"

	previewComputeFrameCount      = 5
	previewComputeFrameRate       = 2
	previewComputeMaxWidth        = 420
	previewComputeClientWait      = 10 * time.Second
	previewComputeNodeFreshness   = 20 * time.Second
	previewComputeMaxFrameBytes   = 320 * 1024
	previewComputeMaxPayloadBytes = 1600 * 1024
)

// MediaComputePreviewThumbnailInput 是 hover 预览图任务的私有输入。
// 客户端只做稀疏 Seek + 单帧解码，Server 不再读取整段源视频。
type MediaComputePreviewThumbnailInput struct {
	MediaID     string    `json:"media_id"`
	HighlightID string    `json:"highlight_id"`
	Fingerprint string    `json:"fingerprint"`
	StreamURL   string    `json:"stream_url"`
	FrameTimes  []float64 `json:"frame_times"`
	MaxWidth    int       `json:"max_width"`
	FrameRate   int       `json:"frame_rate"`
}

type MediaComputePreviewFrame struct {
	Time       float64 `json:"time"`
	Mime       string  `json:"mime"`
	DataBase64 string  `json:"data_base64"`
}

type MediaComputePreviewThumbnailResult struct {
	Fingerprint string                     `json:"fingerprint"`
	HighlightID string                     `json:"highlight_id"`
	Frames      []MediaComputePreviewFrame `json:"frames"`
}

type previewComputeOutcome struct {
	Path string
	Err  error
}

type previewComputeWaitState struct {
	mu      sync.Mutex
	waiters map[string]chan previewComputeOutcome
}

var previewComputeStates sync.Map

func previewComputeState(s *MediaAnalysisService) *previewComputeWaitState {
	if value, ok := previewComputeStates.Load(s); ok {
		return value.(*previewComputeWaitState)
	}
	created := &previewComputeWaitState{waiters: make(map[string]chan previewComputeOutcome)}
	actual, _ := previewComputeStates.LoadOrStore(s, created)
	return actual.(*previewComputeWaitState)
}

func (s *MediaAnalysisService) addPreviewComputeWaiter(taskID string) chan previewComputeOutcome {
	state := previewComputeState(s)
	state.mu.Lock()
	defer state.mu.Unlock()
	ch := make(chan previewComputeOutcome, 1)
	state.waiters[taskID] = ch
	return ch
}

func (s *MediaAnalysisService) removePreviewComputeWaiter(taskID string) {
	state := previewComputeState(s)
	state.mu.Lock()
	delete(state.waiters, taskID)
	state.mu.Unlock()
}

// HasIdleComputeNode 用于用户交互型任务的快速决策。它只把最近活跃且空闲的节点
// 视为可立即接单，避免 hover 请求为了已经退出或正忙的客户端白等一个租约周期。
func (s *MediaAnalysisService) HasIdleComputeNode(capability string) bool {
	state := mediaAnalysisState(s)
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()
	cleanupWorkersLocked(state, now)
	for _, worker := range state.workers {
		if worker.State != "idle" || now.Sub(worker.LastSeen) > previewComputeNodeFreshness {
			continue
		}
		if mediaComputeNodeCanRun(worker.MediaAnalysisWorkerHeartbeat, capability) {
			return true
		}
	}
	return false
}

func previewThumbnailFrameTimes(highlight model.VideoHighlight) []float64 {
	span := math.Max(0, highlight.EndTime-highlight.StartTime)
	if span <= 0 {
		return nil
	}
	duration := math.Min(2.5, math.Max(1, span))
	start := highlight.StartTime + math.Max(0, (span-duration)/2)
	step := duration / previewComputeFrameCount
	out := make([]float64, 0, previewComputeFrameCount)
	for i := 0; i < previewComputeFrameCount; i++ {
		point := start + (float64(i)+0.5)*step
		out = append(out, math.Min(highlight.EndTime, math.Max(highlight.StartTime, point)))
	}
	return out
}

func previewOutcome(waiter <-chan previewComputeOutcome) (previewComputeOutcome, bool) {
	select {
	case outcome := <-waiter:
		return outcome, true
	default:
		return previewComputeOutcome{}, false
	}
}

// EnsureHighlightPreviewDistributed 把现有 hover-lazy Animated WebP 接入 Media Compute Node V2。
// auto: Desktop -> Android -> Server fallback；client_preferred: 只等客户端；server_only: 直接 Server；off: 禁止新计算。
func (s *MediaAnalysisService) EnsureHighlightPreviewDistributed(mediaID, highlightID string) (string, error) {
	highlight, err := s.highlightRepo.FindByID(highlightID)
	if err != nil || highlight.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := existingPreviewPath(highlight); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}

	// 同一 highlight 的并发 hover 继续串行化，避免重复注册客户端任务或重复 Server FFmpeg。
	s.previewMu.Lock()
	defer s.previewMu.Unlock()

	highlight, err = s.highlightRepo.FindByID(highlightID)
	if err != nil || highlight.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := existingPreviewPath(highlight); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	if err := s.ensureSupported(media); err != nil {
		return "", err
	}

	mode := s.ExecutionMode()
	switch mode {
	case MediaAnalysisModeOff:
		return "", ErrMediaAnalysisDisabled
	case MediaAnalysisModeServerOnly:
		return s.generateAndPersistHighlightPreviewServer(media, highlight)
	}

	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		return "", err
	}
	frameTimes := previewThumbnailFrameTimes(*highlight)
	if len(frameTimes) == 0 {
		return "", ErrMediaAnalysisWorkerResult
	}

	if !s.HasIdleComputeNode(MediaComputeCapabilityPreviewThumbnailV1) {
		if mode == MediaAnalysisModeClientPreferred {
			return "", ErrMediaAnalysisWorkerNoTask
		}
		return s.generateAndPersistHighlightPreviewServer(media, highlight)
	}

	input := MediaComputePreviewThumbnailInput{
		MediaID: media.ID, HighlightID: highlight.ID, Fingerprint: fingerprint,
		StreamURL: "/api/stream/" + media.ID + "/direct", FrameTimes: frameTimes,
		MaxWidth: previewComputeMaxWidth, FrameRate: previewComputeFrameRate,
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	taskID := uuid.NewString()
	waiter := s.addPreviewComputeWaiter(taskID)
	waiterActive := true
	defer func() {
		if waiterActive {
			s.removePreviewComputeWaiter(taskID)
		}
		s.UnregisterComputeTask(taskID)
	}()
	if err := s.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: taskID, MediaID: media.ID, Fingerprint: fingerprint,
		JobType: MediaComputeJobPreviewThumbnailV1,
		RequiredCapability: MediaComputeCapabilityPreviewThumbnailV1,
		Input: inputJSON,
	}); err != nil {
		return "", err
	}

	timer := time.NewTimer(previewComputeClientWait)
	defer timer.Stop()
	select {
	case outcome := <-waiter:
		if outcome.Err == nil && strings.TrimSpace(outcome.Path) != "" {
			return outcome.Path, nil
		}
		if mode == MediaAnalysisModeClientPreferred {
			s.removePreviewComputeWaiter(taskID)
			waiterActive = false
			s.UnregisterComputeTask(taskID)
			if outcome.Err != nil {
				return "", outcome.Err
			}
			return "", ErrMediaAnalysisWorkerNoTask
		}
	case <-timer.C:
		// remove 会与完成端的最终封装临界区互斥。如果客户端已经进入最终完成，
		// 这里会等它结束，然后优先读取已经写入 waiter 的结果，而不是重复启动 Server fallback。
		s.removePreviewComputeWaiter(taskID)
		waiterActive = false
		s.UnregisterComputeTask(taskID)
		if outcome, ok := previewOutcome(waiter); ok && outcome.Err == nil && strings.TrimSpace(outcome.Path) != "" {
			return outcome.Path, nil
		}
		if mode == MediaAnalysisModeClientPreferred {
			return "", ErrMediaAnalysisWorkerNoTask
		}
		return s.generateAndPersistHighlightPreviewServer(media, highlight)
	}

	// 客户端主动回报了空/失败结果且当前是 auto：撤销远端任务再走 Server fallback。
	s.removePreviewComputeWaiter(taskID)
	waiterActive = false
	s.UnregisterComputeTask(taskID)
	return s.generateAndPersistHighlightPreviewServer(media, highlight)
}

func (s *MediaAnalysisService) generateAndPersistHighlightPreviewServer(media *model.Media, highlight *model.VideoHighlight) (string, error) {
	if media == nil || highlight == nil {
		return "", ErrMediaNotFound
	}
	dir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", media.ID, "previews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	output := filepath.Join(dir, highlight.ID+"-preview.webp")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := s.generatePreview(ctx, media.FilePath, *highlight, output); err != nil {
		_ = os.Remove(output)
		return "", err
	}
	highlight.PreviewPath = output
	if err := s.highlightRepo.Update(highlight); err != nil {
		_ = os.Remove(output)
		return "", err
	}
	return output, nil
}

func (s *MediaAnalysisService) completePreviewThumbnailComputeTask(
	taskID string,
	remote *mediaAnalysisRemoteTask,
	descriptor mediaComputeTaskDescriptor,
	payload json.RawMessage,
) error {
	if remote == nil {
		return ErrMediaAnalysisWorkerClaim
	}
	// 从这里到最终持久化持有 waiter 锁。超时取消会等待该临界区结束；
	// 若完成已开始，结果会优先返回给 hover 请求，不会再与 Server fallback 竞争写同一文件。
	waitState := previewComputeState(s)
	waitState.mu.Lock()
	waiter := waitState.waiters[taskID]
	if waiter == nil {
		waitState.mu.Unlock()
		return ErrMediaAnalysisWorkerClaim
	}
	defer waitState.mu.Unlock()

	var input MediaComputePreviewThumbnailInput
	if err := json.Unmarshal(descriptor.Input, &input); err != nil {
		return ErrMediaAnalysisWorkerResult
	}
	var result MediaComputePreviewThumbnailResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return ErrMediaAnalysisWorkerResult
	}
	if input.MediaID == "" || input.HighlightID == "" || input.Fingerprint == "" ||
		result.Fingerprint != input.Fingerprint || result.HighlightID != input.HighlightID ||
		remote.MediaID != input.MediaID || remote.Fingerprint != input.Fingerprint ||
		len(input.FrameTimes) < 2 || len(input.FrameTimes) > 8 || len(result.Frames) != len(input.FrameTimes) {
		return ErrMediaAnalysisWorkerResult
	}

	media, err := s.mediaRepo.FindByID(input.MediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		return err
	}
	if fingerprint != input.Fingerprint {
		return ErrMediaAnalysisFingerprintChanged
	}
	highlight, err := s.highlightRepo.FindByID(input.HighlightID)
	if err != nil || highlight.MediaID != input.MediaID {
		return gorm.ErrRecordNotFound
	}

	mime := ""
	decoded := make([][]byte, 0, len(result.Frames))
	payloadBytes := 0
	for index, frame := range result.Frames {
		if math.Abs(frame.Time-input.FrameTimes[index]) > 1.0 {
			return ErrMediaAnalysisWorkerResult
		}
		frameMime := strings.ToLower(strings.TrimSpace(frame.Mime))
		if mime == "" {
			mime = frameMime
		} else if mime != frameMime {
			return ErrMediaAnalysisWorkerResult
		}
		data, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(frame.DataBase64))
		if decodeErr != nil || len(data) == 0 || len(data) > previewComputeMaxFrameBytes || !validPreviewFrameData(frameMime, data) {
			return ErrMediaAnalysisWorkerResult
		}
		payloadBytes += len(data)
		if payloadBytes > previewComputeMaxPayloadBytes {
			return ErrMediaAnalysisWorkerResult
		}
		decoded = append(decoded, data)
	}

	baseDir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", media.ID, "previews")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return err
	}
	tempDir := filepath.Join(baseDir, ".preview-"+taskID)
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	ext := thumbnailExtension(mime)
	for index, data := range decoded {
		framePath := filepath.Join(tempDir, fmt.Sprintf("frame-%03d%s", index, ext))
		if err := os.WriteFile(framePath, data, 0o644); err != nil {
			return err
		}
	}
	outputTemp := filepath.Join(tempDir, "preview.webp")
	frameRate := clampInt(input.FrameRate, 1, 5)
	maxWidth := clampInt(input.MaxWidth, 160, previewComputeMaxWidth)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	sequence := filepath.Join(tempDir, "frame-%03d"+ext)
	filter := fmt.Sprintf("scale=%d:-2:flags=fast_bilinear", maxWidth)
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-framerate", fmt.Sprintf("%d", frameRate), "-i", sequence,
		"-vf", filter, "-loop", "0", "-c:v", "libwebp", "-quality", "54", "-y", outputTemp,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("preview frame assembly ffmpeg: %w: %s", err, strings.TrimSpace(string(data)))
	}

	output := filepath.Join(baseDir, highlight.ID+"-preview.webp")
	_ = os.Remove(output)
	if err := os.Rename(outputTemp, output); err != nil {
		return err
	}
	highlight.PreviewPath = output
	if err := s.highlightRepo.Update(highlight); err != nil {
		_ = os.Remove(output)
		return err
	}

	workerID := remote.ClaimedBy
	s.UnregisterComputeTask(taskID)
	select {
	case waiter <- previewComputeOutcome{Path: output}:
	default:
	}
	delete(waitState.waiters, taskID)
	if s.logger != nil {
		s.logger.Infof("distributed highlight preview completed media=%s highlight=%s worker=%s", media.ID, highlight.ID, workerID)
	}
	return nil
}

func validPreviewFrameData(mime string, data []byte) bool {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff
	case "image/png":
		return len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG" && data[4] == 0x0d && data[5] == 0x0a && data[6] == 0x1a && data[7] == 0x0a
	case "image/webp":
		return len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP"
	default:
		return false
	}
}
