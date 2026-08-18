package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// FileWatcherService 文件系统实时监听服务
// 监听媒体库目录下的文件变更（新增/删除/重命名），自动触发增量扫描
type FileWatcherService struct {
	cfg             *config.Config
	logger          *zap.SugaredLogger
	libRepo         *repository.LibraryRepo
	mediaRepo       *repository.MediaRepo
	seriesRepo      *repository.SeriesRepo
	scanner         *ScannerService
	metadata        *MetadataService
	scanPostProcess *ScanPostProcessService
	wsHub           *WSHub

	watcher     *fsnotify.Watcher
	mu          sync.Mutex
	watching    map[string]string              // root path -> libraryID 正在监听的媒体库根目录
	watchedDirs map[string]map[string]struct{} // libraryID -> 已向 fsnotify 注册的目录，删除时直接复用，禁止再次遍历磁盘
	debounce    map[string]*time.Timer         // libraryID -> debounce timer
	stopCh      chan struct{}
}

// NewFileWatcherService 创建文件监听服务
func NewFileWatcherService(
	cfg *config.Config,
	logger *zap.SugaredLogger,
	libRepo *repository.LibraryRepo,
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	scanner *ScannerService,
	metadata *MetadataService,
) *FileWatcherService {
	return &FileWatcherService{
		cfg:         cfg,
		logger:      logger,
		libRepo:     libRepo,
		mediaRepo:   mediaRepo,
		seriesRepo:  seriesRepo,
		scanner:     scanner,
		metadata:    metadata,
		watching:    make(map[string]string),
		watchedDirs: make(map[string]map[string]struct{}),
		debounce:    make(map[string]*time.Timer),
		stopCh:      make(chan struct{}),
	}
}

// SetWSHub 设置 WebSocket Hub
func (fw *FileWatcherService) SetWSHub(hub *WSHub) {
	fw.wsHub = hub
}

// SetScanPostProcessService 设置扫描后处理服务，用于增量扫描后同步执行 AI 整理。
func (fw *FileWatcherService) SetScanPostProcessService(spp *ScanPostProcessService) {
	fw.scanPostProcess = spp
}

// Start 启动文件监听服务
// 自动为所有 EnableFileWatch=true 的媒体库注册监听
func (fw *FileWatcherService) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	fw.watcher = watcher

	// 获取所有媒体库，为启用了文件监控的媒体库注册监听
	libs, err := fw.libRepo.List()
	if err != nil {
		fw.logger.Errorf("获取媒体库列表失败（文件监听初始化）: %v", err)
	} else {
		for _, lib := range libs {
			if lib.EnableFileWatch {
				fw.WatchLibrary(&lib)
			}
		}
	}

	// 启动事件处理协程
	go fw.eventLoop()

	fw.logger.Infof("文件监听服务已启动，监听 %d 个媒体库", fw.GetWatchingCount())
	return nil
}

// Stop 停止文件监听服务
func (fw *FileWatcherService) Stop() {
	close(fw.stopCh)
	if fw.watcher != nil {
		fw.watcher.Close()
	}
	fw.mu.Lock()
	for _, timer := range fw.debounce {
		timer.Stop()
	}
	fw.mu.Unlock()
	fw.logger.Info("文件监听服务已停止")
}

