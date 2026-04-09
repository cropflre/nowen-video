package service

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== 字幕预处理事件常量 ====================

const (
	EventSubPreStarted   = "sub_preprocess_started"
	EventSubPreProgress  = "sub_preprocess_progress"
	EventSubPreCompleted = "sub_preprocess_completed"
	EventSubPreFailed    = "sub_preprocess_failed"
	EventSubPreSkipped   = "sub_preprocess_skipped"
)

// SubPreProgressData 字幕预处理进度事件数据
type SubPreProgressData struct {
	TaskID     string  `json:"task_id"`
	MediaID    string  `json:"media_id"`
	MediaTitle string  `json:"media_title"`
	Status     string  `json:"status"`
	Phase      string  `json:"phase"`
	Progress   float64 `json:"progress"`
	Message    string  `json:"message"`
	Error      string  `json:"error,omitempty"`
}

// SubtitlePreprocessService 字幕预处理服务
type SubtitlePreprocessService struct {
	cfg        *config.Config
	repo       *repository.SubtitlePreprocessRepo
	mediaRepo  *repository.MediaRepo
	asrService *ASRService
	scanner    *ScannerService
	logger     *zap.SugaredLogger
	wsHub      *WSHub

	// 工作协程控制
	workerCount int32
	maxWorkers  int32
	jobQueue    chan string // 任务 ID 队列
	cancelJobs  sync.Map    // 取消的任务 ID 集合

	// 进度广播节流
	lastBroadcast sync.Map // taskID -> time.Time
}

// NewSubtitlePreprocessService 创建字幕预处理服务
func NewSubtitlePreprocessService(
	cfg *config.Config,
	repo *repository.SubtitlePreprocessRepo,
	mediaRepo *repository.MediaRepo,
	asrService *ASRService,
	scanner *ScannerService,
	logger *zap.SugaredLogger,
) *SubtitlePreprocessService {
	// 字幕预处理默认 1 个 worker（避免与视频预处理争抢资源）
	maxWorkers := int32(cfg.AI.SubtitlePreprocessWorkers)
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	s := &SubtitlePreprocessService{
		cfg:        cfg,
		repo:       repo,
		mediaRepo:  mediaRepo,
		asrService: asrService,
		scanner:    scanner,
		logger:     logger,
		maxWorkers: maxWorkers,
		jobQueue:   make(chan string, 200),
	}

	// 启动工作协程池
	for i := int32(0); i < maxWorkers; i++ {
		go s.worker(int(i))
	}

	// 恢复未完成的任务
	go s.recoverPendingTasks()

	return s
}

// SetWSHub 设置 WebSocket Hub
func (s *SubtitlePreprocessService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== 公开 API ====================

// SubmitMedia 提交单个媒体进行字幕预处理
func (s *SubtitlePreprocessService) SubmitMedia(mediaID string, targetLangs []string, forceRegenerate bool) (*model.SubtitlePreprocessTask, error) {
	// 检查是否已有活跃任务
	existing, err := s.repo.FindActiveByMediaID(mediaID)
	if err == nil && existing != nil {
		return existing, nil
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("媒体不存在: %w", err)
	}

	// STRM 远程流也支持（ASR 服务已支持远程流）
	// 但本地文件需要检查是否存在
	if media.StreamURL == "" {
		if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("媒体文件不存在: %s", media.FilePath)
		}
	}

	// 序列化目标语言列表
	targetLangsStr := ""
	if len(targetLangs) > 0 {
		targetLangsStr = strings.Join(targetLangs, ",")
	}

	task := &model.SubtitlePreprocessTask{
		MediaID:         mediaID,
		Status:          "pending",
		Phase:           "check",
		Message:         "等待处理...",
		SourceLang:      "auto",
		TargetLangs:     targetLangsStr,
		ForceRegenerate: forceRegenerate,
		MediaTitle:      media.DisplayTitle(),
	}

	if err := s.repo.Create(task); err != nil {
		return nil, fmt.Errorf("创建字幕预处理任务失败: %w", err)
	}

	// 入队
	select {
	case s.jobQueue <- task.ID:
	default:
		s.logger.Warnf("字幕预处理队列已满，任务 %s 将在下次调度时处理", task.ID)
	}

	return task, nil
}

