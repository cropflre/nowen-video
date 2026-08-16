package service

import (
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
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

const mediaHighlightTaskType = "media_highlight"

var (
	ErrMediaAnalysisInProgress = errors.New("media analysis already in progress")
	ErrMediaAnalysisUnsupported = errors.New("media source does not support local analysis")
)

// MediaAnalysisService provides local FFmpeg-only media understanding.
// It deliberately has no AIService dependency: highlight generation must work
// offline, without an API key and without consuming tokens.
type MediaAnalysisService struct {
	cfg           *config.Config
	mediaRepo     *repository.MediaRepo
	highlightRepo *repository.VideoHighlightRepo
	taskRepo      *repository.AIAnalysisTaskRepo // legacy table/model reused as durable task storage
	logger        *zap.SugaredLogger
	wsHub         *WSHub
	semaphore     chan struct{}
}

type MediaHighlightList struct {
	Highlights []model.VideoHighlight `json:"highlights"`
	Stale      bool                   `json:"stale"`
	Fingerprint string                `json:"fingerprint,omitempty"`
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
		semaphore:     make(chan struct{}, 1), // NAS-safe default: only one FFmpeg analysis at a time.
	}
	// A process restart cannot safely resume an FFmpeg subprocess. Make stale
	// running rows explicit instead of leaving the UI stuck forever.
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

	s.updateTask(task, "audio_analysis", 10, "")
	segments, audioErr := s.analyzeAudio(media)
	if audioErr != nil {
		s.logger.Debugf("media analysis audio fallback media=%s err=%v", media.ID, audioErr)
	}
	s.updateTask(task, "audio_analysis", 40, "")

	s.updateTask(task, "scene_analysis", 42, "")
	scenes, sceneErr := s.detectSceneChanges(media)
	if sceneErr != nil {
		s.logger.Debugf("media analysis scene fallback media=%s err=%v", media.ID, sceneErr)
	}
	s.updateTask(task, "scene_analysis", 60, "")

	s.updateTask(task, "ranking", 62, "")
	highlights := s.rankHighlights(media, segments, scenes)
	if len(highlights) == 0 {
		highlights = s.heuristicHighlights(media)
	}
	for i := range highlights {
		highlights[i].ID = uuid.NewString()
		highlights[i].MediaID = media.ID
		highlights[i].Source = "ffmpeg"
		highlights[i].Fingerprint = fingerprint
		highlights[i].Version = 1
	}
	s.updateTask(task, "ranking", 70, "")

	oldHighlights, _ := s.highlightRepo.ListByMediaID(media.ID)
	runDir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", media.ID, task.ID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		s.failTask(task, "thumbnail", err)
		return
	}

	for i := range highlights {
		progressBase := 70.0 + (float64(i)/math.Max(1, float64(len(highlights))))*25.0
		s.updateTask(task, "thumbnail", progressBase, "")
		thumb := filepath.Join(runDir, highlights[i].ID+".webp")
		if err := s.generateThumbnail(media.FilePath, highlights[i], thumb); err == nil {
			highlights[i].Thumbnail = thumb
		} else {
			s.logger.Debugf("generate highlight thumbnail media=%s: %v", media.ID, err)
		}

		s.updateTask(task, "preview", progressBase+2, "")
		preview := filepath.Join(runDir, highlights[i].ID+"-preview.webp")
		if err := s.generatePreview(media.FilePath, highlights[i], preview); err == nil {
			highlights[i].PreviewPath = preview
		} else {
			s.logger.Debugf("generate highlight preview media=%s: %v", media.ID, err)
		}
	}

	s.updateTask(task, "persist", 96, "")
	if err := s.highlightRepo.ReplaceByMediaID(media.ID, highlights); err != nil {
		_ = os.RemoveAll(runDir)
		s.failTask(task, "persist", err)
		return
	}
	// Only after DB replacement succeeds do we remove previous preview assets.
	for _, old := range oldHighlights {
		s.removeHighlightAssets(old)
	}

	completed := time.Now()
	task.Status = "completed"
	task.Stage = "completed"
	task.Progress = 100
	task.CompletedAt = &completed
	resultJSON, _ := json.Marshal(map[string]any{
		"highlight_count": len(highlights),
		"analysis_method": "audio_scene",
		"fingerprint": fingerprint,
	})
	task.Result = string(resultJSON)
	task.Error = ""
	_ = s.taskRepo.Update(task)
	s.broadcastTask(task)
	s.logger.Infof("local media highlights completed media=%s count=%d", media.Title, len(highlights))
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
	_ = s.taskRepoDeleteByMedia(mediaID)
}

