use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

fn main() {
    // ========== Tauri 默认 build ==========
    tauri_build::build();

    // ========== M2: libmpv 动态生成 MSVC 导入库 ==========
    // 当启用 embed-mpv feature 时，自动从 resources/mpv/libmpv-2.dll
    // 解析 PE 导出表 → 生成 mpv.def → 调用 lib.exe 生成 mpv.lib，
    // 并把路径通过 rustc-link-search 告诉 libmpv2-sys。
    #[cfg(feature = "embed-mpv")]
    generate_mpv_import_lib();
}

#[cfg(feature = "embed-mpv")]
fn generate_mpv_import_lib() {
    let manifest_dir = PathBuf::from(env::var("CARGO_MANIFEST_DIR").unwrap());
    let out_dir = PathBuf::from(env::var("OUT_DIR").unwrap());

    let dll_path = manifest_dir.join("resources").join("mpv").join("libmpv-2.dll");
    println!("cargo:rerun-if-changed={}", dll_path.display());
    println!("cargo:rerun-if-changed=build.rs");

    if !dll_path.exists() {
        println!(
            "cargo:warning=libmpv-2.dll 不存在于 {:?}，请运行 scripts/fetch-assets.ps1",
            dll_path
        );
        // 不 panic，让 cargo check 能过，构建期错误由 rustc 抛
        println!("cargo:rustc-link-search=native={}", out_dir.display());
        return;
    }

    // 只在 Windows MSVC 工具链下生成 .lib；其它平台假定系统已装 libmpv
    let target = env::var("TARGET").unwrap_or_default();
    if !target.contains("windows") || !target.contains("msvc") {
        return;
    }

    let lib_out = out_dir.join("mpv.lib");
    let def_out = out_dir.join("mpv.def");

    // 如果 .lib 比 .dll 新，且 .def 存在 → 跳过
    if lib_out.exists() && def_out.exists() {
        if let (Ok(lib_meta), Ok(dll_meta)) = (fs::metadata(&lib_out), fs::metadata(&dll_path)) {
            if let (Ok(lib_mtime), Ok(dll_mtime)) = (lib_meta.modified(), dll_meta.modified()) {
                if lib_mtime >= dll_mtime {
                    println!("cargo:rustc-link-search=native={}", out_dir.display());
                    return;
                }
            }
        }
    }

    println!(
        "cargo:warning=从 libmpv-2.dll 生成 MSVC 导入库 mpv.lib ..."
    );

    // 1. 读 DLL 导出表，生成 mpv.def
    let exports = parse_dll_exports(&dll_path).expect("读取 libmpv-2.dll 导出表失败");
    let mut def = String::new();
    def.push_str("LIBRARY libmpv-2.dll\r\nEXPORTS\r\n");
    for name in &exports {
        def.push_str(&format!("    {}\r\n", name));
    }
    fs::write(&def_out, def).expect("写 mpv.def 失败");

    // 2. 调 lib.exe /def:mpv.def /out:mpv.lib /machine:x64
    let arch = if target.contains("x86_64") {
        "x64"
    } else if target.contains("aarch64") {
        "arm64"
    } else if target.contains("i686") {
        "x86"
    } else {
        "x64"
    };

    let lib_exe = find_msvc_tool("lib.exe").expect(
        "未找到 lib.exe（MSVC 工具链）。请在 x64 Developer Command Prompt 中构建，\
         或安装 Visual Studio 2022 Build Tools 的 C++ 组件",
    );

    let status = Command::new(&lib_exe)
        .arg(format!("/def:{}", def_out.display()))
        .arg(format!("/out:{}", lib_out.display()))
        .arg(format!("/machine:{}", arch))
        .arg("/nologo")
        .status()
        .expect("调用 lib.exe 失败");

    if !status.success() {
        panic!("lib.exe 生成 mpv.lib 失败（退出码 {:?}）", status.code());
    }

    println!("cargo:warning=生成完成: {}", lib_out.display());
    println!("cargo:rustc-link-search=native={}", out_dir.display());
}

