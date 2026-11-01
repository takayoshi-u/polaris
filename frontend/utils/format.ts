import { marked } from 'marked'
import DOMPurify from 'dompurify'
import {
  buildGanttHeaderGroups as _buildGanttHeaderGroups,
  calculateSprintMetrics as _calculateSprintMetrics,
  calculateVelocityStats as _calculateVelocityStats,
  formatLocalYMD as _formatLocalYMD,
  getGanttScaleConfig as _getGanttScaleConfig,
  isDoneStatusName as _isDoneStatusName,
  isTaskCompleted as _isTaskCompleted,
  resolveDoneStatusIds as _resolveDoneStatusIds
} from './agile_gantt.mjs'

/**
 * Formats a Date object (or date-like value) to local YYYY-MM-DD using local calendar getters.
 */
export function formatLocalYMD(date: Date | string | number = new Date()): string {
  return _formatLocalYMD(date)
}

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

/**
 * Formats an ISO date string to a Japanese relative time string (e.g. "たった今", "5分前", "2時間前", "昨日").
 */
export function formatRelativeTime(isoStr?: string | null): string {
  if (!isoStr) return '-'
  const d = new Date(isoStr)
  if (Number.isNaN(d.getTime())) return isoStr
  const now = Date.now()
  const diffSec = Math.floor((now - d.getTime()) / 1000)
  if (diffSec < 60) return 'たった今'
  const diffMin = Math.floor(diffSec / 60)
  if (diffMin < 60) return `${diffMin}分前`
  const diffHour = Math.floor(diffMin / 60)
  if (diffHour < 24) return `${diffHour}時間前`
  const diffDay = Math.floor(diffHour / 24)
  if (diffDay === 1) return '昨日'
  if (diffDay < 7) return `${diffDay}日前`
  return formatJSTDateTime(isoStr)
}

export interface TaskEmbedInfo {
  id: string
  title: string
  status_id?: string
  progress?: number
  planned_hours?: number | null
}

/**
 * Escapes HTML characters in a string to prevent XSS.
 */
export function escapeHtml(str?: string | null): string {
  if (!str) return ''
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

/**
 * Renders Markdown for Wiki with task embed tokens (task:UUID or #taskTitle) replaced with rich interactive pills.
 */
export function renderWikiMarkdownWithTaskEmbeds(
  mdText?: string | null,
  tasks: TaskEmbedInfo[] = [],
  statusMap: Map<string, { name: string; color: string }> = new Map()
): string {
  if (!mdText) return ''

  let processed = mdText

  // 1. Match task:UUID
  processed = processed.replace(/\btask:([0-9a-fA-F-]{36})\b/g, (match, taskId) => {
    const t = tasks.find(x => x.id === taskId)
    if (!t) return match
    const st = t.status_id ? statusMap.get(t.status_id) : undefined
    const stColor = st?.color || '#3b82f6'
    const stName = escapeHtml(st?.name || 'ステータス未設定')
    const escapedTitle = escapeHtml(t.title)
    const escapedId = escapeHtml(t.id)
    const prog = t.progress ?? 0
    return `<a href="javascript:void(0)" class="wiki-task-pill" data-task-id="${escapedId}" style="display: inline-flex; align-items: center; gap: 6px; padding: 2px 8px; border-radius: 6px; background: rgba(59, 130, 246, 0.1); border: 1px solid rgba(59, 130, 246, 0.3); color: var(--text-main); text-decoration: none; font-size: 12px; font-weight: 500;"><span style="width: 8px; height: 8px; border-radius: 50%; background: ${stColor};"></span><strong style="color: var(--primary);">📋 ${escapedTitle}</strong><span style="font-size: 10px; color: var(--text-muted);">(${stName} ${prog}%)</span></a>`
  })

  // 2. Match #TaskTitle (when exact task title matches)
  if (tasks.length > 0) {
    const titleMap = new Map<string, TaskEmbedInfo>()
    for (const t of tasks) {
      if (t.title && t.title.trim()) {
        titleMap.set(t.title.trim(), t)
      }
    }
    processed = processed.replace(/#([^\s#<>\[\]]+)/g, (match, rawTitle) => {
      const t = titleMap.get(rawTitle)
      if (!t) return match
      const st = t.status_id ? statusMap.get(t.status_id) : undefined
      const stColor = st?.color || '#3b82f6'
      const stName = escapeHtml(st?.name || 'ステータス未設定')
      const escapedTitle = escapeHtml(t.title)
      const escapedId = escapeHtml(t.id)
      const prog = t.progress ?? 0
      return `<a href="javascript:void(0)" class="wiki-task-pill" data-task-id="${escapedId}" style="display: inline-flex; align-items: center; gap: 6px; padding: 2px 8px; border-radius: 6px; background: rgba(59, 130, 246, 0.1); border: 1px solid rgba(59, 130, 246, 0.3); color: var(--text-main); text-decoration: none; font-size: 12px; font-weight: 500;"><span style="width: 8px; height: 8px; border-radius: 50%; background: ${stColor};"></span><strong style="color: var(--primary);">📋 ${escapedTitle}</strong><span style="font-size: 10px; color: var(--text-muted);">(${stName} ${prog}%)</span></a>`
    })
  }

  const rawHtml = marked.parse(processed, { async: false }) as string
  if (typeof window !== 'undefined' && DOMPurify?.sanitize) {
    return DOMPurify.sanitize(rawHtml, {
      ADD_ATTR: ['data-task-id', 'target']
    })
  }
  return rawHtml
}


