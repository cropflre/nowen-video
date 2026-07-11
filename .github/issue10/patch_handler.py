from pathlib import Path

p = Path('internal/handler/admin_metadata.go')
s = p.read_text(encoding='utf-8')
old = '''\t\t"data": gin.H{
\t\t\t"configured":  configured,
\t\t\t"masked_key":  maskedKey,
\t\t\t"api_proxy":   h.cfg.GetTMDbAPIProxy(),
\t\t\t"image_proxy": h.cfg.GetTMDbImageProxy(),
\t\t},'''
new = '''\t\t"data": gin.H{
\t\t\t"configured":                configured,
\t\t\t"masked_key":                maskedKey,
\t\t\t"api_proxy":                 h.cfg.GetTMDbAPIProxy(),
\t\t\t"image_proxy":               h.cfg.GetTMDbImageProxy(),
\t\t\t"network_proxy":             h.cfg.GetTMDbNetworkProxy(),
\t\t\t"api_proxy_base_url":        h.cfg.GetTMDbAPIProxy(),
\t\t\t"image_proxy_base_url":      h.cfg.GetTMDbImageProxy(),
\t\t\t"network_proxy_configured": h.cfg.GetTMDbNetworkProxy() != "",
\t\t},'''
assert s.count(old) == 1
s = s.replace(old, new, 1)
start = s.index('// ==================== TMDb 代理配置 ====================')
end = s.index('// ==================== 手动元数据匹配 ====================', start)
s = s[:start] + '// TMDb 代理配置实现见 admin_tmdb_proxy.go\n\n' + s[end:]
p.write_text(s, encoding='utf-8')
