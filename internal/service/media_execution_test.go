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

func TestMediaExecutionOwnsSingleProcessLocalRuntime(t *testing.T) {
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
	runtime := execution.ExecutionRuntime()
	if runtime == nil {
		t.Fatal("media execution did not expose FFmpeg runtime")
	}
	if execution.ExecutionRuntime() != runtime {
		t.Fatal("media execution created more than one runtime")
	}
}
