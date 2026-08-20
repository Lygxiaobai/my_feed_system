<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { track } from '../analytics/track'
import { createWatchSession } from '../analytics/watch'
import AppShell from '../components/AppShell.vue'
import CommentListSkeleton from '../components/CommentListSkeleton.vue'
import FeedStageSkeleton from '../components/FeedStageSkeleton.vue'
import UserAvatar from '../components/UserAvatar.vue'
import VideoPlayer, { type VideoPlayerHandle } from '../components/VideoPlayer.vue'
import { ApiError } from '../api/client'
import * as commentApi from '../api/comment'
import * as feedApi from '../api/feed'
import * as likeApi from '../api/like'
import type { Comment, CommentReply, FeedVideoItem } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import { countComments, hasCommentID, insertPublishedComment, removeCommentByID } from '../utils/comments'

type TabKey = 'recommend' | 'hot' | 'following'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()

const tab = ref<TabKey>('recommend')
const scroller = ref<HTMLDivElement | null>(null)

const q = computed(() => (typeof route.query.q === 'string' ? route.query.q.trim().toLowerCase() : ''))
const canInteractWithLike = computed(() => auth.isLoggedIn)
const canInteractWithFollow = computed(() => auth.isLoggedIn)

const recommend = reactive({
  items: [] as FeedVideoItem[],
  loading: false,
  error: '',
  hasMore: false,
  asOf: 0,
  nextOffset: 0,
})

const hot = reactive({
  items: [] as FeedVideoItem[],
  loading: false,
  error: '',
  hasMore: false,
  nextLikesCountBefore: undefined as number | undefined,
  nextIdBefore: undefined as number | undefined,
})

const following = reactive({
  items: [] as FeedVideoItem[],
  loading: false,
  error: '',
  hasMore: false,
  nextTime: 0,
  nextId: 0,
})

const likeBusy = reactive<Record<string, boolean>>({})
const followBusy = reactive<Record<string, boolean>>({})
const watchSession = createWatchSession()

const muted = ref(true)
const activeIndex = ref(0)
const playerMap = new Map<number, VideoPlayerHandle>()
let slideObserver: IntersectionObserver | null = null
let tapTimer: number | undefined
let playRequestId = 0

function readMutedPreference() {
  try {
    const saved = window.localStorage.getItem('feed.muted')
    return saved === null ? true : saved === 'true'
  } catch {
    return true
  }
}

const currentState = computed(() => {
  if (tab.value === 'hot') return hot
  if (tab.value === 'following') return following
  return recommend
})

function routeTab(): TabKey {
  if (route.name === 'likes') return 'hot'
  if (route.name === 'following') return 'following'
  return 'recommend'
}

function tabRoutePath(nextTab: TabKey) {
  if (nextTab === 'hot') return '/likes'
  if (nextTab === 'following') return '/following'
  return '/'
}

async function switchTab(nextTab: TabKey) {
  if (route.path === tabRoutePath(nextTab)) {
    tab.value = nextTab
    return
  }
  await router.push(tabRoutePath(nextTab))
}

async function syncLikedState(items: FeedVideoItem[]) {
  if (!items.length) return
  if (!auth.isLoggedIn) {
    items.forEach((item) => {
      item.is_liked = false
    })
    return
  }

  try {
    const likedIds = await likeApi.listLikedVideoIds(items.map((item) => item.id))
    const likedSet = new Set(likedIds)
    items.forEach((item) => {
      item.is_liked = likedSet.has(item.id)
    })
  } catch {
    items.forEach((item) => {
      item.is_liked = false
    })
  }
}

const filteredItems = computed(() => {
  const items = currentState.value.items
  if (!q.value) return items
  return items.filter((v) => v.title.toLowerCase().includes(q.value) || v.author.username.toLowerCase().includes(q.value))
})

const activeItem = computed(() => filteredItems.value[activeIndex.value] ?? null)
const myAccountId = computed(() => auth.claims?.account_id ?? 0)

function setPlayerRef(id: number, el: VideoPlayerHandle | null) {
  if (el) {
    el.setMuted(muted.value)
    playerMap.set(id, el)
  } else {
    playerMap.delete(id)
  }
}

function getScrollerHeight() {
  return scroller.value?.clientHeight ?? 0
}

