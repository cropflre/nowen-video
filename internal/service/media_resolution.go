package service

import (
	"strconv"
	"strings"
)

// parseResolutionHeight normalizes the compact resolution labels stored on a
// Media row. Playback planning owns this helper; it has no relationship with
// the retired persistent Runtime worker.
func parseResolutionHeight(resolution string) int {
	value := strings.TrimSpace(resolution)
	switch value {
	case "4K":
		return 2160
	case "2K":
		return 1440
	case "1080p":
		return 1080
	case "720p":
		return 720
	case "480p":
		return 480
	case "360p":
		return 360
	default:
		if strings.HasSuffix(value, "p") {
			height, err := strconv.Atoi(strings.TrimSuffix(value, "p"))
			if err == nil && height > 0 {
				return height
			}
		}
		return 0
	}
}
