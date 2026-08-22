<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '../../api/client'
import * as adminApi from '../../api/admin'
import type { AdminVideo } from '../../api/admin'
import { AUDIT_STATUS_LABEL } from '../../api/admin'
import { useToastStore } from '../../stores/toast'

const emit = defineEmits<{ changed: [] }>()
const route = useRoute()
const router = useRouter()
const toast = useToastStore()

const query = ref('')
const loading = ref(false)
const takingDown = ref(false)
const video = ref<AdminVideo | null>(null)
const note = ref('')

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN')
}

async function lookup(raw?: string) {
  const text = (raw ?? query.value).trim()
  const id = Number(text)
  if (!Number.isInteger(id) || id <= 0) {
    toast.error('请输入有效的视频 ID')
    return
  }
  loading.value = true
  try {
    video.value = await adminApi.lookupAdminVideo(id)
    query.value = String(id)
    if (route.query.id !== String(id)) {
      await router.replace({ path: '/admin/videos', query: { id: String(id) } })
    }
  } catch (e) {
    video.value = null
    toast.error(e instanceof ApiError ? e.message : '查询失败')
  } finally {
    loading.value = false
  }
}

async function takedown() {
  if (!video.value || takingDown.value) return
  const reason = note.value.trim()
  if (!reason) {
    toast.error('下架必须填写处置说明')
    return
  }
  if (!window.confirm(`确认下架「${video.value.title}」？公开信息流将不再展示该内容。`)) return

  takingDown.value = true
  try {
    await adminApi.takedownAdminVideo(video.value.id, reason)
    toast.success('已下架')
    note.value = ''
    emit('changed')
    await lookup(String(video.value.id))
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '下架失败')
  } finally {
    takingDown.value = false
  }
}

onMounted(() => {
  const initial = String(route.query.id || '')
  if (initial) {
    query.value = initial
    void lookup(initial)
  }
})
</script>

<template>
  <div>
    <p class="title">内容查询</p>
    <p class="subtle">按视频 ID 查看任意审核状态。未过审内容对普通用户仍显示为不存在。</p>

    <div class="row search">
      <input v-model.trim="query" inputmode="numeric" placeholder="视频 ID" @keydown.enter="lookup()" />
      <button class="primary" type="button" :disabled="loading" @click="lookup()">查询</button>
    </div>

    <div v-if="video" class="card detail">
      <div class="head">
        <img v-if="video.cover_url" :src="video.cover_url" alt="" />
        <div>
          <h2>{{ video.title }}</h2>
          <p class="subtle">
            #{{ video.id }} · {{ video.username }} · {{ AUDIT_STATUS_LABEL[video.audit_status] }}
            · 待处理举报 {{ video.pending_reports }}
          </p>
          <p class="subtle">{{ formatTime(video.created_at) }} · 赞 {{ video.likes_count }} · 评 {{ video.comment_count }}</p>
          <p v-if="video.description">{{ video.description }}</p>
          <div class="row">
            <button class="ghost" type="button" @click="router.push({ path: '/admin/users', query: { id: String(video.author_id) } })">
              查看作者
            </button>
            <a class="ghost" :href="`/video/${video.id}`" target="_blank" rel="noreferrer">打开前台页</a>
          </div>
        </div>
      </div>

      <video v-if="video.play_url" :src="video.play_url" :poster="video.cover_url" controls playsinline />

      <template v-if="video.audit_status !== 'rejected'">
        <label>下架说明</label>
        <textarea v-model="note" :maxlength="adminApi.ADMIN_NOTE_MAX" placeholder="写明依据，会记入审核流水" />
        <button class="danger" type="button" :disabled="takingDown" @click="takedown">确认下架</button>
      </template>
      <p v-else class="subtle">该内容已下架。如需补充依据，可再次查询后联系留存流水。</p>
    </div>
  </div>
</template>

<style scoped>
.search {
  margin: 16px 0;
}

.search input {
  max-width: 240px;
}

.detail {
  display: grid;
  gap: 14px;
}

.head {
  display: grid;
  grid-template-columns: 160px minmax(0, 1fr);
  gap: 14px;
}

.head img {
  width: 160px;
  height: 120px;
  object-fit: cover;
  border-radius: 12px;
  background: var(--fill);
}

.head h2 {
  margin: 0 0 6px;
  font-size: 18px;
}

video {
  width: 100%;
  max-height: 420px;
  border-radius: 12px;
  background: #000;
}

.ghost {
  display: inline-flex;
  align-items: center;
  text-decoration: none;
}

@media (max-width: 720px) {
  .head {
    grid-template-columns: 1fr;
  }

  .head img {
    width: 100%;
    height: 160px;
  }
}
</style>
