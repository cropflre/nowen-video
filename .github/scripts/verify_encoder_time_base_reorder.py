#!/usr/bin/env python3
import hashlib
import json
import sys
from pathlib import Path

EXPECTED_CASES = [
    "reorder-cfr-24-b2-origin-zero-v1",
    "reorder-cfr-30000-1001-b3-origin-zero-v1",
    "reorder-vfr-24-30-b3-origin-zero-v1",
    "reorder-cfr-30-b3-origin-positive-5s-v1",
    "reorder-cfr-30-b3-origin-negative-2s-v1",
    "reorder-cfr-30-b3-long-gop-origin-zero-v1",
]
EXPECTED_CANDIDATES = [
    "encoder-time-base-avtb-v1",
    "encoder-time-base-90k-v1",
]
PERCEPTUAL_HASH_HEX_LENGTH = 32
PERCEPTUAL_MAX_HAMMING_DISTANCE = 8


def fail(message: str) -> None:
    raise AssertionError(message)


def verify_packet_order(order: dict, frame_count: int) -> None:
    if order["packet_count"] != frame_count:
        fail("packet count differs from cadence frame count")
    if order["dts_non_monotonic_count"] != 0 or order["dts_duplicate_count"] != 0:
        fail("DTS is not strictly monotonic")
    if order["reordered_packet_count"] <= 0:
        fail("B-frame reordering was not observed")
    if order["adjacent_pts_inversion_count"] <= 0:
        fail("decode-order PTS inversion was not observed")
    if order["max_presentation_reorder_depth"] <= 0:
        fail("presentation reorder depth was not observed")
    if order["reordered_packet_count"] != order["pts_before_dts_count"] + order["pts_after_dts_count"]:
        fail("composition offset counters are inconsistent")
    if order["reordered_packet_count"] + order["pts_equal_dts_count"] != order["packet_count"]:
        fail("composition offset total is inconsistent")
    dts_total = sum(bucket["count"] for bucket in order["dts_delta_histogram"])
    if dts_total != order["packet_count"] - 1:
        fail("DTS histogram count is inconsistent")
    offset_total = sum(bucket["count"] for bucket in order["composition_offset_histogram"])
    if offset_total != order["packet_count"]:
        fail("composition offset histogram count is inconsistent")
    dts_ticks = [bucket["delta_ticks"] for bucket in order["dts_delta_histogram"]]
    offset_ticks = [bucket["offset_ticks"] for bucket in order["composition_offset_histogram"]]
    if dts_ticks != sorted(set(dts_ticks)) or any(value <= 0 for value in dts_ticks):
        fail("DTS histogram ordering is invalid")
    if offset_ticks != sorted(set(offset_ticks)):
        fail("composition offset histogram ordering is invalid")


def verify_perceptual_sequence(sequence: dict, frame_count: int) -> None:
    hashes = sequence["frame_hashes"]
    if sequence["frame_count"] != frame_count or len(hashes) != frame_count:
        fail("perceptual frame count differs from cadence frame count")
    for value in hashes:
        if len(value) != PERCEPTUAL_HASH_HEX_LENGTH:
            fail("perceptual frame hash length is invalid")
        int(value, 16)
    canonical = "\n".join(hashes).encode()
    if hashlib.sha256(canonical).hexdigest() != sequence["sequence_sha256"]:
        fail("perceptual sequence identity is invalid")


