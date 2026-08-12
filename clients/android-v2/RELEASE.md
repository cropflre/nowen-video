# Android Release Guide

本文档描述 **Nowen Video Android 正式版** 的签名与发布流程。当前实现源码仍位于 `clients/android-v2/`，但 `V2` 仅是仓库内部的历史目录名，不再代表独立产品。正式 APK 使用历史包名 `com.nowen.video`，用于直接覆盖旧 Android V1。

## 当前发布结论

- 正式 Android 产品：**Nowen Video Android**。
- 正式 applicationId：`com.nowen.video`。
- 最低系统：Android 8.0 / API 26。
- 历史签名信任锚：GitHub Release `v1.2.5` 中的 `nowen-video-android-1.2.5.apk`。
- 当前标签已到 `v1.2.8`，首个 V1 接管版按 `v1.2.9` 准备。
- `android/` 旧客户端仅保留迁移和历史兼容参考，不再作为正式发布客户端。

## 最重要的安全规则

**不要为正式版生成新的 keystore。**

Android 要允许 `com.nowen.video` 从 V1 原位覆盖升级，新 APK 必须由 **与 V1 完全相同的私钥** 签名。只要签名不同，即使 applicationId 一样、版本号更高，Android 也会拒绝覆盖安装，用户只能卸载旧版后重新安装。

因此生产发布只接受：

> 当年用于签署 `v1.2.5` 正式 APK 的那份历史 V1 keystore。

仓库已经有两层 fail-closed 门禁：

1. `.github/workflows/android-signing-identity.yml` 会把仓库配置的 keystore 与公开的 V1 正式 APK 做证书比对；
2. `.github/workflows/release-android-v2.yml` 每次生产构建都会再次从 V1 APK 动态提取证书指纹，只有最终 APK 与 V1 指纹完全一致才允许继续。

证书指纹不再人工复制为 Secret，避免“Secret 中指纹写错但大家以为没问题”的双重真相源。

## 1. 找回历史 V1 keystore

需要找回以下四项：

- V1 正式 keystore 文件（`.jks` / `.keystore`）；
- keystore password；
- key alias；
- key password。

旧 V1 Gradle 配置曾使用这些本地/环境字段：

```text
RELEASE_STORE_FILE
RELEASE_STORE_PASSWORD
RELEASE_KEY_ALIAS
RELEASE_KEY_PASSWORD
```

常见查找位置包括原来发布 Android V1 的开发机、备份盘、密码管理器或 CI 私密配置。**不要把 keystore、密码或 Base64 内容提交到 Git、Issue、PR、聊天记录或构建日志。**

如果历史私钥永久遗失，就不存在技术手段从已发布 APK 反推出私钥，也无法继续做无卸载覆盖升级；这种情况必须作为单独的产品迁移事故处理，不能生成新 key 冒充旧 key。

## 2. 本地验证历史 keystore

在拥有历史 keystore 的可信设备上：

```bash
export ANDROID_KEYSTORE_PASSWORD='你的 V1 keystore 密码'
export ANDROID_KEY_PASSWORD='你的 V1 key 密码'

bash scripts/android-v2-signing-preflight.sh \
  --version 1.2.9 \
  --keystore '/安全路径/nowen-video-release.jks' \
  --alias '你的 V1 alias' \
  --repository cropflre/nowen-video \
  --report dist/android/signing-preflight.json
```

该预检会校验：

- keystore 可以读取；
- alias 存在；
- key password 可以真正完成签名；
- versionName / versionCode 合法；
- 当前源码与发布上下文合法；
- 输出报告不包含私钥和密码。

若要同时强制确认它就是历史 V1 签名 key，使用下一节的 `--set-github-secrets`。该模式会先下载公开 `v1.2.5` APK 做签名比对，**不匹配时不会写任何 GitHub Secret**。

## 3. 配置 GitHub Actions Secrets

正式发布只需要四项官方 Secrets：

| Secret | 内容 |
|---|---|
| `ANDROID_KEYSTORE_BASE64` | 历史 V1 keystore 的单行 Base64 |
| `ANDROID_KEYSTORE_PASSWORD` | V1 keystore password |
| `ANDROID_KEY_ALIAS` | V1 private-key alias |
| `ANDROID_KEY_PASSWORD` | V1 key password |

仓库暂时兼容旧的 `ANDROID_V2_*` Secret 名称，但新的配置统一写入 `ANDROID_*`。旧名称只用于过渡，后续可以在确认官方名称稳定后删除。

推荐直接使用仓库脚本安全写入：

```bash
export ANDROID_KEYSTORE_PASSWORD='你的 V1 keystore 密码'
export ANDROID_KEY_PASSWORD='你的 V1 key 密码'

bash scripts/android-v2-signing-preflight.sh \
  --version 1.2.9 \
  --keystore '/安全路径/nowen-video-release.jks' \
  --alias '你的 V1 alias' \
  --repository cropflre/nowen-video \
  --report dist/android/signing-preflight.json \
  --set-github-secrets
```

前置要求：

