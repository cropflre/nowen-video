
/**
 * DesktopEventBinder —— 在根组件挂一个，统一处理桌面端事件
 *
 * - 菜单点击 → 按 action id 分发（打开设置、全屏切换等）
 * - 文件关联打开 → 解析路径并跳转播放页
 * - Deep Link → 解析 URL 并导航
 *
 * 浏览器环境不渲染也不监听。
 */

import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { toast } from 'react-hot-toast'
import { desktop } from './bridge'

export default function DesktopEventBinder() {
  const navigate = useNavigate()

  useEffect(() => {
    if (!desktop.isDesktop) return
    const cleanups: Array<() => void> = []

    // 菜单事件
    desktop
      .onMenuAction((actionId) => {
        switch (actionId) {
          case 'menu_open_file':
            desktop.pickFile().then((p) => {
              if (p) toast(`文件已选择: ${p}\n请在媒体库中导入后播放`)
            })
            break
          case 'menu_open_folder':
            desktop.pickFolder().then((p) => {
              if (p) toast(`目录已选择: ${p}\n请在"媒体库设置"中添加此目录`)
            })
            break
          case 'menu_settings':
            navigate('/admin')
            break
          case 'menu_fullscreen':
            desktop.windowToggleFullscreen()
            break
          case 'menu_play_pause':
            // 可选：向播放页广播（目前通过 web 播放器自己的快捷键处理）
            break
          case 'menu_restart_backend':
            toast.promise(desktop.sidecarRestart(), {
              loading: '正在重启后端...',
              success: '后端已重启',
              error: '重启失败',
            })
            break
          case 'menu_about':
            toast('nowen-video Desktop\nPC 端原画原音观影客户端', { duration: 4000 })
            break
          default:
            break
        }
      })
      .then((u) => cleanups.push(u))

    // 文件打开事件（双击 / 命令行）
    desktop
      .onOpenFiles((paths) => {
        if (!paths || !paths.length) return
        toast(`🎬 打开文件: ${paths[0].split(/[\\/]/).pop()}`, { duration: 3000 })
        // 未来可以：POST 到后端临时加库 → 跳到播放页
        // 目前简化为提示用户手动导入
      })
      .then((u) => cleanups.push(u))

    // Deep Link 事件（nowen-video://play?media_id=123）
    desktop
      .onDeepLink((urlStr) => {
        try {
          const u = new URL(urlStr)
          if (u.protocol !== 'nowen-video:') return
          const mediaId = u.searchParams.get('media_id')
          const path = u.pathname.replace(/^\/+/, '')
          if (path === 'play' && mediaId) {
            navigate(`/play/${mediaId}`)
          } else if (path === 'settings') {
            navigate('/admin')
          } else {
            toast(`Deep Link: ${urlStr}`)
          }
        } catch (e) {
          console.warn('deep-link 解析失败:', e)
        }
      })
      .then((u) => cleanups.push(u))

    return () => {
      cleanups.forEach((f) => f())
    }
  }, [navigate])

  return null
}
