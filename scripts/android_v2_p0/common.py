from __future__ import annotations

import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
from dataclasses import dataclass
from typing import Any

PRODUCT = "Nowen Video Android V2"
PACKAGE_ID = "com.nowen.video.v2"
LEGACY_PACKAGE_ID = "com.nowen.video"
MIN_SDK = 26
TARGET_SDK = 35
STATUSES = {"PENDING", "PASS", "FAIL", "BLOCKED", "N/A"}
ROOT = pathlib.Path(__file__).resolve().parents[2]
CASES_PATH = ROOT / "clients/android-v2/P0_CASES.json"
SENSITIVE = [
    re.compile(r"(?i)authorization\s*[:=]"),
    re.compile(r"(?i)bearer\s+[A-Za-z0-9._~+/=-]+"),
    re.compile(r"(?i)(token|password|passwd|secret|cookie)\s*[:=]"),
    re.compile(r"https?://[^\s]+"),
    re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b"),
]


def fail(message: str) -> None:
    raise SystemExit(f"error: {message}")


def load_json(path: pathlib.Path) -> dict[str, Any] | list[Any]:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except FileNotFoundError:
        fail(f"required file is missing: {path}")
    except json.JSONDecodeError as exc:
        fail(f"invalid JSON in {path}: {exc}")


