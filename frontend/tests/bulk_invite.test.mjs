import test from 'node:test'
import assert from 'node:assert/strict'

function parseInvitationText(text, allowedDomains = [], existingEmails = new Set()) {
  if (!text || !text.trim()) return []

  const lines = text.split(/\r?\n/).map(l => l.trim()).filter(l => l.length > 0)
  if (lines.length === 0) return []

  const results = []
  const seenEmails = new Set()

  let startIndex = 0
  const firstLine = lines[0].toLowerCase()
  if (
    firstLine.includes('email') ||
    firstLine.includes('メール') ||
    firstLine.startsWith('mail,') ||
    firstLine.startsWith('address,')
  ) {
    startIndex = 1
  }

  for (let i = startIndex; i < lines.length; i++) {
    const line = lines[i]
    let email = ''
    let displayName = ''

    let parts = []
    if (line.includes('\t')) {
      parts = line.split('\t')
    } else if (line.includes(',')) {
      parts = line.split(',')
    } else if (line.includes(';')) {
      parts = line.split(';')
    } else {
      parts = [line]
    }

    email = parts[0].trim().replace(/^["']|["']$/g, '')
    if (parts.length > 1) {
      displayName = parts.slice(1).join(' ').trim().replace(/^["']|["']$/g, '')
    }

    if (!email) continue

    const lowerEmail = email.toLowerCase()
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
    const isValidFormat = emailRegex.test(lowerEmail)
    const domain = lowerEmail.includes('@') ? lowerEmail.split('@')[1].trim() : ''

    let isDomainAllowed = true
    if (allowedDomains && allowedDomains.length > 0) {
      isDomainAllowed = allowedDomains.some(d => {
        const cleanD = d.toLowerCase().trim().replace(/^@/, '').replace(/^\./, '')
        return domain === cleanD || domain.endsWith('.' + cleanD)
      })
    }

    const isDuplicateInBatch = seenEmails.has(lowerEmail)
    seenEmails.add(lowerEmail)

    const isAlreadyRegistered = existingEmails.has(lowerEmail)

    if (!displayName && domain) {
      displayName = lowerEmail.split('@')[0]
    }

    results.push({
      email,
      displayName,
      domain,
      isDomainAllowed,
      isDuplicateInBatch,
      isAlreadyRegistered,
      isValidEmailFormat: isValidFormat
    })
  }

  return results
}

test('parseInvitationText parses single email per line, CSV, and TSV', () => {
  const input = `
user1@example.com
user2@example.com, Yamada Taro
user3@example.com\tSato Ken
`
  const parsed = parseInvitationText(input)
  assert.equal(parsed.length, 3)
  assert.equal(parsed[0].email, 'user1@example.com')
  assert.equal(parsed[0].displayName, 'user1')
  assert.equal(parsed[1].displayName, 'Yamada Taro')
  assert.equal(parsed[2].displayName, 'Sato Ken')
})

test('parseInvitationText skips header row automatically', () => {
  const inputWithHeader = `email,display_name
alice@company.com,Alice
bob@company.com,Bob`

  const parsed = parseInvitationText(inputWithHeader)
  assert.equal(parsed.length, 2)
  assert.equal(parsed[0].email, 'alice@company.com')
  assert.equal(parsed[1].email, 'bob@company.com')
})

test('parseInvitationText validates allowed domains and duplicate detection', () => {
  const input = `
ok1@company.com
ng@other.com
ok1@company.com
existing@company.com
`
  const allowed = ['company.com']
  const existing = new Set(['existing@company.com'])

  const parsed = parseInvitationText(input, allowed, existing)
  assert.equal(parsed.length, 4)

  assert.equal(parsed[0].isDomainAllowed, true)
  assert.equal(parsed[0].isDuplicateInBatch, false)

  // Disallowed domain
  assert.equal(parsed[1].isDomainAllowed, false)

  // Duplicate in batch
  assert.equal(parsed[2].isDuplicateInBatch, true)

  // Subdomain match (tail-matching)
  const inputSub = `sub.company.com_user@sub.company.com`
  const parsedSub = parseInvitationText(inputSub, allowed, existing)
  assert.equal(parsedSub[0].isDomainAllowed, true)
})

