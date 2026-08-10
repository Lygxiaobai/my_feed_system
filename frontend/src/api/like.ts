import { postJson } from './client'
import type { IsLikedResponse, ListLikedVideoIDsResponse } from './types'

export function like(videoId: number) {
  return postJson<null>('/like/like', { video_id: videoId }, { authRequired: true })
}

export function unlike(videoId: number) {
  return postJson<null>('/like/unlike', { video_id: videoId }, { authRequired: true })
}

// 点赞关系已经在 API 的 MySQL 事务中提交；MQ 只处理热度等派生数据，因此无需轮询关系表。
export async function setLikedAndConfirm(videoId: number, liked: boolean) {
  if (liked) await like(videoId)
  else await unlike(videoId)
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
