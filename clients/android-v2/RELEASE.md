# Android V2 Release Guide

本文档说明 Android V2 的正式签名、证书指纹锁定、GitHub Actions Secrets、候选包生成与公开发布流程。

相关文档：

- 普通用户安装与当前能力：[README.md](./README.md)
- 包名、versionCode 与旧版迁移：[MIGRATION.md](./MIGRATION.md)
- Android 8 / 13 / 15 真机放行：[SMOKE_TEST.md](./SMOKE_TEST.md)
- 自动生成的发布说明模板：[RELEASE_NOTES_TEMPLATE.md](./RELEASE_NOTES_TEMPLATE.md)

## 安全原则

- keystore、密码、私钥和 Base64 内容不得提交到 Git。
- 正式 keystore 必须离线备份；遗失后无法继续为 `com.nowen.video.v2` 发布可覆盖升级的新版本。
- RC 与 Stable 必须永久使用同一份 Android V2 release keystore。
- PR 中的临时 keystore 只用于验证构建和验签链路，产物不得分发。
- 非 PR 发布必须同时校验 keystore 证书和最终 APK 证书的 SHA-256 指纹。
- `signing-preflight.json`、`release-manifest.json` 和证书指纹可以公开，但不能包含密码或私钥。

## 1. 生成长期 release keystore

建议在仓库目录之外生成：

```bash
mkdir -p "$HOME/keys/nowen-video"

keytool -genkeypair -v \
  -keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  -alias nowen-video-android-v2 \
  -keyalg RSA \
  -keysize 4096 \
  -validity 10000
```

请离线记录并备份：

- keystore 文件；
- keystore 密码；
- key alias；
- key 密码；
- 证书 SHA-256 指纹；
- 至少一份不与开发机同盘的加密备份。

仓库 `.gitignore` 已忽略 `*.keystore` 和 `*.jks`，但不要把正式密钥长期放在仓库目录中。

## 2. 正式签名预检

`scripts/android-v2-signing-preflight.sh` 会在构建前完成：

- 校验 versionName 并计算 versionCode；
- 检查当前分支必须为 `main`；
- 检查工作区必须干净；
- 拉取并确认 `HEAD == origin/main`；
- 检查目标 Android tag 尚未存在；
- 校验 keystore 文件与 store password；
- 校验 alias 存在；
- 使用临时 JAR 验证 key password 与私钥可用于签名；
- 提取并规范化证书 SHA-256；
- 输出不含敏感值的 JSON 预检报告。

先设置密码环境变量。密码不会作为命令行参数出现在进程列表中：

```bash
export ANDROID_V2_KEYSTORE_PASSWORD='replace-me'
export ANDROID_V2_KEY_PASSWORD='replace-me'
```

执行预检：

```bash
bash scripts/android-v2-signing-preflight.sh \
  --version 0.1.0-rc.1 \
  --keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  --alias nowen-video-android-v2 \
  --report dist/android-v2/signing-preflight.json
```

成功后会显示：

- versionName；
- versionCode；
- source commit；
- 证书 SHA-256。

不会显示或写入：

- keystore 密码；
- key 密码；
- keystore Base64；
- 私钥内容。

自测预检脚本：

```bash
bash scripts/android-v2-signing-preflight.sh --self-test
```

自测会生成一次性 keystore，并确认错误 store password、错误 key password、缺失 alias 和非法指纹都会失败。

## 3. 配置 GitHub Actions Secrets

正式发布需要五项 Secrets：

| Secret | 内容 |
|---|---|
| `ANDROID_V2_KEYSTORE_BASE64` | 正式 keystore 的单行 Base64 |
| `ANDROID_V2_KEYSTORE_PASSWORD` | keystore 密码 |
| `ANDROID_V2_KEY_ALIAS` | 私钥 alias |
| `ANDROID_V2_KEY_PASSWORD` | 私钥密码 |
| `ANDROID_V2_CERTIFICATE_SHA256` | 正式证书 SHA-256，允许有或无冒号 |

推荐由预检脚本直接配置，避免 Base64 或密码出现在终端历史中：

