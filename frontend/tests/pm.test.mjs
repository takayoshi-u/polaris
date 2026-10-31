import test from 'node:test'
import assert from 'node:assert/strict'

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

function calculateAutoScheduleUpdates(changedTaskId, newPlannedEndDate, dependencies, allTasks) {
  const taskMap = new Map()
  for (const t of allTasks) {
    taskMap.set(t.id, t)
  }

  const succMap = new Map()
  for (const d of dependencies) {
    if (!succMap.has(d.predecessor_id)) {
      succMap.set(d.predecessor_id, [])
    }
    succMap.get(d.predecessor_id).push(d)
  }

  const updates = new Map()
  const visited = new Set()

  function propagate(currentId, currentEndDate) {
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

function calculateGanttDependencyPath(fromX, fromY, toX, toY) {
  if (toX >= fromX + 16) {
    const midX = Math.round(fromX + (toX - fromX) / 2)
    return `M ${fromX} ${fromY} L ${midX} ${fromY} L ${midX} ${toY} L ${toX} ${toY}`
  }

  const leadX = fromX + 12
  const midY = Math.round((fromY + toY) / 2)
  const backX = toX - 12
  return `M ${fromX} ${fromY} L ${leadX} ${fromY} L ${leadX} ${midY} L ${backX} ${midY} L ${backX} ${toY} L ${toX} ${toY}`
}

function generateWeeksList(referenceYMD, weekCount = 6) {
  const parts = referenceYMD.split('-').map(Number)
  const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
  const dayOfWeek = dt.getUTCDay()
  const diffToMonday = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
  dt.setUTCDate(dt.getUTCDate() + diffToMonday)

  const weeks = []
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

function countBusinessDays(startYMD, endYMD) {
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

function calculateMemberWorkloads(userIds, tasks, weeks, hoursPerDay = 8.0) {
  const result = new Map()
  const capacityHours = Math.round(hoursPerDay * 5 * 100) / 100

  for (const uid of userIds) {
    const userWeeks = []
    const userTasks = tasks.filter(t => t.assignees?.some(a => a.type === 'user' && a.id === uid))

    for (const wk of weeks) {
      let allocatedTotal = 0
      const contributions = []

      for (const t of userTasks) {
        if (!t.planned_start_date || !t.planned_end_date) continue
        const tStart = t.planned_start_date
        const tEnd = t.planned_end_date
        const pHours = Number(t.planned_hours) || 0
        if (pHours <= 0) continue

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
            totalPlannedHours: pHours,
            allocatedHours: taskHoursForWeek
          })
        }
      }

      allocatedTotal = Math.round(allocatedTotal * 100) / 100
      const ratio = capacityHours > 0 ? (allocatedTotal / capacityHours) * 100 : 0
      let status = 'low'
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

test('calculateAutoScheduleUpdates slides successor forward when predecessor pushes out', () => {
  const allTasks = [
    {
      id: 'task-1',
      title: 'Task 1',
      planned_start_date: '2026-11-01',
      planned_end_date: '2026-11-03'
    },
    {
      id: 'task-2',
      title: 'Task 2',
      planned_start_date: '2026-11-04',
      planned_end_date: '2026-11-07'
    },
    {
      id: 'task-3',
      title: 'Task 3',
      planned_start_date: '2026-11-08',
      planned_end_date: '2026-11-10'
    }
  ]

  const dependencies = [
    {
      id: 'dep-1',
      predecessor_id: 'task-1',
      successor_id: 'task-2',
      dependency_type: 'FS',
      lag_days: 0
    },
    {
      id: 'dep-2',
      predecessor_id: 'task-2',
      successor_id: 'task-3',
      dependency_type: 'FS',
      lag_days: 1
    }
  ]

  // Task 1 planned_end_date pushes from 2026-11-03 to 2026-11-06
  const updates = calculateAutoScheduleUpdates('task-1', '2026-11-06', dependencies, allTasks)

  assert.equal(updates.length, 2)
  const u2 = updates.find(u => u.taskId === 'task-2')
  assert.equal(u2.planned_start_date, '2026-11-07')
  assert.equal(u2.planned_end_date, '2026-11-10')

  const u3 = updates.find(u => u.taskId === 'task-3')
  assert.equal(u3.planned_start_date, '2026-11-12')
  assert.equal(u3.planned_end_date, '2026-11-14')
})

test('calculateGanttDependencyPath generates valid SVG paths', () => {
  const forwardPath = calculateGanttDependencyPath(100, 50, 200, 112)
  assert.equal(forwardPath, 'M 100 50 L 150 50 L 150 112 L 200 112')

  const backPath = calculateGanttDependencyPath(200, 50, 150, 112)
  assert.equal(backPath, 'M 200 50 L 212 50 L 212 81 L 138 81 L 138 112 L 150 112')
})

test('generateWeeksList and countBusinessDays calculate periods correctly', () => {
  const weeks = generateWeeksList('2026-11-04', 3)
  assert.equal(weeks.length, 3)
  assert.equal(weeks[0].start, '2026-11-02') // Monday
  assert.equal(weeks[0].end, '2026-11-08')   // Sunday

  assert.equal(countBusinessDays('2026-11-02', '2026-11-06'), 5)
  assert.equal(countBusinessDays('2026-11-02', '2026-11-08'), 5)
})

test('calculateMemberWorkloads identifies low, optimal, and overload statuses', () => {
  const weeks = generateWeeksList('2026-11-02', 2)
  const tasks = [
    {
      id: 't1',
      title: 'Optimal Task',
      planned_hours: 36, // 36h / 40h = 90% (optimal)
      planned_start_date: '2026-11-02',
      planned_end_date: '2026-11-06',
      assignees: [{ type: 'user', id: 'u1' }]
    },
    {
      id: 't2',
      title: 'Heavy Task',
      planned_hours: 48, // 48h / 40h = 120% (overload)
      planned_start_date: '2026-11-09',
      planned_end_date: '2026-11-13',
      assignees: [{ type: 'user', id: 'u1' }]
    }
  ]

  const workloads = calculateMemberWorkloads(['u1'], tasks, weeks, 8.0)
  const u1Weeks = workloads.get('u1')
  assert.equal(u1Weeks.length, 2)
  assert.equal(u1Weeks[0].status, 'optimal')
  assert.equal(u1Weeks[0].ratioPercent, 90)
  assert.equal(u1Weeks[1].status, 'overload')
  assert.equal(u1Weeks[1].ratioPercent, 120)
})
