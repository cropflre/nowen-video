package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== 硬链接整理服务 ====================
//
// 在 AI 自动整理（scan_postprocess）完成后，根据 Media + MediaClassification
// 的识别信息，在用户指定的输出目录（Library.OrganizeOutputDir）下创建硬链接目录树。
//
// 目录结构与"一键入库整理"完全一致（见 ingest_lazy.go::resolveDestPath）：
//   - 电影：  {root}/Movies/<电影文件夹>/<规范文件名>
//   - 剧集：  {root}/TV Shows/<剧集文件夹>/Season XX/<规范文件名>
//   - 未识别：{root}/_unsorted/<原文件名>
//
// 命名采用 BuildStandardNames（与一键入库/智能重命名/扫描归类三处共享同一渲染器），
// 确保跨入口一致：例如剧集统一为 "<剧名> SxxEyy.<ext>"，电影统一为 "<片名> (YYYY).<ext>"。
//
// 硬链接优势：
//   - 零额外空间占用（同一磁盘/分区内）
//   - 源文件完全不变（零风险）
//   - 删除硬链接不影响源文件
//
// 安全约束：
//   - 跨卷（不同磁盘分区）时硬链接无法创建，会降级记录错误
//   - 目标已存在时跳过（不覆盖，安全策略）
//   - 仅对文件创建硬链接，不会操作目录

// OrganizeHardlinkService 硬链接整理服务
type OrganizeHardlinkService struct {
	classRepo   *repository.ScanClassificationRepo
	mediaRepo   *repository.MediaRepo
	libraryRepo *repository.LibraryRepo
	logger      *zap.SugaredLogger
}

// NewOrganizeHardlinkService 构造硬链接整理服务
func NewOrganizeHardlinkService(
	classRepo *repository.ScanClassificationRepo,
	mediaRepo *repository.MediaRepo,
	libraryRepo *repository.LibraryRepo,
	logger *zap.SugaredLogger,
) *OrganizeHardlinkService {
	return &OrganizeHardlinkService{
		classRepo:   classRepo,
		mediaRepo:   mediaRepo,
		libraryRepo: libraryRepo,
		logger:      logger,
	}
}

// OrganizeHardlinkResult 单条硬链接操作结果
type OrganizeHardlinkResult struct {
	MediaID  string `json:"media_id"`
	SrcPath  string `json:"src_path"`
	DstPath  string `json:"dst_path"`
	Skipped  bool   `json:"skipped"` // 目标已存在，跳过
	Created  bool   `json:"created"` // 成功创建硬链接
	ErrorMsg string `json:"error_msg,omitempty"`
}

