---
title: account
status: active
code:
  - backend/internal/account/service.go
related:
  - backend/internal/account/handler.go
  - backend/internal/account/repo.go
  - backend/internal/account/entity.go
  - backend/internal/account/email.go
  - backend/internal/account/otp.go
  - backend/internal/account/smtp.go
  - backend/internal/account/token_cache.go
---
# account

## raw source
Accounts support password login, email one-time-code login that also registers, identity lookup, logout, password changes, and profile changes with authenticated state enforced by the backend. Creating an account also grants a one-time registration coin gift owned by the wallet contract.

## expanded spec
Passwords are not stored as plaintext. Login establishes the service's current authentication state, logout invalidates it, and account mutations must preserve the relationship between identity, token state, and persisted account data.

An account may have no password when it was created by email verification. Email login is login-or-register: a verified address binds to one identity row (`provider=email`) and either opens the existing account or creates one. Account creation and the registration gift succeed or fail together. Ordinary addresses receive a six-digit code by SMTP; the code is stored only as a hash with a short lifetime and is consumed on success. Addresses whose local part is only digits and whose domain is the configured test domain do not send mail and accept any six-digit code after a send session has been opened. Failed verification does not distinguish missing, expired, or wrong codes. WeChat and QQ identities are reserved in the binding table and are not accepted as login providers yet.
