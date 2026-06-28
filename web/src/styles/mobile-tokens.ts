// 移动端设计 Token
// Hills Pro 风格：柔和浅色背景 + 毛玻璃 + 大圆角卡片

export const mobileTokens = {
  // 背景色
  bg: '#F8F5FB',
  bgAlt: '#F4F1F8',
  bgWarm: '#E5B5A7',

  // 文字色
  text: '#171820',
  textMuted: '#6B6E7A',
  textSecondary: '#8E919E',

  // 主题色
  primary: '#142060',
  primarySoft: '#DFE2FF',
  primaryLight: '#E8EAFF',

  // 卡片
  card: 'rgba(255, 255, 255, 0.72)',
  cardBorder: 'rgba(28, 32, 56, 0.08)',

  // 毛玻璃
  glass: 'rgba(246, 244, 252, 0.82)',
  glassBorder: 'rgba(255, 255, 255, 0.72)',

  // 阴影
  shadow: '0 18px 44px rgba(31, 35, 70, 0.14)',
  shadowSm: '0 10px 30px rgba(30, 35, 60, 0.06)',
  shadowInset: 'inset 0 1px 0 rgba(255, 255, 255, 0.65)',

  // 选中态
  active: 'rgba(224, 226, 255, 0.86)',
  activeText: '#0F1D5A',

  // 渐变
  heroGradient: 'linear-gradient(180deg, rgba(255,255,255,0.20), rgba(255,255,255,0.52))',

  // 圆角
  radius: {
    xs: '8px',
    sm: '12px',
    md: '16px',
    lg: '20px',
    xl: '24px',
    '2xl': '28px',
    '3xl': '32px',
    full: '999px',
  },

  // 间距
  spacing: {
    xs: '8px',
    sm: '12px',
    md: '16px',
    lg: '24px',
    xl: '32px',
    '2xl': '40px',
    '3xl': '48px',
  },

  // 字号
  fontSize: {
    xs: '12px',
    sm: '14px',
    md: '16px',
    lg: '18px',
    xl: '22px',
    '2xl': '26px',
    '3xl': '32px',
    '4xl': '36px',
  },

  // 底部导航
  nav: {
    height: '72px',
    width: '70%',
    maxWidth: '520px',
    bottom: '18px',
    background: 'rgba(246, 244, 252, 0.82)',
    border: '1px solid rgba(255, 255, 255, 0.72)',
    borderRadius: '999px',
    boxShadow: '0 18px 44px rgba(31, 35, 70, 0.14), inset 0 1px 0 rgba(255, 255, 255, 0.65)',
    activeBackground: 'rgba(224, 226, 255, 0.86)',
    activeColor: '#0F1D5A',
  },
} as const

// 移动端 CSS 变量
export const mobileCSSVariables = `
  --mobile-bg: ${mobileTokens.bg};
  --mobile-bg-alt: ${mobileTokens.bgAlt};
  --mobile-text: ${mobileTokens.text};
  --mobile-muted: ${mobileTokens.textMuted};
  --mobile-primary: ${mobileTokens.primary};
  --mobile-primary-soft: ${mobileTokens.primarySoft};
  --mobile-card: ${mobileTokens.card};
  --mobile-glass: ${mobileTokens.glass};
  --mobile-border: ${mobileTokens.cardBorder};
  --mobile-shadow: ${mobileTokens.shadow};
`
