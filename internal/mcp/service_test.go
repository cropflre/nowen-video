package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newTestService 搭一个内存库的 Service：不碰 data/ 目录。
func newTestService(t *testing.T, allowDirect bool) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:mcp-test-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := model.AutoMigrateLite(db, false); err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	cfg := &config.Config{}
	cfg.MCP.AllowDirectRating = allowDirect
	logger := zap.NewNop().Sugar()
	metadata := service.NewMetadataService(repos.Media, repos.Series, repos.Person, repos.MediaPerson, cfg, logger)
	return NewService(repos, metadata, cfg, logger)
}

func seedMovie(t *testing.T, svc *Service, id, title, mpaa, tmdbData string) {
	t.Helper()
	m := &model.Media{ID: id, LibraryID: "lib-1", Title: title, MediaType: "movie", MPAA: mpaa, TMDbRatingData: tmdbData, FilePath: "/tmp/" + id + ".mkv"}
	if err := svc.db.Create(m).Error; err != nil {
		t.Fatal(err)
	}
}

func seedSeriesWithEpisodes(t *testing.T, svc *Service, seriesID string, episodeIDs ...string) {
	t.Helper()
	sr := &model.Series{ID: seriesID, LibraryID: "lib-1", Title: "剧集" + seriesID, FolderPath: "/tmp/" + seriesID}
	if err := svc.db.Create(sr).Error; err != nil {
		t.Fatal(err)
	}
	for _, epID := range episodeIDs {
		ep := &model.Media{ID: epID, LibraryID: "lib-1", Title: "第1集", MediaType: "episode", SeriesID: seriesID, FilePath: "/tmp/" + epID + ".mkv"}
		if err := svc.db.Create(ep).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func countAudit(t *testing.T, svc *Service, action string) int64 {
	t.Helper()
	var n int64
	if err := svc.db.Model(&model.AuditLog{}).Where("action = ?", action).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// ==================== 分级建议规则 ====================

func TestSuggestFamilyRatingRule(t *testing.T) {
	usPG := `{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"PG"}]}]}`
	usPG13 := `{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"PG-13"}]}]}`
	usAndGB := `{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"PG"}]},{"iso_3166_1":"GB","release_dates":[{"certification":"12"}]}]}`
	twoUS := `{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"PG"},{"certification":"R"}]}]}`
	usTVMA := `{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"TV-MA"}]}]}`
	seriesTV14 := `{"results":[{"iso_3166_1":"US","rating":"TV-14"}]}`
	seriesTVMA := `{"results":[{"iso_3166_1":"US","rating":"TV-MA"}]}`

	cases := []struct {
		name       string
		mpaa       string
		tmdb       string
		series     bool
		wantLevel  string
		wantSource string
	}{
		{"本地明确分级", "PG-13", "", false, "PG-13", "本地 NFO"},
		{"本地 US 前缀", "US: R", "", false, "R", "本地 NFO"},
		{"TMDb 美国分级", "", usPG, false, "PG", "TMDb · 美国"},
		{"TMDb 优先于本地一致", "PG-13", usPG13, false, "PG-13", "TMDb · 美国"},
		{"剧集 TV-14", "", seriesTV14, true, "PG-13", "TMDb · 美国"},
		{"剧集 TV-MA", "", seriesTVMA, true, "NC-17", "TMDb · 美国"},
		{"本地非美分级不建议", "JP-18+", "", false, "", ""},
		{"本地与 TMDb 冲突不建议", "R", usPG, false, "", ""},
		{"混合地区不建议", "", usAndGB, false, "", ""},
		{"多个美国分级不建议", "", twoUS, false, "", ""},
		{"电影 TV-MA 无法映射不建议", "", usTVMA, false, "", ""},
		{"TMDb 数据损坏不建议", "", "{not json", false, "", ""},
		{"什么都没有不建议", "", "", false, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sug, note := suggestFamilyRating(c.mpaa, c.tmdb, c.series)
			if c.wantLevel == "" {
				if sug != nil {
					t.Fatalf("不该给建议，却给了 %+v", sug)
				}
				if note == "" {
					t.Fatal("不建议时应给出原因")
				}
				return
			}
			if sug == nil {
				t.Fatalf("该给建议却没给，原因：%s", note)
			}
			if sug.Level != c.wantLevel || sug.Source != c.wantSource {
				t.Fatalf("建议不对：got %+v want level=%s source=%s", sug, c.wantLevel, c.wantSource)
			}
		})
	}
}

// ==================== 搜索 / 详情 / 未确认清单 ====================

func TestSearchGetAndUnconfirmed(t *testing.T) {
	svc := newTestService(t, false)
	seedMovie(t, svc, "m1", "一家人的温暖故事", "PG", "")
	seedMovie(t, svc, "m2", "动作大片", "", `{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"PG-13"}]}]}`)
	seedMovie(t, svc, "m3", "科幻惊悚", "", "")
	seedSeriesWithEpisodes(t, svc, "s1", "e1", "e2")
	// m1 已确认 PG；e1 已确认 G。
	if err := svc.db.Create(&model.ContentRating{MediaID: "m1", Level: "PG"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.db.Create(&model.ContentRating{MediaID: "e1", Level: "G"}).Error; err != nil {
		t.Fatal(err)
	}

	// 全部搜索：3 部电影 + 1 个剧集（剧集不计入电影）。
	res, err := svc.SearchMedia("", "", "all", "all", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 4 {
		t.Fatalf("total=%d want 4", res.Total)
	}
	// 确认状态过滤。
	res, err = svc.SearchMedia("", "", "confirmed", "movie", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "m1" {
		t.Fatalf("confirmed 过滤不对：%+v", res)
	}
	res, err = svc.SearchMedia("", "", "unconfirmed", "movie", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 2 {
		t.Fatalf("unconfirmed total=%d want 2", res.Total)
	}
	// 关键词。
	res, err = svc.SearchMedia("温暖", "", "all", "movie", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 1 || res.Items[0].ID != "m1" {
		t.Fatalf("关键词搜索不对：%+v", res)
	}
	// 详情：m2 的建议应为 TMDb PG-13，且未确认。
	detail, err := svc.GetMedia("movie", "m2")
	if err != nil {
		t.Fatal(err)
	}
	d := detail.(*MediaDetail)
	if d.FamilyRatingConfirmed || d.Suggestion == nil || d.Suggestion.Level != "PG-13" {
		t.Fatalf("详情不对：%+v", d)
	}
	// 剧集详情：2 集，1 集已确认。
	sdetail, err := svc.GetMedia("series", "s1")
	if err != nil {
		t.Fatal(err)
	}
	sd := sdetail.(*SeriesDetail)
	if sd.TotalEpisodes != 2 || sd.ConfirmedEpisodes != 1 {
		t.Fatalf("剧集详情统计不对：%+v", sd)
	}
	// 未确认清单（电影）：m2、m3。
	un, err := svc.ListUnconfirmedRatings("movie", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if un.Total != 2 {
		t.Fatalf("未确认清单 total=%d want 2", un.Total)
	}
	// 未确认清单（剧集）：s1 有一集未确认。
	un, err = svc.ListUnconfirmedRatings("series", 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if un.Total != 1 || un.Items[0].ID != "s1" {
		t.Fatalf("剧集未确认清单不对：%+v", un)
	}
	// 不存在的 id。
	if _, err := svc.GetMedia("movie", "nope"); err == nil {
		t.Fatal("不存在的影片应报错")
	}
	if _, err := svc.GetMedia("bad-kind", "m1"); err == nil {
		t.Fatal("非法的 kind 应报错")
	}
	// 分页归一化：page 0 → 1，size 超限 → 100。
	res, err = svc.SearchMedia("", "", "all", "movie", 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	if res.Page != 1 || res.PageSize != 100 {
		t.Fatalf("分页归一化不对：page=%d size=%d", res.Page, res.PageSize)
	}
}

// ==================== 写分级开关与审计 ====================

func TestSetFamilyRatingGateAndAudit(t *testing.T) {
	// 开关关闭：拒绝，且库里无变化、无审计。
	svc := newTestService(t, false)
	seedMovie(t, svc, "m1", "测试片", "", "")
	if _, err := svc.SetFamilyRating("movie", "m1", "PG-13"); err == nil {
		t.Fatal("开关关闭时应拒绝写分级")
	} else if !contains(err.Error(), "allow_direct_rating") {
		t.Fatalf("拒绝原因应说明开关，got: %v", err)
	}
	if n := countAudit(t, svc, "mcp.set_family_rating"); n != 0 {
		t.Fatalf("被拒绝的写操作不应记审计，got %d", n)
	}
	var c int64
	if err := svc.db.Model(&model.ContentRating{}).Count(&c).Error; err != nil {
		t.Fatal(err)
	}
	if c != 0 {
		t.Fatalf("被拒绝的写操作不应写库，got %d", c)
	}

	// 开关开启：写入、改档、撤回，审计逐条记。
	svc.cfg.MCP.AllowDirectRating = true
	out, err := svc.SetFamilyRating("movie", "m1", "PG-13")
	if err != nil {
		t.Fatal(err)
	}
	if out["level"] != "PG-13" || out["confirmed"] != true {
		t.Fatalf("写入结果不对：%+v", out)
	}
	var r model.ContentRating
	if err := svc.db.Where("media_id = ?", "m1").First(&r).Error; err != nil || r.Level != "PG-13" {
		t.Fatalf("分级未写入：%+v %v", r, err)
	}
	if n := countAudit(t, svc, "mcp.set_family_rating"); n != 1 {
		t.Fatalf("审计应记 1 条，got %d", n)
	}
	// 改档。注意：gorm 的 First 会把结构体里已有的主键当成查询条件，
	// 复读必须用新的结构体变量，否则会查到旧行。
	if _, err := svc.SetFamilyRating("movie", "m1", "R"); err != nil {
		t.Fatal(err)
	}
	var r2 model.ContentRating
	if err := svc.db.Where("media_id = ?", "m1").First(&r2).Error; err != nil || r2.Level != "R" {
		t.Fatalf("改档未生效：%+v %v", r2, err)
	}
	// 撤回（level 为空）。
	if _, err := svc.SetFamilyRating("movie", "m1", ""); err != nil {
		t.Fatal(err)
	}
	var r3 model.ContentRating
	if err := svc.db.Where("media_id = ?", "m1").First(&r3).Error; err != gorm.ErrRecordNotFound {
		t.Fatalf("撤回后应无确认记录，got %+v %v", r3, err)
	}
	// 非法分级。
	if _, err := svc.SetFamilyRating("movie", "m1", "XXX"); err == nil {
		t.Fatal("非法分级应报错")
	}
	// 不存在的影片。
	if _, err := svc.SetFamilyRating("movie", "nope", "G"); err == nil {
		t.Fatal("不存在的影片应报错")
	}
}

func TestSetFamilyRatingSeriesAppliesToEpisodes(t *testing.T) {
	svc := newTestService(t, true)
	seedSeriesWithEpisodes(t, svc, "s1", "e1", "e2")
	out, err := svc.SetFamilyRating("series", "s1", "PG")
	if err != nil {
		t.Fatal(err)
	}
	if out["affected"] != 2 {
		t.Fatalf("剧集确认应作用于 2 集，got %+v", out)
	}
	var n int64
	if err := svc.db.Model(&model.ContentRating{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("应写入 2 条确认，got %d", n)
	}
	// 无剧集的剧集无法确认。
	seedSeriesWithEpisodes(t, svc, "s2")
	if _, err := svc.SetFamilyRating("series", "s2", "PG"); err == nil {
		t.Fatal("无剧集的剧集应报错")
	}
}

func TestClearFamilyRating(t *testing.T) {
	svc := newTestService(t, true)
	seedMovie(t, svc, "m1", "测试片", "", "")
	if _, err := svc.SetFamilyRating("movie", "m1", "G"); err != nil {
		t.Fatal(err)
	}
	out, err := svc.ClearFamilyRating("movie", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if out["changed"] != true {
		t.Fatalf("首次清除应 changed=true，got %+v", out)
	}
	out, err = svc.ClearFamilyRating("movie", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if out["changed"] != false {
		t.Fatalf("重复清除应 changed=false，got %+v", out)
	}
	if n := countAudit(t, svc, "mcp.clear_family_rating"); n != 2 {
		t.Fatalf("清除审计应记 2 条，got %d", n)
	}
	// 开关关闭时清除也被拒绝。
	svc.cfg.MCP.AllowDirectRating = false
	if _, err := svc.ClearFamilyRating("movie", "m1"); err == nil {
		t.Fatal("开关关闭时清除也应拒绝")
	}
}

// ==================== 元数据更新 / 清除 ====================

func TestUpdateMediaMetadataWhitelist(t *testing.T) {
	svc := newTestService(t, true)
	seedMovie(t, svc, "m1", "旧标题", "", "")
	seedSeriesWithEpisodes(t, svc, "s1", "e1")

	out, err := svc.UpdateMediaMetadata("movie", "m1", MetadataUpdateInput{
		Title: strptr("新标题"), Year: intptr(2020), Genres: strptr("剧情,家庭"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["changed"] != true {
		t.Fatalf("应标记 changed，got %+v", out)
	}
	var m model.Media
	if err := svc.db.Where("id = ?", "m1").First(&m).Error; err != nil {
		t.Fatal(err)
	}
	if m.Title != "新标题" || m.Year != 2020 || m.Genres != "剧情,家庭" {
		t.Fatalf("元数据未更新：%+v", m)
	}
	if n := countAudit(t, svc, "mcp.update_media_metadata"); n != 1 {
		t.Fatalf("审计应记 1 条，got %d", n)
	}
	// 非法年份。
	if _, err := svc.UpdateMediaMetadata("movie", "m1", MetadataUpdateInput{Year: intptr(1800)}); err == nil {
		t.Fatal("非法年份应报错")
	}
	// 空标题。
	if _, err := svc.UpdateMediaMetadata("movie", "m1", MetadataUpdateInput{Title: strptr("  ")}); err == nil {
		t.Fatal("空标题应报错")
	}
	// 一个字段都不给。
	if _, err := svc.UpdateMediaMetadata("movie", "m1", MetadataUpdateInput{}); err == nil {
		t.Fatal("无字段应报错")
	}
	// 剧集也能改。
	if _, err := svc.UpdateMediaMetadata("series", "s1", MetadataUpdateInput{Title: strptr("新剧名")}); err != nil {
		t.Fatal(err)
	}
	var sr model.Series
	if err := svc.db.Where("id = ?", "s1").First(&sr).Error; err != nil || sr.Title != "新剧名" {
		t.Fatalf("剧集标题未更新：%+v %v", sr, err)
	}
}

func TestClearTMDbRatingData(t *testing.T) {
	svc := newTestService(t, false)
	seedMovie(t, svc, "m1", "测试片", "", `{"results":[]}`)
	out, err := svc.ClearTMDbRatingData("movie", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if out["changed"] != true {
		t.Fatalf("首次清除应 changed=true，got %+v", out)
	}
	var m model.Media
	if err := svc.db.Where("id = ?", "m1").First(&m).Error; err != nil || m.TMDbRatingData != "" {
		t.Fatalf("分级数据未清除：%+v %v", m, err)
	}
	out, err = svc.ClearTMDbRatingData("movie", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if out["changed"] != false {
		t.Fatalf("重复清除应 changed=false，got %+v", out)
	}
	if n := countAudit(t, svc, "mcp.clear_tmdb_rating_data"); n != 2 {
		t.Fatalf("审计应记 2 条，got %d", n)
	}
}

// ==================== 重刮：失败时优雅返回 ====================

func TestRescrapeMediaFailsGracefullyWithoutTMDb(t *testing.T) {
	// 用本地 httptest 冒充 TMDb，让刮削快速失败，不碰真实网络。
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer proxy.Close()
	svc := newTestService(t, false)
	svc.cfg.Secrets.TMDbAPIKey = "test"
	svc.cfg.Secrets.TMDbAPIProxy = proxy.URL
	// 换上走代理的 MetadataService（NewService 时的那个用的是默认配置）。
	svc.metadata = service.NewMetadataService(svc.media, svc.series, svc.person, svc.mediaPerson, svc.cfg, svc.logger)
	// 用纯随机 ASCII 标题：TMDb 代理 404、豆瓣建议接口也搜不到，
	// 刮削走完所有 Fallback 后应返回错误，而不是静默成功。
	// 注意标题里不能含真实中文词（如“不存在”），否则豆瓣模糊匹配会命中。
	seedMovie(t, svc, "m1", "zxqvkjw9f3k2x", "", "")
	if _, err := svc.RescrapeMedia("movie", "m1"); err == nil {
		t.Fatal("TMDb 不可用时重刮应返回错误，而不是 panic 或静默成功")
	}
	if n := countAudit(t, svc, "mcp.rescrape_media"); n != 1 {
		t.Fatalf("失败的重刮也应记审计，got %d", n)
	}
	if _, err := svc.RescrapeMedia("movie", "nope"); err == nil {
		t.Fatal("不存在的影片应报错")
	}
}

func strptr(s string) *string { return &s }
func intptr(i int) *int       { return &i }

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
