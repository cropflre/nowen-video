# Nowen Video Android

这是 Nowen Video 唯一正式 Android 客户端，源码位于仓库根目录 `android/`。

## 技术栈

- Kotlin + Jetpack Compose
- Media3
- Hilt
- Retrofit / OkHttp
- Paging 3
- WorkManager
- Android Keystore

## 系统要求

- minSdk: Android 8.0 / API 26
- targetSdk: API 35
- applicationId: `com.nowen.video`

## 本地构建

```bash
cd android
./gradlew testDebugUnitTest lintDebug assembleDebug
```

Windows:

```powershell
cd android
.\gradlew.bat testDebugUnitTest lintDebug assembleDebug
```

正式签名和发布流程见 [RELEASE.md](./RELEASE.md)，设备冒烟流程见 [SMOKE_TEST.md](./SMOKE_TEST.md)。

## 产品身份

仓库不再维护 Android V1 / V2 两套客户端，也不再执行旧客户端数据迁移。当前 `android/` 就是正式 Android 产品源码。

由于旧 Android V1 与当前正式版不再要求使用同一签名，已经安装旧 V1 的设备可能需要先卸载旧应用再安装当前正式版。之后所有正式 Android 版本必须持续使用同一份新的生产 keystore。
