---
title: theme-ui
status: active
code:
  - frontend/src/theme.ts
  - frontend/src/stores/theme.ts
  - frontend/src/components/ThemePicker.vue
related:
  - frontend/index.html
  - frontend/src/main.ts
  - frontend/src/style.css
  - frontend/src/components/AppShell.vue
  - frontend/src/components/AppIcon.vue
  - frontend/src/views/SettingsView.vue
desc: Client-owned light and dark appearance with system follow or a manual lock.
---
# theme-ui

## raw source
The web application offers light and dark appearance. The visitor can follow the operating system or lock one mode by hand. The choice is local to the browser and does not require a signed-in session.

## expanded spec
Appearance has one client-side owner in `stores/theme.ts`. The stored preference is `system`, `light`, or `dark`. Missing or invalid storage falls back to `system`. `system` resolves through `prefers-color-scheme` and updates live when that media query changes. `light` and `dark` ignore the operating system until the visitor returns to `system`.

The resolved mode is applied on `document.documentElement` as `data-theme` plus `color-scheme`, and the `theme-color` meta follows it. An inline script in `index.html` applies the stored preference before Vue mounts so the first paint does not flash the opposite palette. Bootstrap starts the store so later preference or system changes stay in sync, including another tab writing the same storage key.

The settings page always hosts the three-way picker, including for signed-out visitors. Account login, rename, and logout stay on that page but must not gate appearance. Application chrome also exposes the same three choices from the desktop sidebar footer, and from a floating control above the tab bar on compact screens; the menu opens upward so it stays on screen. Playback overlays that sit on a video remain high-contrast against the picture; they do not invert with the chrome palette.

The preference is stored under `feed.theme`. A privacy mode that blocks `localStorage` still lets the current session switch; only persistence is skipped.
