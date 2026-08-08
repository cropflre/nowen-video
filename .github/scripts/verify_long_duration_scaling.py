#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path

SHARD_REPORT_SCHEMA = "ffmpeg-long-duration-scaling-shard-v3"
AGGREGATE_REPORT_SCHEMA = "ffmpeg-long-duration-scaling-aggregate-v3"
SHARD_SCHEMA = "long-duration-scaling-shard-evidence-v3"
AGGREGATE_SCHEMA = "long-duration-scaling-aggregate-evidence-v3"
TIMESTAMP_VERSION = "hls-timestamp-normalization-v1"
START_TOLERANCE_US = 3_000_000
END_TOLERANCE_US = 50_000
CHECKPOINT_TOLERANCE_US = 50_000
AV_SKEW_TOLERANCE_US = 50_000
CROSS_CANDIDATE_TOLERANCE_US = 2_000
HLS_SEGMENT_US = 2_000_000
SEGMENT_TOLERANCE = 10

CANDIDATES = {
    "encoder-time-base-avtb-v1": "1/1000000",
    "encoder-time-base-90k-v1": "1/90000",
}

PROFILES = [
    {
        "id": "profile-mp4-aac-44100-cfr-v1",
        "source_case_id": "real-mp4-h264-aac-cfr-30-aac-44100-v1",
        "container": "mp4",
        "frame_rate_mode": "cfr",
        "audio_codec": "aac",
        "audio_sample_rate": 44100,
        "b_frames": 3,
        "source_origin_micros": 0,
        "has_edit_list": False,
    },
    {
        "id": "profile-mp4-edit-list-v1",
        "source_case_id": "real-mp4-h264-aac-cfr-30000-1001-edit-list-v1",
        "container": "mp4",
        "frame_rate_mode": "cfr",
        "audio_codec": "aac",
        "audio_sample_rate": 48000,
        "b_frames": 3,
        "source_origin_micros": 5_000_000,
        "has_edit_list": True,
    },
    {
        "id": "profile-mkv-vfr-24-30-v1",
        "source_case_id": "real-mkv-h264-aac-vfr-24-30-v1",
        "container": "matroska",
        "frame_rate_mode": "vfr",
        "audio_codec": "aac",
        "audio_sample_rate": 48000,
        "b_frames": 3,
        "source_origin_micros": 0,
        "has_edit_list": False,
    },
    {
        "id": "profile-mpegts-positive-origin-v1",
        "source_case_id": "real-mpegts-h264-aac-cfr-30-b3-v1",
        "container": "mpegts",
        "frame_rate_mode": "cfr",
        "audio_codec": "aac",
        "audio_sample_rate": 48000,
        "b_frames": 3,
        "source_origin_micros": 1_400_000,
        "has_edit_list": False,
    },
    {
        "id": "profile-mkv-opus-v1",
        "source_case_id": "real-mkv-h264-opus-cfr-25-v1",
        "container": "matroska",
        "frame_rate_mode": "cfr",
        "audio_codec": "opus",
        "audio_sample_rate": 48000,
        "b_frames": 2,
        "source_origin_micros": 0,
        "has_edit_list": False,
    },
]
PROFILE_BY_ID = {profile["id"]: profile for profile in PROFILES}

TIERS = [
    {
        "id": "multi-hour-breadth-2h-v1",
        "purpose": "exercise every certified profile for two continuous hours using the canonical AVTB candidate",
        "duration_micros": 2 * 60 * 60 * 1_000_000,
        "checkpoint_interval_micros": 30 * 60 * 1_000_000,
        "repeat_count": 1,
        "profile_ids": [profile["id"] for profile in PROFILES],
        "candidate_ids": ["encoder-time-base-avtb-v1"],
    },
    {
        "id": "multi-hour-depth-6h-v1",
        "purpose": "exercise six-hour clock-grid and VFR sentinels with both encoder time-base candidates",
        "duration_micros": 6 * 60 * 60 * 1_000_000,
        "checkpoint_interval_micros": 60 * 60 * 1_000_000,
        "repeat_count": 1,
        "profile_ids": ["profile-mp4-aac-44100-cfr-v1", "profile-mkv-vfr-24-30-v1"],
        "candidate_ids": ["encoder-time-base-avtb-v1", "encoder-time-base-90k-v1"],
    },
]
TIER_BY_ID = {tier["id"]: tier for tier in TIERS}


