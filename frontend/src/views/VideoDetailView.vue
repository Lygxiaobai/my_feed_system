<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { track } from '../analytics/track'
import { createWatchSession } from '../analytics/watch'
import {
  bindWatchProgress,
  flushWatchProgress,
  noteWatchProgress,
  resolveResumeSeconds,
  unbindWatchProgress,
} from '../history/progress'
import AppIcon from '../components/AppIcon.vue'
import AppShell from '../components/AppShell.vue'
import CommentListSkeleton from '../components/CommentListSkeleton.vue'
import DanmakuLayer from '../components/DanmakuLayer.vue'
import FeedStageSkeleton from '../components/FeedStageSkeleton.vue'
import ReportSheet from '../components/ReportSheet.vue'
import ShareSheet from '../components/ShareSheet.vue'
import TipSheet from '../components/TipSheet.vue'
import UserAvatar from '../components/UserAvatar.vue'
import VideoPlayer, { type VideoPlayerHandle } from '../components/VideoPlayer.vue'
import { ApiError } from '../api/client'
import * as commentApi from '../api/comment'
import * as likeApi from '../api/like'
import type { Comment, CommentReply, Video } from '../api/types'
import * as videoApi from '../api/video'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import { countComments, hasCommentID, insertPublishedComment, removeCommentByID } from '../utils/comments'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()

const id = computed(() => Number(route.params.id))

const state = reactive({
  loading: false,
  error: '',
  video: null as Video | null,
  isLiked: null as boolean | null,
  busy: false,
})

const muted = ref(true)
const danmakuEnabled = ref(true)
const playTime = ref(0)
const playing = ref(false)
const player = ref<VideoPlayerHandle | null>(null)
const watchSession = createWatchSession()
let tapTimer: number | undefined
let videoLoadRequestId = 0

const drawer = reactive({
  open: false,
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

async function needLogin() {
  toast.error('请先登录')
  await router.push('/account')
}

async function loadVideo() {
  const requestId = ++videoLoadRequestId
  const requestedVideoId = id.value
  if (!Number.isFinite(id.value) || id.value <= 0) {
    state.error = '无效的 video id'
    return
  }
  state.loading = true
  state.error = ''
  try {
    const video = await videoApi.getDetail(requestedVideoId)
    if (requestId !== videoLoadRequestId || requestedVideoId !== id.value) return
    state.video = video
  } catch (e) {
    if (requestId !== videoLoadRequestId || requestedVideoId !== id.value) return
    state.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (requestId === videoLoadRequestId) state.loading = false
  }
}

async function loadIsLiked() {
  const requestedVideoId = id.value
  if (!auth.isLoggedIn) {
    state.isLiked = null
    return
  }
  try {
    const res = await likeApi.isLiked(requestedVideoId)
    if (requestedVideoId !== id.value) return
    state.isLiked = res.is_liked
  } catch {
    if (requestedVideoId === id.value) state.isLiked = null
  }
}

async function play() {
  await player.value?.play()
}

function toggleMute() {
  muted.value = !muted.value
  player.value?.setMuted(muted.value)
  try {
    window.localStorage.setItem('feed.muted', String(muted.value))
  } catch {
    // 某些隐私模式会禁用 localStorage，静音功能本身仍需正常工作。
  }
}

function toggleDanmaku() {
  danmakuEnabled.value = !danmakuEnabled.value
  try {
    window.localStorage.setItem('feed.danmaku', String(danmakuEnabled.value))
  } catch {
    // 隐私模式写不了 localStorage，当次会话的开关仍要生效。
  }
}

function onPlayerPlaying() {
  playing.value = true
  if (state.video) watchSession.play(state.video.id, { from: 'detail' })
}

function onPlayerPaused() {
  playing.value = false
  if (state.video) flushWatchProgress(state.video.id)
}

function onPlayerTime(seconds: number) {
  playTime.value = seconds
  if (state.video) noteWatchProgress(state.video.id, seconds, player.value?.duration() ?? 0)
}

async function applyResume(videoId: number) {
  const seconds = await resolveResumeSeconds(videoId)
  if (id.value !== videoId || seconds <= 0) return
  player.value?.seek(seconds)
}

function togglePlayPause() {
  void player.value?.toggle()
}

function onStageClick() {
  if (tapTimer !== undefined) window.clearTimeout(tapTimer)
  tapTimer = window.setTimeout(() => {
    tapTimer = undefined
    togglePlayPause()
  }, 240)
}

function onStageDoubleClick() {
  if (tapTimer !== undefined) {
    window.clearTimeout(tapTimer)
    tapTimer = undefined
  }
  void toggleLike()
}

async function toggleLike() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (state.busy) return

  const videoId = state.video.id
  const previousLiked = !!state.isLiked
  const nextLiked = !previousLiked
  state.busy = true
  state.isLiked = nextLiked
  state.video.likes_count = Math.max(0, state.video.likes_count + (nextLiked ? 1 : -1))
  try {
    await likeApi.setLikedAndConfirm(videoId, nextLiked)
    track(nextLiked ? 'video_like' : 'video_unlike', { video_id: videoId, from: 'detail' })
  } catch (e) {
    if (state.video?.id === videoId) {
      state.isLiked = previousLiked
      state.video.likes_count = Math.max(0, state.video.likes_count + (previousLiked ? 1 : -1))
    }
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    state.busy = false
  }
}

async function toggleFollow() {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (state.busy) return
  if (social.isPending(state.video.author_id)) return
  if (auth.claims?.account_id && auth.claims.account_id === state.video.author_id) return

  state.busy = true
  try {
    if (social.isFollowing(state.video.author_id)) {
      await social.unfollow(state.video.author_id)
      track('unfollow', { author_id: state.video.author_id, from: 'detail' })
      toast.info('已取关')
    } else {
      await social.follow(state.video.author_id, state.video.username)
      track('follow', { author_id: state.video.author_id, from: 'detail' })
      toast.success('已关注')
    }
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    state.busy = false
  }
}

const isOwnVideo = computed(() => !!state.video && auth.claims?.account_id === state.video.author_id)
const canTip = computed(() => !!state.video && !isOwnVideo.value && (state.video.audit_status ?? 'approved') === 'approved')
const tipSheet = ref<InstanceType<typeof TipSheet> | null>(null)

function openTip() {
  if (!state.video) return
  if (!auth.isLoggedIn) return void needLogin()
  if (!canTip.value) return
  tipSheet.value?.openGive()
}

function openTips() {
  if (!state.video) return
  if (!auth.isLoggedIn) return void needLogin()
  tipSheet.value?.openInbox()
}

const shareSheet = ref<InstanceType<typeof ShareSheet> | null>(null)
const reportSheet = ref<InstanceType<typeof ReportSheet> | null>(null)

function share() {
  if (!state.video) return
  track('video_share', { video_id: state.video.id, from: 'detail' })
  shareSheet.value?.openFor(state.video.id)
}

function openReport() {
  if (!state.video) return
  reportSheet.value?.openFor(state.video.id)
}

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
  if (state.video) {
    state.video.comment_count = countComments(comments)
  }
}

