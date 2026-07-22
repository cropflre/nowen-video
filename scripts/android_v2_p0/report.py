from __future__ import annotations

import argparse
import pathlib
import shutil
from typing import Any

from .common import PRODUCT, SENSITIVE, command, fail, load_json, write_json
from .session import blockers, now


def _md(value: Any) -> str:
    return str(value).replace("|", "\\|").replace("\n", "<br>")


def render_session(payload: dict[str, Any], output: pathlib.Path) -> None:
    candidate, device, final = payload["candidate"], payload["device"], payload["final"]
    lines = [
        "# Android V2 RC 真机 P0 会话报告", "",
        f"> 会话结论：**{final['verdict']}**  ", f"> 测试范围：**{payload['scope']}**  ",
        "> 单设备 PASS 不等于 RC 已放行；仍需同候选版本的 API 26 / 33 / 35 矩阵与发布负责人确认。", "",
        "## 候选包", "", "| 项目 | 值 |", "|---|---|",
        f"| 版本 | `{candidate['version_name']}` (`{candidate['version_code']}`) |",
        f"| source commit | `{candidate['source_commit']}` |",
        f"| certificate SHA-256 | `{candidate['certificate_sha256']}` |",
        f"| APK / SHA-256 | `{candidate['apk_name']}` / `{candidate['apk_sha256']}` |",
        f"| workflow run | `{candidate.get('workflow_run_id') or 'unknown'}` |", "",
        "## 测试环境", "", "| 项目 | 值 |", "|---|---|",
        f"| 测试人 | {_md(payload['environment']['tester'])} |",
        f"| 服务器版本 | {_md(payload['environment']['server_version'])} |",
        f"| 网络类型 | {payload['environment']['network_profile']} |",
        f"| 设备 | {_md(device['manufacturer'])} {_md(device['model'])} |",
        f"| Android / API | {_md(device['android_version'])} / {device['api_level']} |",
        f"| ABI | {_md(device['abi'])} |",
        f"| 分辨率 / density | {_md(device['screen_size'])} / {_md(device['density'])} |",
        f"| 设备标识哈希前缀 | `{device['serial_sha256_prefix']}` |",
        f"| build fingerprint | `{_md(device['build_fingerprint'])}` |", "",
        "## 自动检查", "", "| ID | 状态 | 检查项 | 结果 |", "|---|---|---|---|",
    ]
    for item in payload["automatic_checks"]:
        lines.append(f"| {item['id']} | **{item['status']}** | {_md(item['title'])} | {_md(item['detail'])} |")
    lines += ["", "## 人工 P0", ""]
    section = None
    for item in payload["manual_cases"]:
        if item["section"] != section:
            section = item["section"]
            lines += [f"### {section}", "", "| ID | 状态 | 用例 | 备注 |", "|---|---|---|---|"]
        lines.append(f"| {item['id']} | **{item['status']}** | {_md(item['title'])} | {_md(item.get('note',''))} |")
    lines += ["", "## 阻断摘要", ""]
    lines += [f"- {_md(item)}" for item in final.get("blocking_items", [])] or ["- 无"]
    lines += [
        "", "## 本地证据", "", f"证据目录：`{payload['evidence']['directory']}`", "",
        "原始 logcat、截图和 UI XML 不得直接上传公开 Issue；必须先检查并脱敏。此报告不包含账号、Token、服务器地址、密码或私钥。", "",
        f"更新时间：`{payload['updated_at_utc']}`",
    ]
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def matrix(args: argparse.Namespace) -> int:
    directories = [pathlib.Path(value).resolve() for value in args.session_dir]
    if len(directories) < 3:
        fail("matrix requires at least three session directories")
    sessions: list[dict[str, Any]] = []
    for directory in directories:
        payload = load_json(directory / "p0-session.json")
        if not isinstance(payload, dict):
            fail(f"invalid session payload: {directory}")
        verdict, why = blockers(payload)
        if verdict != "PASS" or payload.get("final", {}).get("verdict") != "PASS":
            fail(f"session is not finalized PASS: {directory} ({verdict}; {', '.join(why[:3])})")
        sessions.append(payload)
    anchor = sessions[0]["candidate"]
    for payload in sessions[1:]:
        for key in ("version_name", "version_code", "source_commit", "certificate_sha256", "apk_sha256"):
            if payload["candidate"].get(key) != anchor.get(key):
                fail(f"matrix sessions do not use the same candidate: {key}")
    observed = {int(payload["device"]["api_level"]) for payload in sessions}
    missing = [api for api in (26, 33, 35) if api not in observed]
    full_count = sum(payload.get("scope") == "full" for payload in sessions)
    blocking = []
    if missing:
        blocking.append("missing exact API session(s): " + ", ".join(map(str, missing)))
    if full_count == 0:
        blocking.append("at least one finalized full-scope session is required")
    verdict = "PASS" if not blocking else "BLOCKED"
    output = pathlib.Path(args.output_dir).resolve()
    output.mkdir(parents=True, exist_ok=True)
    report = {
        "schema_version": 1, "product": PRODUCT, "generated_at_utc": now(), "verdict": verdict,
        "candidate": anchor,
        "requirements": {
            "exact_api_levels": [26, 33, 35], "observed_api_levels": sorted(observed),
            "full_scope_session_required": True, "full_scope_session_count": full_count,
        },
        "sessions": [{
            "session_id": payload["session_id"], "scope": payload["scope"],
            "api_level": payload["device"]["api_level"],
            "device": f"{payload['device']['manufacturer']} {payload['device']['model']}",
            "serial_sha256_prefix": payload["device"]["serial_sha256_prefix"],
            "verdict": payload["final"]["verdict"],
        } for payload in sessions],
        "blocking_items": blocking,
        "meaning": "Matrix PASS confirms recorded P0 coverage for this candidate; public release still requires release-owner approval.",
        "sensitive_values_included": False,
    }
    write_json(output / "p0-matrix.json", report)
    render_matrix(report, output / "P0_MATRIX_REPORT.md")
    print(f"P0 matrix verdict: {verdict}")
    return 0 if verdict == "PASS" else 3