// OrganizeBatchResult 批量操作统计
type OrganizeBatchResult struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// ProcessMediaHardlink 对单条已分类的 Media 创建硬链接。
//
// 逻辑：
//  1. 读取 Media 主表（MediaType/Season/Episode/Title/Year/TMDbID/IMDbID）
//  2. 通过与"一键入库整理"相同的 BuildStandardNames 渲染目标路径
//  3. 如果目标路径已存在 → 跳过
//  4. 创建目标目录 → os.Link(源文件, 目标路径)
//
// 如果 OrganizeOutputDir 为空，返回 nil（不执行任何操作）。
func (s *OrganizeHardlinkService) ProcessMediaHardlink(mediaID string, outputDir string) (*OrganizeHardlinkResult, error) {
	if mediaID == "" {
		return nil, errors.New("mediaID 为空")
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil || media == nil {
		return nil, fmt.Errorf("media 不存在: %s", mediaID)
	}
	classification, err := s.classRepo.FindByMediaID(mediaID)
	if err != nil || classification == nil {
		return nil, fmt.Errorf("media 分类记录不存在: %s", mediaID)
	}
	return s.ProcessResolvedMediaHardlink(media, classification, outputDir)
}

// ProcessResolvedMediaHardlink 使用已加载的 Media/Classification 创建硬链接。
// 扫描后处理流程已经持有这两个对象，走该方法可避免每集额外 2 次 DB 查询。
func (s *OrganizeHardlinkService) ProcessResolvedMediaHardlink(media *model.Media, classification *model.MediaClassification, outputDir string) (*OrganizeHardlinkResult, error) {
	if outputDir == "" {
		return nil, nil // 未配置输出目录，静默跳过
	}
	if media == nil || media.ID == "" {
		return nil, errors.New("media 为空")
	}
	srcPath := media.FilePath
	if srcPath == "" {
		return nil, fmt.Errorf("media 文件路径为空: %s", media.ID)
	}
	if classification == nil {
		return nil, fmt.Errorf("media 分类记录不存在: %s", media.ID)
	}
	if classification.Status != model.ClassificationStatusProcessed {
		return nil, nil // 未完成分类，不创建硬链接
	}

	dstPath := s.buildHardlinkPath(outputDir, classification, media)
	if dstPath == "" {
		return nil, nil // 无法构建目标路径
	}

	result := &OrganizeHardlinkResult{
		MediaID: media.ID,
		SrcPath: srcPath,
		DstPath: dstPath,
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		result.ErrorMsg = fmt.Sprintf("源文件不存在或无法访问: %v", err)
		return result, nil
	}
	if srcInfo.IsDir() {
		result.ErrorMsg = "源路径为目录，跳过"
		result.Skipped = true
		return result, nil
	}

	if _, err := os.Stat(dstPath); err == nil {
		result.Skipped = true
		s.syncMediaToHardlink(media, classification, dstPath, true)
		return result, nil
	}

	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		result.ErrorMsg = fmt.Sprintf("创建目标目录失败: %v", err)
		return result, nil
	}

	if err := os.Link(srcPath, dstPath); err != nil {
		if isOrganizeCrossDeviceError(err) {
			result.ErrorMsg = "跨卷无法创建硬链接（源文件与输出目录不在同一磁盘分区）"
		} else {
			result.ErrorMsg = fmt.Sprintf("创建硬链接失败: %v", err)
		}
		return result, nil
	}

	result.Created = true
	// 刚创建的目标路径无需再查一次 FindByFilePath；直接轻量同步路径和标题等主数据。
	s.syncMediaToHardlink(media, classification, dstPath, false)
	s.logger.Debugf("[OrganizeHardlink] 硬链接创建成功: %s -> %s", srcPath, dstPath)
	return result, nil
}

// syncMediaToHardlink 将项目中的媒体记录同步到整理后的硬链接路径和规范标题。
// 源目录仍由 Library.Path 监听；项目播放/展示则使用硬链接目录下的规范路径和 AI 整理标题。
func (s *OrganizeHardlinkService) syncMediaToHardlink(media *model.Media, c *model.MediaClassification, dstPath string, checkExisting bool) {
	if media == nil || media.ID == "" || dstPath == "" {
		return
	}

	// 目标路径已存在时，可能是重新扫描源目录产生了重复记录。
	// 先把最新 AI 整理结果同步到已存在的硬链接记录，再删除源目录重复记录。
	if checkExisting {
		if existing, err := s.mediaRepo.FindByFilePath(dstPath); err == nil && existing != nil && existing.ID != "" {
			if existing.ID != media.ID {
				s.updateOrganizedFields(existing, c, dstPath)
				if err := s.classRepo.DeleteByMediaID(media.ID); err != nil {
					s.logger.Warnf("[OrganizeHardlink] 删除重复分类记录失败 media_id=%s err=%v", media.ID, err)
				}
				if err := s.mediaRepo.DeleteByID(media.ID); err != nil {
					s.logger.Warnf("[OrganizeHardlink] 删除重复源媒体记录失败 media_id=%s err=%v", media.ID, err)
				} else {
					s.logger.Infof("[OrganizeHardlink] 源目录重复记录已合并到硬链接路径: %s -> %s", media.FilePath, dstPath)
				}
				return
			}
		}
	}

	s.updateOrganizedFields(media, c, dstPath)
}

func (s *OrganizeHardlinkService) updateOrganizedFields(media *model.Media, c *model.MediaClassification, dstPath string) {
	fields := s.buildOrganizedFields(media, c, dstPath)
	if len(fields) == 0 {
		return
	}
	oldPath := media.FilePath
	if err := s.mediaRepo.UpdateOrganizedFields(media.ID, fields); err != nil {
		s.logger.Warnf("[OrganizeHardlink] 同步媒体整理数据失败 media_id=%s %s -> %s err=%v", media.ID, oldPath, dstPath, err)
		return
	}
	applyOrganizedFields(media, fields)
	s.logger.Debugf("[OrganizeHardlink] 媒体整理数据已同步: media_id=%s path=%s -> %s fields=%d", media.ID, oldPath, dstPath, len(fields))
}

