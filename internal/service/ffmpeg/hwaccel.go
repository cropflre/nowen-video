package ffmpeg

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nowen-video/nowen-video/internal/config"
	"go.uber.org/zap"
)

// 支持的硬件加速模式
const (
	HWAccelNone  = "none"
	HWAccelNVENC = "nvenc"
	HWAccelQSV   = "qsv"
	HWAccelVAAPI = "vaapi"
)

// DetectHWAccel 检测可用的硬件加速方式。
//
// 逻辑（与 transcode.go 历史实现一致）：
//  1. 如果 cfg.App.HWAccel 显式指定为 nvenc/qsv/vaapi/none，直接返回；
//  2. 否则尝试 NVIDIA NVENC（通过 nvidia-smi + ffmpeg -encoders 双重验证）；
//  3. Linux 下检查 /dev/dri/renderD128，再分别验证 QSV / VAAPI；
//  4. 都不可用则返回 "none"（纯软件编码）。
//
// ffmpegPath 为 ffmpeg 二进制路径（通常来自 cfg.App.FFmpegPath）。
func DetectHWAccel(cfg *config.Config, logger *zap.SugaredLogger) string {
	if cfg.App.HWAccel != "auto" && cfg.App.HWAccel != "" {
		return cfg.App.HWAccel
	}

	ffmpegPath := cfg.App.FFmpegPath

	// 1) NVIDIA NVENC
	if detectNvidiaSmi(logger) {
		cmd := exec.Command(ffmpegPath, "-hide_banner", "-encoders")
		if output, err := cmd.Output(); err == nil && strings.Contains(string(output), "h264_nvenc") {
			logger.Info("检测到NVIDIA GPU，使用NVENC硬件加速")
			return HWAccelNVENC
		}
	}

	// 2) Intel QSV / VAAPI（Linux only）
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
			cmd := exec.Command(ffmpegPath, "-hide_banner", "-encoders")
			if output, err := cmd.Output(); err == nil {
				out := string(output)
				if strings.Contains(out, "h264_qsv") {
					logger.Info("检测到Intel QSV，使用QSV硬件加速")
					return HWAccelQSV
				}
				if strings.Contains(out, "h264_vaapi") {
					logger.Info("检测到VAAPI，使用VAAPI硬件加速")
					return HWAccelVAAPI
				}
			}
		}
	}

	logger.Warn("未检测到硬件加速，使用软件编码")
	return HWAccelNone
}

// detectNvidiaSmi 在 PATH / Windows 常见路径 / 直接执行 三个层面尝试检测 nvidia-smi。
// 该实现兼容 Windows 下 PATH 不完整但 CreateProcess 仍能解析 System32 的场景。
func detectNvidiaSmi(logger *zap.SugaredLogger) bool {
	if _, err := exec.LookPath("nvidia-smi"); err == nil {
		return true
	}

	if runtime.GOOS == "windows" {
		commonPaths := []string{
			filepath.Join(os.Getenv("SystemRoot"), "System32", "nvidia-smi.exe"),
			filepath.Join(os.Getenv("ProgramFiles"), "NVIDIA Corporation", "NVSMI", "nvidia-smi.exe"),
		}
		for _, p := range commonPaths {
			if _, err := os.Stat(p); err == nil {
				if out, err := exec.Command(p, "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
					gpuName := strings.TrimSpace(string(out))
					logger.Infof("通过路径 %s 检测到 GPU: %s", p, gpuName)
					return true
				}
			}
		}

		// 某些环境下 System32 不在 Go 的 LookPath 搜索范围内，但 CreateProcess 可以找到
		if out, err := exec.Command("nvidia-smi", "--query-gpu=name", "--format=csv,noheader").Output(); err == nil {
			gpuName := strings.TrimSpace(string(out))
			logger.Infof("直接执行 nvidia-smi 检测到 GPU: %s", gpuName)
			return true
		}
	}

	return false
}
