#!/usr/bin/env python3
import argparse
import hashlib
import json
from pathlib import Path

SPEC_HASH = "ae9623f2c051868115a926b3a5cf881fbb58cc3408ff82a52ace1905332267fc"

EXPECTED = [
    {
        "id": "real-mp4-h264-aac-cfr-24000-1001-v1",
        "suffix": ".mp4",
        "container": "mp4",
        "mode": "cfr",
        "rates": [(24_000, 1_001)],
        "frames": 960,
        "gop": 48,
        "bframes": 2,
        "audio": ("aac", 48_000),
        "origin": 0,
        "edit_list": False,
    },
    {
        "id": "real-mp4-h264-aac-cfr-30000-1001-edit-list-v1",
        "suffix": ".mp4",
        "container": "mp4",
        "mode": "cfr",
        "rates": [(30_000, 1_001)],
        "frames": 1_199,
        "gop": 60,
        "bframes": 3,
        "audio": ("aac", 48_000),
        "origin": 5_000_000,
        "edit_list": True,
    },
    {
        "id": "real-mkv-h264-aac-vfr-24-30-v1",
        "suffix": ".mkv",
        "container": "matroska",
        "mode": "vfr",
        "rates": [(24, 1), (30, 1)],
        "frames": 1_080,
        "gop": 60,
        "bframes": 3,
        "audio": ("aac", 48_000),
        "origin": 0,
        "edit_list": False,
    },
    {
        "id": "real-mpegts-h264-aac-cfr-30-b3-v1",
        "suffix": ".ts",
        "container": "mpegts",
        "mode": "cfr",
        "rates": [(30, 1)],
        "frames": 1_200,
        "gop": 60,
        "bframes": 3,
        "audio": ("aac", 48_000),
        "origin": 1_400_000,
        "edit_list": False,
    },
    {
        "id": "real-mkv-h264-opus-cfr-25-v1",
        "suffix": ".mkv",
        "container": "matroska",
        "mode": "cfr",
        "rates": [(25, 1)],
        "frames": 1_000,
        "gop": 50,
        "bframes": 2,
        "audio": ("opus", 48_000),
        "origin": 0,
        "edit_list": False,
    },
    {
        "id": "real-mp4-h264-aac-cfr-30-aac-44100-v1",
        "suffix": ".mp4",
        "container": "mp4",
        "mode": "cfr",
        "rates": [(30, 1)],
        "frames": 1_200,
        "gop": 60,
        "bframes": 3,
        "audio": ("aac", 44_100),
        "origin": 0,
        "edit_list": False,
    },
]


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def frame_duration_micros(rate) -> int:
    numerator, denominator = rate
    return round(1_000_000 * denominator / numerator)


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    args = parser.parse_args()

    manifest_path = args.manifest.resolve()
    root = manifest_path.parent
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))

    assert manifest["schema_version"] == "real-media-corpus-manifest-v1"
    assert manifest["spec_version"] == "real-media-corpus-spec-v1"
    assert manifest["spec_hash"] == SPEC_HASH
    assert manifest["generator_version"] == "deterministic-real-media-corpus-generator-v1"
    assert manifest["generation_repeat_count"] == 2
    assert manifest["ffmpeg_version"].startswith("ffmpeg version ")
    assert manifest["ffprobe_version"].startswith("ffprobe version ")
    assert manifest["seamless_allowed"] is False
    assert manifest["discontinuity_required"] is True
    assert [asset["case_id"] for asset in manifest["assets"]] == [case["id"] for case in EXPECTED]

    total_bytes = 0
    for asset, expected in zip(manifest["assets"], EXPECTED):
        relative = Path(asset["relative_path"])
        assert not relative.is_absolute()
        assert relative.parts[0] == "assets"
        assert relative.name == expected["id"] + expected["suffix"]
        media_path = (root / relative).resolve()
        assert root in media_path.parents
        assert media_path.is_file()
        actual_size = media_path.stat().st_size
        actual_hash = sha256_file(media_path)
        assert actual_size == asset["size_bytes"] > 0
        assert actual_hash == asset["sha256"]
        assert len(asset["command_sha256"]) == 64
        assert asset["repeat_sha256"] == [actual_hash, actual_hash]
        total_bytes += actual_size

        probe = asset["probe"]
        assert probe["container"] == expected["container"]
        assert probe["video_codec"] == "h264"
        assert probe["video_profile"] == "high"
        assert probe["pixel_format"] == "yuv420p"
        assert probe["width"] == 640
        assert probe["height"] == 360
        assert probe["color_primaries"] == "bt709"
        assert probe["color_transfer"] == "bt709"
        assert probe["color_matrix"] == "bt709"
        assert probe["frame_rate_mode"] == expected["mode"]
        assert [(rate["numerator"], rate["denominator"]) for rate in probe["observed_rates"]] == expected["rates"]
        assert probe["frame_count"] == expected["frames"]
        assert probe["key_frame_count"] > 0
        assert probe["max_key_frame_interval"] == expected["gop"]
        assert probe["has_b_frame_reorder"] is True
        assert probe["max_presentation_reorder_depth"] == expected["bframes"]
        assert probe["max_composition_offset_micros"] > 0
        assert probe["audio_codec"] == expected["audio"][0]
        assert probe["audio_sample_rate"] == expected["audio"][1]
        assert probe["audio_channels"] == 2
        assert probe["audio_track_count"] == 1
        assert probe["has_edit_list"] is expected["edit_list"]
        assert abs(probe["duration_micros"] - 40_000_000) <= 100_000
        start_tolerance = frame_duration_micros(expected["rates"][0]) + 1_000
        assert abs(probe["start_micros"] - expected["origin"]) <= start_tolerance
        for time_base_name in ("video_time_base", "audio_time_base"):
            time_base = probe[time_base_name]
            assert time_base["numerator"] > 0
            assert time_base["denominator"] > 0

    staging = list(root.glob(".corpus-staging-*"))
    assert not staging, staging
    canonical = json.dumps(manifest, separators=(",", ":"), ensure_ascii=False).encode("utf-8")
    print(json.dumps({
        "asset_count": len(EXPECTED),
        "manifest_sha256": hashlib.sha256(canonical).hexdigest(),
        "spec_hash": SPEC_HASH,
        "total_bytes": total_bytes,
    }, sort_keys=True))


if __name__ == "__main__":
    main()
