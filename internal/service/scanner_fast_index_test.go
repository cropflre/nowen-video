package service

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupFastIndexTestEnv(t *testing.T, ffprobePath string) (*ScannerService, *repository.Repositories, *config.Config) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := gorm.Open(sqlite.Open(dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&model.Library{}, &model.Series{}, &model.Media{}); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	repos := repository.NewRepositories(db)
	cfg := &config.Config{
		App: config.AppConfig{
			FFprobePath: ffprobePath,
		},
	}
	scanner := NewScannerService(repos.Media, repos.Series, cfg, zap.NewNop().Sugar())
	t.Cleanup(func() {
		scanner.ShutdownAsyncEnrich()
	})
	return scanner, repos, cfg
}

func writeFakeFFprobe(t *testing.T, scriptBody string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake ffprobe requires POSIX shell")
	}
	path := filepath.Join(t.TempDir(), "fake-ffprobe.sh")
	content := "#!/bin/sh\n" + scriptBody + "\n"
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write fake ffprobe: %v", err)
	}
	return path
}

// TestFastIndexMovieLibraryStreaming2000 验证两阶段流式快扫：
// 即使 ffprobe 单次耗时 500ms，2000 部电影的第一阶段入库仍应在数秒内完成，且本地 NFO/海报/字幕准确命中。
func TestFastIndexMovieLibraryStreaming2000(t *testing.T) {
	slowFFprobe := writeFakeFFprobe(t, `sleep 0.5; echo '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac"}],"format":{"duration":"5400.0"}}'`)
	scanner, repos, _ := setupFastIndexTestEnv(t, slowFFprobe)

	mediaRoot := t.TempDir()
	const totalMovies = 2000
	for i := 0; i < totalMovies; i++ {
		dir := filepath.Join(mediaRoot, fmt.Sprintf("bucket-%02d", i/100))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		videoPath := filepath.Join(dir, fmt.Sprintf("Movie %04d (2024).mkv", i))
		if err := os.WriteFile(videoPath, []byte("mock"), 0o644); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			// 为第一部影片放入同名海报、外挂字幕和 NFO，验证目录快照缓存完整生效
			_ = os.WriteFile(filepath.Join(dir, "Movie 0000 (2024)-poster.jpg"), []byte("img"), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "Movie 0000 (2024).zh.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), 0o644)
			_ = os.WriteFile(filepath.Join(dir, "Movie 0000 (2024).nfo"), []byte(`<movie><title>测试高速入库影片</title><year>2024</year><rating>8.8</rating></movie>`), 0o644)
		}
	}

	lib := &model.Library{
		ID:   "lib-fast-2000",
		Name: "万级影库测试",
		Path: mediaRoot,
		Type: "movie",
	}
	if err := repos.Library.Create(lib); err != nil {
		t.Fatal(err)
	}
	lib.EnableFileFilter = false
	lib.MinFileSize = 0

	start := time.Now()
	count, err := scanner.ScanLibrary(lib)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("ScanLibrary failed: %v", err)
	}
	if count != totalMovies {
		t.Fatalf("expected %d movies indexed, got %d", totalMovies, count)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Phase 1 fast index took too long: %v (expected < 5s for %d items)", elapsed, totalMovies)
	}
	t.Logf("Phase 1 流式快扫 %d 部电影耗时: %v", totalMovies, elapsed)

	// 验证首部影片的本地 NFO、海报和外挂字幕均已通过目录快照缓存正确关联
	firstPath := filepath.Join(mediaRoot, "bucket-00", "Movie 0000 (2024).mkv")
	m, err := repos.Media.FindByFilePath(firstPath)
	if err != nil {
		t.Fatalf("FindByFilePath failed: %v", err)
	}
	if m.Title != "测试高速入库影片" {
		t.Errorf("expected NFO title %q, got %q", "测试高速入库影片", m.Title)
	}
	if m.PosterPath == "" {
		t.Errorf("expected local poster to be matched via scanDirCache")
	}
	if m.SubtitlePaths == "" {
		t.Errorf("expected external subtitle to be matched via scanDirCache")
	}
}

