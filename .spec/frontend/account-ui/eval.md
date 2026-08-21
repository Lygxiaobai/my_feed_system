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
  - name: account-password-login-page
    description: A visitor opens password login from the unsigned account surface.
    expected: The password form is on its own page and is not mixed with the email code fields; a successful login returns to the authenticated hub.
    tags:
      - frontend-e2e
      - desktop
  - name: account-wallet-entry
    description: A signed-in user opens the wallet from the account hub.
    expected: The wallet page loads for that session and a signed-out visitor cannot stay on the wallet.
    tags:
      - frontend-e2e
      - desktop
  - name: topbar-utility-entries
    description: A user uses the desktop top-bar entries aligned with search.
    expected: The recharge entry is labeled 充积分 and opens the wallet; publish and the avatar open the existing publish and account pages; the notifications bell opens the inbox; client, wallpaper, and messages open frontend placeholders; a signed-out click on an authenticated entry goes to the account page.
    tags:
      - frontend-e2e
      - desktop
  - name: account-logout
    description: A logged-in user can log out and cannot continue using authenticated-only views.
    expected: Local authentication state is cleared and protected navigation requires login again.
    tags:
      - frontend-e2e
      - desktop