// BatchSubmit 批量提交字幕预处理任务
func (s *SubtitlePreprocessService) BatchSubmit(mediaIDs []string, targetLangs []string, forceRegenerate bool) ([]*model.SubtitlePreprocessTask, error) {
	var tasks []*model.SubtitlePreprocessTask
	for _, id := range mediaIDs {
		task, err := s.SubmitMedia(id, targetLangs, forceRegenerate)
		if err != nil {
			s.logger.Warnf("批量提交字幕预处理跳过 %s: %v", id, err)
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// SubmitLibrary 提交整个媒体库的所有视频进行字幕预处理
func (s *SubtitlePreprocessService) SubmitLibrary(libraryID string, targetLangs []string, forceRegenerate bool) (int, error) {
	medias, err := s.mediaRepo.ListByLibraryID(libraryID)
	if err != nil {
		return 0, fmt.Errorf("查询媒体库失败: %w", err)
	}

	count := 0
	for _, media := range medias {
		// 跳过已有活跃任务的
		if _, err := s.repo.FindActiveByMediaID(media.ID); err == nil {
			continue
		}
		// 跳过已完成且非强制重新生成的
		if !forceRegenerate {
			if existing, err := s.repo.FindByMediaID(media.ID); err == nil && existing.Status == "completed" {
				continue
			}
		}

		if _, err := s.SubmitMedia(media.ID, targetLangs, forceRegenerate); err == nil {
			count++
		}
	}

	return count, nil
}

// CancelTask 取消任务
func (s *SubtitlePreprocessService) CancelTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status == "completed" || task.Status == "cancelled" || task.Status == "skipped" {
		return fmt.Errorf("任务状态 %s 不可取消", task.Status)
	}

	s.cancelJobs.Store(taskID, true)
	task.Status = "cancelled"
	task.Message = "已取消"
	s.repo.Update(task)

	return nil
}

// RetryTask 重试失败的任务
func (s *SubtitlePreprocessService) RetryTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status != "failed" {
		return fmt.Errorf("只有失败的任务可以重试")
	}

	task.Status = "pending"
	task.Error = ""
	task.Message = "重试中..."
	s.repo.Update(task)

	select {
	case s.jobQueue <- task.ID:
	default:
	}

	return nil
}

// GetTask 获取任务详情
func (s *SubtitlePreprocessService) GetTask(taskID string) (*model.SubtitlePreprocessTask, error) {
	return s.repo.FindByID(taskID)
}

// GetMediaTask 获取媒体的字幕预处理任务
func (s *SubtitlePreprocessService) GetMediaTask(mediaID string) (*model.SubtitlePreprocessTask, error) {
	return s.repo.FindByMediaID(mediaID)
}

// ListTasks 分页获取任务列表
func (s *SubtitlePreprocessService) ListTasks(page, pageSize int, status string) ([]model.SubtitlePreprocessTask, int64, error) {
	tasks, total, err := s.repo.ListAll(page, pageSize, status)
	if err != nil {
		return tasks, total, err
	}
	// 用关联的 Media 信息补充/修正 media_title（兼容旧任务缺少集数信息的情况）
	for i := range tasks {
		if tasks[i].Media.ID != "" {
			tasks[i].MediaTitle = tasks[i].Media.DisplayTitle()
		}
	}
	return tasks, total, err
}

// GetStatistics 获取字幕预处理统计
func (s *SubtitlePreprocessService) GetStatistics() map[string]interface{} {
	counts, _ := s.repo.CountByStatus()

	return map[string]interface{}{
		"status_counts":  counts,
		"max_workers":    s.maxWorkers,
		"active_workers": atomic.LoadInt32(&s.workerCount),
		"queue_size":     len(s.jobQueue),
		"asr_enabled":    s.asrService != nil && s.asrService.IsEnabled(),
	}
}

// DeleteTask 删除任务（仅终态任务可删除）
func (s *SubtitlePreprocessService) DeleteTask(taskID string) error {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	if task.Status == "running" {
		return fmt.Errorf("运行中的任务不可删除，请先取消")
	}

	return s.repo.DeleteByID(taskID)
}

// ==================== 工作协程 ====================

func (s *SubtitlePreprocessService) worker(id int) {
	s.logger.Infof("字幕预处理工作协程 #%d 启动", id)
	for taskID := range s.jobQueue {
		// 检查是否已取消
		if _, cancelled := s.cancelJobs.LoadAndDelete(taskID); cancelled {
			continue
		}

		atomic.AddInt32(&s.workerCount, 1)
		s.processTask(taskID)
		atomic.AddInt32(&s.workerCount, -1)

		// 清理广播节流缓存
		s.lastBroadcast.Delete(taskID)
	}
}

func (s *SubtitlePreprocessService) processTask(taskID string) {
	task, err := s.repo.FindByID(taskID)
	if err != nil {
		s.logger.Warnf("字幕预处理任务不存在: %s", taskID)
		return
	}

	if task.Status == "cancelled" || task.Status == "completed" || task.Status == "skipped" {
		return
	}

	now := time.Now()
	task.Status = "running"
	task.StartedAt = &now
	task.Message = "开始字幕预处理..."
	s.repo.Update(task)

	s.broadcastEvent(EventSubPreStarted, task)
	s.logger.Infof("开始字幕预处理: %s (%s)", task.MediaTitle, task.MediaID)

	media, err := s.mediaRepo.FindByID(task.MediaID)
	if err != nil {
		s.failTask(task, fmt.Sprintf("媒体不存在: %v", err))
		return
	}

	// ========== Phase 1: 检查现有字幕 ==========
	if s.isCancelled(taskID) {
		return
	}
	s.updatePhase(task, "check", 5, "正在检查现有字幕...")

	existingVTTPath, subtitleSource := s.checkExistingSubtitles(media, task.ForceRegenerate)

	if existingVTTPath != "" && !task.ForceRegenerate {
		task.OriginalVTTPath = existingVTTPath
		task.SubtitleSource = subtitleSource
		task.CueCount = s.countVTTCues(existingVTTPath)
		s.logger.Infof("发现已有字幕: %s (来源: %s, %d 条)", task.MediaTitle, subtitleSource, task.CueCount)

		// 如果没有翻译需求，直接完成
		if task.TargetLangs == "" {
			s.completeTask(task, "已有字幕，无需处理")
			return
		}

		// 跳到翻译阶段
		s.updatePhase(task, "translate", 50, "已有字幕，开始翻译...")
	} else {
		// ========== Phase 2: 字幕格式标准化（提取/转换为 VTT） ==========
		if s.isCancelled(taskID) {
			return
		}

		// 检查是否有内嵌或外挂字幕可以提取
		extractedPath, extractSource := s.tryExtractSubtitle(media)
		if extractedPath != "" && !task.ForceRegenerate {
			task.OriginalVTTPath = extractedPath
			task.SubtitleSource = extractSource
			task.CueCount = s.countVTTCues(extractedPath)
			s.updatePhase(task, "extract", 30, fmt.Sprintf("已提取字幕（%d 条, %s）", task.CueCount, extractSource))
		} else {
			// ========== Phase 3: AI 字幕生成 ==========
			if s.isCancelled(taskID) {
				return
			}

			if s.asrService == nil || !s.asrService.IsEnabled() {
				// 无 AI 服务且无现有字幕，跳过
				task.Status = "skipped"
				task.Message = "无可用字幕且 AI 服务未启用"
				task.Phase = "done"
				completedAt := time.Now()
				task.CompletedAt = &completedAt
				s.repo.Update(task)
				s.broadcastEvent(EventSubPreSkipped, task)
				s.logger.Infof("字幕预处理跳过: %s (无可用字幕且 AI 未启用)", task.MediaTitle)
				return
			}

			s.updatePhase(task, "generate", 20, "正在生成 AI 字幕...")

			vttPath, err := s.generateAISubtitle(media, task)
			if err != nil {
				s.failTask(task, fmt.Sprintf("AI 字幕生成失败: %v", err))
				return
			}

			task.OriginalVTTPath = vttPath
			task.SubtitleSource = "ai_generated"
			task.CueCount = s.countVTTCues(vttPath)
			s.updatePhase(task, "generate", 50, fmt.Sprintf("AI 字幕生成完成（%d 条）", task.CueCount))
		}
	}

	// ========== Phase 2.5: 字幕内容预处理（清洗/标准化） ==========
	if s.isCancelled(taskID) {
		return
	}

	if s.cfg.AI.SubCleanEnabled && task.OriginalVTTPath != "" {
		s.updatePhase(task, "clean", 40, "正在清洗字幕内容...")

		cleaner := NewSubtitleCleaner(s.buildCleanConfig(), s.logger)
		report, err := cleaner.CleanVTT(task.OriginalVTTPath)
		if err != nil {
			s.logger.Warnf("字幕清洗失败（不影响后续流程）: %v", err)
			s.updatePhase(task, "clean", 45, fmt.Sprintf("字幕清洗失败: %v", err))
		} else {
			task.CueCount = report.ProcessedCueCount
			msg := fmt.Sprintf("字幕清洗完成（%d→%d 条, 编码: %s",
				report.OriginalCueCount, report.ProcessedCueCount, report.DetectedEncoding)
			if report.RemovedAds > 0 {
				msg += fmt.Sprintf(", 去广告: %d", report.RemovedAds)
			}
			if report.MergedCues > 0 {
				msg += fmt.Sprintf(", 合并: %d", report.MergedCues)
			}
			msg += "）"
			s.updatePhase(task, "clean", 48, msg)
		}
	}

	// ========== Phase 4: 多语言翻译（可选） ==========
	if s.isCancelled(taskID) {
		return
	}

	if task.TargetLangs != "" && task.OriginalVTTPath != "" {
		targetLangs := strings.Split(task.TargetLangs, ",")
		translatedPaths := make(map[string]string)

		for i, lang := range targetLangs {
			lang = strings.TrimSpace(lang)
			if lang == "" {
				continue
			}
			if s.isCancelled(taskID) {
				return
			}

			progress := 50 + float64(i+1)/float64(len(targetLangs))*40
			s.updatePhase(task, "translate", progress,
				fmt.Sprintf("正在翻译为 %s (%d/%d)...", lang, i+1, len(targetLangs)))

			translatedPath, err := s.translateSubtitle(media, lang)
			if err != nil {
				s.logger.Warnf("翻译为 %s 失败: %v", lang, err)
				continue
			}
			translatedPaths[lang] = translatedPath
		}

		// 序列化翻译路径
		if len(translatedPaths) > 0 {
			var parts []string
			for lang, path := range translatedPaths {
				parts = append(parts, fmt.Sprintf("%s=%s", lang, path))
			}
			task.TranslatedPaths = strings.Join(parts, "|")
		}
	}

	// ========== 完成 ==========
	s.completeTask(task, "")
}

// ==================== 内部方法 ====================

// checkExistingSubtitles 检查媒体是否已有可用字幕
func (s *SubtitlePreprocessService) checkExistingSubtitles(media *model.Media, forceRegenerate bool) (vttPath string, source string) {
	if forceRegenerate {
		return "", ""
	}

	// 1. 检查 AI 已生成的字幕
	if s.asrService != nil {
		if path, err := s.asrService.GetVTTPath(media.ID); err == nil && path != "" {
			if _, err := os.Stat(path); err == nil {
				return path, "ai_cached"
			}
		}
	}

	// 2. 检查外挂 VTT 字幕
	if media.SubtitlePaths != "" {
		for _, subPath := range strings.Split(media.SubtitlePaths, "|") {
			subPath = strings.TrimSpace(subPath)
			if subPath == "" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(subPath))
			if ext == ".vtt" {
				if _, err := os.Stat(subPath); err == nil {
					return subPath, "external_vtt"
				}
			}
		}
	}

	return "", ""
}

