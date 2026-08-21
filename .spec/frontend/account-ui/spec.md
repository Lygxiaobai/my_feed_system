---
title: account-ui
status: active
code:
  - frontend/src/views/AccountView.vue
related:
  - frontend/src/views/PasswordLoginView.vue
  - frontend/src/views/RegisterView.vue
  - frontend/src/views/ChangePasswordView.vue
  - frontend/src/views/SettingsView.vue
  - frontend/src/views/UserProfileView.vue
  - frontend/src/api/account.ts
  - frontend/src/stores/auth.ts
  - frontend/src/router/index.ts
  - frontend/src/views/OpsView.vue
  - frontend/src/views/WalletView.vue
  - frontend/src/views/PlaceholderView.vue
  - frontend/src/components/AppIcon.vue
  - frontend/src/components/AppShell.vue
---
# account-ui

## raw source
The web application supports email one-time-code login that also registers, optional password login, account settings, password changes, logout, a wallet entry on the signed-in hub, and public profile views.

## expanded spec
Authenticated and unauthenticated routes remain distinct. Successful account mutations update or clear local authentication state as required, while failed requests show an actionable error instead of pretending the mutation succeeded.

The unsigned account surface leads with email and a six-digit code. Sending a code starts a short cooldown on the button. WeChat and QQ controls are visible but only explain that they are not available yet. Password login lives on a dedicated route so it is not mixed into the email form. The dedicated register route sends the visitor back to this email flow instead of collecting a separate username and password. The signed-in hub also reaches the wallet, daily check-in, and lottery pages. The desktop top bar reaches the same wallet and publish pages, the account hub, and frontend-only placeholders for a client download, wallpapers, notifications, and messages. The recharge control is labeled 充积分. Those placeholders do not call a backend. An account bound to a digits-only test-domain email also sees an operations entry on the signed-in hub; that surface is owned by the ops-ui spec.
