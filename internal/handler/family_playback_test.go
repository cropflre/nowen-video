package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

func TestFamilyHTTPBudgetRequiredForStreamsAndStopIsAccountScoped(t *testing.T) {
	db, repos, _ := childFixture(t)
	if err := db.Model(&model.UserPermission{}).Where("user_id = ?", "child").Update("daily_time_limit", 1).Error; err != nil {
		t.Fatal(err)
	}
	r := childRouter(t, db, repos)
	user, _ := repos.User.FindByID("child")
	token := childToken(t, user)
	response := performChildRequest(t, r, "GET", "/api/users/me/family-controls", token, "")
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"daily_limit_seconds":60`) {
		t.Fatal(response.Code, response.Body.String())
	}
	response = performChildRequest(t, r, "GET", "/api/stream/safe/direct", token, "")
	if response.Code != 403 {
		t.Fatal("unmetered stream admitted", response.Code)
	}
	response = performChildRequest(t, r, "POST", "/api/users/me/viewing/start", token, `{"media_id":"DO_NOT_LEAK_ADULT","user_id":"parent"}`)
	if response.Code != 403 {
		t.Fatal("unreviewed film received grant", response.Code)
	}
	response = performChildRequest(t, r, "POST", "/api/users/me/viewing/start", token, `{"media_id":"safe"}`)
	var grant struct {
		Data service.FamilyPlaybackStatus `json:"data"`
	}
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &grant) != nil || grant.Data.LeaseID == "" {
		t.Fatal(response.Code, response.Body.String())
	}
	response = performChildRequest(t, r, "GET", "/api/stream/safe/direct", token, "")
	if response.Code != 200 {
		t.Fatal("metered stream refused", response.Code)
	}
	other := &model.User{ID: "other-child", Username: "other-child", Password: "hash", Role: "user", ChildMode: true}
	if err := repos.User.Create(other); err != nil {
		t.Fatal(err)
	}
	stopBody, _ := json.Marshal(map[string]string{"lease_id": grant.Data.LeaseID})
	response = performChildRequest(t, r, "POST", "/api/users/me/viewing/stop", childToken(t, other), string(stopBody))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if err := service.NewFamilyPlaybackService(db).CheckAccess("child", "safe"); err != nil {
		t.Fatal("another account stopped child's grant", err)
	}
	response = performChildRequest(t, r, "POST", "/api/users/me/viewing/stop", token, string(stopBody))
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	response = performChildRequest(t, r, "GET", "/api/stream/safe/direct", token, "")
	if response.Code != 403 {
		t.Fatal("stopped grant still streams", response.Code)
	}
	if err := db.Model(&model.FamilyViewingDay{}).Where("user_id = ?", "child").Update("charged_seconds", 60).Error; err != nil {
		t.Fatal(err)
	}
	response = performChildRequest(t, r, "POST", "/api/users/me/viewing/start", token, `{"media_id":"safe","position":0,"duration":0}`)
	if response.Code != 403 || !strings.Contains(response.Body.String(), "FAMILY_TIME_LIMIT") {
		t.Fatal(response.Code, response.Body.String())
	}
	response = performChildRequest(t, r, "GET", "/api/media/safe", token, "")
	if response.Code != 200 {
		t.Fatal("spent budget blocks film information", response.Code)
	}
}

func TestPausedFamilySessionCanKeepAliveButCannotFetchSegments(t *testing.T) {
	db, repos, permissions := childFixture(t)
	if err := db.Model(&model.UserPermission{}).Where("user_id = ?", "child").Update("daily_time_limit", 1).Error; err != nil {
		t.Fatal(err)
	}
	runtime := &childSessionStub{id: "safe"}
	h := NewPlaybackSessionHandler(runtime, permissions, repos.Media, zap.NewNop().Sugar())
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "child"); c.Set("role", "user"); c.Next() })
	r.GET("/status/:sessionID", h.Status)
	r.POST("/heartbeat/:sessionID", h.Heartbeat)
	r.GET("/segment/:sessionID/:generationID/:file", h.Segment)
	if got := performChildRequest(t, r, "GET", "/status/test", "", ""); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := performChildRequest(t, r, "POST", "/heartbeat/test", "", `{"paused":true}`); got.Code != 200 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := performChildRequest(t, r, "POST", "/heartbeat/test", "", `{"paused":false}`); got.Code != 403 {
		t.Fatal(got.Code, got.Body.String())
	}
	if got := performChildRequest(t, r, "GET", "/segment/test/1/segment.ts", "", ""); got.Code != 403 {
		t.Fatal(got.Code, got.Body.String())
	}
	if runtime.opens != 0 {
		t.Fatal("unmetered segment opened")
	}
	if err := db.Where("media_id = ?", "safe").Delete(&model.ContentRating{}).Error; err != nil {
		t.Fatal(err)
	}
	if got := performChildRequest(t, r, "POST", "/heartbeat/test", "", `{"paused":true}`); got.Code != 403 {
		t.Fatal("paused heartbeat bypassed content revocation", got.Code)
	}
}
