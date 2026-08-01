package service

import (
	"fmt"
	"math"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"
)

type transcodeEncodingIdentity struct {
	Version   string
	Hash      string
	Canonical string
}

func startupEncodingIdentity(probe *model.MediaProbeRecord, profileID string) (transcodeEncodingIdentity, error) {
	plan, err := startupEncodingPlan(probe, profileID)
	if err != nil {
		return transcodeEncodingIdentity{}, err
	}
	version, hash, canonical, err := encodingplan.Identity(plan)
	if err != nil {
		return transcodeEncodingIdentity{}, err
	}
	return transcodeEncodingIdentity{Version: version, Hash: hash, Canonical: canonical}, nil
}

func startupEncodingPlan(probe *model.MediaProbeRecord, profileID string) (encodingplan.Plan, error) {
	if probe == nil {
		return encodingplan.Plan{}, fmt.Errorf("startup encoding plan requires media probe")
	}
	profile, ok := transcodeprofile.Runtime(profileID)
	if !ok {
		return encodingplan.Plan{}, fmt.Errorf("unknown startup encoding profile %q", profileID)
	}

	frameRateMilli := 0
	frameRate := probe.FrameRate()
	if frameRate > 0 && !math.IsNaN(frameRate) && !math.IsInf(frameRate, 0) {
		frameRateMilli = int(math.Round(frameRate * 1000))
	}
	colorPolicy := "source_sdr"
	primaries := "source"
	transfer := "source"
	matrix := "source"
	if probe.HDR {
		colorPolicy = "hdr_to_bt709"
		primaries = "bt709"
		transfer = "bt709"
		matrix = "bt709"
	}

	return encodingplan.Plan{
		SchemaVersion: encodingplan.SchemaVersion,
		ProfileID:     profileID,
		Transport: encodingplan.TransportPlan{
			Protocol:          "hls",
			Container:         "mpegts",
			SegmentFormat:     "mpegts",
			SegmentDurationMS: int64(hlsTargetSegmentSeconds * 1000),
		},
		Video: encodingplan.VideoPlan{
			Codec:                "h264",
			Width:                profile.Width,
			Height:               profile.Height,
			PixelFormatContract:  "yuv420p-8bit",
			FrameRatePolicy:      "source",
			SourceFrameRateMilli: frameRateMilli,
			GOPSize:              probe.GOPSize(hlsTargetSegmentSeconds),
			KeyframeIntervalMS:   int64(hlsTargetSegmentSeconds * 1000),
			ForceKeyframes:       true,
			SceneCut:             false,
			ColorPolicy:          colorPolicy,
			ColorPrimaries:       primaries,
			Transfer:             transfer,
			Matrix:               matrix,
		},
		Audio: encodingplan.AudioPlan{
			Codec:            "aac",
			Bitrate:          profile.AudioBitrate,
			Channels:         2,
			Track:            -1,
			SampleRatePolicy: "source",
		},
	}, nil
}

func sameEncodingPlan(
	versionA,
	hashA,
	canonicalA,
	versionB,
	hashB,
	canonicalB string,
) bool {
	if versionA == "" || hashA == "" || versionB == "" || hashB == "" {
		return false
	}
	if versionA != versionB || hashA != hashB {
		return false
	}
	return canonicalA == "" || canonicalB == "" || canonicalA == canonicalB
}
