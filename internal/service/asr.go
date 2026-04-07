package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== ASR 语音识别服务 ====================

// ASRConfig ASR 配置
type ASRConfig struct {
	// Whisper API 地址（OpenAI 兼容，如 https://api.openai.com/v1）
	APIBase string
	// API 密钥
	APIKey string
	// 模型名称（如 whisper-1）
	Model string
	// 请求超时（秒）
	Timeout int
	// 最大并发任务数
	MaxConcurrent int
	// Phase 2: 本地 whisper.cpp 可执行文件路径（留空则使用云端 API）
	WhisperCppPath string
	// Phase 2: 本地 Whisper 模型文件路径（如 ggml-large-v3.bin）
	WhisperModelPath string
	// Phase 2: 本地 Whisper 线程数（默认 4）
	WhisperThreads int
	// Phase 2: 是否优先使用本地引擎
	PreferLocal bool
}

// ASRService AI 语音识别字幕生成服务
type ASRService struct {
	cfg       ASRConfig
	appCfg    *config.Config
	mediaRepo *repository.MediaRepo
	aiService *AIService // Phase 4: 用于字幕翻译
	logger    *zap.SugaredLogger
	client    *http.Client
	wsHub     *WSHub

	// 并发控制
	semaphore chan struct{}

	// 任务状态管理
	tasks   map[string]*ASRTask
	tasksMu sync.RWMutex
}

// ASRTask ASR 任务状态
type ASRTask struct {
	MediaID   string    `json:"media_id"`
	Status    string    `json:"status"` // pending / extracting / transcribing / converting / translating / completed / failed
	Progress  float64   `json:"progress"`
	Message   string    `json:"message"`
	Language  string    `json:"language"`
	Engine    string    `json:"engine,omitempty"` // cloud / local
	VTTPath   string    `json:"vtt_path,omitempty"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// WhisperSegment Whisper API 返回的分段数据
type WhisperSegment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

// WhisperResponse Whisper API 响应
type WhisperResponse struct {
	Task     string           `json:"task"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
	Text     string           `json:"text"`
	Segments []WhisperSegment `json:"segments"`
}

// ASR 事件类型常量
const (
	EventASRStarted   = "asr_started"   // ASR 任务开始
	EventASRProgress  = "asr_progress"  // ASR 进度更新
	EventASRCompleted = "asr_completed" // ASR 任务完成
	EventASRFailed    = "asr_failed"    // ASR 任务失败

	// Phase 4: 字幕翻译事件
	EventTranslateProgress  = "translate_progress"  // 翻译进度
	EventTranslateCompleted = "translate_completed" // 翻译完成
	EventTranslateFailed    = "translate_failed"    // 翻译失败
)

