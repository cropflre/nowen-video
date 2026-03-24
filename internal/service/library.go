package service

import (
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// LibraryService 媒体库服务
type LibraryService struct {
	repo      *repository.LibraryRepo
	mediaRepo *repository.MediaRepo
	scanner   *ScannerService
	metadata  *MetadataService
	logger    *zap.SugaredLogger
	scanning  sync.Map // 记录正在扫描的媒体库ID
}

func NewLibraryService(
	repo *repository.LibraryRepo,
	mediaRepo *repository.MediaRepo,
	scanner *ScannerService,
	metadata *MetadataService,
	logger *zap.SugaredLogger,
) *LibraryService {
	return &LibraryService{
		repo:      repo,
		mediaRepo: mediaRepo,
		scanner:   scanner,
		metadata:  metadata,
		logger:    logger,
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
		if lib.Type == "tvshow" {
			// 剧集库：优先刮削合集元数据，然后同步到各集
			seriesSuccess, seriesFailed := s.metadata.ScrapeAllSeries(id)
			if seriesSuccess > 0 || seriesFailed > 0 {
				s.logger.Infof("媒体库 %s 剧集合集刮削: 成功 %d, 失败 %d", lib.Name, seriesSuccess, seriesFailed)
			}
		} else {
			// 电影库：逐个刮削
			mediaList, err := s.mediaRepo.ListByLibraryID(id)
			if err == nil && len(mediaList) > 0 {
				success, failed := s.metadata.ScrapeLibrary(id, mediaList)
				if success > 0 || failed > 0 {
					s.logger.Infof("媒体库 %s 元数据刮削: 成功 %d, 失败 %d", lib.Name, success, failed)
				}
			}
		}
	}()

	return nil
}

// Delete 删除媒体库
func (s *LibraryService) Delete(id string) error {
	return s.repo.Delete(id)
}

// Update 更新媒体库信息
func (s *LibraryService) Update(lib *model.Library) error {
	return s.repo.Update(lib)
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

		// 第一步：清除该媒体库下所有旧媒体记录
		if err := s.mediaRepo.DeleteByLibraryID(id); err != nil {
			s.logger.Errorf("清除媒体库旧记录失败: %s, 错误: %v", lib.Name, err)
			return
		}
		s.logger.Infof("已清除媒体库 %s 的旧索引数据", lib.Name)

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
		if lib.Type == "tvshow" {
			seriesSuccess, seriesFailed := s.metadata.ScrapeAllSeries(id)
			if seriesSuccess > 0 || seriesFailed > 0 {
				s.logger.Infof("媒体库 %s 重建索引刮削: 成功 %d, 失败 %d", lib.Name, seriesSuccess, seriesFailed)
			}
		} else {
			mediaList, err := s.mediaRepo.ListByLibraryID(id)
			if err == nil && len(mediaList) > 0 {
				success, failed := s.metadata.ScrapeLibrary(id, mediaList)
				if success > 0 || failed > 0 {
					s.logger.Infof("媒体库 %s 重建索引刮削: 成功 %d, 失败 %d", lib.Name, success, failed)
				}
			}
		}
	}()

	return nil
}