```bash
export ANDROID_V2_KEYSTORE_PASSWORD='replace-me'
export ANDROID_V2_KEY_PASSWORD='replace-me'

bash scripts/android-v2-signing-preflight.sh \
  --version 0.1.0-rc.1 \
  --keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  --alias nowen-video-android-v2 \
  --repository cropflre/nowen-video \
  --report dist/android-v2/signing-preflight.json \
  --set-github-secrets
```

该模式需要已安装并登录 GitHub CLI：

```bash
gh auth status
gh repo view cropflre/nowen-video
```

脚本会通过标准输入配置五项 Secrets，随后只复查 Secret 名称是否存在；GitHub 不允许读取 Secret 值。

手工转换 keystore 时，可使用：

Linux：

```bash
base64 -w 0 nowen-video-android-v2-release.jks
```

macOS：

```bash
base64 < nowen-video-android-v2-release.jks | tr -d '\n'
```

PowerShell：

```powershell
[Convert]::ToBase64String([IO.File]::ReadAllBytes('nowen-video-android-v2-release.jks'))
```

不要把输出粘贴到 Issue、PR、聊天记录或构建日志中。

## 4. 版本规则

允许的 versionName：

```text
MAJOR.MINOR.PATCH-alpha.N
MAJOR.MINOR.PATCH-beta.N
MAJOR.MINOR.PATCH-rc.N
MAJOR.MINOR.PATCH
```

`scripts/android-v2-version.sh` 是 versionCode 的唯一计算来源。workflow 不接受手工 versionCode。

```bash
bash scripts/android-v2-version.sh --self-test
bash scripts/android-v2-version.sh 0.1.0-rc.1
```

RC1 固定为：

```text
versionName = 0.1.0-rc.1
versionCode = 100501
applicationId = com.nowen.video.v2
minSdk = 26
targetSdk = 35
```

升级顺序：

```text
0.1.0-alpha.1 < 0.1.0-beta.1 < 0.1.0-rc.1 < 0.1.0
```

详细公式见 [MIGRATION.md](./MIGRATION.md)。

## 5. 本地签名构建

```bash
export ANDROID_VERSION_NAME='0.1.0-rc.1'
export ANDROID_VERSION_CODE="$(bash scripts/android-v2-version.sh "$ANDROID_VERSION_NAME")"
export ANDROID_SIGNING_STORE_FILE="$HOME/keys/nowen-video/nowen-video-android-v2-release.jks"
export ANDROID_SIGNING_STORE_PASSWORD='replace-me'
export ANDROID_SIGNING_KEY_ALIAS='nowen-video-android-v2'
export ANDROID_SIGNING_KEY_PASSWORD='replace-me'

./android/gradlew -p clients/android-v2 \
  clean testDebugUnitTest lintDebug assembleRelease bundleRelease
```

产物：

```text
clients/android-v2/app/build/outputs/apk/release/app-release.apk
clients/android-v2/app/build/outputs/bundle/release/app-release.aab
```

四项 Gradle 签名变量必须全部设置或全部不设置。只设置一部分时构建会失败。

本地 Gradle 构建不会自动生成完整发布记录；可分发候选包应由 `release-android-v2` workflow 生成。

## 6. 手动验证正式 Secrets

在 GitHub Actions 中手动运行 `release-android-v2`，输入：

```text
0.1.0-rc.1
```

非 PR workflow 会按顺序执行：

1. 自测版本、manifest 和签名预检脚本；
2. 检查五项正式 Secrets；
3. 解码 keystore；
4. 验证 store password、alias、key password 和 keystore 证书指纹；
5. 执行 Android 单元测试、Lint、APK 与 AAB 构建；
6. 验证 APK 与 AAB 签名；
7. 再次验证最终 APK 证书指纹；
8. 从 APK 反读 applicationId、versionName、versionCode、minSdk 和 targetSdk；
9. 生成 `release-manifest.json`、`SHA256SUMS.txt`、`signing-preflight.json` 和 `RELEASE_NOTES.md`；
10. 上传保存 30 天的 workflow artifact。

手动运行不会创建 GitHub Release，适合在推 tag 前验证正式 Secrets。

## 7. 通过 tag 创建草稿 Release

Android V2 使用独立 tag，避免触发现有桌面端 `v*.*.*` 发布流程：

