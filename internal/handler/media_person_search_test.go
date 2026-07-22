package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"gorm.io/gorm"
)

func TestPersonSearchMatchesOriginalName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Person{}); err != nil {
		t.Fatalf("migrate person: %v", err)
	}

	repos := repository.NewRepositories(db)
	people := []model.Person{
		{Name: "新垣结衣", OrigName: "Yui Aragaki"},
		{Name: "石原里美", OrigName: "Satomi Ishihara"},
	}
	for i := range people {
		if err := repos.Person.Create(&people[i]); err != nil {
			t.Fatalf("create person: %v", err)
		}
	}

	h := &MediaHandler{personRepo: repos.Person}
	router := gin.New()
	router.GET("/api/persons/:id", h.GetPersonDetail)

	request := httptest.NewRequest(http.MethodGet, "/api/persons/search?q=Aragaki&limit=5", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body.String())
	}
	var body struct {
		Data []model.Person `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].Name != "新垣结衣" {
		t.Fatalf("unexpected people: %#v", body.Data)
	}
}

func TestPersonSearchRejectsBlankQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	repos := repository.NewRepositories(db)
	h := &MediaHandler{personRepo: repos.Person}
	router := gin.New()
	router.GET("/api/persons/:id", h.GetPersonDetail)

	request := httptest.NewRequest(http.MethodGet, "/api/persons/search?q=%20%20", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
}
