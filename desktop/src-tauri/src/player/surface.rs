//! Desktop Player Surface 边界。
//!
//! PlayerManager 只依赖 `PlayerSurface`，不再直接接触 HWND/NSWindow/WID。
//! Windows 使用纯 Win32 + OpenGL Surface 承载 libmpv Render API；
//! macOS/Linux 暂时保留兼容窗口后端，后续只替换平台 Surface 实现。

#[cfg(target_os = "windows")]
mod windows;

#[cfg(target_os = "windows")]
pub use windows::{destroy, ensure, resync, sync_bounds, PlayerSurface};

#[cfg(not(target_os = "windows"))]
mod fallback {
    use anyhow::{anyhow, Context, Result};
    use tauri::{AppHandle, Manager, WebviewUrl, WebviewWindowBuilder};

    pub const PLAYER_SURFACE_LABEL: &str = "player-surface";

    #[derive(Debug, Clone, Copy)]
    pub struct PlayerSurface {
        native_window_id: i64,
    }

    impl PlayerSurface {
        pub(crate) fn native_window_id(self) -> i64 {
            self.native_window_id
        }
    }

    pub fn ensure(app: &AppHandle) -> Result<PlayerSurface> {
        if let Some(window) = app.get_webview_window(PLAYER_SURFACE_LABEL) {
            return Ok(PlayerSurface {
                native_window_id: native_window_handle(&window)?,
            });
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
        .context("创建 Desktop Player Surface 失败")?;

        Ok(PlayerSurface {
            native_window_id: native_window_handle(&window)?,
        })
    }

    fn native_window_handle(window: &tauri::WebviewWindow) -> Result<i64> {
        #[cfg(target_os = "macos")]
        {
            let ns_window = window.ns_window().context("获取 Player Surface NSWindow 失败")?;
            Ok(ns_window as isize as i64)
        }

        #[cfg(target_os = "linux")]
        {
            let _ = window.gtk_window().context("获取 Player Surface GtkWindow 失败")?;
            Err(anyhow!("Linux Player Surface 尚未接入 XID/Render API"))
        }

        #[cfg(not(any(target_os = "macos", target_os = "linux")))]
        {
            let _ = window;
            Err(anyhow!("当前平台暂不支持 Desktop Player Surface"))
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
            .ok_or_else(|| anyhow!("Desktop Player Surface 尚未创建"))?;

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

    pub fn resync(_app: &AppHandle) -> Result<()> {
        Ok(())
    }

    pub fn destroy(app: &AppHandle) -> Result<()> {
        if let Some(surface) = app.get_webview_window(PLAYER_SURFACE_LABEL) {
            surface.close().ok();
        }
        Ok(())
    }
}

#[cfg(not(target_os = "windows"))]
pub use fallback::{destroy, ensure, resync, sync_bounds, PlayerSurface};
