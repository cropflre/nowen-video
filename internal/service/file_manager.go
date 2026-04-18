package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// FileManagerService 影视文件管理服务
type FileManagerService struct {
	mediaRepo  *repository.MediaRepo
	seriesRepo *repository.SeriesRepo
	metadata   *MetadataService
	ai         *AIService
	wsHub      *WSHub
	logger     *zap.SugaredLogger

	// 操作日志
	opLogMu sync.Mutex
	opLogs  []FileOperationLog
}

// FileOperationLog 文件操作日志
type FileOperationLog struct {
	ID        string    `json:"id"`
	Action    string    `json:"action"`     // import / edit / delete / scrape / rename / batch_scrape / batch_rename
	MediaID   string    `json:"media_id"`   // 关联的媒体ID
	Detail    string    `json:"detail"`     // 操作详情
	OldValue  string    `json:"old_value"`  // 旧值（用于回滚）
	NewValue  string    `json:"new_value"`  // 新值
	UserID    string    `json:"user_id"`    // 操作者
	CreatedAt time.Time `json:"created_at"` // 操作时间
}

// FileImportRequest 文件导入请求
type FileImportRequest struct {
	FilePath  string `json:"file_path"`
	Title     string `json:"title"`
	MediaType string `json:"media_type"` // movie / episode
	LibraryID string `json:"library_id"`
	Year      int    `json:"year"`
	Overview  string `json:"overview"`
}

// BatchImportResult 批量导入结果
type BatchImportResult struct {
	Total    int      `json:"total"`
	Success  int      `json:"success"`
	Failed   int      `json:"failed"`
	Skipped  int      `json:"skipped"`
	Errors   []string `json:"errors"`
	MediaIDs []string `json:"media_ids"`
}

// RenamePreview AI重命名预览
type RenamePreview struct {
	MediaID     string `json:"media_id"`
	OldTitle    string `json:"old_title"`
	NewTitle    string `json:"new_title"`
	OldFilePath string `json:"old_file_path"`
	NewFilePath string `json:"new_file_path"`
	Reason      string `json:"reason"` // AI给出的重命名理由
}

// RenameTemplate 重命名模板
type RenameTemplate struct {
	Pattern string `json:"pattern"` // 如 "{title} ({year}) [{resolution}]"
	Example string `json:"example"` // 示例输出
}

// ScrapeProgress 刮削进度
type ScrapeProgress struct {
	MediaID  string `json:"media_id"`
	Title    string `json:"title"`
	Status   string `json:"status"`   // pending / scraping / done / failed
	Progress int    `json:"progress"` // 0-100
	Message  string `json:"message"`
}

// FileManagerStats 文件管理统计
type FileManagerStats struct {
	TotalFiles       int64 `json:"total_files"`
	MovieCount       int64 `json:"movie_count"`
	EpisodeCount     int64 `json:"episode_count"`
	ScrapedCount     int64 `json:"scraped_count"`
	UnscrapedCount   int64 `json:"unscraped_count"`
	TotalSizeBytes   int64 `json:"total_size_bytes"`
	RecentImports    int64 `json:"recent_imports"`    // 最近7天导入
	RecentOperations int   `json:"recent_operations"` // 最近操作数
}

// NewFileManagerService 创建文件管理服务
func NewFileManagerService(
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	metadata *MetadataService,
	ai *AIService,
	logger *zap.SugaredLogger,
) *FileManagerService {
	return &FileManagerService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		metadata:   metadata,
		ai:         ai,
		logger:     logger,
		opLogs:     make([]FileOperationLog, 0, 500),
	}
}

// SetWSHub 设置 WebSocket Hub
func (s *FileManagerService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== 文件列表与查询 ====================

// ListFiles 获取影视文件列表（支持多条件筛选）
func (s *FileManagerService) ListFiles(page, size int, libraryID, mediaType, keyword, sortBy, sortOrder string, scrapedOnly *bool) ([]model.Media, int64, error) {
	return s.mediaRepo.ListFilesAdvanced(page, size, libraryID, mediaType, keyword, sortBy, sortOrder, scrapedOnly)
}

// FolderNode 文件夹树节点
type FolderNode struct {
	Name      string        `json:"name"`       // 文件夹名称
	Path      string        `json:"path"`       // 完整路径
	Children  []*FolderNode `json:"children"`   // 子文件夹
	FileCount int           `json:"file_count"` // 直接子文件数量
}

// GetFolderTree 获取文件夹树形结构
func (s *FileManagerService) GetFolderTree(libraryID string) ([]*FolderNode, error) {
	paths, err := s.mediaRepo.GetAllFilePaths(libraryID)
	if err != nil {
		return nil, fmt.Errorf("获取文件路径失败: %w", err)
	}

	// 统计每个目录下的直接文件数
	dirFileCount := make(map[string]int)
	dirSet := make(map[string]bool)

	for _, p := range paths {
		// 标准化路径分隔符
		normalized := strings.ReplaceAll(p, "\\", "/")
		dir := filepath.Dir(normalized)
		dir = strings.ReplaceAll(dir, "\\", "/")
		dirFileCount[dir]++
		dirSet[dir] = true

		// 记录所有祖先目录
		parts := strings.Split(dir, "/")
		for i := 1; i <= len(parts); i++ {
			ancestor := strings.Join(parts[:i], "/")
			if ancestor == "" {
				continue
			}
			dirSet[ancestor] = true
		}
	}

	// 构建树
	root := make(map[string]*FolderNode)
	var rootNodes []*FolderNode

	// 收集所有目录并排序
	var allDirs []string
	for d := range dirSet {
		allDirs = append(allDirs, d)
	}
	// 按路径长度排序，确保父目录先处理
	for i := 0; i < len(allDirs); i++ {
		for j := i + 1; j < len(allDirs); j++ {
			if len(allDirs[i]) > len(allDirs[j]) {
				allDirs[i], allDirs[j] = allDirs[j], allDirs[i]
			}
		}
	}

	for _, dir := range allDirs {
		name := filepath.Base(dir)
		name = strings.ReplaceAll(name, "\\", "/")
		if name == "." || name == "" {
			continue
		}

		node := &FolderNode{
			Name:      name,
			Path:      dir,
			Children:  make([]*FolderNode, 0),
			FileCount: dirFileCount[dir],
		}
		root[dir] = node

		// 查找父目录
		parentDir := filepath.Dir(dir)
		parentDir = strings.ReplaceAll(parentDir, "\\", "/")
		if parentNode, ok := root[parentDir]; ok {
			parentNode.Children = append(parentNode.Children, node)
		} else {
			rootNodes = append(rootNodes, node)
		}
	}

	// 按名称排序根节点
	sortFolderNodes(rootNodes)
	// 递归排序每个节点的子节点
	sortFolderChildren(rootNodes)

	return rootNodes, nil
}

// ListFilesByFolder 按文件夹路径查询文件
func (s *FileManagerService) ListFilesByFolder(folderPath string, page, size int, libraryID, mediaType, keyword, sortBy, sortOrder string, scrapedOnly *bool) ([]model.Media, int64, []string, error) {
	// 获取文件列表
	files, total, err := s.mediaRepo.ListByFolderPath(folderPath, page, size, libraryID, mediaType, keyword, sortBy, sortOrder, scrapedOnly)
	if err != nil {
		return nil, 0, nil, err
	}

	// 获取当前文件夹下的子文件夹列表
	allPaths, err := s.mediaRepo.GetAllFilePaths(libraryID)
	if err != nil {
		return files, total, nil, nil
	}

	normalizedFolder := strings.ReplaceAll(folderPath, "\\", "/")
	if !strings.HasSuffix(normalizedFolder, "/") {
		normalizedFolder += "/"
	}

	subFolderSet := make(map[string]bool)
	for _, p := range allPaths {
		normalized := strings.ReplaceAll(p, "\\", "/")
		if !strings.HasPrefix(normalized, normalizedFolder) {
			continue
		}
		// 获取相对路径
		relative := strings.TrimPrefix(normalized, normalizedFolder)
		// 如果包含 /，说明在子文件夹中
		if idx := strings.Index(relative, "/"); idx > 0 {
			subFolder := relative[:idx]
			subFolderSet[subFolder] = true
		}
	}

	var subFolders []string
	for f := range subFolderSet {
		subFolders = append(subFolders, f)
	}
	// 按名称排序（不区分大小写）
	sort.Slice(subFolders, func(i, j int) bool {
		return strings.ToLower(subFolders[i]) < strings.ToLower(subFolders[j])
	})

	return files, total, subFolders, nil
}

// CreateFolder 在指定路径下创建文件夹
func (s *FileManagerService) CreateFolder(parentPath, folderName, userID string) error {
	if parentPath == "" || folderName == "" {
		return fmt.Errorf("路径和文件夹名不能为空")
	}
	// 检查文件夹名是否包含非法字符
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalidChars {
		if strings.Contains(folderName, ch) {
			return fmt.Errorf("文件夹名包含非法字符: %s", ch)
		}
	}
	fullPath := filepath.Join(parentPath, folderName)
	if _, err := os.Stat(fullPath); err == nil {
		return fmt.Errorf("文件夹已存在: %s", fullPath)
	}
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return fmt.Errorf("创建文件夹失败: %w", err)
	}
	s.addOpLog("create_folder", "", fmt.Sprintf("创建文件夹: %s", fullPath), "", fullPath, userID)
	return nil
}

