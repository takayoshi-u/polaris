import test from 'node:test'
import assert from 'node:assert/strict'

function parseDirectUserText(
  text,
  allowedDomains = [],
  existingEmails = new Set(),
  batchDefaultPassword = ''
) {
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
    let password = ''

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
      displayName = parts[1].trim().replace(/^["']|["']$/g, '')
    }
    if (parts.length > 2) {
      password = parts.slice(2).join(',').trim().replace(/^["']|["']$/g, '')
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

    let passwordMode = 'auto_generate'
    if (password) {
      passwordMode = 'custom'
    } else if (batchDefaultPassword.trim()) {
      passwordMode = 'batch_default'
    }

    results.push({
      email,
      displayName,
      password: password || undefined,
      passwordMode,
      domain,
      isDomainAllowed,
      isDuplicateInBatch,
      isAlreadyRegistered,
      isValidEmailFormat: isValidFormat
    })
  }

  return results
}

test('parseDirectUserText parses 3-column CSV with custom passwords', () => {
  const input = `
user1@example.com, 山田 太郎, CustomPass123!
user2@example.com\t佐藤 健\tKenSecret456!
"user3@example.com","鈴木 一朗","Ichiro789!"
`
  const parsed = parseDirectUserText(input)
  assert.equal(parsed.length, 3)
  assert.equal(parsed[0].email, 'user1@example.com')
  assert.equal(parsed[0].displayName, '山田 太郎')
  assert.equal(parsed[0].password, 'CustomPass123!')
  assert.equal(parsed[0].passwordMode, 'custom')

  assert.equal(parsed[1].displayName, '佐藤 健')
  assert.equal(parsed[1].password, 'KenSecret456!')
  assert.equal(parsed[1].passwordMode, 'custom')

  assert.equal(parsed[2].displayName, '鈴木 一朗')
  assert.equal(parsed[2].password, 'Ichiro789!')
  assert.equal(parsed[2].passwordMode, 'custom')
})

test('parseDirectUserText assigns batch default password mode when password is blank', () => {
  const input = `
user1@example.com, 田中 花子
user2@example.com, 高橋 次郎, SpecificPass1!
`
  const parsed = parseDirectUserText(input, [], new Set(), 'BatchPass2026!')
  assert.equal(parsed.length, 2)
  assert.equal(parsed[0].passwordMode, 'batch_default')
  assert.equal(parsed[0].password, undefined)
  assert.equal(parsed[1].passwordMode, 'custom')
  assert.equal(parsed[1].password, 'SpecificPass1!')
})

test('parseDirectUserText falls back to auto_generate when no password and no batch default', () => {
  const input = `solo@example.com`
  const parsed = parseDirectUserText(input)
  assert.equal(parsed.length, 1)
  assert.equal(parsed[0].displayName, 'solo')
  assert.equal(parsed[0].passwordMode, 'auto_generate')
  assert.equal(parsed[0].password, undefined)
})

test('parseDirectUserText skips header row automatically', () => {
  const inputWithHeader = `email,display_name,password
alice@company.com,Alice,PassA1!
bob@company.com,Bob,PassB2!`
  const parsed = parseDirectUserText(inputWithHeader)
  assert.equal(parsed.length, 2)
  assert.equal(parsed[0].email, 'alice@company.com')
})

test('parseDirectUserText validates allowed domains, duplicates, and existing users', () => {
  const existing = new Set(['already@allowed.com'])
  const allowed = ['allowed.com']
  const input = `
valid@allowed.com, Valid
invalid@other.com, Other
already@allowed.com, Existing
valid@allowed.com, Duplicate
not-an-email, InvalidFormat
`
  const parsed = parseDirectUserText(input, allowed, existing)
  assert.equal(parsed.length, 5)

  // valid
  assert.equal(parsed[0].isValidEmailFormat, true)
  assert.equal(parsed[0].isDomainAllowed, true)
  assert.equal(parsed[0].isDuplicateInBatch, false)
  assert.equal(parsed[0].isAlreadyRegistered, false)

  // invalid domain
  assert.equal(parsed[1].isValidEmailFormat, true)
  assert.equal(parsed[1].isDomainAllowed, false)

  // already registered
  assert.equal(parsed[2].isAlreadyRegistered, true)

  // duplicate in batch
  assert.equal(parsed[3].isDuplicateInBatch, true)

  // format invalid
  assert.equal(parsed[4].isValidEmailFormat, false)
})