function scrollToIndex(idx: number) {
  const el = scroller.value
  if (!el) return
  const h = getScrollerHeight()
  if (!h) return
  const next = Math.max(0, Math.min(idx, Math.max(0, filteredItems.value.length - 1)))
  el.scrollTo({ top: next * h, behavior: 'smooth' })
}

let scrollRaf = 0
function onScroll() {
  if (slideObserver) return
  if (!scroller.value) return
  if (scrollRaf) return
  scrollRaf = window.requestAnimationFrame(() => {
    scrollRaf = 0
    const el = scroller.value
    if (!el) return
    const h = el.clientHeight
    if (!h) return
    const idx = Math.round(el.scrollTop / h)
    if (idx !== activeIndex.value) activeIndex.value = idx
  })
}

function setupSlideObserver() {
  slideObserver?.disconnect()
  slideObserver = null
  if (!scroller.value || typeof IntersectionObserver === 'undefined') return

  slideObserver = new IntersectionObserver(
    (entries) => {
      const visible = entries
        .filter((entry) => entry.isIntersecting)
        .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0]
      if (!visible || visible.intersectionRatio < 0.65) return

      const nextIndex = Number((visible.target as HTMLElement).dataset.index)
      if (Number.isInteger(nextIndex) && nextIndex !== activeIndex.value) {
        activeIndex.value = nextIndex
      }
    },
    {
      root: scroller.value,
      threshold: [0.25, 0.5, 0.65, 0.8, 1],
    },
  )

  scroller.value.querySelectorAll<HTMLElement>('.slide').forEach((slide) => {
    slideObserver?.observe(slide)
  })
}

function pauseAllPlayers() {
  for (const player of playerMap.values()) player.pause()
}

async function playActive() {
  const requestId = ++playRequestId
  const item = activeItem.value
  if (!item) return
  pauseAllPlayers()
  await nextTick()
  if (requestId !== playRequestId) return
  await playerMap.get(item.id)?.play()
}

function toggleMute() {
  muted.value = !muted.value
  try {
    window.localStorage.setItem('feed.muted', String(muted.value))
  } catch {
    // 某些隐私模式会禁用 localStorage，静音功能本身仍需正常工作。
  }
  for (const player of playerMap.values()) player.setMuted(muted.value)
}

function togglePlayPause() {
  const item = activeItem.value
  if (!item) return
  void playerMap.get(item.id)?.toggle()
}

function onStageClick() {
  if (tapTimer !== undefined) window.clearTimeout(tapTimer)
  tapTimer = window.setTimeout(() => {
    tapTimer = undefined
    togglePlayPause()
  }, 240)
}

async function needLogin() {
  toast.error('请先登录')
  await router.push('/account')
}

async function loadRecommend(reset: boolean) {
  if (recommend.loading) return
  recommend.loading = true
  recommend.error = ''
  try {
    const res = await feedApi.listByPopularity({
      limit: 10,
      as_of: reset ? 0 : recommend.asOf,
      offset: reset ? 0 : recommend.nextOffset,
    })
    recommend.hasMore = res.has_more
    recommend.asOf = res.as_of
    recommend.nextOffset = res.next_offset
    recommend.items = reset ? res.video_list : recommend.items.concat(res.video_list)
    await syncLikedState(res.video_list)
  } catch (e) {
    recommend.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    recommend.loading = false
  }
}

async function loadHot(reset: boolean) {
  if (hot.loading) return
  hot.loading = true
  hot.error = ''
  try {
    const res = await feedApi.listLikesCount({
      limit: 10,
      likes_count_before: reset ? undefined : hot.nextLikesCountBefore,
      id_before: reset ? undefined : hot.nextIdBefore,
    })
    hot.hasMore = res.has_more
    hot.nextLikesCountBefore = res.next_likes_count_before
    hot.nextIdBefore = res.next_id_before
    hot.items = reset ? res.video_list : hot.items.concat(res.video_list)
    await syncLikedState(res.video_list)
  } catch (e) {
    hot.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    hot.loading = false
  }
}

