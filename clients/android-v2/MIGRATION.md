# Android 正式替换与迁移策略

本文档记录原 Android V2 模块化客户端正式接管 Android V1 后的包名、签名、versionCode、数据迁移和回滚规则。

## 最终决策

| 项目 | 正式策略 |
|---|---|
| 历史 V1 applicationId | `com.nowen.video` |
| 当前正式 Android applicationId | `com.nowen.video` |
| 当前 Debug applicationId | `com.nowen.video.debug` |
| 源码 namespace | `com.nowen.video.v2`（仅内部代码边界） |
| 正式签名 | **必须继续使用 V1 原正式签名证书** |
| V1 → 当前 Android | 原位覆盖升级 |
| 服务器配置 | 首次升级自动尝试迁移 |
| V1 Token / 密码 | 不迁移，必要时重新登录 |
| 独立 V2 测试包 `com.nowen.video.v2` | 不属于正式升级链，可并行存在后手动卸载 |

## 为什么现在可以接管 `com.nowen.video`

V2 的服务器发现、认证、媒体库、搜索、剧集、原生播放、字幕、历史/收藏、离线下载和多服务器能力已经形成稳定模块边界，并拥有 Android 8 / 13 / 15 的自动化与真机验证链路。继续保留第二个公开包名只会让用户面对“V1 / V2”两个产品入口，因此从本阶段开始：

**V2 是代码代际名称，不再是产品名称。对外只有 Nowen Video Android。**

## 覆盖升级的三个硬条件

Android 原位升级必须同时满足：

1. applicationId 相同：`com.nowen.video`；
2. 签名证书相同；
3. 新 APK versionCode 高于设备已安装版本。

代码已经把这三项都纳入正式发布链路。

### applicationId

`clients/android-v2/app/build.gradle.kts` 的 Release applicationId 已接管 `com.nowen.video`。源码 package/namespace 暂时保留 `com.nowen.video.v2`，避免为了产品命名做大范围 Kotlin 包迁移而引入无意义回归。

### 签名

正式发布 workflow 继续沿用现有 `ANDROID_V2_*` Secret 名称，避免仓库配置一次性迁移，但这些 Secret **现在必须装载历史 V1 的 release keystore**。

发布时不仅校验配置的 SHA-256，还会下载已经公开发布的 `v1.2.5` V1 APK，现场读取其签名证书并与新 APK 比对。两者不一致时发布直接失败，禁止生成“必须卸载旧版才能安装”的伪升级版本。

### versionCode

`scripts/android-v2-version.sh` 是唯一计算来源。当前公式在原模块化版本规则上增加 `10,000,000` 的正式接管偏移：

```text
base = 10,000,000 + MAJOR * 10,000,000 + MINOR * 100,000 + PATCH * 1,000

alpha.N = base + 100 + N
beta.N  = base + 300 + N
rc.N    = base + 500 + N
stable  = base + 999
```

这保证任何正式接管版本都高于历史 V1 的 versionCode 区间，同时保持：

```text
alpha < beta < rc < stable
```

示例：

| versionName | versionCode |
|---|---:|
| `0.1.0-alpha.1` | `10100101` |
| `0.1.0-beta.1` | `10100301` |
| `0.1.0-rc.1` | `10100501` |
| `0.1.0` | `10100999` |
| `1.2.3-rc.4` | `20203504` |
| `1.2.3` | `20203999` |

正式版本应与仓库产品版本保持一致，例如 `v1.2.9` 标签生成 Android `1.2.9`，而不是重新从 `0.1.0` 对用户计数。

## V1 数据迁移

切回相同 applicationId 后，新客户端会运行在原 V1 Android sandbox 中，因此可以读取 V1 私有 DataStore，而不需要共享 UID、导出组件或存储权限。

已支持读取：

- `server_profiles`：多服务器名称、URL、活跃服务器；
- `nowen_prefs`：非常旧版本保存的单一 `server_url` 作为兜底。

迁移行为：

1. 在当前会话初始化前执行一次；
2. 对 URL 做当前客户端的标准化与合法性校验；
3. 导入服务器名称、URL 和活跃服务器选择；
4. 写入 `nowen_v2_session`；
5. 写入一次性迁移标记，后续启动不重复导入。

### 为什么不迁移 Token

V1 的 `server_profiles` / `nowen_prefs` 曾以普通偏好值保存 JWT；当前 Android 使用 `Android Keystore + AES/GCM` 的 `nowen_v2_credentials`。把旧明文 Token 直接搬进新凭据层会扩大凭据暴露面，也可能把已经失效的会话带入新客户端。

因此：

- **服务器地址迁移；**
- **Token、密码不迁移；**
- 用户升级后如进入登录页，重新登录一次即可；
- 新 Token 只进入 Android Keystore 保护的凭据仓库。

## 安装矩阵

| 场景 | 结果 |
|---|---|
| 官方 V1 → 当前正式 Android | `adb install -r` / 系统更新，原位覆盖 |
| 当前正式 Android → 更高版本 | 原位覆盖，当前数据保留 |
| 高 versionCode → 低 versionCode | Android 默认拒绝，预期行为 |
| `com.nowen.video.v2` 旧测试包 + 当前正式版 | 两者可暂时并存，互不迁移 |
| Debug + Release | `com.nowen.video.debug` 与 `com.nowen.video` 可并行 |
| 先卸载 V1 再安装当前版 | 无法读取 V1 私有数据，不推荐 |

## 回滚策略

不使用 APK 降级作为生产回滚，也不要求用户卸载应用。

如果正式 Android 出现严重问题：

1. 停止继续放量；
2. 修复问题；
3. 用相同 V1 正式签名发布更高 versionCode；
4. 用户直接覆盖升级到修复版本。

旧 `android/` V1 源码继续保留在仓库中作为迁移、数据格式和历史兼容参考，但**不再参与正式 Android 发布**。

## 发布验收

正式 Android 发布至少满足：

- Release applicationId 为 `com.nowen.video`；
- versionCode 高于 V1 区间且与版本脚本一致；
- APK / AAB 均成功签名；
- APK 签名与历史 V1 正式 APK 一致；
- release manifest、SHA256SUMS 与实际产物一致；
- V1 覆盖安装成功，不要求卸载；
- V1 服务器地址迁移成功；
- 未迁移旧明文 Token；
- Android 8 / 13 / 15 启动、登录、播放和下载关键链路通过。
