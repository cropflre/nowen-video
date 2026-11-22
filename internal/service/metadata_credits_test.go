package service

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
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newCreditsTestService 搭一个内存库的 MetadataService：不碰 data/ 目录。
func newCreditsTestService(t *testing.T) (*MetadataService, *gorm.DB, *repository.Repositories) {
	t.Helper()
	dsn := fmt.Sprintf("file:credits-test-%d?mode=memory&cache=shared", time.Now().UnixNano())
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
	cfg.Secrets.TMDbAPIKey = "test-key"
	cfg.Cache.CacheDir = t.TempDir()
	svc := NewMetadataService(repos.Media, repos.Series, repos.Person, repos.MediaPerson, cfg, zap.NewNop().Sugar())
	return svc, db, repos
}

// fakeTMDbCreditsServer 挡住 TMDb 的搜索/详情/credits 接口，返回固定数据。
// 海报与头像路径都留空，避免测试里触发真实下载。
func fakeTMDbCreditsServer() *httptest.Server {
	creditsJSON := `{"cast":[{"id":101,"name":"演员甲","character":"主角","profile_path":""}],"crew":[{"id":201,"name":"导演乙","job":"Director","profile_path":""}]}`
	mux := http.NewServeMux()
	mux.HandleFunc("/3/search/tv", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":7,"name":"测试剧集","original_name":"Test Show","overview":"剧集简介","poster_path":"","backdrop_path":"","first_air_date":"2020-01-01","vote_average":8.0}]}`))
	})
	mux.HandleFunc("/3/search/movie", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":7,"title":"测试电影","original_title":"Test Movie","overview":"电影简介","poster_path":"","backdrop_path":"","release_date":"2021-05-01","vote_average":7.5}]}`))
	})
	mux.HandleFunc("/3/movie/7", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"title":"测试电影","original_title":"Test Movie","overview":"电影简介","poster_path":"","backdrop_path":"","release_date":"2021-05-01","vote_average":7.5,"genres":[],"release_dates":{"results":[]}}`))
	})
	mux.HandleFunc("/3/movie/7/credits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(creditsJSON))
	})
	mux.HandleFunc("/3/tv/7", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":7,"name":"测试剧集","original_name":"Test Show","overview":"剧集简介","poster_path":"","backdrop_path":"","first_air_date":"2020-01-01","vote_average":8.0}`))
	})
	mux.HandleFunc("/3/tv/7/credits", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(creditsJSON))
	})
	return httptest.NewServer(mux)
}

func personNames(t *testing.T, repos *repository.Repositories, rows []model.MediaPerson) []string {
	t.Helper()
	var names []string
	for _, row := range rows {
		person, err := repos.Person.FindByID(row.PersonID)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, person.Name+":"+row.Role)
	}
	return names
}

func TestSaveCreditsEmptyResultKeepsExisting(t *testing.T) {
	svc, db, repos := newCreditsTestService(t)

	person := &model.Person{Name: "旧演员", TMDbID: 999}
	if err := db.Create(person).Error; err != nil {
		t.Fatal(err)
	}
	if err := repos.MediaPerson.Create(&model.MediaPerson{MediaID: "m1", PersonID: person.ID, Role: "actor", SortOrder: 0}); err != nil {
		t.Fatal(err)
	}

	// 空结果与 nil 都不能清掉已有演职。
	svc.saveCreditsForMedia("m1", &TMDbCredits{})
	svc.saveCreditsForMedia("m1", nil)
	rows, err := repos.MediaPerson.ListByMediaID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PersonID != person.ID {
		t.Fatalf("empty credits wiped existing cast: %+v", rows)
	}

	// 非空结果正常替换旧关联。
	svc.saveCreditsForMedia("m1", &TMDbCredits{Cast: []TMDbCastMember{{ID: 1, Name: "新演员"}}})
	rows, err = repos.MediaPerson.ListByMediaID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row after replace, got %d", len(rows))
	}
	if got := personNames(t, repos, rows); got[0] != "新演员:actor" {
		t.Fatalf("unexpected replacement: %v", got)
	}
}

func TestSaveCreditsForSeriesEmptyResultKeepsExisting(t *testing.T) {
	svc, db, repos := newCreditsTestService(t)

	person := &model.Person{Name: "旧演员", TMDbID: 999}
	if err := db.Create(person).Error; err != nil {
		t.Fatal(err)
	}
	if err := repos.MediaPerson.Create(&model.MediaPerson{SeriesID: "s1", PersonID: person.ID, Role: "actor", SortOrder: 0}); err != nil {
		t.Fatal(err)
	}

	svc.saveCreditsForSeries("s1", &TMDbCredits{})
	rows, err := repos.MediaPerson.ListBySeriesID("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].PersonID != person.ID {
		t.Fatalf("empty credits wiped existing series cast: %+v", rows)
	}
}