async function loadFollowing(reset: boolean) {
  if (!auth.isLoggedIn) {
    following.error = '登录后才能查看关注流'
    return
  }
  if (following.loading) return
  following.loading = true
  following.error = ''
  try {
    const res = await feedApi.listByFollowing({
      limit: 10,
      latest_time: reset ? 0 : following.nextTime,
      last_id: reset ? 0 : following.nextId,
    })
    following.hasMore = res.has_more
    following.nextTime = res.next_time
    following.nextId = res.next_id
    following.items = reset ? res.video_list : following.items.concat(res.video_list)
    await syncLikedState(res.video_list)
  } catch (e) {
    following.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    following.loading = false
  }
}

async function ensureTabLoaded() {
  if (tab.value === 'recommend' && recommend.items.length === 0) await loadRecommend(true)
  if (tab.value === 'hot' && hot.items.length === 0) await loadHot(true)
  if (tab.value === 'following' && following.items.length === 0) await loadFollowing(true)
}

async function loadMoreIfNeeded() {
  const idx = activeIndex.value
  const items = filteredItems.value
  if (items.length === 0) return
  if (idx < items.length - 3) return

  if (tab.value === 'recommend' && recommend.hasMore) await loadRecommend(false)
  if (tab.value === 'hot' && hot.hasMore) await loadHot(false)
  if (tab.value === 'following' && following.hasMore) await loadFollowing(false)
}

async function toggleLike(item: FeedVideoItem) {
  if (!auth.isLoggedIn) return needLogin()
  const key = String(item.id)
  if (likeBusy[key]) return
  likeBusy[key] = true
  const previousLiked = item.is_liked
  const nextLiked = !previousLiked
  item.is_liked = nextLiked
  item.likes_count = Math.max(0, item.likes_count + (nextLiked ? 1 : -1))
  try {
    await likeApi.setLikedAndConfirm(item.id, nextLiked)
    track(nextLiked ? 'video_like' : 'video_unlike', { video_id: item.id, feed: tab.value })
  } catch (e) {
    item.is_liked = previousLiked
    item.likes_count = Math.max(0, item.likes_count + (previousLiked ? 1 : -1))
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    likeBusy[key] = false
  }
}

async function toggleFollow(authorId: number) {
  if (!auth.isLoggedIn) return needLogin()
  const key = String(authorId)
  if (followBusy[key] || social.isPending(authorId)) return
  followBusy[key] = true
  try {
    if (social.isFollowing(authorId)) {
      await social.unfollow(authorId)
      track('unfollow', { author_id: authorId, feed: tab.value })
      toast.info('已取关')
    } else {
      const author = currentState.value.items.find((item) => item.author.id === authorId)?.author
      await social.follow(authorId, author?.username)
      track('follow', { author_id: authorId, feed: tab.value })
      toast.success('已关注')
    }
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    followBusy[key] = false
  }
}

async function share(item: FeedVideoItem) {
  const url = `${location.origin}/video/${item.id}`
  try {
    await navigator.clipboard.writeText(url)
    toast.success('链接已复制')
  } catch {
    window.prompt('复制链接', url)
  }
}

const drawer = reactive({
  open: false,
  video: null as FeedVideoItem | null,
  commentsLoading: false,
  commentsRefreshing: false,
  commentSubmitting: false,
  commentDeletingId: 0,
  error: '',
  comments: [] as Comment[],
  content: '',
  replyTarget: null as Comment | CommentReply | null,
  expandedReplies: {} as Record<number, boolean>,
})

const commentSyncAttempts = 6
const commentSyncDelayMs = 400

function clearReplyTarget() {
  drawer.replyTarget = null
}

function isRepliesOpen(commentId: number) {
  return !!drawer.expandedReplies[commentId]
}

function toggleReplies(commentId: number) {
  drawer.expandedReplies[commentId] = !drawer.expandedReplies[commentId]
}

function expandReplies(commentId: number) {
  if (commentId > 0) drawer.expandedReplies[commentId] = true
}

function applyComments(comments: Comment[]) {
  drawer.comments = comments
  if (drawer.video) {
    drawer.video.comment_count = countComments(comments)
  }
}

