import { useAuthStore } from '../stores/auth'

export class ApiError extends Error {
  status: number
  /** 后端返回的五位业务错误码，例如 A0230（登录已过期）。 */
  code: string
  /** 本次请求的唯一标识，与后端日志中的 request_id 一致，用于报障时溯源。 */
  requestId: string
  payload?: unknown

  constructor(
    message: string,
    status: number,
    options?: { code?: string; requestId?: string; payload?: unknown },
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = options?.code ?? ''
    this.requestId = options?.requestId ?? ''
    this.payload = options?.payload
  }
}

/** 用户主动取消导致的中断，调用方据此静默复位，不当作失败提示。 */
export class AbortedError extends Error {
  constructor() {
    super('已取消')
    this.name = 'AbortedError'
  }
}

/**
 * 后端统一响应结构。
 * code 为 "00000" 表示成功，其余为五位业务错误码；
 * message 是可直接展示给用户的提示（后端已剥离内部错误细节）；
 * data 是真正的业务数据；requestId 用于把前端报错对应到后端日志。
 */
type Envelope = {
  code?: string
  message?: string
  data?: unknown
  requestId?: string
}

/** 成功错误码，与后端 response.Success 保持一致。 */
const SUCCESS_CODE = '00000'

const API_BASE = (import.meta.env.VITE_API_BASE as string | undefined) ?? '/api'

function getDefaultErrorMessage(status: number) {
  return `请求失败 (${status})`
}

function getMissingTokenMessage() {
  return '请先登录'
}

/**
 * 统一拆信封。
 * 放在这一层处理，是为了让上层 api 模块与 types.ts 保持原样——
 * data 的内部形状（{video}、{videos}、{comments} 等）没有变化。
 */
function unwrap<T>(raw: unknown, status: number): T {
  const envelope = raw && typeof raw === 'object' ? (raw as Envelope) : undefined

  // 网关或反向代理返回的非 JSON 错误页不带 code，按 HTTP 状态处理。
  if (!envelope || typeof envelope.code !== 'string') {
    if (status < 200 || status >= 300) {
      throw new ApiError(getDefaultErrorMessage(status), status)
    }
    return raw as T
  }

  if (envelope.code !== SUCCESS_CODE) {
    throw new ApiError(envelope.message || getDefaultErrorMessage(status), status, {
      code: envelope.code,
      requestId: envelope.requestId,
      payload: envelope.data,
    })
  }

  return envelope.data as T
}

function apiOrigin() {
  return new URL(API_BASE, window.location.origin).origin
}

export function resolveAssetUrl(url?: string) {
  if (!url) return ''
  if (/^https?:\/\//i.test(url)) return url
  return new URL(url, apiOrigin()).toString()
}

export async function postJson<T>(
  path: string,
  body: unknown,
  options?: { authRequired?: boolean; headers?: Record<string, string> },
): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    throw new ApiError(getMissingTokenMessage(), 401)
  }

  const headers: Record<string, string> = { 'Content-Type': 'application/json', ...(options?.headers ?? {}) }
  if (token) headers.Authorization = `Bearer ${token}`

  const res = await fetch(`${API_BASE}${path}`, {
    method: 'POST',
    headers,
    body: JSON.stringify(body ?? {}),
  })

  const data = parseResponseBody(await res.text())

  // 401 无论出现在信封内还是网关层，都要清掉本地 token。
  if (res.status === 401) {
    auth.clearToken()
  }

  return unwrap<T>(data, res.status)
}

function parseResponseBody(text: string) {
  if (!text) return null
  try {
    return JSON.parse(text) as unknown
  } catch {
    return text as unknown
  }
}

/**
 * 带上传进度的表单提交。
 * 这里用 XMLHttpRequest 而不是 fetch，是因为 fetch 无法上报请求体的发送进度，
 * 而大体积视频上传必须给用户真实的百分比反馈。
 */
export function postFormWithProgress<T>(
  path: string,
  body: FormData,
  options?: { authRequired?: boolean; onProgress?: (percent: number) => void; signal?: AbortSignal },
): Promise<T> {
  const auth = useAuthStore()
  const token = auth.token

  if (options?.authRequired && !token) {
    return Promise.reject(new ApiError(getMissingTokenMessage(), 401))
  }
  if (options?.signal?.aborted) {
    return Promise.reject(new AbortedError())
  }

  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}${path}`)
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)

    const onProgress = options?.onProgress
    if (onProgress) {
      xhr.upload.onprogress = (event) => {
        if (!event.lengthComputable || event.total <= 0) return
        onProgress(Math.min(100, Math.round((event.loaded / event.total) * 100)))
      }
    }

    const signal = options?.signal
    const abort = () => xhr.abort()
    signal?.addEventListener('abort', abort)
    const cleanup = () => signal?.removeEventListener('abort', abort)

    xhr.onload = () => {
      cleanup()
      const data = parseResponseBody(xhr.responseText)
      if (xhr.status === 401) auth.clearToken()
      try {
        resolve(unwrap<T>(data, xhr.status))
      } catch (error) {
        reject(error)
      }
    }

    xhr.onerror = () => {
      cleanup()
      reject(new ApiError('网络异常，请检查连接后重试', 0))
    }

    xhr.ontimeout = () => {
      cleanup()
      reject(new ApiError('上传超时，请重试', 0))
    }

    xhr.onabort = () => {
      cleanup()
      reject(new AbortedError())
    }

    xhr.send(body)
  })
}
