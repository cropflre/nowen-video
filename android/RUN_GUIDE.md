# 安卓端运行指南

## 方案一：使用 Android Studio（推荐）

### 1. 安装 Android Studio
- 下载：https://developer.android.com/studio
- 安装后打开 Android Studio

### 2. 打开项目
- 选择 "Open an existing project"
- 选择 `android` 目录

### 3. 配置 SDK
- Android Studio 会自动下载所需的 SDK
- 或者手动安装：`SDK Manager` → `SDK Platforms` → Android 14 (API 34)

### 4. 运行项目
- 连接安卓设备或启动模拟器
- 点击 "Run" 按钮（绿色三角形）
- 或使用快捷键 `Shift + F10`

---

## 方案二：使用 Gradle 命令行

### 1. 安装 Android SDK
```bash
# Windows (使用 scoop)
scoop install android-sdk

# 或手动下载
# https://developer.android.com/studio#command-tools
```

### 2. 配置环境变量
```bash
# 添加到系统环境变量
ANDROID_HOME=C:\Users\你的用户名\AppData\Local\Android\Sdk
ANDROID_SDK_ROOT=%ANDROID_HOME%
PATH=%PATH%;%ANDROID_HOME%\platform-tools;%ANDROID_HOME%\tools
```

### 3. 构建 APK
```bash
cd android

# 构建 debug APK
./gradlew assembleDebug

# 构建 release APK
./gradlew assembleRelease
```

### 4. 安装到设备
```bash
# 连接设备（开启 USB 调试）
adb install app/build/outputs/apk/debug/app-debug.apk
```

---

## 方案三：使用 GitHub Actions（云端构建）

### 1. 创建 workflow 文件
创建 `.github/workflows/build-android.yml`：

```yaml
name: Build Android

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]
  workflow_dispatch:

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
    - uses: actions/checkout@v4

    - name: Set up JDK 17
      uses: actions/setup-java@v4
      with:
        java-version: '17'
        distribution: 'temurin'

    - name: Setup Android SDK
      uses: android-actions/setup-android@v3

    - name: Grant execute permission for gradlew
      run: chmod +x android/gradlew

    - name: Build Debug APK
      run: |
        cd android
        ./gradlew assembleDebug

    - name: Upload APK
      uses: actions/upload-artifact@v4
      with:
        name: app-debug
        path: android/app/build/outputs/apk/debug/app-debug.apk
```

### 2. 触发构建
- 推送代码到 GitHub
- 在 Actions 页面手动触发

### 3. 下载 APK
- 在 Actions 运行完成后，下载 Artifact 中的 APK

---

## 方案四：使用 Android 模拟器

### 1. 安装 Android Studio
- 同方案一

### 2. 创建模拟器
- `Tools` → `Device Manager` → `Create Device`
- 选择设备型号（如 Pixel 6）
- 选择系统镜像（如 Android 14）
- 完成创建

### 3. 运行
- 启动模拟器
- 点击 "Run" 按钮

---

## 常见问题

### 1. SDK 版本不匹配
```
Could not determine the dependencies of task ':app:compileDebugJavaWithJavac'.
> Failed to find target with hash string 'android-35'
```

**解决方案：**
- 安装 Android SDK 35
- 或修改 `build.gradle.kts` 中的 `compileSdk` 和 `targetSdk`

### 2. JDK 版本错误
```
Unsupported class file major version 65
```

**解决方案：**
- 安装 JDK 17
- 设置 `JAVA_HOME` 环境变量

### 3. Gradle 下载慢
```
Timeout downloading gradle-x.x-bin.zip
```

**解决方案：**
- 使用国内镜像源
- 修改 `gradle/wrapper/gradle-wrapper.properties`：
  ```properties
  distributionUrl=https\://mirrors.cloud.tencent.com/gradle/gradle-8.7-bin.zip
  ```

---

## 项目配置

### 版本信息
- Min SDK: 26 (Android 8.0)
- Target SDK: 35 (Android 15)
- Compile SDK: 35
- JDK: 17

### 主要依赖
- Jetpack Compose
- Hilt (DI)
- Retrofit (网络)
- Room (数据库)
- Media3 (播放器)
- Coil (图片加载)

---

## 快速开始（推荐）

1. 安装 Android Studio
2. 打开 `android` 目录
3. 等待 Gradle 同步完成
4. 连接设备或启动模拟器
5. 点击 "Run" 按钮

**预计时间：** 10-15 分钟（首次）
