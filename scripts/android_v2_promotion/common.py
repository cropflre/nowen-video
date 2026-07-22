from __future__ import annotations

import pathlib
import re
from dataclasses import dataclass
from typing import Any

from android_v2_p0.common import (
    PRODUCT,
    fail,
    fingerprint,
    load_json,
    sha256_file,
    verify_candidate as verify_p0_candidate,
)

HEX40 = re.compile(r"[0-9a-f]{40}")
SENSITIVE_ASSET = re.compile(
    r"(?i)(debug|unsigned|keystore|\.jks$|\.keystore$|secret|credential|password|token)"
)


def commit_sha(value: str) -> str:
    normalized = str(value).strip().lower()
    if not HEX40.fullmatch(normalized):
        fail("source commit must contain exactly 40 hexadecimal characters")
    return normalized


def normalized_text(value: str) -> str:
    return value.replace("\r\n", "\n").rstrip() + "\n"


@dataclass(frozen=True)
class Candidate:
    directory: pathlib.Path
    version_name: str
    version_code: int
    source_commit: str
    certificate_sha256: str
    apk_name: str
    apk_sha256: str
    aab_name: str
    aab_sha256: str
    files: tuple[str, ...]

    def path(self, name: str) -> pathlib.Path:
        return self.directory / name


def verify_candidate(directory: pathlib.Path, version_name: str) -> Candidate:
    directory = directory.resolve()
    manifest = load_json(directory / "release-manifest.json")
    if not isinstance(manifest, dict):
        fail("release-manifest.json root must be an object")
    source_commit = commit_sha(str(manifest.get("source", {}).get("commit", "")))
    certificate = fingerprint(str(manifest.get("signing", {}).get("certificate_sha256", "")))
    verified = verify_p0_candidate(directory, version_name, source_commit, certificate)
    if not verified.get("apk_verified") or not verified.get("aab_verified"):
        fail("candidate APK and AAB signatures must both be verified before promotion")
    try:
        version_code = int(manifest.get("version", {}).get("code"))
        apk = verified["artifacts"]["apk"]
        aab = verified["artifacts"]["aab"]
    except (KeyError, TypeError, ValueError):
        fail("candidate verification result is incomplete")
    files = (
        str(apk["name"]),
        str(aab["name"]),
        "SHA256SUMS.txt",
        "release-manifest.json",
        "signing-preflight.json",
        "RELEASE_NOTES.md",
    )
    for name in files:
        if not (directory / name).is_file():
            fail(f"required candidate release file is missing: {name}")
    return Candidate(
        directory=directory,
        version_name=version_name,
        version_code=version_code,
        source_commit=source_commit,
        certificate_sha256=certificate,
        apk_name=str(apk["name"]),
        apk_sha256=str(apk["sha256"]),
        aab_name=str(aab["name"]),
        aab_sha256=str(aab["sha256"]),
        files=files,
    )


def verify_matrix(directory: pathlib.Path, candidate: Candidate) -> dict[str, Any]:
    directory = directory.resolve()
    payload = load_json(directory / "p0-matrix.json")
    report_path = directory / "P0_MATRIX_REPORT.md"
    if not isinstance(payload, dict):
        fail("p0-matrix.json root must be an object")
    if not report_path.is_file():
        fail(f"required file is missing: {report_path}")
    errors: list[str] = []

    def expect(actual: Any, expected: Any, label: str) -> None:
        if actual != expected:
            errors.append(f"{label}: expected {expected!r}, got {actual!r}")

    expect(payload.get("schema_version"), 1, "matrix schema")
    expect(payload.get("product"), PRODUCT, "matrix product")
    expect(payload.get("verdict"), "PASS", "matrix verdict")
    expect(payload.get("sensitive_values_included"), False, "matrix sensitive flag")
    anchor = payload.get("candidate", {})
    expect(anchor.get("version_name"), candidate.version_name, "matrix versionName")
    expect(int(anchor.get("version_code", -1)), candidate.version_code, "matrix versionCode")
    expect(str(anchor.get("source_commit", "")).lower(), candidate.source_commit, "matrix source commit")
    expect(fingerprint(str(anchor.get("certificate_sha256", ""))), candidate.certificate_sha256, "matrix certificate")
    expect(str(anchor.get("apk_sha256", "")).lower(), candidate.apk_sha256, "matrix APK hash")
    requirements = payload.get("requirements", {})
    expect(requirements.get("exact_api_levels"), [26, 33, 35], "required API levels")
    expect(sorted(requirements.get("observed_api_levels", [])), [26, 33, 35], "observed API levels")
    try:
        if int(requirements.get("full_scope_session_count", 0)) < 1:
            errors.append("matrix requires at least one full-scope session")
    except (TypeError, ValueError):
        errors.append("matrix full_scope_session_count is invalid")
    sessions = payload.get("sessions", [])
    if not isinstance(sessions, list) or len(sessions) < 3:
        errors.append("matrix requires at least three sessions")
    elif any(item.get("verdict") != "PASS" for item in sessions):
        errors.append("every matrix session must be finalized PASS")
    if payload.get("blocking_items"):
        errors.append("matrix contains blocking items")
    report = normalized_text(report_path.read_text(encoding="utf-8"))
    for token, label in (
        (candidate.version_name, "version"),
        (candidate.source_commit, "commit"),
        (candidate.certificate_sha256, "certificate"),
        (candidate.apk_sha256, "APK hash"),
        ("**PASS**", "PASS verdict"),
    ):
        if token not in report:
            errors.append(f"matrix report missing {label}: {token}")
    if errors:
        fail("P0 matrix verification failed:\n- " + "\n- ".join(errors))
    return payload


def exact_file_hashes(candidate: Candidate) -> dict[str, str]:
    return {name: sha256_file(candidate.path(name)) for name in candidate.files}
