package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const (
	mediaHighlightTaskType = "media_highlight"
	coarseWindowSeconds    = 6.0
	refineWindowSeconds    = 10.0
	analysisWorkers        = 2
	thumbnailWorkers       = 2
	maxHighlightCount      = 8
	maxRefineCandidates    = 10
)

var (
	ErrMediaAnalysisInProgress = errors.New("media analysis already in progress")
	ErrMediaAnalysisUnsupported = errors.New("media source does not support local analysis")
)

// MediaAnalysisService provides local FFmpeg-only media understanding.
// Highlight generation deliberately has no AIService dependency and uses a
// sparse two-stage scanner so a movie is not decoded from beginning to end.
type MediaAnalysisService struct {
	cfg           *config.Config
	mediaRepo     *repository.MediaRepo
	highlightRepo *repository.VideoHighlightRepo
	taskRepo      *repository.AIAnalysisTaskRepo // legacy table/model reused as durable task storage
	logger        *zap.SugaredLogger
	wsHub         *WSHub
	semaphore     chan struct{}
	previewMu     sync.Mutex
}

type MediaHighlightList struct {
	Highlights  []model.VideoHighlight `json:"highlights"`
	Stale       bool                   `json:"stale"`
	Fingerprint string                 `json:"fingerprint,omitempty"`
}

type sparseSample struct {
	Start      float64
	Center     float64
	MeanDB     float64
	MaxDB      float64
	AudioScore float64
	SceneCount int
	Score      float64
	Method     string
}

type sampleResult struct {
	Sample sparseSample
	Err    error
}

func NewMediaAnalysisService(
	cfg *config.Config,
	mediaRepo *repository.MediaRepo,
	highlightRepo *repository.VideoHighlightRepo,
	taskRepo *repository.AIAnalysisTaskRepo,
	logger *zap.SugaredLogger,
) *MediaAnalysisService {
	s := &MediaAnalysisService{
		cfg:           cfg,
		mediaRepo:     mediaRepo,
		highlightRepo: highlightRepo,
		taskRepo:      taskRepo,
		logger:        logger,
		semaphore:     make(chan struct{}, 1), // NAS-safe default: one movie analysis at a time.
	}
	// FFmpeg subprocesses cannot safely resume after a process restart.
	if err := taskRepo.MarkRunningInterrupted(mediaHighlightTaskType); err != nil {
		logger.Warnf("mark interrupted media analysis tasks: %v", err)
	}
	return s
}

func (s *MediaAnalysisService) SetWSHub(hub *WSHub) { s.wsHub = hub }

func (s *MediaAnalysisService) ListHighlights(mediaID string) (*MediaHighlightList, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	highlights, err := s.highlightRepo.ListByMediaID(mediaID)
	if err != nil {
		return nil, err
	}
	fingerprint, _ := s.mediaFingerprint(media)
	stale := false
	for i := range highlights {
		if highlights[i].Fingerprint != "" && fingerprint != "" && highlights[i].Fingerprint != fingerprint {
			stale = true
			break
		}
	}
	return &MediaHighlightList{Highlights: highlights, Stale: stale, Fingerprint: fingerprint}, nil
}

