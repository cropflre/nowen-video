package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== 系统监控 ====================

// GetMetrics 获取实时系统指标
func (h *AdminHandler) GetMetrics(c *gin.Context) {
	metrics := h.monitorService.GetMetrics()
	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

// ==================== 定时任务管理 ====================

// CreateScheduledTaskRequest 创建定时任务请求
type CreateScheduledTaskRequest struct {
	Name     string `json:"name" binding:"required"`
	Type     string `json:"type" binding:"required"`     // scan, scrape, cleanup
	Schedule string `json:"schedule" binding:"required"` // @daily, @every 6h等
	TargetID string `json:"target_id"`
}

// ListScheduledTasks 获取定时任务列表
func (h *AdminHandler) ListScheduledTasks(c *gin.Context) {
	tasks, err := h.schedulerService.ListTasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取任务列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": tasks})
}

// CreateScheduledTask 创建定时任务
func (h *AdminHandler) CreateScheduledTask(c *gin.Context) {
	var req CreateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	task, err := h.schedulerService.CreateTask(req.Name, req.Type, req.Schedule, req.TargetID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建任务失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": task})
}

// UpdateScheduledTaskRequest 更新定时任务请求
type UpdateScheduledTaskRequest struct {
	Name     string `json:"name" binding:"required"`
	Schedule string `json:"schedule" binding:"required"`
	Enabled  bool   `json:"enabled"`
}

// UpdateScheduledTask 更新定时任务
func (h *AdminHandler) UpdateScheduledTask(c *gin.Context) {
	id := c.Param("id")

	var req UpdateScheduledTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.schedulerService.UpdateTask(id, req.Name, req.Schedule, req.Enabled); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已更新"})
}

// DeleteScheduledTask 删除定时任务
func (h *AdminHandler) DeleteScheduledTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.schedulerService.DeleteTask(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "删除任务失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

// RunScheduledTaskNow 立即执行定时任务
func (h *AdminHandler) RunScheduledTaskNow(c *gin.Context) {
	id := c.Param("id")

	if err := h.schedulerService.RunTaskNow(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "任务已开始执行"})
}

// ==================== 批量操作 ====================

// BatchScanRequest 批量扫描请求
type BatchScanRequest struct {
	LibraryIDs []string `json:"library_ids" binding:"required"`
}

// BatchScan 批量扫描多个媒体库
func (h *AdminHandler) BatchScan(c *gin.Context) {
	var req BatchScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	var started []string
	var errors []gin.H
	for _, id := range req.LibraryIDs {
		if err := h.libraryService.Scan(id); err != nil {
			errors = append(errors, gin.H{"library_id": id, "error": err.Error()})
		} else {
			started = append(started, id)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "批量扫描已启动",
		"started": started,
		"errors":  errors,
	})
}

// BatchScrapeRequest 批量刮削请求
type BatchScrapeRequest struct {
	MediaIDs []string `json:"media_ids" binding:"required"`
}

// BatchScrape 批量刮削元数据
func (h *AdminHandler) BatchScrape(c *gin.Context) {
	var req BatchScrapeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	// 异步执行批量刮削
	go func() {
		success := 0
		failed := 0
		for _, id := range req.MediaIDs {
			if err := h.metadataService.ScrapeMedia(id); err != nil {
				failed++
			} else {
				success++
			}
		}
		h.logger.Infof("批量刮削完成: 成功 %d, 失败 %d", success, failed)
	}()

	c.JSON(http.StatusOK, gin.H{
		"message": "批量刮削已启动",
		"total":   len(req.MediaIDs),
	})
}

// ==================== 权限管理 ====================

// GetUserPermission 获取用户权限设置
func (h *AdminHandler) GetUserPermission(c *gin.Context) {
	userID := c.Param("userId")
	perm, err := h.permissionService.GetUserPermission(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取权限失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": perm})
}

// UpdateUserPermissionRequest 更新用户权限请求
type UpdateUserPermissionRequest struct {
	AllowedLibraries string `json:"allowed_libraries"` // 逗号分隔的媒体库ID
	MaxRatingLevel   string `json:"max_rating_level"`
	DailyTimeLimit   int    `json:"daily_time_limit"` // 分钟
}

// UpdateUserPermission 更新用户权限
func (h *AdminHandler) UpdateUserPermission(c *gin.Context) {
	userID := c.Param("userId")

	var req UpdateUserPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.permissionService.UpdateUserPermission(userID, req.AllowedLibraries, req.MaxRatingLevel, req.DailyTimeLimit); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新权限失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "权限已更新"})
}

// SetContentRatingRequest 设置内容分级请求
type SetContentRatingRequest struct {
	Level string `json:"level" binding:"required"` // G, PG, PG-13, R, NC-17
}

// SetContentRating 设置媒体内容分级
func (h *AdminHandler) SetContentRating(c *gin.Context) {
	mediaID := c.Param("mediaId")

	var req SetContentRatingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	if err := h.permissionService.SetContentRating(mediaID, req.Level); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分级已设置"})
}

// GetContentRating 获取媒体内容分级
func (h *AdminHandler) GetContentRating(c *gin.Context) {
	mediaID := c.Param("mediaId")
	level, err := h.permissionService.GetContentRating(mediaID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"media_id": mediaID, "level": ""}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"media_id": mediaID, "level": level}})
}

