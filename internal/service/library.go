package service

import (
	"fmt"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// LibraryService 媒体库服务
type LibraryService struct {
	repo        *repository.LibraryRepo
	mediaRepo   *repository.MediaRepo
	seriesRepo  *repository.SeriesRepo
	scanner     *ScannerService
	metadata    *MetadataService
	logger      *zap.SugaredLogger
	scanning    sync.Map            // 记录正在扫描的媒体库ID
	wsHub       *WSHub              // WebSocket事件广播
	fileWatcher *FileWatcherService // 文件监听服务
}

func NewLibraryService(
	repo *repository.LibraryRepo,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	scanner *ScannerService,
	metadata *MetadataService,
	logger *zap.SugaredLogger,
) *LibraryService {
	return &LibraryService{
		repo:       repo,
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		scanner:    scanner,
		metadata:   metadata,
		logger:     logger,
	}
}

// SetWSHub 设置WebSocket Hub（延迟注入，避免循环依赖）
func (s *LibraryService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// SetFileWatcher 设置文件监听服务（延迟注入）
func (s *LibraryService) SetFileWatcher(fw *FileWatcherService) {
	s.fileWatcher = fw
}

// CleanOrphanedData 清理孤立数据：删除 library_id 指向已不存在的媒体库的 Media 和 Series 记录
// 用于处理历史遗留的数据不一致问题（旧版本删除媒体库时未级联清理关联数据）
func (s *LibraryService) CleanOrphanedData() {
	libs, err := s.repo.List()
	if err != nil {
		s.logger.Errorf("清理孤立数据失败（获取媒体库列表出错）: %v", err)
		return
	}

	// 收集所有有效的媒体库 ID
	var validIDs []string
	for _, lib := range libs {
		validIDs = append(validIDs, lib.ID)
	}

	// 清理孤立的 Media 记录
	mediaCount, err := s.mediaRepo.CleanOrphanedByLibraryIDs(validIDs)
	if err != nil {
		s.logger.Errorf("清理孤立媒体数据失败: %v", err)
	} else if mediaCount > 0 {
		s.logger.Infof("已清理 %d 条孤立媒体记录", mediaCount)
	}

	// 清理幽灵 Media 记录（library_id 为空的无主记录，由豆瓣刮削 Series 时误创建）
	ghostCount, err := s.mediaRepo.CleanGhostMedia()
	if err != nil {
		s.logger.Errorf("清理幽灵媒体数据失败: %v", err)
	} else if ghostCount > 0 {
		s.logger.Infof("已清理 %d 条幽灵媒体记录（library_id 为空）", ghostCount)
	}

	// 清理孤立的 Series 记录
	seriesCount, err := s.seriesRepo.CleanOrphanedByLibraryIDs(validIDs)
	if err != nil {
		s.logger.Errorf("清理孤立剧集合集数据失败: %v", err)
	} else if seriesCount > 0 {
		s.logger.Infof("已清理 %d 条孤立剧集合集记录", seriesCount)
	}

	// 清理空合集（episode_count 为 0 的合集记录，通常是文件被删除后的残留）
	emptyCount, err := s.seriesRepo.CleanEmptySeries()
	if err != nil {
		s.logger.Errorf("清理空合集数据失败: %v", err)
	} else if emptyCount > 0 {
		s.logger.Infof("已清理 %d 条空合集记录（episode_count = 0）", emptyCount)
	}

	if mediaCount > 0 || ghostCount > 0 || seriesCount > 0 || emptyCount > 0 {
		s.logger.Infof("数据清理完成（孤立媒体: %d, 幽灵媒体: %d, 孤立合集: %d, 空合集: %d）", mediaCount, ghostCount, seriesCount, emptyCount)
	}
}

// Create 创建媒体库
func (s *LibraryService) Create(name, path, libType string) (*model.Library, error) {
	lib := &model.Library{
		Name: name,
		Path: path,
		Type: libType,
	}
	if err := s.repo.Create(lib); err != nil {
		return nil, err
	}
	s.logger.Infof("创建媒体库: %s -> %s", name, path)

	// 如果启用了文件监控，自动注册监听
	if lib.EnableFileWatch && s.fileWatcher != nil {
		s.fileWatcher.WatchLibrary(lib)
	}

	return lib, nil
}

// List 获取所有媒体库
func (s *LibraryService) List() ([]model.Library, error) {
	return s.repo.List()
}

// Scan 触发扫描（异步）- 包含文件扫描和元数据刮削
func (s *LibraryService) Scan(id string) error {
	lib, err := s.repo.FindByID(id)
	if err != nil {
		return ErrLibraryNotFound
	}

	// 防止重复扫描
	if _, scanning := s.scanning.LoadOrStore(id, true); scanning {
		return ErrScanInProgress
	}

	go func() {
		defer s.scanning.Delete(id)

		// 第一步：扫描文件
		count, err := s.scanner.ScanLibrary(lib)
		if err != nil {
			s.logger.Errorf("扫描媒体库失败: %s, 错误: %v", lib.Name, err)
			return
		}

		now := time.Now()
		lib.LastScan = &now
		s.repo.Update(lib)

		s.logger.Infof("媒体库 %s 文件扫描完成，新增 %d 个媒体", lib.Name, count)

		// 第二步：自动刮削元数据（如果配置了TMDb API Key）
		if lib.Type == "tvshow" || lib.Type == "mixed" {
			// 剧集库/混合库：优先刮削合集元数据，然后同步到各集
			seriesSuccess, seriesFailed := s.metadata.ScrapeAllSeries(id)
			if seriesSuccess > 0 || seriesFailed > 0 {
				s.logger.Infof("媒体库 %s 剧集合集刮削: 成功 %d, 失败 %d", lib.Name, seriesSuccess, seriesFailed)
			}
		}
		if lib.Type != "tvshow" {
			// 电影库/混合库：刮削电影类型的媒体
			mediaList, err := s.mediaRepo.ListByLibraryID(id)
			if err == nil && len(mediaList) > 0 {
				// 混合库只刮削电影类型的媒体，剧集已由上面的合集刮削处理
				if lib.Type == "mixed" {
					var movieList []model.Media
					for _, m := range mediaList {
						if m.MediaType == "movie" {
							movieList = append(movieList, m)
						}
					}
					mediaList = movieList
				}
				if len(mediaList) > 0 {
					success, failed := s.metadata.ScrapeLibrary(id, mediaList)
					if success > 0 || failed > 0 {
						s.logger.Infof("媒体库 %s 元数据刮削: 成功 %d, 失败 %d", lib.Name, success, failed)
					}
				}
			}
		}
	}()

	return nil
}

// Delete 删除媒体库
// Delete 删除媒体库（级联清理关联的媒体和剧集合集数据）
func (s *LibraryService) Delete(id string) error {
	// 先获取媒体库信息（用于日志和事件通知）
	lib, _ := s.repo.FindByID(id)
	libName := id
	if lib != nil {
		libName = lib.Name
	}

	// 取消文件监听
	if lib != nil && s.fileWatcher != nil {
		s.fileWatcher.UnwatchLibrary(lib.Path)
	}

	// 级联删除关联数据
	if err := s.mediaRepo.DeleteByLibraryID(id); err != nil {
		s.logger.Errorf("删除媒体库 %s 的媒体数据失败: %v", libName, err)
	}
	if err := s.seriesRepo.DeleteByLibraryID(id); err != nil {
		s.logger.Errorf("删除媒体库 %s 的剧集合集数据失败: %v", libName, err)
	}

	// 删除媒体库记录本身
	if err := s.repo.Delete(id); err != nil {
		return err
	}

	s.logger.Infof("媒体库 %s 已删除（关联数据已清理）", libName)

	// 广播媒体库删除事件，通知前端刷新
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventLibraryDeleted, &LibraryChangedData{
			LibraryID:   id,
			LibraryName: libName,
			Action:      "deleted",
			Message:     fmt.Sprintf("媒体库「%s」已删除", libName),
		})
	}

	return nil
}

