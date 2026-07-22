from __future__ import annotations

import pathlib
import shutil
import subprocess
import tempfile
from typing import Any

from android_v2_p0.common import PRODUCT, fail, sha256_file, write_json

from .common import Candidate, verify_candidate, verify_matrix
from .github import (
    GitHubCLI,
    download_release,
    release_metadata,
    resolve_tag_commit,
    sync_release,
    validate_allowlist,
    verify_release_files,
)

VERSION = "0.1.0-rc.1"
COMMIT = "8" * 40
CERTIFICATE = "ab" * 32
TAG = f"android-v2-v{VERSION}"


class FakeGitHub(GitHubCLI):
    def __init__(self, assets: pathlib.Path) -> None:
        super().__init__("fake-gh", "cropflre/nowen-video")
        self.assets = assets
        self.commit = COMMIT
        self.tag_object = "1" * 40
        self.release: dict[str, Any] = {
            "id": 123,
            "tag_name": TAG,
            "draft": True,
            "prerelease": True,
            "body": "tag rebuild notes\n",
            "assets": [],
        }
        self.refresh()

    def refresh(self) -> None:
        self.release["assets"] = [
            {"name": path.name, "size": path.stat().st_size}
            for path in sorted(self.assets.iterdir())
        ]

    def run(self, *args: str, timeout: int = 120) -> subprocess.CompletedProcess[str]:
        del timeout
        if args[:2] == ("auth", "status"):
            return subprocess.CompletedProcess(args, 0, "", "")
        if args[:2] == ("release", "download"):
            destination = pathlib.Path(args[args.index("--dir") + 1])
            destination.mkdir(parents=True, exist_ok=True)
            for path in self.assets.iterdir():
                shutil.copy2(path, destination / path.name)
            return subprocess.CompletedProcess(args, 0, "", "")
        if args[:2] == ("release", "upload"):
            repository_index = args.index("--repo")
            for value in args[3:repository_index]:
                path = pathlib.Path(value)
                shutil.copy2(path, self.assets / path.name)
            self.refresh()
            return subprocess.CompletedProcess(args, 0, "", "")
        fail(f"unsupported fake-gh command: {' '.join(args)}")

    def api_json(self, endpoint: str) -> dict[str, Any]:
        if "/releases/tags/" in endpoint:
            return dict(self.release)
        if "/git/ref/tags/" in endpoint:
            return {"object": {"type": "tag", "sha": self.tag_object}}
        if "/git/tags/" in endpoint:
            return {"object": {"type": "commit", "sha": self.commit}}
        fail(f"unsupported fake-gh endpoint: {endpoint}")

    def patch_release(self, release_id: int, fields: list[tuple[str, str]]) -> dict[str, Any]:
        if release_id != self.release["id"]:
            fail("fake-gh release ID mismatch")
        for key, value in fields:
            if value.startswith("@"):
                self.release[key] = pathlib.Path(value[1:]).read_text(encoding="utf-8")
            elif value in {"true", "false"}:
                self.release[key] = value == "true"
            else:
                self.release[key] = value
        self.refresh()
        return dict(self.release)


