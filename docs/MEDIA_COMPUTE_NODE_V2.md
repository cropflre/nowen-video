# Media Compute Node V2

Media Compute Node V2 把原本只服务于“精彩片段”的远程 Worker 协议提升为可复用的媒体计算节点协议。

## 当前已经统一的边界

V2 统一以下生命周期：

1. Node heartbeat / online state
2. capability declaration
3. capability-aware task claim
4. Desktop preference window
5. claim token / lease
6. progress + lease renewal
7. failure release
8. generic job result envelope

当前真正接入生产结果 adapter 的第一个 job 仍然是：

- `job_type = highlight_v1`
- `required_capability = highlight_v1`

精彩片段原有调度策略不变：

1. 空闲 Desktop 在短优先窗口内优先领取 `highlight_v1`。
2. Desktop 不可用、忙碌、播放中、心跳过期或不具备该 capability 时，Android 可以领取。
3. `auto` 模式没有可用客户端时，Server Sparse V2 兜底。
4. 进度上报继续续租并刷新节点在线状态。

重要变化是：Desktop 优先现在按 **required capability** 生效。一个只会执行 `thumbnail_v1` 的 Desktop 不会阻塞 Android 去领取 `waveform_v1`。

## V2 Claim envelope

```json
{
  "protocol_version": 2,
  "job_type": "highlight_v1",
  "required_capability": "highlight_v1",
  "task_id": "...",
  "claim_token": "...",
  "lease_expires_at": "...",
  "input": {
    "media_id": "...",
    "fingerprint": "...",
    "duration": 7200,
    "stream_url": "/api/stream/.../direct",
    "sample_times": [120, 360],
    "max_highlights": 8,
    "engine_version": 3
  }
}
```

客户端必须先按 `job_type` 分派执行器，再解析对应的 `input`。Android V2 使用通用 `JsonObject` 保存 input，Desktop V2 使用 unknown + job adapter 校验，因此协议不再被精彩片段字段锁死。

## V2 result envelope

```json
{
  "claim_token": "...",
  "job_type": "highlight_v1",
  "result": {
    "fingerprint": "...",
    "highlights": []
  }
}
```

服务端先验证 claim 与 job_type，再交给对应结果 adapter。当前 `highlight_v1` adapter 继续执行原来的 fingerprint、时间范围、分数、缩略图魔数、单图大小和总 payload 安全校验。

## V1 兼容

为了允许 Server、Desktop、Android 分批升级：

- 现有 `/api/media-analysis/workers/**` URL 暂时保留为兼容传输层。
- highlight Claim 顶层继续镜像 `media_id / fingerprint / duration / stream_url / sample_times / max_highlights / engine_version`。
- 旧客户端会忽略 V2 新字段，并继续读取旧扁平字段。
- complete 路由同时接受 V1 扁平结果和 V2 `job_type + result` envelope。

因此 V2 不要求所有端同一时间升级。

## 通用任务注册

服务端新增 `RegisterComputeTask(MediaComputeTaskRegistration)` / `UnregisterComputeTask(taskID)` 协调边界。后续业务只需要提供：

- `task_id`
- `job_type`
- `required_capability`
- job 私有 `input` JSON
- 可选 `media_id / fingerprint`

节点协调层负责在线状态、capability 匹配、Claim、租约和优先级；业务服务继续负责自己的 durable task、Server fallback 与 result adapter。

这意味着接入第二个 job 时，不需要再创建一套 Worker heartbeat / claim / lease 协议。

## Node view

管理端节点响应新增：

- `client_protocol_version`
- `current_job_type`

新 Desktop/Android 分别通过 `desktop-v2/...`、`android-v2/...` 标识理解 V2 envelope。旧客户端显示为协议版本 1。

## 新 job 接入规则

1. 每个任务定义独立 `job_type`。
2. 每个执行能力定义独立 `required_capability`。
3. job 私有参数只进入 `input`，禁止继续增加 Claim 顶层业务字段。
4. Desktop/Android 只声明自己真实可执行的 capabilities。
5. Server 必须按 required capability 选择节点，不能因为节点在线就分配未知 job。
6. progress / lease / heartbeat / failure 使用统一节点生命周期。
7. job 结果必须通过 Server adapter 校验后才能持久化，客户端结果永远不直接信任。
8. 每个业务 dispatcher 必须定义 Server fallback，或显式采用 client-only 策略；不能无限等待不存在的节点。

## 下一验证阶段

V2 的协议与 capability-aware 调度已经不再依赖精彩片段。下一步应接入一个**第二真实 job**验证抽象，优先建议：

1. `preview_thumbnail_v1`：复用 Desktop libmpv / Android 硬件抽帧，风险最低。
2. 然后 `chapter_detect_v1` 或 `waveform_v1`。

当第二个 job 完整跑通 Desktop / Android / Server fallback 后，再考虑把历史 `/media-analysis/workers/**` 路由迁移为 `/media-compute/**`，而不是为了命名提前破坏兼容性。
