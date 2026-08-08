// Package service exposes audio-track metadata while runtime media generation
// is owned exclusively by ephemeral Playback Sessions.
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

type AudioTrackInfo struct {
	Index    int    `json:"index"`
	AudioIdx int    `json:"audio_idx"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Channels int    `json:"channels"`
	Default  bool   `json:"default"`
}

func probeAudioTracks(media *model.Media, ffprobePath string, logger *zap.SugaredLogger) []AudioTrackInfo {
	if media == nil || media.StreamURL != "" || IsWebDAVPath(media.FilePath) {
		return nil
	}
	if _, err := os.Stat(media.FilePath); err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "error",
		"-select_streams", "a",
		"-show_entries", "stream=index,codec_name,channels:stream_tags=language,title:stream_disposition=default",
		"-of", "json",
		media.FilePath,
	)
	out, err := cmd.Output()
	if err != nil {
		if logger != nil {
			logger.Debugf("probeAudioTracks ffprobe failed: %v", err)
		}
		return nil
	}

	var probe struct {
		Streams []struct {
			Index       int               `json:"index"`
			CodecName   string            `json:"codec_name"`
			Channels    int               `json:"channels"`
			Tags        map[string]string `json:"tags"`
			Disposition struct {
				Default int `json:"default"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		return nil
	}
	tracks := make([]AudioTrackInfo, 0, len(probe.Streams))
	for audioIdx, stream := range probe.Streams {
		track := AudioTrackInfo{
			Index:    stream.Index,
			AudioIdx: audioIdx,
			Codec:    stream.CodecName,
			Channels: stream.Channels,
			Default:  stream.Disposition.Default == 1,
		}
		if stream.Tags != nil {
			track.Language = stream.Tags["language"]
			track.Title = stream.Tags["title"]
		}
		tracks = append(tracks, track)
	}
	return tracks
}

func (s *StreamService) GetAudioTracks(mediaID string) []AudioTrackInfo {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil
	}
	return probeAudioTracks(media, s.cfg.App.FFprobePath, s.logger)
}

// The following methods are source-compatibility fences for old callers. They
// deliberately perform no filesystem access and never invoke FFmpeg.
func (s *StreamService) ServeOnDemandSegment(_ string, _ string, _ string, _ http.ResponseWriter, _ *http.Request) error {
	return ErrPersistentRuntimeTranscodeRetired
}

func (s *StreamService) GetAudioPlaylist(_ string, _ int) (string, error) {
	return "", ErrPersistentRuntimeTranscodeRetired
}

func (s *StreamService) ServeAudioSegment(_ string, _ int, _ string, _ http.ResponseWriter, _ *http.Request) error {
	return ErrPersistentRuntimeTranscodeRetired
}

func buildAudioMediaEntries(_ string, _ []AudioTrackInfo) string {
	return ""
}

func (s *StreamService) BuildAudioMediaEntries(_ string, _ []AudioTrackInfo) string {
	return ""
}
