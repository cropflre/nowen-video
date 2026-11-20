# MCP 接入（第一阶段：stdio）

nowen-video 内置 MCP server，让外部 AI 客户端（Claude Desktop 等）直接对
影片做查询、分级建议/确认、刮削与元数据增删改查。

第一阶段只做分级与刮削域，不做 HTTP 传输、不调内置 AI、不删内置 AI。
后续战略是去掉内置 AI、只依赖 MCP/外部 AI；MCP 工具只包确定性的非 LLM
能力（TMDb 刮削、本地分级规则、数据库读写）。

## 启动

```bash
server-lite mcp
```

- 与正常启动共用同一配置与数据库（`NOWEN_APP_DATA_DIR` 指定的数据目录）。
- 只走 stdio（JSON-RPC 2.0）：stdout 留给协议帧，日志只写 stderr。
- 不启动 HTTP 服务、mDNS、Emby 发现与文件监听。

## Claude Desktop 配置示例

```json
{
  "mcpServers": {
    "nowen-video": {
      "command": "/path/to/server-lite",
      "args": ["mcp"],
      "env": {
        "NOWEN_APP_DATA_DIR": "/path/to/data"
      }
    }
  }
}
```

把 `/path/to/server-lite` 换成实际二进制路径，`/path/to/data` 换成
nowen-video 的数据目录（与正常启动同一目录，否则看到的是空库）。
Windows 上 command 写 `server-lite.exe` 的绝对路径。

## 工具清单

| 工具 | 说明 |
| --- | --- |
| `search_media` | 搜索影片/剧集：关键词、类型、分级确认状态过滤、分页 |
| `get_media` | 取一部影片/剧集的详情：分级、确认状态、分级建议、TMDb 原始分级数据 |
| `list_unconfirmed_ratings` | 未确认分级的影片/剧集清单（分页） |
| `suggest_family_rating` | 只给分级建议，不写库 |
| `set_family_rating` | 写家长确认分级（默认关闭，见下） |
| `update_media_metadata` | 改元数据，只接受白名单字段：title、orig_title、year、overview、genres |
| `rescrape_media` | 对一部影片/剧集触发一次 TMDb 重刮 |
| `clear_tmdb_rating_data` | 删除某条刮削分级数据（只清 `tmdb_rating_data`） |
| `clear_family_rating` | 把确认分级清回未确认（默认关闭，见下） |

所有工具的 `kind` 为 `movie`（影片，默认）或 `series`（剧集）。
剧集的确认作用于当前全部剧集；新入库的剧集仍需再次确认。

分级建议规则与前端一致：仅美国明确分级才给建议，冲突、混合、未知
一律不建议；建议不自动保存、不放行儿童，家长确认后才生效。

## 直接写分级的开关

`set_family_rating` / `clear_family_rating` 默认是关闭的：
未开启时调用返回明确错误，AI 只能用 `suggest_family_rating` 给建议，
由家长在管理端确认。

开启方式（二选一，重启生效）：

```yaml
# config.yaml
mcp:
  allow_direct_rating: true
```

```bash
NOWEN_MCP_ALLOW_DIRECT_RATING=true server-lite mcp
```

## 安全红线

- **stdio 模式没有 token**：进程边界即信任边界，只允许本机可信客户端
  启动。不要把 `server-lite mcp` 的 stdin/stdout 暴露给网络或不可信进程。
- **第一阶段不开放远程 HTTP 传输**：不要自行用 socat/ncat 之类把
  stdio 转成网络服务，那会绕过所有访问控制。
- **写操作都记审计**：每一次写工具调用都在 `audit_logs` 表记一条，
  operator 为 `mcp`，action 为 `mcp.<工具名>`，detail 含改前改后值。
  可在管理端审计日志里查看。
- **没有 delete_media**：第一阶段不提供删除影片记录的工具；
  `clear_*` 只清分级/刮削数据，不删影片。
- **不调内置 AI**：重刮只走 TMDb/豆瓣/Bangumi 等确定性刮削源，
  不走 AI 元数据增强。
