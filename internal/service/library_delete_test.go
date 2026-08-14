package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

func TestLibraryDeleteClearsDerivedCachesAndKeepsOtherLibraryData(t *testing.T) {
	db := setupTestDB(t)
	if err := db.AutoMigrate(&model.MediaProbeRecord{}); err != nil {
		t.Fatalf("迁移媒体探测缓存失败: %v", err)
	}
	repos := repository.NewRepositories(db)
	logger := zap.NewNop().Sugar()

	deletedLibrary := model.Library{ID: "library-delete", Name: "待删除", Path: "/media/delete", Type: "movie"}
	keptLibrary := model.Library{ID: "library-keep", Name: "保留", Path: "/media/keep", Type: "movie"}
	if err := db.Create(&deletedLibrary).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&keptLibrary).Error; err != nil {
		t.Fatal(err)
	}

	deletedCollection := model.MovieCollection{ID: "collection-delete", Name: "待清空合集", MediaCount: 1, FileCount: 1}
	keptCollection := model.MovieCollection{ID: "collection-keep", Name: "保留合集", MediaCount: 1, FileCount: 1}
	if err := db.Create(&deletedCollection).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&keptCollection).Error; err != nil {
		t.Fatal(err)
	}

	deletedMedia := model.Media{ID: "media-delete", LibraryID: deletedLibrary.ID, Title: "待删除影片", FilePath: "/media/delete/a.mp4", MediaType: "movie", CollectionID: deletedCollection.ID}
	keptMedia := model.Media{ID: "media-keep", LibraryID: keptLibrary.ID, Title: "保留影片", FilePath: "/media/keep/b.mp4", MediaType: "movie", CollectionID: keptCollection.ID, Rating: 9}
	if err := db.Create(&deletedMedia).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&keptMedia).Error; err != nil {
		t.Fatal(err)
	}

	for _, mediaID := range []string{deletedMedia.ID, keptMedia.ID} {
		if err := db.Create(&model.PlaybackStats{UserID: "user", MediaID: mediaID, WatchMinutes: 5, Date: "2026-08-13"}).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Create(&model.MediaProbeRecord{
			MediaID:           mediaID,
			SourceFingerprint: "fingerprint-" + mediaID,
			SourcePath:        "/" + mediaID + ".mp4",
			ProbeVersion:      model.MediaProbeVersion,
			ProbedAt:          time.Now(),
		}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := repos.RecommendCache.Set("user", `[{"media":{"id":"media-delete"}}]`, 60); err != nil {
		t.Fatal(err)
	}

	svc := NewLibraryService(
		repos.Library,
		repos.Media,
		repos.Series,
		repos.Favorite,
		repos.WatchHistory,
		repos.MediaPerson,
		repos.ScanClassification,
		repos.RecommendCache,
		repos.PlaybackStats,
		repos.MediaProbe,
		repos.MovieCollection,
		nil,
		nil,
		nil,
		logger,
	)
	if err := svc.Delete(deletedLibrary.ID); err != nil {
		t.Fatalf("删除媒体库失败: %v", err)
	}

	assertCount := func(table, where string, args []any, want int64) {
		t.Helper()
		var got int64
		query := db.Table(table)
		if where != "" {
			query = query.Where(where, args...)
		}
		if err := query.Count(&got).Error; err != nil {
			t.Fatalf("统计 %s 失败: %v", table, err)
		}
		if got != want {
			t.Fatalf("%s 数量=%d，期望=%d", table, got, want)
		}
	}

	assertCount("recommend_caches", "", nil, 0)
	assertCount("media_probe_cache", "media_id = ?", []any{deletedMedia.ID}, 0)
	assertCount("media_probe_cache", "media_id = ?", []any{keptMedia.ID}, 1)
	assertCount("playback_stats", "media_id = ?", []any{deletedMedia.ID}, 0)
	assertCount("playback_stats", "media_id = ?", []any{keptMedia.ID}, 1)
	assertCount("media", "id = ?", []any{deletedMedia.ID}, 0)
	assertCount("media", "id = ?", []any{keptMedia.ID}, 1)
	assertCount("movie_collections", "id = ?", []any{deletedCollection.ID}, 0)
	assertCount("movie_collections", "id = ?", []any{keptCollection.ID}, 1)
}
