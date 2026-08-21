<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import * as dmApi from '../api/dm'
import type { DMConversation, DMMessage, DMRelation, DMThread } from '../api/dm'
import { useAuthStore } from '../stores/auth'
import { useDMStore } from '../stores/dm'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import UserAvatar from './UserAvatar.vue'

const props = withDefaults(
  defineProps<{
    variant?: 'dropdown' | 'page'
  }>(),
  { variant: 'page' },
)

const emit = defineEmits<{
  close: []
}>()

const INBOX_POLL_MS = 8_000
const THREAD_POLL_MS = 3_000
const GAP_MS = 5 * 60 * 1000
const WEEKDAYS = ['星期日', '星期一', '星期二', '星期三', '星期四', '星期五', '星期六']

const auth = useAuthStore()
const dm = useDMStore()
const social = useSocialStore()
const toast = useToastStore()
const route = useRoute()
const router = useRouter()

const inbox = ref<DMConversation[]>([])
const inboxLoading = ref(false)
const thread = ref<DMThread>(emptyThread())
const threadLoading = ref(false)
const draft = ref('')
const sending = ref(false)
const followBusy = ref(false)
const scroller = ref<HTMLElement | null>(null)
const stickToBottom = ref(true)
const localPeerId = ref(0)

let inboxTimer = 0
let threadTimer = 0
let inboxInflight = false
let threadInflight = false

function emptyThread(): DMThread {
  return {
    peer: { id: 0, username: '' },
    relation: 'none',
    can_send: false,
    remaining: 0,
    messages: [],
    has_more: false,
  }
}

function routePeerId() {
  const raw = route.query.u
  const n = typeof raw === 'string' ? Number(raw) : 0
  return Number.isFinite(n) && n > 0 ? n : 0
}

const peerId = computed(() => (props.variant === 'dropdown' ? dm.activePeerId : localPeerId.value))
const chatOpen = computed(() => peerId.value > 0)
const lastMineId = computed(() => {
  const mine = thread.value.messages.filter((item) => item.mine)
  const last = mine[mine.length - 1]
  return last?.id ?? 0
})
const railItems = computed(() => {
  const items = inbox.value.slice()
  const openId = peerId.value
  if (openId && !items.some((item) => item.peer.id === openId) && thread.value.peer.id === openId) {
    items.unshift({
      peer: thread.value.peer,
      preview: '',
      unread: 0,
      last_at: '',
      last_sender_id: 0,
    })
  }
  return items
})

function relationLabel(relation: DMRelation) {
  switch (relation) {
    case 'friend':
      return '互相关注'
    case 'following':
      return '已关注'
    case 'follower':
      return '对方关注了你'
    default:
      return '陌生人'
  }
}

function quotaHint(row: DMThread) {
  if (row.relation === 'friend') return ''
  if (row.can_send) return '未互关只能发送一条私信'
  if (row.relation === 'following') return '等待对方回关后即可继续聊天'
  return '关注对方并互关后即可继续聊天'
}

function formatListTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  if (d.toDateString() === now.toDateString()) {
    return d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  }
  const startOfToday = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime()
  const startOfThat = new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const dayDiff = Math.round((startOfToday - startOfThat) / 86_400_000)
  if (dayDiff > 0 && dayDiff < 7) {
    return WEEKDAYS[d.getDay()] ?? d.toLocaleDateString('zh-CN', { month: 'numeric', day: 'numeric' })
  }
  const mm = String(d.getMonth() + 1).padStart(2, '0')
  const dd = String(d.getDate()).padStart(2, '0')
  return `${mm}-${dd}`
}

function formatChatTime(iso: string) {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const opts: Intl.DateTimeFormatOptions = { hour: '2-digit', minute: '2-digit' }
  if (d.toDateString() === now.toDateString()) return d.toLocaleTimeString('zh-CN', opts)
  return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', ...opts })
}

function showTimeDivider(index: number, item: DMMessage) {
  if (index === 0) return true
  const prev = thread.value.messages[index - 1]
  if (!prev) return true
  return new Date(item.created_at).getTime() - new Date(prev.created_at).getTime() > GAP_MS
}

function mergeInbox(items: DMConversation[]) {
  inbox.value = items ?? []
}

function mergeThread(next: DMThread, mode: 'replace' | 'append') {
  if (mode === 'replace') {
    thread.value = next
    return
  }
  const seen = new Set(thread.value.messages.map((item) => item.id))
  const extra = next.messages.filter((item) => !seen.has(item.id))
  thread.value = {
    ...next,
    messages: thread.value.messages.concat(extra),
  }
}