// tryExtractSubtitle 尝试从内嵌或外挂字幕提取 VTT
// 返回值: (vttPath, source) - source 为 "extracted" 或 "ocr_extracted"
func (s *SubtitlePreprocessService) tryExtractSubtitle(media *model.Media) (string, string) {
	// STRM 远程流不支持字幕提取
	if media.StreamURL != "" {
		return "", ""
	}

	filePath := media.FilePath

	// 1. 尝试提取内嵌文本字幕
	tracks, err := s.scanner.GetSubtitleTracks(filePath)
	if err == nil && len(tracks) > 0 {
		// 优先选择默认字幕轨道，其次选择第一个非图形字幕
		var bestTrack *SubtitleTrack
		for i := range tracks {
			if tracks[i].Bitmap {
				continue // 跳过图形字幕
			}
			if bestTrack == nil || tracks[i].Default {
				bestTrack = &tracks[i]
			}
		}

		if bestTrack != nil {
			vttPath, err := s.scanner.ExtractSubtitle(filePath, bestTrack.Index, "vtt")
			if err == nil {
				return vttPath, "extracted"
			}
			s.logger.Warnf("提取内嵌字幕失败: %v", err)
		}

		// P3: 如果只有图形字幕，尝试 OCR 提取
		if bestTrack == nil && s.cfg.AI.OCREnabled {
			for i := range tracks {
				if tracks[i].Bitmap {
					vttPath, err := s.extractBitmapSubtitleOCR(filePath, &tracks[i])
					if err == nil {
						return vttPath, "ocr_extracted"
					}
					s.logger.Warnf("OCR 提取图形字幕失败: %v", err)
					break // 只尝试第一个图形字幕轨道
				}
			}
		}
	}

	// 2. 尝试转换外挂字幕为 VTT
	if media.SubtitlePaths != "" {
		for _, subPath := range strings.Split(media.SubtitlePaths, "|") {
			subPath = strings.TrimSpace(subPath)
			if subPath == "" {
				continue
			}
			ext := strings.ToLower(filepath.Ext(subPath))
			switch ext {
			case ".srt", ".ass", ".ssa":
				vttPath, err := s.scanner.ConvertSubtitleToVTT(subPath)
				if err == nil {
					return vttPath, "extracted"
				}
				s.logger.Warnf("转换外挂字幕失败: %v", err)
			case ".vtt":
				if _, err := os.Stat(subPath); err == nil {
					return subPath, "extracted"
				}
			}
		}
	}

	return "", ""
}

