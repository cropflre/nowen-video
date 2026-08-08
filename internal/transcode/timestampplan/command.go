package timestampplan

import (
	"fmt"
	"strings"
)

// ApplyFFmpeg binds a validated Timestamp Plan to one complete FFmpeg argument
// vector. Timestamp-preservation flags are global/input options, while muxer
// and frame-timing flags are inserted immediately before the output path.
func ApplyFFmpeg(args []string, plan Plan) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("ffmpeg arguments do not contain an output path")
	}
	if err := plan.Validate(); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(args)+6)
	if args[0] == "-y" {
		result = append(result, args[0], "-copyts", "-start_at_zero")
		args = args[1:]
	} else {
		result = append(result, "-copyts", "-start_at_zero")
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("ffmpeg arguments do not contain an output path")
	}
	result = append(result, args[:len(args)-1]...)
	result = append(result,
		"-avoid_negative_ts", plan.AvoidNegativeTS,
		"-fps_mode", plan.FPSMode,
	)
	result = append(result, args[len(args)-1])
	return result, nil
}

func CommandSummary(args []string) string {
	interesting := make([]string, 0, 8)
	for index := 0; index < len(args); index++ {
		switch args[index] {
		case "-copyts", "-start_at_zero":
			interesting = append(interesting, args[index])
		case "-ss", "-avoid_negative_ts", "-fps_mode":
			if index+1 < len(args) {
				interesting = append(interesting, args[index], args[index+1])
				index++
			}
		}
	}
	return strings.Join(interesting, " ")
}
