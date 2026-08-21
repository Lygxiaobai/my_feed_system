import { postJson } from './client'
import type { DanmakuItem, DanmakuListEnvelope, DanmakuSendResponse } from './types'

export function listByVideo(videoId: number) {
  return postJson<DanmakuListEnvelope>('/danmaku/list', { video_id: videoId }).then((res) => res.items ?? [])
}

export function send(videoId: number, content: string, offsetMs: number) {
  return postJson<DanmakuSendResponse>(
    '/danmaku/send',
    {
      video_id: videoId,
      content,
      offset_ms: offsetMs,
    },
    { authRequired: true },
  ).then((res) => res.item)
}

export type { DanmakuItem }
