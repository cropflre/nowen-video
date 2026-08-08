package service

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// PreprocessArtifactService is the formal durable media-output boundary.
// Existing API names remain source-compatible while the old generic
// PreprocessService name is removed incrementally from handlers and clients.
type PreprocessArtifactService = PreprocessService

// NewPreprocessArtifactService constructs explicit administrator preprocessing
// from the stateless media execution platform. It never depends on the retired
// persistent Runtime queue or playback Artifact store.
func NewPreprocessArtifactService(
	cfg *config.Config,
	repo *repository.PreprocessRepo,
	mediaRepo *repository.MediaRepo,
	abrService *ABRService,
	execution *MediaExecutionService,
	logger *zap.SugaredLogger,
) (*PreprocessArtifactService, error) {
	if execution == nil {
		return nil, fmt.Errorf("media execution service is required")
	}
	return NewPreprocessService(
		cfg,
		repo,
		mediaRepo,
		abrService,
		execution.GetHWAccelInfo(),
		logger,
	), nil
}

// BindMediaExecution migrates an already-constructed compatibility instance to
// the formal execution boundary. Full server assembly uses this during the
// transition because Services still exposes the historical concrete type.
func (s *PreprocessService) BindMediaExecution(execution *MediaExecutionService) error {
	if s == nil {
		return nil
	}
	if execution == nil {
		return fmt.Errorf("media execution service is required")
	}
	s.mu.Lock()
	s.hwAccel = execution.GetHWAccelInfo()
	s.mu.Unlock()
	return nil
}

func (s *PreprocessService) MediaExecutionHWAccel() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hwAccel
}
