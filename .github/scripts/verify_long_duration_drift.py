#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path

SOURCE_CASE = "real-mp4-h264-aac-cfr-30-aac-44100-v1"
DURATION_US = 30 * 60 * 1_000_000
CHECKPOINT_US = 5 * 60 * 1_000_000
REPEATS = 2
EXPECTED_CANDIDATES = [
    ("encoder-time-base-avtb-v1", "1/1000000"),
    ("encoder-time-base-90k-v1", "1/90000"),
]


def canonical_hash(value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()
    return hashlib.sha256(encoded).hexdigest()


def validate_sha256(value, label):
    assert isinstance(value, str) and len(value) == 64, f"{label} is not SHA-256"
    int(value, 16)


def validate_stream(stream, kind, evidence):
    assert stream["kind"] == kind
    assert stream["time_base"]
    assert stream["packet_count"] > 0
    assert abs(stream["start_micros"]) <= evidence["start_tolerance_micros"]
    assert stream["duration_micros"] == stream["end_micros"] - stream["start_micros"]
    assert stream["end_error_micros"] == stream["duration_micros"] - DURATION_US
    assert abs(stream["end_error_micros"]) <= evidence["end_tolerance_micros"]
    checkpoints = stream["checkpoints"]
    assert len(checkpoints) == DURATION_US // CHECKPOINT_US + 1
    previous = -1
    for index, checkpoint in enumerate(checkpoints):
        target = index * CHECKPOINT_US
        assert checkpoint["target_micros"] == target
        assert checkpoint["error_micros"] == checkpoint["presentation_micros"] - target
        assert abs(checkpoint["error_micros"]) <= evidence["checkpoint_tolerance_micros"]
        assert checkpoint["presentation_micros"] >= previous
        previous = checkpoint["presentation_micros"]


def build_summary(runs, evidence):
    max_video = 0
    max_audio = 0
    max_skew = 0
    max_checkpoint = 0
    metrics = []
    for run in runs:
        checkpoint_max = max(
            abs(checkpoint["error_micros"])
            for stream in (run["video"], run["audio"])
            for checkpoint in stream["checkpoints"]
        )
        max_video = max(max_video, abs(run["video"]["end_error_micros"]))
        max_audio = max(max_audio, abs(run["audio"]["end_error_micros"]))
        max_skew = max(max_skew, abs(run["final_av_skew_micros"]))
        max_checkpoint = max(max_checkpoint, checkpoint_max)
        metrics.append([
            run["video"]["end_error_micros"],
            run["audio"]["end_error_micros"],
            run["final_av_skew_micros"],
            checkpoint_max,
        ])
    variance = max(
        max(values[column] for values in metrics) - min(values[column] for values in metrics)
        for column in range(4)
    )
    stable = (
        len(runs) == REPEATS
        and max_video <= evidence["end_tolerance_micros"]
        and max_audio <= evidence["end_tolerance_micros"]
        and max_skew <= evidence["av_skew_tolerance_micros"]
        and max_checkpoint <= evidence["checkpoint_tolerance_micros"]
        and variance <= evidence["repeat_variance_tolerance_micros"]
    )
    return {
        "repeat_count": len(runs),
        "maximum_absolute_video_end_error_micros": max_video,
        "maximum_absolute_audio_end_error_micros": max_audio,
        "maximum_absolute_av_skew_micros": max_skew,
        "maximum_absolute_checkpoint_error_micros": max_checkpoint,
        "maximum_repeat_metric_variance_micros": variance,
        "stable": stable,
    }


def build_comparison(left, right, evidence):
    video = audio = skew = checkpoint = 0
    for left_run, right_run in zip(left["runs"], right["runs"]):
        video = max(video, abs(left_run["video"]["end_error_micros"] - right_run["video"]["end_error_micros"]))
        audio = max(audio, abs(left_run["audio"]["end_error_micros"] - right_run["audio"]["end_error_micros"]))
        skew = max(skew, abs(left_run["final_av_skew_micros"] - right_run["final_av_skew_micros"]))
        for kind in ("video", "audio"):
            for left_checkpoint, right_checkpoint in zip(left_run[kind]["checkpoints"], right_run[kind]["checkpoints"]):
                assert left_checkpoint["target_micros"] == right_checkpoint["target_micros"]
                checkpoint = max(checkpoint, abs(left_checkpoint["error_micros"] - right_checkpoint["error_micros"]))
    tolerance = evidence["cross_candidate_tolerance_micros"]
    return {
        "candidate_a_id": left["id"],
        "candidate_b_id": right["id"],
        "maximum_video_end_difference_micros": video,
        "maximum_audio_end_difference_micros": audio,
        "maximum_av_skew_difference_micros": skew,
        "maximum_checkpoint_difference_micros": checkpoint,
        "equivalent": max(video, audio, skew, checkpoint) <= tolerance,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("report", type=Path)
    args = parser.parse_args()

    report = json.loads(args.report.read_text())
    assert report["schema_version"] == "ffmpeg-long-duration-drift-matrix-v1"
    assert report["contract_version"] == "long-duration-drift-evidence-v1"
    validate_sha256(report["contract_hash"], "contract hash")

    spec = report["spec"]
    manifest = report["manifest"]
    evidence = report["evidence"]
    assert canonical_hash(spec) == evidence["spec_hash"] == manifest["spec_hash"]
    assert canonical_hash(manifest) == evidence["manifest_hash"]
    assert canonical_hash(evidence) == report["contract_hash"]
    assert evidence["timestamp_plan_version"] == "hls-timestamp-normalization-v1"
    validate_sha256(evidence["timestamp_plan_hash"], "timestamp plan hash")
    assert evidence["duration_micros"] == DURATION_US
    assert evidence["checkpoint_interval_micros"] == CHECKPOINT_US
    assert evidence["repeat_count"] == REPEATS
    assert evidence["seamless_allowed"] is False
    assert evidence["discontinuity_required"] is True

    assets = {asset["case_id"]: asset for asset in manifest["assets"]}
    asset = assets[SOURCE_CASE]
    source = evidence["source"]
    assert source["case_id"] == SOURCE_CASE
    assert source["relative_path"] == asset["relative_path"]
    assert source["sha256"] == asset["sha256"]
    assert source["size_bytes"] == asset["size_bytes"]
    validate_sha256(source["sha256"], "source hash")
    validate_sha256(source["asset_evidence_hash"], "asset evidence hash")
    assert canonical_hash(asset) == source["asset_evidence_hash"]

    candidates = evidence["candidates"]
    assert [(candidate["id"], candidate["encoder_time_base"]) for candidate in candidates] == EXPECTED_CANDIDATES
    for candidate in candidates:
        assert [run["ordinal"] for run in candidate["runs"]] == [1, 2]
        for run in candidate["runs"]:
            validate_sha256(run["command_hash"], "command hash")
            validate_sha256(run["manifest_sha256"], "manifest hash")
            validate_sha256(run["attestation_hash"], "attestation hash")
            assert run["attestation_version"] == "hls-produced-media-attestation-v1"
            assert 890 <= run["segment_count"] <= 910
            validate_stream(run["video"], "video", evidence)
            validate_stream(run["audio"], "audio", evidence)
            assert run["final_av_skew_micros"] == run["video"]["end_micros"] - run["audio"]["end_micros"]
            assert abs(run["final_av_skew_micros"]) <= evidence["av_skew_tolerance_micros"]
        assert candidate["summary"] == build_summary(candidate["runs"], evidence)
        assert candidate["summary"]["stable"] is True

    assert evidence["comparison"] == build_comparison(candidates[0], candidates[1], evidence)
    assert evidence["comparison"]["equivalent"] is True

    print(json.dumps({
        "source_case": SOURCE_CASE,
        "duration_minutes": DURATION_US // 60_000_000,
        "checkpoints_per_stream": DURATION_US // CHECKPOINT_US + 1,
        "executions": len(candidates) * REPEATS,
        "contract_hash": report["contract_hash"],
        "timestamp_plan_hash": evidence["timestamp_plan_hash"],
        "maximum_checkpoint_difference_micros": evidence["comparison"]["maximum_checkpoint_difference_micros"],
    }, sort_keys=True))


if __name__ == "__main__":
    main()
