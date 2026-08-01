#!/usr/bin/env python3

import hashlib
import json
import sys
from pathlib import Path

SCHEMA = "ffmpeg-output-cadence-matrix-v1"
CONTRACT = "hls-output-cadence-evidence-v1"

EXPECTED = [
    ("source-cfr-24-origin-zero-v1", "cfr"),
    ("source-cfr-25-origin-zero-v1", "cfr"),
    ("source-cfr-30000-1001-origin-zero-v1", "cfr"),
    ("source-vfr-24-30-origin-zero-v1", "vfr"),
    ("source-cfr-30-origin-positive-5s-v1", "cfr"),
    ("source-cfr-30-origin-negative-2s-v1", "cfr"),
]

# Histogram tuples are (delta_ticks, delta_micros, count).
BASELINE = {
    "source-cfr-24-origin-zero-v1": {
        "frames": (960, 720, 240),
        "source_full": [(41666, 41666, 320), (41667, 41667, 639)],
        "source_startup": [(41666, 41666, 240), (41667, 41667, 479)],
        "source_continuation": [(41666, 41666, 80), (41667, 41667, 159)],
        "output_startup": [(3750, 41667, 719)],
        "output_continuation": [(3750, 41667, 239)],
        "preservation": "preserved_exact",
    },
    "source-cfr-25-origin-zero-v1": {
        "frames": (1000, 750, 250),
        "source_full": [(40000, 40000, 999)],
        "source_startup": [(40000, 40000, 749)],
        "source_continuation": [(40000, 40000, 249)],
        "output_startup": [(3600, 40000, 749)],
        "output_continuation": [(3600, 40000, 249)],
        "preservation": "preserved_exact",
    },
    "source-cfr-30000-1001-origin-zero-v1": {
        "frames": (1199, 900, 299),
        "source_full": [(33366, 33366, 399), (33367, 33367, 799)],
        "source_startup": [(33366, 33366, 300), (33367, 33367, 599)],
        "source_continuation": [(33366, 33366, 99), (33367, 33367, 199)],
        "output_startup": [(3003, 33367, 899)],
        "output_continuation": [(3003, 33367, 298)],
        "preservation": "preserved_exact",
    },
    "source-vfr-24-30-origin-zero-v1": {
        "frames": (1080, 780, 300),
        "source_full": [
            (33333, 33333, 399),
            (33334, 33334, 200),
            (41666, 41666, 160),
            (41667, 41667, 320),
        ],
        "source_startup": [
            (33333, 33333, 199),
            (33334, 33334, 100),
            (41666, 41666, 160),
            (41667, 41667, 320),
        ],
        "source_continuation": [(33333, 33333, 199), (33334, 33334, 100)],
        "output_startup": [(1, 11, 60), (3749, 41656, 60), (3750, 41667, 659)],
        "output_continuation": [(1, 11, 60), (3749, 41656, 60), (3750, 41667, 179)],
        "preservation": "changed",
    },
    "source-cfr-30-origin-positive-5s-v1": {
        "frames": (1200, 900, 300),
        "source_full": [(33333, 33333, 799), (33334, 33334, 400)],
        "source_startup": [(33333, 33333, 599), (33334, 33334, 300)],
        "source_continuation": [(33333, 33333, 199), (33334, 33334, 100)],
        "output_startup": [(3000, 33333, 899)],
        "output_continuation": [(3000, 33333, 299)],
        "preservation": "preserved_exact",
    },
    "source-cfr-30-origin-negative-2s-v1": {
        "frames": (1200, 900, 300),
        "source_full": [(33333, 33333, 799), (33334, 33334, 400)],
        "source_startup": [(33333, 33333, 599), (33334, 33334, 300)],
        "source_continuation": [(33333, 33333, 199), (33334, 33334, 100)],
        "output_startup": [(3000, 33333, 899)],
        "output_continuation": [(3000, 33333, 299)],
        "preservation": "preserved_exact",
    },
}


