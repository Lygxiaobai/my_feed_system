<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import * as danmakuApi from '../api/danmaku'
import type { DanmakuItem } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const TRACK_COUNT = 8
const MAX_CONTENT = 50
const FLY_MS = 7500
// 一条弹幕还在飞的时间窗。中途进来或列表晚到时，只补还在画面上的，不把更早的一次倒完。
const CATCHUP_MS = 7800
const COLORS = ['#ffffff', '#ffe082', '#80d8ff', '#ffab91', '#c5e1a5', '#f8bbd0']
const listCache = new Map<number, DanmakuItem[]>()

const props = defineProps<{
  videoId: number
  currentTime: number
  playing: boolean
  enabled: boolean
}>()

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const layerEl = ref<HTMLDivElement | null>(null)
const travelPx = ref(360)
const draft = ref('')
const sending = ref(false)
const items = ref<DanmakuItem[]>([])
const flying = ref<FlyingItem[]>([])

type FlyingItem = {
  key: string
  id: number
  content: string
  color: string
  mine: boolean
  live: boolean
  track: number
  duration: number
  delay: number
}

let lastMs = -1
const spawned = new Set<number>()
const trackFreeAt = Array.from({ length: TRACK_COUNT }, () => 0)
let nextTempId = -1
let resizeObserver: ResizeObserver | null = null

const canSend = computed(() => draft.value.trim().length > 0 && !sending.value)
const myAccountId = computed(() => auth.claims?.account_id ?? 0)

function measureTravel() {
  travelPx.value = Math.max(240, layerEl.value?.clientWidth ?? 360)
}

function flyDuration(content: string) {
  return Math.min(9000, Math.max(6200, FLY_MS + content.length * 40))
}

function colorFor(id: number) {
  if (id < 0) return '#ffffff'
  return COLORS[Math.abs(id) % COLORS.length] ?? '#ffffff'
}

function resetPlaybackCursor(startMs: number) {
  lastMs = startMs
  spawned.clear()
  flying.value = []
  for (let i = 0; i < TRACK_COUNT; i += 1) trackFreeAt[i] = 0
}

function pickTrack() {
  const now = performance.now()
  let best = 0
  for (let i = 0; i < TRACK_COUNT; i += 1) {
    const freeAt = trackFreeAt[i] ?? 0
    if (freeAt <= now) return i
    if (freeAt < (trackFreeAt[best] ?? 0)) best = i
  }
  return best
}

function spawnItem(item: DanmakuItem, live = false, elapsedMs = 0) {
  if (spawned.has(item.id)) return
  const duration = flyDuration(item.content)
  if (elapsedMs >= duration) {
    spawned.add(item.id)
    return
  }
  spawned.add(item.id)
  const track = pickTrack()
  const remaining = duration - elapsedMs
  trackFreeAt[track] = performance.now() + remaining * 0.42
  const mine = !!myAccountId.value && myAccountId.value === item.author_id
  flying.value.push({
    key: `${item.id}-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`,
    id: item.id,
    content: item.content,
    color: mine ? '#ffffff' : colorFor(item.id),
    mine,
    live,
    track,
    duration,
    delay: -elapsedMs,
  })
}

function spawnWindow(tMs: number) {
  for (const item of items.value) {
    const elapsed = tMs - item.offset_ms
    if (elapsed < 0) continue
    if (elapsed > CATCHUP_MS) {
      spawned.add(item.id)
      continue
    }
    spawnItem(item, false, elapsed)
  }
  lastMs = tMs
}

function syncToTime(seconds: number) {
  if (!props.enabled) return
  const tMs = Math.max(0, Math.round(seconds * 1000))

  // 首次对齐、循环或回退：按当前进度补还在飞的，而不是从零开始漏掉画面上该有的。
  if (lastMs < 0 || tMs + 800 < lastMs) {
    resetPlaybackCursor(tMs)
    spawnWindow(tMs)
    return
  }

  // 大幅快进同样只补新位置还在飞的，避免把中间整段倒出来。
  if (tMs > lastMs + 2000) {
    resetPlaybackCursor(tMs)
    spawnWindow(tMs)
    return
  }

  for (const item of items.value) {
    if (item.offset_ms > lastMs && item.offset_ms <= tMs) spawnItem(item)
  }
  lastMs = tMs
}

function applyItems(next: DanmakuItem[]) {
  items.value = next
  if (!props.enabled) return
  // 列表可能在播放已经开始之后才到。只补还在飞的，不清掉已经发出去的（含刚发送的本地条）。
  spawnWindow(Math.max(0, Math.round(props.currentTime * 1000)))
}

async function loadItems(videoId: number) {
  const cached = listCache.get(videoId)
  if (cached) applyItems(cached)
  try {
    const next = await danmakuApi.listByVideo(videoId)
    if (props.videoId !== videoId) return
    listCache.set(videoId, next)
    applyItems(next)
  } catch {
    if (props.videoId === videoId && !cached) items.value = []
  }
}

async function needLogin() {
  toast.error('请先登录')
  await router.push('/account')
}

