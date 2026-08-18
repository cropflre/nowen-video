use serde::{Deserialize, Serialize};
use std::collections::HashMap;

#[derive(Debug, Clone, Deserialize)]
pub struct HighlightCaptureRequest {
    pub url: String,
    pub time: f64,
    #[serde(default)]
    pub headers: HashMap<String, String>,
    #[serde(default = "default_max_width")]
    pub max_width: u32,
}

#[derive(Debug, Clone, Serialize)]
pub struct HighlightCaptureResult {
    pub data_base64: String,
    pub mime: String,
    pub byte_size: usize,
    pub max_width: u32,
}

fn default_max_width() -> u32 {
    640
}

#[tauri::command]
pub async fn highlight_capture_frame(
    request: HighlightCaptureRequest,
) -> Result<HighlightCaptureResult, String> {
    tokio::task::spawn_blocking(move || capture_frame_blocking(request))
        .await
        .map_err(|error| format!("桌面精彩片段抽帧线程异常: {error}"))?
}

#[cfg(feature = "embed-mpv")]
fn sanitize_headers(input: &HashMap<String, String>) -> Result<Vec<String>, String> {
    let mut output = Vec::new();
    for (name, value) in input {
        if !name.eq_ignore_ascii_case("authorization") && !name.eq_ignore_ascii_case("user-agent") {
            return Err(format!("桌面精彩片段不允许透传请求头: {name}"));
        }
        if name.contains('\r') || name.contains('\n') || value.contains('\r') || value.contains('\n') {
            return Err("桌面精彩片段请求头包含非法换行".to_string());
        }
        output.push(format!("{}: {}", name.trim(), value.trim()));
    }
    Ok(output)
}

#[cfg(feature = "embed-mpv")]
fn find_webp(directory: &std::path::Path) -> Result<Option<std::path::PathBuf>, String> {
    let entries = std::fs::read_dir(directory)
        .map_err(|error| format!("读取桌面精彩片段临时目录失败: {error}"))?;
    for entry in entries {
        let path = entry
            .map_err(|error| format!("读取桌面精彩片段临时文件失败: {error}"))?
            .path();
        if path
            .extension()
            .and_then(|value| value.to_str())
            .is_some_and(|value| value.eq_ignore_ascii_case("webp"))
        {
            let len = std::fs::metadata(&path).map(|value| value.len()).unwrap_or(0);
            if len > 0 {
                return Ok(Some(path));
            }
        }
    }
    Ok(None)
}

#[cfg(feature = "embed-mpv")]
fn is_webp(data: &[u8]) -> bool {
    data.len() >= 12 && &data[..4] == b"RIFF" && &data[8..12] == b"WEBP"
}

#[cfg(feature = "embed-mpv")]
fn encode_base64(data: &[u8]) -> String {
    const TABLE: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut output = String::with_capacity(((data.len() + 2) / 3) * 4);
    let mut index = 0;
    while index + 3 <= data.len() {
        let value = ((data[index] as u32) << 16)
            | ((data[index + 1] as u32) << 8)
            | data[index + 2] as u32;
        output.push(TABLE[((value >> 18) & 0x3f) as usize] as char);
        output.push(TABLE[((value >> 12) & 0x3f) as usize] as char);
        output.push(TABLE[((value >> 6) & 0x3f) as usize] as char);
        output.push(TABLE[(value & 0x3f) as usize] as char);
        index += 3;
    }
    match data.len() - index {
        1 => {
            let value = (data[index] as u32) << 16;
            output.push(TABLE[((value >> 18) & 0x3f) as usize] as char);
            output.push(TABLE[((value >> 12) & 0x3f) as usize] as char);
            output.push('=');
            output.push('=');
        }
        2 => {
            let value = ((data[index] as u32) << 16) | ((data[index + 1] as u32) << 8);
            output.push(TABLE[((value >> 18) & 0x3f) as usize] as char);
            output.push(TABLE[((value >> 12) & 0x3f) as usize] as char);
            output.push(TABLE[((value >> 6) & 0x3f) as usize] as char);
            output.push('=');
        }
        _ => {}
    }
    output
}

#[cfg(feature = "embed-mpv")]
struct TempCaptureDir(std::path::PathBuf);

