package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
)

type agentMetadataFake struct {
	calls  atomic.Int32
	detail TMDbMovieDetail
	err    error
}

func (f *agentMetadataFake) FetchAgentMovieMetadata(context.Context, int) (*TMDbMovieDetail, error) {
	f.calls.Add(1)
	return &f.detail, f.err
}
func metadataFixture(t *testing.T) (*AgentScanService, *agentMetadataFake) {
	t.Helper()
	s, dir := agentScanFixture(t, nil)
	f := &agentMetadataFake{detail: TMDbMovieDetail{ID: 42, Title: "资料源片名", OriginalTitle: "Source original", Overview: "新的简介", ReleaseDate: "2020-02-02", Genres: []TMDbGenre{{ID: 1, Name: "剧情"}}}}
	s.metadata = f
	m := model.Media{ID: "movie", LibraryID: "movies", Title: "手工片名", MediaType: "movie", TMDbID: 42, FilePath: filepath.Join(dir, "original.mp4"), Rating: 9.1, Year: 2020, OrigTitle: "手工原名", ScrapeStatus: "manual", PosterPath: "custom-poster", Overview: "  "}
	if err := os.WriteFile(m.FilePath, []byte("original-source-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&m).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&model.ContentRating{MediaID: m.ID, Level: "R"}).Error; err != nil {
		t.Fatal(err)
	}
	return s, f
}
func TestAgentMetadataPreviewConfirmUndoPreservesExistingData(t *testing.T) {
	s, f := metadataFixture(t)
	ctx := context.Background()
	before, _, err := loadAgentMetadata(s.db, "movie")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := s.PreviewMetadata(ctx, "owner", "movie")
	if err != nil || preview.Status != "draft" || len(preview.Changes) != 3 || f.calls.Load() != 1 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
	var unchanged model.Media
	s.db.First(&unchanged, "id = ?", "movie")
	if unchanged.Overview != "  " {
		t.Fatal("preview wrote metadata")
	}
	if preview.MediaTitle != "手工片名" || preview.SourceTitle != "资料源片名" || !strings.HasSuffix(preview.SourceURL, "/42") {
		t.Fatal("missing accurate source and target")
	}
	const n = 8
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.Confirm(ctx, "owner", preview.ID, preview.Digest)
			if e == nil && r.Status != "completed" {
				e = errors.New("confirmation lacked receipt")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	s.db.First(&unchanged, "id = ?", "movie")
	if unchanged.Overview != "新的简介" || unchanged.Genres != "剧情" || unchanged.Premiered != "2020-02-02" || unchanged.OrigTitle != "手工原名" || unchanged.Title != "手工片名" || unchanged.Rating != 9.1 || unchanged.ScrapeStatus != "manual" || unchanged.PosterPath != "custom-poster" {
		t.Fatalf("unsafe patch %+v", unchanged)
	}
	if f.calls.Load() != 1 {
		t.Fatal("confirmation fetched provider again")
	}
	// Recreate service to prove the approved patch and receipt are server-owned.
	restarted := NewAgentScanService(s.db, nil)
	runs, e := restarted.List(ctx, "owner")
	if e != nil || len(runs) != 1 || runs[0].Status != "completed" {
		t.Fatal("receipt not durable", e)
	}
	for i := 0; i < 2; i++ {
		r, e := restarted.UndoMetadata(ctx, "owner", preview.ID, preview.Digest)
		if e != nil || r.Status != "reverted" {
			t.Fatal("undo", e)
		}
	}
	s.db.First(&unchanged, "id = ?", "movie")
	if unchanged.Overview != before.Overview || unchanged.Genres != "" || unchanged.Premiered != "" || unchanged.Title != before.Title {
		t.Fatal("undo did not restore exact blanks")
	}
	data, _ := os.ReadFile(unchanged.FilePath)
	if string(data) != "original-source-bytes" {
		t.Fatal("source modified")
	}
	var rating model.ContentRating
	s.db.First(&rating, "media_id = ?", "movie")
	if rating.Level != "R" {
		t.Fatal("child rating changed")
	}
}
func TestAgentMetadataRejectsOwnershipChangesExpiryAndUnsafeUndo(t *testing.T) {
	s, _ := metadataFixture(t)
	ctx := context.Background()
	for _, u := range []string{"viewer", "child"} {
		if _, e := s.PreviewMetadata(ctx, u, "movie"); !errors.Is(e, ErrForbidden) {
			t.Fatal("unauthorized preview")
		}
	}
	p, e := s.PreviewMetadata(ctx, "owner", "movie")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Confirm(ctx, "other-admin", p.ID, p.Digest); e == nil {
		t.Fatal("other owner confirmed")
	}
	if _, e = s.Confirm(ctx, "owner", p.ID, strings.Repeat("0", 64)); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("bad digest accepted")
	}
	s.db.Model(&model.Media{}).Where("id = ?", "movie").Update("overview", "另一管理员填写")
	if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("stale data overwritten", e)
	}
	p, e = s.PreviewMetadata(ctx, "owner", "movie")
	if e != nil {
		t.Fatal(e)
	}
	saved := s.now
	s.now = func() time.Time { return saved().Add(11 * time.Minute) }
	if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("expired approval accepted")
	}
	s.now = saved
	p, e = s.PreviewMetadata(ctx, "owner", "movie")
	if e != nil {
		t.Fatal(e)
	}
	s.Cancel(ctx, "owner", p.ID)
	r, e := s.Confirm(ctx, "owner", p.ID, p.Digest)
	if e != nil || r.Status != "cancelled" {
		t.Fatal("cancelled preview executed")
	}
	p, e = s.PreviewMetadata(ctx, "owner", "movie")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); e != nil {
		t.Fatal(e)
	}
	s.db.Model(&model.Media{}).Where("id = ?", "movie").Update("genres", "用户再次编辑")
	if _, e = s.UndoMetadata(ctx, "owner", p.ID, p.Digest); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("undo overwrote later edit")
	}
	var m model.Media
	s.db.First(&m, "id = ?", "movie")
	if m.Genres != "用户再次编辑" || m.Premiered != "2020-02-02" {
		t.Fatal("unsafe partial undo")
	}
}
func TestAgentMetadataRejectsPermissionAndLibraryChanges(t *testing.T) {
	for _, change := range []string{"token_version", "disabled", "must_change_pwd", "role", "library"} {
		t.Run(change, func(t *testing.T) {
			s, _ := metadataFixture(t)
			ctx := context.Background()
			p, e := s.PreviewMetadata(ctx, "owner", "movie")
			if e != nil {
				t.Fatal(e)
			}
			switch change {
			case "library":
				s.db.Model(&model.Library{}).Where("id = ?", "movies").Update("name", "新库名")
			case "role":
				s.db.Model(&model.User{}).Where("id = ?", "owner").Update("role", "user")
			case "token_version":
				s.db.Model(&model.User{}).Where("id = ?", "owner").Update("token_version", 2)
			default:
				s.db.Model(&model.User{}).Where("id = ?", "owner").Update(change, true)
			}
			if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); e == nil {
				t.Fatal("changed authorization or library accepted")
			}
			var m model.Media
			s.db.First(&m, "id = ?", "movie")
			if m.Overview != "  " {
				t.Fatal("unauthorized mutation")
			}
		})
	}
}
func TestAgentMetadataFailuresNeverWriteAndCandidateCommandsAreScoped(t *testing.T) {
	s, f := metadataFixture(t)
	ctx := context.Background()
	agent := NewViewerAgent(s.db, nil)
	for _, u := range []string{"viewer", "child"} {
		if _, e := agent.Chat(ctx, u, ViewerAgentRequest{Message: "补全资料"}); !errors.Is(e, ErrForbidden) {
			t.Fatal("non-admin candidates")
		}
	}
	out, e := agent.Chat(ctx, "owner", ViewerAgentRequest{Message: "补全资料"})
	if e != nil || !out.MetadataCandidates || len(out.Cards) != 1 {
		t.Fatalf("candidates %+v %v", out, e)
	}
	out, e = agent.Chat(ctx, "owner", ViewerAgentRequest{Message: "补全这部资料", CurrentMediaID: "movie"})
	if e != nil || out.Action == nil || out.Action.Kind != "prepare_metadata" {
		t.Fatal("current target", e)
	}
	out, e = agent.Chat(ctx, "owner", ViewerAgentRequest{Message: "补全第二部资料", Context: []AgentRef{{ID: "unmatched", Kind: "media"}, {ID: "movie", Kind: "media"}}})
	if e != nil || out.Action == nil || out.Action.Target.ID != "movie" {
		t.Fatal("ordinal selection", e)
	}
	out, e = agent.Chat(ctx, "owner", ViewerAgentRequest{Message: "补全所有资料"})
	if e != nil || out.Action != nil || len(out.Cards) != 0 {
		t.Fatal("unsupported bulk action", e)
	}
	f.err = errors.New("private provider credential must not escape")
	if _, e = s.PreviewMetadata(ctx, "owner", "movie"); !errors.Is(e, ErrAgentMetadataSource) {
		t.Fatal("source error leaked", e)
	}
	var count int64
	s.db.Model(&model.AgentRun{}).Count(&count)
	if count != 0 {
		t.Fatal("failed fetch created receipt")
	}
	f.err = nil
	f.detail.ID = 99
	if _, e = s.PreviewMetadata(ctx, "owner", "movie"); !errors.Is(e, ErrAgentMetadataSource) {
		t.Fatal("mismatched source accepted")
	}
	f.detail.ID = 42
	f.detail.Overview = ""
	f.detail.Genres = nil
	f.detail.ReleaseDate = "invalid"
	if _, e = s.PreviewMetadata(ctx, "owner", "movie"); !errors.Is(e, ErrAgentMetadataNoChanges) {
		t.Fatal("empty patch accepted", e)
	}
	s.db.Model(&model.Media{}).Where("id = ?", "movie").Update("tm_db_id", 0)
	if _, e = s.PreviewMetadata(ctx, "owner", "movie"); !errors.Is(e, ErrAgentMetadataUnsupported) {
		t.Fatal("unmatched movie accepted", e)
	}
	out, e = agent.Chat(ctx, "owner", ViewerAgentRequest{Message: "补全资料"})
	if e != nil || len(out.Cards) != 0 {
		t.Fatal("unmatched candidate included")
	}
}

