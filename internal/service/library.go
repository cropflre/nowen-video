package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// LibraryService 媒体库服务
type LibraryService struct {
	repo                   *repository.LibraryRepo
	mediaRepo              *repository.MediaRepo
	seriesRepo             *repository.SeriesRepo
	favRepo                *repository.FavoriteRepo           // 收藏仓储（用于级联清理）
	historyRepo            *repository.WatchHistoryRepo       // 观看历史仓储（用于级联清理）
	mediaPersonRepo        *repository.MediaPersonRepo        // 演职人员关联仓储（用于级联清理刮削数据）
	scanClassificationRepo *repository.ScanClassificationRepo // 扫描归类仓储（用于级联清理分类记录）
	cfg                    *config.Config                     // 用于访问 CacheDir 以清理磁盘上的刮削缓存
	scanner                *ScannerService
	metadata               *MetadataService
	seriesService          *SeriesService          // 剧集合集服务（用于扫描后自动合并）
	collectionService      *CollectionService      // 电影系列合集服务（用于扫描后自动匹配）
	scanPostProcess        *ScanPostProcessService // AI 自动整理：虚拟归类与命名（扫描后同步执行）
	logger                 *zap.SugaredLogger
	scanning               sync.Map            // 记录正在扫描的媒体库ID
	scanStates             sync.Map            // 持久化当前扫描阶段：libraryID -> *ScanPhaseData
	wsHub                  *WSHub              // WebSocket事件广播
	fileWatcher            *FileWatcherService // 文件监听服务
}

