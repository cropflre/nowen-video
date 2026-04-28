package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// Profile 目标码率/分辨率配置。
// 所有字段都是 FFmpeg 能直接识别的字符串（如 "800k"、"1920"）。
type Profile struct {
	Width        int
	Height       int
	VideoBitrate string
	AudioBitrate string
	// MaxBitrate / BufSize 为空时默认与 VideoBitrate 相等（等价于低延迟 1x buffer）。
	MaxBitrate string
	BufSize    string
}

// BuildOptions 组装 FFmpeg 命令行的所有参数。
// 字段按 "输入 / 编码 / HLS / 其他" 分组，详见注释。
type BuildOptions struct {
	// ---------- 输入/输出 ----------
	InputPath  string   // 必填，源文件路径或 URL
	OutputDir  string   // 必填，输出目录，将在其中生成 stream.m3u8 + segNNNN.ts
	ExtraInput []string // 可选，-i 前额外插入的前置参数（例如 WebDAV 的 -reconnect 等）

	// ---------- 编码器相关 ----------
	HWAccel      string  // "", "none", "nvenc", "qsv", "vaapi"
	Profile      Profile // 目标分辨率/码率
	VAAPIDevice  string  // vaapi 模式下的设备路径（如 /dev/dri/renderD128）
	X264Preset   string  // software 模式下 -preset，留空则使用 "medium"
	QSVPreset    string  // qsv 模式下 -preset，留空则使用 "medium"
	Threads      int     // -threads 值；<=0 时省略
	UseCRF       bool    // 软件编码是否使用 CRF 恒定质量（transcode 场景用）；否则用 -b:v 固定码率
	CRF          int     // UseCRF=true 时生效，<=0 默认 23
	// QSVGlobalQuality >0 时 qsv 走 CQP 质量模式（-global_quality 值），忽略 -b:v/-maxrate/-bufsize；
	// <=0 时走码率模式（与预处理行为一致）。
	QSVGlobalQuality int
	SoftwareTune string  // x264 -tune 值（"zerolatency"/""），留空则不加
	NvencTune    string  // nvenc -tune 值（"ll"/""），留空则不加
	// QSVAttachOutputFormat 是否在 qsv 模式下显式指定 -hwaccel_output_format qsv。
	// transcode 场景建议 false（允许 FFmpeg 在 QSV 解码失败时回退软件解码），
	// 预处理场景可为 true（全 GPU 管线，性能更高）。
	QSVAttachOutputFormat bool
	// VideoFilter 完整的 -vf 值。**若非空则直接使用**（例如 HDR tonemap 链），
	// 为空时会按 HWAccel 自动生成 scale 滤镜。
	VideoFilter string

	// ---------- HLS 输出 ----------
	HLSTime       int    // 每片秒数；<=0 默认 4
	HLSFlags      string // 完整 -hls_flags 值；为空时使用 "independent_segments"
	HLSPlaylistType string // event / vod / ""(不设置)
	StartNumber   int    // <=0 时不设置
	ForceKeyFrames bool  // 是否加 "-force_key_frames expr:gte(t,n_forced*HLSTime)"
	// GOPSize -g 与 -keyint_min 的值；<=0 时默认按 HLSTime*25 估算
	GOPSize int

	// ---------- 输入 seek（仅 transcode 使用）----------
	StartOffsetSec float64 // >0.5 时在 -i 前插入 -ss

	// SkipVAAPIRateLimits VAAPI 分支是否省略 -maxrate/-bufsize/-keyint_min。
	// 仅用于与历史 transcode 实现保持字节一致；新场景不建议开启。
	SkipVAAPIRateLimits bool
}

