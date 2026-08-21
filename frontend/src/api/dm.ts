import { postJson } from './client'

export type DMRelation = 'friend' | 'following' | 'follower' | 'none'

export type DMPeer = {
  id: number
  username: string
}

export type DMConversation = {
  peer: DMPeer
  preview: string
  unread: number
  last_at: string
  last_sender_id: number
}

export type DMMessage = {
  id: number
  sender_id: number
  body: string
  mine: boolean
  read: boolean
  created_at: string
}

export type DMInbox = {
  items: DMConversation[]
}

export type DMThread = {
  peer: DMPeer
  relation: DMRelation
  can_send: boolean
  remaining: number
  messages: DMMessage[]
  has_more: boolean
}

export type DMSendResult = {
  message: DMMessage
  relation: DMRelation
  can_send: boolean
  remaining: number
}

export type DMUnreadCount = {
  count: number
}

export function listInbox() {
  return postJson<DMInbox>('/dm/inbox', {}, { authRequired: true })
}

export function unreadCount() {
  return postJson<DMUnreadCount>('/dm/unreadCount', {}, { authRequired: true })
}

export function openThread(input: { peer_id: number; after_id?: number; before_id?: number; limit?: number }) {
  return postJson<DMThread>('/dm/thread', input, { authRequired: true })
}

export function markRead(peerId: number) {
  return postJson<null>('/dm/markRead', { peer_id: peerId }, { authRequired: true })
}

export function sendMessage(peerId: number, content: string) {
  return postJson<DMSendResult>('/dm/send', { peer_id: peerId, content }, { authRequired: true })
}
