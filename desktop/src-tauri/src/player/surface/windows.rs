//! Windows Desktop Player Surface。
//!
//! 这里不创建 Tauri WebView。渲染线程持有一个纯 Win32/OpenGL 窗口，并通过
//! libmpv Render API 把视频画到默认 framebuffer。主 Tauri/WebView 窗口保持在
//! Surface 上方，用透明播放区域承载 React 控件层。

use anyhow::{anyhow, Context, Result};
use libmpv2_sys as mpv;
use std::ffi::{c_char, c_void};
use std::mem::size_of;
use std::ptr::{null, null_mut};
use std::sync::{
    mpsc::{self, Receiver, Sender},
    Arc, Condvar, Mutex, OnceLock,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;
use tauri::{AppHandle, Manager};
use windows::core::{w, PCSTR};
use windows::Win32::Foundation::{HINSTANCE, HWND, LPARAM, LRESULT, RECT, WPARAM};
use windows::Win32::Graphics::Gdi::{GetDC, ReleaseDC};
use windows::Win32::Graphics::OpenGL::{
    wglCreateContext, wglDeleteContext, wglGetProcAddress, wglMakeCurrent, ChoosePixelFormat,
    SetPixelFormat, SwapBuffers, HGLRC, PFD_DOUBLEBUFFER, PFD_DRAW_TO_WINDOW, PFD_MAIN_PLANE,
    PFD_SUPPORT_OPENGL, PFD_TYPE_RGBA, PIXELFORMATDESCRIPTOR,
};
use windows::Win32::System::LibraryLoader::{GetModuleHandleA, GetModuleHandleW, GetProcAddress};
use windows::Win32::UI::WindowsAndMessaging::{
    CreateWindowExW, DefWindowProcW, DestroyWindow, DispatchMessageW, GetWindowRect, PeekMessageW,
    RegisterClassW, SetWindowPos, ShowWindow, TranslateMessage, CS_OWNDC, HWND_BOTTOM, MSG,
    PM_REMOVE, SW_HIDE, SW_SHOWNOACTIVATE, SWP_NOACTIVATE, SWP_NOOWNERZORDER, SWP_SHOWWINDOW,
    WINDOW_EX_STYLE, WNDCLASSW, WM_ERASEBKGND, WM_PAINT, WS_EX_NOACTIVATE, WS_EX_TOOLWINDOW,
    WS_POPUP,
};

const CLASS_NAME: windows::core::PCWSTR = w!("NowenVideoRenderSurface");
const WINDOW_NAME: windows::core::PCWSTR = w!("Nowen Video Render Surface");

// mpv_render_param_type，值来自 libmpv render.h 的稳定 ABI。
const RENDER_PARAM_INVALID: u32 = 0;
const RENDER_PARAM_API_TYPE: u32 = 1;
const RENDER_PARAM_OPENGL_INIT_PARAMS: u32 = 2;
const RENDER_PARAM_OPENGL_FBO: u32 = 3;
const RENDER_PARAM_FLIP_Y: u32 = 4;

#[derive(Debug, Clone, Copy, Default)]
struct SurfaceBounds {
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
}

#[derive(Default)]
struct WakeState {
    pending: Mutex<bool>,
    condvar: Condvar,
}

impl WakeState {
    fn wake(&self) {
        if let Ok(mut pending) = self.pending.lock() {
            *pending = true;
            self.condvar.notify_one();
        }
    }

    fn wait(&self) {
        if let Ok(mut pending) = self.pending.lock() {
            if !*pending {
                let result = self.condvar.wait_timeout(pending, Duration::from_millis(16));
                if let Ok((guard, _)) = result {
                    pending = guard;
                } else {
                    return;
                }
            }
            *pending = false;
        }
    }
}

enum SurfaceCommand {
    SetBounds(SurfaceBounds),
    Resync,
    AttachRenderer {
        mpv_handle: usize,
        reply: mpsc::SyncSender<std::result::Result<(), String>>,
    },
    DetachRenderer,
    Shutdown,
}

struct SurfaceInner {
    tx: Sender<SurfaceCommand>,
    wake: Arc<WakeState>,
    thread: Mutex<Option<JoinHandle<()>>>,
}

#[derive(Clone)]
pub struct PlayerSurface {
    inner: Arc<SurfaceInner>,
}

impl std::fmt::Debug for PlayerSurface {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        formatter.debug_struct("PlayerSurface").finish_non_exhaustive()
    }
}