func TestLoadAndSaveTMDbCreditsFailureKeepsExisting(t *testing.T) {
	svc, db, repos := newCreditsTestService(t)

	person := &model.Person{Name: "旧演员", TMDbID: 999}
	if err := db.Create(person).Error; err != nil {
		t.Fatal(err)
	}
	if err := repos.MediaPerson.Create(&model.MediaPerson{MediaID: "m1", PersonID: person.ID, Role: "actor", SortOrder: 0}); err != nil {
		t.Fatal(err)
	}

	called := false
	svc.loadAndSaveTMDbCredits(
		func(int) (*TMDbCredits, error) { return nil, fmt.Errorf("boom") },
		func(*TMDbCredits) { called = true },
		"m1", "标题", 7,
	)
	if called {
		t.Fatal("save must not run when credits fetch fails")
	}
	rows, err := repos.MediaPerson.ListByMediaID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("failed fetch changed existing cast: %+v", rows)
	}
}

func TestScrapeTVFetchesCredits(t *testing.T) {
	server := fakeTMDbCreditsServer()
	defer server.Close()
	svc, db, repos := newCreditsTestService(t)
	svc.client = server.Client()
	svc.cfg.Secrets.TMDbAPIProxy = server.URL

	media := &model.Media{ID: "m1", LibraryID: "lib", Title: "测试剧集", MediaType: "episode", FilePath: "/tmp/m1.mkv"}
	if err := db.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.scrapeTV(media, "测试剧集", 2020, ScrapeModeReplace); err != nil {
		t.Fatalf("scrapeTV: %v", err)
	}
	if media.Overview != "剧集简介" {
		t.Fatalf("overview not applied: %q", media.Overview)
	}
	rows, err := repos.MediaPerson.ListByMediaID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 cast rows, got %d: %+v", len(rows), rows)
	}
	names := personNames(t, repos, rows)
	want := map[string]bool{"演员甲:actor": true, "导演乙:director": true}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("unexpected cast row: %q (all: %v)", n, names)
		}
	}
}

func TestScrapeTVByTMDbIDFetchesCredits(t *testing.T) {
	server := fakeTMDbCreditsServer()
	defer server.Close()
	svc, db, repos := newCreditsTestService(t)
	svc.client = server.Client()
	svc.cfg.Secrets.TMDbAPIProxy = server.URL

	media := &model.Media{ID: "m1", LibraryID: "lib", Title: "测试剧集", MediaType: "episode", FilePath: "/tmp/m1.mkv"}
	if err := db.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.scrapeTVByTMDbID(media, 7, ScrapeModeReplace); err != nil {
		t.Fatalf("scrapeTVByTMDbID: %v", err)
	}
	if media.TMDbID != 7 {
		t.Fatalf("TMDbID = %d, want 7", media.TMDbID)
	}
	rows, err := repos.MediaPerson.ListByMediaID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 cast rows, got %d: %+v", len(rows), rows)
	}
}

func TestScrapeMovieFetchesCredits(t *testing.T) {
	server := fakeTMDbCreditsServer()
	defer server.Close()
	svc, db, repos := newCreditsTestService(t)
	svc.client = server.Client()
	svc.cfg.Secrets.TMDbAPIProxy = server.URL

	media := &model.Media{ID: "m1", LibraryID: "lib", Title: "测试电影", MediaType: "movie", FilePath: "/tmp/m1.mkv"}
	if err := db.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.scrapeMovie(media, "测试电影", 2021, ScrapeModeReplace); err != nil {
		t.Fatalf("scrapeMovie: %v", err)
	}
	rows, err := repos.MediaPerson.ListByMediaID("m1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 cast rows, got %d: %+v", len(rows), rows)
	}
}

func TestTMDbProviderScrapeSeriesFetchesCredits(t *testing.T) {
	server := fakeTMDbCreditsServer()
	defer server.Close()
	svc, db, repos := newCreditsTestService(t)
	svc.client = server.Client()
	svc.cfg.Secrets.TMDbAPIProxy = server.URL

	series := &model.Series{ID: "s1", LibraryID: "lib", Title: "测试剧集", FolderPath: "/tmp/s1"}
	if err := db.Create(series).Error; err != nil {
		t.Fatal(err)
	}
	provider := NewTMDbProvider(svc)
	if err := provider.ScrapeSeries(series, "测试剧集", 2020, "primary"); err != nil {
		t.Fatalf("ScrapeSeries: %v", err)
	}
	if series.TMDbID != 7 {
		t.Fatalf("TMDbID = %d, want 7", series.TMDbID)
	}
	rows, err := repos.MediaPerson.ListBySeriesID("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 cast rows, got %d: %+v", len(rows), rows)
	}
}
