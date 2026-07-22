# Nowen Video Android V2

基于 **Kotlin、Jetpack Compose、Media3、Paging 3 和 WorkManager** 的原生 Android 客户端。V2 与旧版 `android/` 并行开发，RC1 使用独立包名，不会覆盖旧客户端。

> 当前阶段：**RC1 候选版**  
> 最低系统：**Android 8.0 / API 26**  
> 目标系统：**Android 15 / API 35**

## 文档入口

- [正式签名、版本号与发布流程](./RELEASE.md)
- [包名、覆盖升级与旧版迁移策略](./MIGRATION.md)
- [Android 8 / 13 / 15 真机冒烟检查表](./SMOKE_TEST.md)

## 当前能力

### 服务器与账号

- 保存、切换和删除多个 Nowen Video 服务器。
- 使用 mDNS 自动发现局域网服务器；无广播时回退到私有 IPv4 `/24` 网段常用端口探测。
- CameraX + bundled ML Kit 设备端二维码识别，支持 URL、JSON 和 `nowen-video://server`。
- Android Keystore + AES/GCM 按服务器加密保存 Token。
- 登录、退出登录和首次强制修改密码。
- 多服务器、多账号凭据和下载任务隔离。

### 浏览与搜索

- 五栏主导航：首页、媒体库、搜索、下载、我的。
- 首页展示继续观看、媒体库和最近添加；附属接口失败不会导致整页白屏。
- Paging 3 媒体库，支持媒体库、类型、标题、标签、年份和排序筛选。
- 840dp 以上启用海报网格与详情预览双栏布局。
- 搜索并发聚合影视、人物和电影合集；单个分类接口失败时保留其他成功结果。
- 电影、单集、剧集、人物和电影合集详情导航。
- 剧集季选择、单集列表、下一集查询和自动续播。
- 收藏与观看历史使用 Paging 3 跨页加载，支持取消收藏、删除单条历史和清空历史。

### 播放

- Media3 原生播放，支持 Direct Play、Remux、HLS 和预处理流。
- 播放进度恢复，每 10 秒定时上报，并在暂停、拖动、退后台和退出时补报。
- 断网进度写入本地队列，恢复连接后按服务器和账号补同步。
- 实时音轨、内嵌字幕和外挂字幕选择。
- 播放速度、画面比例和自动下一集偏好持久化。
- 下一集信息和 5 秒自动续播倒计时。

### 离线

- WorkManager 前台下载、HTTP Range 断点续传和网络约束调度。
- 下载暂停、继续、失败重试、删除、启动恢复和残留维护。
- 默认仅 Wi-Fi 下载，可允许移动网络。
- 离线空间上限支持 5、10、20、50 和 100 GB。
- Media3 本地文件播放；离线播放进度恢复联网后补同步。

## 安装 RC1

公开测试包应来自 Android V2 专属 GitHub Release，文件通常包括：

```text
nowen-video-android-v2-<version>.apk
nowen-video-android-v2-<version>.aab
SHA256SUMS.txt
```

普通用户安装 **APK**。AAB 用于应用商店或分发平台，不可直接在手机上点击安装。

### 校验文件

Linux / macOS：

```bash
sha256sum -c SHA256SUMS.txt
```

Windows PowerShell 可计算单个文件：

```powershell
Get-FileHash .\nowen-video-android-v2-0.1.0-rc.1.apk -Algorithm SHA256
```

计算结果应与 `SHA256SUMS.txt` 一致。

### ADB 安装或覆盖升级

```bash
adb install -r nowen-video-android-v2-0.1.0-rc.1.apk
```

- 首次安装会创建 `com.nowen.video.v2`。
- RC1 → 更高 RC → Stable 只有在 **applicationId 相同、正式签名相同、versionCode 更高** 时才能原位升级。
- 旧版 `com.nowen.video` 可与 V2 并行安装。
- 不要通过卸载 V2 来完成正式升级；卸载会清除 V2 本地会话、偏好和下载记录。

## 连接服务器

1. 确保手机和 Nowen Video 服务器网络互通。
2. 首次启动阅读并关闭旧版迁移说明。
3. 选择局域网发现、扫描二维码或手动添加。
4. 等待健康检测通过后登录。
5. 旧版服务器和 Token 不会自动迁移，需要重新添加并登录。

常用手动地址示例：

```text
http://192.168.1.10:8080
https://video.example.com
```

## 本地构建

项目复用仓库根目录的 Gradle Wrapper，需要 **JDK 17** 和 Android SDK 35。

### Debug APK

Debug 使用 `com.nowen.video.v2.debug` 和 Android debug key，可与正式 V2 并行安装：

