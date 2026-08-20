---
title: account-ui
status: active
code:
  - frontend/src/views/AccountView.vue
related:
  - frontend/src/views/RegisterView.vue
  - frontend/src/views/ChangePasswordView.vue
  - frontend/src/views/SettingsView.vue
  - frontend/src/views/UserProfileView.vue
  - frontend/src/api/account.ts
  - frontend/src/stores/auth.ts
  - frontend/src/router/index.ts
---
# account-ui

## raw source
The web application supports email one-time-code login that also registers, optional password login, account settings, password changes, logout, and public profile views.

## expanded spec
Authenticated and unauthenticated routes remain distinct. Successful account mutations update or clear local authentication state as required, while failed requests show an actionable error instead of pretending the mutation succeeded.

The unsigned account surface leads with email and a six-digit code. Sending a code starts a short cooldown on the button. WeChat and QQ controls are visible but only explain that they are not available yet. Password login remains reachable as a secondary control so existing accounts can still sign in. The dedicated register route sends the visitor back to this email flow instead of collecting a separate username and password.
