package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func newMediaProbeTestRepo(t *testing.T) *MediaProbeRepo {
	t.Helper()
	dsn := fmt.Sprintf("file:media-probe-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewMediaProbeRepo(db)
	if err := repo.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestMediaProbeRepoFreshnessAndUpsert(t *testing.T) {
	repo := newMediaProbeTestRepo(t)
	record := &model.MediaProbeRecord{
		MediaID:           "media-1",
		SourceFingerprint: "fingerprint-a",
		SourcePath:        "/media/movie.mkv",
		ProbeVersion:      model.MediaProbeVersion,
		VideoCodec:        "hevc",
		FrameRateNum:      24000,
		FrameRateDen:      1001,
		ProbedAt:          time.Now(),
	}
	if err := repo.Upsert(record); err != nil {
		t.Fatal(err)
	}
	fresh, err := repo.FindFresh("media-1", "fingerprint-a", model.MediaProbeVersion)
	if err != nil || fresh.VideoCodec != "hevc" {
		t.Fatalf("fresh probe missing: record=%+v err=%v", fresh, err)
	}
	if _, err := repo.FindFresh("media-1", "fingerprint-b", model.MediaProbeVersion); !IsNotFound(err) {
		t.Fatalf("stale fingerprint was accepted: %v", err)
	}

	record.SourceFingerprint = "fingerprint-b"
	record.VideoCodec = "h264"
	record.HDR = false
	if err := repo.Upsert(record); err != nil {
		t.Fatal(err)
	}
	updated, err := repo.FindFresh("media-1", "fingerprint-b", model.MediaProbeVersion)
	if err != nil || updated.VideoCodec != "h264" {
		t.Fatalf("upsert did not replace technical metadata: record=%+v err=%v", updated, err)
	}
	if _, err := repo.FindFresh("media-1", "fingerprint-a", model.MediaProbeVersion); !IsNotFound(err) {
		t.Fatalf("old fingerprint remained fresh: %v", err)
	}
}