function wait(ms: number) {
  return new Promise<void>((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

function closeDrawer() {
  drawer.open = false
  drawer.video = null
  drawer.commentsLoading = false
  drawer.commentsRefreshing = false
  drawer.commentSubmitting = false
  drawer.commentDeletingId = 0
  drawer.comments = []
  drawer.content = ''
  drawer.error = ''
  drawer.expandedReplies = {}
  clearReplyTarget()
}

async function openComments(item: FeedVideoItem) {
  if (drawer.video?.id !== item.id) {
    drawer.comments = []
  }
  drawer.open = true
  drawer.video = item
  drawer.content = ''
  clearReplyTarget()
  await loadComments()
}

async function loadComments() {
  if (!drawer.video) return
  const videoId = drawer.video.id
  drawer.commentsLoading = drawer.comments.length === 0
  drawer.commentsRefreshing = drawer.comments.length > 0
  drawer.error = ''
  try {
    const comments = await commentApi.listAll(videoId)
    if (!drawer.open || drawer.video?.id !== videoId) return
    applyComments(comments)
  } catch (e) {
    if (!drawer.open || drawer.video?.id !== videoId) return
    drawer.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (drawer.video?.id === videoId) {
      drawer.commentsLoading = false
      drawer.commentsRefreshing = false
    }
  }
}

function isDrawerBusy() {
  return drawer.commentsLoading || drawer.commentsRefreshing || drawer.commentSubmitting || drawer.commentDeletingId !== 0
}

function startReply(target: Comment | CommentReply) {
  drawer.replyTarget = target
  const rootId = 'replies' in target ? target.id : target.root_comment_id || target.parent_comment_id
  expandReplies(rootId)
}

async function syncCommentsUntil(commentId: number, shouldExist: boolean) {
  const videoId = drawer.video?.id
  if (!videoId) return

  for (let attempt = 0; attempt < commentSyncAttempts; attempt += 1) {
    if (attempt > 0) {
      await wait(commentSyncDelayMs)
    }
    if (!drawer.open || drawer.video?.id !== videoId) {
      return
    }

    try {
      const comments = await commentApi.listAll(videoId)
      if (hasCommentID(comments, commentId) === shouldExist) {
        drawer.error = ''
        applyComments(comments)
        if (drawer.replyTarget && !hasCommentID(comments, drawer.replyTarget.id)) {
          clearReplyTarget()
        }
        return
      }
    } catch {
      // Ignore transient refresh failures and keep optimistic state.
    }
  }
}

async function publishComment() {
  if (!drawer.video) return
  if (!auth.isLoggedIn) return needLogin()
  const content = drawer.content.trim()
  if (!content) return
  const videoId = drawer.video.id
  drawer.commentSubmitting = true
  drawer.error = ''
  try {
    const res = await commentApi.publish(videoId, content, drawer.replyTarget?.id)
    if (!drawer.open || drawer.video?.id !== videoId) return
    drawer.content = ''
    clearReplyTarget()
    applyComments(insertPublishedComment(drawer.comments, res.comment))
    expandReplies(res.comment.root_comment_id || res.comment.parent_comment_id)
    void syncCommentsUntil(res.comment.id, true)
    track('comment_submit', { video_id: videoId, is_reply: !!res.comment.parent_comment_id })
    toast.success('评论已发布')
  } catch (e) {
    if (!drawer.open || drawer.video?.id !== videoId) return
    drawer.error = e instanceof ApiError ? e.message : String(e)
    toast.error(drawer.error)
  } finally {
    if (drawer.video?.id === videoId) drawer.commentSubmitting = false
  }
}

function canDeleteComment(c: Comment | CommentReply) {
  const myId = auth.claims?.account_id
  return !!myId && (myId === c.author_id || myId === drawer.video?.author.id)
}

async function deleteComment(commentId: number) {
  if (!drawer.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (!window.confirm('确认删除这条评论？')) return
  const videoId = drawer.video.id
  drawer.commentDeletingId = commentId
  drawer.error = ''
  try {
    await commentApi.remove(commentId)
    if (!drawer.open || drawer.video?.id !== videoId) return
    const nextComments = removeCommentByID(drawer.comments, commentId)
    applyComments(nextComments)
    if (drawer.replyTarget && !hasCommentID(nextComments, drawer.replyTarget.id)) clearReplyTarget()
    void syncCommentsUntil(commentId, false)
    toast.info('评论已删除')
  } catch (e) {
    if (!drawer.open || drawer.video?.id !== videoId) return
    drawer.error = e instanceof ApiError ? e.message : String(e)
    toast.error(drawer.error)
  } finally {
    if (drawer.video?.id === videoId) drawer.commentDeletingId = 0
  }
}

async function onKeydown(e: KeyboardEvent) {
  const t = e.target as HTMLElement | null
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA')) return
  if (drawer.open) return

  if (e.key === 'ArrowDown') {
    e.preventDefault()
    scrollToIndex(activeIndex.value + 1)
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    scrollToIndex(activeIndex.value - 1)
  } else if (e.key === ' ') {
    e.preventDefault()
    togglePlayPause()
  } else if (e.key.toLowerCase() === 'm') {
    e.preventDefault()
    toggleMute()
  } else if (e.key.toLowerCase() === 'c') {
    if (activeItem.value) {
      e.preventDefault()
      await openComments(activeItem.value)
    }
  }
}

function onStageDoubleClick(item: FeedVideoItem) {
  if (tapTimer !== undefined) {
    window.clearTimeout(tapTimer)
    tapTimer = undefined
  }
  if (!auth.isLoggedIn) return
  void toggleLike(item)
}

watch(
  () => activeItem.value?.id,
  async (currentId, previousId) => {
    if (previousId && previousId !== currentId) {
      watchSession.end(playerMap.get(previousId) ?? undefined, { feed: tab.value })
    }
    await nextTick()
    await playActive()
    await loadMoreIfNeeded()
  },
)

watch(
  () => tab.value,
  async () => {
    watchSession.end(activeItem.value ? playerMap.get(activeItem.value.id) : undefined, { feed: tab.value })
    activeIndex.value = 0
    pauseAllPlayers()
    playerMap.clear()
    if (scroller.value) scroller.value.scrollTop = 0
    await ensureTabLoaded()
    await nextTick()
    setupSlideObserver()
    await playActive()
  },
)

watch(
  () => q.value,
  async () => {
    activeIndex.value = 0
    if (scroller.value) scroller.value.scrollTop = 0
    await nextTick()
    setupSlideObserver()
    await playActive()
  },
)

watch(
  () => filteredItems.value.length,
  async (len) => {
    if (len === 0) activeIndex.value = 0
    else if (activeIndex.value > len - 1) activeIndex.value = len - 1
    await nextTick()
    setupSlideObserver()
  },
)

watch(
  () => auth.isLoggedIn,
  async (v) => {
    if (tab.value === 'following' && v && following.items.length === 0) {
      await loadFollowing(true)
    }
    if (!v) {
      await syncLikedState(recommend.items)
      await syncLikedState(hot.items)
      await syncLikedState(following.items)
    }
  },
)

watch(
  () => route.name,
  () => {
    const nextTab = routeTab()
    if (tab.value !== nextTab) {
      tab.value = nextTab
    }
  },
  { immediate: true },
)

onMounted(async () => {
  muted.value = readMutedPreference()
  await ensureTabLoaded()
  await nextTick()
  setupSlideObserver()
  await playActive()
  window.addEventListener('keydown', onKeydown)
})

onBeforeUnmount(() => {
  if (tapTimer !== undefined) window.clearTimeout(tapTimer)
  watchSession.end(activeItem.value ? playerMap.get(activeItem.value.id) : undefined, { feed: tab.value })
  slideObserver?.disconnect()
  slideObserver = null
  pauseAllPlayers()
  playerMap.clear()
  window.removeEventListener('keydown', onKeydown)
})
</script>

<template>
  <AppShell full>
    <div class="page">
      <div class="tabs">
        <button class="tab" :class="{ on: tab === 'recommend' }" type="button" @click="switchTab('recommend')">推荐</button>
        <button class="tab" :class="{ on: tab === 'following' }" type="button" @click="switchTab('following')">关注</button>
        <button class="tab" :class="{ on: tab === 'hot' }" type="button" @click="switchTab('hot')">点赞榜</button>

        <div class="tabs-right">
          <button class="chip" type="button" @click="toggleMute">{{ muted ? '静音' : '有声' }}</button>
          <RouterLink class="chip" :to="activeItem ? `/video/${activeItem.id}` : '/video'">详情</RouterLink>
        </div>
      </div>

      <div ref="scroller" class="scroller" @scroll="onScroll">
        <section v-if="currentState.loading && currentState.items.length === 0" class="slide">
          <FeedStageSkeleton />
        </section>
        <div v-else-if="currentState.error && currentState.items.length === 0" class="center-hint bad">
          {{ currentState.error }}
        </div>
        <div v-else-if="filteredItems.length === 0" class="center-hint">没有匹配内容</div>

        <section
          v-for="(item, idx) in filteredItems"
          :key="`${tab}-${item.id}`"
          class="slide"
          :data-index="idx"
          :class="{ active: idx === activeIndex }"
        >
          <div class="stage" @click="onStageClick" @dblclick.prevent="onStageDoubleClick(item)">
            <VideoPlayer
              :ref="(el) => setPlayerRef(item.id, el as VideoPlayerHandle | null)"
              :src="item.play_url"
              :poster="item.cover_url"
              :active="idx === activeIndex"
              :enabled="Math.abs(idx - activeIndex) <= 1"
              :muted="muted"
              @playing="watchSession.play(item.id, { feed: tab })"
            />
            <div class="grad" />

            <div class="meta">
              <RouterLink class="author-link" :to="`/u/${item.author.id}`" @click.stop>
                <UserAvatar :username="item.author.username" :id="item.author.id" :size="34" />
              </RouterLink>
              <div class="title">{{ item.title }}</div>
              <div v-if="item.description" class="desc">{{ item.description }}</div>
            </div>

            <div class="actions">
              <button
                v-if="canInteractWithLike"
                class="act"
                type="button"
                :disabled="!!likeBusy[String(item.id)]"
                @click.stop="toggleLike(item)"
              >
                <span class="icon" :class="{ liked: item.is_liked }">♥</span>
                <span class="count">{{ item.likes_count }}</span>
              </button>

              <div v-else class="act metric">
                <span class="icon">♥</span>
                <span class="count">{{ item.likes_count }}</span>
              </div>

              <button class="act" type="button" @click.stop="openComments(item)">
                <span class="icon">💬</span>
                <span class="count">{{ item.comment_count }}</span>
              </button>

              <button
                v-if="canInteractWithFollow && (!myAccountId || myAccountId !== item.author.id)"
                class="act"
                type="button"
                :disabled="!!followBusy[String(item.author.id)] || social.isPending(item.author.id)"
                @click.stop="toggleFollow(item.author.id)"
              >
                <span class="icon">＋</span>
                <span class="count">{{ social.isFollowing(item.author.id) ? '已关注' : '关注' }}</span>
              </button>

              <button class="act" type="button" @click.stop="share(item)">
                <span class="icon">↗</span>
                <span class="count">分享</span>
              </button>
            </div>
          </div>
        </section>
        <section v-if="currentState.loading && currentState.items.length > 0" class="slide">
          <FeedStageSkeleton />
        </section>
      </div>

      <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
        <div class="drawer">
          <div class="drawer-head">
            <div class="drawer-title">{{ drawer.video?.title ?? '评论' }}</div>
            <button class="drawer-x" type="button" @click="closeDrawer">×</button>
          </div>

          <div class="drawer-body">
            <CommentListSkeleton v-if="drawer.commentsLoading && drawer.comments.length === 0" />
            <div v-else-if="drawer.error" class="drawer-hint bad">{{ drawer.error }}</div>
            <div v-else-if="drawer.comments.length === 0" class="drawer-hint">暂无评论</div>

            <div class="comment" v-for="c in drawer.comments" :key="c.id">
              <div class="comment-top">
                <div class="comment-user">{{ c.username }}</div>
                <div class="comment-meta mono">{{ new Date(c.created_at).toLocaleString() }}</div>
              </div>
              <div class="comment-content">{{ c.content }}</div>
              <div class="comment-actions comment-actions-left">
                <button class="chip" type="button" :disabled="isDrawerBusy()" @click="startReply(c)">回复</button>
                <button v-if="canDeleteComment(c)" class="chip danger" type="button" :disabled="isDrawerBusy()" @click="deleteComment(c.id)">
                  删除
                </button>
                <button v-if="c.replies.length > 0" class="chip" type="button" @click="toggleReplies(c.id)">
                  {{ isRepliesOpen(c.id) ? '收起' : `展开 ${c.replies.length} 条回复` }}
                </button>
              </div>

              <div v-if="c.replies.length > 0 && isRepliesOpen(c.id)" class="reply-list">
                <div class="reply" v-for="reply in c.replies" :key="reply.id">
                  <div class="comment-top">
                    <div class="comment-user">{{ reply.username }}</div>
                    <div class="comment-meta mono">{{ new Date(reply.created_at).toLocaleString() }}</div>
                  </div>
                  <div class="comment-content">
                    <span v-if="reply.reply_to_username" class="reply-prefix">回复 @{{ reply.reply_to_username }}：</span>{{ reply.content }}
                  </div>
                  <div class="comment-actions comment-actions-left">
                    <button class="chip" type="button" :disabled="isDrawerBusy()" @click="startReply(reply)">回复</button>
                    <button v-if="canDeleteComment(reply)" class="chip danger" type="button" :disabled="isDrawerBusy()" @click="deleteComment(reply.id)">
                      删除
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="drawer-foot">
            <div v-if="drawer.replyTarget" class="reply-banner">
              <span>回复 @{{ drawer.replyTarget.username }}</span>
              <button class="chip" type="button" :disabled="isDrawerBusy()" @click="clearReplyTarget">取消</button>
            </div>
            <textarea v-model="drawer.content" :placeholder="drawer.replyTarget ? `回复 @${drawer.replyTarget.username}…` : '说点什么…'" :disabled="isDrawerBusy()" />
            <div class="row" style="justify-content: space-between; margin-top: 8px">
              <button class="chip" type="button" :disabled="isDrawerBusy()" @click="loadComments">刷新</button>
              <button class="chip primary" type="button" :disabled="isDrawerBusy() || !drawer.content.trim()" @click="publishComment">
                发送
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.page {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.tabs {
  height: 52px;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 0 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.25);
  backdrop-filter: blur(16px);
}

.tab {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.88);
  border-radius: 999px;
  padding: 8px 14px;
  cursor: pointer;
}

.tab.on {
  border-color: rgba(254, 44, 85, 0.5);
  background: rgba(254, 44, 85, 0.16);
}

.tabs-right {
  margin-left: auto;
  display: flex;
  gap: 10px;
  align-items: center;
}

.scroller {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  scroll-snap-type: y mandatory;
  scroll-behavior: smooth;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.scroller::-webkit-scrollbar {
  width: 0;
  height: 0;
}

.center-hint {
  height: calc(100% - 60px);
  display: grid;
  place-items: center;
  color: rgba(255, 255, 255, 0.78);
}

.center-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.slide {
  height: 100%;
  min-height: 100%;
  box-sizing: border-box;
  scroll-snap-align: start;
  scroll-snap-stop: always;
  padding: 18px 14px;
  display: grid;
  place-items: center;
}

.stage {
  width: min(980px, calc(100vw - 28px));
  height: min(100%, calc(var(--app-height, 100dvh) - var(--topbar-h, 56px) - 52px - 36px - var(--bottom-nav-h, 0px)));
  position: relative;
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(0, 0, 0, 0.35);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.55);
}

.video {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  background: rgba(0, 0, 0, 0.4);
}

.grad {
  position: absolute;
  inset: 0;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.68), rgba(0, 0, 0, 0.12) 40%, rgba(0, 0, 0, 0) 70%);
  pointer-events: none;
}

.meta {
  position: absolute;
  left: 16px;
  bottom: 18px;
  max-width: min(620px, calc(100% - 96px));
}

.author-link {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font-weight: 800;
  letter-spacing: 0.2px;
  margin-bottom: 6px;
  text-decoration: none;
}

.author-link:hover {
  text-decoration: none;
}

.author-name {
  text-shadow: 0 14px 30px rgba(0, 0, 0, 0.55);
}

.title {
  font-size: 16px;
  font-weight: 700;
  margin-bottom: 6px;
}

.desc {
  color: rgba(255, 255, 255, 0.74);
  font-size: 13px;
  line-height: 1.35;
}

.actions {
  position: absolute;
  right: 12px;
  bottom: 18px;
  display: grid;
  gap: 12px;
}

.act {
  width: 70px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.32);
  color: rgba(255, 255, 255, 0.92);
  padding: 10px 10px;
  cursor: pointer;
  display: grid;
  gap: 6px;
  justify-items: center;
}