// RenameFolder 重命名文件夹
func (s *FileManagerService) RenameFolder(folderPath, newName, userID string) error {
	if folderPath == "" || newName == "" {
		return fmt.Errorf("路径和新名称不能为空")
	}
	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, ch := range invalidChars {
		if strings.Contains(newName, ch) {
			return fmt.Errorf("文件夹名包含非法字符: %s", ch)
		}
	}
	info, err := os.Stat(folderPath)
	if err != nil {
		return fmt.Errorf("文件夹不存在: %s", folderPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是文件夹: %s", folderPath)
	}
	parentDir := filepath.Dir(folderPath)
	newPath := filepath.Join(parentDir, newName)
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("目标文件夹已存在: %s", newPath)
	}
	if err := os.Rename(folderPath, newPath); err != nil {
		return fmt.Errorf("重命名文件夹失败: %w", err)
	}

	// 更新数据库中所有受影响的文件路径
	oldPrefix := strings.ReplaceAll(folderPath, "\\", "/")
	newPrefix := strings.ReplaceAll(newPath, "\\", "/")
	if !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}
	if !strings.HasSuffix(newPrefix, "/") {
		newPrefix += "/"
	}
	if err := s.mediaRepo.UpdateFilePathPrefix(oldPrefix, newPrefix); err != nil {
		s.logger.Warnf("更新文件路径前缀失败: %v", err)
	}

	s.addOpLog("rename_folder", "", fmt.Sprintf("重命名文件夹: %s → %s", folderPath, newPath), folderPath, newPath, userID)
	s.broadcastEvent("folder_renamed", map[string]interface{}{
		"old_path": folderPath,
		"new_path": newPath,
	})
	return nil
}

// DeleteFolder 删除文件夹（仅删除空文件夹，或删除文件夹及其数据库记录）
func (s *FileManagerService) DeleteFolder(folderPath string, force bool, userID string) error {
	if folderPath == "" {
		return fmt.Errorf("文件夹路径不能为空")
	}
	info, err := os.Stat(folderPath)
	if err != nil {
		return fmt.Errorf("文件夹不存在: %s", folderPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("路径不是文件夹: %s", folderPath)
	}

	if !force {
		// 非强制模式：仅删除空文件夹
		entries, err := os.ReadDir(folderPath)
		if err != nil {
			return fmt.Errorf("读取文件夹失败: %w", err)
		}
		if len(entries) > 0 {
			return fmt.Errorf("文件夹不为空，包含 %d 个项目。如需强制删除请确认", len(entries))
		}
		if err := os.Remove(folderPath); err != nil {
			return fmt.Errorf("删除文件夹失败: %w", err)
		}
	} else {
		// 强制模式：删除文件夹及所有内容
		if err := os.RemoveAll(folderPath); err != nil {
			return fmt.Errorf("删除文件夹失败: %w", err)
		}
		// 删除数据库中该文件夹下的所有文件记录
		normalizedPath := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalizedPath, "/") {
			normalizedPath += "/"
		}
		if err := s.mediaRepo.DeleteByPathPrefix(normalizedPath); err != nil {
			s.logger.Warnf("删除文件夹下的数据库记录失败: %v", err)
		}
	}

	s.addOpLog("delete_folder", "", fmt.Sprintf("删除文件夹: %s (force=%v)", folderPath, force), folderPath, "", userID)
	s.broadcastEvent("folder_deleted", map[string]interface{}{
		"path":  folderPath,
		"force": force,
	})
	return nil
}

// GetFileDetail 获取文件详情
func (s *FileManagerService) GetFileDetail(id string) (*model.Media, error) {
	media, err := s.mediaRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("文件不存在")
	}
	// 如果是剧集，加载合集信息
	if media.MediaType == "episode" && media.SeriesID != "" {
		series, seriesErr := s.seriesRepo.FindByIDOnly(media.SeriesID)
		if seriesErr == nil {
			media.Series = series
		}
	}
	return media, nil
}

// ==================== 文件导入 ====================

