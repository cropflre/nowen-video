package probe

import (
	"strings"
	"testing"
)

func TestParseFFprobeOutputDoesNotTreatSDRHEVCAsHDR(t *testing.T) {
	record, err := parseFFprobeOutput([]byte(`{
		"streams": [
			{
				"index": 0,
				"codec_type": "video",
				"codec_name": "hevc",
				"width": 3840,
				"height": 2160,
				"pix_fmt": "yuv420p10le",
				"avg_frame_rate": "24000/1001",
				"color_transfer": "bt709",
				"color_primaries": "bt709",
				"color_space": "bt709"
			},
			{
				"index": 1,
				"codec_type": "audio",
				"codec_name": "truehd",
				"channels": 8,
				"channel_layout": "7.1",
				"sample_rate": "48000",
				"tags": {"language": "eng"},
				"disposition": {"default": 1}
			}
		],
		"format": {"format_name": "matroska,webm", "duration": "120.125"}
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if record.HDR {
		t.Fatal("ordinary SDR HEVC must not be classified as HDR")
	}
	if record.FrameRateNum != 24000 || record.FrameRateDen != 1001 || record.GOPSize(2) != 48 {
		t.Fatalf("unexpected frame-rate normalization: %+v", record)
	}
	if record.BitDepth != 10 {
		t.Fatalf("expected 10-bit pixel format, got %d", record.BitDepth)
	}
	if record.DurationMS != 120125 {
		t.Fatalf("unexpected duration: %d", record.DurationMS)
	}
	streams := record.AudioStreams()
	if len(streams) != 1 || streams[0].Codec != "truehd" || streams[0].Channels != 8 || !streams[0].Default {
		t.Fatalf("unexpected audio stream normalization: %+v", streams)
	}
}

func TestParseFFprobeOutputDetectsPQHLGAndMetadataHDR(t *testing.T) {
	cases := []struct {
		name     string
		transfer string
		sideData string
	}{
		{name: "pq", transfer: "smpte2084"},
		{name: "hlg", transfer: "arib-std-b67"},
		{name: "mastering", transfer: "bt709", sideData: `,"side_data_list":[{"side_data_type":"Mastering display metadata"}]`},
		{name: "dolby-vision", transfer: "bt709", sideData: `,"side_data_list":[{"side_data_type":"DOVI configuration record"}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","width":1920,"height":1080,"pix_fmt":"yuv420p10le","avg_frame_rate":"25/1","color_transfer":"` + tc.transfer + `"` + tc.sideData + `}],"format":{"duration":"1"}}`
			record, err := parseFFprobeOutput([]byte(payload))
			if err != nil {
				t.Fatal(err)
			}
			if !record.HDR {
				t.Fatalf("expected HDR for %s: %+v", tc.name, record)
			}
		})
	}
}

func TestParseFFprobeOutputRequiresVideo(t *testing.T) {
	_, err := parseFFprobeOutput([]byte(`{"streams":[{"codec_type":"audio","codec_name":"aac"}]}`))
	if err == nil || !strings.Contains(err.Error(), "no video stream") {
		t.Fatalf("expected missing-video error, got %v", err)
	}
}

func TestParseFrameRateReducesFraction(t *testing.T) {
	numerator, denominator := parseFrameRate("60000/2002")
	if numerator != 30000 || denominator != 1001 {
		t.Fatalf("fraction not reduced: %d/%d", numerator, denominator)
	}
	if numerator, denominator := parseFrameRate("0/0"); numerator != 0 || denominator != 0 {
		t.Fatalf("invalid rate accepted: %d/%d", numerator, denominator)
	}
}
