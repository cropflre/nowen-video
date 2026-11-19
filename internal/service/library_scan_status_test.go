package service

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

func TestMissingLibraryFolderLeavesVisibleFailureAndPreservesIndex(t *testing.T) {
	db := setupTestDB(t)
	svc := &LibraryService{repo: repository.NewRepositories(db).Library, logger: zap.NewNop().Sugar()}

	for _, operation := range []struct {
		name string
		run  func(string) error
	}{
		{"scan", svc.Scan},
		{"reindex", svc.Reindex},
	} {
		t.Run(operation.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "not-mounted")
			lib := model.Library{ID: operation.name, Name: "Movies", Path: path, Type: "movie"}
			if err := db.Create(&lib).Error; err != nil {
				t.Fatal(err)
			}
			media := model.Media{ID: operation.name, LibraryID: lib.ID, Title: "Existing", FilePath: filepath.Join(path, "existing.mp4")}
			if err := db.Create(&media).Error; err != nil {
				t.Fatal(err)
			}
			if err := operation.run(lib.ID); err != nil {
				t.Fatal(err)
			}

			deadline := time.Now().Add(2 * time.Second)
			for {
				failures, err := svc.RecentScanFailures()
				if err != nil {
					t.Fatal(err)
				}
				if strings.Contains(failures[lib.ID], "文件夹无法访问") && len(svc.ActiveScanPhases()) == 0 {
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("failure was not published as a terminal status: %v", failures)
				}
				time.Sleep(5 * time.Millisecond)
			}
			restarted := &LibraryService{repo: svc.repo}
			failures, err := restarted.RecentScanFailures()
			if err != nil || !strings.Contains(failures[lib.ID], "文件夹无法访问") {
				t.Fatalf("a new library service must read the saved failure: %v, %v", failures, err)
			}
			if err := db.First(&model.Media{}, "id = ?", media.ID).Error; err != nil {
				t.Fatalf("an inaccessible folder must not clear the existing index: %v", err)
			}
		})
	}
}
