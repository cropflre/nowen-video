from __future__ import annotations

import json
import pathlib
import shutil
import subprocess
from typing import Any

from android_v2_p0.common import command, fail, fingerprint, load_json, sha256_file

from .common import Candidate, SENSITIVE_ASSET, commit_sha, exact_file_hashes, normalized_text


class GitHubCLI:
    def __init__(self, executable: str, repository: str) -> None:
        self.executable = executable
        self.repository = repository

    def run(self, *args: str, timeout: int = 120) -> subprocess.CompletedProcess[str]:
        result = command([self.executable, *args], timeout=timeout)
        if result.returncode:
            detail = ((result.stdout or "") + (result.stderr or "")).strip()
            fail(f"GitHub CLI command failed: {' '.join(args)}\n{detail}")
        return result

    def api_json(self, endpoint: str) -> dict[str, Any]:
        result = self.run("api", endpoint)
        try:
            payload = json.loads(result.stdout or "")
        except json.JSONDecodeError as exc:
            fail(f"GitHub API returned invalid JSON for {endpoint}: {exc}")
        if not isinstance(payload, dict):
            fail(f"GitHub API returned a non-object for {endpoint}")
        return payload

    def patch_release(self, release_id: int, fields: list[tuple[str, str]]) -> dict[str, Any]:
        argv = ["api", "--method", "PATCH", f"repos/{self.repository}/releases/{release_id}"]
        for key, value in fields:
            argv += ["-F", f"{key}={value}"]
        result = self.run(*argv)
        try:
            payload = json.loads(result.stdout or "")
        except json.JSONDecodeError as exc:
            fail(f"GitHub API returned invalid release update JSON: {exc}")
        if not isinstance(payload, dict):
            fail("GitHub API returned invalid release update payload")
        return payload


def resolve_tag_commit(gh: GitHubCLI, tag: str) -> str:
    ref = gh.api_json(f"repos/{gh.repository}/git/ref/tags/{tag}")
    obj = ref.get("object", {})
    kind, sha = obj.get("type"), str(obj.get("sha", "")).lower()
    for _ in range(8):
        if kind == "commit":
            return commit_sha(sha)
        if kind != "tag":
            fail(f"unsupported tag object type: {kind!r}")
        tagged = gh.api_json(f"repos/{gh.repository}/git/tags/{commit_sha(sha)}")
        obj = tagged.get("object", {})
        kind, sha = obj.get("type"), str(obj.get("sha", "")).lower()
    fail("annotated tag chain is too deep")


def release_metadata(gh: GitHubCLI, tag: str, *, draft: bool) -> dict[str, Any]:
    payload = gh.api_json(f"repos/{gh.repository}/releases/tags/{tag}")
    errors: list[str] = []
    if payload.get("tag_name") != tag:
        errors.append("release tag_name does not match requested tag")
    if payload.get("prerelease") is not True:
        errors.append("release must be marked prerelease")
    if payload.get("draft") is not draft:
        errors.append(f"release draft must be {draft}")
    if errors:
        fail("release state verification failed:\n- " + "\n- ".join(errors))
    return payload


def asset_names(release: dict[str, Any]) -> list[str]:
    assets = release.get("assets", [])
    if not isinstance(assets, list):
        fail("release assets payload is invalid")
    names = [str(item.get("name", "")) for item in assets if isinstance(item, dict)]
    if len(names) != len(set(names)):
        fail("release contains duplicate asset names")
    return names


def validate_allowlist(release: dict[str, Any], candidate: Candidate) -> None:
    names = asset_names(release)
    dangerous = sorted(name for name in names if SENSITIVE_ASSET.search(name))
    if dangerous:
        fail("release contains dangerous asset name(s): " + ", ".join(dangerous))
    expected, actual = set(candidate.files), set(names)
    missing, extra = sorted(expected - actual), sorted(actual - expected)
    if missing or extra:
        detail = []
        if missing:
            detail.append("missing: " + ", ".join(missing))
        if extra:
            detail.append("unexpected: " + ", ".join(extra))
        fail("release asset allowlist mismatch: " + "; ".join(detail))


def download_release(gh: GitHubCLI, tag: str, destination: pathlib.Path) -> None:
    if destination.exists():
        shutil.rmtree(destination)
    destination.mkdir(parents=True)
    gh.run("release", "download", tag, "--repo", gh.repository, "--dir", str(destination), "--clobber", timeout=300)


def verify_release_files(directory: pathlib.Path, candidate: Candidate, release: dict[str, Any]) -> None:
    errors: list[str] = []
    expected_hashes = exact_file_hashes(candidate)
    for name, expected in expected_hashes.items():
        path = directory / name
        if not path.is_file():
            errors.append(f"downloaded release file is missing: {name}")
        elif sha256_file(path) != expected:
            errors.append(f"release asset differs from P0-tested candidate: {name}")
    expected_body = normalized_text(candidate.path("RELEASE_NOTES.md").read_text(encoding="utf-8"))
    if normalized_text(str(release.get("body") or "")) != expected_body:
        errors.append("release body differs from candidate RELEASE_NOTES.md")
    if not errors:
        manifest = load_json(directory / "release-manifest.json")
        preflight = load_json(directory / "signing-preflight.json")
        if not isinstance(manifest, dict) or not isinstance(preflight, dict):
            errors.append("release JSON metadata is invalid")
        else:
            if str(manifest.get("source", {}).get("commit", "")).lower() != candidate.source_commit:
                errors.append("release manifest source commit is inconsistent")
            if fingerprint(str(manifest.get("signing", {}).get("certificate_sha256", ""))) != candidate.certificate_sha256:
                errors.append("release manifest certificate is inconsistent")
            if str(preflight.get("source", {}).get("commit", "")).lower() != candidate.source_commit:
                errors.append("release preflight source commit is inconsistent")
            if fingerprint(str(preflight.get("signing", {}).get("certificate_sha256", ""))) != candidate.certificate_sha256:
                errors.append("release preflight certificate is inconsistent")
    if errors:
        fail("release asset verification failed:\n- " + "\n- ".join(errors))


def sync_release(gh: GitHubCLI, tag: str, candidate: Candidate, release: dict[str, Any]) -> None:
    validate_allowlist(release, candidate)
    gh.run(
        "release", "upload", tag,
        *[str(candidate.path(name)) for name in candidate.files],
        "--repo", gh.repository, "--clobber", timeout=600,
    )
    gh.patch_release(
        int(release["id"]),
        [("body", f"@{candidate.path('RELEASE_NOTES.md')}"), ("draft", "true"), ("prerelease", "true")],
    )