func (s *OrganizeHardlinkService) buildOrganizedFields(media *model.Media, c *model.MediaClassification, dstPath string) map[string]any {
	fields := map[string]any{}
	if media.FilePath != dstPath {
		fields["file_path"] = dstPath
	}
	if c == nil {
		return fields
	}
	if title := strings.TrimSpace(c.ParsedTitle); title != "" && title != media.Title {
		fields["title"] = title
		if strings.TrimSpace(media.OrigTitle) == "" && strings.TrimSpace(media.Title) != "" {
			fields["orig_title"] = media.Title
		}
	}
	if c.ParsedYear > 0 && c.ParsedYear != media.Year {
		fields["year"] = c.ParsedYear
	}
	// 电影可以同步 ID；剧集的单集 AI ID 容易抖动，不写入主表，避免再次造成拆分/误聚合。
	if strings.ToLower(strings.TrimSpace(media.MediaType)) != "episode" {
		if c.ParsedTMDbID > 0 && c.ParsedTMDbID != media.TMDbID {
			fields["tmdb_id"] = c.ParsedTMDbID
		}
		if imdbID := strings.TrimSpace(c.ParsedIMDbID); imdbID != "" && imdbID != media.IMDbID {
			fields["imdb_id"] = imdbID
		}
	}
	return fields
}

func (s *OrganizeHardlinkService) stableEpisodeTitle(media *model.Media, c *model.MediaClassification) string {
	if media == nil {
		return ""
	}
	// 最高优先级：源媒体库根目录下的第一层目录。它代表用户导入的一个影视根目录，
	// 同一根目录下所有集数必须锁定到同一个输出父目录。
	if title := s.sourceRootEpisodeTitle(media); title != "" {
		return title
	}
	if title := stableEpisodeTitleFromPath(media.FilePath); title != "" {
		return title
	}
	if c != nil {
		return strings.TrimSpace(c.ParsedTitle)
	}
	return ""
}

func (s *OrganizeHardlinkService) sourceRootEpisodeTitle(media *model.Media) string {
	if media == nil || media.FilePath == "" || media.LibraryID == "" || s.libraryRepo == nil {
		return ""
	}
	lib, err := s.libraryRepo.FindByID(media.LibraryID)
	if err != nil || lib == nil {
		return ""
	}
	filePath := filepath.Clean(media.FilePath)
	for _, root := range lib.AllPaths() {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || !isPathUnderRoot(filePath, root) {
			continue
		}
		rel, err := filepath.Rel(root, filePath)
		if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
			continue
		}
		parts := strings.Split(filepath.Clean(rel), string(filepath.Separator))
		if len(parts) < 2 { // 文件直接在根目录下，没有影视根目录可锁定
			continue
		}
		candidate := cleanStableSeriesTitle(parts[0])
		if bracket := bestBracketSeriesTitle(candidate); bracket != "" {
			candidate = bracket
		}
		candidate = NormalizeSeriesTitle(candidate)
		if candidate != "" && !isNoisySeriesTitle(candidate) {
			return candidate
		}
	}
	return ""
}

func isPathUnderRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

func stableEpisodeTitleFromPath(srcPath string) string {
	if srcPath == "" {
		return ""
	}
	cur := filepath.Dir(srcPath)
	for i := 0; i < 5; i++ {
		base := filepath.Base(cur)
		if base == "" || base == "." || base == string(filepath.Separator) {
			break
		}
		if title := bestBracketSeriesTitle(base); title != "" {
			return title
		}
		cur = filepath.Dir(cur)
	}
	return ""
}

func bestBracketSeriesTitle(name string) string {
	segments := bracketSegments(name)
	best := ""
	for _, seg := range segments {
		seg = cleanStableSeriesTitle(seg)
		if seg == "" || isNoisySeriesTitle(seg) || !containsCJK(seg) {
			continue
		}
		if len([]rune(seg)) > len([]rune(best)) {
			best = seg
		}
	}
	return best
}

func bracketSegments(s string) []string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '[' {
			continue
		}
		for j := i + 1; j < len(s); j++ {
			if s[j] == ']' {
				if seg := strings.TrimSpace(s[i+1 : j]); seg != "" {
					out = append(out, seg)
				}
				i = j
				break
			}
		}
	}
	return out
}

