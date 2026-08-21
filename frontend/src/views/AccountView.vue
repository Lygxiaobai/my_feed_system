<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { track } from '../analytics/track'
import AppShell from '../components/AppShell.vue'
import Skeleton from '../components/Skeleton.vue'
import UserAvatar from '../components/UserAvatar.vue'
import UserListSkeleton from '../components/UserListSkeleton.vue'
import VideoGridSkeleton from '../components/VideoGridSkeleton.vue'
import { ApiError } from '../api/client'
import * as accountApi from '../api/account'
import * as opsApi from '../api/ops'
import type { AuditStatus, SocialRelation, Video } from '../api/types'
import * as videoApi from '../api/video'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()

const busy = ref(false)
const emailForm = reactive({ email: '', code: '' })
const sendingCode = ref(false)
const opsAllowed = ref(false)
const countdown = ref(0)
let countdownTimer: number | undefined

const me = computed(() => ({
  id: auth.claims?.account_id ?? 0,
  username: auth.claims?.username ?? '',
}))

const myVideos = reactive({
  loading: false,
  error: '',
  items: [] as Video[],
})

const totalReceivedLikes = computed(() => myVideos.items.reduce((sum, item) => sum + (item.likes_count ?? 0), 0))

type VideoTab = 'works' | 'likes'
const videoTab = ref<VideoTab>('works')

let myVideosReq = 0
async function loadMyVideos() {
  const id = me.value.id
  if (!auth.isLoggedIn || !id) {
    myVideos.items = []
    myVideos.error = ''
    myVideos.loading = false
    return
  }
  if (myVideos.loading) return

  const req = ++myVideosReq
  myVideos.loading = true
  myVideos.error = ''
  try {
    const vids = await videoApi.listByAuthorId(id)
    if (req !== myVideosReq) return
    myVideos.items = vids
  } catch (e) {
    if (req !== myVideosReq) return
    myVideos.error = e instanceof ApiError ? e.message : String(e)
    myVideos.items = []
  } finally {
    if (req === myVideosReq) myVideos.loading = false
  }
}

const likedVideos = reactive({
  loading: false,
  loaded: false,
  error: '',
  items: [] as Video[],
})

const likedVideoCountText = computed(() => {
  if (!auth.isLoggedIn) return '0'
  if (likedVideos.loading || !likedVideos.loaded) return '…'
  return String(likedVideos.items.length)
})

let likedVideosReq = 0

async function loadLikedVideos() {
  if (!auth.isLoggedIn) {
    likedVideos.loading = false
    likedVideos.loaded = false
    likedVideos.error = ''
    likedVideos.items = []
    return
  }
  if (likedVideos.loading) return

  const req = ++likedVideosReq
  likedVideos.loading = true
  likedVideos.error = ''
  try {
    const vids = await videoApi.listLiked()
    if (req !== likedVideosReq) return
    likedVideos.items = vids
    likedVideos.loaded = true
  } catch (e) {
    if (req !== likedVideosReq) return
    likedVideos.error = e instanceof ApiError ? e.message : String(e)
    likedVideos.items = []
    likedVideos.loaded = false
  } finally {
    if (req === likedVideosReq) likedVideos.loading = false
  }
}

async function goVideo(id: number) {
  await router.push(`/video/${id}`)
}

/**
 * 把审核状态映射成角标文案。
 *
 * 只在自己的作品列表里展示——他人看到的列表本就只有已过审内容。
 * 被拒时刻意只给通用文案，不展示命中了什么规则：一旦回显，
 * 就能通过反复修改试探出词库边界。
 */
function auditBadge(status?: AuditStatus): { text: string; tone: string } | null {
  switch (status) {
    case 'pending':
      return { text: '审核中', tone: 'wait' }
    case 'reviewing':
      return { text: '人工复审中', tone: 'wait' }
    case 'rejected':
      return { text: '未通过', tone: 'bad' }
    default:
      // approved 或字段缺失时不显示角标，避免正常内容平白多出干扰元素。
      return null
  }
}

function openWorksVideos() {
  videoTab.value = 'works'
  void loadMyVideos()
}

function openLikedVideos() {
  videoTab.value = 'likes'
  void loadLikedVideos()
}

onUnmounted(() => {
  if (countdownTimer !== undefined) window.clearInterval(countdownTimer)
})

function startCountdown() {
  countdown.value = 60
  if (countdownTimer !== undefined) window.clearInterval(countdownTimer)
  countdownTimer = window.setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0 && countdownTimer !== undefined) {
      window.clearInterval(countdownTimer)
      countdownTimer = undefined
    }
  }, 1000)
}

async function onSendCode() {
  const email = emailForm.email.trim()
  if (!email) {
    toast.error('请输入邮箱')
    return
  }
  if (sendingCode.value || countdown.value > 0) return
  sendingCode.value = true
  try {
    await accountApi.sendEmailCode(email)
    startCountdown()
    toast.success('验证码已发送')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    sendingCode.value = false
  }
}

