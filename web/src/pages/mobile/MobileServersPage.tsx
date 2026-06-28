import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Plus } from 'lucide-react'
import {
  MobilePageHeader,
  ServerCard,
  FloatingActionButton,
} from '@/components/mobile'

interface MobileServersPageProps {
  onEnterServer: () => void
}

/**
 * 移动端服务器页面
 * Hills Pro 风格
 */
export default function MobileServersPage({ onEnterServer }: MobileServersPageProps) {
  const [serverName, setServerName] = useState('')
  const [isConnected, setIsConnected] = useState(false)

  // 获取服务器信息
  useEffect(() => {
    // nowen-video 是单服务应用，获取系统信息
    fetch('/api/health')
      .then(res => res.json())
      .then(data => {
        if (data?.status === 'ok') {
          setServerName('Nowen Video')
          setIsConnected(true)
        }
      })
      .catch(() => {
        setServerName('Nowen Video')
        setIsConnected(false)
      })
  }, [])

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title="服务器"
      />

      {/* 服务器列表 */}
      <div
        className="flex flex-wrap gap-4 px-8"
        style={{ marginTop: '20px' }}
      >
        {/* 当前服务器卡片 */}
        <motion.div
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.3 }}
        >
          <ServerCard
            name={serverName || 'Nowen Video'}
            lastAccess={isConnected ? '当前服务器' : '连接失败'}
            isConnected={isConnected}
            onClick={onEnterServer}
          />
        </motion.div>
      </div>

      {/* 悬浮添加按钮 */}
      <FloatingActionButton
        icon={<Plus size={24} />}
        onClick={() => {
          // 单服务应用，跳转到设置页配置
          window.location.href = '/admin'
        }}
        label="服务器设置"
      />
    </>
  )
}
