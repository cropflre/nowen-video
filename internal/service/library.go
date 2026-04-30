package service

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// LibraryService 媒体库服务
type LibraryService struct {
	repo              *repository.LibraryRepo
	mediaRepo         *repository.MediaRepo
	seriesRepo        *repository.SeriesRepo
	favRepo           *repository.FavoriteRepo     // 收藏仓储（用于级联清理）
	historyRepo       *repository.WatchHistoryRepo // 观看历史仓储（用于级联清理）
	mediaPersonRepo   *repository.MediaPersonRepo  // 演职人员关联仓储（用于级联清理刮削数据）
	cfg               *config.Config               // 用于访问 CacheDir 以清理磁盘上的刮削缓存
	scanner           *ScannerService
	metadata          *MetadataService
	seriesService     *SeriesService     // 剧集合集服务（用于扫描后自动合并）
	collectionService *CollectionService // 电影系列合集服务（用于扫描后自动匹配）
	logger            *zap.SugaredLogger
	scanning          sync.Map            // 记录正在扫描的媒体库ID
	wsHub             *WSHub              // WebSocket事件广播
	fileWatcher       *FileWatcherService // 文件监听服务
}

func NewLibraryService(
	repo *repository.LibraryRepo,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	favRepo *repository.FavoriteRepo,
	historyRepo *repository.WatchHistoryRepo,
	mediaPersonRepo *repository.MediaPersonRepo,
	cfg *config.Config,
	scanner *ScannerService,
	metadata *MetadataService,
	logger *zap.SugaredLogger,
) *LibraryService {
	return &LibraryService{
		repo:            repo,
		mediaRepo:       mediaRepo,
		seriesRepo:      seriesRepo,
		favRepo:         favRepo,
		historyRepo:     historyRepo,
		mediaPersonRepo: mediaPersonRepo,
		cfg:             cfg,
		scanner:         scanner,
		metadata:        metadata,
		logger:          logger,
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

// SetSeriesService 设置剧集合集服务（延迟注入，用于扫描后自动合并重复剧集）
func (s *LibraryService) SetSeriesService(ss *SeriesService) {
	s.seriesService = ss
}

// SetCollectionService 设置电影系列合集服务（延迟注入，用于扫描后自动匹配系列电影）
func (s *LibraryService) SetCollectionService(cs *CollectionService) {
	s.collectionService = cs
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

	// 清理孤立的收藏记录（media_id 指向的媒体已不存在）
	if s.favRepo != nil {
		favCount, err := s.favRepo.CleanOrphaned()
		if err != nil {
			s.logger.Errorf("清理孤立收藏数据失败: %v", err)
		} else if favCount > 0 {
			s.logger.Infof("已清理 %d 条孤立收藏记录", favCount)
		}
	}

	// 清理孤立的观看历史记录（media_id 指向的媒体已不存在）
	if s.historyRepo != nil {
		historyCount, err := s.historyRepo.CleanOrphaned()
		if err != nil {
			s.logger.Errorf("清理孤立观看历史数据失败: %v", err)
		} else if historyCount > 0 {
			s.logger.Infof("已清理 %d 条孤立观看历史记录", historyCount)
		}
	}
}

// Create 创建媒体库（单路径，兼容旧调用）
func (s *LibraryService) Create(name, path, libType string) (*model.Library, error) {
	return s.CreateWithPaths(name, []string{path}, libType)
}

// CreateWithPaths 创建媒体库（支持多个路径）
// paths[0] 作为主路径写入 Path，其余写入 ExtraPaths
func (s *LibraryService) CreateWithPaths(name string, paths []string, libType string) (*model.Library, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("至少需要一个媒体文件夹路径")
	}
	lib := &model.Library{
		Name: name,
		Type: libType,
	}
	lib.SetAllPaths(paths)
	if err := s.repo.Create(lib); err != nil {
		return nil, err
	}
	if len(paths) > 1 {
		s.logger.Infof("创建媒体库: %s -> %d 个路径 (主: %s)", name, len(paths), lib.Path)
	} else {
		s.logger.Infof("创建媒体库: %s -> %s", name, lib.Path)
	}

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

	// 同步检查每个媒体库路径是否存在且可访问
	allPaths := lib.AllPaths()
	if len(allPaths) == 0 {
		return fmt.Errorf("媒体库未配置任何路径")
	}
	totalEntries := 0
	for _, p := range allPaths {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				s.logger.Errorf("媒体库路径不存在: %s (媒体库: %s)", p, lib.Name)
				return fmt.Errorf("媒体库路径不存在: %s，请检查路径是否正确以及Docker卷映射是否配置", p)
			}
			if os.IsPermission(err) {
				s.logger.Errorf("无权限访问媒体库路径: %s (媒体库: %s)", p, lib.Name)
				return fmt.Errorf("无权限访问媒体库路径: %s，请检查文件权限", p)
			}
			s.logger.Errorf("访问媒体库路径失败: %s, 错误: %v", p, err)
			return fmt.Errorf("访问媒体库路径失败: %v", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("媒体库路径不是目录: %s", p)
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			s.logger.Errorf("读取媒体库目录失败: %s, 错误: %v", p, err)
			return fmt.Errorf("读取媒体库目录失败: %v", err)
		}
		totalEntries += len(entries)
	}
	// 所有路径均为空时才提示
	if totalEntries == 0 {
		s.logger.Warnf("媒体库所有路径均为空 (媒体库: %s)", lib.Name)
		return fmt.Errorf("媒体库目录为空，请确认路径下有影视文件")
	}

	// 防止重复扫描
	if _, scanning := s.scanning.LoadOrStore(id, true); scanning {
		return ErrScanInProgress
	}

	go func() {
		defer s.scanning.Delete(id)

		// 计算总步骤数（根据媒体库类型和设置动态确定）
		stepTotal := 1 // 基础：扫描
		if lib.AutoScrapeMetadata {
			stepTotal++ // 刮削
		}
		if lib.Type == "tvshow" || lib.Type == "mixed" {
			stepTotal++ // 自动合并剧集
		}
		if lib.Type != "tvshow" {
			stepTotal++ // 自动匹配电影系列合集
		}
		stepCurrent := 0

		// 广播阶段事件的辅助函数
		broadcastPhase := func(phase, message string) {
			stepCurrent++
			if s.wsHub != nil {
				s.wsHub.BroadcastEvent(EventScanPhase, &ScanPhaseData{
					LibraryID:   id,
					LibraryName: lib.Name,
					Phase:       phase,
					StepCurrent: stepCurrent,
					StepTotal:   stepTotal,
					Message:     message,
				})
			}
		}

		// 第一步：扫描文件
		broadcastPhase("scanning", fmt.Sprintf("正在扫描媒体文件: %s", lib.Name))
		count, err := s.scanner.ScanLibrary(lib)
		if err != nil {
			s.logger.Errorf("扫描媒体库失败: %s, 错误: %v", lib.Name, err)
			return
		}

		now := time.Now()
		lib.LastScan = &now
		s.repo.Update(lib)

		s.logger.Infof("媒体库 %s 文件扫描完成，新增 %d 个媒体", lib.Name, count)

		// 第二步：自动刮削元数据（如果媒体库开启了自动刮削）
		if lib.AutoScrapeMetadata {
			broadcastPhase("scraping", fmt.Sprintf("正在识别媒体信息: %s", lib.Name))
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
		} else {
			s.logger.Infof("媒体库 %s 已关闭自动刮削，跳过元数据识别", lib.Name)
		}

		// 第三步：自动合并同名剧集（如「女神咖啡厅 第一季」和「女神咖啡厅 第二季」）
		if s.seriesService != nil && (lib.Type == "tvshow" || lib.Type == "mixed") {
			broadcastPhase("merging", fmt.Sprintf("正在合并同名剧集: %s", lib.Name))
			results, err := s.seriesService.AutoMergeDuplicates()
			if err != nil {
				s.logger.Warnf("媒体库 %s 自动合并剧集失败: %v", lib.Name, err)
			} else if len(results) > 0 {
				totalMerged := 0
				for _, r := range results {
					totalMerged += r.MergedCount
				}
				s.logger.Infof("媒体库 %s 自动合并完成: %d 组, 共合并 %d 条重复记录", lib.Name, len(results), totalMerged)
			}
		}

		// 第四步：自动匹配电影系列合集（在刮削完成后执行，确保标题已更新）
		if s.collectionService != nil && lib.Type != "tvshow" {
			broadcastPhase("matching", fmt.Sprintf("正在匹配电影系列合集: %s", lib.Name))
			collCount, err := s.collectionService.AutoMatchCollections()
			if err != nil {
				s.logger.Warnf("媒体库 %s 自动匹配合集失败: %v", lib.Name, err)
			} else if collCount > 0 {
				s.logger.Infof("媒体库 %s 自动创建 %d 个电影系列合集", lib.Name, collCount)
			}
		}

		// 广播全部完成事件
		if s.wsHub != nil {
			s.wsHub.BroadcastEvent(EventScanPhase, &ScanPhaseData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       "completed",
				StepCurrent: stepTotal,
				StepTotal:   stepTotal,
				Message:     fmt.Sprintf("媒体库 %s 处理完成", lib.Name),
			})
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

	// 取消文件监听（逐个路径）
	if lib != nil && s.fileWatcher != nil {
		for _, p := range lib.AllPaths() {
			s.fileWatcher.UnwatchLibrary(p)
		}
	}

	// 先收集该媒体库下所有 media_id 和 series_id，用于后续删除磁盘上的刮削缓存（图片、缩略图、转码等）
	var mediaIDs []string
	var seriesIDs []string
	if mediaList, err := s.mediaRepo.ListByLibraryID(id); err == nil {
		for _, m := range mediaList {
			mediaIDs = append(mediaIDs, m.ID)
		}
	}
	if seriesList, err := s.seriesRepo.ListByLibraryID(id); err == nil {
		for _, se := range seriesList {
			seriesIDs = append(seriesIDs, se.ID)
		}
	}

	// 级联删除关联数据（先清理收藏和观看历史，再删除媒体和合集）
	if s.favRepo != nil {
		if err := s.favRepo.DeleteByLibraryMediaIDs(id); err != nil {
			s.logger.Errorf("删除媒体库 %s 的收藏数据失败: %v", libName, err)
		}
	}
	if s.historyRepo != nil {
		if err := s.historyRepo.DeleteByLibraryMediaIDs(id); err != nil {
			s.logger.Errorf("删除媒体库 %s 的观看历史数据失败: %v", libName, err)
		}
	}
	// 清理演职人员关联（刮削产生的元数据）
	if s.mediaPersonRepo != nil {
		if err := s.mediaPersonRepo.DeleteByLibraryMediaIDs(id); err != nil {
			s.logger.Errorf("删除媒体库 %s 的演职人员关联(media)失败: %v", libName, err)
		}
		if err := s.mediaPersonRepo.DeleteByLibrarySeriesIDs(id); err != nil {
			s.logger.Errorf("删除媒体库 %s 的演职人员关联(series)失败: %v", libName, err)
		}
	}
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

	// 清理磁盘上的刮削缓存（海报/背景、缩略图、转码、预处理）
	s.cleanScrapedCacheFiles(mediaIDs, seriesIDs, libName)

	s.logger.Infof("媒体库 %s 已删除（关联数据及刮削缓存已清理）", libName)

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
// 同时级联清理关联的收藏和观看历史记录
func (s *LibraryService) DeleteMedia(mediaID string) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return fmt.Errorf("影片不存在")
	}
	s.logger.Infof("删除影片: %s (%s)", media.Title, mediaID)

	// 级联清理关联的收藏和观看历史
	if s.favRepo != nil {
		if err := s.favRepo.DeleteByMediaID(mediaID); err != nil {
			s.logger.Errorf("删除影片 %s 的收藏数据失败: %v", media.Title, err)
		}
	}
	if s.historyRepo != nil {
		if err := s.historyRepo.DeleteByMediaID(mediaID); err != nil {
			s.logger.Errorf("删除影片 %s 的观看历史数据失败: %v", media.Title, err)
		}
	}

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

	// 同步检查每个媒体库路径是否存在且可访问
	for _, p := range lib.AllPaths() {
		info, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("媒体库路径不存在: %s，请检查路径是否正确以及Docker卷映射是否配置", p)
			}
			if os.IsPermission(err) {
				return fmt.Errorf("无权限访问媒体库路径: %s，请检查文件权限", p)
			}
			return fmt.Errorf("访问媒体库路径失败: %v", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("媒体库路径不是目录: %s", p)
		}
	}

	// 防止重复操作
	if _, scanning := s.scanning.LoadOrStore(id, true); scanning {
		return ErrScanInProgress
	}

	go func() {
		defer s.scanning.Delete(id)

		// 计算总步骤数（清理 + 扫描 + 可选刮削 + 可选合并 + 可选匹配）
		stepTotal := 2 // 基础：清理 + 扫描
		if lib.AutoScrapeMetadata {
			stepTotal++ // 刮削
		}
		if lib.Type == "tvshow" || lib.Type == "mixed" {
			stepTotal++ // 自动合并剧集
		}
		if lib.Type != "tvshow" {
			stepTotal++ // 自动匹配电影系列合集
		}
		stepCurrent := 0

		broadcastPhase := func(phase, message string) {
			stepCurrent++
			if s.wsHub != nil {
				s.wsHub.BroadcastEvent(EventScanPhase, &ScanPhaseData{
					LibraryID:   id,
					LibraryName: lib.Name,
					Phase:       phase,
					StepCurrent: stepCurrent,
					StepTotal:   stepTotal,
					Message:     message,
				})
			}
		}

		// 第一步：清除该媒体库下所有旧媒体和合集记录
		broadcastPhase("cleaning", fmt.Sprintf("正在清除旧索引数据: %s", lib.Name))
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
		broadcastPhase("scanning", fmt.Sprintf("正在扫描媒体文件: %s", lib.Name))
		count, err := s.scanner.ScanLibrary(lib)
		if err != nil {
			s.logger.Errorf("重建索引扫描失败: %s, 错误: %v", lib.Name, err)
			return
		}

		now := time.Now()
		lib.LastScan = &now
		s.repo.Update(lib)

		s.logger.Infof("媒体库 %s 索引重建完成，共 %d 个媒体", lib.Name, count)

		// 第三步：自动刮削元数据（如果媒体库开启了自动刮削）
		if lib.AutoScrapeMetadata {
			broadcastPhase("scraping", fmt.Sprintf("正在识别媒体信息: %s", lib.Name))
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
		} else {
			s.logger.Infof("媒体库 %s 已关闭自动刮削，跳过元数据识别", lib.Name)
		}

		// 重建索引后自动合并同名剧集
		if s.seriesService != nil && (lib.Type == "tvshow" || lib.Type == "mixed") {
			broadcastPhase("merging", fmt.Sprintf("正在合并同名剧集: %s", lib.Name))
			results, err := s.seriesService.AutoMergeDuplicates()
			if err != nil {
				s.logger.Warnf("媒体库 %s 重建索引后自动合并失败: %v", lib.Name, err)
			} else if len(results) > 0 {
				totalMerged := 0
				for _, r := range results {
					totalMerged += r.MergedCount
				}
				s.logger.Infof("媒体库 %s 重建索引后自动合并: %d 组, 共合并 %d 条", lib.Name, len(results), totalMerged)
			}
		}

		// 重建索引后自动匹配电影系列合集
		if s.collectionService != nil && lib.Type != "tvshow" {
			broadcastPhase("matching", fmt.Sprintf("正在匹配电影系列合集: %s", lib.Name))
			collCount, err := s.collectionService.AutoMatchCollections()
			if err != nil {
				s.logger.Warnf("媒体库 %s 重建索引后自动匹配合集失败: %v", lib.Name, err)
			} else if collCount > 0 {
				s.logger.Infof("媒体库 %s 重建索引后自动创建 %d 个电影系列合集", lib.Name, collCount)
			}
		}

		// 广播全部完成事件
		if s.wsHub != nil {
			s.wsHub.BroadcastEvent(EventScanPhase, &ScanPhaseData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       "completed",
				StepCurrent: stepTotal,
				StepTotal:   stepTotal,
				Message:     fmt.Sprintf("媒体库 %s 重建索引完成", lib.Name),
			})
		}
	}()

	return nil
}