impl PlayerSurface {
    pub(crate) fn attach_renderer(&self, mpv_handle: *mut mpv::mpv_handle) -> Result<()> {
        if mpv_handle.is_null() {
            return Err(anyhow!("Player Core mpv handle 为空"));
        }
        let (reply_tx, reply_rx) = mpsc::sync_channel(1);
        self.send(SurfaceCommand::AttachRenderer {
            mpv_handle: mpv_handle as usize,
            reply: reply_tx,
        })?;
        match reply_rx.recv_timeout(Duration::from_secs(5)) {
            Ok(Ok(())) => Ok(()),
            Ok(Err(error)) => Err(anyhow!(error)),
            Err(error) => Err(anyhow!("等待 Render API 初始化超时: {}", error)),
        }
    }

    pub(crate) fn detach_renderer(&self) {
        let _ = self.send(SurfaceCommand::DetachRenderer);
    }

    fn send(&self, command: SurfaceCommand) -> Result<()> {
        self.inner
            .tx
            .send(command)
            .map_err(|error| anyhow!("Player Surface 渲染线程不可用: {}", error))?;
        self.inner.wake.wake();
        Ok(())
    }

    fn shutdown(&self) {
        let _ = self.send(SurfaceCommand::Shutdown);
        if let Ok(mut thread) = self.inner.thread.lock() {
            if let Some(handle) = thread.take() {
                let _ = handle.join();
            }
        }
    }
}

static SURFACE: OnceLock<Mutex<Option<PlayerSurface>>> = OnceLock::new();

fn surface_slot() -> &'static Mutex<Option<PlayerSurface>> {
    SURFACE.get_or_init(|| Mutex::new(None))
}

pub fn ensure(app: &AppHandle) -> Result<PlayerSurface> {
    let mut slot = surface_slot()
        .lock()
        .map_err(|_| anyhow!("Player Surface 状态锁已损坏"))?;
    if let Some(surface) = slot.as_ref() {
        return Ok(surface.clone());
    }

    let main = app
        .get_webview_window("main")
        .ok_or_else(|| anyhow!("主窗口不存在"))?;
    let main_hwnd = main.hwnd().context("获取主窗口 HWND 失败")?;

    let (tx, rx) = mpsc::channel();
    let (ready_tx, ready_rx) = mpsc::sync_channel(1);
    let wake = Arc::new(WakeState::default());
    let thread_wake = Arc::clone(&wake);
    let main_hwnd_addr = main_hwnd.0 as isize;

    let thread = thread::Builder::new()
        .name("player-render-win32".into())
        .spawn(move || {
            let result = RenderThread::new(HWND(main_hwnd_addr as *mut c_void));
            match result {
                Ok(mut renderer) => {
                    let _ = ready_tx.send(Ok(()));
                    renderer.run(rx, thread_wake);
                }
                Err(error) => {
                    let _ = ready_tx.send(Err(error.to_string()));
                }
            }
        })
        .context("创建 Windows Player Render 线程失败")?;

    match ready_rx.recv_timeout(Duration::from_secs(5)) {
        Ok(Ok(())) => {}
        Ok(Err(error)) => {
            let _ = thread.join();
            return Err(anyhow!("初始化 Windows Player Surface 失败: {}", error));
        }
        Err(error) => {
            return Err(anyhow!("等待 Windows Player Surface 初始化超时: {}", error));
        }
    }

    let surface = PlayerSurface {
        inner: Arc::new(SurfaceInner {
            tx,
            wake,
            thread: Mutex::new(Some(thread)),
        }),
    };
    *slot = Some(surface.clone());
    Ok(surface)
}