// ImportFile 导入单个影视文件
func (s *FileManagerService) ImportFile(req FileImportRequest, userID string) (*model.Media, error) {
	// 验证文件路径
	if req.FilePath == "" {
		return nil, fmt.Errorf("文件路径不能为空")
	}

	// 检查文件是否存在
	if _, err := os.Stat(req.FilePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("文件不存在: %s", req.FilePath)
	}

	// 检查是否已导入
	existing, err := s.mediaRepo.FindByFilePath(req.FilePath)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("文件已存在于系统中 (ID: %s, 标题: %s)", existing.ID, existing.Title)
	}

	// 获取文件信息
	fileInfo, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("无法读取文件信息: %w", err)
	}

	// 自动提取标题
	title := req.Title
	if title == "" {
		title = s.extractTitleFromPath(req.FilePath)
	}

	// 默认媒体类型
	mediaType := req.MediaType
	if mediaType == "" {
		mediaType = "movie"
	}

	// 检测分辨率
	resolution := s.detectResolutionFromFilename(filepath.Base(req.FilePath))

	media := &model.Media{
		LibraryID:  req.LibraryID,
		Title:      title,
		Year:       req.Year,
		Overview:   req.Overview,
		FilePath:   req.FilePath,
		FileSize:   fileInfo.Size(),
		MediaType:  mediaType,
		Resolution: resolution,
	}

	if err := s.mediaRepo.Create(media); err != nil {
		return nil, fmt.Errorf("导入文件失败: %w", err)
	}

	// 记录操作日志
	s.addOpLog("import", media.ID, fmt.Sprintf("导入文件: %s", req.FilePath), "", media.ID, userID)

	// 广播事件
	s.broadcastEvent("file_imported", map[string]interface{}{
		"media_id": media.ID,
		"title":    media.Title,
	})

	return media, nil
}

// BatchImportFiles 批量导入影视文件
func (s *FileManagerService) BatchImportFiles(files []FileImportRequest, userID string) *BatchImportResult {
	result := &BatchImportResult{
		Total:    len(files),
		MediaIDs: make([]string, 0),
	}

	for i, req := range files {
		media, err := s.ImportFile(req, userID)
		if err != nil {
			if strings.Contains(err.Error(), "已存在") {
				result.Skipped++
			} else {
				result.Failed++
			}
			result.Errors = append(result.Errors, fmt.Sprintf("[%d] %s: %s", i+1, req.FilePath, err.Error()))
		} else {
			result.Success++
			result.MediaIDs = append(result.MediaIDs, media.ID)
		}

		// 广播进度
		s.broadcastEvent("batch_import_progress", map[string]interface{}{
			"current": i + 1,
			"total":   len(files),
			"success": result.Success,
			"failed":  result.Failed,
			"skipped": result.Skipped,
		})
	}

	return result
}

// ScanDirectoryFiles 扫描目录获取可导入的影视文件列表
func (s *FileManagerService) ScanDirectoryFiles(dirPath string) ([]map[string]interface{}, error) {
	if dirPath == "" {
		return nil, fmt.Errorf("目录路径不能为空")
	}

	info, err := os.Stat(dirPath)
	if err != nil {
		return nil, fmt.Errorf("目录不存在: %s", dirPath)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("路径不是目录: %s", dirPath)
	}

	// 支持的视频格式
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true, ".m4v": true,
		".ts": true, ".rmvb": true, ".rm": true, ".3gp": true,
		".strm": true, // STRM 远程流文件
	}

	var files []map[string]interface{}
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !videoExts[ext] {
			return nil
		}

		// 检查是否已导入
		existing, _ := s.mediaRepo.FindByFilePath(path)
		imported := existing != nil

		files = append(files, map[string]interface{}{
			"path":     path,
			"name":     info.Name(),
			"size":     info.Size(),
			"ext":      ext,
			"modified": info.ModTime(),
			"imported": imported,
			"title":    s.extractTitleFromPath(path),
		})

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("扫描目录失败: %w", err)
	}

	return files, nil
}

// ==================== 文件编辑 ====================

// UpdateFileInfo 更新文件信息
func (s *FileManagerService) UpdateFileInfo(mediaID string, updates map[string]interface{}, userID string) (*model.Media, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, fmt.Errorf("文件不存在")
	}

	// 保存旧值用于回滚
	oldJSON, _ := json.Marshal(media)

	// 应用更新
	if v, ok := updates["title"].(string); ok && v != "" {
		media.Title = v
	}
	if v, ok := updates["orig_title"].(string); ok {
		media.OrigTitle = v
	}
	if v, ok := updates["year"]; ok {
		if year, ok := v.(float64); ok {
			media.Year = int(year)
		}
	}
	if v, ok := updates["overview"].(string); ok {
		media.Overview = v
	}
	if v, ok := updates["genres"].(string); ok {
		media.Genres = v
	}
	if v, ok := updates["rating"]; ok {
		if rating, ok := v.(float64); ok {
			media.Rating = rating
		}
	}
	if v, ok := updates["media_type"].(string); ok {
		media.MediaType = v
	}
	if v, ok := updates["country"].(string); ok {
		media.Country = v
	}
	if v, ok := updates["language"].(string); ok {
		media.Language = v
	}
	if v, ok := updates["tagline"].(string); ok {
		media.Tagline = v
	}
	if v, ok := updates["studio"].(string); ok {
		media.Studio = v
	}

	if err := s.mediaRepo.Update(media); err != nil {
		return nil, fmt.Errorf("更新失败: %w", err)
	}

	// 记录操作日志
	newJSON, _ := json.Marshal(media)
	s.addOpLog("edit", mediaID, "编辑文件信息", string(oldJSON), string(newJSON), userID)

	return media, nil
}

// ==================== 安全删除 ====================

// DeleteFile 安全删除文件记录（仅删除数据库记录，不删除原始文件）
func (s *FileManagerService) DeleteFile(mediaID, userID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return fmt.Errorf("文件不存在")
	}

	// 保存旧值用于回滚
	oldJSON, _ := json.Marshal(media)

	// 仅删除数据库记录
	if err := s.mediaRepo.DeleteByID(mediaID); err != nil {
		return fmt.Errorf("删除失败: %w", err)
	}

	// 记录操作日志
	s.addOpLog("delete", mediaID, fmt.Sprintf("删除文件记录: %s (%s)", media.Title, media.FilePath), string(oldJSON), "", userID)

	// 广播事件
	s.broadcastEvent("file_deleted", map[string]interface{}{
		"media_id": mediaID,
		"title":    media.Title,
	})

	return nil
}

// BatchDeleteFiles 批量安全删除
func (s *FileManagerService) BatchDeleteFiles(mediaIDs []string, userID string) (deleted int, errors []string) {
	for _, id := range mediaIDs {
		if err := s.DeleteFile(id, userID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
		} else {
			deleted++
		}
	}
	return
}

// ==================== AI智能刮削 ====================

// ScrapeFileMetadata 对单个文件执行AI智能刮削
func (s *FileManagerService) ScrapeFileMetadata(mediaID, source, userID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return fmt.Errorf("文件不存在")
	}

	// 异步执行刮削
	go s.executeScrapeFile(media, source, userID)

	return nil
}

// BatchScrapeFiles 批量刮削
func (s *FileManagerService) BatchScrapeFiles(mediaIDs []string, source, userID string) (started int, errors []string) {
	for _, id := range mediaIDs {
		if err := s.ScrapeFileMetadata(id, source, userID); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", id, err.Error()))
		} else {
			started++
		}
	}
	return
}

