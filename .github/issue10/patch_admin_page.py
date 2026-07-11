from pathlib import Path

p = Path('web/src/pages/AdminPage.tsx')
s = p.read_text(encoding='utf-8')
needle = "import StorageTab from '@/components/admin/StorageTab'"
assert s.count(needle) == 1
s = s.replace(needle, needle + "\nimport TMDbProxySettings from '@/components/admin/TMDbProxySettings'", 1)

start = s.index('  // TMDb 代理（API/图片镜像）')
end = s.index('  // 豆瓣 Cookie 配置状态', start)
s = s[:start] + s[end:]

for line in [
    "        setTmdbApiProxy(tmdbRes.data.data?.api_proxy || '')\n",
    "        setTmdbImageProxy(tmdbRes.data.data?.image_proxy || '')\n",
]:
    assert s.count(line) == 1
    s = s.replace(line, '', 1)

start = s.index('  const showTmdbProxyMessage')
end = s.index('  const handleSaveTMDbKey = async () => {', start)
s = s[:start] + s[end:]

start = s.index('                {/* TMDb 代理（API/图片镜像） */}')
end = s.index('                {/* 功能说明 */}', start)
replacement = '''                <TMDbProxySettings
                  config={tmdbConfig}
                  onConfigChange={setTmdbConfig}
                />

'''
s = s[:start] + replacement + s[end:]
p.write_text(s, encoding='utf-8')
