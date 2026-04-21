package emby

import (
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== Library Views ====================
//
// Emby 左侧导航：/Users/{id}/Views 返回顶层的 "库" 列表（CollectionFolder）。
// Infuse 进入服务器后首先调这个接口获取媒体库。

// UserViewsHandler 对应 /Users/{userId}/Views。
func (h *Handler) UserViewsHandler(c *gin.Context) {
	libs, err := h.libraryRepo.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to load libraries"})
		return
	}
	items := make([]BaseItemDto, 0, len(libs))
	for i := range libs {
		items = append(items, h.mapLibraryToView(&libs[i]))
	}
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
		StartIndex:       0,
	})
}

// MediaFoldersHandler 对应 /Library/MediaFolders（管理员视角的物理库）。
// 与 UserViews 输出类似，但不过滤权限。
func (h *Handler) MediaFoldersHandler(c *gin.Context) {
	h.UserViewsHandler(c)
}

// ==================== 通用 Items 查询 ====================

// itemsQuery 描述 Emby /Items 支持的主要筛选维度（最小可用子集）。
//
// Infuse 常用以下参数：
//   - ParentId      指定某个库或 Series/Season 的 Id
//   - IncludeItemTypes=Movie,Series,Episode
//   - Recursive=true
//   - SortBy=SortName,ProductionYear,DateCreated,DatePlayed
//   - SortOrder=Ascending|Descending
//   - StartIndex / Limit
//   - SearchTerm
//   - Filters=IsFavorite
//   - IsPlayed / IsFavorite / IsNotPlayed
type itemsQuery struct {
	parentUUID     string
	includeTypes   []string // Movie / Series / Episode / Folder
	recursive      bool
	sortBy         []string
	sortOrder      string
	startIndex     int
	limit          int
	searchTerm     string
	filterIsFav    bool
	filterIsPlayed *bool // nil = don't filter
	genreFilter    string
	yearFilter     int
}

func (h *Handler) parseItemsQuery(c *gin.Context) itemsQuery {
	q := itemsQuery{
		parentUUID:   h.idMap.Resolve(c.Query("ParentId")),
		recursive:    boolQuery(c, "Recursive"),
		startIndex:   getIntQuery(c, "StartIndex", 0),
		limit:        getIntQuery(c, "Limit", 100),
		searchTerm:   strings.TrimSpace(c.Query("SearchTerm")),
		sortOrder:    c.Query("SortOrder"),
		genreFilter:  c.Query("Genres"),
		yearFilter:   getIntQuery(c, "Years", 0),
	}
	if q.limit <= 0 || q.limit > 500 {
		q.limit = 100
	}
	if it := c.Query("IncludeItemTypes"); it != "" {
		q.includeTypes = splitAndTrim(it)
	}
	if sb := c.Query("SortBy"); sb != "" {
		q.sortBy = splitAndTrim(sb)
	}
	if f := c.Query("Filters"); f != "" {
		for _, v := range splitAndTrim(f) {
			switch strings.ToLower(v) {
			case "isfavorite":
				q.filterIsFav = true
			case "isplayed":
				b := true
				q.filterIsPlayed = &b
			case "isunplayed", "isnotplayed":
				b := false
				q.filterIsPlayed = &b
			}
		}
	}
	if c.Query("IsFavorite") == "true" {
		q.filterIsFav = true
	}
	if v := c.Query("IsPlayed"); v == "true" || v == "false" {
		b := v == "true"
		q.filterIsPlayed = &b
	}
	return q
}

// ItemsHandler 对应：
//   GET /Users/{userId}/Items
//   GET /Items
//
// 在 nowen 里，顶层条目有两种：Movie（media_type="movie" 且 series_id 为空）和 Series。
// 进入 Series 则列出 Episodes。
// 进入 Library 则列出它下面的 Movies + Series。
func (h *Handler) ItemsHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	q := h.parseItemsQuery(c)

	// 如果指定了 ParentId，判断它是 Library / Series / Season
	if q.parentUUID != "" {
		if _, err := h.libraryRepo.FindByID(q.parentUUID); err == nil {
			h.listLibraryChildren(c, userID, q)
			return
		}
		if _, err := h.seriesRepo.FindByIDOnly(q.parentUUID); err == nil {
			h.listSeriesEpisodes(c, userID, q, q.parentUUID)
			return
		}
	}

	// 没有 ParentId → 按 IncludeItemTypes 全局查询
	h.listGlobalItems(c, userID, q)
}