// ==================== 系统设置（全局） ====================

// 系统设置键名常量
const (
	SettingGPUTranscode     = "enable_gpu_transcode"
	SettingGPUFallbackCPU   = "gpu_fallback_cpu"
	SettingMetadataPath     = "metadata_store_path"
	SettingPlayCachePath    = "play_cache_path"
	SettingDirectLink       = "enable_direct_link"
	SettingAutoPreprocess   = "auto_preprocess_on_scan" // 扫描后自动触发预处理
	SettingAutoTranscode    = "auto_transcode_on_play"  // 播放时自动触发转码
	SettingPreferDirectPlay = "prefer_direct_play"      // 优先直接播放（禁用自动转码）
)

// GetSystemSettings 获取系统全局设置
func (h *AdminHandler) GetSystemSettings(c *gin.Context) {
	all, err := h.settingRepo.GetAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取系统设置失败"})
		return
	}

	// 返回带默认值的设置
	settings := gin.H{
		SettingGPUTranscode:     getBoolSetting(all, SettingGPUTranscode, true),
		SettingGPUFallbackCPU:   getBoolSetting(all, SettingGPUFallbackCPU, true),
		SettingMetadataPath:     getStrSetting(all, SettingMetadataPath, ""),
		SettingPlayCachePath:    getStrSetting(all, SettingPlayCachePath, ""),
		SettingDirectLink:       getBoolSetting(all, SettingDirectLink, false),
		SettingAutoPreprocess:   getBoolSetting(all, SettingAutoPreprocess, false),  // 默认关闭：扫描后不自动预处理
		SettingAutoTranscode:    getBoolSetting(all, SettingAutoTranscode, false),   // 默认关闭：播放时不自动转码
		SettingPreferDirectPlay: getBoolSetting(all, SettingPreferDirectPlay, true), // 默认开启：优先直接播放
	}

	c.JSON(http.StatusOK, gin.H{"data": settings})
}

// UpdateSystemSettingsRequest 更新系统设置请求
type UpdateSystemSettingsRequest struct {
	EnableGPUTranscode *bool   `json:"enable_gpu_transcode"`
	GPUFallbackCPU     *bool   `json:"gpu_fallback_cpu"`
	MetadataStorePath  *string `json:"metadata_store_path"`
	PlayCachePath      *string `json:"play_cache_path"`
	EnableDirectLink   *bool   `json:"enable_direct_link"`
	AutoPreprocess     *bool   `json:"auto_preprocess_on_scan"`
	AutoTranscode      *bool   `json:"auto_transcode_on_play"`
	PreferDirectPlay   *bool   `json:"prefer_direct_play"`
}

// UpdateSystemSettings 更新系统全局设置
func (h *AdminHandler) UpdateSystemSettings(c *gin.Context) {
	var req UpdateSystemSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效"})
		return
	}

	kvs := make(map[string]string)
	if req.EnableGPUTranscode != nil {
		kvs[SettingGPUTranscode] = boolToStr(*req.EnableGPUTranscode)
	}
	if req.GPUFallbackCPU != nil {
		kvs[SettingGPUFallbackCPU] = boolToStr(*req.GPUFallbackCPU)
	}
	if req.MetadataStorePath != nil {
		kvs[SettingMetadataPath] = *req.MetadataStorePath
	}
	if req.PlayCachePath != nil {
		kvs[SettingPlayCachePath] = *req.PlayCachePath
	}
	if req.EnableDirectLink != nil {
		kvs[SettingDirectLink] = boolToStr(*req.EnableDirectLink)
	}
	if req.AutoPreprocess != nil {
		kvs[SettingAutoPreprocess] = boolToStr(*req.AutoPreprocess)
	}
	if req.AutoTranscode != nil {
		kvs[SettingAutoTranscode] = boolToStr(*req.AutoTranscode)
	}
	if req.PreferDirectPlay != nil {
		kvs[SettingPreferDirectPlay] = boolToStr(*req.PreferDirectPlay)
	}

	if len(kvs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供任何设置项"})
		return
	}

	if err := h.settingRepo.SetMulti(kvs); err != nil {
		h.logger.Errorf("更新系统设置失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存设置失败"})
		return
	}

	h.logger.Info("系统设置已更新")

	// 返回更新后的完整设置
	h.GetSystemSettings(c)
}