// WatchLibrary 为指定媒体库注册文件监听（支持多路径）
func (fw *FileWatcherService) WatchLibrary(lib *model.Library) {
	if fw.watcher == nil {
		return
	}

	allPaths := lib.AllPaths()
	if len(allPaths) == 0 {
		return
	}

	for _, libPath := range allPaths {
		// 根目录先登记到内存，避免文件系统遍历期间长期持有全局锁。
		fw.mu.Lock()
		if _, exists := fw.watching[libPath]; exists {
			fw.mu.Unlock()
			continue
		}
		fw.watching[libPath] = lib.ID
		if _, exists := fw.watchedDirs[lib.ID]; !exists {
			fw.watchedDirs[lib.ID] = make(map[string]struct{})
		}
		fw.mu.Unlock()

		// 仅注册阶段遍历目录；实际注册成功的目录会记录到 watchedDirs，
		// 删除媒体库时直接使用该内存索引，不再对大容量 NAS/机械盘做第二次 Walk。
		watchCount := 0
		err := filepath.Walk(libPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的目录
			}
			if !info.IsDir() {
				return nil
			}
			// 忽略隐藏目录
			if strings.HasPrefix(filepath.Base(path), ".") && path != libPath {
				return filepath.SkipDir
			}
			if watchErr := fw.watcher.Add(path); watchErr != nil {
				fw.logger.Debugf("监听目录失败: %s - %v", path, watchErr)
				return nil
			}

			fw.mu.Lock()
			if dirs := fw.watchedDirs[lib.ID]; dirs != nil {
				dirs[path] = struct{}{}
			}
			fw.mu.Unlock()
			watchCount++
			return nil
		})

		if err != nil {
			fw.logger.Errorf("遍历媒体库目录失败: %s - %v", libPath, err)
			fw.UnwatchLibrary(libPath)
			continue
		}

		fw.logger.Infof("已开始监听媒体库: %s (%s), 监听 %d 个目录", lib.Name, libPath, watchCount)
	}
}

// UnwatchLibrary 取消指定媒体库的文件监听。
// 注意：这里不能再 filepath.Walk(libPath)。对于数千目录的 NAS/机械盘，
// 重新遍历目录树会让“删除媒体库”被文件系统 I/O 阻塞几十秒甚至数分钟。
func (fw *FileWatcherService) UnwatchLibrary(libPath string) {
	if fw.watcher == nil {
		return
	}

	fw.mu.Lock()
	libID, exists := fw.watching[libPath]
	if !exists {
		fw.mu.Unlock()
		return
	}

	// 从注册阶段维护的内存索引中摘出属于当前根目录的 watcher。
	// 先更新 bookkeeping 再释放锁，实际 fsnotify.Remove 在锁外执行，
	// 避免大量 watcher 注销阻塞事件查找与其他媒体库操作。
	dirsToRemove := make([]string, 0)
	if dirs := fw.watchedDirs[libID]; dirs != nil {
		for dir := range dirs {
			if pathWithinRoot(dir, libPath) {
				dirsToRemove = append(dirsToRemove, dir)
				delete(dirs, dir)
			}
		}
		if len(dirs) == 0 {
			delete(fw.watchedDirs, libID)
		}
	}
	delete(fw.watching, libPath)

	// 取消可能还在等待的防抖定时器
	if timer, ok := fw.debounce[libID]; ok {
		timer.Stop()
		delete(fw.debounce, libID)
	}
	fw.mu.Unlock()

	for _, dir := range dirsToRemove {
		if err := fw.watcher.Remove(dir); err != nil {
			// 目录可能已经被用户移除，watcher 也可能已经自行失效；
			// 注销失败不能阻塞媒体库删除。
			fw.logger.Debugf("取消目录监听失败: %s - %v", dir, err)
		}
	}

	fw.logger.Infof("已停止监听媒体库: %s, 注销 %d 个目录", libPath, len(dirsToRemove))
}

// pathWithinRoot 判断 path 是否位于 root 本身或其子目录中。
// 使用 filepath.Rel 而不是字符串前缀，避免 X:\\movie 错误匹配 X:\\movie-backup。
func pathWithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

// eventLoop 事件处理循环
func (fw *FileWatcherService) eventLoop() {
	for {
		select {
		case <-fw.stopCh:
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}
			fw.handleEvent(event)

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Warnf("文件监听错误: %v", err)
		}
	}
}

