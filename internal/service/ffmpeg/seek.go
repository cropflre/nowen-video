package ffmpeg

import (
	"strconv"
	"strings"
)

// WithInputSeekMicros replaces or inserts the input-side -ss value using up to
// microsecond precision. Whole and centisecond-aligned offsets keep the legacy
// two-decimal representation, while frame/AAC packet boundaries retain their
// exact six-decimal position.
func WithInputSeekMicros(args []string, offsetMicros int64) []string {
	result := append([]string(nil), args...)
	if offsetMicros <= 500_000 {
		return result
	}
	value := formatSeekMicros(offsetMicros)
	inputIndex := indexOf(result, "-i")
	if inputIndex < 0 {
		return result
	}
	for index := 0; index+1 < inputIndex; index++ {
		if result[index] == "-ss" {
			result[index+1] = value
			return result
		}
	}
	insertAt := 0
	if len(result) > 0 && result[0] == "-y" {
		insertAt = 1
	}
	result = append(result, "", "")
	copy(result[insertAt+2:], result[insertAt:len(result)-2])
	result[insertAt] = "-ss"
	result[insertAt+1] = value
	return result
}

func formatSeekMicros(offsetMicros int64) string {
	value := strconv.FormatFloat(float64(offsetMicros)/1_000_000, 'f', 6, 64)
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return value
	}
	fraction := strings.TrimRight(parts[1], "0")
	for len(fraction) < 2 {
		fraction += "0"
	}
	return parts[0] + "." + fraction
}

func indexOf(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
