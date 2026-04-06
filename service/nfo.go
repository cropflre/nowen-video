package service

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"alex-desktop/model"
	"go.uber.org/zap"
)

// NFOService NFO 本地元数据解析服务
// 支持 Kodi / Emby / Jellyfin 风格的 NFO XML 文件
// 增强：宽松兼容非标准字段、日期归一化、原始 XML 保留
type NFOService struct {
	logger *zap.SugaredLogger
}

func NewNFOService(logger *zap.SugaredLogger) *NFOService {
	return &NFOService{logger: logger}
}

// ==================== NFO XML 结构体（增强版） ====================

// NFOMovie 电影 NFO XML 根元素（宽松兼容）
type NFOMovie struct {
	XMLName xml.Name `xml:"movie"`
	// 标准字段
	Title     string  `xml:"title"`
	OrigTitle string  `xml:"originaltitle"`
	SortTitle string  `xml:"sorttitle"`
	Year      int     `xml:"year"`
	Plot      string  `xml:"plot"`
	Outline   string  `xml:"outline"`
	Tagline   string  `xml:"tagline"`
	Rating    float64 `xml:"rating"`
	Runtime   int     `xml:"runtime"`
	Studio    string  `xml:"studio"`
	Country   string  `xml:"country"`
	TMDbID    int     `xml:"tmdbid"`
	DoubanID  string  `xml:"doubanid"`
	Genres    []string `xml:"genre"`
	Tags      []string `xml:"tag"`
	Directors []string `xml:"director"`
	Actors    []NFOActor `xml:"actor"`
	Set       string `xml:"set"`
	// 增强字段：日期（多种来源，后续归一化）
	Premiered   string `xml:"premiered"`
	ReleaseDate string `xml:"releasedate"`
	Release     string `xml:"release"`
	// 增强字段：评分/分级
	CriticRating float64 `xml:"criticrating"`
	MPAA         string  `xml:"mpaa"`
	CustomRating string  `xml:"customrating"`
	CountryCode  string  `xml:"countrycode"`
	// 增强字段：制作信息（非标准但常见于特定刮削器）
	OriginalPlot string `xml:"originalplot"`
	Maker        string `xml:"maker"`
	Publisher    string `xml:"publisher"`
	Label        string `xml:"label"`
	Num          string `xml:"num"`
	// 增强字段：远程图片路径
	Poster string `xml:"poster"`
	Cover  string `xml:"cover"`
	Fanart string `xml:"fanart"`
	Thumb  string `xml:"thumb"`
	// 增强字段：站点来源 Provider IDs
	JavbusID      string `xml:"javbusid"`
	AiravCcid     string `xml:"airav_ccid"`
	JavdbSearchID string `xml:"javdbsearchid"`
	LockData      string `xml:"lockdata"`
	DateAdded     string `xml:"dateadded"`
	Trailer       string `xml:"trailer"`
	Votes         string `xml:"votes"`
	Website       string `xml:"website"`
	FileInfo      *NFORawSection `xml:"fileinfo"`
}

// NFOTVShow 剧集 NFO XML 根元素
type NFOTVShow struct {
	XMLName   xml.Name   `xml:"tvshow"`
	Title     string     `xml:"title"`
	OrigTitle string     `xml:"originaltitle"`
	Year      int        `xml:"year"`
	Plot      string     `xml:"plot"`
	Rating    float64    `xml:"rating"`
	Studio    string     `xml:"studio"`
	Country   string     `xml:"country"`
	TMDbID    int        `xml:"tmdbid"`
	DoubanID  string     `xml:"doubanid"`
	Genres    []string   `xml:"genre"`
	Tags      []string   `xml:"tag"`
	Directors []string   `xml:"director"`
	Actors    []NFOActor `xml:"actor"`
	// 增强日期字段
	Premiered   string `xml:"premiered"`
	ReleaseDate string `xml:"releasedate"`
}

// NFOActor NFO 演员信息（宽松兼容：name 可能为空）
type NFOActor struct {
	Name      string `xml:"name"`
	Role      string `xml:"role"`
	Thumb     string `xml:"thumb"`
	SortOrder int    `xml:"sortorder"`
}

