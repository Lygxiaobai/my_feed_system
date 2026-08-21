<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

export type PlaybackStatus = 'idle' | 'loading' | 'ready' | 'playing' | 'paused' | 'buffering' | 'error'

export type VideoPlayerHandle = {
  play: () => Promise<boolean>
  pause: () => void
  toggle: () => Promise<boolean>
  setMuted: (muted: boolean) => void
  isPaused: () => boolean
  currentTime: () => number
  duration: () => number
  seek: (seconds: number) => void
}

const emit = defineEmits<{
  playing: []
  paused: []
  timeupdate: [seconds: number]
}>()

const props = withDefaults(
  defineProps<{
    src: string
    poster?: string
    active?: boolean
    enabled?: boolean
    muted?: boolean
  }>(),
  {
    poster: '',
    active: false,
    enabled: true,
    muted: true,
  },
)

const videoEl = ref<HTMLVideoElement | null>(null)
const trackEl = ref<HTMLDivElement | null>(null)
const status = ref<PlaybackStatus>('idle')
const errorMessage = ref('')
const playhead = ref(0)
const total = ref(0)
const scrubbing = ref(false)
let pendingSeek: number | null = null

function formatClock(seconds: number) {
  if (!Number.isFinite(seconds) || seconds < 0) return '00:00'
  const whole = Math.floor(seconds)
  const hours = Math.floor(whole / 3600)
  const minutes = Math.floor((whole % 3600) / 60)
  const rest = whole % 60
  const mm = String(minutes).padStart(2, '0')
  const ss = String(rest).padStart(2, '0')
  return hours > 0 ? `${hours}:${mm}:${ss}` : `${mm}:${ss}`
}

const playedPercent = computed(() => {
  if (total.value <= 0) return 0
  return Math.min(100, Math.max(0, (playhead.value / total.value) * 100))
})

function syncClock() {
  if (!scrubbing.value) playhead.value = currentTime()
  total.value = duration()
}

function setStatus(nextStatus: PlaybackStatus) {
  status.value = nextStatus
}

async function play() {
  const video = videoEl.value
  if (!video || !props.enabled || !props.src) return false

  video.muted = props.muted
  errorMessage.value = ''
  if (video.readyState < 3) setStatus('loading')

  try {
    await video.play()
    setStatus('playing')
    return true
  } catch (error) {
    // 切换视频时主动暂停会触发 AbortError，不应把正常切换显示成播放失败。
    if (error instanceof DOMException && error.name === 'AbortError' && !props.active) {
      return false
    }
    errorMessage.value = '视频暂时无法播放'
    setStatus('error')
    return false
  }
}

function pause() {
  const video = videoEl.value
  if (!video) return
  video.pause()
  if (status.value !== 'error') setStatus('paused')
}

async function toggle() {
  if (videoEl.value?.paused) return play()
  pause()
  return true
}

function setMuted(muted: boolean) {
  if (videoEl.value) videoEl.value.muted = muted
}

function isPaused() {
  return videoEl.value?.paused ?? true
}

function retry() {
  const video = videoEl.value
  if (!video || !props.enabled || !props.src) return
  errorMessage.value = ''
  setStatus('loading')
  video.load()
  if (props.active) void play()
}

function onLoadStart() {
  setStatus('loading')
}

function applyPendingSeek() {
  const video = videoEl.value
  if (!video || pendingSeek == null) return
  if (video.readyState < 1) return
  const durationValue = duration()
  const next = durationValue > 0 ? Math.min(pendingSeek, Math.max(0, durationValue - 0.05)) : pendingSeek
  video.currentTime = next
  playhead.value = next
  pendingSeek = null
}

function seek(seconds: number) {
  // 进度条拖回开头是合法操作，0 必须能 seek。
  if (!Number.isFinite(seconds) || seconds < 0) return
  pendingSeek = seconds
  applyPendingSeek()
}

function secondsFromPointer(event: PointerEvent) {
  const el = trackEl.value
  if (!el || total.value <= 0) return 0
  const rect = el.getBoundingClientRect()
  if (rect.width <= 0) return 0
  const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width))
  return ratio * total.value
}

function onScrubStart(event: PointerEvent) {
  if (total.value <= 0) return
  event.preventDefault()
  trackEl.value?.setPointerCapture(event.pointerId)
  scrubbing.value = true
  const next = secondsFromPointer(event)
  playhead.value = next
  seek(next)
}

function onScrubMove(event: PointerEvent) {
  if (!scrubbing.value) return
  const next = secondsFromPointer(event)
  playhead.value = next
  seek(next)
}

function onScrubEnd(event: PointerEvent) {
  if (!scrubbing.value) return
  scrubbing.value = false
  if (trackEl.value?.hasPointerCapture(event.pointerId)) {
    trackEl.value.releasePointerCapture(event.pointerId)
  }
}

function onLoadedMetadata() {
  if (status.value !== 'playing') setStatus('ready')
  syncClock()
  applyPendingSeek()
}

function onCanPlay() {
  if (status.value !== 'playing') setStatus('ready')
}

function onPlaying() {
  setStatus('playing')
  emit('playing')
}

function currentTime() {
  return videoEl.value?.currentTime ?? 0
}

function duration() {
  const value = videoEl.value?.duration
  return Number.isFinite(value) ? (value as number) : 0
}

