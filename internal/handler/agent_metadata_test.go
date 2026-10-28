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

type httpAgentMetadata struct {
	httpAgentScanExecutor
	fetches atomic.Int32
}

func (f *httpAgentMetadata) FetchAgentMovieMetadata(context.Context, int) (*service.TMDbMovieDetail, error) {
	f.fetches.Add(1)
	return &service.TMDbMovieDetail{ID: 42, Title: "源片名", Overview: "补全后的简介"}, nil
}
func TestAgentMetadataHTTPApprovalOwnershipAndUndo(t *testing.T) {
	db, repos, _ := childFixture(t)
	if e := db.Model(&model.Media{}).Where("id = ?", "safe").Update("tm_db_id", 42).Error; e != nil {
		t.Fatal(e)
	}
	fake := &httpAgentMetadata{}
	r := gin.New()
	api := r.Group("/api", middleware.JWTAuth(childTestSecret), ChildAccessGate(db))
	RegisterViewerAgent(api, db, nil, fake)
	parent, _ := repos.User.FindByID("parent")
	child, _ := repos.User.FindByID("child")
	token := childToken(t, parent)
	base := "/api/admin/agent/runs"
	for _, request := range []struct{ path, body string }{{base + "/metadata", `{"media_id":"safe"}`}, {base + "/unknown/undo", `{"approval_digest":"` + strings.Repeat("0", 64) + `"}`}} {
		for _, uidToken := range []string{"", childToken(t, child)} {
			w := performChildRequest(t, r, "POST", request.path, uidToken, request.body)
			if w.Code != 401 && w.Code != 403 {
				t.Fatal("non-admin metadata endpoint", w.Code)
			}
		}
	}
	if fake.fetches.Load() != 0 {
		t.Fatal("unauthorized provider request")
	}
	w := performChildRequest(t, r, "POST", base+"/metadata", token, `{"media_id":"safe"}`)
	if w.Code != 200 {
		t.Fatal("preview", w.Code, w.Body)
	}
	var envelope struct {
		Data service.AgentRunView `json:"data"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &envelope); e != nil {
		t.Fatal(e)
	}
	p := envelope.Data
	if p.Status != "draft" || p.MediaID != "safe" || len(p.Changes) != 1 {
		t.Fatal("missing exact preview")
	}
	db.Create(&model.User{ID: "another-admin", Username: "another-admin", Role: "admin", Password: "hash"})
	another, _ := repos.User.FindByID("another-admin")
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/confirm", childToken(t, another), `{"approval_digest":"`+p.Digest+`"}`); w.Code != 404 {
		t.Fatal("another admin approved", w.Code)
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/confirm", token, `{"approval_digest":"`+strings.Repeat("0", 64)+`"}`); w.Code != 409 {
		t.Fatal("wrong digest approved")
	}
	for i := 0; i < 2; i++ {
		w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/confirm", token, `{"approval_digest":"`+p.Digest+`"}`)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"completed"`) {
			t.Fatal("confirm", w.Code, w.Body)
		}
	}
	if fake.fetches.Load() != 1 {
		t.Fatal("confirmation repeated source fetch")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/undo", childToken(t, another), `{"approval_digest":"`+p.Digest+`"}`); w.Code != 404 {
		t.Fatal("another admin undo", w.Code)
	}
	for i := 0; i < 2; i++ {
		w = performChildRequest(t, r, "POST", base+"/"+p.ID+"/undo", token, `{"approval_digest":"`+p.Digest+`"}`)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"reverted"`) {
			t.Fatal("undo", w.Code, w.Body)
		}
	}
	var m model.Media
	db.First(&m, "id = ?", "safe")
	if m.Overview != "" {
		t.Fatal("undo did not restore blank")
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("cacheable approval")
	}
}
