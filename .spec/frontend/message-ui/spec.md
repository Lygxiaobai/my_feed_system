---
title: message-ui
status: active
code:
  - frontend/src/components/MessagePanel.vue
related:
  - frontend/src/views/MessagesView.vue
  - frontend/src/api/dm.ts
  - frontend/src/stores/dm.ts
  - frontend/src/components/AppShell.vue
  - frontend/src/router/index.ts
  - frontend/src/views/UserProfileView.vue
---
# message-ui

## raw source
登录用户从顶栏「消息」打开私信下拉，先看到会话列表；点某一条后在同一面板里展开聊天记录。资料页的「发私信」直接展开与对方的会话。

## expanded spec
私信和通知分开。铃铛仍只打开互动通知。桌面点「消息」会在当前页上方打开下拉，不跳走；未登录则去账号页，并且不请求私信接口。消息角标跟踪私信未读，不和通知角标混算。

下拉先列出会话：头像、名字、最后一条摘要、时间和未读红点。点进一条后面板变宽，左侧是头像栏，右侧是该会话的记录和输入框。「关闭会话」回到列表；关闭整块面板才离开私信。紧凑顶栏和 `/messages` 仍是先列表再展开，避免一上来占满整屏。资料页在桌面打开同一块下拉并选中对方，小屏才走到 `/messages?u=`。

互关时输入框可连续发送；未互关时输入框在用掉唯一一条后锁定，并提示关注或等待回关。自己发出的最新一条在对方打开后显示「已读」。本轮只发文本，打开后面板轮询列表和当前线程，不建立长连接。

## change rules
改入口、角标或额度提示必须对照 `backend/dm`。把私信塞进通知下拉属于新能力，必须先改本 spec。
