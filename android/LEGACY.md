# Android V1 — Legacy Reference

`android/app` 是 Nowen Video 历史 Android V1 源码，现已退出正式发布链路。

当前正式 Android 客户端由 `clients/android-v2` 的模块化实现提供，并已经接管生产 applicationId：

```text
com.nowen.video
```

## 保留本目录的原因

本目录暂不物理删除，因为：

1. `android/gradlew`、`android/gradlew.bat` 和 `android/gradle/wrapper` 仍被当前 Android 构建复用；
2. V1 DataStore / 偏好结构是覆盖升级迁移的历史契约依据；
3. V1 源码用于兼容性、回归和历史问题定位。

## 禁止事项

- 不再从 `android/app` 生成用户正式 APK；
- 不再为 V1 增加新功能；
- 不要重新启用 V1 Release workflow；
- 不要用新签名覆盖 `com.nowen.video`；
- 不要删除旧数据格式定义，除非对应迁移契约已经显式退休。

## 正式 Android 源码

```text
clients/android-v2/
```

虽然目录和 Kotlin namespace 中仍含 `v2`，它现在只表示内部代码代际，不再表示第二个可并行发布的产品。

正式迁移和签名策略见：

```text
clients/android-v2/MIGRATION.md
clients/android-v2/README.md
```
