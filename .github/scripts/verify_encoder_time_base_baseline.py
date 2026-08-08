#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

TOOLCHAIN = {
    "ffmpeg_version": "ffmpeg version 6.1.1-3ubuntu5 Copyright (c) 2000-2023 the FFmpeg developers",
    "ffprobe_version": "ffprobe version 6.1.1-3ubuntu5 Copyright (c) 2007-2023 the FFmpeg developers",
}

# source_startup, source_continuation, output_startup, output_continuation,
# startup_dominant_us, continuation_dominant_us, maximum_frame_delta,
# video_boundary_delta_us, audio_boundary_delta_us, startup_end_skew_us,
# continuation_start_skew_us, boundary_delta_skew_us, skew_transition_us,
# projection_residual_us
EXPECTED = {
    "candidate-cfr-24000-1001-origin-zero-v1": (720, 240, 720, 240, 41711, 41711, 0, -21322, -58667, -13989, -51333, -37345, -37344, 1),
    "candidate-cfr-24-origin-zero-v1": (720, 240, 720, 240, 41667, 41667, 0, -21333, -58667, 16000, -21333, -37334, -37333, 1),
    "candidate-cfr-25-origin-zero-v1": (750, 250, 750, 250, 40000, 40000, 0, -21333, -58667, 16000, -21333, -37334, -37333, 1),
    "candidate-cfr-30000-1001-origin-zero-v1": (900, 299, 900, 299, 33367, 33367, 0, -21333, -58667, -14000, -51333, -37334, -37333, 1),
    "candidate-cfr-30-origin-zero-v1": (900, 300, 900, 300, 33333, 33333, 0, -21333, -58667, 16000, -21333, -37334, -37333, 1),
    "candidate-cfr-50-origin-zero-v1": (1500, 500, 1500, 500, 20000, 20000, 0, -21333, -58667, 16000, -21333, -37334, -37333, 1),
    "candidate-cfr-60000-1001-origin-zero-v1": (1799, 599, 1799, 599, 16678, 16678, 0, -21322, -58667, 2689, -34655, -37345, -37344, 1),
    "candidate-vfr-24-30-origin-zero-v1": (780, 300, 780, 300, 41667, 33333, 0, -29667, -58667, 7666, -21333, -29000, -28999, 1),
    "candidate-vfr-25-30-origin-zero-v1": (800, 300, 800, 300, 40000, 33333, 0, -28000, -58667, 9333, -21333, -30667, -30666, 1),
    "candidate-vfr-30000-1001-60000-1001-origin-zero-v1": (1199, 599, 1199, 600, 33367, 16678, 1, -38011, -58667, -14000, -34655, -20656, -20655, 1),
    "candidate-cfr-30-origin-positive-5s-v1": (900, 300, 900, 300, 33333, 33333, 0, -21333, -58667, 16000, -21333, -37334, -37333, 1),
    "candidate-cfr-30-origin-negative-2s-v1": (900, 300, 900, 300, 33333, 33333, 0, -21333, -58667, 16000, -21333, -37334, -37333, 1),
}

CANDIDATES = ["encoder-time-base-avtb-v1", "encoder-time-base-90k-v1"]
AV_KEYS = [
    "video_boundary_delta_micros",
    "audio_boundary_delta_micros",
    "startup_end_skew_micros",
    "continuation_start_skew_micros",
    "boundary_delta_skew_micros",
    "skew_transition_micros",
    "projection_residual_micros",
]


def exact_range(summary, key, expected):
    value = summary[key]
    assert value == {"min": expected, "max": expected, "span": 0}, (key, value, expected)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    report = json.loads(args.report.read_text())
    evidence = report["evidence"]

    assert evidence["ffmpeg_version"] == TOOLCHAIN["ffmpeg_version"]
    assert evidence["ffprobe_version"] == TOOLCHAIN["ffprobe_version"]
    assert [case["case"]["id"] for case in evidence["cases"]] == list(EXPECTED)

    tolerance_cases = []
    for case in evidence["cases"]:
        case_id = case["case"]["id"]
        (
            source_startup,
            source_continuation,
            output_startup,
            output_continuation,
            startup_dominant,
            continuation_dominant,
            maximum_frame_delta,
            *av_values,
        ) = EXPECTED[case_id]
        assert case["source_startup_timeline"]["frame_count"] == source_startup
        assert case["source_continuation_timeline"]["frame_count"] == source_continuation

        assert [candidate["spec"]["id"] for candidate in case["candidates"]] == CANDIDATES
        for candidate in case["candidates"]:
            summary = candidate["summary"]
            exact_range(summary, "startup_frame_count", output_startup)
            exact_range(summary, "continuation_frame_count", output_continuation)
            exact_range(summary, "startup_dominant_delta_micros", startup_dominant)
            exact_range(summary, "continuation_dominant_delta_micros", continuation_dominant)
            exact_range(summary, "startup_near_zero_delta_count", 0)
            exact_range(summary, "continuation_near_zero_delta_count", 0)
            exact_range(summary, "startup_duplicate_pts_count", 0)
            exact_range(summary, "continuation_duplicate_pts_count", 0)
            exact_range(summary, "startup_adjacent_duplicate_frame_count", 0)
            exact_range(summary, "continuation_adjacent_duplicate_frame_count", 0)
            assert summary["maximum_absolute_frame_count_delta"] == maximum_frame_delta
            assert summary["boundary_frame_tolerance_used"] == (maximum_frame_delta == 1)
            assert summary["sequence_stable"] is True
            assert summary["cadence_stable"] is True
            assert summary["av_sync_stable"] is True
            assert summary["all_preserved"] is True
            assert summary["stable"] is True
            for key, expected in zip(AV_KEYS, av_values):
                exact_range(summary, key, expected)

            for run in candidate["runs"]:
                assert run["startup_timeline"]["frame_count"] == output_startup
                assert run["continuation_timeline"]["frame_count"] == output_continuation
                assert run["startup_timeline"]["dominant_delta_micros"] == startup_dominant
                assert run["continuation_timeline"]["dominant_delta_micros"] == continuation_dominant
                assert abs(run["startup_mapping"]["frame_count_delta"]) <= 1
                assert abs(run["continuation_mapping"]["frame_count_delta"]) <= 1

        comparison = case["comparison"]
        assert comparison["candidate_a_id"] == CANDIDATES[0]
        assert comparison["candidate_b_id"] == CANDIDATES[1]
        assert comparison["startup_sequence_equivalent"] is True
        assert comparison["continuation_sequence_equivalent"] is True
        assert comparison["frame_mapping_equivalent"] is True
        assert comparison["cadence_equivalent"] is True
        assert comparison["max_av_sync_metric_difference_micros"] == 0
        assert comparison["av_sync_within_tolerance"] is True
        assert comparison["equivalent"] is True

        if maximum_frame_delta:
            tolerance_cases.append(case_id)

    assert tolerance_cases == ["candidate-vfr-30000-1001-60000-1001-origin-zero-v1"]
    print(json.dumps({
        "contract_hash": report["contract_hash"],
        "exact_cases": len(EXPECTED),
        "tolerance_cases": tolerance_cases,
        "toolchain": "ffmpeg-6.1.1-3ubuntu5",
    }, sort_keys=True))


if __name__ == "__main__":
    main()
