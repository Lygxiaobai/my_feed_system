<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import { track } from '../analytics/track'
import { ApiError } from '../api/client'
import { REPORT_DETAIL_MAX, REPORT_REASONS, reportVideo, type ReportReason } from '../api/report'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const open = ref(false)
const busy = ref(false)
const videoId = ref(0)
const reason = ref<ReportReason | ''>('')
const detail = ref('')

// 「其他」没有预置语义，不写明理由的话审核员无从判断。
const detailRequired = computed(() => reason.value === 'other')
const canSubmit = computed(() => {
  if (!reason.value || busy.value) return false
  if (detailRequired.value && detail.value.trim() === '') return false
  return detail.value.length <= REPORT_DETAIL_MAX
})

async function openFor(id: number) {
  if (!auth.isLoggedIn) {
    toast.error('请先登录')
    await router.push('/account')
    return
  }
  videoId.value = id
  reason.value = ''
  detail.value = ''
  open.value = true
}

function close() {
  open.value = false
}

async function submit() {
  if (!canSubmit.value || !reason.value) return
  busy.value = true
  try {
    await reportVideo({ video_id: videoId.value, reason: reason.value, detail: detail.value.trim() })
    track('report_submit', { video_id: videoId.value, reason: reason.value })
    // 明确告知「已受理、会人工核实」：举报不会立即产生可见变化，
    // 不给回执的话用户会以为没提交成功而反复点击。
    toast.success('举报已提交，我们会尽快人工核实')
    close()
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    busy.value = false
  }
}

defineExpose({ openFor, close })
</script>

<template>
  <Teleport to="body">
    <div v-if="open" class="mask" @click.self="close">
      <div class="sheet" role="dialog" aria-modal="true" aria-label="举报">
        <div class="head">
          <div>
            <p class="title">举报</p>
            <p class="subtle">请选择最贴近的理由，我们会人工核实后处理</p>
          </div>
          <button class="x" type="button" aria-label="关闭" @click="close">×</button>
        </div>

        <div class="reasons">
          <button
            v-for="item in REPORT_REASONS"
            :key="item.value"
            class="reason"
            type="button"
            :class="{ on: reason === item.value }"
            :disabled="busy"
            @click="reason = item.value"
          >
            {{ item.label }}
          </button>
        </div>

        <label class="lab" for="report-detail">
          补充说明{{ detailRequired ? '（必填）' : '（选填）' }}
        </label>
        <textarea
          id="report-detail"
          v-model="detail"
          class="detail"
          rows="3"
          :maxlength="REPORT_DETAIL_MAX"
          :disabled="busy"
          :placeholder="detailRequired ? '请具体说明问题所在' : '可补充具体时间点或细节，便于核实'"
        />
        <p class="counter">{{ detail.length }} / {{ REPORT_DETAIL_MAX }}</p>

        <button class="primary wide" type="button" :disabled="!canSubmit" @click="submit">
          {{ busy ? '提交中…' : '提交举报' }}
        </button>
        <p class="subtle">提交后内容不会立即下线，我们会先人工核实再决定处理方式。</p>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.mask {
  position: fixed;
  inset: 0;
  z-index: 140;
  display: grid;
  place-items: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.58);
  backdrop-filter: blur(10px);
}

.sheet {
  width: min(420px, 100%);
  max-height: min(80vh, 720px);
  overflow: auto;
  padding: 18px;
  border-radius: 20px;
  border: 1px solid rgba(var(--fg), 0.12);
  background: var(--surface);
  display: grid;
  gap: 10px;
}

.head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 12px;
}

.title {
  margin: 0;
  font-size: 18px;
  font-weight: 800;
}

.subtle {
  margin: 4px 0 0;
  color: rgba(var(--fg), 0.62);
  font-size: 13px;
}

.x {
  width: 34px;
  height: 34px;
  border-radius: 12px;
  border: 1px solid rgba(var(--fg), 0.14);
  background: rgba(var(--fg), 0.06);
  color: rgba(var(--fg), 0.9);
  cursor: pointer;
  font-size: 20px;
  line-height: 1;
}

.reasons {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
}

.reason {
  padding: 10px 12px;
  border-radius: 12px;
  border: 1px solid rgba(var(--fg), 0.12);
  background: rgba(var(--fg), 0.05);
  color: inherit;
  cursor: pointer;
  font-size: 14px;
}

.reason.on {
  border-color: rgba(254, 44, 85, 0.7);
  background: rgba(254, 44, 85, 0.16);
}

.lab {
  font-size: 13px;
  color: rgba(var(--fg), 0.7);
}

.detail {
  width: 100%;
  resize: vertical;
}

.counter {
  margin: 0;
  text-align: right;
  font-size: 12px;
  color: rgba(var(--fg), 0.45);
}

.wide {
  width: 100%;
}
</style>
