<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as opsApi from '../api/ops'
import type { OpsLogLine, OpsMetrics } from '../api/ops'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const tab = ref<'monitor' | 'logs'>('monitor')
const grafanaPath = ref('/grafana/d/feed-overview?kiosk=1&theme=dark')
const metrics = ref<OpsMetrics | null>(null)
const query = ref('{service="backend"}')
const sinceMinutes = ref(60)
const lines = ref<OpsLogLine[]>([])
const loadingLogs = ref(false)
const ready = ref(false)

onMounted(async () => {
  if (!auth.isLoggedIn) {
    await router.replace('/account')
    return
  }
  try {
    const access = await opsApi.opsAccess()
    if (!access.allowed) {
      toast.error('仅测试邮箱可查看运维信息')
      await router.replace('/account')
      return
    }
    grafanaPath.value = access.grafana_path || grafanaPath.value
    ready.value = true
    void loadMetrics()
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
    await router.replace('/account')
  }
})

async function loadMetrics() {
  try {
    metrics.value = await opsApi.opsMetrics()
  } catch {
    metrics.value = null
  }
}

async function searchLogs() {
  if (loadingLogs.value) return
  loadingLogs.value = true
  try {
    const res = await opsApi.opsLogs(query.value.trim(), sinceMinutes.value)
    lines.value = res.lines
    if (res.lines.length === 0) toast.info('这段时间没有匹配的日志')
  } catch (e) {
    const msg = e instanceof ApiError ? e.message : String(e)
    toast.error(msg)
  } finally {
    loadingLogs.value = false
  }
}

function formatRate(value?: number | null) {
  if (value == null || Number.isNaN(value)) return '—'
  return value.toFixed(2)
}
</script>

<template>
  <AppShell>
    <div v-if="ready" class="card">
      <div class="row" style="justify-content: space-between; align-items: center">
        <p class="title" style="margin: 0">运维</p>
        <div class="row" style="gap: 8px">
          <button class="ghost" type="button" :class="{ active: tab === 'monitor' }" @click="tab = 'monitor'">监控</button>
          <button class="ghost" type="button" :class="{ active: tab === 'logs' }" @click="tab = 'logs'">日志</button>
        </div>
      </div>
      <p class="subtle" style="margin-top: 8px">仅数字@lmr.com 测试账号可见。仪表盘为只读。</p>

      <template v-if="tab === 'monitor'">
        <div class="row" style="margin-top: 12px; gap: 10px">
          <div class="metric static">
            <div class="metric-num">{{ formatRate(metrics?.qps) }}</div>
            <div class="metric-label">QPS</div>
          </div>
          <div class="metric static">
            <div class="metric-num">{{ formatRate(metrics?.error_rate) }}</div>
            <div class="metric-label">5xx 占比</div>
          </div>
        </div>
        <iframe class="grafana" :src="grafanaPath" title="Grafana 监控"></iframe>
      </template>

      <template v-else>
        <div class="grid" style="margin-top: 12px">
          <div>
            <label>LogQL</label>
            <input v-model.trim="query" @keydown.enter="searchLogs" />
          </div>
          <div class="row" style="gap: 8px">
            <button class="ghost" type="button" :class="{ active: sinceMinutes === 15 }" @click="sinceMinutes = 15">15 分钟</button>
            <button class="ghost" type="button" :class="{ active: sinceMinutes === 60 }" @click="sinceMinutes = 60">1 小时</button>
            <button class="ghost" type="button" :class="{ active: sinceMinutes === 360 }" @click="sinceMinutes = 360">6 小时</button>
            <button class="primary" type="button" :disabled="loadingLogs" @click="searchLogs">查询</button>
          </div>
        </div>
        <div class="log-box">
          <div v-if="loadingLogs" class="subtle">查询中…</div>
          <div v-else-if="lines.length === 0" class="subtle">还没有结果</div>
          <div v-for="(item, index) in lines" :key="index" class="log-line">
            <span class="log-time">{{ item.time }}</span>
            <span v-if="item.labels" class="log-labels">{{ item.labels }}</span>
            <span class="log-text">{{ item.line }}</span>
          </div>
        </div>
      </template>
    </div>
  </AppShell>
</template>

<style scoped>
.ghost {
  border: 1px solid rgba(255, 255, 255, 0.14);
  background: rgba(0, 0, 0, 0.18);
  color: rgba(255, 255, 255, 0.86);
  border-radius: 12px;
  padding: 10px 12px;
  cursor: pointer;
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
}

.metric-num {
  font-size: 18px;
  font-weight: 900;
}

.metric-label {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.65);
}

.grafana {
  width: 100%;
  height: min(72vh, 760px);
  margin-top: 12px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 12px;
  background: #111;
}

.log-box {
  margin-top: 12px;
  max-height: min(64vh, 640px);
  overflow: auto;
  padding: 10px 12px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.35);
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
  line-height: 1.55;
}

.log-line {
  display: grid;
  grid-template-columns: 64px auto 1fr;
  gap: 8px;
  padding: 3px 0;
}

.log-time {
  color: rgba(255, 255, 255, 0.5);
}

.log-labels {
  color: #25f4ee;
  white-space: nowrap;
}

.log-text {
  color: rgba(255, 255, 255, 0.88);
  overflow-wrap: anywhere;
}
</style>
