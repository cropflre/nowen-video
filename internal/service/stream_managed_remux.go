package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
)

type ManagedRemuxMode string

const (
	ManagedRemuxCopyAudio      ManagedRemuxMode = "remux"
	ManagedRemuxTranscodeAudio ManagedRemuxMode = "smart_remux"
)

// managedRemuxVideoCodecs are safe to copy into fragmented MP4 for the broad
// Web/desktop/mobile playback contract. HEVC remains capability-gated by the
// playback planner; H.264 is the universal smart-remux path.
var managedRemuxVideoCodecs = map[string]bool{
	"h264": true,
	"avc":  true,
	"avc1": true,
	"h265": true,
	"hevc": true,
}

// MP4 audio copy is deliberately conservative. Everything else is converted to
// AAC-LC while video remains bit-for-bit copied, covering DTS/TrueHD/FLAC/Opus
// sources without the startup cost of full video transcoding.
var mp4CopyAudioCodecs = map[string]bool{
	"":    true,
	"aac": true,
	"mp3": true,
}

func managedRemuxMode(media *model.Media) (ManagedRemuxMode, bool) {
	if media == nil || media.StreamURL != "" {
		return "", false
	}
	videoCodec := strings.ToLower(strings.TrimSpace(media.VideoCodec))
	if !managedRemuxVideoCodecs[videoCodec] {
		return "", false
	}
	audioCodec := strings.ToLower(strings.TrimSpace(media.AudioCodec))
	if mp4CopyAudioCodecs[audioCodec] {
		return ManagedRemuxCopyAudio, true
	}
	return ManagedRemuxTranscodeAudio, true
}

// CanManagedRemuxByID reports whether video can be copied into fragmented MP4.
// Full video transcoding remains the fallback for incompatible video codecs.
func (s *StreamService) CanManagedRemuxByID(mediaID string) (ManagedRemuxMode, bool, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", false, ErrMediaNotFound
	}
	mode, ok := managedRemuxMode(media)
	return mode, ok, nil
}

// ManagedRemuxStream is the production remux path used by all first-party
// clients. It runs inside the shared media execution runtime and automatically
// selects either zero-copy audio or AAC audio conversion while always copying
// compatible video.
func (s *StreamService) ManagedRemuxStream(mediaID string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	mode, ok := managedRemuxMode(media)
	if !ok {
		return fmt.Errorf("媒体视频编码不适合 Remux: %s", media.VideoCodec)
	}
	if s.transcoder == nil || s.transcoder.ExecutionRuntime() == nil {
		return fmt.Errorf("媒体执行 Runtime 不可用")
	}

	inputPath := media.FilePath
	if IsWebDAVPath(inputPath) {
		inputPath = ResolveRemoteFFmpegURL(s.cfg, inputPath)
	} else if _, err := os.Stat(inputPath); err != nil {
		return fmt.Errorf("媒体文件不可访问: %w", err)
	}

	args := make([]string, 0, 32)
	if start := strings.TrimSpace(r.URL.Query().Get("start")); start != "" && start != "0" {
		args = append(args, "-ss", start)
	}
	args = append(args,
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a:0?",
		"-c:v", "copy",
	)
	if mode == ManagedRemuxTranscodeAudio {
		args = append(args,
			"-c:a", "aac",
			"-profile:a", "aac_low",
			"-b:a", "192k",
			"-ac", "2",
		)
	} else {
		args = append(args, "-c:a", "copy")
	}
	args = append(args,
		"-sn",
		"-dn",
		"-movflags", "frag_keyframe+empty_moov+default_base_moof+faststart",
		"-avoid_negative_ts", "make_zero",
		"-f", "mp4",
		"pipe:1",
	)

	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "none")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Stream-Mode", string(mode))
	w.Header().Set("X-Video-Codec", "copy")
	if mode == ManagedRemuxTranscodeAudio {
		w.Header().Set("X-Audio-Codec", "aac")
	} else {
		w.Header().Set("X-Audio-Codec", "copy")
	}

	ctx := r.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	result := s.transcoder.ExecutionRuntime().Run(ctx, transcodegovernor.KindRemux, transcodeexecutor.Command{
		Path:       s.cfg.App.FFmpegPath,
		Args:       args,
		Stdout:     w,
		StderrTail: 60,
		Prepare: func(cmd *exec.Cmd) {
			setLowPriority(cmd)
		},
	}, transcodeexecutor.Callbacks{})

	if ctx.Err() != nil || result.Cancelled {
		s.logger.Debugf("Managed Remux 客户端已断开 media=%s mode=%s", mediaID, mode)
		return nil
	}
	if result.Err != nil {
		return fmt.Errorf("managed remux 失败 mode=%s: %s", mode, result.ErrorText())
	}
	return nil
}
