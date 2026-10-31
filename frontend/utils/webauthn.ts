export function isWebAuthnSupported(): boolean {
  return typeof window !== 'undefined' &&
    window.PublicKeyCredential !== undefined &&
    typeof window.navigator?.credentials?.create === 'function' &&
    typeof window.navigator?.credentials?.get === 'function'
}

export function bufferToBase64URL(buffer: ArrayBuffer): string {
  const bytes = new Uint8Array(buffer)
  let binary = ''
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i])
  }
  return btoa(binary)
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=+$/, '')
}

export function base64URLToBuffer(base64url: string): ArrayBuffer {
  let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/')
  while (base64.length % 4 !== 0) {
    base64 += '='
  }
  const binary = atob(base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return bytes.buffer
}

export async function performWebAuthnRegister(options: any): Promise<{ client_data_json: string; attestation_object: string }> {
  if (!isWebAuthnSupported()) {
    throw new Error('お使いのブラウザまたは環境はWebAuthn（セキュリティキー認証）に対応していません')
  }

  const publicKeyOptions: PublicKeyCredentialCreationOptions = {
    ...options,
    challenge: typeof options.challenge === 'string' ? base64URLToBuffer(options.challenge) : options.challenge,
    user: {
      ...options.user,
      id: typeof options.user.id === 'string' ? new TextEncoder().encode(options.user.id) : options.user.id
    }
  }

  const credential = await navigator.credentials.create({
    publicKey: publicKeyOptions
  }) as PublicKeyCredential | null

  if (!credential) {
    throw new Error('セキュリティキーの登録がキャンセルされました')
  }

  const response = credential.response as AuthenticatorAttestationResponse
  return {
    client_data_json: bufferToBase64URL(response.clientDataJSON),
    attestation_object: bufferToBase64URL(response.attestationObject)
  }
}

export async function performWebAuthnLogin(options: any): Promise<{ credential_id: string; client_data_json: string; authenticator_data: string; signature: string }> {
  if (!isWebAuthnSupported()) {
    throw new Error('お使いのブラウザまたは環境はWebAuthn（セキュリティキー認証）に対応していません')
  }

  const publicKeyOptions: PublicKeyCredentialRequestOptions = {
    ...options,
    challenge: typeof options.challenge === 'string' ? base64URLToBuffer(options.challenge) : options.challenge,
    allowCredentials: options.allowCredentials?.map((c: any) => ({
      ...c,
      id: typeof c.id === 'string' ? base64URLToBuffer(c.id) : c.id
    }))
  }

  const credential = await navigator.credentials.get({
    publicKey: publicKeyOptions
  }) as PublicKeyCredential | null

  if (!credential) {
    throw new Error('セキュリティキー認証がキャンセルされました')
  }

  const response = credential.response as AuthenticatorAssertionResponse
  return {
    credential_id: bufferToBase64URL(credential.rawId),
    client_data_json: bufferToBase64URL(response.clientDataJSON),
    authenticator_data: bufferToBase64URL(response.authenticatorData),
    signature: bufferToBase64URL(response.signature)
  }
}