// ==================== 重复媒体检测 ====================

// DetectDuplicates 检测媒体库中的重复媒体
func (s *LibraryService) DetectDuplicates(libraryID string) ([]DuplicateGroup, error) {
	return s.scanner.DetectDuplicates(libraryID)
}

// MarkDuplicates 标记重复媒体
func (s *LibraryService) MarkDuplicates(libraryID string) (int, error) {
	return s.scanner.MarkDuplicates(libraryID)
}

// cleanScrapedCacheFiles 清理指定媒体/合集在磁盘上的刮削缓存文件
// 包括：海报/背景图、缩略图、转码分片、预处理产物
func (s *LibraryService) cleanScrapedCacheFiles(mediaIDs, seriesIDs []string, libName string) {
	if s.cfg == nil || s.cfg.Cache.CacheDir == "" {
		return
	}
	cacheDir := s.cfg.Cache.CacheDir

	removeDir := func(p string) {
		if p == "" {
			return
		}
		if _, err := os.Stat(p); err != nil {
			return
		}
		if err := os.RemoveAll(p); err != nil {
			s.logger.Debugf("清理刮削缓存目录失败 %s: %v", p, err)
		}
	}

	// 媒体级缓存：images/{media_id}、thumbnails/{media_id}、transcode/{media_id}、preprocess/{media_id}、covers/{media_id}
	for _, mid := range mediaIDs {
		if mid == "" {
			continue
		}
		removeDir(filepath.Join(cacheDir, "images", mid))
		removeDir(filepath.Join(cacheDir, "thumbnails", mid))
		removeDir(filepath.Join(cacheDir, "transcode", mid))
		removeDir(filepath.Join(cacheDir, "preprocess", mid))
		removeDir(filepath.Join(cacheDir, "covers", mid))
	}

	// 合集级缓存：images/series/{series_id}
	for _, sid := range seriesIDs {
		if sid == "" {
			continue
		}
		removeDir(filepath.Join(cacheDir, "images", "series", sid))
	}

	if len(mediaIDs) > 0 || len(seriesIDs) > 0 {
		s.logger.Infof("媒体库 %s 刮削缓存已清理（media: %d, series: %d）", libName, len(mediaIDs), len(seriesIDs))
	}
}
