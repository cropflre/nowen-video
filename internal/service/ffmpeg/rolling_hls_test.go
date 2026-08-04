package ffmpeg

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildRollingHLSArgs(t *testing.T) {
	outputDir := t.TempDir()
	args := BuildRollingHLSArgs(BuildOptions{
		InputPath: "input.mkv",
		OutputDir: outputDir,
		Profile: Profile{
			Width:        1280,
			Height:       720,
			VideoBitrate: "3000k",
			AudioBitrate: "128k",
		},
		HLSTime:  2,
		HLSFlags: "delete_segments+temp_file+independent_segments",
	}, RollingHLSOptions{
		ListSize:        30,
		DeleteThreshold: 10,
		SegmentPattern:  "seg_%06d.ts",
	})

	requireArgPair(t, args, "-hls_list_size", "30")
	requireArgPair(t, args, "-hls_delete_threshold", "10")
	requireArgPair(t, args, "-hls_segment_filename", filepath.Join(outputDir, "seg_%06d.ts"))
	require.Equal(t, filepath.Join(outputDir, "stream.m3u8"), args[len(args)-1])
	require.NotContains(t, args, "-map")
}

func TestBuildRollingHLSArgsMapsSelectedSessionAudio(t *testing.T) {
	outputDir := t.TempDir()
	args := BuildRollingHLSArgs(BuildOptions{
		InputPath: "input.mkv",
		OutputDir: outputDir,
		Profile: Profile{
			Width:        1920,
			Height:       1080,
			VideoBitrate: "6000k",
			AudioBitrate: "192k",
		},
	}, RollingHLSOptions{
		ListSize:        30,
		DeleteThreshold: 10,
		MapAudioTrack:   true,
		AudioTrack:      2,
	})

	requireArgPair(t, args, "-map", "0:v:0")
	require.Contains(t, args, "0:a:2?")
	require.Equal(t, filepath.Join(outputDir, "stream.m3u8"), args[len(args)-1])
}

func TestBuildRollingHLSArgsDefaultsNegativeAudioTrackToFirst(t *testing.T) {
	args := BuildRollingHLSArgs(BuildOptions{
		InputPath: "input.mkv",
		OutputDir: t.TempDir(),
		Profile: Profile{
			Width:        1280,
			Height:       720,
			VideoBitrate: "3000k",
			AudioBitrate: "128k",
		},
	}, RollingHLSOptions{
		ListSize:        30,
		DeleteThreshold: 10,
		MapAudioTrack:   true,
		AudioTrack:      -1,
	})
	require.Contains(t, args, "0:a:0?")
}

func TestValidateRollingHLSOptions(t *testing.T) {
	require.NoError(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 30, DeleteThreshold: 10}))
	require.NoError(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 30, DeleteThreshold: 10, MapAudioTrack: true, AudioTrack: -1}))
	require.Error(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 0, DeleteThreshold: 10}))
	require.Error(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 10, DeleteThreshold: 11}))
	require.Error(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 30, DeleteThreshold: 10, MapAudioTrack: true, AudioTrack: -2}))
}

func requireArgPair(t *testing.T, args []string, key, expected string) {
	t.Helper()
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key && args[index+1] == expected {
			return
		}
	}
	t.Fatalf("argument pair %s %s not found in %v", key, expected, args)
}
