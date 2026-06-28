import { useState } from 'react'
import { motion } from 'framer-motion'
import { Download, MoreVertical, Plus } from 'lucide-react'
import {
  MobilePageHeader,
  ServerCard,
  FloatingActionButton,
} from '@/components/mobile'

/**
 * 移动端服务器页面
 * Hills Pro 风格
 */
export default function MobileServersPage() {
  const [serverName] = useState('Nowen Video')
  const [isConnected] = useState(true)

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title="服务器"
        actions={[
          {
            icon: <Download size={22} />,
            onClick: () => {
              // TODO: 实现导入功能
            },
            label: '导入',
          },
          {
            icon: <MoreVertical size={22} />,
            onClick: () => {
              // TODO: 实现更多菜单
            },
            label: '更多',
          },
        ]}
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
            name={serverName}
            lastAccess="当前服务器"
            isConnected={isConnected}
            onClick={() => {
              // 进入服务器详情页（首页）
              // TODO: 实现服务器详情页导航
            }}
          />
        </motion.div>
      </div>

      {/* 悬浮添加按钮 */}
      <FloatingActionButton
        icon={<Plus size={24} />}
        onClick={() => {
          // TODO: 实现添加服务器功能
        }}
        label="添加服务器"
      />
    </>
  )
}
