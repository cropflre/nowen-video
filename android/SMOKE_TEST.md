# Android Device Smoke Test

正式 Android 客户端至少验证以下系统：

- Android 8.0 / API 26
- Android 13 / API 33
- Android 15 / API 35

GitHub Actions workflow：`.github/workflows/android-device-smoke.yml`。

本地连接设备后可运行：

```bash
bash scripts/android-device-smoke.sh 35
```

冒烟测试至少覆盖应用首次启动、Compose 根界面渲染和无崩溃启动。正式发布前还应人工验证服务器配置、登录、媒体库、详情、播放、字幕、进度同步、离线任务和前后台切换。
