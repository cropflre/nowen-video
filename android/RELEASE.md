# Android Release Guide

Nowen Video 只维护一个正式 Android 客户端：`android/`。

## 签名原则

旧 Android V1 的 keystore 不再作为发布约束。当前正式 Android 使用一份全新的长期 production keystore；从首次正式签名版本开始，这把 key 必须永久保留。

需要四项 GitHub Actions Secrets：

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

不得把 keystore、密码或 Base64 私钥内容提交到 Git、Issue、PR、聊天群或公开网盘。

## 首次建立 production 签名基线

推荐只执行一次 bootstrap。脚本会生成 RSA 4096 production keystore、随机强密码、证书 SHA-256 指纹和本地 secret 备份；加上 `--set-github-secrets` 后会通过 `gh` CLI 写入仓库 Actions Secrets。

```bash
bash scripts/android-signing-bootstrap.sh \
  --output "$HOME/keys/nowen-video-android-production.jks" \
  --repository cropflre/nowen-video \
  --set-github-secrets
```

前置条件：本机已安装 JDK、OpenSSL、GitHub CLI，并且 `gh auth login` 已登录有仓库管理权限的账号。

bootstrap 成功后至少离线备份两份：

- `nowen-video-android-production.jks`
- `nowen-video-android-production.secrets.env`
- `nowen-video-android-production.certificate-sha256.txt`

**首次正式签名版本发布之后，不得重新生成 production key。** 丢失 keystore 或密码会导致后续 `com.nowen.video` 正式版本无法对当前新版 Android 做覆盖升级。

## 已有 production keystore 时重新配置 GitHub Secrets

如果只是换电脑、GitHub Secrets 丢失或需要重新配置，不要生成新 key。使用原始 keystore 和密码执行：

```bash
export ANDROID_KEYSTORE_PASSWORD='原始 keystore 密码'
export ANDROID_KEY_PASSWORD='原始 key 密码'

bash scripts/android-signing-preflight.sh \
  --version 1.2.9 \
  --keystore '/安全路径/nowen-video-android-production.jks' \
  --alias 'nowen-video' \
  --repository cropflre/nowen-video \
  --report dist/android/signing-preflight.json \
  --set-github-secrets
```

## CI 验证

Android CI：`.github/workflows/android.yml`

设备冒烟：`.github/workflows/android-device-smoke.yml`

正式签名候选：手动运行 `.github/workflows/release-android.yml`，输入目标 `version_name`。手动运行只构建并保存签名产物，不创建 Release。

候选产物必须同时满足：

- `applicationId = com.nowen.video`
- APK / AAB 都有有效签名
- `versionName` / `versionCode` 与发布输入一致
- APK、AAB、release manifest 和 SHA256SUMS 相互一致
- 签名预检报告不包含密码、私钥或 Token

## 正式发布

确认 Android CI、设备冒烟和手动签名候选均通过后，创建统一产品 tag：

```bash
git checkout main
git pull --ff-only
git tag -a v1.2.9 -m 'Nowen Video v1.2.9'
git push origin v1.2.9
```

发布流水线会生成 APK、AAB、SHA256SUMS、release manifest、签名预检报告和 Release Notes，并将 GitHub Release 保持为 Draft 供最终人工确认。
