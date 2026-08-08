let observer: MutationObserver | null = null

function activateBlobSubtitleTrack(track: HTMLTrackElement) {
  if (!track.isConnected) return
  if (track.kind !== 'subtitles' && track.kind !== 'captions') return
  if (!track.src.startsWith('blob:')) return

  // 新创建的 HTMLTrackElement 默认 mode=disabled。Chromium 在 disabled 状态下
  // 可以不加载 track.src；如果调用方等 load 事件后才切 showing，就会形成死锁。
  // Nowen 的浏览器字幕轨道均以 blob: WebVTT 注入，所以在插入 DOM 后立即启用它。
  if (track.track.mode === 'disabled') {
    track.track.mode = 'showing'
  }
}

function scanAddedNode(node: Node) {
  if (node instanceof HTMLTrackElement) {
    activateBlobSubtitleTrack(node)
    return
  }
  if (!(node instanceof Element)) return
  node.querySelectorAll<HTMLTrackElement>('track[src^="blob:"]').forEach(activateBlobSubtitleTrack)
}

/**
 * 浏览器字幕轨道激活兼容层。
 *
 * VideoPlayer 和在线字幕面板都会动态插入 blob: WebVTT <track>。某些 Chromium
 * 版本不会为 mode=disabled 的轨道触发 load，因此统一在 DOM 插入后立即切 showing。
 * 只处理 blob: subtitles/captions，不干预 hls.js 自己维护的 TextTrack。
 */
export function installSubtitleTrackActivationGuard() {
  if (observer || typeof document === 'undefined' || typeof MutationObserver === 'undefined') return

  document.querySelectorAll<HTMLTrackElement>('video > track[src^="blob:"]').forEach(activateBlobSubtitleTrack)

  observer = new MutationObserver((records) => {
    for (const record of records) {
      record.addedNodes.forEach(scanAddedNode)
    }
  })
  observer.observe(document.documentElement, { childList: true, subtree: true })
}