export interface AutoScheduleUpdate {
  taskId: string
  planned_start_date: string
  planned_end_date: string
}

/**
 * Calculates cascading schedule updates for downstream successor tasks
 * when a predecessor task's planned end date changes.
 * Prevents cyclic loops with a visited set.
 */
export function calculateAutoScheduleUpdates(
  changedTaskId: string,
  newPlannedEndDate: string,
  dependencies: { predecessor_id: string; successor_id: string; dependency_type?: string; lag_days?: number }[],
  allTasks: { id: string; planned_start_date?: string | null; planned_end_date?: string | null }[]
): AutoScheduleUpdate[] {
  const taskMap = new Map<string, { id: string; planned_start_date?: string | null; planned_end_date?: string | null }>()
  for (const t of allTasks) {
    taskMap.set(t.id, t)
  }

  // Predecessor -> Successor dependencies map
  const succMap = new Map<string, typeof dependencies>()
  for (const d of dependencies) {
    if (!succMap.has(d.predecessor_id)) {
      succMap.set(d.predecessor_id, [])
    }
    succMap.get(d.predecessor_id)!.push(d)
  }

  const updates = new Map<string, AutoScheduleUpdate>()
  const visited = new Set<string>()

  function propagate(currentId: string, currentEndDate: string) {
    if (visited.has(currentId)) return
    visited.add(currentId)

    const deps = succMap.get(currentId) || []
    for (const dep of deps) {
      const succTask = taskMap.get(dep.successor_id)
      if (!succTask || !succTask.planned_start_date || !succTask.planned_end_date) continue

      const depType = dep.dependency_type || 'FS'
      if (depType === 'FS') {
        const minStartDate = addDaysYMD(currentEndDate, 1 + (dep.lag_days || 0))
        const existingUpdate = updates.get(succTask.id)
        const currentSuccStart = existingUpdate ? existingUpdate.planned_start_date : succTask.planned_start_date

        if (currentSuccStart < minStartDate) {
          const durationDays = Math.max(diffDaysYMD(succTask.planned_start_date, succTask.planned_end_date), 0)
          const newStart = minStartDate
          const newEnd = addDaysYMD(newStart, durationDays)

          updates.set(succTask.id, {
            taskId: succTask.id,
            planned_start_date: newStart,
            planned_end_date: newEnd
          })

          propagate(succTask.id, newEnd)
        }
      }
    }
    visited.delete(currentId)
  }

  propagate(changedTaskId, newPlannedEndDate)
  return Array.from(updates.values())
}

/**
 * Computes an SVG orthogonal routing path from predecessor bar end to successor bar start.
 */