// listLibraryChildren 列出指定媒体库下的直接子条目：
//   - media_type=movie 且 series_id 为空的 Media → Movie
//   - 所有该库下的 Series → Series
func (h *Handler) listLibraryChildren(c *gin.Context, userID string, q itemsQuery) {
	includeMovie, includeSeries, includeEpisode := h.decideTypes(q.includeTypes)

	// 记录并合并结果
	var items []BaseItemDto

	if (includeMovie || len(q.includeTypes) == 0) && !q.recursive {
		list, _, err := h.mediaRepo.ListNonEpisode(1, 5000, q.parentUUID)
		if err == nil {
			items = append(items, h.filterAndMapMedia(list, userID, q)...)
		}
	}
	if includeSeries || len(q.includeTypes) == 0 {
		seriesList, err := h.seriesRepo.ListByLibraryID(q.parentUUID)
		if err == nil {
			items = append(items, h.filterAndMapSeries(seriesList, userID, q)...)
		}
	}
	if includeEpisode && q.recursive {
		epList, err := h.mediaRepo.ListByLibraryID(q.parentUUID)
		if err == nil {
			filtered := make([]model.Media, 0, len(epList))
			for i := range epList {
				if epList[i].MediaType == "episode" {
					filtered = append(filtered, epList[i])
				}
			}
			items = append(items, h.filterAndMapMedia(filtered, userID, q)...)
		}
	}

	h.writeItemsResult(c, items, q)
}

// listSeriesEpisodes 列出某个 Series 的全部 Episode。
func (h *Handler) listSeriesEpisodes(c *gin.Context, userID string, q itemsQuery, seriesUUID string) {
	eps, err := h.mediaRepo.ListBySeriesID(seriesUUID)
	if err != nil {
		c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: []BaseItemDto{}})
		return
	}
	items := h.filterAndMapMedia(eps, userID, q)
	h.writeItemsResult(c, items, q)
}

// listGlobalItems 在没有 ParentId 时，跨所有媒体库做查询。
// 常见用途：全局搜索 SearchTerm、全局 "最新添加" 列表。
func (h *Handler) listGlobalItems(c *gin.Context, userID string, q itemsQuery) {
	includeMovie, includeSeries, includeEpisode := h.decideTypes(q.includeTypes)

	var items []BaseItemDto

	// 如果是搜索，优先走 Search API
	if q.searchTerm != "" {
		if includeMovie || includeEpisode || len(q.includeTypes) == 0 {
			res, _, err := h.mediaRepo.Search(q.searchTerm, 1, 200)
			if err == nil {
				items = append(items, h.filterAndMapMedia(res, userID, q)...)
			}
		}
		if includeSeries || len(q.includeTypes) == 0 {
			res, _, err := h.seriesRepo.SearchSeries(q.searchTerm, 1, 200)
			if err == nil {
				items = append(items, h.filterAndMapSeries(res, userID, q)...)
			}
		}
		h.writeItemsResult(c, items, q)
		return
	}

	// 非搜索：根据类型取全库
	if includeMovie || len(q.includeTypes) == 0 {
		res, _, err := h.mediaRepo.ListNonEpisode(1, 2000, "")
		if err == nil {
			items = append(items, h.filterAndMapMedia(res, userID, q)...)
		}
	}
	if includeSeries || len(q.includeTypes) == 0 {
		res, _, err := h.seriesRepo.List(1, 2000, "")
		if err == nil {
			items = append(items, h.filterAndMapSeries(res, userID, q)...)
		}
	}
	if includeEpisode {
		// 仅在显式请求时返回全部 Episode，避免结果爆炸
		eps, err := h.mediaRepo.ListByMediaType("episode")
		if err == nil {
			items = append(items, h.filterAndMapMedia(eps, userID, q)...)
		}
	}

	h.writeItemsResult(c, items, q)
}

