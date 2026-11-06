package handler

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service"
)

type httpAgentMatch struct {
	httpAgentMetadata
	searches atomic.Int32
}

func (f *httpAgentMatch) SearchAgentMovieMatches(context.Context, string, int) ([]service.TMDbMovie, error) {
	f.searches.Add(1)
	return []service.TMDbMovie{{ID: 42, Title: "safe", ReleaseDate: "2020-02-02", Overview: "same movie"}, {ID: 99, Title: "safe", ReleaseDate: "1990-01-01"}}, nil
}
func TestAgentMovieMatchHTTPSeparatesSearchSelectConfirmAndFill(t *testing.T) {
	db, repos, _ := childFixture(t)
	fake := &httpAgentMatch{}
	r := gin.New()
	api := r.Group("/api", middleware.JWTAuth(childTestSecret), ChildAccessGate(db))
	RegisterViewerAgent(api, db, nil, fake)
	parent, _ := repos.User.FindByID("parent")
	child, _ := repos.User.FindByID("child")
	token := childToken(t, parent)
	db.Create(&model.User{ID: "match-other", Username: "match-other", Role: "admin", Password: "hash"})
	other, _ := repos.User.FindByID("match-other")
	base := "/api/admin/agent/runs"
	for _, rq := range []struct{ path, body string }{{base + "/matches", `{"media_id":"safe"}`}, {base + "/unknown/select", `{"candidate_id":42,"approval_digest":"` + strings.Repeat("0", 64) + `"}`}} {
		for _, tok := range []string{"", childToken(t, child)} {
			w := performChildRequest(t, r, "POST", rq.path, tok, rq.body)
			if w.Code != 401 && w.Code != 403 {
				t.Fatal("unprivileged matching", w.Code)
			}
		}
	}
	if fake.searches.Load() != 0 {
		t.Fatal("unauthorized provider call")
	}
	w := performChildRequest(t, r, "POST", base+"/matches", token, `{"media_id":"safe","query":"safe","year":2020}`)
	if w.Code != 200 {
		t.Fatal("search", w.Code, w.Body)
	}
	decode := func(raw []byte) service.AgentRunView {
		t.Helper()
		var v struct {
			Data service.AgentRunView `json:"data"`
		}
		if e := json.Unmarshal(raw, &v); e != nil {
			t.Fatal(e)
		}
		return v.Data
	}
	p := decode(w.Body.Bytes())
	if p.Kind != "match_movie" || p.SelectedMatch != nil || len(p.MatchCandidates) != 2 {
		t.Fatal("candidate view")
	}
	body := `{"approval_digest":"` + p.Digest + `"}`
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/confirm", token, body); w.Code != 400 {
		t.Fatal("confirmed without selection")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/select", childToken(t, other), `{"candidate_id":42,"approval_digest":"`+p.Digest+`"}`); w.Code != 404 {
		t.Fatal("foreign owner selected")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/select", token, `{"candidate_id":12345,"approval_digest":"`+p.Digest+`"}`); w.Code != 400 {
		t.Fatal("forged candidate accepted")
	}
	w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/select", token, `{"candidate_id":42,"approval_digest":"`+p.Digest+`"}`)
	if w.Code != 200 {
		t.Fatal("selection", w.Code, w.Body)
	}
	p = decode(w.Body.Bytes())
	if p.SelectedMatch == nil || p.SelectedMatch.ID != 42 {
		t.Fatal("missing selection")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/confirm", token, body); w.Code != 409 {
		t.Fatal("old approval accepted")
	}
	body = `{"approval_digest":"` + p.Digest + `"}`
	for i := 0; i < 2; i++ {
		w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/confirm", token, body)
		if w.Code != 200 || decode(w.Body.Bytes()).Status != "completed" {
			t.Fatal("confirm", w.Code, w.Body)
		}
	}
	var m model.Media
	db.First(&m, "id = ?", "safe")
	if m.TMDbID != 42 || m.Title != "safe" || m.Overview != "" || fake.searches.Load() != 1 || fake.fetches.Load() != 0 {
		t.Fatal("matching changed fields or refetched")
	}
	w = performChildRequest(t, r, "POST", base+"/metadata", token, `{"media_id":"safe"}`)
	if w.Code != 200 {
		t.Fatal("fill preview", w.Code, w.Body)
	}
	fill := decode(w.Body.Bytes())
	w = performChildRequest(t, r, "POST", base+"/"+fill.ID+"/confirm", token, `{"approval_digest":"`+fill.Digest+`"}`)
	if w.Code != 200 {
		t.Fatal("fill confirm")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/undo", token, body); w.Code != 409 {
		t.Fatal("dependent fill lost identity")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+fill.ID+"/undo", token, `{"approval_digest":"`+fill.Digest+`"}`); w.Code != 200 {
		t.Fatal("undo fill")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/undo", token, body); w.Code != 200 || decode(w.Body.Bytes()).Status != "reverted" {
		t.Fatal("undo match", w.Code, w.Body)
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("cacheable preview")
	}
}