pub fn sync_bounds(
    _app: &AppHandle,
    x: i32,
    y: i32,
    width: u32,
    height: u32,
    visible: bool,
) -> Result<()> {
    let slot = surface_slot()
        .lock()
        .map_err(|_| anyhow!("Player Surface 状态锁已损坏"))?;
    let surface = slot
        .as_ref()
        .ok_or_else(|| anyhow!("Windows Player Surface 尚未创建"))?;
    surface.send(SurfaceCommand::SetBounds(SurfaceBounds {
        x,
        y,
        width: width.max(1),
        height: height.max(1),
        visible,
    }))
}

pub fn resync(_app: &AppHandle) -> Result<()> {
    let slot = surface_slot()
        .lock()
        .map_err(|_| anyhow!("Player Surface 状态锁已损坏"))?;
    if let Some(surface) = slot.as_ref() {
        surface.send(SurfaceCommand::Resync)?;
    }
    Ok(())
}

pub fn destroy(_app: &AppHandle) -> Result<()> {
    let surface = surface_slot()
        .lock()
        .map_err(|_| anyhow!("Player Surface 状态锁已损坏"))?
        .take();
    if let Some(surface) = surface {
        surface.shutdown();
    }
    Ok(())
}

struct RenderThread {
    main_hwnd: HWND,
    hwnd: HWND,
    hdc: windows::Win32::Graphics::Gdi::HDC,
    glrc: HGLRC,
    bounds: SurfaceBounds,
    render_context: *mut mpv::mpv_render_context,
    callback_ctx: *mut c_void,
}

impl RenderThread {
    fn new(main_hwnd: HWND) -> Result<Self> {
        unsafe {
            let module = GetModuleHandleW(None).context("获取 Windows 模块句柄失败")?;
            let instance = HINSTANCE(module.0);
            let class = WNDCLASSW {
                style: CS_OWNDC,
                lpfnWndProc: Some(surface_window_proc),
                hInstance: instance,
                lpszClassName: CLASS_NAME,
                ..Default::default()
            };
            let _ = RegisterClassW(&class);

            let hwnd = CreateWindowExW(
                WINDOW_EX_STYLE(WS_EX_TOOLWINDOW.0 | WS_EX_NOACTIVATE.0),
                CLASS_NAME,
                WINDOW_NAME,
                WS_POPUP,
                0,
                0,
                16,
                16,
                None,
                None,
                Some(instance),
                None,
            )
            .context("创建 Win32 Player Surface 窗口失败")?;

            let hdc = GetDC(Some(hwnd));
            if hdc.0 == 0 {
                let _ = DestroyWindow(hwnd);
                return Err(anyhow!("获取 Player Surface HDC 失败"));
            }

            let pfd = PIXELFORMATDESCRIPTOR {
                nSize: size_of::<PIXELFORMATDESCRIPTOR>() as u16,
                nVersion: 1,
                dwFlags: PFD_DRAW_TO_WINDOW | PFD_SUPPORT_OPENGL | PFD_DOUBLEBUFFER,
                iPixelType: PFD_TYPE_RGBA,
                cColorBits: 32,
                cAlphaBits: 8,
                cDepthBits: 24,
                cStencilBits: 8,
                iLayerType: PFD_MAIN_PLANE.0 as u8,
                ..Default::default()
            };
            let pixel_format = ChoosePixelFormat(hdc, &pfd);
            if pixel_format == 0 {
                ReleaseDC(Some(hwnd), hdc);
                let _ = DestroyWindow(hwnd);
                return Err(anyhow!("选择 OpenGL PixelFormat 失败"));
            }
            SetPixelFormat(hdc, pixel_format, &pfd).context("设置 OpenGL PixelFormat 失败")?;

            let glrc = wglCreateContext(hdc).context("创建 WGL Context 失败")?;
            wglMakeCurrent(hdc, glrc).context("激活 WGL Context 失败")?;

            Ok(Self {
                main_hwnd,
                hwnd,
                hdc,
                glrc,
                bounds: SurfaceBounds::default(),
                render_context: null_mut(),
                callback_ctx: null_mut(),
            })
        }
    }

