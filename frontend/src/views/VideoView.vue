<script setup lang="ts">
import { computed, onUnmounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRouter } from 'vue-router'

import { track } from '../analytics/track'
import AppShell from '../components/AppShell.vue'
import { AbortedError, ApiError } from '../api/client'
import * as videoApi from '../api/video'
import type { Video } from '../api/types'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

/** 发布流程的阶段，取代原先用自由文本描述进度的做法，也避免把后端术语暴露给用户。 */
type PublishPhase = 'idle' | 'uploading' | 'processing' | 'publishing'

const phase = ref<PublishPhase>('idle')
const uploadPercent = ref(0)
const fileError = ref('')
const published = ref<Video | null>(null)
const publishRequestKey = ref('')
const videoInput = ref<HTMLInputElement | null>(null)
const previewVideoUrl = ref('')
let abortController: AbortController | null = null

const publishForm = reactive({
  title: '',
  description: '',
  video: null as File | null,
})

const formatFileSize = videoApi.formatFileSize
const maxSizeText = videoApi.formatFileSize(videoApi.MAX_VIDEO_BYTES)

const busy = computed(() => phase.value !== 'idle')
const canSubmit = computed(
  () => !busy.value && publishForm.title.trim().length > 0 && !!publishForm.video && !fileError.value,
)

const phaseText = computed(() => {
  if (phase.value === 'uploading') return `上传中 ${uploadPercent.value}%`
  if (phase.value === 'processing') return '处理中'
  if (phase.value === 'publishing') return '发布中'
  return ''
})

// 只有上传阶段能拿到真实百分比，处理和发布阶段用不确定态动画表示仍在进行。
const indeterminate = computed(() => phase.value === 'processing' || phase.value === 'publishing')

function setPreviewVideo(file: File | null) {
  if (previewVideoUrl.value) URL.revokeObjectURL(previewVideoUrl.value)
  previewVideoUrl.value = file ? URL.createObjectURL(file) : ''
}

watch(
  () => publishForm.video,
  (file) => setPreviewVideo(file),
)

watch(
  () => [publishForm.title, publishForm.description, publishForm.video],
  () => {
    if (!busy.value) publishRequestKey.value = ''
  },
)

onUnmounted(() => {
  abortController?.abort()
  setPreviewVideo(null)
})

function pickVideo(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0] ?? null
  if (!file) {
    clearVideo()
    return
  }

  // 在发起任何网络请求之前校验，避免超限文件整份传完才被服务端拒绝。
  const message = videoApi.validateVideoFile(file)
  if (message) {
    publishForm.video = null
    fileError.value = message
    input.value = ''
    toast.error(message)
    return
  }

  fileError.value = ''
  publishForm.video = file
}

function clearVideo() {
  publishForm.video = null
  fileError.value = ''
  if (videoInput.value) videoInput.value.value = ''
}

function cancel() {
  abortController?.abort()
}

function awaitingReview(video: Video | null) {
  return video?.audit_status === 'pending' || video?.audit_status === 'reviewing'
}

async function onPublish() {
  if (busy.value) return
  if (!auth.isLoggedIn) {
    toast.error('请先登录')
    await router.push('/account')
    return
  }

  const title = publishForm.title.trim()
  const description = publishForm.description.trim()
  const file = publishForm.video
  if (!title) {
    toast.error('请输入标题')
    return
  }
  if (!file) {
    toast.error('请选择视频文件')
    return
  }

  const controller = new AbortController()
  abortController = controller
  published.value = null
  uploadPercent.value = 0
  phase.value = 'uploading'

  try {
    const task = await videoApi.uploadVideo(file, {
      onProgress: (percent) => {
        uploadPercent.value = percent
      },
      signal: controller.signal,
    })

    phase.value = 'processing'
    const isReady = task.status === 'ready' && !!task.play_url && !!task.cover_url
    const readyTask = isReady ? task : await videoApi.waitForVideoUpload(task.id, { signal: controller.signal })

    phase.value = 'publishing'
    if (!publishRequestKey.value) publishRequestKey.value = videoApi.createIdempotencyKey()
    // 封面由后端转码时自动取视频首帧生成，用户侧不再有封面这个概念。
    const res = await videoApi.publishVideo(
      {
        title,
        description,
        play_url: readyTask.play_url!,
        cover_url: readyTask.cover_url!,
      },
      { idempotencyKey: publishRequestKey.value },
    )

    published.value = res
    track('video_publish', { video_id: res.id })
    publishRequestKey.value = ''
    publishForm.title = ''
    publishForm.description = ''
    clearVideo()
    // 审核开启时发布为待审，关闭时发布即公开。按返回状态提示，避免用户误以为失败。
    toast.success(awaitingReview(res) ? '已提交，审核通过后将出现在信息流中' : '发布成功')
  } catch (error) {
    // 用户主动取消属于正常操作，不当作失败处理。
    if (error instanceof AbortedError) {
      toast.info('已取消')
    } else {
      toast.error(error instanceof ApiError ? error.message : String(error))
    }
  } finally {
    abortController = null
    phase.value = 'idle'
    uploadPercent.value = 0
  }
}
</script>

