
//! 自动更新（M5）
//!
//! 基于 Tauri Updater 插件，从 GitHub Releases 拉取更新包。
//! 前端通过 `check_update` / `install_update` 命令触发。

use anyhow::{anyhow, Context, Result};
use serde::Serialize;
use tauri::AppHandle;
use tauri_plugin_updater::UpdaterExt;

#[derive(Debug, Serialize, Clone)]
pub struct UpdateInfo {
    pub available: bool,
    pub version: String,
    pub current_version: String,
    pub notes: String,
    pub pub_date: String,
}

impl UpdateInfo {
    pub fn none(current: &str) -> Self {
        Self {
            available: false,
            version: current.to_string(),
            current_version: current.to_string(),
            notes: String::new(),
            pub_date: String::new(),
        }
    }
}

/// 检查更新（不下载）
pub async fn check(app: &AppHandle) -> Result<UpdateInfo> {
    let current = app.package_info().version.to_string();
    let updater = app
        .updater()
        .context("获取 updater 失败（是否已配置 endpoints？）")?;

    let check = updater.check().await.context("检查更新失败")?;
    match check {
        Some(update) => Ok(UpdateInfo {
            available: true,
            version: update.version.clone(),
            current_version: current,
            notes: update.body.clone().unwrap_or_default(),
            pub_date: update
                .date
                .map(|d| d.to_string())
                .unwrap_or_default(),
        }),
        None => Ok(UpdateInfo::none(&current)),
    }
}

/// 下载并安装更新（安装后应用会自动重启）
pub async fn download_and_install(app: &AppHandle) -> Result<()> {
    let updater = app.updater().context("获取 updater 失败")?;
    let check = updater
        .check()
        .await
        .context("检查更新失败")?
        .ok_or_else(|| anyhow!("没有可用的更新"))?;

    log::info!("开始下载更新 v{}...", check.version);
    check
        .download_and_install(
            |chunk_len, content_len| {
                if let Some(total) = content_len {
                    let pct = chunk_len as f64 / total as f64 * 100.0;
                    log::debug!("下载进度: {:.1}%", pct);
                }
            },
            || {
                log::info!("下载完成，准备安装");
            },
        )
        .await
        .context("下载/安装更新失败")?;

    log::info!("更新安装完成，即将重启");
    app.restart();
}
