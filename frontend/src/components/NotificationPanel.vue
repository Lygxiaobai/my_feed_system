<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '../api/client'
import * as notificationApi from '../api/notification'
import type { NotificationFilter, NotificationItem, NotificationKind } from '../api/notification'
import { useAuthStore } from '../stores/auth'
import { useNotificationStore } from '../stores/notification'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import AppIcon, { type AppIconName } from './AppIcon.vue'
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

const filters: { value: NotificationFilter; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'follow', label: '关注' },
  { value: 'like', label: '赞' },
  { value: 'mention', label: '@' },
  { value: 'reply', label: '评论' },
  { value: 'tip', label: '打赏' },
]

const kindIcon: Record<NotificationKind, AppIconName> = {
  follow: 'user-plus',
  like: 'heart-fill',
  comment: 'comment',
  reply: 'comment',
  mention: 'at',
  tip: 'coin',
}

const kindTone: Record<NotificationKind, string> = {
  follow: 'follow',
  like: 'like',
  comment: 'reply',
  reply: 'reply',
  mention: 'mention',
  tip: 'tip',
}

const relationLabel: Record<string, string> = {
  friend: '朋友',
  following: '关注',
}

const router = useRouter()
const auth = useAuthStore()
const notif = useNotificationStore()
const social = useSocialStore()
const toast = useToastStore()

const kind = ref<NotificationFilter>('all')
const items = ref<NotificationItem[]>([])
const cursor = ref('')
const hasMore = ref(false)
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')

const emptyHint = computed(() => {
  if (kind.value === 'all') return '还没有互动消息'
  return '这一类暂时是空的'
})