// NFOExtraFields 存储到 Media.NfoExtraFields 的 JSON 结构
// NOTE: 后续可根据需要扩展字段
type NFOExtraFields struct {
	SortTitle    string            `json:"sort_title,omitempty"`
	Outline      string            `json:"outline,omitempty"`
	OriginalPlot string            `json:"original_plot,omitempty"`
	MPAA         string            `json:"mpaa,omitempty"`
	CustomRating string            `json:"custom_rating,omitempty"`
	CriticRating float64           `json:"critic_rating,omitempty"`
	CountryCode  string            `json:"country_code,omitempty"`
	Maker        string            `json:"maker,omitempty"`
	Publisher    string            `json:"publisher,omitempty"`
	Label        string            `json:"label,omitempty"`
	Num          string            `json:"num,omitempty"`
	Poster       string            `json:"poster,omitempty"`
	Cover        string            `json:"cover,omitempty"`
	Fanart       string            `json:"fanart,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	ProviderIDs  map[string]string `json:"provider_ids,omitempty"`
}

type NFORawSection struct {
	InnerXML string `xml:",innerxml"`
}

type NFOEditorData struct {
	NFOPath     string `json:"nfo_path"`
	Title       string `json:"title"`
	Code        string `json:"code"`
	ReleaseDate string `json:"release_date"`
	Director    string `json:"director"`
	Series      string `json:"series"`
	Publisher   string `json:"publisher"`
	Maker       string `json:"maker"`
	Genres      string `json:"genres"`
	Actors      string `json:"actors"`
	Plot        string `json:"plot"`
	Runtime     string `json:"runtime"`
	FileSize    string `json:"file_size"`
	Resolution  string `json:"resolution"`
	VideoCodec  string `json:"video_codec"`
	Rating      string `json:"rating"`
}

func joinEditorList(values []string) string {
	if len(values) == 0 {
		return ""
	}

	seen := make(map[string]bool)
	items := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		items = append(items, value)
	}
	return strings.Join(items, " / ")
}

func splitEditorList(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '，', '/', '、', '\n', '\r', ';', '|':
			return true
		default:
			return false
		}
	})

	items := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		items = append(items, field)
	}
	return items
}

func joinActorNames(actors []NFOActor) string {
	if len(actors) == 0 {
		return ""
	}
	names := make([]string, 0, len(actors))
	for _, actor := range actors {
		name := strings.TrimSpace(actor.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return joinEditorList(names)
}

func splitEditorValues(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\uFF0C', '\u3001', '/', '\n', '\r', ';', '|':
			return true
		default:
			return false
		}
	})

	items := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" || seen[field] {
			continue
		}
		seen[field] = true
		items = append(items, field)
	}
	return items
}

func buildEditorActors(raw string) []NFOActor {
	names := splitEditorValues(raw)
	actors := make([]NFOActor, 0, len(names))
	for index, name := range names {
		actors = append(actors, NFOActor{
			Name:      name,
			SortOrder: index,
		})
	}
	return actors
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func formatEditorFloat(value float64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func formatEditorInt(value int) string {
	if value <= 0 {
		return ""
	}
	return strconv.Itoa(value)
}

func formatEditorFileSize(size int64) string {
	if size <= 0 {
		return ""
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case size >= gb:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.1f MB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.1f KB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func deriveEditorCode(media *model.Media) string {
	if media == nil {
		return ""
	}

	if media.NfoExtraFields != "" {
		var extra NFOExtraFields
		if err := json.Unmarshal([]byte(media.NfoExtraFields), &extra); err == nil && strings.TrimSpace(extra.Num) != "" {
			return strings.TrimSpace(extra.Num)
		}
	}

	filename := filepath.Base(media.FilePath)
	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	return strings.TrimSpace(stem)
}

func releaseYear(value string, fallback int) int {
	value = strings.TrimSpace(value)
	if len(value) >= 4 {
		if year, err := strconv.Atoi(value[:4]); err == nil && year > 0 {
			return year
		}
	}
	return fallback
}

func (s *NFOService) LoadEditorData(nfoPath string, media *model.Media) (*NFOEditorData, error) {
	data := &NFOEditorData{
		NFOPath:     strings.TrimSpace(nfoPath),
		Title:       strings.TrimSpace(media.Title),
		Code:        deriveEditorCode(media),
		ReleaseDate: strings.TrimSpace(media.ReleaseDateNormalized),
		Publisher:   strings.TrimSpace(media.Studio),
		Maker:       strings.TrimSpace(media.Studio),
		Genres:      joinEditorList(strings.Split(strings.TrimSpace(media.Genres), ",")),
		Actors:      joinEditorList(strings.Split(strings.TrimSpace(media.Actor), ",")),
		Plot:        strings.TrimSpace(media.Overview),
		Runtime:     formatEditorInt(media.Runtime),
		FileSize:    formatEditorFileSize(media.FileSize),
		Resolution:  strings.TrimSpace(media.Resolution),
		VideoCodec:  strings.TrimSpace(media.VideoCodec),
		Rating:      formatEditorFloat(media.Rating),
	}
	if media.Series != nil {
		data.Series = strings.TrimSpace(media.Series.Title)
	}

	if strings.TrimSpace(nfoPath) == "" {
		return data, nil
	}

	content, err := os.ReadFile(nfoPath)
	if err != nil {
		if os.IsNotExist(err) {
			return data, nil
		}
		return nil, fmt.Errorf("读取 NFO 文件失败: %w", err)
	}

	var movie NFOMovie
	if err := xml.Unmarshal(content, &movie); err != nil {
		return nil, fmt.Errorf("解析 NFO 文件失败: %w", err)
	}

	data.Title = firstNonEmpty(movie.Title, data.Title)
	data.Code = firstNonEmpty(movie.Num, data.Code)
	data.ReleaseDate = firstNonEmpty(movie.ReleaseDate, movie.Premiered, movie.Release, data.ReleaseDate)
	data.Director = firstNonEmpty(joinEditorList(movie.Directors), data.Director)
	data.Series = firstNonEmpty(movie.Set, data.Series)
	data.Publisher = firstNonEmpty(movie.Publisher, movie.Label, data.Publisher)
	data.Maker = firstNonEmpty(movie.Maker, movie.Studio, data.Maker)
	data.Genres = firstNonEmpty(joinEditorList(append(movie.Genres, movie.Tags...)), data.Genres)
	data.Actors = firstNonEmpty(joinActorNames(movie.Actors), data.Actors)
	data.Plot = firstNonEmpty(movie.Plot, movie.Outline, data.Plot)
	data.Runtime = firstNonEmpty(formatEditorInt(movie.Runtime), data.Runtime)
	data.Rating = firstNonEmpty(formatEditorFloat(movie.Rating), data.Rating)

	return data, nil
}

func (s *NFOService) SaveEditorData(nfoPath string, data *NFOEditorData) error {
	nfoPath = strings.TrimSpace(nfoPath)
	if nfoPath == "" {
		return fmt.Errorf("empty nfo path")
	}
	if data == nil {
		return fmt.Errorf("empty nfo editor data")
	}

	var movie NFOMovie
	if content, err := os.ReadFile(nfoPath); err == nil {
		if err := xml.Unmarshal(content, &movie); err != nil {
			return fmt.Errorf("解析 NFO 文件失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("读取 NFO 文件失败: %w", err)
	}

	movie.XMLName = xml.Name{Local: "movie"}
	movie.Title = strings.TrimSpace(data.Title)
	movie.Num = strings.TrimSpace(data.Code)
	movie.Premiered = strings.TrimSpace(data.ReleaseDate)
	movie.ReleaseDate = strings.TrimSpace(data.ReleaseDate)
	movie.Release = strings.TrimSpace(data.ReleaseDate)
	movie.Set = strings.TrimSpace(data.Series)
	movie.Directors = splitEditorValues(data.Director)
	movie.Publisher = strings.TrimSpace(data.Publisher)
	movie.Label = strings.TrimSpace(data.Publisher)
	movie.Maker = strings.TrimSpace(data.Maker)
	movie.Studio = strings.TrimSpace(data.Maker)
	movie.Genres = splitEditorValues(data.Genres)
	movie.Tags = append([]string(nil), movie.Genres...)
	movie.Actors = buildEditorActors(data.Actors)
	movie.Plot = strings.TrimSpace(data.Plot)

	if runtime, err := strconv.Atoi(strings.TrimSpace(data.Runtime)); err == nil {
		movie.Runtime = runtime
	}
	if rating, err := strconv.ParseFloat(strings.TrimSpace(data.Rating), 64); err == nil {
		movie.Rating = rating
	}
	movie.Year = releaseYear(data.ReleaseDate, movie.Year)

	output, err := xml.MarshalIndent(movie, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 NFO 文件失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(nfoPath), 0755); err != nil {
		return fmt.Errorf("创建 NFO 目录失败: %w", err)
	}

	fileContent := append([]byte(xml.Header), output...)
	fileContent = append(fileContent, '\n')
	if err := os.WriteFile(nfoPath, fileContent, 0644); err != nil {
		return fmt.Errorf("写入 NFO 文件失败: %w", err)
	}

	return nil
}

// ==================== 解析方法 ====================

// ParseMovieNFO 解析电影 NFO 文件并将数据应用到 Media 对象
func (s *NFOService) ParseMovieNFO(nfoPath string, media *model.Media) error {
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return fmt.Errorf("读取NFO文件失败: %w", err)
	}

	// 保留原始 XML 文本
	media.NfoRawXml = string(data)

	var nfo NFOMovie
	if err := xml.Unmarshal(data, &nfo); err != nil {
		// 尝试作为 tvshow 解析
		var tvNFO NFOTVShow
		if err2 := xml.Unmarshal(data, &tvNFO); err2 != nil {
			return fmt.Errorf("解析NFO XML失败: %w", err)
		}
		// 如果是 tvshow 格式，转换后应用
		s.applyTVShowNFOToMedia(media, &tvNFO)
		return nil
	}

	s.applyMovieNFOToMedia(media, &nfo)
	return nil
}

// ParseTVShowNFO 解析剧集 NFO 文件并将数据应用到 Series 对象
func (s *NFOService) ParseTVShowNFO(nfoPath string, series *model.Series) error {
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return fmt.Errorf("读取NFO文件失败: %w", err)
	}

	var nfo NFOTVShow
	if err := xml.Unmarshal(data, &nfo); err != nil {
		return fmt.Errorf("解析NFO XML失败: %w", err)
	}

	s.applyTVShowNFOToSeries(series, &nfo)
	return nil
}

// GetActorsFromNFO 从 NFO 文件中提取演员列表
func (s *NFOService) GetActorsFromNFO(nfoPath string) ([]NFOActor, []string, error) {
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		return nil, nil, err
	}

	// 先尝试 movie
	var movie NFOMovie
	if err := xml.Unmarshal(data, &movie); err == nil && movie.Title != "" {
		return movie.Actors, movie.Directors, nil
	}

	// 再尝试 tvshow
	var tvshow NFOTVShow
	if err := xml.Unmarshal(data, &tvshow); err == nil && tvshow.Title != "" {
		return tvshow.Actors, tvshow.Directors, nil
	}

	return nil, nil, fmt.Errorf("无法解析NFO文件")
}

// ==================== 本地图片扫描 ====================

// FindLocalImages 在指定目录下查找本地图片（poster/fanart/banner 等）
// 支持 jpg、png、webp 等常见图片格式
func (s *NFOService) FindLocalImages(dir string) (poster, backdrop string) {
	// 常见本地海报文件名（按优先级排序）
	posterNames := []string{
		"poster.jpg", "poster.png", "poster.webp",
		"cover.jpg", "cover.png", "cover.webp",
		"folder.jpg", "folder.png", "folder.webp",
		"thumb.jpg", "thumb.png", "thumb.webp",
		"movie.jpg", "movie.png",
		"show.jpg", "show.png",
	}
	// 常见本地背景图文件名
	backdropNames := []string{
		"fanart.jpg", "fanart.png", "fanart.webp",
		"backdrop.jpg", "backdrop.png", "backdrop.webp",
		"banner.jpg", "banner.png", "banner.webp",
		"background.jpg", "background.png", "background.webp",
		"clearart.jpg", "clearart.png",
		"landscape.jpg", "landscape.png",
	}

	for _, name := range posterNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			poster = path
			break
		}
	}

	for _, name := range backdropNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			backdrop = path
			break
		}
	}

	imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	entries, err := os.ReadDir(dir)
	if err == nil {
		hasToken := func(name, token string) bool {
			lower := strings.ToLower(name)
			ext := strings.ToLower(filepath.Ext(lower))
			stem := strings.TrimSuffix(lower, ext)
			normalized := "-" + strings.NewReplacer("_", "-", ".", "-", " ", "-").Replace(stem) + "-"
			return strings.Contains(normalized, "-"+token+"-")
		}
		findByTokens := func(tokens []string, excludeTokens []string) string {
			for _, token := range tokens {
				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}
					name := entry.Name()
					ext := strings.ToLower(filepath.Ext(name))
					if !imageExts[ext] || !hasToken(name, token) {
						continue
					}
					skip := false
					for _, excludeToken := range excludeTokens {
						if hasToken(name, excludeToken) {
							skip = true
							break
						}
					}
					if skip {
						continue
					}
					return filepath.Join(dir, name)
				}
			}
			return ""
		}

		if poster == "" {
			poster = findByTokens([]string{"poster"}, nil)
		}
		if backdrop == "" {
			backdrop = findByTokens([]string{"fanart"}, nil)
		}
		if backdrop == "" {
			backdrop = findByTokens([]string{"backdrop"}, nil)
		}
		if backdrop == "" {
			backdrop = findByTokens([]string{"background", "banner", "clearart", "landscape"}, nil)
		}
		if poster == "" {
			poster = findByTokens([]string{"cover", "folder", "thumb", "movie", "show"}, []string{"fanart", "backdrop", "background", "banner", "clearart", "landscape"})
		}
	}

	// 如果没有找到标准命名的海报，尝试查找目录中的第一张图片作为海报
	if poster == "" {
		entries, err := os.ReadDir(dir)
		if err == nil {
			imageExts := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
			for _, entry := range entries {
				if !entry.IsDir() {
					ext := strings.ToLower(filepath.Ext(entry.Name()))
					if imageExts[ext] {
						// 排除已识别为backdrop的文件
						candidate := filepath.Join(dir, entry.Name())
						if candidate != backdrop {
							poster = candidate
							break
						}
					}
				}
			}
		}
	}

	return poster, backdrop
}

// FindNFOFile 在指定目录下查找 NFO 文件
func (s *NFOService) FindNFOFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".nfo") {
			return filepath.Join(dir, entry.Name())
		}
	}
	return ""
}

// FindNFOForMedia 根据媒体文件路径查找关联的 NFO 文件
func (s *NFOService) FindNFOForMedia(mediaFilePath string) string {
	// 策略1: 同名 .nfo 文件
	ext := filepath.Ext(mediaFilePath)
	nfoPath := strings.TrimSuffix(mediaFilePath, ext) + ".nfo"
	if _, err := os.Stat(nfoPath); err == nil {
		return nfoPath
	}

	// 策略2: 目录下任意 .nfo 文件
	dir := filepath.Dir(mediaFilePath)
	return s.FindNFOFile(dir)
}

// ==================== 日期归一化 ====================

// normalizeReleaseDate 从多个日期字段中选择优先级最高的并格式化
// 优先级: releasedate > premiered > release
func normalizeReleaseDate(releasedate, premiered, release string) string {
	candidates := []string{
		strings.TrimSpace(releasedate),
		strings.TrimSpace(premiered),
		strings.TrimSpace(release),
	}
	for _, d := range candidates {
		if d != "" && len(d) >= 4 {
			// 尝试识别常见日期格式，归一化为 YYYY-MM-DD
			// 已经是合法格式的直接返回
			return d
		}
	}
	return ""
}

// ==================== 应用 NFO 数据（增强版） ====================

func (s *NFOService) applyMovieNFOToMedia(media *model.Media, nfo *NFOMovie) {
	if nfo.Title != "" {
		media.Title = nfo.Title
	}
	if nfo.OrigTitle != "" {
		media.OrigTitle = nfo.OrigTitle
	}
	if nfo.Year > 0 {
		media.Year = nfo.Year
	}
	if nfo.Plot != "" {
		media.Overview = nfo.Plot
	}
	if nfo.Rating > 0 {
		media.Rating = nfo.Rating
	}
	if nfo.Runtime > 0 {
		media.Runtime = nfo.Runtime
	}
	// genre 和 tag 合并去重展示
	allGenres := append(nfo.Genres, nfo.Tags...)
	if len(allGenres) > 0 {
		seen := make(map[string]bool)
		var deduped []string
		for _, g := range allGenres {
			g = strings.TrimSpace(g)
			if g != "" && !seen[g] {
				seen[g] = true
				deduped = append(deduped, g)
			}
		}
		media.Genres = strings.Join(deduped, ",")
	}
	if nfo.Tagline != "" {
		media.Tagline = nfo.Tagline
	}
	if nfo.Studio != "" {
		media.Studio = nfo.Studio
	}
	if nfo.Country != "" {
		media.Country = nfo.Country
	}
	if nfo.TMDbID > 0 {
		media.TMDbID = nfo.TMDbID
	}
	if nfo.DoubanID != "" {
		media.DoubanID = nfo.DoubanID
	}

	// 日期归一化
	normalized := normalizeReleaseDate(nfo.ReleaseDate, nfo.Premiered, nfo.Release)
	if normalized != "" {
		media.ReleaseDateNormalized = normalized
	}

	// 构建扩展字段 JSON
	extra := NFOExtraFields{
		SortTitle:    nfo.SortTitle,
		Outline:      nfo.Outline,
		OriginalPlot: nfo.OriginalPlot,
		MPAA:         nfo.MPAA,
		CustomRating: nfo.CustomRating,
		CriticRating: nfo.CriticRating,
		CountryCode:  nfo.CountryCode,
		Maker:        nfo.Maker,
		Publisher:    nfo.Publisher,
		Label:        nfo.Label,
		Num:          nfo.Num,
		Poster:       nfo.Poster,
		Cover:        nfo.Cover,
		Fanart:       nfo.Fanart,
		Tags:         nfo.Tags,
	}

	// 收集 provider IDs
	providerIDs := make(map[string]string)
	if nfo.JavbusID != "" {
		providerIDs["javbusid"] = nfo.JavbusID
	}
	if nfo.AiravCcid != "" {
		providerIDs["airav_ccid"] = nfo.AiravCcid
	}
	if nfo.JavdbSearchID != "" {
		providerIDs["javdbsearchid"] = nfo.JavdbSearchID
	}
	if len(providerIDs) > 0 {
		extra.ProviderIDs = providerIDs
	}

	// 只在有实际内容时序列化存储
	if s.hasExtraContent(&extra) {
		if data, err := json.Marshal(extra); err == nil {
			media.NfoExtraFields = string(data)
		}
	}
}

// hasExtraContent 检查扩展字段是否有实际内容（避免写入空 JSON）
func (s *NFOService) hasExtraContent(extra *NFOExtraFields) bool {
	return extra.SortTitle != "" || extra.Outline != "" || extra.OriginalPlot != "" ||
		extra.MPAA != "" || extra.CustomRating != "" || extra.CriticRating > 0 ||
		extra.CountryCode != "" || extra.Maker != "" || extra.Publisher != "" ||
		extra.Label != "" || extra.Num != "" || extra.Poster != "" ||
		extra.Cover != "" || extra.Fanart != "" || len(extra.Tags) > 0 ||
		len(extra.ProviderIDs) > 0
}

func (s *NFOService) applyTVShowNFOToMedia(media *model.Media, nfo *NFOTVShow) {
	if nfo.Title != "" {
		media.Title = nfo.Title
	}
	if nfo.OrigTitle != "" {
		media.OrigTitle = nfo.OrigTitle
	}
	if nfo.Year > 0 {
		media.Year = nfo.Year
	}
	if nfo.Plot != "" {
		media.Overview = nfo.Plot
	}
	if nfo.Rating > 0 {
		media.Rating = nfo.Rating
	}
	allGenres := append(nfo.Genres, nfo.Tags...)
	if len(allGenres) > 0 {
		seen := make(map[string]bool)
		var deduped []string
		for _, g := range allGenres {
			g = strings.TrimSpace(g)
			if g != "" && !seen[g] {
				seen[g] = true
				deduped = append(deduped, g)
			}
		}
		media.Genres = strings.Join(deduped, ",")
	}
	if nfo.Country != "" {
		media.Country = nfo.Country
	}
	// 日期归一化
	normalized := normalizeReleaseDate(nfo.ReleaseDate, nfo.Premiered, "")
	if normalized != "" {
		media.ReleaseDateNormalized = normalized
	}
}

func (s *NFOService) applyTVShowNFOToSeries(series *model.Series, nfo *NFOTVShow) {
	if nfo.Title != "" {
		series.Title = nfo.Title
	}
	if nfo.OrigTitle != "" {
		series.OrigTitle = nfo.OrigTitle
	}
	if nfo.Year > 0 {
		series.Year = nfo.Year
	}
	if nfo.Plot != "" {
		series.Overview = nfo.Plot
	}
	if nfo.Rating > 0 {
		series.Rating = nfo.Rating
	}
	allGenres := append(nfo.Genres, nfo.Tags...)
	if len(allGenres) > 0 {
		seen := make(map[string]bool)
		var deduped []string
		for _, g := range allGenres {
			g = strings.TrimSpace(g)
			if g != "" && !seen[g] {
				seen[g] = true
				deduped = append(deduped, g)
			}
		}
		series.Genres = strings.Join(deduped, ",")
	}
	if nfo.Studio != "" {
		series.Studio = nfo.Studio
	}
	if nfo.Country != "" {
		series.Country = nfo.Country
	}
	if nfo.TMDbID > 0 {
		series.TMDbID = nfo.TMDbID
	}
	if nfo.DoubanID != "" {
		series.DoubanID = nfo.DoubanID
	}
}
