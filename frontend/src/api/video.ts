import { AbortedError, postFormWithProgress, postJson, resolveAssetUrl } from './client'
import type { BackendVideoEnvelope, BackendVideosEnvelope, Video } from './types'

/** 与后端 media.defaultMaxVideoBytes（256 MiB）保持一致，超出前端直接拒绝。 */
export const MAX_VIDEO_BYTES = 256 * 1024 * 1024

export function formatFileSize(bytes: number) {
  const mb = bytes / 1024 / 1024
  if (mb >= 1) return `${mb.toFixed(1)} MB`
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}

/** 选择文件时立即校验，避免把注定被服务端拒绝的文件整份上传完才失败。 */
export function validateVideoFile(file: File): string | null {
  if (file.size <= 0) return '文件内容为空，请重新选择'
  // 部分系统给出的 type 为空，此时交由后端与 accept 属性兜底。
  if (file.type && !file.type.startsWith('video/')) return '只能上传视频文件'
  if (file.size > MAX_VIDEO_BYTES) {
    return `视频不能超过 ${formatFileSize(MAX_VIDEO_BYTES)}，当前 ${formatFileSize(file.size)}`
  }
  return null
}

/**
 * 生成发布请求的幂等键。
 * crypto.randomUUID 只在安全上下文（HTTPS / localhost）下存在，本站以 HTTP 提供服务，
 * 直接调用会抛 TypeError 导致发布失败，因此必须降级到 getRandomValues 手工拼 v4 UUID。
 */
export function createIdempotencyKey() {
  const cryptoApi = globalThis.crypto as Crypto | undefined

  if (typeof cryptoApi?.randomUUID === 'function') {
    return cryptoApi.randomUUID()
  }

  if (typeof cryptoApi?.getRandomValues === 'function') {
    const bytes = new Uint8Array(16)
    cryptoApi.getRandomValues(bytes)
    // 按 RFC 4122 置版本号 4 与变体位。
    bytes[6] = ((bytes[6] ?? 0) & 0x0f) | 0x40
    bytes[8] = ((bytes[8] ?? 0) & 0x3f) | 0x80
    const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('')
    return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
  }

  return `${Date.now().toString(16)}-${Math.random().toString(16).slice(2, 10)}`
}

function normalizeVideo(video: Video): Video {
  return {
    ...video,
    play_url: resolveAssetUrl(video.play_url),
    cover_url: resolveAssetUrl(video.cover_url),
    comment_count: video.comment_count ?? 0,
  }
}

export async function publishVideo(
  input: { title: string; description: string; play_url: string; cover_url: string },
  options?: { idempotencyKey?: string },
) {
  const res = await postJson<BackendVideoEnvelope>('/video/publish', input, {
    authRequired: true,
    headers: options?.idempotencyKey ? { 'Idempotency-Key': options.idempotencyKey } : undefined,
  })
  return normalizeVideo(res.video)
}

export type MediaTaskStatus = 'processing' | 'ready' | 'failed'

export type VideoUploadTask = {
  id: number
  status: MediaTaskStatus
  play_url?: string
  cover_url?: string
  content_type: string
  error_message?: string
  created_at: string
  updated_at: string
}

type UploadVideoResponse = {
  task: VideoUploadTask
}

function normalizeUploadTask(task: VideoUploadTask): VideoUploadTask {
  return {
    ...task,
    play_url: task.play_url ? resolveAssetUrl(task.play_url) : undefined,
    cover_url: task.cover_url ? resolveAssetUrl(task.cover_url) : undefined,
  }
}

export async function uploadVideo(
  file: File,
  options?: { onProgress?: (percent: number) => void; signal?: AbortSignal },
) {
  const fd = new FormData()
  fd.append('file', file)
  const res = await postFormWithProgress<UploadVideoResponse>('/video/uploadVideo', fd, {
    authRequired: true,
    onProgress: options?.onProgress,
    signal: options?.signal,
  })
  return normalizeUploadTask(res.task)
}

export function getVideoUploadTask(taskId: number) {
  return postJson<{ task: VideoUploadTask }>(
    '/video/mediaTaskStatus',
    { task_id: taskId },
    { authRequired: true },
  ).then((res) => normalizeUploadTask(res.task))
}

/** 可被 signal 中断的等待，取消时立刻结束轮询而不是空转到下一次 tick。 */
function waitFor(ms: number, signal?: AbortSignal) {
  return new Promise<void>((resolve, reject) => {
    if (signal?.aborted) {
      reject(new AbortedError())
      return
    }
    let timer = 0
    const onAbort = () => {
      window.clearTimeout(timer)
      reject(new AbortedError())
    }
    timer = window.setTimeout(() => {
      signal?.removeEventListener('abort', onAbort)
      resolve()
    }, ms)
    signal?.addEventListener('abort', onAbort, { once: true })
  })
}

export async function waitForVideoUpload(
  taskId: number,
  options?: { intervalMs?: number; maxAttempts?: number; signal?: AbortSignal },
) {
  const intervalMs = options?.intervalMs ?? 1000
  const maxAttempts = options?.maxAttempts ?? 180
  const signal = options?.signal

  for (let attempt = 0; attempt < maxAttempts; attempt += 1) {
    if (signal?.aborted) throw new AbortedError()

    const task = await getVideoUploadTask(taskId)
    if (task.status === 'ready' && task.play_url && task.cover_url) return task
    if (task.status === 'failed') {
      throw new Error(task.error_message || '视频处理失败，请重新上传')
    }
    await waitFor(intervalMs, signal)
  }

  throw new Error('视频处理超时，请稍后重试')
}

export async function listByAuthorId(authorId: number) {
  const res = await postJson<BackendVideosEnvelope>('/video/listByAuthorID', { author_id: authorId })
  return res.videos.map(normalizeVideo)
}

export async function listLiked() {
  const res = await postJson<BackendVideosEnvelope>('/video/listLiked', {}, { authRequired: true })
  return res.videos.map(normalizeVideo)
}

export async function getDetail(id: number) {
  const res = await postJson<BackendVideoEnvelope>('/video/getDetail', { id })
  return normalizeVideo(res.video)
}
