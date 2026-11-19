/** @type {import('tailwindcss').Config} */
export default {
  content: [
    './index.html',
    './src/**/*.{js,ts,jsx,tsx}',
  ],
  // 项目使用 <html data-theme="dark|light"> 切换主题，
  // 通过 selector 模式让 Tailwind 的 dark: 变体与此对齐。
  darkMode: ['selector', '[data-theme="dark"]'],
  theme: {
    extend: {
      // 字体栈的唯一来源是 design-system.css 的 --nv-font-* 令牌。
      fontFamily: {
        display: ['var(--nv-font-display)'],
        body: ['var(--nv-font-sans)'],
        mono: ['var(--nv-font-mono)'],
      },
      // 颜色不由 Tailwind 定义：全站主色 / 氛围光的唯一来源是
      // ui-theme.css 的 --nv-* 令牌。历史上的 primary（cyan）/ accent
      // （purple）色板与 deep-space 背景在 src/ 中已无人消费，删除；
      // 氛围光改走 --nv-ambient-cyan / --nv-ambient-purple-soft 令牌。
      boxShadow: {
        'card-hover': 'var(--nv-shadow-card-hover)',
      },
      animation: {
        'fade-in': 'fadeIn 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-up': 'slideUp 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-down': 'slideDown 0.4s cubic-bezier(0.22, 1, 0.36, 1)',
        'scale-in': 'scaleIn 0.3s cubic-bezier(0.22, 1, 0.36, 1)',
        'slide-right': 'slideRight 0.3s cubic-bezier(0.22, 1, 0.36, 1)',
        'float': 'float 6s ease-in-out infinite',
        'shimmer': 'shimmer 2s linear infinite',
        'ripple': 'ripple 0.6s ease-out',
        'particle-burst': 'particleBurst 0.5s ease-out forwards',
        'page-enter': 'pageEnter 0.5s cubic-bezier(0.22, 1, 0.36, 1)',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0', filter: 'blur(4px)' },
          '100%': { opacity: '1', filter: 'blur(0)' },
        },
        slideUp: {
          '0%': { opacity: '0', transform: 'translateY(20px)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        },
        slideDown: {
          '0%': { opacity: '0', transform: 'translateY(-20px)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        },
        scaleIn: {
          '0%': { opacity: '0', transform: 'scale(0.9)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'scale(1)', filter: 'blur(0)' },
        },
        slideRight: {
          '0%': { opacity: '0', transform: 'translateX(-10px)' },
          '100%': { opacity: '1', transform: 'translateX(0)' },
        },
        float: {
          '0%, 100%': { transform: 'translateY(0)' },
          '50%': { transform: 'translateY(-10px)' },
        },
        shimmer: {
          '0%': { backgroundPosition: '-200% 0' },
          '100%': { backgroundPosition: '200% 0' },
        },
        ripple: {
          '0%': { transform: 'scale(0.8)', opacity: '1' },
          '100%': { transform: 'scale(2)', opacity: '0' },
        },
        particleBurst: {
          '0%': { transform: 'scale(1)', opacity: '1' },
          '100%': { transform: 'scale(1.5)', opacity: '0' },
        },
        pageEnter: {
          '0%': { opacity: '0', transform: 'translateY(12px)', filter: 'blur(4px)' },
          '100%': { opacity: '1', transform: 'translateY(0)', filter: 'blur(0)' },
        },
      },
      backdropBlur: {
        xs: '2px',
      },
      borderRadius: {
        '2xl': '1rem',
        '3xl': '1.5rem',
      },
    },
  },
  plugins: [],
}
