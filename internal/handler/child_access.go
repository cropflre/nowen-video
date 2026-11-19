package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service"
	"gorm.io/gorm"
)

// ChildAccessGate is mounted after authentication. Child reads use filtered SQL
// queries; every other child endpoint must be explicitly admitted below.
func ChildAccessGate(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		catalog := service.NewChildCatalog(db).WithContext(c.Request.Context())
		policy, err := catalog.Policy(c.GetString("user_id"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "暂时无法确认账号权限"})
			return
		}
		if !policy.ChildMode {
			c.Next()
			return
		}
		c.Writer = childResponseWriter{ResponseWriter: c.Writer}
		c.Header("Cache-Control", "private, no-store")
		c.Header("Pragma", "no-cache")
		gate := childRequest{c: c, catalog: catalog, policy: policy, db: db.WithContext(c.Request.Context())}
		if gate.dispatch() {
			c.Next()
		} else {
			c.Abort()
		}
	}
}

type childRequest struct {
	c       *gin.Context
	catalog *service.ChildCatalog
	policy  service.ChildPolicy
	db      *gorm.DB
}

func (h childRequest) fail(status int) bool {
	h.c.JSON(status, gin.H{"error": "此内容暂未向儿童账号开放"})
	return false
}
func (h childRequest) respond(data interface{}, err error) bool {
	if err != nil {
		h.c.JSON(http.StatusServiceUnavailable, gin.H{"error": "暂时无法读取影片"})
	} else {
		h.c.JSON(http.StatusOK, gin.H{"data": data})
	}
	return false
}
func (h childRequest) page() (int, int) {
	page, _ := strconv.Atoi(h.c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(h.c.DefaultQuery("size", "30"))
	if page < 1 || page > 1_000_000 {
		page = 1
	}
	if size < 1 || size > 2000 {
		size = 30
	}
	return page, size
}
func (h childRequest) limit() int {
	n, _ := strconv.Atoi(h.c.DefaultQuery("limit", "20"))
	if n < 1 || n > 100 {
		return 20
	}
	return n
}
func (h childRequest) filter() service.ChildCatalogFilter {
	c := h.c
	f := service.ChildCatalogFilter{Query: c.Query("q"), LibraryID: c.Query("library_id"), Genre: c.Query("genre"), Type: c.Query("type"), Sort: c.Query("sort"), Order: c.Query("order")}
	if f.Sort == "" {
		f.Sort = c.Query("sort_by")
	}
	if f.Order == "" {
		f.Order = c.Query("sort_order")
	}
	f.YearMin, _ = strconv.Atoi(c.Query("year_from"))
	if f.YearMin == 0 {
		f.YearMin, _ = strconv.Atoi(c.Query("year_min"))
	}
	f.YearMax, _ = strconv.Atoi(c.Query("year_to"))
	if f.YearMax == 0 {
		f.YearMax, _ = strconv.Atoi(c.Query("year_max"))
	}
	f.MinRating, _ = strconv.ParseFloat(c.Query("min_rating"), 64)
	return f
}
func (h childRequest) paged(data interface{}, total int64, err error) bool {
	if err != nil {
		return h.respond(nil, err)
	}
	page, size := h.page()
	h.c.JSON(http.StatusOK, gin.H{"data": data, "total": total, "page": page, "size": size})
	return false
}
func (h childRequest) mediaOK(id string) bool  { return h.catalog.CheckMedia(h.policy, id) == nil }
func (h childRequest) seriesOK(id string) bool { return h.catalog.CheckSeries(h.policy, id) == nil }
func (h childRequest) dispatch() bool {
	c := h.c
	route := c.FullPath()
	method := c.Request.Method
	page, size := h.page()
	filter := h.filter()
	if (route == "/api/users/me/family-controls" && method == http.MethodGet) ||
		((route == "/api/users/me/viewing/start" || route == "/api/users/me/viewing/renew" || route == "/api/users/me/viewing/stop") && method == http.MethodPost) {
		return true
	}
	if ((route == "/api/users/me/devices" || route == "/api/users/me/device-status") && method == http.MethodGet) ||
		(route == "/api/users/me/devices/:deviceId" && method == http.MethodDelete) ||
		((route == "/api/users/me/devices/revoke-all" || route == "/api/auth/logout" || route == "/api/users/me/device-pairing/inspect" || route == "/api/users/me/device-pairing/approve") && method == http.MethodPost) {
		return true
	}
	if route == "/api/users/me" && (method == http.MethodGet || method == http.MethodPut) {
		return true
	}
	sessionMethods := map[string]string{
		"/api/playback/sessions":                                                  http.MethodPost,
		"/api/playback/sessions/:sessionID/status":                                http.MethodGet,
		"/api/playback/sessions/:sessionID/heartbeat":                             http.MethodPost,
		"/api/playback/sessions/:sessionID/restart":                               http.MethodPost,
		"/api/playback/sessions/:sessionID":                                       http.MethodDelete,
		"/api/playback/sessions/:sessionID/generations/:generationID/stream.m3u8": http.MethodGet,
		"/api/playback/sessions/:sessionID/generations/:generationID/:file":       http.MethodGet,
	}
	if expected, ok := sessionMethods[route]; ok && method == expected {
		// Ownership plus current media visibility is revalidated by every session
		// handler, including segment access after a parent's policy change.
		return true
	}
	if route == "/api/stats/playback" && method == http.MethodPost {
		return h.fail(http.StatusForbidden)
	}
	if strings.HasPrefix(route, "/api/users/me/progress/") { // Progress identifiers are never trusted from request bodies.
		if h.mediaOK(c.Param("mediaId")) {
			return true
		}
		return h.fail(http.StatusNotFound)
	}
	if strings.HasPrefix(route, "/api/users/me/favorites/") {
		if h.mediaOK(c.Param("mediaId")) {
			return true
		}
		return h.fail(http.StatusNotFound)
	}
	if method != http.MethodGet {
		return h.fail(http.StatusForbidden)
	}
	switch route {
	case "/api/libraries":
		items, err := h.catalog.Libraries(h.policy)
		return h.respond(items, err)
	case "/api/media", "/api/media/aggregated", "/api/search", "/api/search/advanced":
		if route == "/api/media/aggregated" {
			filter.Type = "movie"
		}
		items, total, err := h.catalog.Media(h.policy, filter, page, size)
		return h.paged(items, total, err)
	case "/api/media/recent":
		items, _, err := h.catalog.Media(h.policy, filter, 1, h.limit())
		return h.respond(items, err)
	case "/api/media/mixed", "/api/media/recent/mixed":
		if route == "/api/media/recent/mixed" {
			page = 1
			size = h.limit()
		}
		result, err := h.catalog.Mixed(h.policy, filter, page, size)
		if err != nil {
			return h.respond(nil, err)
		}
		c.JSON(http.StatusOK, gin.H{"data": result.Items, "total": result.Total, "movie_count": result.MovieCount, "series_count": result.SeriesCount, "page": page, "size": size})
		return false
	case "/api/search/mixed", "/api/media/recent/aggregated":
		filter.Type = "movie"
		media, mediaTotal, err := h.catalog.Media(h.policy, filter, page, size)
		if err != nil {
			return h.respond(nil, err)
		}
		shows, showTotal, err := h.catalog.Series(h.policy, filter, page, size)
		if err != nil {
			return h.respond(nil, err)
		}
		c.JSON(http.StatusOK, gin.H{"media": media, "series": shows, "media_total": mediaTotal, "series_total": showTotal, "page": page, "size": size})
		return false
	case "/api/media/continue":
		items, _, err := h.catalog.History(h.policy, true, 1, h.limit())
		return h.respond(items, err)
	case "/api/users/me/history":
		items, total, err := h.catalog.History(h.policy, false, page, size)
		return h.paged(items, total, err)
	case "/api/users/me/favorites":
		items, total, err := h.catalog.Favorites(h.policy, page, size)
		return h.paged(items, total, err)
	case "/api/recommend", "/api/recommend/similar/:mediaId":
		if id := c.Param("mediaId"); id != "" && !h.mediaOK(id) {
			return h.fail(http.StatusNotFound)
		}
		filter.Type = "movie"
		items, _, err := h.catalog.Media(h.policy, filter, 1, h.limit())
		if err != nil {
			return h.respond(nil, err)
		}
		recs := []gin.H{}
		for _, item := range items {
			if item.ID != c.Param("mediaId") {
				recs = append(recs, gin.H{"media": item, "score": 0, "reason": "家长已确认分级"})
			}
		}
		return h.respond(recs, nil)
	case "/api/series":
		items, total, err := h.catalog.Series(h.policy, filter, page, size)
		return h.paged(items, total, err)
	case "/api/series/:id", "/api/series/:id/seasons", "/api/series/:id/seasons/:season", "/api/series/:id/next":
		return h.series(route)
	case "/api/series/:id/poster", "/api/series/:id/backdrop", "/api/series/:id/logo":
		if h.seriesOK(c.Param("id")) {
			return true
		}
		return h.fail(http.StatusNotFound)
	case "/api/series/:id/persons":
		if !h.seriesOK(c.Param("id")) {
			return h.fail(http.StatusNotFound)
		}
		return h.respond([]interface{}{}, nil)
	case "/api/media/:id/enhanced":
		if !h.mediaOK(c.Param("id")) {
			return h.fail(http.StatusNotFound)
		}
		return h.respond(gin.H{"tech_specs": nil, "file_info": nil, "library": nil, "playback_stats": nil}, nil)
	case "/api/media/:id/versions":
		items := []model.Media{}
		err := h.catalog.MediaQuery(h.policy).Where("media.id = ?", c.Param("id")).Find(&items).Error
		if err == nil && len(items) == 0 {
			return h.fail(http.StatusNotFound)
		}
		return h.respond(items, err)
	case "/api/media/:id/collection":
		if !h.mediaOK(c.Param("id")) {
			return h.fail(http.StatusNotFound)
		}
		return h.respond(nil, nil)
	case "/api/media/:id/persons", "/api/media/:id/comments":
		if !h.mediaOK(c.Param("id")) {
			return h.fail(http.StatusNotFound)
		}
		c.JSON(http.StatusOK, gin.H{"data": []interface{}{}, "total": 0, "avg_rating": 0, "rating_count": 0})
		return false
	case "/api/media/:id", "/api/media/:id/poster", "/api/media/:id/backdrop", "/api/media/:id/logo",
		"/api/media/:id/highlights", "/api/media/:id/highlights/status", "/api/media/:id/highlights/:highlightId/thumbnail", "/api/media/:id/highlights/:highlightId/preview", "/api/media/:id/chapters",
		"/api/stream/:id/info", "/api/stream/:id/plan", "/api/stream/:id/direct", "/api/stream/:id/remux", "/api/stream/:id/strm-seg",
		"/api/subtitle/:id/tracks", "/api/subtitle/:id/extract/:index":
		if h.mediaOK(c.Param("id")) {
			if strings.HasPrefix(route, "/api/stream/") && route != "/api/stream/:id/info" && route != "/api/stream/:id/plan" || strings.HasSuffix(route, "/preview") {
				if err := service.NewFamilyPlaybackService(h.db).CheckAccess(h.policy.UserID, c.Param("id")); err != nil {
					c.JSON(http.StatusForbidden, gin.H{"error": "请重新确认观看时间后播放", "code": "FAMILY_LEASE_REQUIRED"})
					return false
				}
			}
			return true
		}
		return h.fail(http.StatusNotFound)
	case "/api/subtitle/external":
		if h.externalSubtitleOK(c.Query("path")) {
			return true
		}
		return h.fail(http.StatusNotFound)
	case "/api/playlists", "/api/collections", "/api/collections/search", "/api/users/me/login-logs":
		return h.paged([]interface{}{}, 0, nil)
	case "/api/persons/:id":
		if c.Param("id") == "search" {
			return h.respond([]interface{}{}, nil)
		}
		return h.fail(http.StatusNotFound)
	case "/api/ai/search":
		return h.respond(gin.H{"parsed": false}, nil)
	default:
		return h.fail(http.StatusForbidden)
	}
}
func (h childRequest) series(route string) bool {
	id := h.c.Param("id")
	if !h.seriesOK(id) {
		return h.fail(http.StatusNotFound)
	}
	query := h.catalog.MediaQuery(h.policy).Where("media.series_id = ?", id)
	if route == "/api/series/:id/next" {
		season, _ := strconv.Atoi(h.c.Query("season"))
		episode, _ := strconv.Atoi(h.c.Query("episode"))
		var next model.Media
		err := query.Where("season_num > ? OR (season_num = ? AND episode_num > ?)", season, season, episode).Order("season_num ASC, episode_num ASC, id ASC").First(&next).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return h.respond(nil, nil)
		}
		return h.respond(&next, err)
	}
	if route == "/api/series/:id/seasons/:season" {
		season, _ := strconv.Atoi(h.c.Param("season"))
		query = query.Where("season_num = ?", season)
	}
	episodes := []model.Media{}
	if err := query.Order("season_num ASC, episode_num ASC, id ASC").Find(&episodes).Error; err != nil {
		return h.respond(nil, err)
	}
	if route == "/api/series/:id/seasons/:season" {
		return h.respond(episodes, nil)
	}
	seasons := []service.SeasonInfo{}
	for _, episode := range episodes {
		if len(seasons) == 0 || seasons[len(seasons)-1].SeasonNum != episode.SeasonNum {
			seasons = append(seasons, service.SeasonInfo{SeasonNum: episode.SeasonNum, Episodes: []model.Media{}})
		}
		last := &seasons[len(seasons)-1]
		last.Episodes = append(last.Episodes, episode)
		last.EpisodeCount++
	}
	if route == "/api/series/:id/seasons" {
		return h.respond(seasons, nil)
	}
	var series model.Series
	if err := h.catalog.SeriesQuery(h.policy).First(&series, "series.id = ?", id).Error; err != nil {
		return h.respond(nil, err)
	}
	series.Episodes = episodes
	series.EpisodeCount = len(episodes)
	series.SeasonCount = len(seasons)
	return h.respond(series, nil)
}
func (h childRequest) externalSubtitleOK(requested string) bool {
	if requested == "" {
		return false
	}
	items := []model.Media{}
	if err := h.catalog.MediaQuery(h.policy).Where("subtitle_paths LIKE ?", "%"+requested+"%").Select("subtitle_paths").Find(&items).Error; err != nil {
		return false
	}
	for _, item := range items {
		paths := []string{}
		if json.Unmarshal([]byte(item.SubtitlePaths), &paths) != nil {
			paths = strings.FieldsFunc(item.SubtitlePaths, func(r rune) bool { return r == '\n' || r == ',' || r == ';' || r == '|' })
		}
		for _, path := range paths {
			if filepath.Clean(strings.TrimSpace(path)) == filepath.Clean(requested) {
				return true
			}
		}
	}
	return false
}

// Image/stream handlers may otherwise replace the middleware's cache header.
type childResponseWriter struct{ gin.ResponseWriter }

func (w childResponseWriter) protect() {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Pragma", "no-cache")
	if !strings.Contains(strings.ToLower(strings.Join(w.Header().Values("Vary"), ",")), "authorization") {
		w.Header().Add("Vary", "Authorization")
	}
}
func (w childResponseWriter) WriteHeader(code int) { w.protect(); w.ResponseWriter.WriteHeader(code) }
func (w childResponseWriter) WriteHeaderNow()      { w.protect(); w.ResponseWriter.WriteHeaderNow() }
func (w childResponseWriter) Write(data []byte) (int, error) {
	w.protect()
	return w.ResponseWriter.Write(data)
}
func (w childResponseWriter) WriteString(data string) (int, error) {
	w.protect()
	return w.ResponseWriter.WriteString(data)
}
func (w childResponseWriter) Flush() { w.protect(); w.ResponseWriter.Flush() }
