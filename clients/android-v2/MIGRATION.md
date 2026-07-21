# Android V2 版本与迁移策略

本文档记录 Android V2 在 RC1 和首个稳定版本阶段的包名、签名、版本号、升级与旧版迁移决策。

## 决策摘要

| 项目 | RC1 / 首个稳定版本决策 |
|---|---|
| 旧版 applicationId | `com.nowen.video` |
| V2 Release applicationId | `com.nowen.video.v2` |
| V2 Debug applicationId | `com.nowen.video.v2.debug` |
| V2 签名 | 使用同一份长期保存的 V2 release keystore |
| 与旧版关系 | 独立安装、独立数据、互不覆盖 |
| 服务器迁移 | 用户重新添加，支持手动、局域网发现和二维码 |
| 登录迁移 | 不复制旧版 Token，用户重新登录 |
| RC → Stable | 同一 V2 applicationId + 同一签名 + 递增 versionCode，支持覆盖升级 |

## 为什么 RC1 不切换到旧版包名

旧版客户端使用 `com.nowen.video`，V2 当前使用 `com.nowen.video.v2`。RC1 继续保留独立包名，原因如下：

1. **避免覆盖尚未完成全量真机验证的旧版客户端。** 用户可以同时保留旧版和 V2，对比播放、下载和连接结果。
2. **Android 应用数据按 applicationId 隔离。** 直接改成旧包名并不等于安全迁移，还必须拥有完全相同的签名，并处理旧版 DataStore、Room、缓存和偏好结构。
3. **旧版会话模型与 V2 不同。** 旧版服务器资料中包含明文 Token；V2 使用 Android Keystore + AES/GCM 按服务器加密 Token，不能简单复制文件。
4. **保留回退路径。** V2 出现设备兼容问题时，用户仍可打开旧版，不需要卸载、降级或清除数据。

在 V2 完成至少一个稳定版本周期、真机覆盖和迁移演练之前，不切换 `com.nowen.video`。

## 为什么不自动读取旧版数据

旧版和 V2 处于不同 Android 沙箱：

- 旧版 DataStore：`nowen_prefs`、`server_profiles`
- V2 DataStore：`nowen_v2_session`
- V2 凭据：`nowen_v2_credentials` + Android Keystore

不同 applicationId 默认无法读取对方的私有 DataStore、SharedPreferences 或数据库。为绕过系统隔离而增加导出组件、共享 UID 或宽泛文件权限，会扩大凭据泄露面，因此 RC1 不采用。

### RC1 迁移行为

- V2 首次启动明确提示：不会覆盖或读取旧版数据。
- 用户通过局域网发现、二维码或手动地址重新添加服务器。
- 用户重新输入账号密码登录。
- V2 登录后仅在自己的 Android Keystore 加密存储 Token。
- 旧版数据保持原样，卸载 V2 不影响旧版。

### 后续可选迁移能力

未来如需降低迁移成本，只迁移非敏感信息：

- 由旧版显式导出服务器名称和 URL；
- 使用带用户确认的二维码或 `nowen-video://server` 深链导入；
- 不导出密码、JWT、刷新凭据或 Android Keystore 密钥；
- 导入后仍执行服务器健康检测并要求重新登录。

## 版本名称规则

允许以下格式：

```text
MAJOR.MINOR.PATCH-alpha.N
MAJOR.MINOR.PATCH-beta.N
MAJOR.MINOR.PATCH-rc.N
MAJOR.MINOR.PATCH
```

约束：

- `MAJOR`：0–199
- `MINOR`：0–99
- `PATCH`：0–99
- 预发布序号 `N`：1–99
- 不接受 `preview`、`snapshot`、无序号 RC 等自由格式

示例：

```text
0.1.0-alpha.1
0.1.0-beta.1
0.1.0-rc.1
0.1.0
```

## versionCode 规则

`scripts/android-v2-version.sh` 是唯一计算来源：

```text
base = MAJOR * 10,000,000 + MINOR * 100,000 + PATCH * 1,000

alpha.N = base + 100 + N
beta.N  = base + 300 + N
rc.N    = base + 500 + N
stable  = base + 999
```

因此同一语义版本始终满足：

```text
alpha < beta < rc < stable
```

示例：

| versionName | versionCode |
|---|---:|
| `0.1.0-alpha.1` | `100101` |
| `0.1.0-beta.1` | `100301` |
| `0.1.0-rc.1` | `100501` |
| `0.1.0` | `100999` |
| `1.2.3-rc.4` | `10203504` |
| `1.2.3` | `10203999` |

发布 workflow 不再允许手工填写 versionCode，防止覆盖升级失败或版本倒退。

## 渠道与安装关系

| 构建 | applicationId | 签名 | 是否覆盖 V2 Release |
|---|---|---|---|
| Debug | `com.nowen.video.v2.debug` | Android debug key | 否，可并行安装 |
| RC | `com.nowen.video.v2` | V2 release key | 是 |
| Stable | `com.nowen.video.v2` | 同一 V2 release key | 是 |
| 旧版 | `com.nowen.video` | 旧版 release key | 否，可并行安装 |

RC 与 Stable 必须使用相同的正式 V2 keystore。临时 CI keystore 产物只用于构建验证，不能分发给用户。

## 升级与回滚矩阵

| 场景 | 预期结果 | 验证要求 |
|---|---|---|
| 旧版 + V2 RC 并行安装 | 两个图标、数据互不影响 | 必测 |
| V2 RC1 → RC2 | 原位升级，服务器和登录会话保留 | 必测 |
| V2 RC → Stable | 原位升级，下载记录和偏好保留 | 必测 |
| V2 Stable → 更高 Stable | 原位升级 | 必测 |
| V2 高 versionCode → 低 versionCode | Android 默认拒绝安装 | 预期行为 |
| 卸载 V2 后重装 | V2 本地数据清除，旧版不受影响 | 必测 |
| V2 故障时打开旧版 | 旧版继续可用 | 必测 |

不把 APK 降级作为正式回滚方案。发布失败时应修复并发布更高 versionCode；需要保留用户数据时不得要求卸载。

## 切换正式旧包名的前置条件

只有同时满足以下条件，才重新评估 V2 是否接管 `com.nowen.video`：

- V2 至少完成一个稳定版本周期；
- P0 真机回归覆盖 Android 8、13、15；
- 已确认旧版正式签名可安全用于 V2，或有明确的新包迁移方案；
- 已设计并验证旧版数据导出/导入或明确放弃迁移的用户告知；
- 覆盖升级、回滚、下载文件和会话恢复测试通过；
- 发布说明明确告知用户风险与操作步骤。
