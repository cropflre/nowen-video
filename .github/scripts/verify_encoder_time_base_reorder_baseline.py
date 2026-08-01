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
# startup_reordered, continuation_reordered,
# startup_pts_inversions, continuation_pts_inversions,
# startup_reorder_depth, continuation_reorder_depth,
# startup_max_composition_us, continuation_max_composition_us,
# video_boundary_delta_us, audio_boundary_delta_us,
# startup_end_skew_us, continuation_start_skew_us,
# exact_startup_pixels, exact_continuation_pixels
EXPECTED = {
    "reorder-cfr-24-b2-origin-zero-v1": (
        720, 240, 720, 240, 41667, 41667, 0,
        255, 85, 240, 80, 2, 2, 125000, 125000,
        -41667, -79000, 16000, -21333, True, True,
    ),
    "reorder-cfr-30000-1001-b3-origin-zero-v1": (
        900, 299, 900, 299, 33367, 33367, 0,
        240, 80, 225, 75, 3, 3, 133467, 133467,
        -33367, -70700, -14000, -51333, False, True,
    ),
    "reorder-vfr-24-30-b3-origin-zero-v1": (
        780, 300, 780, 300, 41667, 33333, 0,
        210, 80, 195, 75, 3, 3, 166667, 133333,
        -41667, -79000, 16000, -21333, True, True,
    ),
    "reorder-cfr-30-b3-origin-positive-5s-v1": (
        900, 300, 900, 300, 33333, 33333, 0,
        240, 80, 225, 75, 3, 3, 133333, 133333,
        -33333, -70667, 16000, -21333, False, False,
    ),
    "reorder-cfr-30-b3-origin-negative-2s-v1": (
        900, 300, 900, 300, 33333, 33333, 0,
        240, 80, 225, 75, 3, 3, 133333, 133333,
        -33333, -70667, 16000, -21333, False, False,
    ),
    "reorder-cfr-30-b3-long-gop-origin-zero-v1": (
        900, 300, 900, 300, 33333, 33333, 0,
        240, 80, 225, 75, 3, 3, 133333, 133333,
        -33333, -70667, 16000, -21333, False, False,
    ),
}

CASE_POLICY = {
    "reorder-cfr-24-b2-origin-zero-v1": ("cfr", 0, 48, 2, 3),
    "reorder-cfr-30000-1001-b3-origin-zero-v1": ("cfr", 0, 60, 3, 4),
    "reorder-vfr-24-30-b3-origin-zero-v1": ("vfr", 0, 60, 3, 4),
    "reorder-cfr-30-b3-origin-positive-5s-v1": ("cfr", 5_000_000, 60, 3, 4),
    "reorder-cfr-30-b3-origin-negative-2s-v1": ("cfr", -2_000_000, 60, 3, 4),
    "reorder-cfr-30-b3-long-gop-origin-zero-v1": ("cfr", 0, 300, 3, 4),
}

CANDIDATES = ["encoder-time-base-avtb-v1", "encoder-time-base-90k-v1"]


def exact_range(summary, key, expected):
    value = summary[key]
    assert value == {"min": expected, "max": expected, "span": 0}, (key, value, expected)


