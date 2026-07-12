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
- 独立服务器会话和请求 Host 重写
- 首页、搜索、续播三种后端媒体响应兼容

## 模块

```text
clients/android-v2/
├── app                  Android Application 与 Activity
├── core/model           领域模型和 API 契约
├── core/designsystem    主题、Token 和通用 Compose 组件
├── core/data            会话、Keystore、Retrofit 与 Repository
└── feature/main         服务器、认证、首页、媒体库、详情和播放器
```

当前先保持五个稳定模块，避免重写初期过度拆分。后续在功能边界稳定后再拆出 `feature:player`、`feature:downloads` 等模块。

## 本地构建

项目复用旧 Android 工程提交的 Gradle Wrapper：

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

仓库的 `Android V2` 工作流会对每次相关 PR 和 main push 执行同一组单测、Lint 与 APK 构建门禁。

## 与旧版并行安装

V2 当前使用独立应用 ID：

```text
com.nowen.video.v2
```

旧客户端不会被覆盖。V2 稳定并完成迁移验证后，再决定是否切换正式应用 ID 和签名。

## 下一阶段

- 播放进度定时上报与离线补报
- 音轨、字幕、倍速、画面比例和下一集
- Paging 3、媒体库筛选与平板双栏布局
- WorkManager + Media3 离线下载
- 局域网自动发现和扫码添加服务器
- 收藏、历史、合集与人物详情
