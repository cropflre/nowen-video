package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"alex-desktop/config"
	"alex-desktop/model"
	"alex-desktop/repository"
	"alex-desktop/service"
	"fmt"
	"regexp"
	"sort"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/glebarez/sqlite"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// App struct
type App struct {
	ctx     context.Context
	db      *gorm.DB
	repos   *repository.Repositories
	scanner *service.ScannerService
	logger  *zap.SugaredLogger
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 1. 初始化极简 logger
	l, _ := zap.NewDevelopment()
	a.logger = l.Sugar()

	// 2. 初始化持久层配置 (SQLite本地库)
	dbPath := "alex.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		a.logger.Fatalf("连接数据库失败: %v", err)
	}
	a.db = db

	// 初始化库表结构
	err = model.AutoMigrate(db)
	if err != nil {
		a.logger.Fatalf("数据库迁移失败: %v", err)
	}

	// 3. 构建 Repositories 单例
	a.repos = repository.NewRepositories(a.db)

	// 4. 注入之前写好的最小化 Shim 层
	cfg := config.NewConfig()
	wsHub := service.NewWSHub(ctx)

	// 5. 将桌面级组件全部注入核心 Scanner
	a.scanner = service.NewScannerService(a.repos.Media, a.repos.Series, cfg, a.logger)
	a.scanner.SetWSHub(wsHub)
	// a.scanner.SetMatchRuleRepo(a.repos.MatchRule)

	a.logger.Infof("Application backend started successfully! DB: %s", dbPath)
}

// -----------------------------------------------------
// Wails Bind 导出的暴露核心方法
// -----------------------------------------------------

func (a *App) GetLibraries() ([]model.Library, error) {
	return a.repos.Library.List()
}

func (a *App) CreateLibrary(lib *model.Library) error {
	return a.repos.Library.Create(lib)
}

func (a *App) UpdateLibrary(lib *model.Library) error {
	return a.repos.Library.Update(lib)
}

func (a *App) ScanLibrary(libraryID string) error {
	lib, err := a.repos.Library.FindByID(libraryID)
	if err != nil {
		return err
	}
	// 利用本地并行协程进行扫描防卡死
	go func() {
		_, err := a.scanner.ScanLibrary(lib)
		if err != nil {
			a.logger.Errorf("Scan error: %v", err)
		}
	}()
	return nil
}

type StatsItem struct {
	Name        string `json:"name"`
	Count       int    `json:"count"`
	Image       string `json:"image"`
	FilterValue string `json:"filter_value"`
}

func (a *App) GetMediaList(libraryID string, page, size int, sortBy, sortOrder, keyword string, filterType, filterValue string) (interface{}, error) {
	// 扩展原生后端的检索逻辑，增加分类过滤支持
	query := a.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR orig_title LIKE ? OR file_path LIKE ?", "%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	// 处理 4 种统计过滤
	switch filterType {
	case "directory":
		query = query.Where("file_path LIKE ?", filterValue+"%")
	case "actor":
		query = query.Joins("JOIN media_person ON media_person.media_id = media.id").Where("media_person.person_id = ?", filterValue)
	case "genre":
		query = query.Where("genres LIKE ?", "%"+filterValue+"%")
	case "series":
		query = query.Where("series_id = ?", filterValue)
	case "watched":
		// 使用 watch_histories 表关联：desktop_user 且 completed 为真
		query = query.Joins("JOIN watch_histories ON watch_histories.media_id = media.id").
			Where("watch_histories.user_id = ? AND watch_histories.completed = ?", "desktop_user", true)
	case "favorite":
		// 使用 favorites 表关联
		query = query.Joins("JOIN favorites ON favorites.media_id = media.id").
			Where("favorites.user_id = ?", "desktop_user")
	}

	var total int64
	query.Count(&total)

	var media []model.Media
	// 简单的排序处理
	sortStr := "created_at DESC"
	if sortBy != "" {
		dir := "DESC"
		if strings.ToLower(sortOrder) == "asc" {
			dir = "ASC"
		}
		sortStr = sortBy + " " + dir
	}

	err := query.Order(sortStr).Offset((page - 1) * size).Limit(size).Find(&media).Error
	return map[string]interface{}{
		"items": media,
		"total": total,
	}, err
}

