import { defineStore } from 'pinia'
import { ref } from 'vue'

import * as notificationApi from '../api/notification'
import { useAuthStore } from './auth'

const POLL_MS = 30_000

export const useNotificationStore = defineStore('notification', () => {
  const unread = ref(0)
  let timer = 0
  let inflight = false

  async function refreshUnread() {
    const auth = useAuthStore()
    if (!auth.isLoggedIn || inflight) return
    inflight = true
    try {
      const res = await notificationApi.unreadCount()
      unread.value = Number(res.count) || 0
    } catch {
      // 角标失败不能打断浏览；下次轮询再试。
    } finally {
      inflight = false
    }
  }

  function startPolling() {
    stopPolling()
    void refreshUnread()
    timer = window.setInterval(() => {
      if (document.visibilityState === 'hidden') return
      void refreshUnread()
    }, POLL_MS)
  }

  function stopPolling() {
    if (timer) {
      window.clearInterval(timer)
      timer = 0
    }
  }

  function clear() {
    unread.value = 0
    stopPolling()
  }

  function applyLocalRead(count: number) {
    unread.value = Math.max(0, unread.value - count)
  }

  function applyAllRead() {
    unread.value = 0
  }

  return { unread, refreshUnread, startPolling, stopPolling, clear, applyLocalRead, applyAllRead }
})
