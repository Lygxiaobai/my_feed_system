---
scenarios:
  - name: topbar-messages-opens-inbox
    description: A signed-in user opens 消息 from the desktop top bar.
    expected: A private-message dropdown lists conversations over the current page; a signed-out click goes to the account page.
    tags:
      - frontend-e2e
      - desktop
  - name: dropdown-expands-thread
    description: A signed-in user clicks one conversation in the messages dropdown.
    expected: The same panel expands to show that thread's history and composer; 关闭会话 returns to the list without leaving the page.
    tags:
      - frontend-e2e
      - desktop
  - name: profile-starts-thread
    description: A signed-in user opens another account's profile and taps 发私信.
    expected: On desktop the messages dropdown opens on that thread; on a compact viewport they reach `/messages` with that account selected.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: stranger-composer-locks-after-one
    description: A signed-in user who does not mutually follow the peer sends one message.
    expected: The message appears in the expanded thread with the sender's avatar on the right; the composer then refuses further input and explains that mutual follow is required.
    tags:
      - frontend-e2e
      - desktop
  - name: thread-bubbles-show-sender-avatar
    description: A signed-in user views a thread that contains both their own and the peer's messages.
    expected: Each bubble shows the sender's avatar. The peer's avatar sits on the left and the viewer's avatar sits on the right, using the same identity seed as the top-bar avatar.
    tags:
      - frontend-e2e
      - desktop
      - mobile
  - name: compact-messages-list-then-chat
    description: A signed-in user on the compact top bar opens 消息, picks a conversation, then closes it.
    expected: The list is shown first, the chosen chat replaces the list, and closing returns to the list without leaving `/messages`.
    tags:
      - frontend-e2e
      - mobile
