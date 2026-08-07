---
scenarios:
  - name: account-registration-login
    description: A new user can register and log in with valid credentials, while invalid credentials are rejected.
    expected: Registration creates one account, valid login establishes an authenticated session, and invalid login does not establish one.
    tags:
      - backend-api
  - name: account-logout
    description: An authenticated user can log out and then access to protected account behavior is rejected.
    expected: The session is invalidated after logout and a later request using the old session is no longer accepted.
    tags:
      - backend-api
  - name: account-password-change
    description: An authenticated user can change the password with the correct old password.
    expected: The old password is rejected after the change, the new password can log in, and the account identity is preserved.
    tags:
      - backend-api