// ASRProgressData ASR 进度事件数据
type ASRProgressData struct {
	MediaID  string  `json:"media_id"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
	Engine   string  `json:"engine,omitempty"`
	VTTPath  string  `json:"vtt_path,omitempty"`
	Error    string  `json:"error,omitempty"`
}

// NewASRService 创建 ASR 服务
func NewASRService(appCfg *config.Config, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *ASRService {
	// 从 AI 配置中读取 ASR 相关配置
	// ASR 复用 AI 的 API 配置，但使用 Whisper 模型
	aiCfg := appCfg.AI

	timeout := aiCfg.Timeout
	if timeout <= 0 {
		timeout = 300 // ASR 需要更长的超时时间
	}

	maxConcurrent := aiCfg.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 2
	}

	whisperThreads := aiCfg.WhisperThreads
	if whisperThreads <= 0 {
		whisperThreads = 4
	}

	s := &ASRService{
		cfg: ASRConfig{
			APIBase:          aiCfg.APIBase,
			APIKey:           aiCfg.APIKey,
			Model:            "whisper-1",
			Timeout:          timeout,
			MaxConcurrent:    maxConcurrent,
			WhisperCppPath:   aiCfg.WhisperCppPath,
			WhisperModelPath: aiCfg.WhisperModelPath,
			WhisperThreads:   whisperThreads,
			PreferLocal:      aiCfg.PreferLocalWhisper,
		},
		appCfg:    appCfg,
		mediaRepo: mediaRepo,
		logger:    logger,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		semaphore: make(chan struct{}, maxConcurrent),
		tasks:     make(map[string]*ASRTask),
	}

	if aiCfg.Enabled && aiCfg.APIKey != "" {
		logger.Info("ASR 语音识别服务已就绪（复用 AI 配置）")
	} else {
		logger.Info("ASR 语音识别服务未启用（需要配置 AI API）")
	}

	// Phase 2: 检测本地 whisper.cpp
	if s.cfg.WhisperCppPath != "" {
		if _, err := exec.LookPath(s.cfg.WhisperCppPath); err == nil {
			logger.Infof("本地 whisper.cpp 已检测到: %s", s.cfg.WhisperCppPath)
		} else {
			logger.Warnf("本地 whisper.cpp 路径无效: %s", s.cfg.WhisperCppPath)
			s.cfg.WhisperCppPath = ""
		}
	}

	return s
}

// SetWSHub 设置 WebSocket Hub
func (s *ASRService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// SetAIService 设置 AI 服务（Phase 4: 字幕翻译需要）
func (s *ASRService) SetAIService(ai *AIService) {
	s.aiService = ai
}

// SetWhisperCppPath Phase 2: 设置本地 whisper.cpp 路径
func (s *ASRService) SetWhisperCppPath(path string) {
	s.cfg.WhisperCppPath = path
}

// SetWhisperModelPath Phase 2: 设置本地 Whisper 模型路径
func (s *ASRService) SetWhisperModelPath(path string) {
	s.cfg.WhisperModelPath = path
}

// SetPreferLocal Phase 2: 设置是否优先使用本地引擎
func (s *ASRService) SetPreferLocal(prefer bool) {
	s.cfg.PreferLocal = prefer
}

// IsEnabled 检查 ASR 服务是否可用（云端 API 或本地引擎任一可用即可）
func (s *ASRService) IsEnabled() bool {
	cloudEnabled := s.cfg.APIKey != "" && s.cfg.APIBase != ""
	localEnabled := s.cfg.WhisperCppPath != "" && s.cfg.WhisperModelPath != ""
	return cloudEnabled || localEnabled
}

// IsLocalEnabled Phase 2: 检查本地引擎是否可用
func (s *ASRService) IsLocalEnabled() bool {
	return s.cfg.WhisperCppPath != "" && s.cfg.WhisperModelPath != ""
}

// IsCloudEnabled 检查云端 API 是否可用
func (s *ASRService) IsCloudEnabled() bool {
	return s.cfg.APIKey != "" && s.cfg.APIBase != ""
}

// GetTask 获取任务状态
func (s *ASRService) GetTask(mediaID string) *ASRTask {
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()
	return s.tasks[mediaID]
}

// GenerateSubtitle 为指定媒体生成 AI 字幕（异步）
func (s *ASRService) GenerateSubtitle(mediaID string, language string) (*ASRTask, error) {
	if !s.IsEnabled() {
		return nil, fmt.Errorf("ASR 服务未启用，请先配置 AI API")
	}

	// 检查是否已有进行中的任务
	s.tasksMu.RLock()
	existing := s.tasks[mediaID]
	s.tasksMu.RUnlock()
	if existing != nil && (existing.Status == "extracting" || existing.Status == "transcribing" || existing.Status == "converting") {
		return existing, nil // 返回已有任务
	}

	// 获取媒体信息
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("媒体不存在: %w", err)
	}

	// Phase 3: STRM 远程流也支持（通过下载音频片段）
	isSTRM := media.StreamURL != ""

	// 本地文件检查
	if !isSTRM {
		if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
			return nil, fmt.Errorf("媒体文件不存在: %s", media.FilePath)
		}
	}

	// 确定输入源
	inputSource := media.FilePath
	if isSTRM {
		inputSource = media.StreamURL
	}

	// 检查是否已有缓存的 AI 字幕
	vttPath := s.getVTTCachePath(inputSource, language)
	if _, err := os.Stat(vttPath); err == nil {
		task := &ASRTask{
			MediaID:   mediaID,
			Status:    "completed",
			Progress:  100,
			Message:   "AI 字幕已存在（缓存）",
			Language:  language,
			VTTPath:   vttPath,
			CreatedAt: time.Now(),
		}
		return task, nil
	}

	// 确定使用的引擎
	engine := "cloud"
	if s.cfg.PreferLocal && s.IsLocalEnabled() {
		engine = "local"
	} else if !s.IsCloudEnabled() && s.IsLocalEnabled() {
		engine = "local"
	}

	// 创建任务
	task := &ASRTask{
		MediaID:   mediaID,
		Status:    "pending",
		Progress:  0,
		Message:   "等待处理...",
		Language:  language,
		Engine:    engine,
		CreatedAt: time.Now(),
	}

	s.tasksMu.Lock()
	s.tasks[mediaID] = task
	s.tasksMu.Unlock()

	// 异步执行
	go s.processASRTask(inputSource, mediaID, language, isSTRM, engine)

	return task, nil
}

// processASRTask 处理 ASR 任务的完整流程
func (s *ASRService) processASRTask(inputSource string, mediaID string, language string, isSTRM bool, engine string) {
	// 并发控制
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	var engineLabel string
	if engine == "local" {
		engineLabel = "本地引擎"
	} else {
		engineLabel = "云端 API"
	}

	if isSTRM {
		s.updateTask(mediaID, "extracting", 5, fmt.Sprintf("正在从远程流提取音频（%s）...", engineLabel))
	} else {
		s.updateTask(mediaID, "extracting", 10, fmt.Sprintf("正在提取音频（%s）...", engineLabel))
	}

	// 第一步：提取音频
	var audioPath string
	var err error
	if isSTRM {
		// Phase 3: 从远程流提取音频
		audioPath, err = s.extractAudioFromRemote(inputSource, mediaID)
	} else {
		audioPath, err = s.extractAudio(inputSource, mediaID)
	}
	if err != nil {
		s.failTask(mediaID, fmt.Sprintf("音频提取失败: %v", err))
		return
	}
	defer os.Remove(audioPath) // 清理临时音频文件

	s.updateTask(mediaID, "transcribing", 30, fmt.Sprintf("正在进行语音识别（%s）...", engineLabel))

	// 第二步：语音识别（根据引擎选择）
	var result *WhisperResponse
	if engine == "local" {
		// Phase 2: 本地 whisper.cpp
		result, err = s.callWhisperLocal(audioPath, language)
	} else {
		// Phase 1: 云端 Whisper API
		result, err = s.callWhisperAPI(audioPath, language)
	}
	if err != nil {
		s.failTask(mediaID, fmt.Sprintf("语音识别失败: %v", err))
		return
	}

	s.updateTask(mediaID, "converting", 80, "正在生成字幕文件...")

	// 第三步：将识别结果转换为 VTT 格式
	vttPath, err := s.convertToVTT(inputSource, language, result)
	if err != nil {
		s.failTask(mediaID, fmt.Sprintf("字幕生成失败: %v", err))
		return
	}

	// 完成
	completionMsg := fmt.Sprintf("AI 字幕生成完成（%s，%d 条字幕，%s）", result.Language, len(result.Segments), engineLabel)
	s.tasksMu.Lock()
	if task, ok := s.tasks[mediaID]; ok {
		task.Status = "completed"
		task.Progress = 100
		task.Message = completionMsg
		task.VTTPath = vttPath
	}
	s.tasksMu.Unlock()

	// 广播完成事件
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventASRCompleted, ASRProgressData{
			MediaID:  mediaID,
			Status:   "completed",
			Progress: 100,
			Message:  completionMsg,
			Engine:   engine,
			VTTPath:  vttPath,
		})
	}

	s.logger.Infof("AI 字幕生成完成: mediaID=%s, language=%s, segments=%d, engine=%s", mediaID, result.Language, len(result.Segments), engine)
}

// extractAudio 使用 FFmpeg 从视频中提取音频
func (s *ASRService) extractAudio(videoPath string, mediaID string) (string, error) {
	// 创建临时音频文件
	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "asr_temp")
	os.MkdirAll(cacheDir, 0755)

	audioPath := filepath.Join(cacheDir, fmt.Sprintf("%s_audio.wav", mediaID))

	// 使用 FFmpeg 提取音频：16kHz 单声道 WAV（Whisper 推荐格式）
	cmd := exec.Command(s.appCfg.App.FFmpegPath,
		"-y",            // 覆盖已有文件
		"-i", videoPath, // 输入视频
		"-vn",                  // 不要视频
		"-acodec", "pcm_s16le", // PCM 16-bit 编码
		"-ar", "16000", // 16kHz 采样率
		"-ac", "1", // 单声道
		"-f", "wav", // WAV 格式
		audioPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("FFmpeg 提取音频失败: %w, stderr: %s", err, stderr.String())
	}

	// 检查文件大小
	fi, err := os.Stat(audioPath)
	if err != nil {
		return "", fmt.Errorf("音频文件不存在: %w", err)
	}

	s.logger.Infof("音频提取完成: %s (%.1f MB)", audioPath, float64(fi.Size())/(1024*1024))

	// Whisper API 限制文件大小为 25MB
	// 如果超过，需要分段处理
	if fi.Size() > 25*1024*1024 {
		s.logger.Infof("音频文件超过 25MB，将进行分段处理")
		return audioPath, nil // 分段逻辑在 callWhisperAPI 中处理
	}

	return audioPath, nil
}

// callWhisperAPI 调用 Whisper API 进行语音识别
func (s *ASRService) callWhisperAPI(audioPath string, language string) (*WhisperResponse, error) {
	fi, err := os.Stat(audioPath)
	if err != nil {
		return nil, err
	}

	// 如果文件超过 25MB，分段处理
	if fi.Size() > 25*1024*1024 {
		return s.callWhisperAPIChunked(audioPath, language)
	}

	return s.callWhisperAPISingle(audioPath, language)
}

// callWhisperAPISingle 单次调用 Whisper API（文件 <= 25MB）
func (s *ASRService) callWhisperAPISingle(audioPath string, language string) (*WhisperResponse, error) {
	// 打开音频文件
	audioFile, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("打开音频文件失败: %w", err)
	}
	defer audioFile.Close()

	// 构建 multipart 请求
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 添加音频文件
	part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("创建表单失败: %w", err)
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return nil, fmt.Errorf("写入音频数据失败: %w", err)
	}

	// 添加模型参数
	writer.WriteField("model", s.cfg.Model)
	writer.WriteField("response_format", "verbose_json")
	writer.WriteField("timestamp_granularities[]", "segment")

	// 如果指定了语言
	if language != "" && language != "auto" {
		writer.WriteField("language", language)
	}

	writer.Close()

	// 发送请求
	apiURL := strings.TrimRight(s.cfg.APIBase, "/") + "/audio/transcriptions"
	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Whisper API 请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Whisper API 返回 HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result WhisperResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("解析 Whisper 响应失败: %w", err)
	}

	return &result, nil
}

// callWhisperAPIChunked 分段调用 Whisper API（文件 > 25MB）
func (s *ASRService) callWhisperAPIChunked(audioPath string, language string) (*WhisperResponse, error) {
	// 使用 FFmpeg 将音频分割为多个 10 分钟的片段
	cacheDir := filepath.Dir(audioPath)
	baseName := strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath))
	chunkPattern := filepath.Join(cacheDir, fmt.Sprintf("%s_chunk_%%03d.wav", baseName))

	cmd := exec.Command(s.appCfg.App.FFmpegPath,
		"-y",
		"-i", audioPath,
		"-f", "segment",
		"-segment_time", "600", // 10 分钟一段
		"-acodec", "pcm_s16le",
		"-ar", "16000",
		"-ac", "1",
		chunkPattern,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("音频分段失败: %w, stderr: %s", err, stderr.String())
	}

	// 查找所有分段文件
	pattern := filepath.Join(cacheDir, fmt.Sprintf("%s_chunk_*.wav", baseName))
	chunks, err := filepath.Glob(pattern)
	if err != nil || len(chunks) == 0 {
		return nil, fmt.Errorf("未找到音频分段文件")
	}
	sort.Strings(chunks)
	defer func() {
		for _, chunk := range chunks {
			os.Remove(chunk)
		}
	}()

	s.logger.Infof("音频已分割为 %d 个片段", len(chunks))

	// 逐段识别并合并结果
	var allSegments []WhisperSegment
	var detectedLanguage string
	var totalDuration float64
	var allText strings.Builder
	timeOffset := 0.0

	for i, chunk := range chunks {
		s.logger.Infof("正在识别第 %d/%d 段...", i+1, len(chunks))

		result, err := s.callWhisperAPISingle(chunk, language)
		if err != nil {
			return nil, fmt.Errorf("第 %d 段识别失败: %w", i+1, err)
		}

		if detectedLanguage == "" {
			detectedLanguage = result.Language
		}

		// 调整时间偏移
		for _, seg := range result.Segments {
			seg.Start += timeOffset
			seg.End += timeOffset
			allSegments = append(allSegments, seg)
		}

		allText.WriteString(result.Text)
		allText.WriteString(" ")

		timeOffset += result.Duration
		totalDuration += result.Duration
	}

	return &WhisperResponse{
		Task:     "transcribe",
		Language: detectedLanguage,
		Duration: totalDuration,
		Text:     strings.TrimSpace(allText.String()),
		Segments: allSegments,
	}, nil
}

// convertToVTT 将 Whisper 识别结果转换为 WebVTT 格式
func (s *ASRService) convertToVTT(videoPath string, language string, result *WhisperResponse) (string, error) {
	vttPath := s.getVTTCachePath(videoPath, language)
	os.MkdirAll(filepath.Dir(vttPath), 0755)

	var buf strings.Builder
	buf.WriteString("WEBVTT\n")
	buf.WriteString(fmt.Sprintf("Kind: captions\n"))
	buf.WriteString(fmt.Sprintf("Language: %s\n", result.Language))
	buf.WriteString("\n")

	for i, seg := range result.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}

		buf.WriteString(fmt.Sprintf("%d\n", i+1))
		buf.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTime(seg.Start), formatVTTTime(seg.End)))
		buf.WriteString(text)
		buf.WriteString("\n\n")
	}

	if err := os.WriteFile(vttPath, []byte(buf.String()), 0644); err != nil {
		return "", fmt.Errorf("写入 VTT 文件失败: %w", err)
	}

	return vttPath, nil
}

// getVTTCachePath 获取 AI 字幕的缓存路径
func (s *ASRService) getVTTCachePath(videoPath string, language string) string {
	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles", "ai")
	baseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	if language == "" || language == "auto" {
		language = "auto"
	}
	return filepath.Join(cacheDir, fmt.Sprintf("%s_ai_%s.vtt", baseName, language))
}

// GetVTTPath 获取已生成的 AI 字幕文件路径（供 handler 使用）
func (s *ASRService) GetVTTPath(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", fmt.Errorf("媒体不存在: %w", err)
	}

	// 查找所有可能的 AI 字幕文件
	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles", "ai")
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	pattern := filepath.Join(cacheDir, fmt.Sprintf("%s_ai_*.vtt", baseName))

	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", fmt.Errorf("未找到 AI 字幕文件")
}

// DeleteSubtitle 删除已生成的 AI 字幕
func (s *ASRService) DeleteSubtitle(mediaID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return fmt.Errorf("媒体不存在: %w", err)
	}

	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles", "ai")
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	pattern := filepath.Join(cacheDir, fmt.Sprintf("%s_ai_*.vtt", baseName))

	matches, _ := filepath.Glob(pattern)
	for _, m := range matches {
		os.Remove(m)
	}

	// 清理任务状态
	s.tasksMu.Lock()
	delete(s.tasks, mediaID)
	s.tasksMu.Unlock()

	return nil
}

// updateTask 更新任务状态并广播
func (s *ASRService) updateTask(mediaID string, status string, progress float64, message string) {
	s.tasksMu.Lock()
	if task, ok := s.tasks[mediaID]; ok {
		task.Status = status
		task.Progress = progress
		task.Message = message
	}
	s.tasksMu.Unlock()

	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventASRProgress, ASRProgressData{
			MediaID:  mediaID,
			Status:   status,
			Progress: progress,
			Message:  message,
		})
	}
}

// failTask 标记任务失败并广播
func (s *ASRService) failTask(mediaID string, errMsg string) {
	s.tasksMu.Lock()
	if task, ok := s.tasks[mediaID]; ok {
		task.Status = "failed"
		task.Error = errMsg
		task.Message = errMsg
	}
	s.tasksMu.Unlock()

	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventASRFailed, ASRProgressData{
			MediaID:  mediaID,
			Status:   "failed",
			Progress: 0,
			Message:  errMsg,
			Error:    errMsg,
		})
	}

	s.logger.Warnf("ASR 任务失败: mediaID=%s, error=%s", mediaID, errMsg)
}

// formatVTTTime 格式化 VTT 时间戳 (HH:MM:SS.mmm)
func formatVTTTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	sec := int(seconds) % 60
	ms := int((seconds - float64(int(seconds))) * 1000)
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, sec, ms)
}

// ==================== Phase 2: 本地 whisper.cpp 引擎 ====================

// callWhisperLocal 使用本地 whisper.cpp 进行语音识别
func (s *ASRService) callWhisperLocal(audioPath string, language string) (*WhisperResponse, error) {
	if s.cfg.WhisperCppPath == "" || s.cfg.WhisperModelPath == "" {
		return nil, fmt.Errorf("本地 whisper.cpp 未配置")
	}

	// 输出 JSON 格式到临时文件
	cacheDir := filepath.Dir(audioPath)
	outputPath := filepath.Join(cacheDir, strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath)))

	// 构建 whisper.cpp 命令参数
	args := []string{
		"-m", s.cfg.WhisperModelPath,
		"-f", audioPath,
		"-oj",             // 输出 JSON 格式
		"-of", outputPath, // 输出文件前缀
		"-t", fmt.Sprintf("%d", s.cfg.WhisperThreads), // 线程数
		"-pp", // 打印进度
	}

	// 如果指定了语言
	if language != "" && language != "auto" {
		args = append(args, "-l", language)
	} else {
		args = append(args, "-l", "auto")
	}

	s.logger.Infof("调用本地 whisper.cpp: %s %v", s.cfg.WhisperCppPath, args)

	cmd := exec.Command(s.cfg.WhisperCppPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("whisper.cpp 执行失败: %w, stderr: %s", err, stderr.String())
	}

	// 读取 JSON 输出
	jsonPath := outputPath + ".json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("读取 whisper.cpp 输出失败: %w", err)
	}
	defer os.Remove(jsonPath)

	// 解析 whisper.cpp 的 JSON 格式
	var whisperOutput struct {
		SystemInfo string `json:"systeminfo"`
		Model      struct {
			Type string `json:"type"`
		} `json:"model"`
		Params struct {
			Language string `json:"language"`
		} `json:"params"`
		Result struct {
			Language string `json:"language"`
		} `json:"result"`
		Transcription []struct {
			Timestamps struct {
				From string `json:"from"`
				To   string `json:"to"`
			} `json:"timestamps"`
			Offsets struct {
				From int `json:"from"`
				To   int `json:"to"`
			} `json:"offsets"`
			Text string `json:"text"`
		} `json:"transcription"`
	}

	if err := json.Unmarshal(jsonData, &whisperOutput); err != nil {
		return nil, fmt.Errorf("解析 whisper.cpp JSON 失败: %w", err)
	}

	// 转换为统一的 WhisperResponse 格式
	var segments []WhisperSegment
	var allText strings.Builder
	var maxEnd float64

	for i, t := range whisperOutput.Transcription {
		startSec := float64(t.Offsets.From) / 1000.0
		endSec := float64(t.Offsets.To) / 1000.0
		text := strings.TrimSpace(t.Text)
		if text == "" {
			continue
		}

		segments = append(segments, WhisperSegment{
			ID:    i,
			Start: startSec,
			End:   endSec,
			Text:  text,
		})
		allText.WriteString(text)
		allText.WriteString(" ")

		if endSec > maxEnd {
			maxEnd = endSec
		}
	}

	detectedLang := whisperOutput.Result.Language
	if detectedLang == "" {
		detectedLang = whisperOutput.Params.Language
	}
	if detectedLang == "" {
		detectedLang = "auto"
	}

	s.logger.Infof("本地 whisper.cpp 识别完成: language=%s, segments=%d", detectedLang, len(segments))

	return &WhisperResponse{
		Task:     "transcribe",
		Language: detectedLang,
		Duration: maxEnd,
		Text:     strings.TrimSpace(allText.String()),
		Segments: segments,
	}, nil
}

// ==================== Phase 3: STRM 远程流音频提取 ====================

// extractAudioFromRemote 从远程流 URL 提取音频
func (s *ASRService) extractAudioFromRemote(remoteURL string, mediaID string) (string, error) {
	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "asr_temp")
	os.MkdirAll(cacheDir, 0755)

	audioPath := filepath.Join(cacheDir, fmt.Sprintf("%s_remote_audio.wav", mediaID))

	// 使用 FFmpeg 直接从远程 URL 提取音频
	// FFmpeg 原生支持 HTTP/HTTPS 远程流输入
	cmd := exec.Command(s.appCfg.App.FFmpegPath,
		"-y",
		"-i", remoteURL, // 远程流 URL 作为输入
		"-vn",                  // 不要视频
		"-acodec", "pcm_s16le", // PCM 16-bit 编码
		"-ar", "16000", // 16kHz 采样率
		"-ac", "1", // 单声道
		"-f", "wav", // WAV 格式
		"-t", "7200", // 最多提取 2 小时（安全限制）
		"-user_agent", "nowen-video/1.0", // 设置 User-Agent
		audioPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	s.logger.Infof("从远程流提取音频: %s", remoteURL)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("从远程流提取音频失败: %w, stderr: %s", err, stderr.String())
	}

	// 检查文件
	fi, err := os.Stat(audioPath)
	if err != nil {
		return "", fmt.Errorf("远程流音频文件不存在: %w", err)
	}

	s.logger.Infof("远程流音频提取完成: %s (%.1f MB)", audioPath, float64(fi.Size())/(1024*1024))

	return audioPath, nil
}

// ==================== Phase 4: 字幕翻译 ====================

// TranslateSubtitle 将已有字幕翻译为目标语言
func (s *ASRService) TranslateSubtitle(mediaID string, targetLang string) (*ASRTask, error) {
	if s.aiService == nil || !s.aiService.IsEnabled() {
		return nil, fmt.Errorf("AI 服务未启用，无法进行字幕翻译")
	}

	if targetLang == "" {
		return nil, fmt.Errorf("请指定目标翻译语言")
	}

	// 检查是否已有进行中的翻译任务
	taskKey := mediaID + "_translate_" + targetLang
	s.tasksMu.RLock()
	existing := s.tasks[taskKey]
	s.tasksMu.RUnlock()
	if existing != nil && (existing.Status == "translating") {
		return existing, nil
	}

	// 查找源字幕文件（优先 AI 生成的，其次外挂字幕）
	srcVTTPath, err := s.findSourceSubtitle(mediaID)
	if err != nil {
		return nil, fmt.Errorf("未找到可翻译的源字幕: %w", err)
	}

	// 检查是否已有翻译缓存
	translatedPath := s.getTranslatedVTTPath(srcVTTPath, targetLang)
	if _, err := os.Stat(translatedPath); err == nil {
		task := &ASRTask{
			MediaID:   mediaID,
			Status:    "completed",
			Progress:  100,
			Message:   fmt.Sprintf("翻译字幕已存在（%s，缓存）", targetLang),
			Language:  targetLang,
			VTTPath:   translatedPath,
			CreatedAt: time.Now(),
		}
		return task, nil
	}

	// 创建翻译任务
	task := &ASRTask{
		MediaID:   mediaID,
		Status:    "pending",
		Progress:  0,
		Message:   fmt.Sprintf("等待翻译为 %s...", targetLang),
		Language:  targetLang,
		CreatedAt: time.Now(),
	}

	s.tasksMu.Lock()
	s.tasks[taskKey] = task
	s.tasksMu.Unlock()

	// 异步执行翻译
	go s.processTranslateTask(srcVTTPath, mediaID, targetLang, taskKey)

	return task, nil
}

// processTranslateTask 处理字幕翻译任务
func (s *ASRService) processTranslateTask(srcVTTPath string, mediaID string, targetLang string, taskKey string) {
	s.semaphore <- struct{}{}
	defer func() { <-s.semaphore }()

	s.updateTaskByKey(taskKey, mediaID, "translating", 10, fmt.Sprintf("正在读取源字幕...（目标: %s）", targetLang))

	// 读取源 VTT 文件
	srcContent, err := os.ReadFile(srcVTTPath)
	if err != nil {
		s.failTaskByKey(taskKey, mediaID, fmt.Sprintf("读取源字幕失败: %v", err))
		return
	}

	// 解析 VTT 内容，提取文本行
	cues := parseVTTCues(string(srcContent))
	if len(cues) == 0 {
		s.failTaskByKey(taskKey, mediaID, "源字幕为空，无法翻译")
		return
	}

	s.updateTaskByKey(taskKey, mediaID, "translating", 20, fmt.Sprintf("正在翻译 %d 条字幕为 %s...", len(cues), targetLang))

	// 分批翻译（每批 30 条，避免超出 token 限制）
	batchSize := 30
	translatedCues := make([]vttCue, len(cues))
	copy(translatedCues, cues)

	for i := 0; i < len(cues); i += batchSize {
		end := i + batchSize
		if end > len(cues) {
			end = len(cues)
		}

		batch := cues[i:end]
		progress := 20 + float64(i)/float64(len(cues))*70
		s.updateTaskByKey(taskKey, mediaID, "translating", progress,
			fmt.Sprintf("正在翻译 %d/%d 条字幕...", i+len(batch), len(cues)))

		// 构建翻译请求
		var textLines []string
		for _, cue := range batch {
			textLines = append(textLines, cue.text)
		}

		translated, err := s.translateBatch(textLines, targetLang)
		if err != nil {
			s.logger.Warnf("翻译批次 %d-%d 失败: %v", i, end, err)
			// 翻译失败的保留原文
			continue
		}

		// 更新翻译结果
		for j, text := range translated {
			if i+j < len(translatedCues) {
				translatedCues[i+j].text = text
			}
		}
	}

	s.updateTaskByKey(taskKey, mediaID, "converting", 92, "正在生成翻译字幕文件...")

	// 生成翻译后的 VTT 文件
	translatedPath := s.getTranslatedVTTPath(srcVTTPath, targetLang)
	os.MkdirAll(filepath.Dir(translatedPath), 0755)

	var buf strings.Builder
	buf.WriteString("WEBVTT\n")
	buf.WriteString(fmt.Sprintf("Kind: captions\n"))
	buf.WriteString(fmt.Sprintf("Language: %s\n", targetLang))
	buf.WriteString(fmt.Sprintf("X-Translated-From: %s\n", filepath.Base(srcVTTPath)))
	buf.WriteString("\n")

	for i, cue := range translatedCues {
		if cue.text == "" {
			continue
		}
		buf.WriteString(fmt.Sprintf("%d\n", i+1))
		buf.WriteString(fmt.Sprintf("%s --> %s\n", cue.startTime, cue.endTime))
		buf.WriteString(cue.text)
		buf.WriteString("\n\n")
	}

	if err := os.WriteFile(translatedPath, []byte(buf.String()), 0644); err != nil {
		s.failTaskByKey(taskKey, mediaID, fmt.Sprintf("写入翻译字幕失败: %v", err))
		return
	}

	// 完成
	completionMsg := fmt.Sprintf("字幕翻译完成（%s，%d 条）", targetLang, len(translatedCues))
	s.tasksMu.Lock()
	if task, ok := s.tasks[taskKey]; ok {
		task.Status = "completed"
		task.Progress = 100
		task.Message = completionMsg
		task.VTTPath = translatedPath
	}
	s.tasksMu.Unlock()

	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventTranslateCompleted, ASRProgressData{
			MediaID:  mediaID,
			Status:   "completed",
			Progress: 100,
			Message:  completionMsg,
			VTTPath:  translatedPath,
		})
	}

	s.logger.Infof("字幕翻译完成: mediaID=%s, targetLang=%s, cues=%d", mediaID, targetLang, len(translatedCues))
}

// translateBatch 批量翻译文本
func (s *ASRService) translateBatch(texts []string, targetLang string) ([]string, error) {
	// 构建翻译 prompt
	numberedText := ""
	for i, text := range texts {
		numberedText += fmt.Sprintf("%d|%s\n", i+1, text)
	}

	langNames := map[string]string{
		"zh": "中文", "en": "英文", "ja": "日文", "ko": "韩文",
		"fr": "法文", "de": "德文", "es": "西班牙文", "pt": "葡萄牙文",
		"ru": "俄文", "it": "意大利文", "ar": "阿拉伯文", "th": "泰文",
	}
	langName := langNames[targetLang]
	if langName == "" {
		langName = targetLang
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的字幕翻译器。请将以下字幕文本翻译为%s。

规则：
1. 每行格式为 "序号|原文"，请保持相同格式输出 "序号|译文"
2. 保持序号不变
3. 翻译要自然流畅，符合字幕风格（简洁、口语化）
4. 不要添加任何解释或额外内容
5. 如果原文已经是目标语言，直接保留原文`, langName)

	result, err := s.aiService.ChatCompletion(systemPrompt, numberedText, 0.3, 4096)
	if err != nil {
		return nil, fmt.Errorf("AI 翻译失败: %w", err)
	}

	// 解析翻译结果
	translated := make([]string, len(texts))
	copy(translated, texts) // 默认保留原文

	lines := strings.Split(strings.TrimSpace(result), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		var idx int
		if _, err := fmt.Sscanf(parts[0], "%d", &idx); err != nil {
			continue
		}
		if idx >= 1 && idx <= len(translated) {
			translated[idx-1] = strings.TrimSpace(parts[1])
		}
	}

	return translated, nil
}