type agentMetadataTransport func(*http.Request) (*http.Response, error)

func (f agentMetadataTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAgentMetadataSourceUsesBoundedReadOnlyProtocol(t *testing.T) {
	cfg := &config.Config{}
	cfg.Secrets.TMDbAPIKey = "test-not-a-real-key"
	cfg.Secrets.TMDbAPIProxy = "https://tmdb.example"
	calls := 0
	svc := &MetadataService{cfg: cfg, client: &http.Client{Transport: agentMetadataTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/3/movie/42" || r.URL.Query().Get("language") != "zh-CN" || r.URL.Query().Get("api_key") != cfg.Secrets.TMDbAPIKey {
			t.Fatal("wrong provider protocol")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"id":42,"title":"来源","overview":"简介"}`)), Header: make(http.Header)}, nil
	})}}
	detail, e := svc.FetchAgentMovieMetadata(context.Background(), 42)
	if e != nil || detail.ID != 42 || calls != 1 {
		t.Fatal("fetch", e)
	}
	svc.client.Transport = agentMetadataTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1))), Header: make(http.Header)}, nil
	})
	if _, e = svc.FetchAgentMovieMetadata(context.Background(), 42); !errors.Is(e, ErrAgentMetadataSource) {
		t.Fatal("oversize response accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.client.Transport = agentMetadataTransport(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	if _, e = svc.FetchAgentMovieMetadata(ctx, 42); !errors.Is(e, ErrAgentMetadataSource) {
		t.Fatal("cancel ignored")
	}
}

func TestAgentMetadataRestoresLegacyNullFields(t *testing.T) {
	s, _ := metadataFixture(t)
	if e := s.db.Model(&model.Media{}).Where("id = ?", "movie").Update("premiered", nil).Error; e != nil {
		t.Fatal(e)
	}
	ctx := context.Background()
	p, e := s.PreviewMetadata(ctx, "owner", "movie")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UndoMetadata(ctx, "owner", p.ID, p.Digest); e != nil {
		t.Fatal(e)
	}
	var count int64
	s.db.Model(&model.Media{}).Where("id = ? AND premiered IS NULL", "movie").Count(&count)
	if count != 1 {
		t.Fatal("legacy null was not restored")
	}
}