def verify_packet_order(packet, frame_count, reordered, inversions, depth, max_composition):
    assert packet["packet_count"] == frame_count
    assert packet["reordered_packet_count"] == reordered
    assert packet["pts_after_dts_count"] == reordered
    assert packet["adjacent_pts_inversion_count"] == inversions
    assert packet["max_presentation_reorder_depth"] == depth
    assert packet["max_composition_offset_micros"] == max_composition
    assert packet["min_composition_offset_micros"] == 0
    assert packet["pts_before_dts_count"] == 0
    assert packet["dts_duplicate_count"] == 0
    assert packet["dts_non_monotonic_count"] == 0
    assert sum(item["count"] for item in packet["dts_delta_histogram"]) == frame_count - 1
    assert sum(item["count"] for item in packet["composition_offset_histogram"]) == frame_count


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    report = json.loads(args.report.read_text())
    evidence = report["evidence"]

    assert report["schema_version"] == "ffmpeg-encoder-time-base-reorder-matrix-v1"
    assert report["contract_version"] == "encoder-time-base-reorder-evidence-v1"
    assert evidence["schema_version"] == report["contract_version"]
    assert evidence["ffmpeg_version"] == TOOLCHAIN["ffmpeg_version"]
    assert evidence["ffprobe_version"] == TOOLCHAIN["ffprobe_version"]
    assert evidence["repeat_count"] == 3
    assert evidence["packet_variance_ticks"] == 1
    assert evidence["perceptual_max_hamming_distance"] == 8
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True
    assert [case["case"]["base"]["id"] for case in evidence["cases"]] == list(EXPECTED)

    exact_pixel_mismatch_windows = []
    for case in evidence["cases"]:
        case_id = case["case"]["base"]["id"]
        (
            source_startup,
            source_continuation,
            output_startup,
            output_continuation,
            startup_dominant,
            continuation_dominant,
            maximum_frame_delta,
            startup_reordered,
            continuation_reordered,
            startup_inversions,
            continuation_inversions,
            startup_depth,
            continuation_depth,
            startup_composition,
            continuation_composition,
            video_boundary_delta,
            audio_boundary_delta,
            startup_end_skew,
            continuation_start_skew,
            exact_startup_pixels,
            exact_continuation_pixels,
        ) = EXPECTED[case_id]

        base_spec = case["case"]["base"]
        source_mode, source_offset, gop_size, b_frames, reference_frames = CASE_POLICY[case_id]
        assert base_spec["source_mode"] == source_mode
        assert base_spec["source_offset_micros"] == source_offset
        assert base_spec["gop_size"] == gop_size
        assert case["case"]["b_frames"] == b_frames
        assert case["case"]["b_adapt"] == 0
        assert case["case"]["reference_frames"] == reference_frames
        assert case["case"]["open_gop"] is False
        assert case["source_startup_timeline"]["frame_count"] == source_startup
        assert case["source_continuation_timeline"]["frame_count"] == source_continuation

        assert [candidate["spec"]["id"] for candidate in case["candidates"]] == CANDIDATES
        for candidate in case["candidates"]:
            summary = candidate["summary"]
            base = summary["base"]
            exact_range(base, "startup_frame_count", output_startup)
            exact_range(base, "continuation_frame_count", output_continuation)
            exact_range(base, "startup_dominant_delta_micros", startup_dominant)
            exact_range(base, "continuation_dominant_delta_micros", continuation_dominant)
            exact_range(base, "startup_near_zero_delta_count", 0)
            exact_range(base, "continuation_near_zero_delta_count", 0)
            exact_range(base, "startup_duplicate_pts_count", 0)
            exact_range(base, "continuation_duplicate_pts_count", 0)
            exact_range(base, "startup_adjacent_duplicate_frame_count", 0)
            exact_range(base, "continuation_adjacent_duplicate_frame_count", 0)
            exact_range(base, "video_boundary_delta_micros", video_boundary_delta)
            exact_range(base, "audio_boundary_delta_micros", audio_boundary_delta)
            exact_range(base, "startup_end_skew_micros", startup_end_skew)
            exact_range(base, "continuation_start_skew_micros", continuation_start_skew)
            assert base["maximum_absolute_frame_count_delta"] == maximum_frame_delta
            assert base["boundary_frame_tolerance_used"] is False
            assert base["sequence_stable"] is True
            assert base["cadence_stable"] is True
            assert base["av_sync_stable"] is True
            assert base["all_preserved"] is True
            assert base["stable"] is True

            exact_range(summary, "startup_reordered_packet_count", startup_reordered)
            exact_range(summary, "continuation_reordered_packet_count", continuation_reordered)
            exact_range(summary, "startup_max_reorder_depth", startup_depth)
            exact_range(summary, "continuation_max_reorder_depth", continuation_depth)
            exact_range(summary, "startup_max_composition_offset_micros", startup_composition)
            exact_range(summary, "continuation_max_composition_offset_micros", continuation_composition)
            assert summary["packet_order_stable"] is True
            assert summary["perceptual_sequence_stable"] is True
            assert summary["strict_dts"] is True
            assert summary["reorder_observed"] is True
            assert summary["stable"] is True

            for run in candidate["runs"]:
                verify_packet_order(
                    run["startup_packet_order"], output_startup,
                    startup_reordered, startup_inversions, startup_depth, startup_composition,
                )
                verify_packet_order(
                    run["continuation_packet_order"], output_continuation,
                    continuation_reordered, continuation_inversions, continuation_depth, continuation_composition,
                )
                assert run["startup_perceptual_sequence"]["frame_count"] == output_startup
                assert run["continuation_perceptual_sequence"]["frame_count"] == output_continuation
                assert len(run["startup_perceptual_sequence"]["frame_hashes"]) == output_startup
                assert len(run["continuation_perceptual_sequence"]["frame_hashes"]) == output_continuation
                assert run["base"]["boundary"]["seamless_allowed"] is False
                assert run["base"]["boundary"]["discontinuity_required"] is True
                assert run["base"]["av_sync"]["seamless_allowed"] is False
                assert run["base"]["av_sync"]["discontinuity_required"] is True

        comparison = case["comparison"]
        base_comparison = comparison["base"]
        assert base_comparison["candidate_a_id"] == CANDIDATES[0]
        assert base_comparison["candidate_b_id"] == CANDIDATES[1]
        assert base_comparison["startup_sequence_equivalent"] is exact_startup_pixels
        assert base_comparison["continuation_sequence_equivalent"] is exact_continuation_pixels
        assert base_comparison["frame_mapping_equivalent"] is True
        assert base_comparison["cadence_equivalent"] is True
        assert base_comparison["max_av_sync_metric_difference_micros"] == 0
        assert base_comparison["av_sync_within_tolerance"] is True
        assert base_comparison["equivalent"] is (exact_startup_pixels and exact_continuation_pixels)
        assert comparison["semantic_base_equivalent"] is True
        assert comparison["startup_packet_order_equivalent"] is True
        assert comparison["continuation_packet_order_equivalent"] is True
        assert comparison["equivalent"] is True

        for window, frame_count, exact_pixels in (
            ("startup", output_startup, exact_startup_pixels),
            ("continuation", output_continuation, exact_continuation_pixels),
        ):
            perceptual = comparison[f"{window}_perceptual_comparison"]
            assert perceptual["frame_count"] == frame_count
            assert perceptual["exact_hash_match_count"] == frame_count
            assert perceptual["max_hamming_distance"] == 0
            assert perceptual["total_hamming_distance"] == 0
            assert perceptual["mean_hamming_milli"] == 0
            assert perceptual["equivalent"] is True
            if not exact_pixels:
                exact_pixel_mismatch_windows.append(f"{case_id}:{window}")

    assert exact_pixel_mismatch_windows == [
        "reorder-cfr-30000-1001-b3-origin-zero-v1:startup",
        "reorder-cfr-30-b3-origin-positive-5s-v1:startup",
        "reorder-cfr-30-b3-origin-positive-5s-v1:continuation",
        "reorder-cfr-30-b3-origin-negative-2s-v1:startup",
        "reorder-cfr-30-b3-origin-negative-2s-v1:continuation",
        "reorder-cfr-30-b3-long-gop-origin-zero-v1:startup",
        "reorder-cfr-30-b3-long-gop-origin-zero-v1:continuation",
    ]
    print(json.dumps({
        "contract_hash": report["contract_hash"],
        "exact_cases": len(EXPECTED),
        "exact_pixel_mismatch_windows": exact_pixel_mismatch_windows,
        "perceptual_max_hamming_observed": 0,
        "toolchain": "ffmpeg-6.1.1-3ubuntu5",
    }, sort_keys=True))


if __name__ == "__main__":
    main()