// vttCue VTT 字幕条目
type vttCue struct {
	startTime string
	endTime   string
	text      string
}

// parseVTTCues 解析 VTT 文件中的字幕条目
func parseVTTCues(content string) []vttCue {
	var cues []vttCue
	lines := strings.Split(content, "\n")

	i := 0
	// 跳过 WEBVTT 头部
	for i < len(lines) {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			break
		}
		i++
	}

	for i < len(lines) {
		line := strings.TrimSpace(lines[i])
		i++

		// 跳过空行和序号行
		if line == "" {
			continue
		}

		// 查找时间戳行
		if !strings.Contains(line, "-->") {
			// 可能是序号行，继续找时间戳
			if i < len(lines) && strings.Contains(lines[i], "-->") {
				line = strings.TrimSpace(lines[i])
				i++
			} else {
				continue
			}
		}

		// 解析时间戳
		parts := strings.Split(line, "-->")
		if len(parts) != 2 {
			continue
		}

		startTime := strings.TrimSpace(parts[0])
		endTime := strings.TrimSpace(parts[1])

		// 收集文本行
		var textLines []string
		for i < len(lines) {
			textLine := strings.TrimSpace(lines[i])
			if textLine == "" {
				i++
				break
			}
			textLines = append(textLines, textLine)
			i++
		}

		if len(textLines) > 0 {
			cues = append(cues, vttCue{
				startTime: startTime,
				endTime:   endTime,
				text:      strings.Join(textLines, "\n"),
			})
		}
	}

	return cues
}

