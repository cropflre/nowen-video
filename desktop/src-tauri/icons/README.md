
# 应用图标

Tauri 打包时需要以下文件，**全部必需**：

```
icons/
├── 32x32.png          # 32×32 PNG
├── 128x128.png        # 128×128 PNG
├── 128x128@2x.png     # 256×256 PNG（视网膜）
├── icon.icns          # macOS 图标包
├── icon.ico           # Windows 图标包
└── icon.png           # 通用回退（1024×1024）
```

## 快速生成

推荐使用 Tauri 官方图标生成工具：

```bash
# 准备一张 1024×1024 的 PNG 源图
cargo install tauri-cli --locked
cargo tauri icon path/to/your/logo.png
```

该命令会自动生成上述所有规格的图标到 `icons/` 目录。

## 临时占位

开发阶段可以先从主项目 `web/public/` 中找到 logo 图片，用上面命令生成一套基础图标。