func (s *MediaAnalysisService) taskRepoDeleteByMedia(mediaID string) error {
	// Keep repository API focused; this table is legacy and cleanup is best effort.
	return nil
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

type analysisAudioSegment struct {
	Start float64
	End   float64
	RMS   float64
}

func (s *MediaAnalysisService) analyzeAudio(media *model.Media) ([]analysisAudioSegment, error) {
	if media.Duration <= 0 {
		return nil, errors.New("视频时长未知")
	}
	segmentDuration := 10.0
	if media.Duration < 60 {
		segmentDuration = 5
	}
	samples := int(segmentDuration * 48000)
	filter := fmt.Sprintf("aresample=48000,asetnsamples=n=%d:p=0,astats=metadata=1:reset=1,ametadata=print:key=lavfi.astats.Overall.RMS_level:file=-", samples)
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-hide_banner", "-nostats", "-i", media.FilePath,
		"-af", filter,
		"-vn", "-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("FFmpeg 音频分析失败: %w", err)
	}
	segments := make([]analysisAudioSegment, 0, int(media.Duration/segmentDuration)+1)
	current := 0.0
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(strings.ToLower(line), "rms_level=") {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) < 2 {
			continue
		}
		value := strings.TrimSpace(parts[len(parts)-1])
		if strings.EqualFold(value, "-inf") {
			current += segmentDuration
			continue
		}
		rms, parseErr := strconv.ParseFloat(value, 64)
		if parseErr != nil {
			continue
		}
		end := math.Min(media.Duration, current+segmentDuration)
		segments = append(segments, analysisAudioSegment{Start: current, End: end, RMS: rms})
		current += segmentDuration
		if current >= media.Duration {
			break
		}
	}
	if len(segments) < 3 {
		return nil, errors.New("FFmpeg 音频能量样本不足")
	}
	return segments, nil
}

func (s *MediaAnalysisService) detectSceneChanges(media *model.Media) ([]float64, error) {
	if media.Duration <= 0 {
		return nil, errors.New("视频时长未知")
	}
	// Analysis is intentionally low-resolution / low-FPS so 4K HEVC media does
	// not require full-resolution frame analysis on a NAS.
	filter := "scale=480:-2:flags=fast_bilinear,fps=2,select='gt(scene,0.30)',showinfo"
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-hide_banner", "-nostats", "-i", media.FilePath,
		"-vf", filter,
		"-an", "-vsync", "vfr", "-f", "null", "-",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("FFmpeg 场景检测失败: %w", err)
	}
	scenes := make([]float64, 0, 128)
	for _, line := range strings.Split(string(output), "\n") {
		idx := strings.Index(line, "pts_time:")
		if idx < 0 {
			continue
		}
		value := strings.Fields(line[idx+len("pts_time:"):])
		if len(value) == 0 {
			continue
		}
		if t, parseErr := strconv.ParseFloat(value[0], 64); parseErr == nil {
			scenes = append(scenes, t)
		}
	}
	return scenes, nil
}

type highlightCandidate struct {
	Start  float64
	Score  float64
	Method string
}

