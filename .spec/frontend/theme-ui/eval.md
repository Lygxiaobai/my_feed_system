---
scenarios:
  - name: theme-follows-system
    description: A visitor leaves appearance on 跟随系统, or has no stored preference.
    expected: Chrome, cards, and page text follow the current prefers-color-scheme value. Switching the operating system between light and dark updates the page without a reload.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: theme-manual-override
    description: A visitor picks 浅色 or 深色 from settings, the desktop sidebar footer, or the compact bottom-left chrome control.
    expected: The chosen palette applies immediately and stays put when the operating system preference changes. Choosing 跟随系统 returns control to the operating system.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: theme-persists
    description: A visitor sets a preference and reloads the site.
    expected: The same preference is restored and the first paint already uses the matching palette. A signed-out visitor can complete the same flow.
    tags:
      - frontend-e2e
      - desktop
      - mobile