func cleanStableSeriesTitle(s string) string {
	s = strings.TrimSpace(s)
	for _, suffix := range []string{" 第二季", " 第一季", " 第三季", " 第四季", " 第五季", " 第2季", " 第1季", " 第3季", " 第4季", " 第5季"} {
		s = strings.TrimSuffix(s, suffix)
	}
	return strings.TrimSpace(s)
}

func isNoisySeriesTitle(s string) bool {
	lower := strings.ToLower(s)
	for _, token := range []string{"dbd-raws", "raws", "bdrip", "webrip", "web-dl", "hevc", "flac", "mkv", "1080", "720", "2160", "tv全集", "全集", "简繁", "外挂", "字幕", "01-", "s01"} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return false
}

func applyOrganizedFields(media *model.Media, fields map[string]any) {
	if v, ok := fields["file_path"].(string); ok {
		media.FilePath = v
	}
	if v, ok := fields["title"].(string); ok {
		media.Title = v
	}
	if v, ok := fields["orig_title"].(string); ok {
		media.OrigTitle = v
	}
	if v, ok := fields["year"].(int); ok {
		media.Year = v
	}
	if v, ok := fields["tmdb_id"].(int); ok {
		media.TMDbID = v
	}
	if v, ok := fields["imdb_id"].(string); ok {
		media.IMDbID = v
	}
}

// ProcessBatchHardlink 批量执行硬链接。
// mediaIDs 为需要处理的媒体 ID 列表，outputDir 为输出根目录。
func (s *OrganizeHardlinkService) ProcessBatchHardlink(mediaIDs []string, outputDir string) OrganizeBatchResult {
	result := OrganizeBatchResult{Total: len(mediaIDs)}
	if outputDir == "" {
		result.Skipped = len(mediaIDs)
		return result
	}

	for _, id := range mediaIDs {
		r, err := s.ProcessMediaHardlink(id, outputDir)
		if err != nil {
			result.Failed++
			s.logger.Warnf("[OrganizeHardlink] 处理失败 media_id=%s err=%v", id, err)
			continue
		}
		if r == nil {
			result.Skipped++
			continue
		}
		if r.Created {
			result.Created++
		} else if r.Skipped {
			result.Skipped++
		} else if r.ErrorMsg != "" {
			result.Failed++
			s.logger.Warnf("[OrganizeHardlink] media_id=%s error=%s", id, r.ErrorMsg)
		}
	}
	return result
}

// ProcessLibraryHardlink 对整个媒体库执行硬链接整理
func (s *OrganizeHardlinkService) ProcessLibraryHardlink(libraryID string) (*OrganizeBatchResult, error) {
	if libraryID == "" {
		return nil, errors.New("libraryID 为空")
	}

	lib, err := s.libraryRepo.FindByID(libraryID)
	if err != nil || lib == nil {
		return nil, fmt.Errorf("媒体库不存在: %s", libraryID)
	}

	outputDir := strings.TrimSpace(lib.OrganizeOutputDir)
	if outputDir == "" {
		return &OrganizeBatchResult{}, nil // 未配置输出目录
	}

	medias, err := s.mediaRepo.ListByLibraryID(libraryID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(medias))
	for _, m := range medias {
		ids = append(ids, m.ID)
	}

	result := s.ProcessBatchHardlink(ids, outputDir)
	s.logger.Infof("[OrganizeHardlink] 媒体库 %s 硬链接完成: 创建=%d 跳过=%d 失败=%d",
		lib.Name, result.Created, result.Skipped, result.Failed)
	return &result, nil
}