// Update 更新媒体库信息
func (s *LibraryService) Update(lib *model.Library) error {
	return s.repo.Update(lib)
}

// DeleteMedia 删除单个媒体记录（仅从数据库移除，不删除文件）
func (s *LibraryService) DeleteMedia(mediaID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return fmt.Errorf("影片不存在")
	}
	s.logger.Infof("删除影片: %s (%s)", media.Title, mediaID)
	return s.mediaRepo.DeleteByID(mediaID)
}

// UpdateMedia 更新媒体信息
func (s *LibraryService) UpdateMedia(media *model.Media) error {
	return s.mediaRepo.Update(media)
}

// GetMediaByID 获取单个媒体（用于管理操作）
func (s *LibraryService) GetMediaByID(id string) (*model.Media, error) {
	return s.mediaRepo.FindByID(id)
}

// DeleteSeries 删除剧集合集记录（仅从数据库移除，不删除文件）
func (s *LibraryService) DeleteSeries(seriesID string) error {
	series, err := s.seriesRepo.FindByID(seriesID)
	if err != nil {
		return fmt.Errorf("剧集合集不存在")
	}
	s.logger.Infof("删除剧集合集: %s (%s)", series.Title, seriesID)
	return s.seriesRepo.Delete(seriesID)
}

// UpdateSeries 更新剧集合集信息
func (s *LibraryService) UpdateSeries(series *model.Series) error {
	return s.seriesRepo.Update(series)
}

