//! macOS Desktop Player Surface（无边框上层子窗口贴合模式）。
//!
//! 让 libmpv 自建硬件加速播放窗口（border=no, osc=no, auto-window-resize=no），
//! 并在 sync_bounds 时通过 Cocoa runtime 找到该原生 NSWindow，去掉阴影、设为鼠标穿透
//! （ignoresMouseEvents=YES），以 NSWindowAbove 挂载为主窗口的子窗口，精准贴合前端传入的
//! surfaceRef 区域（控制栏隐藏时铺满全屏，显示控制栏或选择器时自动避让），彻底避免透明挖空导致的白屏。

use anyhow::{anyhow, Context, Result};
use std::ffi::{c_char, c_void};
use tauri::{AppHandle, Manager};

#[allow(dead_code)]
pub const MPV_WINDOW_TITLE: &str = "Nowen Video Player";

#[derive(Debug, Clone, Copy, Default)]
pub struct PlayerSurface;

#[repr(C)]
#[derive(Debug, Clone, Copy)]
struct NSPoint {
    x: f64,
    y: f64,
}

#[repr(C)]
#[derive(Debug, Clone, Copy)]
struct NSSize {
    width: f64,
    height: f64,
}

#[repr(C)]
#[derive(Debug, Clone, Copy)]
struct NSRect {
    origin: NSPoint,
    size: NSSize,
}

#[link(name = "objc")]
extern "C" {
    fn objc_getClass(name: *const c_char) -> *mut c_void;
    fn sel_registerName(name: *const c_char) -> *const c_void;
    fn objc_msgSend();
}

unsafe fn sel(name: &[u8]) -> *const c_void {
    sel_registerName(name.as_ptr() as *const c_char)
}

unsafe fn cls(name: &[u8]) -> *mut c_void {
    objc_getClass(name.as_ptr() as *const c_char)
}

unsafe fn msg_void_ptr(obj: *mut c_void, s: &[u8]) -> *mut c_void {
    let f: unsafe extern "C" fn(*mut c_void, *const c_void) -> *mut c_void =
        std::mem::transmute(objc_msgSend as *const c_void);
    f(obj, sel(s))
}

unsafe fn msg_usize(obj: *mut c_void, s: &[u8]) -> usize {
    let f: unsafe extern "C" fn(*mut c_void, *const c_void) -> usize =
        std::mem::transmute(objc_msgSend as *const c_void);
    f(obj, sel(s))
}

unsafe fn msg_set_bool(obj: *mut c_void, s: &[u8], val: bool) {
    let f: unsafe extern "C" fn(*mut c_void, *const c_void, i8) =
        std::mem::transmute(objc_msgSend as *const c_void);
    f(obj, sel(s), if val { 1 } else { 0 });
}

unsafe fn get_frame(ns_window: *mut c_void) -> NSRect {
    let f: unsafe extern "C" fn(*mut c_void, *const c_void) -> NSRect =
        std::mem::transmute(objc_msgSend as *const c_void);
    f(ns_window, sel(b"frame\0"))
}

extern "C" {
    fn object_getClassName(obj: *mut c_void) -> *const c_char;
}

