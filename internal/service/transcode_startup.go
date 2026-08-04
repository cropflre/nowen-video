package service

import (
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
)

const (
	startupStreamPlannerVersion = "startup-hls-v3"
	startupStreamArtifactKind   = "startup_hls"
	startupStreamDurationMS     = int64(30_000)
	startupStreamPriority       = 10
)

// StartupStreamEligible is retained for historical diagnostics and migration
// tests. Eligibility no longer causes a file or Job to be created.
func StartupStreamEligible(media *model.Media, probe *model.MediaProbeRecord) bool {
	if media == nil || probe == nil || strings.TrimSpace(media.ID) == "" {
		return false
	}
	path := strings.ToLower(strings.TrimSpace(media.FilePath))
	if path == "" || strings.HasSuffix(path, ".strm") || IsWebDAVPath(media.FilePath) {
		return false
	}
	if probe.DurationMS > 0 && probe.DurationMS <= int64(hlsTargetSegmentSeconds*1000) {
		return false
	}
	codec := strings.ToLower(strings.TrimSpace(probe.VideoCodec))
	universalH264 := codec == "h264" || codec == "avc" || codec == "avc1"
	return !universalH264 || probe.BitDepth > 8 || probe.HDR
}

func startupStreamProfile(probe *model.MediaProbeRecord) string {
	height := 720
	if probe != nil && probe.Height > 0 && probe.Height < height {
		height = probe.Height
	}
	profiles := transcodeprofile.NamesUpToHeight(height)
	if len(profiles) == 0 {
		return "360p"
	}
	return profiles[len(profiles)-1]
}

func startupStreamActiveKey(
	media *model.Media,
	probe *model.MediaProbeRecord,
	quality,
	encodingPlanHash,
	timestampPlanHash string,
) string {
	fingerprint := transcodeSourceFingerprint(media)
	if probe != nil && probe.SourceFingerprint != "" {
		fingerprint = probe.SourceFingerprint
	}
	return stableHash(strings.Join([]string{
		media.ID,
		string(transcodedomain.IntentStartupHLS),
		quality,
		fmt.Sprintf("%d", startupStreamDurationMS),
		fingerprint,
		startupStreamPlannerVersion,
		encodingPlanHash,
		timestampPlanHash,
	}, "|"))
}

// SubmitStartupStream is a compatibility method for downstream callers built
// against the former API. It is now a hard execution boundary: startup media
// can only be generated inside an ephemeral Playback Session Generation.
func (s *TranscodeService) SubmitStartupStream(_ *model.Media, _ *model.MediaProbeRecord) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

func artifactLegacyTaskID(s *TranscodeService, jobID string) *string {
	if s == nil || s.executionRepo == nil || jobID == "" {
		return nil
	}
	job, err := s.executionRepo.FindJobByID(jobID)
	if err != nil {
		return nil
	}
	return job.LegacyTaskID
}

func dereferenceString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func startupStreamOutputArgs(args []string, durationMS int64) []string {
	if durationMS <= 0 || len(args) == 0 {
		return args
	}
	// Retained only for historical command certification. No production path
	// submits an immutable startup artifact after runtime retirement.
	body := make([]string, 0, len(args)+2)
	for index := 0; index < len(args)-1; index++ {
		arg := args[index]
		if arg == "-hls_playlist_type" && index+1 < len(args)-1 {
			index++
			continue
		}
		if arg == "-hls_flags" && index+1 < len(args)-1 {
			flags := strings.Split(args[index+1], "+")
			kept := make([]string, 0, len(flags))
			for _, flag := range flags {
				if flag != "append_list" {
					kept = append(kept, flag)
				}
			body = append(body, arg, strings.Join(kept, "+"))
			index++
			continue
		}
		body = append(body, arg)
	}
	seconds := float64(durationMS) / 1000
	body = append(body,
		"-t", fmt.Sprintf("%.3f", seconds),
		"-hls_playlist_type", "vod",
	)
	return append(body, args[len(args)-1])
}