#[cfg(feature = "embed-mpv")]
impl Drop for TempCaptureDir {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

#[cfg(feature = "embed-mpv")]
fn capture_frame_blocking(request: HighlightCaptureRequest) -> Result<HighlightCaptureResult, String> {
    use libmpv2::events::Event;
    use libmpv2::Mpv;
    use std::fs;
    use std::sync::atomic::{AtomicU64, Ordering};
    use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
    use url::Url;

    const MAX_CAPTURE_BYTES: usize = 2 * 1024 * 1024;
    const CAPTURE_TIMEOUT: Duration = Duration::from_secs(30);
    static CAPTURE_SEQUENCE: AtomicU64 = AtomicU64::new(1);

    if !request.time.is_finite() || request.time < 0.0 {
        return Err("精彩片段抽帧时间无效".to_string());
    }
    let max_width = request.max_width.clamp(320, 960);
    let parsed = Url::parse(request.url.trim()).map_err(|_| "精彩片段媒体地址无效".to_string())?;
    if !matches!(parsed.scheme(), "http" | "https") || parsed.host_str().is_none() {
        return Err("桌面计算节点只允许读取 HTTP/HTTPS 媒体流".to_string());
    }

    let headers = sanitize_headers(&request.headers)?;
    let temp_root = std::env::temp_dir().join(format!(
        "nowen-highlight-{}-{}-{}",
        std::process::id(),
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_nanos(),
        CAPTURE_SEQUENCE.fetch_add(1, Ordering::Relaxed),
    ));
    fs::create_dir_all(&temp_root)
        .map_err(|error| format!("创建桌面精彩片段临时目录失败: {error}"))?;
    let _guard = TempCaptureDir(temp_root.clone());
    let output_dir = temp_root
        .to_str()
        .ok_or_else(|| "桌面精彩片段临时目录编码无效".to_string())?
        .to_string();
    let filter = format!("lavfi=[scale={max_width}:-2]");
    let start = format!("+{:.3}", request.time);

    let mut mpv = Mpv::with_initializer(|initializer| {
        initializer.set_option("terminal", "no")?;
        initializer.set_option("audio", "no")?;
        initializer.set_option("sub", "no")?;
        initializer.set_option("osc", "no")?;
        initializer.set_option("vo", "image")?;
        initializer.set_option("vo-image-format", "webp")?;
        initializer.set_option("vo-image-webp-quality", 68_i64)?;
        initializer.set_option("vo-image-webp-compression", 3_i64)?;
        initializer.set_option("vo-image-outdir", output_dir.clone())?;
        initializer.set_option("frames", 1_i64)?;
        initializer.set_option("start", start.clone())?;
        // auto-copy 只选择把硬解帧拷回系统内存的安全模式，既可以利用 GPU，
        // 又能继续经过 scale / image VO，失败时 libmpv 会自动回退软件解码。
        initializer.set_option("hwdec", "auto-copy")?;
        initializer.set_option("vf", filter.clone())?;
        initializer.set_option("msg-level", "all=warn")?;
        Ok(())
    })
    .map_err(|error| format!("初始化桌面 libmpv 抽帧器失败: {error:?}"))?;

    if !headers.is_empty() {
        mpv.set_property("http-header-fields", headers.join("\r\n"))
            .map_err(|error| format!("设置桌面媒体认证请求头失败: {error:?}"))?;
    }

    mpv.command("loadfile", &[parsed.as_str(), "replace"])
        .map_err(|error| format!("桌面 libmpv 加载媒体失败: {error:?}"))?;

    let deadline = Instant::now() + CAPTURE_TIMEOUT;
    let mut playback_ended = false;
    while Instant::now() < deadline {
        if let Some(path) = find_webp(&temp_root)? {
            let bytes = fs::read(&path)
                .map_err(|error| format!("读取桌面精彩片段缩略图失败: {error}"))?;
            if bytes.len() > MAX_CAPTURE_BYTES {
                return Err(format!("桌面精彩片段缩略图过大: {} 字节", bytes.len()));
            }
            if !is_webp(&bytes) {
                return Err("桌面 libmpv 返回的缩略图不是有效 WebP".to_string());
            }
            return Ok(HighlightCaptureResult {
                data_base64: encode_base64(&bytes),
                mime: "image/webp".to_string(),
                byte_size: bytes.len(),
                max_width,
            });
        }

        match mpv.event_context_mut().wait_event(0.20) {
            Some(Ok(Event::EndFile(_))) => playback_ended = true,
            Some(Err(error)) => {
                return Err(format!("桌面 libmpv 抽帧失败: {error:?}"));
            }
            _ => {}
        }

        if playback_ended {
            // image VO 在 EndFile 前后存在非常短的落盘窗口，给文件系统一次刷新机会。
            std::thread::sleep(Duration::from_millis(80));
            if let Some(path) = find_webp(&temp_root)? {
                let bytes = fs::read(&path)
                    .map_err(|error| format!("读取桌面精彩片段缩略图失败: {error}"))?;
                if bytes.len() <= MAX_CAPTURE_BYTES && is_webp(&bytes) {
                    return Ok(HighlightCaptureResult {
                        data_base64: encode_base64(&bytes),
                        mime: "image/webp".to_string(),
                        byte_size: bytes.len(),
                        max_width,
                    });
                }
            }
            return Err("桌面 libmpv 已结束解码，但没有生成有效缩略图".to_string());
        }
    }

    Err("桌面精彩片段抽帧超时".to_string())
}

#[cfg(not(feature = "embed-mpv"))]
fn capture_frame_blocking(_request: HighlightCaptureRequest) -> Result<HighlightCaptureResult, String> {
    Err("当前桌面构建未启用 libmpv，不能作为精彩片段计算节点".to_string())
}