async function onEmailLogin() {
  if (busy.value) return
  const email = emailForm.email.trim()
  const code = emailForm.code.trim()
  if (!email || !code) {
    toast.error('请输入邮箱和验证码')
    return
  }

  busy.value = true
  try {
    const res = await accountApi.verifyEmail(email, code)
    auth.setToken(res.token)
    if (res.created) track('register')
    track('login')
    toast.success(res.created ? '注册并登录成功' : '登录成功')
    await social.refreshMine()
    await Promise.all([loadMyVideos(), loadLikedVideos()])
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    busy.value = false
  }
}

function onOauthSoon() {
  toast.info('即将开放')
}

async function goSettings() {
  await router.push('/settings')
}

async function goWallet() {
  await router.push('/wallet')
}

async function goOps() {
  await router.push('/ops')
}

async function loadOpsAccess() {
  if (!auth.isLoggedIn) {
    opsAllowed.value = false
    return
  }
  try {
    const access = await opsApi.opsAccess()
    opsAllowed.value = access.allowed
  } catch {
    opsAllowed.value = false
  }
}

type ListTab = 'followers' | 'following'
const drawer = reactive({
  open: false,
  tab: 'followers' as ListTab,
})

function openFollowers() {
  drawer.tab = 'followers'
  drawer.open = true
}

function openFollowing() {
  drawer.tab = 'following'
  drawer.open = true
}

function closeDrawer() {
  drawer.open = false
}

const listTitle = computed(() => (drawer.tab === 'followers' ? '粉丝' : '关注'))
const listItems = computed(() => (drawer.tab === 'followers' ? social.followers : social.vloggers))
const drawerLoading = computed(() => (drawer.tab === 'followers' ? social.followersLoading : social.vloggersLoading))
const drawerError = computed(() => (drawer.tab === 'followers' ? social.followersError : social.vloggersError))
const socialErrorHint = computed(() => social.followersError || social.vloggersError)

function relationUserId(item: SocialRelation) {
  return drawer.tab === 'followers' ? item.follower_id : item.vlogger_id
}

function relationUsername(item: SocialRelation) {
  return drawer.tab === 'followers'
    ? item.follower_username || `用户 #${relationUserId(item)}`
    : item.vlogger_username || `用户 #${relationUserId(item)}`
}

async function goUser(item: SocialRelation) {
  drawer.open = false
  await router.push(`/u/${relationUserId(item)}`)
}

watch(
  () => auth.isLoggedIn,
  (v) => {
    if (!v) {
      drawer.open = false
      myVideosReq += 1
      likedVideosReq += 1
      myVideos.loading = false
      myVideos.items = []
      myVideos.error = ''

      likedVideos.loading = false
      likedVideos.loaded = false
      likedVideos.items = []
      likedVideos.error = ''

      videoTab.value = 'works'
      opsAllowed.value = false
    }
  },
)

watch(
  () => me.value.id,
  (id) => {
    if (auth.isLoggedIn && id) {
      void loadMyVideos()
      void loadLikedVideos()
      void loadOpsAccess()
    } else {
      opsAllowed.value = false
    }
  },
  { immediate: true },
)
</script>

