---
scenarios:
  - name: desktop-bell-opens-inbox
    description: A signed-in user clicks the desktop top-bar bell, filters to likes, then opens a like notice.
    expected: A dropdown lists interactive notices without leaving the page; the like filter hides other kinds; opening a row marks it read and lands on the video.
    tags:
      - frontend-e2e
      - desktop
  - name: follow-back-from-notice
    description: A signed-in user receives a follow notice from an account they do not follow and taps 回关.
    expected: The follow request is sent, the control disappears, and a signed-out visitor who hits the bell is sent to the account page instead.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: compact-bell-opens-page
    description: A signed-in user on the compact top bar opens the bell.
    expected: They reach the notifications page with the same filters and an unread badge that clears after 全部已读.
    tags:
      - frontend-e2e
      - mobile
