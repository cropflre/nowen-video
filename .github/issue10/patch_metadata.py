from pathlib import Path

p = Path('internal/service/metadata.go')
s = p.read_text(encoding='utf-8')
needle = '''\ttransport := &http.Transport{
\t\tDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {'''
replacement = '''\ttransport := &http.Transport{
\t\tProxy: tmdbNetworkProxyFunc(cfg),
\t\tDialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {'''
assert s.count(needle) == 1
s = s.replace(needle, replacement, 1)
needle = '''\tlogger.Infof("TMDb HTTP 客户端已初始化 (API代理: %s, 图片代理: %s)",
\t\tdefaultIfEmpty(cfg.Secrets.TMDbAPIProxy, "官方直连"),
\t\tdefaultIfEmpty(cfg.Secrets.TMDbImageProxy, "官方直连"))'''
replacement = '''\tlogger.Infof("TMDb HTTP 客户端已初始化 (API反代: %s, 图片反代: %s, 网络出口: %s)",
\t\tdefaultIfEmpty(cfg.Secrets.TMDbAPIProxy, "官方地址"),
\t\tdefaultIfEmpty(cfg.Secrets.TMDbImageProxy, "官方地址"),
\t\tdefaultIfEmpty(proxyDisplayURL(cfg.Secrets.TMDbNetworkProxy), "直接连接"))'''
assert s.count(needle) == 1
p.write_text(s.replace(needle, replacement, 1), encoding='utf-8')
