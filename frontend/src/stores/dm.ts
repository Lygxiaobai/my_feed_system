import { defineStore } from 'pinia'
import { ref } from 'vue'

import * as dmApi from '../api/dm'
import { useAuthStore } from './auth'

const POLL_MS = 15_000

export const useDMStore = defineStore('dm', () => {
  const unread = ref(0)
  const panelOpen = ref(false)
  const activePeerId = ref(0)
  let timer = 0
  let inflight = false

  async function refreshUnread() {
    const auth = useAuthStore()
    if (!auth.isLoggedIn || inflight) return
    inflight = true
    try {
      const res = await dmApi.unreadCount()
      unread.value = Number(res.count) || 0
    } catch {
      // 角标失败不能打断浏览。
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

  function openInbox() {
    panelOpen.value = true
    activePeerId.value = 0
  }

  function openChat(peerId: number) {
    if (peerId <= 0) return
    panelOpen.value = true
    activePeerId.value = peerId
  }

  function setActivePeer(peerId: number) {
    activePeerId.value = peerId > 0 ? peerId : 0
  }

  function closePanel() {
    panelOpen.value = false
    activePeerId.value = 0
  }

  function clear() {
    unread.value = 0
    closePanel()
    stopPolling()
  }

  function applyUnread(count: number) {
    unread.value = Math.max(0, count)
  }

  return {
    unread,
    panelOpen,
    activePeerId,
    refreshUnread,
    startPolling,
    stopPolling,
    openInbox,
    openChat,
    setActivePeer,
    closePanel,
    clear,
    applyUnread,
  }
})
