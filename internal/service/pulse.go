package service

import (
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// PulseService is a compatibility tombstone.
//
// The Pulse analytics product and its runtime implementation were removed.
// This zero-state type only keeps the legacy Full composition source-compatible
// until the old /api/admin/pulse/* routes can be deleted in a breaking release.
// It starts no goroutines, stores no dependencies and performs no database work.
type PulseService struct{}

func NewPulseService(_ *repository.PulseRepo, _ *zap.SugaredLogger) *PulseService {
	return &PulseService{}
}

func (*PulseService) SetWSHub(_ *WSHub) {}
