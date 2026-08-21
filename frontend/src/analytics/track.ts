export const PRODUCT_EVENTS = [
  'page_view',
  'search',
  'video_play',
  'video_watch',
  'video_like',
  'video_unlike',
  'video_share',
  'comment_submit',
  'follow',
  'unfollow',
  'video_publish',
  'login',
  'register',
  'logout',
  'wallet_recharge',
  'wallet_tip',
  'wallet_checkin',
  'wallet_lottery',
  'report_submit',
  'danmaku_send',
  'dm_send',
] as const

export type ProductEventName = (typeof PRODUCT_EVENTS)[number]
export type ProductProps = Record<string, string | number | boolean | undefined>

type QueuedEvent = {
  event: ProductEventName
  page: string
  ts: number
  properties?: ProductProps
}

const VISITOR_KEY = 'feed.visitor_id'
const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'
const FLUSH_DELAY_MS = 2000
const MAX_QUEUE = 20

const allowed = new Set<string>(PRODUCT_EVENTS)
const queue: QueuedEvent[] = []
let flushTimer: number | undefined
let flushing = false

function createVisitorID() {
  const bytes = new Uint8Array(16)
  crypto.getRandomValues(bytes)
  const version = bytes[6]
  const variant = bytes[8]
  if (version !== undefined) bytes[6] = (version & 0x0f) | 0x40
  if (variant !== undefined) bytes[8] = (variant & 0x3f) | 0x80
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

function readVisitorID() {
  try {
    const saved = window.localStorage.getItem(VISITOR_KEY)
    if (saved) return saved
    const next = createVisitorID()
    window.localStorage.setItem(VISITOR_KEY, next)
    return next
  } catch {
    return createVisitorID()
  }
}

function cleanProps(input?: ProductProps) {
  if (!input) return undefined
  const out: Record<string, string | number | boolean> = {}
  for (const [key, value] of Object.entries(input)) {
    if (value === undefined || value === '') continue
    out[key] = value
  }
  return Object.keys(out).length ? out : undefined
}

function currentPage() {
  return `${window.location.pathname}${window.location.search}`
}

function authToken() {
  try {
    return window.localStorage.getItem('jwt_token')
  } catch {
    return null
  }
}

async function send(events: QueuedEvent[]) {
  if (events.length === 0) return
  const headers: Record<string, string> = { 'Content-Type': 'application/json' }
  const token = authToken()
  if (token) headers.Authorization = `Bearer ${token}`

  await fetch(`${API_BASE}/event/report`, {
    method: 'POST',
    headers,
    body: JSON.stringify({
      visitor_id: readVisitorID(),
      events,
    }),
    keepalive: true,
  })
}

export function flush() {
  if (flushing || queue.length === 0) return
  flushing = true
  const batch = queue.splice(0, MAX_QUEUE)
  void send(batch).catch(() => {
    // 埋点失败不影响主流程；丢这一批而不是塞回队列，避免失败请求自我放大。
  }).finally(() => {
    flushing = false
    if (queue.length > 0) scheduleFlush()
  })
}

function scheduleFlush() {
  if (flushTimer !== undefined) return
  flushTimer = window.setTimeout(() => {
    flushTimer = undefined
    flush()
  }, FLUSH_DELAY_MS)
}

export function track(event: ProductEventName, properties?: ProductProps) {
  if (typeof window === 'undefined' || !allowed.has(event)) return
  queue.push({
    event,
    page: currentPage(),
    ts: Date.now(),
    properties: cleanProps(properties),
  })
  if (queue.length >= MAX_QUEUE) {
    flush()
    return
  }
  scheduleFlush()
}

let listening = false

export function installAnalyticsFlush() {
  if (listening || typeof window === 'undefined') return
  listening = true
  window.addEventListener('pagehide', flush)
  window.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flush()
  })
}
