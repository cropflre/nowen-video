//! Desktop Player Surface 边界。

#[cfg(target_os = "windows")]
mod windows;

#[cfg(target_os = "windows")]
pub use windows::{destroy, ensure, resync, sync_bounds, PlayerSurface};

#[cfg(target_os = "macos")]
mod macos;

#[cfg(target_os = "macos")]
pub use macos::{destroy, ensure, resync, sync_bounds, PlayerSurface};

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
mod native_window {
    use anyhow::Result;
    use tauri::AppHandle;

    #[derive(Debug, Clone, Copy, Default)]
    pub struct PlayerSurface;

    pub fn ensure(_app: &AppHandle) -> Result<PlayerSurface> {
        Ok(PlayerSurface)
    }

    pub fn sync_bounds(_app: &AppHandle, _x: i32, _y: i32, _width: u32, _height: u32, _visible: bool) -> Result<()> {
        Ok(())
    }

    pub fn resync(_app: &AppHandle) -> Result<()> {
        Ok(())
    }

    pub fn destroy(_app: &AppHandle) -> Result<()> {
        Ok(())
    }
}

#[cfg(not(any(target_os = "windows", target_os = "macos")))]
pub use native_window::{destroy, ensure, resync, sync_bounds, PlayerSurface};
