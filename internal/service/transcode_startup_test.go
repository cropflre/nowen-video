package service

import (
	"errors"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestSubmitStartupStreamIsRetired(t *testing.T) {
	service := &TranscodeService{}
	task, err := service.SubmitStartupStream(
		&model.Media{ID: "media-1", FilePath: "/media/movie.mkv"},
		&model.MediaProbeRecord{VideoCodec: "hevc", DurationMS: 60_000},
	)
	if task != nil || !errors.Is(err, ErrPersistentRuntimeTranscodeRetired) {
		t.Fatalf("startup stream submission must be rejected task=%+v err=%v", task, err)
	}
}