// executeScrapeFile 执行单个文件的刮削
func (s *FileManagerService) executeScrapeFile(media *model.Media, source, userID string) {
	// 广播开始
	s.broadcastEvent("file_scrape_progress", ScrapeProgress{
		MediaID:  media.ID,
		Title:    media.Title,
		Status:   "scraping",
		Progress: 10,
		Message:  "开始刮削...",
	})

	oldJSON, _ := json.Marshal(media)

	var scrapeErr error
	searchTitle := media.Title
	if media.OrigTitle != "" {
		searchTitle = media.OrigTitle
	}

	switch source {
	case "tmdb", "":
		// 默认使用TMDb
		scrapeErr = s.scrapeFromTMDb(media, searchTitle)
	case "bangumi":
		scrapeErr = s.scrapeFromBangumi(media, searchTitle)
	case "ai":
		scrapeErr = s.scrapeWithAIEnhance(media, searchTitle)
	default:
		scrapeErr = s.scrapeFromTMDb(media, searchTitle)
	}

	if scrapeErr != nil {
		// 如果主数据源失败，尝试AI增强
		if s.ai != nil && s.ai.IsEnabled() && source != "ai" {
			s.broadcastEvent("file_scrape_progress", ScrapeProgress{
				MediaID:  media.ID,
				Title:    media.Title,
				Status:   "scraping",
				Progress: 60,
				Message:  "主数据源失败，尝试AI增强...",
			})
			scrapeErr = s.scrapeWithAIEnhance(media, searchTitle)
		}
	}

	if scrapeErr != nil {
		s.broadcastEvent("file_scrape_progress", ScrapeProgress{
			MediaID:  media.ID,
			Title:    media.Title,
			Status:   "failed",
			Progress: 0,
			Message:  scrapeErr.Error(),
		})
		s.addOpLog("scrape", media.ID, fmt.Sprintf("刮削失败: %s", scrapeErr.Error()), "", "", userID)
		return
	}

	// 保存更新
	if err := s.mediaRepo.Update(media); err != nil {
		s.logger.Errorf("保存刮削结果失败: %v", err)
		return
	}

	newJSON, _ := json.Marshal(media)
	s.addOpLog("scrape", media.ID, fmt.Sprintf("刮削完成: %s", media.Title), string(oldJSON), string(newJSON), userID)

	// 广播完成
	s.broadcastEvent("file_scrape_progress", ScrapeProgress{
		MediaID:  media.ID,
		Title:    media.Title,
		Status:   "done",
		Progress: 100,
		Message:  "刮削完成",
	})
}

// scrapeFromTMDb 从TMDb刮削元数据
func (s *FileManagerService) scrapeFromTMDb(media *model.Media, searchTitle string) error {
	searchType := "movie"
	if media.MediaType == "episode" {
		searchType = "tv"
	}

	results, err := s.metadata.SearchTMDb(searchType, searchTitle, media.Year)
	if err != nil {
		return fmt.Errorf("TMDb搜索失败: %w", err)
	}
	if len(results) == 0 {
		return fmt.Errorf("TMDb未找到匹配结果: %s", searchTitle)
	}

	// 应用第一个结果
	r := results[0]
	title := r.Title
	if title == "" {
		title = r.Name
	}
	if title != "" {
		media.Title = title
	}
	if r.OriginalTitle != "" {
		media.OrigTitle = r.OriginalTitle
	}
	if r.Overview != "" {
		media.Overview = r.Overview
	}
	if r.VoteAverage > 0 {
		media.Rating = r.VoteAverage
	}
	if r.PosterPath != "" {
		media.PosterPath = "https://image.tmdb.org/t/p/w500" + r.PosterPath
	}
	if r.BackdropPath != "" {
		media.BackdropPath = "https://image.tmdb.org/t/p/w1280" + r.BackdropPath
	}
	if r.ReleaseDate != "" && len(r.ReleaseDate) >= 4 {
		fmt.Sscanf(r.ReleaseDate[:4], "%d", &media.Year)
		media.Premiered = r.ReleaseDate
	} else if r.FirstAirDate != "" && len(r.FirstAirDate) >= 4 {
		fmt.Sscanf(r.FirstAirDate[:4], "%d", &media.Year)
		media.Premiered = r.FirstAirDate
	}
	media.TMDbID = r.ID

	return nil
}

// scrapeFromBangumi 从Bangumi刮削元数据
func (s *FileManagerService) scrapeFromBangumi(media *model.Media, searchTitle string) error {
	subjects, err := s.metadata.SearchBangumi(searchTitle, 2, 0)
	if err != nil {
		return fmt.Errorf("Bangumi搜索失败: %w", err)
	}
	if len(subjects) == 0 {
		return fmt.Errorf("Bangumi未找到匹配结果: %s", searchTitle)
	}

	sub := subjects[0]
	if sub.NameCN != "" {
		media.Title = sub.NameCN
	}
	media.OrigTitle = sub.Name
	media.Overview = sub.Summary
	if sub.Rating != nil {
		media.Rating = sub.Rating.Score
	}
	if sub.Images != nil && sub.Images.Large != "" {
		media.PosterPath = sub.Images.Large
	}
	if sub.AirDate != "" && len(sub.AirDate) >= 4 {
		fmt.Sscanf(sub.AirDate[:4], "%d", &media.Year)
	}
	media.BangumiID = sub.ID

	// 提取标签作为类型
	var genres []string
	for _, tag := range sub.Tags {
		if tag.Count > 10 {
			genres = append(genres, tag.Name)
		}
		if len(genres) >= 5 {
			break
		}
	}
	if len(genres) > 0 {
		media.Genres = strings.Join(genres, ",")
	}

	return nil
}

// scrapeWithAIEnhance 使用AI增强刮削
func (s *FileManagerService) scrapeWithAIEnhance(media *model.Media, searchTitle string) error {
	if s.ai == nil || !s.ai.IsEnabled() {
		return fmt.Errorf("AI服务未启用")
	}

	return s.ai.EnrichMetadata(media, searchTitle)
}

// ==================== AI批量重命名 ====================

// PreviewRename 预览AI重命名结果
func (s *FileManagerService) PreviewRename(mediaIDs []string, template string) ([]RenamePreview, error) {
	var previews []RenamePreview

	for _, id := range mediaIDs {
		media, err := s.mediaRepo.FindByID(id)
		if err != nil {
			continue
		}

		newTitle := s.generateRenameTitle(media, template)
		newFilePath := s.generateRenameFilePath(media, newTitle)

		previews = append(previews, RenamePreview{
			MediaID:     media.ID,
			OldTitle:    media.Title,
			NewTitle:    newTitle,
			OldFilePath: media.FilePath,
			NewFilePath: newFilePath,
			Reason:      s.generateRenameReason(media, newTitle),
		})
	}

	return previews, nil
}

