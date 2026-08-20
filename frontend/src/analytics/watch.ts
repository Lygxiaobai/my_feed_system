import { track, type ProductProps } from './track'

type WatchHandle = {
  currentTime: () => number
  duration: () => number
}

export function createWatchSession() {
  let current: { videoId: number; started: boolean } | null = null

  function play(videoId: number, properties?: ProductProps) {
    if (!videoId) return
    if (current?.videoId === videoId && current.started) return
    if (current && current.videoId !== videoId) end()
    current = { videoId, started: true }
    track('video_play', { video_id: videoId, ...properties })
  }

  function end(handle?: WatchHandle, properties?: ProductProps) {
    if (!current?.started) {
      current = null
      return
    }
    const watchMs = handle ? Math.max(0, Math.round(handle.currentTime() * 1000)) : undefined
    const durationMs = handle ? Math.max(0, Math.round(handle.duration() * 1000)) : undefined
    track('video_watch', {
      video_id: current.videoId,
      watch_ms: watchMs,
      duration_ms: durationMs,
      ...properties,
    })
    current = null
  }

  return { play, end }
}