<template>
  <AppShell>
    <div class="publish-wrap">
      <div class="card publish-card">
        <p class="title" style="margin: 0">发布视频</p>

        <div class="grid form-grid" style="margin-top: 16px">
          <div>
            <label>标题</label>
            <input v-model.trim="publishForm.title" class="big-input" :disabled="busy" placeholder="给视频起个标题" />
          </div>

          <div>
            <label>描述</label>
            <textarea v-model.trim="publishForm.description" class="big-input" :disabled="busy" placeholder="选填" />
          </div>

          <div>
            <label>视频文件</label>
            <input
              ref="videoInput"
              class="file-native"
              type="file"
              accept="video/*"
              :disabled="busy"
              @change="pickVideo"
            />
            <div class="file-box">
              <button type="button" :disabled="busy" @click="videoInput?.click()">选择视频</button>
              <div class="file-name" :class="publishForm.video ? '' : 'muted'">
                {{ publishForm.video ? publishForm.video.name : '未选择文件' }}
              </div>
              <button v-if="publishForm.video" type="button" :disabled="busy" @click="clearVideo">清除</button>
            </div>
            <div v-if="fileError" class="file-tip bad">{{ fileError }}</div>
            <div v-else-if="publishForm.video" class="file-tip">
              {{ formatFileSize(publishForm.video.size) }}，上限 {{ maxSizeText }}
            </div>
          </div>

          <div v-if="previewVideoUrl" class="preview-card">
            <video class="video" :src="previewVideoUrl" controls playsinline preload="metadata" />
          </div>

          <div v-if="busy" class="progress">
            <div class="progress-head">
              <span>{{ phaseText }}</span>
              <button class="cancel-btn" type="button" @click="cancel">取消</button>
            </div>
            <div class="progress-track">
              <div
                class="progress-bar"
                :class="{ indeterminate }"
                :style="indeterminate ? undefined : { width: `${uploadPercent}%` }"
              />
            </div>
          </div>

          <div class="row" style="justify-content: flex-end; margin-top: 8px">
            <button class="primary big-btn" type="button" :disabled="!canSubmit" @click="onPublish">发布</button>
          </div>
        </div>

        <div v-if="published" class="card" style="margin-top: 14px">
          <div class="row" style="justify-content: space-between; align-items: center">
            <div>
              <div class="title" style="margin: 0">{{ published.title }}</div>
              <div class="audit-tip">
                {{
                  awaitingReview(published)
                    ? '审核通过后才会出现在信息流中，你可以先在「我的」里查看'
                    : '已出现在信息流中'
                }}
              </div>
            </div>
            <RouterLink class="pill" :to="`/video/${published.id}`">去播放</RouterLink>
          </div>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.publish-wrap {
  display: grid;
  justify-items: center;
}

.publish-card {
  width: min(980px, 100%);
  padding: 22px;
}

.form-grid {
  gap: 16px;
}

.file-native {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.file-box {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid rgba(var(--fg), 0.12);
  background: rgba(var(--fg), 0.06);
  border-radius: 14px;
  min-height: 46px;
}

.file-box button {
  padding: 8px 10px;
  border-radius: 12px;
}

.file-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 13px;
  color: rgba(var(--fg), 0.88);
}

.muted {
  color: rgba(var(--fg), 0.55);
}

.file-tip {
  margin-top: 6px;
  font-size: 12px;
  color: rgba(var(--fg), 0.6);
}

.file-tip.bad {
  color: rgba(254, 44, 85, 0.92);
}

.audit-tip {
  margin-top: 4px;
  font-size: 12px;
  color: rgba(var(--fg), 0.6);
}

.big-input {
  box-sizing: border-box;
  width: 100%;
  max-width: 100%;
  padding: 12px 14px;
  font-size: 14px;
  border-radius: 14px;
}

.big-btn {
  padding: 12px 18px;
  font-size: 14px;
  border-radius: 14px;
}

.preview-card {
  border: 1px solid rgba(var(--fg), 0.12);
  background: rgba(var(--fg), 0.05);
  border-radius: 16px;
  padding: 12px;
}

.video {
  width: 100%;
  max-height: 420px;
  aspect-ratio: 16/9;
  object-fit: contain;
  border-radius: 14px;
  border: 1px solid rgba(var(--fg), 0.1);
  background: rgba(0, 0, 0, 0.35);
}

.progress {
  display: grid;
  gap: 8px;
}

.progress-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
  color: rgba(var(--fg), 0.86);
}

.cancel-btn {
  padding: 6px 12px;
  border-radius: 12px;
  font-size: 12px;
  min-height: 0;
}

.progress-track {
  height: 6px;
  border-radius: 999px;
  background: rgba(var(--fg), 0.1);
  overflow: hidden;
}

.progress-bar {
  height: 100%;
  border-radius: 999px;
  background: linear-gradient(90deg, rgba(37, 244, 238, 0.9), rgba(254, 44, 85, 0.9));
  transition: width 0.2s ease;
}

/* 处理和发布阶段拿不到进度，用往复动画表示仍在进行。 */
.progress-bar.indeterminate {
  width: 40%;
  animation: progress-slide 1.1s ease-in-out infinite;
}

@keyframes progress-slide {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(250%);
  }
}

@media (prefers-reduced-motion: reduce) {
  .progress-bar.indeterminate {
    width: 100%;
    animation: none;
  }
}
</style>
