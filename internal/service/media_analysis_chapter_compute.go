package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
)

const (
	MediaComputeJobChapterDetectV1         = "chapter_detect_v1"
	MediaComputeCapabilityChapterDetectV1 = "chapter_detect_v1"

	mediaChapterTaskType        = "chapter_gen"
	chapterProbeGapSeconds      = 3.0
	chapterCaptureWidth         = 240
	chapterEngineVersion        = 1
	chapterServerProbeWindow    = 8.0
	chapterMaxSupportedSamples  = 72
	chapterMaxSupportedChapters = 12
)

// MediaComputeChapterDetectInput 是 chapter_detect_v1 的私有输入。
// 客户端只负责围绕候选中心点做前后稀疏抽帧并给出 scene-change score，
// Server 负责阈值、最小间距、最终章节区间与持久化。
type MediaComputeChapterDetectInput struct {
	MediaID           string    `json:"media_id"`
	Fingerprint       string    `json:"fingerprint"`
	Duration          float64   `json:"duration"`
	StreamURL         string    `json:"stream_url"`
	SampleTimes       []float64 `json:"sample_times"`
	ProbeGapSeconds   float64   `json:"probe_gap_seconds"`
	MinChapterSeconds float64   `json:"min_chapter_seconds"`
	MaxChapters       int       `json:"max_chapters"`
	CaptureWidth      int       `json:"capture_width"`
	EngineVersion     int       `json:"engine_version"`
}

type MediaComputeChapterCandidate struct {
	Time  float64 `json:"time"`
	Score float64 `json:"score"`
}

type MediaComputeChapterDetectResult struct {
	Fingerprint string                         `json:"fingerprint"`
	Candidates  []MediaComputeChapterCandidate `json:"candidates"`
}

type chapterRepositoryState struct {
	repo *repository.VideoChapterRepo
}

var mediaChapterRepositories sync.Map

// AttachMediaChapterRepository 给通用 MediaAnalysisService 挂载章节仓储，避免继续扩张其构造函数。
// 同时把服务重启前未完成的 chapter_gen 标记为 interrupted。
func AttachMediaChapterRepository(s *MediaAnalysisService, repo *repository.VideoChapterRepo) {
	if s == nil || repo == nil {
		return
	}
	mediaChapterRepositories.Store(s, chapterRepositoryState{repo: repo})
	if s.taskRepo != nil {
		if err := s.taskRepo.MarkRunningInterrupted(mediaChapterTaskType); err != nil && s.logger != nil {
			s.logger.Warnf("mark interrupted chapter tasks: %v", err)
		}
	}
}

func mediaChapterRepository(s *MediaAnalysisService) *repository.VideoChapterRepo {
	if s == nil {
		return nil
	}
	value, ok := mediaChapterRepositories.Load(s)
	if !ok {
		return nil
	}
	return value.(chapterRepositoryState).repo
}

func (s *MediaAnalysisService) ListChapters(mediaID string) ([]model.VideoChapter, error) {
	if _, err := s.mediaRepo.FindByID(mediaID); err != nil {
		return nil, ErrMediaNotFound
	}
	repo := mediaChapterRepository(s)
	if repo == nil {
		return nil, errors.New("chapter repository unavailable")
	}
	return repo.ListByMediaID(mediaID)
}

func (s *MediaAnalysisService) ListAnalysisTasks(mediaID string) ([]model.AIAnalysisTask, error) {
	if s.taskRepo == nil {
		return nil, errors.New("analysis task repository unavailable")
	}
	return s.taskRepo.ListByMediaID(mediaID)
}

func (s *MediaAnalysisService) AnalysisTask(taskID string) (*model.AIAnalysisTask, error) {
	if s.taskRepo == nil {
		return nil, errors.New("analysis task repository unavailable")
	}
	return s.taskRepo.FindByID(taskID)
}

func (s *MediaAnalysisService) CleanupChapters(mediaID string) {
	if repo := mediaChapterRepository(s); repo != nil {
		if err := repo.DeleteByMediaID(mediaID); err != nil && s.logger != nil {
			s.logger.Warnf("cleanup media chapters media=%s: %v", mediaID, err)
		}
	}
}

func chapterSampleCount(duration float64) int {
	switch {
	case duration <= 0:
		return 0
	case duration < 10*60:
		return 12
	case duration < 30*60:
		return 24
	case duration < 60*60:
		return 36
	case duration < 120*60:
		return 48
	default:
		return 60
	}
}

