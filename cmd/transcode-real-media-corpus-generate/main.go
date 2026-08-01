package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/nowen-video/nowen-video/internal/transcode/corpusgenerator"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func main() {
	outputDir := flag.String("output-dir", "real-media-corpus-v1", "write corpus assets and manifest under this directory")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "FFmpeg executable")
	ffprobe := flag.String("ffprobe", "ffprobe", "FFprobe executable")
	flag.Parse()

	manifest, hash, err := corpusgenerator.Generate(context.Background(), corpusgenerator.Options{
		FFmpegPath:  *ffmpeg,
		FFprobePath: *ffprobe,
		OutputDir:   *outputDir,
		Spec:        transcodecorpus.DefaultSpec(),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("generated %d assets manifest_sha256=%s output=%s\n", len(manifest.Assets), hash, *outputDir)
}
