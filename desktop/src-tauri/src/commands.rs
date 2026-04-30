
//! Tauri 命令注册 —— 前端通过 invoke 调用

use crate::embed_window;
use crate::mpv::PlayOptions;
use crate::settings::Settings;
use crate::sidecar::SidecarStatus;
use crate::strategy::{self, EngineDecision, MediaProfile};
use crate::updater::{self, UpdateInfo};
use crate::AppState;
use serde::Serialize;
use tauri::{AppHandle, Manager, State};

// ============ sidecar 相关 ============

#[tauri::command]
pub fn sidecar_status(state: State<AppState>) -> Result<SidecarStatus, String> {
    let mut mgr = state.sidecar.lock().map_err(|e| e.to_string())?;
    Ok(mgr.status())
}

#[tauri::command]
pub async fn sidecar_restart(app: AppHandle, state: State<'_, AppState>) -> Result<(), String> {
    {
        let mut mgr = state.sidecar.lock().map_err(|e| e.to_string())?;
        mgr.stop();
    } // MutexGuard 在这里释放，不跨 await
    tokio::time::sleep(std::time::Duration::from_millis(500)).await;
    let mut mgr = state.sidecar.lock().map_err(|e| e.to_string())?;
    mgr.start(&app).map_err(|e| e.to_string())
}

// ============ 播放器相关 ============

