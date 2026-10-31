import test from 'node:test'
import assert from 'node:assert/strict'

function formatRelativeTime(isoStr) {
  if (!isoStr) return ''
  const date = new Date(isoStr)
  if (isNaN(date.getTime())) return isoStr
  const now = new Date()
  const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000)

  if (diffSec < 0) return 'たった今'
  if (diffSec < 60) return `${Math.max(1, diffSec)}秒前`
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}分前`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}時間前`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay < 30) return `${diffDay}日前`
  return isoStr.slice(0, 10)
}

function resolveTaskEmbeds(text, tasks) {
  let resolved = text
  const taskByIdRegex = /\btask:([0-9a-fA-F-]{36})\b/g
  resolved = resolved.replace(taskByIdRegex, (match, taskId) => {
    const t = tasks.find(x => x.id === taskId)
    if (t) {
      return `[TASK:${t.id}:${t.title}]`
    }
    return match
  })

  for (const t of tasks) {
    if (!t.title) continue
    const escaped = t.title.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const nameRegex = new RegExp(`(^|\\s)#${escaped}(\\s|$)`, 'g')
    resolved = resolved.replace(nameRegex, (m, p1, p2) => `${p1}[TASK:${t.id}:${t.title}]${p2}`)
  }

  return resolved
}

test('formatRelativeTime formats past intervals correctly', () => {
  const now = new Date()

  const thirtySecAgo = new Date(now.getTime() - 30 * 1000).toISOString()
  assert.equal(formatRelativeTime(thirtySecAgo), '30秒前')

  const fiveMinAgo = new Date(now.getTime() - 5 * 60 * 1000).toISOString()
  assert.equal(formatRelativeTime(fiveMinAgo), '5分前')

  const threeHoursAgo = new Date(now.getTime() - 3 * 3600 * 1000).toISOString()
  assert.equal(formatRelativeTime(threeHoursAgo), '3時間前')

  const twoDaysAgo = new Date(now.getTime() - 2 * 86400 * 1000).toISOString()
  assert.equal(formatRelativeTime(twoDaysAgo), '2日前')
})

test('resolveTaskEmbeds replaces task:UUID and #タスク名 patterns', () => {
  const tasks = [
    { id: '11111111-2222-3333-4444-555555555555', title: '認証基盤設計' },
    { id: '99999999-8888-7777-6666-555555555555', title: 'API実装' }
  ]

  const textWithUUID = '関連タスクは task:11111111-2222-3333-4444-555555555555 です'
  const resUUID = resolveTaskEmbeds(textWithUUID, tasks)
  assert.equal(resUUID, '関連タスクは [TASK:11111111-2222-3333-4444-555555555555:認証基盤設計] です')

  const textWithName = '進捗は #API実装 を確認してください'
  const resName = resolveTaskEmbeds(textWithName, tasks)
  assert.equal(resName, '進捗は [TASK:99999999-8888-7777-6666-555555555555:API実装] を確認してください')
})

function escapeHtml(str) {
  if (!str) return ''
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

test('escapeHtml sanitizes special characters', () => {
  const dangerous = '<script>alert("XSS & test\'s")</script>'
  const sanitized = escapeHtml(dangerous)
  assert.equal(sanitized, '&lt;script&gt;alert(&quot;XSS &amp; test&#39;s&quot;)&lt;/script&gt;')
})
