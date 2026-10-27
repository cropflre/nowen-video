import { useEffect, useRef, useState } from 'react'
import { adminApi } from '@/api'
import type { InviteCode } from '@/types'
import { Button, Section } from '@/components/design-system'
import { useToast } from '@/components/Toast'
import { useDesktop } from '@/desktop'

export default function FamilyInvite() {
  const [invites, setInvites] = useState<InviteCode[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [busy, setBusy] = useState(false)
  const [selectedLink, setSelectedLink] = useState('')
  const busyRef = useRef(false)
  const toast = useToast()
  const runtime = useDesktop()
  const load = async () => {
    setLoading(true); setError(false)
    try { setInvites((await adminApi.listInviteCodes()).data.data || []) }
    catch { setError(true) }
    finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [])
  const linkFor = (code: string) => {
    const base = runtime.isDesktop ? runtime.serverBaseUrl : window.location.origin
    if (!base || !/^https?:\/\//.test(base)) return ''
    const url = new URL('/login', base)
    url.hash = new URLSearchParams({ invite: code }).toString()
    return url.toString()
  }
  const create = async () => {
    if (busyRef.current) return
    busyRef.current = true; setBusy(true)
    try {
      const invite = (await adminApi.createInviteCode({ max_uses: 1, expires_in_hours: 24, note: '家庭邀请' })).data.data
      setInvites((items) => [invite, ...items])
      setSelectedLink(linkFor(invite.code))
      toast.success('邀请已创建，24 小时内可使用一次')
    } catch { toast.error('未能创建邀请，请重试') }
    finally { busyRef.current = false; setBusy(false) }
  }
  const revoke = async (invite: InviteCode) => {
    if (busyRef.current) return
    busyRef.current = true; setBusy(true)
    try {
      await adminApi.deleteInviteCode(invite.id)
      setInvites((items) => items.filter((item) => item.id !== invite.id))
      if (selectedLink === linkFor(invite.code)) setSelectedLink('')
      toast.success('邀请已撤销')
    } catch { toast.error('未能撤销邀请，请重试') }
    finally { busyRef.current = false; setBusy(false) }
  }
  const active = invites.filter((invite) => invite.note === '家庭邀请' && (!invite.max_uses || invite.used_count < invite.max_uses) && (!invite.expires_at || Date.parse(invite.expires_at) > Date.now()))
  return (
    <Section title="邀请家人" description="家人打开链接后设置自己的账号，观看记录各自保存。邀请不会开放公开注册。">
      <div className="space-y-3">
        <Button onClick={() => void create()} disabled={busy || loading} variant="primary">创建家庭邀请</Button>
        {error && <div role="alert">暂时无法读取邀请。<Button onClick={() => void load()}>重试</Button></div>}
        {selectedLink && <div className="space-y-2">
          <label className="block text-sm">家庭邀请链接<input readOnly value={selectedLink} onFocus={(event) => event.target.select()} className="nv-input mt-1 w-full" aria-label="家庭邀请链接" /></label>
          <Button onClick={() => void (navigator.clipboard?.writeText(selectedLink) || Promise.reject()).then(() => toast.success('邀请链接已复制')).catch(() => toast.info('请选中上方链接后复制'))}>复制链接</Button>
          <p className="text-sm text-[var(--nv-text-tertiary)]">家人的设备需要能访问这个地址。局域网地址只适用于家中网络。</p>
        </div>}
        {active.map((invite) => <div key={invite.id} className="flex flex-wrap items-center justify-between gap-2 border-b border-[var(--nv-border-subtle)] py-2">
          <span className="text-sm">{invite.expires_at ? `${new Date(invite.expires_at).toLocaleString()} 前可加入` : '等待家人加入'}</span>
          <div className="flex gap-2"><Button disabled={busy} onClick={() => setSelectedLink(linkFor(invite.code))}>查看链接</Button><Button disabled={busy} onClick={() => void revoke(invite)}>撤销邀请</Button></div>
        </div>)}
      </div>
    </Section>
  )
}
