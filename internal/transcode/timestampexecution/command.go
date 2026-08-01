package timestampexecution

import (
	"fmt"
	"strconv"
	"strings"
)

// ApplyContinuation adds explicit PTS shaping to one complete FFmpeg output
// argument vector. The input slice is never mutated. This adapter is currently
// used only by deterministic certification fixtures.
func ApplyContinuation(args []string, plan Plan) ([]string, error) {
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	if len(args) == 0 || strings.TrimSpace(args[len(args)-1]) == "" {
		return nil, fmt.Errorf("ffmpeg arguments do not contain an output path")
	}
	result := append([]string(nil), args...)
	var err error
	if plan.VideoPTSShiftMicros > 0 {
		result, err = mergeOutputFilter(result, "-vf", "setpts=PTS+"+formatShiftSeconds(plan.VideoPTSShiftMicros)+"/TB")
		if err != nil {
			return nil, err
		}
	}
	if plan.AudioPTSShiftMicros > 0 {
		result, err = mergeOutputFilter(result, "-af", "asetpts=PTS+"+formatShiftSeconds(plan.AudioPTSShiftMicros)+"/TB")
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func mergeOutputFilter(args []string, option, expression string) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("ffmpeg arguments are empty")
	}
	matched := -1
	for index := 0; index < len(args)-1; index++ {
		if args[index] != option {
			continue
		}
		if index+1 >= len(args)-1 || strings.TrimSpace(args[index+1]) == "" {
			return nil, fmt.Errorf("%s does not have a filter expression", option)
		}
		if matched >= 0 {
			return nil, fmt.Errorf("multiple %s options are not supported by timestamp execution v2", option)
		}
		matched = index
	}
	result := append([]string(nil), args...)
	if matched >= 0 {
		result[matched+1] = result[matched+1] + "," + expression
		return result, nil
	}
	output := result[len(result)-1]
	body := append([]string(nil), result[:len(result)-1]...)
	body = append(body, option, expression, output)
	return body, nil
}

func formatShiftSeconds(micros int64) string {
	return strconv.FormatFloat(float64(micros)/1_000_000, 'f', 6, 64)
}