```bash
chmod +x android/gradlew
./android/gradlew -p clients/android-v2 testDebugUnitTest lintDebug assembleDebug
adb install -r clients/android-v2/app/build/outputs/apk/debug/app-debug.apk
```

Windows：

```powershell
.\android\gradlew.bat -p clients\android-v2 testDebugUnitTest lintDebug assembleDebug
adb install -r clients\android-v2\app\build\outputs\apk\debug\app-debug.apk
```

### 未签名 Release

```bash
./android/gradlew -p clients/android-v2 assembleRelease
```

产物：

```text
clients/android-v2/app/build/outputs/apk/release/app-release-unsigned.apk
```

**未签名 Release 仅用于验证 R8、资源压缩和 Release 变体，不具备 Android 安装签名，不能分发或安装。**

### 正式签名 APK / AAB

正式构建需要同时配置四项签名环境变量和版本变量：

```bash
export ANDROID_VERSION_NAME='0.1.0-rc.1'
export ANDROID_VERSION_CODE="$(bash scripts/android-v2-version.sh "$ANDROID_VERSION_NAME")"
export ANDROID_SIGNING_STORE_FILE="$HOME/keys/nowen-video-android-v2-release.jks"
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

密钥生成、GitHub Actions Secrets、tag 和草稿 Release 流程见 [RELEASE.md](./RELEASE.md)。

## 自动化门禁

每个相关 Pull Request 会执行：

- Android 版本策略脚本自测。
- 服务端 handler / service 契约测试。
- Android 单元测试。
- instrumentation APK 编译。
- Lint、Debug APK 和未签名 Release APK 构建。
- 临时 keystore 签名 APK/AAB 构建与验签。
- Android 8、13、15 首次启动模拟器冒烟。

本地执行首次启动 instrumentation 用例：

```bash
./android/gradlew -p clients/android-v2 \
  :app:connectedDebugAndroidTest \
  -Pandroid.testInstrumentationRunnerArguments.class=com.nowen.video.v2.AppLaunchSmokeTest
```

## 权限与系统行为

| 权限 / 能力 | 用途 |
|---|---|
| 网络与网络状态 | API、播放、下载和重试判断 |
| Wi-Fi 状态与 MulticastLock | mDNS 与局域网发现 |
| 相机 | 扫描服务器二维码，仅在用户进入扫码页后申请 |
| 前台服务 `dataSync` | 长时间离线下载 |
| Android 13+ 通知 | 显示前台下载进度 |

- 相机识别在设备本地完成，画面不会上传。
- HTTP 局域网服务器可连接，但公网部署应优先使用 HTTPS。
- 拒绝通知权限可能影响下载前台通知的可见性，应按真机检查表验证设备行为。

## RC1 已知限制

- 局域网探测只扫描当前私有 IPv4 `/24` 和预设常用端口；跨 VLAN、访客 Wi-Fi、AP 隔离或复杂 IPv6 网络可能需要手动输入地址。
- 路由器屏蔽组播时 mDNS 不可用，会自动回退 HTTP 探测，但不保证发现非标准端口。
- 离线下载面向 Direct Play、Remux 或预处理后的单文件地址；仅返回 HLS/m3u8 的媒体不会下载为单文件。
- V2 不读取旧版私有数据和 Token；服务器需要重新添加并登录。
- RC1 是公开测试候选版本，关键播放格式、厂商后台限制、通知和覆盖升级仍需真机验证。
- Debug、RC 和 Stable 的数据目录按 applicationId 隔离；Debug 数据不会自动迁移到 Release。

## 模块结构

```text
clients/android-v2/
├── app                  Application、Activity、权限、启动恢复和前台服务声明
├── core/model           领域模型、筛选、下载、发现与 API 契约
├── core/designsystem    主题、Token 和通用 Compose 组件
├── core/data            会话、Keystore、Retrofit、Paging、发现、下载和 Repository
└── feature/main         服务器、认证、首页、媒体库、搜索、详情、播放器与下载中心
```

当前保持五个稳定模块，等功能边界和复用关系稳定后再拆分播放器、下载等独立 feature。

## 提交问题

提交 GitHub Issue 前请先完成 [SMOKE_TEST.md](./SMOKE_TEST.md) 中对应模块的检查，并附上：

```text
标题：[Android V2 RC1][设备/API][模块] 简短现象

V2 版本 / commit：
服务器版本 / commit：
设备型号 / Android API：
网络环境：
媒体格式与播放方式：
复现步骤：
实际结果：
预期结果：
复现率：
日志时间范围：
附件：截图 / 录屏 / logcat / 服务端日志
是否阻断 RC：是 / 否
```

安全问题、Token、服务器公网地址、账号和密钥不要直接粘贴到公开 Issue。