def shard_id(tier_id, profile_id, candidate_id):
    return f"{tier_id}--{profile_id}--{candidate_id}"


EXPECTED_SHARDS = [
    {
        "id": shard_id(tier["id"], profile_id, candidate_id),
        "tier_id": tier["id"],
        "profile_id": profile_id,
        "candidate_id": candidate_id,
    }
    for tier in TIERS
    for profile_id in tier["profile_ids"]
    for candidate_id in tier["candidate_ids"]
]


def canonical_hash(value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def validate_sha256(value, label):
    assert isinstance(value, str) and len(value) == 64, f"{label} is not SHA-256"
    int(value, 16)


def validate_source_contract(spec, manifest, evidence):
    assert canonical_hash(spec) == evidence["spec_hash"] == manifest["spec_hash"]
    assert canonical_hash(manifest) == evidence["manifest_hash"]
    assert evidence["spec_version"] == manifest["spec_version"]
    assert evidence["manifest_version"] == manifest["schema_version"]


def validate_stream(stream, kind, tier):
    duration = tier["duration_micros"]
    checkpoint_interval = tier["checkpoint_interval_micros"]
    assert stream["kind"] == kind
    assert stream["time_base"]
    assert stream["packet_count"] > 0
    assert abs(stream["start_micros"]) <= START_TOLERANCE_US
    assert stream["duration_micros"] == stream["end_micros"] - stream["start_micros"]
    assert stream["end_error_micros"] == stream["duration_micros"] - duration
    assert abs(stream["end_error_micros"]) <= END_TOLERANCE_US
    checkpoints = stream["checkpoints"]
    assert len(checkpoints) == duration // checkpoint_interval + 1
    previous = -1
    for index, checkpoint in enumerate(checkpoints):
        target = index * checkpoint_interval
        assert checkpoint["target_micros"] == target
        assert checkpoint["error_micros"] == checkpoint["presentation_micros"] - target
        assert abs(checkpoint["error_micros"]) <= CHECKPOINT_TOLERANCE_US
        assert checkpoint["presentation_micros"] >= previous
        previous = checkpoint["presentation_micros"]


def build_summary(runs):
    max_video = max(abs(run["video"]["end_error_micros"]) for run in runs)
    max_audio = max(abs(run["audio"]["end_error_micros"]) for run in runs)
    max_skew = max(abs(run["final_av_skew_micros"]) for run in runs)
    max_checkpoint = max(
        abs(checkpoint["error_micros"])
        for run in runs
        for stream in (run["video"], run["audio"])
        for checkpoint in stream["checkpoints"]
    )
    return {
        "repeat_count": len(runs),
        "maximum_absolute_video_end_error_micros": max_video,
        "maximum_absolute_audio_end_error_micros": max_audio,
        "maximum_absolute_av_skew_micros": max_skew,
        "maximum_absolute_checkpoint_error_micros": max_checkpoint,
        "maximum_repeat_metric_variance_micros": 0,
        "stable": (
            len(runs) == 1
            and max_video <= END_TOLERANCE_US
            and max_audio <= END_TOLERANCE_US
            and max_skew <= AV_SKEW_TOLERANCE_US
            and max_checkpoint <= CHECKPOINT_TOLERANCE_US
        ),
    }


def validate_candidate(candidate, expected_candidate_id, tier):
    assert candidate["id"] == expected_candidate_id
    assert candidate["encoder_time_base"] == CANDIDATES[expected_candidate_id]
    assert len(candidate["runs"]) == 1
    run = candidate["runs"][0]
    assert run["ordinal"] == 1
    for key in ("command_hash", "manifest_sha256", "attestation_hash"):
        validate_sha256(run[key], key)
    assert run["attestation_version"] == "hls-produced-media-attestation-v1"
    expected_segments = (tier["duration_micros"] + HLS_SEGMENT_US - 1) // HLS_SEGMENT_US
    assert expected_segments - SEGMENT_TOLERANCE <= run["segment_count"] <= expected_segments + SEGMENT_TOLERANCE
    validate_stream(run["video"], "video", tier)
    validate_stream(run["audio"], "audio", tier)
    assert run["final_av_skew_micros"] == run["video"]["end_micros"] - run["audio"]["end_micros"]
    assert abs(run["final_av_skew_micros"]) <= AV_SKEW_TOLERANCE_US
    assert candidate["summary"] == build_summary(candidate["runs"])
    assert candidate["summary"]["stable"] is True


def validate_profile_against_spec(profile, spec):
    cases = {case["id"]: case for case in spec["cases"]}
    case = cases[profile["source_case_id"]]
    source = case["source"]
    assert source["container"] == profile["container"]
    assert source["video"]["frame_rate_mode"] == profile["frame_rate_mode"]
    assert source["video"]["b_frames"] == profile["b_frames"]
    assert source["audio"]["codec"] == profile["audio_codec"]
    assert source["audio"]["sample_rate"] == profile["audio_sample_rate"]
    assert source["timeline"]["origin_micros"] == profile["source_origin_micros"]
    assert source["timeline"]["has_edit_list"] == profile["has_edit_list"]


def validate_shard_contract(evidence, spec, manifest):
    assert evidence["schema_version"] == SHARD_SCHEMA
    validate_source_contract(spec, manifest, evidence)
    assert evidence["timestamp_plan_version"] == TIMESTAMP_VERSION
    validate_sha256(evidence["timestamp_plan_hash"], "timestamp plan hash")
    assert evidence["certification_ffmpeg_version"]
    assert evidence["certification_ffprobe_version"]
    assert evidence["source_generator_version"] == manifest["generator_version"]
    assert evidence["source_ffmpeg_version"] == manifest["ffmpeg_version"]
    assert evidence["source_ffprobe_version"] == manifest["ffprobe_version"]
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True

    shard = evidence["shard"]
    assert shard in EXPECTED_SHARDS
    tier = TIER_BY_ID[shard["tier_id"]]
    profile = PROFILE_BY_ID[shard["profile_id"]]
    assert evidence["tier"] == tier
    assert evidence["profile"] == profile
    assert shard["profile_id"] in tier["profile_ids"]
    assert shard["candidate_id"] in tier["candidate_ids"]
    validate_profile_against_spec(profile, spec)

    assets = {asset["case_id"]: asset for asset in manifest["assets"]}
    asset = assets[profile["source_case_id"]]
    source = evidence["source"]
    assert source["case_id"] == profile["source_case_id"]
    assert source["relative_path"] == asset["relative_path"]
    assert source["sha256"] == asset["sha256"]
    assert source["size_bytes"] == asset["size_bytes"]
    assert source["asset_evidence_hash"] == canonical_hash(asset)
    validate_sha256(source["sha256"], "source hash")
    validate_sha256(source["asset_evidence_hash"], "asset evidence hash")
    validate_candidate(evidence["candidate"], shard["candidate_id"], tier)


def build_comparison(left, right):
    left_run = left["runs"][0]
    right_run = right["runs"][0]
    video = abs(left_run["video"]["end_error_micros"] - right_run["video"]["end_error_micros"])
    audio = abs(left_run["audio"]["end_error_micros"] - right_run["audio"]["end_error_micros"])
    skew = abs(left_run["final_av_skew_micros"] - right_run["final_av_skew_micros"])
    checkpoint = 0
    for kind in ("video", "audio"):
        for left_checkpoint, right_checkpoint in zip(left_run[kind]["checkpoints"], right_run[kind]["checkpoints"]):
            assert left_checkpoint["target_micros"] == right_checkpoint["target_micros"]
            checkpoint = max(checkpoint, abs(left_checkpoint["error_micros"] - right_checkpoint["error_micros"]))
    return {
        "candidate_a_id": left["id"],
        "candidate_b_id": right["id"],
        "maximum_video_end_difference_micros": video,
        "maximum_audio_end_difference_micros": audio,
        "maximum_av_skew_difference_micros": skew,
        "maximum_checkpoint_difference_micros": checkpoint,
        "equivalent": max(video, audio, skew, checkpoint) <= CROSS_CANDIDATE_TOLERANCE_US,
    }


def validate_shard_report(report):
    assert report["schema_version"] == SHARD_REPORT_SCHEMA
    assert report["contract_version"] == SHARD_SCHEMA
    validate_sha256(report["contract_hash"], "shard contract hash")
    validate_shard_contract(report["evidence"], report["spec"], report["manifest"])
    assert canonical_hash(report["evidence"]) == report["contract_hash"]
    evidence = report["evidence"]
    return {
        "kind": "shard",
        "shard": evidence["shard"]["id"],
        "duration_minutes": evidence["tier"]["duration_micros"] // 60_000_000,
        "segments": evidence["candidate"]["runs"][0]["segment_count"],
        "maximum_checkpoint_error_micros": evidence["candidate"]["summary"]["maximum_absolute_checkpoint_error_micros"],
        "contract_hash": report["contract_hash"],
    }


def validate_aggregate_report(report):
    assert report["schema_version"] == AGGREGATE_REPORT_SCHEMA
    assert report["contract_version"] == AGGREGATE_SCHEMA
    validate_sha256(report["contract_hash"], "aggregate contract hash")
    evidence = report["evidence"]
    assert evidence["schema_version"] == AGGREGATE_SCHEMA
    validate_source_contract(report["spec"], report["manifest"], evidence)
    assert evidence["timestamp_plan_version"] == TIMESTAMP_VERSION
    validate_sha256(evidence["timestamp_plan_hash"], "timestamp plan hash")
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True
    assert canonical_hash(evidence) == report["contract_hash"]

    bindings = evidence["shards"]
    assert len(bindings) == len(EXPECTED_SHARDS)
    candidates = {}
    for expected, binding in zip(EXPECTED_SHARDS, bindings):
        assert binding["shard_id"] == expected["id"]
        assert binding["contract_version"] == SHARD_SCHEMA
        validate_sha256(binding["contract_hash"], "bound shard contract hash")
        shard_evidence = binding["evidence"]
        assert shard_evidence["shard"] == expected
        validate_shard_contract(shard_evidence, report["spec"], report["manifest"])
        assert canonical_hash(shard_evidence) == binding["contract_hash"]
        assert shard_evidence["timestamp_plan_version"] == evidence["timestamp_plan_version"]
        assert shard_evidence["timestamp_plan_hash"] == evidence["timestamp_plan_hash"]
        candidates[expected["id"]] = shard_evidence["candidate"]

    expected_comparisons = []
    for tier in TIERS:
        if len(tier["candidate_ids"]) != 2:
            continue
        for profile_id in tier["profile_ids"]:
            left_id = shard_id(tier["id"], profile_id, tier["candidate_ids"][0])
            right_id = shard_id(tier["id"], profile_id, tier["candidate_ids"][1])
            expected_comparisons.append({
                "tier_id": tier["id"],
                "profile_id": profile_id,
                "comparison": build_comparison(candidates[left_id], candidates[right_id]),
            })
    assert evidence["comparisons"] == expected_comparisons
    assert all(item["comparison"]["equivalent"] for item in expected_comparisons)

    encoded_minutes = sum(
        TIER_BY_ID[shard["tier_id"]]["duration_micros"] // 60_000_000
        for shard in EXPECTED_SHARDS
    )
    max_end_error = max(
        max(
            binding["evidence"]["candidate"]["summary"]["maximum_absolute_video_end_error_micros"],
            binding["evidence"]["candidate"]["summary"]["maximum_absolute_audio_end_error_micros"],
        )
        for binding in bindings
    )
    max_checkpoint_difference = max(
        (item["comparison"]["maximum_checkpoint_difference_micros"] for item in expected_comparisons),
        default=0,
    )
    return {
        "kind": "aggregate",
        "shards": len(bindings),
        "encoded_minutes": encoded_minutes,
        "maximum_end_error_micros": max_end_error,
        "maximum_checkpoint_difference_micros": max_checkpoint_difference,
        "contract_hash": report["contract_hash"],
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()
    report = json.loads(args.report.read_text())
    if report.get("schema_version") == SHARD_REPORT_SCHEMA:
        result = validate_shard_report(report)
    elif report.get("schema_version") == AGGREGATE_REPORT_SCHEMA:
        result = validate_aggregate_report(report)
    else:
        raise AssertionError(f"unsupported report schema {report.get('schema_version')!r}")
    print(json.dumps(result, sort_keys=True))


if __name__ == "__main__":
    main()
