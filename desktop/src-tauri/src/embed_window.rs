
//! 嵌入式 mpv 窗口管理（M4）
//!
//! 策略：创建一个无边框的 Tauri 子窗口作为 mpv 的渲染容器，
//! 前端通过 JS API 同步它的位置/大小与播放器占位元素对齐。
//!
//! 运行时两条路径：
//! 1. `embed-mpv` feature 启用 + libmpv 可用 → 真正 FFI 嵌入渲染
//! 2. 否则 → 回退为外部进程（传统模式），子窗口作为占位（不显示）

use anyhow::{anyhow, Context, Result};
use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindowBuilder};

pub const EMBED_WINDOW_LABEL: &str = "mpv-embed";

/// 确保 mpv 嵌入窗口存在（不存在则创建）
pub fn ensure_embed_window(app: &AppHandle) -> Result<i64> {
    if let Some(win) = app.get_webview_window(EMBED_WINDOW_LABEL) {
        return native_window_handle(&win);
    }

    let win = WebviewWindowBuilder::new(
        app,
        EMBED_WINDOW_LABEL,
        WebviewUrl::External("about:blank".parse().unwrap()),
    )
    .title("mpv-embed")
    .inner_size(800.0, 450.0)
    .decorations(false)
    .resizable(false)
    .always_on_top(false)
    .skip_taskbar(true)
    .visible(false) // 初始隐藏，前端调用时显示
    .transparent(false)
    .build()
    .context("创建 mpv 嵌入窗口失败")?;

    native_window_handle(&win)
}

/// 获取窗口的原生 handle（作为 mpv 的 wid 属性值）
fn native_window_handle(window: &tauri::WebviewWindow) -> Result<i64> {
    #[cfg(target_os = "windows")]
    {
        let hwnd = window.hwnd().context("获取 HWND 失败")?;
        // Tauri 返回 windows::HWND；内部存 *mut c_void
        Ok(hwnd.0 as isize as i64)
    }

    #[cfg(target_os = "macos")]
    {
        let ns_window = window.ns_window().context("获取 NSWindow 失败")?;
        Ok(ns_window as isize as i64)
    }

    #[cfg(target_os = "linux")]
    {
        let gtk_window = window.gtk_window().context("获取 GtkWindow 失败")?;
        // GTK window 的 XID
        // 实际使用时需要 gdk-x11 获取 xid；这里返回占位
        let _ = gtk_window;
        Err(anyhow!("Linux 嵌入模式暂未实现 XID 获取"))
    }

    #[cfg(not(any(target_os = "windows", target_os = "macos", target_os = "linux")))]
    {
        let _ = window;
        Err(anyhow!("当前平台不支持嵌入 mpv"))
    }
}

/// 同步嵌入窗口位置 —— 由前端传入布局矩形
pub fn sync_embed_bounds(
    app: &AppHandle,
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
) -> Result<()> {
    let win = app
        .get_webview_window(EMBED_WINDOW_LABEL)
        .ok_or_else(|| anyhow!("嵌入窗口未创建"))?;

    // 注意：x/y 是相对主窗口的 client 坐标，需要加上主窗口屏幕坐标
    if let Some(main) = app.get_webview_window("main") {
        if let Ok(main_pos) = main.outer_position() {
            win.set_position(tauri::PhysicalPosition::new(
                main_pos.x + x,
                main_pos.y + y,
            ))
            .ok();
        }
    }
    win.set_size(tauri::PhysicalSize::new(width, height)).ok();

    if visible {
        let _ = win.show();
    } else {
        let _ = win.hide();
    }
    Ok(())
}

/// 销毁嵌入窗口
pub fn destroy_embed_window(app: &AppHandle) -> Result<()> {
    if let Some(win) = app.get_webview_window(EMBED_WINDOW_LABEL) {
        win.close().ok();
    }
    Ok(())
}
