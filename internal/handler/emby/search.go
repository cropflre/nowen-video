package emby

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== Search / Hints ====================
//
// Hills 的"统一搜索"与 Yamby 的顶部搜索框都会调用
// GET /Search/Hints?SearchTerm=xxx&UserId=xxx&Limit=N（部分版本用 /Search/Hints 的
// lowercase 变体）。它只需要轻量的 hint 结构，不需要完整 BaseItemDto。
//
// 此前未实现该端点，客户端搜索会拿到非 JSON 响应或 404，表现为"搜索无结果"。

// SearchHint 是 Emby SearchHint 的最小可用子集。
type SearchHint struct {
	ItemId          string `json:"ItemId"`
	Id              string `json:"Id"`
	Name            string `json:"Name"`
	Type            string `json:"Type"`
	MediaType       string `json:"MediaType"`
	ProductionYear  int    `json:"ProductionYear"`
	RunTimeTicks    int64  `json:"RunTimeTicks"`
	PrimaryImageTag string `json:"PrimaryImageTag,omitempty"`
	Series          string `json:"Series,omitempty"`
}

// SearchHintResult 对应 /Search/Hints 的响应。
type SearchHintResult struct {
	SearchHints      []SearchHint `json:"SearchHints"`
	TotalRecordCount int          `json:"TotalRecordCount"`
}

// SearchHintsHandler 对应 GET /Search/Hints。
func (h *Handler) SearchHintsHandler(c *gin.Context) {
	term := strings.TrimSpace(firstNonEmptyQuery(c, "SearchTerm", "searchTerm", "Query", "query"))
	limit := getIntQuery(c, "Limit", 30)
	if limit <= 0 || limit > 200 {
		limit = 30
	}

	hints := make([]SearchHint, 0, limit)
	if term != "" {
		if list, _, err := h.mediaRepo.Search(term, 1, limit); err == nil {
			for i := range list {
				hints = append(hints, hintFromItem(h.mapMediaToItem(&list[i], nil)))
			}
		}
		if list, _, err := h.seriesRepo.SearchSeries(term, 1, limit); err == nil {
			for i := range list {
				hints = append(hints, hintFromItem(h.mapSeriesToItem(&list[i], nil)))
			}
		}
	}
	if len(hints) > limit {
		hints = hints[:limit]
	}

	c.JSON(http.StatusOK, SearchHintResult{
		SearchHints:      hints,
		TotalRecordCount: len(hints),
	})
}

// hintFromItem 把已映射的条目降维成 SearchHint。
func hintFromItem(it BaseItemDto) SearchHint {
	return SearchHint{
		ItemId:          it.Id,
		Id:              it.Id,
		Name:            it.Name,
		Type:            it.Type,
		MediaType:       "Video",
		ProductionYear:  it.ProductionYear,
		RunTimeTicks:    it.RunTimeTicks,
		PrimaryImageTag: it.ImageTags["Primary"],
		Series:          it.SeriesName,
	}
}

// firstNonEmptyQuery 按顺序返回第一个非空的 query 值（兼容不同客户端的大小写习惯）。
func firstNonEmptyQuery(c *gin.Context, keys ...string) string {
	for _, k := range keys {
		if v := c.Query(k); v != "" {
			return v
		}
	}
	return ""
}
