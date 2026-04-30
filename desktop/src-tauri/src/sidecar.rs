
//! Go sidecar 进程管理
//!
//! 负责在应用启动时拉起 `cmd/server` 编译得到的二进制，
//! 应用退出时优雅关闭，运行期间监控健康状态。

use crate::settings::Settings;
use anyhow::{anyhow, Context, Result};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, Instant};
use tauri::{AppHandle, Manager};
#[cfg(target_os = "windows")]
use std::os::windows::process::CommandExt;

pub struct SidecarManager {
    settings: Settings,
    child: Option<Child>,
    started_at: Option<Instant>,
}

impl SidecarManager {
    pub fn new(settings: Settings) -> Self {
        Self {
            settings,
            child: None,
            started_at: None,
        }
    }

    /// 是否已启动
    pub fn is_running(&mut self) -> bool {
        if let Some(child) = &mut self.child {
            match child.try_wait() {
                Ok(None) => true,           // 还在跑
                Ok(Some(_)) => false,       // 已退出
                Err(_) => false,
            }
        } else {
            false
        }
    }

    /// 启动 sidecar
    pub fn start(&mut self, app: &AppHandle) -> Result<()> {
        if self.is_running() {
            log::info!("sidecar 已在运行，跳过启动");
            return Ok(());
        }

        let bin_path = resolve_sidecar_path(app)?;
        log::info!("sidecar 路径: {}", bin_path.display());

        if !bin_path.exists() {
            return Err(anyhow!(
                "sidecar 二进制未找到: {}\n请先运行 desktop/scripts/build-sidecar 构建",
                bin_path.display()
            ));
        }

        let mut cmd = Command::new(&bin_path);
        cmd.stdout(Stdio::piped())
            .stderr(Stdio::piped());

        // Windows 下隐藏控制台窗口
        #[cfg(target_os = "windows")]
        {
            const CREATE_NO_WINDOW: u32 = 0x08000000;
            cmd.creation_flags(CREATE_NO_WINDOW);
        }

        // 设置工作目录为二进制所在目录，确保能读到 config.yaml
        if let Some(parent) = bin_path.parent() {
            cmd.current_dir(parent);
        }

        let child = cmd.spawn().context("启动 Go sidecar 失败")?;
        log::info!("Go sidecar 已启动，PID={}", child.id());
        self.child = Some(child);
        self.started_at = Some(Instant::now());

        Ok(())
    }

    /// 停止 sidecar
    pub fn stop(&mut self) {
        if let Some(mut child) = self.child.take() {
            log::info!("停止 sidecar 进程 PID={}...", child.id());
            // 尽量优雅退出，Windows 无 SIGTERM 概念，直接 kill
            let _ = child.kill();
            let _ = child.wait();
            log::info!("sidecar 已停止");
        }
        self.started_at = None;
    }

    /// 运行状态
    pub fn status(&mut self) -> SidecarStatus {
        let running = self.is_running();
        let pid = self.child.as_ref().map(|c| c.id());
        let uptime_secs = self
            .started_at
            .map(|t| t.elapsed().as_secs())
            .unwrap_or(0);

        SidecarStatus {
            running,
            pid,
            port: self.settings.server.sidecar_port,
            mode: format!("{:?}", self.settings.server.mode).to_lowercase(),
            uptime_secs,
        }
    }

    /// 健康检查（HTTP）
    pub async fn health_check(&self) -> bool {
        let port = self.settings.server.sidecar_port;
        let url = format!("http://127.0.0.1:{}/api/health", port);
        match reqwest::Client::new()
            .get(&url)
            .timeout(Duration::from_secs(3))
            .send()
            .await
        {
            Ok(resp) => resp.status().is_success(),
            Err(_) => false,
        }
    }
}

#[derive(Debug, serde::Serialize)]
pub struct SidecarStatus {
    pub running: bool,
    pub pid: Option<u32>,
    pub port: u16,
    pub mode: String,
    pub uptime_secs: u64,
}

/// 解析 sidecar 二进制路径
///
/// 优先级：
/// 1. 打包后：resources/bin/nowen-video(.exe)
/// 2. dev 模式：desktop/bin/nowen-video(.exe)
/// 3. dev 模式：项目根目录 server.exe / nowen-video.exe
fn resolve_sidecar_path(app: &AppHandle) -> Result<std::path::PathBuf> {
    // 1. 打包后的资源路径
    if let Ok(res_dir) = app.path().resource_dir() {
        let name = if cfg!(target_os = "windows") {
            "nowen-video.exe"
        } else {
            "nowen-video"
        };
        let candidate = res_dir.join("bin").join(name);
        if candidate.exists() {
            return Ok(candidate);
        }
    }

    // 2. dev 模式：desktop/bin
    let cwd = std::env::current_dir().context("获取工作目录失败")?;
    let names: &[&str] = if cfg!(target_os = "windows") {
        &["nowen-video.exe", "server.exe"]
    } else {
        &["nowen-video", "server"]
    };

    // 从 src-tauri 目录向上查找
    let candidates = [
        cwd.join("../bin"),
        cwd.join("../../bin"),
        cwd.join("../../"), // 项目根
        cwd.clone(),
    ];

    for dir in &candidates {
        for name in names {
            let p = dir.join(name);
            if p.exists() {
                return Ok(p);
            }
        }
    }

    // 3. 回退默认
    Ok(cwd.join("bin").join(if cfg!(target_os = "windows") {
        "nowen-video.exe"
    } else {
        "nowen-video"
    }))
}
