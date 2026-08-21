import { postJson, resolveAssetUrl } from './client'

export type NotificationKind = 'follow' | 'like' | 'comment' | 'reply' | 'mention' | 'tip'

export type NotificationFilter = 'all' | 'follow' | 'like' | 'mention' | 'reply' | 'tip'

export type NotificationActor = {
  id: number
  username: string
}

export type NotificationVideo = {
  id: number
  cover_url: string
  title: string
}

export type NotificationItem = {
  id: number
  kind: NotificationKind
  actors: NotificationActor[]
  actor_count: number
  text: string
  action_text: string
  relation: 'friend' | 'following' | ''
  followed: boolean
  video: NotificationVideo | null
  coins: number
  unread: boolean
  created_at: string
}

export type NotificationList = {
  items: NotificationItem[]
  next_cursor: string
  has_more: boolean
}

export type UnreadCount = {
  count: number
  by_kind: Partial<Record<NotificationKind, number>>
}

function normalizeItem(item: NotificationItem): NotificationItem {
  return {
    ...item,
    actors: item.actors ?? [],
    video: item.video
      ? {
          ...item.video,
          cover_url: resolveAssetUrl(item.video.cover_url),
        }
      : null,
  }
}

export async function listNotifications(input?: { kind?: NotificationFilter; cursor?: string; limit?: number }) {
  const res = await postJson<NotificationList>('/notification/list', input ?? {}, { authRequired: true })
  return {
    items: (res.items ?? []).map(normalizeItem),
    next_cursor: res.next_cursor ?? '',
    has_more: !!res.has_more,
  }
}

export function unreadCount() {
  return postJson<UnreadCount>('/notification/unreadCount', {}, { authRequired: true })
}

export function markRead(ids: number[]) {
  return postJson<null>('/notification/markRead', { ids }, { authRequired: true })
}

export function markAllRead(kind?: NotificationFilter) {
  return postJson<null>('/notification/markAllRead', { kind: kind && kind !== 'all' ? kind : '' }, { authRequired: true })
}
