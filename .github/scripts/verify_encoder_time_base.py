#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path

EXPECTED_CASES = [
    "candidate-cfr-24000-1001-origin-zero-v1",
    "candidate-cfr-24-origin-zero-v1",
    "candidate-cfr-25-origin-zero-v1",
    "candidate-cfr-30000-1001-origin-zero-v1",
    "candidate-cfr-30-origin-zero-v1",
    "candidate-cfr-50-origin-zero-v1",
    "candidate-cfr-60000-1001-origin-zero-v1",
    "candidate-vfr-24-30-origin-zero-v1",
    "candidate-vfr-25-30-origin-zero-v1",
    "candidate-vfr-30000-1001-60000-1001-origin-zero-v1",
    "candidate-cfr-30-origin-positive-5s-v1",
    "candidate-cfr-30-origin-negative-2s-v1",
]
EXPECTED_CANDIDATES = [
    ("encoder-time-base-avtb-v1", "1/1000000"),
    ("encoder-time-base-90k-v1", "1/90000"),
]
BOUNDARY_FRAME_TOLERANCE = 1


def sha256_json(value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def validate_hash(value, label):
    assert isinstance(value, str) and len(value) == 64, f"{label} is not SHA-256"
    int(value, 16)


def validate_timeline(timeline, expected_kind):
    assert timeline["kind"] == expected_kind
    assert timeline["frame_count"] >= 2
    assert timeline["duplicate_pts_count"] == 0
    assert timeline["non_monotonic_pts_count"] == 0
    assert timeline["near_zero_delta_count"] == 0
    histogram = timeline["delta_histogram"]
    assert histogram
    assert sum(bucket["count"] for bucket in histogram) == timeline["frame_count"] - 1
    ticks = [bucket["delta_ticks"] for bucket in histogram]
    assert ticks == sorted(set(ticks))
    assert all(bucket["delta_ticks"] > 0 and bucket["count"] > 0 for bucket in histogram)
    dominant = min(
        (bucket for bucket in histogram),
        key=lambda bucket: (-bucket["count"], bucket["delta_ticks"]),
    )
    assert timeline["dominant_delta_ticks"] == dominant["delta_ticks"]
    assert timeline["dominant_delta_micros"] == dominant["delta_micros"]
    assert timeline["dominant_delta_count"] == dominant["count"]


def validate_fingerprint(fingerprint, frame_count):
    assert fingerprint["frame_count"] == frame_count
    assert 0 < fingerprint["unique_frame_count"] <= frame_count
    assert fingerprint["adjacent_duplicate_count"] == 0
    for key in ("sequence_sha256", "first_frame_sha256", "last_frame_sha256"):
        validate_hash(fingerprint[key], f"fingerprint {key}")


def validate_mapping(mapping, source_frames, output_frames):
    assert mapping["input_frames"] == source_frames
    assert mapping["output_frames"] == output_frames
    delta = output_frames - source_frames
    assert mapping["frame_count_delta"] == delta
    assert mapping["count_tolerance"] == BOUNDARY_FRAME_TOLERANCE
    assert mapping["projected_duplicate_frames"] == max(delta, 0)
    assert mapping["projected_dropped_frames"] == max(-delta, 0)
    assert abs(delta) <= BOUNDARY_FRAME_TOLERANCE
    expected_status = "aligned" if delta == 0 else "within_tolerance"
    assert mapping["status"] == expected_status
    return abs(delta)


def validate_range(metric, tolerance=0):
    assert metric["min"] <= metric["max"]
    assert metric["span"] == metric["max"] - metric["min"]
    assert metric["span"] <= tolerance


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()

    report = json.loads(args.report.read_text())
    assert report["schema_version"] == "ffmpeg-encoder-time-base-candidate-matrix-v1"
    assert report["contract_version"] == "encoder-time-base-candidate-evidence-v1"
    validate_hash(report["contract_hash"], "contract hash")
    evidence = report["evidence"]
    assert sha256_json(evidence) == report["contract_hash"]
    assert evidence["schema_version"] == report["contract_version"]
    assert evidence["repeat_count"] == 3
    assert evidence["variance_tolerance_micros"] == 1
    assert evidence["cross_candidate_tolerance_micros"] == 1000
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True
    assert [case["case"]["id"] for case in evidence["cases"]] == EXPECTED_CASES

    for case in evidence["cases"]:
        spec = case["case"]
        source_startup = case["source_startup_timeline"]
        source_continuation = case["source_continuation_timeline"]
        validate_timeline(source_startup, "source_startup")
        validate_timeline(source_continuation, "source_continuation")
        assert source_startup["window_start_micros"] == spec["source_offset_micros"]
        assert source_startup["window_end_micros"] == spec["source_offset_micros"] + spec["expected_boundary_micros"]
        assert source_continuation["window_start_micros"] == spec["source_offset_micros"] + spec["expected_boundary_micros"]
        assert source_continuation["window_end_micros"] == spec["source_offset_micros"] + spec["duration_micros"]

        candidates = case["candidates"]
        assert [(item["spec"]["id"], item["spec"]["encoder_time_base"]) for item in candidates] == EXPECTED_CANDIDATES
        for candidate in candidates:
            candidate_id = candidate["spec"]["id"]
            runs = candidate["runs"]
            assert [run["ordinal"] for run in runs] == [1, 2, 3]
            maximum_absolute_frame_delta = 0
            for run in runs:
                ordinal = run["ordinal"]
                startup_kind = f"candidate_{spec['id']}_{candidate_id}_run_{ordinal:02d}_startup"
                continuation_kind = f"candidate_{spec['id']}_{candidate_id}_run_{ordinal:02d}_continuation"
                validate_hash(run["startup_command_hash"], "startup command")
                validate_hash(run["continuation_command_hash"], "continuation command")
                validate_timeline(run["startup_timeline"], startup_kind)
                validate_timeline(run["continuation_timeline"], continuation_kind)
                maximum_absolute_frame_delta = max(
                    maximum_absolute_frame_delta,
                    validate_mapping(run["startup_mapping"], source_startup["frame_count"], run["startup_timeline"]["frame_count"]),
                    validate_mapping(run["continuation_mapping"], source_continuation["frame_count"], run["continuation_timeline"]["frame_count"]),
                )
                validate_fingerprint(run["startup_fingerprint"], run["startup_timeline"]["frame_count"])
                validate_fingerprint(run["continuation_fingerprint"], run["continuation_timeline"]["frame_count"])
                validate_hash(run["boundary_hash"], "boundary hash")
                validate_hash(run["av_sync_hash"], "A/V sync hash")
                assert run["boundary"]["seamless_allowed"] is False
                assert run["boundary"]["discontinuity_required"] is True
                assert run["av_sync"]["seamless_allowed"] is False
                assert run["av_sync"]["discontinuity_required"] is True

            summary = candidate["summary"]
            assert summary["repeat_count"] == 3
            assert summary["maximum_absolute_frame_count_delta"] == maximum_absolute_frame_delta
            assert summary["boundary_frame_tolerance_used"] is (maximum_absolute_frame_delta > 0)
            assert summary["sequence_stable"] is True
            assert summary["cadence_stable"] is True
            assert summary["av_sync_stable"] is True
            assert summary["all_preserved"] is True
            assert summary["stable"] is True
            for key in (
                "startup_frame_count",
                "continuation_frame_count",
                "startup_dominant_delta_micros",
                "continuation_dominant_delta_micros",
                "startup_near_zero_delta_count",
                "continuation_near_zero_delta_count",
                "startup_duplicate_pts_count",
                "continuation_duplicate_pts_count",
                "startup_adjacent_duplicate_frame_count",
                "continuation_adjacent_duplicate_frame_count",
            ):
                validate_range(summary[key], 0)
            for key in (
                "video_boundary_delta_micros",
                "audio_boundary_delta_micros",
                "startup_end_skew_micros",
                "continuation_start_skew_micros",
                "boundary_delta_skew_micros",
                "skew_transition_micros",
                "projection_residual_micros",
            ):
                validate_range(summary[key], 1)

        comparison = case["comparison"]
        assert comparison["candidate_a_id"] == EXPECTED_CANDIDATES[0][0]
        assert comparison["candidate_b_id"] == EXPECTED_CANDIDATES[1][0]
        assert comparison["startup_sequence_equivalent"] is True
        assert comparison["continuation_sequence_equivalent"] is True
        assert comparison["frame_mapping_equivalent"] is True
        assert comparison["cadence_equivalent"] is True
        assert comparison["av_sync_within_tolerance"] is True
        assert comparison["max_av_sync_metric_difference_micros"] <= 1000
        assert comparison["equivalent"] is True

    print(json.dumps({
        "cases": len(evidence["cases"]),
        "candidates": len(EXPECTED_CANDIDATES),
        "repeats": evidence["repeat_count"],
        "contract_hash": report["contract_hash"],
    }, sort_keys=True))


if __name__ == "__main__":
    main()