// generateAISubtitle 调用 ASR 服务生成 AI 字幕（同步等待完成）
func (s *SubtitlePreprocessService) generateAISubtitle(media *model.Media, task *model.SubtitlePreprocessTask) (string, error) {
	// 触发 ASR 生成
	asrTask, err := s.asrService.GenerateSubtitle(media.ID, task.SourceLang)
	if err != nil {
		return "", err
	}

	// 如果已经完成（缓存命中）
	if asrTask.Status == "completed" && asrTask.VTTPath != "" {
		task.DetectedLanguage = asrTask.Language
		return asrTask.VTTPath, nil
	}

	// 轮询等待 ASR 任务完成
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(30 * time.Minute) // 最长等待 30 分钟

	for {
		select {
		case <-ticker.C:
			if s.isCancelled(task.ID) {
				return "", fmt.Errorf("任务已取消")
			}

			asrTask = s.asrService.GetTask(media.ID)
			if asrTask == nil {
				// 任务可能已完成并被清理，检查缓存
				if vttPath, err := s.asrService.GetVTTPath(media.ID); err == nil {
					return vttPath, nil
				}
				return "", fmt.Errorf("ASR 任务丢失")
			}

			switch asrTask.Status {
			case "completed":
				task.DetectedLanguage = asrTask.Language
				return asrTask.VTTPath, nil
			case "failed":
				return "", fmt.Errorf("ASR 失败: %s", asrTask.Error)
			default:
				// 更新进度
				asrProgress := 20 + asrTask.Progress*0.3 // 映射到 20%~50%
				s.updatePhase(task, "generate", asrProgress,
					fmt.Sprintf("AI 字幕生成中: %s (%.0f%%)", asrTask.Message, asrTask.Progress))
			}

		case <-timeout:
			return "", fmt.Errorf("ASR 任务超时（30分钟）")
		}
	}
}

