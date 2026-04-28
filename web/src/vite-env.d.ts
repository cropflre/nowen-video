/// <reference types="vite/client" />

/**
 * mp4box.js 类型声明（npm 包自带的类型不完整）
 */
declare module 'mp4box' {
  interface MP4Info {
    duration: number
    timescale: number
    tracks: any[]
  }

  interface MP4File {
    onReady: (info: MP4Info) => void
    onError: (err: any) => void
    onSamples: (id: number, user: any, samples: any[]) => void
    appendBuffer: (buf: ArrayBuffer & { fileStart?: number }) => number
    setExtractionOptions: (trackId: number, user: any, options: { nbSamples?: number }) => void
    getTrackById: (id: number) => any
    start: () => void
    stop: () => void
    flush: () => void
    seek: (time: number, useRAP: boolean) => { offset: number; time: number }
  }

  const MP4Box: {
    createFile: () => MP4File
    DataStream: {
      new (buffer?: ArrayBuffer, byteOffset?: number, endianness?: boolean): {
        buffer: ArrayBuffer
      }
      BIG_ENDIAN: boolean
      LITTLE_ENDIAN: boolean
    }
  }
  export default MP4Box
}

/**
 * WebCodecs API 类型（部分 TS lib 里未声明或版本不完整，这里补齐）
 * 当 tsconfig lib 里已有 DOM.Iterable 或升级 ts 5.x 后可去掉
 */
// 这些通常在 lib.dom.d.ts 中已有，留空以避免覆盖