func chapterSampleTimes(duration float64) []float64 {
	count := minInt(chapterSampleCount(duration), chapterMaxSupportedSamples)
	if count <= 0 || duration <= 0 {
		return nil
	}
	margin := math.Min(30, duration*0.05)
	start := margin
	end := math.Max(start, duration-margin)
	if end-start < 1 {
		return []float64{duration / 2}
	}
	step := (end - start) / float64(count+1)
	out := make([]float64, 0, count)
	for i := 1; i <= count; i++ {
		out = append(out, start+float64(i)*step)
	}
	return out
}

func chapterMaxChapters(duration float64) int {
	if duration < 120 {
		return 1
	}
	count := int(math.Round(duration/300.0)) + 1
	return clampInt(count, 2, chapterMaxSupportedChapters)
}

func chapterMinSpacing(duration float64, maxChapters int) float64 {
	if maxChapters <= 1 || duration <= 0 {
		return 60
	}
	return math.Max(60, math.Min(180, duration/float64(maxChapters)*0.55))
}

func newChapterDetectInput(media *model.Media, fingerprint string) MediaComputeChapterDetectInput {
	maxChapters := chapterMaxChapters(media.Duration)
	return MediaComputeChapterDetectInput{
		MediaID:           media.ID,
		Fingerprint:       fingerprint,
		Duration:          media.Duration,
		StreamURL:         "/api/stream/" + media.ID + "/direct",
		SampleTimes:       chapterSampleTimes(media.Duration),
		ProbeGapSeconds:   chapterProbeGapSeconds,
		MinChapterSeconds: chapterMinSpacing(media.Duration, maxChapters),
		MaxChapters:       maxChapters,
		CaptureWidth:      chapterCaptureWidth,
		EngineVersion:     chapterEngineVersion,
	}
}

// AnalyzeChaptersDistributed 是正式后端的章节生成入口。
// auto: Desktop -> Android -> Server sparse fallback；client_preferred: 只等客户端；server_only: 直接 Server；off: 禁止新任务。
func (s *MediaAnalysisService) AnalyzeChaptersDistributed(mediaID string) (*model.AIAnalysisTask, error) {
	if s.ExecutionMode() == MediaAnalysisModeOff {
		return nil, ErrMediaAnalysisDisabled
	}
	if mediaChapterRepository(s) == nil {
		return nil, errors.New("chapter repository unavailable")
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	if err := s.ensureSupported(media); err != nil {
		return nil, err
	}
	if existing, err := s.taskRepo.FindActiveByMediaAndType(mediaID, mediaChapterTaskType); err == nil {
		return existing, ErrMediaAnalysisInProgress
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	task := &model.AIAnalysisTask{
		MediaID: mediaID, TaskType: mediaChapterTaskType,
		Status: "pending", Stage: "queued", Progress: 0,
	}
	if err := s.taskRepo.Create(task); err != nil {
		return nil, err
	}
	go s.dispatchChapterDetectTask(task.ID, mediaID)
	return task, nil
}

func (s *MediaAnalysisService) dispatchChapterDetectTask(taskID, mediaID string) {
	mode := s.ExecutionMode()
	if mode == MediaAnalysisModeOff {
		if task, err := s.taskRepo.FindByID(taskID); err == nil {
			s.failTask(task, "disabled", ErrMediaAnalysisDisabled)
		}
		return
	}
	if mode == MediaAnalysisModeServerOnly {
		s.runChapterDetectServerTask(taskID, mediaID)
		return
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", ErrMediaNotFound)
		}
		return
	}
	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", err)
		}
		return
	}
	chapterInput := newChapterDetectInput(media, fingerprint)
	if len(chapterInput.SampleTimes) == 0 {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", ErrMediaAnalysisWorkerResult)
		}
		return
	}
	inputJSON, err := json.Marshal(chapterInput)
	if err != nil {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", err)
		}
		return
	}
	createdAt := time.Now()
	if err := s.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: taskID, MediaID: mediaID, Fingerprint: fingerprint,
		JobType: MediaComputeJobChapterDetectV1,
		RequiredCapability: MediaComputeCapabilityChapterDetectV1,
		Input: inputJSON, CreatedAt: createdAt,
	}); err != nil {
		if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
			s.failTask(task, "dispatch", err)
		}
		return
	}
	if task, taskErr := s.taskRepo.FindByID(taskID); taskErr == nil {
		s.updateTask(task, "waiting_client", 2, "")
	}

	ticker := time.NewTicker(remoteDispatchPoll)
	defer ticker.Stop()
	for range ticker.C {
		task, taskErr := s.taskRepo.FindByID(taskID)
		if taskErr != nil {
			s.UnregisterComputeTask(taskID)
			return
		}
		if task.Status == "completed" || task.Status == "failed" || task.Status == "interrupted" {
			s.UnregisterComputeTask(taskID)
			return
		}

		currentMode := s.ExecutionMode()
		if currentMode == MediaAnalysisModeOff {
			s.UnregisterComputeTask(taskID)
			s.failTask(task, "disabled", ErrMediaAnalysisDisabled)
			return
		}
		if currentMode == MediaAnalysisModeServerOnly {
			s.UnregisterComputeTask(taskID)
			s.runChapterDetectServerTask(taskID, mediaID)
			return
		}

		now := time.Now()
		state := mediaAnalysisState(s)
		state.mu.Lock()
		remote := state.remoteTasks[taskID]
		if remote == nil {
			state.mu.Unlock()
			return
		}
		claimed := remote.ClaimedBy != "" && remote.LeaseUntil.After(now)
		expiredClaim := remote.ClaimedBy != "" && !remote.LeaseUntil.After(now)
		if expiredClaim {
			if worker, ok := state.workers[remote.ClaimedBy]; ok {
				worker.State, worker.TaskID = "idle", ""
				state.workers[remote.ClaimedBy] = worker
			}
			remote.ClaimedBy, remote.ClaimToken, remote.WorkerKind = "", "", ""
			remote.LeaseUntil = time.Time{}
		}
		state.mu.Unlock()

		if claimed {
			continue
		}
		if currentMode == MediaAnalysisModeClientPreferred {
			if expiredClaim {
				s.updateTask(task, "waiting_client", maxFloat(2, task.Progress), "客户端章节检测租约已过期，等待其他客户端")
			}
			continue
		}
		if now.Sub(createdAt) >= remoteWorkerGrace || expiredClaim {
			s.UnregisterComputeTask(taskID)
			if s.logger != nil {
				s.logger.Infof("chapter compute client unavailable, fallback to server media=%s task=%s", mediaID, taskID)
			}
			s.runChapterDetectServerTask(taskID, mediaID)
			return
		}
	}
}

