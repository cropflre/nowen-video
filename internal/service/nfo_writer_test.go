package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

func TestWriteMediaNFOCreatesLowercaseLocalFile(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "Movie.Title.2024.mkv")
	if err := os.WriteFile(videoPath, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewNFOService(zap.NewNop().Sugar())
	media := &model.Media{
		Title:    "Movie Title",
		Year:     2024,
		Overview: "A good scrape result.",
		Rating:   8.6,
		Genres:   "剧情,科幻",
		FilePath: videoPath,
		TMDbID:   12345,
	}
	people := []model.MediaPerson{
		{Role: "director", Person: model.Person{Name: "Jane Director"}},
		{Role: "actor", Character: "Lead", SortOrder: 1, Person: model.Person{Name: "Alex Actor", ProfileURL: "actor.jpg"}},
	}

	nfoPath, err := svc.WriteMediaNFO(videoPath, media, people, NFOWriteOptions{ExistingPolicy: NFOExistingSkip})
	if err != nil {
		t.Fatalf("WriteMediaNFO failed: %v", err)
	}
	if nfoPath != filepath.Join(dir, "Movie.Title.2024.nfo") {
		t.Fatalf("expected lowercase same-name .nfo path, got %q", nfoPath)
	}

	data, err := os.ReadFile(nfoPath)
	if err != nil {
		t.Fatalf("expected local nfo file to exist: %v", err)
	}
	content := string(data)
	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		"<movie>",
		"<title>Movie Title</title>",
		"<uniqueid type=\"tmdb\" default=\"true\">12345</uniqueid>",
		"<director>Jane Director</director>",
		"<name>Alex Actor</name>",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("generated nfo missing %q:\n%s", want, content)
		}
	}
}

func TestWriteMediaNFOSkipsExistingFile(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "Existing.mp4")
	nfoPath := filepath.Join(dir, "Existing.nfo")
	if err := os.WriteFile(nfoPath, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewNFOService(zap.NewNop().Sugar())
	gotPath, err := svc.WriteMediaNFO(videoPath, &model.Media{Title: "New Title"}, nil, NFOWriteOptions{ExistingPolicy: NFOExistingSkip})
	if err != nil {
		t.Fatalf("WriteMediaNFO failed: %v", err)
	}
	if gotPath != nfoPath {
		t.Fatalf("expected existing nfo path %q, got %q", nfoPath, gotPath)
	}
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep me" {
		t.Fatalf("expected existing nfo to be preserved, got %q", string(data))
	}
}

func TestWriteMediaNFOOverwritesExistingFile(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "Overwrite.mp4")
	nfoPath := filepath.Join(dir, "Overwrite.nfo")
	if err := os.WriteFile(nfoPath, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	svc := NewNFOService(zap.NewNop().Sugar())
	_, err := svc.WriteMediaNFO(videoPath, &model.Media{Title: "Fresh", Overview: "Updated"}, nil, NFOWriteOptions{ExistingPolicy: NFOExistingOverwrite})
	if err != nil {
		t.Fatalf("WriteMediaNFO failed: %v", err)
	}
	data, err := os.ReadFile(nfoPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "<title>Fresh</title>") {
		t.Fatalf("expected nfo to be overwritten, got:\n%s", string(data))
	}
}

func TestWriteMediaNFOUsesEpisodeDetailsRootForEpisodes(t *testing.T) {
	dir := t.TempDir()
	videoPath := filepath.Join(dir, "Show.S01E02.mp4")

	svc := NewNFOService(zap.NewNop().Sugar())
	_, err := svc.WriteMediaNFO(videoPath, &model.Media{
		Title:        "Show",
		MediaType:    "episode",
		SeasonNum:    1,
		EpisodeNum:   2,
		EpisodeTitle: "Pilot Again",
		Overview:     "Episode plot.",
	}, nil, NFOWriteOptions{ExistingPolicy: NFOExistingSkip})
	if err != nil {
		t.Fatalf("WriteMediaNFO failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Show.S01E02.nfo"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "<episodedetails>") || !strings.Contains(content, "<episode>2</episode>") {
		t.Fatalf("expected episode nfo, got:\n%s", content)
	}
}
