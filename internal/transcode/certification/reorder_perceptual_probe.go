package certification

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
)

const perceptualFrameBytes = 9 * 8

func probePerceptualFrameSequence(ctx context.Context, ffmpegPath, inputPath string) (transcodereorder.PerceptualFrameSequence, error) {
	args := []string{
		"-v", "error",
		"-i", inputPath,
		"-map", "0:v:0",
		"-vf", "scale=9:8:flags=area,format=gray",
		"-fps_mode", "passthrough",
		"-pix_fmt", "gray",
		"-f", "rawvideo",
		"-",
	}
	command := exec.CommandContext(ctx, ffmpegPath, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return transcodereorder.PerceptualFrameSequence{}, fmt.Errorf("ffmpeg perceptual decode failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	if len(output) == 0 || len(output)%perceptualFrameBytes != 0 {
		return transcodereorder.PerceptualFrameSequence{}, fmt.Errorf("perceptual rawvideo length %d is invalid", len(output))
	}
	hashes := make([]string, 0, len(output)/perceptualFrameBytes)
	for offset := 0; offset < len(output); offset += perceptualFrameBytes {
		hashes = append(hashes, perceptualFrameHash(output[offset:offset+perceptualFrameBytes]))
	}
	sequence, err := transcodereorder.NewPerceptualFrameSequence(hashes)
	if err != nil {
		return transcodereorder.PerceptualFrameSequence{}, err
	}
	return sequence, nil
}

func perceptualFrameHash(frame []byte) string {
	if len(frame) != perceptualFrameBytes {
		return ""
	}
	var averageSum int
	for row := 0; row < 8; row++ {
		for column := 0; column < 8; column++ {
			averageSum += int(frame[row*9+column])
		}
	}
	average := averageSum / 64
	var averageHash uint64
	var differenceHash uint64
	bitIndex := 0
	for row := 0; row < 8; row++ {
		for column := 0; column < 8; column++ {
			if int(frame[row*9+column]) >= average {
				averageHash |= uint64(1) << uint(bitIndex)
			}
			if frame[row*9+column] > frame[row*9+column+1] {
				differenceHash |= uint64(1) << uint(bitIndex)
			}
			bitIndex++
		}
	}
	return fmt.Sprintf("%016x%016x", averageHash, differenceHash)
}
