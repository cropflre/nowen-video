// Package ffmpeg 提供 FFmpeg 相关的公共能力，供 transcode / preprocess / abr
// 等上层服务复用。此包不感知任何业务模型，也不依赖 repository，保证可以被
// 任意上层服务直接调用。
package ffmpeg

import (
	"github.com/nowen-video/nowen-video/internal/config"
)

// CalcThreads 返回 FFmpeg 的线程数配置。
//
// 【最佳性能策略】固定返回 0，让 FFmpeg 自行探测并用满所有 CPU 核心
// （等价于 "-threads 0"）；上层 encoder 在 Threads<=0 时会直接省略该参数。
func CalcThreads(cfg *config.Config) int {
	_ = cfg
	return 0
}
