<script setup lang="ts">
import { ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'

import { track } from '../analytics/track'
import { ApiError } from '../api/client'
import * as videoApi from '../api/video'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'
import Toaster from './Toaster.vue'
import UserAvatar from './UserAvatar.vue'

type HeaderAction = {
  key: string
  label: string
  icon: string
  to: string
  auth?: boolean
}

const headerActions: HeaderAction[] = [
  { key: 'wallet', label: '充钻石', icon: '◇', to: '/wallet', auth: true },
  { key: 'client', label: '客户端', icon: '↓', to: '/client' },
  { key: 'wallpaper', label: '壁纸', icon: '☆', to: '/wallpaper' },
  { key: 'notify', label: '通知', icon: '◉', to: '/notifications', auth: true },
  { key: 'messages', label: '消息', icon: '✈', to: '/messages', auth: true },
  { key: 'publish', label: '投稿', icon: '+', to: '/video', auth: true },
]

const props = defineProps<{ full?: boolean }>()

const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()
const router = useRouter()
const route = useRoute()

const search = ref(typeof route.query.q === 'string' ? route.query.q : '')
const searchOpen = ref(false)

watch(
  () => route.query.q,
  (v) => {
    search.value = typeof v === 'string' ? v : ''
  },
)

watch(
  () => route.fullPath,
  () => {
    searchOpen.value = false
  },
)

watch(
  () => auth.isLoggedIn,
  (v) => {
    if (v) void social.refreshMine()
    else social.clear()
  },
  { immediate: true },
)

/**
 * 搜索框同时承担「粘贴口令直达」的职责，这与抖音的交互位置一致：
 * 用户拿到的是一整段文案，最自然的动作就是往搜索框里一贴。
 *
 * 不做「打开页面时自动嗅探剪贴板」：navigator.clipboard.readText 只在安全上下文
 * 可用，而本站的明文 IP 入口不是安全上下文，那条路径下必然失败。
 */
async function onSearch() {
  const q = search.value.trim()
  searchOpen.value = false
  if (!q) {
    await router.push({ path: '/', query: {} })
    return
  }

  const confidence = videoApi.shareTextConfidence(q)
  if (confidence !== 'none') {
    try {
      const video = await videoApi.resolveShare(q)
      search.value = ''
      await router.push(`/video/${video.id}`)
      return
    } catch (e) {
      // 口令形态明确却解析不出来，多半是内容已下架或口令抄错，直接告诉用户；
      // 只是「碰巧 8 位」的普通搜索词则静默退回搜索。
      if (confidence === 'certain') {
        toast.error(e instanceof ApiError ? e.message : '口令无效或内容已下架')
        return
      }
    }
  }

  track('search', { query: q })
  await router.push({ path: '/', query: { q } })
}

async function goLogin() {
  await router.push('/account')
}

async function goSettings() {
  await router.push('/settings')
}

function headerActionTo(action: HeaderAction) {
  if (action.auth && !auth.isLoggedIn) return '/account'
  return action.to
}
</script>

<template>
  <div class="dy-shell" :class="{ full: props.full }">
    <aside class="dy-aside desktop-only">
      <RouterLink class="dy-logo" to="/">ShortVideo</RouterLink>

      <nav class="dy-nav">
        <RouterLink class="dy-nav-link" to="/">推荐</RouterLink>
        <RouterLink class="dy-nav-link" to="/following">关注</RouterLink>
        <RouterLink class="dy-nav-link" to="/likes">点赞榜</RouterLink>
        <RouterLink class="dy-nav-link" to="/hot">热榜</RouterLink>
        <RouterLink class="dy-nav-link" to="/video">发布</RouterLink>
        <RouterLink class="dy-nav-link" to="/account">账号</RouterLink>
        <RouterLink class="dy-nav-link" to="/settings">设置</RouterLink>
      </nav>

      <div class="dy-aside-foot">
        <div class="dy-user-actions">
          <button v-if="!auth.isLoggedIn" class="dy-btn dy-btn-primary" type="button" @click="goLogin">登录</button>
          <button v-else class="dy-btn dy-btn-primary" type="button" @click="goSettings">设置</button>
        </div>
      </div>
    </aside>

    <div class="dy-main">
      <header class="dy-topbar">
        <div class="dy-top-left desktop-only" />

        <RouterLink class="dy-mobile-brand mobile-only" to="/">ShortVideo</RouterLink>

        <div class="dy-search desktop-only">
          <input v-model="search" class="dy-search-input" aria-label="搜索" @keydown.enter="onSearch" />
          <button class="dy-btn dy-btn-primary" type="button" @click="onSearch">搜索</button>
        </div>

        <div class="dy-top-actions">
          <button class="dy-icon-btn mobile-only" type="button" aria-label="搜索" @click="searchOpen = !searchOpen">
            ⌕
          </button>
          <RouterLink
            v-for="action in headerActions"
            :key="action.key"
            class="dy-head-act desktop-only"
            :class="{ on: route.path === action.to }"
            :to="headerActionTo(action)"
          >
            <span class="dy-head-icon" aria-hidden="true">{{ action.icon }}</span>
            <span class="dy-head-label">{{ action.label }}</span>
          </RouterLink>
          <RouterLink class="dy-head-avatar" to="/account" :title="auth.isLoggedIn ? '账号' : '登录'">
            <UserAvatar
              :username="auth.isLoggedIn ? (auth.claims?.username ?? '') : '登录'"
              :id="auth.claims?.account_id"
              :size="34"
            />
          </RouterLink>
        </div>
      </header>

      <div v-if="searchOpen" class="dy-mobile-search mobile-only">
        <input
          v-model="search"
          class="dy-search-input"
          aria-label="搜索"
          autofocus
          @keydown.enter="onSearch"
        />
        <button class="dy-btn dy-btn-primary" type="button" @click="onSearch">搜索</button>
      </div>

      <div class="dy-content" :class="props.full ? 'full' : 'padded'">
        <template v-if="props.full">
          <slot />
        </template>
        <template v-else>
          <div class="container">
            <slot />
          </div>
        </template>
      </div>
    </div>

    <nav class="dy-bottom-nav mobile-only" aria-label="底部导航">
      <RouterLink class="dy-tab" :class="{ on: route.path === '/' }" to="/">
        <span class="dy-tab-icon">⌂</span>
        <span>首页</span>
      </RouterLink>
      <RouterLink class="dy-tab" :class="{ on: route.path === '/hot' }" to="/hot">
        <span class="dy-tab-icon">▲</span>
        <span>热榜</span>
      </RouterLink>
      <RouterLink class="dy-tab publish" to="/video">
        <span class="dy-tab-publish">+</span>
        <span>发布</span>
      </RouterLink>
      <RouterLink class="dy-tab" :class="{ on: route.path.startsWith('/account') }" to="/account">
        <span class="dy-tab-icon">☺</span>
        <span>我的</span>
      </RouterLink>
    </nav>

    <Toaster />
  </div>
</template>

<style scoped>
.dy-shell {
  height: var(--app-height, 100dvh);
  min-height: var(--app-height, 100dvh);
  display: grid;
  grid-template-columns: 240px 1fr;
  background: radial-gradient(1200px 900px at 20% -25%, rgba(254, 44, 85, 0.18), transparent 60%),
    radial-gradient(900px 700px at 90% 10%, rgba(37, 244, 238, 0.12), transparent 55%), transparent;
  overflow: hidden;
}

.dy-aside {
  border-right: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(16px);
  padding: 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  min-height: 0;
}

.dy-logo {
  font-weight: 900;
  letter-spacing: 0.4px;
  font-size: 18px;
  padding: 10px 10px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  text-decoration: none;
}

.dy-nav {
  display: grid;
  gap: 8px;
}

.dy-nav-link {
  padding: 10px 10px;
  border-radius: 12px;
  border: 1px solid transparent;
  background: rgba(255, 255, 255, 0.04);
  text-decoration: none;
}

.dy-nav-link.router-link-active {
  border-color: rgba(254, 44, 85, 0.42);
  background: rgba(254, 44, 85, 0.12);
}

.dy-aside-foot {
  margin-top: auto;
  display: grid;
  gap: 10px;
  padding-top: 12px;
  border-top: 1px solid rgba(255, 255, 255, 0.08);
}

.dy-user-actions {
  display: flex;
  gap: 10px;
}

.dy-btn {
  appearance: none;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.9);
  border-radius: 12px;
  padding: 10px 12px;
  cursor: pointer;
  text-decoration: none;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  justify-content: center;
  font-size: 13px;
  min-height: 40px;
}