// findSourceSubtitle 查找可用的源字幕文件（优先 AI 生成的）
func (s *ASRService) findSourceSubtitle(mediaID string) (string, error) {
	// 优先查找 AI 生成的字幕
	vttPath, err := s.GetVTTPath(mediaID)
	if err == nil && vttPath != "" {
		return vttPath, nil
	}

	// 查找外挂字幕（VTT 格式）
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", fmt.Errorf("媒体不存在: %w", err)
	}

	dir := filepath.Dir(media.FilePath)
	base := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))

	// 查找同名 VTT/SRT 文件
	for _, ext := range []string{".vtt", ".srt"} {
		candidate := filepath.Join(dir, base+ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// 查找缓存目录中的字幕
	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles")
	pattern := filepath.Join(cacheDir, "**", base+"*.vtt")
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", fmt.Errorf("未找到可翻译的源字幕")
}

// getTranslatedVTTPath 获取翻译字幕的缓存路径
func (s *ASRService) getTranslatedVTTPath(srcVTTPath string, targetLang string) string {
	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles", "translated")
	baseName := strings.TrimSuffix(filepath.Base(srcVTTPath), filepath.Ext(srcVTTPath))
	return filepath.Join(cacheDir, fmt.Sprintf("%s_%s.vtt", baseName, targetLang))
}

// GetTranslateTask 获取翻译任务状态
func (s *ASRService) GetTranslateTask(mediaID string, targetLang string) *ASRTask {
	taskKey := mediaID + "_translate_" + targetLang
	s.tasksMu.RLock()
	defer s.tasksMu.RUnlock()
	return s.tasks[taskKey]
}

// GetTranslatedVTTPath 获取已翻译的字幕文件路径
func (s *ASRService) GetTranslatedVTTPath(mediaID string, targetLang string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", fmt.Errorf("媒体不存在: %w", err)
	}

	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles", "translated")
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	pattern := filepath.Join(cacheDir, fmt.Sprintf("%s*_%s.vtt", baseName, targetLang))

	matches, _ := filepath.Glob(pattern)
	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", fmt.Errorf("未找到翻译字幕")
}