type chapterProbeResult struct {
	Candidate MediaComputeChapterCandidate
	Err       error
}

func chapterServerBudget(sampleCount int) time.Duration {
	seconds := 20 + sampleCount*2
	if seconds > 100 {
		seconds = 100
	}
	return time.Duration(seconds) * time.Second
}

func (s *MediaAnalysisService) serverChapterCandidates(media *model.Media, task *model.AIAnalysisTask, input MediaComputeChapterDetectInput) []MediaComputeChapterCandidate {
	if len(input.SampleTimes) == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), chapterServerBudget(len(input.SampleTimes)))
	defer cancel()

	jobs := make(chan float64)
	results := make(chan chapterProbeResult, len(input.SampleTimes))
	workers := minInt(analysisWorkers, len(input.SampleTimes))
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for center := range jobs {
				count, err := s.sampleSceneWindow(ctx, media, center, chapterServerProbeWindow)
				score := 0.0
				if err == nil && count > 0 {
					score = math.Min(1, 0.28+float64(count)*0.18)
				}
				results <- chapterProbeResult{Candidate: MediaComputeChapterCandidate{Time: center, Score: score}, Err: err}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, center := range input.SampleTimes {
			select {
			case jobs <- center:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { wg.Wait(); close(results) }()

	candidates := make([]MediaComputeChapterCandidate, 0, len(input.SampleTimes))
	completed := 0
	for result := range results {
		completed++
		candidates = append(candidates, result.Candidate)
		if result.Err != nil && !errors.Is(result.Err, context.Canceled) && !errors.Is(result.Err, context.DeadlineExceeded) && s.logger != nil {
			s.logger.Debugf("chapter sparse scene probe failed media=%s center=%.1f: %v", media.ID, result.Candidate.Time, result.Err)
		}
		progress := 8 + 70*(float64(completed)/float64(len(input.SampleTimes)))
		s.updateTask(task, "server_scene_probe", progress, "")
	}
	return candidates
}

func validChapterCandidate(candidate MediaComputeChapterCandidate, duration float64) bool {
	return !math.IsNaN(candidate.Time) && !math.IsInf(candidate.Time, 0) &&
		!math.IsNaN(candidate.Score) && !math.IsInf(candidate.Score, 0) &&
		candidate.Time > 0 && candidate.Time < duration && candidate.Score >= 0 && candidate.Score <= 1
}

func selectChapterPoints(candidates []MediaComputeChapterCandidate, duration, minSpacing float64, maxChapters int) []MediaComputeChapterCandidate {
	if duration <= 0 || maxChapters <= 1 {
		return nil
	}
	clean := make([]MediaComputeChapterCandidate, 0, len(candidates))
	edge := math.Min(math.Max(30, minSpacing*0.4), duration*0.2)
	for _, candidate := range candidates {
		if !validChapterCandidate(candidate, duration) || candidate.Time < edge || candidate.Time > duration-edge {
			continue
		}
		clean = append(clean, candidate)
	}
	if len(clean) == 0 {
		return nil
	}
	ranked := append([]MediaComputeChapterCandidate(nil), clean...)
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].Score > ranked[j].Score })
	thresholdIndex := int(math.Floor(float64(len(ranked)) * 0.35))
	if thresholdIndex >= len(ranked) {
		thresholdIndex = len(ranked) - 1
	}
	threshold := math.Max(0.08, ranked[thresholdIndex].Score)
	selected := make([]MediaComputeChapterCandidate, 0, maxChapters-1)
	for _, candidate := range ranked {
		if candidate.Score < threshold {
			continue
		}
		farEnough := true
		for _, existing := range selected {
			if math.Abs(existing.Time-candidate.Time) < minSpacing {
				farEnough = false
				break
			}
		}
		if !farEnough {
			continue
		}
		selected = append(selected, candidate)
		if len(selected) >= maxChapters-1 {
			break
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Time < selected[j].Time })
	return selected
}