// TestFastIndexAsyncProbeAndStillImageCleanup 验证 Phase 2 后台异步探测回写技术信息，
// 并自动清理伪装成视频的单帧静态图片记录。
func TestFastIndexAsyncProbeAndStillImageCleanup(t *testing.T) {
	fakeFFprobe := writeFakeFFprobe(t, `
for arg in "$@"; do last="$arg"; done
case "$last" in
  *still_thumb*)
    echo '{"streams":[{"codec_type":"video","codec_name":"mjpeg","width":320,"height":180}],"format":{"duration":"0.04"}}'
    ;;
  *)
    echo '{"streams":[{"codec_type":"video","codec_name":"hevc","width":3840,"height":2160},{"codec_type":"audio","codec_name":"aac"}],"format":{"duration":"7200.5"}}'
    ;;
esac
`)
	scanner, repos, _ := setupFastIndexTestEnv(t, fakeFFprobe)

	mediaRoot := t.TempDir()
	normalPath := filepath.Join(mediaRoot, "Inception (2010).mkv")
	stillPath := filepath.Join(mediaRoot, "still_thumb.mp4")
	if err := os.WriteFile(normalPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stillPath, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}

	lib := &model.Library{
		ID:   "lib-async-probe",
		Name: "异步探测测试",
		Path: mediaRoot,
		Type: "movie",
	}
	if err := repos.Library.Create(lib); err != nil {
		t.Fatal(err)
	}
	lib.EnableFileFilter = false
	lib.MinFileSize = 0

	count, err := scanner.ScanLibrary(lib)
	if err != nil {
		t.Fatalf("ScanLibrary failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 initial fast-indexed items, got %d", count)
	}

	// 等待后台异步探测完成
	scanner.WaitAsyncEnrich()

	// 正常影片应已补全技术规格
	normalMedia, err := repos.Media.FindByFilePath(normalPath)
	if err != nil {
		t.Fatalf("normal movie missing after async enrich: %v", err)
	}
	if normalMedia.VideoCodec != "hevc" || normalMedia.AudioCodec != "aac" || normalMedia.Resolution != "4K" || normalMedia.Duration < 7200 {
		t.Fatalf("unexpected probed metadata: codec=%s audio=%s res=%s dur=%v",
			normalMedia.VideoCodec, normalMedia.AudioCodec, normalMedia.Resolution, normalMedia.Duration)
	}

	// 伪装静态图片应已被后台探测自动清理
	if _, err := repos.Media.FindByFilePath(stillPath); err == nil {
		t.Fatalf("expected disguised still-image media to be removed after async probe")
	}
}

// TestEnsureMediaProbedOnDemand 验证按需即探（用户抢在后台 Worker 前点击播放时立即单飞探测并落库）。
func TestEnsureMediaProbedOnDemand(t *testing.T) {
	fakeFFprobe := writeFakeFFprobe(t, `echo '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac"}],"format":{"duration":"3600.0"}}'`)
	scanner, repos, cfg := setupFastIndexTestEnv(t, fakeFFprobe)

	videoPath := filepath.Join(t.TempDir(), "OnDemand.mp4")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	media := &model.Media{
		LibraryID: "lib-ondemand",
		Title:     "OnDemand",
		FilePath:  videoPath,
		FileSize:  5,
		MediaType: "movie",
	}
	if err := repos.Media.Create(media); err != nil {
		t.Fatal(err)
	}

	streamSvc := NewStreamService(repos.Media, repos.Series, nil, cfg, zap.NewNop().Sugar())
	streamSvc.SetScanner(scanner)

	playInfo, err := streamSvc.GetMediaPlayInfo(media.ID)
	if err != nil {
		t.Fatalf("GetMediaPlayInfo failed: %v", err)
	}
	if playInfo.VideoCodec != "h264" || playInfo.AudioCodec != "aac" || playInfo.Duration != 3600 {
		t.Fatalf("expected on-demand probed PlayInfo, got %+v", playInfo)
	}
	if !playInfo.CanDirectPlay {
		t.Errorf("expected h264+aac mp4 to be direct playable after on-demand probe")
	}

	// 验证已持久化到数据库
	reloaded, err := repos.Media.FindByID(media.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.VideoCodec != "h264" || reloaded.Resolution != "1080p" || reloaded.Duration != 3600 {
		t.Fatalf("expected DB record updated by on-demand probe, got codec=%s res=%s dur=%v",
			reloaded.VideoCodec, reloaded.Resolution, reloaded.Duration)
	}
}
