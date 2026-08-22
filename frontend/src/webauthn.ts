function isRecord(value: unknown): value is Record<string, unknown> {
  return !!value && typeof value === 'object'
}

export function passkeySupported(): boolean {
  return typeof window !== 'undefined' && window.isSecureContext && typeof window.PublicKeyCredential === 'function'
}

export async function conditionalMediationAvailable(): Promise<boolean> {
  if (!passkeySupported()) return false
  const probe = PublicKeyCredential.isConditionalMediationAvailable
  if (typeof probe !== 'function') return false
  try {
    return await probe.call(PublicKeyCredential)
  } catch {
    return false
  }
}

function toBuffer(value: unknown): ArrayBuffer {
  if (typeof value !== 'string' || value.length === 0) {
    throw new Error('通行密钥参数无效')
  }
  const pad = '='.repeat((4 - (value.length % 4)) % 4)
  const binary = atob(value.replace(/-/g, '+').replace(/_/g, '/') + pad)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}

function toB64url(value: ArrayBuffer): string {
  const bytes = new Uint8Array(value)
  let binary = ''
  for (let i = 0; i < bytes.length; i += 1) {
    binary += String.fromCharCode(bytes[i] ?? 0)
  }
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '')
}

function asTransports(value: unknown): AuthenticatorTransport[] | undefined {
  if (!Array.isArray(value)) return undefined
  return value.filter((item): item is AuthenticatorTransport => typeof item === 'string') as AuthenticatorTransport[]
}

export function decodeCreationOptions(options: unknown): PublicKeyCredentialCreationOptions {
  if (!isRecord(options) || !isRecord(options.publicKey)) {
    throw new Error('通行密钥参数无效')
  }
  const pk = options.publicKey
  if (!isRecord(pk.user)) {
    throw new Error('通行密钥参数无效')
  }
  const exclude = Array.isArray(pk.excludeCredentials) ? pk.excludeCredentials : []
  return {
    ...(pk as unknown as PublicKeyCredentialCreationOptions),
    challenge: toBuffer(pk.challenge),
    user: {
      id: toBuffer(pk.user.id),
      name: String(pk.user.name ?? ''),
      displayName: String(pk.user.displayName ?? ''),
    },
    excludeCredentials: exclude.filter(isRecord).map((item) => ({
      type: 'public-key' as const,
      id: toBuffer(item.id),
      transports: asTransports(item.transports),
    })),
  }
}

export function decodeRequestOptions(options: unknown): PublicKeyCredentialRequestOptions {
  if (!isRecord(options) || !isRecord(options.publicKey)) {
    throw new Error('通行密钥参数无效')
  }
  const pk = options.publicKey
  const allow = Array.isArray(pk.allowCredentials) ? pk.allowCredentials : []
  return {
    ...(pk as unknown as PublicKeyCredentialRequestOptions),
    challenge: toBuffer(pk.challenge),
    allowCredentials: allow.filter(isRecord).map((item) => ({
      type: 'public-key' as const,
      id: toBuffer(item.id),
      transports: asTransports(item.transports),
    })),
  }
}

export function encodeCredential(cred: PublicKeyCredential): Record<string, unknown> {
  const response = cred.response
  const body: Record<string, unknown> = {
    id: cred.id,
    rawId: toB64url(cred.rawId),
    type: cred.type,
    clientExtensionResults: cred.getClientExtensionResults(),
  }
  if (cred.authenticatorAttachment) {
    body.authenticatorAttachment = cred.authenticatorAttachment
  }
  const encoded: Record<string, unknown> = {
    clientDataJSON: toB64url(response.clientDataJSON),
  }
  if ('attestationObject' in response) {
    const att = response as AuthenticatorAttestationResponse
    encoded.attestationObject = toB64url(att.attestationObject)
    if (typeof att.getTransports === 'function') {
      encoded.transports = att.getTransports()
    }
  }
  if ('authenticatorData' in response) {
    const assertion = response as AuthenticatorAssertionResponse
    encoded.authenticatorData = toB64url(assertion.authenticatorData)
    encoded.signature = toB64url(assertion.signature)
    encoded.userHandle = assertion.userHandle ? toB64url(assertion.userHandle) : null
  }
  body.response = encoded
  return body
}

export async function createPasskey(options: unknown): Promise<PublicKeyCredential> {
  const cred = await navigator.credentials.create({
    publicKey: decodeCreationOptions(options),
  })
  if (!(cred instanceof PublicKeyCredential)) {
    throw new Error('未收到通行密钥')
  }
  return cred
}

export async function getPasskey(
  options: unknown,
  extra?: { conditional?: boolean; signal?: AbortSignal },
): Promise<PublicKeyCredential> {
  const cred = await navigator.credentials.get({
    publicKey: decodeRequestOptions(options),
    mediation: extra?.conditional ? 'conditional' : undefined,
    signal: extra?.signal,
  })
  if (!(cred instanceof PublicKeyCredential)) {
    throw new Error('未收到通行密钥')
  }
  return cred
}

export function isPasskeyCanceled(err: unknown): boolean {
  return err instanceof DOMException && (err.name === 'NotAllowedError' || err.name === 'AbortError')
}

export const passkeyUnavailableTip = '通行密钥只能在 HTTPS 安全上下文中使用，请通过站点域名访问'