func (s *MediaAnalysisService) LatestTask(mediaID string) (*model.AIAnalysisTask, error) {
	if active, err := s.taskRepo.FindActiveByMediaAndType(mediaID, mediaHighlightTaskType); err == nil {
		return active, nil
	}
	tasks, err := s.taskRepo.ListByMediaID(mediaID)
	if err != nil {
		return nil, err
	}
	for i := range tasks {
		if tasks[i].TaskType == mediaHighlightTaskType {
			return &tasks[i], nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *MediaAnalysisService) AnalyzeHighlights(mediaID string) (*model.AIAnalysisTask, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	if err := s.ensureSupported(media); err != nil {
		return nil, err
	}
	if existing, err := s.taskRepo.FindActiveByMediaAndType(mediaID, mediaHighlightTaskType); err == nil {
		return existing, ErrMediaAnalysisInProgress
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	task := &model.AIAnalysisTask{
		MediaID:  mediaID,
		TaskType: mediaHighlightTaskType,
		Status:   "pending",
		Stage:    "queued",
		Progress: 0,
	}
	if err := s.taskRepo.Create(task); err != nil {
		return nil, err
	}
	go s.runHighlightTask(task.ID, mediaID)
	return task, nil
}

func (s *MediaAnalysisService) runHighlightTask(taskID, mediaID string) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		s.failTask(task, "probe", ErrMediaNotFound)
		return
	}

	now := time.Now()
	task.Status = "running"
	task.StartedAt = &now
	s.updateTask(task, "probe", 5, "")

	if err := s.ensureSupported(media); err != nil {
		s.failTask(task, "probe", err)
		return
	}
	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		s.failTask(task, "probe", err)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			s.failTask(task, task.Stage, fmt.Errorf("内部错误: %v", r))
		}
	}()

	// V2: sparse coarse scan. Input-level -ss seeks into short windows instead of
	// reading the entire audio stream. A stage budget guarantees fast fallback.
	s.updateTask(task, "coarse_analysis", 8, "")
	coarse := s.coarseAnalyze(media, task)
	if len(coarse) == 0 {
		s.logger.Debugf("media analysis sparse audio unavailable media=%s, using structural fallback", media.ID)
	}

	// V2: only the strongest coarse candidates get short scene refinement.
	s.updateTask(task, "refine_analysis", 48, "")
	refined := s.refineAnalyze(media, task, coarse)

	s.updateTask(task, "ranking", 69, "")
	highlights := s.rankSparseHighlights(media, refined)
	if len(highlights) == 0 {
		highlights = s.heuristicHighlights(media)
	}
	for i := range highlights {
		highlights[i].ID = uuid.NewString()
		highlights[i].MediaID = media.ID
		highlights[i].Source = "ffmpeg"
		highlights[i].Fingerprint = fingerprint
		highlights[i].Version = 2
	}
	s.updateTask(task, "ranking", 72, "")

	oldHighlights, _ := s.highlightRepo.ListByMediaID(media.ID)
	runDir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", media.ID, task.ID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		s.failTask(task, "thumbnail", err)
		return
	}

	// Only static thumbnails block task completion. Animated previews are lazily
	// generated on first hover, removing up to N extra FFmpeg jobs from the hot path.
	s.generateThumbnails(media, task, highlights, runDir)

	s.updateTask(task, "persist", 96, "")
	if err := s.highlightRepo.ReplaceByMediaID(media.ID, highlights); err != nil {
		_ = os.RemoveAll(runDir)
		s.failTask(task, "persist", err)
		return
	}
	for _, old := range oldHighlights {
		s.removeHighlightAssets(old)
	}

	completed := time.Now()
	task.Status = "completed"
	task.Stage = "completed"
	task.Progress = 100
	task.CompletedAt = &completed
	resultJSON, _ := json.Marshal(map[string]any{
		"highlight_count":  len(highlights),
		"analysis_method": "sparse_audio_scene_v2",
		"fingerprint":     fingerprint,
		"engine_version":  2,
	})
	task.Result = string(resultJSON)
	task.Error = ""
	_ = s.taskRepo.Update(task)
	s.broadcastTask(task)
	s.logger.Infof("local media highlights v2 completed media=%s count=%d", media.Title, len(highlights))
}

