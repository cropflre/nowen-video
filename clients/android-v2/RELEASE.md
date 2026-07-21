# Android V2 Release Guide

本文档说明 Android V2 的正式签名、GitHub Actions Secret、本地构建和发布流程。

## 安全原则

- keystore、密码和私钥不得提交到 Git。
- 正式 keystore 应离线备份；遗失后无法为同一 applicationId 发布可覆盖升级的新版本。
- 普通 `Android V2` CI 不读取正式密钥，只构建未签名 Release APK。
- `release-android-v2` 在 Pull Request 中使用一次性临时 keystore，仅用于验证签名配置和 APK/AAB 验签链路。
- 只有手动运行或推送 Android 专属 tag 时才读取正式 Secrets。

## 生成正式 keystore

示例命令：

```bash
keytool -genkeypair -v \
  -keystore nowen-video-android-v2-release.jks \
  -alias nowen-video-android-v2 \
  -keyalg RSA \
  -keysize 4096 \
  -validity 10000
```

请妥善记录：

- keystore 文件
- keystore 密码
- key alias
- key 密码

仓库 `.gitignore` 已忽略 `*.keystore` 和 `*.jks`，但仍不要把密钥放进仓库目录长期保存。

## 配置 GitHub Actions Secrets

将 keystore 转为单行 Base64：

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

在 GitHub 仓库的 Actions Secrets 中配置：

| Secret | 内容 |
|---|---|
| `ANDROID_V2_KEYSTORE_BASE64` | keystore 的单行 Base64 |
| `ANDROID_V2_KEYSTORE_PASSWORD` | keystore 密码 |
| `ANDROID_V2_KEY_ALIAS` | key alias |
| `ANDROID_V2_KEY_PASSWORD` | key 密码 |

workflow 会把 keystore 解码到 GitHub Runner 的临时目录，构建结束后随 Runner 销毁；密码不会写入 artifact。

## 本地签名构建

设置以下环境变量：

```bash
export ANDROID_SIGNING_STORE_FILE="$HOME/keys/nowen-video-android-v2-release.jks"
export ANDROID_SIGNING_STORE_PASSWORD='replace-me'
export ANDROID_SIGNING_KEY_ALIAS='nowen-video-android-v2'
export ANDROID_SIGNING_KEY_PASSWORD='replace-me'
export ANDROID_VERSION_NAME='0.1.0-rc.1'
export ANDROID_VERSION_CODE='1'
```

执行：

```bash
./android/gradlew -p clients/android-v2 \
  clean testDebugUnitTest lintDebug assembleRelease bundleRelease
```

产物：

```text
clients/android-v2/app/build/outputs/apk/release/app-release.apk
clients/android-v2/app/build/outputs/bundle/release/app-release.aab
```

四项签名变量必须全部设置或全部不设置。只设置其中一部分时 Gradle 会直接失败，避免误以为生成了正式签名包。

## 手动构建正式 artifact

在 GitHub Actions 中运行 `release-android-v2`，填写：

- `version_name`：例如 `0.1.0-rc.1`
- `version_code`：正整数，并且必须高于已发布版本

手动运行会：

1. 校验四项签名 Secret；
2. 执行单元测试和 Lint；
3. 构建签名 APK 与 AAB；
4. 使用 `apksigner` 和 `jarsigner` 验证签名；
5. 生成 `SHA256SUMS.txt`；
6. 上传保存 30 天的 workflow artifact。

手动运行不会自动创建 GitHub Release。

## 通过 tag 创建草稿 Release

Android V2 使用独立 tag，避免触发现有桌面端 `v*.*.*` 发布流程：

```bash
git tag android-v2-v0.1.0
git push origin android-v2-v0.1.0
```

支持带后缀的版本名，例如：

```text
android-v2-v0.1.0-rc.1
```

workflow 从 tag 解析 `versionName`，并按以下规则生成 `versionCode`：

```text
MAJOR * 1,000,000 + MINOR * 1,000 + PATCH
```

例如 `android-v2-v1.2.3` 对应 `versionCode = 1002003`。RC 后缀不改变 versionCode，因此同一 `MAJOR.MINOR.PATCH` 不应重复发布多个需要覆盖安装的 RC 包；正式版本策略将在版本迁移任务中进一步收口。

Tag 构建成功后会创建草稿 GitHub Release，包含：

- 签名 APK
- 签名 AAB
- `SHA256SUMS.txt`

发布前仍需完成真机安装、覆盖升级和关键播放链路回归。

## 验证产物

APK：

```bash
apksigner verify --verbose --print-certs nowen-video-android-v2-0.1.0.apk
```

AAB：

```bash
jarsigner -verify -strict nowen-video-android-v2-0.1.0.aab
```

SHA-256：

```bash
sha256sum -c SHA256SUMS.txt
```