    fn run(&mut self, rx: Receiver<SurfaceCommand>, wake: Arc<WakeState>) {
        let mut running = true;
        while running {
            while let Ok(command) = rx.try_recv() {
                match command {
                    SurfaceCommand::SetBounds(bounds) => {
                        self.bounds = bounds;
                        self.apply_bounds();
                    }
                    SurfaceCommand::Resync => self.apply_bounds(),
                    SurfaceCommand::AttachRenderer { mpv_handle, reply } => {
                        let result = self.attach_renderer(mpv_handle as *mut mpv::mpv_handle);
                        let _ = reply.send(result.map_err(|error| error.to_string()));
                    }
                    SurfaceCommand::DetachRenderer => self.detach_renderer(),
                    SurfaceCommand::Shutdown => {
                        running = false;
                        break;
                    }
                }
            }

            self.pump_messages();
            if running && self.render_context.is_null() == false && self.bounds.visible {
                let should_render = wake.pending.lock().map(|pending| *pending).unwrap_or(true);
                if should_render {
                    if let Err(error) = self.render_frame() {
                        log::error!("Windows Player Render API 绘制失败: {}", error);
                    }
                }
            }
            if running {
                wake.wait();
            }
        }

        self.detach_renderer();
        unsafe {
            let _ = wglMakeCurrent(self.hdc, HGLRC::default());
            let _ = wglDeleteContext(self.glrc);
            ReleaseDC(Some(self.hwnd), self.hdc);
            let _ = DestroyWindow(self.hwnd);
        }
    }

    fn apply_bounds(&self) {
        unsafe {
            if !self.bounds.visible {
                ShowWindow(self.hwnd, SW_HIDE);
                return;
            }

            let mut rect = RECT::default();
            if GetWindowRect(self.main_hwnd, &mut rect).is_err() {
                return;
            }
            let x = rect.left + self.bounds.x;
            let y = rect.top + self.bounds.y;
            let _ = SetWindowPos(
                self.hwnd,
                Some(self.main_hwnd),
                x,
                y,
                self.bounds.width.max(1) as i32,
                self.bounds.height.max(1) as i32,
                SWP_NOACTIVATE | SWP_NOOWNERZORDER | SWP_SHOWWINDOW,
            );
            ShowWindow(self.hwnd, SW_SHOWNOACTIVATE);
        }
    }

    fn pump_messages(&self) {
        unsafe {
            let mut message = MSG::default();
            while PeekMessageW(&mut message, Some(self.hwnd), 0, 0, PM_REMOVE).as_bool() {
                TranslateMessage(&message);
                DispatchMessageW(&message);
            }
        }
    }

    fn attach_renderer(&mut self, mpv_handle: *mut mpv::mpv_handle) -> Result<()> {
        self.detach_renderer();
        if mpv_handle.is_null() {
            return Err(anyhow!("Render API mpv handle 为空"));
        }

        unsafe {
            wglMakeCurrent(self.hdc, self.glrc).context("重新激活 WGL Context 失败")?;

            let mut init = mpv::mpv_opengl_init_params {
                get_proc_address: Some(gl_get_proc_address),
                get_proc_address_ctx: null_mut(),
            };
            let mut params = [
                mpv::mpv_render_param {
                    type_: RENDER_PARAM_API_TYPE,
                    data: mpv::MPV_RENDER_API_TYPE_OPENGL.as_ptr() as *mut c_void,
                },
                mpv::mpv_render_param {
                    type_: RENDER_PARAM_OPENGL_INIT_PARAMS,
                    data: &mut init as *mut _ as *mut c_void,
                },
                mpv::mpv_render_param {
                    type_: RENDER_PARAM_INVALID,
                    data: null_mut(),
                },
            ];

            let mut context: *mut mpv::mpv_render_context = null_mut();
            let result = mpv::mpv_render_context_create(&mut context, mpv_handle, params.as_mut_ptr());
            if result < 0 || context.is_null() {
                return Err(anyhow!("创建 libmpv Render Context 失败，错误码 {}", result));
            }

            let callback_wake = Arc::new(WakeState::default());
            // Render callback 只负责把渲染线程唤醒，不调用任何 mpv/OpenGL API。
            let callback_ctx = Box::into_raw(Box::new(callback_wake)) as *mut c_void;
            mpv::mpv_render_context_set_update_callback(
                context,
                Some(render_update_callback),
                callback_ctx,
            );
            self.render_context = context;
            self.callback_ctx = callback_ctx;
        }
        Ok(())
    }

