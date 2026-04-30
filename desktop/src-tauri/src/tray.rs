
//! 系统托盘 + 原生菜单（M7）

use anyhow::Result;
use tauri::{
    menu::{Menu, MenuItem, PredefinedMenuItem, Submenu},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    AppHandle, Emitter, Manager,
};

/// 构建应用主菜单（顶部菜单栏，Windows/Linux）
pub fn build_app_menu(app: &AppHandle) -> Result<Menu<tauri::Wry>> {
    // 文件菜单
    let open_file = MenuItem::with_id(app, "menu_open_file", "打开文件...", true, Some("CmdOrCtrl+O"))?;
    let open_folder =
        MenuItem::with_id(app, "menu_open_folder", "打开文件夹...", true, Some("CmdOrCtrl+Shift+O"))?;
    let exit = MenuItem::with_id(app, "menu_exit", "退出", true, Some("CmdOrCtrl+Q"))?;
    let file_menu = Submenu::with_id_and_items(
        app,
        "menu_file",
        "文件",
        true,
        &[
            &open_file,
            &open_folder,
            &PredefinedMenuItem::separator(app)?,
            &exit,
        ],
    )?;

    // 播放菜单
    let play_pause = MenuItem::with_id(app, "menu_play_pause", "播放/暂停", true, Some("Space"))?;
    let fullscreen = MenuItem::with_id(app, "menu_fullscreen", "全屏", true, Some("F11"))?;
    let play_menu = Submenu::with_id_and_items(
        app,
        "menu_play",
        "播放",
        true,
        &[&play_pause, &fullscreen],
    )?;

    // 工具菜单
    let settings = MenuItem::with_id(app, "menu_settings", "设置", true, Some("CmdOrCtrl+,"))?;
    let restart_backend =
        MenuItem::with_id(app, "menu_restart_backend", "重启后端服务", true, None::<&str>)?;
    let check_update =
        MenuItem::with_id(app, "menu_check_update", "检查更新", true, None::<&str>)?;
    let tools_menu = Submenu::with_id_and_items(
        app,
        "menu_tools",
        "工具",
        true,
        &[
            &settings,
            &restart_backend,
            &PredefinedMenuItem::separator(app)?,
            &check_update,
        ],
    )?;

    // 帮助菜单
    let website = MenuItem::with_id(app, "menu_website", "访问官网", true, None::<&str>)?;
    let issues = MenuItem::with_id(app, "menu_issues", "反馈问题", true, None::<&str>)?;
    let about = MenuItem::with_id(app, "menu_about", "关于", true, None::<&str>)?;
    let help_menu = Submenu::with_id_and_items(
        app,
        "menu_help",
        "帮助",
        true,
        &[&website, &issues, &PredefinedMenuItem::separator(app)?, &about],
    )?;

    let menu = Menu::with_items(app, &[&file_menu, &play_menu, &tools_menu, &help_menu])?;
    Ok(menu)
}

/// 处理菜单点击
pub fn handle_menu_event(app: &AppHandle, event_id: &str) {
    log::info!("菜单点击: {}", event_id);

    // 主窗口广播事件给前端处理业务逻辑
    if let Some(win) = app.get_webview_window("main") {
        let _ = win.emit("menu-action", event_id.to_string());
        let _ = win.show();
        let _ = win.set_focus();
    }

    match event_id {
        "menu_exit" => {
            app.exit(0);
        }
        "menu_website" => {
            let _ = crate::commands::open_url_internal("https://github.com/nowen-video/nowen-video");
        }
        "menu_issues" => {
            let _ = crate::commands::open_url_internal(
                "https://github.com/nowen-video/nowen-video/issues",
            );
        }
        "menu_check_update" => {
            let handle = app.clone();
            tauri::async_runtime::spawn(async move {
                match crate::updater::check(&handle).await {
                    Ok(info) => {
                        let _ = handle.emit("update-available", info);
                    }
                    Err(e) => log::error!("检查更新失败: {}", e),
                }
            });
        }
        _ => {}
    }
}

/// 创建系统托盘
pub fn build_tray(app: &AppHandle) -> Result<()> {
    let show = MenuItem::with_id(app, "tray_show", "显示主窗口", true, None::<&str>)?;
    let hide = MenuItem::with_id(app, "tray_hide", "隐藏", true, None::<&str>)?;
    let quit = MenuItem::with_id(app, "tray_quit", "退出", true, None::<&str>)?;

    let menu = Menu::with_items(
        app,
        &[&show, &hide, &PredefinedMenuItem::separator(app)?, &quit],
    )?;

    let mut builder = TrayIconBuilder::with_id("main-tray")
        .tooltip("nowen-video")
        .menu(&menu)
        .on_menu_event(|app, event| match event.id.as_ref() {
            "tray_show" => {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.show();
                    let _ = win.set_focus();
                    let _ = win.unminimize();
                }
            }
            "tray_hide" => {
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.hide();
                }
            }
            "tray_quit" => {
                app.exit(0);
            }
            _ => {}
        })
        .on_tray_icon_event(|tray, event| {
            if let TrayIconEvent::Click {
                button: MouseButton::Left,
                button_state: MouseButtonState::Up,
                ..
            } = event
            {
                let app = tray.app_handle();
                if let Some(win) = app.get_webview_window("main") {
                    let _ = win.show();
                    let _ = win.set_focus();
                    let _ = win.unminimize();
                }
            }
        });

    // 使用应用默认图标（若已配置）
    if let Some(icon) = app.default_window_icon() {
        builder = builder.icon(icon.clone());
    }

    builder.build(app)?;
    Ok(())
}
