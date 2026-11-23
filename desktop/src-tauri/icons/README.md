
# 应用图标

Tauri 打包图标与源图

```
icons/
├── 32x32.png          # 32×32 PNG
├── 128x128.png        # 128×128 PNG
├── 128x128@2x.png     # 256×256 PNG（视网膜）
├── icon.icns          # macOS 图标包
├── icon.ico           # Windows 图标包
├── icon.png           # 托盘与通用图标（512×512）
└── source-1024.png    # 1024×1024 导出图
```

## 更新图标

图标源文件是 `web/public/assets/nowen-video.svg`。可以在 `desktop/src-tauri` 目录运行以下命令。

```bash
npx --yes @tauri-apps/cli@^2 icon ../../web/public/assets/nowen-video.svg --ios-color '#0D1D33' --output icons
```

`source-1024.png` 从同一 SVG 以 1024×1024 单独导出。修改源图时同步更新飞牛、网页应用及原生 Android 的图标资源。
