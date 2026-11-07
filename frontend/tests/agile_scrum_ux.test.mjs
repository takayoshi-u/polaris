import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import {
  addDaysYMD,
  isDoneStatusName,
  resolveDoneStatusIds,
  isTaskCompleted,
  calculateSprintMetrics,
  calculateVelocityStats,
  getGanttScaleConfig,
  buildGanttHeaderGroups,
  formatLocalYMD,
  calculateDefaultSprintDates,
  calculateSprintEndDateFromStart
} from '../utils/agile_gantt.mjs'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

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

test('calculateDefaultSprintDates and calculateSprintEndDateFromStart compute next-sprint dates and respect custom duration', () => {
  // 1. First sprint (no existing sprints -> starts todayYMD, ends todayYMD + 13d for 14-day duration, Sprint 1)
  const firstDefaults = calculateDefaultSprintDates([], 14, '2026-11-01')
  assert.equal(firstDefaults.start_date, '2026-11-01')
  assert.equal(firstDefaults.end_date, '2026-11-14')
  assert.equal(firstDefaults.suggested_name, 'Sprint 1')
  assert.equal(firstDefaults.duration_days, 14)

  // 2. Subsequent sprint (existing sprints with end_date: '2026-11-14' and '2026-11-05' -> starts '2026-11-15', ends '2026-11-28', Sprint 3)
  const existingSprints = [
    { id: 'sp1', name: 'Sprint 1', start_date: '2026-10-23', end_date: '2026-11-05' },
    { id: 'sp2', name: 'Sprint 2', start_date: '2026-11-01', end_date: '2026-11-14' }
  ]
  const nextDefaults = calculateDefaultSprintDates(existingSprints, 14, '2026-11-01')
  assert.equal(nextDefaults.start_date, '2026-11-15')
  assert.equal(nextDefaults.end_date, '2026-11-28')
  assert.equal(nextDefaults.suggested_name, 'Sprint 3')

  // 3. Custom duration: 7 days -> start + 6d, 21 days -> start + 20d
  const sevenDayDefaults = calculateDefaultSprintDates(existingSprints, 7, '2026-11-01')
  assert.equal(sevenDayDefaults.start_date, '2026-11-15')
  assert.equal(sevenDayDefaults.end_date, '2026-11-21')
  assert.equal(sevenDayDefaults.duration_days, 7)

  assert.equal(calculateSprintEndDateFromStart('2026-11-15', 7), '2026-11-21')
  assert.equal(calculateSprintEndDateFromStart('2026-11-15', 21), '2026-12-05')
  assert.equal(calculateSprintEndDateFromStart('', 14), '')
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

test('sprint_duration_days clamps invalid values (1..90, fallback 14)', () => {
  const end = (d) => calculateSprintEndDateFromStart('2026-01-01', d)
  assert.equal(end(0), '2026-01-14')
  assert.equal(end(NaN), '2026-01-14')
  assert.equal(end(null), '2026-01-14')
  assert.equal(end(undefined), '2026-01-14')
  assert.equal(end(999), '2026-03-31')
  assert.equal(end(-5), '2026-01-01')
  assert.equal(calculateDefaultSprintDates([], 999, '2026-01-01').duration_days, 90)
  assert.equal(calculateDefaultSprintDates([], NaN, '2026-01-01').duration_days, 14)
  assert.equal(calculateDefaultSprintDates([], null, '2026-01-01').end_date, '2026-01-14')
})

test('sprints with null/invalid end_date are ignored, falling back to today', () => {
  const r = calculateDefaultSprintDates(
    [{ name: 'x', end_date: null }, { name: 'y', end_date: 'garbage' }, { name: 'z', end_date: '2026-02-31' }, null],
    7, '2026-05-10')
  assert.equal(r.start_date, '2026-05-10')
  assert.equal(r.end_date, '2026-05-16')
  assert.doesNotMatch(JSON.stringify(r), /NaN|undefined/)
})

test('suggested sprint name goes above existing max number', () => {
  const r = calculateDefaultSprintDates([{ name: 'Sprint 5', end_date: '2026-01-10' }], 14, '2026-01-01')
  assert.equal(r.suggested_name, 'Sprint 6')
  assert.equal(r.start_date, '2026-01-11')
})

test('calculateSprintEndDateFromStart is safe for malformed dates', () => {
  for (const bad of ['abc', '2026-1-1', '2026-13-45', '2026-02-31', '', null, undefined, 123, {}]) {
    let out
    assert.doesNotThrow(() => { out = calculateSprintEndDateFromStart(bad, 14) })
    assert.equal(out, '')
  }
})

test('project page sends sprint_duration_days and passes it to SprintBoardView', () => {
  const src = fs.readFileSync(path.join(__dirname, '../pages/projects/[id].vue'), 'utf8')
  assert.match(src, /const projForm = reactive\(\{[^}]*sprint_duration_days:\s*14/s)
  assert.match(src, /projForm\.sprint_duration_days\s*=\s*p\.sprint_duration_days/)
  assert.match(src, /v-model\.number="projForm\.sprint_duration_days"/)
  assert.match(src, /body:\s*JSON\.stringify\(projForm\)/)
  assert.match(src, /:sprint-duration-days="project\?\.sprint_duration_days \|\| 14"/)
})

test('ItemDetailModal unifies title display and inline edit in header, eliminating duplicate template input', () => {
  const modalSrc = fs.readFileSync(path.join(__dirname, '../components/ItemDetailModal.vue'), 'utf8')
  const cssSrc = fs.readFileSync(path.join(__dirname, '../assets/css/main.css'), 'utf8')

  // Header inline title input when editable (based on isKeyEditable('title') and canEditAny)
  assert.ok(modalSrc.includes('ref="titleInputRef"'), 'titleInputRef must be defined on header title input')
  assert.ok(modalSrc.includes('v-model="form.title"'), 'form.title must be bound via v-model in header')
  assert.ok(modalSrc.includes('unified-title-input'), 'unified-title-input class must be used')
  assert.ok(modalSrc.includes('placeholder="タイトルを入力..."'), 'Header title input must have proper placeholder')
  assert.ok(modalSrc.includes("isKeyEditable('title')"), 'isKeyEditable title check must be present')
  assert.ok(modalSrc.includes('unified-title-heading'), 'unified-title-heading must be used for view-only state')

  // Template grid avoids duplicate text input, contains NO title notice or slot, and collapses empty rows
  assert.ok(
    !modalSrc.includes('title-field-unified-notice'),
    'Template grid must NOT contain title notice class'
  )
  assert.ok(
    !modalSrc.includes('※ タイトルは最上部のヘッダーで直接編集できます'),
    'Template grid must NOT contain redundant title redirection notice'
  )
  assert.ok(
    !modalSrc.includes("v-else-if=\"field.field_source === 'standard' && field.field_key === 'title'\""),
    'Duplicate title slot/field in template grid must be completely removed'
  )
  assert.ok(
    modalSrc.includes('hasVisibleFields(row)'),
    'Row loop must check hasVisibleFields to skip empty row containers'
  )
  assert.ok(
    modalSrc.includes("!(field.field_source === 'standard' && field.field_key === 'title')"),
    'Field loop must skip any standard title field slot'
  )

  // Styling for unified title input
  assert.ok(cssSrc.includes('.unified-title-input'), 'main.css must contain .unified-title-input styling')
  assert.ok(cssSrc.includes('.unified-title-input:hover'), 'main.css must contain hover styling')
  assert.ok(cssSrc.includes('.unified-title-input:focus'), 'main.css must contain focus styling')
})

test('WikiView implements data loss prevention, Ctrl+S shortcut, and zero-pages empty state CTA', () => {
  const wikiSrc = fs.readFileSync(path.join(__dirname, '../components/WikiView.vue'), 'utf8')

  // Data loss prevention
  assert.ok(wikiSrc.includes('const isDirty = computed'), 'WikiView must define isDirty computed state')
  assert.ok(wikiSrc.includes('initialSnapshot'), 'WikiView must use initialSnapshot for dirty comparison')
  assert.ok(wikiSrc.includes("addEventListener('beforeunload'"), 'WikiView must attach beforeunload event listener')
  assert.ok(wikiSrc.includes("removeEventListener('beforeunload'"), 'WikiView must remove beforeunload event listener on unmount')
  assert.ok(wikiSrc.includes('onBeforeRouteLeave'), 'WikiView must guard route leave on uncommitted changes')

  // Ctrl+S / Cmd+S shortcut
  assert.ok(wikiSrc.includes("e.key.toLowerCase() === 's'"), 'WikiView must detect Ctrl+S / Cmd+S shortcut')
  assert.ok(wikiSrc.includes('savePage()'), 'WikiView shortcut must invoke savePage()')

  // Empty state when 0 wiki pages
  assert.ok(wikiSrc.includes('wiki-empty-state'), 'WikiView must render wiki-empty-state container when 0 pages')
  assert.ok(wikiSrc.includes('＋ 最初のWikiドキュメントを作成'), 'WikiView must feature prominent empty state CTA button')
})

test('Kanban, List, and SprintBoard implement friendly empty states and direct task creation per cell', () => {
  const projectPageSrc = fs.readFileSync(path.join(__dirname, '../pages/projects/[id].vue'), 'utf8')
  const sprintSrc = fs.readFileSync(path.join(__dirname, '../components/SprintBoardView.vue'), 'utf8')
  const cssSrc = fs.readFileSync(path.join(__dirname, '../assets/css/main.css'), 'utf8')

  // Kanban & List Empty States
  assert.ok(projectPageSrc.includes('kanban-empty-state'), 'Kanban must render kanban-empty-state when 0 issues')
  assert.ok(projectPageSrc.includes('list-empty-state'), 'List view must render list-empty-state when 0 issues')
  assert.ok(projectPageSrc.includes('＋ 新規課題を作成'), 'Empty state must feature ＋ 新規課題を作成 button')

  // Sprint Empty State
  assert.ok(sprintSrc.includes('sprint-empty-state'), 'SprintBoardView must render sprint-empty-state when 0 sprints')
  assert.ok(sprintSrc.includes('＋ 最初のスプリントを作成'), 'SprintBoardView must feature ＋ 最初のスプリントを作成 button')

  // Kanban Direct Task Creation per Cell
  assert.ok(projectPageSrc.includes('kanban-cell-add-task-btn'), 'Kanban cell must render add task quick button')
  assert.ok(projectPageSrc.includes('openCreateTaskModal(iss, st.id)'), 'Cell add task must pass issue and status_id to openCreateTaskModal')
  assert.ok(projectPageSrc.includes('function openCreateTaskModal'), 'openCreateTaskModal helper must be implemented')

  // Card Affordance
  assert.ok(cssSrc.includes('transform: translateY(-2px)'), 'Kanban card must feature hover lift transform')
  assert.ok(cssSrc.includes('box-shadow: 0 4px 12px'), 'Kanban card must feature elevated hover box-shadow')
})

test('ItemDetailModal organizes long sections into clean sub-tabs under details', () => {
  const modalSrc = fs.readFileSync(path.join(__dirname, '../components/ItemDetailModal.vue'), 'utf8')

  // Sub-tabs switcher & reactive state
  assert.ok(modalSrc.includes('item-detail-subtabs'), 'Modal must render item-detail-subtabs bar')
  assert.ok(modalSrc.includes('const detailSubTab = ref'), 'Modal must define detailSubTab reactive state')

  // 4 distinct sub-tabs
  assert.ok(modalSrc.includes("detailSubTab === 'general'"), 'General subtab must exist for basic info')
  assert.ok(modalSrc.includes("detailSubTab === 'deps_attachments'"), 'Dependencies & Attachments subtab must exist')
  assert.ok(modalSrc.includes("detailSubTab === 'risk_retrospective'"), 'Risk & Retrospective subtab must exist for issues')
  assert.ok(modalSrc.includes("detailSubTab === 'comments'"), 'Comments subtab must exist')

  // History tab accessibility & save bar preservation
  assert.ok(modalSrc.includes("modalTab === 'history'"), 'History activity tab must remain accessible')
  assert.ok(modalSrc.includes('isDirty && canEditAny'), 'Sticky floating dirty bar must remain available for instant saves')
})
