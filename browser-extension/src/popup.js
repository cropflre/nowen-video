const $ = (id) => document.getElementById(id);

let currentMetadata = null;
let statusTimer = null;

const defaults = {
  serverUrl: "http://127.0.0.1:8080",
  username: "admin",
  token: "",
  videoPath: "",
};

const setStatus = (message, data) => {
  $("status").textContent = data ? `${message}\n${JSON.stringify(data, null, 2)}` : message;
};

const setMediaStatus = (message, kind = "idle") => {
  const el = $("mediaStatus");
  el.textContent = message;
  el.className = `media-status ${kind}`;
};

const normalizeServerUrl = (value) => (value || defaults.serverUrl).replace(/\/+$/, "");

const saveSettings = async () => {
  await chrome.storage.local.set({
    serverUrl: normalizeServerUrl($("serverUrl").value),
    username: $("username").value.trim(),
    token: defaults.token,
    videoPath: $("videoPath").value.trim(),
  });
};

const loadSettings = async () => {
  const stored = await chrome.storage.local.get(defaults);
  defaults.token = stored.token || "";
  $("serverUrl").value = stored.serverUrl || defaults.serverUrl;
  $("username").value = stored.username || defaults.username;
  $("videoPath").value = stored.videoPath || "";
};

const activeTab = async () => {
  const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
  if (!tab?.id) throw new Error("没有可用的当前标签页");
  return tab;
};

const renderSummary = (metadata) => {
  currentMetadata = metadata;
  $("siteBadge").textContent = metadata.source || "已读取";
  $("summary").classList.remove("empty");
  $("summary").innerHTML = `
    <strong>${escapeHtml(metadata.code || "")} ${escapeHtml(metadata.title || "")}</strong>
    <dl>
      <dt>来源</dt><dd>${escapeHtml(metadata.source || "")}</dd>
      <dt>发行日</dt><dd>${escapeHtml(metadata.release_date || "")}</dd>
      <dt>片商</dt><dd>${escapeHtml(metadata.studio || "")}</dd>
      <dt>演员</dt><dd>${escapeHtml((metadata.actresses || []).join(", "))}</dd>
      <dt>标签</dt><dd>${escapeHtml((metadata.genres || []).join(", "))}</dd>
      <dt>剧照</dt><dd>${metadata.extra_fanart?.length || 0} 张</dd>
      <dt>短评</dt><dd>${metadata.short_reviews?.length || 0} 条</dd>
    </dl>
  `;
};

const escapeHtml = (value) =>
  String(value || "")
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");

const login = async () => {
  const serverUrl = normalizeServerUrl($("serverUrl").value);
  const username = $("username").value.trim();
  const password = $("password").value;
  if (!username || !password) throw new Error("请输入用户名和密码");

  const response = await fetch(`${serverUrl}/api/auth/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username, password }),
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `登录失败 HTTP ${response.status}`);
  defaults.token = body.token || body.data?.token || "";
  if (!defaults.token) throw new Error("登录响应中没有 token");
  await saveSettings();
  $("password").value = "";
  setStatus("已登录本地 Nowen 服务。");
  await checkMediaStatus();
};

const extractCurrentPage = async () => {
  const tab = await activeTab();
  const response = await chrome.tabs.sendMessage(tab.id, { type: "NOWEN_EXTRACT_ADULT_METADATA" });
  if (!response?.ok) throw new Error(response?.error || "读取当前页失败");
  renderSummary(response.metadata);
  await saveSettings();
  setStatus("当前页元数据已读取。");
};

const importToNowen = async () => {
  if (!currentMetadata) throw new Error("请先读取当前页面");
  const serverUrl = normalizeServerUrl($("serverUrl").value);
  const videoPath = $("videoPath").value.trim();
  if (!videoPath) throw new Error("请输入本机视频文件路径");
  if (!defaults.token) throw new Error("请先登录本地 Nowen 服务");

  await saveSettings();
  const response = await fetch(`${serverUrl}/api/admin/adult-scraper/browser/import`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${defaults.token}`,
    },
    body: JSON.stringify({
      video_path: videoPath,
      metadata: currentMetadata,
      short_reviews: currentMetadata.short_reviews || [],
    }),
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `写入失败 HTTP ${response.status}`);
  setStatus("已写入 NFO/图片。", body.data);
  await checkMediaStatus();
};

const checkMediaStatus = async () => {
  const serverUrl = normalizeServerUrl($("serverUrl").value);
  const videoPath = $("videoPath").value.trim();
  if (!videoPath) {
    setMediaStatus("未检查刮削状态。");
    return;
  }
  if (!defaults.token) {
    setMediaStatus("未登录，无法检查本地 NFO。", "warn");
    return;
  }

  const response = await fetch(
    `${serverUrl}/api/admin/adult-scraper/browser/status?path=${encodeURIComponent(videoPath)}`,
    { headers: { Authorization: `Bearer ${defaults.token}` } },
  );
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `状态检查失败 HTTP ${response.status}`);

  const data = body.data || {};
  if (data.needs_scrape) {
    setMediaStatus("未刮削，需要刮削", "warn");
  } else {
    const count = data.directory_nfos?.length || 0;
    setMediaStatus(count > 1 ? `已刮削，目录内有 ${count} 个 NFO` : "已刮削，已找到 NFO", "ok");
  }
};

const scheduleMediaStatusCheck = () => {
  clearTimeout(statusTimer);
  statusTimer = setTimeout(() => {
    saveSettings();
    checkMediaStatus().catch((error) => setMediaStatus(error.message || String(error), "error"));
  }, 350);
};

const run = async (button, task) => {
  const oldText = button.textContent;
  button.disabled = true;
  button.textContent = "处理中";
  try {
    await task();
  } catch (error) {
    setStatus(error.message || String(error));
  } finally {
    button.disabled = false;
    button.textContent = oldText;
  }
};

document.addEventListener("DOMContentLoaded", async () => {
  await loadSettings();
  $("loginBtn").addEventListener("click", () => run($("loginBtn"), login));
  $("extractBtn").addEventListener("click", () => run($("extractBtn"), extractCurrentPage));
  $("importBtn").addEventListener("click", () => run($("importBtn"), importToNowen));
  $("serverUrl").addEventListener("change", saveSettings);
  $("username").addEventListener("change", saveSettings);
  $("videoPath").addEventListener("input", scheduleMediaStatusCheck);
  checkMediaStatus().catch((error) => setMediaStatus(error.message || String(error), "error"));
});
