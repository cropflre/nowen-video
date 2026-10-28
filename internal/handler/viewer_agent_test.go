package handler

import (
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/middleware"
)

func TestViewerAgentHTTPRequiresAuthAndKeepsChildScope(t *testing.T) {
	db, repos, _ := childFixture(t)
	r := gin.New()
	api := r.Group("/api")
	api.Use(middleware.JWTAuth(childTestSecret), ChildAccessGate(db))
	RegisterViewerAgent(api, db, nil)
	child, _ := repos.User.FindByID("child")
	token := childToken(t, child)
	if response := performChildRequest(t, r, "POST", "/api/agent/chat", token, `{"message":"检查影视库","role":"admin"}`); response.Code != 403 || strings.Contains(response.Body.String(), "DO_NOT_LEAK") {
		t.Fatalf("child read management: %d %s", response.Code, response.Body)
	}
	parent, _ := repos.User.FindByID("parent")
	if response := performChildRequest(t, r, "POST", "/api/agent/chat", childToken(t, parent), `{"message":"检查影视库"}`); response.Code != 200 || !strings.Contains(response.Body.String(), "management") {
		t.Fatalf("parent inspection: %d %s", response.Code, response.Body)
	}
	if response := performChildRequest(t, r, "POST", "/api/agent/chat", "", `{"message":"帮我选一部"}`); response.Code != 401 {
		t.Fatalf("no auth %d", response.Code)
	}
	for _, body := range []string{`{"message":"帮我选一部","user_id":"parent","role":"admin"}`, `{"message":"继续看"}`, `{"message":"我的想看"}`, `{"message":"收藏第二部","context":[{"id":"safe","kind":"media"},{"id":"DO_NOT_LEAK_ADULT","kind":"media"}]}`} {
		response := performChildRequest(t, r, "POST", "/api/agent/chat", token, body)
		if response.Code != 200 || strings.Contains(response.Body.String(), "DO_NOT_LEAK") || strings.Contains(response.Body.String(), "file_path") {
			t.Fatalf("scope failed %d %s", response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("missing private cache policy")
		}
	}
	for _, body := range []string{`{`, `{"message":""}`, `{"message":"` + strings.Repeat("字", 501) + `"}`, `{"message":"` + strings.Repeat("x", 9000) + `"}`} {
		if response := performChildRequest(t, r, "POST", "/api/agent/chat", token, body); response.Code != 400 {
			t.Fatalf("input status %d", response.Code)
		}
	}
}