.act:hover {
  background: rgba(255, 255, 255, 0.1);
}

.metric {
  cursor: default;
}

.metric:hover {
  background: rgba(0, 0, 0, 0.32);
}

.act:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.icon {
  font-size: 20px;
  line-height: 1;
  opacity: 0.92;
}

.icon.liked {
  color: rgba(254, 44, 85, 1);
  text-shadow: 0 10px 20px rgba(254, 44, 85, 0.25);
}

.count {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.8);
}

.chip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 7px 10px;
  border-radius: 999px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.28);
  color: rgba(255, 255, 255, 0.86);
  font-size: 12px;
  text-decoration: none;
}

.chip.primary {
  border-color: rgba(254, 44, 85, 0.45);
  background: rgba(254, 44, 85, 0.14);
}

.chip.danger {
  border-color: rgba(254, 44, 85, 0.55);
  background: rgba(254, 44, 85, 0.12);
}

.drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(10px);
  z-index: 120;
  display: grid;
  justify-items: end;
}

.drawer {
  width: min(420px, calc(100vw - 18px));
  height: 100vh;
  background: rgba(0, 0, 0, 0.65);
  border-left: 1px solid rgba(255, 255, 255, 0.12);
  display: grid;
  grid-template-rows: auto 1fr auto;
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.drawer-title {
  font-weight: 800;
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.drawer-x {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.9);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
}

.drawer-body {
  overflow: auto;
  padding: 12px 14px;
  display: grid;
  gap: 10px;
}

.drawer-foot {
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  padding: 12px 14px;
}

.drawer-foot textarea {
  width: 100%;
  min-height: 82px;
  resize: none;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.9);
  padding: 10px 12px;
  outline: none;
}