async function refreshInbox() {
  if (!auth.isLoggedIn || inboxInflight) return
  inboxInflight = true
  try {
    const res = await dmApi.listInbox()
    mergeInbox(res.items ?? [])
    const unread = (res.items ?? []).reduce((sum, item) => sum + (Number(item.unread) || 0), 0)
    dm.applyUnread(unread)
  } catch (e) {
    if (inbox.value.length === 0) {
      toast.error(e instanceof ApiError ? e.message : '会话列表加载失败')
    }
  } finally {
    inboxInflight = false
    inboxLoading.value = false
  }
}

async function loadThread(mode: 'replace' | 'append') {
  if (!auth.isLoggedIn || peerId.value <= 0 || threadInflight) return
  threadInflight = true
  if (mode === 'replace') threadLoading.value = true
  try {
    const latest = thread.value.messages[thread.value.messages.length - 1]
    const afterId = mode === 'append' ? (latest?.id ?? 0) : 0
    const res = await dmApi.openThread({
      peer_id: peerId.value,
      after_id: afterId || undefined,
    })
    mergeThread(res, afterId ? 'append' : 'replace')
    await refreshInbox()
    if (stickToBottom.value || mode === 'replace') {
      await scrollToBottom()
    }
  } catch (e) {
    if (mode === 'replace') {
      thread.value = emptyThread()
      toast.error(e instanceof ApiError ? e.message : '会话加载失败')
    }
  } finally {
    threadInflight = false
    threadLoading.value = false
  }
}

async function scrollToBottom() {
  await nextTick()
  const el = scroller.value
  if (!el) return
  el.scrollTop = el.scrollHeight
}

function onScroll() {
  const el = scroller.value
  if (!el) return
  stickToBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 80
}

function setPeer(id: number) {
  stickToBottom.value = true
  draft.value = ''
  if (props.variant === 'dropdown') {
    dm.setActivePeer(id)
    return
  }
  localPeerId.value = id
  const query = id > 0 ? { u: String(id) } : {}
  void router.replace({ path: '/messages', query })
}

function openPeer(id: number) {
  if (id <= 0) return
  thread.value = emptyThread()
  setPeer(id)
}

function closeChat() {
  thread.value = emptyThread()
  setPeer(0)
}

function closePanel() {
  closeChat()
  emit('close')
}

async function send() {
  const text = draft.value.trim()
  if (!text || !peerId.value || sending.value || !thread.value.can_send) return
  sending.value = true
  try {
    const res = await dmApi.sendMessage(peerId.value, text)
    draft.value = ''
    thread.value = {
      ...thread.value,
      relation: res.relation,
      can_send: res.can_send,
      remaining: res.remaining,
      messages: thread.value.messages.concat(res.message),
    }
    stickToBottom.value = true
    await scrollToBottom()
    await refreshInbox()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '发送失败')
    if (e instanceof ApiError && e.code === 'A0503') {
      thread.value = { ...thread.value, can_send: false, remaining: 0 }
    }
  } finally {
    sending.value = false
  }
}

async function followPeer() {
  const id = peerId.value
  if (!id || followBusy.value || social.isPending(id)) return
  followBusy.value = true
  try {
    await social.follow(id, thread.value.peer.username)
    await loadThread('replace')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '关注失败')
  } finally {
    followBusy.value = false
  }
}

async function goProfile() {
  if (peerId.value <= 0) return
  if (props.variant === 'dropdown') emit('close')
  await router.push(`/u/${peerId.value}`)
}

function startTimers() {
  stopTimers()
  inboxTimer = window.setInterval(() => {
    if (document.visibilityState === 'hidden') return
    void refreshInbox()
  }, INBOX_POLL_MS)
  threadTimer = window.setInterval(() => {
    if (document.visibilityState === 'hidden' || !chatOpen.value) return
    void loadThread('append')
  }, THREAD_POLL_MS)
}

function stopTimers() {
  if (inboxTimer) window.clearInterval(inboxTimer)
  if (threadTimer) window.clearInterval(threadTimer)
  inboxTimer = 0
  threadTimer = 0
}

watch(
  peerId,
  async (id, prev) => {
    if (!auth.isLoggedIn) return
    if (id <= 0) {
      thread.value = emptyThread()
      return
    }
    if (id !== prev) {
      thread.value = emptyThread()
      stickToBottom.value = true
      await loadThread('replace')
    }
  },
)

onMounted(async () => {
  if (props.variant === 'page') {
    localPeerId.value = routePeerId()
  }
  if (!auth.isLoggedIn) return
  inboxLoading.value = true
  await refreshInbox()
  if (peerId.value) await loadThread('replace')
  startTimers()
})

watch(
  () => route.query.u,
  () => {
    if (props.variant !== 'page') return
    localPeerId.value = routePeerId()
  },
)

onUnmounted(stopTimers)
</script>

