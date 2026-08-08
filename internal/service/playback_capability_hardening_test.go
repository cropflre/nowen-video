package service

import "testing"

func TestOptionalBrowserAudioCodecsRequirePositiveCapability(t *testing.T) {
	for _, tc := range []struct {
		name      string
		codec     string
		supported PlaybackClientCapabilities
	}{
		{
			name:      "flac",
			codec:     "flac",
			supported: PlaybackClientCapabilities{AudioSupportsFLAC: true},
		},
		{
			name:      "opus",
			codec:     "opus",
			supported: PlaybackClientCapabilities{AudioSupportsOpus: true},
		},
		{
			name:      "ac3",
			codec:     "ac3",
			supported: PlaybackClientCapabilities{AudioSupportsAC3: true},
		},
		{
			name:      "eac3",
			codec:     "eac3",
			supported: PlaybackClientCapabilities{AudioSupportsEAC3: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if audioCodecCompatibleWithCaps(tc.codec, PlaybackClientCapabilities{}) {
				t.Fatalf("%s must stay conservative without a positive client capability", tc.codec)
			}
			if !audioCodecCompatibleWithCaps(tc.codec, tc.supported) {
				t.Fatalf("%s must be accepted after a positive client capability", tc.codec)
			}
		})
	}
}

func TestStableBrowserAudioBaselineRemainsCompatible(t *testing.T) {
	for _, codec := range []string{"aac", "mp3", "vorbis"} {
		if !audioCodecCompatibleWithCaps(codec, PlaybackClientCapabilities{}) {
			t.Fatalf("stable browser codec %s must remain baseline-compatible", codec)
		}
	}
}

func TestSmartRemuxFollowsOptionalAudioCapability(t *testing.T) {
	info := &MediaPlayInfo{
		MediaID:          "m1",
		PreferDirectPlay: true,
		VideoCodec:       "h264",
		AudioCodec:       "flac",
	}

	unsupported := PlaybackClientCapabilities{SupportsRemux: true}
	if !canSmartRemuxInfoWithCaps(info, unsupported) {
		t.Fatal("FLAC without a positive client capability should use smart remux")
	}

	supported := PlaybackClientCapabilities{SupportsRemux: true, AudioSupportsFLAC: true}
	if canSmartRemuxInfoWithCaps(info, supported) {
		t.Fatal("FLAC with a positive client capability must not be forced through smart remux")
	}
}