// ExecuteRename 执行重命名（仅修改数据库中的标题，不修改原始文件）
func (s *FileManagerService) ExecuteRename(mediaIDs []string, template, userID string) (renamed int, errors []string) {
	for _, id := range mediaIDs {
		media, err := s.mediaRepo.FindByID(id)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: 文件不存在", id))
			continue
		}

		oldTitle := media.Title
		newTitle := s.generateRenameTitle(media, template)

		if newTitle == oldTitle {
			continue // 无需重命名
		}

		media.Title = newTitle
		if err := s.mediaRepo.Update(media); err != nil {
			errors = append(errors, fmt.Sprintf("%s: 更新失败 - %s", id, err.Error()))
			continue
		}

		renamed++
		s.addOpLog("rename", id, fmt.Sprintf("重命名: %s → %s", oldTitle, newTitle), oldTitle, newTitle, userID)
	}

	// 广播完成
	s.broadcastEvent("batch_rename_complete", map[string]interface{}{
		"renamed": renamed,
		"errors":  len(errors),
	})

	return
}

// AIGenerateRenames 使用AI智能生成重命名建议（支持多语言翻译）
func (s *FileManagerService) AIGenerateRenames(mediaIDs []string, targetLang string) ([]RenamePreview, error) {
	if s.ai == nil || !s.ai.IsEnabled() {
		return nil, fmt.Errorf("AI服务未启用")
	}

	// 语言名称映射
	langNames := map[string]string{
		"zh": "中文", "en": "English", "ja": "日本語", "ko": "한국어",
		"fr": "Français", "de": "Deutsch", "es": "Español", "pt": "Português",
		"ru": "Русский", "it": "Italiano", "th": "ไทย", "vi": "Tiếng Việt",
		"ar": "العربية", "hi": "हिन्दी",
	}

	var previews []RenamePreview

	for _, id := range mediaIDs {
		media, err := s.mediaRepo.FindByID(id)
		if err != nil {
			continue
		}

		// 构建AI提示词，根据是否指定目标语言调整
		var systemPrompt, userPrompt string
		if targetLang != "" {
			langName := langNames[targetLang]
			if langName == "" {
				langName = targetLang
			}
			systemPrompt = fmt.Sprintf(
				"你是一个影视文件命名专家，擅长生成规范化的影视文件名。请将生成的标题翻译为%s（%s）。",
				langName, targetLang,
			)
			userPrompt = fmt.Sprintf(
				"请为以下影视文件生成一个规范化的标题，并将标题翻译为%s。当前标题: '%s', 原始标题: '%s', 年份: %d, 类型: %s, 分辨率: %s。"+
					"请返回一个规范的标题格式，如: '翻译后的电影名 (年份) [分辨率]'。标题部分必须使用%s。只返回标题，不要其他内容。",
				langName, media.Title, media.OrigTitle, media.Year, media.MediaType, media.Resolution, langName,
			)
		} else {
			systemPrompt = "你是一个影视文件命名专家，擅长生成规范化的影视文件名。"
			userPrompt = fmt.Sprintf(
				"请为以下影视文件生成一个规范化的标题。当前标题: '%s', 年份: %d, 类型: %s, 分辨率: %s。"+
					"请返回一个规范的标题格式，如: '电影名 (年份) [分辨率]'。只返回标题，不要其他内容。",
				media.Title, media.Year, media.MediaType, media.Resolution,
			)
		}

		response, err := s.ai.ChatCompletion(systemPrompt, userPrompt, 0.3, 150)
		if err != nil {
			// AI失败时使用模板生成
			response = s.generateRenameTitle(media, "{title} ({year}) [{resolution}]")
		}

		newTitle := strings.TrimSpace(response)
		if newTitle == "" || newTitle == media.Title {
			continue
		}

		reason := "AI智能生成"
		if targetLang != "" {
			langName := langNames[targetLang]
			if langName == "" {
				langName = targetLang
			}
			reason = fmt.Sprintf("AI智能生成（%s）", langName)
		}

		previews = append(previews, RenamePreview{
			MediaID:     media.ID,
			OldTitle:    media.Title,
			NewTitle:    newTitle,
			OldFilePath: media.FilePath,
			NewFilePath: s.generateRenameFilePath(media, newTitle),
			Reason:      reason,
		})
	}

	return previews, nil
}

// GetRenameTemplates 获取可用的重命名模板
func (s *FileManagerService) GetRenameTemplates() []RenameTemplate {
	return []RenameTemplate{
		{Pattern: "{title} ({year})", Example: "肖申克的救赎 (1994)"},
		{Pattern: "{title} ({year}) [{resolution}]", Example: "肖申克的救赎 (1994) [1080p]"},
		{Pattern: "{title}.{year}.{resolution}", Example: "肖申克的救赎.1994.1080p"},
		{Pattern: "{orig_title} - {title} ({year})", Example: "The Shawshank Redemption - 肖申克的救赎 (1994)"},
		{Pattern: "[{year}] {title} [{resolution}]", Example: "[1994] 肖申克的救赎 [1080p]"},
	}
}

// ==================== 统计信息 ====================

// GetStats 获取文件管理统计
func (s *FileManagerService) GetStats() (*FileManagerStats, error) {
	stats := &FileManagerStats{}

	// 总文件数
	_, total, err := s.mediaRepo.List(1, 1, "")
	if err != nil {
		return nil, err
	}
	stats.TotalFiles = total

	// 电影数量
	movieCount, err := s.mediaRepo.CountByMediaType("movie")
	if err == nil {
		stats.MovieCount = movieCount
	}

	// 剧集数量
	episodeCount, err := s.mediaRepo.CountByMediaType("episode")
	if err == nil {
		stats.EpisodeCount = episodeCount
	}

	// 已刮削数量（有TMDb ID或Bangumi ID的）
	scrapedCount, err := s.mediaRepo.CountScraped()
	if err == nil {
		stats.ScrapedCount = scrapedCount
	}
	stats.UnscrapedCount = stats.TotalFiles - stats.ScrapedCount

	// 总文件大小
	totalSize, err := s.mediaRepo.SumFileSize()
	if err == nil {
		stats.TotalSizeBytes = totalSize
	}

	// 最近7天导入
	recentCount, err := s.mediaRepo.CountRecentImports(7)
	if err == nil {
		stats.RecentImports = recentCount
	}

	// 最近操作数
	s.opLogMu.Lock()
	stats.RecentOperations = len(s.opLogs)
	s.opLogMu.Unlock()

	return stats, nil
}

// ==================== 操作日志 ====================

// GetOperationLogs 获取操作日志
func (s *FileManagerService) GetOperationLogs(limit int) []FileOperationLog {
	s.opLogMu.Lock()
	defer s.opLogMu.Unlock()

	if limit <= 0 || limit > len(s.opLogs) {
		limit = len(s.opLogs)
	}

	// 返回最近的日志（倒序）
	result := make([]FileOperationLog, limit)
	for i := 0; i < limit; i++ {
		result[i] = s.opLogs[len(s.opLogs)-1-i]
	}
	return result
}

