//! 窗口视觉特效（M1 Hills 化核心视觉）
//!
//! - Windows 11：默认 **Mica**（云母），主题跟随系统；失败回退 **Acrylic**
//! - Windows 10：**Acrylic**（不支持 Mica）
//! - macOS：**HUD Window Vibrancy**（类毛玻璃）
//! - Linux：无操作（GTK 原生支持有限）
//!
//! 主窗口采用无边框 + 特效背景 + 前端自绘标题栏（对齐 Hills Lite 视觉）

use tauri::{AppHandle, Manager, WebviewWindow};

#[cfg(target_os = "windows")]
use window_vibrancy::{apply_acrylic, apply_mica, clear_acrylic, clear_mica};

#[cfg(target_os = "macos")]
use window_vibrancy::{apply_vibrancy, clear_vibrancy, NSVisualEffectMaterial, NSVisualEffectState};

/// 应用主窗口的默认视觉特效。按“Mica → Acrylic → 透明回退”的优先级尝试。
pub fn apply_main_window_effect(app: &AppHandle) {
    let Some(win) = app.get_webview_window("main") else {
        log::warn!("vibrancy: 主窗口未找到，跳过视觉特效应用");
        return;
    };
    apply_effect(&win);
}

/// 运行时启用/禁用主窗口特效。
pub fn set_main_window_effect(app: &AppHandle, enabled: bool) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "主窗口不存在".to_string())?;
    if enabled {
        apply_effect(&window);
    } else {
        clear_effect(&window);
    }
    Ok(())
}

/// 应用效果到指定窗口。
pub fn apply_effect(window: &WebviewWindow) {
    #[cfg(target_os = "windows")]
    {
        if is_win11_or_newer() {
            match apply_mica(window, None) {
                Ok(_) => {
                    log::info!("vibrancy: 已应用 Mica（Windows 11）");
                    return;
                }
                Err(e) => {
                    log::warn!("vibrancy: Mica 应用失败，回退 Acrylic: {}", e);
                }
            }
        }

        match apply_acrylic(window, Some((18, 18, 18, 125))) {
            Ok(_) => log::info!("vibrancy: 已应用 Acrylic"),
            Err(e) => log::warn!("vibrancy: Acrylic 应用失败: {}", e),
        }
    }

    #[cfg(target_os = "macos")]
    {
        match apply_vibrancy(
            window,
            NSVisualEffectMaterial::HudWindow,
            Some(NSVisualEffectState::Active),
            Some(12.0),
        ) {
            Ok(_) => log::info!("vibrancy: 已应用 macOS HUD Window Vibrancy"),
            Err(e) => log::warn!("vibrancy: macOS Vibrancy 应用失败: {}", e),
        }
    }

    #[cfg(not(any(target_os = "windows", target_os = "macos")))]
    {
        let _ = window;
        log::debug!("vibrancy: 当前平台未实现窗口特效");
    }
}

/// 清除窗口特效（用于播放器沉浸模式切换）。
pub fn clear_effect(window: &WebviewWindow) {
    #[cfg(target_os = "windows")]
    {
        let _ = clear_mica(window);
        let _ = clear_acrylic(window);
    }

    #[cfg(target_os = "macos")]
    {
        let _ = clear_vibrancy(window);
    }

    #[cfg(not(any(target_os = "windows", target_os = "macos")))]
    {
        let _ = window;
    }
}

/// 是否 Windows 11 或更新版本（build ≥ 22000）。
#[cfg(target_os = "windows")]
fn is_win11_or_newer() -> bool {
    use std::mem::MaybeUninit;
    #[link(name = "ntdll")]
    extern "system" {
        fn RtlGetVersion(lpVersionInformation: *mut OsVersionInfoW) -> i32;
    }

    #[repr(C)]
    struct OsVersionInfoW {
        dw_os_version_info_size: u32,
        dw_major_version: u32,
        dw_minor_version: u32,
        dw_build_number: u32,
        dw_platform_id: u32,
        sz_csd_version: [u16; 128],
    }

    unsafe {
        let mut info: MaybeUninit<OsVersionInfoW> = MaybeUninit::zeroed();
        let ptr = info.as_mut_ptr();
        (*ptr).dw_os_version_info_size = std::mem::size_of::<OsVersionInfoW>() as u32;
        if RtlGetVersion(ptr) == 0 {
            let v = info.assume_init();
            v.dw_major_version >= 10 && v.dw_build_number >= 22000
        } else {
            false
        }
    }
}