func (s *MediaAnalysisService) DeleteHighlights(mediaID string) error {
	highlights, _ := s.highlightRepo.ListByMediaID(mediaID)
	if err := s.highlightRepo.DeleteByMediaID(mediaID); err != nil {
		return err
	}
	for _, h := range highlights {
		s.removeHighlightAssets(h)
	}
	return os.RemoveAll(filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", mediaID))
}

// CleanupMedia is safe to call before deleting the media DB row.
func (s *MediaAnalysisService) CleanupMedia(mediaID string) {
	if err := s.DeleteHighlights(mediaID); err != nil && !errors.Is(err, os.ErrNotExist) {
		s.logger.Warnf("cleanup media analysis assets media=%s: %v", mediaID, err)
	}
}

func (s *MediaAnalysisService) HighlightAsset(mediaID, highlightID, kind string) (string, error) {
	h, err := s.highlightRepo.FindByID(highlightID)
	if err != nil || h.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	path := h.Thumbnail
	if kind == "preview" {
		path = h.PreviewPath
		if path == "" {
			path = h.GifPath
		}
	}
	if strings.TrimSpace(path) == "" {
		return "", os.ErrNotExist
	}
	if _, err := os.Stat(path); err != nil {
		return "", err
	}
	return path, nil
}

// EnsureHighlightPreview implements hover-lazy animated previews. It is safe for
// repeated HTTP requests: the first request creates and persists the preview,
// subsequent requests serve the cached file.
func (s *MediaAnalysisService) EnsureHighlightPreview(mediaID, highlightID string) (string, error) {
	h, err := s.highlightRepo.FindByID(highlightID)
	if err != nil || h.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := existingPreviewPath(h); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}

	// Duplicate hover requests for the same page should not spawn duplicate FFmpeg jobs.
	s.previewMu.Lock()
	defer s.previewMu.Unlock()

	h, err = s.highlightRepo.FindByID(highlightID)
	if err != nil || h.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := existingPreviewPath(h); path != "" {
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

	dir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", mediaID, "previews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	output := filepath.Join(dir, highlightID+"-preview.webp")
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	if err := s.generatePreview(ctx, media.FilePath, *h, output); err != nil {
		_ = os.Remove(output)
		return "", err
	}
	h.PreviewPath = output
	if err := s.highlightRepo.Update(h); err != nil {
		_ = os.Remove(output)
		return "", err
	}
	return output, nil
}

func existingPreviewPath(h *model.VideoHighlight) string {
	if h == nil {
		return ""
	}
	if strings.TrimSpace(h.PreviewPath) != "" {
		return h.PreviewPath
	}
	return strings.TrimSpace(h.GifPath)
}

func (s *MediaAnalysisService) ensureSupported(media *model.Media) error {
	if media == nil || strings.TrimSpace(media.FilePath) == "" || strings.TrimSpace(media.StreamURL) != "" || strings.EqualFold(filepath.Ext(media.FilePath), ".strm") {
		return ErrMediaAnalysisUnsupported
	}
	info, err := os.Stat(media.FilePath)
	if err != nil || info.IsDir() {
		if err != nil {
			return err
		}
		return ErrMediaAnalysisUnsupported
	}
	return nil
}

func (s *MediaAnalysisService) mediaFingerprint(media *model.Media) (string, error) {
	info, err := os.Stat(media.FilePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d:%d:%.3f", info.Size(), info.ModTime().UnixNano(), media.Duration), nil
}

func adaptiveSampleCount(duration float64) int {
	switch {
	case duration <= 0:
		return 0
	case duration < 20*60:
		return 12
	case duration < 60*60:
		return 18
	case duration < 120*60:
		return 24
	default:
		return 28
	}
}

func sparseSampleStarts(duration, window float64, count int) []float64 {
	if duration <= 0 || count <= 0 {
		return nil
	}
	window = math.Min(window, math.Max(1, duration))
	minCenter := math.Max(window/2, duration*0.05)
	maxCenter := math.Min(duration-window/2, duration*0.95)
	if maxCenter <= minCenter {
		return []float64{math.Max(0, (duration-window)/2)}
	}
	starts := make([]float64, 0, count)
	if count == 1 {
		return []float64{math.Max(0, (minCenter+maxCenter)/2-window/2)}
	}
	step := (maxCenter - minCenter) / float64(count-1)
	for i := 0; i < count; i++ {
		center := minCenter + float64(i)*step
		starts = append(starts, math.Max(0, math.Min(duration-window, center-window/2)))
	}
	return starts
}

func coarseStageBudget(duration float64) time.Duration {
	count := adaptiveSampleCount(duration)
	return time.Duration(16+count/2) * time.Second
}

func refineStageBudget(candidateCount int) time.Duration {
	return time.Duration(12+candidateCount) * time.Second
}

func (s *MediaAnalysisService) coarseAnalyze(media *model.Media, task *model.AIAnalysisTask) []sparseSample {
	count := adaptiveSampleCount(media.Duration)
	starts := sparseSampleStarts(media.Duration, coarseWindowSeconds, count)
	if len(starts) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), coarseStageBudget(media.Duration))
	defer cancel()
	jobs := make(chan float64)
	results := make(chan sampleResult, len(starts))
	workers := minInt(analysisWorkers, len(starts))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for start := range jobs {
				sample, err := s.sampleAudioWindow(ctx, media, start, coarseWindowSeconds)
				results <- sampleResult{Sample: sample, Err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, start := range starts {
			select {
			case jobs <- start:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	samples := make([]sparseSample, 0, len(starts))
	completed := 0
	for result := range results {
		completed++
		if result.Err == nil {
			samples = append(samples, result.Sample)
		} else if !errors.Is(result.Err, context.Canceled) && !errors.Is(result.Err, context.DeadlineExceeded) {
			s.logger.Debugf("sparse audio sample failed media=%s start=%.1f: %v", media.ID, result.Sample.Start, result.Err)
		}
		progress := 8 + 38*(float64(completed)/float64(len(starts)))
		s.updateTask(task, "coarse_analysis", progress, "")
	}
	if ctx.Err() != nil {
		s.logger.Debugf("sparse audio stage budget reached media=%s completed=%d/%d", media.ID, completed, len(starts))
	}
	normalizeAudioScores(samples)
	return samples
}

func (s *MediaAnalysisService) sampleAudioWindow(ctx context.Context, media *model.Media, start, duration float64) (sparseSample, error) {
	sample := sparseSample{Start: start, Center: start + duration/2, MeanDB: -120, MaxDB: -120, Method: "sparse_audio"}
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-nostats", "-loglevel", "info",
		"-ss", fmt.Sprintf("%.3f", start), "-i", media.FilePath,
		"-t", fmt.Sprintf("%.2f", duration), "-vn",
		"-af", "volumedetect", "-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return sample, ctx.Err()
		}
		return sample, fmt.Errorf("FFmpeg 音频窗口分析失败: %w", err)
	}
	mean, max, ok := parseVolumeDetect(string(output))
	if !ok {
		return sample, errors.New("FFmpeg 音量窗口数据不足")
	}
	sample.MeanDB = mean
	sample.MaxDB = max
	return sample, nil
}

func parseVolumeDetect(output string) (mean, max float64, ok bool) {
	mean, max = -120, -120
	meanOK, maxOK := false, false
	for _, line := range strings.Split(output, "\n") {
		lower := strings.ToLower(line)
		if idx := strings.Index(lower, "mean_volume:"); idx >= 0 {
			if v, parsed := parseDBValue(line[idx+len("mean_volume:"):]); parsed {
				mean, meanOK = v, true
			}
		}
		if idx := strings.Index(lower, "max_volume:"); idx >= 0 {
			if v, parsed := parseDBValue(line[idx+len("max_volume:"):]); parsed {
				max, maxOK = v, true
			}
		}
	}
	return mean, max, meanOK && maxOK
}

func parseDBValue(value string) (float64, bool) {
	field := strings.Fields(strings.TrimSpace(value))
	if len(field) == 0 || strings.EqualFold(field[0], "-inf") {
		return -120, false
	}
	v, err := strconv.ParseFloat(field[0], 64)
	return v, err == nil
}

func normalizeAudioScores(samples []sparseSample) {
	if len(samples) == 0 {
		return
	}
	minMean, maxMean := samples[0].MeanDB, samples[0].MeanDB
	minPeak, maxPeak := samples[0].MaxDB, samples[0].MaxDB
	for _, sample := range samples[1:] {
		minMean = math.Min(minMean, sample.MeanDB)
		maxMean = math.Max(maxMean, sample.MeanDB)
		minPeak = math.Min(minPeak, sample.MaxDB)
		maxPeak = math.Max(maxPeak, sample.MaxDB)
	}
	for i := range samples {
		meanScore := normalizeRange(samples[i].MeanDB, minMean, maxMean)
		peakScore := normalizeRange(samples[i].MaxDB, minPeak, maxPeak)
		samples[i].AudioScore = 5.5 + 4.5*(meanScore*0.72+peakScore*0.28)
		samples[i].Score = samples[i].AudioScore
	}
}

func normalizeRange(value, minValue, maxValue float64) float64 {
	if maxValue-minValue < 0.001 {
		return 0.5
	}
	return math.Max(0, math.Min(1, (value-minValue)/(maxValue-minValue)))
}

func (s *MediaAnalysisService) refineAnalyze(media *model.Media, task *model.AIAnalysisTask, coarse []sparseSample) []sparseSample {
	if len(coarse) == 0 {
		s.updateTask(task, "refine_analysis", 68, "")
		return nil
	}
	ordered := append([]sparseSample(nil), coarse...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].AudioScore > ordered[j].AudioScore })
	if len(ordered) > maxRefineCandidates {
		ordered = ordered[:maxRefineCandidates]
	}

	ctx, cancel := context.WithTimeout(context.Background(), refineStageBudget(len(ordered)))
	defer cancel()
	jobs := make(chan sparseSample)
	results := make(chan sampleResult, len(ordered))
	workers := minInt(analysisWorkers, len(ordered))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sample := range jobs {
				count, err := s.sampleSceneWindow(ctx, media, sample.Center, refineWindowSeconds)
				if err == nil {
					sample.SceneCount = count
					sceneScore := math.Min(10, 5+float64(count)*0.85)
					sample.Score = sample.AudioScore*0.75 + sceneScore*0.25
					sample.Method = "sparse_audio_scene"
				}
				results <- sampleResult{Sample: sample, Err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, sample := range ordered {
			select {
			case jobs <- sample:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	refinedByStart := make(map[string]sparseSample, len(ordered))
	completed := 0
	for result := range results {
		completed++
		refinedByStart[fmt.Sprintf("%.3f", result.Sample.Start)] = result.Sample
		if result.Err != nil && !errors.Is(result.Err, context.Canceled) && !errors.Is(result.Err, context.DeadlineExceeded) {
			s.logger.Debugf("sparse scene sample failed media=%s center=%.1f: %v", media.ID, result.Sample.Center, result.Err)
		}
		progress := 48 + 20*(float64(completed)/float64(len(ordered)))
		s.updateTask(task, "refine_analysis", progress, "")
	}
	if ctx.Err() != nil {
		s.logger.Debugf("sparse scene stage budget reached media=%s completed=%d/%d", media.ID, completed, len(ordered))
	}

	out := append([]sparseSample(nil), coarse...)
	for i := range out {
		if refined, ok := refinedByStart[fmt.Sprintf("%.3f", out[i].Start)]; ok {
			out[i] = refined
		}
	}
	return out
}

func (s *MediaAnalysisService) sampleSceneWindow(ctx context.Context, media *model.Media, center, duration float64) (int, error) {
	start := math.Max(0, center-duration/2)
	if start+duration > media.Duration {
		start = math.Max(0, media.Duration-duration)
	}
	filter := "scale=320:-2:flags=fast_bilinear,fps=1,select='gt(scene,0.30)',showinfo"
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-nostats",
		"-ss", fmt.Sprintf("%.3f", start), "-i", media.FilePath,
		"-t", fmt.Sprintf("%.2f", duration), "-vf", filter,
		"-an", "-vsync", "vfr", "-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("FFmpeg 场景窗口分析失败: %w", err)
	}
	count := 0
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, "pts_time:") {
			count++
		}
	}
	return count, nil
}

func (s *MediaAnalysisService) rankSparseHighlights(media *model.Media, samples []sparseSample) []model.VideoHighlight {
	if media.Duration <= 0 || len(samples) == 0 {
		return nil
	}
	ordered := append([]sparseSample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Score > ordered[j].Score })

	selected := make([]sparseSample, 0, maxHighlightCount)
	for _, candidate := range ordered {
		tooClose := false
		for _, existing := range selected {
			if math.Abs(existing.Center-candidate.Center) < 45 {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= maxHighlightCount {
			break
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Center < selected[j].Center })

	result := make([]model.VideoHighlight, 0, len(selected))
	for i, candidate := range selected {
		start := math.Max(0, candidate.Center-10)
		end := math.Min(media.Duration, start+30)
		if end-start < 15 {
			start = math.Max(0, end-30)
		}
		method := candidate.Method
		if method == "" {
			method = "sparse_audio"
		}
		result = append(result, model.VideoHighlight{
			Title:          highlightTitle(candidate.Center, media.Duration, candidate.Score, i),
			StartTime:      start,
			EndTime:        end,
			Score:          math.Round(candidate.Score*10) / 10,
			Tags:           media.Genres,
			AnalysisMethod: method,
		})
	}
	return result
}

func highlightTitle(position, duration, score float64, index int) string {
	if duration <= 0 {
		return fmt.Sprintf("精彩片段 %d", index+1)
	}
	ratio := position / duration
	switch {
	case ratio < 0.12:
		return "开场高能"
	case ratio > 0.85:
		return "结局高潮"
	case ratio > 0.62:
		return "后期转折"
	case score >= 9:
		return "高潮片段"
	case score >= 8:
		return "精彩时刻"
	case ratio < 0.32:
		return "前期精彩"
	default:
		return fmt.Sprintf("精彩片段 %d", index+1)
	}
}

func (s *MediaAnalysisService) heuristicHighlights(media *model.Media) []model.VideoHighlight {
	if media.Duration <= 0 {
		return nil
	}
	points := []struct {
		ratio float64
		title string
		score float64
	}{
		{0.25, "第一幕转折", 7.0},
		{0.50, "中点高潮", 8.0},
		{0.75, "第二幕转折", 8.5},
	}
	out := make([]model.VideoHighlight, 0, len(points))
	for _, point := range points {
		start := math.Max(0, media.Duration*point.ratio-15)
		out = append(out, model.VideoHighlight{
			Title:          point.title,
			StartTime:      start,
			EndTime:        math.Min(media.Duration, start+30),
			Score:          point.score,
			Tags:           media.Genres,
			AnalysisMethod: "heuristic",
		})
	}
	return out
}

func (s *MediaAnalysisService) generateThumbnails(media *model.Media, task *model.AIAnalysisTask, highlights []model.VideoHighlight, runDir string) {
	if len(highlights) == 0 {
		s.updateTask(task, "thumbnail", 94, "")
		return
	}
	type thumbnailJob struct{ Index int }
	type thumbnailResult struct {
		Index int
		Path  string
		Err   error
	}
	jobs := make(chan thumbnailJob)
	results := make(chan thumbnailResult, len(highlights))
	workers := minInt(thumbnailWorkers, len(highlights))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				path := filepath.Join(runDir, highlights[job.Index].ID+".webp")
				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				err := s.generateThumbnail(ctx, media.FilePath, highlights[job.Index], path)
				cancel()
				results <- thumbnailResult{Index: job.Index, Path: path, Err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range highlights {
			jobs <- thumbnailJob{Index: i}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	completed := 0
	for result := range results {
		completed++
		if result.Err == nil {
			highlights[result.Index].Thumbnail = result.Path
		} else {
			s.logger.Debugf("generate highlight thumbnail media=%s: %v", media.ID, result.Err)
			_ = os.Remove(result.Path)
		}
		progress := 72 + 22*(float64(completed)/float64(len(highlights)))
		s.updateTask(task, "thumbnail", progress, "")
	}
}

func (s *MediaAnalysisService) generateThumbnail(ctx context.Context, filePath string, highlight model.VideoHighlight, output string) error {
	middle := (highlight.StartTime + highlight.EndTime) / 2
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", middle), "-i", filePath,
		"-frames:v", "1", "-vf", "scale=640:-2:flags=fast_bilinear",
		"-c:v", "libwebp", "-quality", "76", "-y", output,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("thumbnail ffmpeg: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}

func (s *MediaAnalysisService) generatePreview(ctx context.Context, filePath string, highlight model.VideoHighlight, output string) error {
	duration := math.Min(2.5, math.Max(1, highlight.EndTime-highlight.StartTime))
	start := highlight.StartTime + math.Max(0, (highlight.EndTime-highlight.StartTime-duration)/2)
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", start), "-i", filePath,
		"-t", fmt.Sprintf("%.2f", duration), "-an",
		"-vf", "fps=5,scale=420:-2:flags=fast_bilinear",
		"-loop", "0", "-c:v", "libwebp", "-quality", "54", "-y", output,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("preview ffmpeg: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}

func (s *MediaAnalysisService) removeHighlightAssets(h model.VideoHighlight) {
	for _, path := range []string{h.Thumbnail, h.PreviewPath, h.GifPath} {
		path = strings.TrimSpace(path)
		if path != "" {
			_ = os.Remove(path)
		}
	}
	if h.Thumbnail != "" {
		_ = os.Remove(filepath.Dir(h.Thumbnail))
	}
}

func (s *MediaAnalysisService) updateTask(task *model.AIAnalysisTask, stage string, progress float64, errText string) {
	task.Stage = stage
	task.Progress = progress
	if errText != "" {
		task.Error = errText
	}
	_ = s.taskRepo.Update(task)
	s.broadcastTask(task)
}

func (s *MediaAnalysisService) failTask(task *model.AIAnalysisTask, stage string, err error) {
	now := time.Now()
	task.Status = "failed"
	task.Stage = stage
	task.Error = err.Error()
	task.CompletedAt = &now
	_ = s.taskRepo.Update(task)
	s.broadcastTask(task)
}

func (s *MediaAnalysisService) broadcastTask(task *model.AIAnalysisTask) {
	if s.wsHub == nil || task == nil {
		return
	}
	event := "media_analysis_progress"
	if task.Status == "completed" || task.Status == "failed" || task.Status == "interrupted" {
		event = "media_analysis_complete"
	}
	s.wsHub.BroadcastEvent(event, map[string]any{
		"task_id": task.ID,
		"media_id": task.MediaID,
		"status": task.Status,
		"stage": task.Stage,
		"progress": task.Progress,
		"error": task.Error,
	})
}