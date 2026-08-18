# Media Compute Node V2

Media Compute Node V2 将原本只服务于“精彩片段”的远程 Worker 协议提升为可复用的媒体计算节点协议。

## 当前边界

当前真正接入的第一个 job 仍然是：

- `job_type = highlight_v1`
- `required_capability = highlight_v1`

调度策略不变：

1. 空闲 Desktop 在短优先窗口内优先领取。
2. Desktop 不可用、忙碌、播放中或心跳过期后，Android 可以领取。
3. `auto` 模式没有可用客户端时，Server Sparse V2 兜底。
4. 进度上报继续续租并刷新节点在线状态。

这次重构只改变“协议与执行器边界”，不改变已经稳定的租约、fingerprint、安全校验和 Server fallback 行为。

## V2 Claim

V2 Claim 增加统一任务信封：

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

客户端必须先按 `job_type` 分派执行器，再读取 `input`。不能继续假定每个 Claim 都一定是精彩片段。

## V1 兼容

服务器暂时继续在 Claim 顶层返回：

- `media_id`
- `fingerprint`
- `duration`
- `stream_url`
- `sample_times`
- `max_highlights`
- `engine_version`

因此已经发布、只认识旧扁平协议的 Android/Desktop 客户端仍能领取并完成精彩片段任务。

现有 `/api/media-analysis/workers/**` URL 也暂时保留为兼容传输层。V2 的核心契约是任务信封与能力分派，不要求客户端和服务器在同一版本同时切换 URL。

## 节点视图

管理端节点响应新增：

- `client_protocol_version`
- `current_job_type`

新 Desktop/Android 分别通过 `desktop-v2/...`、`android-v2/...` 版本标识声明已经理解 V2 Claim。旧客户端继续显示为协议版本 1。

## 后续 job 接入规则

后续增加缩略图、章节、预览图、音频波形、字幕处理时，应遵循以下规则：

1. 每个任务定义独立 `job_type`。
2. 每个执行能力定义独立 `required_capability`。
3. job 私有参数只进入 `input`，禁止继续增加顶层扁平字段。
4. Desktop/Android 只声明自己真实可执行的 capabilities。
5. Server 必须按 required capability 选择节点，不能因为节点在线就把未知 job 分配给它。
6. progress / lease / heartbeat / failure 继续使用统一节点生命周期。
7. job 的结果校验和持久化仍由 Server 对应 adapter 负责，客户端结果永远不能直接信任。
8. 新 job 必须有 Server fallback 或显式声明 `client_only`，不能无限等待不存在的节点。

## 推荐下一步

下一步优先把“通用任务描述符 + capability-aware 选取”下沉到服务端调度器，让 `highlight_v1` 不再是调度器内部的隐式默认值。完成后再接入第二个真实 job（建议预览图或章节检测），用第二种任务验证 V2 抽象不是只换了一层命名。