/// 用 cc crate 机制定位 MSVC 工具链中的 lib.exe / dumpbin.exe
#[cfg(feature = "embed-mpv")]
fn find_msvc_tool(tool: &str) -> Option<PathBuf> {
    // 先看 PATH
    if let Ok(paths) = env::var("PATH") {
        for p in env::split_paths(&paths) {
            let cand = p.join(tool);
            if cand.exists() {
                return Some(cand);
            }
        }
    }
    // 通过 cc crate 询问 MSVC 工具链目录
    let target = env::var("TARGET").unwrap_or_default();
    if let Ok(tool_ref) = cc::windows_registry::find_tool(&target, "link.exe")
        .ok_or("link.exe not found")
    {
        // link.exe 所在目录通常也有 lib.exe
        if let Some(dir) = tool_ref.path().parent() {
            let cand = dir.join(tool);
            if cand.exists() {
                return Some(cand);
            }
        }
        // env 里会有完整 PATH
        for (k, v) in tool_ref.env() {
            if k.to_string_lossy().eq_ignore_ascii_case("PATH") {
                for p in env::split_paths(v) {
                    let cand = p.join(tool);
                    if cand.exists() {
                        return Some(cand);
                    }
                }
            }
        }
    }
    None
}

/// 解析 PE DLL 的导出表，返回导出符号名列表
#[cfg(feature = "embed-mpv")]
fn parse_dll_exports(dll: &Path) -> Option<Vec<String>> {
    // 轻量手写 PE parser（避免新增 build-dependency）
    let data = fs::read(dll).ok()?;
    if data.len() < 0x40 {
        return None;
    }
    // DOS header: MZ
    if &data[0..2] != b"MZ" {
        return None;
    }
    let pe_offset = u32::from_le_bytes(data[0x3C..0x40].try_into().ok()?) as usize;
    if pe_offset + 24 > data.len() || &data[pe_offset..pe_offset + 4] != b"PE\0\0" {
        return None;
    }

    // COFF header 紧接 "PE\0\0" 4 字节
    let coff = pe_offset + 4;
    let num_sections = u16::from_le_bytes(data[coff + 2..coff + 4].try_into().ok()?) as usize;
    let opt_header_size =
        u16::from_le_bytes(data[coff + 16..coff + 18].try_into().ok()?) as usize;
    let opt_header = coff + 20;

    // PE32+ magic = 0x20B
    let magic = u16::from_le_bytes(data[opt_header..opt_header + 2].try_into().ok()?);
    let is_pe32_plus = magic == 0x20B;

    // Data Directories 偏移：PE32 = 96, PE32+ = 112
    let dd_offset = opt_header + if is_pe32_plus { 112 } else { 96 };
    // Export Directory RVA = DataDirectory[0]
    let export_rva =
        u32::from_le_bytes(data[dd_offset..dd_offset + 4].try_into().ok()?) as usize;
    let export_size =
        u32::from_le_bytes(data[dd_offset + 4..dd_offset + 8].try_into().ok()?) as usize;
    if export_rva == 0 || export_size == 0 {
        return None;
    }

    // 读 section headers，建立 RVA→文件偏移映射
    let sec_start = opt_header + opt_header_size;
    let mut sections = Vec::with_capacity(num_sections);
    for i in 0..num_sections {
        let s = sec_start + i * 40;
        if s + 40 > data.len() {
            return None;
        }
        let virt_size = u32::from_le_bytes(data[s + 8..s + 12].try_into().ok()?) as usize;
        let virt_addr = u32::from_le_bytes(data[s + 12..s + 16].try_into().ok()?) as usize;
        let raw_ptr = u32::from_le_bytes(data[s + 20..s + 24].try_into().ok()?) as usize;
        sections.push((virt_addr, virt_size, raw_ptr));
    }
    let rva_to_off = |rva: usize| -> Option<usize> {
        for &(va, vs, rp) in &sections {
            if rva >= va && rva < va + vs.max(1) {
                return Some(rp + (rva - va));
            }
        }
        None
    };

    let exp_off = rva_to_off(export_rva)?;
    if exp_off + 40 > data.len() {
        return None;
    }
    // Export Directory Table 布局
    let num_names =
        u32::from_le_bytes(data[exp_off + 24..exp_off + 28].try_into().ok()?) as usize;
    let name_table_rva =
        u32::from_le_bytes(data[exp_off + 32..exp_off + 36].try_into().ok()?) as usize;
    let name_table_off = rva_to_off(name_table_rva)?;

    let mut names = Vec::with_capacity(num_names);
    for i in 0..num_names {
        let p = name_table_off + i * 4;
        if p + 4 > data.len() {
            break;
        }
        let name_rva = u32::from_le_bytes(data[p..p + 4].try_into().ok()?) as usize;
        if let Some(name_off) = rva_to_off(name_rva) {
            // C 字符串读到 '\0'
            let mut end = name_off;
            while end < data.len() && data[end] != 0 {
                end += 1;
            }
            if let Ok(s) = std::str::from_utf8(&data[name_off..end]) {
                names.push(s.to_string());
            }
        }
    }
    Some(names)
}