def write_json(path: pathlib.Path, payload: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(payload, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def sha256_file(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def fingerprint(value: str) -> str:
    normalized = re.sub(r"[:\s]", "", value).lower()
    if not re.fullmatch(r"[0-9a-f]{64}", normalized):
        fail("certificate SHA-256 must contain exactly 64 hexadecimal characters")
    return normalized


def safe_text(value: str) -> str:
    value = value.strip()
    if any(pattern.search(value) for pattern in SENSITIVE):
        fail("public report text appears to contain a URL, IP address, credential, token, or secret")
    return value


def command(argv: list[str], *, timeout: int = 60, text: bool = True) -> subprocess.CompletedProcess[Any]:
    try:
        return subprocess.run(argv, capture_output=True, check=False, timeout=timeout, text=text)
    except FileNotFoundError:
        fail(f"required command not found: {argv[0]}")
    except subprocess.TimeoutExpired:
        fail(f"command timed out: {' '.join(argv)}")


@dataclass
class Adb:
    executable: str
    serial: str | None = None

    def run(self, *args: str, timeout: int = 60, text: bool = True) -> subprocess.CompletedProcess[Any]:
        argv = [self.executable]
        if self.serial:
            argv += ["-s", self.serial]
        return command(argv + list(args), timeout=timeout, text=text)

    def shell(self, *args: str, timeout: int = 60) -> str:
        return (self.run("shell", *args, timeout=timeout).stdout or "").strip()


def parse_devices(output: str) -> dict[str, dict[str, str]]:
    devices: dict[str, dict[str, str]] = {}
    for line in output.splitlines():
        parts = line.strip().split()
        if len(parts) < 2 or line.startswith("List of devices") or line.startswith("*"):
            continue
        metadata = {"state": parts[1]}
        for token in parts[2:]:
            if ":" in token:
                key, value = token.split(":", 1)
                metadata[key] = value
        devices[parts[0]] = metadata
    return devices


def select_device(adb_path: str, serial: str | None) -> tuple[Adb, dict[str, str]]:
    result = Adb(adb_path).run("devices", "-l")
    if result.returncode:
        fail(f"adb devices failed: {(result.stderr or '').strip()}")
    devices = parse_devices(result.stdout or "")
    if serial:
        if serial not in devices:
            fail(f"requested device is not connected: {serial}")
        if devices[serial].get("state") != "device":
            fail(f"requested device is not authorized/online: {serial} ({devices[serial].get('state')})")
        return Adb(adb_path, serial), devices[serial]
    online = [(key, value) for key, value in devices.items() if value.get("state") == "device"]
    if not online:
        fail("no authorized ADB device is connected")
    if len(online) > 1:
        fail("multiple authorized ADB devices are connected; pass --serial")
    return Adb(adb_path, online[0][0]), online[0][1]


def _parse_sums(path: pathlib.Path) -> dict[str, str]:
    if not path.is_file():
        fail(f"required file is missing: {path}")
    result: dict[str, str] = {}
    for line in path.read_text(encoding="utf-8").splitlines():
        match = re.fullmatch(r"([0-9a-fA-F]{64})\s+\*?(.+)", line.strip())
        if not match:
            fail(f"invalid SHA256SUMS line: {line}")
        result[match.group(2)] = match.group(1).lower()
    return result


def _find_apksigner() -> pathlib.Path | None:
    direct = shutil.which("apksigner")
    if direct:
        return pathlib.Path(direct)
    for variable in ("ANDROID_HOME", "ANDROID_SDK_ROOT"):
        root = os.environ.get(variable)
        if root:
            candidates = sorted((pathlib.Path(root) / "build-tools").glob("*/apksigner"))
            if candidates:
                return candidates[-1]
    return None


def _verify_apk(apk: pathlib.Path, expected: str) -> tuple[bool, str]:
    tool = _find_apksigner()
    if not tool:
        return False, "apksigner not found"
    result = command([str(tool), "verify", "--verbose", "--print-certs", str(apk)], timeout=120)
    text = (result.stdout or "") + (result.stderr or "")
    match = re.search(r"certificate SHA-256 digest:\s*([0-9a-fA-F:]+)", text, re.I)
    if result.returncode or not match:
        return False, text[-1000:] or "unable to read APK certificate"
    actual = fingerprint(match.group(1))
    return actual == expected, f"APK certificate SHA-256: {actual}"


def _verify_aab(aab: pathlib.Path) -> tuple[bool, str]:
    tool = shutil.which("jarsigner")
    if not tool:
        return False, "jarsigner not found"
    result = command([tool, "-verify", str(aab)], timeout=120)
    text = ((result.stdout or "") + (result.stderr or ""))[-1000:]
    return result.returncode == 0, text or "AAB signature verified"


def verify_candidate(directory: pathlib.Path, version: str, commit: str, cert: str) -> dict[str, Any]:
    commit = commit.lower()
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        fail("expected commit must be a 40-character hexadecimal SHA")
    cert = fingerprint(cert)
    manifest = load_json(directory / "release-manifest.json")
    verification = load_json(directory / "candidate-verification.json")
    sums = _parse_sums(directory / "SHA256SUMS.txt")
    if not isinstance(manifest, dict) or not isinstance(verification, dict):
        fail("candidate JSON root must be an object")
    errors: list[str] = []

    def expect(actual: Any, expected: Any, label: str) -> None:
        if actual != expected:
            errors.append(f"{label}: expected {expected!r}, got {actual!r}")

    expect(manifest.get("product"), PRODUCT, "product")
    expect(manifest.get("version", {}).get("name"), version, "versionName")
    expect(manifest.get("application", {}).get("id"), PACKAGE_ID, "applicationId")
    expect(int(manifest.get("application", {}).get("min_sdk", -1)), MIN_SDK, "minSdk")
    expect(int(manifest.get("application", {}).get("target_sdk", -1)), TARGET_SDK, "targetSdk")
    expect(str(manifest.get("source", {}).get("commit", "")).lower(), commit, "source commit")
    expect(fingerprint(str(manifest.get("signing", {}).get("certificate_sha256", ""))), cert, "manifest certificate")
    expect(str(verification.get("source_commit", "")).lower(), commit, "verification commit")
    expect(fingerprint(str(verification.get("certificate_sha256", ""))), cert, "verification certificate")
    expect(verification.get("checksums_verified"), True, "checksums_verified")
    expect(verification.get("metadata_verified"), True, "metadata_verified")

    artifacts = {str(item.get("type")): item for item in manifest.get("artifacts", [])}
    if set(artifacts) != {"apk", "aab"}:
        errors.append("manifest must contain exactly apk and aab artifacts")
    resolved: dict[str, dict[str, Any]] = {}
    for kind in ("apk", "aab"):
        item = artifacts.get(kind)
        if not item:
            continue
        name = str(item.get("name", ""))
        path = directory / name
        if not path.is_file():
            errors.append(f"missing {kind.upper()}: {name}")
            continue
        digest, size = sha256_file(path), path.stat().st_size
        expect(digest, str(item.get("sha256", "")).lower(), f"manifest {kind} hash")
        expect(size, int(item.get("size_bytes", -1)), f"manifest {kind} size")
        expect(sums.get(name), digest, f"SHA256SUMS {kind} hash")
        resolved[kind] = {"name": name, "path": path, "sha256": digest, "size_bytes": size}

    notes_path = directory / "RELEASE_NOTES.md"
    notes = notes_path.read_text(encoding="utf-8") if notes_path.is_file() else ""
    for token, label in ((version, "version"), (commit, "commit"), (cert, "certificate")):
        if token not in notes:
            errors.append(f"Release Notes missing {label}: {token}")
    if "{{" in notes or "}}" in notes:
        errors.append("Release Notes contain unresolved placeholders")
    if errors:
        fail("candidate verification failed:\n- " + "\n- ".join(errors))

    apk_ok = bool(verification.get("apk_signature_verified"))
    apk_detail = "candidate-verification.json records local APK verification"
    if not apk_ok:
        apk_ok, apk_detail = _verify_apk(resolved["apk"]["path"], cert)
    aab_ok = bool(verification.get("aab_signature_verified"))
    aab_detail = "candidate-verification.json records local AAB verification"
    if not aab_ok:
        aab_ok, aab_detail = _verify_aab(resolved["aab"]["path"])
    run_file = directory / "workflow-run-id.txt"
    return {
        "manifest": manifest,
        "artifacts": resolved,
        "apk_verified": apk_ok,
        "apk_detail": apk_detail.strip(),
        "aab_verified": aab_ok,
        "aab_detail": aab_detail.strip(),
        "run_id": run_file.read_text(encoding="utf-8").strip() if run_file.is_file() else None,
    }