// handleEvent 处理单个文件系统事件
func (fw *FileWatcherService) handleEvent(event fsnotify.Event) {
	// 只关注创建、删除、重命名事件
	if event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	// 判断是否为视频文件或目录
	ext := strings.ToLower(filepath.Ext(event.Name))
	isVideo := supportedExts[ext]
	isDir := false
	if info, err := os.Stat(event.Name); err == nil {
		isDir = info.IsDir()
	}

	// 非视频文件且非目录，忽略
	if !isVideo && !isDir {
		return
	}

	// 先确定归属媒体库。根目录映射在 WatchLibrary 开始时就已登记，
	// 因此新建子目录可以立即归档到正确的 watchedDirs 索引。
	libraryID := fw.findLibraryID(event.Name)
	if libraryID == "" {
		return
	}

	// 如果是新创建的目录，自动添加到监听列表并同步维护内存索引，
	// 确保后续删除媒体库时无需重新遍历磁盘即可完整注销。
	if isDir && event.Op&fsnotify.Create != 0 {
		if err := fw.watcher.Add(event.Name); err != nil {
			fw.logger.Debugf("监听新目录失败: %s - %v", event.Name, err)
		} else {
			fw.mu.Lock()
			if _, exists := fw.watchedDirs[libraryID]; !exists {
				fw.watchedDirs[libraryID] = make(map[string]struct{})
			}
			fw.watchedDirs[libraryID][event.Name] = struct{}{}
			fw.mu.Unlock()
		}
	}

	fw.logger.Debugf("文件变更: %s [%s] -> 媒体库 %s",
		event.Name,
		event.Op.String(),
		libraryID)

	// 对 Remove/Rename 事件：即时删除对应的媒体记录（视频文件级别），
	// 避免等防抖 3 秒扫描时 UI 仍显示已不存在的文件（幽灵记录）。
	if isVideo && event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		if m, err := fw.mediaRepo.FindByFilePath(event.Name); err == nil && m != nil {
			if delErr := fw.mediaRepo.DeleteByID(m.ID); delErr != nil {
				fw.logger.Warnf("即时删除失效媒体记录失败: %s, 错误: %v", event.Name, delErr)
			} else {
				fw.logger.Infof("即时删除失效媒体记录（文件已移除/重命名）: %s", event.Name)
				// 通知前端立即刷新
				if fw.wsHub != nil {
					fw.wsHub.BroadcastEvent(EventLibraryUpdated, &LibraryChangedData{
						LibraryID:   libraryID,
						LibraryName: "",
						Action:      "media_removed",
						Message:     "文件已移除: " + filepath.Base(event.Name),
					})
				}
			}
		}
	}

	// 防抖：3秒内合并多个事件，只触发一次增量扫描
	fw.debounceScan(libraryID)
}

// findLibraryID 根据文件路径查找所属媒体库 ID
func (fw *FileWatcherService) findLibraryID(filePath string) string {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	for watchedPath, libID := range fw.watching {
		if pathWithinRoot(filePath, watchedPath) {
			return libID
		}
	}
	return ""
}

// debounceScan 防抖触发增量扫描
// 收到文件变更事件后等待 3 秒，3 秒内如果又有新事件则重置计时器
// 这样可以把批量操作（如拷贝一个文件夹的多个文件）合并为一次扫描
func (fw *FileWatcherService) debounceScan(libraryID string) {
	fw.mu.Lock()
	defer fw.mu.Unlock()

	// 如果已有定时器，重置
	if timer, ok := fw.debounce[libraryID]; ok {
		timer.Stop()
	}

	// 创建新的防抖定时器（3秒后触发）
	fw.debounce[libraryID] = time.AfterFunc(3*time.Second, func() {
		fw.triggerIncrementalScan(libraryID)
	})
}

