import { defineStore } from 'pinia'
import { ref } from 'vue'

import {
  applyTheme,
  parsePreference,
  readPreference,
  resolveTheme,
  THEME_STORAGE_KEY,
  writePreference,
  type ResolvedTheme,
  type ThemePreference,
} from '../theme'

export const useThemeStore = defineStore('theme', () => {
  const preference = ref<ThemePreference>(readPreference())
  const resolved = ref<ResolvedTheme>(resolveTheme(preference.value))

  let media: MediaQueryList | null = null
  let started = false

  function syncResolved() {
    resolved.value = resolveTheme(preference.value)
    applyTheme(resolved.value)
  }

  function setPreference(next: ThemePreference) {
    preference.value = next
    writePreference(next)
    syncResolved()
  }

  function onMediaChange() {
    if (preference.value === 'system') syncResolved()
  }

  function onStorage(event: StorageEvent) {
    if (event.key !== THEME_STORAGE_KEY) return
    preference.value = parsePreference(event.newValue)
    syncResolved()
  }

  function start() {
    if (started) return
    started = true
    syncResolved()
    try {
      media = window.matchMedia('(prefers-color-scheme: dark)')
      media.addEventListener('change', onMediaChange)
    } catch {
      media = null
    }
    window.addEventListener('storage', onStorage)
  }

  function stop() {
    if (!started) return
    started = false
    media?.removeEventListener('change', onMediaChange)
    window.removeEventListener('storage', onStorage)
    media = null
  }

  return { preference, resolved, setPreference, start, stop }
})
