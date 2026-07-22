# Android V2 RC 真机 P0 助手

本指南用于对已经通过 `scripts/android-v2-release-candidate.sh` 核验的正式候选包执行实体设备 P0。完整用例来源仍是 [SMOKE_TEST.md](./SMOKE_TEST.md)。助手只负责候选包锁定、ADB 自动检查、证据采集、人工结果记录和矩阵汇总，不会代替人工播放、下载、网络和升级验证。

## 安全边界

- 默认使用 `adb install -r` 覆盖安装，不清除 V2 数据。
- 只有显式指定 `--install-mode fresh` 才会卸载 V2 后重装。
- 不会登录账号、输入密码、读取应用私有目录或自动公开 Release。
- 原始 logcat、截图和 UI XML 写入 `evidence-local-only/`，可能包含服务器信息，禁止未经检查直接上传公开 Issue。
- 公开 Markdown 报告只保留候选版本、证书、设备公开属性、用例状态和脱敏备注。
- 单设备 `PASS` 不是 RC 放行；矩阵必须包含同一候选包的 API 26、33、35，并至少有一台 `full` 会话。

## 准备

需要：

- Python 3.10+
- Android Platform Tools：`adb`
- Android SDK Build Tools：`apksigner`
- JDK 17：`jarsigner`
- 已下载并核验的正式候选目录，例如 `dist/android-v2/rc1-candidate`
- 已开启 USB 调试且授权的 Android 设备

确认设备：

```bash
adb devices -l
```

连接多台设备时必须在命令中加 `--serial <serial>`，助手不会猜测目标设备。

## 1. 创建完整 P0 会话

在主测试设备执行全部 P0：

```bash
python3 scripts/android-v2-device-p0.py prepare \
  --candidate-dir "$PWD/dist/android-v2/rc1-candidate" \
  --session-dir "$PWD/dist/android-v2/p0/main-device" \
  --version 0.1.0-rc.1 \
  --expected-commit '<RC main commit SHA>' \
  --expected-fingerprint '<release certificate SHA-256>' \
  --tester 'cropflre' \
  --server-version '<server version or commit>' \
  --network-profile MIXED \
  --scope full \
  --install-mode replace
```

自动执行：

1. 重新核验候选包哈希、manifest、commit、证书和 Release Notes；
2. 本地验证 APK/AAB 签名；
3. 精确选择一台已授权设备；
4. 采集设备型号、Android/API、ABI、分辨率、density、build fingerprint、存储和电量；
5. 记录旧版与 V2 安装状态；
6. 安装或检查候选 APK；
7. 核对设备上的 versionName/versionCode；
8. 启动应用并确认进程和前台 Activity；
9. 采集启动 logcat、截图、UI hierarchy 和 dumpsys；
10. 检测启动阶段的崩溃、ANR 和 native fatal signal；
11. 根据 `P0_CASES.json` 生成待填写清单。

输出：

```text
p0-session.json
P0_REPORT.md
evidence-local-only/
```

## 2. API 26 / 33 / 35 启动会话

另外两台设备可以使用 `startup` 范围，只要求安装、迁移提示、服务器设置入口和正式包身份相关项：

```bash
python3 scripts/android-v2-device-p0.py prepare \
  --candidate-dir "$PWD/dist/android-v2/rc1-candidate" \
  --session-dir "$PWD/dist/android-v2/p0/api26" \
  --version 0.1.0-rc.1 \
  --expected-commit '<RC main commit SHA>' \
  --expected-fingerprint '<release certificate SHA-256>' \
  --tester 'cropflre' \
  --server-version '<server version or commit>' \
  --network-profile LAN \
  --scope startup
```

API 33、35 分别建立独立目录。三台设备必须使用完全相同的 commit、证书和 APK SHA-256。

## 3. 记录人工结果

每个 `--result` 使用稳定 case ID：

```bash
python3 scripts/android-v2-device-p0.py record \
  --session-dir "$PWD/dist/android-v2/p0/main-device" \
  --result INST-01=PASS \
  --result INST-02=PASS \
  --result AUTH-01=BLOCKED \
  --note 'AUTH-01=测试路由器禁用了组播，已改用另一网络复测'
```

允许状态：

- `PASS`
- `FAIL`
- `BLOCKED`
- `N/A`

`FAIL`、`BLOCKED` 和 `N/A` 必须附公开安全的备注。备注禁止包含 URL、IP、Token、密码、Cookie 或其他凭据；使用“已脱敏服务器”“测试账号”等描述。

## 4. 完成单设备会话

```bash
python3 scripts/android-v2-device-p0.py finalize \
  --session-dir "$PWD/dist/android-v2/p0/main-device"
```

Fail-closed 规则：

- 自动检查出现 `FAIL` 或 `BLOCKED`，不能 PASS；
- 任一必测 P0 仍为 `PENDING`，不能 PASS；
- 用例目录哈希、scope、case ID 或 required case 被修改，结果为 `BLOCKED`；
- 人工用例出现 `FAIL` 或 `BLOCKED`，按对应结论输出；
- 只有自动检查全部通过、必测项全部记录且无阻断，单设备会话才为 `PASS`。

## 5. 汇总 RC 设备矩阵

```bash
python3 scripts/android-v2-device-p0.py matrix \
  --session-dir "$PWD/dist/android-v2/p0/main-device" \
  --session-dir "$PWD/dist/android-v2/p0/api26" \
  --session-dir "$PWD/dist/android-v2/p0/api33" \
  --session-dir "$PWD/dist/android-v2/p0/api35" \
  --output-dir "$PWD/dist/android-v2/p0/matrix"
```

矩阵 PASS 要求：

- 所有输入会话已经 finalized PASS；
- versionName、versionCode、commit、证书和 APK SHA-256 完全一致；
- 精确覆盖 API 26、33、35；
- 至少一个 `full` 会话。

输出：

```text
p0-matrix.json
P0_MATRIX_REPORT.md
```

矩阵 PASS 仍不会自动公开草稿 prerelease。

## 6. 显式发布脱敏报告到 #55

确认 Markdown 不包含私有信息后：

```bash
python3 scripts/android-v2-device-p0.py post \
  --report "$PWD/dist/android-v2/p0/matrix/P0_MATRIX_REPORT.md" \
  --repository cropflre/nowen-video \
  --issue 55
```

`post` 会拒绝 `PENDING` 报告，以及检测到 URL、IP、凭据或 Token 特征的内容。它不会上传 `evidence-local-only/`。

## Fresh install

只有确认允许清除当前 V2 私有数据时才使用：

```bash
--install-mode fresh
```

该模式会先执行 `adb uninstall com.nowen.video.v2`。旧版 `com.nowen.video` 不会被卸载。

## 离线自测

```bash
python3 scripts/android-v2-device-p0.py self-test
```

自测使用 fake ADB，覆盖：

- 候选包结构与哈希核验；
- prepare 自动采集；
- record；
- finalize；
- API 26/33/35 matrix；
- 错误候选 commit 拒绝。