// ListTranslatedSubtitles 列出某媒体所有已翻译的字幕
func (s *ASRService) ListTranslatedSubtitles(mediaID string) ([]map[string]string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("媒体不存在: %w", err)
	}

	cacheDir := filepath.Join(s.appCfg.Cache.CacheDir, "subtitles", "translated")
	baseName := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))
	pattern := filepath.Join(cacheDir, fmt.Sprintf("%s*.vtt", baseName))

	matches, _ := filepath.Glob(pattern)
	var results []map[string]string
	for _, m := range matches {
		name := strings.TrimSuffix(filepath.Base(m), ".vtt")
		// 提取语言代码（最后一个下划线后的部分）
		parts := strings.Split(name, "_")
		if len(parts) >= 2 {
			lang := parts[len(parts)-1]
			results = append(results, map[string]string{
				"language": lang,
				"path":     m,
			})
		}
	}

	return results, nil
}

// updateTaskByKey 按 taskKey 更新任务状态并广播
func (s *ASRService) updateTaskByKey(taskKey string, mediaID string, status string, progress float64, message string) {
	s.tasksMu.Lock()
	if task, ok := s.tasks[taskKey]; ok {
		task.Status = status
		task.Progress = progress
		task.Message = message
	}
	s.tasksMu.Unlock()

	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventTranslateProgress, ASRProgressData{
			MediaID:  mediaID,
			Status:   status,
			Progress: progress,
			Message:  message,
		})
	}
}

