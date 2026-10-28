package handler

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service"
)

type httpAgentScanExecutor struct{ calls atomic.Int32 }

func (f *httpAgentScanExecutor) ScanNewMovies(*model.Library) (int, error) {
	f.calls.Add(1)
	return 3, nil
}
func TestAgentScanHTTPAuthorizesPreviewAndKeepsExecutionReceipt(t *testing.T) {
	db, repos, _ := childFixture(t)
	fake := &httpAgentScanExecutor{}
	if err := db.Create(&model.User{ID: "second-parent", Username: "second-parent", Role: "admin", Password: "hash"}).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	api := r.Group("/api")
	api.Use(middleware.JWTAuth(childTestSecret), ChildAccessGate(db))
	RegisterViewerAgent(api, db, nil, fake)
	parent, _ := repos.User.FindByID("parent")
	token := childToken(t, parent)
	child, _ := repos.User.FindByID("child")
	childJWT := childToken(t, child)
	base := "/api/admin/agent/runs"
	for _, request := range []struct{ method, path, body string }{{"GET", base, ""}, {"POST", base, `{"library_id":"kids"}`}, {"POST", base + "/unknown/confirm", `{"approval_digest":"` + strings.Repeat("0", 64) + `"}`}} {
		if w := performChildRequest(t, r, request.method, request.path, childJWT, request.body); w.Code != 403 {
			t.Fatalf("child endpoint status %d", w.Code)
		}
		if w := performChildRequest(t, r, request.method, request.path, "", request.body); w.Code != 401 {
			t.Fatalf("unauthenticated endpoint status %d", w.Code)
		}
	}
	w := performChildRequest(t, r, "POST", base, token, `{"library_id":"kids"}`)
	if w.Code != 200 {
		t.Fatalf("preview %d %s", w.Code, w.Body)
	}
	var envelope struct {
		Data service.AgentRunView `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	run := envelope.Data
	if run.Status != "draft" || fake.calls.Load() != 0 || len(run.Paths) != 1 {
		t.Fatal("preview executed or lacks scope")
	}
	second, _ := repos.User.FindByID("second-parent")
	if w = performChildRequest(t, r, "POST", base+"/"+run.ID+"/confirm", childToken(t, second), `{"approval_digest":"`+run.Digest+`"}`); w.Code != 404 {
		t.Fatal("another parent claimed preview")
	}
	if w = performChildRequest(t, r, "POST", base+"/"+run.ID+"/confirm", token, `{"approval_digest":"`+strings.Repeat("0", 64)+`"}`); w.Code != 409 {
		t.Fatal("tampered approval accepted")
	}
	for i := 0; i < 2; i++ {
		w = performChildRequest(t, r, "POST", base+"/"+run.ID+"/confirm", token, `{"approval_digest":"`+run.Digest+`"}`)
		if w.Code != 200 {
			t.Fatalf("confirm %d %s", w.Code, w.Body)
		}
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		w = performChildRequest(t, r, "GET", base, token, "")
		if strings.Contains(w.Body.String(), `"status":"completed"`) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if fake.calls.Load() != 1 || !strings.Contains(w.Body.String(), `"new_count":3`) {
		t.Fatalf("missing single execution receipt %d %s", fake.calls.Load(), w.Body)
	}
	if w.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatal("task receipts cacheable")
	}
}