// GetDirectoryStats 获取目录聚合统计
func (a *App) GetDirectoryStats(libraryID string) ([]StatsItem, error) {
	var filePaths []string
	err := a.db.Model(&model.Media{}).Where("library_id = ?", libraryID).Pluck("file_path", &filePaths).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, p := range filePaths {
		dir := filepath.ToSlash(filepath.Dir(p))
		counts[dir]++
	}

	var results []StatsItem
	for dir, count := range counts {
		results = append(results, StatsItem{
			Name:        filepath.Base(dir),
			Count:       count,
			FilterValue: dir,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Count > results[j].Count })
	return results, nil
}

// GetActorStats 获取演员聚合统计
func (a *App) GetActorStats(libraryID string) ([]StatsItem, error) {
	type Result struct {
		ID         string
		Name       string
		ProfileURL string
		Count      int
	}
	var rawResults []Result
	err := a.db.Raw(`
		SELECT p.id, p.name, p.profile_url, COUNT(mp.media_id) as count
		FROM persons p
		JOIN media_person mp ON p.id = mp.person_id
		JOIN media m ON mp.media_id = m.id
		WHERE m.library_id = ?
		GROUP BY p.id
		ORDER BY count DESC
	`, libraryID).Scan(&rawResults).Error
	if err != nil {
		return nil, err
	}

	var results []StatsItem
	for _, r := range rawResults {
		results = append(results, StatsItem{
			Name:        r.Name,
			Count:       r.Count,
			Image:       r.ProfileURL,
			FilterValue: r.ID,
		})
	}
	return results, nil
}

// GetGenreStats 获取类别聚合统计
func (a *App) GetGenreStats(libraryID string) ([]StatsItem, error) {
	var genresStrs []string
	err := a.db.Model(&model.Media{}).Where("library_id = ?", libraryID).Pluck("genres", &genresStrs).Error
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, s := range genresStrs {
		parts := strings.Split(s, ",")
		for _, g := range parts {
			g = strings.TrimSpace(g)
			if g != "" {
				counts[g]++
			}
		}
	}

	var results []StatsItem
	for name, count := range counts {
		results = append(results, StatsItem{
			Name:        name,
			Count:       count,
			FilterValue: name,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].Count > results[j].Count })
	return results, nil
}

// GetSeriesStats 获取系列聚合统计
func (a *App) GetSeriesStats(libraryID string) ([]StatsItem, error) {
	tagRegex := regexp.MustCompile(`<[^>]*>`)

	type Result struct {
		ID         string
		Title      string
		PosterPath string
		Count      int
	}
	var rawResults []Result
	err := a.db.Raw(`
		SELECT s.id, s.title, s.poster_path, COUNT(m.id) as count
		FROM series s
		JOIN media m ON s.id = m.series_id
		WHERE s.library_id = ?
		GROUP BY s.id
		ORDER BY count DESC
	`, libraryID).Scan(&rawResults).Error
	if err != nil {
		return nil, err
	}

	var results []StatsItem
	for _, r := range rawResults {
		// 清洗 Title 中的 XML/HTML 标签
		cleanTitle := tagRegex.ReplaceAllString(r.Title, "")
		cleanTitle = strings.TrimSpace(cleanTitle)
		if cleanTitle == "" {
			continue
		}

		results = append(results, StatsItem{
			Name:        cleanTitle,
			Count:       r.Count,
			Image:       r.PosterPath,
			FilterValue: r.ID,
		})
	}
	return results, nil
}

func (a *App) GetMediaDetail(mediaID string) (*model.Media, error) {
	return a.repos.Media.FindByID(mediaID)
}

type DesktopSettings struct {
	// 播放设置
	PlayerPath        string `json:"player_path"`
	UseExternalPlayer bool   `json:"use_external_player"`
	LoopPlayback      bool   `json:"loop_playback"`
	ReadFileInfo      bool   `json:"read_file_info"`

	// 外观设置
	Theme             string `json:"theme"`
	PosterRadius      int    `json:"poster_radius"`
	BackdropBlur      int    `json:"backdrop_blur"`
	MinWindowWidth    int    `json:"min_window_width"`
	ShowSubtitleTag   bool   `json:"show_subtitle_tag"`
	ShowResolutionTag bool   `json:"show_resolution_tag"`
	ShowCountTag      bool   `json:"show_count_tag"`
	ShowGenreInList   bool   `json:"show_genre_in_list"`
	ShowSeriesInList  bool   `json:"show_series_in_list"`
	StaticLoading     bool   `json:"static_loading"`

	// 快捷设置
	Hotkey        string `json:"hotkey"`
	MinToTray     bool   `json:"min_to_tray"`
	MaxNoTaskbar  bool   `json:"max_no_taskbar"`
	ShowPrompt    bool   `json:"show_prompt"`
	StartWithOS   bool   `json:"start_with_os"`

	// 扫描设置
	SkipNoNfo        bool   `json:"skip_no_nfo"`
	GetResolution    bool   `json:"get_resolution"`
	UseEverything    bool   `json:"use_everything"`
	EverythingAddr   string `json:"everything_addr"`
	ScanFromVideoDir bool   `json:"scan_from_video_dir"`

	// Emby 设置
	EmbyEnabled bool   `json:"emby_enabled"`
	EmbyUser    string `json:"emby_user"`
	EmbyURL     string `json:"emby_url"`
	EmbyAPIKey  string `json:"emby_api_key"`
}

var settingsPath = "settings.json"

func (a *App) GetDesktopSettings() (*DesktopSettings, error) {
	var settings DesktopSettings
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		json.Unmarshal(data, &settings)
	}
	// 设置默认值，确保旧配置文件兼容
	if settings.Theme == "" {
		settings.Theme = "dark"
	}
	if settings.PosterRadius == 0 {
		settings.PosterRadius = 4
	}
	if settings.MinWindowWidth == 0 {
		settings.MinWindowWidth = 1024
	}
	if settings.EverythingAddr == "" {
		settings.EverythingAddr = "http://127.0.0.1:80"
	}
	return &settings, nil
}

