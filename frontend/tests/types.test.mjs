import test from 'node:test'
import assert from 'node:assert/strict'

function sortTypesByDisplayOrder(types) {
  return [...types].sort((a, b) => a.display_order - b.display_order)
}

function filterItemsByType(items, typeId) {
  if (!typeId) return items
  return items.filter(item => item.type_id === typeId)
}

function isSystemCommonType(itemType) {
  return itemType.project_id === null
}

test('ItemType helper functions: sort, filter and scope evaluation', () => {
  const types = [
    { id: 't2', project_id: 'p1', name: 'バグ', color: '#ef4444', display_order: 2 },
    { id: 't1', project_id: null, name: 'タスク', color: '#3b82f6', display_order: 1 },
    { id: 't3', project_id: 'p1', name: '機能追加', color: '#10b981', display_order: 3 }
  ]

  // Sort
  const sorted = sortTypesByDisplayOrder(types)
  assert.equal(sorted[0].name, 'タスク')
  assert.equal(sorted[1].name, 'バグ')
  assert.equal(sorted[2].name, '機能追加')

  // Common vs Project specific
  assert.equal(isSystemCommonType(types[0]), false)
  assert.equal(isSystemCommonType(types[1]), true)

  // Filter
  const tasks = [
    { id: 'task-1', title: 'DB設計', type_id: 't1' },
    { id: 'task-2', title: 'ログイン不具合修正', type_id: 't2' },
    { id: 'task-3', title: 'ドキュメント更新', type_id: null }
  ]

  const bugTasks = filterItemsByType(tasks, 't2')
  assert.equal(bugTasks.length, 1)
  assert.equal(bugTasks[0].id, 'task-2')

  const allTasks = filterItemsByType(tasks, '')
  assert.equal(allTasks.length, 3)
})
