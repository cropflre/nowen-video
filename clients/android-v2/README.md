# Nowen Video Android

Nowen Video 的正式原生 Android 客户端，基于 **Kotlin、Jetpack Compose、Media3、Paging 3 和 WorkManager**。

> `clients/android-v2/` 仍作为源码目录保留，`com.nowen.video.v2` 仍作为 Kotlin namespace 保留，但 **V2 只代表代码代际，不再代表独立产品**。正式 Release 已接管历史 `com.nowen.video`，Android V1 不再发布。
>
> 最低系统：**Android 8.0 / API 26**  
> 目标系统：**Android 15 / API 35**

## 正式替换 V1

当前 Release applicationId：

```text
com.nowen.video
```

Debug applicationId：

```text
com.nowen.video.debug
```

这意味着来自官方签名链的 Android V1 可以被当前客户端**原位覆盖升级**，无需先卸载。

覆盖升级的安全条件由发布流水线强制验证：

- applicationId 必须为 `com.nowen.video`；
- versionCode 必须进入高于 V1 历史范围的保留区间；
- 正式 APK 必须继续使用 V1 原签名证书；
- workflow 会下载已公开的 V1 `v1.2.5` APK 并现场比对证书 SHA-256；
- 签名不一致直接阻断发布。

详见 [MIGRATION.md](./MIGRATION.md)。

## V1 数据迁移

原位升级时 Android 会保留 V1 应用沙箱。当前客户端首次初始化会尝试读取：

- V1 `server_profiles`：服务器名称、URL、当前服务器；
- 更早版本 `nowen_prefs` 中的 `server_url` 作为兜底。

**不会迁移 V1 的明文 Token、密码或其他敏感凭据。** 当前客户端使用 Android Keystore + AES/GCM 保存新 Token，因此升级后如出现登录页，重新登录一次即可。

不要为了升级先卸载 V1；卸载会清除旧应用沙箱，使服务器地址自动迁移失去来源。

旧测试阶段的 `com.nowen.video.v2` APK 与正式包不是同一个 applicationId，不属于正式升级链。确认正式版可用后可自行卸载旧测试包。

## 当前能力

### 服务器与账号

- 保存、切换和删除多个 Nowen Video 服务器。
- mDNS 自动发现局域网服务器；无广播时回退到私有 IPv4 `/24` 常用端口探测。
- CameraX + bundled ML Kit 设备端二维码识别，支持 URL、JSON 和 `nowen-video://server`。
- Android Keystore + AES/GCM 按服务器加密保存 Token。
- 登录、退出和首次强制修改密码。
- 多服务器、多账号凭据和下载任务隔离。

### 浏览与搜索

- 首页、媒体库、搜索、下载、我的五栏主导航。
- 首页继续观看、媒体库和最近添加。
- Paging 3 媒体库及筛选/排序。
- 影视、人物和电影合集聚合搜索。
- 电影、单集、剧集、人物和合集详情。
- 剧集季选择、单集列表、下一集和自动续播。
- 收藏与观看历史跨页加载。

### 播放

- Media3 原生 Direct Play、Remux、HLS 和预处理流。
- 播放进度恢复、定时上报和断网补同步。
- 音轨、内嵌字幕、外挂字幕选择。
- 播放速度、画面比例和自动下一集偏好。
- 下一集信息和自动续播倒计时。

### 离线

- WorkManager 前台下载。
- HTTP Range 断点续传。
- 暂停、继续、失败重试、启动恢复和残留维护。
- Wi-Fi 下载策略和空间上限。
- Media3 本地文件离线播放与联网后进度补同步。

## 安装正式版本

正式 Android 产物附加到统一的 Nowen Video `vX.Y.Z` GitHub Release：

```text
nowen-video-android-<version>.apk
nowen-video-android-<version>.aab
SHA256SUMS.txt
release-manifest.json
```

普通用户安装 APK；AAB 用于应用商店或分发平台。

覆盖安装：

```bash
adb install -r nowen-video-android-<version>.apk
```

如果设备安装的是官方 V1 且签名一致，应直接升级而不是要求卸载。

## 本地构建

项目继续复用仓库根目录 `android/gradlew`，需要 JDK 17 和 Android SDK 35。

Debug：

```bash
chmod +x android/gradlew
./android/gradlew -p clients/android-v2 testDebugUnitTest lintDebug assembleDebug
adb install -r clients/android-v2/app/build/outputs/apk/debug/app-debug.apk
```

Windows：

```powershell
.\android\gradlew.bat -p clients\android-v2 testDebugUnitTest lintDebug assembleDebug
```

未签名 Release：

```bash
./android/gradlew -p clients/android-v2 assembleRelease
```

正式签名构建：

```bash
export ANDROID_VERSION_NAME='1.2.9'
export ANDROID_VERSION_CODE="$(bash scripts/android-v2-version.sh "$ANDROID_VERSION_NAME")"
export ANDROID_SIGNING_STORE_FILE="$HOME/keys/nowen-video-release.jks"
export ANDROID_SIGNING_STORE_PASSWORD='replace-me'
export ANDROID_SIGNING_KEY_ALIAS='nowen-video'
export ANDROID_SIGNING_KEY_PASSWORD='replace-me'

./android/gradlew -p clients/android-v2 \
  clean testDebugUnitTest lintDebug assembleRelease bundleRelease
```

**生产签名必须是历史 Android V1 正式签名。不要生成一把新 key 来发布 `com.nowen.video`。**

## 自动化门禁

相关变更会执行：

- Android versionCode 策略自测；
- 服务端 API 契约测试；
- Android 单元测试；
- instrumentation 编译；
- Lint / Debug / Release 构建；
- 临时 keystore 签名冒烟；
- Android 8 / 13 / 15 首次启动模拟器冒烟；
- 正式 tag 发布时额外执行 V1 签名证书一致性校验。

## 目录结构

```text
clients/android-v2/
├── app                  Application、Activity、权限、启动恢复
├── core/model           领域模型和 API 契约
├── core/designsystem    主题与通用 Compose 组件
├── core/data            会话、Keystore、Retrofit、Paging、迁移、下载
└── feature/main         服务器、认证、首页、媒体库、搜索、详情、播放器、下载中心
```

## 问题反馈

```text
标题：[Android][设备/API][模块] 简短现象

Android 版本 / commit：
服务器版本 / commit：
设备型号 / Android API：
网络环境：
媒体格式与播放方式：
复现步骤：
实际结果：
预期结果：
复现率：
附件：截图 / 录屏 / logcat
```

公开 Issue 不要附带 Token、密码、私有服务器地址、账号或签名密钥。