// failTaskByKey 按 taskKey 标记任务失败并广播
func (s *ASRService) failTaskByKey(taskKey string, mediaID string, errMsg string) {
	s.tasksMu.Lock()
	if task, ok := s.tasks[taskKey]; ok {
		task.Status = "failed"
		task.Error = errMsg
		task.Message = errMsg
	}
	s.tasksMu.Unlock()

	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventTranslateFailed, ASRProgressData{
			MediaID:  mediaID,
			Status:   "failed",
			Progress: 0,
			Message:  errMsg,
			Error:    errMsg,
		})
	}

	s.logger.Warnf("翻译任务失败: mediaID=%s, error=%s", mediaID, errMsg)
}

// GetStatus 获取 ASR 服务状态
func (s *ASRService) GetStatus() map[string]interface{} {
	s.tasksMu.RLock()
	activeTasks := 0
	for _, task := range s.tasks {
		if task.Status == "extracting" || task.Status == "transcribing" || task.Status == "converting" || task.Status == "translating" {
			activeTasks++
		}
	}
	totalTasks := len(s.tasks)
	s.tasksMu.RUnlock()

	return map[string]interface{}{
		"enabled":            s.IsEnabled(),
		"cloud_enabled":      s.IsCloudEnabled(),
		"local_enabled":      s.IsLocalEnabled(),
		"prefer_local":       s.cfg.PreferLocal,
		"model":              s.cfg.Model,
		"whisper_cpp_path":   s.cfg.WhisperCppPath,
		"whisper_model_path": s.cfg.WhisperModelPath,
		"max_concurrent":     s.cfg.MaxConcurrent,
		"active_tasks":       activeTasks,
		"total_tasks":        totalTasks,
		"translate_enabled":  s.aiService != nil && s.aiService.IsEnabled(),
	}
}