// decideTypes 根据 IncludeItemTypes 决定返回 Movie/Series/Episode 三类的开关。
func (h *Handler) decideTypes(types []string) (movie, series, episode bool) {
	if len(types) == 0 {
		return true, true, false
	}
	for _, t := range types {
		switch strings.ToLower(t) {
		case "movie":
			movie = true
		case "series":
			series = true
		case "episode":
			episode = true
		case "video":
			movie = true
			episode = true
		case "folder", "boxset", "collectionfolder":
			// 忽略：Library 列表由 /Views 提供
		}
	}
	return
}

// filterAndMapMedia 把 Media 列表转为 DTO，并应用 UserData 过滤（收藏/已播放）。
func (h *Handler) filterAndMapMedia(list []model.Media, userID string, q itemsQuery) []BaseItemDto {
	out := make([]BaseItemDto, 0, len(list))
	for i := range list {
		m := &list[i]

		// 类型过滤
		if len(q.includeTypes) > 0 {
			wantMovie, wantSeries, wantEpisode := h.decideTypes(q.includeTypes)
			_ = wantSeries
			if m.MediaType == "episode" {
				if !wantEpisode {
					continue
				}
			} else {
				if !wantMovie {
					continue
				}
			}
		}

		// 年份筛选
		if q.yearFilter > 0 && m.Year != q.yearFilter {
			continue
		}
		// 类型（genre）筛选
		if q.genreFilter != "" && !strings.Contains(strings.ToLower(m.Genres), strings.ToLower(q.genreFilter)) {
			continue
		}
		// 搜索（如果上层没有走 search API 的 fallback）
		if q.searchTerm != "" {
			needle := strings.ToLower(q.searchTerm)
			if !strings.Contains(strings.ToLower(m.Title), needle) &&
				!strings.Contains(strings.ToLower(m.OrigTitle), needle) {
				continue
			}
		}

		ud := h.buildUserItemData(userID, m)

		// 收藏/播放筛选
		if q.filterIsFav && (ud == nil || !ud.IsFavorite) {
			continue
		}
		if q.filterIsPlayed != nil {
			if ud == nil {
				if *q.filterIsPlayed {
					continue
				}
			} else if ud.Played != *q.filterIsPlayed {
				continue
			}
		}

		out = append(out, h.mapMediaToItem(m, ud))
	}
	return out
}

// filterAndMapSeries 把 Series 列表转为 DTO。
func (h *Handler) filterAndMapSeries(list []model.Series, userID string, q itemsQuery) []BaseItemDto {
	out := make([]BaseItemDto, 0, len(list))
	for i := range list {
		s := &list[i]
		if q.yearFilter > 0 && s.Year != q.yearFilter {
			continue
		}
		if q.genreFilter != "" && !strings.Contains(strings.ToLower(s.Genres), strings.ToLower(q.genreFilter)) {
			continue
		}
		if q.searchTerm != "" {
			needle := strings.ToLower(q.searchTerm)
			if !strings.Contains(strings.ToLower(s.Title), needle) &&
				!strings.Contains(strings.ToLower(s.OrigTitle), needle) {
				continue
			}
		}
		out = append(out, h.mapSeriesToItem(s, nil))
	}
	return out
}

// writeItemsResult 对已映射完成的条目做排序、分页，然后输出。
func (h *Handler) writeItemsResult(c *gin.Context, items []BaseItemDto, q itemsQuery) {
	sortItems(items, q.sortBy, q.sortOrder)

	total := len(items)
	start := q.startIndex
	if start < 0 {
		start = 0
	}
	if start > total {
		start = total
	}
	end := start + q.limit
	if q.limit <= 0 || end > total {
		end = total
	}
	paged := items[start:end]

	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{
		Items:            paged,
		TotalRecordCount: total,
		StartIndex:       start,
	})
}

