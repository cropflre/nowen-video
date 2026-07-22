# Android V2 RC 草稿 prerelease 发布门禁

本指南用于在正式候选包完成实体设备 P0 后，将 **实际测试过的候选 APK/AAB** 同步到 GitHub 草稿 prerelease，并在最终校验通过后显式公开。

入口：

```text
scripts/android-v2-release-promote.py
```

## 为什么需要资产同步

Android RC 流程分为两次构建：

1. 手动 `release-android-v2` workflow 生成正式候选包，供实体设备 P0；
2. 推送 tag 后，tag workflow 再次构建并创建草稿 prerelease。

即使两次构建使用相同 source commit 和长期证书，APK/AAB 也不应默认视为字节级可复现。公开附件必须与 P0 实际测试的候选目录完全一致。

发布门禁会比较以下六个文件的 SHA-256：

```text
nowen-video-android-v2-<version>.apk
nowen-video-android-v2-<version>.aab
SHA256SUMS.txt
release-manifest.json
signing-preflight.json
RELEASE_NOTES.md
```

任何文件不同都不允许公开。

## 前置条件

必须已经完成：

- 长期 release keystore 和备份；
- 五项正式 GitHub Actions Secrets；
- 正式候选目录的 `candidate-verification.json`；
- APK 和 AAB 本地验签；
- API 26、33、35 P0 matrix 为 `PASS`；
- tag workflow 成功；
- GitHub Release 仍为 `draft=true`、`prerelease=true`；
- GitHub CLI 已登录并拥有 Release 写权限。

目录示例：

```text
dist/android-v2/rc1-candidate/
dist/android-v2/p0/matrix/
```

## 安全边界

- `verify` 只读，不修改 Release；
- `sync` 只覆盖仍处于草稿状态的 prerelease 附件和正文；
- `publish` 只公开已经重新验证的草稿 prerelease；
- 不创建 tag；
- 不读取 keystore、密码或 GitHub Secrets；
- 已公开 Release 不允许同步资产；
- stable Release 不允许通过该工具发布；
- 出现 Debug、unsigned、keystore、凭据或未知附件时立即失败。

## 1. 只读验证草稿

```bash
python3 scripts/android-v2-release-promote.py verify \
  --candidate-dir "$PWD/dist/android-v2/rc1-candidate" \
  --matrix-dir "$PWD/dist/android-v2/p0/matrix" \
  --version 0.1.0-rc.1 \
  --tag android-v2-v0.1.0-rc.1 \
  --repository cropflre/nowen-video \
  --output-dir "$PWD/dist/android-v2/promotion/verify"
```

该命令验证：

1. 候选 manifest、checksums、签名状态和 Release Notes；
2. P0 matrix 为 PASS；
3. matrix 精确覆盖 API 26、33、35；
4. 至少存在一个 `full` 会话；
5. candidate 与 matrix 的版本、commit、证书和 APK SHA-256 一致；
6. tag 最终解析到候选 source commit，包括 annotated tag；
7. Release 仍为草稿 prerelease；
8. 附件名称严格匹配六文件 allowlist；
9. Release 正文与候选 `RELEASE_NOTES.md` 一致；
10. 每个 Release 附件与 P0 候选文件逐字节一致。

首次执行很可能会因为 tag workflow 重新构建的 APK/AAB 与候选包不同而失败。这是预期的 fail-closed 行为。

## 2. 显式同步 P0 候选资产

确认目标版本后执行：

```bash
python3 scripts/android-v2-release-promote.py sync \
  --candidate-dir "$PWD/dist/android-v2/rc1-candidate" \
  --matrix-dir "$PWD/dist/android-v2/p0/matrix" \
  --version 0.1.0-rc.1 \
  --tag android-v2-v0.1.0-rc.1 \
  --repository cropflre/nowen-video \
  --output-dir "$PWD/dist/android-v2/promotion/sync" \
  --confirm-version 0.1.0-rc.1
```

`sync` 会：

- 再次验证候选、P0 matrix、tag 和草稿状态；
- 使用 `gh release upload --clobber` 覆盖六个批准附件；
- 使用候选 `RELEASE_NOTES.md` 覆盖 Release 正文；
- 保持 `draft=true`；
- 保持 `prerelease=true`；
- 重新下载所有附件；
- 再次做逐字节校验；
- 生成发布门禁报告。

不会公开 Release。

## 3. 再次只读验证

同步完成后建议再次执行 `verify`。成功时会生成：

```text
release-promotion-verification.json
RELEASE_PROMOTION_REPORT.md
```

报告不包含 keystore、密码、Token、服务器地址或设备原始日志。

## 4. 显式公开 prerelease

只有候选、P0 matrix、tag、Release 正文和六个附件全部通过后才执行：

```bash
python3 scripts/android-v2-release-promote.py publish \
  --candidate-dir "$PWD/dist/android-v2/rc1-candidate" \
  --matrix-dir "$PWD/dist/android-v2/p0/matrix" \
  --version 0.1.0-rc.1 \
  --tag android-v2-v0.1.0-rc.1 \
  --repository cropflre/nowen-video \
  --output-dir "$PWD/dist/android-v2/promotion/publish" \
  --confirm-version 0.1.0-rc.1
```

`publish` 会在公开前执行完整只读门禁，然后将：

```text
draft=false
prerelease=true
```

公开后再次读取 Release、下载附件并验证，最后生成 `publish` 报告。

## 明确失败条件

以下任一情况都会拒绝同步或发布：

- candidate verification 未记录 APK/AAB 验签通过；
- P0 matrix 不是 PASS；
- API 26、33、35 覆盖不完整；
- 没有 `full` P0 会话；
- candidate 与 matrix 的 commit、证书、版本或 APK 哈希不同；
- tag 指向错误提交；
- Release 已公开；
- Release 不是 prerelease；
- 附件缺失或存在未知附件；
- 附件名称出现 Debug、unsigned、keystore、secret、token 等危险特征；
- Release 正文不是候选 Release Notes；
- 任一附件与实际 P0 候选目录不同；
- `--confirm-version` 与候选版本不一致。

## 离线自测

```bash
python3 -Werror scripts/android-v2-release-promote.py self-test
```

fake-gh 自测覆盖：

- 模拟 tag workflow 重新构建不同 APK；
- 只读验证必须拒绝不同产物；
- 显式同步测试过的候选附件；
- 同步后逐字节验证；
- 显式发布 prerelease；
- 错误 matrix commit 拒绝；
- 错误 tag commit 拒绝。
