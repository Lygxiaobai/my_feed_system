---
title: account
status: active
code:
  - backend/internal/account/service.go
related:
  - backend/internal/account/handler.go
  - backend/internal/account/repo.go
  - backend/internal/account/entity.go
  - backend/internal/account/token_cache.go
---
# account

## raw source
Accounts support registration, login, identity lookup, logout, password changes, and profile changes with authenticated state enforced by the backend.

## expanded spec
Passwords are not stored as plaintext. Login establishes the service's current authentication state, logout invalidates it, and account mutations must preserve the relationship between identity, token state, and persisted account data. HTTP handlers, persistence, entities, and token caching are supporting implementation details of this contract.
