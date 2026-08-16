import { streamApi } from '@/api'
import { mediaAnalysisApi, type MediaHighlight } from '@/api/mediaAnalysis'

const HERO_SELECTOR = '.nv-media-detail-page .nv-detail-hero'
const INNER_SELECTOR = '.nv-detail-hero-inner'
const SLIDESHOW_CLASS = 'nv-detail-highlight-slideshow'
const MAX_HIGHLIGHT_SLIDES = 6
const SLIDE_INTERVAL_MS = 6500
const REFRESH_INTERVAL_MS = 60_000

interface HeroSession {
  mediaId: string
  hero: HTMLElement
  controller: AbortController
  slideTimer: number | null
  refreshTimer: number | null
  layer: HTMLDivElement | null
}

let session: HeroSession | null = null
let observer: MutationObserver | null = null
let scheduled = false

function mediaIdFromPathname() {
  const match = window.location.pathname.match(/^\/media\/([^/?#]+)/)
  return match ? decodeURIComponent(match[1]) : ''
}

function enhanceTitle(hero: HTMLElement) {
  const title = hero.querySelector<HTMLHeadingElement>('h1')
  if (!title) return

  const text = (title.textContent || '').trim()
  title.title = text
  title.classList.toggle('nv-detail-title-long', text.length > 42)
  title.classList.toggle('nv-detail-title-very-long', text.length > 88)
}

function originalBackdropUrl(hero: HTMLElement) {
  const image = hero.querySelector<HTMLImageElement>(':scope > div.absolute.inset-0 img')
  return image?.currentSrc || image?.src || ''
}

function highlightUrls(highlights: MediaHighlight[]) {
  return [...highlights]
    .filter((item) => Boolean(item.thumbnail_url))
    .sort((left, right) => right.score - left.score || left.start_time - right.start_time)
    .slice(0, MAX_HIGHLIGHT_SLIDES)
    .map((item) => streamApi.withTokenUrl(item.thumbnail_url!))
}

function uniqueUrls(urls: string[]) {
  const seen = new Set<string>()
  return urls.filter((url) => {
    if (!url || seen.has(url)) return false
    seen.add(url)
    return true
  })
}

function clearSlides(target: HeroSession) {
  if (target.slideTimer !== null) {
    window.clearInterval(target.slideTimer)
    target.slideTimer = null
  }
  target.layer?.remove()
  target.layer = null
  target.hero.classList.remove('nv-detail-hero-has-highlight-slides')
}

function renderSlides(target: HeroSession, urls: string[]) {
  clearSlides(target)
  if (urls.length < 2 || !target.hero.isConnected) return

  const layer = document.createElement('div')
  layer.className = SLIDESHOW_CLASS
  layer.setAttribute('aria-hidden', 'true')

  const slides = urls.map((url, index) => {
    const slide = document.createElement('div')
    slide.className = `nv-detail-highlight-slide${index === 0 ? ' is-active' : ''}`

    const image = document.createElement('img')
    image.src = url
    image.alt = ''
    image.decoding = 'async'
    image.loading = index < 2 ? 'eager' : 'lazy'
    image.addEventListener('error', () => slide.classList.add('is-failed'), { once: true })

    slide.appendChild(image)
    layer.appendChild(slide)
    return slide
  })

  const inner = target.hero.querySelector<HTMLElement>(INNER_SELECTOR)
  target.hero.insertBefore(layer, inner || null)
  target.layer = layer
  target.hero.classList.add('nv-detail-hero-has-highlight-slides')

  if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return

  let activeIndex = 0
  target.slideTimer = window.setInterval(() => {
    if (document.hidden || !target.hero.isConnected) return
    const nextIndex = (activeIndex + 1) % slides.length
    slides[activeIndex]?.classList.remove('is-active')
    slides[nextIndex]?.classList.add('is-active')
    activeIndex = nextIndex
  }, SLIDE_INTERVAL_MS)
}

async function refreshHighlights(target: HeroSession) {
  try {
    const response = await mediaAnalysisApi.getHighlights(target.mediaId)
    if (target.controller.signal.aborted || session !== target || !target.hero.isConnected) return

    const base = originalBackdropUrl(target.hero)
    const urls = uniqueUrls([base, ...highlightUrls(response.data.data?.highlights || [])])
    renderSlides(target, urls)
  } catch {
    if (session === target) clearSlides(target)
  }
}

function destroySession() {
  if (!session) return
  session.controller.abort()
  clearSlides(session)
  if (session.refreshTimer !== null) window.clearInterval(session.refreshTimer)
  session = null
}

function syncHero() {
  scheduled = false
  const mediaId = mediaIdFromPathname()
  const hero = document.querySelector<HTMLElement>(HERO_SELECTOR)

  if (!mediaId || !hero) {
    destroySession()
    return
  }

  enhanceTitle(hero)

  if (session?.mediaId === mediaId && session.hero === hero) return

  destroySession()
  const next: HeroSession = {
    mediaId,
    hero,
    controller: new AbortController(),
    slideTimer: null,
    refreshTimer: null,
    layer: null,
  }
  session = next
  void refreshHighlights(next)
  next.refreshTimer = window.setInterval(() => void refreshHighlights(next), REFRESH_INTERVAL_MS)
}

function scheduleSync() {
  if (scheduled) return
  scheduled = true
  window.requestAnimationFrame(syncHero)
}

export function installMediaDetailHeroEnhancer() {
  if (typeof window === 'undefined' || typeof document === 'undefined' || observer) return

  observer = new MutationObserver(scheduleSync)
  observer.observe(document.body, { childList: true, subtree: true })
  window.addEventListener('popstate', scheduleSync)
  scheduleSync()
}
