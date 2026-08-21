<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import type { Video } from '../api/types'
import * as videoApi from '../api/video'
import { useToastStore } from '../stores/toast'

/**
 * 抖音式分享识别：打开站点后读剪贴板，或在页面上粘贴，直接认出作品。
 * 不进搜索。搜索框只作为「碰巧 8 位」口令的后备，由 AppShell 处理。
 *
 * 不在输入框（评论、弹幕、密码）里抢粘贴。
 * 明文 HTTP 没有 clipboard.readText，剪贴板嗅探会静默失败，粘贴事件仍然可用。
 */
const router = useRouter()
const route = useRoute()
const toast = useToastStore()

const offer = ref<Video | null>(null)
const sourceText = ref('')

let inflight = false
let askedClipboardOnce = false
let sniffTimer = 0

function isTypingTarget(el: EventTarget | null) {
  if (!(el instanceof HTMLElement)) return false
  if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) return true
  return el.isContentEditable
}

function isSearchInput(el: EventTarget | null) {
  return el instanceof HTMLInputElement && el.classList.contains('dy-search-input')
}

function alreadyOnVideo(id: number) {
  return route.name === 'video-detail' && String(route.params.id) === String(id)
}

async function resolveCertain(text: string) {
  const video = await videoApi.resolveShare(text)
  videoApi.rememberHandledShare(text)
  return video
}

async function openVideo(video: Video) {
  offer.value = null
  sourceText.value = ''
  if (alreadyOnVideo(video.id)) return
  await router.push(`/video/${video.id}`)
}

async function onPaste(event: ClipboardEvent) {
  const text = event.clipboardData?.getData('text') ?? ''
  const confidence = videoApi.shareTextConfidence(text)
  if (confidence === 'none') return

  const target = event.target
  if (isTypingTarget(target) && !isSearchInput(target)) return
  if (confidence === 'maybe') return

  event.preventDefault()
  if (inflight) return
  inflight = true
  try {
    const video = await resolveCertain(text)
    if (alreadyOnVideo(video.id)) return
    await openVideo(video)
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '口令无效或内容已下架')
  } finally {
    inflight = false
  }
}

async function clipboardPermission(): Promise<'granted' | 'prompt' | 'denied' | 'na'> {
  const clip = navigator.clipboard
  if (!clip || typeof clip.readText !== 'function') return 'na'
  try {
    const query = await navigator.permissions.query({ name: 'clipboard-read' as PermissionName })
    if (query.state === 'granted' || query.state === 'denied' || query.state === 'prompt') {
      return query.state
    }
    return 'prompt'
  } catch {
    // Safari 没有 clipboard-read 权限查询，每次 readText 都可能弹出「允许粘贴」。
    return 'prompt'
  }
}

async function sniffClipboard() {
  const perm = await clipboardPermission()
  if (perm === 'na' || perm === 'denied') return
  if (perm === 'prompt' && askedClipboardOnce) return
  askedClipboardOnce = true

  try {
    const text = await navigator.clipboard.readText()
    if (videoApi.shareTextConfidence(text) !== 'certain') return
    if (videoApi.clipboardShareAlreadyHandled(text)) return
    if (route.name === 'share-landing') return
    if (inflight || offer.value) return
    inflight = true
    try {
      const video = await resolveCertain(text)
      if (alreadyOnVideo(video.id)) return
      sourceText.value = text
      offer.value = video
    } catch {
      videoApi.rememberHandledShare(text)
    } finally {
      inflight = false
    }
  } catch {
    // 用户拒绝粘贴授权，或明文入口没有剪贴板 API。等主动粘贴。
  }
}

function scheduleSniff() {
  window.clearTimeout(sniffTimer)
  sniffTimer = window.setTimeout(() => {
    if (document.visibilityState !== 'visible') return
    void sniffClipboard()
  }, 200)
}

function onVisibility() {
  if (document.visibilityState === 'visible') scheduleSniff()
}

function acceptOffer() {
  if (offer.value) void openVideo(offer.value)
}

function dismiss() {
  if (sourceText.value) videoApi.rememberHandledShare(sourceText.value)
  offer.value = null
  sourceText.value = ''
}

onMounted(() => {
  document.addEventListener('paste', onPaste, true)
  document.addEventListener('visibilitychange', onVisibility)
  window.addEventListener('focus', scheduleSniff)
  scheduleSniff()
})

onUnmounted(() => {
  document.removeEventListener('paste', onPaste, true)
  document.removeEventListener('visibilitychange', onVisibility)
  window.removeEventListener('focus', scheduleSniff)
  window.clearTimeout(sniffTimer)
})
</script>

<template>
  <div v-if="offer" class="inbox" role="dialog" aria-label="识别到分享内容">
    <img v-if="offer.cover_url" class="cover" :src="offer.cover_url" alt="" />
    <div class="meta">
      <p class="kicker">识别到分享内容</p>
      <p class="title">{{ offer.title }}</p>
      <p class="author">@{{ offer.username }}</p>
    </div>
    <button class="go" type="button" @click="acceptOffer">打开看看</button>
    <button class="x" type="button" aria-label="关闭" @click="dismiss">×</button>
  </div>
</template>

<style scoped>
.inbox {
  position: fixed;
  left: 50%;
  bottom: 24px;
  z-index: 160;
  transform: translateX(-50%);
  width: min(420px, calc(100vw - 24px));
  display: grid;
  grid-template-columns: 56px 1fr auto auto;
  gap: 10px;
  align-items: center;
  padding: 10px 10px 10px 10px;
  border-radius: 16px;
  border: 1px solid rgba(var(--fg), 0.14);
  background: var(--surface);
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(16px);
}

.cover {
  width: 56px;
  height: 74px;
  border-radius: 10px;
  object-fit: cover;
  background: rgba(var(--fg), 0.06);
}

.meta {
  min-width: 0;
}

.kicker {
  margin: 0 0 4px;
  color: #fe2c55;
  font-size: 12px;
  font-weight: 700;
}

.title {
  margin: 0;
  font-size: 14px;
  font-weight: 700;
  line-height: 1.35;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.author {
  margin: 4px 0 0;
  color: rgba(var(--fg), 0.56);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.go {
  border: 0;
  border-radius: 999px;
  padding: 8px 12px;
  background: #fe2c55;
  color: #fff;
  font-size: 13px;
  font-weight: 700;
  cursor: pointer;
  white-space: nowrap;
}

.x {
  width: 32px;
  height: 32px;
  border-radius: 10px;
  border: 1px solid rgba(var(--fg), 0.14);
  background: rgba(var(--fg), 0.06);
  color: rgba(var(--fg), 0.88);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
}

@media (max-width: 900px) {
  .inbox {
    bottom: calc(var(--bottom-nav-h, 56px) + var(--safe-bottom, 0px) + 12px);
  }
}
</style>
