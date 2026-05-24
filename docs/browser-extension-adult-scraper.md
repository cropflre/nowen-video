# Chrome 浏览器刮削扩展

这个扩展用于在已打开的 JavDB/JavBus 详情页中读取页面元数据，并通过本地 Nowen Video 服务把 `.nfo`、`-poster.jpg`、`-fanart.jpg`、`extrafanart/`、`.actors/` 写入视频同目录。

## 安装

1. 打开 Chrome 的 `chrome://extensions/`。
2. 打开右上角“开发者模式”。
3. 选择“加载已解压的扩展程序”。
4. 选择仓库中的 `browser-extension` 目录。

## 使用

1. 启动 Nowen Video 服务，确认 `adult_scraper.enabled` 已开启。
2. 在 Chrome 中打开 `https://javdb.com/` 或 `https://www.javbus.com/` 的影片详情页。
3. 点击扩展图标。
4. 填写服务地址，默认是 `http://127.0.0.1:8080`。
5. 使用 Nowen 管理员账号登录。
6. 填写本机视频文件完整路径，例如 `D:\Media\SSIS-001.mp4`。
7. 插件会自动检查该路径所在目录是否已有 `.nfo`，没有时显示“未刮削，需要刮削”。
8. 点击“读取当前页”，确认识别到番号和标题。
9. 如果当前页是 JavDB，插件会同时读取页面上的短评数量。
10. 点击“写入 NFO/图片”。

## 输出文件

给定 `D:\Media\SSIS-001.mp4`，成功后会生成或更新：

- `D:\Media\SSIS-001.nfo`
- `D:\Media\SSIS-001-poster.jpg`
- `D:\Media\SSIS-001-fanart.jpg`
- `D:\Media\extrafanart\fanart1.jpg` 等，取决于配置
- `D:\Media\.actors\演员名.jpg`，取决于配置

如果该视频已经扫描入库，JavDB 短评还会写入本地弹幕表。播放该媒体时，播放器会自动加载这些短评并以弹幕形式展示；播放器右下角的气泡按钮可以开关短评弹幕。

## 注意

- 扩展只从当前页面读取公开可见的字段，不会自动绕过验证码。
- 写文件动作由本地 Nowen Video 后端完成，所以视频路径必须是运行服务的机器上可访问的本地路径。
- 如果图片站点启用了防盗链，后端会按数据源自动设置 Referer；仍失败时可以在管理页配置站点 Cookie 或使用已有服务端聚合刮削。
