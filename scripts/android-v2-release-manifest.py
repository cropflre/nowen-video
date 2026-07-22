#!/usr/bin/env python3
"""Generate a verifiable Android V2 release manifest."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import sys
from pathlib import Path
from typing import Any

SCHEMA_VERSION = 1
EXPECTED_APPLICATION_ID = "com.nowen.video.v2"
EXPECTED_MIN_SDK = 26
EXPECTED_TARGET_SDK = 35


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def normalize_digest(value: str) -> str:
    normalized = value.strip().lower().replace(":", "")
    if len(normalized) != 64 or any(ch not in "0123456789abcdef" for ch in normalized):
        raise ValueError("signing certificate SHA-256 must contain exactly 64 hexadecimal characters")
    return normalized


def require_file(path: Path, label: str) -> Path:
    if not path.is_file():
        raise ValueError(f"{label} not found: {path}")
    return path


def infer_channel(version_name: str, event_name: str) -> str:
    if event_name == "pull_request":
        return "ci-smoke"
    if "-rc." in version_name:
        return "rc"
    if "-beta." in version_name:
        return "beta"
    if "-alpha." in version_name:
        return "alpha"
    return "stable"


def build_manifest(args: argparse.Namespace) -> dict[str, Any]:
    apk = require_file(Path(args.apk), "APK")
    aab = require_file(Path(args.aab), "AAB")

    if args.application_id != EXPECTED_APPLICATION_ID:
        raise ValueError(
            f"unexpected applicationId: {args.application_id}; expected {EXPECTED_APPLICATION_ID}"
        )
    if args.min_sdk != EXPECTED_MIN_SDK:
        raise ValueError(f"unexpected minSdk: {args.min_sdk}; expected {EXPECTED_MIN_SDK}")
    if args.target_sdk != EXPECTED_TARGET_SDK:
        raise ValueError(
            f"unexpected targetSdk: {args.target_sdk}; expected {EXPECTED_TARGET_SDK}"
        )
    if args.apk_version_name != args.version_name:
        raise ValueError(
            f"APK versionName mismatch: {args.apk_version_name}; expected {args.version_name}"
        )
    if args.apk_version_code != args.version_code:
        raise ValueError(
            f"APK versionCode mismatch: {args.apk_version_code}; expected {args.version_code}"
        )

    certificate_sha256 = normalize_digest(args.certificate_sha256)
    artifacts = []
    for artifact_type, path in (("apk", apk), ("aab", aab)):
        artifacts.append(
            {
                "type": artifact_type,
                "name": path.name,
                "size_bytes": path.stat().st_size,
                "sha256": sha256_file(path),
            }
        )

    return {
        "schema_version": SCHEMA_VERSION,
        "product": "Nowen Video Android V2",
        "channel": infer_channel(args.version_name, args.event_name),
        "version": {
            "name": args.version_name,
            "code": args.version_code,
        },
        "application": {
            "id": args.application_id,
            "min_sdk": args.min_sdk,
            "target_sdk": args.target_sdk,
        },
        "source": {
            "repository": args.repository,
            "commit": args.commit,
            "ref": args.ref,
        },
        "workflow": {
            "event": args.event_name,
            "run_id": args.run_id,
            "run_attempt": args.run_attempt,
        },
        "signing": {
            "certificate_sha256": certificate_sha256,
        },
        "artifacts": artifacts,
    }


def write_manifest(manifest: dict[str, Any], output: Path) -> None:
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def run_self_test() -> None:
    import tempfile

    with tempfile.TemporaryDirectory() as directory:
        root = Path(directory)
        apk = root / "sample.apk"
        aab = root / "sample.aab"
        apk.write_bytes(b"apk-content")
        aab.write_bytes(b"aab-content")
        args = argparse.Namespace(
            apk=str(apk),
            aab=str(aab),
            version_name="0.1.0-rc.1",
            version_code=100501,
            apk_version_name="0.1.0-rc.1",
            apk_version_code=100501,
            application_id=EXPECTED_APPLICATION_ID,
            min_sdk=EXPECTED_MIN_SDK,
            target_sdk=EXPECTED_TARGET_SDK,
            certificate_sha256="AA:" * 31 + "AA",
            repository="cropflre/nowen-video",
            commit="a" * 40,
            ref="refs/tags/android-v2-v0.1.0-rc.1",
            event_name="push",
            run_id="123",
            run_attempt="1",
        )
        manifest = build_manifest(args)
        assert manifest["channel"] == "rc"
        assert manifest["version"]["code"] == 100501
        assert manifest["signing"]["certificate_sha256"] == "aa" * 32
        assert manifest["artifacts"][0]["sha256"] == sha256_file(apk)
        assert manifest["artifacts"][1]["size_bytes"] == len(b"aab-content")

        args.apk_version_code = 100500
        try:
            build_manifest(args)
        except ValueError as error:
            assert "versionCode mismatch" in str(error)
        else:
            raise AssertionError("versionCode mismatch must fail")

        args.apk_version_code = 100501
        args.application_id = "com.example.invalid"
        try:
            build_manifest(args)
        except ValueError as error:
            assert "unexpected applicationId" in str(error)
        else:
            raise AssertionError("applicationId mismatch must fail")

    print("Android V2 release manifest self-test passed")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("--apk")
    parser.add_argument("--aab")
    parser.add_argument("--output", default="release-manifest.json")
    parser.add_argument("--version-name")
    parser.add_argument("--version-code", type=int)
    parser.add_argument("--apk-version-name")
    parser.add_argument("--apk-version-code", type=int)
    parser.add_argument("--application-id")
    parser.add_argument("--min-sdk", type=int)
    parser.add_argument("--target-sdk", type=int)
    parser.add_argument("--certificate-sha256")
    parser.add_argument("--repository", default=os.getenv("GITHUB_REPOSITORY", ""))
    parser.add_argument("--commit", default=os.getenv("GITHUB_SHA", ""))
    parser.add_argument("--ref", default=os.getenv("GITHUB_REF", ""))
    parser.add_argument("--event-name", default=os.getenv("GITHUB_EVENT_NAME", ""))
    parser.add_argument("--run-id", default=os.getenv("GITHUB_RUN_ID", ""))
    parser.add_argument("--run-attempt", default=os.getenv("GITHUB_RUN_ATTEMPT", ""))
    args = parser.parse_args()

    if args.self_test:
        return args

    required = (
        "apk",
        "aab",
        "version_name",
        "version_code",
        "apk_version_name",
        "apk_version_code",
        "application_id",
        "min_sdk",
        "target_sdk",
        "certificate_sha256",
        "repository",
        "commit",
        "ref",
        "event_name",
        "run_id",
        "run_attempt",
    )
    missing = [name.replace("_", "-") for name in required if getattr(args, name) in (None, "")]
    if missing:
        parser.error("missing required arguments or environment values: " + ", ".join(missing))
    return args


def main() -> int:
    args = parse_args()
    if args.self_test:
        run_self_test()
        return 0
    try:
        manifest = build_manifest(args)
        write_manifest(manifest, Path(args.output))
    except ValueError as error:
        print(f"error: {error}", file=sys.stderr)
        return 1
    print(f"Wrote Android V2 release manifest to {args.output}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
