# Tauri C 档资源目录

本目录由 `scripts/fetch-assets.ps1` 按需填充，资源文件已在 `.gitignore` 中忽略。

## 目录
- `mpv/`      libmpv-2.dll 等（嵌入模式必需，~35 MB）
- `shaders/`  Anime4K v4 GLSL（~1 MB）
- `fonts/`    思源黑体 CN（~30 MB，字幕兜底）
- `yt-dlp.exe`  在线流解析器（~13 MB）

## 首次构建

```powershell
powershell -ExecutionPolicy Bypass -File scripts/fetch-assets.ps1
```

跳过部分资源（例如只要 libmpv 不要字体）：

```powershell
powershell -ExecutionPolicy Bypass -File scripts/fetch-assets.ps1 -Skip "fonts;yt-dlp;shaders"
```

> 如果这些目录为空，Tauri 仍可构建，只是高级功能（Anime4K / 字幕兜底字体 / 在线流嗅探）将不可用。