def fixture(root: pathlib.Path) -> tuple[pathlib.Path, pathlib.Path]:
    candidate = root / "candidate"
    candidate.mkdir()
    apk_name = f"nowen-video-android-v2-{VERSION}.apk"
    aab_name = f"nowen-video-android-v2-{VERSION}.aab"
    (candidate / apk_name).write_bytes(b"tested-apk")
    (candidate / aab_name).write_bytes(b"tested-aab")
    apk_hash = sha256_file(candidate / apk_name)
    aab_hash = sha256_file(candidate / aab_name)
    write_json(candidate / "release-manifest.json", {
        "schema_version": 1,
        "product": PRODUCT,
        "channel": "rc",
        "version": {"name": VERSION, "code": 100501},
        "application": {"id": "com.nowen.video.v2", "min_sdk": 26, "target_sdk": 35},
        "source": {"commit": COMMIT},
        "signing": {"certificate_sha256": CERTIFICATE},
        "artifacts": [
            {"type": "apk", "name": apk_name, "sha256": apk_hash, "size_bytes": (candidate / apk_name).stat().st_size},
            {"type": "aab", "name": aab_name, "sha256": aab_hash, "size_bytes": (candidate / aab_name).stat().st_size},
        ],
    })
    write_json(candidate / "signing-preflight.json", {
        "source": {"commit": COMMIT},
        "version": {"name": VERSION},
        "signing": {"certificate_sha256": CERTIFICATE},
    })
    write_json(candidate / "candidate-verification.json", {
        "source_commit": COMMIT,
        "certificate_sha256": CERTIFICATE,
        "checksums_verified": True,
        "metadata_verified": True,
        "apk_signature_verified": True,
        "aab_signature_verified": True,
    })
    (candidate / "SHA256SUMS.txt").write_text(
        f"{apk_hash}  {apk_name}\n{aab_hash}  {aab_name}\n", encoding="utf-8"
    )
    (candidate / "RELEASE_NOTES.md").write_text(
        f"# RC\n\n{VERSION}\n{COMMIT}\n{CERTIFICATE}\n{apk_hash}\n{aab_hash}\n", encoding="utf-8"
    )
    matrix = root / "matrix"
    matrix.mkdir()
    write_json(matrix / "p0-matrix.json", {
        "schema_version": 1,
        "product": PRODUCT,
        "verdict": "PASS",
        "candidate": {
            "version_name": VERSION,
            "version_code": 100501,
            "source_commit": COMMIT,
            "certificate_sha256": CERTIFICATE,
            "apk_sha256": apk_hash,
        },
        "requirements": {
            "exact_api_levels": [26, 33, 35],
            "observed_api_levels": [26, 33, 35],
            "full_scope_session_count": 1,
        },
        "sessions": [
            {"session_id": "api26", "verdict": "PASS"},
            {"session_id": "api33", "verdict": "PASS"},
            {"session_id": "api35", "verdict": "PASS"},
        ],
        "blocking_items": [],
        "sensitive_values_included": False,
    })
    (matrix / "P0_MATRIX_REPORT.md").write_text(
        f"# Matrix\n\n**PASS**\n{VERSION}\n{COMMIT}\n{CERTIFICATE}\n{apk_hash}\n", encoding="utf-8"
    )
    return candidate, matrix


def run() -> int:
    with tempfile.TemporaryDirectory(prefix="android-v2-promotion-self-test-") as temporary:
        root = pathlib.Path(temporary)
        candidate_dir, matrix_dir = fixture(root)
        candidate: Candidate = verify_candidate(candidate_dir, VERSION)
        verify_matrix(matrix_dir, candidate)
        assets = root / "assets"
        assets.mkdir()
        for name in candidate.files:
            shutil.copy2(candidate.path(name), assets / name)
        next(assets.glob("*.apk")).write_bytes(b"tag-rebuilt-apk")
        gh = FakeGitHub(assets)
        if resolve_tag_commit(gh, TAG) != candidate.source_commit:
            fail("fake-gh tag resolution failed")
        release = release_metadata(gh, TAG, draft=True)
        validate_allowlist(release, candidate)
        with tempfile.TemporaryDirectory() as download:
            download_release(gh, TAG, pathlib.Path(download))
            try:
                verify_release_files(pathlib.Path(download), candidate, release)
            except SystemExit:
                pass
            else:
                fail("rebuilt tag asset should not pass exact-candidate verification")
        sync_release(gh, TAG, candidate, release)
        release = release_metadata(gh, TAG, draft=True)
        with tempfile.TemporaryDirectory() as download:
            download_release(gh, TAG, pathlib.Path(download))
            verify_release_files(pathlib.Path(download), candidate, release)
        gh.patch_release(int(release["id"]), [("draft", "false"), ("prerelease", "true")])
        published = release_metadata(gh, TAG, draft=False)
        with tempfile.TemporaryDirectory() as download:
            download_release(gh, TAG, pathlib.Path(download))
            verify_release_files(pathlib.Path(download), candidate, published)
        import json
        payload = json.loads((matrix_dir / "p0-matrix.json").read_text(encoding="utf-8"))
        payload["candidate"]["source_commit"] = "9" * 40
        write_json(matrix_dir / "p0-matrix.json", payload)
        try:
            verify_matrix(matrix_dir, candidate)
        except SystemExit:
            pass
        else:
            fail("wrong matrix commit should be rejected")
        gh.commit = "7" * 40
        if resolve_tag_commit(gh, TAG) == candidate.source_commit:
            fail("wrong tag commit should be rejected")
    print("Android V2 release promotion fake-gh self-test passed")
    return 0
