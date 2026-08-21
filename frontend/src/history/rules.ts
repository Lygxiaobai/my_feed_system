/** 与 backend/internal/history/classify.go 保持同一套阈值，本地缓存和游客恢复也走这套。 */
export const MIN_PERSIST_MS = 3000
export const MIN_PERSIST_RATIO = 0.2
export const COMPLETE_REMAIN_MS = 2000
export const COMPLETE_RATIO = 0.95
export const RESUME_MIN_MS = 2000
export const RESUME_MIN_RATIO = 0.1

export function shouldPersist(positionMs: number, durationMs: number) {
  if (positionMs <= 0) return false
  if (positionMs >= MIN_PERSIST_MS) return true
  return durationMs > 0 && positionMs >= durationMs * MIN_PERSIST_RATIO
}

export function isCompleted(positionMs: number, durationMs: number) {
  if (durationMs <= 0 || positionMs <= 0) return false
  if (durationMs - positionMs <= COMPLETE_REMAIN_MS) return true
  return positionMs >= durationMs * COMPLETE_RATIO
}

export function shouldResume(positionMs: number, durationMs: number, completed: boolean) {
  if (completed || positionMs < RESUME_MIN_MS) return false
  if (durationMs > 0 && positionMs < durationMs * RESUME_MIN_RATIO) return false
  return !isCompleted(positionMs, durationMs)
}

export function formatWatchClock(ms: number) {
  const total = Math.max(0, Math.floor(ms / 1000))
  const minutes = Math.floor(total / 60)
  const seconds = total % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}