function wait(ms: number) {
  return new Promise<void>((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

function closeDrawer() {
  drawer.open = false
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

async function loadComments() {
  if (!state.video) return
  const videoId = state.video.id
  drawer.commentsLoading = drawer.comments.length === 0
  drawer.commentsRefreshing = drawer.comments.length > 0
  drawer.error = ''
  try {
    const comments = await commentApi.listAll(videoId)
    if (!drawer.open || state.video?.id !== videoId) return
    applyComments(comments)
  } catch (e) {
    if (!drawer.open || state.video?.id !== videoId) return
    drawer.error = e instanceof ApiError ? e.message : String(e)
  } finally {
    if (state.video?.id === videoId) {
      drawer.commentsLoading = false
      drawer.commentsRefreshing = false
    }
  }
}

function isDrawerBusy() {
  return drawer.commentsLoading || drawer.commentsRefreshing || drawer.commentSubmitting || drawer.commentDeletingId !== 0
}

async function openComments() {
  drawer.open = true
  drawer.content = ''
  clearReplyTarget()
  await loadComments()
}

function startReply(target: Comment | CommentReply) {
  drawer.replyTarget = target
  const rootId = 'replies' in target ? target.id : target.root_comment_id || target.parent_comment_id
  expandReplies(rootId)
}

async function syncCommentsUntil(commentID: number, shouldExist: boolean) {
  const videoID = state.video?.id
  if (!videoID) return

  for (let attempt = 0; attempt < commentSyncAttempts; attempt += 1) {
    if (attempt > 0) {
      await wait(commentSyncDelayMs)
    }
    if (!drawer.open || state.video?.id !== videoID) {
      return
    }

    try {
      const comments = await commentApi.listAll(videoID)
      if (hasCommentID(comments, commentID) === shouldExist) {
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
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  const content = drawer.content.trim()
  if (!content) return

  const videoId = state.video.id
  drawer.commentSubmitting = true
  drawer.error = ''
  try {
    const res = await commentApi.publish(videoId, content, drawer.replyTarget?.id)
    if (!drawer.open || state.video?.id !== videoId) return
    drawer.content = ''
    clearReplyTarget()
    applyComments(insertPublishedComment(drawer.comments, res.comment))
    expandReplies(res.comment.root_comment_id || res.comment.parent_comment_id)
    void syncCommentsUntil(res.comment.id, true)
    track('comment_submit', { video_id: videoId, is_reply: !!res.comment.parent_comment_id, from: 'detail' })
    toast.success('评论已发布')
  } catch (e) {
    if (!drawer.open || state.video?.id !== videoId) return
    drawer.error = e instanceof ApiError ? e.message : String(e)
    toast.error(drawer.error)
  } finally {
    if (state.video?.id === videoId) drawer.commentSubmitting = false
  }
}

function canDeleteComment(c: Comment | CommentReply) {
  const myId = auth.claims?.account_id
  return !!myId && (myId === c.author_id || myId === state.video?.author_id)
}

async function deleteComment(commentId: number) {
  if (!state.video) return
  if (!auth.isLoggedIn) return needLogin()
  if (!window.confirm('确认删除这条评论？')) return

  const videoId = state.video.id
  drawer.commentDeletingId = commentId
  drawer.error = ''
  try {
    await commentApi.remove(commentId)
    if (!drawer.open || state.video?.id !== videoId) return
    const nextComments = removeCommentByID(drawer.comments, commentId)
    applyComments(nextComments)
    if (drawer.replyTarget && !hasCommentID(nextComments, drawer.replyTarget.id)) clearReplyTarget()
    void syncCommentsUntil(commentId, false)
    toast.info('评论已删除')
  } catch (e) {
    if (!drawer.open || state.video?.id !== videoId) return
    drawer.error = e instanceof ApiError ? e.message : String(e)
    toast.error(drawer.error)
  } finally {
    if (state.video?.id === videoId) drawer.commentDeletingId = 0
  }
}

watch(
  () => id.value,
  async () => {
    unbindWatchProgress()
    watchSession.end(player.value ?? undefined, { from: 'detail' })
    player.value?.pause()
    playTime.value = 0
    playing.value = false
    closeDrawer()
    await loadVideo()
    await loadIsLiked()
    await nextTick()
    if (state.video) {
      await applyResume(state.video.id)
      bindWatchProgress(state.video.id, player.value)
    }
    await play()
  },
)

watch(
  () => auth.isLoggedIn,
  async () => {
    await loadIsLiked()
  },
)

onMounted(async () => {
  try {
    const saved = window.localStorage.getItem('feed.muted')
    if (saved !== null) muted.value = saved === 'true'
    const danmakuSaved = window.localStorage.getItem('feed.danmaku')
    if (danmakuSaved !== null) danmakuEnabled.value = danmakuSaved === 'true'
  } catch {
    // 某些隐私模式会禁用 localStorage，默认静音仍可保证自动播放。
  }
  await loadVideo()
  await loadIsLiked()
  await nextTick()
  if (state.video) {
    await applyResume(state.video.id)
    bindWatchProgress(state.video.id, player.value)
  }
  await play()
})

onBeforeUnmount(() => {
  if (tapTimer !== undefined) window.clearTimeout(tapTimer)
  unbindWatchProgress()
  watchSession.end(player.value ?? undefined, { from: 'detail' })
  player.value?.pause()
})
</script>

<template>
  <AppShell full>
    <div class="page">
      <div class="top">
        <div class="top-left">
          <RouterLink class="chip" to="/">← 返回推荐</RouterLink>
        </div>
        <div class="top-right" />
      </div>

      <div class="wrap">
        <FeedStageSkeleton v-if="state.loading" />
        <div v-else-if="state.error" class="center-hint bad">{{ state.error }}</div>

        <div v-else-if="state.video" class="stage has-composer" @click="onStageClick" @dblclick.prevent="onStageDoubleClick">
          <VideoPlayer
            ref="player"
            :src="state.video.play_url"
            :poster="state.video.cover_url"
            :active="true"
            :muted="muted"
            @playing="onPlayerPlaying"
            @paused="onPlayerPaused"
            @timeupdate="onPlayerTime"
          />
          <DanmakuLayer
            :video-id="state.video.id"
            :current-time="playTime"
            :playing="playing"
            :enabled="danmakuEnabled"
            :muted="muted"
            @toggle-enabled="toggleDanmaku"
            @toggle-muted="toggleMute"
          />
          <div class="grad" />

          <div class="meta">
            <RouterLink class="author-link" :to="`/u/${state.video.author_id}`" @click.stop>
              <UserAvatar :username="state.video.username" :id="state.video.author_id" :size="34" />
            </RouterLink>
            <div class="title">{{ state.video.title }}</div>
            <div v-if="state.video.description" class="desc">{{ state.video.description }}</div>
          </div>

          <div class="actions">
            <button class="act" type="button" :disabled="state.busy" @click.stop="toggleLike">
              <span class="icon" :class="{ liked: !!state.isLiked }">
                <AppIcon :name="state.isLiked ? 'heart-fill' : 'heart'" :size="28" />
              </span>
              <span class="count">{{ state.video.likes_count }}</span>
            </button>

            <button class="act" type="button" @click.stop="openComments">
              <span class="icon">
                <AppIcon name="comment" :size="26" />
              </span>
              <span class="count">{{ state.video.comment_count }}</span>
            </button>

            <button
              v-if="!auth.claims?.account_id || auth.claims.account_id !== state.video.author_id"
              class="act"
              type="button"
              :disabled="state.busy || social.isPending(state.video.author_id)"
              @click.stop="toggleFollow"
            >
              <span class="icon">
                <AppIcon name="plus" :size="26" />
              </span>
              <span class="count">{{ social.isFollowing(state.video.author_id) ? '已关注' : '关注' }}</span>
            </button>

            <button class="act" type="button" @click.stop="share">
              <span class="icon">
                <AppIcon name="share" :size="26" />
              </span>
              <span class="count">分享</span>
            </button>

            <button v-if="canTip" class="act" type="button" :disabled="state.busy" @click.stop="openTip">
              <span class="icon">
                <AppIcon name="coin" :size="26" />
              </span>
              <span class="count">打赏</span>
            </button>
            <button v-if="auth.isLoggedIn" class="act" type="button" @click.stop="openTips">
              <span class="icon">
                <AppIcon name="list" :size="24" />
              </span>
              <span class="count">打赏记录</span>
            </button>

            <button v-if="!isOwnVideo" class="act" type="button" @click.stop="openReport">
              <span class="icon">
                <AppIcon name="flag" :size="24" />
              </span>
              <span class="count">举报</span>
            </button>
          </div>
        </div>
      </div>

      <TipSheet
        v-if="state.video"
        ref="tipSheet"
        :video-id="state.video.id"
        :author-username="state.video.username"
        :is-author="isOwnVideo"
      />
      <ShareSheet ref="shareSheet" />
      <ReportSheet ref="reportSheet" />

      <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
        <div class="drawer">
          <div class="drawer-head">
            <div class="drawer-title">评论</div>
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

.top {
  height: 52px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.25);
  backdrop-filter: blur(16px);
}

.top-right {
  display: flex;
  gap: 8px;
  align-items: center;
}

.wrap {
  flex: 1;
  min-height: 0;
  display: grid;
}

.center-hint {
  place-self: center;
  color: rgba(255, 255, 255, 0.78);
}

.center-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.stage {
  width: 100%;
  height: 100%;
  min-height: 0;
  position: relative;
  overflow: hidden;
  background: #000;
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

.stage.has-composer .meta {
  bottom: 88px;
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
  text-shadow: 0 1px 8px rgba(0, 0, 0, 0.65);
}

.desc {
  color: rgba(255, 255, 255, 0.74);
  font-size: 13px;
  line-height: 1.35;
}

.actions {
  position: absolute;
  right: 10px;
  bottom: 18px;
  z-index: 3;
  display: grid;
  gap: 14px;
}

.act {
  appearance: none;
  width: 52px;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.96);
  padding: 0;
  cursor: pointer;
  display: grid;
  gap: 4px;
  justify-items: center;
}

.act:hover {
  color: #fff;
}

.act:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.icon {
  width: 40px;
  height: 40px;
  display: grid;
  place-items: center;
  filter: drop-shadow(0 2px 6px rgba(0, 0, 0, 0.55));
}

.icon.liked {
  color: rgba(254, 44, 85, 1);
}

.count {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.92);
  text-shadow: 0 1px 4px rgba(0, 0, 0, 0.75);
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

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}

@media (max-width: 900px) {
  .top {
    height: 48px;
    padding: 0 10px;
    gap: 8px;
  }

  .meta {
    left: 12px;
    right: 84px;
    bottom: calc(14px + env(safe-area-inset-bottom, 0px));
    max-width: none;
  }

  .stage.has-composer .meta {
    bottom: calc(84px + env(safe-area-inset-bottom, 0px));
  }

  .actions {
    right: 6px;
    bottom: calc(18px + env(safe-area-inset-bottom, 0px));
    gap: 12px;
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
