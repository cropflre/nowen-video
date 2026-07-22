# Nowen Video Android V2 {{VERSION_NAME}}

> {{RELEASE_STATUS}}

## 版本信息

| 项目 | 值 |
|---|---|
| versionName | `{{VERSION_NAME}}` |
| versionCode | `{{VERSION_CODE}}` |
| applicationId | `com.nowen.video.v2` |
| 最低系统 | Android 8.0 / API 26 |
| 目标系统 | Android 15 / API 35 |
| source commit | `{{SOURCE_COMMIT}}` |
| 签名证书 SHA-256 | `{{CERTIFICATE_SHA256}}` |

## 当前能力

### 服务器与账号

- 多服务器保存、切换和删除。
- 局域网 mDNS 发现、私有 IPv4 `/24` 常用端口回退探测、二维码和手动地址添加。
- 登录、退出、首次强制修改密码。
- Android Keystore + AES/GCM 按服务器加密保存 Token。
- 多服务器、多账号凭据、历史和下载任务隔离。

### 浏览、搜索与详情

- 首页、媒体库、搜索、下载和“我的”五栏导航。
- Paging 3 媒体库、收藏与观看历史。
- 影视、人物和电影合集并发聚合搜索，单个分类失败时保留其他结果。
- 电影、单集、剧集、人物和电影合集详情导航。
- 剧集季选择、单集列表、下一集查询和自动续播。

### 播放与离线

- Media3 Direct Play、Remux、HLS 和预处理流播放。
- 音轨、内嵌字幕、外挂字幕、速度、画面比例和自动下一集。
- 播放进度恢复、定时上报、退后台补报和断网后补同步。
- WorkManager 前台下载、HTTP Range 断点续传、暂停、继续、重试和启动恢复。
- 本地文件完全离线播放，恢复联网后同步播放进度。

## 安装和迁移

普通用户安装 APK；AAB 仅用于应用商店或分发平台，不能直接在手机上安装。

```bash
adb install -r nowen-video-android-v2-{{VERSION_NAME}}.apk
```

- V2 正式包名为 `com.nowen.video.v2`，可与旧版 `com.nowen.video` 并行安装。
- 旧版服务器、Token 和本地数据不会自动迁移，需要在 V2 中重新添加服务器并登录。
- 后续 V2 覆盖升级必须保持相同 applicationId、相同正式签名并使用更高 versionCode。
- 不要通过卸载 V2 完成升级；卸载会清除 V2 本地会话、偏好和下载记录。

## 已知限制

- 局域网发现只扫描当前私有 IPv4 `/24` 和预设常用端口；跨 VLAN、访客 Wi-Fi、AP 隔离或复杂 IPv6 网络可能需要手动输入地址。
- 路由器屏蔽组播时 mDNS 不可用，HTTP 回退探测不保证发现非标准端口。
- 离线下载面向 Direct Play、Remux 或预处理后的单文件地址；仅返回 HLS/m3u8 的媒体不会下载为单文件。
- 厂商后台限制、前台下载通知、部分媒体格式和覆盖升级仍需要持续真机验证。
- Debug、RC 和 Stable 数据按 applicationId 隔离；Debug 数据不会自动迁移到正式 V2。

## 产物完整性

| 文件 | SHA-256 |
|---|---|
| APK | `{{APK_SHA256}}` |
| AAB | `{{AAB_SHA256}}` |

下载 APK、AAB、`SHA256SUMS.txt` 和 `release-manifest.json` 后执行：

```bash
sha256sum -c SHA256SUMS.txt
apksigner verify --verbose --print-certs nowen-video-android-v2-{{VERSION_NAME}}.apk
jarsigner -verify nowen-video-android-v2-{{VERSION_NAME}}.aab
python3 -m json.tool release-manifest.json > /dev/null
```

APK 签名证书 SHA-256 必须与本说明和 `release-manifest.json` 一致。

## 反馈模板

```text
标题：[Android V2][设备/API][模块] 简短现象

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
是否阻断当前版本：是 / 否
```

请勿在公开 Issue 中粘贴 Token、密码、服务器公网地址、账号、keystore 或其他凭据。
