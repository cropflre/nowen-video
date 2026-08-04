package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// RollingHLSOptions applies session-scoped storage and stream-selection limits
// to the canonical HLS encoder arguments without duplicating codec/backend
// planning. Audio selection is emitted only for Session playback callers; other
// HLS producers retain their historical automatic stream selection.
type RollingHLSOptions struct {
	ListSize        int
	DeleteThreshold int
	SegmentPattern  string
	MapAudioTrack   bool
	AudioTrack      int
}

func BuildRollingHLSArgs(opts BuildOptions, rolling RollingHLSOptions) []string {
	args := BuildHLSArgs(opts)
	if len(args) == 0 {
		return nil
	}

	if rolling.ListSize <= 0 {
		rolling.ListSize = 30
	}
	if rolling.DeleteThreshold <= 0 {
		rolling.DeleteThreshold = 10
	}
	if rolling.SegmentPattern == "" {
		rolling.SegmentPattern = "seg_%06d.ts"
	}

	for index := 0; index+1 < len(args); index++ {
		switch args[index] {
		case "-hls_list_size":
			args[index+1] = strconv.Itoa(rolling.ListSize)
		case "-hls_segment_filename":
			args[index+1] = filepath.Join(opts.OutputDir, rolling.SegmentPattern)
		}
	}

	outputIndex := len(args) - 1
	result := make([]string, 0, len(args)+8)
	result = append(result, args[:outputIndex]...)
	if rolling.MapAudioTrack {
		audioTrack := rolling.AudioTrack
		if audioTrack < 0 {
			audioTrack = 0
		}
		result = append(result,
			"-map", "0:v:0",
			"-map", fmt.Sprintf("0:a:%d?", audioTrack),
		)
	}
	result = append(result,
		"-hls_delete_threshold",
		strconv.Itoa(rolling.DeleteThreshold),
	)
	result = append(result, args[outputIndex])
	return result
}

func ValidateRollingHLSOptions(rolling RollingHLSOptions) error {
	if rolling.ListSize < 1 {
		return fmt.Errorf("rolling HLS list size must be positive")
	}
	if rolling.DeleteThreshold < 1 {
		return fmt.Errorf("rolling HLS delete threshold must be positive")
	}
	if rolling.DeleteThreshold > rolling.ListSize {
		return fmt.Errorf("rolling HLS delete threshold must not exceed list size")
	}
	if rolling.MapAudioTrack && rolling.AudioTrack < -1 {
		return fmt.Errorf("rolling HLS audio track must be -1 or greater")
	}
	return nil
}
