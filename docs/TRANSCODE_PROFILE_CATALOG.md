# 转码质量目录

`internal/transcode/profile` 是 Nowen Video 的服务端质量档位权威。实时转码、ABR 状态和旧预处理流水线不得再各自维护一套分辨率名称、尺寸和音频码率。

## 档位顺序

```text
360p -> 480p -> 720p -> 1080p -> 2K -> 4K
```

顺序用于：

- 根据源视频高度过滤可用档位。
- 生成 ABR 状态与 Master Playlist。
- 选择不超过源分辨率的最高预处理档位。
- 保证 Web、Android 和兼容客户端看到稳定的档位名称。

## 两种显式码率策略

统一质量目录不代表实时播放与长期产物必须使用相同码率。

### Runtime

用于按需实时 HLS。它保留现有实时转码码率，并继续允许软件 CRF、QSV Global Quality 等编码器质量模式覆盖固定码率行为。

### Persistent

用于 ABR 与旧 PreprocessService 的长期产物。该策略包含：

- `video_bitrate`
- `max_bitrate`
- `buf_size`
- 与 Runtime 共用的尺寸和音频码率

两种策略由同一个 `Preset` 派生，因此不会出现“1080p 在一个模块是 1920×1080，在另一个模块却使用另一套命名或音频码率”的漂移。

## 兼容边界

`service.ABRProfile` 暂时保留为共享 `EncodingProfile` 的类型别名，使尚未迁移的 `PreprocessService` 无需复制质量表。

后续迁移 PreprocessService 时应直接消费 `internal/transcode/profile`，并删除服务层的 `abrProfiles` 兼容变量。

## 变更规则

调整档位时必须同时满足：

1. 保持档位按高度递增。
2. 不允许同名档位出现不同尺寸。
3. Runtime 与 Persistent 的差异必须作为明确字段存在，不能复制成两张表。
4. 返回的目录切片必须是副本，调用方不能修改全局配置。
5. 更新目录契约测试，并验证 Lite / Full 编译和 Docker 构建。
