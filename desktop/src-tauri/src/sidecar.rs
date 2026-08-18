//! Go Media Core sidecar 生命周期管理。
//!
//! Desktop 2.0 不再占用固定端口。每次启动内嵌 Media Core 时选择一个
//! 127.0.0.1 空闲端口，并通过 NOWEN_APP_PORT 传给官方 Go server runtime。
//! 同时显式标记 NOWEN_DESKTOP_RUNTIME=1，使 Go 端只监听 loopback 且关闭 mDNS。

use crate::settings::Settings;
use anyhow::{anyhow, Context, Result};
use std::net::TcpListener;
#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};
use tauri::{AppHandle, Manager};

pub struct SidecarManager {
    settings: Settings,
    child: Option<Child>,
    started_at: Option<Instant>,
    runtime_port: u16,
}

impl SidecarManager {
    pub fn new(settings: Settings) -> Self {
        Self {
            settings,
            child: None,
            started_at: None,
            runtime_port: 0,
        }
    }

    pub fn is_running(&mut self) -> bool {
        match &mut self.child {
            Some(child) => matches!(child.try_wait(), Ok(None)),
            None => false,
        }
    }

    pub fn start(&mut self, app: &AppHandle) -> Result<()> {
        if self.is_running() {
            log::info!("Go Media Core 已在运行，跳过重复启动");
            return Ok(());
        }

        let bin_path = resolve_sidecar_path(app)?;
        if !bin_path.exists() {
            return Err(anyhow!(
                "Go Media Core 二进制未找到: {}\n请先构建 Desktop sidecar",
                bin_path.display()
            ));
        }

        let port = reserve_runtime_port()?;
        log::info!(
            "启动 Go Media Core: {}，监听 127.0.0.1:{}",
            bin_path.display(),
            port
        );

        let mut command = Command::new(&bin_path);
        command
            .env("NOWEN_APP_PORT", port.to_string())
            .env("NOWEN_DESKTOP_RUNTIME", "1")
            // Desktop sidecar 的 stdout/stderr 不使用管道，避免无人消费管道导致日志量大时阻塞子进程。
            .stdout(Stdio::null())
            .stderr(Stdio::null());

        #[cfg(target_os = "windows")]
        {
            const CREATE_NO_WINDOW: u32 = 0x08000000;
            command.creation_flags(CREATE_NO_WINDOW);
        }

        if let Some(parent) = bin_path.parent() {
            command.current_dir(parent);
        }

        let child = command.spawn().context("启动 Go Media Core 失败")?;
        log::info!("Go Media Core 已启动，PID={}", child.id());
        self.child = Some(child);
        self.started_at = Some(Instant::now());
        self.runtime_port = port;
        Ok(())
    }

    pub fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            log::info!("停止 Go Media Core，PID={}", child.id());
            let _ = child.kill();
            let _ = child.wait();
        }
        self.started_at = None;
        self.runtime_port = 0;
    }

    pub fn status(&mut self) -> SidecarStatus {
        let running = self.is_running();
        let pid = self.child.as_ref().map(|child| child.id());
        let uptime_secs = self
            .started_at
            .map(|started| started.elapsed().as_secs())
            .unwrap_or(0);

        SidecarStatus {
            running,
            pid,
            port: self.runtime_port,
            mode: format!("{:?}", self.settings.server.mode).to_lowercase(),
            uptime_secs,
        }
    }

    pub async fn health_check(&self) -> bool {
        if self.runtime_port == 0 {
            return false;
        }
        let url = format!("http://127.0.0.1:{}/api/health", self.runtime_port);
        match reqwest::Client::new()
            .get(url)
            .timeout(Duration::from_secs(3))
            .send()
            .await
        {
            Ok(response) => response.status().is_success(),
            Err(_) => false,
        }
    }
}

#[derive(Debug, Clone, serde::Serialize)]
pub struct SidecarStatus {
    pub running: bool,
    pub pid: Option<u32>,
    pub port: u16,
    pub mode: String,
    pub uptime_secs: u64,
}

fn reserve_runtime_port() -> Result<u16> {
    let listener = TcpListener::bind(("127.0.0.1", 0)).context("无法分配 Desktop Sidecar 端口")?;
    let port = listener
        .local_addr()
        .context("无法读取 Desktop Sidecar 端口")?
        .port();
    drop(listener);
    Ok(port)
}

fn resolve_sidecar_path(app: &AppHandle) -> Result<std::path::PathBuf> {
    let exe_name = if cfg!(target_os = "windows") {
        "nowen-video-server.exe"
    } else {
        "nowen-video-server"
    };

    #[cfg(target_os = "windows")]
    let sibling_candidates: &[&str] = &[
        "nowen-video-server.exe",
        "nowen-video-server-x86_64-pc-windows-msvc.exe",
        "nowen-video-server-aarch64-pc-windows-msvc.exe",
    ];
    #[cfg(not(target_os = "windows"))]
    let sibling_candidates: &[&str] = &[
        "nowen-video-server",
        "nowen-video-server-x86_64-unknown-linux-gnu",
        "nowen-video-server-aarch64-unknown-linux-gnu",
        "nowen-video-server-x86_64-apple-darwin",
        "nowen-video-server-aarch64-apple-darwin",
    ];

    if let Ok(main_exe) = std::env::current_exe() {
        if let Some(directory) = main_exe.parent() {
            for name in sibling_candidates {
                let candidate = directory.join(name);
                if candidate.exists() && candidate != main_exe {
                    return Ok(candidate);
                }
            }
        }
    }

    if let Ok(resource_dir) = app.path().resource_dir() {
        let candidate = resource_dir.join("bin").join(exe_name);
        if candidate.exists() {
            return Ok(candidate);
        }
    }

    let cwd = std::env::current_dir().context("获取工作目录失败")?;
    let directories = [
        cwd.join("bin"),
        cwd.join("../bin"),
        cwd.join("../../src-tauri/bin"),
        cwd.clone(),
    ];

    for directory in directories {
        let candidate = directory.join(exe_name);
        if candidate.exists() {
            return Ok(candidate);
        }
    }

    Ok(cwd.join("bin").join(exe_name))
}