// translateSubtitle 调用 ASR 翻译服务翻译字幕（同步等待完成）
func (s *SubtitlePreprocessService) translateSubtitle(media *model.Media, targetLang string) (string, error) {
	if s.asrService == nil {
		return "", fmt.Errorf("ASR 服务不可用")
	}

	// 触发翻译
	translateTask, err := s.asrService.TranslateSubtitle(media.ID, targetLang)
	if err != nil {
		return "", err
	}

	// 如果已经完成（缓存命中）
	if translateTask.Status == "completed" && translateTask.VTTPath != "" {
		return translateTask.VTTPath, nil
	}

	// 轮询等待翻译完成
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-ticker.C:
			translateTask = s.asrService.GetTranslateTask(media.ID, targetLang)
			if translateTask == nil {
				// 检查缓存
				if vttPath, err := s.asrService.GetTranslatedVTTPath(media.ID, targetLang); err == nil {
					return vttPath, nil
				}
				return "", fmt.Errorf("翻译任务丢失")
			}

			switch translateTask.Status {
			case "completed":
				return translateTask.VTTPath, nil
			case "failed":
				return "", fmt.Errorf("翻译失败: %s", translateTask.Error)
			}

		case <-timeout:
			return "", fmt.Errorf("翻译任务超时（10分钟）")
		}
	}
}

// countVTTCues 统计 VTT 文件中的字幕条目数
func (s *SubtitlePreprocessService) countVTTCues(vttPath string) int {
	content, err := os.ReadFile(vttPath)
	if err != nil {
		return 0
	}
	cues := parseVTTCues(string(content))
	return len(cues)
}

// ==================== P3: 图形字幕 OCR 支持 ====================

