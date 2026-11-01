import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  isDoneStatusName,
  resolveDoneStatusIds,
  isTaskCompleted,
  calculateSprintMetrics,
  calculateVelocityStats,
  getGanttScaleConfig,
  buildGanttHeaderGroups,
  formatLocalYMD
} from '../utils/agile_gantt.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

function addDaysYMD(ymd, days) {
  const parts = ymd.split('-').map(Number)
  const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
  dt.setUTCDate(dt.getUTCDate() + days)
  const y = dt.getUTCFullYear()
  const m = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const d = String(dt.getUTCDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function generateDateRangeYMD(startYMD, count) {
  const list = []
  for (let i = 0; i < count; i++) {
    list.push(addDaysYMD(startYMD, i))
  }
  return list
}

test('isDoneStatusName, resolveDoneStatusIds, and isTaskCompleted evaluate progress, status name, and fallback display_order safely', () => {
  assert.equal(isDoneStatusName('完了'), true)
  assert.equal(isDoneStatusName('クローズ'), true)
  assert.equal(isDoneStatusName('Done'), true)
  assert.equal(isDoneStatusName('Closed'), true)
  assert.equal(isDoneStatusName('close'), true)
  assert.equal(isDoneStatusName('  Close  '), true)
  assert.equal(isDoneStatusName('disclosed'), false)
  assert.equal(isDoneStatusName('enclosure'), false)
  assert.equal(isDoneStatusName('進行中'), false)
  assert.equal(isDoneStatusName(null), false)
  assert.equal(isDoneStatusName(''), false)

  // Single-status project: should NOT treat the single status as done unless its name matches Done
  const singleStatus = [{ id: 'only1', name: 'Open', display_order: 1 }]
  assert.equal(resolveDoneStatusIds(singleStatus).size, 0)
  assert.equal(isTaskCompleted({ progress: 50, status_id: 'only1' }, singleStatus), false)
  assert.equal(isTaskCompleted({ progress: 100, status_id: 'only1' }, singleStatus), true)

  // Multi-status project with no explicit Done name -> falls back to max display_order
  const statuses = [
    { id: 's1', name: '未着手', display_order: 1 },
    { id: 's2', name: '進行中', display_order: 2 },
    { id: 's3', name: 'レビュー済', display_order: 3 }
  ]
  assert.deepEqual([...resolveDoneStatusIds(statuses)], ['s3'])

  // 1. Completed by progress === 100
  assert.equal(isTaskCompleted({ progress: 100, status_id: 's1' }, statuses), true)

  // 2. '完了' + 'Archive' columns: only '完了' is resolved as done, NOT 'Archive'
  const customStatuses = [
    { id: 'c1', name: 'Open', display_order: 1 },
    { id: 'c2', name: '完了 (Done)', display_order: 2 },
    { id: 'c3', name: 'Archive', display_order: 3 }
  ]
  assert.deepEqual([...resolveDoneStatusIds(customStatuses)], ['c2'])
  assert.equal(isTaskCompleted({ progress: 50, status_id: 'c2' }, customStatuses), true)
  assert.equal(isTaskCompleted({ progress: 50, status_id: 'c3' }, customStatuses), false)

  // 3. Completed by max display_order status when no explicit done status name exists and statuses.length >= 2
  assert.equal(isTaskCompleted({ progress: 80, status_id: 's3' }, statuses), true)

  // 4. Incomplete task
  assert.equal(isTaskCompleted({ progress: 40, status_id: 's2' }, statuses), false)
})

test('calculateSprintMetrics computes planned SP, completed SP, completion rate, and falls back to authoritative sprintObj', () => {
  const statuses = [
    { id: 's1', name: 'Todo', display_order: 1 },
    { id: 's2', name: 'In Progress', display_order: 2 },
    { id: 's3', name: 'Done', display_order: 3 }
  ]

  const tasks = [
    { id: 't1', story_points: 5, planned_hours: 10, actual_hours: 8, progress: 100, status_id: 's3' },
    { id: 't2', story_points: 3, planned_hours: 6, actual_hours: 5, progress: 20, status_id: 's3' }, // done by status
    { id: 't3', story_points: 8, planned_hours: 16, actual_hours: 4, progress: 50, status_id: 's2' }, // incomplete
    { id: 't4', story_points: null, planned_hours: 4, actual_hours: 0, progress: 0, status_id: 's1' } // null SP
  ]

  const metrics = calculateSprintMetrics(tasks, statuses)
  assert.equal(metrics.taskCount, 4)
  assert.equal(metrics.completedTaskCount, 2)
  assert.equal(metrics.totalSP, 16)
  assert.equal(metrics.completedSP, 8)
  assert.equal(metrics.completionRate, 50)
  assert.equal(metrics.plannedHours, 36)
  assert.equal(metrics.actualHours, 17)

  // Edge case: 0 tasks without sprintObj
  const emptyMetrics = calculateSprintMetrics([], statuses)
  assert.equal(emptyMetrics.taskCount, 0)
  assert.equal(emptyMetrics.completedTaskCount, 0)
  assert.equal(emptyMetrics.totalSP, 0)
  assert.equal(emptyMetrics.completedSP, 0)
  assert.equal(emptyMetrics.completionRate, 0)

  // Fallback to authoritative sprintObj when tasks is empty
  const fallbackMetrics = calculateSprintMetrics([], statuses, {
    id: 'sp-auth',
    task_count: 5,
    completed_task_count: 3,
    total_story_points: 20,
    completed_story_points: 15,
    total_planned_hours: 40,
    total_actual_hours: 28
  })
  assert.equal(fallbackMetrics.taskCount, 5)
  assert.equal(fallbackMetrics.completedTaskCount, 3)
  assert.equal(fallbackMetrics.totalSP, 20)
  assert.equal(fallbackMetrics.completedSP, 15)
  assert.equal(fallbackMetrics.completionRate, 75)
  assert.equal(fallbackMetrics.plannedHours, 40)
  assert.equal(fallbackMetrics.actualHours, 28)

  // Edge case: tasks with null SP only
  const nullSpMetrics = calculateSprintMetrics([{ id: 't5', story_points: null, progress: 100, status_id: 's3' }], statuses)
  assert.equal(nullSpMetrics.totalSP, 0)
  assert.equal(nullSpMetrics.completedSP, 0)
  assert.equal(nullSpMetrics.completionRate, 0)
})

test('calculateVelocityStats computes per-sprint velocity and uses authoritative Sprint fields when allTasks is filtered or empty', () => {
  const statuses = [
    { id: 's1', name: 'Todo', display_order: 1 },
    { id: 's2', name: 'Done', display_order: 2 }
  ]

  const sprints = [
    { id: 'sp1', name: 'Sprint 1', status: 'closed', task_count: 2, completed_task_count: 2, total_story_points: 15, completed_story_points: 15 },
    { id: 'sp2', name: 'Sprint 2', status: 'closed', task_count: 2, completed_task_count: 1, total_story_points: 20, completed_story_points: 12 },
    { id: 'sp3', name: 'Sprint 3', status: 'active', task_count: 1, completed_task_count: 1, total_story_points: 8, completed_story_points: 8 },
    { id: 'sp4', name: 'Sprint 4', status: 'planned', task_count: 0, completed_task_count: 0, total_story_points: 0, completed_story_points: 0 }
  ]

  const allTasks = [
    { id: 't1', sprint_id: 'sp1', story_points: 10, progress: 100, status_id: 's2' },
    { id: 't2', sprint_id: 'sp1', story_points: 5, progress: 100, status_id: 's2' }, // sp1 completed = 15
    { id: 't3', sprint_id: 'sp2', story_points: 12, progress: 100, status_id: 's2' },
    { id: 't4', sprint_id: 'sp2', story_points: 8, progress: 30, status_id: 's1' },  // sp2 completed = 12, planned = 20
    { id: 't5', sprint_id: 'sp3', story_points: 8, progress: 100, status_id: 's2' }  // sp3 completed = 8 (active)
  ]

  const stats = calculateVelocityStats(sprints, allTasks, statuses)
  assert.equal(stats.sprints.length, 4)
  assert.equal(stats.sprints[0].completedSP, 15)
  assert.equal(stats.sprints[1].completedSP, 12)
  assert.equal(stats.sprints[1].plannedSP, 20)
  // Average across closed sprints: (15 + 12) / 2 = 13.5
  assert.equal(stats.averageVelocity, 13.5)
  assert.equal(stats.recentVelocity, 12)

  // Authoritative fallback when allTasks is empty or filtered (e.g. closed issues excluded)
  const filteredStats = calculateVelocityStats(sprints, [], statuses)
  assert.equal(filteredStats.sprints[0].plannedSP, 15)
  assert.equal(filteredStats.sprints[0].completedSP, 15)
  assert.equal(filteredStats.sprints[1].plannedSP, 20)
  assert.equal(filteredStats.sprints[1].completedSP, 12)
  assert.equal(filteredStats.averageVelocity, 13.5)
  assert.equal(filteredStats.recentVelocity, 12)

  // Fallback when no closed sprints exist yet (uses active sprint)
  const activeOnlyStats = calculateVelocityStats([sprints[2], sprints[3]], allTasks, statuses)
  assert.equal(activeOnlyStats.averageVelocity, 8)
})

test('formatLocalYMD formats local calendar dates without UTC shifts', () => {
  const sample = new Date(2026, 10, 5, 23, 55, 0) // Local Nov 5, 2026
  assert.equal(formatLocalYMD(sample), '2026-11-05')
})

test('buildGanttHeaderGroups differentiates day, week, and month scales with exact width sums', () => {
  // 1. Day scale: 30 days, 48px
  const dayCfg = getGanttScaleConfig('day')
  assert.equal(dayCfg.count, 30)
  assert.equal(dayCfg.baseDayWidth, 48)
  const days30 = generateDateRangeYMD('2026-10-25', dayCfg.count)
  const dayGroups = buildGanttHeaderGroups(days30, 'day', dayCfg.baseDayWidth)
  assert.equal(dayGroups.lowerGroups.length, 30)
  assert.equal(dayGroups.upperGroups.length, 2) // Oct and Nov 2026
  const dayUpperSum = dayGroups.upperGroups.reduce((acc, g) => acc + g.widthPx, 0)
  const dayLowerSum = dayGroups.lowerGroups.reduce((acc, g) => acc + g.widthPx, 0)
  assert.equal(dayUpperSum, 30 * 48)
  assert.equal(dayLowerSum, 30 * 48)

  // 2. Week scale: 91 days, 16px
  const weekCfg = getGanttScaleConfig('week')
  assert.equal(weekCfg.count, 91)
  assert.equal(weekCfg.baseDayWidth, 16)
  const days91 = generateDateRangeYMD('2026-11-02', weekCfg.count) // starts Monday
  const weekGroups = buildGanttHeaderGroups(days91, 'week', weekCfg.baseDayWidth)
  assert.equal(weekGroups.lowerGroups.length, 13) // 91 / 7 = 13 weeks
  assert.equal(weekGroups.lowerGroups[0].widthPx, 7 * 16)
  const weekUpperSum = weekGroups.upperGroups.reduce((acc, g) => acc + g.widthPx, 0)
  const weekLowerSum = weekGroups.lowerGroups.reduce((acc, g) => acc + g.widthPx, 0)
  assert.equal(weekUpperSum, 91 * 16)
  assert.equal(weekLowerSum, 91 * 16)

  // 3. Month scale: 180 days, 6px
  const monthCfg = getGanttScaleConfig('month')
  assert.equal(monthCfg.count, 180)
  assert.equal(monthCfg.baseDayWidth, 6)
  const days180 = generateDateRangeYMD('2026-11-01', monthCfg.count)
  const monthGroups = buildGanttHeaderGroups(days180, 'month', monthCfg.baseDayWidth)
  // Upper tier in month scale is grouped by Year (2026年, 2027年)
  assert.equal(monthGroups.upperGroups.length, 2)
  assert.equal(monthGroups.upperGroups[0].label, '2026年')
  assert.equal(monthGroups.upperGroups[1].label, '2027年')
  // Lower tier in month scale is grouped by Month (11月, 12月, 1月, ...)
  assert.ok(monthGroups.lowerGroups.length >= 6 && monthGroups.lowerGroups.length <= 7)
  const monthUpperSum = monthGroups.upperGroups.reduce((acc, g) => acc + g.widthPx, 0)
  const monthLowerSum = monthGroups.lowerGroups.reduce((acc, g) => acc + g.widthPx, 0)
  assert.equal(monthUpperSum, 180 * 6)
  assert.equal(monthLowerSum, 180 * 6)
})

test('Source files contain required Agile/Scrum and UX enhancements', () => {
  const formatSrc = fs.readFileSync(path.join(__dirname, '../utils/format.ts'), 'utf8')
  assert.ok(formatSrc.includes('export function isDoneStatusName'))
  assert.ok(formatSrc.includes('export function isTaskCompleted'))
  assert.ok(formatSrc.includes('export function calculateSprintMetrics'))
  assert.ok(formatSrc.includes('export function calculateVelocityStats'))
  assert.ok(formatSrc.includes('export function buildGanttHeaderGroups'))

  const modalSrc = fs.readFileSync(path.join(__dirname, '../components/ItemDetailModal.vue'), 'utf8')
  assert.ok(modalSrc.includes('premortem'))
  assert.ok(modalSrc.includes('postmortem'))
  assert.ok(modalSrc.includes('exportPostmortemToWiki'))
  assert.ok(modalSrc.includes('未保存の変更があります'))

  const sprintSrc = fs.readFileSync(path.join(__dirname, '../components/SprintBoardView.vue'), 'utf8')
  assert.ok(sprintSrc.includes('velocity'))
  assert.ok(sprintSrc.includes('rolloverModalOpen'))
  assert.ok(sprintSrc.includes('updateTaskStoryPoints'))

  const projectPageSrc = fs.readFileSync(path.join(__dirname, '../pages/projects/[id].vue'), 'utf8')
  assert.ok(projectPageSrc.includes('buildGanttHeaderGroups'))
  assert.ok(projectPageSrc.includes('sprintsWithDates'))
  assert.ok(projectPageSrc.includes('filters.sprint_id'))
  assert.ok(projectPageSrc.includes('allProjectIssues'))
})
