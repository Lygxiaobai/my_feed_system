import { postJson, resolveAssetUrl } from './client'
import type { AuditStatus } from './types'

export type AdminAccess = {
  allowed: boolean
}

export type AdminOverview = {
  pending_reports: number
  account_id: number
  username: string
}

export type AdminVideo = {
  id: number
  author_id: number
  username: string
  title: string
  description?: string
  play_url: string
  cover_url: string
  likes_count: number
  comment_count: number
  audit_status: AuditStatus
  created_at: string
  pending_reports: number
}

export type AdminAccount = {
  id: number
  username: string
  email?: string
  follower_count: number
  created_at: string
}

export const ADMIN_NOTE_MAX = 500

export function adminAccess() {
  return postJson<AdminAccess>('/admin/access', {}, { authRequired: true })
}

export function adminOverview() {
  return postJson<AdminOverview>('/admin/overview', {}, { authRequired: true })
}

function normalizeVideo(video: AdminVideo): AdminVideo {
  return {
    ...video,
    play_url: resolveAssetUrl(video.play_url),
    cover_url: resolveAssetUrl(video.cover_url),
    comment_count: video.comment_count ?? 0,
    pending_reports: video.pending_reports ?? 0,
  }
}

export async function lookupAdminVideo(videoId: number) {
  const res = await postJson<{ video: AdminVideo }>('/admin/videos/lookup', { video_id: videoId }, { authRequired: true })
  return normalizeVideo(res.video)
}

export function takedownAdminVideo(videoId: number, note: string) {
  return postJson<{ ok: boolean }>('/admin/videos/takedown', { video_id: videoId, note }, { authRequired: true })
}

export async function lookupAdminAccount(input: { id?: number; username?: string; email?: string }) {
  const res = await postJson<{ account: AdminAccount; videos: AdminVideo[] }>(
    '/admin/accounts/lookup',
    input,
    { authRequired: true },
  )
  return {
    account: res.account,
    videos: (res.videos ?? []).map(normalizeVideo),
  }
}

export const AUDIT_STATUS_LABEL: Record<AuditStatus, string> = {
  pending: '待审',
  reviewing: '复审中',
  approved: '公开',
  rejected: '已下架',
}
