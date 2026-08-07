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
The web application supports registration, login, account settings, password changes, logout, and public profile views.

## expanded spec
Authenticated and unauthenticated routes remain distinct. Successful account mutations update or clear local authentication state as required, while failed requests show an actionable error instead of pretending the mutation succeeded.
