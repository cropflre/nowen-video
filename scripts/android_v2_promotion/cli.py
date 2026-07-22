from __future__ import annotations

import argparse
import datetime as dt
import pathlib
import shutil
import tempfile
from typing import Any

from android_v2_p0.common import fail, write_json

from .common import Candidate, verify_candidate, verify_matrix
from .github import (
    GitHubCLI,
    asset_names,
    download_release,
    release_metadata,
    resolve_tag_commit,
    sync_release,
    validate_allowlist,
    verify_release_files,
)
from .selftest import run as self_test

DEFAULT_REPOSITORY = "cropflre/nowen-video"


def now() -> str:
    return dt.datetime.now(dt.timezone.utc).isoformat()


def client(args: argparse.Namespace) -> GitHubCLI:
    executable = args.gh or shutil.which("gh")
    if not executable:
        fail("required command not found: gh")
    gh = GitHubCLI(executable, args.repository)
    gh.run("auth", "status")
    return gh


def verify_remote(gh: GitHubCLI, tag: str, candidate: Candidate, *, draft: bool) -> dict[str, Any]:
    resolved = resolve_tag_commit(gh, tag)
    if resolved != candidate.source_commit:
        fail(f"tag resolves to {resolved}, expected {candidate.source_commit}")
    release = release_metadata(gh, tag, draft=draft)
    validate_allowlist(release, candidate)
    with tempfile.TemporaryDirectory(prefix="android-v2-release-promotion-") as temporary:
        directory = pathlib.Path(temporary)
        download_release(gh, tag, directory)
        verify_release_files(directory, candidate, release)
    return release


def report(operation: str, repository: str, tag: str, candidate: Candidate, matrix: dict[str, Any], release: dict[str, Any]) -> dict[str, Any]:
    return {
        "schema_version": 1,
        "product": "Nowen Video Android V2",
        "generated_at_utc": now(),
        "operation": operation,
        "verdict": "PASS",
        "repository": repository,
        "tag": tag,
        "candidate": {
            "version_name": candidate.version_name,
            "version_code": candidate.version_code,
            "source_commit": candidate.source_commit,
            "certificate_sha256": candidate.certificate_sha256,
            "apk_name": candidate.apk_name,
            "apk_sha256": candidate.apk_sha256,
            "aab_name": candidate.aab_name,
            "aab_sha256": candidate.aab_sha256,
        },
        "p0_matrix_verdict": matrix.get("verdict"),
        "release": {
            "id": release.get("id"),
            "draft": release.get("draft"),
            "prerelease": release.get("prerelease"),
            "asset_names": sorted(asset_names(release)),
        },
        "checks": [
            "candidate checksums, metadata and signatures are verified",
            "P0 matrix is PASS with exact API 26 / 33 / 35 and a full session",
            "tag resolves to the candidate source commit",
            "release remains the expected prerelease",
            "release contains only the six approved assets",
            "release body and every asset exactly match the P0-tested candidate",
        ],
        "sensitive_values_included": False,
    }


def render(payload: dict[str, Any], path: pathlib.Path) -> None:
    candidate = payload["candidate"]
    lines = [
        "# Android V2 RC prerelease 发布门禁报告", "",
        f"> 门禁结论：**{payload['verdict']}**  ",
        f"> 操作：**{payload['operation']}**", "",
        "| 项目 | 值 |", "|---|---|",
        f"| 版本 | `{candidate['version_name']}` (`{candidate['version_code']}`) |",
        f"| Tag | `{payload['tag']}` |",
        f"| Source commit | `{candidate['source_commit']}` |",
        f"| Certificate SHA-256 | `{candidate['certificate_sha256']}` |",
        f"| APK SHA-256 | `{candidate['apk_sha256']}` |",
        f"| AAB SHA-256 | `{candidate['aab_sha256']}` |",
        f"| P0 matrix | `{payload['p0_matrix_verdict']}` |",
        f"| Release draft | `{payload['release']['draft']}` |",
        f"| Release prerelease | `{payload['release']['prerelease']}` |", "",
        "## 校验", "",
    ]
    lines.extend(f"- [x] {item}" for item in payload["checks"])
    lines += [
        "", "该报告不包含 keystore、密码、Token、服务器地址或设备原始日志。",
        "公开 prerelease 的附件与完成 P0 的候选目录逐字节一致。", "",
        f"生成时间：`{payload['generated_at_utc']}`",
    ]
    path.write_text("\n".join(lines) + "\n", encoding="utf-8")


