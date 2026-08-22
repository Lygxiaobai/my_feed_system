<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '../../api/client'
import * as adminApi from '../../api/admin'
import type { AdminAccount, AdminVideo } from '../../api/admin'
import { AUDIT_STATUS_LABEL } from '../../api/admin'
import { useToastStore } from '../../stores/toast'

const route = useRoute()
const router = useRouter()
const toast = useToastStore()

const query = ref('')
const loading = ref(false)
const account = ref<AdminAccount | null>(null)
const videos = ref<AdminVideo[]>([])

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN')
}

function parseQuery(raw: string) {
  const text = raw.trim()
  if (!text) return null
  if (text.includes('@')) return { email: text.toLowerCase() }
  if (/^\d+$/.test(text)) return { id: Number(text) }
  return { username: text }
}

async function lookup(raw?: string) {
  const parsed = parseQuery(raw ?? query.value)
  if (!parsed) {
    toast.error('请输入账号 ID、用户名或邮箱')
    return
  }
  loading.value = true
  try {
    const result = await adminApi.lookupAdminAccount(parsed)
    account.value = result.account
    videos.value = result.videos
    const nextQuery = parsed.id
      ? { id: String(parsed.id) }
      : parsed.username
        ? { username: parsed.username }
        : { email: parsed.email || '' }
    await router.replace({ path: '/admin/users', query: nextQuery })
  } catch (e) {
    account.value = null
    videos.value = []
    toast.error(e instanceof ApiError ? e.message : '查询失败')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  const id = String(route.query.id || '')
  const username = String(route.query.username || '')
  const email = String(route.query.email || '')
  const initial = id || username || email
  if (initial) {
    query.value = initial
    void lookup(initial)
  }
})
</script>

<template>
  <div>
    <p class="title">用户查询</p>
    <p class="subtle">可用账号 ID、用户名或邮箱查找。邮箱只在管理后台展示，不会出现在公开资料里。</p>

    <div class="row search">
      <input v-model.trim="query" placeholder="ID / 用户名 / 邮箱" @keydown.enter="lookup()" />
      <button class="primary" type="button" :disabled="loading" @click="lookup()">查询</button>
    </div>

    <div v-if="account" class="card">
      <h2>{{ account.username }}</h2>
      <p class="subtle">
        #{{ account.id }}
        <template v-if="account.email"> · {{ account.email }}</template>
        · 粉丝 {{ account.follower_count }}
        · 注册 {{ formatTime(account.created_at) }}
      </p>
    </div>

    <div v-if="account" class="grid">
      <button
        v-for="item in videos"
        :key="item.id"
        class="card work"
        type="button"
        @click="router.push({ path: '/admin/videos', query: { id: String(item.id) } })"
      >
        <img v-if="item.cover_url" :src="item.cover_url" alt="" />
        <div>
          <strong>{{ item.title }}</strong>
          <p class="subtle">#{{ item.id }} · {{ AUDIT_STATUS_LABEL[item.audit_status] }}</p>
        </div>
      </button>
      <p v-if="videos.length === 0" class="subtle">该账号还没有作品。</p>
    </div>
  </div>
</template>

<style scoped>
.search {
  margin: 16px 0;
}

.search input {
  max-width: 320px;
}

h2 {
  margin: 0 0 6px;
  font-size: 18px;
}

.grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 14px;
}

.work {
  display: grid;
  grid-template-columns: 88px minmax(0, 1fr);
  gap: 10px;
  text-align: left;
  align-items: center;
}

.work img {
  width: 88px;
  height: 66px;
  object-fit: cover;
  border-radius: 10px;
  background: var(--fill);
}

.work strong {
  display: block;
  font-size: 14px;
}
</style>
