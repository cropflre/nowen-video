import { useEffect, useMemo } from 'react'
import { Film } from 'lucide-react'
import { motion } from 'framer-motion'
import { modalContentVariants } from '@/lib/motion'
import { Modal, ModalHeader, ModalBody } from '@/components/design-system/Modal'

interface TrailerModalProps {
  trailerUrl: string
  onClose: () => void
}

function getYouTubeId(url: string): string | null {
  const match = url.match(/(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/)([a-zA-Z0-9_-]{11})/)
  return match?.[1] || null
}

/** 预告片弹窗：统一使用 Design System Modal，并保留 YouTube 自动播放与外链兜底。 */
export default function TrailerModal({ trailerUrl, onClose }: TrailerModalProps) {
  const videoId = useMemo(() => getYouTubeId(trailerUrl), [trailerUrl])

  useEffect(() => {
    if (videoId) return
    window.open(trailerUrl, '_blank', 'noopener,noreferrer')
    onClose()
  }, [onClose, trailerUrl, videoId])

  if (!videoId) return null

  return (
    <Modal
      onClose={onClose}
      size="video"
      ariaLabel="预告片播放器"
      panelClassName="nv-trailer-modal"
    >
      <ModalHeader
        title="预告片"
        description="YouTube Trailer"
        icon={<Film size={18} aria-hidden="true" />}
        onClose={onClose}
        className="nv-trailer-modal-header"
      />
      <ModalBody className="nv-trailer-modal-body p-0 sm:p-0">
        <motion.div
          className="nv-trailer-stage relative w-full overflow-hidden"
          style={{ aspectRatio: '16 / 9' }}
          variants={modalContentVariants}
          initial="hidden"
          animate="visible"
          exit="exit"
        >
          <iframe
            className="absolute inset-0 h-full w-full"
            src={`https://www.youtube.com/embed/${videoId}?autoplay=1&rel=0&modestbranding=1`}
            title="预告片"
            allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
            allowFullScreen
            style={{ border: 'none' }}
          />
        </motion.div>
      </ModalBody>
    </Modal>
  )
}