def save(args: argparse.Namespace, operation: str, candidate: Candidate, matrix: dict[str, Any], release: dict[str, Any]) -> None:
    output = pathlib.Path(args.output_dir).resolve()
    output.mkdir(parents=True, exist_ok=True)
    tag = args.tag or f"android-v2-v{candidate.version_name}"
    payload = report(operation, args.repository, tag, candidate, matrix, release)
    write_json(output / "release-promotion-verification.json", payload)
    render(payload, output / "RELEASE_PROMOTION_REPORT.md")
    print(f"Release promotion report: {output / 'RELEASE_PROMOTION_REPORT.md'}")


def local(args: argparse.Namespace) -> tuple[Candidate, dict[str, Any], str]:
    candidate = verify_candidate(pathlib.Path(args.candidate_dir), args.version)
    matrix = verify_matrix(pathlib.Path(args.matrix_dir), candidate)
    return candidate, matrix, args.tag or f"android-v2-v{candidate.version_name}"


def verify(args: argparse.Namespace) -> int:
    candidate, matrix, tag = local(args)
    release = verify_remote(client(args), tag, candidate, draft=True)
    save(args, "verify", candidate, matrix, release)
    print("Draft prerelease exactly matches the P0-tested candidate")
    return 0


def sync(args: argparse.Namespace) -> int:
    candidate, matrix, tag = local(args)
    if args.confirm_version != candidate.version_name:
        fail(f"--confirm-version must equal {candidate.version_name}")
    gh = client(args)
    if resolve_tag_commit(gh, tag) != candidate.source_commit:
        fail("refusing to sync because the tag does not resolve to the candidate commit")
    release = release_metadata(gh, tag, draft=True)
    sync_release(gh, tag, candidate, release)
    release = verify_remote(gh, tag, candidate, draft=True)
    save(args, "sync", candidate, matrix, release)
    print("Draft prerelease assets now exactly match the P0-tested candidate")
    return 0


def publish(args: argparse.Namespace) -> int:
    candidate, matrix, tag = local(args)
    if args.confirm_version != candidate.version_name:
        fail(f"--confirm-version must equal {candidate.version_name}")
    gh = client(args)
    release = verify_remote(gh, tag, candidate, draft=True)
    gh.patch_release(int(release["id"]), [("draft", "false"), ("prerelease", "true")])
    published = verify_remote(gh, tag, candidate, draft=False)
    save(args, "publish", candidate, matrix, published)
    print(f"Published prerelease {tag}")
    return 0


def common(item: argparse.ArgumentParser) -> None:
    item.add_argument("--candidate-dir", required=True)
    item.add_argument("--matrix-dir", required=True)
    item.add_argument("--output-dir", required=True)
    item.add_argument("--repository", default=DEFAULT_REPOSITORY)
    item.add_argument("--version", default="0.1.0-rc.1")
    item.add_argument("--tag")
    item.add_argument("--gh", help=argparse.SUPPRESS)


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description="Fail-closed Android V2 prerelease promotion gate")
    sub = root.add_subparsers(dest="command", required=True)
    common(sub.add_parser("verify", help="read-only verification"))
    item = sub.add_parser("sync", help="replace draft assets with the P0-tested candidate")
    common(item)
    item.add_argument("--confirm-version", required=True)
    item = sub.add_parser("publish", help="publish a fully verified draft prerelease")
    common(item)
    item.add_argument("--confirm-version", required=True)
    sub.add_parser("self-test", help="run the offline fake-gh test")
    return root


def main() -> int:
    args = parser().parse_args()
    if args.command == "verify":
        return verify(args)
    if args.command == "sync":
        return sync(args)
    if args.command == "publish":
        return publish(args)
    if args.command == "self-test":
        return self_test()
    fail(f"unsupported command: {args.command}")