func NewLibraryService(
	repo *repository.LibraryRepo,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	favRepo *repository.FavoriteRepo,
	historyRepo *repository.WatchHistoryRepo,
	mediaPersonRepo *repository.MediaPersonRepo,
	scanClassificationRepo *repository.ScanClassificationRepo,
	cfg *config.Config,
	scanner *ScannerService,
	metadata *MetadataService,
	logger *zap.SugaredLogger,
) *LibraryService {
	return &LibraryService{
		repo:                   repo,
		mediaRepo:              mediaRepo,
		seriesRepo:             seriesRepo,
		favRepo:                favRepo,
		historyRepo:            historyRepo,
		mediaPersonRepo:        mediaPersonRepo,
		scanClassificationRepo: scanClassificationRepo,
		cfg:                    cfg,
		scanner:                scanner,
		metadata:               metadata,
		logger:                 logger,
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

// SetScanPostProcessService 设置扫描后处理服务（延迟注入，用于扫描后同步执行 AI 自动整理）
func (s *LibraryService) SetScanPostProcessService(spp *ScanPostProcessService) {
	s.scanPostProcess = spp
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

// ActiveScanPhases 返回当前仍在运行的扫描阶段状态。
// 该状态保存在服务内存中，用于前端切换 Tab / 刷新页面后恢复进度展示。
func (s *LibraryService) ActiveScanPhases() []ScanPhaseData {
	states := make([]ScanPhaseData, 0)
	s.scanStates.Range(func(_, value any) bool {
		if v, ok := value.(*ScanPhaseData); ok && v != nil {
			states = append(states, *v)
		}
		return true
	})
	return states
}

func (s *LibraryService) setScanPhaseState(data *ScanPhaseData) {
	if data == nil || data.LibraryID == "" {
		return
	}
	s.scanStates.Store(data.LibraryID, data)
}

func (s *LibraryService) clearScanPhaseState(libraryID string) {
	if libraryID != "" {
		s.scanStates.Delete(libraryID)
	}
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
		defer s.clearScanPhaseState(id)

		// 异步检查路径（网络驱动器可能较慢，不阻塞 HTTP 响应）
		allPaths := lib.AllPaths()
		if len(allPaths) == 0 {
			s.logger.Errorf("媒体库未配置任何路径 (媒体库: %s)", lib.Name)
			return
		}
		totalEntries := 0
		for _, p := range allPaths {
			info, err := os.Stat(p)
			if err != nil {
				s.logger.Errorf("媒体库路径不可访问: %s (媒体库: %s) err=%v", p, lib.Name, err)
				return
			}
			if !info.IsDir() {
				s.logger.Errorf("媒体库路径不是目录: %s (媒体库: %s)", p, lib.Name)
				return
			}
			entries, _ := os.ReadDir(p)
			totalEntries += len(entries)
		}
		if totalEntries == 0 {
			s.logger.Warnf("媒体库所有路径均为空 (媒体库: %s)", lib.Name)
		}

		// 用户可感知的主阶段固定为：1. 入库进度 → 2. AI 整理进度 → 3. 元数据刮削进度。
		// 合并/匹配等收尾任务不单独占主进度阶段，避免刷新后进度条顺序混乱。
		stepTotal := 3
		needAIOrganize := s.scanPostProcess != nil && lib.AutoOrganizeMode != model.AutoOrganizeOff
		stepCurrent := 0

		// 广播阶段事件的辅助函数
		broadcastPhase := func(phase, message string) {
			stepCurrent++
			data := &ScanPhaseData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       phase,
				StepCurrent: stepCurrent,
				StepTotal:   stepTotal,
				Message:     message,
			}
			s.setScanPhaseState(data)
			if s.wsHub != nil {
				s.wsHub.BroadcastEvent(EventScanPhase, data)
			}
		}
		updatePhaseProgress := func(phase string, current, total int, message string) {
			data := &ScanPhaseData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       phase,
				StepCurrent: stepCurrent,
				StepTotal:   stepTotal,
				Current:     current,
				Total:       total,
				Message:     message,
			}
			s.setScanPhaseState(data)
			if s.wsHub != nil {
				s.wsHub.BroadcastEvent(EventScanPhase, data)
			}
		}

		// 第一步：扫描文件。scan_completed / onScanComplete 由完整多阶段流程结束时统一发送，避免前端提前刷新。
		broadcastPhase("scanning", fmt.Sprintf("正在扫描媒体文件: %s", lib.Name))
		count, err := s.scanner.ScanLibraryWithOptions(lib, ScanLibraryOptions{SuppressCompletedEvent: true, SuppressCompletionCallback: true})
		if err != nil {
			s.logger.Errorf("扫描媒体库失败: %s, 错误: %v", lib.Name, err)
			return
		}

		now := time.Now()
		lib.LastScan = &now
		s.repo.Update(lib)

		s.logger.Infof("媒体库 %s 文件扫描完成，新增 %d 个媒体", lib.Name, count)
		updatePhaseProgress("scanning", count, count, fmt.Sprintf("入库完成: 新增 %d 个媒体", count))

		// 第二步：AI 自动整理 — 识别、归类、命名与可选硬链接。
		broadcastPhase("ai_organizing", fmt.Sprintf("正在 AI 整理: %s", lib.Name))
		if needAIOrganize {
			okCount, totalAI, err := s.scanPostProcess.ProcessUnprocessedLibraryWithProgress(id, func(current, total, okCount int) {
				updatePhaseProgress("ai_organizing", current, total, fmt.Sprintf("AI 整理进度: [%d/%d] 成功 %d", current, total, okCount))
			})
			if err != nil {
				s.logger.Warnf("媒体库 %s AI 自动整理失败: %v", lib.Name, err)
			} else if totalAI == 0 {
				updatePhaseProgress("ai_organizing", 1, 1, fmt.Sprintf("媒体库 %s 无新增待 AI 整理媒体，已跳过", lib.Name))
				s.logger.Infof("媒体库 %s 无新增待 AI 整理媒体，跳过 AI 调用", lib.Name)
			} else {
				s.logger.Infof("媒体库 %s AI 自动整理完成: 成功 %d / 待整理 %d", lib.Name, okCount, totalAI)
				s.syncSeriesTitlesFromEpisodes(id)
			}
		} else {
			updatePhaseProgress("ai_organizing", 1, 1, fmt.Sprintf("媒体库 %s 已跳过 AI 整理", lib.Name))
		}

		// 第三步：自动刮削元数据（如果媒体库开启了自动刮削）
		broadcastPhase("scraping", fmt.Sprintf("正在刮削元数据: %s", lib.Name))
		if lib.AutoScrapeMetadata {
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
			updatePhaseProgress("scraping", 1, 1, fmt.Sprintf("媒体库 %s 已跳过元数据刮削", lib.Name))
		}

		// 收尾：自动合并同名剧集（如「女神咖啡厅 第一季」和「女神咖啡厅 第二季」）
		if s.seriesService != nil && (lib.Type == "tvshow" || lib.Type == "mixed") {
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

		// 收尾：自动匹配电影系列合集（在刮削完成后执行，确保标题已更新）
		if s.collectionService != nil && lib.Type != "tvshow" {
			collCount, err := s.collectionService.AutoMatchCollections()
			if err != nil {
				s.logger.Warnf("媒体库 %s 自动匹配合集失败: %v", lib.Name, err)
			} else if collCount > 0 {
				s.logger.Infof("媒体库 %s 自动创建 %d 个电影系列合集", lib.Name, collCount)
			}

			// 同片多版本折叠：将同一部电影的不同版本标记为 duplicate_of，
			// 让前端列表默认只展示主版本，避免同一部片占据 N 张卡片。
			if marked, err := s.scanner.MarkDuplicates(id); err != nil {
				s.logger.Warnf("媒体库 %s 标记重复版本失败: %v", lib.Name, err)
			} else if marked > 0 {
				s.logger.Infof("媒体库 %s 标记 %d 个同片副本（列表默认隐藏）", lib.Name, marked)
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
			s.wsHub.BroadcastEvent(EventScanCompleted, &ScanProgressData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       "scanning",
				NewFound:    count,
				Message:     fmt.Sprintf("媒体库 %s 处理完成，新增 %d 个媒体", lib.Name, count),
			})
		}
		s.logger.Infof("媒体库 %s 处理完成，共新增 %d 个媒体", lib.Name, count)
		s.scanner.NotifyScanComplete(id)
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
	// 清理扫描归类记录（虚拟归类与命名映射）
	if s.scanClassificationRepo != nil {
		if deleted, err := s.scanClassificationRepo.DeleteByLibraryID(id); err != nil {
			s.logger.Errorf("删除媒体库 %s 的扫描归类记录失败: %v", libName, err)
		} else if deleted > 0 {
			s.logger.Debugf("已清理 %d 条扫描归类记录（媒体库 %s）", deleted, libName)
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

func (s *LibraryService) cleanLibraryIndexData(libraryID string) error {
	// 关联表必须在 media/series 删除前清理，因为多数清理语句依赖子查询 media.library_id / series.library_id。
	if s.scanClassificationRepo != nil {
		if _, err := s.scanClassificationRepo.DeleteByLibraryID(libraryID); err != nil {
			return fmt.Errorf("清理分类记录失败: %w", err)
		}
	}
	if s.mediaPersonRepo != nil {
		if err := s.mediaPersonRepo.DeleteByLibraryMediaIDs(libraryID); err != nil {
			return fmt.Errorf("清理媒体演职员关联失败: %w", err)
		}
		if err := s.mediaPersonRepo.DeleteByLibrarySeriesIDs(libraryID); err != nil {
			return fmt.Errorf("清理剧集演职员关联失败: %w", err)
		}
	}
	if s.favRepo != nil {
		if err := s.favRepo.DeleteByLibraryMediaIDs(libraryID); err != nil {
			return fmt.Errorf("清理收藏记录失败: %w", err)
		}
	}
	if s.historyRepo != nil {
		if err := s.historyRepo.DeleteByLibraryMediaIDs(libraryID); err != nil {
			return fmt.Errorf("清理观看历史失败: %w", err)
		}
	}
	if err := s.mediaRepo.DeleteByLibraryID(libraryID); err != nil {
		return fmt.Errorf("清理媒体记录失败: %w", err)
	}
	if err := s.seriesRepo.DeleteByLibraryID(libraryID); err != nil {
		return fmt.Errorf("清理剧集合集记录失败: %w", err)
	}
	return nil
}

func (s *LibraryService) cleanOrganizeOutputDir(lib *model.Library) {
	if lib == nil {
		return
	}
	outputDir := strings.TrimSpace(lib.OrganizeOutputDir)
	if outputDir == "" {
		return
	}
	for _, p := range lib.AllPaths() {
		if pathEqual(outputDir, p) {
			s.logger.Warnf("跳过清理硬链接输出目录：输出目录与媒体源目录相同 path=%s", outputDir)
			return
		}
	}
	for _, sub := range []string{"Movies", "TV Shows", "_unsorted"} {
		p := filepath.Join(outputDir, sub)
		if err := os.RemoveAll(p); err != nil {
			s.logger.Warnf("清理硬链接输出子目录失败: %s err=%v", p, err)
		}
	}
	s.logger.Infof("硬链接输出目录已清理: %s", outputDir)
}

// syncSeriesTitlesFromEpisodes 用 AI 整理后的单集标题回填剧集合集标题。
// 扫描器建 Series 时经常拿到发布组目录名；若不回填，TMDb 剧集刮削会用 DBD-Raws 这类脏标题搜索。
func (s *LibraryService) syncSeriesTitlesFromEpisodes(libraryID string) {
	seriesList, err := s.seriesRepo.ListByLibraryID(libraryID)
	if err != nil || len(seriesList) == 0 {
		return
	}
	for i := range seriesList {
		ser := &seriesList[i]
		if isTrustedMediaTitle(ser.Title, nil) {
			continue
		}
		episodes, err := s.mediaRepo.ListBySeriesID(ser.ID)
		if err != nil || len(episodes) == 0 {
			continue
		}
		counts := map[string]int{}
		years := map[string]int{}
		for _, ep := range episodes {
			title := strings.TrimSpace(NormalizeSeriesTitle(ep.Title))
			if title == "" || !isTrustedMediaTitle(title, &ep) {
				continue
			}
			counts[title]++
			if ep.Year > 0 && years[title] == 0 {
				years[title] = ep.Year
			}
		}
		bestTitle := ""
		bestCount := 0
		for title, count := range counts {
			if count > bestCount {
				bestTitle = title
				bestCount = count
			}
		}
		if bestTitle == "" {
			continue
		}
		if strings.TrimSpace(ser.OrigTitle) == "" {
			ser.OrigTitle = ser.Title
		}
		ser.Title = bestTitle
		if y := years[bestTitle]; y > 0 && ser.Year == 0 {
			ser.Year = y
		}
		if err := s.seriesRepo.Update(ser); err != nil {
			s.logger.Warnf("同步剧集合集标题失败 series_id=%s title=%q err=%v", ser.ID, bestTitle, err)
		} else {
			s.logger.Infof("剧集合集标题已同步: %s -> %s", ser.OrigTitle, ser.Title)
		}
	}
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
		defer s.clearScanPhaseState(id)

		// 异步检查路径（网络驱动器可能较慢，不阻塞 HTTP 响应）
		for _, p := range lib.AllPaths() {
			info, err := os.Stat(p)
			if err != nil {
				s.logger.Errorf("媒体库路径不可访问: %s (媒体库: %s) err=%v", p, lib.Name, err)
				return
			}
			if !info.IsDir() {
				s.logger.Errorf("媒体库路径不是目录: %s (媒体库: %s)", p, lib.Name)
				return
			}
		}

		// 用户可感知的主阶段固定为：1. 入库进度 → 2. AI 整理进度 → 3. 元数据刮削进度。
		// 重建索引的清理、合并、匹配等属于收尾任务，不单独占主进度阶段。
		stepTotal := 3
		needAIOrganize := s.scanPostProcess != nil && lib.AutoOrganizeMode != model.AutoOrganizeOff
		stepCurrent := 0

		broadcastPhase := func(phase, message string) {
			stepCurrent++
			data := &ScanPhaseData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       phase,
				StepCurrent: stepCurrent,
				StepTotal:   stepTotal,
				Message:     message,
			}
			s.setScanPhaseState(data)
			if s.wsHub != nil {
				s.wsHub.BroadcastEvent(EventScanPhase, data)
			}
		}
		updatePhaseProgress := func(phase string, current, total int, message string) {
			data := &ScanPhaseData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       phase,
				StepCurrent: stepCurrent,
				StepTotal:   stepTotal,
				Current:     current,
				Total:       total,
				Message:     message,
			}
			s.setScanPhaseState(data)
			if s.wsHub != nil {
				s.wsHub.BroadcastEvent(EventScanPhase, data)
			}
		}

		// 清理旧索引属于重建索引准备工作，不单独占主进度阶段。
		if err := s.cleanLibraryIndexData(id); err != nil {
			s.logger.Errorf("清除媒体库旧索引数据失败: %s, 错误: %v", lib.Name, err)
			return
		}
		s.logger.Infof("已清除媒体库 %s 的旧索引数据（含媒体、合集、分类、演职员、收藏和播放历史关联）", lib.Name)

		// 第一步：重新扫描文件。scan_completed / onScanComplete 由完整重建流程结束时统一发送，避免前端提前刷新。
		broadcastPhase("scanning", fmt.Sprintf("正在扫描媒体文件: %s", lib.Name))
		count, err := s.scanner.ScanLibraryWithOptions(lib, ScanLibraryOptions{SuppressCompletedEvent: true, SuppressCompletionCallback: true})
		if err != nil {
			s.logger.Errorf("重建索引扫描失败: %s, 错误: %v", lib.Name, err)
			return
		}

		now := time.Now()
		lib.LastScan = &now
		s.repo.Update(lib)

		s.logger.Infof("媒体库 %s 文件扫描完成，共 %d 个媒体，继续执行重建后处理", lib.Name, count)
		updatePhaseProgress("scanning", count, count, fmt.Sprintf("入库完成: 共 %d 个媒体", count))

		// 第二步：AI 自动整理。必须在刮削前同步标题/路径/硬链接。
		if needAIOrganize {
			s.cleanOrganizeOutputDir(lib)
		}
		broadcastPhase("ai_organizing", fmt.Sprintf("正在 AI 整理: %s", lib.Name))
		if needAIOrganize {
			okCount, totalAI, err := s.scanPostProcess.ProcessUnprocessedLibraryWithProgress(id, func(current, total, okCount int) {
				updatePhaseProgress("ai_organizing", current, total, fmt.Sprintf("AI 整理进度: [%d/%d] 成功 %d", current, total, okCount))
			})
			if err != nil {
				s.logger.Warnf("媒体库 %s 重建索引 AI 自动整理失败: %v", lib.Name, err)
			} else if totalAI == 0 {
				updatePhaseProgress("ai_organizing", 1, 1, fmt.Sprintf("媒体库 %s 无待 AI 整理媒体，已跳过", lib.Name))
				s.logger.Infof("媒体库 %s 重建索引无待 AI 整理媒体", lib.Name)
			} else {
				s.logger.Infof("媒体库 %s 重建索引 AI 自动整理完成: 成功 %d / 待整理 %d", lib.Name, okCount, totalAI)
				s.syncSeriesTitlesFromEpisodes(id)
			}
		} else {
			updatePhaseProgress("ai_organizing", 1, 1, fmt.Sprintf("媒体库 %s 已跳过 AI 整理", lib.Name))
		}

		// 第三步：自动刮削元数据（如果媒体库开启了自动刮削）
		broadcastPhase("scraping", fmt.Sprintf("正在刮削元数据: %s", lib.Name))
		if lib.AutoScrapeMetadata {
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
			updatePhaseProgress("scraping", 1, 1, fmt.Sprintf("媒体库 %s 已跳过元数据刮削", lib.Name))
		}

		// 重建索引后自动合并同名剧集
		if s.seriesService != nil && (lib.Type == "tvshow" || lib.Type == "mixed") {
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
			collCount, err := s.collectionService.AutoMatchCollections()
			if err != nil {
				s.logger.Warnf("媒体库 %s 重建索引后自动匹配合集失败: %v", lib.Name, err)
			} else if collCount > 0 {
				s.logger.Infof("媒体库 %s 重建索引后自动创建 %d 个电影系列合集", lib.Name, collCount)
			}

			// 同片多版本折叠：同 Scan 流程一致
			if marked, err := s.scanner.MarkDuplicates(id); err != nil {
				s.logger.Warnf("媒体库 %s 重建索引后标记重复版本失败: %v", lib.Name, err)
			} else if marked > 0 {
				s.logger.Infof("媒体库 %s 重建索引后标记 %d 个同片副本（列表默认隐藏）", lib.Name, marked)
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
			s.wsHub.BroadcastEvent(EventScanCompleted, &ScanProgressData{
				LibraryID:   id,
				LibraryName: lib.Name,
				Phase:       "scanning",
				NewFound:    count,
				Message:     fmt.Sprintf("媒体库 %s 重建索引完成，共 %d 个媒体", lib.Name, count),
			})
		}
		s.logger.Infof("媒体库 %s 重建索引完成，共 %d 个媒体", lib.Name, count)
		s.scanner.NotifyScanComplete(id)
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
