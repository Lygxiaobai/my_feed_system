export const THEME_STORAGE_KEY = 'feed.theme'

export type ThemePreference = 'system' | 'light' | 'dark'
export type ResolvedTheme = 'light' | 'dark'

const THEME_COLOR: Record<ResolvedTheme, string> = {
  dark: '#0b0b0f',
  light: '#f3f4f7',
}

export function parsePreference(raw: string | null | undefined): ThemePreference {
  if (raw === 'light' || raw === 'dark' || raw === 'system') return raw
  return 'system'
}

export function systemPrefersDark(): boolean {
  try {
    return window.matchMedia('(prefers-color-scheme: dark)').matches
  } catch {
    // 探测失败时保持历史默认深色，避免无故闪成浅色。
    return true
  }
}

export function resolveTheme(pref: ThemePreference): ResolvedTheme {
  if (pref === 'light') return 'light'
  if (pref === 'dark') return 'dark'
  return systemPrefersDark() ? 'dark' : 'light'
}

export function readPreference(): ThemePreference {
  try {
    return parsePreference(window.localStorage.getItem(THEME_STORAGE_KEY))
  } catch {
    return 'system'
  }
}

export function writePreference(pref: ThemePreference) {
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, pref)
  } catch {
    // 隐私模式写不了 localStorage，当次会话的选择仍要生效。
  }
}

export function applyTheme(resolved: ResolvedTheme) {
  const root = document.documentElement
  root.dataset.theme = resolved
  root.style.colorScheme = resolved
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', THEME_COLOR[resolved])
}
