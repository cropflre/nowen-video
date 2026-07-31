package service

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"go.uber.org/zap"
)

var (
	fullTranscodeSignalOnce sync.Once
	fullTranscodeSignalMu   sync.RWMutex
	fullTranscodeService    *TranscodeService
)

// registerFullTranscodeProcessShutdown is installed only by NewServices (the
// Full server aggregate). Lite owns its explicit ordered Shutdown in
// cmd/server-lite/main.go. Full's historical main function already owns the
// process signal and HTTP shutdown path, but did not fence transcode Leases;
// this hook closes that gap without introducing a second HTTP signal loop.
func registerFullTranscodeProcessShutdown(transcoder *TranscodeService, logger *zap.SugaredLogger) {
	if transcoder == nil {
		return
	}
	fullTranscodeSignalMu.Lock()
	fullTranscodeService = transcoder
	fullTranscodeSignalMu.Unlock()

	fullTranscodeSignalOnce.Do(func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-quit
			signal.Stop(quit)
			fullTranscodeSignalMu.RLock()
			current := fullTranscodeService
			fullTranscodeSignalMu.RUnlock()
			if current == nil {
				return
			}
			if logger != nil {
				logger.Info("Full 服务退出：停止转码领取并释放本机 Worker Lease")
			}
			current.FenceForProcessExit()
		}()
	})
}
