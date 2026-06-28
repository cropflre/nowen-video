import { motion } from 'framer-motion'
import {
  Globe,
  Palette,
  FolderOpen,
  Database,
  RefreshCw,
  Play,
  Subtitles,
  Zap,
  Info,
} from 'lucide-react'
import {
  MobilePageHeader,
  MobileSettingGroup,
  MobileSettingItem,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'

/**
 * 移动端设置页面
 * Hills Pro 风格：品牌卡片 + 分组列表
 */
export default function MobileSettingsPage() {
  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader title="设置" />

      {/* 品牌卡片 */}
      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4 }}
        className="mx-8 mb-6"
        style={{
          height: '170px',
          borderRadius: mobileTokens.radius['2xl'],
          background: 'linear-gradient(135deg, #4A5FC1, #6366F1)',
          padding: '28px',
          display: 'flex',
          flexDirection: 'column',
          justifyContent: 'space-between',
          boxShadow: '0 10px 30px rgba(99, 102, 241, 0.3)',
        }}
      >
        <div>
          <h2
            style={{
              fontSize: '36px',
              fontWeight: 700,
              color: '#fff',
              lineHeight: 1.2,
            }}
          >
            Nowen Video
          </h2>
          <p
            style={{
              fontSize: mobileTokens.fontSize.lg,
              color: 'rgba(255, 255, 255, 0.8)',
              marginTop: '8px',
            }}
          >
            私人影音中心
          </p>
        </div>

        <motion.button
          whileTap={{ scale: 0.95 }}
          onClick={() => {
            // TODO: 跳转管理页面
          }}
          style={{
            alignSelf: 'flex-start',
            padding: '12px 24px',
            borderRadius: mobileTokens.radius.full,
            background: 'rgba(255, 255, 255, 0.2)',
            color: '#fff',
            fontSize: mobileTokens.fontSize.md,
            fontWeight: 500,
            backdropFilter: 'blur(10px)',
          }}
        >
          进入管理
        </motion.button>
      </motion.div>

      {/* 设置分组 */}
      <MobileSettingGroup title="通用">
        <MobileSettingItem
          icon={<Globe size={22} />}
          title="语言"
          value="Auto"
          onClick={() => {
            // TODO: 跳转语言设置
          }}
        />
        <MobileSettingItem
          icon={<Palette size={22} />}
          title="主题"
          onClick={() => {
            // TODO: 跳转主题设置
          }}
        />
        <MobileSettingItem
          icon={<FolderOpen size={22} />}
          title="媒体库"
          onClick={() => {
            // TODO: 跳转媒体库管理
          }}
        />
        <MobileSettingItem
          icon={<Database size={22} />}
          title="备份与还原"
          onClick={() => {
            // TODO: 跳转备份还原
          }}
        />
        <MobileSettingItem
          icon={<RefreshCw size={22} />}
          title="同步"
          onClick={() => {
            // TODO: 跳转同步设置
          }}
        />
      </MobileSettingGroup>

      <MobileSettingGroup title="播放器">
        <MobileSettingItem
          icon={<Zap size={22} />}
          title="交互"
          onClick={() => {
            // TODO: 跳转交互设置
          }}
        />
        <MobileSettingItem
          icon={<Play size={22} />}
          title="播放器"
          onClick={() => {
            // TODO: 跳转播放器设置
          }}
        />
        <MobileSettingItem
          icon={<Subtitles size={22} />}
          title="弹幕"
          onClick={() => {
            // TODO: 跳转弹幕设置
          }}
        />
      </MobileSettingGroup>

      <MobileSettingGroup title="关于">
        <MobileSettingItem
          icon={<Info size={22} />}
          title="关于"
          onClick={() => {
            // TODO: 跳转关于页面
          }}
        />
      </MobileSettingGroup>
    </>
  )
}
