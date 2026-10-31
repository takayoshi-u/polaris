import test from 'node:test'
import assert from 'node:assert/strict'

// Helper replicating the status badge logic and the recommended fix
function getPATStatusCurrentBuggy(tok) {
  if (tok.revoked_at) return '失効済み'
  return '有効'
}

function getPATStatusFixed(tok, now = new Date()) {
  if (tok.revoked_at) return '失効済み'
  if (tok.expires_at && new Date(tok.expires_at) < now) return '期限切れ'
  return '有効'
}

test('PAT status evaluation detects expired tokens correctly', () => {
  const activeToken = {
    id: 'tok-1',
    token_prefix: 'plr_pat_abcdef',
    expires_at: '2026-12-31T00:00:00Z',
    revoked_at: null
  }
  const revokedToken = {
    id: 'tok-2',
    token_prefix: 'plr_pat_123456',
    expires_at: '2026-12-31T00:00:00Z',
    revoked_at: '2026-10-30T10:00:00Z'
  }
  const expiredToken = {
    id: 'tok-3',
    token_prefix: 'plr_pat_7890ab',
    expires_at: '2026-10-01T00:00:00Z',
    revoked_at: null
  }

  const now = new Date('2026-10-31T12:00:00Z')

  // Buggy implementation in default.vue incorrectly marks expired token as "有効"
  assert.equal(getPATStatusCurrentBuggy(expiredToken), '有効')

  // Fixed implementation correctly marks expired token as "期限切れ"
  assert.equal(getPATStatusFixed(activeToken, now), '有効')
  assert.equal(getPATStatusFixed(revokedToken, now), '失効済み')
  assert.equal(getPATStatusFixed(expiredToken, now), '期限切れ')
})

test('PAT token prefix format validation', () => {
  const validPrefix = 'plr_pat_a1b2c3'
  assert.equal(validPrefix.startsWith('plr_pat_'), true)
  assert.equal(validPrefix.length, 14) // 'plr_pat_' (8) + 6 hex chars = 14
})
