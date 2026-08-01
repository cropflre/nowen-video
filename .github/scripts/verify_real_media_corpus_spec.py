#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

EXPECTED_CASES = [
    "real-mp4-h264-aac-cfr-24000-1001-v1",
    "real-mp4-h264-aac-cfr-30000-1001-edit-list-v1",
    "real-mkv-h264-aac-vfr-24-30-v1",
    "real-mpegts-h264-aac-cfr-30-b3-v1",
    "real-mkv-h264-opus-cfr-25-v1",
    "real-mp4-h264-aac-cfr-30-aac-44100-v1",
]

REQUIRED_EVIDENCE = {
    "timestamp_plan",
    "output_cadence",
    "packet_order",
    "produced_media_attestation",
    "boundary_packet",
    "av_boundary_sync",
}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    report = json.loads(args.report.read_text(encoding="utf-8"))
    spec = report["spec"]

    assert report["schema_version"] == "real-media-corpus-spec-report-v1"
    assert report["spec_version"] == "real-media-corpus-spec-v1"
    assert len(report["spec_hash"]) == 64
    assert spec["schema_version"] == report["spec_version"]
    assert spec["seamless_allowed"] is False
    assert spec["discontinuity_required"] is True
    assert [case["id"] for case in spec["cases"]] == EXPECTED_CASES

    containers = set()
    frame_rate_modes = set()
    audio_codecs = set()
    sample_rates = set()
    edit_lists = 0
    non_zero_origins = 0
    for case in spec["cases"]:
        source = case["source"]
        video = source["video"]
        audio = source["audio"]
        timeline = source["timeline"]
        assert case["tier"] == "deterministic_container"
        assert case["boundary_micros"] == 30_000_000
        assert timeline["duration_micros"] == 40_000_000
        assert timeline["discontinuous"] is False
        assert set(case["required_evidence"]) == REQUIRED_EVIDENCE
        assert video["codec"] == "h264"
        assert video["profile"] == "high"
        assert video["pixel_format"] == "yuv420p"
        assert video["width"] == 640
        assert video["height"] == 360
        assert video["open_gop"] is False
        assert video["interlaced"] is False
        assert video["hdr"] is False
        assert video["b_frames"] > 0
        assert video["reference_frames"] > 0
        assert audio["channels"] == 2
        assert audio["layout"] == "stereo"
        assert audio["track_count"] == 1

        containers.add(source["container"])
        frame_rate_modes.add(video["frame_rate_mode"])
        audio_codecs.add(audio["codec"])
        sample_rates.add(audio["sample_rate"])
        edit_lists += int(timeline["has_edit_list"])
        non_zero_origins += int(timeline["origin_micros"] != 0)

    assert containers == {"mp4", "matroska", "mpegts"}
    assert frame_rate_modes == {"cfr", "vfr"}
    assert audio_codecs == {"aac", "opus"}
    assert sample_rates == {44_100, 48_000}
    assert edit_lists == 1
    assert non_zero_origins == 2

    print(json.dumps({
        "case_count": len(EXPECTED_CASES),
        "containers": sorted(containers),
        "spec_hash": report["spec_hash"],
    }, sort_keys=True))


if __name__ == "__main__":
    main()