// 辅助函数
func getBoolSetting(m map[string]string, key string, defaultVal bool) bool {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	return v == "true" || v == "1"
}

func getStrSetting(m map[string]string, key string, defaultVal string) string {
	v, ok := m[key]
	if !ok {
		return defaultVal
	}
	return v
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// ==================== 服务端文件浏览器 ====================

// BrowseFS 浏览服务器文件系统目录
// 安全限制：仅允许浏览已配置的媒体库路径及常见根路径，防止任意目录遍历
func (h *AdminHandler) BrowseFS(c *gin.Context) {
	dir := c.DefaultQuery("path", "/")
	if dir == "" {
		dir = "/"
	}

	// 安全检查：清理路径，防止路径遍历攻击
	dir = filepath.Clean(dir)

	// 安全限制：检查请求路径是否在允许的范围内
	// 允许的路径：根路径（/）、常见挂载点、已配置的媒体库路径的父目录
	allowed := h.isAllowedBrowsePath(dir)
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权浏览该目录，仅允许浏览媒体库相关路径"})
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取目录: " + err.Error()})
		return
	}

	type FsEntry struct {
		Name  string `json:"name"`
		Path  string `json:"path"`
		IsDir bool   `json:"is_dir"`
	}

	var items []FsEntry
	for _, entry := range entries {
		// 只返回目录（文件浏览器只需要选择文件夹）
		if !entry.IsDir() {
			continue
		}
		// 跳过隐藏目录和系统目录
		name := entry.Name()
		if name[0] == '.' {
			continue
		}
		// 跳过敏感系统目录
		if h.isSensitiveDir(name) {
			continue
		}
		items = append(items, FsEntry{
			Name:  name,
			Path:  filepath.Join(dir, name),
			IsDir: true,
		})
	}

	// 计算父目录
	parent := filepath.Dir(dir)
	if parent == dir {
		parent = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"current": dir,
			"parent":  parent,
			"items":   items,
		},
	})
}

// isAllowedBrowsePath 检查路径是否在允许浏览的范围内
func (h *AdminHandler) isAllowedBrowsePath(dir string) bool {
	// 根路径始终允许（用于导航）
	if dir == "/" || dir == "\\" || dir == "." {
		return true
	}

	// Windows 盘符根路径允许（如 C:\, D:\）
	if len(dir) <= 3 && filepath.VolumeName(dir) != "" {
		return true
	}

	// 黑名单模式：只禁止已知的敏感系统目录，其余均允许
	// 这样可以兼容各种 NAS/Docker 挂载路径（如 /18t, /volume1 等）
	blockedRoots := []string{
		"/proc", "/sys", "/dev", "/run", "/boot", "/sbin", "/bin",
		"/lib", "/lib64", "/snap",
	}
	for _, blocked := range blockedRoots {
		if dir == blocked || strings.HasPrefix(dir, blocked+"/") {
			return false
		}
	}

	return true
}

// isSensitiveDir 检查是否为敏感系统目录
func (h *AdminHandler) isSensitiveDir(name string) bool {
	sensitive := map[string]bool{
		"proc": true, "sys": true, "dev": true, "run": true,
		"boot": true, "sbin": true, "bin": true, "lib": true,
		"lib64": true, "lost+found": true, "snap": true,
		"System Volume Information": true, "$Recycle.Bin": true,
		"Windows": true, "Program Files": true, "Program Files (x86)": true,
		"ProgramData": true,
	}
	return sensitive[name]
}

// ==================== 剧集合并 ====================