def verify_perceptual_comparison(comparison: dict) -> None:
    if comparison["frame_count"] <= 0:
        fail("perceptual comparison has no frames")
    if comparison["max_hamming_distance"] > PERCEPTUAL_MAX_HAMMING_DISTANCE:
        fail("perceptual frame correspondence exceeded the safety bound")
    if comparison["exact_hash_match_count"] < 0 or comparison["exact_hash_match_count"] > comparison["frame_count"]:
        fail("perceptual exact-match count is invalid")
    if comparison["total_hamming_distance"] < comparison["max_hamming_distance"]:
        fail("perceptual distance aggregate is invalid")
    expected_mean = (comparison["total_hamming_distance"] * 1000 + comparison["frame_count"] // 2) // comparison["frame_count"]
    if comparison["mean_hamming_milli"] != expected_mean:
        fail("perceptual mean distance is inconsistent")
    if comparison["equivalent"] is not True:
        fail("perceptual candidate sequences diverged")


def main() -> None:
    if len(sys.argv) != 2:
        raise SystemExit("usage: verify_encoder_time_base_reorder.py REPORT.json")
    path = Path(sys.argv[1])
    report = json.loads(path.read_text())
    if report["schema_version"] != "ffmpeg-encoder-time-base-reorder-matrix-v1":
        fail("matrix schema drifted")
    if report["contract_version"] != "encoder-time-base-reorder-evidence-v1":
        fail("contract schema drifted")
    evidence = report["evidence"]
    canonical = json.dumps(evidence, ensure_ascii=False, separators=(",", ":"))
    if hashlib.sha256(canonical.encode()).hexdigest() != report["contract_hash"]:
        fail("contract hash is invalid")
    if evidence["repeat_count"] != 3 or evidence["packet_variance_ticks"] != 0:
        fail("repeat or variance policy drifted")
    if evidence["perceptual_max_hamming_distance"] != PERCEPTUAL_MAX_HAMMING_DISTANCE:
        fail("perceptual safety policy drifted")
    if evidence["seamless_allowed"] is not False or evidence["discontinuity_required"] is not True:
        fail("fail-closed policy drifted")
    cases = evidence["cases"]
    if [item["case"]["base"]["id"] for item in cases] != EXPECTED_CASES:
        fail("case registry drifted")

    for case in cases:
        spec = case["case"]
        if spec["b_frames"] <= 0 or spec["b_adapt"] != 0 or spec["open_gop"] is not False:
            fail("B-frame policy drifted")
        candidates = case["candidates"]
        if [item["spec"]["id"] for item in candidates] != EXPECTED_CANDIDATES:
            fail("candidate registry drifted")
        comparison = case["comparison"]
        if comparison["equivalent"] is not True or comparison["semantic_base_equivalent"] is not True:
            fail("AVTB and 90 kHz semantic reorder evidence diverged")
        if comparison["startup_packet_order_equivalent"] is not True or comparison["continuation_packet_order_equivalent"] is not True:
            fail("packet-order evidence diverged between candidates")
        base_comparison = comparison["base"]
        if not base_comparison["frame_mapping_equivalent"] or not base_comparison["cadence_equivalent"] or not base_comparison["av_sync_within_tolerance"]:
            fail("base cadence, mapping or A/V evidence diverged between candidates")
        verify_perceptual_comparison(comparison["startup_perceptual_comparison"])
        verify_perceptual_comparison(comparison["continuation_perceptual_comparison"])

        for candidate in candidates:
            summary = candidate["summary"]
            if not summary["stable"] or not summary["strict_dts"] or not summary["reorder_observed"] or not summary["packet_order_stable"] or not summary["perceptual_sequence_stable"]:
                fail("candidate reorder summary is not stable")
            if not summary["base"]["stable"] or not summary["base"]["all_preserved"]:
                fail("base candidate preservation failed")
            runs = candidate["runs"]
            if [run["ordinal"] for run in runs] != [1, 2, 3]:
                fail("repeat ordinals drifted")
            for run in runs:
                base = run["base"]
                for command_key in ("startup_command_hash", "continuation_command_hash"):
                    if len(base[command_key]) != 64:
                        fail("command identity is invalid")
                for window in ("startup", "continuation"):
                    timeline = base[f"{window}_timeline"]
                    fingerprint = base[f"{window}_fingerprint"]
                    if timeline["near_zero_delta_count"] != 0 or timeline["duplicate_pts_count"] != 0 or timeline["non_monotonic_pts_count"] != 0:
                        fail("candidate PTS cadence is not preserved")
                    if fingerprint["adjacent_duplicate_count"] != 0:
                        fail("decoded adjacent duplicate frame detected")
                    verify_packet_order(run[f"{window}_packet_order"], timeline["frame_count"])
                    verify_perceptual_sequence(run[f"{window}_perceptual_sequence"], timeline["frame_count"])
                if base["boundary"]["seamless_allowed"] is not False or base["boundary"]["discontinuity_required"] is not True:
                    fail("boundary evidence authorized seamless playback")
                if base["av_sync"]["seamless_allowed"] is not False or base["av_sync"]["discontinuity_required"] is not True:
                    fail("A/V evidence authorized seamless playback")

    print(json.dumps({
        case["case"]["base"]["id"]: {
            "startup_exact_pixels": case["comparison"]["base"]["startup_sequence_equivalent"],
            "continuation_exact_pixels": case["comparison"]["base"]["continuation_sequence_equivalent"],
            "startup_perceptual_max": case["comparison"]["startup_perceptual_comparison"]["max_hamming_distance"],
            "continuation_perceptual_max": case["comparison"]["continuation_perceptual_comparison"]["max_hamming_distance"],
            "candidates": {
                candidate["spec"]["id"]: {
                    "startup_reordered": candidate["summary"]["startup_reordered_packet_count"]["min"],
                    "continuation_reordered": candidate["summary"]["continuation_reordered_packet_count"]["min"],
                    "startup_depth": candidate["summary"]["startup_max_reorder_depth"]["min"],
                    "continuation_depth": candidate["summary"]["continuation_max_reorder_depth"]["min"],
                    "startup_max_cts_us": candidate["summary"]["startup_max_composition_offset_micros"]["min"],
                    "continuation_max_cts_us": candidate["summary"]["continuation_max_composition_offset_micros"]["min"],
                }
                for candidate in case["candidates"]
            },
        }
        for case in cases
    }, sort_keys=True))


if __name__ == "__main__":
    main()