function formatRelativeTime(raw: string) {
  const t = new Date(raw).getTime()
  if (!Number.isFinite(t)) return ''
  const diff = Date.now() - t
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)}分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)}小时前`
  const day = new Date(t)
  const yest = new Date()
  yest.setDate(yest.getDate() - 1)
  if (day.toDateString() === yest.toDateString()) return '昨天'
  const mm = String(day.getMonth() + 1).padStart(2, '0')
  const dd = String(day.getDate()).padStart(2, '0')
  if (day.getFullYear() === new Date().getFullYear()) return `${mm}-${dd}`
  return `${day.getFullYear()}-${mm}-${dd}`
}

function actorNames(item: NotificationItem) {
  return item.actors.map((a) => a.username).join('、')
}

function primaryActor(item: NotificationItem) {
  return item.actors[0]
}

async function load(reset: boolean) {
  if (!auth.isLoggedIn) return
  if (reset) {
    loading.value = true
    error.value = ''
    cursor.value = ''
  } else {
    if (!hasMore.value || loadingMore.value) return
    loadingMore.value = true
  }
  try {
    const res = await notificationApi.listNotifications({
      kind: kind.value,
      cursor: reset ? '' : cursor.value,
      limit: props.variant === 'dropdown' ? 12 : 20,
    })
    items.value = reset ? res.items : items.value.concat(res.items)
    cursor.value = res.next_cursor
    hasMore.value = res.has_more
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '通知加载失败'
  } finally {
    loading.value = false
    loadingMore.value = false
  }
}

async function onMarkAll() {
  try {
    await notificationApi.markAllRead(kind.value)
    items.value = items.value.map((item) => ({ ...item, unread: false }))
    notif.applyAllRead()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '标记已读失败')
  }
}

async function markOne(item: NotificationItem) {
  if (!item.unread) return
  item.unread = false
  notif.applyLocalRead(1)
  try {
    await notificationApi.markRead([item.id])
  } catch {
    item.unread = true
    notif.refreshUnread()
  }
}

async function openItem(item: NotificationItem) {
  await markOne(item)
  emit('close')
  const actor = primaryActor(item)
  if (item.kind === 'follow' && actor) {
    await router.push(`/u/${actor.id}`)
    return
  }
  if (item.video?.id) {
    await router.push(`/video/${item.video.id}`)
    return
  }
  if (actor) await router.push(`/u/${actor.id}`)
}

async function openActor(item: NotificationItem, event: Event) {
  event.stopPropagation()
  const actor = primaryActor(item)
  if (!actor) return
  await markOne(item)
  emit('close')
  await router.push(`/u/${actor.id}`)
}

async function followBack(item: NotificationItem, event: Event) {
  event.stopPropagation()
  const actor = primaryActor(item)
  if (!actor || item.followed || social.isPending(actor.id)) return
  try {
    await social.follow(actor.id, actor.username)
    item.followed = true
    item.relation = item.relation === '' ? 'following' : item.relation
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '回关失败')
  }
}

watch(kind, () => {
  void load(true)
})

onMounted(() => {
  void load(true)
  void notif.refreshUnread()
})
</script>

<template>
  <section class="n-panel" :class="props.variant">
    <header class="n-head">
      <div class="n-title">互动消息</div>
      <div class="n-tools">
        <select v-model="kind" class="n-filter" aria-label="筛选通知类型">
          <option v-for="f in filters" :key="f.value" :value="f.value">{{ f.label }}消息</option>
        </select>
        <button class="n-readall" type="button" @click="onMarkAll">全部已读</button>
      </div>
    </header>

    <p v-if="error" class="n-error">{{ error }}</p>
    <p v-else-if="loading" class="n-empty">正在加载…</p>
    <p v-else-if="items.length === 0" class="n-empty">{{ emptyHint }}</p>

    <div v-else class="n-list">
      <button
        v-for="item in items"
        :key="item.id"
        class="n-row"
        :class="{ unread: item.unread }"
        type="button"
        @click="openItem(item)"
      >
        <div class="n-avatar-wrap">
          <UserAvatar :username="primaryActor(item)?.username ?? '用户'" :id="primaryActor(item)?.id" :size="44" />
          <span class="n-kind" :class="kindTone[item.kind]" aria-hidden="true">
            <AppIcon :name="kindIcon[item.kind]" :size="11" />
          </span>
        </div>

        <div class="n-main">
          <div class="n-line">
            <button class="n-name" type="button" @click="openActor(item, $event)">{{ actorNames(item) }}</button>
            <span v-if="item.relation" class="n-rel">{{ relationLabel[item.relation] }}</span>
          </div>
          <div v-if="item.text" class="n-text">{{ item.text }}</div>
          <div class="n-meta">
            <span>{{ item.action_text }}</span>
            <span class="n-dot">·</span>
            <span>{{ formatRelativeTime(item.created_at) }}</span>
          </div>
        </div>

        <div class="n-side">
          <button
            v-if="item.kind === 'follow' && !item.followed"
            class="n-follow"
            type="button"
            @click="followBack(item, $event)"
          >
            回关
          </button>
          <img
            v-else-if="item.video?.cover_url"
            class="n-cover"
            :src="item.video.cover_url"
            :alt="item.video.title || '作品'"
          />
        </div>
      </button>
    </div>

    <button v-if="hasMore && !loading" class="n-more" type="button" :disabled="loadingMore" @click="load(false)">
      {{ loadingMore ? '加载中…' : '加载更多' }}
    </button>
  </section>
</template>

<style scoped>
.n-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
  color: rgba(255, 255, 255, 0.92);
}

.n-panel.dropdown {
  width: min(420px, calc(100vw - 24px));
  max-height: min(560px, calc(100dvh - 80px));
  background: rgba(22, 22, 28, 0.96);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 14px;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(18px);
  overflow: hidden;
}

.n-panel.page {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 16px;
  overflow: hidden;
}

.n-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.n-title {
  font-weight: 800;
  font-size: 15px;
}

.n-tools {
  display: flex;
  align-items: center;
  gap: 8px;
}

.n-filter {
  appearance: none;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.72);
  font-size: 12px;
  padding: 4px 2px;
  cursor: pointer;
}

.n-readall {
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.56);
  font-size: 12px;
  cursor: pointer;
  padding: 4px 0;
}

.n-readall:hover,
.n-filter:hover {
  color: rgba(255, 255, 255, 0.92);
}

.n-list {
  overflow: auto;
  min-height: 0;
}

.n-row {
  width: 100%;
  display: grid;
  grid-template-columns: 48px minmax(0, 1fr) 56px;
  gap: 10px;
  align-items: center;
  padding: 12px 14px;
  border: 0;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}

.n-row:hover {
  background: rgba(255, 255, 255, 0.05);
}

.n-row.unread {
  background: rgba(37, 244, 238, 0.06);
}

.n-avatar-wrap {
  position: relative;
  width: 44px;
  height: 44px;
}

.n-kind {
  position: absolute;
  right: -2px;
  bottom: -2px;
  width: 18px;
  height: 18px;
  border-radius: 999px;
  display: grid;
  place-items: center;
  color: #fff;
  border: 2px solid rgba(16, 16, 20, 0.95);
}

.n-kind.like {
  background: #fe2c55;
}

.n-kind.follow {
  background: #3b82f6;
}

.n-kind.reply {
  background: #2563eb;
}

.n-kind.mention {
  background: #06b6d4;
}

.n-kind.tip {
  background: #f59e0b;
}

.n-main {
  min-width: 0;
  display: grid;
  gap: 3px;
}

.n-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.n-name {
  border: 0;
  background: transparent;
  color: inherit;
  font-weight: 700;
  font-size: 14px;
  padding: 0;
  cursor: pointer;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.n-rel {
  flex: none;
  font-size: 10px;
  line-height: 1;
  padding: 3px 5px;
  border-radius: 4px;
  color: #67e8f9;
  background: rgba(37, 244, 238, 0.12);
}

.n-text {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.82);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.n-meta {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.46);
}

.n-dot {
  margin: 0 4px;
}

.n-side {
  display: grid;
  place-items: center;
}

.n-cover {
  width: 48px;
  height: 48px;
  border-radius: 8px;
  object-fit: cover;
  background: rgba(255, 255, 255, 0.06);
}

.n-follow {
  border: 0;
  border-radius: 8px;
  background: #fe2c55;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  padding: 6px 12px;
  cursor: pointer;
}

.n-empty,
.n-error {
  margin: 0;
  padding: 36px 16px;
  text-align: center;
  color: rgba(255, 255, 255, 0.5);
  font-size: 13px;
}

.n-error {
  color: #fda4af;
}

.n-more {
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.56);
  padding: 12px;
  cursor: pointer;
}
</style>
