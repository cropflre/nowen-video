package service

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestMediaExecutionCompatibilityAdapterHasNoPersistentRuntimeState(t *testing.T) {
	dsn := fmt.Sprintf("file:media-execution-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.App.FFprobePath = "ffprobe"
	cfg.App.FFmpegPath = "ffmpeg"
	cfg.Cache.CacheDir = filepath.Join(t.TempDir(), "cache")

	execution, err := NewMediaExecutionService(db, cfg, zap.NewNop().Sugar())
	if err != nil {
		t.Fatal(err)
	}
	adapter := execution.playbackCompatibilityAdapter()
	if adapter == nil || adapter.ExecutionRuntime() == nil {
		t.Fatal("media execution adapter did not expose FFmpeg runtime")
	}
	if adapter.ExecutionRuntime() != execution.ExecutionRuntime() {
		t.Fatal("playback adapter created a second execution runtime")
	}
	if adapter.repo != nil || adapter.executionRepo != nil || adapter.jobs != nil || adapter.artifactStore != nil {
		t.Fatalf("playback adapter reached persistent runtime state: %+v", adapter)
	}
	if adapter.running != nil || adapter.workerCount != 0 {
		t.Fatal("playback adapter started a persistent worker registry")
	}
}