def canonical_hash(value):
    canonical = json.dumps(value, ensure_ascii=False, separators=(",", ":"))
    return hashlib.sha256(canonical.encode()).hexdigest()


def histogram(timeline):
    return [
        (bucket["delta_ticks"], bucket["delta_micros"], bucket["count"])
        for bucket in timeline["delta_histogram"]
    ]


def validate_timeline(timeline):
    buckets = timeline["delta_histogram"]
    assert buckets
    ticks = [bucket["delta_ticks"] for bucket in buckets]
    assert ticks == sorted(ticks)
    assert len(set(ticks)) == len(ticks)

    positive = (
        timeline["frame_count"]
        - 1
        - timeline["duplicate_pts_count"]
        - timeline["non_monotonic_pts_count"]
    )
    assert sum(bucket["count"] for bucket in buckets) == positive
    assert timeline["distinct_deltas"] == len(buckets)
    assert timeline["min_delta_ticks"] == buckets[0]["delta_ticks"]
    assert timeline["max_delta_ticks"] == buckets[-1]["delta_ticks"]
    assert timeline["min_delta_micros"] == buckets[0]["delta_micros"]
    assert timeline["max_delta_micros"] == buckets[-1]["delta_micros"]
    assert (
        timeline["duration_spread_micros"]
        == timeline["max_delta_micros"] - timeline["min_delta_micros"]
    )

    threshold = max(2, (positive + 99) // 100)
    assert timeline["significant_bucket_minimum_count"] == threshold
    significant = [bucket for bucket in buckets if bucket["count"] >= threshold]
    outliers = [bucket for bucket in buckets if bucket["count"] < threshold]
    assert timeline["significant_delta_count"] == len(significant)
    assert timeline["outlier_delta_count"] == sum(bucket["count"] for bucket in outliers)
    assert timeline["near_zero_delta_count"] == sum(
        bucket["count"] for bucket in buckets if bucket["delta_micros"] < 1000
    )

    dominant = sorted(buckets, key=lambda bucket: (-bucket["count"], bucket["delta_ticks"]))[0]
    assert timeline["dominant_delta_ticks"] == dominant["delta_ticks"]
    assert timeline["dominant_delta_micros"] == dominant["delta_micros"]
    assert timeline["dominant_delta_count"] == dominant["count"]

    material_variable = (
        len(significant) >= 2
        and significant[-1]["delta_micros"] - significant[0]["delta_micros"] >= 5000
    )
    assert timeline["material_variable_duration"] is material_variable
    assert timeline["duplicate_pts_count"] == 0
    assert timeline["non_monotonic_pts_count"] == 0


def verify_case(item, case_id, mode):
    baseline = BASELINE[case_id]
    evidence = item["evidence"]
    source_report = item["source_origin"]
    source_evidence = source_report["evidence"]

    assert item["contract_version"] == CONTRACT
    assert evidence["schema_version"] == CONTRACT
    assert evidence["case_id"] == case_id
    assert evidence["source_mode"] == mode
    assert canonical_hash(evidence) == item["contract_hash"]
    assert canonical_hash(source_evidence) == source_report["contract_hash"]
    assert evidence["source_origin_version"] == source_report["contract_version"]
    assert evidence["source_origin_hash"] == source_report["contract_hash"]
    assert evidence["boundary_evidence_hash"] == source_report["boundary_hash"]
    assert evidence["av_sync_evidence_hash"] == source_report["av_sync_hash"]

    source = evidence["source_timeline"]
    source_startup = evidence["source_startup_timeline"]
    source_continuation = evidence["source_continuation_timeline"]
    output_startup = evidence["startup_timeline"]
    output_continuation = evidence["continuation_timeline"]
    startup_mapping = evidence["startup_mapping"]
    continuation_mapping = evidence["continuation_mapping"]

    for timeline in (
        source,
        source_startup,
        source_continuation,
        output_startup,
        output_continuation,
    ):
        validate_timeline(timeline)

    total_frames, startup_frames, continuation_frames = baseline["frames"]
    assert source["frame_count"] == total_frames
    assert source_startup["frame_count"] == startup_frames
    assert source_continuation["frame_count"] == continuation_frames
    assert source["frame_count"] == source_startup["frame_count"] + source_continuation["frame_count"]

    assert histogram(source) == baseline["source_full"]
    assert histogram(source_startup) == baseline["source_startup"]
    assert histogram(source_continuation) == baseline["source_continuation"]
    assert histogram(output_startup) == baseline["output_startup"]
    assert histogram(output_continuation) == baseline["output_continuation"]

    for mapping, expected_frames in (
        (startup_mapping, startup_frames),
        (continuation_mapping, continuation_frames),
    ):
        assert mapping["input_frames"] == expected_frames
        assert mapping["output_frames"] == expected_frames
        assert mapping["frame_count_delta"] == 0
        assert mapping["projected_duplicate_frames"] == 0
        assert mapping["projected_dropped_frames"] == 0
        assert mapping["count_tolerance"] == 1
        assert mapping["status"] == "aligned"

    is_vfr = mode == "vfr"
    assert source["material_variable_duration"] is is_vfr
    assert source_startup["material_variable_duration"] is is_vfr
    assert source_continuation["material_variable_duration"] is False
    assert evidence["expected_startup_material_variable"] is is_vfr
    assert evidence["expected_continuation_material_variable"] is False

    if is_vfr:
        assert output_startup["material_variable_duration"] is True
        assert output_continuation["material_variable_duration"] is True
        assert source_startup["near_zero_delta_count"] == 0
        assert source_continuation["near_zero_delta_count"] == 0
        assert output_startup["near_zero_delta_count"] == 60
        assert output_continuation["near_zero_delta_count"] == 60
    else:
        assert output_startup["material_variable_duration"] is False
        assert output_continuation["material_variable_duration"] is False
        assert source_startup["near_zero_delta_count"] == 0
        assert source_continuation["near_zero_delta_count"] == 0
        assert output_startup["near_zero_delta_count"] == 0
        assert output_continuation["near_zero_delta_count"] == 0

    assert output_startup["outlier_delta_count"] == 0
    assert output_continuation["outlier_delta_count"] == 0
    assert evidence["preservation_status"] == baseline["preservation"]
    assert evidence["content_duplicate_classification"] == "not_measured"
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True

    return {
        "source_frames": total_frames,
        "startup_frames": startup_frames,
        "continuation_frames": continuation_frames,
        "source_continuation_dominant_us": source_continuation["dominant_delta_micros"],
        "output_continuation_dominant_us": output_continuation["dominant_delta_micros"],
        "startup_near_zero": output_startup["near_zero_delta_count"],
        "continuation_near_zero": output_continuation["near_zero_delta_count"],
        "preservation": evidence["preservation_status"],
    }


def main():
    if len(sys.argv) != 2:
        raise SystemExit("usage: verify_output_cadence.py <output-cadence-matrix-v1.json>")

    path = Path(sys.argv[1])
    matrix = json.loads(path.read_text())
    assert matrix["schema_version"] == SCHEMA
    assert [item["case"]["id"] for item in matrix["cases"]] == [item[0] for item in EXPECTED]

    hashes = set()
    toolchain = None
    summary = {}
    for item, (case_id, mode) in zip(matrix["cases"], EXPECTED):
        summary[case_id] = verify_case(item, case_id, mode)
        hashes.add(item["contract_hash"])
        evidence = item["evidence"]
        current_toolchain = (evidence["ffmpeg_version"], evidence["ffprobe_version"])
        toolchain = current_toolchain if toolchain is None else toolchain
        assert current_toolchain == toolchain

    assert len(hashes) == len(EXPECTED)
    print(json.dumps({"toolchain": toolchain, "summary": summary}, sort_keys=True))


if __name__ == "__main__":
    main()
