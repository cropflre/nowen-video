package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupNFOCastTest(t *testing.T) (*MetadataService, *repository.Repositories) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "nfo_cast.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Library{}, &model.Series{}, &model.Media{}, &model.Person{}, &model.MediaPerson{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	repos := repository.NewRepositories(db)
	logger := zap.NewNop().Sugar()
	meta := NewMetadataService(repos.Media, repos.Series, repos.Person, repos.MediaPerson, &config.Config{}, logger)
	meta.SetNFOService(NewNFOService(logger))
	return meta, repos
}

// TestImportCastFromNFO_Issue94EmbyMovieFormat 验证 Issue #94 中的 Emby/Jellyfin 标准电影 NFO：
// 支持 <actor> 内的 <name>/<role>/<type>/<tmdbid> 解析，并正确映射导演/编剧/演员角色与 TMDbID。
func TestImportCastFromNFO_Issue94EmbyMovieFormat(t *testing.T) {
	meta, repos := setupNFOCastTest(t)

	dir := t.TempDir()
	videoPath := filepath.Join(dir, "The Hobbit (2013).mkv")
	nfoPath := filepath.Join(dir, "The Hobbit (2013).nfo")
	if err := os.WriteFile(videoPath, []byte("mock"), 0o644); err != nil {
		t.Fatal(err)
	}
	nfoXML := `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<movie>
  <title>霍比特人2：史矛革之战</title>
  <originaltitle>The Hobbit: The Desolation of Smaug</originaltitle>
  <actor>
    <name>伊恩·麦克莱恩</name>
    <role>Gandalf the Grey</role>
    <type>Actor</type>
    <tmdbid>1327</tmdbid>
  </actor>
  <actor>
    <name>马丁·弗瑞曼</name>
    <role>Bilbo Baggins</role>
    <type>Actor</type>
    <tmdbid>7060</tmdbid>
  </actor>
  <actor>
    <name>彼得·杰克逊</name>
    <role>Director</role>
    <type>Director</type>
    <tmdbid>108</tmdbid>
  </actor>
  <actor>
    <name>弗兰·威尔士</name>
    <type>Writer</type>
    <tmdbid>126</tmdbid>
  </actor>
</movie>`
	if err := os.WriteFile(nfoPath, []byte(nfoXML), 0o644); err != nil {
		t.Fatal(err)
	}

	movie := &model.Media{
		LibraryID: "lib-1",
		Title:     "霍比特人2：史矛革之战",
		FilePath:  videoPath,
		MediaType: "movie",
	}
	if err := repos.Media.Create(movie); err != nil {
		t.Fatal(err)
	}

	imported := meta.ImportCastFromNFO(movie)
	if imported != 4 {
		t.Fatalf("expected 4 cast members imported, got %d", imported)
	}

	rows, err := repos.MediaPerson.ListByMediaID(movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 {
		t.Fatalf("expected 4 media_person rows, got %d", len(rows))
	}

	byName := make(map[string]model.MediaPerson, len(rows))
	for _, row := range rows {
		if row.Person.Name != "" {
			byName[row.Person.Name] = row
		}
	}

	gandalf, ok := byName["伊恩·麦克莱恩"]
	if !ok || gandalf.Role != "actor" || gandalf.Character != "Gandalf the Grey" || gandalf.Person.TMDbID != 1327 {
		t.Fatalf("unexpected actor record for 伊恩·麦克莱恩: %+v", gandalf)
	}
	director, ok := byName["彼得·杰克逊"]
	if !ok || director.Role != "director" || director.Person.TMDbID != 108 {
		t.Fatalf("unexpected director record for 彼得·杰克逊: %+v", director)
	}
	writer, ok := byName["弗兰·威尔士"]
	if !ok || writer.Role != "writer" || writer.Person.TMDbID != 126 {
		t.Fatalf("unexpected writer record for 弗兰·威尔士: %+v", writer)
	}

	// 幂等性验证：已有演职人员记录时不重复导入
	if again := meta.ImportCastFromNFO(movie); again != 0 {
		t.Fatalf("expected idempotent 0 on second import, got %d", again)
	}
}

// TestBackfillCastFromNFO_IncludesSeriesTVShowNFO 验证启动回填同时覆盖电影 .nfo 与剧集 tvshow.nfo。
func TestBackfillCastFromNFO_IncludesSeriesTVShowNFO(t *testing.T) {
	meta, repos := setupNFOCastTest(t)

	seriesDir := t.TempDir()
	tvNFO := `<?xml version="1.0" encoding="utf-8"?>
<tvshow>
  <title>绝命毒师</title>
  <director>文斯·吉里根</director>
  <actor>
    <name>布莱恩·科兰斯顿</name>
    <role>Walter White</role>
    <type>Actor</type>
    <tmdbid>17419</tmdbid>
  </actor>
</tvshow>`
	if err := os.WriteFile(filepath.Join(seriesDir, "tvshow.nfo"), []byte(tvNFO), 0o644); err != nil {
		t.Fatal(err)
	}

	series := &model.Series{
		LibraryID:    "lib-tv",
		Title:        "绝命毒师",
		FolderPath:   seriesDir,
		EpisodeCount: 1,
	}
	if err := repos.Series.Create(series); err != nil {
		t.Fatal(err)
	}

	processed := meta.BackfillCastFromNFO()
	if processed != 1 {
		t.Fatalf("expected 1 series backfilled, got %d", processed)
	}

	rows, err := repos.MediaPerson.ListBySeriesID(series.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 cast rows for series, got %d", len(rows))
	}

	foundActor := false
	for _, r := range rows {
		if r.Person.Name == "布莱恩·科兰斯顿" && r.Role == "actor" && r.Character == "Walter White" && r.Person.TMDbID == 17419 {
			foundActor = true
		}
	}
	if !foundActor {
		t.Fatalf("expected actor 布莱恩·科兰斯顿 with tmdbid=17419 in series cast, got %+v", rows)
	}
}
