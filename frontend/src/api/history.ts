import { postJson, resolveAssetUrl } from './client'
import type { HistoryItem, HistoryListResponse, HistoryStatus, HistoryUpsertResponse, Video, WatchProgress } from './types'

function normalizeVideo(video: Video): Video {
  return {
    ...video,
    play_url: resolveAssetUrl(video.play_url),
    cover_url: resolveAssetUrl(video.cover_url),
    comment_count: video.comment_count ?? 0,
  }
}

function normalizeItem(item: HistoryItem): HistoryItem {
  return { ...item, video: normalizeVideo(item.video) }
}

export function upsertProgress(videoId: number, positionMs: number, durationMs: number) {
  return postJson<HistoryUpsertResponse>(
    '/history/upsert',
    { video_id: videoId, position_ms: positionMs, duration_ms: durationMs },
    { authRequired: true },
  )
}

export async function listHistory(status: HistoryStatus, cursor = '', limit = 20) {
  const res = await postJson<HistoryListResponse>(
    '/history/list',
    { status, cursor, limit },
    { authRequired: true },
  )
  return {
    items: (res.items ?? []).map(normalizeItem),
    next_cursor: res.next_cursor ?? '',
    has_more: !!res.has_more,
  }
}

export async function listProgress(videoIds: number[]) {
  if (videoIds.length === 0) return [] as WatchProgress[]
  const res = await postJson<{ items: WatchProgress[] }>(
    '/history/progress',
    { video_ids: videoIds },
    { authRequired: true },
  )
  return res.items ?? []
}

export function deleteHistory(videoId: number) {
  return postJson<null>('/history/delete', { video_id: videoId }, { authRequired: true })
}