// buildHardlinkPath 根据 Media 与分类结果，构造硬链接目标路径。
//
// 电影：可以直接使用 SuggestedDir / SuggestedName。
// 剧集：不能直接使用每集 AI 生成的 SuggestedDir，因为不同集可能识别出不同 tmdbid/year，
// 会导致同一部剧被拆成大量文件夹。剧集目录必须按稳定剧名聚合。
func (s *OrganizeHardlinkService) buildHardlinkPath(outputDir string, c *model.MediaClassification, media *model.Media) string {
	if outputDir == "" || media == nil || media.FilePath == "" {
		return ""
	}

	const (
		moviesDir         = "Movies"
		tvShowsDir        = "TV Shows"
		unsortedDir       = "_unsorted"
		unsortedThreshold = 0.5
	)

	srcName := filepath.Base(media.FilePath)

	// 1) 置信度不足 → _unsorted（与一键入库整理一致：保留原文件名）
	if c == nil || c.Confidence < unsortedThreshold {
		return filepath.Join(outputDir, unsortedDir, srcName)
	}

	mediaType := strings.ToLower(strings.TrimSpace(media.MediaType))
	if mediaType == "" && c.Category == "tvshow" {
		mediaType = "episode"
	}

	// 2) 剧集：强制使用稳定剧名目录，不使用每集 SuggestedDir 中可能随机变化的 tmdbid/year。
	if mediaType == "episode" {
		if media.EpisodeNum <= 0 {
			return filepath.Join(outputDir, unsortedDir, srcName)
		}

		title := s.stableEpisodeTitle(media, c)
		if title == "" {
			title = strings.TrimSpace(c.ParsedTitle)
		}
		if title == "" {
			title = strings.TrimSpace(media.Title)
		}
		if title == "" {
			title = strings.TrimSpace(media.OrigTitle)
		}
		if title == "" {
			return filepath.Join(outputDir, unsortedDir, srcName)
		}

		// 对剧集故意不传 Year/TMDbID/IMDbID，避免一集一个 ID 造成目录拆分。
		names := BuildStandardNames(StandardNameInput{
			SourcePath: media.FilePath,
			SourceName: srcName,
			MediaType:  "episode",
			Title:      title,
			SeasonNum:  media.SeasonNum,
			EpisodeNum: media.EpisodeNum,
			Style:      NamingStyleJellyfin,
		})
		if names.FileName == "" {
			return filepath.Join(outputDir, unsortedDir, srcName)
		}

		showFolder := names.ShowFolder
		if showFolder == "" {
			showFolder = title
		}
		seasonDir := names.SeasonDir
		if seasonDir == "" {
			seasonNum := media.SeasonNum
			if seasonNum <= 0 {
				seasonNum = 1
			}
			seasonDir = fmt.Sprintf("Season %02d", seasonNum)
		}
		return filepath.Join(outputDir, tvShowsDir, showFolder, seasonDir, names.FileName)
	}

	// 3) 电影：统一重新渲染，不直接使用 SuggestedDir/SuggestedName。
	// 原因：TMDb 刮削前后 ID 是否存在会变化，若路径带 [tmdbid-x]，会产生「无 ID 目录 + 有 ID 目录」两份硬链接。
	// 硬链接输出目录以稳定可重复为第一优先，因此电影物理路径不带 tmdb/imdb 标签。

	// 4) 电影兜底重新渲染：优先使用 classification 里的 AI/规则识别结果，而不是 Media.Title 原名。
	title := strings.TrimSpace(c.ParsedTitle)
	if title == "" {
		title = strings.TrimSpace(media.Title)
	}
	if title == "" {
		title = strings.TrimSpace(media.OrigTitle)
	}
	if title == "" {
		return filepath.Join(outputDir, unsortedDir, srcName)
	}

	year := c.ParsedYear
	if year <= 0 {
		year = media.Year
	}
	names := BuildStandardNames(StandardNameInput{
		SourcePath: media.FilePath,
		SourceName: srcName,
		MediaType:  mediaType,
		Title:      title,
		Year:       year,
		// 物理硬链接路径不带 ID，避免刮削前后路径漂移。
		TMDbID: 0,
		IMDbID: "",
		Style:  NamingStyleJellyfin,
	})
	if names.FileName == "" {
		return filepath.Join(outputDir, unsortedDir, srcName)
	}

	movieFolder := names.MovieFolder
	if movieFolder == "" {
		movieFolder = strings.TrimSuffix(names.FileName, filepath.Ext(names.FileName))
	}
	return filepath.Join(outputDir, moviesDir, movieFolder, names.FileName)
}

// isOrganizeCrossDeviceError 判定错误是否为跨卷错误
func isOrganizeCrossDeviceError(err error) bool {
	if err == nil {
		return false
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		// Linux: "invalid cross-device link" / EXDEV
		// Windows: "The system cannot move the file to a different disk drive"
		msg := strings.ToLower(linkErr.Err.Error())
		return strings.Contains(msg, "cross-device") ||
			strings.Contains(msg, "different disk") ||
			strings.Contains(msg, "不在同一") ||
			strings.Contains(msg, "exdev")
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "cross-device") || strings.Contains(msg, "different disk")
}
