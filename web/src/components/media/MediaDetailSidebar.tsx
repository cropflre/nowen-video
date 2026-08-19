import type { FileDetail, Media, MediaPlayInfo, PlaybackStatsInfo, TechSpecs } from '@/types'

interface MediaDetailSidebarProps {
  media: Media
  playInfo: MediaPlayInfo | null
  techSpecs: TechSpecs | null
  fileInfo: FileDetail | null
  playbackStats: PlaybackStatsInfo | null
  isAdmin: boolean
  onManageSubtitles: () => void
}

/**
 * Legacy detail summary slot.
 *
 * Playback / subtitle / metadata diagnostics used to render beneath the hero.
 * The current product layout intentionally removes that strip so the detail
 * page flows directly from the hero into the editorial content. Subtitle
 * management now lives beside the hero actions instead.
 *
 * Keep this compatibility component temporarily because MediaDetailPage still
 * imports the slot while the older detail-tab structure is being retired.
 */
export default function MediaDetailSidebar(_props: MediaDetailSidebarProps) {
  return null
}