// sortItems 按 SortBy 排序，支持多键。
func sortItems(items []BaseItemDto, sortBy []string, order string) {
	if len(sortBy) == 0 {
		sortBy = []string{"SortName"}
	}
	asc := !strings.EqualFold(order, "Descending")

	sort.SliceStable(items, func(i, j int) bool {
		for _, key := range sortBy {
			ci := compareBy(items[i], items[j], key)
			if ci == 0 {
				continue
			}
			if asc {
				return ci < 0
			}
			return ci > 0
		}
		return false
	})
}

func compareBy(a, b BaseItemDto, key string) int {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "sortname", "name":
		return strings.Compare(strings.ToLower(a.SortName+a.Name), strings.ToLower(b.SortName+b.Name))
	case "productionyear", "year", "premieredate":
		return intCompare(a.ProductionYear, b.ProductionYear)
	case "communityrating", "rating", "criticrating":
		return floatCompare(a.CommunityRating, b.CommunityRating)
	case "runtime":
		return int64Compare(a.RunTimeTicks, b.RunTimeTicks)
	case "datecreated", "dateadded":
		return strings.Compare(a.DateCreated, b.DateCreated)
	case "datelastcontentadded", "datemodified":
		return strings.Compare(a.DateCreated, b.DateCreated)
	case "random":
		return 0
	}
	return 0
}

func intCompare(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
func int64Compare(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}
func floatCompare(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// ==================== 单个 Item 详情 ====================

// ItemHandler 对应：
//   GET /Users/{userId}/Items/{itemId}
//   GET /Items/{itemId}
func (h *Handler) ItemHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("id"))

	// 媒体库
	if lib, err := h.libraryRepo.FindByID(uuid); err == nil {
		c.JSON(http.StatusOK, h.mapLibraryToView(lib))
		return
	}

	// Series
	if s, err := h.seriesRepo.FindByIDOnly(uuid); err == nil {
		item := h.mapSeriesToItem(s, nil)
		// 附带演员
		item.People = h.loadPeopleForSeries(uuid)
		c.JSON(http.StatusOK, item)
		return
	}

	// Media
	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"Error": err.Error()})
		}
		return
	}

	ud := h.buildUserItemData(userID, m)
	item := h.mapMediaToItem(m, ud)
	item.People = h.loadPeopleForMedia(m.ID)
	item.MediaSources = []MediaSourceInfo{h.buildMediaSource(m, c)}
	item.MediaStreams = item.MediaSources[0].MediaStreams
	c.JSON(http.StatusOK, item)
}

// SimilarItemsHandler 对应 /Items/{id}/Similar。
// 简化实现：返回同 genre 的其他条目。
func (h *Handler) SimilarItemsHandler(c *gin.Context) {
	uuid := h.idMap.Resolve(c.Param("id"))
	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: []BaseItemDto{}})
		return
	}
	genres := splitGenres(m.Genres)
	limit := getIntQuery(c, "Limit", 12)
	list, err := h.mediaRepo.ListByGenres(genres, []string{m.ID}, limit)
	if err != nil {
		c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: []BaseItemDto{}})
		return
	}
	items := make([]BaseItemDto, 0, len(list))
	for i := range list {
		items = append(items, h.mapMediaToItem(&list[i], nil))
	}
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: items, TotalRecordCount: len(items)})
}

// ==================== Series / Seasons / Episodes ====================

// SeasonsHandler 对应 /Shows/{seriesId}/Seasons。
// nowen 没有单独的 Season 表——通过 Media.SeasonNum 聚合返回虚拟 Season 条目。
func (h *Handler) SeasonsHandler(c *gin.Context) {
	seriesUUID := h.idMap.Resolve(c.Param("seriesId"))
	series, err := h.seriesRepo.FindByIDOnly(seriesUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Series not found"})
		return
	}
	seasonNums, err := h.seriesRepo.GetSeasonNumbers(seriesUUID)
	if err != nil {
		seasonNums = nil
	}
	items := make([]BaseItemDto, 0, len(seasonNums))
	for _, sn := range seasonNums {
		items = append(items, BaseItemDto{
			Name:              seasonName(sn),
			ServerId:          h.serverID,
			Id:                h.idMap.ToEmbyID(seriesUUID + "|season|" + itoa(sn)),
			IsFolder:          true,
			Type:              "Season",
			ParentId:          h.idMap.ToEmbyID(seriesUUID),
			SeriesId:          h.idMap.ToEmbyID(seriesUUID),
			SeriesName:        series.Title,
			IndexNumber:       sn,
			ProductionYear:    series.Year,
			ImageTags:         buildSeriesImageTags(series),
			BackdropImageTags: []string{},
			LocationType:      "FileSystem",
		})
	}
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
	})
}

