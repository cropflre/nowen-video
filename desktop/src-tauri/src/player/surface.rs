//! 原生播放器渲染 Surface 管理。
//!
//! 当前 Windows 首发实现使用独立无边框原生窗口承载 libmpv，并由前端同步布局。
//! 该边界被严格封装在 player/surface，后续切换 libmpv Render API 时不会影响
//! PlayerManager、IPC 或 React 播放器控制层。

use anyhow::{anyhow, Context, Result};
use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindowBuilder};

pub const PLAYER_SURFACE_LABEL: &str = "mpv-embed";

pub fn ensure(app: &AppHandle) -> Result<i64> {
    if let Some(window) = app.get_webview_window(PLAYER_SURFACE_LABEL) {
        return native_window_handle(&window);
    }

    let window = WebviewWindowBuilder::new(
        app,
        PLAYER_SURFACE_LABEL,
        WebviewUrl::External("about:blank".parse().expect("about:blank URL 无效")),
    )
    .title("Nowen Video Player Surface")
    .inner_size(800.0, 450.0)
    .decorations(false)
    .resizable(false)
    .always_on_top(false)
    .skip_taskbar(true)
    .visible(false)
    .transparent(false)
    .build()
    .context("创建原生播放器 Surface 失败")?;

    native_window_handle(&window)
}

fn native_window_handle(window: &tauri::WebviewWindow) -> Result<i64> {
    #[cfg(target_os = "windows")]
    {
        let hwnd = window.hwnd().context("获取 Player Surface HWND 失败")?;
        Ok(hwnd.0 as isize as i64)
    }

    #[cfg(target_os = "macos")]
    {
        let ns_window = window.ns_window().context("获取 Player Surface NSWindow 失败")?;
        Ok(ns_window as isize as i64)
    }

    #[cfg(target_os = "linux")]
    {
        let _ = window.gtk_window().context("获取 Player Surface GtkWindow 失败")?;
        Err(anyhow!("Linux 原生播放器 Surface 尚未接入 XID/Render API"))
    }

    #[cfg(not(any(target_os = "windows", target_os = "macos", target_os = "linux")))]
    {
        let _ = window;
        Err(anyhow!("当前平台暂不支持原生播放器 Surface"))
    }
}

pub fn sync_bounds(
    app: &AppHandle,
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
) -> Result<()> {
    let surface = app
        .get_webview_window(PLAYER_SURFACE_LABEL)
        .ok_or_else(|| anyhow!("原生播放器 Surface 尚未创建"))?;

    if let Some(main) = app.get_webview_window("main") {
        if let Ok(main_position) = main.outer_position() {
            surface
                .set_position(tauri::PhysicalPosition::new(
                    main_position.x + x,
                    main_position.y + y,
                ))
                .ok();
        }
    }

    surface
        .set_size(tauri::PhysicalSize::new(width.max(1), height.max(1)))
        .ok();

    if visible {
        let _ = surface.show();
    } else {
        let _ = surface.hide();
    }
    Ok(())
}

pub fn destroy(app: &AppHandle) -> Result<()> {
    if let Some(surface) = app.get_webview_window(PLAYER_SURFACE_LABEL) {
        surface.close().ok();
    }
    Ok(())
}
