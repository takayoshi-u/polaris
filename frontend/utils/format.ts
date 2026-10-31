import { marked } from 'marked'
import DOMPurify from 'dompurify'

/**
 * Formats an ISO timestamp string to JST (Asia/Tokyo) YYYY-MM-DD HH:mm.
 */
export function formatJSTDateTime(isoStr?: string | null): string {
  if (!isoStr) return '-'
  const d = new Date(isoStr)
  if (Number.isNaN(d.getTime())) return isoStr
  return new Intl.DateTimeFormat('ja-JP', {
    timeZone: 'Asia/Tokyo',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false
  }).format(d)
}

/**
 * Formats a number of hours up to 2 decimal places (e.g., 8 -> "8.00", 6.25 -> "6.25").
 */
export function formatHours(val?: number | null): string {
  if (val === undefined || val === null || Number.isNaN(Number(val))) return '-'
  return Number(val).toFixed(2)
}

/**
 * Formats a cost value up to 2 decimal places with thousands separators (e.g., 12500.5 -> "12,500.50").
 */
export function formatCost(val?: number | null): string {
  if (val === undefined || val === null || Number.isNaN(Number(val))) return '-'
  return new Intl.NumberFormat('ja-JP', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(Number(val))
}

/**
 * Validates that a numeric value has at most 2 decimal places.
 */
export function hasAtMostTwoDecimals(val: number): boolean {
  if (!Number.isFinite(val)) return false
  const scaled = val * 100
  return Math.abs(scaled - Math.round(scaled)) < 1e-6
}

/**
 * Formats file size in bytes into human-readable string (B, KB, MB, GB).
 */
export function formatFileSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes < 0) return '0 B'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(2)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

/**
 * Adds `days` to a YYYY-MM-DD date string and returns a new YYYY-MM-DD string.
 */
export function addDaysYMD(ymd: string, days: number): string {
  const parts = ymd.split('-').map(Number)
  if (parts.length !== 3 || parts.some(Number.isNaN)) return ymd
  const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
  dt.setUTCDate(dt.getUTCDate() + days)
  const y = dt.getUTCFullYear()
  const m = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const d = String(dt.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

/**
 * Computes difference in days (end - start) between two YYYY-MM-DD strings.
 */
export function diffDaysYMD(startYMD: string, endYMD: string): number {
  const s = startYMD.split('-').map(Number)
  const e = endYMD.split('-').map(Number)
  if (s.length !== 3 || e.length !== 3) return 0
  const msStart = Date.UTC(s[0], s[1] - 1, s[2])
  const msEnd = Date.UTC(e[0], e[1] - 1, e[2])
  return Math.round((msEnd - msStart) / 86400000)
}

/**
 * Generates an array of YYYY-MM-DD strings starting from `startYMD` for `count` days.
 */
export function generateDateRangeYMD(startYMD: string, count: number): string[] {
  const list: string[] = []
  for (let i = 0; i < count; i++) {
    list.push(addDaysYMD(startYMD, i))
  }
  return list
}

/**
 * Calculates left offset (px) and width (px) for a Gantt bar on a timeline starting at `timelineStartYMD`.
 */
export function calcGanttBarPosition(
  timelineStartYMD: string,
  barStartYMD?: string | null,
  barEndYMD?: string | null,
  dayWidthPx = 44
): { visible: boolean; leftPx: number; widthPx: number } {
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

/**
 * Safely renders Markdown into sanitized HTML using marked + DOMPurify.
 */
export function renderSafeMarkdown(mdText?: string | null): string {
  if (!mdText) return ''
  const rawHtml = marked.parse(mdText, { async: false }) as string
  if (typeof window !== 'undefined' && DOMPurify?.sanitize) {
    return DOMPurify.sanitize(rawHtml)
  }
  return rawHtml
}

