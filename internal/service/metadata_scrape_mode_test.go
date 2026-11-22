package service

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

// fakeTMDbModeServer 挡住 TMDb 搜索/详情/图片接口，返回带海报与类型的固定数据。
// 图片路径返回 1x1 PNG 字节，避免测试里触发真实下载。
func fakeTMDbModeServer() *httptest.Server {
	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	mux := http.NewServeMux()
	mux.HandleFunc("/3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"id":7,"title":"测试电影","original_title":"Test Movie","overview":"新简介","poster_path":"/p7.jpg","backdrop_path":"/b7.jpg","release_date":"2021-05-01","vote_average":8.5,"genre_ids":[18,35]},
			{"id":8,"title":"测试电影续集","original_title":"Test Movie 2","overview":"另一部","poster_path":"/p8.jpg","backdrop_path":"","release_date":"2023-01-01","vote_average":6.0,"genre_ids":[28]}
		]}`))
	})
	mux.HandleFunc("/3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[
			{"id":7,"name":"测试剧集","original_name":"Test Show","overview":"新剧集简介","poster_path":"/p7.jpg","backdrop_path":"/b7.jpg","first_air_date":"2020-01-01","vote_average":8.0,"genre_ids":[18,35]}
		]}`))
	})
	mux.HandleFunc("/3/movie/7", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"title":"测试电影","original_title":"Test Movie","overview":"新简介","poster_path":"/p7.jpg","backdrop_path":"/b7.jpg","release_date":"2021-05-01","vote_average":8.5,"runtime":120,"genres":[{"id":18,"name":"剧情"},{"id":35,"name":"喜剧"}],"release_dates":{"results":[]}}`))
	})
	mux.HandleFunc("/3/tv/7/content_ratings", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[]}`))
	})
	mux.HandleFunc("/3/tv/7/credits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cast":[],"crew":[]}`))
	})
	mux.HandleFunc("/3/movie/7/credits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cast":[],"crew":[]}`))
	})
	mux.HandleFunc("/3/tv/7/season/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":1,"season_number":1,"episodes":[]}`))
	})
	mux.HandleFunc("/t/p/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(png)
	})
	return httptest.NewServer(mux)
}


// TestMatchScoreForResult 打分信号与 bestMatchResult 一致：
// 标题完全相等+年份精确的候选得分最高；年份差 2 年以上被重罚。
func TestMatchScoreForResult(t *testing.T) {
	exact := TMDbMovie{Title: "测试电影", ReleaseDate: "2021-05-01", PosterPath: "/p.jpg", Overview: "简介", VoteAverage: 8.0}
	partial := TMDbMovie{Title: "测试电影续集", ReleaseDate: "2023-01-01", PosterPath: "/p.jpg", Overview: "简介", VoteAverage: 6.0}
	otherYear := TMDbMovie{Title: "测试电影", ReleaseDate: "1995-01-01", PosterPath: "/p.jpg", Overview: "简介", VoteAverage: 8.0}

	exactScore := matchScoreForResult(exact, "测试电影", 2021, 0)
	partialScore := matchScoreForResult(partial, "测试电影", 2021, 1)
	otherYearScore := matchScoreForResult(otherYear, "测试电影", 2021, 2)
	if !(exactScore > partialScore) {
		t.Fatalf("exact score %v should beat partial %v", exactScore, partialScore)
	}
	// 同标题时，年份精确（101）远高于年份差 2 年以上（-33 = 50-100+15+2）。
	if otherYearScore != -33 {
		t.Fatalf("year-mismatch score = %v, want -33", otherYearScore)
	}
	if !(exactScore > otherYearScore) {
		t.Fatalf("exact score %v should beat year-mismatch %v", exactScore, otherYearScore)
	}
	// 年份重罚的意义：同名旧片（标题精确但年份差 2 年以上）输给
	// 当年上映的弱标题匹配，避免热门同名条目串到所有年份。
	sameYearWeak := TMDbMovie{Title: "测试电影前传", ReleaseDate: "2021-03-01", PosterPath: "/p.jpg", Overview: "简介", VoteAverage: 5.0}
	sameYearWeakScore := matchScoreForResult(sameYearWeak, "测试电影", 2021, 3)
	if !(sameYearWeakScore > otherYearScore) {
		t.Fatalf("same-year weak %v should beat year-mismatch %v", sameYearWeakScore, otherYearScore)
	}
	// 精确分 = 标题 50 + 年份 30 + 完整性 15 + 排名 6 = 101。
	if exactScore != 101 {
		t.Fatalf("exact score = %v, want 101", exactScore)
	}

	// 自动选择不受展示打分影响：仍选得分最高的同一条。
	svc, _, _ := newCreditsTestService(t)
	best := svc.bestMatchResult([]TMDbMovie{partial, exact}, "测试电影", 2021)
	if best.Title != "测试电影" {
		t.Fatalf("bestMatchResult picked %q", best.Title)
	}
}

// TestSearchTMDbAttachesMatchScores 搜索响应里每个候选项都带上匹配度分数。
func TestSearchTMDbAttachesMatchScores(t *testing.T) {
	server := fakeTMDbModeServer()
	defer server.Close()
	svc, _, _ := newCreditsTestService(t)
	svc.client = server.Client()
	svc.cfg.Secrets.TMDbAPIProxy = server.URL
	svc.cfg.Secrets.TMDbImageProxy = server.URL

	results, err := svc.SearchTMDb("movie", "测试电影", 2021)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].MatchScore <= results[1].MatchScore {
		t.Fatalf("first candidate score %v should beat second %v", results[0].MatchScore, results[1].MatchScore)
	}
	if results[0].MatchScore != 101 {
		t.Fatalf("first candidate score = %v, want 101", results[0].MatchScore)
	}
}

// TestApplyMovieDetailModes 只补缺失保留已有值、补空字段；
// 全部替换覆盖；锁定字段两档都跳过。
func TestApplyMovieDetailModes(t *testing.T) {
	detail := &TMDbMovieDetail{
		ID: 7, Title: "新标题", OriginalTitle: "New Title", Overview: "新简介",
		VoteAverage: 8.5, ReleaseDate: "2021-05-01", Runtime: 120,
		Genres: []TMDbGenre{{Name: "剧情"}},
	}

	// 只补缺失：已有值保留，空字段补上。
	media := &model.Media{Title: "旧标题", Overview: "旧简介", Rating: 7.0, Year: 2019, Genres: "旧类型"}
	(&MetadataService{}).applyMovieDetail(media, detail, newScrapeGuard(ScrapeModeFillMissing, ""))
	if media.Title != "旧标题" || media.Overview != "旧简介" || media.Rating != 7.0 || media.Year != 2019 || media.Genres != "旧类型" {
		t.Fatalf("fill-missing overwrote existing values: %+v", media)
	}
	if media.OrigTitle != "New Title" {
		t.Fatalf("fill-missing did not fill empty orig_title: %q", media.OrigTitle)
	}

	// 全部替换：覆盖。
	media2 := &model.Media{Title: "旧标题", Overview: "旧简介", Rating: 7.0, Year: 2019, Genres: "旧类型"}
	(&MetadataService{}).applyMovieDetail(media2, detail, newScrapeGuard(ScrapeModeReplace, ""))
	if media2.Title != "新标题" || media2.Overview != "新简介" || media2.Rating != 8.5 || media2.Year != 2021 || media2.Genres != "剧情" {
		t.Fatalf("replace did not overwrite: %+v", media2)
	}

	// 锁定：两档都跳过被锁定字段，其他字段正常写。
	media3 := &model.Media{Title: "旧标题", Overview: "旧简介"}
	(&MetadataService{}).applyMovieDetail(media3, detail, newScrapeGuard(ScrapeModeReplace, `["title"]`))
	if media3.Title != "旧标题" {
		t.Fatalf("locked title overwritten in replace mode: %q", media3.Title)
	}
	if media3.Overview != "新简介" {
		t.Fatalf("unlocked overview not written: %q", media3.Overview)
	}
	media4 := &model.Media{Title: "旧标题", Overview: ""}
	(&MetadataService{}).applyMovieDetail(media4, detail, newScrapeGuard(ScrapeModeFillMissing, `["overview"]`))
	if media4.Overview != "" {
		t.Fatalf("locked empty overview filled in fill-missing mode: %q", media4.Overview)
	}
	if media4.Title != "旧标题" {
		t.Fatalf("fill-missing overwrote title: %q", media4.Title)
	}
}

// TestScrapeMovieFillMissingKeepsPoster 只补缺失档下已有海报不重复下载、
// 简介不覆盖；全部替换档下海报被重新下载。
func TestScrapeMovieFillMissingKeepsPoster(t *testing.T) {
	server := fakeTMDbModeServer()
	defer server.Close()

	newMedia := func() *model.Media {
		dir := t.TempDir()
		return &model.Media{
			ID: "m1", LibraryID: "lib", Title: "测试电影", MediaType: "movie",
			FilePath: filepath.Join(dir, "movie.mkv"),
			Overview: "旧简介", Rating: 7.0, PosterPath: filepath.Join(dir, "old-poster.jpg"),
		}
	}

	// 只补缺失：海报与简介保留。
	svc, db, _ := newCreditsTestService(t)
	svc.client = server.Client()
	svc.cfg.Secrets.TMDbAPIProxy = server.URL
	svc.cfg.Secrets.TMDbImageProxy = server.URL
	media := newMedia()
	if err := db.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	oldPoster := media.PosterPath
	if err := svc.scrapeMovie(media, "测试电影", 2021, ScrapeModeFillMissing); err != nil {
		t.Fatalf("scrapeMovie fill-missing: %v", err)
	}
	if media.Overview != "旧简介" {
		t.Fatalf("fill-missing overwrote overview: %q", media.Overview)
	}
	if media.PosterPath != oldPoster {
		t.Fatalf("fill-missing re-downloaded poster: %q", media.PosterPath)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(media.FilePath), "movie-poster.jpg")); err == nil {
		t.Fatal("fill-missing downloaded a poster file")
	}

	// 全部替换：海报重新下载、简介覆盖。
	svc2, db2, _ := newCreditsTestService(t)
	svc2.client = server.Client()
	svc2.cfg.Secrets.TMDbAPIProxy = server.URL
	svc2.cfg.Secrets.TMDbImageProxy = server.URL
	media2 := newMedia()
	oldPoster2 := media2.PosterPath
	if err := db2.Create(media2).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc2.scrapeMovie(media2, "测试电影", 2021, ScrapeModeReplace); err != nil {
		t.Fatalf("scrapeMovie replace: %v", err)
	}
	if media2.Overview != "新简介" {
		t.Fatalf("replace did not overwrite overview: %q", media2.Overview)
	}
	if media2.PosterPath == oldPoster2 {
		t.Fatalf("replace kept old poster path: %q", media2.PosterPath)
	}
}

// TestScrapeSeriesWithModeLegacyPath 剧集旧版路径：只补缺失保留已有值，
// 全部替换覆盖，锁定字段两档都跳过。
func TestScrapeSeriesWithModeLegacyPath(t *testing.T) {
	server := fakeTMDbModeServer()
	defer server.Close()

	run := func(mode ScrapeMode, locked string) *model.Series {
		svc, db, _ := newCreditsTestService(t)
		svc.client = server.Client()
		svc.cfg.Secrets.TMDbAPIProxy = server.URL
		svc.cfg.Secrets.TMDbImageProxy = server.URL
		series := &model.Series{
			ID: "s1", LibraryID: "lib", Title: "我的剧集", FolderPath: "/shows/我的剧集",
			Overview: "已有简介", Rating: 7.5, Year: 2019, Genres: "已有类型",
			PosterPath: "/old/poster.jpg", LockedFields: locked,
		}
		if err := db.Create(series).Error; err != nil {
			t.Fatal(err)
		}
		if err := svc.ScrapeSeriesWithMode("s1", mode); err != nil {
			t.Fatalf("ScrapeSeriesWithMode(%s): %v", mode, err)
		}
		updated, err := svc.seriesRepo.FindByID("s1")
		if err != nil {
			t.Fatal(err)
		}
		return updated
	}

	fill := run(ScrapeModeFillMissing, "")
	if fill.Overview != "已有简介" || fill.Rating != 7.5 || fill.Year != 2019 || fill.Genres != "已有类型" || fill.PosterPath != "/old/poster.jpg" {
		t.Fatalf("fill-missing overwrote series fields: %+v", fill)
	}
	if fill.TMDbID != 7 {
		t.Fatalf("fill-missing did not record tmdb id: %d", fill.TMDbID)
	}

	replaced := run(ScrapeModeReplace, "")
	if replaced.Overview != "新剧集简介" || replaced.Rating != 8.0 || replaced.Year != 2020 {
		t.Fatalf("replace did not overwrite series fields: %+v", replaced)
	}

	locked := run(ScrapeModeReplace, `["overview","poster"]`)
	if locked.Overview != "已有简介" {
		t.Fatalf("locked overview overwritten: %q", locked.Overview)
	}
	if locked.PosterPath != "/old/poster.jpg" {
		t.Fatalf("locked poster overwritten: %q", locked.PosterPath)
	}
	if locked.Rating != 8.0 {
		t.Fatalf("unlocked rating not overwritten: %v", locked.Rating)
	}
}

// TestProviderChainFillMissingMode 调度链只补缺失档下所有数据源都走 supplement：
// 已有字段保留；全部替换档下首个数据源覆盖写入。
func TestProviderChainFillMissingMode(t *testing.T) {
	server := fakeTMDbModeServer()
	defer server.Close()

	run := func(mode ScrapeMode) *model.Series {
		svc, db, _ := newCreditsTestService(t)
		svc.client = server.Client()
		svc.cfg.Secrets.TMDbAPIProxy = server.URL
		svc.cfg.Secrets.TMDbImageProxy = server.URL
		chain := NewProviderChain(svc.logger)
		chain.Register(NewTMDbProvider(svc))
		svc.SetProviderChain(chain)
		series := &model.Series{
			ID: "s1", LibraryID: "lib", Title: "我的剧集", FolderPath: "/shows/我的剧集",
			Overview: "已有简介", Rating: 7.5,
		}
		if err := db.Create(series).Error; err != nil {
			t.Fatal(err)
		}
		if err := chain.ScrapeSeries(series, "测试剧集", 2020, mode); err != nil {
			t.Fatalf("chain ScrapeSeries(%s): %v", mode, err)
		}
		return series
	}

	fill := run(ScrapeModeFillMissing)
	if fill.Overview != "已有简介" || fill.Rating != 7.5 {
		t.Fatalf("chain fill-missing overwrote: %+v", fill)
	}
	replaced := run(ScrapeModeReplace)
	if replaced.Overview != "新剧集简介" || replaced.Rating != 8.0 {
		t.Fatalf("chain replace did not overwrite: %+v", replaced)
	}
}

// TestNormalizeLockedFields 只保留可锁定字段，去重并排序。
func TestNormalizeLockedFields(t *testing.T) {
	if got := NormalizeLockedFields(`["rating","title","rating","cast","bogus"]`); got != `["title","rating"]` {
		t.Fatalf("NormalizeLockedFields = %q", got)
	}
	if got := NormalizeLockedFields(""); got != "" {
		t.Fatalf("empty NormalizeLockedFields = %q", got)
	}
	if got := NormalizeLockedFields("not json"); got != "" {
		t.Fatalf("invalid NormalizeLockedFields = %q", got)
	}
	if set := ParseLockedFields(`["overview"]`); !set["overview"] || set["title"] {
		t.Fatalf("ParseLockedFields = %v", set)
	}
}
