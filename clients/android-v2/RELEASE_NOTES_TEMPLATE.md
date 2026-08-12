# Nowen Video Android {{VERSION_NAME}}

> {{RELEASE_STATUS}}

## 版本信息

| 项目 | 值 |
|---|---|
| versionName | `{{VERSION_NAME}}` |
| versionCode | `{{VERSION_CODE}}` |
| applicationId | `com.nowen.video` |
| 最低系统 | Android 8.0 / API 26 |
| 目标系统 | Android 15 / API 35 |
| source commit | `{{SOURCE_COMMIT}}` |
| 签名证书 SHA-256 | `{{CERTIFICATE_SHA256}}` |

## 正式替换说明

Android V2 的模块化实现已经正式接管 Nowen Video Android，发布包不再作为独立 V2 应用存在。

- 正式包名恢复并固定为 `com.nowen.video`，与 Android V1 相同。
- 正式发布必须使用 Android V1 原有签名证书；发布流水线会与历史 `v1.2.5` APK 的证书做强校验，不一致直接阻断发布。
- versionCode 使用高于 V1 历史范围的保留区间，支持从 V1 原位覆盖升级。
- 升级时会尽量迁移旧版服务器地址和当前服务器选择。
- V1 中以明文形式保存的 Token、密码等敏感凭据不会导入；用户可能需要重新登录。
- 旧的 `com.nowen.video.v2` 测试包仍是独立应用，不属于正式升级链路。

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

## 安装和升级

普通用户安装 APK；AAB 仅用于应用商店或分发平台，不能直接在手机上安装。

```bash
adb install -r nowen-video-android-{{VERSION_NAME}}.apk
```

如果设备已经安装正式 Android V1，且该 V1 来自官方签名链，以上命令应直接原位升级，不需要卸载旧版。

**不要为了升级先卸载 V1。** 卸载会先清除旧应用沙箱，使自动迁移服务器地址失去数据来源。

## 已知边界

- 局域网发现只扫描当前私有 IPv4 `/24` 和预设常用端口；复杂 VLAN/IPv6 网络可能需要手动输入地址。
- 旧版 Token 不迁移是安全设计，不是迁移失败。
- 旧独立 V2 测试包 `com.nowen.video.v2` 不会自动转换成正式 `com.nowen.video`；可在确认正式版可用后手动卸载测试包。
- APK 降级不是正式回滚方案；发现问题应发布更高 versionCode 的修复版本。

## 产物完整性

| 文件 | SHA-256 |
|---|---|
| APK | `{{APK_SHA256}}` |
| AAB | `{{AAB_SHA256}}` |

```bash
sha256sum -c SHA256SUMS.txt
apksigner verify --verbose --print-certs nowen-video-android-{{VERSION_NAME}}.apk
jarsigner -verify nowen-video-android-{{VERSION_NAME}}.aab
python3 -m json.tool release-manifest.json > /dev/null
```

APK 签名证书 SHA-256 必须与本说明、`release-manifest.json` 以及历史 V1 正式签名保持一致。

## 反馈模板

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
日志时间范围：
附件：截图 / 录屏 / logcat / 服务端日志
是否阻断当前版本：是 / 否
```

请勿在公开 Issue 中粘贴 Token、密码、服务器公网地址、账号、keystore 或其他凭据。