export function calculateGanttDependencyPath(
  fromX: number,
  fromY: number,
  toX: number,
  toY: number
): string {
  if (toX >= fromX + 16) {
    const midX = Math.round(fromX + (toX - fromX) / 2)
    return `M ${fromX} ${fromY} L ${midX} ${fromY} L ${midX} ${toY} L ${toX} ${toY}`
  }

  const leadX = fromX + 12
  const midY = Math.round((fromY + toY) / 2)
  const backX = toX - 12
  return `M ${fromX} ${fromY} L ${leadX} ${fromY} L ${leadX} ${midY} L ${backX} ${midY} L ${backX} ${toY} L ${toX} ${toY}`
}

export interface WeekPeriod {
  start: string // YYYY-MM-DD
  end: string   // YYYY-MM-DD
  label: string
}

export interface TaskWorkloadContribution {
  taskId: string
  taskTitle: string
  issueTitle?: string
  totalPlannedHours: number
  allocatedHours: number
  plannedStartDate?: string | null
  plannedEndDate?: string | null
}

export interface MemberWeekWorkload {
  userId: string
  week: WeekPeriod
  capacityHours: number
  allocatedHours: number
  ratioPercent: number
  status: 'low' | 'optimal' | 'overload'
  contributions: TaskWorkloadContribution[]
}

/**
 * Generates an array of WeekPeriod starting from the Monday of a given reference date.
 */
export function generateWeeksList(referenceYMD: string, weekCount = 6): WeekPeriod[] {
  const parts = referenceYMD.split('-').map(Number)
  const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
  const dayOfWeek = dt.getUTCDay()
  // Monday is 1, Sunday is 0 -> offset to preceding Monday
  const diffToMonday = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
  dt.setUTCDate(dt.getUTCDate() + diffToMonday)

  const weeks: WeekPeriod[] = []
  for (let i = 0; i < weekCount; i++) {
    const y = dt.getUTCFullYear()
    const m = String(dt.getUTCMonth() + 1).padStart(2, '0')
    const d = String(dt.getUTCDate()).padStart(2, '0')
    const startYMD = `${y}-${m}-${d}`

    const endDt = new Date(dt.getTime())
    endDt.setUTCDate(endDt.getUTCDate() + 6)
    const ey = endDt.getUTCFullYear()
    const em = String(endDt.getUTCMonth() + 1).padStart(2, '0')
    const ed = String(endDt.getUTCDate()).padStart(2, '0')
    const endYMD = `${ey}-${em}-${ed}`

    weeks.push({
      start: startYMD,
      end: endYMD,
      label: `${parseInt(m, 10)}/${parseInt(d, 10)}〜`
    })

    dt.setUTCDate(dt.getUTCDate() + 7)
  }
  return weeks
}

/**
 * Calculates the number of working days (Mon-Fri) between two YYYY-MM-DD dates inclusive.
 */
export function countBusinessDays(startYMD: string, endYMD: string): number {
  if (startYMD > endYMD) return 0
  let count = 0
  let cur = startYMD
  while (cur <= endYMD) {
    const parts = cur.split('-').map(Number)
    const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
    const dow = dt.getUTCDay()
    if (dow !== 0 && dow !== 6) {
      count++
    }
    cur = addDaysYMD(cur, 1)
  }
  return count
}

/**
 * Computes member workloads across specified weeks.
 */