def render_matrix(payload: dict[str, Any], output: pathlib.Path) -> None:
    candidate = payload["candidate"]
    lines = [
        "# Android V2 RC P0 设备矩阵报告", "",
        f"> 矩阵结论：**{payload['verdict']}**  ",
        "> 矩阵 PASS 不会自动公开 Release；仍需发布负责人检查草稿附件、已知问题和脱敏记录。", "",
        "## 候选包", "",
        f"- 版本：`{candidate['version_name']}` (`{candidate['version_code']}`)",
        f"- Commit：`{candidate['source_commit']}`",
        f"- 证书：`{candidate['certificate_sha256']}`",
        f"- APK SHA-256：`{candidate['apk_sha256']}`", "",
        "## 会话", "", "| Session | Scope | API | Device | Verdict |", "|---|---|---:|---|---|",
    ]
    for item in payload["sessions"]:
        lines.append(f"| `{item['session_id']}` | {item['scope']} | {item['api_level']} | {_md(item['device'])} (`{item['serial_sha256_prefix']}`) | **{item['verdict']}** |")
    req = payload["requirements"]
    lines += [
        "", "## 覆盖要求", "",
        f"- 精确 API 26 / 33 / 35：观测到 `{req['observed_api_levels']}`",
        f"- 至少一台完整 P0：`{req['full_scope_session_count']}` 个 full session", "",
        "## 阻断摘要", "",
    ]
    lines += [f"- {_md(item)}" for item in payload["blocking_items"]] or ["- 无"]
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")


def post(args: argparse.Namespace) -> None:
    report = pathlib.Path(args.report).resolve()
    if not report.is_file():
        fail(f"report not found: {report}")
    text = report.read_text(encoding="utf-8")
    if "**PENDING**" in text:
        fail("refusing to post a PENDING report")
    if any(pattern.search(text) for pattern in SENSITIVE):
        fail("report appears to contain a URL, IP address, credential, token, or secret")
    tool = shutil.which("gh")
    if not tool:
        fail("required command not found: gh")
    result = command([tool, "issue", "comment", str(args.issue), "--repo", args.repository, "--body-file", str(report)])
    if result.returncode:
        fail(f"unable to post report: {(result.stdout or '')}{(result.stderr or '')}")
    print(f"Posted redacted report to {args.repository}#{args.issue}")
