package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
)

type agentMatchFake struct {
	calls   atomic.Int32
	results []TMDbMovie
	err     error
	query   string
	year    int
	after   func()
}

func (f *agentMatchFake) SearchAgentMovieMatches(ctx context.Context, query string, year int) ([]TMDbMovie, error) {
	f.calls.Add(1)
	f.query = query
	f.year = year
	if f.after != nil {
		f.after()
	}
	return f.results, f.err
}
func matchFixture(t *testing.T) (*AgentScanService, *agentMatchFake) {
	t.Helper()
	s, _ := metadataFixture(t)
	if e := s.db.Model(&model.Media{}).Where("id = ?", "movie").Updates(map[string]interface{}{"tm_db_id": nil, "year": 2020}).Error; e != nil {
		t.Fatal(e)
	}
	f := &agentMatchFake{results: []TMDbMovie{{ID: 42, Title: "手工片名", OriginalTitle: "Source original", ReleaseDate: "2020-02-02", Overview: "本次候选简介"}, {ID: 99, Title: "手工片名", ReleaseDate: "1990-01-01", Overview: "另一部同名影片"}}}
	s.matcher = f
	return s, f
}
func matchTMDbID(t *testing.T, s *AgentScanService) int {
	t.Helper()
	var m model.Media
	if e := s.db.First(&m, "id = ?", "movie").Error; e != nil {
		t.Fatal(e)
	}
	return m.TMDbID
}
func TestAgentMovieMatchRequiresSelectionConfirmationAndSafeUndoOrder(t *testing.T) {
	s, f := matchFixture(t)
	ctx := context.Background()
	p, e := s.PreviewMovieMatch(ctx, "owner", "movie", "", nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(p.MatchCandidates) != 2 || p.SelectedMatch != nil || p.MediaYear != 2020 || f.query != "手工片名" || f.year != 2020 || matchTMDbID(t, s) != 0 {
		t.Fatal("search mutated or auto selected")
	}
	if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); !errors.Is(e, ErrAgentMatchSelection) {
		t.Fatal("confirmed without candidate")
	}
	if _, e = s.SelectMovieMatch(ctx, "owner", p.ID, p.Digest, 12345); !errors.Is(e, ErrAgentMatchSelection) {
		t.Fatal("unreturned candidate accepted")
	}
	oldDigest := p.Digest
	p, e = s.SelectMovieMatch(ctx, "owner", p.ID, p.Digest, 42)
	if e != nil {
		t.Fatal(e)
	}
	if p.SelectedMatch == nil || p.SelectedMatch.ID != 42 || p.Digest == oldDigest || matchTMDbID(t, s) != 0 {
		t.Fatal("selection mutated or binding not rotated")
	}
	if _, e = s.Confirm(ctx, "owner", p.ID, oldDigest); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("old approval remained valid")
	}
	// Count actual identity transitions, not HTTP acknowledgements.
	if e = s.db.Exec("CREATE TABLE match_audit (id INTEGER)").Error; e != nil {
		t.Fatal(e)
	}
	if e = s.db.Exec("CREATE TRIGGER match_change AFTER UPDATE OF tm_db_id ON media WHEN COALESCE(OLD.tm_db_id,0) != COALESCE(NEW.tm_db_id,0) BEGIN INSERT INTO match_audit VALUES (1); END").Error; e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.Confirm(ctx, "owner", p.ID, p.Digest)
			if e == nil && r.Status != "completed" {
				e = errors.New("missing receipt")
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
	var transitions int64
	s.db.Table("match_audit").Count(&transitions)
	if transitions != 1 || f.calls.Load() != 1 || matchTMDbID(t, s) != 42 {
		t.Fatal("repeated matching or source fetch")
	}
	var m model.Media
	s.db.First(&m, "id = ?", "movie")
	if m.Title != "手工片名" || m.Rating != 9.1 || m.Overview != "  " || m.ScrapeStatus != "manual" {
		t.Fatal("match overwrote metadata")
	}
	fill, e := s.PreviewMetadata(ctx, "owner", "movie")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Confirm(ctx, "owner", fill.ID, fill.Digest); e != nil {
		t.Fatal(e)
	}
	if _, e = s.UndoMetadata(ctx, "owner", p.ID, p.Digest); !errors.Is(e, ErrAgentMatchUndo) {
		t.Fatal("association removed before dependent data undo")
	}
	if _, e = s.UndoMetadata(ctx, "owner", fill.ID, fill.Digest); e != nil {
		t.Fatal(e)
	}
	restarted := NewAgentScanService(s.db, nil)
	for i := 0; i < 2; i++ {
		r, e := restarted.UndoMetadata(ctx, "owner", p.ID, p.Digest)
		if e != nil || r.Status != "reverted" {
			t.Fatal("undo match", e)
		}
	}
	var nulls int64
	s.db.Model(&model.Media{}).Where("id = ? AND tm_db_id IS NULL", "movie").Count(&nulls)
	if nulls != 1 {
		t.Fatal("legacy null association not restored")
	}
	s.db.First(&m, "id = ?", "movie")
	data, _ := os.ReadFile(m.FilePath)
	if string(data) != "original-source-bytes" {
		t.Fatal("source changed")
	}
	var rating model.ContentRating
	s.db.First(&rating, "media_id = ?", "movie")
	if rating.Level != "R" {
		t.Fatal("rating changed")
	}
}
func TestAgentMovieMatchRequeryRevokesOldDraftAndEmptyResultsNeverConfirm(t *testing.T) {
	s, f := matchFixture(t)
	ctx := context.Background()
	first, e := s.PreviewMovieMatch(ctx, "owner", "movie", "first search", nil)
	if e != nil {
		t.Fatal(e)
	}
	zero := 0
	f.results = []TMDbMovie{}
	empty, e := s.PreviewMovieMatch(ctx, "owner", "movie", "different name", &zero)
	if e != nil {
		t.Fatal(e)
	}
	if f.query != "different name" || f.year != 0 || len(empty.MatchCandidates) != 0 || empty.SelectedMatch != nil {
		t.Fatal("refined search ignored")
	}
	old, e := s.Confirm(ctx, "owner", first.ID, first.Digest)
	if e != nil || old.Status != "cancelled" || matchTMDbID(t, s) != 0 {
		t.Fatal("old search not invalidated", e)
	}
	if _, e = s.SelectMovieMatch(ctx, "owner", empty.ID, empty.Digest, 42); !errors.Is(e, ErrAgentMatchSelection) {
		t.Fatal("empty results selected")
	}
	f.results = []TMDbMovie{{ID: 42, Title: "Actual", ReleaseDate: "2020-01-01"}}
	selected, e := s.PreviewMovieMatch(ctx, "owner", "movie", "Actual", nil)
	if e != nil {
		t.Fatal(e)
	}
	selected, e = s.SelectMovieMatch(ctx, "owner", selected.ID, selected.Digest, 42)
	if e != nil {
		t.Fatal(e)
	}
	beforeExpiry := selected.ExpiresAt
	selected, e = s.SelectMovieMatch(ctx, "owner", selected.ID, selected.Digest, 42)
	if e != nil || !selected.ExpiresAt.Equal(beforeExpiry) {
		t.Fatal("selection extended expiry")
	}
	saved := s.now
	s.now = func() time.Time { return saved().Add(11 * time.Minute) }
	if _, e = s.Confirm(ctx, "owner", selected.ID, selected.Digest); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("expired selection confirmed")
	}
	s.now = saved
}
func TestAgentMovieMatchOwnerPermissionsAndStaleSnapshots(t *testing.T) {
	s, f := matchFixture(t)
	ctx := context.Background()
	for _, uid := range []string{"viewer", "child"} {
		if _, e := s.PreviewMovieMatch(ctx, uid, "movie", "", nil); !errors.Is(e, ErrForbidden) {
			t.Fatal("unauthorized source request")
		}
	}
	if f.calls.Load() != 0 {
		t.Fatal("provider called before authorization")
	}
	p, e := s.PreviewMovieMatch(ctx, "owner", "movie", "", nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SelectMovieMatch(ctx, "other-admin", p.ID, p.Digest, 42); e == nil {
		t.Fatal("other admin selected")
	}
	p, e = s.SelectMovieMatch(ctx, "owner", p.ID, p.Digest, 42)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Confirm(ctx, "other-admin", p.ID, p.Digest); e == nil {
		t.Fatal("other admin confirmed")
	}
	if _, e = s.UndoMetadata(ctx, "other-admin", p.ID, p.Digest); e == nil {
		t.Fatal("other admin undid")
	}
	for _, change := range []string{"token_version", "disabled", "must_change_pwd", "child_mode", "role", "library", "title", "year", "tm_db_id", "overview"} {
		t.Run(change, func(t *testing.T) {
			s, _ := matchFixture(t)
			p, e := s.PreviewMovieMatch(ctx, "owner", "movie", "", nil)
			if e != nil {
				t.Fatal(e)
			}
			p, e = s.SelectMovieMatch(ctx, "owner", p.ID, p.Digest, 42)
			if e != nil {
				t.Fatal(e)
			}
			switch change {
			case "token_version":
				s.db.Model(&model.User{}).Where("id = ?", "owner").Update(change, 2)
			case "disabled", "must_change_pwd", "child_mode":
				s.db.Model(&model.User{}).Where("id = ?", "owner").Update(change, true)
			case "role":
				s.db.Model(&model.User{}).Where("id = ?", "owner").Update(change, "user")
			case "library":
				s.db.Model(&model.Library{}).Where("id = ?", "movies").Update("name", "changed")
			case "title", "overview":
				s.db.Model(&model.Media{}).Where("id = ?", "movie").Update(change, "another edit")
			case "year":
				s.db.Model(&model.Media{}).Where("id = ?", "movie").Update(change, 1999)
			case "tm_db_id":
				s.db.Model(&model.Media{}).Where("id = ?", "movie").Update(change, 77)
			}
			if _, e = s.Confirm(ctx, "owner", p.ID, p.Digest); e == nil {
				t.Fatal("stale approval accepted")
			}
			if matchTMDbID(t, s) == 42 {
				t.Fatal("stale matching persisted")
			}
		})
	}
}
func TestAgentMovieMatchFailuresCandidateBoundsAndReadOnlyChat(t *testing.T) {
	s, f := matchFixture(t)
	ctx := context.Background()
	badYear := 1700
	if _, e := s.PreviewMovieMatch(ctx, "owner", "movie", strings.Repeat("字", 121), nil); !errors.Is(e, ErrAgentMatchInput) {
		t.Fatal("oversize query accepted")
	}
	if _, e := s.PreviewMovieMatch(ctx, "owner", "movie", "a", &badYear); !errors.Is(e, ErrAgentMatchInput) {
		t.Fatal("bad year accepted")
	}
	f.err = errors.New("secret source error")
	if _, e := s.PreviewMovieMatch(ctx, "owner", "movie", "", nil); !errors.Is(e, ErrAgentMetadataSource) {
		t.Fatal("source error leaked")
	}
	var count int64
	s.db.Model(&model.AgentRun{}).Count(&count)
	if count != 0 {
		t.Fatal("failed fetch created approval")
	}
	f.err = nil
	f.after = func() { s.db.Model(&model.Media{}).Where("id = ?", "movie").Update("title", "changed during search") }
	if _, e := s.PreviewMovieMatch(ctx, "owner", "movie", "", nil); !errors.Is(e, ErrAgentPreviewChanged) {
		t.Fatal("stale provider result accepted")
	}
	f.after = nil
	results := []TMDbMovie{{ID: -1, Title: "bad"}, {ID: 42, Title: "same", Overview: strings.Repeat("a", 1000)}, {ID: 42, Title: "duplicate"}}
	for i := 0; i < 20; i++ {
		results = append(results, TMDbMovie{ID: i + 100, Title: fmt.Sprint(i), ReleaseDate: "bad"})
	}
	candidates := movieMatchCandidates(model.Media{Title: "same"}, results)
	if len(candidates) != 6 || len([]rune(candidates[0].Overview)) != 501 || len(candidates[0].Evidence) != 2 {
		t.Fatal("candidate bounds")
	}
	agent := NewViewerAgent(s.db, nil)
	for _, uid := range []string{"viewer", "child"} {
		if _, e := agent.Chat(ctx, uid, ViewerAgentRequest{Message: "匹配影片"}); !errors.Is(e, ErrForbidden) {
			t.Fatal("viewer got management candidates")
		}
	}
	for _, message := range []string{"匹配影片", "补全资料"} {
		out, e := agent.Chat(ctx, "owner", ViewerAgentRequest{Message: message})
		if e != nil || !out.MatchCandidates || len(out.Cards) != 1 || out.MetadataCandidates {
			t.Fatal("unmatched entry", message, e)
		}
	}
	for _, message := range []string{"匹配第一部", "补全第一部资料"} {
		out, e := agent.Chat(ctx, "owner", ViewerAgentRequest{Message: message, Context: []AgentRef{{ID: "movie", Kind: "media"}}})
		if e != nil || out.Action == nil || out.Action.Kind != "prepare_match" || out.Action.Target.ID != "movie" {
			t.Fatal("match action", message, e)
		}
	}
}
func TestAgentMovieMatchSourceProtocolAndCancellation(t *testing.T) {
	cfg := &config.Config{}
	cfg.Secrets.TMDbAPIKey = "test-not-real"
	cfg.Secrets.TMDbAPIProxy = "https://tmdb.example"
	svc := &MetadataService{cfg: cfg, client: &http.Client{Transport: agentMetadataTransport(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if r.Method != "GET" || r.URL.Path != "/3/search/movie" || q.Get("query") != "电影 & title" || q.Get("year") != "2020" || q.Get("language") != "zh-CN" || q.Get("include_adult") != "false" || q.Get("api_key") != cfg.Secrets.TMDbAPIKey {
			t.Fatal("search protocol")
		}
		if !reflect.DeepEqual(q["page"], []string{"1"}) {
			t.Fatal("unexpected pagination")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"results":[{"id":42,"title":"电影"}]}`)), Header: make(http.Header)}, nil
	})}}
	results, e := svc.SearchAgentMovieMatches(context.Background(), "电影 & title", 2020)
	if e != nil || len(results) != 1 || results[0].ID != 42 {
		t.Fatal("search", e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc.client.Transport = agentMetadataTransport(func(r *http.Request) (*http.Response, error) { return nil, r.Context().Err() })
	if _, e = svc.SearchAgentMovieMatches(ctx, "x", 0); !errors.Is(e, ErrAgentMetadataSource) {
		t.Fatal("context ignored")
	}
}
