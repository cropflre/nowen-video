# Android V2 RC 候选包助手

本指南用于在 **Linux / WSL** 中安全生成 Android V2 正式候选包。完整签名规则见 [RELEASE.md](./RELEASE.md)，真机放行见 [SMOKE_TEST.md](./SMOKE_TEST.md)。

候选包助手：

```text
scripts/android-v2-release-candidate.sh
```

它不会读取或输出私钥内容，也不会把密码写入报告。默认只执行本地签名预检，不会触发 GitHub Actions、推送 tag 或公开 Release。

## 运行前准备

需要：

- JDK 17：`keytool`、`jar`、`jarsigner`
- Python 3
- Git
- GitHub CLI `gh`
- `sha256sum`
- 已登录 GitHub CLI：`gh auth status`
- 仓库处于干净的 `main`，且 `HEAD == origin/main`

APK 本地密码学验签还需要 Android SDK Build Tools 中的 `apksigner`。缺少 `apksigner` 时，workflow 内仍会强制验签，但本地报告会明确记录 APK 验签未执行。

## 第一次：生成长期 keystore

下面的命令会隐藏读取并二次确认密码；keystore 不存在时才会创建：

```bash
git checkout main
git pull --ff-only

bash scripts/android-v2-release-candidate.sh \
  --version 0.1.0-rc.1 \
  --keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  --alias nowen-video-android-v2 \
  --generate-keystore
```

此模式只做：

1. 创建或复用长期 keystore；
2. 检查分支、工作区、远端 main 和 tag 冲突；
3. 验证 store password、alias、key password 与私钥签名；
4. 提取证书 SHA-256；
5. 生成不含敏感值的本地预检报告。

不会配置 GitHub Secrets，也不会开始远程构建。

生成后必须先完成至少一份异盘或离线加密备份。不要把 keystore 放进仓库、Issue、聊天或云盘公开目录。

## 配置 Secrets 并生成正式候选包

确认 keystore 已备份后执行：

```bash
bash scripts/android-v2-release-candidate.sh \
  --version 0.1.0-rc.1 \
  --keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  --alias nowen-video-android-v2 \
  --repository cropflre/nowen-video \
  --output-dir "$PWD/dist/android-v2/rc1-candidate" \
  --configure-secrets \
  --dispatch
```

助手会：

1. 隐藏读取密码；
2. 本地预检长期 keystore；
3. 安全配置五项 GitHub Actions Secrets；
4. 为本次请求生成唯一 `request_id`；
5. 触发 `release-android-v2`；
6. 只选择同时满足“新 run ID、当前 main commit、当前 request_id”的 workflow run；
7. 等待 workflow 成功；
8. 下载 `nowen-video-android-v2-release` artifact；
9. 校验哈希、manifest、预检报告、Release Notes、版本、commit 和证书指纹；
10. 在本机工具可用时再次验证 APK/AAB 签名；
11. 生成 `candidate-verification.json`。

`--dispatch` 强制要求同时使用 `--configure-secrets`，避免远程构建意外复用过期或错误的签名 Secrets。

## 候选包目录

成功后目录包含：

```text
nowen-video-android-v2-0.1.0-rc.1.apk
nowen-video-android-v2-0.1.0-rc.1.aab
SHA256SUMS.txt
release-manifest.json
signing-preflight.json
RELEASE_NOTES.md
workflow-run-id.txt
workflow-request-id.txt
candidate-verification.json
```

本地预检报告写在候选目录旁：

```text
<output-dir>.signing-preflight-local.json
```

这些 JSON 报告不包含密码、keystore Base64 或私钥。

## 只核验已经下载的候选包

```bash
bash scripts/android-v2-release-candidate.sh \
  --version 0.1.0-rc.1 \
  --verify-dir "$PWD/dist/android-v2/rc1-candidate" \
  --expected-commit '<main commit SHA>' \
  --expected-fingerprint '<certificate SHA-256>'
```

以下任一情况都会失败：

- 文件缺失或哈希损坏；
- versionName / versionCode 不一致；
- applicationId 不是 `com.nowen.video.v2`；
- minSdk / targetSdk 不是 26 / 35；
- manifest 或预检报告 commit 不一致；
- keystore、APK 或 manifest 证书指纹不一致；
- Release Notes 缺失版本、commit、证书指纹或仍有模板占位符。

## 验证后创建 RC tag

只有正式候选包完整核验通过后，才能显式添加 `--create-tag`：

```bash
bash scripts/android-v2-release-candidate.sh \
  --version 0.1.0-rc.1 \
  --keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  --alias nowen-video-android-v2 \
  --repository cropflre/nowen-video \
  --output-dir "$PWD/dist/android-v2/rc1-candidate" \
  --configure-secrets \
  --dispatch \
  --create-tag
```

助手会在候选包核验成功后创建并推送：

```text
android-v2-v0.1.0-rc.1
```

Tag workflow 会创建 **草稿 prerelease**。助手不会把草稿改为公开，也不会跳过 [SMOKE_TEST.md](./SMOKE_TEST.md) 的实体设备 P0 放行。

## 安全停止条件

出现以下情况时不要创建 tag：

- workflow 失败；
- 证书指纹不一致；
- source commit 不是预期 main；
- 哈希校验失败；
- APK/AAB 验签失败；
- keystore 尚未完成独立备份；
- 实体设备存在崩溃、覆盖升级失败、数据串号、下载损坏或凭据风险。

## 脚本自测

```bash
bash scripts/android-v2-release-candidate.sh --self-test
```

自测完全离线执行，会验证：

- 长期 keystore 生成命令；
- 生成后的签名预检；
- 候选包目录结构；
- SHA-256 校验；
- manifest、预检报告和 Release Notes 一致性；
- 错误 source commit 必须被拒绝。
