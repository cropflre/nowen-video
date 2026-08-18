//! Desktop 2.0 Tauri IPC。
//!
//! 对前端只暴露产品能力命名，不再暴露外部 mpv、播放内核决策等实验接口。

use crate::player::{self, PlayOptions, PlayerVideoInfo};
use crate::settings::Settings;
use crate::sidecar::SidecarStatus;
use crate::updater::{self, UpdateInfo};
use crate::AppState;
use serde::Serialize;
use tauri::{AppHandle, Manager, State};

#[tauri::command]
pub fn sidecar_status(state: State<AppState>) -> Result<SidecarStatus, String> {
    let mut manager = state.sidecar.lock().map_err(|error| error.to_string())?;
    Ok(manager.status())
}

#[tauri::command]
pub async fn sidecar_restart(app: AppHandle, state: State<'_, AppState>) -> Result<(), String> {
    {
        let mut manager = state.sidecar.lock().map_err(|error| error.to_string())?;
        manager.stop();
    }
    tokio::time::sleep(std::time::Duration::from_millis(500)).await;
    let mut manager = state.sidecar.lock().map_err(|error| error.to_string())?;
    manager.start(&app).map_err(|error| error.to_string())
}

// ============ Desktop Player Core ============

#[tauri::command]
pub fn player_available(state: State<AppState>) -> Result<bool, String> {
    let player = state.player.lock().map_err(|error| error.to_string())?;
    Ok(player.is_available())
}

#[tauri::command]
pub fn player_start(
    app: AppHandle,
    state: State<AppState>,
    session_id: String,
    url: String,
    options: Option<PlayOptions>,
) -> Result<PlayerStartResult, String> {
    let native_window_id = player::surface::ensure(&app).map_err(|error| error.to_string())?;
    let mut player = state.player.lock().map_err(|error| error.to_string())?;
    player
        .start(
            &session_id,
            &url,
            native_window_id,
            options.unwrap_or_default(),
        )
        .map_err(|error| error.to_string())?;

    Ok(PlayerStartResult {
        wid: native_window_id,
        session_id,
    })
}

#[derive(Serialize)]
pub struct PlayerStartResult {
    pub wid: i64,
    pub session_id: String,
}

#[tauri::command]
pub fn player_stop(state: State<AppState>, session_id: String) -> Result<(), String> {
    let mut player = state.player.lock().map_err(|error| error.to_string())?;
    player.stop(&session_id);
    Ok(())
}

#[tauri::command]
pub fn player_sync_surface(
    app: AppHandle,
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
) -> Result<(), String> {
    player::surface::sync_bounds(&app, x, y, width, height, visible)
        .map_err(|error| error.to_string())
}

#[tauri::command]
pub fn player_command(
    state: State<AppState>,
    session_id: String,
    command: String,
    args: Option<Vec<String>>,
) -> Result<(), String> {
    let player = state.player.lock().map_err(|error| error.to_string())?;
    player
        .command(&session_id, &command, &args.unwrap_or_default())
        .map_err(|error| error.to_string())
}

#[tauri::command]
pub fn player_set_property(
    state: State<AppState>,
    session_id: String,
    name: String,
    value: String,
) -> Result<(), String> {
    let player = state.player.lock().map_err(|error| error.to_string())?;
    player
        .set_property(&session_id, &name, &value)
        .map_err(|error| error.to_string())
}

#[tauri::command]
pub fn player_destroy(app: AppHandle) -> Result<(), String> {
    player::surface::destroy(&app).map_err(|error| error.to_string())
}

#[tauri::command]
pub fn player_video_info(
    state: State<AppState>,
    session_id: String,
) -> Result<PlayerVideoInfo, String> {
    let player = state.player.lock().map_err(|error| error.to_string())?;
    player
        .video_info(&session_id)
        .map_err(|error| error.to_string())
}

// ============ 更新 ============

#[tauri::command]
pub async fn check_update(app: AppHandle) -> Result<UpdateInfo, String> {
    updater::check(&app).await.map_err(|error| error.to_string())
}

#[tauri::command]
pub async fn install_update(app: AppHandle) -> Result<(), String> {
    updater::download_and_install(&app)
        .await
        .map_err(|error| error.to_string())
}

// ============ 设置 ============

#[tauri::command]
pub fn get_settings(state: State<AppState>) -> Result<Settings, String> {
    let settings = state.settings.lock().map_err(|error| error.to_string())?;
    Ok(settings.clone())
}

#[tauri::command]
pub fn save_settings(state: State<AppState>, new_settings: Settings) -> Result<(), String> {
    new_settings.save().map_err(|error| error.to_string())?;
    let mut settings = state.settings.lock().map_err(|error| error.to_string())?;
    *settings = new_settings;
    Ok(())
}

// ============ 系统 ============

