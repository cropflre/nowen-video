
//! 文件关联（M6）+ Deep Link 处理
//!
//! 流程：
//! 1. 系统把双击/右键打开的文件路径作为参数传给应用
//! 2. Tauri 启动后我们解析 argv，取出视频文件路径
//! 3. 通过事件通知前端自动打开播放器
//!
//! 支持两种方式：
//! - **命令行参数**：Windows 注册表 / macOS Info.plist / Linux .desktop 关联
//! - **Deep Link 协议**：nowen-video:// URL（跨平台统一）

use std::path::PathBuf;
use tauri::{AppHandle, Emitter, Manager};

/// 支持打开的媒体扩展名
pub const SUPPORTED_EXTS: &[&str] = &[
    "mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "ts", "m2ts", "mts",
    "rm", "rmvb", "asf", "m4v", "3gp", "f4v", "vob", "iso",
];

/// 从命令行参数提取媒体文件路径
pub fn extract_media_paths(args: &[String]) -> Vec<PathBuf> {
    args.iter()
        .skip(1) // 跳过程序自身
        .filter_map(|arg| {
            let p = PathBuf::from(arg);
            if !p.exists() || !p.is_file() {
                return None;
            }
            let ext = p
                .extension()
                .and_then(|s| s.to_str())
                .map(|s| s.to_lowercase())
                .unwrap_or_default();
            if SUPPORTED_EXTS.contains(&ext.as_str()) {
                Some(p)
            } else {
                None
            }
        })
        .collect()
}

/// 处理启动时命令行参数 —— 若带文件则通知前端播放
pub fn handle_startup_args(app: &AppHandle) {
    let args: Vec<String> = std::env::args().collect();
    let paths = extract_media_paths(&args);

    if paths.is_empty() {
        log::debug!("无媒体文件参数");
        return;
    }

    log::info!("启动携带 {} 个媒体文件", paths.len());

    // 主窗口就绪后再通知；这里简单延时
    let handle = app.clone();
    tauri::async_runtime::spawn(async move {
        tokio::time::sleep(std::time::Duration::from_secs(2)).await;
        if let Some(win) = handle.get_webview_window("main") {
            let paths_str: Vec<String> = paths.iter().map(|p| p.to_string_lossy().to_string()).collect();
            let _ = win.emit("open-files", paths_str);
        }
    });
}

/// 处理单实例 —— 第二次启动时把参数转发给已运行实例
pub fn handle_single_instance(app: &AppHandle, argv: Vec<String>, _cwd: String) {
    log::info!("收到第二实例调用: {:?}", argv);

    // 激活主窗口
    if let Some(win) = app.get_webview_window("main") {
        let _ = win.show();
        let _ = win.set_focus();
        let _ = win.unminimize();
    }

    let paths = extract_media_paths(&argv);
    if !paths.is_empty() {
        if let Some(win) = app.get_webview_window("main") {
            let paths_str: Vec<String> = paths.iter().map(|p| p.to_string_lossy().to_string()).collect();
            let _ = win.emit("open-files", paths_str);
        }
    }
}

/// 处理 Deep Link —— nowen-video://play?url=...
pub fn handle_deep_link(app: &AppHandle, urls: Vec<url::Url>) {
    for url in urls {
        log::info!("收到 deep link: {}", url);
        if let Some(win) = app.get_webview_window("main") {
            let _ = win.emit("deep-link", url.to_string());
            let _ = win.show();
            let _ = win.set_focus();
        }
    }
}
