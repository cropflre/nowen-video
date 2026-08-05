package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
)

func TestPreprocessArtifactBindingUsesMediaExecutionCapability(t *testing.T) {
	preprocess := &PreprocessService{hwAccel: ffmpeg.HWAccelNone}
	execution := &MediaExecutionService{hwAccel: ffmpeg.HWAccelQSV}

	if err := preprocess.BindMediaExecution(execution); err != nil {
		t.Fatal(err)
	}
	if got := preprocess.MediaExecutionHWAccel(); got != ffmpeg.HWAccelQSV {
		t.Fatalf("preprocess hw accel = %q, want %q", got, ffmpeg.HWAccelQSV)
	}
}

func TestPreprocessArtifactBindingRejectsMissingExecution(t *testing.T) {
	preprocess := &PreprocessService{}
	if err := preprocess.BindMediaExecution(nil); err == nil {
		t.Fatal("preprocess accepted a missing media execution service")
	}
}
