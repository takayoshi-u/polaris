import test from 'node:test'
import assert from 'node:assert/strict'

function formatHours(val) {
  if (val === undefined || val === null || Number.isNaN(Number(val))) return '-'
  return Number(val).toFixed(2)
}

function formatCost(val) {
  if (val === undefined || val === null || Number.isNaN(Number(val))) return '-'
  return new Intl.NumberFormat('ja-JP', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(Number(val))
}

function hasAtMostTwoDecimals(val) {
  if (!Number.isFinite(val)) return false
  const scaled = val * 100
  return Math.abs(scaled - Math.round(scaled)) < 1e-6
}

function addDaysYMD(ymd, days) {
  const parts = ymd.split('-').map(Number)
  const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
  dt.setUTCDate(dt.getUTCDate() + days)
  const y = dt.getUTCFullYear()
  const m = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const d = String(dt.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function diffDaysYMD(startYMD, endYMD) {
  const s = startYMD.split('-').map(Number)
  const e = endYMD.split('-').map(Number)
  const msStart = Date.UTC(s[0], s[1] - 1, s[2])
  const msEnd = Date.UTC(e[0], e[1] - 1, e[2])
  return Math.round((msEnd - msStart) / 86400000)
}

function calcGanttBarPosition(timelineStartYMD, barStartYMD, barEndYMD, dayWidthPx = 44) {
  if (!barStartYMD || !barEndYMD) {
    return { visible: false, leftPx: 0, widthPx: 0 }
  }
  const offsetDays = diffDaysYMD(timelineStartYMD, barStartYMD)
  const spanDays = diffDaysYMD(barStartYMD, barEndYMD) + 1
  if (spanDays <= 0) {
    return { visible: false, leftPx: 0, widthPx: 0 }
  }
  return {
    visible: true,
    leftPx: offsetDays * dayWidthPx,
    widthPx: spanDays * dayWidthPx
  }
}

test('formatHours and hasAtMostTwoDecimals validate up to 2 decimal places', () => {
  assert.equal(formatHours(8), '8.00')
  assert.equal(formatHours(7.75), '7.75')
  assert.equal(formatHours(null), '-')

  assert.equal(hasAtMostTwoDecimals(8), true)
  assert.equal(hasAtMostTwoDecimals(7.75), true)
  assert.equal(hasAtMostTwoDecimals(0.01), true)
  assert.equal(hasAtMostTwoDecimals(7.125), false)
  assert.equal(hasAtMostTwoDecimals(8.333), false)

  assert.equal(formatCost(12500.5), '12,500.50')
  assert.equal(formatCost(0), '0.00')
  assert.equal(formatCost(null), '-')
})

test('Gantt timeline date math and bar position calculation', () => {
  assert.equal(addDaysYMD('2026-10-28', 5), '2026-11-02')
  assert.equal(diffDaysYMD('2026-10-20', '2026-10-25'), 5)

  const pos = calcGanttBarPosition('2026-10-20', '2026-10-22', '2026-10-25', 48)
  assert.equal(pos.visible, true)
  assert.equal(pos.leftPx, 2 * 48)
  assert.equal(pos.widthPx, 4 * 48)

  const unset = calcGanttBarPosition('2026-10-20', null, null, 48)
  assert.equal(unset.visible, false)
})

