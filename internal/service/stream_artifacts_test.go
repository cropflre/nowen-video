package service

import (
	"errors"
	"os"
	"testing"

	"gorm.io/gorm"
)

func TestArtifactFallbackOnlyAcceptsReadinessErrors(t *testing.T) {
	for _, err := range []error{
		nil,
		ErrArtifactNotReady,
		gorm.ErrRecordNotFound,
		os.ErrNotExist,
	} {
		if !artifactReadinessError(err) {
			t.Fatalf("readiness error was rejected: %v", err)
		}
	}

	for _, err := range []error{
		errors.New("database unavailable"),
		errors.New("permission denied"),
		errors.New("corrupt artifact metadata"),
	} {
		if artifactReadinessError(err) {
			t.Fatalf("infrastructure error was masked as readiness: %v", err)
		}
	}
}