/// 从 NSApplication.sharedApplication.windows 中查找 mpv 创建的原生窗口
unsafe fn find_mpv_window(main_ns: *mut c_void) -> *mut c_void {
    let ns_app_cls = cls(b"NSApplication\0");
    if ns_app_cls.is_null() {
        return std::ptr::null_mut();
    }
    let shared_app = msg_void_ptr(ns_app_cls, b"sharedApplication\0");
    if shared_app.is_null() {
        return std::ptr::null_mut();
    }
    let windows = msg_void_ptr(shared_app, b"windows\0");
    if windows.is_null() {
        return std::ptr::null_mut();
    }
    let count = msg_usize(windows, b"count\0");
    let obj_at: unsafe extern "C" fn(*mut c_void, *const c_void, usize) -> *mut c_void =
        std::mem::transmute(objc_msgSend as *const c_void);
    let is_visible_fn: unsafe extern "C" fn(*mut c_void, *const c_void) -> i8 =
        std::mem::transmute(objc_msgSend as *const c_void);

    let mut fallback: *mut c_void = std::ptr::null_mut();

    for i in 0..count {
        let win = obj_at(windows, sel(b"objectAtIndex:\0"), i);
        if win.is_null() || win == main_ns {
            continue;
        }
        let cls_cstr = object_getClassName(win);
        if cls_cstr.is_null() {
            continue;
        }
        let cls_name = std::ffi::CStr::from_ptr(cls_cstr).to_string_lossy();
        if is_visible_fn(win, sel(b"isVisible\0")) == 0 {
            continue;
        }
        // libmpv 在 macOS 下创建的 Swift 窗口类名包含 swift.Window / MacWindow
        if cls_name.contains("swift.Window") || cls_name.contains("MacWindow") {
            return win;
        }
        // 排除系统辅助窗口（如 NSMenuWindow、NSPopupMenuWindow、ToolTip 等）与 Tauri/WebKit 窗口
        if cls_name.starts_with("_NS")
            || cls_name.contains("Popup")
            || cls_name.contains("Popover")
            || cls_name.contains("Menu")
            || cls_name.contains("StatusBar")
            || cls_name.contains("ToolTip")
            || cls_name.contains("Tooltip")
            || cls_name.contains("Panel")
            || cls_name.contains("Sheet")
            || cls_name.contains("Alert")
            || cls_name.contains("Cursor")
            || cls_name.contains("Tauri")
            || cls_name.contains("Tao")
            || cls_name.contains("WK")
            || cls_name.contains("Web")
        {
            continue;
        }
        if fallback.is_null() {
            fallback = win;
        }
    }
    if !fallback.is_null() {
        let cls_cstr = object_getClassName(fallback);
        if !cls_cstr.is_null() {
            let cls_name = std::ffi::CStr::from_ptr(cls_cstr).to_string_lossy();
            log::info!("macos surface: attached fallback window class={}", cls_name);
        }
    }
    fallback
}

pub fn ensure(_app: &AppHandle) -> Result<PlayerSurface> {
    Ok(PlayerSurface)
}