<template>
  <section class="m-panel" :class="[props.variant, { chat: chatOpen }]">
    <div v-if="!chatOpen" class="m-list">
      <header class="m-head">
        <div class="m-title">私信</div>
      </header>
      <p v-if="inboxLoading && railItems.length === 0" class="m-empty">正在加载…</p>
      <p v-else-if="railItems.length === 0" class="m-empty">还没有会话。从资料页点「发私信」开始。</p>
      <div v-else class="m-rows">
        <button
          v-for="item in railItems"
          :key="item.peer.id"
          class="m-row"
          type="button"
          :class="{ unread: item.unread > 0 }"
          @click="openPeer(item.peer.id)"
        >
          <span class="m-avatar">
            <UserAvatar :username="item.peer.username" :id="item.peer.id" :size="44" />
          </span>
          <span class="m-main">
            <span class="m-name">{{ item.peer.username }}</span>
            <span class="m-preview">{{ item.preview || ' ' }}</span>
          </span>
          <span class="m-side">
            <span class="m-time">{{ formatListTime(item.last_at) }}</span>
            <span v-if="item.unread > 0" class="m-dot" aria-label="未读" />
          </span>
        </button>
      </div>
    </div>

    <div v-else class="m-chat">
      <aside v-if="props.variant === 'dropdown'" class="m-rail">
        <button
          v-for="item in railItems"
          :key="item.peer.id"
          class="m-rail-item"
          type="button"
          :class="{ on: item.peer.id === peerId }"
          :title="item.peer.username"
          @click="openPeer(item.peer.id)"
        >
          <UserAvatar :username="item.peer.username" :id="item.peer.id" :size="44" />
          <span v-if="item.unread > 0" class="m-rail-dot" />
        </button>
      </aside>

      <div class="m-thread">
        <header class="m-chat-head">
          <button v-if="props.variant === 'page'" class="m-back" type="button" aria-label="返回会话列表" @click="closeChat">
            ‹
          </button>
          <button class="m-who" type="button" @click="goProfile">
            <UserAvatar :username="thread.peer.username || '用户'" :id="peerId" :size="32" />
            <span>
              <strong>{{ thread.peer.username || '用户' }}</strong>
              <small>{{ relationLabel(thread.relation) }}</small>
            </span>
          </button>
          <button class="m-ghost" type="button" @click="closeChat">关闭会话</button>
          <button v-if="props.variant === 'dropdown'" class="m-x" type="button" aria-label="关闭" @click="closePanel">
            ×
          </button>
        </header>

        <div ref="scroller" class="m-transcript" @scroll="onScroll">
          <div v-if="threadLoading && thread.messages.length === 0" class="m-empty">加载中…</div>
          <template v-for="(msg, index) in thread.messages" :key="msg.id">
            <div v-if="showTimeDivider(index, msg)" class="m-when">{{ formatChatTime(msg.created_at) }}</div>
            <div class="m-bubble-row" :class="{ mine: msg.mine }">
              <UserAvatar
                v-if="!msg.mine"
                :username="thread.peer.username || '用户'"
                :id="thread.peer.id"
                :size="28"
              />
              <div class="m-bubble-col">
                <div class="m-bubble">{{ msg.body }}</div>
                <div v-if="msg.mine && msg.id === lastMineId" class="m-receipt">
                  {{ msg.read ? '已读' : '' }}
                </div>
              </div>
            </div>
          </template>
          <div v-if="!threadLoading && thread.messages.length === 0" class="m-empty">发一条消息开始聊天</div>
        </div>

        <footer class="m-composer">
          <p v-if="quotaHint(thread)" class="m-quota">
            {{ quotaHint(thread) }}
            <button
              v-if="!thread.can_send && thread.relation !== 'following' && thread.relation !== 'friend'"
              type="button"
              :disabled="followBusy || social.isPending(peerId)"
              @click="followPeer"
            >
              关注
            </button>
          </p>
          <div class="m-box">
            <input
              v-model="draft"
              maxlength="500"
              :disabled="!thread.can_send || sending"
              :placeholder="thread.can_send ? '发送消息' : '暂时不能发送'"
              @keydown.enter.prevent="send"
            />
            <button class="m-send" type="button" :disabled="!thread.can_send || sending || !draft.trim()" @click="send">
              ↑
            </button>
          </div>
        </footer>
      </div>
    </div>
  </section>
</template>

<style scoped>
.m-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  color: rgba(255, 255, 255, 0.92);
}

.m-panel.dropdown {
  width: min(400px, calc(100vw - 24px));
  max-height: min(560px, calc(100dvh - 80px));
  background: rgba(22, 22, 28, 0.96);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(18px);
  overflow: hidden;
}

