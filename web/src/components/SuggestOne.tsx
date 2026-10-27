import { useMemo, useState } from 'react'
import { Link } from 'react-router-dom'
import type { MixedItem } from '@/types'
import { Button, Modal, ModalBody, ModalFooter, ModalHeader, buttonClassName } from '@/components/design-system'

export default function SuggestOne({ items }: { items: MixedItem[] }) {
  const candidates = useMemo(() => {
    const seen = new Set<string>()
    return items.filter((item) => {
      const id = item.type === 'series' ? item.series?.id : item.media?.id
      const key = `${item.type}:${id}`
      if (!id || seen.has(key)) return false
      seen.add(key)
      return true
    })
  }, [items])
  const [selected, setSelected] = useState<MixedItem | null>(null)
  if (!candidates.length) return null
  const choose = () => {
    const choices = candidates.filter((item) => item !== selected)
    setSelected(choices[Math.floor(Math.random() * choices.length)] || candidates[0])
  }
  const media = selected?.type === 'series' ? selected.series : selected?.media
  return (
    <>
      <Button onClick={choose} variant="secondary">帮我选一部</Button>
      <Modal open={Boolean(media)} onClose={() => setSelected(null)} size="sm" ariaLabel="试试这一部">
        <ModalHeader title="试试这一部" onClose={() => setSelected(null)} />
        <ModalBody>
          {media && <div className="space-y-3">
            <h2 className="text-xl font-semibold">{media.title}</h2>
            <p className="text-sm text-[var(--nv-text-tertiary)]">从最近添加的影片中选出</p>
            <p className="line-clamp-4 text-base leading-7">{media.overview || '暂时没有简介，可以先查看详情。'}</p>
          </div>}
        </ModalBody>
        <ModalFooter>
          <Button disabled={candidates.length < 2} onClick={choose}>换一部</Button>
          {media && <Link to={selected?.type === 'series' ? `/series/${media.id}` : `/play/${media.id}`} className={buttonClassName({ variant: 'primary' })}>{selected?.type === 'series' ? '选集播放' : '播放'}</Link>}
        </ModalFooter>
      </Modal>
    </>
  )
}