pub fn sync_bounds(
    app: &AppHandle,
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
) -> Result<()> {
    let main = app
        .get_webview_window("main")
        .ok_or_else(|| anyhow!("找不到主窗口"))?;
    let scale = main.scale_factor().unwrap_or(1.0).max(1.0);
    let main_ns = main.ns_window().context("获取主窗口 NSWindow 失败")?;

    unsafe {
        let mpv_win = find_mpv_window(main_ns);
        if mpv_win.is_null() {
            log::debug!("macos surface: mpv window not found yet");
            return Ok(());
        }

        // 确保无边框、无阴影、鼠标穿透给主窗口控制条，并解除宽高比/尺寸锁定以便全区适配
        let set_style: unsafe extern "C" fn(*mut c_void, *const c_void, usize) =
            std::mem::transmute(objc_msgSend as *const c_void);
        set_style(mpv_win, sel(b"setStyleMask:\0"), 0);
        msg_set_bool(mpv_win, b"setHasShadow:\0", false);
        msg_set_bool(mpv_win, b"setIgnoresMouseEvents:\0", true);

        let set_size: unsafe extern "C" fn(*mut c_void, *const c_void, NSSize) =
            std::mem::transmute(objc_msgSend as *const c_void);
        set_size(
            mpv_win,
            sel(b"setContentAspectRatio:\0"),
            NSSize {
                width: 0.0,
                height: 0.0,
            },
        );
        set_size(
            mpv_win,
            sel(b"setAspectRatio:\0"),
            NSSize {
                width: 0.0,
                height: 0.0,
            },
        );
        set_size(
            mpv_win,
            sel(b"setResizeIncrements:\0"),
            NSSize {
                width: 1.0,
                height: 1.0,
            },
        );
        set_size(
            mpv_win,
            sel(b"setContentMinSize:\0"),
            NSSize {
                width: 100.0,
                height: 100.0,
            },
        );
        set_size(
            mpv_win,
            sel(b"setContentMaxSize:\0"),
            NSSize {
                width: 100000.0,
                height: 100000.0,
            },
        );

        // NSWindowCollectionBehaviorFullScreenAuxiliary = 1 << 8 (256)，允许随主窗口进入 macOS 全屏空间
        let set_behavior: unsafe extern "C" fn(*mut c_void, *const c_void, usize) =
            std::mem::transmute(objc_msgSend as *const c_void);
        set_behavior(mpv_win, sel(b"setCollectionBehavior:\0"), 1 << 8);

        if !visible {
            let remove_child: unsafe extern "C" fn(*mut c_void, *const c_void, *mut c_void) =
                std::mem::transmute(objc_msgSend as *const c_void);
            remove_child(main_ns, sel(b"removeChildWindow:\0"), mpv_win);
            let order_out: unsafe extern "C" fn(*mut c_void, *const c_void, *mut c_void) =
                std::mem::transmute(objc_msgSend as *const c_void);
            order_out(mpv_win, sel(b"orderOut:\0"), std::ptr::null_mut());
            return Ok(());
        }

        let main_frame = get_frame(main_ns);
        let lx = f64::from(x) / scale;
        let ly = f64::from(y) / scale;
        let lw = f64::from(width.max(1)) / scale;
        let lh = f64::from(height.max(1)) / scale;

        let target_rect = NSRect {
            origin: NSPoint {
                x: main_frame.origin.x + lx,
                y: main_frame.origin.y + main_frame.size.height - ly - lh,
            },
            size: NSSize {
                width: lw,
                height: lh,
            },
        };

        // NSWindowAbove = 1：紧贴在主窗口上方且随主窗口同步移动；先挂载再设置精确坐标避免系统屏幕约束偏移
        let parent_win = msg_void_ptr(mpv_win, b"parentWindow\0");
        if parent_win != main_ns {
            let add_child: unsafe extern "C" fn(*mut c_void, *const c_void, *mut c_void, isize) =
                std::mem::transmute(objc_msgSend as *const c_void);
            add_child(main_ns, sel(b"addChildWindow:ordered:\0"), mpv_win, 1);
        }

        let set_frame: unsafe extern "C" fn(*mut c_void, *const c_void, NSRect, i8) =
            std::mem::transmute(objc_msgSend as *const c_void);
        set_frame(mpv_win, sel(b"setFrame:display:\0"), target_rect, 1);

        let mut actual_frame = get_frame(mpv_win);
        if (actual_frame.origin.x - target_rect.origin.x).abs() > 1.0
            || (actual_frame.origin.y - target_rect.origin.y).abs() > 1.0
        {
            let set_origin: unsafe extern "C" fn(*mut c_void, *const c_void, NSPoint) =
                std::mem::transmute(objc_msgSend as *const c_void);
            set_origin(mpv_win, sel(b"setFrameOrigin:\0"), target_rect.origin);
            set_size(mpv_win, sel(b"setContentSize:\0"), target_rect.size);
            actual_frame = get_frame(mpv_win);
        }

        log::info!(
            "macos surface: main=({:.0},{:.0} {:.0}x{:.0}) target=({:.0},{:.0} {:.0}x{:.0}) actual=({:.0},{:.0} {:.0}x{:.0})",
            main_frame.origin.x,
            main_frame.origin.y,
            main_frame.size.width,
            main_frame.size.height,
            target_rect.origin.x,
            target_rect.origin.y,
            target_rect.size.width,
            target_rect.size.height,
            actual_frame.origin.x,
            actual_frame.origin.y,
            actual_frame.size.width,
            actual_frame.size.height
        );
    }
    Ok(())
}

pub fn resync(_app: &AppHandle) -> Result<()> {
    Ok(())
}

pub fn destroy(app: &AppHandle) -> Result<()> {
    if let Some(main) = app.get_webview_window("main") {
        if let Ok(main_ns) = main.ns_window() {
            unsafe {
                let mpv_win = find_mpv_window(main_ns);
                if !mpv_win.is_null() {
                    let remove_child: unsafe extern "C" fn(*mut c_void, *const c_void, *mut c_void) =
                        std::mem::transmute(objc_msgSend as *const c_void);
                    remove_child(main_ns, sel(b"removeChildWindow:\0"), mpv_win);
                }
            }
        }
    }
    Ok(())
}
