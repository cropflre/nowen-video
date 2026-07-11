from pathlib import Path

p = Path('internal/config/config.go')
s = p.read_text(encoding='utf-8')
old = '''\t// TMDb API 代理地址（解决国内直连超时问题，如 https://api.tmdb.org 的镜像）
\t// 留空则使用官方地址 https://api.themoviedb.org
\tTMDbAPIProxy string `mapstructure:"tmdb_api_proxy"`
\t// TMDb 图片代理地址（解决国内图片下载超时，如 https://image.tmdb.org 的镜像）
\t// 留空则使用官方地址 https://image.tmdb.org
\tTMDbImageProxy string `mapstructure:"tmdb_image_proxy"`'''
new = '''\t// TMDb API 反向代理 Base URL。程序会自动拼接 /3/...，不是 HTTP forward proxy。
\tTMDbAPIProxy string `mapstructure:"tmdb_api_proxy"`
\t// TMDb 图片反向代理 Base URL。程序会自动拼接 /t/p/...，不是 HTTP forward proxy。
\tTMDbImageProxy string `mapstructure:"tmdb_image_proxy"`
\t// TMDb 网络出口代理，支持 http/https/socks5/socks5h。
\tTMDbNetworkProxy string `mapstructure:"tmdb_network_proxy"`'''
assert s.count(old) == 1
s = s.replace(old, new, 1)
old = '\tviper.SetDefault("secrets.tmdb_image_proxy", "")'
assert s.count(old) == 1
s = s.replace(old, old + '\n\tviper.SetDefault("secrets.tmdb_network_proxy", "")', 1)
needle = '''func (c *Config) SetTMDbImageProxy(proxy string) error {
\tc.mu.Lock()
\tc.Secrets.TMDbImageProxy = proxy
\tc.mu.Unlock()

\tviper.Set("secrets.tmdb_image_proxy", proxy)
\tc.updateSecretsFile("tmdb_image_proxy", proxy)

\treturn c.saveConfig()
}
'''
addition = needle + '''
// GetTMDbNetworkProxy 获取 TMDb HTTP/SOCKS 网络出口代理。
func (c *Config) GetTMDbNetworkProxy() string {
\tc.mu.RLock()
\tdefer c.mu.RUnlock()
\treturn c.Secrets.TMDbNetworkProxy
}

// SetTMDbNetworkProxy 设置网络出口代理并持久化。
func (c *Config) SetTMDbNetworkProxy(proxy string) error {
\tc.mu.Lock()
\tc.Secrets.TMDbNetworkProxy = proxy
\tc.mu.Unlock()
\tviper.Set("secrets.tmdb_network_proxy", proxy)
\tc.updateSecretsFile("tmdb_network_proxy", proxy)
\treturn c.saveConfig()
}
'''
assert s.count(needle) == 1
p.write_text(s.replace(needle, addition, 1), encoding='utf-8')
