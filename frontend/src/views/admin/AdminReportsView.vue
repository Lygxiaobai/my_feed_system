<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '../../api/client'
import * as adminApi from '../../api/admin'
import type { AdminVideo } from '../../api/admin'
import * as reportApi from '../../api/report'
import type { PendingReportItem, ReportAction } from '../../api/report'
import { reasonLabel } from '../../api/report'
import { useToastStore } from '../../stores/toast'

const emit = defineEmits<{ changed: [] }>()
const router = useRouter()
const toast = useToastStore()

const loading = ref(false)
const items = ref<PendingReportItem[]>([])
const previews = ref<Record<number, AdminVideo>>({})
const notes = ref<Record<number, string>>({})
const busyId = ref(0)

function formatTime(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString('zh-CN')
}

function reasonText(item: PendingReportItem) {
  return Object.entries(item.reason_counts || {})
    .filter(([, count]) => (count ?? 0) > 0)
    .map(([reason, count]) => `${reasonLabel(reason as reportApi.ReportReason)} ${count}`)
    .join(' · ')
}

async function load() {
  if (loading.value) return
  loading.value = true
  try {
    items.value = await reportApi.listPendingReports(50)
    for (const item of items.value) {
      if (previews.value[item.target_id]) continue
      try {
        previews.value[item.target_id] = await adminApi.lookupAdminVideo(item.target_id)
      } catch {
        // 内容可能已被删，队列项仍要能处置。
      }
    }
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '加载队列失败')
  } finally {
    loading.value = false
  }
}

async function decide(item: PendingReportItem, action: ReportAction) {
  if (busyId.value) return
  const note = (notes.value[item.target_id] || '').trim()
  if (action === 'takedown' && !note) {
    toast.error('下架必须填写处置说明')
    return
  }
  const verb = action === 'takedown' ? '下架该内容' : '驳回这些举报'
  if (!window.confirm(`确认${verb}？`)) return

  busyId.value = item.target_id
  try {
    await reportApi.handleReport({ video_id: item.target_id, action, note })
    toast.success(action === 'takedown' ? '已下架' : '已驳回')
    notes.value[item.target_id] = ''
    emit('changed')
    await load()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '处置失败')
  } finally {
    busyId.value = 0
  }
}

onMounted(() => {
  void load()
})
</script>

<template>
  <div>
    <div class="row" style="justify-content: space-between">
      <div>
        <p class="title">举报队列</p>
        <p class="subtle">按内容聚合，一次结论覆盖该作品上所有待处理举报。举报本身不会改变可见性。</p>
      </div>
      <button class="ghost" type="button" :disabled="loading" @click="load">刷新</button>
    </div>

    <div v-if="loading && items.length === 0" class="subtle" style="margin-top: 16px">加载中…</div>
    <div v-else-if="items.length === 0" class="card" style="margin-top: 16px">
      <p class="subtle" style="margin: 0">当前没有待处理举报。</p>
    </div>

    <article v-for="item in items" :key="item.target_id" class="card case">
      <div class="preview">
        <img v-if="previews[item.target_id]?.cover_url" :src="previews[item.target_id]?.cover_url" alt="" />
        <div class="meta">
          <h2>{{ previews[item.target_id]?.title || `视频 #${item.target_id}` }}</h2>
          <p class="subtle">
            #{{ item.target_id }} · {{ previews[item.target_id]?.username || '未知作者' }}
            · {{ item.report_count }} 条举报
          </p>
          <p>{{ reasonText(item) }}</p>
          <p class="subtle">首次 {{ formatTime(item.firstly_at) }} · 最近 {{ formatTime(item.latest_at) }}</p>
          <div v-if="item.samples?.length" class="samples">
            <p v-for="(sample, index) in item.samples" :key="index">{{ sample }}</p>
          </div>
          <button class="ghost" type="button" @click="router.push({ path: '/admin/videos', query: { id: String(item.target_id) } })">
            查看详情
          </button>
        </div>
      </div>
      <label>处置说明</label>
      <textarea
        :value="notes[item.target_id] || ''"
        :maxlength="adminApi.ADMIN_NOTE_MAX"
        placeholder="下架必填；驳回可选"
        @input="notes[item.target_id] = ($event.target as HTMLTextAreaElement).value"
      />
      <div class="row">
        <button type="button" :disabled="busyId === item.target_id" @click="decide(item, 'dismiss')">驳回</button>
        <button class="danger" type="button" :disabled="busyId === item.target_id" @click="decide(item, 'takedown')">
          下架
        </button>
      </div>
    </article>
  </div>
</template>

<style scoped>
.case {
  margin-top: 14px;
}

.preview {
  display: grid;
  grid-template-columns: 160px minmax(0, 1fr);
  gap: 14px;
  margin-bottom: 12px;
}

.preview img {
  width: 160px;
  height: 120px;
  object-fit: cover;
  border-radius: 12px;
  background: var(--fill);
}

.meta h2 {
  margin: 0 0 4px;
  font-size: 16px;
}

.samples {
  margin: 8px 0;
  padding: 8px 10px;
  border-radius: 10px;
  background: var(--fill);
  font-size: 13px;
}

.samples p {
  margin: 0;
}

.samples p + p {
  margin-top: 6px;
}

.ghost {
  margin-top: 8px;
}

@media (max-width: 720px) {
  .preview {
    grid-template-columns: 1fr;
  }

  .preview img {
    width: 100%;
    height: 160px;
  }
}
</style>