<template>
  <AppShell>
    <div v-if="!auth.isLoggedIn" class="login-wrap">
      <div class="card login-card">
        <p class="title">登录 / 注册</p>
        <div class="grid" style="margin-top: 10px">
          <div>
            <label>邮箱</label>
            <input v-model.trim="emailForm.email" type="email" autocomplete="email" />
          </div>
          <div>
            <label>验证码</label>
            <div class="code-row">
              <input
                v-model.trim="emailForm.code"
                inputmode="numeric"
                maxlength="6"
                autocomplete="one-time-code"
                @keydown.enter="onEmailLogin"
              />
              <button class="ghost" type="button" :disabled="sendingCode || countdown > 0" @click="onSendCode">
                {{ countdown > 0 ? `${countdown}s` : '发送验证码' }}
              </button>
            </div>
          </div>
          <button class="primary" type="button" :disabled="busy" @click="onEmailLogin">登录 / 注册</button>
        </div>

        <div class="oauth-row">
          <button class="ghost oauth" type="button" @click="onOauthSoon">微信登录</button>
          <button class="ghost oauth" type="button" @click="onOauthSoon">QQ 登录</button>
        </div>

        <button class="linkish" type="button" @click="router.push('/account/password')">账号密码登录</button>
      </div>
    </div>

    <template v-else>
      <div class="card">
        <div class="row" style="justify-content: space-between; align-items: flex-start">
          <div class="row" style="gap: 12px; align-items: center">
            <UserAvatar :username="me.username" :id="me.id" :size="64" />
            <div>
              <div class="title" style="margin: 0">{{ me.username }}</div>
            </div>
          </div>

          <div class="row">
            <button class="ghost" type="button" @click="router.push('/checkin')">签到</button>
            <button class="ghost" type="button" @click="router.push('/lottery')">抽奖</button>
            <button class="ghost" type="button" @click="goWallet">钱包</button>
            <button v-if="opsAllowed" class="ghost" type="button" @click="goOps">运维</button>
            <button class="ghost" type="button" @click="goSettings">设置</button>
          </div>
        </div>

        <div class="row" style="margin-top: 14px">
          <button class="metric" type="button" :disabled="social.followersLoading" @click="openFollowers">
            <div class="metric-num">
              <Skeleton v-if="social.followersLoading" width="28px" height="18px" />
              <template v-else>{{ social.followerCount }}</template>
            </div>
            <div class="metric-label">粉丝</div>
          </button>
          <button class="metric" type="button" :disabled="social.vloggersLoading" @click="openFollowing">
            <div class="metric-num">
              <Skeleton v-if="social.vloggersLoading" width="28px" height="18px" />
              <template v-else>{{ social.followingCount }}</template>
            </div>
            <div class="metric-label">关注</div>
          </button>
          <button class="metric" type="button" :class="{ active: videoTab === 'works' }" @click="openWorksVideos">
            <div class="metric-num">
              <Skeleton v-if="myVideos.loading" width="28px" height="18px" />
              <template v-else>{{ myVideos.items.length }}</template>
            </div>
            <div class="metric-label">作品</div>
          </button>
          <div class="metric static">
            <div class="metric-num">
              <Skeleton v-if="myVideos.loading" width="36px" height="18px" />
              <template v-else>{{ totalReceivedLikes }}</template>
            </div>
            <div class="metric-label">获赞</div>
          </div>
          <div v-if="socialErrorHint" class="subtle" style="margin-left: 8px">社交信息加载失败：{{ socialErrorHint }}</div>
        </div>
      </div>

      <div class="card" style="margin-top: 14px">
        <div class="row" style="justify-content: space-between">
          <p class="title" style="margin: 0">{{ videoTab === 'works' ? '作品' : '点赞视频' }}</p>
          <div class="row" style="gap: 8px">
            <button class="ghost" type="button" :class="{ active: videoTab === 'works' }" @click="openWorksVideos">作品</button>
            <button class="ghost" type="button" :class="{ active: videoTab === 'likes' }" @click="openLikedVideos">
              点赞视频
              <span class="subtle">({{ likedVideoCountText }})</span>
            </button>
          </div>
        </div>

        <template v-if="videoTab === 'works'">
          <VideoGridSkeleton v-if="myVideos.loading" style="margin-top: 12px" />
          <div v-else-if="myVideos.error" class="hint bad" style="margin-top: 12px">{{ myVideos.error }}</div>
          <div v-else-if="myVideos.items.length === 0" class="hint" style="margin-top: 12px">暂无作品</div>

          <div v-else class="video-grid" style="margin-top: 12px">
            <button v-for="v in myVideos.items" :key="v.id" class="video-card" type="button" @click="goVideo(v.id)">
              <img class="video-cover" :src="v.cover_url" :alt="v.title" loading="lazy" />
              <span v-if="auditBadge(v.audit_status)" class="audit-badge" :class="auditBadge(v.audit_status)?.tone">
                {{ auditBadge(v.audit_status)?.text }}
              </span>
              <div class="video-meta">
                <div class="video-title">{{ v.title }}</div>
                <div class="video-sub subtle">❤️ {{ v.likes_count }} · 💬 {{ v.comment_count }} · {{ new Date(v.created_at).toLocaleDateString() }}</div>
              </div>
            </button>
          </div>
        </template>
        <template v-else>
          <VideoGridSkeleton v-if="likedVideos.loading" style="margin-top: 12px" />
          <div v-else-if="likedVideos.error" class="hint bad" style="margin-top: 12px">{{ likedVideos.error }}</div>
          <div v-else-if="likedVideos.items.length === 0" class="hint" style="margin-top: 12px">暂无点赞视频</div>

          <div v-else class="video-grid" style="margin-top: 12px">
            <button v-for="v in likedVideos.items" :key="v.id" class="video-card" type="button" @click="goVideo(v.id)">
              <img class="video-cover" :src="v.cover_url" :alt="v.title" loading="lazy" />
              <div class="video-meta">
                <div class="video-title">{{ v.title }}</div>
                <div class="video-sub subtle">❤️ {{ v.likes_count }} · 💬 {{ v.comment_count }} · {{ new Date(v.created_at).toLocaleDateString() }}</div>
              </div>
            </button>
          </div>
        </template>
      </div>
    </template>

    <div v-if="drawer.open" class="drawer-backdrop" @click.self="closeDrawer">
      <div class="drawer">
        <div class="drawer-head">
          <div class="drawer-title">{{ listTitle }}</div>
          <button class="drawer-x" type="button" @click="closeDrawer">×</button>
        </div>
        <div class="drawer-body">
          <UserListSkeleton v-if="drawerLoading" />
          <div v-else-if="drawerError" class="drawer-hint bad">{{ drawerError }}</div>
          <div v-else-if="listItems.length === 0" class="drawer-hint">暂无</div>

          <button v-for="u in listItems" v-if="!drawerLoading && !drawerError" :key="u.id" class="user-row" type="button" @click="goUser(u)">
            <UserAvatar :username="relationUsername(u)" :id="relationUserId(u)" :size="40" />
            <div class="user-meta">
              <div class="user-name">{{ relationUsername(u) }}</div>
            </div>
          </button>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.login-wrap {
  display: grid;
  justify-items: center;
  align-content: start;
  padding: clamp(56px, 14vh, 160px) 16px 40px;
}

