package service

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func newArtifactMaintenanceTestService(t *testing.T) (*ArtifactMaintenanceService, *gorm.DB) {
	t.Helper()
	cacheDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "artifact-maintenance.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)
	if err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	artifactStore, err := transcodeartifactstore.New(filepath.Join(cacheDir, "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	return &ArtifactMaintenanceService{
		repo:          repos.Transcode,
		executionRepo: repository.NewTranscodeExecutionRepo(db),
		artifactStore: artifactStore,
		cfg: &config.Config{
			Cache: config.CacheConfig{CacheDir: cacheDir},
		},
		logger:       zap.NewNop().Sugar(),
		diskUsageTTL: 0,
		done:         make(chan struct{}),
	}, db
}
