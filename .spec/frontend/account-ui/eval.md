---
scenarios:
  - name: account-login
    description: A user can register or log in and reach an authenticated account surface.
    expected: Valid credentials establish the session and invalid credentials show a visible failure without entering an authenticated state.
    tags:
      - frontend-e2e
      - desktop
  - name: account-email-login
    description: A visitor enters an email, requests a code, and submits a six-digit code.
    expected: A successful verify stores the session and shows the authenticated account surface; a failed verify stays signed out and shows the server message.
    tags:
      - frontend-e2e
      - desktop
  - name: account-oauth-placeholder
    description: A visitor clicks WeChat or QQ login.
    expected: No session is created and the visitor is told the method is not available yet.
    tags:
      - frontend-e2e
      - desktop
  - name: account-logout
    description: A logged-in user can log out and cannot continue using authenticated-only views.
    expected: Local authentication state is cleared and protected navigation requires login again.
    tags:
      - frontend-e2e
      - desktop