// addOpLog 添加操作日志
func (s *FileManagerService) addOpLog(action, mediaID, detail, oldValue, newValue, userID string) {
	s.opLogMu.Lock()
	defer s.opLogMu.Unlock()

	log := FileOperationLog{
		ID:        uuid.New().String(),
		Action:    action,
		MediaID:   mediaID,
		Detail:    detail,
		OldValue:  oldValue,
		NewValue:  newValue,
		UserID:    userID,
		CreatedAt: time.Now(),
	}

	s.opLogs = append(s.opLogs, log)

	// 保持日志不超过500条
	if len(s.opLogs) > 500 {
		s.opLogs = s.opLogs[len(s.opLogs)-500:]
	}
}

// ==================== 辅助方法 ====================

// extractTitleFromPath 从文件路径提取标题
func (s *FileManagerService) extractTitleFromPath(filePath string) string {
	base := filepath.Base(filePath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	// 移除常见的标记
	cleaners := []string{
		"1080p", "720p", "2160p", "4K", "4k",
		"BluRay", "BDRip", "WEB-DL", "WEBRip", "HDRip",
		"x264", "x265", "H264", "H265", "HEVC", "AVC",
		"AAC", "DTS", "AC3", "FLAC",
		"REMUX", "Remux",
	}

	for _, c := range cleaners {
		name = strings.ReplaceAll(name, c, "")
	}

	// 替换分隔符
	name = strings.ReplaceAll(name, ".", " ")
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")

	// 清理多余空格
	parts := strings.Fields(name)
	if len(parts) > 0 {
		return strings.Join(parts, " ")
	}

	return name
}

// detectResolutionFromFilename 从文件名检测分辨率
func (s *FileManagerService) detectResolutionFromFilename(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.Contains(lower, "2160p") || strings.Contains(lower, "4k"):
		return "4K"
	case strings.Contains(lower, "1080p"):
		return "1080p"
	case strings.Contains(lower, "720p"):
		return "720p"
	case strings.Contains(lower, "480p"):
		return "480p"
	default:
		return ""
	}
}

// generateRenameTitle 根据模板生成重命名标题
func (s *FileManagerService) generateRenameTitle(media *model.Media, template string) string {
	result := template

	result = strings.ReplaceAll(result, "{title}", media.Title)
	result = strings.ReplaceAll(result, "{orig_title}", media.OrigTitle)
	result = strings.ReplaceAll(result, "{year}", fmt.Sprintf("%d", media.Year))
	result = strings.ReplaceAll(result, "{resolution}", media.Resolution)
	result = strings.ReplaceAll(result, "{media_type}", media.MediaType)

	// 清理空的占位符
	if media.Year == 0 {
		result = strings.ReplaceAll(result, " (0)", "")
		result = strings.ReplaceAll(result, ".0.", ".")
	}
	if media.Resolution == "" {
		result = strings.ReplaceAll(result, " []", "")
		result = strings.ReplaceAll(result, "..", ".")
	}
	if media.OrigTitle == "" {
		result = strings.ReplaceAll(result, " -  ", " ")
	}

	return strings.TrimSpace(result)
}

// generateRenameFilePath 生成重命名后的文件路径（仅用于预览，不实际修改文件）
func (s *FileManagerService) generateRenameFilePath(media *model.Media, newTitle string) string {
	dir := filepath.Dir(media.FilePath)
	ext := filepath.Ext(media.FilePath)
	// 清理文件名中的非法字符
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, newTitle)
	return filepath.Join(dir, safeName+ext)
}

// generateRenameReason 生成重命名理由
func (s *FileManagerService) generateRenameReason(media *model.Media, newTitle string) string {
	if media.Title == newTitle {
		return "标题未变更"
	}
	return fmt.Sprintf("基于模板规则，将 '%s' 规范化为 '%s'", media.Title, newTitle)
}

// broadcastEvent 广播事件
func (s *FileManagerService) broadcastEvent(eventType string, data interface{}) {
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(eventType, data)
	}
}

// ==================== 误分类检测与重分类 ====================

// MisclassifiedItem 误分类检测结果项
type MisclassifiedItem struct {
	MediaID       string   `json:"media_id"`
	Title         string   `json:"title"`
	FilePath      string   `json:"file_path"`
	CurrentType   string   `json:"current_type"`   // 当前分类（movie）
	SuggestedType string   `json:"suggested_type"` // 建议分类（episode）
	Confidence    float64  `json:"confidence"`     // 置信度 0-1
	Reasons       []string `json:"reasons"`        // 判断依据
	FileSize      int64    `json:"file_size"`
	DirPath       string   `json:"dir_path"`      // 所在目录
	SiblingCount  int      `json:"sibling_count"` // 同目录下同类文件数
}

// MisclassificationReport 误分类分析报告
type MisclassificationReport struct {
	TotalMovies       int                 `json:"total_movies"`       // 当前标记为电影的总数
	SuspectedEpisodes int                 `json:"suspected_episodes"` // 疑似剧集数量
	HighConfidence    int                 `json:"high_confidence"`    // 高置信度（>0.8）
	MediumConfidence  int                 `json:"medium_confidence"`  // 中置信度（0.5-0.8）
	LowConfidence     int                 `json:"low_confidence"`     // 低置信度（<0.5）
	Items             []MisclassifiedItem `json:"items"`              // 详细列表
	CommonPatterns    []string            `json:"common_patterns"`    // 常见误分类模式
	Suggestions       []string            `json:"suggestions"`        // 优化建议
}

// ReclassifyRequest 重分类请求
type ReclassifyRequest struct {
	MediaIDs       []string `json:"media_ids"`        // 要重分类的文件ID
	NewType        string   `json:"new_type"`         // 新类型（episode）
	AutoLinkSeries bool     `json:"auto_link_series"` // 是否自动关联剧集合集
}

// ReclassifyResult 重分类结果
type ReclassifyResult struct {
	Total        int      `json:"total"`
	Success      int      `json:"success"`
	Failed       int      `json:"failed"`
	Errors       []string `json:"errors"`
	LinkedSeries int      `json:"linked_series"` // 成功关联的剧集合集数
}

// misclassifyEpisodePatterns 误分类检测用的剧集文件名正则模式
var misclassifyEpisodePatterns = []struct {
	Pattern string
	Name    string
}{
	{`(?i)S\d{1,2}E\d{1,3}`, "SxxExx格式（如S01E01）"},
	{`(?i)Season\s*\d+`, "Season关键词"},
	{`(?i)EP?\d{1,3}`, "EP/E+数字格式"},
	{`第\d+[集话]`, "中文第X集/话格式"},
	{`第[一二三四五六七八九十百]+[集话季]`, "中文数字集/话/季格式"},
	{`\[\d{2,3}\]`, "方括号编号格式（如[01]）"},
	{`(?i)\s-\s\d{2,3}\s`, "短横线编号格式（如 - 01 ）"},
	{`(?i)Episode\s*\d+`, "Episode关键词"},
	{`(?i)第\d+季`, "中文第X季格式"},
}

