<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { ApiError } from '../../api/client'
import * as opsApi from '../../api/ops'
import type { OpsLogLine, OpsMetrics } from '../../api/ops'
import { useThemeStore } from '../../stores/theme'
import { useToastStore } from '../../stores/toast'
import type { ResolvedTheme } from '../../theme'

const toast = useToastStore()
const theme = useThemeStore()

const tab = ref<'monitor' | 'logs'>('monitor')
const grafanaPath = ref('/grafana/d/feed-overview?kiosk=1&theme=dark')
const metrics = ref<OpsMetrics | null>(null)
const query = ref('{service="backend"}')
const sinceMinutes = ref(60)
const lines = ref<OpsLogLine[]>([])
const loadingLogs = ref(false)
const ready = ref(false)

onMounted(async () => {
  // 必须打一次 /ops/access：Grafana iframe 带不上 localStorage 里的 JWT，
  // 要靠这里种下的 HttpOnly cookie 过 nginx auth_request。
  try {
    const access = await opsApi.opsAccess()
    if (!access.allowed) {
      toast.error('没有查看运维信息的权限')
      return
    }
    grafanaPath.value = access.grafana_path || grafanaPath.value
    ready.value = true
    void loadMetrics()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : '无法打开运维')
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
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    loadingLogs.value = false
  }
}

function formatRate(value?: number | null) {
  if (value == null || Number.isNaN(value)) return '—'
  return value.toFixed(2)
}

function withGrafanaTheme(path: string, mode: ResolvedTheme) {
  try {
    const url = new URL(path, 'http://local.invalid')
    url.searchParams.set('theme', mode)
    return `${url.pathname}${url.search}${url.hash}`
  } catch {
    return path
  }
}

const grafanaSrc = computed(() => withGrafanaTheme(grafanaPath.value, theme.resolved))
</script>

<template>
  <div>
    <div class="row" style="justify-content: space-between; align-items: center">
      <div>
        <p class="title">运维</p>
        <p class="subtle">只读。Grafana 密码不会交给浏览器，仪表盘走站点登录门禁。</p>
      </div>
      <div class="row" style="gap: 8px">
        <button class="ghost" type="button" :class="{ active: tab === 'monitor' }" @click="tab = 'monitor'">监控</button>
        <button class="ghost" type="button" :class="{ active: tab === 'logs' }" @click="tab = 'logs'">日志</button>
      </div>
    </div>

    <template v-if="ready && tab === 'monitor'">
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
      <iframe class="grafana" :src="grafanaSrc" title="Grafana 监控"></iframe>
    </template>

    <template v-else-if="ready">
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
</template>

<style scoped>
.ghost {
  border: 1px solid rgba(var(--fg), 0.14);
  background: var(--fill);
  color: rgba(var(--fg), 0.86);
  border-radius: 12px;
  padding: 10px 12px;
  cursor: pointer;
}

.ghost.active {
  background: rgba(254, 44, 85, 0.14);
  border-color: rgba(254, 44, 85, 0.55);
}

.metric {
  border: 1px solid rgba(var(--fg), 0.12);
  background: rgba(var(--fg), 0.06);
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
  color: rgba(var(--fg), 0.65);
}

.grafana {
  width: 100%;
  height: min(85vh, 1200px);
  margin-top: 12px;
  border: 1px solid rgba(var(--fg), 0.12);
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
  color: rgba(var(--fg), 0.5);
}

.log-labels {
  color: #25f4ee;
  white-space: nowrap;
}

.log-text {
  color: rgba(var(--fg), 0.88);
  overflow-wrap: anywhere;
}
</style>
