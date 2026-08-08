# Produced-media Attestation

## 1. 目标

Encoding Plan 描述转码器**应该生成什么**；Produced-media Attestation 证明 FFmpeg **实际生成了什么**。

两者职责不可互换：

- Encoding Plan 是提交 Job 前形成的不可变输出契约。
- Attestation 是对已物化 MPEG-TS 分片执行 ffprobe 后形成的不可变证据。
- 只有计划身份和实际证据都成立，Startup / Continuation Artifact 才能进入播放解析链。

当前协议版本：

```text
hls-produced-media-attestation-v1
```

本阶段不会删除 `#EXT-X-DISCONTINUITY`，也不宣称 Sample-perfect 无缝接力。

## 2. 领域模型

Attestation 记录：

- 关联的 `encoding_plan_version` 和 `encoding_plan_hash`
- 验证范围：`first_segment` 或 `complete`
- HLS 分片数量
- 首分片和尾分片文件名
- 视频 Codec、Profile、Level、宽高、Pixel Format、色彩信息、帧率、Time Base
- 音频 Codec、声道、采样率和 Time Base
- 音视频首个 PTS/DTS、最后 PTS/DTS、最后 Packet End PTS
- 对应的毫秒级起止检查点

Canonical JSON 不包含：

- 文件系统绝对路径
- Worker、Job、Attempt 或 Lease Token
- FFmpeg 命令行
- 用户或播放 Session 身份
- 数据库主键

Attestation 使用稳定 JSON 和 SHA-256 生成身份：

```text
attestation_version
attestation_hash
attestation_json
```

## 3. 状态

Artifact 的证明状态：

```text
""              未验证
provisional     已验证当前首分片
verified        已验证完整产物首尾分片
```

### 3.1 provisional

仅用于仍在生成的 Startup Continuation：

- Manifest 已经包含首个 `.ts` 分片。
- Artifact 仍是当前 Job 的 current Attempt。
- Job Lease 必须有效。
- ffprobe 实际输出必须匹配 Encoding Plan。
- 证明写入后，Resolver 才能返回该 Live Artifact。

`provisional` 不等于可以发布。它只证明“当前已经可读取的第一段媒体符合输出契约”。

### 3.2 verified

用于不可变 Published Artifact：

- FFmpeg 已正常结束。
- Artifact Store 的 HLS 文件完整性检查已通过。
- 首分片和尾分片均完成 ffprobe。
- 实际音视频输出匹配 Encoding Plan。
- 证明由当前 Lease 所有者写入。

只有 `verified` Artifact 才能进入 `publishing -> published`。

## 4. 发布协议

正式发布顺序：

```text
FFmpeg 完成
  -> ValidateHLS
  -> ffprobe 首尾分片
  -> VerifyAgainstEncodingPlan
  -> Lease-fenced RecordOwnedArtifactAttestation
  -> PrepareArtifactPublish
  -> 文件系统原子重命名
  -> CommitArtifactPublishAndCompleteJob
```

数据库在 `PrepareArtifactPublish` 和最终 Commit 两个位置再次检查：

- 带 Encoding Plan 的 Artifact 必须为 `attestation_status = verified`
- Attestation version/hash/json 必须存在
- Job、Attempt、Lease 和 current Attempt 必须仍然一致

因此，即使上层服务遗漏了验证调用，计划化 Artifact 也无法进入 Published 状态。

没有 Encoding Plan 的历史 Runtime HLS 继续使用旧发布协议，避免一次性破坏现有播放链。它们不能进入 Startup Bridge。

## 5. Resolver 围栏

### Published Startup / Continuation

必须满足：

- Media、Profile、Source Fingerprint 一致
- Planner Version、Artifact Kind 一致
- Encoding Plan Version/Hash 一致
- `status = published`
- `attestation_status = verified`
- Attestation identity 完整且可重新计算
- Attestation 实际输出匹配 Encoding Plan

### Live Continuation

必须满足：

- Artifact 属于当前 Attempt
- Lease 未过期
- Job 处于 claimed/running
- Encoding Plan 与 Job 一致
- `attestation_status in (provisional, verified)`
- Startup 尾分片与 Continuation 首分片实际 Stream Identity 兼容

未验证的 staging Artifact 不再可读。

## 6. Bridge 兼容条件

`BridgeCompatible` 当前验证：

- Encoding Plan 身份一致
- 视频 Codec 与 Time Base 一致
- 视频宽高、Pixel Format、色彩元数据一致
- 音频 Codec、Time Base、声道和采样率一致

它**不验证**：

- Startup 尾 Packet 与 Continuation 首 Packet 的连续 PTS/DTS 关系
- AAC Priming / Encoder Delay
- GOP 内部 Sample 依赖
- 不同硬件编码器的 Sample Description 完全一致
- 客户端解码器在无 discontinuity 条件下的真实行为

所以 Bridge 继续输出：

```text
#EXT-X-DISCONTINUITY
```

## 7. 数据库迁移

`transcode_artifacts` 新增：

```text
attestation_version
attestation_status
attestation_hash
attestation_json
timeline_start_ms
timeline_end_ms
attested_at
```

迁移是 additive：

- 不删除旧表或字段。
- 不修改旧 Artifact 文件。
- 不为历史行伪造证明。
- 旧行保持空 Attestation，继续用于回滚和诊断。
- 旧 Startup / Continuation Artifact 不会被新 Resolver 复用。
- 后续 Warm-up 或播放提交会生成新版本 Artifact。

## 8. 回滚

旧版本程序可以忽略新增字段。

回滚不会：

- 删除新 Artifact
- 删除旧 Artifact
- 修改媒体源文件
- 重写历史任务

若从新版本回滚到旧版本，旧版本可能重新读取未经过 Attestation 围栏的旧路径；这是程序版本回滚恢复出的旧行为，不代表新版本证明失效。

## 9. 失败语义

主要错误码：

```text
artifact_attestation_failed
```

可能原因：

- Manifest 无分片或 URI 不安全
- ffprobe 不可用、超时或返回非法 JSON
- 分片缺少视频或音频流
- Codec、宽高、Pixel Format、色彩或声道不符合计划
- Packet 时间戳不可用
- Lease 在证明写入前丢失

失败 Artifact 保留错误证据，但不会发布。

## 10. 安全边界

客户端只允许看到：

- Attestation Version
- Attestation Hash
- 是否已通过服务端验证

客户端不应获得：

- Canonical Attestation JSON
- PTS/DTS 明细
- 文件系统路径
- Job、Attempt、Artifact ID
- Lease Token
- ffprobe 命令或原始输出

## 11. 后续阶段

移除 discontinuity 前仍需完成：

1. 定义 Startup 尾检查点到 Continuation 首检查点的正式时间戳关系。
2. 固定音视频 Timestamp Origin 与 MPEG-TS wrap 策略。
3. 记录 AAC Priming、Encoder Delay 和 Padding。
4. 证明跨硬件后端的 Sample Description 一致性。
5. 生成独立 Timeline Continuity Attestation，而不是复用 Stream Identity 证明。
6. 在 Web、ExoPlayer、mpv、Emby 和 Infuse 上执行真实切换测试。
7. 建立失败降级与首帧 P50/P95/P99 基线。
