/**
 * Pure Agile/Scrum and Gantt helper functions shared between the Nuxt app and Node unit tests.
 */

/**
 * Formats a Date object (or date-like value) to local YYYY-MM-DD using local calendar getters.
 * Never uses UTC toISOString() to avoid timezone date shifts.
 */
export function formatLocalYMD(date = new Date()) {
  const d = date instanceof Date ? date : new Date(date)
  if (Number.isNaN(d.getTime())) {
    const now = new Date()
    const y = now.getFullYear()
    const m = String(now.getMonth() + 1).padStart(2, '0')
    const day = String(now.getDate()).padStart(2, '0')
    return `${y}-${m}-${day}`
  }
  const year = d.getFullYear()
  const month = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

/**
 * Checks whether a status name represents a completed/done state.
 * Matches '完了', 'クローズ', 'done', 'closed', or exact 'close'.
 * Does NOT match words like 'disclosed' or 'enclosure'.
 */
export function isDoneStatusName(name) {
  if (!name) return false
  const lower = String(name).trim().toLowerCase()
  if (!lower) return false
  if (lower.includes('完了') || lower.includes('クローズ')) return true
  return /\b(done|closed|close)\b/i.test(lower)
}

/**
 * Resolves the Set of status IDs considered "Done/Completed" for a project.
 * 1. First pass: collect status IDs where isDoneStatusName(st.name) is true.
 * 2. If doneIds.size > 0, return doneIds immediately (prevents post-Done columns like 'Archive' from being treated as the only done column, or non-done columns when an explicit Done column exists).
 * 3. Fallback pass (only if doneIds.size === 0 and statuses.length >= 2): find minOrder and maxOrder; if maxOrder > minOrder, add statuses with display_order === maxOrder.
 */
export function resolveDoneStatusIds(statuses = []) {
  const doneIds = new Set()
  if (!Array.isArray(statuses) || statuses.length === 0) {
    return doneIds
  }

  for (const st of statuses) {
    if (st && st.id && isDoneStatusName(st.name)) {
      doneIds.add(st.id)
    }
  }

  if (doneIds.size > 0) {
    return doneIds
  }

  if (statuses.length >= 2) {
    const orders = statuses.map(s => Number(s?.display_order) || 0)
    const minOrder = Math.min(...orders)
    const maxOrder = Math.max(...orders)
    if (maxOrder > minOrder) {
      for (const st of statuses) {
        if (st && st.id && (Number(st.display_order) || 0) === maxOrder) {
          doneIds.add(st.id)
        }
      }
    }
  }

  return doneIds
}

/**
 * Determines whether a task is considered completed based on its progress or status.
 */
export function isTaskCompleted(task, statuses = []) {
  if (!task) return false
  if ((task.progress ?? 0) >= 100) return true
  if (!task.status_id) return false
  return resolveDoneStatusIds(statuses).has(task.status_id)
}

/**
 * Calculates sprint task and story point completion metrics.
 * When tasks is empty (or fewer than sprintObj.task_count) and sprintObj has authoritative rollup fields,
 * falls back to sprintObj metrics.
 */
export function calculateSprintMetrics(tasks, statuses = [], sprintObj = null) {
  const taskList = Array.isArray(tasks) ? tasks : []
  const sprintTaskCount = Number(sprintObj?.task_count) || 0
  const hasSprintAuthoritative =
    sprintObj !== null &&
    sprintObj !== undefined &&
    (sprintTaskCount > 0 ||
      Number(sprintObj.total_story_points) > 0 ||
      Number(sprintObj.completed_story_points) > 0)

  if (hasSprintAuthoritative && (taskList.length === 0 || (sprintTaskCount > 0 && taskList.length < sprintTaskCount))) {
    const taskCount = sprintTaskCount
    const completedTaskCount = Number(sprintObj.completed_task_count) || 0
    const totalSP = Math.round((Number(sprintObj.total_story_points) || 0) * 10) / 10
    const completedSP = Math.round((Number(sprintObj.completed_story_points) || 0) * 10) / 10
    const plannedHours = Math.round((Number(sprintObj.total_planned_hours) || 0) * 100) / 100
    const actualHours = Math.round((Number(sprintObj.total_actual_hours) || 0) * 100) / 100
    const completionRate = totalSP > 0 ? Math.round((completedSP / totalSP) * 1000) / 10 : 0
    return {
      taskCount,
      completedTaskCount,
      totalSP,
      completedSP,
      completionRate,
      plannedHours,
      actualHours
    }
  }

  let taskCount = 0
  let completedTaskCount = 0
  let totalSP = 0
  let completedSP = 0
  let plannedHours = 0
  let actualHours = 0

  const doneIds = resolveDoneStatusIds(statuses)

  for (const t of taskList) {
    taskCount++
    const sp = Number(t?.story_points) || 0
    const pHours = Number(t?.planned_hours) || 0
    const aHours = Number(t?.actual_hours) || 0
    totalSP += sp
    plannedHours += pHours
    actualHours += aHours

    const isDone = (t?.progress ?? 0) >= 100 || (Boolean(t?.status_id) && doneIds.has(t.status_id))
    if (isDone) {
      completedTaskCount++
      completedSP += sp
    }
  }

  totalSP = Math.round(totalSP * 10) / 10
  completedSP = Math.round(completedSP * 10) / 10
  plannedHours = Math.round(plannedHours * 100) / 100
  actualHours = Math.round(actualHours * 100) / 100

  const completionRate = totalSP > 0 ? Math.round((completedSP / totalSP) * 1000) / 10 : 0

  return {
    taskCount,
    completedTaskCount,
    totalSP,
    completedSP,
    completionRate,
    plannedHours,
    actualHours
  }
}

/**
 * Calculates per-sprint velocity breakdown and average velocity across sprints.
 * Uses tasks for each sprint if present and complete in allTasks, otherwise falls back
 * to authoritative Sprint API fields (total_story_points, completed_story_points, etc.).
 */
export function calculateVelocityStats(sprints, allTasks, statuses = []) {
  const taskList = Array.isArray(allTasks) ? allTasks : []
  const items = (sprints || []).map((sp) => {
    const spTasks = taskList.filter(t => t && t.sprint_id === sp.id)
    const expectedCount = Number(sp.task_count) || 0
    const canComputeFromTasks = spTasks.length > 0 && spTasks.length >= expectedCount && taskList.length >= expectedCount

    let m
    if (canComputeFromTasks) {
      m = calculateSprintMetrics(spTasks, statuses, null)
    } else {
      m = calculateSprintMetrics([], statuses, sp)
    }

    return {
      id: sp.id,
      name: sp.name,
      status: sp.status,
      taskCount: m.taskCount,
      completedTaskCount: m.completedTaskCount,
      plannedSP: m.totalSP,
      completedSP: m.completedSP,
      completionRate: m.completionRate
    }
  })

  const closedItems = items.filter(i => i.status === 'closed')
  const activeAndClosedItems = items.filter(i => i.status === 'closed' || i.status === 'active')
  const basisItems = closedItems.length > 0 ? closedItems : activeAndClosedItems

  let averageVelocity = 0
  if (basisItems.length > 0) {
    const sumCompleted = basisItems.reduce((acc, i) => acc + i.completedSP, 0)
    averageVelocity = Math.round((sumCompleted / basisItems.length) * 10) / 10
  }

  const recentTarget = closedItems.length > 0
    ? closedItems[closedItems.length - 1]
    : activeAndClosedItems.length > 0
      ? activeAndClosedItems[activeAndClosedItems.length - 1]
      : items[items.length - 1]
  const recentVelocity = recentTarget ? recentTarget.completedSP : 0

  const totalPlannedAll = items.reduce((acc, i) => acc + i.plannedSP, 0)
  const totalCompletedAll = items.reduce((acc, i) => acc + i.completedSP, 0)
  const overallCompletionRate = totalPlannedAll > 0
    ? Math.round((totalCompletedAll / totalPlannedAll) * 1000) / 10
    : 0

  return {
    sprints: items,
    sprintStats: items,
    averageVelocity,
    recentVelocity,
    overallCompletionRate
  }
}

/**
 * Returns the default baseDayWidth and day count for each Gantt scale.
 */
export function getGanttScaleConfig(scale) {
  if (scale === 'day') return { baseDayWidth: 48, count: 30 }
  if (scale === 'week') return { baseDayWidth: 16, count: 91 }
  return { baseDayWidth: 6, count: 180 }
}

/**
 * Builds upper and lower tier header groups for the Gantt chart according to scale ('day' | 'week' | 'month').
 */
export function buildGanttHeaderGroups(timelineDays, scale, dayWidth) {
  const cfg = getGanttScaleConfig(scale)
  const effectiveDayWidth = dayWidth !== undefined && dayWidth > 0 ? dayWidth : cfg.baseDayWidth
  const upperGroups = []
  const lowerGroups = []

  const weekdays = ['日', '月', '火', '水', '木', '金', '土']

  for (const ymd of timelineDays || []) {
    const parts = ymd.split('-').map(Number)
    const year = parts[0]
    const month = parts[1]
    const day = parts[2]
    const dt = new Date(Date.UTC(year, month - 1, day))
    const dow = dt.getUTCDay()

    // Upper tier grouping
    const upperLabel = scale === 'month' ? `${year}年` : `${year}年 ${month}月`
    const lastUpper = upperGroups[upperGroups.length - 1]
    if (lastUpper && lastUpper.label === upperLabel) {
      lastUpper.count++
      lastUpper.endYMD = ymd
      lastUpper.widthPx = lastUpper.count * effectiveDayWidth
    } else {
      upperGroups.push({
        key: `upper-${ymd}`,
        label: upperLabel,
        startYMD: ymd,
        endYMD: ymd,
        count: 1,
        widthPx: effectiveDayWidth
      })
    }

    // Lower tier grouping
    if (scale === 'day') {
      lowerGroups.push({
        key: `day-${ymd}`,
        label: `${String(day).padStart(2, '0')} (${weekdays[dow]})`,
        startYMD: ymd,
        endYMD: ymd,
        count: 1,
        widthPx: effectiveDayWidth,
        isWeekend: dow === 0 || dow === 6
      })
    } else if (scale === 'week') {
      const lastLower = lowerGroups[lowerGroups.length - 1]
      // Start a new weekly bucket on Monday (dow === 1) or if first element or bucket reached 7 days
      if (!lastLower || dow === 1 || lastLower.count >= 7) {
        lowerGroups.push({
          key: `week-${ymd}`,
          label: `${month}/${day}〜`,
          startYMD: ymd,
          endYMD: ymd,
          count: 1,
          widthPx: effectiveDayWidth
        })
      } else {
        lastLower.count++
        lastLower.endYMD = ymd
        lastLower.widthPx = lastLower.count * effectiveDayWidth
      }
    } else {
      // scale === 'month': monthly buckets
      const monthKey = `${year}-${String(month).padStart(2, '0')}`
      const monthLabel = `${month}月`
      const lastLower = lowerGroups[lowerGroups.length - 1]
      if (lastLower && lastLower.key === `month-${monthKey}`) {
        lastLower.count++
        lastLower.endYMD = ymd
        lastLower.widthPx = lastLower.count * effectiveDayWidth
      } else {
        lowerGroups.push({
          key: `month-${monthKey}`,
          label: monthLabel,
          startYMD: ymd,
          endYMD: ymd,
          count: 1,
          widthPx: effectiveDayWidth
        })
      }
    }
  }

  return {
    scale,
    baseDayWidth: cfg.baseDayWidth,
    defaultDayCount: cfg.count,
    upperGroups,
    lowerGroups,
    totalWidthPx: (timelineDays?.length || 0) * effectiveDayWidth
  }
}