function onPause() {
  if (status.value !== 'error') setStatus('paused')
  emit('paused')
}

function onTimeUpdate() {
  syncClock()
  emit('timeupdate', currentTime())
}

function onWaiting() {
  if (!videoEl.value?.paused) setStatus('buffering')
}

function onError() {
  errorMessage.value = '视频暂时无法播放'
  setStatus('error')
}

async function syncSource() {
  await nextTick()
  const video = videoEl.value
  if (!video) return

  video.muted = props.muted
  if (!props.enabled || !props.src) {
    pendingSeek = null
    playhead.value = 0
    total.value = 0
    video.pause()
    video.removeAttribute('src')
    video.load()
    setStatus('idle')
    return
  }

  if (props.active) void play()
}

watch(
  [() => props.src, () => props.enabled],
  () => {
    pendingSeek = null
    playhead.value = 0
    total.value = 0
    void syncSource()
  },
  { immediate: true },
)

watch(
  () => props.active,
  (active) => {
    if (active) void play()
    else pause()
  },
)

watch(
  () => props.muted,
  (muted) => {
    setMuted(muted)
  },
)

onBeforeUnmount(() => {
  videoEl.value?.pause()
  videoEl.value?.removeAttribute('src')
})

defineExpose<VideoPlayerHandle>({
  play,
  pause,
  toggle,
  setMuted,
  isPaused,
  currentTime,
  duration,
  seek,
})
</script>

<template>
  <div class="video-player">
    <video
      ref="videoEl"
      class="video"
      :src="enabled ? src : undefined"
      :poster="poster"
      :preload="active ? 'auto' : 'metadata'"
      playsinline
      loop
      @loadstart="onLoadStart"
      @loadedmetadata="onLoadedMetadata"
      @canplay="onCanPlay"
      @playing="onPlaying"
      @pause="onPause"
      @timeupdate="onTimeUpdate"
      @durationchange="syncClock"
      @waiting="onWaiting"
      @stalled="onWaiting"
      @error="onError"
    />

    <div v-if="status === 'loading' || status === 'buffering'" class="status" aria-live="polite">
      {{ status === 'buffering' ? '缓冲中…' : '加载中…' }}
    </div>
    <div v-else-if="status === 'paused' && active" class="paused-indicator" aria-hidden="true">▶</div>
    <button v-if="status === 'error'" class="retry" type="button" @click.stop="retry">重试</button>

    <!-- 点进度条不能冒泡到舞台，否则会误触暂停或双击点赞。 -->
    <div
      v-if="active && enabled && src"
      class="chrome"
      @click.stop
      @dblclick.stop.prevent
    >
      <span class="clock">{{ formatClock(playhead) }} / {{ formatClock(total) }}</span>
      <div
        ref="trackEl"
        class="track"
        :class="{ dragging: scrubbing }"
        role="slider"
        aria-label="播放进度"
        :aria-valuemin="0"
        :aria-valuemax="Math.round(total)"
        :aria-valuenow="Math.round(playhead)"
        :aria-valuetext="`${formatClock(playhead)} / ${formatClock(total)}`"
        @pointerdown="onScrubStart"
        @pointermove="onScrubMove"
        @pointerup="onScrubEnd"
        @pointercancel="onScrubEnd"
      >
        <div class="track-line">
          <div class="played" :style="{ width: `${playedPercent}%` }" />
          <i class="knob" :style="{ left: `${playedPercent}%` }" aria-hidden="true" />
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.video-player {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.4);
}

.video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  background: rgba(0, 0, 0, 0.4);
}

.status,
.paused-indicator,
.retry {
  position: absolute;
  z-index: 1;
}

.status {
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  padding: 8px 12px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.55);
  color: rgba(255, 255, 255, 0.88);
  font-size: 13px;
  pointer-events: none;
}

.paused-indicator {
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  width: 54px;
  height: 54px;
  display: grid;
  place-items: center;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  color: white;
  font-size: 22px;
  pointer-events: none;
}

.retry {
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  padding: 8px 14px;
  border: 1px solid rgba(255, 255, 255, 0.3);
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.65);
  color: white;
  cursor: pointer;
}

.chrome {
  position: absolute;
  left: 16px;
  right: 92px;
  bottom: 56px;
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 10px;
  pointer-events: none;
  user-select: none;
}

.clock {
  flex: none;
  color: rgba(255, 255, 255, 0.92);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.75);
  white-space: nowrap;
}

.track {
  flex: 1;
  min-width: 0;
  height: 16px;
  display: flex;
  align-items: center;
  pointer-events: auto;
  cursor: pointer;
  touch-action: none;
}

.track-line {
  width: 100%;
  height: 2px;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.28);
  position: relative;
}

.track:hover .track-line,
.track.dragging .track-line {
  height: 4px;
}

.played {
  height: 100%;
  width: 0;
  border-radius: 999px;
  background: rgba(255, 255, 255, 0.92);
}

.knob {
  position: absolute;
  top: 50%;
  width: 10px;
  height: 10px;
  margin: -5px 0 0 -5px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.25);
  opacity: 0;
  pointer-events: none;
}

.track:hover .knob,
.track.dragging .knob {
  opacity: 1;
}

@media (max-width: 900px) {
  .chrome {
    left: 12px;
    right: 80px;
    bottom: calc(52px + env(safe-area-inset-bottom, 0px));
  }
}
</style>
