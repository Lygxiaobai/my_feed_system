import { postJson } from './client'
import type {
  Account,
  BackendAccountEnvelope,
  PasskeyBeginResponse,
  PasskeyItem,
  PasskeyListResponse,
  TokenResponse,
} from './types'

export function register(username: string, password: string) {
  return postJson<BackendAccountEnvelope>('/account/register', { username, password })
}

export function login(username: string, password: string) {
  return postJson<TokenResponse>('/account/login', { username, password })
}

export function sendEmailCode(email: string) {
  return postJson<null>('/account/email/sendCode', { email })
}

export function verifyEmail(email: string, code: string) {
  return postJson<TokenResponse>('/account/email/verify', { email, code })
}

export function logout() {
  return postJson<null>('/account/logout', {}, { authRequired: true })
}

export function rename(newUsername: string) {
  return postJson<TokenResponse>('/account/rename', { new_username: newUsername }, { authRequired: true })
}

export function changePassword(oldPassword: string, newPassword: string) {
  return postJson<null>(
    '/account/changePassword',
    {
      old_password: oldPassword,
      new_password: newPassword,
    },
    { authRequired: true },
  )
}

export async function findById(id: number) {
  const res = await postJson<BackendAccountEnvelope>('/account/findByID', { id })
  return res.account as Account
}

export async function findByUsername(username: string) {
  const res = await postJson<BackendAccountEnvelope>('/account/findByUsername', { username })
  return res.account as Account
}

export function beginPasskeyLogin() {
  return postJson<PasskeyBeginResponse>('/account/passkey/login/begin', {})
}

export function finishPasskeyLogin(sessionId: string, credential: Record<string, unknown>) {
  return postJson<TokenResponse>('/account/passkey/login/finish', {
    session_id: sessionId,
    credential,
  })
}

export function beginPasskeyRegister(name?: string) {
  return postJson<PasskeyBeginResponse>(
    '/account/passkey/register/begin',
    name ? { name } : {},
    { authRequired: true },
  )
}

export function finishPasskeyRegister(sessionId: string, credential: Record<string, unknown>) {
  return postJson<{ passkey: PasskeyItem }>(
    '/account/passkey/register/finish',
    { session_id: sessionId, credential },
    { authRequired: true },
  )
}

export function listPasskeys() {
  return postJson<PasskeyListResponse>('/account/passkey/list', {}, { authRequired: true })
}

export function deletePasskey(id: number) {
  return postJson<null>('/account/passkey/delete', { id }, { authRequired: true })
}
