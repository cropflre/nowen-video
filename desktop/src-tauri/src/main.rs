
// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod commands;
mod embed_window;
mod file_assoc;
mod mpv;
mod settings;
mod sidecar;
mod strategy;
mod tray;
mod updater;
mod vibrancy;

use std::sync::{Arc, Mutex};
use tauri::{Emitter, Manager};
use tauri_plugin_global_shortcut::{Code, GlobalShortcutExt, Modifiers, Shortcut, ShortcutState};

/// 全局应用状态
pub struct AppState {
    /// Go sidecar 进程管理
    pub sidecar: Arc<Mutex<sidecar::SidecarManager>>,
    /// mpv 播放器管理
    pub mpv: Arc<Mutex<mpv::MpvManager>>,
    /// 应用设置
    pub settings: Arc<Mutex<settings::Settings>>,
}

fn main() {
    // 初始化日志
    env_logger::Builder::from_env(env_logger::Env::default().default_filter_or("info")).init();

    log::info!("========================================");
    log::info!("   nowen-video Desktop v{}", env!("CARGO_PKG_VERSION"));
    log::info!("========================================");

    let settings = settings::Settings::load().unwrap_or_else(|e| {
        log::warn!("加载应用设置失败，使用默认值: {}", e);
        settings::Settings::default()
    });

    let state = AppState {
        sidecar: Arc::new(Mutex::new(sidecar::SidecarManager::new(settings.clone()))),
        mpv: Arc::new(Mutex::new(mpv::MpvManager::new(settings.clone()))),
        settings: Arc::new(Mutex::new(settings)),
    };

    let mut builder = tauri::Builder::default()
        // 单实例必须最先注册，拦截第二个进程
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
        // 全局快捷键：Ctrl/Cmd+Shift+N 显示主窗口
        .plugin(
            tauri_plugin_global_shortcut::Builder::new()
                .with_shortcuts([Shortcut::new(
                    Some(Modifiers::CONTROL | Modifiers::SHIFT),
                    Code::KeyN,
                )])
                .unwrap()
                .with_handler(|app, shortcut, event| {
                    if event.state == ShortcutState::Pressed {
                        log::info!("全局快捷键触发: {:?}", shortcut);
                        if let Some(win) = app.get_webview_window("main") {
                            let _ = win.show();
                            let _ = win.set_focus();
                            let _ = win.unminimize();
                        }
                    }
                })
                .build(),
        )
        .manage(state)
        // 菜单事件统一回调（App 级别，Tauri 2.0 推荐方式）
        .on_menu_event(|app, event| {
            tray::handle_menu_event(app, event.id().as_ref());
        });

    builder = builder
        .setup(|app| {
            let handle = app.handle().clone();

            // 主窗口视觉特效（Mica / Acrylic / Vibrancy）
            vibrancy::apply_main_window_effect(&handle);

            // 应用菜单（Windows / Linux 挂到窗口；macOS 挂到应用）
            match tray::build_app_menu(&handle) {
                Ok(menu) => {
                    #[cfg(not(target_os = "macos"))]
                    {
                        if let Some(main) = handle.get_webview_window("main") {
                            let _ = main.set_menu(menu);
                        }
                    }
                    #[cfg(target_os = "macos")]
                    {
                        let _ = app.set_menu(menu);
                    }
                }
                Err(e) => log::warn!("构建菜单失败: {}", e),
            }

            // 系统托盘
            if let Err(e) = tray::build_tray(&handle) {
                log::warn!("创建托盘失败: {}", e);
            }

            // Deep Link 监听（M6）
            {
                use tauri_plugin_deep_link::DeepLinkExt;
                let h3 = handle.clone();
                handle.deep_link().on_open_url(move |event| {
                    file_assoc::handle_deep_link(&h3, event.urls());
                });
                // 注册 deep link 协议（运行时注册，首次启动生效）
                #[cfg(any(target_os = "windows", target_os = "linux"))]
                {
                    if let Err(e) = handle.deep_link().register("nowen-video") {
                        log::warn!("注册 deep link 协议失败: {}", e);
                    }
                }
            }

            // 处理启动命令行文件参数（文件关联）
            file_assoc::handle_startup_args(&handle);

            // 异步启动 sidecar
            let h_side = handle.clone();
            tauri::async_runtime::spawn(async move {
                let state: tauri::State<AppState> = h_side.state();
                let settings = state.settings.lock().unwrap().clone();

                if settings.server.mode == settings::ServerMode::Embedded {
                    log::info!("启动内嵌 Go sidecar...");
                    let mut sidecar = state.sidecar.lock().unwrap();
                    if let Err(e) = sidecar.start(&h_side) {
                        log::error!("sidecar 启动失败: {}", e);
                    }
                } else {
                    log::info!("使用远程 server 模式: {}", settings.server.remote_url);
                }
            });

            // 启动后 3 秒静默检查更新（M5）
            let h_upd = handle.clone();
            tauri::async_runtime::spawn(async move {
                tokio::time::sleep(std::time::Duration::from_secs(3)).await;
                match updater::check(&h_upd).await {
                    Ok(info) if info.available => {
                        log::info!("检测到新版本: {}", info.version);
                        let _ = h_upd.emit("update-available", info);
                    }
                    Ok(_) => log::debug!("当前已是最新版本"),
                    Err(e) => log::debug!("检查更新失败（可忽略）: {}", e),
                }
            });

            Ok(())
        })
        .on_window_event(|window, event| {
            if let tauri::WindowEvent::CloseRequested { api, .. } = event {
                let app = window.app_handle();

                // 仅主窗口关闭时考虑隐藏到托盘
                if window.label() == "main" {
                    let state: tauri::State<AppState> = app.state();
                    let minimize_to_tray = state
                        .settings
                        .lock()
                        .map(|s| s.window.minimize_to_tray)
                        .unwrap_or(false);

                    if minimize_to_tray {
                        log::info!("主窗口关闭 —— 隐藏到托盘（设置启用）");
                        let _ = window.hide();
                        api.prevent_close();
                        return;
                    }
                }

                log::info!("窗口 {} 关闭，清理资源...", window.label());
                let state: tauri::State<AppState> = app.state();

                // 停止 mpv 进程
                if let Ok(mut mpv) = state.mpv.lock() {
                    mpv.stop_all();
                }

                // 停止 sidecar（仅主窗口关闭时）
                if window.label() == "main" {
                    if let Ok(mut sidecar) = state.sidecar.lock() {
                        sidecar.stop();
                    }
                }
            }
        })
        .invoke_handler(tauri::generate_handler![
            // sidecar
            commands::sidecar_status,
            commands::sidecar_restart,
            // 播放器（外部进程）
            commands::play_with_mpv,
            commands::stop_mpv,
            commands::mpv_available,
            commands::decide_engine,
            // 播放器（libmpv 嵌入 - M4）
            commands::mpv_embed_start,
            commands::mpv_embed_sync,
            commands::mpv_embed_command,
            commands::mpv_embed_set_property,
            commands::mpv_embed_destroy,
            // 自动更新（M5）
            commands::check_update,
            commands::install_update,
            // 设置
            commands::get_settings,
            commands::save_settings,
            // 系统
            commands::open_url,
            commands::platform_info,
            commands::pick_file,
            commands::pick_folder,
            // 窗口管理（M7）
            commands::window_minimize,
            commands::window_toggle_fullscreen,
            commands::window_hide_to_tray,
            // 窗口管理（M1 Hills 化：自绘标题栏）
            commands::window_toggle_maximize,
            commands::window_is_maximized,
            commands::window_close,
            commands::window_set_effect,
        ]);

    builder
        .run(tauri::generate_context!())
        .expect("Tauri 应用启动失败");
}