.dy-btn:hover {
  background: rgba(255, 255, 255, 0.1);
}

.dy-btn-primary {
  border-color: rgba(254, 44, 85, 0.5);
  background: rgba(254, 44, 85, 0.16);
}

.dy-btn-primary:hover {
  background: rgba(254, 44, 85, 0.24);
}

.dy-btn-ghost {
  border-color: rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.15);
}

.dy-main {
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.dy-topbar {
  height: var(--topbar-h, 56px);
  flex-shrink: 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(16px);
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(200px, 480px) auto;
  gap: 12px;
  align-items: center;
  padding: 0 14px;
  padding-top: var(--safe-top, 0px);
  box-sizing: content-box;
  min-height: var(--topbar-h, 56px);
  z-index: 30;
}

.dy-search {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 10px;
  align-items: center;
  max-width: 680px;
  width: 100%;
  justify-self: center;
}

.dy-search-input {
  width: 100%;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 999px;
  color: rgba(255, 255, 255, 0.9);
  padding: 10px 14px;
  outline: none;
  font-size: 16px;
}

.dy-search-input:focus {
  border-color: rgba(37, 244, 238, 0.42);
  box-shadow: 0 0 0 3px rgba(37, 244, 238, 0.14);
}

.dy-top-actions {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 2px;
  min-width: 0;
}

.dy-head-act {
  width: 52px;
  height: 48px;
  border-radius: 10px;
  text-decoration: none;
  color: rgba(255, 255, 255, 0.78);
  display: grid;
  place-items: center;
  gap: 2px;
  padding: 4px 2px;
}

.dy-head-act:hover,
.dy-head-act.on {
  color: rgba(255, 255, 255, 0.96);
  background: rgba(255, 255, 255, 0.06);
}

.dy-head-icon {
  font-size: 16px;
  line-height: 1;
}

.dy-head-label {
  font-size: 10px;
  line-height: 1.1;
  white-space: nowrap;
}

.dy-head-avatar {
  margin-left: 6px;
  display: grid;
  place-items: center;
  text-decoration: none;
}

.dy-icon-btn {
  width: 40px;
  height: 40px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(255, 255, 255, 0.06);
  color: rgba(255, 255, 255, 0.92);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  display: inline-grid;
  place-items: center;
  padding: 0;
}

.dy-mobile-brand {
  font-weight: 900;
  font-size: 16px;
  text-decoration: none;
  color: rgba(255, 255, 255, 0.95);
  letter-spacing: 0.2px;
}

.dy-mobile-search {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 8px;
  padding: 8px 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(0, 0, 0, 0.4);
  flex-shrink: 0;
}

.dy-content {
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.dy-content.padded {
  overflow: auto;
  -webkit-overflow-scrolling: touch;
  padding-bottom: 0;
}

.dy-content.full {
  overflow: hidden;
}

.dy-bottom-nav {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 80;
  height: var(--bottom-nav-h, 56px);
  padding-bottom: var(--safe-bottom, 0px);
  box-sizing: border-box;
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  align-items: stretch;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(8, 8, 12, 0.92);
  backdrop-filter: blur(16px);
}

.dy-tab {
  appearance: none;
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.62);
  display: grid;
  place-items: center;
  gap: 2px;
  padding: 6px 2px 8px;
  font-size: 11px;
  text-decoration: none;
  cursor: pointer;
  min-width: 0;
}

.dy-tab.on,
.dy-tab.router-link-active {
  color: rgba(255, 255, 255, 0.96);
}

.dy-tab-icon {
  font-size: 16px;
  line-height: 1;
}

.dy-tab-publish {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  display: grid;
  place-items: center;
  font-size: 22px;
  font-weight: 700;
  line-height: 1;
  color: #fff;
  background: linear-gradient(135deg, rgba(37, 244, 238, 0.95), rgba(254, 44, 85, 0.95));
  box-shadow: 0 8px 18px rgba(254, 44, 85, 0.28);
}

.dy-tab.publish {
  color: rgba(255, 255, 255, 0.9);
}

.mobile-only {
  display: none !important;
}

.desktop-only {
  display: initial;
}

.dy-aside.desktop-only {
  display: flex;
}

.dy-search.desktop-only {
  display: grid;
}

.dy-btn.desktop-only {
  display: inline-flex;
}

.dy-head-act.desktop-only {
  display: grid;
}

.dy-top-left.desktop-only {
  display: block;
}

@media (max-width: 900px) {
  .dy-shell {
    grid-template-columns: 1fr;
  }

  .desktop-only {
    display: none !important;
  }

  .mobile-only {
    display: initial !important;
  }

  .dy-bottom-nav.mobile-only {
    display: grid !important;
  }

  .dy-mobile-search.mobile-only {
    display: grid !important;
  }

  .dy-topbar {
    grid-template-columns: auto 1fr auto;
    padding: 0 10px;
    gap: 8px;
  }

  .dy-main {
    padding-bottom: var(--bottom-nav-h, 56px);
  }

  .dy-content.full {
    /* bottom nav already reserved by main padding */
  }

  .dy-top-actions {
    justify-content: flex-end;
  }
}
</style>