#[tauri::command]
pub fn play_with_mpv(
    state: State<AppState>,
    session_id: String,
    url: String,
    options: Option<PlayOptions>,
) -> Result<(), String> {
    let opts = options.unwrap_or_default();
    let mut mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.play(&session_id, &url, opts).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn stop_mpv(state: State<AppState>, session_id: String) -> Result<(), String> {
    let mut mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.stop(&session_id);
    Ok(())
}

#[tauri::command]
pub fn mpv_available(state: State<AppState>) -> Result<MpvAvailability, String> {
    let mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    Ok(MpvAvailability {
        available: mpv.is_available(),
        embed_available: mpv.is_embed_available(),
    })
}

#[derive(Serialize)]
pub struct MpvAvailability {
    pub available: bool,
    pub embed_available: bool,
}

#[tauri::command]
pub fn decide_engine(
    state: State<AppState>,
    profile: MediaProfile,
) -> Result<EngineDecision, String> {
    let settings = state.settings.lock().map_err(|e| e.to_string())?;
    Ok(strategy::decide(&profile, &settings))
}

// ============ M4: libmpv 嵌入播放 ============

#[tauri::command]
pub fn mpv_embed_start(
    app: AppHandle,
    state: State<AppState>,
    session_id: String,
    url: String,
    options: Option<PlayOptions>,
) -> Result<EmbedStartResult, String> {
    let opts = options.unwrap_or_default();

    // 1. 确保嵌入窗口存在，拿到 wid
    let wid = embed_window::ensure_embed_window(&app).map_err(|e| e.to_string())?;

    // 2. 调 libmpv 播放
    let mut mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.play_embedded(&session_id, &url, wid, opts)
        .map_err(|e| e.to_string())?;

    Ok(EmbedStartResult { wid, session_id })
}

#[derive(Serialize)]
pub struct EmbedStartResult {
    pub wid: i64,
    pub session_id: String,
}

#[tauri::command]
pub fn mpv_embed_sync(
    app: AppHandle,
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
) -> Result<(), String> {
    embed_window::sync_embed_bounds(&app, x, y, width, height, visible)
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub fn mpv_embed_command(
    state: State<AppState>,
    session_id: String,
    command: String,
    args: Option<Vec<String>>,
) -> Result<(), String> {
    let mut mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.send_embed_command(&session_id, &command, &args.unwrap_or_default())
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub fn mpv_embed_set_property(
    state: State<AppState>,
    session_id: String,
    name: String,
    value: String,
) -> Result<(), String> {
    let mut mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.set_embed_property(&session_id, &name, &value)
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub fn mpv_embed_destroy(app: AppHandle) -> Result<(), String> {
    embed_window::destroy_embed_window(&app).map_err(|e| e.to_string())
}

// ============ M4 扩展: Anime4K & 视频信息 ============

#[tauri::command]
pub fn mpv_embed_set_anime4k(
    state: State<AppState>,
    session_id: String,
    level: String,
) -> Result<(), String> {
    let mut mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.set_embed_anime4k(&session_id, &level)
        .map_err(|e| e.to_string())
}

#[tauri::command]
pub fn mpv_embed_video_info(
    state: State<AppState>,
    session_id: String,
) -> Result<crate::mpv::EmbedVideoInfo, String> {
    let mpv = state.mpv.lock().map_err(|e| e.to_string())?;
    mpv.get_embed_video_info(&session_id)
        .map_err(|e| e.to_string())
}

// ============ M5: 自动更新 ============

#[tauri::command]
pub async fn check_update(app: AppHandle) -> Result<UpdateInfo, String> {
    updater::check(&app).await.map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn install_update(app: AppHandle) -> Result<(), String> {
    updater::download_and_install(&app)
        .await
        .map_err(|e| e.to_string())
}

// ============ 设置相关 ============

#[tauri::command]
pub fn get_settings(state: State<AppState>) -> Result<Settings, String> {
    let s = state.settings.lock().map_err(|e| e.to_string())?;
    Ok(s.clone())
}

#[tauri::command]
pub fn save_settings(state: State<AppState>, new_settings: Settings) -> Result<(), String> {
    new_settings.save().map_err(|e| e.to_string())?;
    let mut s = state.settings.lock().map_err(|e| e.to_string())?;
    *s = new_settings;
    Ok(())
}

// ============ 系统相关 ============

/// 内部调用版本 —— 不走 Tauri 命令路径
pub fn open_url_internal(url: &str) -> Result<(), String> {
    #[cfg(target_os = "windows")]
    {
        std::process::Command::new("cmd")
            .args(["/c", "start", "", url])
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    #[cfg(target_os = "macos")]
    {
        std::process::Command::new("open")
            .arg(url)
            .spawn()
            .map_err(|e| e.to_string())?;
    }
    #[cfg(target_os = "linux")]
    {
        std::process::Command::new("xdg-open")
            .arg(url)
            .spawn()
            .map_err(|e| e.to_string())?;
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

    let (tx, rx) = tokio::sync::oneshot::channel::<Option<String>>();
    app.dialog().file().pick_file(move |path| {
        let s = path.map(|p| p.to_string());
        let _ = tx.send(s);
    });

    rx.await.map_err(|e| e.to_string())
}

#[tauri::command]
pub async fn pick_folder(app: AppHandle) -> Result<Option<String>, String> {
    use tauri_plugin_dialog::DialogExt;

    let (tx, rx) = tokio::sync::oneshot::channel::<Option<String>>();
    app.dialog().file().pick_folder(move |path| {
        let s = path.map(|p| p.to_string());
        let _ = tx.send(s);
    });

    rx.await.map_err(|e| e.to_string())
}

// ============ M6: 窗口管理 ============

#[tauri::command]
pub fn window_minimize(app: AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window("main") {
        win.minimize().map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[tauri::command]
pub fn window_toggle_fullscreen(app: AppHandle) -> Result<bool, String> {
    if let Some(win) = app.get_webview_window("main") {
        let is_full = win.is_fullscreen().map_err(|e| e.to_string())?;
        win.set_fullscreen(!is_full).map_err(|e| e.to_string())?;
        return Ok(!is_full);
    }
    Ok(false)
}

#[tauri::command]
pub fn window_hide_to_tray(app: AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window("main") {
        win.hide().map_err(|e| e.to_string())?;
    }
    Ok(())
}

// ============ M1 (Hills 化): 自绘标题栏的窗口控制 ============

#[tauri::command]
pub fn window_toggle_maximize(app: AppHandle) -> Result<bool, String> {
    if let Some(win) = app.get_webview_window("main") {
        let is_max = win.is_maximized().map_err(|e| e.to_string())?;
        if is_max {
            win.unmaximize().map_err(|e| e.to_string())?;
        } else {
            win.maximize().map_err(|e| e.to_string())?;
        }
        return Ok(!is_max);
    }
    Ok(false)
}

#[tauri::command]
pub fn window_is_maximized(app: AppHandle) -> Result<bool, String> {
    if let Some(win) = app.get_webview_window("main") {
        return win.is_maximized().map_err(|e| e.to_string());
    }
    Ok(false)
}

#[tauri::command]
pub fn window_close(app: AppHandle) -> Result<(), String> {
    if let Some(win) = app.get_webview_window("main") {
        // 走 CloseRequested 流程（会触发 minimize_to_tray 逻辑）
        let _ = win.close();
    }
    Ok(())
}

/// 应用/清除窗口特效（前端在播放沉浸切换时调用）
#[tauri::command]
pub fn window_set_effect(app: AppHandle, enabled: bool) -> Result<(), String> {
    if let Some(win) = app.get_webview_window("main") {
        if enabled {
            crate::vibrancy::apply_effect(&win);
        } else {
            crate::vibrancy::clear_effect(&win);
        }
    }
    Ok(())
}
