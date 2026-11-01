import test from 'node:test'
import assert from 'node:assert/strict'

// Import pure logic functions for testing under node:test
function isOverflowing(scrollWidth, clientWidth) {
  return scrollWidth > clientWidth
}

function getScrollBounds(scrollLeft, clientWidth, scrollWidth, tolerance = 2) {
  const maxScroll = Math.max(0, scrollWidth - clientWidth)
  const canScrollLeft = scrollLeft > 0
  const canScrollRight = scrollLeft + clientWidth < scrollWidth - tolerance
  return { canScrollLeft, canScrollRight, maxScroll }
}

function calculateNextScrollLeft(currentScrollLeft, direction, step = 280, maxScroll) {
  if (direction === 'left') {
    return Math.max(0, currentScrollLeft - step)
  }
  return Math.min(maxScroll, currentScrollLeft + step)
}

function calculateColumnOffset(columnIndex, totalColumns, scrollWidth, maxScroll) {
  if (totalColumns <= 0 || columnIndex <= 0) return 0
  const targetOffset = Math.round((scrollWidth / totalColumns) * columnIndex)
  if (maxScroll !== undefined) {
    return Math.min(maxScroll, Math.max(0, targetOffset))
  }
  return Math.min(scrollWidth, Math.max(0, targetOffset))
}

function createScrollSynchronizer(lockDurationMs = 50) {
  let isSyncing = false
  let timer = null

  return {
    isSyncing: () => isSyncing,
    sync(source, target) {
      if (isSyncing || !target) return false
      isSyncing = true
      target.scrollLeft = source.scrollLeft
      if (timer) clearTimeout(timer)
      timer = setTimeout(() => {
        isSyncing = false
      }, lockDurationMs)
      return true
    },
    reset() {
      if (timer) clearTimeout(timer)
      isSyncing = false
    }
  }
}

test('isOverflowing accurately detects horizontal overflow', () => {
  assert.equal(isOverflowing(1500, 1000), true, 'Overflow detected when content is wider than container')
  assert.equal(isOverflowing(1000, 1000), false, 'No overflow when content equals container')
  assert.equal(isOverflowing(800, 1000), false, 'No overflow when content is narrower than container')
})

test('getScrollBounds detects extreme left, middle, and extreme right states', () => {
  const clientWidth = 800
  const scrollWidth = 1600
  // maxScroll = 800

  // 1. Extreme left (scrollLeft = 0)
  const leftBounds = getScrollBounds(0, clientWidth, scrollWidth)
  assert.equal(leftBounds.canScrollLeft, false)
  assert.equal(leftBounds.canScrollRight, true)
  assert.equal(leftBounds.maxScroll, 800)

  // 2. Middle (scrollLeft = 300)
  const midBounds = getScrollBounds(300, clientWidth, scrollWidth)
  assert.equal(midBounds.canScrollLeft, true)
  assert.equal(midBounds.canScrollRight, true)

  // 3. Extreme right (scrollLeft = 800)
  const rightBounds = getScrollBounds(800, clientWidth, scrollWidth)
  assert.equal(rightBounds.canScrollLeft, true)
  assert.equal(rightBounds.canScrollRight, false)

  // 4. Within tolerance (scrollLeft = 799, within 2px tolerance)
  const nearRightBounds = getScrollBounds(799, clientWidth, scrollWidth, 2)
  assert.equal(nearRightBounds.canScrollRight, false)
})

test('calculateNextScrollLeft steps left and right with boundary clamping', () => {
  const maxScroll = 1000
  const step = 280

  // Step right
  assert.equal(calculateNextScrollLeft(0, 'right', step, maxScroll), 280)
  assert.equal(calculateNextScrollLeft(280, 'right', step, maxScroll), 560)
  // Clamp right
  assert.equal(calculateNextScrollLeft(900, 'right', step, maxScroll), 1000)

  // Step left
  assert.equal(calculateNextScrollLeft(560, 'left', step, maxScroll), 280)
  // Clamp left
  assert.equal(calculateNextScrollLeft(100, 'left', step, maxScroll), 0)
})

test('calculateColumnOffset computes horizontal scroll position for column jump', () => {
  const totalColumns = 5
  const scrollWidth = 1500 // each column is 300px
  const clientWidth = 800
  const maxScroll = scrollWidth - clientWidth // 700

  // Column 0 (first column)
  assert.equal(calculateColumnOffset(0, totalColumns, scrollWidth, maxScroll), 0)

  // Column 1
  assert.equal(calculateColumnOffset(1, totalColumns, scrollWidth, maxScroll), 300)

  // Column 2
  assert.equal(calculateColumnOffset(2, totalColumns, scrollWidth, maxScroll), 600)

  // Column 4 (last column) should clamp to maxScroll
  assert.equal(calculateColumnOffset(4, totalColumns, scrollWidth, maxScroll), 700)

  // Edge cases: totalColumns = 0 or negative
  assert.equal(calculateColumnOffset(2, 0, scrollWidth, maxScroll), 0)
  assert.equal(calculateColumnOffset(-1, totalColumns, scrollWidth, maxScroll), 0)
})

test('createScrollSynchronizer coordinates 2-way scroll without feedback loops', async () => {
  const synchronizer = createScrollSynchronizer(30)

  const topEl = { scrollLeft: 0 }
  const mainEl = { scrollLeft: 0 }

  // 1. User scrolls top bar to 150
  topEl.scrollLeft = 150
  const sync1 = synchronizer.sync(topEl, mainEl)
  assert.equal(sync1, true)
  assert.equal(mainEl.scrollLeft, 150)
  assert.equal(synchronizer.isSyncing(), true)

  // 2. Incoming echoed scroll from mainEl while locked should be blocked
  const syncBlocked = synchronizer.sync(mainEl, topEl)
  assert.equal(syncBlocked, false, 'Feedback loop should be prevented while syncing')

  // 3. Wait for lock timeout to expire
  await new Promise(resolve => setTimeout(resolve, 40))
  assert.equal(synchronizer.isSyncing(), false)

  // 4. Now scrolling mainEl to 300 works
  mainEl.scrollLeft = 300
  const sync2 = synchronizer.sync(mainEl, topEl)
  assert.equal(sync2, true)
  assert.equal(topEl.scrollLeft, 300)

  synchronizer.reset()
  assert.equal(synchronizer.isSyncing(), false)
})
