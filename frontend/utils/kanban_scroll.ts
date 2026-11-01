/**
 * Detects whether content in container is overflowing horizontally.
 */
export function isOverflowing(scrollWidth: number, clientWidth: number): boolean {
  return scrollWidth > clientWidth
}

/**
 * Calculates current scroll bounds and whether scrolling left or right is possible.
 */
export function getScrollBounds(
  scrollLeft: number,
  clientWidth: number,
  scrollWidth: number,
  tolerance = 2
): { canScrollLeft: boolean; canScrollRight: boolean; maxScroll: number } {
  const maxScroll = Math.max(0, scrollWidth - clientWidth)
  const canScrollLeft = scrollLeft > 0
  const canScrollRight = scrollLeft + clientWidth < scrollWidth - tolerance
  return { canScrollLeft, canScrollRight, maxScroll }
}

/**
 * Calculates next scrollLeft position clamped within [0, maxScroll].
 */
export function calculateNextScrollLeft(
  currentScrollLeft: number,
  direction: 'left' | 'right',
  step = 280,
  maxScroll: number
): number {
  if (direction === 'left') {
    return Math.max(0, currentScrollLeft - step)
  }
  return Math.min(maxScroll, currentScrollLeft + step)
}

/**
 * Computes the horizontal scroll offset to jump to a specific column index.
 */
export function calculateColumnOffset(
  columnIndex: number,
  totalColumns: number,
  scrollWidth: number,
  maxScroll?: number
): number {
  if (totalColumns <= 0 || columnIndex <= 0) return 0
  const targetOffset = Math.round((scrollWidth / totalColumns) * columnIndex)
  if (maxScroll !== undefined) {
    return Math.min(maxScroll, Math.max(0, targetOffset))
  }
  return Math.min(scrollWidth, Math.max(0, targetOffset))
}

/**
 * Helper to coordinate 2-way synchronized horizontal scrolling without feedback loops.
 */
export function createScrollSynchronizer(lockDurationMs = 50) {
  let isSyncing = false
  let timer: any = null

  return {
    isSyncing: () => isSyncing,
    sync(source: { scrollLeft: number }, target: { scrollLeft: number } | null): boolean {
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
