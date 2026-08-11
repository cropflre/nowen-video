// 番号刮削 - Cookie 登录管理（参考 mdcx）
// 功能：为每个站点配置完整 Cookie 字符串，解锁登录态刮削。
import { useCallback, useEffect, useState } from 'react'
import { adultScraperApi } from '@/api'
import { Cookie, Save, TestTube2, CheckCircle2, AlertCircle, Loader2, Eye, EyeOff } from 'lucide-react'
import { useDialog } from '@/components/Dialog'
import { Button, Surface, Textarea } from '@/components/design-system'

interface SiteDef {
  id: string
  name: string
  home: string
  tips: string
}

const SITES: SiteDef[] = [
  { id: 'javbus', name: 'JavBus', home: 'https://www.javbus.com/', tips: '登录后可访问无码区（havf/uncensored）' },
  { id: 'javdb', name: 'JavDB', home: 'https://javdb.com/', tips: '强烈建议配置。登录后：高清封面、完整演员、评分、预告片全部可用' },
  { id: 'freejavbt', name: 'Freejavbt', home: 'https://freejavbt.com/', tips: '可选，偶发性 CF 验证时建议设置' },
  { id: 'jav321', name: 'JAV321', home: 'https://www.jav321.com/', tips: '可选，一般无需 Cookie' },
  { id: 'fanza', name: 'Fanza (DMM)', home: 'https://www.dmm.co.jp/', tips: '访问日本区资源建议配置 DMM 账号 Cookie' },
  { id: 'mgstage', name: 'MGStage', home: 'https://www.mgstage.com/', tips: '素人番号（MGS / 200GANA）建议登录态' },
  { id: 'fc2hub', name: 'FC2Hub', home: 'https://fc2hub.com/', tips: 'FC2-PPV 番号可选登录态' },
]

type CookieMap = Record<string, string>

export default function AdultCookieLoginPanel() {
  const dialog = useDialog()
  const [cookies, setCookies] = useState<CookieMap>({})
  const [visible, setVisible] = useState<Record<string, boolean>>({})
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState<string>('')
  const [testResults, setTestResults] = useState<Record<string, { ok: boolean; message: string }>>({})
  const [loaded, setLoaded] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await adultScraperApi.getConfig()
      const c = res.data.data.cookies || {}
      setCookies({
        javbus: c.javbus || '', javdb: c.javdb || '', freejavbt: c.freejavbt || '',
        jav321: c.jav321 || '', fanza: c.fanza || '', mgstage: c.mgstage || '', fc2hub: c.fc2hub || '',
      })
    } finally {
      setLoaded(true)
    }
  }, [])

  useEffect(() => { void load() }, [load])

  const handleSave = async () => {
    setSaving(true)
    try {
      await adultScraperApi.updateConfig({
        cookie_javbus: cookies.javbus,
        cookie_javdb: cookies.javdb,
        cookie_freejavbt: cookies.freejavbt,
        cookie_jav321: cookies.jav321,
        cookie_fanza: cookies.fanza,
        cookie_mgstage: cookies.mgstage,
        cookie_fc2hub: cookies.fc2hub,
      })
      await dialog.alert({ title: 'Cookie 已保存', variant: 'success' })
    } catch (err: any) {
      await dialog.alert({ title: '保存失败', message: err?.response?.data?.error || err?.message, variant: 'error' })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async (siteId: string) => {
    setTesting(siteId)
    try {
      const res = await adultScraperApi.testCookie(siteId)
      const { ok, message } = res.data.data
      setTestResults((current) => ({ ...current, [siteId]: { ok, message } }))
    } catch (err: any) {
      setTestResults((current) => ({
        ...current,
        [siteId]: { ok: false, message: err?.response?.data?.error || err?.message || '测试失败' },
      }))
    } finally {
      setTesting('')
    }
  }

  return (
    <div className="space-y-4">
      <Surface className="p-4">
        <div className="mb-1 flex items-center gap-2 text-sm font-medium text-[var(--nv-text-primary)]">
          <Cookie className="h-4 w-4 text-[var(--nv-status-warning)]" aria-hidden="true" />
          Cookie 登录（参考 mdcx 设计）
        </div>
        <p className="text-xs leading-5 text-[var(--nv-text-secondary)]">
          为每个站点配置登录态 Cookie 可解锁：<strong className="text-[var(--nv-text-primary)]">高清封面、完整演员、评分、预告片、无码区资源</strong>等。
        </p>
        <p className="mt-1 text-xs leading-5 text-[var(--nv-text-secondary)]">
          <strong>获取方法</strong>：浏览器登录站点 → <kbd className="rounded bg-[var(--nv-bg-control)] px-1">F12</kbd> → Network → 任一请求 → 复制 <code>Cookie:</code> 头的完整字符串（不要加 “Cookie: ” 前缀）
        </p>
      </Surface>

      <div className="space-y-3">
        {SITES.map((site) => {
          const value = cookies[site.id] || ''
          const show = visible[site.id] || false
          const result = testResults[site.id]
          return (
            <Surface key={site.id} className="p-4">
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium text-[var(--nv-text-primary)]">{site.name}</span>
                <a href={site.home} target="_blank" rel="noreferrer" className="text-xs text-[var(--nv-action-primary)] hover:underline">{site.home}</a>
                <span className="ml-auto text-xs text-[var(--nv-text-tertiary)]">{value ? `${value.length} 字符` : '未设置'}</span>
              </div>
              <div className="mb-2 text-xs text-[var(--nv-text-tertiary)]">💡 {site.tips}</div>
              <div className="flex gap-2">
                <Textarea
                  rows={2}
                  value={show ? value : value ? '•'.repeat(Math.min(value.length, 60)) : ''}
                  readOnly={!show}
                  onChange={(event) => setCookies((current) => ({ ...current, [site.id]: event.target.value }))}
                  placeholder={loaded ? '粘贴完整 Cookie 字符串，如 _jdb_session=xxx; locale=zh; ...' : '加载中...'}
                  className="flex-1 resize-y font-mono text-xs"
                />
                <div className="flex flex-col gap-1">
                  <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setVisible((current) => ({ ...current, [site.id]: !show }))} title={show ? '隐藏' : '显示'}>
                    {show ? <EyeOff className="h-3.5 w-3.5" aria-hidden="true" /> : <Eye className="h-3.5 w-3.5" aria-hidden="true" />}
                  </Button>
                  <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => handleTest(site.id)} disabled={testing === site.id} title="测试连通性">
                    {testing === site.id ? <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" /> : <TestTube2 className="h-3.5 w-3.5" aria-hidden="true" />}
                  </Button>
                </div>
              </div>
              {result && (
                <div className={`mt-2 flex items-start gap-1 text-xs ${result.ok ? 'text-[var(--nv-status-success)]' : 'text-[var(--nv-status-danger)]'}`}>
                  {result.ok ? <CheckCircle2 className="mt-0.5 h-3 w-3" aria-hidden="true" /> : <AlertCircle className="mt-0.5 h-3 w-3" aria-hidden="true" />}
                  <span>{result.message}</span>
                </div>
              )}
            </Surface>
          )
        })}
      </div>

      <div className="flex items-center justify-end gap-2">
        <Button type="button" variant="secondary" onClick={() => void load()}>重新加载</Button>
        <Button type="button" variant="primary" onClick={handleSave} disabled={saving || !loaded} loading={saving}>
          {!saving && <Save className="h-4 w-4" aria-hidden="true" />}
          保存所有 Cookie
        </Button>
      </div>
    </div>
  )
}
