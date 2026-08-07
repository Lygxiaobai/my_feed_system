<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

export type PlaybackStatus = 'idle' | 'loading' | 'ready' | 'playing' | 'paused' | 'buffering' | 'error'

export type VideoPlayerHandle = {
  play: () => Promise<boolean>
  pause: () => void
  toggle: () => Promise<boolean>
  setMuted: (muted: boolean) => void
  isPaused: () => boolean
}

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
const status = ref<PlaybackStatus>('idle')
const errorMessage = ref('')

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

function onLoadedMetadata() {
  if (status.value !== 'playing') setStatus('ready')
}

function onCanPlay() {
  if (status.value !== 'playing') setStatus('ready')
}

function onPlaying() {
  setStatus('playing')
}

function onPause() {
  if (status.value !== 'error') setStatus('paused')
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
      @waiting="onWaiting"
      @stalled="onWaiting"
      @error="onError"
    />

    <div v-if="status === 'loading' || status === 'buffering'" class="status" aria-live="polite">
      {{ status === 'buffering' ? '缓冲中…' : '加载中…' }}
    </div>
    <div v-else-if="status === 'paused' && active" class="paused-indicator" aria-hidden="true">▶</div>
    <button v-if="status === 'error'" class="retry" type="button" @click.stop="retry">重试</button>
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
</style>
