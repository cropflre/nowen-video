from pathlib import Path

p = Path('web/src/types/index.ts')
s = p.read_text(encoding='utf-8')
old = '''export interface TMDbConfigStatus {
  configured: boolean
  masked_key: string
  api_proxy?: string
  image_proxy?: string
}'''
new = '''export interface TMDbConfigStatus {
  configured: boolean
  masked_key: string
  /** API 反向代理 Base URL，程序自动拼接 /3/... */
  api_proxy?: string
  /** 图片反向代理 Base URL，程序自动拼接 /t/p/... */
  image_proxy?: string
  /** HTTP/HTTPS/SOCKS5 网络出口代理 */
  network_proxy?: string
  network_proxy_configured?: boolean
  api_proxy_base_url?: string
  image_proxy_base_url?: string
}'''
assert s.count(old) == 1
p.write_text(s.replace(old, new, 1), encoding='utf-8')

p = Path('web/src/api/admin.ts')
s = p.read_text(encoding='utf-8')
start = s.index('  // 更新 TMDb 代理（API/图片）。任一字段传空表示恢复官方直连')
end = s.index('  // 批量操作', start)
block = '''  // 更新 TMDb 连接配置：反向代理 Base URL + HTTP/SOCKS 网络出口代理
  updateTMDbProxy: (apiProxy: string, imageProxy: string, networkProxy: string) =>
    api.put<{ message: string; data: { api_proxy: string; image_proxy: string; network_proxy: string } }>(
      '/admin/settings/tmdb/proxy',
      { api_proxy: apiProxy, image_proxy: imageProxy, network_proxy: networkProxy },
    ),

  clearTMDbProxy: () =>
    api.delete<{ message: string; data: { api_proxy: string; image_proxy: string; network_proxy: string } }>(
      '/admin/settings/tmdb/proxy',
    ),

  testTMDbProxy: (apiProxy: string, imageProxy: string, networkProxy: string) =>
    api.post<{ data: {
      api: { ok: boolean; message: string; target: string }
      image: { ok: boolean; message: string; target: string }
      network: { ok: boolean; configured: boolean; message: string; target: string }
    } }>('/admin/settings/tmdb/proxy/test', {
      api_proxy: apiProxy,
      image_proxy: imageProxy,
      network_proxy: networkProxy,
    }),

'''
p.write_text(s[:start] + block + s[end:], encoding='utf-8')