func uniformChapterPoints(duration float64, maxChapters int) []MediaComputeChapterCandidate {
	if duration < 120 || maxChapters <= 1 {
		return nil
	}
	chapterCount := clampInt(int(math.Round(duration/300.0)), 2, maxChapters)
	step := duration / float64(chapterCount)
	out := make([]MediaComputeChapterCandidate, 0, chapterCount-1)
	for i := 1; i < chapterCount; i++ {
		out = append(out, MediaComputeChapterCandidate{Time: float64(i) * step, Score: 0.35})
	}
	return out
}

func buildVideoChapters(mediaID string, duration float64, points []MediaComputeChapterCandidate) []model.VideoChapter {
	ordered := append([]MediaComputeChapterCandidate(nil), points...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Time < ordered[j].Time })
	starts := make([]float64, 0, len(ordered)+1)
	starts = append(starts, 0)
	for _, point := range ordered {
		if point.Time > 0 && point.Time < duration {
			starts = append(starts, point.Time)
		}
	}
	chapters := make([]model.VideoChapter, 0, len(starts))
	for i, start := range starts {
		end := duration
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		confidence := 0.55
		sceneType := "opening"
		if i > 0 {
			confidence = math.Min(0.95, 0.55+ordered[i-1].Score*0.4)
			sceneType = "transition"
		}
		chapters = append(chapters, model.VideoChapter{
			ID: uuid.NewString(), MediaID: mediaID,
			Title: fmt.Sprintf("第 %d 章", i+1),
			StartTime: start, EndTime: end,
			SceneType: sceneType, Confidence: confidence, Source: "analysis",
		})
	}
	return chapters
}

func (s *MediaAnalysisService) persistChapterResult(task *model.AIAnalysisTask, media *model.Media, fingerprint, method, workerKind, workerID string, candidates []MediaComputeChapterCandidate, input MediaComputeChapterDetectInput) error {
	repo := mediaChapterRepository(s)
	if repo == nil {
		return errors.New("chapter repository unavailable")
	}
	points := selectChapterPoints(candidates, media.Duration, input.MinChapterSeconds, input.MaxChapters)
	if len(points) == 0 {
		points = uniformChapterPoints(media.Duration, input.MaxChapters)
		if method != "" {
			method += "+uniform_fallback"
		} else {
			method = "uniform_fallback"
		}
	}
	chapters := buildVideoChapters(media.ID, media.Duration, points)
	if err := repo.ReplaceByMediaID(media.ID, chapters); err != nil {
		return err
	}

	completed := time.Now()
	task.Status, task.Stage, task.Progress, task.Error = "completed", "completed", 100, ""
	task.CompletedAt = &completed
	resultJSON, _ := json.Marshal(map[string]any{
		"chapter_count": len(chapters), "analysis_method": method,
		"worker_kind": workerKind, "worker_id": workerID,
		"fingerprint": fingerprint, "engine_version": chapterEngineVersion,
	})
	task.Result = string(resultJSON)
	if err := s.taskRepo.Update(task); err != nil {
		return err
	}
	s.broadcastTask(task)
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent("ai_analysis_complete", map[string]any{
			"task_id": task.ID, "media_id": media.ID, "type": mediaChapterTaskType, "count": len(chapters),
		})
	}
	return nil
}