// GetSeriesByID 获取单个剧集合集（用于管理操作）
func (s *LibraryService) GetSeriesByID(id string) (*model.Series, error) {
	return s.seriesRepo.FindByID(id)
}

// FindByID 查找媒体库
func (s *LibraryService) FindByID(id string) (*model.Library, error) {
	return s.repo.FindByID(id)
}

// Reindex 重建媒体库索引（删除旧媒体记录后重新扫描）
func (s *LibraryService) Reindex(id string) error {
	lib, err := s.repo.FindByID(id)
	if err != nil {
		return ErrLibraryNotFound
	}

	// 防止重复操作
	if _, scanning := s.scanning.LoadOrStore(id, true); scanning {
		return ErrScanInProgress
	}

	go func() {
		defer s.scanning.Delete(id)

		// 第一步：清除该媒体库下所有旧媒体和合集记录
		if err := s.mediaRepo.DeleteByLibraryID(id); err != nil {
			s.logger.Errorf("清除媒体库旧媒体记录失败: %s, 错误: %v", lib.Name, err)
			return
		}
		if err := s.seriesRepo.DeleteByLibraryID(id); err != nil {
			s.logger.Errorf("清除媒体库旧合集记录失败: %s, 错误: %v", lib.Name, err)
			return
		}
		s.logger.Infof("已清除媒体库 %s 的旧索引数据（含媒体和合集）", lib.Name)

		// 第二步：重新扫描文件
		count, err := s.scanner.ScanLibrary(lib)
		if err != nil {
			s.logger.Errorf("重建索引扫描失败: %s, 错误: %v", lib.Name, err)
			return
		}

		now := time.Now()
		lib.LastScan = &now
		s.repo.Update(lib)

		s.logger.Infof("媒体库 %s 索引重建完成，共 %d 个媒体", lib.Name, count)

		// 第三步：自动刮削元数据
		if lib.Type == "tvshow" || lib.Type == "mixed" {
			seriesSuccess, seriesFailed := s.metadata.ScrapeAllSeries(id)
			if seriesSuccess > 0 || seriesFailed > 0 {
				s.logger.Infof("媒体库 %s 重建索引刮削(剧集): 成功 %d, 失败 %d", lib.Name, seriesSuccess, seriesFailed)
			}
		}
		if lib.Type != "tvshow" {
			mediaList, err := s.mediaRepo.ListByLibraryID(id)
			if err == nil && len(mediaList) > 0 {
				if lib.Type == "mixed" {
					var movieList []model.Media
					for _, m := range mediaList {
						if m.MediaType == "movie" {
							movieList = append(movieList, m)
						}
					}
					mediaList = movieList
				}
				if len(mediaList) > 0 {
					success, failed := s.metadata.ScrapeLibrary(id, mediaList)
					if success > 0 || failed > 0 {
						s.logger.Infof("媒体库 %s 重建索引刮削(电影): 成功 %d, 失败 %d", lib.Name, success, failed)
					}
				}
			}
		}
	}()

	return nil
}
