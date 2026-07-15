# Nowen Video Android V2

全新的 Kotlin + Jetpack Compose 原生客户端，与旧版 `android/` 并行开发。

## 当前能力

- 多服务器保存、切换、删除与连接探测
- Android Keystore 加密保存每台服务器的 Token
- 登录与首次强制修改密码
- Hills Calm × Nowen Deep Space 设计系统
- 首页：继续观看、媒体库、最近添加
- 五栏主导航：首页、媒体库、搜索、下载、我的
- Paging 3 媒体库：按服务端稳定分页加载电影与剧集合集
- 媒体库筛选：媒体库、内容类型、标题、类型标签、年份范围
- 媒体库排序：最近添加、标题、年份、评分及升降序
- 840dp 起启用海报网格与详情预览双栏布局，手机保持单栏导航
- 电影/单集详情页
- Media3 原生播放，支持 Direct Play、Remux、HLS 和预处理流地址
- 播放进度恢复、每 10 秒定时上报、暂停/拖动/退后台/退出时即时补报
- 断网进度持久化队列，按服务器和账号隔离，恢复连接后自动补同步
- Media3 实时音轨、内嵌字幕和外挂字幕选择
- 播放速度、画面比例和自动下一集偏好持久化
- 剧集播放结束后显示下一集信息与 5 秒自动续播倒计时
- WorkManager 前台离线下载、HTTP Range 断点续传与系统约束调度
- 离线任务按服务器和账号隔离，Token 在 Worker 执行时从 Android Keystore 读取
- 下载暂停、继续、失败重试、删除、启动恢复和过期残留维护
- 默认仅 Wi-Fi 下载，可切换移动网络；离线空间上限支持 5–100 GB
- Media3 本地文件播放，离线观看进度继续写入待同步队列
- 独立服务器会话和请求 Host 重写
- 首页、搜索、续播三种后端媒体响应兼容

## 模块

```text
clients/android-v2/
├── app                  Android Application、Activity、启动下载恢复与前台服务声明
├── core/model           领域模型、筛选条件、下载状态和 API 契约
├── core/designsystem    主题、Token 和通用 Compose 组件
├── core/data            会话、Keystore、Retrofit、Paging、WorkManager、Repository、偏好与离线队列
└── feature/main         服务器、认证、首页、自适应媒体库、详情、在线/离线播放器和下载中心
```

当前先保持五个稳定模块，避免重写初期过度拆分。后续在功能边界稳定后再拆出 `feature:player`、`feature:downloads` 等模块。

## 媒体库分页与筛选

- `/api/media/mixed` 在服务端完成筛选、排序后再分页，避免客户端本地筛选造成重复、漏项和空页。
- 支持 `library_id`、`type`、`genre`、`q`、`year_from`、`year_to`、`sort`、`order` 参数。
- Android 每页加载 36 项，预取距离为 12，关闭占位符；筛选条件变化时自动取消旧分页流并创建新流。
- 手机点击海报进入完整详情；宽屏设备在右侧直接预览详情，并可立即播放或打开完整详情。
- 分页首屏、追加页、空结果和网络错误均提供独立加载与重试状态。

## 离线下载

- 影片详情页可创建、暂停、继续或重试离线任务；下载中心统一展示进度、错误与存储占用。
- WorkManager 根据网络与存储条件执行任务，默认要求非计费网络，并以 `dataSync` 前台服务显示下载通知。
- 文件先写入 `.part`，再次执行时发送 HTTP `Range` 请求；服务器不支持 Range 时会安全地从头重新下载。
- 任务记录与文件目录按 `serverId + userId` 隔离，任务参数中不保存 Token，Worker 通过现有 Keystore 凭据访问服务器。
- 默认离线配额为 20 GB，可选择 5、10、20、50 或 100 GB；下载前和写入过程中都会检查配额及设备剩余空间。
- 应用启动时恢复排队或下载中的任务，每日维护会标记丢失文件并清理长期残留的无主文件。
- 已完成文件由独立 Media3 本地播放器打开；无网络时的播放进度先保存在本机，恢复连接后自动同步。
- 当前设备端下载面向 Direct Play、Remux 或预处理后的单文件地址。仅返回 HLS/m3u8 的媒体会明确提示，避免把播放列表误存为视频文件。

## 播放进度策略

- 打开在线播放器时先重试当前服务器、当前账号的离线进度，再读取服务端进度。
- 离线播放器只读取本机待同步进度，不依赖当前网络状态。
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

仓库的 `Android V2` 工作流会同时执行后端媒体契约测试、Android 单元测试、Lint 与 APK 构建门禁，并保留日志和 Debug APK。

## 与旧版并行安装

V2 当前使用独立应用 ID：

```text
com.nowen.video.v2
```

旧客户端不会被覆盖。V2 稳定并完成迁移验证后，再决定是否切换正式应用 ID 和签名。

## 下一阶段

- 局域网自动发现和扫码添加服务器
- 收藏、历史、合集与人物详情