func (s *MediaAnalysisService) runChapterDetectServerTask(taskID, mediaID string) {
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
	if task.StartedAt == nil {
		task.StartedAt = &now
	}
	task.Status = "running"
	s.updateTask(task, "server_scene_probe", math.Max(5, task.Progress), "")
	if err := s.ensureSupported(media); err != nil {
		s.failTask(task, "probe", err)
		return
	}
	fingerprint, err := s.mediaFingerprint(media)
	if err != nil {
		s.failTask(task, "probe", err)
		return
	}
	input := newChapterDetectInput(media, fingerprint)
	candidates := s.serverChapterCandidates(media, task, input)
	s.updateTask(task, "chapter_merge", 88, "")
	if err := s.persistChapterResult(task, media, fingerprint, "server_sparse_scene_v1", "server", "", candidates, input); err != nil {
		s.failTask(task, "persist", err)
		return
	}
	if s.logger != nil {
		s.logger.Infof("distributed chapter detection server fallback completed media=%s task=%s", media.ID, taskID)
	}
}

func nearestChapterSampleIndex(samples []float64, value float64) (int, float64) {
	bestIndex := -1
	bestDistance := math.MaxFloat64
	for index, sample := range samples {
		distance := math.Abs(sample - value)
		if distance < bestDistance {
			bestIndex, bestDistance = index, distance
		}
	}
	return bestIndex, bestDistance
}

func (s *MediaAnalysisService) completeChapterDetectComputeTask(taskID string, remote *mediaAnalysisRemoteTask, descriptor mediaComputeTaskDescriptor, payload json.RawMessage) error {
	if remote == nil {
		return ErrMediaAnalysisWorkerClaim
	}
	var input MediaComputeChapterDetectInput
	if err := json.Unmarshal(descriptor.Input, &input); err != nil {
		return ErrMediaAnalysisWorkerResult
	}
	var result MediaComputeChapterDetectResult
	if err := json.Unmarshal(payload, &result); err != nil {
		return ErrMediaAnalysisWorkerResult
	}
	if input.MediaID == "" || input.Fingerprint == "" || input.Duration <= 0 ||
		input.EngineVersion != chapterEngineVersion || len(input.SampleTimes) == 0 ||
		result.Fingerprint != input.Fingerprint || remote.MediaID != input.MediaID || remote.Fingerprint != input.Fingerprint ||
		len(result.Candidates) != len(input.SampleTimes) {
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

	seen := make(map[int]struct{}, len(input.SampleTimes))
	canonical := make([]MediaComputeChapterCandidate, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		if !validChapterCandidate(candidate, input.Duration) {
			return ErrMediaAnalysisWorkerResult
		}
		index, distance := nearestChapterSampleIndex(input.SampleTimes, candidate.Time)
		if index < 0 || distance > 1.5 {
			return ErrMediaAnalysisWorkerResult
		}
		if _, exists := seen[index]; exists {
			return ErrMediaAnalysisWorkerResult
		}
		seen[index] = struct{}{}
		canonical = append(canonical, MediaComputeChapterCandidate{Time: input.SampleTimes[index], Score: candidate.Score})
	}

	task, err := s.taskRepo.FindByID(taskID)
	if err != nil {
		return err
	}
	s.updateTask(task, "chapter_merge", 95, "")
	workerKind := normalizeWorkerKind(remote.WorkerKind)
	if err := s.persistChapterResult(task, media, fingerprint, "distributed_scene_probe_v1", workerKind, remote.ClaimedBy, canonical, input); err != nil {
		return err
	}
	s.UnregisterComputeTask(taskID)
	if s.logger != nil {
		s.logger.Infof("distributed chapter detection completed media=%s task=%s worker=%s", media.ID, taskID, remote.ClaimedBy)
	}
	return nil
}

// chapterCandidateSummary 只用于测试/调试，确保输出稳定排序。
func chapterCandidateSummary(items []MediaComputeChapterCandidate) string {
	copyItems := append([]MediaComputeChapterCandidate(nil), items...)
	sort.Slice(copyItems, func(i, j int) bool { return copyItems[i].Time < copyItems[j].Time })
	parts := make([]string, 0, len(copyItems))
	for _, item := range copyItems {
		parts = append(parts, fmt.Sprintf("%.1f:%.3f", item.Time, item.Score))
	}
	return strings.Join(parts, ",")
}
