#!/usr/bin/env python3
"""Verify the exact FFmpeg 6.1 VFR layer-isolation evidence baseline."""

from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path

EXPECTED_VARIANTS = [
    {
        "id": "production-hls-v1",
        "layer": "production_baseline",
        "container": "hls-mpegts",
        "fps_mode": "passthrough",
        "encoder_time_base": "auto",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 60,
        "histogram": [(1, 11, 60), (3749, 41656, 60), (3750, 41667, 179)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "changed",
        "sequence_reference": "none",
        "sequence_matches": False,
    },
    {
        "id": "fps-mode-vfr-hls-v1",
        "layer": "output_sync_policy",
        "container": "hls-mpegts",
        "fps_mode": "vfr",
        "encoder_time_base": "auto",
        "frames": 241,
        "delta": -59,
        "mapping_status": "drop_projection",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 0,
        "histogram": [(3750, 41667, 240)],
        "unique_frames": 241,
        "adjacent_duplicates": 0,
        "classification": "changed",
        "sequence_reference": "baseline",
        "sequence_matches": False,
    },
    {
        "id": "fps-mode-cfr-hls-v1",
        "layer": "output_sync_policy",
        "container": "hls-mpegts",
        "fps_mode": "cfr",
        "encoder_time_base": "auto",
        "frames": 962,
        "delta": 662,
        "mapping_status": "duplicate_projection",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 0,
        "histogram": [(3750, 41667, 961)],
        "unique_frames": 260,
        "adjacent_duplicates": 645,
        "classification": "changed",
        "sequence_reference": "baseline",
        "sequence_matches": False,
    },
    {
        "id": "encoder-time-base-avtb-hls-v1",
        "layer": "encoder_time_base",
        "container": "hls-mpegts",
        "fps_mode": "passthrough",
        "encoder_time_base": "1/1000000",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 0,
        "histogram": [(3000, 33333, 299)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "preserved",
        "sequence_reference": "baseline",
        "sequence_matches": True,
    },
    {
        "id": "encoder-time-base-90k-hls-v1",
        "layer": "encoder_time_base",
        "container": "hls-mpegts",
        "fps_mode": "passthrough",
        "encoder_time_base": "1/90000",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 0,
        "histogram": [(3000, 33333, 299)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "preserved",
        "sequence_reference": "baseline",
        "sequence_matches": True,
    },
    {
        "id": "matroska-default-v1",
        "layer": "encoded_container",
        "container": "matroska",
        "fps_mode": "passthrough",
        "encoder_time_base": "auto",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 60,
        "non_monotonic_pts": 0,
        "near_zero": 0,
        "histogram": [(41, 41000, 80), (42, 42000, 159)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "changed",
        "sequence_reference": "baseline",
        "sequence_matches": True,
    },
    {
        "id": "matroska-avtb-v1",
        "layer": "encoder_time_base",
        "container": "matroska",
        "fps_mode": "passthrough",
        "encoder_time_base": "1/1000000",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 0,
        "histogram": [(33, 33000, 199), (34, 34000, 100)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "preserved",
        "sequence_reference": "baseline",
        "sequence_matches": True,
    },
    {
        "id": "matroska-remux-mpegts-v1",
        "layer": "mpegts_muxer",
        "container": "mpegts",
        "fps_mode": "not_applicable",
        "encoder_time_base": "copy",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 60,
        "histogram": [(1, 11, 60), (3689, 40989, 20), (3690, 41000, 60), (3779, 41989, 40), (3780, 42000, 119)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "changed",
        "sequence_reference": "parent",
        "sequence_matches": True,
        "parent": "matroska-default-v1",
    },
    {
        "id": "matroska-remux-hls-v1",
        "layer": "hls_muxer",
        "container": "hls-mpegts",
        "fps_mode": "not_applicable",
        "encoder_time_base": "copy",
        "frames": 300,
        "delta": 0,
        "mapping_status": "aligned",
        "duplicate_pts": 0,
        "non_monotonic_pts": 0,
        "near_zero": 60,
        "histogram": [(1, 11, 60), (3689, 40989, 20), (3690, 41000, 60), (3779, 41989, 40), (3780, 42000, 119)],
        "unique_frames": 300,
        "adjacent_duplicates": 0,
        "classification": "changed",
        "sequence_reference": "parent",
        "sequence_matches": True,
        "parent": "matroska-default-v1",
    },
]


def canonical_hash(value: object) -> str:
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def histogram(timeline: dict) -> list[tuple[int, int, int]]:
    return [
        (bucket["delta_ticks"], bucket["delta_micros"], bucket["count"])
        for bucket in timeline["delta_histogram"]
    ]


def validate_sha256(value: str) -> None:
    assert len(value) == 64
    int(value, 16)


def validate_timeline(timeline: dict, expected_kind: str) -> None:
    assert timeline["kind"] == expected_kind
    assert timeline["frame_count"] >= 2
    assert timeline["window_end_micros"] > timeline["window_start_micros"]
    assert timeline["duplicate_pts_count"] >= 0
    assert timeline["non_monotonic_pts_count"] >= 0

    buckets = timeline["delta_histogram"]
    assert buckets
    ticks = [item["delta_ticks"] for item in buckets]
    assert ticks == sorted(ticks)
    assert len(ticks) == len(set(ticks))
    assert all(item["delta_ticks"] > 0 and item["count"] > 0 for item in buckets)

    positive_count = (
        timeline["frame_count"]
        - 1
        - timeline["duplicate_pts_count"]
        - timeline["non_monotonic_pts_count"]
    )
    assert sum(item["count"] for item in buckets) == positive_count
    assert timeline["distinct_deltas"] == len(buckets)
    assert timeline["min_delta_ticks"] == buckets[0]["delta_ticks"]
    assert timeline["max_delta_ticks"] == buckets[-1]["delta_ticks"]
    assert timeline["min_delta_micros"] == buckets[0]["delta_micros"]
    assert timeline["max_delta_micros"] == buckets[-1]["delta_micros"]
    assert timeline["duration_spread_micros"] == timeline["max_delta_micros"] - timeline["min_delta_micros"]
    assert timeline["near_zero_delta_count"] == sum(
        item["count"] for item in buckets if item["delta_micros"] < 1000
    )

    dominant = sorted(buckets, key=lambda item: (-item["count"], item["delta_ticks"]))[0]
    assert timeline["dominant_delta_ticks"] == dominant["delta_ticks"]
    assert timeline["dominant_delta_micros"] == dominant["delta_micros"]
    assert timeline["dominant_delta_count"] == dominant["count"]

    threshold = max(2, (positive_count + 99) // 100)
    significant = [item for item in buckets if item["count"] >= threshold]
    outliers = [item for item in buckets if item["count"] < threshold]
    assert timeline["significant_bucket_minimum_count"] == threshold
    assert timeline["significant_delta_count"] == len(significant)
    assert timeline["outlier_delta_count"] == sum(item["count"] for item in outliers)
    material = (
        len(significant) >= 2
        and significant[-1]["delta_micros"] - significant[0]["delta_micros"] >= 5000
    )
    assert timeline["material_variable_duration"] is material


def validate_mapping(mapping: dict, input_frames: int, output_frames: int) -> None:
    delta = output_frames - input_frames
    assert mapping["input_frames"] == input_frames
    assert mapping["output_frames"] == output_frames
    assert mapping["frame_count_delta"] == delta
    assert mapping["count_tolerance"] == 1
    assert mapping["projected_duplicate_frames"] == max(delta, 0)
    assert mapping["projected_dropped_frames"] == max(-delta, 0)
    if delta > 1:
        expected_status = "duplicate_projection"
    elif delta < -1:
        expected_status = "drop_projection"
    elif delta:
        expected_status = "within_tolerance"
    else:
        expected_status = "aligned"
    assert mapping["status"] == expected_status


def main() -> int:
    if len(sys.argv) != 2:
        raise SystemExit("usage: verify_vfr_isolation.py <matrix.json>")
    path = Path(sys.argv[1])
    report = json.loads(path.read_text())

    assert report["schema_version"] == "ffmpeg-vfr-layer-isolation-matrix-v1"
    assert report["contract_version"] == "vfr-layer-isolation-evidence-v1"
    validate_sha256(report["contract_hash"])
    evidence = report["evidence"]
    assert canonical_hash(evidence) == report["contract_hash"]
    assert evidence["schema_version"] == report["contract_version"]
    assert evidence["case_id"] == "source-vfr-24-30-origin-zero-v1"
    assert evidence["window_start_micros"] == 30_000_000
    assert evidence["window_end_micros"] == 40_000_000
    assert evidence["baseline_output_cadence_version"] == "hls-output-cadence-evidence-v1"
    validate_sha256(evidence["baseline_output_cadence_hash"])
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True
    assert evidence["ffmpeg_version"].startswith("ffmpeg version 6.1.1-3ubuntu5")
    assert evidence["ffprobe_version"].startswith("ffprobe version 6.1.1-3ubuntu5")

    source = evidence["source_timeline"]
    validate_timeline(source, "source_continuation")
    assert source["frame_count"] == 300
    assert source["duplicate_pts_count"] == 0
    assert source["non_monotonic_pts_count"] == 0
    assert source["near_zero_delta_count"] == 0
    assert histogram(source) == [(33333, 33333, 199), (33334, 33334, 100)]

    variants = evidence["variants"]
    assert [item["spec"]["id"] for item in variants] == [item["id"] for item in EXPECTED_VARIANTS]
    assert len({item["command_hash"] for item in variants}) == len(variants)
    by_id = {item["spec"]["id"]: item for item in variants}
    baseline_sequence = by_id["production-hls-v1"]["fingerprint"]["sequence_sha256"]
    validate_sha256(baseline_sequence)

    for variant, expected in zip(variants, EXPECTED_VARIANTS):
        spec = variant["spec"]
        timeline = variant["timeline"]
        mapping = variant["mapping"]
        fingerprint = variant["fingerprint"]

        assert spec["id"] == expected["id"]
        assert spec["layer"] == expected["layer"]
        assert spec["container"] == expected["container"]
        assert spec["fps_mode"] == expected["fps_mode"]
        assert spec["encoder_time_base"] == expected["encoder_time_base"]
        assert spec["copy_only"] is ("parent" in expected)
        if "parent" in expected:
            assert spec["parent_variant_id"] == expected["parent"]
        else:
            assert "parent_variant_id" not in spec

        validate_sha256(variant["command_hash"])
        validate_timeline(timeline, "isolation_" + expected["id"])
        assert timeline["frame_count"] == expected["frames"]
        assert timeline["duplicate_pts_count"] == expected["duplicate_pts"]
        assert timeline["non_monotonic_pts_count"] == expected["non_monotonic_pts"]
        assert timeline["near_zero_delta_count"] == expected["near_zero"]
        assert histogram(timeline) == expected["histogram"]

        validate_mapping(mapping, source["frame_count"], expected["frames"])
        assert mapping["frame_count_delta"] == expected["delta"]
        assert mapping["status"] == expected["mapping_status"]

        assert fingerprint["frame_count"] == expected["frames"]
        assert fingerprint["unique_frame_count"] == expected["unique_frames"]
        assert fingerprint["adjacent_duplicate_count"] == expected["adjacent_duplicates"]
        for key in ("sequence_sha256", "first_frame_sha256", "last_frame_sha256"):
            validate_sha256(fingerprint[key])

        assert variant["cadence_classification"] == expected["classification"]
        assert variant["sequence_reference"] == expected["sequence_reference"]
        assert variant["sequence_matches_reference"] is expected["sequence_matches"]
        if expected["sequence_reference"] == "baseline":
            assert variant["sequence_reference_variant_id"] == "production-hls-v1"
            assert (fingerprint["sequence_sha256"] == baseline_sequence) is expected["sequence_matches"]
        elif expected["sequence_reference"] == "parent":
            parent = by_id[expected["parent"]]
            assert variant["sequence_reference_variant_id"] == expected["parent"]
            assert fingerprint["sequence_sha256"] == parent["fingerprint"]["sequence_sha256"]
            assert fingerprint["frame_count"] == parent["fingerprint"]["frame_count"]
        else:
            assert "sequence_reference_variant_id" not in variant

    assert by_id["encoder-time-base-avtb-hls-v1"]["fingerprint"]["sequence_sha256"] == baseline_sequence
    assert by_id["encoder-time-base-90k-hls-v1"]["fingerprint"]["sequence_sha256"] == baseline_sequence
    assert by_id["matroska-default-v1"]["fingerprint"]["sequence_sha256"] == baseline_sequence
    assert by_id["matroska-avtb-v1"]["fingerprint"]["sequence_sha256"] == baseline_sequence

    print(json.dumps({
        "contract_hash": report["contract_hash"],
        "source_frames": source["frame_count"],
        "variants": {
            item["spec"]["id"]: {
                "frames": item["timeline"]["frame_count"],
                "duplicate_pts": item["timeline"]["duplicate_pts_count"],
                "near_zero": item["timeline"]["near_zero_delta_count"],
                "adjacent_duplicates": item["fingerprint"]["adjacent_duplicate_count"],
                "classification": item["cadence_classification"],
            }
            for item in variants
        },
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
