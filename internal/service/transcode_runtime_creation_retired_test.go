package service

import (
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestCreateExecutionJobIsRetiredWithoutDatabaseWrite(t *testing.T) {
	service, db, _ := newRuntimeRetirementTestService(t)
	job, err := service.createExecutionJob(
		&model.Media{ID: "media-new", FilePath: "/media/movie.mkv"},
		"720p",
		0,
		"legacy-task",
		TranscodePriorityInteractive,
	)
	if job != nil || !errors.Is(err, ErrPersistentRuntimeTranscodeRetired) {
		t.Fatalf("persistent runtime job creation must be rejected job=%+v err=%v", job, err)
	}
	var count int64
	if err := db.Model(&model.TranscodeJobRecord{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("retired runtime creation wrote %d job rows", count)
	}
}