// triggerIncrementalScan 触发增量扫描
func (fw *FileWatcherService) triggerIncrementalScan(libraryID string) {
	lib, err := fw.libRepo.FindByID(libraryID)
	if err != nil {
		fw.logger.Errorf("文件监听触发扫描失败（媒体库不存在）: %s", libraryID)
		return
	}

	fw.logger.Infof("文件变更触发增量扫描: %s (主路径: %s)", lib.Name, lib.Path)

	// 通过 WebSocket 通知前端
	if fw.wsHub != nil {
		fw.wsHub.BroadcastEvent(EventLibraryUpdated, &LibraryChangedData{
			LibraryID:   libraryID,
			LibraryName: lib.Name,
			Action:      "file_changed",
			Message:     "检测到文件变更，正在增量扫描...",
		})
	}

	// 执行扫描（复用已有的扫描逻辑）。增量链路会在 AI 整理/刮削结束后再通知刷新。
	count, err := fw.scanner.ScanLibraryWithOptions(lib, ScanLibraryOptions{SuppressCompletedEvent: true, SuppressCompletionCallback: true})
	if err != nil {
		fw.logger.Errorf("增量扫描失败: %s - %v", lib.Name, err)
		return
	}

	// 更新最后扫描时间
	now := time.Now()
	lib.LastScan = &now
	fw.libRepo.Update(lib)

	fw.logger.Infof("增量扫描完成: %s, 新增 %d 个媒体", lib.Name, count)

	// 增量扫描后只处理新增且未整理过的媒体，修正标题并创建硬链接，再进入刮削。
	if count > 0 && fw.scanPostProcess != nil && lib.AutoOrganizeMode != model.AutoOrganizeOff {
		if okCount, totalAI, err := fw.scanPostProcess.ProcessUnprocessedLibraryWithProgress(libraryID, nil); err != nil {
			fw.logger.Warnf("增量 AI 整理失败: %s - %v", lib.Name, err)
		} else if totalAI == 0 {
			fw.logger.Infof("增量扫描无新增待 AI 整理媒体: %s", lib.Name)
		} else {
			fw.logger.Infof("增量 AI 整理完成: %s, 成功 %d / 待整理 %d", lib.Name, okCount, totalAI)
		}
	}

	// 如果有新增媒体，自动触发刮削
	if count > 0 {
		go func() {
			if lib.Type == "tvshow" {
				success, failed := fw.metadata.ScrapeAllSeries(libraryID)
				if success > 0 || failed > 0 {
					fw.logger.Infof("增量刮削完成: %s, 成功 %d, 失败 %d", lib.Name, success, failed)
				}
			} else {
				mediaList, err := fw.mediaRepo.ListByLibraryID(libraryID)
				if err == nil && len(mediaList) > 0 {
					success, failed := fw.metadata.ScrapeLibrary(libraryID, mediaList)
					if success > 0 || failed > 0 {
						fw.logger.Infof("增量刮削完成: %s, 成功 %d, 失败 %d", lib.Name, success, failed)
					}
				}
			}

			// 通知前端刷新
			if fw.wsHub != nil {
				fw.wsHub.BroadcastEvent(EventLibraryUpdated, &LibraryChangedData{
					LibraryID:   libraryID,
					LibraryName: lib.Name,
					Action:      "updated",
					Message:     "增量扫描与刮削已完成",
				})
				fw.wsHub.BroadcastEvent(EventScanCompleted, &ScanProgressData{
					LibraryID:   libraryID,
					LibraryName: lib.Name,
					Phase:       "scanning",
					NewFound:    count,
					Message:     fmt.Sprintf("增量扫描完成: %s, 新增 %d 个媒体", lib.Name, count),
				})
			}
			fw.scanner.NotifyScanComplete(libraryID)
		}()
	} else {
		if fw.wsHub != nil {
			fw.wsHub.BroadcastEvent(EventScanCompleted, &ScanProgressData{
				LibraryID:   libraryID,
				LibraryName: lib.Name,
				Phase:       "scanning",
				NewFound:    count,
				Message:     fmt.Sprintf("增量扫描完成: %s, 新增 %d 个媒体", lib.Name, count),
			})
		}
		fw.scanner.NotifyScanComplete(libraryID)
	}

	// 清理防抖定时器
	fw.mu.Lock()
	delete(fw.debounce, libraryID)
	fw.mu.Unlock()
}

// GetWatchingCount 获取当前监听的媒体库数量
func (fw *FileWatcherService) GetWatchingCount() int {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	return len(fw.watching)
}
