package service

import (
	"testing"

	"go.uber.org/zap"
)

func TestTaskCenterConstructionRemovesRuntimeTranscodeProjection(t *testing.T) {
	center := NewTaskCenterServiceWithoutRuntimeTranscode(nil, nil, nil, zap.NewNop().Sugar())
	if center == nil {
		t.Fatal("task center was not constructed")
	}
	if center.transcodeRepo != nil {
		t.Fatal("task center retained historical transcode_tasks projection")
	}
}

func TestTaskActionConstructionRemovesRuntimeTranscodeExecutor(t *testing.T) {
	dispatcher := NewTaskActionDispatcherWithoutRuntimeTranscode(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop().Sugar(),
	)
	if dispatcher == nil {
		t.Fatal("task action dispatcher was not constructed")
	}
	if dispatcher.transcode != nil || dispatcher.transcodeLookup != nil || dispatcher.mediaResolver != nil {
		t.Fatal("task action dispatcher retained runtime transcode operations")
	}
}
