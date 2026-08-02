#!/usr/bin/env python3
import argparse
import hashlib
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
EXPECTED_CANDIDATES = [
    ("encoder-time-base-avtb-v1", "1/1000000"),
    ("encoder-time-base-90k-v1", "1/90000"),
]
EXPECTED_REQUIRED_EVIDENCE = [
    "timestamp_plan",
    "output_cadence",
    "packet_order",
    "produced_media_attestation",
    "boundary_packet",
    "av_boundary_sync",
]


def canonical_hash(value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def validate_sha256(value, label):
    assert isinstance(value, str) and len(value) == 64, f"{label} is not SHA-256"
    int(value, 16)


def base_case_evidence(reorder):
    return {
        "case": reorder["case"]["base"],
        "source_startup_timeline": reorder["source_startup_timeline"],
        "source_continuation_timeline": reorder["source_continuation_timeline"],
        "candidates": [
            {
                "spec": candidate["spec"],
                "runs": [run["base"] for run in candidate["runs"]],
                "summary": candidate["summary"]["base"],
            }
            for candidate in reorder["candidates"]
        ],
        "comparison": reorder["comparison"]["base"],
    }


def validate_run(run, source, timestamp_identity, case_id, candidate_id, ordinal):
    assert run["ordinal"] == ordinal
    base = run["base"]
    assert base["ordinal"] == ordinal
    validate_sha256(base["startup_command_hash"], "startup command hash")
    validate_sha256(base["continuation_command_hash"], "continuation command hash")
    assert base["startup_timeline"]["kind"] == f"candidate_{case_id}_{candidate_id}_run_{ordinal:02d}_startup"
    assert base["continuation_timeline"]["kind"] == f"candidate_{case_id}_{candidate_id}_run_{ordinal:02d}_continuation"
    assert abs(base["startup_mapping"]["frame_count_delta"]) <= 1
    assert abs(base["continuation_mapping"]["frame_count_delta"]) <= 1
    assert base["startup_mapping"]["input_frames"] == source["source_startup_timeline"]["frame_count"]
    assert base["continuation_mapping"]["input_frames"] == source["source_continuation_timeline"]["frame_count"]
    assert base["startup_mapping"]["output_frames"] == base["startup_timeline"]["frame_count"]
    assert base["continuation_mapping"]["output_frames"] == base["continuation_timeline"]["frame_count"]

    boundary = base["boundary"]
    assert boundary["case_id"] == f"{case_id}/{candidate_id}/run-{ordinal:02d}"
    assert boundary["timestamp_plan_version"] == timestamp_identity["version"]
    assert boundary["timestamp_plan_hash"] == timestamp_identity["hash"]
    assert boundary["startup_attestation_version"] == "hls-produced-media-attestation-v1"
    assert boundary["continuation_attestation_version"] == "hls-produced-media-attestation-v1"
    validate_sha256(boundary["startup_attestation_hash"], "startup attestation hash")
    validate_sha256(boundary["continuation_attestation_hash"], "continuation attestation hash")
    assert boundary["seamless_allowed"] is False
    assert boundary["discontinuity_required"] is True
    validate_sha256(base["boundary_hash"], "boundary hash")
    assert canonical_hash(boundary) == base["boundary_hash"]

    av_sync = base["av_sync"]
    assert av_sync["case_id"] == boundary["case_id"]
    assert av_sync["fixture_id"] == boundary["fixture_id"]
    assert av_sync["seamless_allowed"] is False
    assert av_sync["discontinuity_required"] is True
    validate_sha256(base["av_sync_hash"], "A/V sync hash")
    assert canonical_hash(av_sync) == base["av_sync_hash"]

    for key in ("startup_packet_order", "continuation_packet_order"):
        packet = run[key]
        assert packet["packet_count"] > 0
        assert packet["dts_non_monotonic_count"] == 0
        assert packet["dts_duplicate_count"] == 0
        assert packet["reordered_packet_count"] > 0
        assert packet["adjacent_pts_inversion_count"] > 0
        assert packet["max_presentation_reorder_depth"] > 0

    assert run["startup_perceptual_sequence"]["frame_count"] == base["startup_timeline"]["frame_count"]
    assert run["continuation_perceptual_sequence"]["frame_count"] == base["continuation_timeline"]["frame_count"]
    validate_sha256(run["startup_perceptual_sequence"]["sequence_sha256"], "startup perceptual sequence")
    validate_sha256(run["continuation_perceptual_sequence"]["sequence_sha256"], "continuation perceptual sequence")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()

    report = json.loads(args.report.read_text())
    assert report["schema_version"] == "ffmpeg-real-media-corpus-candidate-matrix-v1"
    assert report["contract_version"] == "real-media-corpus-candidate-evidence-v1"
    validate_sha256(report["contract_hash"], "contract hash")

    spec = report["spec"]
    manifest = report["manifest"]
    evidence = report["evidence"]
    assert spec["schema_version"] == "real-media-corpus-spec-v1"
    assert manifest["schema_version"] == "real-media-corpus-manifest-v1"
    assert evidence["schema_version"] == report["contract_version"]
    assert canonical_hash(spec) == evidence["spec_hash"] == manifest["spec_hash"]
    assert canonical_hash(manifest) == evidence["manifest_hash"]
    assert canonical_hash(evidence) == report["contract_hash"]
    assert evidence["spec_version"] == spec["schema_version"]
    assert evidence["manifest_version"] == manifest["schema_version"]
    assert evidence["source_generator_version"] == manifest["generator_version"]
    assert evidence["source_ffmpeg_version"] == manifest["ffmpeg_version"]
    assert evidence["source_ffprobe_version"] == manifest["ffprobe_version"]
    assert evidence["certification_ffmpeg_version"]
    assert evidence["certification_ffprobe_version"]
    assert evidence["repeat_count"] == 3
    assert evidence["packet_order_comparison_tolerance_ticks"] == 1
    assert evidence["decoded_frame_comparison_policy"] == "perceptual_frame_sequence_v1"
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True

    assert [case["id"] for case in spec["cases"]] == EXPECTED_CASES
    assert [asset["case_id"] for asset in manifest["assets"]] == EXPECTED_CASES
    assert [case["source"]["case_id"] for case in evidence["cases"]] == EXPECTED_CASES

    for index, bound in enumerate(evidence["cases"]):
        case_spec = spec["cases"][index]
        asset = manifest["assets"][index]
        source = bound["source"]
        reorder = bound["evidence"]
        case_id = EXPECTED_CASES[index]

        assert source["asset_index"] == index
        assert source["case_id"] == asset["case_id"] == case_spec["id"] == case_id
        assert source["relative_path"] == asset["relative_path"]
        assert source["sha256"] == asset["sha256"]
        assert source["size_bytes"] == asset["size_bytes"]
        validate_sha256(source["sha256"], "source file hash")
        validate_sha256(source["asset_evidence_hash"], "asset evidence hash")
        assert canonical_hash(asset) == source["asset_evidence_hash"]
        assert bound["required_evidence"] == case_spec["required_evidence"]
        assert sorted(bound["required_evidence"]) == sorted(EXPECTED_REQUIRED_EVIDENCE)

        assert reorder["case"]["base"]["id"] == case_id
        assert reorder["case"]["base"]["audio_sample_rate"] == case_spec["source"]["audio"]["sample_rate"]
        assert reorder["case"]["base"]["source_offset_micros"] == asset["probe"]["start_micros"]
        assert reorder["case"]["base"]["gop_size"] == case_spec["source"]["video"]["gop_size"]
        assert reorder["case"]["b_frames"] == case_spec["source"]["video"]["b_frames"]
        assert reorder["case"]["reference_frames"] == case_spec["source"]["video"]["reference_frames"]
        assert reorder["case"]["open_gop"] is False

        validate_sha256(bound["timestamp_plan"]["hash"], "timestamp plan hash")
        assert bound["timestamp_plan"]["version"] == "hls-timestamp-normalization-v1"
        validate_sha256(bound["time_base_candidate"]["hash"], "semantic time-base case hash")
        assert bound["time_base_candidate"]["version"] == "encoder-time-base-semantic-candidate-evidence-v1"
        validate_sha256(bound["reorder_candidate"]["hash"], "reorder case hash")
        assert bound["reorder_candidate"]["version"] == "encoder-time-base-reorder-evidence-v1"
        assert canonical_hash(base_case_evidence(reorder)) == bound["time_base_candidate"]["hash"]
        assert canonical_hash(reorder) == bound["reorder_candidate"]["hash"]

        candidates = reorder["candidates"]
        assert [(item["spec"]["id"], item["spec"]["encoder_time_base"]) for item in candidates] == EXPECTED_CANDIDATES
        for candidate in candidates:
            candidate_id = candidate["spec"]["id"]
            assert [run["ordinal"] for run in candidate["runs"]] == [1, 2, 3]
            for ordinal, run in enumerate(candidate["runs"], 1):
                validate_run(run, reorder, bound["timestamp_plan"], case_id, candidate_id, ordinal)
            assert candidate["summary"]["base"]["stable"] is True
            assert candidate["summary"]["base"]["all_preserved"] is True
            assert candidate["summary"]["packet_order_stable"] is True
            assert candidate["summary"]["perceptual_sequence_stable"] is True
            assert candidate["summary"]["strict_dts"] is True
            assert candidate["summary"]["reorder_observed"] is True
            assert candidate["summary"]["stable"] is True

        base_comparison = reorder["comparison"]["base"]
        assert base_comparison["frame_mapping_equivalent"] is True
        assert base_comparison["cadence_equivalent"] is True
        assert base_comparison["av_sync_within_tolerance"] is True
        assert reorder["comparison"]["semantic_base_equivalent"] is True
        assert reorder["comparison"]["startup_packet_order_equivalent"] is True
        assert reorder["comparison"]["continuation_packet_order_equivalent"] is True
        assert reorder["comparison"]["startup_perceptual_comparison"]["equivalent"] is True
        assert reorder["comparison"]["continuation_perceptual_comparison"]["equivalent"] is True
        assert reorder["comparison"]["equivalent"] is True

    print(json.dumps({
        "cases": len(evidence["cases"]),
        "candidates": len(EXPECTED_CANDIDATES),
        "repeats": evidence["repeat_count"],
        "manifest_hash": evidence["manifest_hash"],
        "contract_hash": report["contract_hash"],
    }, sort_keys=True))


if __name__ == "__main__":
    main()