.drawer-hint {
  color: rgba(255, 255, 255, 0.78);
  padding: 12px 0;
}

.drawer-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.comment {
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.05);
  border-radius: 14px;
  padding: 10px 10px;
}

.reply-list {
  margin-top: 10px;
  display: grid;
  gap: 8px;
}

.reply {
  border-left: 2px solid rgba(254, 44, 85, 0.25);
  padding-left: 10px;
}

.comment-top {
  display: grid;
  gap: 3px;
}

.comment-user {
  font-weight: 700;
  font-size: 13px;
}

.comment-meta {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.55);
}

.comment-content {
  margin-top: 8px;
  font-size: 13px;
  line-height: 1.35;
  color: rgba(255, 255, 255, 0.86);
  white-space: pre-wrap;
  word-break: break-word;
}

.reply-prefix {
  color: rgba(254, 44, 85, 0.9);
}

.comment-actions {
  margin-top: 10px;
  display: flex;
  justify-content: flex-end;
}

.comment-actions-left {
  justify-content: flex-start;
  gap: 8px;
}

.reply-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-bottom: 8px;
  padding: 8px 10px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.8);
  font-size: 12px;
}

@media (max-width: 900px) {
  .tabs {
    height: 48px;
    gap: 6px;
    padding: 0 10px;
    overflow-x: auto;
    overflow-y: hidden;
    flex-wrap: nowrap;
    -webkit-overflow-scrolling: touch;
  }

  .tab {
    flex: 0 0 auto;
    padding: 7px 12px;
    font-size: 13px;
  }

  .tabs-right {
    gap: 6px;
  }

  .chip {
    padding: 6px 9px;
    font-size: 12px;
  }

  .slide {
    padding: 0;
  }

  .stage {
    width: 100%;
    height: 100%;
    border-radius: 0;
    border: none;
    box-shadow: none;
  }

  .meta {
    left: 12px;
    right: 84px;
    bottom: calc(14px + env(safe-area-inset-bottom, 0px));
    max-width: none;
  }

  .actions {
    right: 8px;
    bottom: calc(18px + env(safe-area-inset-bottom, 0px));
    gap: 10px;
  }

  .act {
    width: 64px;
    padding: 8px 6px;
    border-radius: 14px;
  }

  .icon {
    font-size: 22px;
  }

  .drawer-backdrop {
    justify-items: center;
    align-items: end;
    padding-bottom: var(--bottom-nav-h, 56px);
  }

  .drawer {
    width: 100%;
    max-width: 100vw;
    height: min(70dvh, 560px);
    border-left: none;
    border-top: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 18px 18px 0 0;
    overflow: hidden;
  }

  .drawer-foot {
    padding-bottom: calc(12px + env(safe-area-inset-bottom, 0px));
  }
}
</style>
