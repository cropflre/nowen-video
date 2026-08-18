//! Desktop 2.0 资源目录解析。
//!
//! 首发 Player Core 只在 Rust 层直接读取字幕 fallback 字体；libmpv 动态库由
//! app/bootstrap 在进程启动阶段独立定位。Anime4K/Shader 已移出 2.0 首发范围。

use std::path::{Path, PathBuf};
use std::sync::OnceLock;

static RESOURCE_ROOT: OnceLock<Option<PathBuf>> = OnceLock::new();

pub fn resource_root() -> Option<&'static Path> {
    RESOURCE_ROOT.get_or_init(resolve_resource_root).as_deref()
}

fn resolve_resource_root() -> Option<PathBuf> {
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            let candidate = dir.join("resources");
            if candidate.exists() {
                return Some(candidate);
            }
            if let Some(parent) = dir.parent() {
                let candidate = parent.join("resources");
                if candidate.exists() {
                    return Some(candidate);
                }
            }
        }
    }

    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("resources");
    if manifest.exists() {
        return Some(manifest);
    }

    let cwd = std::env::current_dir().ok()?.join("desktop/src-tauri/resources");
    if cwd.exists() {
        return Some(cwd);
    }

    None
}

/// 字幕 fallback 字体目录。
pub fn font_dir() -> Option<PathBuf> {
    resource_root().map(|root| root.join("fonts"))
}
