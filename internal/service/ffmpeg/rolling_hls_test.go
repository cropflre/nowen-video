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
}

func TestValidateRollingHLSOptions(t *testing.T) {
	require.NoError(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 30, DeleteThreshold: 10}))
	require.Error(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 0, DeleteThreshold: 10}))
	require.Error(t, ValidateRollingHLSOptions(RollingHLSOptions{ListSize: 10, DeleteThreshold: 11}))
}

func requireArgPair(t *testing.T, args []string, key, expected string) {
	t.Helper()
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key {
			require.Equal(t, expected, args[index+1])
			return
		}
	}
	t.Fatalf("argument %s not found in %v", key, args)
}