func (a *App) UpdateDesktopSettings(settings *DesktopSettings) error {
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(settingsPath, data, 0644)
}

// RestartApp 尝试重启软件
func (a *App) RestartApp() {
	a.logger.Infof("Software restart requested...")
	// 极简且保守实现
	// 在开发模式下，通常最好手动重启，避免干扰 wails dev 的热重载
	executable, err := os.Executable()
	if err != nil {
		a.logger.Errorf("Failed to get executable: %v", err)
		return
	}
	
	// 如果是正常运行模式，尝试拉起新进程
	// 注意：在某些环境下可能失败，此处作为最小尝试
	cmd := exec.Command(executable, os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	err = cmd.Start()
	if err != nil {
		a.logger.Errorf("Failed to restart: %v", err)
	}
	os.Exit(0)
}

// PlayWithExternalPlayer 调用系统原生的外部播放器或配置播放器打开媒体
func (a *App) PlayWithExternalPlayer(mediaID string) error {
	media, err := a.repos.Media.FindByID(mediaID)
	if err != nil {
		return err
	}
	settings, _ := a.GetDesktopSettings()
	var cmd *exec.Cmd

	if settings.PlayerPath != "" {
		cmd = exec.Command(settings.PlayerPath, media.FilePath)
	} else {
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", media.FilePath)
		case "darwin":
			cmd = exec.Command("open", media.FilePath)
		default:
			cmd = exec.Command("xdg-open", media.FilePath)
		}
	}
	return cmd.Start()
}

// OpenMediaFolder 调用本地系统打开文件所属的原生文件管理器
func (a *App) OpenMediaFolder(mediaID string) error {
	media, err := a.repos.Media.FindByID(mediaID)
	if err != nil {
		return err
	}
	dir := filepath.Dir(media.FilePath)
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	return cmd.Start()
}

func (a *App) ToggleFavorite(mediaID string) error {
	var fav model.Favorite
	err := a.db.Where("media_id = ? AND user_id = ?", mediaID, "desktop_user").First(&fav).Error
	if err == nil {
		return a.db.Delete(&fav).Error
	}
	newFav := model.Favorite{
		UserID:  "desktop_user",
		MediaID: mediaID,
	}
	return a.db.Create(&newFav).Error
}

// ToggleWatched 切换已看状态
func (a *App) ToggleWatched(mediaID string) error {
	var wh model.WatchHistory
	err := a.db.Where("media_id = ? AND user_id = ?", mediaID, "desktop_user").First(&wh).Error
	if err == nil {
		wh.Completed = !wh.Completed
		return a.db.Save(&wh).Error
	}
	newWh := model.WatchHistory{
		UserID:    "desktop_user",
		MediaID:   mediaID,
		Completed: true,
	}
	return a.db.Create(&newWh).Error
}

// DeleteMedia 只从数据库删除记录，不移动文件
func (a *App) DeleteMedia(mediaID string) error {
	return a.repos.Media.DeleteByID(mediaID)
}

// OpenNFO 尝试找到并打开关联的 .nfo 文件
func (a *App) OpenNFO(mediaID string) error {
	media, err := a.repos.Media.FindByID(mediaID)
	if err != nil {
		return err
	}
	
	// 1. 同名 nfo
	nfoPath := strings.TrimSuffix(media.FilePath, filepath.Ext(media.FilePath)) + ".nfo"
	if _, err := os.Stat(nfoPath); os.IsNotExist(err) {
		// 2. 目录下的 movie.nfo
		dir := filepath.Dir(media.FilePath)
		nfoPath = filepath.Join(dir, "movie.nfo")
	}

	if _, err := os.Stat(nfoPath); os.IsNotExist(err) {
		return fmt.Errorf("找不到对应的 .nfo 文件")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", nfoPath)
	case "darwin":
		cmd = exec.Command("open", nfoPath)
	default:
		cmd = exec.Command("xdg-open", nfoPath)
	}
	return cmd.Start()
}