export function calculateMemberWorkloads(
  userIds: string[],
  tasks: {
    id: string
    title: string
    planned_hours?: number | null
    planned_start_date?: string | null
    planned_end_date?: string | null
    assignees?: { type: string; id: string }[]
  }[],
  weeks: WeekPeriod[],
  hoursPerDay = 8.0,
  taskParentMap?: Map<string, string>
): Map<string, MemberWeekWorkload[]> {
  const result = new Map<string, MemberWeekWorkload[]>()
  const capacityHours = Math.round(hoursPerDay * 5 * 100) / 100

  for (const uid of userIds) {
    const userWeeks: MemberWeekWorkload[] = []
    const userTasks = tasks.filter(t => t.assignees?.some(a => a.type === 'user' && a.id === uid))

    for (const wk of weeks) {
      let allocatedTotal = 0
      const contributions: TaskWorkloadContribution[] = []

      for (const t of userTasks) {
        if (!t.planned_start_date || !t.planned_end_date) continue
        const tStart = t.planned_start_date
        const tEnd = t.planned_end_date
        const pHours = Number(t.planned_hours) || 0
        if (pHours <= 0) continue

        // Check date overlap
        const overlapStart = tStart > wk.start ? tStart : wk.start
        const overlapEnd = tEnd < wk.end ? tEnd : wk.end
        if (overlapStart > overlapEnd) continue

        const totalBusinessDays = countBusinessDays(tStart, tEnd)
        const overlapBusinessDays = countBusinessDays(overlapStart, overlapEnd)

        let taskHoursForWeek = 0
        if (totalBusinessDays > 0) {
          taskHoursForWeek = (overlapBusinessDays / totalBusinessDays) * pHours
        } else {
          const totalDays = Math.max(diffDaysYMD(tStart, tEnd) + 1, 1)
          const overlapDays = Math.max(diffDaysYMD(overlapStart, overlapEnd) + 1, 0)
          taskHoursForWeek = (overlapDays / totalDays) * pHours
        }

        taskHoursForWeek = Math.round(taskHoursForWeek * 100) / 100
        if (taskHoursForWeek > 0) {
          allocatedTotal += taskHoursForWeek
          contributions.push({
            taskId: t.id,
            taskTitle: t.title,
            issueTitle: taskParentMap?.get(t.id),
            totalPlannedHours: pHours,
            allocatedHours: taskHoursForWeek,
            plannedStartDate: t.planned_start_date,
            plannedEndDate: t.planned_end_date
          })
        }
      }

      allocatedTotal = Math.round(allocatedTotal * 100) / 100
      const ratio = capacityHours > 0 ? (allocatedTotal / capacityHours) * 100 : 0
      let status: 'low' | 'optimal' | 'overload' = 'low'
      if (ratio > 100) {
        status = 'overload'
      } else if (ratio >= 80) {
        status = 'optimal'
      }

      userWeeks.push({
        userId: uid,
        week: wk,
        capacityHours,
        allocatedHours: allocatedTotal,
        ratioPercent: Math.round(ratio * 10) / 10,
        status,
        contributions
      })
    }

    result.set(uid, userWeeks)
  }

  return result
}

export interface ParsedInviteRow {
  email: string
  displayName: string
  domain: string
  isDomainAllowed: boolean
  isDuplicateInBatch: boolean
  isAlreadyRegistered: boolean
  isValidEmailFormat: boolean
}

/**
 * Parses multi-line invitation text or CSV/TSV data into structured invitation rows,
 * verifying against allowed domains and existing users.
 */