.m-panel.dropdown.chat {
  width: min(720px, calc(100vw - 24px));
  height: min(640px, calc(100dvh - 80px));
  max-height: min(640px, calc(100dvh - 80px));
}

.m-panel.page {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  overflow: hidden;
  min-height: min(640px, calc(100dvh - 140px));
}

.m-panel.page.chat {
  min-height: min(640px, calc(100dvh - 140px));
}

.m-list,
.m-chat {
  min-height: 0;
  flex: 1;
  display: flex;
  flex-direction: column;
}

.m-head,
.m-chat-head {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.m-title {
  font-weight: 800;
  font-size: 15px;
}

.m-rows {
  overflow: auto;
  min-height: 0;
}

.m-row {
  width: 100%;
  display: grid;
  grid-template-columns: 48px minmax(0, 1fr) auto;
  gap: 10px;
  align-items: center;
  padding: 12px 14px;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.m-row:hover,
.m-row.unread {
  background: rgba(255, 255, 255, 0.05);
}

.m-avatar {
  display: grid;
}

.m-main {
  min-width: 0;
  display: grid;
  gap: 2px;
}

.m-name {
  font-weight: 700;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.m-preview {
  color: rgba(255, 255, 255, 0.5);
  font-size: 12px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.m-side {
  display: grid;
  justify-items: end;
  gap: 8px;
  align-self: start;
  padding-top: 2px;
}

.m-time {
  color: rgba(255, 255, 255, 0.4);
  font-size: 11px;
}

.m-dot,
.m-rail-dot {
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: #fe2c55;
}

.m-empty {
  margin: 0;
  padding: 28px 16px;
  text-align: center;
  color: rgba(255, 255, 255, 0.55);
  font-size: 13px;
}

.m-chat {
  flex-direction: row;
}

.m-rail {
  width: 72px;
  flex-shrink: 0;
  overflow: auto;
  padding: 10px 8px;
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  display: grid;
  align-content: start;
  gap: 10px;
}

.m-rail-item {
  position: relative;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  border-radius: 999px;
}

.m-rail-item.on :deep(.avatar) {
  box-shadow: 0 0 0 2px #25f4ee;
}

.m-rail-dot {
  position: absolute;
  top: -2px;
  right: -2px;
}

.m-thread {
  min-width: 0;
  flex: 1;
  display: grid;
  grid-template-rows: auto 1fr auto;
}

.m-who {
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  min-width: 0;
  flex: 1;
  text-align: left;
  padding: 0;
}

.m-who strong {
  display: block;
  font-size: 14px;
}

.m-who small {
  color: rgba(255, 255, 255, 0.5);
  font-size: 12px;
}

.m-ghost,
.m-back,
.m-x {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.18);
  color: rgba(255, 255, 255, 0.86);
  border-radius: 12px;
  padding: 7px 10px;
  cursor: pointer;
}

.m-back,
.m-x {
  width: 34px;
  padding: 0;
  font-size: 20px;
  line-height: 32px;
}

.m-transcript {
  overflow: auto;
  padding: 14px 16px 8px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.m-when {
  align-self: center;
  color: rgba(255, 255, 255, 0.4);
  font-size: 12px;
}

.m-bubble-row {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  max-width: min(78%, 420px);
}

.m-bubble-row.mine {
  align-self: flex-end;
  flex-direction: row-reverse;
}

.m-bubble-col {
  min-width: 0;
  display: grid;
  gap: 4px;
}

.m-bubble-row.mine .m-bubble-col {
  justify-items: end;
}

.m-bubble {
  padding: 9px 12px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.1);
  white-space: pre-wrap;
  word-break: break-word;
  line-height: 1.45;
}

.m-bubble-row.mine .m-bubble {
  background: rgba(254, 44, 85, 0.86);
}

.m-receipt {
  min-height: 14px;
  color: rgba(255, 255, 255, 0.42);
  font-size: 11px;
}

.m-composer {
  padding: 8px 12px 12px;
}

.m-quota {
  margin: 0 0 8px;
  color: rgba(255, 255, 255, 0.55);
  font-size: 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.m-quota button {
  border: 0;
  background: transparent;
  color: #25f4ee;
  cursor: pointer;
  padding: 0;
}

.m-box {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  align-items: center;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 999px;
  padding: 5px 5px 5px 14px;
}

.m-box input {
  border: 0;
  background: transparent;
  color: inherit;
  outline: none;
  min-width: 0;
  font: inherit;
}

.m-box input:disabled {
  opacity: 0.55;
}

.m-send {
  width: 34px;
  height: 34px;
  border-radius: 999px;
  border: 0;
  background: #fe2c55;
  color: #fff;
  font-size: 16px;
  cursor: pointer;
}

.m-send:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
</style>