// BuildHLSArgs 根据 opts 构建完整的 FFmpeg 参数列表（不含 ffmpeg 二进制路径）。
//
// 返回的参数顺序与历史 transcode.go / preprocess.go / abr.go 的实现保持一致，
// 以便回归测试时可以做字节级对比。
func BuildHLSArgs(opts BuildOptions) []string {
	hlsTime := opts.HLSTime
	if hlsTime <= 0 {
		hlsTime = 4
	}
	gop := opts.GOPSize
	if gop <= 0 {
		gop = hlsTime * 25
	}
	gopStr := strconv.Itoa(gop)

	outputPath := filepath.Join(opts.OutputDir, "stream.m3u8")
	segmentPath := filepath.Join(opts.OutputDir, "seg%04d.ts")

	// -------- baseArgs（-y + 可选 -ss + 硬件加速前置 + -i） --------
	baseArgs := []string{"-y"}
	if opts.StartOffsetSec > 0.5 {
		baseArgs = append(baseArgs, "-ss", fmt.Sprintf("%.2f", opts.StartOffsetSec))
	}
	baseArgs = append(baseArgs, opts.ExtraInput...)

	// 硬件加速前置参数
	switch opts.HWAccel {
	case HWAccelNVENC:
		baseArgs = append(baseArgs, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
	case HWAccelQSV:
		baseArgs = append(baseArgs, "-hwaccel", "qsv")
		if opts.QSVAttachOutputFormat {
			baseArgs = append(baseArgs, "-hwaccel_output_format", "qsv")
		}
	case HWAccelVAAPI:
		dev := opts.VAAPIDevice
		baseArgs = append(baseArgs,
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
			"-vaapi_device", dev,
		)
	}

	baseArgs = append(baseArgs, "-i", opts.InputPath)

	// -------- videoArgs --------
	videoArgs := buildVideoArgs(opts, gopStr)

	// -------- audioArgs --------
	audioArgs := []string{"-c:a", "aac", "-b:a", opts.Profile.AudioBitrate, "-ac", "2"}

	// -------- hlsArgs --------
	hlsFlags := opts.HLSFlags
	if hlsFlags == "" {
		hlsFlags = "independent_segments"
	}
	hlsArgs := []string{
		"-f", "hls",
		"-hls_time", strconv.Itoa(hlsTime),
		"-hls_list_size", "0",
		"-hls_segment_filename", segmentPath,
		"-hls_flags", hlsFlags,
	}
	if opts.HLSPlaylistType != "" {
		hlsArgs = append(hlsArgs, "-hls_playlist_type", opts.HLSPlaylistType)
	}
	if opts.StartNumber > 0 {
		hlsArgs = append(hlsArgs, "-start_number", strconv.Itoa(opts.StartNumber))
	}
	if opts.ForceKeyFrames {
		hlsArgs = append(hlsArgs, "-force_key_frames", fmt.Sprintf("expr:gte(t,n_forced*%d)", hlsTime))
	}
	hlsArgs = append(hlsArgs, outputPath)

	// -------- 组装 --------
	args := baseArgs
	if opts.Threads > 0 {
		args = append(args, "-threads", strconv.Itoa(opts.Threads))
	}
	args = append(args, videoArgs...)
	args = append(args, audioArgs...)
	args = append(args, hlsArgs...)
	return args
}

// buildVideoArgs 按 HWAccel 分支生成视频编码参数。
func buildVideoArgs(opts BuildOptions, gopStr string) []string {
	p := opts.Profile
	maxRate := p.MaxBitrate
	if maxRate == "" {
		maxRate = p.VideoBitrate
	}
	bufSize := p.BufSize
	if bufSize == "" {
		bufSize = p.VideoBitrate
	}

	// 未显式指定 VideoFilter 时，按 HWAccel 自动生成 scale
	scale := opts.VideoFilter

	switch opts.HWAccel {
	case HWAccelNVENC:
		if scale == "" {
			scale = fmt.Sprintf("scale_cuda=%d:%d:format=nv12", p.Width, p.Height)
		}
		args := []string{
			"-c:v", "h264_nvenc",
			"-preset", "p4",
		}
		if opts.NvencTune != "" {
			args = append(args, "-tune", opts.NvencTune)
		}
		args = append(args,
			"-b:v", p.VideoBitrate,
			"-maxrate", maxRate,
			"-bufsize", bufSize,
			"-g", gopStr,
			"-keyint_min", gopStr,
			"-sc_threshold", "0",
			"-vf", scale,
		)
		return args

	case HWAccelQSV:
		preset := opts.QSVPreset
		if preset == "" {
			preset = "medium"
		}
		if scale == "" {
			scale = fmt.Sprintf("scale_qsv=%d:%d", p.Width, p.Height)
		}
		if opts.QSVGlobalQuality > 0 {
			// CQP 质量模式：对应 transcode 实时场景，不锁码率、以恒定质量编码。
			return []string{
				"-c:v", "h264_qsv",
				"-preset", preset,
				"-global_quality", strconv.Itoa(opts.QSVGlobalQuality),
				"-g", gopStr,
				"-pix_fmt", "nv12",
				"-vf", scale,
			}
		}
		return []string{
			"-c:v", "h264_qsv",
			"-preset", preset,
			"-b:v", p.VideoBitrate,
			"-maxrate", maxRate,
			"-bufsize", bufSize,
			"-pix_fmt", "yuv420p",
			"-vf", scale,
			"-g", gopStr,
			"-keyint_min", gopStr,
		}

	case HWAccelVAAPI:
		if scale == "" {
			scale = fmt.Sprintf("scale_vaapi=w=%d:h=%d", p.Width, p.Height)
		}
		if opts.SkipVAAPIRateLimits {
			return []string{
				"-c:v", "h264_vaapi",
				"-b:v", p.VideoBitrate,
				"-g", gopStr,
				"-pix_fmt", "yuv420p",
				"-vf", scale,
			}
		}
		return []string{
			"-c:v", "h264_vaapi",
			"-b:v", p.VideoBitrate,
			"-maxrate", maxRate,
			"-bufsize", bufSize,
			"-pix_fmt", "yuv420p",
			"-vf", scale,
			"-g", gopStr,
			"-keyint_min", gopStr,
		}

	default:
		// 软件编码 libx264
		preset := opts.X264Preset
		if preset == "" {
			preset = "medium"
		}
		if scale == "" {
			scale = fmt.Sprintf("scale=%d:%d", p.Width, p.Height)
		}
		args := []string{
			"-c:v", "libx264",
			"-preset", preset,
		}
		if opts.SoftwareTune != "" {
			args = append(args, "-tune", opts.SoftwareTune)
		}
		if opts.UseCRF {
			crf := opts.CRF
			if crf <= 0 {
				crf = 23
			}
			args = append(args, "-crf", strconv.Itoa(crf))
		} else {
			args = append(args,
				"-b:v", p.VideoBitrate,
				"-maxrate", maxRate,
				"-bufsize", bufSize,
			)
		}
		args = append(args,
			"-g", gopStr,
			"-keyint_min", gopStr,
			"-sc_threshold", "0",
			"-pix_fmt", "yuv420p",
			"-vf", scale,
		)
		return args
	}
}
