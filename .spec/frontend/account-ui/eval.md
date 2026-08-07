---
scenarios:
  - name: account-login
    description: A user can register or log in and reach an authenticated account surface.
    expected: Valid credentials establish the session and invalid credentials show a visible failure without entering an authenticated state.
    tags:
      - frontend-e2e
      - desktop
  - name: account-logout
    description: A logged-in user can log out and cannot continue using authenticated-only views.
    expected: Local authentication state is cleared and protected navigation requires login again.
    tags:
      - frontend-e2e
      - desktop
