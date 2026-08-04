package ffmpeg

import (
	"fmt"
	"path/filepath"
	"strconv"
)

// RollingHLSOptions applies session-scoped storage limits to the canonical HLS
// encoder arguments without duplicating codec/backend planning.
type RollingHLSOptions struct {
	ListSize        int
	DeleteThreshold int
	SegmentPattern  string
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
	result := make([]string, 0, len(args)+2)
	result = append(result, args[:outputIndex]...)
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
	return nil
}
