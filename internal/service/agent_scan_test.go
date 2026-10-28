package service

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

type controlledAgentScan struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
	err     error
}

func (f *controlledAgentScan) ScanNewMovies(*model.Library) (int, error) {
	f.calls.Add(1)
	if f.started != nil {
		close(f.started)
	}
	if f.release != nil {
		<-f.release
	}
	return 2, f.err
}
func agentScanFixture(t *testing.T, executor AgentScanExecutor) (*AgentScanService, string) {
	t.Helper()
	db := setupTestDB(t)
	sql, _ := db.DB()
	sql.SetMaxOpenConns(1)
	t.Cleanup(func() { sql.Close() })
	for _, u := range []model.User{{ID: "owner", Username: "owner", Role: "admin", Password: "hash"}, {ID: "other-admin", Username: "other-admin", Role: "admin", Password: "hash"}, {ID: "viewer", Username: "viewer", Role: "user", Password: "hash"}, {ID: "child", Username: "child", Role: "user", ChildMode: true, Password: "hash"}} {
		u := u
		if err := db.Create(&u).Error; err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	lib := model.Library{ID: "movies", Name: "测试电影", Path: dir, Type: "movie"}
	if err := db.Create(&lib).Error; err != nil {
		t.Fatal(err)
	}
	return NewAgentScanService(db, executor), dir
}
func waitAgentRun(t *testing.T, s *AgentScanService, id, status string) AgentRunView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		runs, err := s.List(context.Background(), "owner")
		if err != nil {
			t.Fatal(err)
		}
		for _, run := range runs {
			if run.ID == id && run.Status == status {
				return run
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run did not reach %s", status)
	return AgentRunView{}
}
func TestAgentScanPreviewOwnershipSnapshotAndOneTimeConfirmation(t *testing.T) {
	f := &controlledAgentScan{started: make(chan struct{}), release: make(chan struct{})}
	s, _ := agentScanFixture(t, f)
	ctx := context.Background()
	for _, uid := range []string{"viewer", "child"} {
		if _, err := s.Preview(ctx, uid, "movies"); !errors.Is(err, ErrForbidden) {
			t.Fatal("non-admin created preview")
		}
	}
	preview, err := s.Preview(ctx, "owner", "movies")
	if err != nil || preview.Status != "draft" || f.calls.Load() != 0 {
		t.Fatalf("preview executed work: %+v %v", preview, err)
	}
	if _, err = s.Confirm(ctx, "other-admin", preview.ID, preview.Digest); err == nil {
		t.Fatal("another admin confirmed owner preview")
	}
	if _, err = s.Confirm(ctx, "owner", preview.ID, "wrong"); !errors.Is(err, ErrAgentPreviewChanged) {
		t.Fatal("wrong digest accepted")
	}
	if err = s.db.Model(&model.Library{}).Where("id = ?", "movies").Update("min_file_size", 10).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.Confirm(ctx, "owner", preview.ID, preview.Digest); !errors.Is(err, ErrAgentPreviewChanged) {
		t.Fatal("changed library executed stale preview")
	}
	preview, err = s.Preview(ctx, "owner", "movies")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Confirm(ctx, "owner", preview.ID, preview.Digest); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	<-f.started
	if f.calls.Load() != 1 {
		t.Fatalf("executions=%d", f.calls.Load())
	}
	other, err := s.Preview(ctx, "other-admin", "movies")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Confirm(ctx, "other-admin", other.ID, other.Digest); !errors.Is(err, ErrAgentRunBusy) {
		t.Fatalf("parallel job accepted: %v", err)
	}
	close(f.release)
	receipt := waitAgentRun(t, s, preview.ID, "completed")
	if receipt.NewCount != 2 {
		t.Fatalf("missing actual result: %+v", receipt)
	}
	if _, err = s.Confirm(ctx, "owner", preview.ID, preview.Digest); err != nil || f.calls.Load() != 1 {
		t.Fatal("replay restarted completed task")
	}
	restarted := NewAgentScanService(s.db, f)
	runs, err := restarted.List(ctx, "owner")
	if err != nil || len(runs) != 2 {
		t.Fatalf("receipts lost on service recreation: %+v %v", runs, err)
	}
	if runs, err = restarted.List(ctx, "other-admin"); err != nil || len(runs) != 1 {
		t.Fatal("task history crossed owners")
	}
}
func TestAgentScanExpiryCancellationAndLostWorkerDoNotAutoRun(t *testing.T) {
	f := &controlledAgentScan{started: make(chan struct{}), release: make(chan struct{})}
	s, _ := agentScanFixture(t, f)
	ctx := context.Background()
	var nanos atomic.Int64
	nanos.Store(time.Now().UnixNano())
	s.now = func() time.Time { return time.Unix(0, nanos.Load()) }
	p, err := s.Preview(ctx, "owner", "movies")
	if err != nil {
		t.Fatal(err)
	}
	nanos.Add(int64(11 * time.Minute))
	if _, err = s.Confirm(ctx, "owner", p.ID, p.Digest); !errors.Is(err, ErrAgentPreviewChanged) {
		t.Fatal("expired preview accepted")
	}
	p, err = s.Preview(ctx, "owner", "movies")
	if err != nil {
		t.Fatal(err)
	}
	if run, err := s.Cancel(ctx, "owner", p.ID); err != nil || run.Status != "cancelled" {
		t.Fatal("cancel failed")
	}
	if run, err := s.Confirm(ctx, "owner", p.ID, p.Digest); err != nil || run.Status != "cancelled" || f.calls.Load() != 0 {
		t.Fatal("cancelled preview executed")
	}
	p, err = s.Preview(ctx, "owner", "movies")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Confirm(ctx, "owner", p.ID, p.Digest); err != nil {
		t.Fatal(err)
	}
	<-f.started
	nanos.Add(int64(3 * time.Minute))
	restarted := NewAgentScanService(s.db, f)
	restarted.now = s.now
	receipt := waitAgentRun(t, restarted, p.ID, "interrupted")
	if receipt.ActiveLibraryID != nil || f.calls.Load() != 1 {
		t.Fatal("lost worker auto-replayed or lock persisted")
	}
	close(f.release)
	time.Sleep(20 * time.Millisecond)
	receipt = waitAgentRun(t, restarted, p.ID, "interrupted")
	if receipt.Status != "interrupted" {
		t.Fatal("late worker overwrote interruption")
	}
}
func TestAgentScanRevokedPermissionAndExecutorFailure(t *testing.T) {
	f := &controlledAgentScan{err: errors.New("unreachable folder")}
	s, _ := agentScanFixture(t, f)
	ctx := context.Background()
	p, err := s.Preview(ctx, "owner", "movies")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.db.Model(&model.User{}).Where("id = ?", "owner").Update("token_version", 1).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = s.Confirm(ctx, "owner", p.ID, p.Digest); !errors.Is(err, ErrAgentPreviewChanged) {
		t.Fatal("revoked preview accepted")
	}
	p, err = s.Preview(ctx, "owner", "movies")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Confirm(ctx, "owner", p.ID, p.Digest); err != nil {
		t.Fatal(err)
	}
	run := waitAgentRun(t, s, p.ID, "failed")
	if run.NewCount != 2 || run.ActiveLibraryID != nil {
		t.Fatal("partial failure lost real count or lock")
	}
	for _, change := range []struct {
		field        string
		value, reset interface{}
	}{{"disabled", true, false}, {"must_change_pwd", true, false}, {"role", "user", "admin"}} {
		preview, err := s.Preview(ctx, "owner", "movies")
		if err != nil {
			t.Fatal(err)
		}
		if err = s.db.Model(&model.User{}).Where("id = ?", "owner").Update(change.field, change.value).Error; err != nil {
			t.Fatal(err)
		}
		if _, err = s.Confirm(ctx, "owner", preview.ID, preview.Digest); !errors.Is(err, ErrForbidden) {
			t.Fatalf("revoked actor %s confirmed work: %v", change.field, err)
		}
		if _, err = s.List(ctx, "owner"); !errors.Is(err, ErrForbidden) {
			t.Fatalf("revoked actor %s read receipts: %v", change.field, err)
		}
		if err = s.db.Model(&model.User{}).Where("id = ?", "owner").Update(change.field, change.reset).Error; err != nil {
			t.Fatal(err)
		}
	}
	if f.calls.Load() != 1 {
		t.Fatal("permission rejection executed work")
	}
}

func TestAgentAddOnlyScannerPreservesExistingAndMissingMedia(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required for real scanner verification")
	}
	s, dir := agentScanFixture(t, nil)
	db := s.db
	repos := repository.NewRepositories(db)
	nested := filepath.Join(dir, "new")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(nested, "New Movie (2026).mp4")
	cmd := exec.Command("ffmpeg", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=navy:s=64x64:r=1", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", file)
	if err := cmd.Run(); err != nil {
		t.Fatal("cannot generate scanner sample")
	}
	existingFile := filepath.Join(dir, "Existing.mp4")
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(existingFile, raw, 0600); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(dir, "missing.mp4")
	for _, m := range []model.Media{{ID: "existing", LibraryID: "movies", Title: "原有标题", FilePath: existingFile, MediaType: "movie", FileSize: 123, Rating: 9}, {ID: "missing", LibraryID: "movies", Title: "离线影片", FilePath: missing, MediaType: "movie"}} {
		m := m
		if err = db.Create(&m).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Model(&model.Library{}).Where("id = ?", "movies").Updates(map[string]interface{}{"enable_file_filter": false, "min_file_size": 0}).Error; err != nil {
		t.Fatal(err)
	}
	lib, err := repos.Library.FindByID("movies")
	if err != nil {
		t.Fatal(err)
	}
	lib.SetAllPaths([]string{dir, nested}) // An overlapping root must not index twice.
	cfg := &config.Config{}
	cfg.App.FFprobePath = "ffprobe"
	scanner := NewScannerService(repos.Media, repos.Series, cfg, zap.NewNop().Sugar())
	hub := NewWSHub(zap.NewNop().Sugar())
	scanner.SetWSHub(hub)
	callbacks := make(chan string, 1)
	scanner.SetOnScanComplete(func(id string) { callbacks <- id })
	count, err := scanner.ScanLibraryWithOptions(lib, ScanLibraryOptions{AddOnly: true})
	if err != nil || count != 1 {
		t.Fatalf("scan result %d %v", count, err)
	}
	if len(hub.broadcast) != 0 {
		t.Fatal("owner-only scan titles or paths broadcast globally")
	}
	select {
	case <-callbacks:
		t.Fatal("add-only scan started post-processing callback")
	case <-time.After(20 * time.Millisecond):
	}
	old, err := repos.Media.FindByID("existing")
	if err != nil || old.Title != "原有标题" || old.FileSize != 123 || old.Rating != 9 {
		t.Fatal("existing metadata modified")
	}
	if _, err = repos.Media.FindByID("missing"); err != nil {
		t.Fatal("offline record removed by add-only scan")
	}
	if count, err = scanner.ScanLibraryWithOptions(lib, ScanLibraryOptions{AddOnly: true, SuppressCompletionCallback: true}); err != nil || count != 0 {
		t.Fatalf("repeat scan duplicated media %d %v", count, err)
	}
	after, err := os.ReadFile(file)
	if err != nil || string(raw) != string(after) {
		t.Fatal("source file changed")
	}
}