    fn detach_renderer(&mut self) {
        unsafe {
            if !self.render_context.is_null() {
                mpv::mpv_render_context_set_update_callback(self.render_context, None, null_mut());
                mpv::mpv_render_context_free(self.render_context);
                self.render_context = null_mut();
            }
            if !self.callback_ctx.is_null() {
                drop(Box::from_raw(self.callback_ctx as *mut Arc<WakeState>));
                self.callback_ctx = null_mut();
            }
        }
    }

    fn render_frame(&self) -> Result<()> {
        if self.render_context.is_null() {
            return Ok(());
        }
        unsafe {
            wglMakeCurrent(self.hdc, self.glrc).context("激活渲染 WGL Context 失败")?;
            let mut fbo = mpv::mpv_opengl_fbo {
                fbo: 0,
                w: self.bounds.width.max(1) as i32,
                h: self.bounds.height.max(1) as i32,
                internal_format: 0,
            };
            let mut flip_y: i32 = 1;
            let mut params = [
                mpv::mpv_render_param {
                    type_: RENDER_PARAM_OPENGL_FBO,
                    data: &mut fbo as *mut _ as *mut c_void,
                },
                mpv::mpv_render_param {
                    type_: RENDER_PARAM_FLIP_Y,
                    data: &mut flip_y as *mut _ as *mut c_void,
                },
                mpv::mpv_render_param {
                    type_: RENDER_PARAM_INVALID,
                    data: null_mut(),
                },
            ];
            let result = mpv::mpv_render_context_render(self.render_context, params.as_mut_ptr());
            if result < 0 {
                return Err(anyhow!("mpv_render_context_render 失败，错误码 {}", result));
            }
            SwapBuffers(self.hdc).context("交换 OpenGL Buffer 失败")?;
            mpv::mpv_render_context_report_swap(self.render_context);
        }
        Ok(())
    }
}

impl Drop for RenderThread {
    fn drop(&mut self) {
        self.detach_renderer();
    }
}

unsafe extern "system" fn surface_window_proc(
    hwnd: HWND,
    message: u32,
    wparam: WPARAM,
    lparam: LPARAM,
) -> LRESULT {
    match message {
        WM_ERASEBKGND => LRESULT(1),
        WM_PAINT => LRESULT(0),
        _ => unsafe { DefWindowProcW(hwnd, message, wparam, lparam) },
    }
}

unsafe extern "C" fn render_update_callback(ctx: *mut c_void) {
    if ctx.is_null() {
        return;
    }
    let wake = unsafe { &*(ctx as *const Arc<WakeState>) };
    wake.wake();
}

unsafe extern "C" fn gl_get_proc_address(_ctx: *mut c_void, name: *const c_char) -> *mut c_void {
    if name.is_null() {
        return null_mut();
    }

    unsafe {
        if let Some(proc) = wglGetProcAddress(PCSTR(name as *const u8)) {
            return proc as *const () as *mut c_void;
        }
        if let Ok(module) = GetModuleHandleA(PCSTR(b"opengl32.dll\0".as_ptr())) {
            if let Some(proc) = GetProcAddress(module, PCSTR(name as *const u8)) {
                return proc as *const () as *mut c_void;
            }
        }
    }
    null_mut()
}
