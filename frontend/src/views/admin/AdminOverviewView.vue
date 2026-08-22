<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { ApiError } from '../../api/client'
import * as adminApi from '../../api/admin'
import type { AdminOverview } from '../../api/admin'
import { useToastStore } from '../../stores/toast'

const router = useRouter()
const toast = useToastStore()
const loading = ref(true)
const overview = ref<AdminOverview | null>(null)

onMounted(async () => {
  try {
    overview.value = await adminApi.adminOverview()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '加载概览失败')
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <div>
    <p class="title">概览</p>
    <p class="subtle">只有审核员白名单能进。内容处置和只读观测都在这里，测试邮箱进不来。</p>

    <div v-if="loading" class="subtle" style="margin-top: 16px">加载中…</div>
    <div v-else class="cards">
      <button class="card metric" type="button" @click="router.push('/admin/reports')">
        <div class="metric-num">{{ overview?.pending_reports ?? 0 }}</div>
        <div class="metric-label">待处理举报</div>
      </button>
      <button class="card metric" type="button" @click="router.push('/admin/ops')">
        <div class="metric-num">运维</div>
        <div class="metric-label">监控与日志</div>
      </button>
      <div class="card metric static">
        <div class="metric-num">{{ overview?.username || '—' }}</div>
        <div class="metric-label">当前审核员 · #{{ overview?.account_id }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cards {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.metric {
  text-align: left;
  min-height: 112px;
}

.metric.static {
  cursor: default;
}

.metric-num {
  font-size: 28px;
  font-weight: 800;
  word-break: break-all;
}

.metric-label {
  margin-top: 6px;
  color: var(--muted);
  font-size: 13px;
}

@media (max-width: 720px) {
  .cards {
    grid-template-columns: 1fr;
  }
}
</style>
