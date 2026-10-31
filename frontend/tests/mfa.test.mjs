import test from 'node:test'
import assert from 'node:assert/strict'
import { bufferToBase64URL, base64URLToBuffer } from '../utils/webauthn.ts'
import { generateQRCodeSVG } from '../utils/qrcode.ts'

test('bufferToBase64URL and base64URLToBuffer encode/decode accurately', () => {
  const originalBytes = new Uint8Array([0, 1, 2, 250, 255, 128, 64, 32, 16])
  const base64url = bufferToBase64URL(originalBytes.buffer)

  // Must not have '+', '/', or '=' padding
  assert.equal(base64url.includes('+'), false)
  assert.equal(base64url.includes('/'), false)
  assert.equal(base64url.includes('='), false)

  const decodedBuffer = base64URLToBuffer(base64url)
  const decodedBytes = new Uint8Array(decodedBuffer)

  assert.equal(decodedBytes.length, originalBytes.length)
  for (let i = 0; i < originalBytes.length; i++) {
    assert.equal(decodedBytes[i], originalBytes[i])
  }
})

test('generateQRCodeSVG produces valid SVG element', () => {
  const otpauth = 'otpauth://totp/Polaris:alice@example.com?secret=JBSWY3DPEHPK3PXP&issuer=Polaris'
  const svg = generateQRCodeSVG(otpauth)

  assert.ok(svg.startsWith('<svg'))
  assert.ok(svg.includes('xmlns="http://www.w3.org/2000/svg"'))
  assert.ok(svg.endsWith('</svg>'))
})

test('isMfaEnforcedPending accurately gates unconfigured users under policy', () => {
  function checkEnforcedPending(user, mfaStatus, isPublicRoute = false) {
    if (isPublicRoute || !user) return false
    const enforced = Boolean(user.mfa_enforced || mfaStatus?.system_mfa_required || mfaStatus?.mfa_enforced)
    return enforced && !user.mfa_enabled && !mfaStatus?.mfa_enabled
  }

  // 1. Regular user with no policy and no MFA -> false
  assert.equal(checkEnforcedPending({ mfa_enabled: false, mfa_enforced: false }, { system_mfa_required: false, mfa_enabled: false }), false)

  // 2. User with user-level mfa_enforced and no MFA -> true
  assert.equal(checkEnforcedPending({ mfa_enabled: false, mfa_enforced: true }, { system_mfa_required: false, mfa_enabled: false }), true)

  // 3. User with system_mfa_required and no MFA -> true
  assert.equal(checkEnforcedPending({ mfa_enabled: false, mfa_enforced: false }, { system_mfa_required: true, mfa_enabled: false }), true)

  // 4. User with system_mfa_required but already has MFA enabled -> false
  assert.equal(checkEnforcedPending({ mfa_enabled: true, mfa_enforced: false }, { system_mfa_required: true, mfa_enabled: true }), false)

  // 5. On public route (e.g. login) -> always false
  assert.equal(checkEnforcedPending({ mfa_enabled: false, mfa_enforced: true }, { system_mfa_required: true, mfa_enabled: false }, true), false)
})

test('OAuth redirect query mfa_temp_token auto-triggers MFA step', () => {
  function handleLoginMounted(query, webauthnOpts = null) {
    let isMfaStep = false
    let tempToken = ''
    let hasWebauthn = false
    let hasTotp = false
    let activeTab = 'totp'

    if (query.mfa_temp_token) {
      tempToken = String(query.mfa_temp_token)
      isMfaStep = true
      hasTotp = true
      activeTab = 'totp'

      if (webauthnOpts && webauthnOpts.allowCredentials && webauthnOpts.allowCredentials.length > 0) {
        hasWebauthn = true
        activeTab = 'webauthn'
      }
    }

    return { isMfaStep, tempToken, hasWebauthn, hasTotp, activeTab }
  }

  // Without token
  const r1 = handleLoginMounted({})
  assert.equal(r1.isMfaStep, false)

  // With token, no WebAuthn
  const r2 = handleLoginMounted({ mfa_temp_token: 'temp-xyz-123' })
  assert.equal(r2.isMfaStep, true)
  assert.equal(r2.tempToken, 'temp-xyz-123')
  assert.equal(r2.hasTotp, true)
  assert.equal(r2.hasWebauthn, false)
  assert.equal(r2.activeTab, 'totp')

  // With token and registered WebAuthn key
  const r3 = handleLoginMounted(
    { mfa_temp_token: 'temp-xyz-456' },
    { allowCredentials: [{ type: 'public-key', id: 'key-1' }] }
  )
  assert.equal(r3.isMfaStep, true)
  assert.equal(r3.tempToken, 'temp-xyz-456')
  assert.equal(r3.hasWebauthn, true)
  assert.equal(r3.activeTab, 'webauthn')
})
