# Nowen Video Android V2

全新的 Kotlin + Jetpack Compose 原生客户端，与旧版 `android/` 并行开发。

## 当前能力

- 多服务器保存、切换、删除与连接探测
- Android Keystore 加密保存每台服务器的 Token
- 登录与首次强制修改密码
- Hills Calm × Nowen Deep Space 设计系统
- 首页：继续观看、媒体库、最近添加
- 五栏主导航：首页、媒体库、搜索、下载、我的
- 原生媒体库网格与混合电影/剧集解析
- 电影/单集详情页
- Media3 原生播放，支持 Direct Play、Remux、HLS 和预处理流地址
- 播放进度恢复、每 10 秒定时上报、暂停/拖动/退后台/退出时即时补报
- 断网进度持久化队列，按服务器和账号隔离，恢复连接后自动补同步
- Media3 实时音轨、内嵌字幕和外挂字幕选择
- 播放速度、画面比例和自动下一集偏好持久化
- 剧集播放结束后显示下一集信息与 5 秒自动续播倒计时
- 独立服务器会话和请求 Host 重写
- 首页、搜索、续播三种后端媒体响应兼容

## 模块

```text
clients/android-v2/
├── app                  Android Application 与 Activity
├── core/model           领域模型和 API 契约
├── core/designsystem    主题、Token 和通用 Compose 组件
├── core/data            会话、Keystore、Retrofit、Repository、播放器偏好与离线进度队列
└── feature/main         服务器、认证、首页、媒体库、详情和播放器
```

当前先保持五个稳定模块，避免重写初期过度拆分。后续在功能边界稳定后再拆出 `feature:player`、`feature:downloads` 等模块。

## 播放进度策略

- 打开播放器时先重试当前服务器、当前账号的离线进度，再读取服务端进度。
- 已观看达到 95% 或服务端标记完成时，从头播放，避免恢复到片尾字幕。
- 播放中每 10 秒上报一次；暂停、拖动进度、播放结束、应用退到后台和退出播放器时立即补报。
- 上报前先写入本地 DataStore，网络失败时保留最新记录；同一服务器、账号、媒体只保留最新进度。
- 待同步队列最多保存 200 条，防止长期离线导致本地数据无限增长。
- 单元测试覆盖无效进度拒绝、超范围钳制和 95% 完播重置规则。

## 播放器设置

- 音轨与内嵌字幕直接读取 ExoPlayer 当前 Tracks，切换后立即生效。
- 外挂字幕通过服务端字幕接口注入 MediaItem，兼容 SRT、ASS/SSA、WebVTT 和 TTML。
- 倍速支持 0.5x 至 2x；画面支持适应、裁切和拉伸。
- 倍速、画面比例、自动下一集使用独立 DataStore 保存，重新进入播放器后继续沿用。
- 自动下一集使用当前媒体的 series、season、episode 查询后端，不通过列表位置猜测下一集。

## 本地构建

项目复用仓库根 Android 工程的 Gradle Wrapper：

```bash
./android/gradlew -p clients/android-v2 testDebugUnitTest lintDebug assembleDebug
```

Windows：

```powershell
.\android\gradlew.bat -p clients\android-v2 testDebugUnitTest lintDebug assembleDebug
```

Debug APK：

```text
clients/android-v2/app/build/outputs/apk/debug/app-debug.apk
```

仓库的 `Android V2` 工作流会对每次相关 PR 和 main push 执行同一组单测、Lint 与 APK 构建门禁，并保留失败日志便于定位 Kotlin 与 Android 资源问题。

## 与旧版并行安装

V2 当前使用独立应用 ID：

```text
com.nowen.video.v2
```

旧客户端不会被覆盖。V2 稳定并完成迁移验证后，再决定是否切换正式应用 ID 和签名。

## 下一阶段

- Paging 3、媒体库筛选与平板双栏布局
- WorkManager + Media3 离线下载
- 局域网自动发现和扫码添加服务器
- 收藏、历史、合集与人物详情
