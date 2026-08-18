// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod app;
mod commands;
mod embed_window;
mod file_assoc;
mod mpv;
mod resources;
mod runtime;
mod settings;
mod sidecar;
mod strategy;
mod tray;
mod updater;
mod vibrancy;

pub use runtime::AppState;

use tauri::Manager;
use tauri_plugin_global_shortcut::{Code, Modifiers, Shortcut, ShortcutState};

fn main() {
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    log::info!("========================================");
    log::info!(" Nowen Video Desktop 2.0 runtime {}", env!("CARGO_PKG_VERSION"));
    log::info!("========================================");

    app::ensure_libmpv_runtime();

    let settings = settings::Settings::load().unwrap_or_else(|error| {
        log::warn!("加载应用设置失败，使用默认值: {}", error);
        settings::Settings::default()
    });

    let state = AppState::new(settings);

    tauri::Builder::default()
        .plugin(tauri_plugin_single_instance::init(|app, argv, cwd| {
            file_assoc::handle_single_instance(app, argv, cwd);
        }))
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_http::init())
        .plugin(tauri_plugin_os::init())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_deep_link::init())
        .plugin(
            tauri_plugin_global_shortcut::Builder::new()
                .with_shortcuts([Shortcut::new(
                    Some(Modifiers::CONTROL | Modifiers::SHIFT),
                    Code::KeyN,
                )])
                .expect("注册 Desktop 全局快捷键失败")
                .with_handler(|app, shortcut, event| {
                    if event.state != ShortcutState::Pressed {
                        return;
                    }
                    log::info!("全局快捷键触发: {:?}", shortcut);
                    if let Some(window) = app.get_webview_window("main") {
                        let _ = window.show();
                        let _ = window.set_focus();
                        let _ = window.unminimize();
                    }
                })
                .build(),
        )
        .manage(state)
        .on_menu_event(|app, event| {
            tray::handle_menu_event(app, event.id().as_ref());
        })
        .setup(app::setup)
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                let app = window.app_handle();

                if window.label() == "main" {
                    let state: tauri::State<AppState> = app.state();
                    let minimize_to_tray = state
                        .settings
                        .lock()
                        .map(|settings| settings.window.minimize_to_tray)
                        .unwrap_or(false);

                    if minimize_to_tray {
                        log::info!("主窗口关闭请求转为隐藏到托盘");
                        let _ = window.hide();
                        api.prevent_close();
                        return;
                    }
                }

                log::info!("窗口 {} 关闭，释放 Desktop 运行时资源", window.label());
                let state: tauri::State<AppState> = app.state();

                if let Ok(mut mpv) = state.mpv.lock() {
                    mpv.stop_all();
                }

                if window.label() == "main" {
                    if let Ok(mut sidecar) = state.sidecar.lock() {
                        sidecar.stop();
                    }
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            commands::sidecar_status,
            commands::sidecar_restart,
            commands::play_with_mpv,
            commands::stop_mpv,
            commands::mpv_available,
            commands::decide_engine,
            commands::mpv_embed_start,
            commands::mpv_embed_sync,
            commands::mpv_embed_command,
            commands::mpv_embed_set_property,
            commands::mpv_embed_destroy,
            commands::mpv_embed_set_anime4k,
            commands::mpv_embed_video_info,
            commands::check_update,
            commands::install_update,
            commands::get_settings,
            commands::save_settings,
            commands::open_url,
            commands::platform_info,
            commands::pick_file,
            commands::pick_folder,
            commands::window_minimize,
            commands::window_toggle_fullscreen,
            commands::window_hide_to_tray,
            commands::window_toggle_maximize,
            commands::window_is_maximized,
            commands::window_close,
            commands::window_set_effect,
            commands::window_pip_enter,
            commands::window_pip_exit,
            commands::window_pip_is_active,
            commands::window_set_always_on_top,
        ])
        .run(tauri::generate_context!())
        .expect("Tauri 应用启动失败");
}
