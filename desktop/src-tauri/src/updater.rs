//! Desktop 2.0 自动更新。
//!
//! 更新签名密钥完成正式配置前默认关闭。发布流水线显式设置
//! NOWEN_DESKTOP_UPDATER_ENABLED=1 后才允许检查/安装更新，避免空公钥配置进入正式运行链路。

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

pub fn enabled() -> bool {
    option_env!("NOWEN_DESKTOP_UPDATER_ENABLED") == Some("1")
}

pub async fn check(app: &AppHandle) -> Result<UpdateInfo> {
    let current = app.package_info().version.to_string();
    if !enabled() {
        return Ok(UpdateInfo::none(&current));
    }

    let updater = app
        .updater()
        .context("获取 updater 失败（请检查正式更新端点与签名配置）")?;

    match updater.check().await.context("检查更新失败")? {
        Some(update) => Ok(UpdateInfo {
            available: true,
            version: update.version.clone(),
            current_version: current,
            notes: update.body.clone().unwrap_or_default(),
            pub_date: update.date.map(|date| date.to_string()).unwrap_or_default(),
        }),
        None => Ok(UpdateInfo::none(&current)),
    }
}

pub async fn download_and_install(app: &AppHandle) -> Result<()> {
    if !enabled() {
        return Err(anyhow!("Desktop 自动更新尚未启用正式签名配置"));
    }

    let updater = app.updater().context("获取 updater 失败")?;
    let update = updater
        .check()
        .await
        .context("检查更新失败")?
        .ok_or_else(|| anyhow!("没有可用的更新"))?;

    log::info!("开始下载 Desktop 更新 v{}", update.version);
    update
        .download_and_install(
            |chunk_len, content_len| {
                if let Some(total) = content_len {
                    let percent = chunk_len as f64 / total as f64 * 100.0;
                    log::debug!("更新下载进度: {:.1}%", percent);
                }
            },
            || log::info!("Desktop 更新下载完成，准备安装"),
        )
        .await
        .context("下载/安装更新失败")?;

    app.restart();
}