// extractBitmapSubtitleOCR 通过 FFmpeg + Tesseract 提取图形字幕（PGS/VobSub）为文本字幕
// 流程：
// 1. FFmpeg 将图形字幕导出为带时间戳的图片序列
// 2. Tesseract 对每张图片进行 OCR 识别
// 3. 将识别结果组装为 VTT 格式
func (s *SubtitlePreprocessService) extractBitmapSubtitleOCR(filePath string, track *SubtitleTrack) (string, error) {
	if !s.cfg.AI.OCREnabled {
		return "", fmt.Errorf("OCR 未启用")
	}

	// 检查 Tesseract 是否可用
	tesseractBin := s.cfg.AI.TesseractPath
	if tesseractBin == "" {
		tesseractBin = "tesseract"
	}
	if _, err := exec.LookPath(tesseractBin); err != nil {
		return "", fmt.Errorf("Tesseract 未安装或不在 PATH 中: %v", err)
	}

	tesseractLang := s.cfg.AI.TesseractLang
	if tesseractLang == "" {
		tesseractLang = "eng"
	}

	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "nowen-ocr-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	s.logger.Infof("OCR 提取图形字幕: 轨道 #%d (%s), 语言: %s", track.Index, track.Codec, tesseractLang)

	// 第一步：用 FFmpeg 将图形字幕导出为 SRT（利用 FFmpeg 的内置 OCR 能力）
	// 对于 PGS 字幕，先尝试用 FFmpeg 直接导出为图片序列
	supFilePath := filepath.Join(tmpDir, "subtitle.sup")

	// 导出图形字幕流为 .sup 文件
	exportArgs := []string{
		"-i", filePath,
		"-map", fmt.Sprintf("0:%d", track.Index),
		"-c:s", "copy",
		"-y", supFilePath,
	}
	cmd := exec.Command("ffmpeg", exportArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Warnf("FFmpeg 导出图形字幕失败: %v, 输出: %s", err, string(output))
		// 回退方案：尝试直接导出为图片序列
		return s.extractBitmapSubtitleViaImages(filePath, track, tmpDir, tesseractBin, tesseractLang)
	}

	// 第二步：将 .sup 文件转换为图片序列 + 时间戳
	// 使用 FFmpeg 将 sup 转为带时间戳的图片
	imgDir := filepath.Join(tmpDir, "images")
	os.MkdirAll(imgDir, 0755)

	// 将 sup 转为图片序列
	imgArgs := []string{
		"-i", supFilePath,
		"-vsync", "0",
		"-y",
		filepath.Join(imgDir, "sub_%06d.png"),
	}
	cmd = exec.Command("ffmpeg", imgArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		s.logger.Warnf("FFmpeg 导出字幕图片失败: %v, 输出: %s", err, string(output))
		return s.extractBitmapSubtitleViaImages(filePath, track, tmpDir, tesseractBin, tesseractLang)
	}

	// 同时导出时间戳信息（通过 ffprobe 获取字幕帧时间）
	timestamps, err := s.extractSubtitleTimestamps(supFilePath)
	if err != nil || len(timestamps) == 0 {
		s.logger.Warnf("获取字幕时间戳失败: %v", err)
		return s.extractBitmapSubtitleViaImages(filePath, track, tmpDir, tesseractBin, tesseractLang)
	}

	// 第三步：对每张图片进行 OCR
	imgFiles, _ := filepath.Glob(filepath.Join(imgDir, "sub_*.png"))
	if len(imgFiles) == 0 {
		return "", fmt.Errorf("未导出任何字幕图片")
	}

	// 确保图片数和时间戳数匹配
	maxItems := len(imgFiles)
	if len(timestamps) < maxItems {
		maxItems = len(timestamps)
	}

	var cues []vttCue
	for i := 0; i < maxItems; i++ {
		// OCR 识别
		text, err := s.ocrImage(tesseractBin, imgFiles[i], tesseractLang)
		if err != nil {
			s.logger.Debugf("OCR 识别图片 %d 失败: %v", i, err)
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		cues = append(cues, vttCue{
			startTime: timestamps[i].Start,
			endTime:   timestamps[i].End,
			text:      text,
		})
	}

	if len(cues) == 0 {
		return "", fmt.Errorf("OCR 未识别出任何字幕文本")
	}

	// 第四步：生成 VTT 文件
	vttPath := filepath.Join(filepath.Dir(filePath), fmt.Sprintf(".nowen_ocr_%d.vtt", track.Index))
	if err := s.writeVTTFile(vttPath, cues); err != nil {
		return "", fmt.Errorf("写入 VTT 文件失败: %v", err)
	}

	s.logger.Infof("OCR 提取图形字幕完成: %d 条字幕", len(cues))
	return vttPath, nil
}

// extractBitmapSubtitleViaImages 回退方案：直接从视频文件导出字幕图片并 OCR
func (s *SubtitlePreprocessService) extractBitmapSubtitleViaImages(filePath string, track *SubtitleTrack, tmpDir string, tesseractBin string, tesseractLang string) (string, error) {
	imgDir := filepath.Join(tmpDir, "images_fallback")
	os.MkdirAll(imgDir, 0755)

	// 直接从视频文件导出字幕帧为图片
	// 使用 subtitle filter 将字幕烧录到空白背景上
	args := []string{
		"-i", filePath,
		"-filter_complex", fmt.Sprintf("[0:%d]scale=iw:ih[sub]", track.Index),
		"-map", "[sub]",
		"-vsync", "0",
		"-y",
		filepath.Join(imgDir, "frame_%06d.png"),
	}
	cmd := exec.Command("ffmpeg", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("FFmpeg 图形字幕导出失败: %v, 输出: %s", err, string(output))
	}

	// 获取时间戳
	timestamps, err := s.extractSubtitleTimestampsFromVideo(filePath, track.Index)
	if err != nil {
		return "", fmt.Errorf("获取字幕时间戳失败: %v", err)
	}

	imgFiles, _ := filepath.Glob(filepath.Join(imgDir, "frame_*.png"))
	if len(imgFiles) == 0 {
		return "", fmt.Errorf("未导出任何字幕图片")
	}

	maxItems := len(imgFiles)
	if len(timestamps) < maxItems {
		maxItems = len(timestamps)
	}

	var cues []vttCue
	for i := 0; i < maxItems; i++ {
		text, err := s.ocrImage(tesseractBin, imgFiles[i], tesseractLang)
		if err != nil {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		cues = append(cues, vttCue{
			startTime: timestamps[i].Start,
			endTime:   timestamps[i].End,
			text:      text,
		})
	}

	if len(cues) == 0 {
		return "", fmt.Errorf("OCR 未识别出任何字幕文本")
	}

	vttPath := filepath.Join(filepath.Dir(filePath), fmt.Sprintf(".nowen_ocr_%d.vtt", track.Index))
	if err := s.writeVTTFile(vttPath, cues); err != nil {
		return "", fmt.Errorf("写入 VTT 文件失败: %v", err)
	}

	s.logger.Infof("OCR 回退方案提取完成: %d 条字幕", len(cues))
	return vttPath, nil
}

// ocrTimestamp OCR 时间戳对
type ocrTimestamp struct {
	Start string
	End   string
}

// extractSubtitleTimestamps 从 .sup 文件提取字幕时间戳
func (s *SubtitlePreprocessService) extractSubtitleTimestamps(supFilePath string) ([]ocrTimestamp, error) {
	// 使用 ffprobe 获取字幕帧时间信息
	args := []string{
		"-v", "quiet",
		"-show_entries", "packet=pts_time,duration_time",
		"-of", "csv=p=0",
		supFilePath,
	}
	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe 失败: %v", err)
	}

	var timestamps []ocrTimestamp
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		ptsTime := strings.TrimSpace(parts[0])
		durationTime := strings.TrimSpace(parts[1])

		startSec := parseFloatSafe(ptsTime)
		durSec := parseFloatSafe(durationTime)
		if durSec <= 0 {
			durSec = 3.0 // 默认显示 3 秒
		}

		timestamps = append(timestamps, ocrTimestamp{
			Start: formatVTTTime(startSec),
			End:   formatVTTTime(startSec + durSec),
		})
	}

	return timestamps, nil
}

