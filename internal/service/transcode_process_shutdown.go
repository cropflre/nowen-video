package service

import "go.uber.org/zap"

// registerFullTranscodeProcessShutdown is retained as a temporary source-level
// compatibility bridge for NewServices. Full now owns the ordered shutdown
// sequence explicitly in cmd/server/main.go, so this hook must not subscribe to
// process signals or fence leases ahead of the graceful deadline.
func registerFullTranscodeProcessShutdown(_ *TranscodeService, _ *zap.SugaredLogger) {}
