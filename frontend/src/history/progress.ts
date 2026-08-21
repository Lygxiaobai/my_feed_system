import { postJsonKeepalive } from '../api/client'
import * as historyApi from '../api/history'
import { useAuthStore } from '../stores/auth'
import { isCompleted, shouldPersist, shouldResume } from './rules'

export type WatchHandle = {
  currentTime: () => number
  duration: () => number
}

const HEARTBEAT_MS = 10_000
const LOOP_BACK_MS = 1000
const CACHE_LIMIT = 80

type CacheEntry = {
  positionMs: number
  durationMs: number
  completed: boolean
  updatedAt: number
}

type CacheFile = {
  entries: Record<string, CacheEntry>
}

type Session = {
  videoId: number
  handle: WatchHandle | null
  lastMs: number
  timer: number | undefined
}

let session: Session | null = null
let pagehideBound = false

function cacheKey() {
  try {
    const accountId = useAuthStore().claims?.account_id
    return accountId ? `watch_progress:${accountId}` : 'watch_progress:guest'
  } catch {
    return 'watch_progress:guest'
  }
}

function readCache(): CacheFile {
  try {
    const raw = window.localStorage.getItem(cacheKey())
    if (!raw) return { entries: {} }
    const parsed = JSON.parse(raw) as CacheFile
    if (!parsed || typeof parsed !== 'object' || !parsed.entries) return { entries: {} }
    return parsed
  } catch {
    return { entries: {} }
  }
}

function writeCache(file: CacheFile) {
  const ids = Object.keys(file.entries)
  if (ids.length > CACHE_LIMIT) {
    const sorted = ids.sort((a, b) => (file.entries[a]?.updatedAt ?? 0) - (file.entries[b]?.updatedAt ?? 0))
    for (const id of sorted.slice(0, ids.length - CACHE_LIMIT)) {
      delete file.entries[id]
    }
  }
  try {
    window.localStorage.setItem(cacheKey(), JSON.stringify(file))
  } catch {
    // 隐私模式写不了本地缓存，登录用户仍可走服务端。
  }
}

function rememberLocal(videoId: number, positionMs: number, durationMs: number, completed: boolean) {
  const file = readCache()
  file.entries[String(videoId)] = {
    positionMs: completed ? 0 : positionMs,
    durationMs,
    completed,
    updatedAt: Date.now(),
  }
  writeCache(file)
}

function readLocal(videoId: number) {
  return readCache().entries[String(videoId)] ?? null
}

function toMs(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return 0
  return Math.round(seconds * 1000)
}

function persist(videoId: number, positionMs: number, durationMs: number, keepalive: boolean) {
  const completed = isCompleted(positionMs, durationMs)
  if (!completed && !shouldPersist(positionMs, durationMs)) return

  rememberLocal(videoId, positionMs, durationMs, completed)

  let loggedIn = false
  try {
    loggedIn = useAuthStore().isLoggedIn
  } catch {
    loggedIn = false
  }
  if (!loggedIn) return

  const body = { video_id: videoId, position_ms: positionMs, duration_ms: durationMs }
  if (keepalive) {
    postJsonKeepalive('/history/upsert', body)
    return
  }
  void historyApi.upsertProgress(videoId, positionMs, durationMs).catch(() => {
    // 进度失败不能打断播放，下次心跳或离开时再试。
  })
}

function currentSnapshot(target: Session) {
  const durationMs = target.handle ? toMs(target.handle.duration()) : 0
  const positionMs = target.handle ? toMs(target.handle.currentTime()) : target.lastMs
  return { positionMs, durationMs }
}

function stopTimer() {
  if (session?.timer !== undefined) {
    window.clearInterval(session.timer)
    session.timer = undefined
  }
}

function ensurePagehide() {
  if (pagehideBound || typeof window === 'undefined') return
  pagehideBound = true
  const flush = () => flushWatchProgress(undefined, { keepalive: true })
  window.addEventListener('pagehide', flush)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flush()
  })
}

export function bindWatchProgress(videoId: number, handle?: WatchHandle | null) {
  if (!videoId) return
  ensurePagehide()
  if (session?.videoId === videoId) {
    session.handle = handle ?? session.handle
    return
  }
  if (session) flushWatchProgress(session.videoId)
  stopTimer()
  session = {
    videoId,
    handle: handle ?? null,
    lastMs: 0,
    timer: window.setInterval(() => {
      flushWatchProgress(videoId)
    }, HEARTBEAT_MS),
  }
}

export function noteWatchProgress(videoId: number, seconds: number, durationSec: number) {
  if (!session || session.videoId !== videoId) return
  const nextMs = toMs(seconds)
  const durationMs = toMs(durationSec)
  // 循环回零时用回跳前的进度落一次「已看完」，避免把 0 写成未看完。
  if (session.lastMs > 0 && nextMs + LOOP_BACK_MS < session.lastMs && isCompleted(session.lastMs, durationMs)) {
    persist(videoId, session.lastMs, durationMs, false)
  }
  session.lastMs = nextMs
}

export function flushWatchProgress(videoId?: number, options?: { keepalive?: boolean }) {
  if (!session) return
  if (videoId != null && session.videoId !== videoId) return
  const { positionMs, durationMs } = currentSnapshot(session)
  persist(session.videoId, positionMs, durationMs, !!options?.keepalive)
}

export function unbindWatchProgress(videoId?: number) {
  if (!session) return
  if (videoId != null && session.videoId !== videoId) return
  flushWatchProgress(session.videoId)
  stopTimer()
  session = null
}

export async function resolveResumeSeconds(videoId: number) {
  if (!videoId) return 0

  let loggedIn = false
  try {
    loggedIn = useAuthStore().isLoggedIn
  } catch {
    loggedIn = false
  }

  if (loggedIn) {
    try {
      const items = await historyApi.listProgress([videoId])
      const item = items.find((row) => row.video_id === videoId)
      if (item) return item.resume_ms > 0 ? item.resume_ms / 1000 : 0
    } catch {
      // 服务端读失败时退回本机缓存。
    }
  }

  const local = readLocal(videoId)
  if (!local || !shouldResume(local.positionMs, local.durationMs, local.completed)) return 0
  return local.positionMs / 1000
}