// extractSubtitleTimestampsFromVideo 从视频文件直接提取字幕流时间戳
func (s *SubtitlePreprocessService) extractSubtitleTimestampsFromVideo(filePath string, streamIndex int) ([]ocrTimestamp, error) {
	args := []string{
		"-v", "quiet",
		"-select_streams", fmt.Sprintf("%d", streamIndex),
		"-show_entries", "packet=pts_time,duration_time",
		"-of", "csv=p=0",
		filePath,
	}
	cmd := exec.Command("ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe 失败: %v", err)
	}

	var timestamps []ocrTimestamp
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		startSec := parseFloatSafe(strings.TrimSpace(parts[0]))
		durSec := parseFloatSafe(strings.TrimSpace(parts[1]))
		if durSec <= 0 {
			durSec = 3.0
		}

		timestamps = append(timestamps, ocrTimestamp{
			Start: formatVTTTime(startSec),
			End:   formatVTTTime(startSec + durSec),
		})
	}

	return timestamps, nil
}

// ocrImage 对单张图片进行 OCR 识别
func (s *SubtitlePreprocessService) ocrImage(tesseractBin string, imgPath string, lang string) (string, error) {
	// tesseract input.png stdout -l chi_sim+eng --psm 6
	args := []string{
		imgPath,
		"stdout",
		"-l", lang,
		"--psm", "6", // 假设为单个文本块
	}
	cmd := exec.Command(tesseractBin, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// 清理 OCR 输出：去除多余空行、特殊字符
	text := strings.TrimSpace(string(output))
	// 去除 Tesseract 的常见垃圾输出
	text = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f]`).ReplaceAllString(text, "")
	// 合并多行为单行（保留换行）
	lines := strings.Split(text, "\n")
	var cleanLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	return strings.Join(cleanLines, "\n"), nil
}

// writeVTTFile 将 cues 写入 VTT 文件
func (s *SubtitlePreprocessService) writeVTTFile(vttPath string, cues []vttCue) error {
	f, err := os.Create(vttPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	w.WriteString("WEBVTT\n\n")

	for i, cue := range cues {
		fmt.Fprintf(w, "%d\n", i+1)
		fmt.Fprintf(w, "%s --> %s\n", cue.startTime, cue.endTime)
		fmt.Fprintf(w, "%s\n\n", cue.text)
	}

	return w.Flush()
}

// parseFloatSafe 安全解析浮点数
func parseFloatSafe(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// ==================== 辅助方法 ====================

func (s *SubtitlePreprocessService) isCancelled(taskID string) bool {
	_, cancelled := s.cancelJobs.Load(taskID)
	return cancelled
}

func (s *SubtitlePreprocessService) updatePhase(task *model.SubtitlePreprocessTask, phase string, progress float64, message string) {
	task.Phase = phase
	task.Progress = progress
	task.Message = message
	s.repo.Update(task)

	// 节流广播：每 3 秒最多广播一次
	now := time.Now()
	if lastTime, ok := s.lastBroadcast.Load(task.ID); ok {
		if now.Sub(lastTime.(time.Time)) < 3*time.Second {
			return
		}
	}
	s.lastBroadcast.Store(task.ID, now)
	s.broadcastEvent(EventSubPreProgress, task)
}

func (s *SubtitlePreprocessService) failTask(task *model.SubtitlePreprocessTask, errMsg string) {
	task.Status = "failed"
	task.Error = errMsg
	task.Message = errMsg
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	if task.StartedAt != nil {
		task.ElapsedSec = completedAt.Sub(*task.StartedAt).Seconds()
	}
	s.repo.Update(task)

	s.broadcastEvent(EventSubPreFailed, task)
	s.logger.Warnf("字幕预处理失败: %s - %s", task.MediaTitle, errMsg)
}

func (s *SubtitlePreprocessService) completeTask(task *model.SubtitlePreprocessTask, customMsg string) {
	completedAt := time.Now()
	task.Status = "completed"
	task.Phase = "done"
	task.Progress = 100
	task.CompletedAt = &completedAt
	if task.StartedAt != nil {
		task.ElapsedSec = completedAt.Sub(*task.StartedAt).Seconds()
	}

	if customMsg != "" {
		task.Message = customMsg
	} else {
		parts := []string{fmt.Sprintf("字幕预处理完成（来源: %s", task.SubtitleSource)}
		if task.CueCount > 0 {
			parts = append(parts, fmt.Sprintf(", %d 条字幕", task.CueCount))
		}
		if task.TranslatedPaths != "" {
			translatedCount := len(strings.Split(task.TranslatedPaths, "|"))
			parts = append(parts, fmt.Sprintf(", 翻译 %d 种语言", translatedCount))
		}
		task.Message = strings.Join(parts, "") + "）"
	}

	s.repo.Update(task)
	s.broadcastEvent(EventSubPreCompleted, task)
	s.logger.Infof("字幕预处理完成: %s, 来源: %s, 字幕数: %d, 耗时: %.1fs",
		task.MediaTitle, task.SubtitleSource, task.CueCount, task.ElapsedSec)
}

func (s *SubtitlePreprocessService) broadcastEvent(eventType string, task *model.SubtitlePreprocessTask) {
	if s.wsHub == nil {
		return
	}
	s.wsHub.BroadcastEvent(eventType, SubPreProgressData{
		TaskID:     task.ID,
		MediaID:    task.MediaID,
		MediaTitle: task.MediaTitle,
		Status:     task.Status,
		Phase:      task.Phase,
		Progress:   task.Progress,
		Message:    task.Message,
		Error:      task.Error,
	})
}

// BatchDeleteTasks 批量删除任务（跳过运行中的任务）
func (s *SubtitlePreprocessService) BatchDeleteTasks(taskIDs []string) (int64, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	return s.repo.DeleteByIDs(taskIDs)
}

// BatchCancelTasks 批量取消任务
func (s *SubtitlePreprocessService) BatchCancelTasks(taskIDs []string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	cancelled := 0
	for _, id := range taskIDs {
		if err := s.CancelTask(id); err == nil {
			cancelled++
		}
	}
	return cancelled, nil
}

// BatchRetryTasks 批量重试任务
func (s *SubtitlePreprocessService) BatchRetryTasks(taskIDs []string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, nil
	}
	retried := 0
	for _, id := range taskIDs {
		if err := s.RetryTask(id); err == nil {
			retried++
		}
	}
	return retried, nil
}

// buildCleanConfig 从应用配置构建字幕清洗配置
func (s *SubtitlePreprocessService) buildCleanConfig() SubtitleCleanConfig {
	return SubtitleCleanConfig{
		AutoDetectEncoding:   true, // 始终启用编码检测
		FallbackEncoding:     s.cfg.AI.SubCleanFallbackEnc,
		RemoveHTMLTags:       s.cfg.AI.SubCleanRemoveHTML,
		RemoveASSStyles:      s.cfg.AI.SubCleanRemoveASSStyle,
		NormalizePunctuation: s.cfg.AI.SubCleanNormalizePunct,
		RemoveSDH:            s.cfg.AI.SubCleanRemoveSDH,
		RemoveAds:            s.cfg.AI.SubCleanRemoveAds,
		TimeOffsetMs:         s.cfg.AI.SubCleanTimeOffsetMs,
		MinDurationMs:        s.cfg.AI.SubCleanMinDurationMs,
		MaxDurationMs:        s.cfg.AI.SubCleanMaxDurationMs,
		MinGapMs:             s.cfg.AI.SubCleanMinGapMs,
		MergeShortCues:       s.cfg.AI.SubCleanMergeShort,
		SplitLongCues:        s.cfg.AI.SubCleanSplitLong,
		MaxCharsPerLine:      s.cfg.AI.SubCleanMaxCharsPerLine,
		MaxLinesPerCue:       s.cfg.AI.SubCleanMaxLinesPerCue,
		BackupOriginal:       s.cfg.AI.SubCleanBackup,
	}
}

// recoverPendingTasks 恢复服务重启前未完成的任务
func (s *SubtitlePreprocessService) recoverPendingTasks() {
	time.Sleep(5 * time.Second) // 等待服务完全启动

	tasks, err := s.repo.ListPending(50)
	if err != nil {
		return
	}

	// 将之前 running 的任务重置为 pending
	running, _ := s.repo.ListRunning()
	for _, task := range running {
		task.Status = "pending"
		task.Message = "服务重启后恢复..."
		s.repo.Update(&task)
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		select {
		case s.jobQueue <- task.ID:
		default:
		}
	}

	if len(tasks) > 0 {
		s.logger.Infof("恢复 %d 个未完成的字幕预处理任务", len(tasks))
	}
}
