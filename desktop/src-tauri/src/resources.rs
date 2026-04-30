//! 资源目录解析（M4）
//!
//! 打包后 `resources/` 会被 tauri-build 复制到可执行文件相邻处。
//! 调试模式下则读取 `desktop/src-tauri/resources`（项目源码目录）。
//!
//! 目录结构：
//! ```text
//! resources/
//!   mpv/libmpv-2.dll
//!   shaders/Anime4K_*.glsl   (39 个 V4.0.1 官方 shader)
//!   fonts/                   (字幕渲染字体)
//!   bin/yt-dlp.exe
//! ```

use std::path::{Path, PathBuf};
use std::sync::OnceLock;

static RESOURCE_ROOT: OnceLock<Option<PathBuf>> = OnceLock::new();

/// 获取 resources 根目录（惰性 + 缓存）
pub fn resource_root() -> Option<&'static Path> {
    RESOURCE_ROOT
        .get_or_init(resolve_resource_root)
        .as_deref()
}

/// 查找顺序：
/// 1. 主 exe 同目录下的 `resources/`（NSIS 打包产物）
/// 2. 主 exe 父目录下的 `resources/`（portable / 扁平产物）
/// 3. `CARGO_MANIFEST_DIR/resources/`（dev 模式）
/// 4. 当前工作目录下的 `desktop/src-tauri/resources/`
fn resolve_resource_root() -> Option<PathBuf> {
    // 1 & 2: 跟 exe 走
    if let Ok(exe) = std::env::current_exe() {
        if let Some(dir) = exe.parent() {
            let candidate = dir.join("resources");
            if candidate.exists() {
                return Some(candidate);
            }
            // Tauri 有时会把资源放在 exe 父目录（如 macOS bundle）
            if let Some(parent) = dir.parent() {
                let candidate = parent.join("resources");
                if candidate.exists() {
                    return Some(candidate);
                }
            }
        }
    }

    // 3: 开发模式
    let manifest = PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("resources");
    if manifest.exists() {
        return Some(manifest);
    }

    // 4: 兜底
    let cwd = std::env::current_dir().ok()?.join("desktop/src-tauri/resources");
    if cwd.exists() {
        return Some(cwd);
    }

    None
}

/// shader 目录（包含 Anime4K_*.glsl）
pub fn shader_dir() -> Option<PathBuf> {
    resource_root().map(|r| r.join("shaders"))
}

/// 字体目录（字幕 fallback 字体）
pub fn font_dir() -> Option<PathBuf> {
    resource_root().map(|r| r.join("fonts"))
}

/// 查找某个 shader 的完整路径
#[allow(dead_code)]
pub fn shader_path(name: &str) -> Option<PathBuf> {
    let dir = shader_dir()?;
    let p = dir.join(name);
    p.exists().then_some(p)
}

/// 把多个 shader 拼成 mpv 的 glsl-shaders 属性值
/// mpv 的分隔符：Unix 用 `:`，Windows 用 `;`
pub fn join_shader_paths(names: &[&str]) -> Option<String> {
    let dir = shader_dir()?;
    let mut parts = Vec::with_capacity(names.len());
    for name in names {
        let p = dir.join(name);
        if !p.exists() {
            log::warn!("shader 不存在: {}", p.display());
            continue;
        }
        parts.push(p.to_string_lossy().to_string());
    }
    if parts.is_empty() {
        return None;
    }
    #[cfg(windows)]
    let sep = ";";
    #[cfg(not(windows))]
    let sep = ":";
    Some(parts.join(sep))
}