.login-card {
  width: min(420px, 100%);
}

.code-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
}

.oauth-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-top: 14px;
}

.oauth {
  text-align: center;
}

.linkish {
  margin-top: 14px;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.62);
  padding: 0;
  cursor: pointer;
  text-align: left;
}

.linkish:hover {
  color: rgba(255, 255, 255, 0.9);
}

.ghost {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.18);
  color: rgba(255, 255, 255, 0.86);
  border-radius: 12px;
  padding: 10px 12px;
  cursor: pointer;
}

.ghost:hover {
  background: rgba(255, 255, 255, 0.1);
}

.ghost.active {
  background: rgba(254, 44, 85, 0.14);
  border-color: rgba(254, 44, 85, 0.55);
}

.metric {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  border-radius: 16px;
  padding: 12px 14px;
  min-width: 120px;
  cursor: pointer;
  display: grid;
  gap: 4px;
  text-align: left;
}

.metric:hover {
  background: rgba(255, 255, 255, 0.1);
}

.metric.active {
  background: rgba(254, 44, 85, 0.14);
  border-color: rgba(254, 44, 85, 0.55);
}

.metric.static {
  cursor: default;
}

.metric:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.metric-num {
  font-size: 18px;
  font-weight: 900;
  letter-spacing: 0.2px;
}

.metric-label {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.65);
}

.hint {
  color: rgba(255, 255, 255, 0.78);
}

.hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.drawer-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.55);
  backdrop-filter: blur(10px);
  z-index: 120;
  display: grid;
  justify-items: center;
  align-items: center;
  padding: 16px;
}

.drawer {
  width: min(520px, calc(100vw - 18px));
  max-height: min(78vh, 720px);
  background: rgba(0, 0, 0, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 18px;
  overflow: hidden;
  display: grid;
  grid-template-rows: auto 1fr;
}

.drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 14px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.drawer-title {
  font-weight: 900;
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

.drawer-hint {
  color: rgba(255, 255, 255, 0.78);
  padding: 12px 0;
}

.drawer-hint.bad {
  color: rgba(254, 44, 85, 0.92);
}

.video-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 1100px) {
  .video-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 800px) {
  .video-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

.video-card {
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.05);
  border-radius: 16px;
  overflow: hidden;
  cursor: pointer;
  padding: 0;
  text-align: left;
  position: relative;
}

.audit-badge {
  position: absolute;
  top: 8px;
  left: 8px;
  padding: 3px 8px;
  border-radius: 999px;
  font-size: 11px;
  line-height: 1.4;
  backdrop-filter: blur(8px);
  border: 1px solid rgba(255, 255, 255, 0.18);
}

.audit-badge.wait {
  background: rgba(0, 0, 0, 0.55);
  color: rgba(255, 255, 255, 0.9);
}

.audit-badge.bad {
  background: rgba(254, 44, 85, 0.85);
  color: #fff;
  border-color: rgba(254, 44, 85, 0.6);
}

.video-card:hover {
  background: rgba(255, 255, 255, 0.08);
}

.video-cover {
  width: 100%;
  aspect-ratio: 9/12;
  object-fit: cover;
  display: block;
  background: rgba(0, 0, 0, 0.35);
}

.video-meta {
  padding: 10px 10px;
}

.video-title {
  font-weight: 800;
  font-size: 13px;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.video-sub {
  margin-top: 6px;
  font-size: 12px;
}

.user-row {
  text-align: left;
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 12px;
  align-items: center;
  padding: 10px 10px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.05);
  cursor: pointer;
}

.user-row:hover {
  background: rgba(255, 255, 255, 0.08);
}

.user-meta {
  min-width: 0;
}

.user-name {
  font-weight: 800;
}

.user-id {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.6);
}

.mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace;
}
</style>



