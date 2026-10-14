package emby

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
)

// ==================== 兼容性补齐端点 ====================
//
// Hills / Yamby（以及部分 Emby/Jellyfin 第三方客户端）在进入服务器初始化和浏览
// 过程中会调用以下端点。官方 Emby 都有实现；缺失时客户端会拿到 404 或落到前端
// index.html，轻则报错、重则中断初始化。
//
// 这些端点的数据都来自本服务已有的媒体库/媒体表，不引入新表、不改变既有语义。

// VirtualFoldersHandler 对应 GET /Library/VirtualFolders。
// Emby 返回 VirtualFolderInfo 数组（含物理路径），用于客户端的"文件夹"视图与设置页。
func (h *Handler) VirtualFoldersHandler(c *gin.Context) {
	libs, err := h.libraryRepo.List()
	if err != nil {
		c.JSON(http.StatusOK, []gin.H{})
		return
	}
	out := make([]gin.H, 0, len(libs))
	for i := range libs {
		lib := &libs[i]
		view := h.mapLibraryToView(lib)
		out = append(out, gin.H{
			"Name":           lib.Name,
			"ItemId":         view.Id,
			"Id":             view.Id,
			"Locations":      lib.AllPaths(),
			"CollectionType": view.CollectionType,
			"LibraryOptions": gin.H{
				"PathInfos": []gin.H{},
			},
		})
	}
	c.JSON(http.StatusOK, out)
}

// FiltersHandler 对应 GET /Items/Filters（旧版）。
// Filters2Handler 对应 GET /Items/Filters2（新版，Yamby 等使用）。
//
// 返回可用筛选项：Genres / OfficialRatings / Tags / Years。
func (h *Handler) FiltersHandler(c *gin.Context) {
	c.JSON(http.StatusOK, h.buildQueryFilters())
}

// Filters2Handler 与 FiltersHandler 同构。
// Emby 的 Filters2 额外支持 IncludeItemTypes 等参数，但返回结构一致。
func (h *Handler) Filters2Handler(c *gin.Context) {
	c.JSON(http.StatusOK, h.buildQueryFilters())
}

// buildQueryFilters 聚合当前媒体库可用的筛选维度。
func (h *Handler) buildQueryFilters() gin.H {
	medias, _, _ := h.mediaRepo.List(1, 2000, "")

	genres := make(map[string]struct{})
	ratings := make(map[string]struct{})
	years := make(map[int]struct{})
	for i := range medias {
		m := &medias[i]
		for _, g := range splitGenres(m.Genres) {
			genres[g] = struct{}{}
		}
		if m.MPAA != "" {
			ratings[m.MPAA] = struct{}{}
		}
		if m.Year > 0 {
			years[m.Year] = struct{}{}
		}
	}

	genreList := make([]string, 0, len(genres))
	for g := range genres {
		genreList = append(genreList, g)
	}
	sort.Strings(genreList)

	ratingList := make([]string, 0, len(ratings))
	for r := range ratings {
		ratingList = append(ratingList, r)
	}
	sort.Strings(ratingList)

	// Years 在 Emby 中是字符串数组，且按降序返回
	yearList := make([]int, 0, len(years))
	for y := range years {
		yearList = append(yearList, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(yearList)))
	yearStrs := make([]string, 0, len(yearList))
	for _, y := range yearList {
		yearStrs = append(yearStrs, strconv.Itoa(y))
	}

	return gin.H{
		"Genres":          genreList,
		"Tags":            []string{},
		"OfficialRatings": ratingList,
		"Years":           yearStrs,
	}
}

// ==================== 空集合端点 ====================
//
// 这些能力本服务不提供（没有本地预告片 / 花絮 / 分段视频），但 Emby 客户端会在
// 详情页无条件拉取。返回空数组即可让客户端正常渲染，比 404 更友好。

// IntrosHandler 对应 /Items/{id}/Intros 与 /Users/{userId}/Items/{id}/Intros。
func (h *Handler) IntrosHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []BaseItemDto{})
}

// LocalTrailersHandler 对应 /Items/{id}/LocalTrailers 与
// /Users/{userId}/Items/{id}/LocalTrailers。
func (h *Handler) LocalTrailersHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []BaseItemDto{})
}

// AdditionalPartsHandler 对应 /Videos/{id}/AdditionalParts 与
// /Items/{id}/AdditionalParts。
func (h *Handler) AdditionalPartsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []BaseItemDto{})
}

// SpecialFeaturesHandler 对应 /Users/{userId}/Items/{id}/SpecialFeatures。
func (h *Handler) SpecialFeaturesHandler(c *gin.Context) {
	c.JSON(http.StatusOK, []BaseItemDto{})
}