pub fn open_url_internal(url: &str) -> Result<(), String> {
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new("cmd")
            .args(["/c", "start", "", url])
            .spawn()
            .map_err(|error| error.to_string())?;
    }
    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .arg(url)
            .spawn()
            .map_err(|error| error.to_string())?;
    }
    #[cfg(target_os = "linux")]
    {
        std::process::Command::new("xdg-open")
            .arg(url)
            .spawn()
            .map_err(|error| error.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub fn open_url(url: String) -> Result<(), String> {
    open_url_internal(&url)
}

#[tauri::command]
pub fn platform_info() -> PlatformInfo {
    PlatformInfo {
        os: std::env::consts::OS.to_string(),
        arch: std::env::consts::ARCH.to_string(),
        family: std::env::consts::FAMILY.to_string(),
        is_desktop: true,
    }
}

#[derive(Serialize)]
pub struct PlatformInfo {
    pub os: String,
    pub arch: String,
    pub family: String,
    pub is_desktop: bool,
}

#[tauri::command]
pub async fn pick_file(app: AppHandle) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;

    let (sender, receiver) = tokio::sync::oneshot::channel::<Option<String>>();
    app.dialog().file().pick_file(move |path| {
        let value = path.map(|path| path.to_string());
        let _ = sender.send(value);
    });

    receiver.await.map_err(|error| error.to_string())
}

#[tauri::command]
pub async fn pick_folder(app: AppHandle) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;

    let (sender, receiver) = tokio::sync::oneshot::channel::<Option<String>>();
    app.dialog().file().pick_folder(move |path| {
        let value = path.map(|path| path.to_string());
        let _ = sender.send(value);
    });

    receiver.await.map_err(|error| error.to_string())
}

// ============ 窗口 ============

#[tauri::command]
pub fn window_minimize(app: AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        window.minimize().map_err(|error| error.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub fn window_toggle_fullscreen(app: AppHandle) -> Result<bool, String> {
    if let Some(window) = app.get_webview_window("main") {
        let fullscreen = window.is_fullscreen().map_err(|error| error.to_string())?;
        window
            .set_fullscreen(!fullscreen)
            .map_err(|error| error.to_string())?;
        return Ok(!fullscreen);
    }
    Ok(false)
}

#[tauri::command]
pub fn window_hide_to_tray(app: AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        window.hide().map_err(|error| error.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub fn window_toggle_maximize(app: AppHandle) -> Result<bool, String> {
    if let Some(window) = app.get_webview_window("main") {
        let maximized = window.is_maximized().map_err(|error| error.to_string())?;
        if maximized {
            window.unmaximize().map_err(|error| error.to_string())?;
        } else {
            window.maximize().map_err(|error| error.to_string())?;
        }
        return Ok(!maximized);
    }
    Ok(false)
}

#[tauri::command]
pub fn window_is_maximized(app: AppHandle) -> Result<bool, String> {
    if let Some(window) = app.get_webview_window("main") {
        return window.is_maximized().map_err(|error| error.to_string());
    }
    Ok(false)
}

#[tauri::command]
pub fn window_close(app: AppHandle) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        let _ = window.close();
    }
    Ok(())
}

#[tauri::command]
pub fn window_set_effect(app: AppHandle, enabled: bool) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        if enabled {
            crate::vibrancy::apply_effect(&window);
        } else {
            crate::vibrancy::clear_effect(&window);
        }
    }
    Ok(())
}

static PIP_STATE: std::sync::Mutex<Option<PipState>> = std::sync::Mutex::new(None);

#[derive(Clone, Copy, Debug)]
struct PipState {
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    maximized: bool,
}

#[tauri::command]
pub fn window_pip_enter(app: AppHandle) -> Result<(), String> {
    use tauri::{LogicalPosition, LogicalSize};

    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "main window not found".to_string())?;

    let position = window.outer_position().map_err(|error| error.to_string())?;
    let size = window.outer_size().map_err(|error| error.to_string())?;
    let scale = window.scale_factor().map_err(|error| error.to_string())?;
    let maximized = window.is_maximized().map_err(|error| error.to_string())?;

    let mut state = PIP_STATE.lock().map_err(|error| error.to_string())?;
    if state.is_none() {
        *state = Some(PipState {
            x: (position.x as f64 / scale) as i32,
            y: (position.y as f64 / scale) as i32,
            width: (size.width as f64 / scale) as u32,
            height: (size.height as f64 / scale) as u32,
            maximized,
        });
    }
    drop(state);

    if maximized {
        let _ = window.unmaximize();
    }

    let monitor = window.current_monitor().map_err(|error| error.to_string())?;
    let (screen_width, screen_height) = monitor
        .map(|monitor| {
            let size = monitor.size();
            let scale = monitor.scale_factor();
            (
                (size.width as f64 / scale) as u32,
                (size.height as f64 / scale) as u32,
            )
        })
        .unwrap_or((1920, 1080));

    let width = 480_u32;
    let height = 270_u32;
    let margin = 24_i32;
    let x = screen_width as i32 - width as i32 - margin;
    let y = screen_height as i32 - height as i32 - margin;

    let _ = window.set_always_on_top(true);
    let _ = window.set_size(LogicalSize::new(width, height));
    let _ = window.set_position(LogicalPosition::new(x, y));
    let _ = window.set_resizable(true);
    Ok(())
}

#[tauri::command]
pub fn window_pip_exit(app: AppHandle) -> Result<(), String> {
    use tauri::{LogicalPosition, LogicalSize};

    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "main window not found".to_string())?;

    let mut guard = PIP_STATE.lock().map_err(|error| error.to_string())?;
    let state = guard.take();
    drop(guard);

    let _ = window.set_always_on_top(false);

    if let Some(state) = state {
        let _ = window.set_size(LogicalSize::new(state.width, state.height));
        let _ = window.set_position(LogicalPosition::new(state.x, state.y));
        if state.maximized {
            let _ = window.maximize();
        }
    }
    Ok(())
}

#[tauri::command]
pub fn window_pip_is_active() -> Result<bool, String> {
    let state = PIP_STATE.lock().map_err(|error| error.to_string())?;
    Ok(state.is_some())
}

#[tauri::command]
pub fn window_set_always_on_top(app: AppHandle, enabled: bool) -> Result<(), String> {
    if let Some(window) = app.get_webview_window("main") {
        window
            .set_always_on_top(enabled)
            .map_err(|error| error.to_string())?;
    }
    Ok(())
}
