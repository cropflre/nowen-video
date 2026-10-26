import assert from 'node:assert/strict'
import { mkdtemp, rm, writeFile } from 'node:fs/promises'
import { createRequire } from 'node:module'
import { tmpdir } from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { test } from 'node:test'
import { build } from 'esbuild'

// Render the real components with the same TypeScript/alias configuration as Vite.
const webRoot = fileURLToPath(new URL('..', import.meta.url))
const temp = await mkdtemp(path.join(tmpdir(), 'nowen-media-tech-'))
let ui
try {
  const bundle = await build({
    stdin: {
      contents: `
        import React from 'react'
        import { renderToStaticMarkup } from 'react-dom/server'
        import Overview from './src/components/media/MediaDetailTechOverview'
        import Specs from './src/components/media/MediaTechSpecs'
        export { mediaSourceInfo, mediaDynamicRange, realMediaCodec } from './src/utils/mediaTechSpecs'
        export const overview = props => renderToStaticMarkup(React.createElement(Overview, props))
        export const specs = props => renderToStaticMarkup(React.createElement(Specs, props))
      `,
      resolveDir: webRoot,
      loader: 'tsx',
    },
    tsconfig: path.join(webRoot, 'tsconfig.json'),
    bundle: true,
    platform: 'node',
    format: 'cjs',
    write: false,
    logLevel: 'silent',
    define: { 'process.env.NODE_ENV': '"production"' },
  })
  const output = path.join(temp, 'render.cjs')
  await writeFile(output, bundle.outputFiles[0].contents)
  ui = createRequire(import.meta.url)(output)
} finally {
  await rm(temp, { recursive: true, force: true })
}

const media = { title: 'STRM fixture', file_path: '/movies/movie.strm', file_size: 128, video_codec: 'strm_mkv', audio_codec: '', resolution: '' }
const fileInfo = { file_ext: '.strm', file_size: 128 }
const stream = { codec_type: 'video', codec_name: 'hevc', width: 3840, height: 2160, frame_rate: '23.976', pix_fmt: 'yuv420p10le', color_transfer: 'smpte2084' }
const techSpecs = { streams: [stream, { codec_type: 'audio', codec_name: 'aac', channels: 6 }], format: { format_name: 'matroska,webm', size: '10737418240', bit_rate: '8000000' } }

test('unprobed STRM never claims SDR, codec, source container or index size as video size', () => {
  const html = ui.overview({ media, fileInfo, techSpecs: null })
  assert.doesNotMatch(html, /SDR|STRM_MKV|0 MB/)
  assert.match(html, /STRM 远程视频/)
  assert.deepEqual(ui.mediaSourceInfo(media, null, fileInfo), { isSTRM: true, size: 0, container: '--' })
  const details = ui.specs({ media, fileInfo, techSpecs: null })
  assert.doesNotMatch(details, /strm_mkv|0 MB|无内嵌字幕/)
  assert.match(details, /大小未知/)
})

test('remote specs render actual frame rate, HDR, container, size and audio', () => {
  const html = ui.overview({ media, fileInfo, techSpecs })
  for (const text of ['3840×2160', 'HEVC', 'HDR10', '23.976 fps', 'AAC · 5.1', 'MATROSKA', '10.00 GB', '8.0 Mbps']) {
    assert.ok(html.includes(text), `missing ${text}`)
  }
  assert.doesNotMatch(html, /STRM_MKV|0 MB/)
  const details = ui.specs({ media, fileInfo, techSpecs })
  assert.match(details, /Matroska \(MKV\)/)
  assert.match(details, /10.00 GB/)
})

test('unknown remote size cannot fall back to index bytes', () => {
  for (const size of ['', 'N/A', '-1', '0']) {
    const specs = { ...techSpecs, format: { ...techSpecs.format, size } }
    assert.equal(ui.mediaSourceInfo(media, specs, fileInfo).size, 0)
  }
})

test('local files retain their local size and extension fallback', () => {
  const local = { ...media, file_path: '/movies/movie.mkv', file_size: 1073741824, video_codec: 'hevc' }
  const file = { file_ext: '.mkv', file_size: 1073741824 }
  assert.deepEqual(ui.mediaSourceInfo(local, null, file), { isSTRM: false, size: 1073741824, container: 'MKV' })
  const html = ui.overview({ media: local, fileInfo: file, techSpecs: null })
  assert.match(html, /HEVC/)
  assert.match(html, /1.00 GB/)
})

test('missing color metadata stays unknown and 10-bit alone is not HDR', () => {
  assert.equal(ui.mediaDynamicRange(), '--')
  assert.equal(ui.mediaDynamicRange({ codec_type: 'video' }), '--')
  assert.equal(ui.mediaDynamicRange({ pix_fmt: 'yuv420p' }), 'SDR')
  assert.equal(ui.mediaDynamicRange({ pix_fmt: 'yuv420p10le' }), '10-bit')
  assert.equal(ui.realMediaCodec('STRM_UNKNOWN'), undefined)
})