// EpisodesHandler 对应 /Shows/{seriesId}/Episodes。
// 支持 Season 过滤：?SeasonId=xxx 或 ?Season=1。
func (h *Handler) EpisodesHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	seriesUUID := h.idMap.Resolve(c.Param("seriesId"))

	seasonNum := getIntQuery(c, "Season", 0)
	// SeasonId 来自我们自己生成的虚拟 ID（seriesUUID|season|N），在 mapper 中已反向解析
	if sid := c.Query("SeasonId"); sid != "" {
		if resolved := h.idMap.Resolve(sid); resolved != "" {
			// 形如 "uuid|season|N"
			if parts := strings.Split(resolved, "|season|"); len(parts) == 2 {
				seasonNum = atoiSafe(parts[1])
			}
		}
	}

	var list []model.Media
	var err error
	if seasonNum > 0 {
		list, err = h.mediaRepo.ListBySeriesAndSeason(seriesUUID, seasonNum)
	} else {
		list, err = h.mediaRepo.ListBySeriesID(seriesUUID)
	}
	if err != nil {
		c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: []BaseItemDto{}})
		return
	}

	items := make([]BaseItemDto, 0, len(list))
	for i := range list {
		ud := h.buildUserItemData(userID, &list[i])
		items = append(items, h.mapMediaToItem(&list[i], ud))
	}
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
	})
}

// NextUpHandler 对应 /Shows/NextUp。
// 返回用户正在追看的剧集下一集；简化实现：取 ContinueWatching 中的 episode。
func (h *Handler) NextUpHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := getIntQuery(c, "Limit", 20)
	histories, err := h.watchRepo.ContinueWatching(userID, limit*2)
	if err != nil {
		c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: []BaseItemDto{}})
		return
	}
	items := make([]BaseItemDto, 0, limit)
	for i := range histories {
		m := &histories[i].Media
		if m.ID == "" || m.MediaType != "episode" {
			continue
		}
		ud := h.buildUserItemData(userID, m)
		items = append(items, h.mapMediaToItem(m, ud))
		if len(items) >= limit {
			break
		}
	}
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
	})
}

// LatestItemsHandler 对应 /Users/{userId}/Items/Latest（首页"最新添加"）。
func (h *Handler) LatestItemsHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := getIntQuery(c, "Limit", 20)
	parentUUID := h.idMap.Resolve(c.Query("ParentId"))

	var medias []model.Media
	var err error
	if parentUUID != "" {
		// 取某个库下最近新增的非剧集
		medias, err = h.mediaRepo.RecentNonEpisodeAll(parentUUID)
	} else {
		medias, err = h.mediaRepo.RecentNonEpisode(limit * 2)
	}
	if err != nil {
		c.JSON(http.StatusOK, []BaseItemDto{})
		return
	}
	if len(medias) > limit {
		medias = medias[:limit]
	}
	items := make([]BaseItemDto, 0, len(medias))
	for i := range medias {
		ud := h.buildUserItemData(userID, &medias[i])
		items = append(items, h.mapMediaToItem(&medias[i], ud))
	}
	// 注意：Latest 接口返回的是裸数组，不是 QueryResult
	c.JSON(http.StatusOK, items)
}