```bash
git checkout main
git pull --ff-only
git status --short
git rev-parse HEAD

bash scripts/android-v2-signing-preflight.sh \
  --version 0.1.0-rc.1 \
  --keystore "$HOME/keys/nowen-video/nowen-video-android-v2-release.jks" \
  --alias nowen-video-android-v2 \
  --expected-fingerprint "$ANDROID_V2_CERTIFICATE_SHA256" \
  --report dist/android-v2/signing-preflight.json

git tag android-v2-v0.1.0-rc.1
git push origin android-v2-v0.1.0-rc.1
```

Tag workflow 成功后会创建草稿 GitHub Release，并自动将含 `-alpha`、`-beta` 或 `-rc` 的版本标记为 prerelease。

草稿附件包含：

- 正式签名 APK；
- 正式签名 AAB；
- `SHA256SUMS.txt`；
- `release-manifest.json`；
- `signing-preflight.json`；
- `RELEASE_NOTES.md`。

Release 正文使用同一次构建生成的 `RELEASE_NOTES.md`，其中版本、commit、证书指纹和 APK/AAB 哈希来自 `release-manifest.json`，不是人工填写。

## 8. 候选包追溯与验证

`release-manifest.json` 记录：

- 发布渠道；
- versionName 与 versionCode；
- applicationId、minSdk 和 targetSdk；
- repository、commit 与 ref；
- workflow event、run ID 与 attempt；
- 最终 APK 证书 SHA-256；
- APK/AAB 文件名、大小和 SHA-256。

`signing-preflight.json` 记录：

- 预检时间；
- source commit 与分支；
- versionName 与 versionCode；
- alias 与 keystore 证书 SHA-256；
- `sensitive_values_included: false`。

校验哈希：

```bash
sha256sum -c SHA256SUMS.txt
```

校验 APK：

```bash
apksigner verify --verbose --print-certs \
  nowen-video-android-v2-0.1.0-rc.1.apk
```

校验 AAB：

```bash
jarsigner -verify nowen-video-android-v2-0.1.0-rc.1.aab
```

校验 JSON：

```bash
python3 -m json.tool release-manifest.json > /dev/null
python3 -m json.tool signing-preflight.json > /dev/null
```

必须确认：

- APK/AAB 哈希与 `SHA256SUMS.txt`、`release-manifest.json` 一致；
- `source.commit` 是本轮候选版本的 main commit；
- keystore、最终 APK、manifest 和 `ANDROID_V2_CERTIFICATE_SHA256` 四处指纹完全一致；
- versionName、versionCode、applicationId 和 SDK 基线符合候选版本；
- 公开附件不包含 Debug APK、未签名 APK、PR 临时签名包、keystore 或凭据。

## 9. RC1 发布前检查

只有以下项目全部完成，才能把草稿 Release 改为公开 prerelease：

- [ ] 长期 release keystore 已离线备份。
- [ ] 五项 Android V2 Secrets 已配置。
- [ ] 本地签名预检通过并保存非敏感报告。
- [ ] 发布 commit 已合并到 `main`，且 `HEAD == origin/main`。
- [ ] `Android V2` 标准门禁全部通过。
- [ ] 手动 `release-android-v2` 使用正式 Secrets 构建成功。
- [ ] Android 8、13、15 自动启动冒烟全部通过。
- [ ] Tag workflow 创建草稿 Release。
- [ ] 同一次 tag 构建生成 APK、AAB、校验和、manifest、预检报告和 Release Notes。
- [ ] keystore 与最终 APK 证书指纹一致，并记录在 RC Issue。
- [ ] Fresh install、旧版并行安装和覆盖升级通过。
- [ ] [SMOKE_TEST.md](./SMOKE_TEST.md) 所有 P0 为 `PASS`，或存在明确批准的非阻断已知问题。
- [ ] Release Notes 包含能力、最低系统、迁移、已知限制、哈希与反馈模板。
- [ ] 回归设备、服务器版本、workflow 链接和测试结论已附到 RC Issue。

出现签名不一致、覆盖升级失败、凭据泄露、数据串号、下载损坏或 P0 崩溃时，必须停止发布并使用更高 versionCode 修复，不能要求测试用户卸载降级。
