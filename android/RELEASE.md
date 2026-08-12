# Android Release Guide

Nowen Video 只维护一个正式 Android 客户端：`android/`。

## 签名原则

旧 Android V1 的 keystore 不再作为发布约束。请为当前正式 Android 选择一份长期生产 keystore，并从首次正式发布开始永久保留。

需要四项 GitHub Actions Secrets：

- `ANDROID_KEYSTORE_BASE64`
- `ANDROID_KEYSTORE_PASSWORD`
- `ANDROID_KEY_ALIAS`
- `ANDROID_KEY_PASSWORD`

不得把 keystore、密码或 Base64 私钥内容提交到 Git、Issue、PR 或聊天记录。

## 配置正式签名

```bash
export ANDROID_KEYSTORE_PASSWORD='你的 keystore 密码'
export ANDROID_KEY_PASSWORD='你的 key 密码'

bash scripts/android-signing-preflight.sh \
  --version 1.2.9 \
  --keystore '/安全路径/nowen-video-android-release.jks' \
  --alias 'nowen-video' \
  --repository cropflre/nowen-video \
  --report dist/android/signing-preflight.json \
  --set-github-secrets
```

如果还没有生产 keystore，请在安全位置生成一次并离线备份：

```bash
keytool -genkeypair -v \
  -keystore "$HOME/keys/nowen-video-android-release.jks" \
  -alias nowen-video \
  -keyalg RSA \
  -keysize 4096 \
  -validity 10000
```

## CI 验证

Android CI：`.github/workflows/android.yml`

设备冒烟：`.github/workflows/android-device-smoke.yml`

正式签名候选：手动运行 `.github/workflows/release-android.yml`，输入目标 `version_name`。手动运行只构建并保存签名产物，不创建 Release。

## 正式发布

确认 Android CI、设备冒烟和手动签名候选均通过后，创建统一产品 tag：

```bash
git checkout main
git pull --ff-only
git tag -a v1.2.9 -m 'Nowen Video v1.2.9'
git push origin v1.2.9
```

发布流水线会生成 APK、AAB、SHA256SUMS、release manifest、签名预检报告和 Release Notes，并将 GitHub Release 保持为 Draft 供最终人工确认。