// ResumeHandler 对应 /Users/{userId}/Items/Resume（"继续观看"）。
func (h *Handler) ResumeHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := getIntQuery(c, "Limit", 20)
	histories, err := h.watchRepo.ContinueWatching(userID, limit)
	if err != nil {
		c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: []BaseItemDto{}})
		return
	}
	items := make([]BaseItemDto, 0, len(histories))
	for i := range histories {
		m := &histories[i].Media
		if m.ID == "" {
			continue
		}
		ud := &UserItemData{
			PlaybackPositionTicks: secondsToTicks(histories[i].Position),
			PlayCount:             1,
			IsFavorite:            h.favoriteRepo.Exists(userID, m.ID),
			Played:                histories[i].Completed,
			Key:                   m.ID,
			LastPlayedDate:        formatEmbyTime(histories[i].UpdatedAt),
		}
		if histories[i].Duration > 0 {
			ud.PlayedPercentage = histories[i].Position / histories[i].Duration * 100.0
		}
		items = append(items, h.mapMediaToItem(m, ud))
	}
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{
		Items:            items,
		TotalRecordCount: len(items),
	})
}

// GenresHandler 对应 /Genres。简化：聚合 Media 的 Genres 列表。
func (h *Handler) GenresHandler(c *gin.Context) {
	// 直接取最近 2000 个 media 的 genres 聚合
	medias, _, _ := h.mediaRepo.List(1, 2000, "")
	set := make(map[string]struct{}, 64)
	for _, m := range medias {
		for _, g := range splitGenres(m.Genres) {
			set[g] = struct{}{}
		}
	}
	items := make([]BaseItemDto, 0, len(set))
	for g := range set {
		items = append(items, BaseItemDto{
			Name:     g,
			Id:       h.idMap.ToEmbyID("genre:" + g),
			Type:     "Genre",
			ServerId: h.serverID,
			IsFolder: true,
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	c.JSON(http.StatusOK, QueryResult[BaseItemDto]{Items: items, TotalRecordCount: len(items)})
}

// ==================== Helpers ====================

func (h *Handler) loadPeopleForMedia(mediaUUID string) []BaseItemPerson {
	mps, err := h.mediaPersonRepo.ListByMediaID(mediaUUID)
	if err != nil {
		return nil
	}
	return h.buildPeople(mps)
}

func (h *Handler) loadPeopleForSeries(seriesUUID string) []BaseItemPerson {
	mps, err := h.mediaPersonRepo.ListBySeriesID(seriesUUID)
	if err != nil {
		return nil
	}
	return h.buildPeople(mps)
}

func (h *Handler) buildPeople(mps []model.MediaPerson) []BaseItemPerson {
	out := make([]BaseItemPerson, 0, len(mps))
	for i := range mps {
		mp := &mps[i]
		t := "Actor"
		switch strings.ToLower(mp.Role) {
		case "director":
			t = "Director"
		case "writer":
			t = "Writer"
		case "producer":
			t = "Producer"
		}
		out = append(out, BaseItemPerson{
			Name: mp.Person.Name,
			Id:   h.idMap.ToEmbyID(mp.PersonID),
			Type: t,
			Role: mp.Character,
		})
	}
	return out
}

// buildUserItemData 组装 Emby UserData。
func (h *Handler) buildUserItemData(userID string, m *model.Media) *UserItemData {
	if userID == "" || m == nil {
		return nil
	}
	ud := &UserItemData{Key: m.ID}
	if hst, err := h.watchRepo.GetByUserAndMedia(userID, m.ID); err == nil && hst != nil {
		ud.PlaybackPositionTicks = secondsToTicks(hst.Position)
		ud.Played = hst.Completed
		if hst.Duration > 0 {
			ud.PlayedPercentage = hst.Position / hst.Duration * 100.0
		}
		ud.LastPlayedDate = formatEmbyTime(hst.UpdatedAt)
		if hst.Completed || hst.Position > 0 {
			ud.PlayCount = 1
		}
	}
	ud.IsFavorite = h.favoriteRepo.Exists(userID, m.ID)
	return ud
}

func boolQuery(c *gin.Context, key string) bool {
	v := strings.ToLower(c.Query(key))
	return v == "true" || v == "1"
}

func splitAndTrim(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func seasonName(n int) string {
	if n <= 0 {
		return "Specials"
	}
	return "Season " + itoa(n)
}