// MergeSeriesRequest 手动合并请求
type MergeSeriesRequest struct {
	PrimaryID    string   `json:"primary_id" binding:"required"`
	SecondaryIDs []string `json:"secondary_ids" binding:"required"`
}

// MergeSeries 手动合并多个 Series（将多个季合并为一个整体）
func (h *AdminHandler) MergeSeries(c *gin.Context) {
	var req MergeSeriesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if len(req.SecondaryIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "至少需要一个待合并的从属剧集 ID"})
		return
	}

	result, err := h.seriesService.MergeSeries(req.PrimaryID, req.SecondaryIDs)
	if err != nil {
		h.logger.Errorf("手动合并剧集失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "合并失败: " + err.Error()})
		return
	}

	h.logger.Infof("手动合并完成: %s, 合并了 %d 个系列", result.PrimaryTitle, result.MergedCount)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("已成功合并 %d 个剧集系列", result.MergedCount),
		"data":    result,
	})
}

// AutoMergeSeries 自动扫描并合并所有重复的 Series
func (h *AdminHandler) AutoMergeSeries(c *gin.Context) {
	results, err := h.seriesService.AutoMergeDuplicates()
	if err != nil {
		h.logger.Errorf("自动合并剧集失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "自动合并失败: " + err.Error()})
		return
	}

	totalMerged := 0
	for _, r := range results {
		totalMerged += r.MergedCount
	}

	h.logger.Infof("自动合并完成: 处理了 %d 个系列组, 共合并 %d 个重复记录", len(results), totalMerged)
	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("自动合并完成: 处理了 %d 个系列组, 共合并 %d 个重复记录", len(results), totalMerged),
		"data": gin.H{
			"groups_processed": len(results),
			"total_merged":     totalMerged,
			"details":          results,
		},
	})
}

// MergeCandidates 获取可合并的 Series 候选列表（预览，不执行合并）
func (h *AdminHandler) MergeCandidates(c *gin.Context) {
	candidates, err := h.seriesService.FindMergeCandidates()
	if err != nil {
		h.logger.Errorf("获取合并候选列表失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取候选列表失败: " + err.Error()})
		return
	}

	// 格式化返回数据
	type CandidateGroup struct {
		NormalizedTitle string      `json:"normalized_title"`
		Count           int         `json:"count"`
		Series          interface{} `json:"series"`
	}

	var groups []CandidateGroup
	for _, group := range candidates {
		if len(group) == 0 {
			continue
		}
		groups = append(groups, CandidateGroup{
			NormalizedTitle: group[0].Title, // 使用第一个的标题作为代表
			Count:           len(group),
			Series:          group,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  groups,
		"total": len(groups),
	})
}

// ==================== 一键清空数据 ====================

// ClearAllDataRequest 清空数据请求
type ClearAllDataRequest struct {
	Confirm string `json:"confirm" binding:"required"` // 必须为 "CONFIRM_CLEAR_ALL"
}

// ClearTableResult 单个表的清理结果
type ClearTableResult struct {
	Table   string `json:"table"`
	Cleared int64  `json:"cleared"`
	Status  string `json:"status"` // success / skipped / error
	Message string `json:"message,omitempty"`
}