// misclassifyDirPatterns 误分类检测用的剧集目录名正则模式
var misclassifyDirPatterns = []struct {
	Pattern string
	Name    string
}{
	{`(?i)Season\s*\d+`, "Season目录"},
	{`(?i)S\d{1,2}`, "S+数字目录"},
	{`第[一二三四五六七八九十\d]+季`, "中文季目录"},
	{`(?i)series`, "Series关键词"},
	{`(?i)TV\s*Show`, "TV Show关键词"},
}

// AnalyzeMisclassification 分析文件库中的误分类情况
func (s *FileManagerService) AnalyzeMisclassification() (*MisclassificationReport, error) {
	// 获取所有标记为movie的文件
	movies, err := s.mediaRepo.ListByMediaType("movie")
	if err != nil {
		return nil, fmt.Errorf("查询电影文件失败: %w", err)
	}

	report := &MisclassificationReport{
		TotalMovies: len(movies),
		Items:       make([]MisclassifiedItem, 0),
	}

	// 按目录分组统计
	dirFileCount := make(map[string]int)
	dirFiles := make(map[string][]string) // dir -> file titles
	for _, m := range movies {
		dir := filepath.Dir(m.FilePath)
		dirFileCount[dir]++
		dirFiles[dir] = append(dirFiles[dir], m.Title)
	}

	// 分析每个文件
	patternStats := make(map[string]int) // 统计各模式命中次数
	for _, m := range movies {
		item := s.analyzeMediaClassification(&m, dirFileCount, dirFiles)
		if item != nil {
			report.Items = append(report.Items, *item)
			for _, reason := range item.Reasons {
				patternStats[reason]++
			}
		}
	}

	report.SuspectedEpisodes = len(report.Items)

	// 按置信度分类统计
	for _, item := range report.Items {
		switch {
		case item.Confidence >= 0.8:
			report.HighConfidence++
		case item.Confidence >= 0.5:
			report.MediumConfidence++
		default:
			report.LowConfidence++
		}
	}

	// 按置信度降序排序
	for i := 0; i < len(report.Items); i++ {
		for j := i + 1; j < len(report.Items); j++ {
			if report.Items[j].Confidence > report.Items[i].Confidence {
				report.Items[i], report.Items[j] = report.Items[j], report.Items[i]
			}
		}
	}

	// 生成常见模式
	for pattern, count := range patternStats {
		if count > 1 {
			report.CommonPatterns = append(report.CommonPatterns, fmt.Sprintf("%s（命中 %d 个文件）", pattern, count))
		}
	}

	// 生成优化建议
	report.Suggestions = s.generateMisclassificationSuggestions(report)

	return report, nil
}

// analyzeMediaClassification 分析单个文件的分类是否正确
func (s *FileManagerService) analyzeMediaClassification(media *model.Media, dirFileCount map[string]int, dirFiles map[string][]string) *MisclassifiedItem {
	var reasons []string
	var confidence float64

	fileName := filepath.Base(media.FilePath)
	dirPath := filepath.Dir(media.FilePath)
	dirName := filepath.Base(dirPath)
	parentDirName := filepath.Base(filepath.Dir(dirPath))

	// 1. 文件名模式匹配
	for _, p := range misclassifyEpisodePatterns {
		matched, _ := regexp.MatchString(p.Pattern, fileName)
		if matched {
			reasons = append(reasons, fmt.Sprintf("文件名匹配剧集模式: %s", p.Name))
			confidence += 0.3
		}
	}

	// 也检查标题
	for _, p := range misclassifyEpisodePatterns {
		matched, _ := regexp.MatchString(p.Pattern, media.Title)
		if matched {
			reasons = append(reasons, fmt.Sprintf("标题匹配剧集模式: %s", p.Name))
			confidence += 0.2
		}
	}

	// 2. 目录名模式匹配
	for _, p := range misclassifyDirPatterns {
		matchedDir, _ := regexp.MatchString(p.Pattern, dirName)
		matchedParent, _ := regexp.MatchString(p.Pattern, parentDirName)
		if matchedDir || matchedParent {
			reasons = append(reasons, fmt.Sprintf("目录名匹配剧集模式: %s", p.Name))
			confidence += 0.25
		}
	}

	// 3. 同目录下文件数量分析（同目录下有多个视频文件很可能是剧集）
	siblingCount := dirFileCount[dirPath]
	if siblingCount >= 3 {
		reasons = append(reasons, fmt.Sprintf("同目录下有 %d 个视频文件（剧集特征）", siblingCount))
		confidence += 0.2
		if siblingCount >= 6 {
			confidence += 0.1 // 更多文件，更可能是剧集
		}
		if siblingCount >= 12 {
			confidence += 0.1 // 12集以上，非常可能是剧集
		}
	}

	// 4. 文件大小分析（剧集单集通常在200MB-3GB之间，且同目录文件大小相近）
	if media.FileSize > 0 {
		sizeMB := float64(media.FileSize) / (1024 * 1024)
		if sizeMB >= 100 && sizeMB <= 3000 && siblingCount >= 3 {
			reasons = append(reasons, fmt.Sprintf("文件大小 %.0fMB 符合剧集单集特征", sizeMB))
			confidence += 0.1
		}
	}

	// 5. 目录名包含"季"相关关键词
	seasonKeywords := []string{"第一季", "第二季", "第三季", "第四季", "第五季",
		"Season 1", "Season 2", "Season 3", "Season 4", "Season 5",
		"S01", "S02", "S03", "S04", "S05"}
	for _, kw := range seasonKeywords {
		if strings.Contains(dirName, kw) || strings.Contains(parentDirName, kw) || strings.Contains(media.Title, kw) {
			reasons = append(reasons, fmt.Sprintf("包含季关键词: %s", kw))
			confidence += 0.3
			break
		}
	}

	// 6. 文件名包含连续编号（如 01, 02, 03...）
	numPattern := regexp.MustCompile(`(?:^|\D)(\d{1,3})(?:\D|$)`)
	if matches := numPattern.FindStringSubmatch(fileName); len(matches) > 1 {
		if siblingCount >= 3 {
			reasons = append(reasons, "文件名含编号且同目录有多个文件")
			confidence += 0.15
		}
	}

	// 置信度上限为1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	// 只返回有一定置信度的结果
	if confidence < 0.3 || len(reasons) == 0 {
		return nil
	}

	return &MisclassifiedItem{
		MediaID:       media.ID,
		Title:         media.Title,
		FilePath:      media.FilePath,
		CurrentType:   media.MediaType,
		SuggestedType: "episode",
		Confidence:    confidence,
		Reasons:       reasons,
		FileSize:      media.FileSize,
		DirPath:       dirPath,
		SiblingCount:  siblingCount,
	}
}