export function parseInvitationText(
  text: string,
  allowedDomains: string[] = [],
  existingEmails: Set<string> = new Set()
): ParsedInviteRow[] {
  if (!text || !text.trim()) return []

  const lines = text.split(/\r?\n/).map(l => l.trim()).filter(l => l.length > 0)
  if (lines.length === 0) return []

  const results: ParsedInviteRow[] = []
  const seenEmails = new Set<string>()

  // Skip header line if detected
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

    let parts: string[] = []
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

export interface ParsedDirectUserRow {
  email: string
  displayName: string
  password?: string
  passwordMode: 'custom' | 'batch_default' | 'auto_generate'
  domain: string
  isDomainAllowed: boolean
  isDuplicateInBatch: boolean
  isAlreadyRegistered: boolean
  isValidEmailFormat: boolean
}

export function parseDirectUserText(
  text: string,
  allowedDomains: string[] = [],
  existingEmails: Set<string> = new Set(),
  batchDefaultPassword = ''
): ParsedDirectUserRow[] {
  if (!text || !text.trim()) return []

  const lines = text.split(/\r?\n/).map(l => l.trim()).filter(l => l.length > 0)
  if (lines.length === 0) return []

  const results: ParsedDirectUserRow[] = []
  const seenEmails = new Set<string>()

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

    let parts: string[] = []
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

    let passwordMode: 'custom' | 'batch_default' | 'auto_generate' = 'auto_generate'
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

/**
 * Checks whether a status name represents a completed/done state.
 */
export function isDoneStatusName(name?: string | null): boolean {
  return _isDoneStatusName(name)
}

/**
 * Resolves the Set of status IDs considered "Done/Completed" for a project.
 */
export function resolveDoneStatusIds(
  statuses: { id: string; name: string; display_order: number }[] = []
): Set<string> {
  return _resolveDoneStatusIds(statuses)
}

/**
 * Determines whether a task is considered completed based on its progress or status.
 */
export function isTaskCompleted(
  task: { progress?: number | null; status_id?: string | null },
  statuses: { id: string; name: string; display_order: number }[] = []
): boolean {
  return _isTaskCompleted(task, statuses)
}

export interface SprintMetricsResult {
  taskCount: number
  completedTaskCount: number
  totalSP: number
  completedSP: number
  completionRate: number
  plannedHours: number
  actualHours: number
}

/**
 * Calculates sprint task and story point completion metrics.
 */
export function calculateSprintMetrics(
  tasks: {
    story_points?: number | null
    planned_hours?: number | null
    actual_hours?: number | null
    progress?: number | null
    status_id?: string | null
  }[],
  statuses: { id: string; name: string; display_order: number }[] = [],
  sprintObj: {
    task_count?: number | null
    completed_task_count?: number | null
    total_story_points?: number | null
    completed_story_points?: number | null
    total_planned_hours?: number | null
    total_actual_hours?: number | null
  } | null = null
): SprintMetricsResult {
  return _calculateSprintMetrics(tasks, statuses, sprintObj)
}

export interface SprintVelocityItem {
  id: string
  name: string
  status: string
  taskCount: number
  completedTaskCount: number
  plannedSP: number
  completedSP: number
  completionRate: number
}

export interface VelocityStatsResult {
  sprints: SprintVelocityItem[]
  sprintStats: SprintVelocityItem[]
  averageVelocity: number
  recentVelocity: number
  overallCompletionRate: number
}

/**
 * Calculates per-sprint velocity breakdown and average velocity across sprints.
 */
export function calculateVelocityStats(
  sprints: {
    id: string
    name: string
    status: string
    task_count?: number | null
    completed_task_count?: number | null
    total_story_points?: number | null
    completed_story_points?: number | null
    total_planned_hours?: number | null
    total_actual_hours?: number | null
  }[],
  allTasks: {
    sprint_id?: string | null
    story_points?: number | null
    planned_hours?: number | null
    actual_hours?: number | null
    progress?: number | null
    status_id?: string | null
  }[],
  statuses: { id: string; name: string; display_order: number }[] = []
): VelocityStatsResult {
  return _calculateVelocityStats(sprints, allTasks, statuses)
}

export interface GanttHeaderGroup {
  key: string
  label: string
  startYMD: string
  endYMD: string
  count: number
  widthPx: number
  isWeekend?: boolean
}

export interface GanttScaleHeaderResult {
  scale: 'day' | 'week' | 'month'
  baseDayWidth: number
  defaultDayCount: number
  upperGroups: GanttHeaderGroup[]
  lowerGroups: GanttHeaderGroup[]
  totalWidthPx: number
}

/**
 * Returns the default baseDayWidth and day count for each Gantt scale.
 */
export function getGanttScaleConfig(scale: 'day' | 'week' | 'month'): { baseDayWidth: number; count: number } {
  return _getGanttScaleConfig(scale)
}

/**
 * Builds upper and lower tier header groups for the Gantt chart according to scale ('day' | 'week' | 'month').
 */
export function buildGanttHeaderGroups(
  timelineDays: string[],
  scale: 'day' | 'week' | 'month',
  dayWidth?: number
): GanttScaleHeaderResult {
  return _buildGanttHeaderGroups(timelineDays, scale, dayWidth)
}