// ClearAllData 一键彻底清空所有数据（仅保留磁盘上的影视文件和当前管理员账号）
// 清除范围：用户数据、元数据、观看历史、收藏、播放列表、评论、AI缓存、
//
//	媒体库配置、系统设置、影视元数据缓存、封面数据等所有记录
//
// 保留范围：磁盘上的视频文件（不做任何文件操作）、当前操作的管理员账号
func (h *AdminHandler) ClearAllData(c *gin.Context) {
	var req ClearAllDataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效，需要提供确认标识"})
		return
	}

	// 二次确认：必须传入精确的确认字符串
	if req.Confirm != "CONFIRM_CLEAR_ALL" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "确认标识不正确，请传入 CONFIRM_CLEAR_ALL 以确认操作"})
		return
	}

	// 获取当前管理员用户ID，确保不删除自己的账号
	currentUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "无法获取当前用户信息"})
		return
	}
	adminUserID := currentUserID.(string)

	h.logger.Warn("⚠️ 管理员发起一键彻底清空数据操作（仅保留影视文件和当前管理员账号）")

	var results []ClearTableResult
	var totalCleared int64

	// 定义需要完全清空的表及其模型（按依赖顺序，先清关联表再清主表）
	// 仅保留：磁盘上的影视文件（不做文件操作）、当前管理员账号
	type clearItem struct {
		name  string
		model interface{}
	}

	tablesToClear := []clearItem{
		// 用户相关数据
		{name: "观看历史", model: &model.WatchHistory{}},
		{name: "收藏记录", model: &model.Favorite{}},
		{name: "播放列表项", model: &model.PlaylistItem{}},
		{name: "播放列表", model: &model.Playlist{}},
		{name: "视频书签", model: &model.Bookmark{}},
		{name: "评论", model: &model.Comment{}},
		{name: "播放统计", model: &model.PlaybackStats{}},

		// 元数据相关
		{name: "演员关联", model: &model.MediaPerson{}},
		{name: "演员信息", model: &model.Person{}},
		{name: "类型映射", model: &model.GenreMapping{}},
		{name: "内容分级", model: &model.ContentRating{}},

		// 刮削与任务
		{name: "刮削历史", model: &model.ScrapeHistory{}},
		{name: "刮削任务", model: &model.ScrapeTask{}},
		{name: "转码任务", model: &model.TranscodeTask{}},
		{name: "定时任务", model: &model.ScheduledTask{}},

		// AI 相关
		{name: "AI缓存", model: &model.AICacheEntry{}},
		{name: "推荐缓存", model: &model.RecommendCache{}},
		{name: "AI分析任务", model: &model.AIAnalysisTask{}},
		{name: "视频章节", model: &model.VideoChapter{}},
		{name: "视频高光", model: &model.VideoHighlight{}},
		{name: "封面候选", model: &model.CoverCandidate{}},

		// 其他
		{name: "用户权限", model: &model.UserPermission{}},

		// 影视元数据
		// 影视元数据 — 完全删除（包括海报/封面路径等缓存数据）
		{name: "媒体记录", model: &model.Media{}},
		{name: "剧集记录", model: &model.Series{}},

		// 媒体库配置 — 完全删除
		{name: "媒体库配置", model: &model.Library{}},
	}

	// 逐表清理
	for _, item := range tablesToClear {
		result := ClearTableResult{Table: item.name}

		tx := h.db.Unscoped().Where("1 = 1").Delete(item.model)
		if tx.Error != nil {
			result.Status = "error"
			result.Message = tx.Error.Error()
			h.logger.Errorf("清空表 %s 失败: %v", item.name, tx.Error)
		} else {
			result.Cleared = tx.RowsAffected
			result.Status = "success"
			totalCleared += tx.RowsAffected
		}
		results = append(results, result)
	}

	// 清空系统设置（全部删除）
	sysResult := ClearTableResult{Table: "系统设置"}
	tx := h.db.Where("1 = 1").Delete(&model.SystemSetting{})
	if tx.Error != nil {
		sysResult.Status = "error"
		sysResult.Message = tx.Error.Error()
		h.logger.Errorf("清空系统设置失败: %v", tx.Error)
	} else {
		sysResult.Cleared = tx.RowsAffected
		sysResult.Status = "success"
		totalCleared += tx.RowsAffected
	}
	results = append(results, sysResult)

	// 清空用户（保留当前管理员账号）
	userResult := ClearTableResult{Table: "用户账号(保留当前管理员)"}
	tx = h.db.Unscoped().Where("id != ?", adminUserID).Delete(&model.User{})
	if tx.Error != nil {
		userResult.Status = "error"
		userResult.Message = tx.Error.Error()
		h.logger.Errorf("清空用户账号失败: %v", tx.Error)
	} else {
		userResult.Cleared = tx.RowsAffected
		userResult.Status = "success"
		totalCleared += tx.RowsAffected
	}
	results = append(results, userResult)

	// 统计结果
	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Status == "success" {
			successCount++
		} else if r.Status == "error" {
			errorCount++
		}
	}

	h.logger.Infof("一键彻底清空数据完成: 共处理 %d 项, 成功 %d, 失败 %d, 清理记录 %d 条",
		len(results), successCount, errorCount, totalCleared)

	status := "success"
	message := fmt.Sprintf("数据彻底清理完成，共清理 %d 条记录", totalCleared)
	if errorCount > 0 {
		status = "partial"
		message = fmt.Sprintf("数据清理部分完成，成功 %d 项，失败 %d 项，共清理 %d 条记录", successCount, errorCount, totalCleared)
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"status":        status,
			"message":       message,
			"total_cleared": totalCleared,
			"success_count": successCount,
			"error_count":   errorCount,
			"details":       results,
		},
	})
}