// GetMediaFiles 获取当前目录下的所有视频文件
func (a *App) GetMediaFiles(mediaID string) ([]string, error) {
	media, err := a.repos.Media.FindByID(mediaID)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(media.FilePath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	var files []string
	exts := map[string]bool{".mp4":true, ".mkv":true, ".avi":true, ".mov":true, ".wmv":true, ".flv":true, ".webm":true, ".ts":true, ".strm":true}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			if exts[strings.ToLower(filepath.Ext(entry.Name()))] {
				files = append(files, filepath.Join(dir, entry.Name()))
			}
		}
	}
	return files, nil
}

// GetMediaPreviews 收集本地预览图片（由高到低优先级：extrafanart > BTS > thumb > fanart > poster）
func (a *App) GetMediaPreviews(mediaID string) ([]string, error) {
	media, err := a.repos.Media.FindByID(mediaID)
	if err != nil {
		return nil, err
	}
	dir := filepath.Dir(media.FilePath)
	
	var previews []string
	seen := make(map[string]bool)
	imgExts := map[string]bool{".jpg": true, ".png": true, ".jpeg": true, ".webp": true}

	addIfNew := func(path string) {
		name := filepath.Base(path)
		if !seen[name] {
			previews = append(previews, path)
			seen[name] = true
		}
	}

	// 1. 扫描 extrafanart 和 behind the scenes 目录（最高级生产资料）
	subDirs := []string{"extrafanart", "behind the scenes"}
	for _, sub := range subDirs {
		subPath := filepath.Join(dir, sub)
		if sEntries, err := os.ReadDir(subPath); err == nil {
			for _, se := range sEntries {
				if !se.IsDir() && imgExts[strings.ToLower(filepath.Ext(se.Name()))] {
					addIfNew(filepath.Join(subPath, se.Name()))
				}
			}
		}
	}

	// 2. 扫描当前层级中的剧照/背景类关键词 (优先 thumb 和 fanart)
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.IsDir() || !imgExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		nameLower := strings.ToLower(e.Name())
		if strings.Contains(nameLower, "thumb") || strings.Contains(nameLower, "fanart") || strings.Contains(nameLower, "backdrop") || strings.Contains(nameLower, "preview") {
			addIfNew(filepath.Join(dir, e.Name()))
		}
	}

	// 3. 扫描其他所有图片 (排除明显的 poster)
	for _, e := range entries {
		if e.IsDir() || !imgExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		nameLower := strings.ToLower(e.Name())
		if !strings.Contains(nameLower, "poster") && !strings.Contains(nameLower, "folder") && !strings.Contains(nameLower, "cover") {
			addIfNew(filepath.Join(dir, e.Name()))
		}
	}

	// 4. 最后兜底所有图片 (包括 poster)
	for _, e := range entries {
		if e.IsDir() || !imgExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		addIfNew(filepath.Join(dir, e.Name()))
	}

	return previews, nil
}

// PlayFile 播放指定绝对路径的文件
func (a *App) PlayFile(filePath string) error {
	settings, _ := a.GetDesktopSettings()
	var cmd *exec.Cmd
	if settings.PlayerPath != "" {
		cmd = exec.Command(settings.PlayerPath, filePath)
	} else {
		switch runtime.GOOS {
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", "", filePath)
		case "darwin":
			cmd = exec.Command("open", filePath)
		default:
			cmd = exec.Command("xdg-open", filePath)
		}
	}
	return cmd.Start()
}

// DeleteLibrary 删除一个媒体库及其关联的所有媒体记录
func (a *App) DeleteLibrary(libraryID string) error {
	if err := a.repos.Media.DeleteByLibraryID(libraryID); err != nil {
		a.logger.Errorf("删除媒体库关联媒体失败: %v", err)
	}
	return a.repos.Library.Delete(libraryID)
}

// ScanLibraryWithMode 带刷新模式的扫描入口
// mode: "incremental" | "overwrite" | "delete_update"
// NOTE: 当前 scanner 核心仍只走默认逻辑，mode 参数预留用于后续扩展
func (a *App) ScanLibraryWithMode(libraryID string, mode string) error {
	lib, err := a.repos.Library.FindByID(libraryID)
	if err != nil {
		return err
	}
	a.logger.Infof("ScanLibraryWithMode: id=%s mode=%s", libraryID, mode)
	go func() {
		_, err := a.scanner.ScanLibrary(lib)
		if err != nil {
			a.logger.Errorf("Scan error (mode=%s): %v", mode, err)
		}
	}()
	return nil
}

// SelectDirectory 调起真正的桌面级弹窗选择媒体库路径
func (a *App) SelectDirectory() (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "请选择库文件夹",
	})
}
