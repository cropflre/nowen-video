# Android Release Guide

Nowen Video 只维护一个正式 Android 客户端：`android/`。

## Production 签名基线

旧 Android V1 的 keystore 已彻底退出发布链路。当前正式 Android 使用新的独立 production key，并从首次正式签名版本开始永久沿用。

固定签名证书：

```text
applicationId: com.nowen.video
key alias: nowen-video
certificate SHA-256: 07ac3f214fbb8ac44e85fa1f65610dcbcff8fe04876c417364c3905dfb8b6bcd
```

公开证书位于 `android/signing/production-cert.pem`。证书本身不是秘密；**私钥 keystore 和密码才是必须离线保管的资产**。

正式发布流水线已经固定上述 SHA-256。GitHub Secrets 如果被误配成另一把 key，签名预检和 APK 二次校验都会直接失败，避免产生无法覆盖升级的正式包。

需要四项 GitHub Actions Secrets：

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

不得把 keystore、密码或 Base64 私钥内容提交到 Git、Issue、PR、聊天群或公开网盘。

## 首次配置 / 恢复 GitHub Secrets

production key 已经建立，**不要再生成新 key**。准备原始 keystore 及密码备份后执行：

```bash
bash scripts/android-signing-bootstrap.sh \
  --keystore '/安全路径/nowen-video-android-production.jks' \
  --secrets-env '/安全路径/nowen-video-android-production.secrets.env' \
  --repository cropflre/nowen-video \
  --set-github-secrets
```

脚本会先验证 keystore 的证书 SHA-256 必须等于仓库固定指纹，然后才允许写入 GitHub Secrets。不同证书会被拒绝。

如果不使用 `.secrets.env`，也可以先导出：

```bash
export ANDROID_KEYSTORE_PASSWORD='原始 keystore 密码'
export ANDROID_KEY_PASSWORD='原始 key 密码'

bash scripts/android-signing-bootstrap.sh \
  --keystore '/安全路径/nowen-video-android-production.jks' \
  --repository cropflre/nowen-video \
  --set-github-secrets
```

至少离线备份两份：

- `nowen-video-android-production.jks`
- `nowen-video-android-production.secrets.env`
- `nowen-video-android-production.certificate-sha256.txt`

**不得重新生成或轮换 production key。** 丢失 keystore 或密码会导致后续 `com.nowen.video` 正式版本无法对当前新版 Android 做覆盖升级。

## CI 验证

Android CI：`.github/workflows/android.yml`

设备冒烟：`.github/workflows/android-device-smoke.yml`

正式签名候选：手动运行 `.github/workflows/release-android.yml`，输入目标 `version_name`。手动运行只构建并保存签名产物，不创建 Release。

候选产物必须同时满足：

- `applicationId = com.nowen.video`
- APK / AAB 都有有效签名
- APK 签名证书 SHA-256 与 production 基线完全一致
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
