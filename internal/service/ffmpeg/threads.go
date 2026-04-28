// Package ffmpeg 提供 FFmpeg 相关的公共能力，供 transcode / preprocess / abr
// 等上层服务复用。此包不感知任何业务模型，也不依赖 repository，保证可以被
// 任意上层服务直接调用。
package ffmpeg

import (
	"runtime"

	"github.com/nowen-video/nowen-video/internal/config"
)

// CalcThreads 根据 config.App.ResourceLimit 动态计算 FFmpeg 线程数。
//
// 规则（与历史 transcode/preprocess/abr 三处实现完全一致）：
//   - ResourceLimit <= 0 或 > 80 时按 80% 处理；
//   - threads = NumCPU * ResourceLimit / 100；
//   - threads 至少为 1；
//   - threads 不超过 CPU 核心数。
func CalcThreads(cfg *config.Config) int {
	resourceLimit := cfg.App.ResourceLimit
	if resourceLimit <= 0 || resourceLimit > 80 {
		resourceLimit = 80
	}
	cpuCount := runtime.NumCPU()
	threads := cpuCount * resourceLimit / 100
	if threads < 1 {
		threads = 1
	}
	if threads > cpuCount {
		threads = cpuCount
	}
	return threads
}