func (s *MediaAnalysisService) rankHighlights(media *model.Media, audio []analysisAudioSegment, scenes []float64) []model.VideoHighlight {
	duration := media.Duration
	if duration <= 0 {
		return nil
	}
	candidates := make([]highlightCandidate, 0, len(audio)+len(scenes))

	if len(audio) >= 3 {
		var sum, sum2 float64
		valid := 0
		for _, seg := range audio {
			if seg.RMS <= -100 {
				continue
			}
			sum += seg.RMS
			sum2 += seg.RMS * seg.RMS
			valid++
		}
		if valid > 0 {
			mean := sum / float64(valid)
			variance := sum2/float64(valid) - mean*mean
			std := 0.0
			if variance > 0 {
				std = math.Sqrt(variance)
			}
			threshold := mean + 0.45*std
			for _, seg := range audio {
				if seg.RMS <= threshold || seg.RMS <= -100 {
					continue
				}
				audioScore := 6.0
				if std > 0 {
					audioScore = math.Min(10, 6+4*((seg.RMS-mean)/(3*std)))
				}
				sceneCount := 0
				center := (seg.Start + seg.End) / 2
				for _, t := range scenes {
					if math.Abs(t-center) <= 30 {
						sceneCount++
					}
				}
				sceneScore := math.Min(10, 5+float64(sceneCount)*0.65)
				finalScore := audioScore*0.7 + sceneScore*0.3
				candidates = append(candidates, highlightCandidate{Start: seg.Start, Score: finalScore, Method: "audio_scene"})
			}
		}
	}

	if len(candidates) < 3 && len(scenes) > 0 {
		for _, center := range scenes {
			count := 0
			for _, other := range scenes {
				if math.Abs(other-center) <= 30 {
					count++
				}
			}
			if count >= 2 {
				candidates = append(candidates, highlightCandidate{Start: center, Score: math.Min(9.5, 6+float64(count)*0.45), Method: "scene"})
			}
		}
	}

	// Avoid generic opening/credits regions and rank strongest candidates first.
	minTime := duration * 0.05
	maxTime := duration * 0.95
	filtered := candidates[:0]
	for _, c := range candidates {
		if duration >= 300 && (c.Start < minTime || c.Start > maxTime) {
			continue
		}
		filtered = append(filtered, c)
	}
	candidates = filtered
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })

	selected := make([]highlightCandidate, 0, 8)
	for _, c := range candidates {
		tooClose := false
		for _, existing := range selected {
			if math.Abs(existing.Start-c.Start) < 45 {
				tooClose = true
				break
			}
		}
		if tooClose {
			continue
		}
		selected = append(selected, c)
		if len(selected) >= 8 {
			break
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Start < selected[j].Start })

	result := make([]model.VideoHighlight, 0, len(selected))
	for i, c := range selected {
		start := math.Max(0, c.Start-5)
		end := math.Min(duration, start+30)
		title := highlightTitle(c.Start, duration, c.Score, i)
		result = append(result, model.VideoHighlight{
			Title:          title,
			StartTime:      start,
			EndTime:        end,
			Score:          math.Round(c.Score*10) / 10,
			Tags:           media.Genres,
			AnalysisMethod: c.Method,
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
	for _, p := range points {
		start := math.Max(0, media.Duration*p.ratio-15)
		out = append(out, model.VideoHighlight{
			Title:          p.title,
			StartTime:      start,
			EndTime:        math.Min(media.Duration, start+30),
			Score:          p.score,
			Tags:           media.Genres,
			AnalysisMethod: "heuristic",
		})
	}
	return out
}

func (s *MediaAnalysisService) generateThumbnail(filePath string, highlight model.VideoHighlight, output string) error {
	middle := (highlight.StartTime + highlight.EndTime) / 2
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", middle), "-i", filePath,
		"-frames:v", "1", "-vf", "scale=640:-2:flags=lanczos",
		"-c:v", "libwebp", "-quality", "78", "-y", output,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("thumbnail ffmpeg: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}

func (s *MediaAnalysisService) generatePreview(filePath string, highlight model.VideoHighlight, output string) error {
	duration := math.Min(3, math.Max(1, highlight.EndTime-highlight.StartTime))
	start := highlight.StartTime + math.Max(0, (highlight.EndTime-highlight.StartTime-duration)/2)
	cmd := exec.Command(s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", start), "-i", filePath,
		"-t", fmt.Sprintf("%.2f", duration), "-an",
		"-vf", "fps=6,scale=480:-2:flags=lanczos",
		"-loop", "0", "-c:v", "libwebp", "-quality", "58", "-y", output,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
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