// generateMisclassificationSuggestions 生成误分类优化建议
func (s *FileManagerService) generateMisclassificationSuggestions(report *MisclassificationReport) []string {
	var suggestions []string

	if report.HighConfidence > 0 {
		suggestions = append(suggestions, fmt.Sprintf("🔴 有 %d 个文件高度疑似剧集（置信度>80%%），建议优先处理", report.HighConfidence))
	}
	if report.MediumConfidence > 0 {
		suggestions = append(suggestions, fmt.Sprintf("🟡 有 %d 个文件中度疑似剧集（置信度50%%-80%%），建议人工确认后处理", report.MediumConfidence))
	}
	if report.LowConfidence > 0 {
		suggestions = append(suggestions, fmt.Sprintf("🟢 有 %d 个文件低度疑似（置信度<50%%），可暂时忽略", report.LowConfidence))
	}

	if report.SuspectedEpisodes == 0 {
		suggestions = append(suggestions, "✅ 未发现明显的误分类文件，文件库分类状态良好")
	} else {
		ratio := float64(report.SuspectedEpisodes) / float64(report.TotalMovies) * 100
		if ratio > 20 {
			suggestions = append(suggestions, fmt.Sprintf("⚠️ 误分类比例较高（%.1f%%），建议检查文件导入流程中的类型识别逻辑", ratio))
		}
		suggestions = append(suggestions, "💡 建议先处理高置信度文件，确认无误后再处理中低置信度文件")
		suggestions = append(suggestions, "💡 重分类后建议重新刮削元数据，以获取正确的剧集信息")
	}

	return suggestions
}

// ReclassifyFiles 批量重分类文件
func (s *FileManagerService) ReclassifyFiles(req ReclassifyRequest, userID string) (*ReclassifyResult, error) {
	if len(req.MediaIDs) == 0 {
		return nil, fmt.Errorf("未指定要重分类的文件")
	}
	if req.NewType == "" {
		req.NewType = "episode"
	}

	result := &ReclassifyResult{
		Total: len(req.MediaIDs),
	}

	// 逐个处理以便记录详细日志
	for _, id := range req.MediaIDs {
		media, err := s.mediaRepo.FindByID(id)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: 文件不存在", id))
			continue
		}

		oldType := media.MediaType
		if oldType == req.NewType {
			continue // 类型相同，跳过
		}

		// 保存旧值
		oldJSON, _ := json.Marshal(media)

		// 更新类型
		media.MediaType = req.NewType

		// 如果重分类为episode且开启自动关联，尝试关联到Series
		if req.NewType == "episode" && req.AutoLinkSeries {
			s.tryLinkToSeries(media)
		}

		if err := s.mediaRepo.Update(media); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s (%s): 更新失败 - %s", id, media.Title, err.Error()))
			continue
		}

		result.Success++

		// 记录操作日志
		newJSON, _ := json.Marshal(media)
		s.addOpLog("reclassify", id,
			fmt.Sprintf("重分类: %s → %s (%s)", oldType, req.NewType, media.Title),
			string(oldJSON), string(newJSON), userID)

		if media.SeriesID != "" {
			result.LinkedSeries++
		}
	}

	// 广播完成事件
	s.broadcastEvent("reclassify_complete", map[string]interface{}{
		"total":   result.Total,
		"success": result.Success,
		"failed":  result.Failed,
	})

	return result, nil
}

// tryLinkToSeries 尝试将文件关联到已有的剧集合集
func (s *FileManagerService) tryLinkToSeries(media *model.Media) {
	dirPath := filepath.Dir(media.FilePath)

	// 1. 先尝试通过目录路径匹配已有的Series
	series, err := s.seriesRepo.FindByFolderPath(dirPath)
	if err == nil && series != nil {
		media.SeriesID = series.ID
		// 尝试从文件名提取季集编号
		s.extractSeasonEpisodeNum(media)
		return
	}

	// 2. 尝试父目录
	parentDir := filepath.Dir(dirPath)
	series, err = s.seriesRepo.FindByFolderPath(parentDir)
	if err == nil && series != nil {
		media.SeriesID = series.ID
		s.extractSeasonEpisodeNum(media)
		return
	}

	// 3. 尝试通过标题匹配
	// 从目录名或标题中提取可能的剧集名
	dirName := filepath.Base(dirPath)
	// 移除季信息
	cleanTitle := regexp.MustCompile(`(?i)\s*(Season\s*\d+|S\d+|第[一二三四五六七八九十\d]+季)\s*`).ReplaceAllString(dirName, "")
	cleanTitle = strings.TrimSpace(cleanTitle)

	if cleanTitle != "" && media.LibraryID != "" {
		series, err = s.seriesRepo.FindByTitleAndLibrary(cleanTitle, media.LibraryID)
		if err == nil && series != nil {
			media.SeriesID = series.ID
			s.extractSeasonEpisodeNum(media)
			return
		}
	}
}

// extractSeasonEpisodeNum 从文件名提取季集编号
func (s *FileManagerService) extractSeasonEpisodeNum(media *model.Media) {
	fileName := filepath.Base(media.FilePath)
	dirName := filepath.Base(filepath.Dir(media.FilePath))

	// 提取季号
	seasonPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)S(\d{1,2})`),
		regexp.MustCompile(`(?i)Season\s*(\d{1,2})`),
		regexp.MustCompile(`第(\d+)季`),
	}
	for _, p := range seasonPatterns {
		// 先从目录名提取
		if matches := p.FindStringSubmatch(dirName); len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &media.SeasonNum)
			break
		}
		// 再从文件名提取
		if matches := p.FindStringSubmatch(fileName); len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &media.SeasonNum)
			break
		}
	}

	// 默认季号为1
	if media.SeasonNum == 0 {
		media.SeasonNum = 1
	}

	// 提取集号
	episodePatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)S\d{1,2}E(\d{1,3})`),
		regexp.MustCompile(`(?i)EP?(\d{1,3})`),
		regexp.MustCompile(`第(\d+)[集话]`),
		regexp.MustCompile(`\[(\d{2,3})\]`),
		regexp.MustCompile(`(?:^|\D)(\d{2,3})(?:\D|$)`),
	}
	for _, p := range episodePatterns {
		if matches := p.FindStringSubmatch(fileName); len(matches) > 1 {
			fmt.Sscanf(matches[1], "%d", &media.EpisodeNum)
			break
		}
	}
}

// sortFolderNodes 按名称排序文件夹节点（不区分大小写）
func sortFolderNodes(nodes []*FolderNode) {
	sort.Slice(nodes, func(i, j int) bool {
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}

// sortFolderChildren 递归排序所有子节点
func sortFolderChildren(nodes []*FolderNode) {
	for _, node := range nodes {
		if len(node.Children) > 0 {
			sortFolderNodes(node.Children)
			sortFolderChildren(node.Children)
		}
	}
}
