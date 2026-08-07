import { postJson } from './client'
import type { IsLikedResponse, ListLikedVideoIDsResponse, MessageResponse } from './types'

export function like(videoId: number) {
  return postJson<MessageResponse>('/like/like', { video_id: videoId }, { authRequired: true })
}

export function unlike(videoId: number) {
  return postJson<MessageResponse>('/like/unlike', { video_id: videoId }, { authRequired: true })
}

function wait(ms: number) {
  return new Promise<void>((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

// 点赞接口使用异步事件更新关系表，前端确认最终状态后才允许下一次操作。
export async function setLikedAndConfirm(videoId: number, liked: boolean) {
  if (liked) await like(videoId)
  else await unlike(videoId)

  for (let attempt = 0; attempt < 6; attempt += 1) {
    const current = await isLiked(videoId)
    if (current.is_liked === liked) return
    await wait(250)
  }

  throw new Error('点赞状态同步超时，请稍后重试')
}

export function isLiked(videoId: number) {
  return postJson<IsLikedResponse>('/like/isLiked', { video_id: videoId }, { authRequired: true })
}

export async function listLikedVideoIds(videoIds: number[]) {
  if (videoIds.length === 0) return []
  const res = await postJson<ListLikedVideoIDsResponse>(
    '/like/listLikedVideoIDs',
    { video_ids: videoIds },
    { authRequired: true },
  )
  return res.video_ids
}
