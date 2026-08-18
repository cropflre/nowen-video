use tauri::{Emitter, Manager};

use crate::{file_assoc, runtime::AppState, settings, tray, updater, vibrancy};

/// 完成 Desktop 2.0 应用级初始化。
///
/// main.rs 只负责组装 Tauri Builder；具体启动流程集中在这里，后续 Sidecar、
/// Player Core、Updater 都可以独立演进，不再持续膨胀入口文件。
pub fn setup(app: &mut tauri::App) -> Result<(), Box<dyn std::error::Error>> {
    let handle = app.handle().clone();

    vibrancy::apply_main_window_effect(&handle);

    #[cfg(target_os = "macos")]
    {
        match tray::build_app_menu(&handle) {
            Ok(menu) => {
                let _ = app.set_menu(menu);
            }
            Err(error) => log::warn!("构建 macOS 主菜单失败: {}", error),
        }
    }

    if let Err(error) = tray::build_tray(&handle) {
        log::warn!("创建系统托盘失败: {}", error);
    }

    configure_deep_link(&handle);
    file_assoc::handle_startup_args(&handle);

    start_sidecar(handle.clone());
    start_update_check(handle);

    Ok(())
}

fn configure_deep_link(handle: &tauri::AppHandle) {
    use tauri_plugin_deep_link::DeepLinkExt;

    let event_handle = handle.clone();
    handle.deep_link().on_open_url(move |event| {
        file_assoc::handle_deep_link(&event_handle, event.urls());
    });

    #[cfg(any(target_os = "windows", target_os = "linux"))]
    if let Err(error) = handle.deep_link().register("nowen-video") {
        log::warn!("注册 nowen-video Deep Link 失败: {}", error);
    }
}

fn start_sidecar(handle: tauri::AppHandle) {
    tauri::async_runtime::spawn(async move {
        let state: tauri::State<AppState> = handle.state();
        let current_settings = match state.settings.lock() {
            Ok(settings) => settings.clone(),
            Err(error) => {
                log::error!("读取桌面端设置失败: {}", error);
                return;
            }
        };

        if current_settings.server.mode != settings::ServerMode::Embedded {
            log::info!("Desktop 使用远程服务器: {}", current_settings.server.remote_url);
            return;
        }

        log::info!("启动 Desktop 内嵌 Go Media Core...");
        match state.sidecar.lock() {
            Ok(mut sidecar) => {
                if let Err(error) = sidecar.start(&handle) {
                    log::error!("Go Media Core 启动失败: {}", error);
                }
            }
            Err(error) => log::error!("获取 SidecarManager 失败: {}", error),
        }
    });
}

fn start_update_check(handle: tauri::AppHandle) {
    tauri::async_runtime::spawn(async move {
        tokio::time::sleep(std::time::Duration::from_secs(3)).await;
        match updater::check(&handle).await {
            Ok(info) if info.available => {
                log::info!("检测到 Desktop 新版本: {}", info.version);
                let _ = handle.emit("update-available", info);
            }
            Ok(_) => log::debug!("Desktop 当前已是最新版本"),
            Err(error) => log::debug!("检查 Desktop 更新失败: {}", error),
        }
    });
}

/// 在 Windows 生产环境中确保打包的 libmpv 动态库可被加载。
pub fn ensure_libmpv_runtime() {
    #[cfg(all(target_os = "windows", feature = "embed-mpv"))]
    ensure_windows_libmpv_runtime();
}

#[cfg(all(target_os = "windows", feature = "embed-mpv"))]
fn ensure_windows_libmpv_runtime() {
    use std::path::PathBuf;

    let mut candidates: Vec<PathBuf> = Vec::new();

    if let Some(manifest) = option_env!("CARGO_MANIFEST_DIR") {
        candidates.push(PathBuf::from(manifest).join("resources").join("mpv"));
    }

    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            candidates.push(dir.join("resources").join("mpv"));
            candidates.push(dir.to_path_buf());
        }
    }

    for dir in candidates {
        let dll = dir.join("libmpv-2.dll");
        if dll.exists() {
            log::info!("libmpv runtime: {}", dll.display());
            add_to_windows_dll_search_path(&dir);
            return;
        }
    }

    log::warn!("未找到 libmpv-2.dll，Desktop Player Core 将不可用");
}

#[cfg(all(target_os = "windows", feature = "embed-mpv"))]
fn add_to_windows_dll_search_path(dir: &std::path::Path) {
    use std::os::windows::ffi::OsStrExt;

    let wide: Vec<u16> = dir
        .as_os_str()
        .encode_wide()
        .chain(std::iter::once(0))
        .collect();

    unsafe {
        #[link(name = "kernel32")]
        extern "system" {
            fn SetDllDirectoryW(path: *const u16) -> i32;
        }

        if SetDllDirectoryW(wide.as_ptr()) == 0 {
            log::warn!("SetDllDirectoryW 配置 libmpv 搜索路径失败");
        }
    }

    let path = std::env::var_os("PATH").unwrap_or_default();
    let mut new_path = std::ffi::OsString::from(dir.as_os_str());
    new_path.push(";");
    new_path.push(path);
    std::env::set_var("PATH", new_path);
}
