package probe

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func TestProbeCachesSingleFlightAndInvalidatesSourceChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("uses a temporary executable")
	}
	tempDir := t.TempDir()
	mediaPath := filepath.Join(tempDir, "movie.mkv")
	if err := os.WriteFile(mediaPath, []byte("first-version"), 0o644); err != nil {
		t.Fatal(err)
	}
	countPath := filepath.Join(tempDir, "count.log")
	ffprobePath := filepath.Join(tempDir, "fake-ffprobe.sh")
	script := fmt.Sprintf(`#!/bin/sh
printf 'x\n' >> %q
sleep 0.1
cat <<'JSON'
{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","width":1920,"height":1080,"pix_fmt":"yuv420p10le","avg_frame_rate":"24000/1001","color_transfer":"bt709"}],"format":{"format_name":"matroska","duration":"60"}}
JSON
`, countPath)
	if err := os.WriteFile(ffprobePath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	dsn := fmt.Sprintf("file:probe-service-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, ffprobePath, nil)
	if err != nil {
		t.Fatal(err)
	}
	media := &model.Media{ID: "media-1", FilePath: mediaPath}

	const callers = 8
	var wg sync.WaitGroup
	errorsCh := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			record, probeErr := service.Probe(context.Background(), media)
			if probeErr != nil {
				errorsCh <- probeErr
				return
			}
			if record.FrameRateNum != 24000 || record.FrameRateDen != 1001 || record.HDR {
				errorsCh <- fmt.Errorf("unexpected probe record: %+v", record)
			}
		}()
	}
	wg.Wait()
	close(errorsCh)
	for probeErr := range errorsCh {
		t.Fatal(probeErr)
	}
	if got := countLines(t, countPath); got != 1 {
		t.Fatalf("single-flight executed ffprobe %d times", got)
	}
	if service.Stats().Executions != 1 {
		t.Fatalf("unexpected execution stats: %+v", service.Stats())
	}

	if _, err := service.Probe(context.Background(), media); err != nil {
		t.Fatal(err)
	}
	if got := countLines(t, countPath); got != 1 {
		t.Fatalf("fresh cache executed ffprobe again: %d", got)
	}
	if service.Stats().CacheHits == 0 {
		t.Fatalf("cache hit was not observed: %+v", service.Stats())
	}

	if err := os.WriteFile(mediaPath, []byte("second-version-with-different-size"), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(mediaPath, future, future); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Probe(context.Background(), media); err != nil {
		t.Fatal(err)
	}
	if got := countLines(t, countPath); got != 2 {
		t.Fatalf("changed source did not invalidate cache: %d", got)
	}
	if service.Stats().Executions != 2 {
		t.Fatalf("unexpected invalidation stats: %+v", service.Stats())
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, value := range data {
		if value == '\n' {
			count++
		}
	}
	return count
}