async function submit() {
  const content = draft.value.trim()
  if (!content || sending.value) return
  if (!auth.isLoggedIn) return void needLogin()

  const videoId = props.videoId
  const offsetMs = Math.max(0, Math.round(props.currentTime * 1000))
  const temp: DanmakuItem = {
    id: nextTempId,
    video_id: videoId,
    author_id: myAccountId.value,
    username: '',
    content,
    offset_ms: offsetMs,
    created_at: new Date().toISOString(),
  }
  nextTempId -= 1
  sending.value = true
  draft.value = ''
  spawnItem(temp, true)

  try {
    const saved = await danmakuApi.send(videoId, content, offsetMs)
    if (props.videoId !== videoId) return
    spawned.add(saved.id)
    const next = [...items.value.filter((item) => item.id !== temp.id), saved].sort((a, b) => {
      if (a.offset_ms !== b.offset_ms) return a.offset_ms - b.offset_ms
      return a.id - b.id
    })
    items.value = next
    listCache.set(videoId, next)
  } catch (e) {
    flying.value = flying.value.filter((item) => item.id !== temp.id)
    draft.value = content
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    if (props.videoId === videoId) sending.value = false
  }
}

function onAnimationEnd(key: string) {
  flying.value = flying.value.filter((item) => item.key !== key)
}

function onComposerClick() {
  if (!auth.isLoggedIn) void needLogin()
}

watch(
  () => props.videoId,
  (videoId) => {
    resetPlaybackCursor(-1)
    items.value = listCache.get(videoId) ?? []
    void loadItems(videoId)
  },
)

watch(
  () => props.currentTime,
  (seconds) => {
    syncToTime(seconds)
  },
)

watch(
  () => props.enabled,
  (enabled) => {
    if (!enabled) {
      resetPlaybackCursor(-1)
      return
    }
    resetPlaybackCursor(-1)
    syncToTime(props.currentTime)
  },
)

onMounted(() => {
  measureTravel()
  if (typeof ResizeObserver !== 'undefined' && layerEl.value) {
    resizeObserver = new ResizeObserver(() => measureTravel())
    resizeObserver.observe(layerEl.value)
  }
  void loadItems(props.videoId)
  void nextTick(() => syncToTime(props.currentTime))
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
})
</script>

<template>
  <div v-if="enabled" ref="layerEl" class="danmaku-root">
    <div class="stage" :style="{ '--travel': `${travelPx}px` }" aria-hidden="true">
      <span
        v-for="item in flying"
        :key="item.key"
        class="item"
        :class="{ mine: item.mine, live: item.live }"
        :style="{
          '--track': String(item.track),
          '--dur': `${item.duration}ms`,
          '--delay': `${item.delay}ms`,
          '--play': item.live || playing ? 'running' : 'paused',
          color: item.color,
        }"
        @animationend="onAnimationEnd(item.key)"
      >
        {{ item.content }}
      </span>
    </div>

    <form class="composer" @click.stop @dblclick.stop.prevent @submit.prevent="submit">
      <span class="mark" aria-hidden="true">弹</span>
      <input
        v-model="draft"
        class="box"
        type="text"
        :maxlength="MAX_CONTENT"
        autocomplete="off"
        enterkeyhint="send"
        :placeholder="auth.isLoggedIn ? '善语结善缘，恶语伤人心' : '登录后发弹幕'"
        @focus="onComposerClick"
      />
      <button class="send" type="submit" :disabled="!canSend">发送</button>
    </form>
  </div>
</template>

<style scoped>
.danmaku-root {
  position: absolute;
  inset: 0;
  z-index: 2;
  pointer-events: none;
}

.stage {
  position: absolute;
  left: 0;
  right: 0;
  top: 8%;
  height: 46%;
  overflow: hidden;
}

.item {
  position: absolute;
  left: 100%;
  top: calc(var(--track) * 12.2%);
  white-space: nowrap;
  font-family: 'PingFang SC', 'Hiragino Sans GB', 'Microsoft YaHei', sans-serif;
  font-size: 17px;
  font-weight: 700;
  line-height: 1.35;
  letter-spacing: 0.4px;
  -webkit-text-stroke: 0.7px rgba(0, 0, 0, 0.82);
  paint-order: stroke fill;
  text-shadow:
    0 1px 2px rgba(0, 0, 0, 0.88),
    0 0 5px rgba(0, 0, 0, 0.5);
  animation: danmaku-fly var(--dur) linear var(--delay, 0ms) forwards;
  animation-play-state: var(--play, running);
  will-change: transform;
}

.item.mine {
  padding: 0 8px;
  border: 1px solid rgba(37, 244, 238, 0.92);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.22);
  -webkit-text-stroke: 0;
  text-shadow:
    0 1px 2px rgba(0, 0, 0, 0.85),
    0 0 6px rgba(0, 0, 0, 0.55);
}

@keyframes danmaku-fly {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(calc(-100% - var(--travel)));
  }
}

.composer {
  pointer-events: auto;
  position: absolute;
  left: 16px;
  right: 92px;
  bottom: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.mark {
  flex: none;
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: rgba(254, 44, 85, 0.92);
  color: #fff;
  font-size: 13px;
  font-weight: 800;
}

.box {
  flex: 1;
  min-width: 0;
  height: 38px;
  border: 1px solid rgba(255, 255, 255, 0.16);
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.48);
  color: rgba(255, 255, 255, 0.92);
  padding: 0 14px;
  outline: none;
  font-size: 13px;
}

.box::placeholder {
  color: rgba(255, 255, 255, 0.52);
}

.send {
  flex: none;
  height: 38px;
  padding: 0 14px;
  border-radius: 999px;
  border: 1px solid rgba(254, 44, 85, 0.45);
  background: rgba(254, 44, 85, 0.88);
  color: white;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
}

.send:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

@media (max-width: 900px) {
  .composer {
    left: 12px;
    right: 80px;
    bottom: calc(10px + env(safe-area-inset-bottom, 0px));
  }

  .item {
    font-size: 16px;
  }

  .box {
    font-size: 12px;
  }
}
</style>
