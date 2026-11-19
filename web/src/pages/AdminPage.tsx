import { useState } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { ChevronLeft, ChevronRight, FolderOpen, HardDrive, MonitorPlay, Settings, ShieldAlert, Sparkles, Stethoscope, Users } from 'lucide-react'
import AdminLibraries from '@/components/admin/AdminLibraries'
import AdvancedSystemSettings from '@/components/admin/AdvancedSystemSettings'
import AITab from '@/components/admin/AITab'
import { AdminPageHeader } from '@/components/admin/AdminPrimitives'
import MetadataSettings from '@/components/admin/MetadataSettings'
import StorageTab from '@/components/admin/StorageTab'
import UsersTab from '@/components/admin/UsersTab'
import FamilyInvite from '@/components/FamilyInvite'
import { useTranslation } from '@/i18n'
import type { User } from '@/types'

const advancedSections = [
  { id: 'storage', label: 'admin.simple.storage', icon: HardDrive },
  { id: 'playback', label: 'admin.simple.playback', icon: MonitorPlay },
  { id: 'ai', label: 'admin.simple.metadata', icon: Sparkles },
  { id: 'diagnostics', label: 'admin.simple.diagnostics', icon: Stethoscope },
  { id: 'maintenance', label: 'admin.simple.maintenance', icon: ShieldAlert },
] as const

function resolveView(hash: string) {
  const id = hash.slice(1)
  if (id === 'users' || id === 'advanced') return id
  if (id === 'dashboard') return 'advanced'
  return advancedSections.find((section) => section.id === id)?.id ?? 'library'
}

export default function AdminPage() {
  const { hash } = useLocation()
  const { t } = useTranslation()
  const [users, setUsers] = useState<User[]>([])
  const view = resolveView(hash)
  const section = advancedSections.find((item) => item.id === view)
  const isAdvanced = view !== 'library' && view !== 'users'

  return (
    <div className="nv-admin-page">
      <div className="space-y-2">
        <AdminPageHeader
          className="nv-admin-simple-header"
          title={t(section?.label ?? (isAdvanced ? 'admin.simple.advanced' : 'admin.title'))}
          actions={isAdvanced ? (
            <Link to={section ? '#advanced' : '#library'} className="nv-button nv-button--ghost nv-button--sm">
              <ChevronLeft size={16} aria-hidden="true" />
              {t(section ? 'admin.simple.backToAdvanced' : 'admin.simple.backToManage')}
            </Link>
          ) : (
            <Link to="#advanced" className="nv-button nv-button--ghost nv-button--sm">
              <Settings size={16} aria-hidden="true" />{t('admin.simple.advanced')}
            </Link>
          )}
        />

        {!isAdvanced && (
          <nav aria-label={t('admin.title')} className="nv-admin-main-nav">
            <Link to="#library" aria-current={view === 'library' ? 'page' : undefined} className={`nv-admin-tab${view === 'library' ? ' is-active' : ''}`}>
              <FolderOpen size={16} aria-hidden="true" />{t('admin.simple.libraries')}
            </Link>
            <Link to="#users" aria-current={view === 'users' ? 'page' : undefined} className={`nv-admin-tab${view === 'users' ? ' is-active' : ''}`}>
              <Users size={16} aria-hidden="true" />{t('admin.simple.family')}
            </Link>
          </nav>
        )}
      </div>

      <div key={view} className="nv-admin-content-enter space-y-5">
        {view === 'library' && <AdminLibraries />}
        {view === 'users' && (
          <>
            <FamilyInvite />
            <UsersTab users={users} setUsers={setUsers} />
          </>
        )}
        {view === 'advanced' && (
          <nav aria-label={t('admin.simple.advanced')} className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
            {advancedSections.map(({ id, label, icon: Icon }) => (
              <Link key={id} to={`#${id}`} className="flex min-h-12 items-center gap-3 rounded px-2 py-2 text-sm font-medium hover:bg-[var(--nv-fill-hover)]">
                <Icon size={17} aria-hidden="true" className="text-[var(--nv-text-tertiary)]" />
                <span className="flex-1">{t(label)}</span>
                <ChevronRight size={16} aria-hidden="true" className="text-[var(--nv-text-tertiary)]" />
              </Link>
            ))}
          </nav>
        )}
        {view === 'ai' && (
          <>
            <MetadataSettings />
            <AITab />
          </>
        )}
        {view === 'storage' && <StorageTab />}
        {(view === 'storage' || view === 'playback' || view === 'diagnostics' || view === 'maintenance') && (
          <AdvancedSystemSettings section={view} />
        )}
      </div>
    </div>
  )
}