```bash
gh auth status
gh repo view cropflre/nowen-video
```

`--set-github-secrets` 的顺序是：

1. 验证本地 keystore 和密码；
2. 下载公开的 V1 `v1.2.5` APK；
3. 提取 V1 APK 的真实签名证书；
4. 比较本地 keystore 证书；
5. 只有完全一致时，才写入四项 `ANDROID_*` Secrets。

脚本不会上传本地报告中的敏感值，也不会把 keystore 内容打印到终端。

## 4. GitHub 签名身份门禁

Secrets 配置后，在 GitHub Actions 手动运行：

```text
Android Signing Identity
```

必须得到：

```text
Verify V1 signing identity = PASS
```

门禁检查：

- 四项生产 Secret 是否存在；
- keystore 是否能读取指定 alias；
- keystore 证书 SHA-256；
- 历史 V1 APK 是否确实为 `com.nowen.video`；
- V1 APK 证书 SHA-256；
- 两者是否完全一致。

这一步不构建正式版本、不创建 tag、不发布 Release，因此可以安全重复执行。

## 5. 版本规则

`scripts/android-v2-version.sh` 是 Android versionCode 的唯一来源。接管后的 versionCode 被分配到高于历史 V1 的区间，避免系统把新正式版视为降级。

```bash
bash scripts/android-v2-version.sh --self-test
bash scripts/android-v2-version.sh 1.2.9
```

允许：

```text
MAJOR.MINOR.PATCH-alpha.N
MAJOR.MINOR.PATCH-beta.N
MAJOR.MINOR.PATCH-rc.N
MAJOR.MINOR.PATCH
```

同一语义版本内保持：

```text
alpha < beta < rc < stable
```

正式产物必须同时满足：

```text
applicationId = com.nowen.video
versionCode > historical V1 range
minSdk = 26
targetSdk = 35
signer = historical V1 signer
```

## 6. 生产构建验证（不发布）

在 GitHub Actions 手动运行 `release-android`，输入例如：

```text
version_name = 1.2.9
```

手动运行只生成并保留签名候选产物，不创建 GitHub Release，适合正式打 tag 前最后验收。

流水线会执行：

1. Android 版本策略自测；
2. 从公开 V1 APK 推导历史证书指纹；
3. 用历史 V1 指纹验证配置的 keystore；
4. Unit Test + Lint；
5. 构建 signed APK / AAB；
6. 从最终 APK 反读 applicationId、versionName、versionCode、minSdk、targetSdk；
7. 再次验证最终 APK 签名等于 V1；
8. 生成 `release-manifest.json`、`signing-preflight.json`、`SHA256SUMS.txt`、`RELEASE_NOTES.md`；
9. 上传 `nowen-video-android-release` Actions artifact，保留 30 天。

任一步失败都不允许进入正式 tag 发布。

## 7. 正式发布 v1.2.9

只有以下条件全部为 PASS 才允许打 tag：

- `Android` CI 通过；
- Android 8 / API 26 设备冒烟通过；
- Android 13 / API 33 设备冒烟通过；
- Android 15 / API 35 设备冒烟通过；
- `Android Signing Identity` 通过；
- `release-android` 手动生产构建通过；
- APK 可以在安装了 V1 的测试设备上直接覆盖安装，并保留应用沙箱；
- 旧服务器配置迁移正常，旧明文 Token/密码不会被迁移，用户按设计重新登录。

然后统一使用产品 tag：

```bash
git checkout main
git pull --ff-only
git status --short

git tag -a v1.2.9 -m 'Nowen Video v1.2.9'
git push origin v1.2.9
```

`v1.2.9` 会进入统一 Nowen Video 产品发布链路。Android workflow 会上传：

```text
nowen-video-android-1.2.9.apk
nowen-video-android-1.2.9.aab
release-manifest.json
signing-preflight.json
SHA256SUMS.txt
RELEASE_NOTES.md
```

GitHub Release 默认保持 **Draft**，待最终人工检查多端产物和说明后再公开。

## 8. V1 → 当前 Android 升级行为

原位升级时应用继续占用 `com.nowen.video` 的同一 Android sandbox。

当前客户端会尝试导入 V1：

- 已保存的服务器名称；
- 服务器 URL；
- 当前服务器选择。

不会迁移：

- V1 明文 Token；
- 密码；
- 任何不能安全升级到 Android Keystore 的旧凭据。

真正检测到并导入 V1 数据时才显示迁移提示；纯新安装不会显示旧版迁移提示。

## 9. 回滚与禁止事项

禁止：

- 为正式版重新生成 keystore；
- 修改 `com.nowen.video` 正式 applicationId；
- 绕过 V1 signer gate；
- 手工降低 versionCode；
- 将生产 keystore 放进仓库；
- 在日志、Issue、PR 或聊天中粘贴 Base64 私钥内容或密码；
- 在签名身份门禁失败时仍创建正式 tag。

如果候选版出现功能问题，应修复代码并生成更高 versionCode 的新候选；不要通过换签名或降低版本号回滚。
