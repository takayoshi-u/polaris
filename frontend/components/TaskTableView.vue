<template>
  <div class="card" style="display: flex; flex-direction: column; gap: 14px; padding: 16px;">
    <!-- Top Table Header & Controls -->
    <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
      <div style="display: flex; align-items: center; gap: 10px;">
        <h2 style="font-size: 16px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
          <span>📑 リスト / テーブルビュー</span>
          <span class="badge badge-slate" style="font-size: 11px;">
            課題 {{ filteredIssues.length }}件 / タスク {{ totalTasksCount }}件
          </span>
        </h2>
      </div>

      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 10px;">
        <!-- Realtime local filter input -->
        <input
          v-model="searchQuery"
          type="text"
          placeholder="テーブル内を検索..."
          style="width: 200px; font-size: 12px; padding: 5px 10px;"
        />

        <div style="display: flex; gap: 6px;">
          <button type="button" class="btn btn-secondary btn-sm" @click="expandAll">すべて展開</button>
          <button type="button" class="btn btn-secondary btn-sm" @click="collapseAll">すべて折りたたむ</button>
        </div>
      </div>
    </div>

    <!-- Bulk Action Floating / Sticky Bar -->
    <div
      v-if="selectedTaskIds.size > 0"
      style="
        padding: 10px 16px;
        background: #1e293b;
        color: #f8fafc;
        border-radius: 8px;
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: 12px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
      "
    >
      <div style="display: flex; align-items: center; gap: 10px; font-size: 13px; font-weight: 600;">
        <span class="badge badge-blue">{{ selectedTaskIds.size }} 件選択中</span>
        <span>選択したタスクの一括操作:</span>
      </div>

      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
        <!-- Bulk Status Change -->
        <div style="display: flex; align-items: center; gap: 6px;">
          <select v-model="bulkStatusId" style="font-size: 12px; padding: 4px 8px; background: #334155; color: #fff; border-color: #475569;">
            <option value="">-- ステータス一括変更 --</option>
            <option v-for="st in statuses" :key="st.id" :value="st.id">
              {{ st.display_order }}. {{ st.name }}
            </option>
          </select>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            :disabled="!bulkStatusId || isBulkUpdating"
            @click="applyBulkStatus"
          >
            適用
          </button>
        </div>

        <button
          type="button"
          class="btn btn-danger btn-sm"
          :disabled="isBulkUpdating"
          @click="applyBulkDelete"
        >
          🗑 一括削除
        </button>

        <button
          type="button"
          class="btn btn-ghost btn-sm"
          style="color: #cbd5e1;"
          @click="clearSelection"
        >
          選択解除
        </button>
      </div>
    </div>

    <!-- Spreadsheet Table Container -->
    <div style="overflow-x: auto; width: 100%; border: 1px solid var(--border-color); border-radius: 8px;">
      <table style="width: 100%; border-collapse: collapse; font-size: 12px; min-width: 980px;">
        <thead>
          <tr style="background: var(--bg-elevated); border-bottom: 2px solid var(--border-color); text-align: left; height: 38px;">
            <th style="width: 38px; text-align: center; padding: 4px 8px;">
              <input
                type="checkbox"
                :checked="isAllSelected"
                :indeterminate.prop="isPartialSelected"
                title="全選択/解除"
                @change="toggleSelectAll"
              />
            </th>
            <th
              style="padding: 8px 10px; cursor: pointer; user-select: none;"
              @click="sortBy('title')"
            >
              タイトル {{ sortKey === 'title' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 140px; padding: 8px 10px; cursor: pointer; user-select: none;"
              @click="sortBy('status')"
            >
              ステータス {{ sortKey === 'status' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 130px; padding: 8px 10px; cursor: pointer; user-select: none;"
              @click="sortBy('assignee')"
            >
              担当者 {{ sortKey === 'assignee' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 90px; padding: 8px 10px; text-align: right; cursor: pointer; user-select: none;"
              @click="sortBy('story_points')"
            >
              SP {{ sortKey === 'story_points' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 110px; padding: 8px 10px; text-align: right; cursor: pointer; user-select: none;"
              @click="sortBy('progress')"
            >
              進捗 {{ sortKey === 'progress' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 100px; padding: 8px 10px; text-align: right; cursor: pointer; user-select: none;"
              @click="sortBy('planned_hours')"
            >
              予定工数(h) {{ sortKey === 'planned_hours' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 100px; padding: 8px 10px; text-align: right; cursor: pointer; user-select: none;"
              @click="sortBy('actual_hours')"
            >
              実績工数(h) {{ sortKey === 'actual_hours' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th
              style="width: 160px; padding: 8px 10px; cursor: pointer; user-select: none;"
              @click="sortBy('dates')"
            >
              予定期間 {{ sortKey === 'dates' ? (sortOrder === 'asc' ? '▲' : '▼') : '' }}
            </th>
            <th style="width: 60px; padding: 8px 10px; text-align: center;">操作</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="iss in sortedIssues" :key="`iss-${iss.id}`">
            <!-- Issue Row (Parent) -->
            <tr
              style="background: var(--bg-card-hover); border-bottom: 1px solid var(--border-color); font-weight: 600;"
            >
              <td style="text-align: center; padding: 6px 8px;">
                <input
                  type="checkbox"
                  :checked="isIssueSelected(iss)"
                  :indeterminate.prop="isIssuePartialSelected(iss)"
                  @change="toggleSelectIssue(iss)"
                />
              </td>
              <td style="padding: 6px 10px;">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    style="padding: 1px 5px; font-size: 11px;"
                    @click="toggleIssueCollapse(iss.id)"
                  >
                    {{ collapsedIssueIds.has(iss.id) ? '▶' : '▼' }}
                  </button>
                  <span :class="['badge', iss.state === 'open' ? 'badge-green' : 'badge-slate']" style="font-size: 10px;">
                    {{ iss.state.toUpperCase() }}
                  </span>
                  <span
                    style="color: var(--primary); cursor: pointer; font-weight: 700;"
                    @click="$emit('open-issue', iss)"
                  >
                    {{ iss.title }}
                  </span>
                  <span style="font-size: 11px; color: var(--text-muted); font-weight: normal;">
                    ({{ (iss.tasks || []).length }}タスク)
                  </span>
                </div>
              </td>
              <td style="padding: 6px 10px; color: var(--text-muted); font-size: 11px;">
                -
              </td>
              <td style="padding: 6px 10px; color: var(--text-muted); font-size: 11px;">
                {{ getUserName(iss.owner_user_id) }} (オーナー)
              </td>
              <td style="padding: 6px 10px; text-align: right; color: var(--text-muted);">
                -
              </td>
              <td style="padding: 6px 10px; text-align: right;">
                <span class="badge badge-blue" style="font-size: 10px;">
                  {{ iss.rollup?.weighted_progress ? iss.rollup.weighted_progress.toFixed(0) : 0 }}%
                </span>
              </td>
              <td style="padding: 6px 10px; text-align: right; font-size: 11px;">
                {{ formatHours(iss.rollup?.total_planned_hours) }}
              </td>
              <td style="padding: 6px 10px; text-align: right; font-size: 11px;">
                {{ formatHours(iss.rollup?.total_actual_hours) }}
              </td>
              <td style="padding: 6px 10px; font-size: 11px; color: var(--text-muted);">
                {{ iss.rollup?.planned_start_date || '-' }} 〜 {{ iss.rollup?.planned_end_date || '-' }}
              </td>
              <td style="padding: 6px 10px; text-align: center;">
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="padding: 2px 5px; font-size: 11px;"
                  title="課題詳細を開く"
                  @click="$emit('open-issue', iss)"
                >
                  詳細
                </button>
              </td>
            </tr>

            <!-- Task Rows under Issue -->
            <template v-if="!collapsedIssueIds.has(iss.id)">
              <tr
                v-for="t in getSortedTasksForIssue(iss)"
                :key="`task-${t.id}`"
                :style="{
                  backgroundColor: selectedTaskIds.has(t.id) ? 'rgba(59, 130, 246, 0.08)' : 'transparent',
                  borderBottom: '1px solid var(--border-color)'
                }"
              >
                <!-- Checkbox -->
                <td style="text-align: center; padding: 4px 8px;">
                  <input
                    type="checkbox"
                    :checked="selectedTaskIds.has(t.id)"
                    @change="toggleSelectTask(t.id)"
                  />
                </td>

                <!-- Title -->
                <td style="padding: 4px 10px 4px 28px;">
                  <div style="display: flex; align-items: center; gap: 6px;">
                    <span style="color: var(--text-muted);">├</span>
                    <span
                      style="cursor: pointer; color: var(--text-main); font-weight: 500;"
                      @click="$emit('open-task', t, iss)"
                    >
                      {{ t.title }}
                    </span>
                    <span v-if="t.story_points" class="badge badge-purple" style="font-size: 10px; padding: 1px 4px;">
                      {{ t.story_points }} SP
                    </span>
                  </div>
                </td>

                <!-- Status Badge with Instant Dropdown -->
                <td style="padding: 4px 10px; position: relative;">
                  <div style="display: inline-block;">
                    <button
                      type="button"
                      class="badge"
                      :style="{
                        backgroundColor: getStatusColor(t.status_id) + '22',
                        color: getStatusColor(t.status_id),
                        border: `1px solid ${getStatusColor(t.status_id)}`,
                        cursor: canEdit ? 'pointer' : 'default',
                        fontSize: '11px',
                        padding: '2px 8px'
                      }"
                      @click="canEdit && toggleStatusDropdown(t.id)"
                    >
                      ● {{ getStatusName(t.status_id) }}
                      <span v-if="canEdit" style="margin-left: 2px; font-size: 9px;">▼</span>
                    </button>

                    <!-- Status Picker Dropdown Menu -->
                    <div
                      v-if="openStatusTaskId === t.id"
                      style="
                        position: absolute;
                        top: 100%;
                        left: 10px;
                        z-index: 50;
                        background: var(--bg-card);
                        border: 1px solid var(--border-color);
                        border-radius: 6px;
                        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
                        padding: 4px;
                        min-width: 130px;
                      "
                    >
                      <div
                        v-for="st in statuses"
                        :key="st.id"
                        style="
                          padding: 5px 8px;
                          cursor: pointer;
                          border-radius: 4px;
                          display: flex;
                          align-items: center;
                          gap: 6px;
                          font-size: 11px;
                        "
                        :style="{ background: t.status_id === st.id ? 'var(--bg-elevated)' : 'transparent' }"
                        @click="inlineUpdateTaskStatus(t, st.id)"
                      >
                        <span :style="{ width: '8px', height: '8px', borderRadius: '50%', backgroundColor: st.color }"></span>
                        <span>{{ st.name }}</span>
                      </div>
                    </div>
                  </div>
                </td>

                <!-- Assignee with Quick Change -->
                <td style="padding: 4px 10px; position: relative;">
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    style="padding: 2px 6px; font-size: 11px; display: inline-flex; align-items: center; gap: 4px;"
                    @click="canEdit && toggleAssigneeDropdown(t.id)"
                  >
                    <span>👤</span>
                    <span>{{ getTaskAssigneeName(t) }}</span>
                  </button>

                  <!-- Quick Assignee Dropdown -->
                  <div
                    v-if="openAssigneeTaskId === t.id"
                    style="
                      position: absolute;
                      top: 100%;
                      left: 10px;
                      z-index: 50;
                      background: var(--bg-card);
                      border: 1px solid var(--border-color);
                      border-radius: 6px;
                      box-shadow: 0 4px 12px rgba(0,0,0,0.15);
                      padding: 4px;
                      min-width: 140px;
                      max-height: 200px;
                      overflow-y: auto;
                    "
                  >
                    <div
                      style="padding: 4px 8px; font-size: 11px; cursor: pointer; border-radius: 4px;"
                      @click="inlineUpdateTaskAssignee(t, null)"
                    >
                      (担当者なし)
                    </div>
                    <div
                      v-for="u in users"
                      :key="u.id"
                      style="padding: 4px 8px; font-size: 11px; cursor: pointer; border-radius: 4px; display: flex; align-items: center; gap: 6px;"
                      @click="inlineUpdateTaskAssignee(t, u.id)"
                    >
                      <span>👤 {{ u.display_name }}</span>
                    </div>
                  </div>
                </td>

                <!-- Story Points Inline Input -->
                <td style="padding: 4px 8px; text-align: right;">
                  <input
                    type="number"
                    min="0"
                    step="0.1"
                    :value="t.story_points ?? ''"
                    :disabled="!canEdit"
                    style="width: 60px; text-align: right; padding: 2px 4px; font-size: 11px;"
                    placeholder="-"
                    @blur="onNumberFieldBlur(t, 'story_points', $event)"
                    @keyup.enter="($event.target as HTMLInputElement).blur()"
                  />
                </td>

                <!-- Progress % Inline Input -->
                <td style="padding: 4px 8px; text-align: right;">
                  <div style="display: flex; align-items: center; justify-content: flex-end; gap: 4px;">
                    <input
                      type="number"
                      min="0"
                      max="100"
                      step="5"
                      :value="t.progress"
                      :disabled="!canEdit"
                      style="width: 50px; text-align: right; padding: 2px 4px; font-size: 11px;"
                      @blur="onNumberFieldBlur(t, 'progress', $event)"
                      @keyup.enter="($event.target as HTMLInputElement).blur()"
                    />
                    <span style="font-size: 10px; color: var(--text-muted);">%</span>
                  </div>
                </td>

                <!-- Planned Hours Inline Input -->
                <td style="padding: 4px 8px; text-align: right;">
                  <input
                    type="number"
                    min="0"
                    step="0.25"
                    :value="t.planned_hours ?? ''"
                    :disabled="!canEdit"
                    style="width: 65px; text-align: right; padding: 2px 4px; font-size: 11px;"
                    placeholder="-"
                    @blur="onNumberFieldBlur(t, 'planned_hours', $event)"
                    @keyup.enter="($event.target as HTMLInputElement).blur()"
                  />
                </td>

                <!-- Actual Hours Inline Input -->
                <td style="padding: 4px 8px; text-align: right;">
                  <input
                    type="number"
                    min="0"
                    step="0.25"
                    :value="t.actual_hours ?? ''"
                    :disabled="!canEdit"
                    style="width: 65px; text-align: right; padding: 2px 4px; font-size: 11px;"
                    placeholder="-"
                    @blur="onNumberFieldBlur(t, 'actual_hours', $event)"
                    @keyup.enter="($event.target as HTMLInputElement).blur()"
                  />
                </td>

                <!-- Planned Dates -->
                <td style="padding: 4px 10px; font-size: 11px; color: var(--text-muted); white-space: nowrap;">
                  {{ t.planned_start_date ? t.planned_start_date.slice(5) : '-' }} 〜 {{ t.planned_end_date ? t.planned_end_date.slice(5) : '-' }}
                </td>

                <!-- Actions -->
                <td style="padding: 4px 6px; text-align: center;">
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    style="padding: 2px 5px; font-size: 11px;"
                    title="タスク詳細を開く"
                    @click="$emit('open-task', t, iss)"
                  >
                    詳細
                  </button>
                </td>
              </tr>
            </template>
          </template>

          <tr v-if="filteredIssues.length === 0">
            <td colspan="10" style="text-align: center; padding: 30px; color: var(--text-muted);">
              該当する課題またはタスクがありません
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Issue, Status, Task, User, SubjectRef } from '~/types/polaris'
import { formatHours, hasAtMostTwoDecimals } from '~/utils/format'

const props = defineProps<{
  projectId: string
  issues: Issue[]
  statuses: Status[]
  users: User[]
  canEdit: boolean
}>()

const emit = defineEmits<{
  (e: 'open-issue', issue: Issue): void
  (e: 'open-task', task: Task, parentIssue: Issue): void
  (e: 'refresh'): void
}>()

const { apiFetch, notifyToast } = useApi()

const searchQuery = ref('')
const collapsedIssueIds = ref<Set<string>>(new Set())
const selectedTaskIds = ref<Set<string>>(new Set())
const bulkStatusId = ref('')
const isBulkUpdating = ref(false)

const openStatusTaskId = ref<string | null>(null)
const openAssigneeTaskId = ref<string | null>(null)

// Sorting
const sortKey = ref<'title' | 'status' | 'assignee' | 'story_points' | 'progress' | 'planned_hours' | 'actual_hours' | 'dates'>('title')
const sortOrder = ref<'asc' | 'desc'>('asc')

function sortBy(key: typeof sortKey.value) {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortOrder.value = 'asc'
  }
}

function toggleIssueCollapse(id: string) {
  if (collapsedIssueIds.value.has(id)) {
    collapsedIssueIds.value.delete(id)
  } else {
    collapsedIssueIds.value.add(id)
  }
  collapsedIssueIds.value = new Set(collapsedIssueIds.value)
}

function expandAll() {
  collapsedIssueIds.value = new Set()
}

function collapseAll() {
  collapsedIssueIds.value = new Set(props.issues.map(i => i.id))
}

const totalTasksCount = computed(() => {
  return props.issues.reduce((acc, i) => acc + (i.tasks?.length || 0), 0)
})

const filteredIssues = computed(() => {
  if (!searchQuery.value.trim()) return props.issues
  const q = searchQuery.value.toLowerCase().trim()
  return props.issues
    .map(iss => {
      const matchIssue = iss.title.toLowerCase().includes(q)
      const matchingTasks = (iss.tasks || []).filter(t => t.title.toLowerCase().includes(q))
      if (matchIssue) return iss
      if (matchingTasks.length > 0) {
        return { ...iss, tasks: matchingTasks }
      }
      return null
    })
    .filter((iss): iss is Issue => iss !== null)
})

const sortedIssues = computed(() => {
  return [...filteredIssues.value].sort((a, b) => {
    return (a.display_order ?? 0) - (b.display_order ?? 0)
  })
})

function getSortedTasksForIssue(iss: Issue): Task[] {
  const tasks = [...(iss.tasks || [])]
  tasks.sort((a, b) => {
    let cmp = 0
    if (sortKey.value === 'title') {
      cmp = a.title.localeCompare(b.title)
    } else if (sortKey.value === 'status') {
      const aOrder = props.statuses.find(s => s.id === a.status_id)?.display_order ?? 999
      const bOrder = props.statuses.find(s => s.id === b.status_id)?.display_order ?? 999
      cmp = aOrder - bOrder
    } else if (sortKey.value === 'assignee') {
      const aName = getTaskAssigneeName(a)
      const bName = getTaskAssigneeName(b)
      cmp = aName.localeCompare(bName)
    } else if (sortKey.value === 'story_points') {
      cmp = (a.story_points ?? 0) - (b.story_points ?? 0)
    } else if (sortKey.value === 'progress') {
      cmp = a.progress - b.progress
    } else if (sortKey.value === 'planned_hours') {
      cmp = (a.planned_hours ?? 0) - (b.planned_hours ?? 0)
    } else if (sortKey.value === 'actual_hours') {
      cmp = (a.actual_hours ?? 0) - (b.actual_hours ?? 0)
    } else if (sortKey.value === 'dates') {
      cmp = (a.planned_start_date || '').localeCompare(b.planned_start_date || '')
    }
    return sortOrder.value === 'asc' ? cmp : -cmp
  })
  return tasks
}

function getStatusColor(statusId: string): string {
  return props.statuses.find(s => s.id === statusId)?.color || '#94a3b8'
}

function getStatusName(statusId: string): string {
  return props.statuses.find(s => s.id === statusId)?.name || '未設定'
}

function getUserName(userId?: string | null): string {
  if (!userId) return '-'
  return props.users.find(u => u.id === userId)?.display_name || userId.slice(0, 8)
}

function getTaskAssigneeName(task: Task): string {
  const firstUser = task.assignees?.find(a => a.type === 'user')
  if (firstUser) {
    return getUserName(firstUser.id)
  }
  return '未割当'
}

// Selection logic
const isAllSelected = computed(() => {
  if (totalTasksCount.value === 0) return false
  return selectedTaskIds.value.size === totalTasksCount.value
})

const isPartialSelected = computed(() => {
  return selectedTaskIds.value.size > 0 && !isAllSelected.value
})

function toggleSelectAll() {
  if (isAllSelected.value) {
    selectedTaskIds.value.clear()
  } else {
    for (const iss of props.issues) {
      for (const t of iss.tasks || []) {
        selectedTaskIds.value.add(t.id)
      }
    }
  }
  selectedTaskIds.value = new Set(selectedTaskIds.value)
}

function toggleSelectTask(id: string) {
  if (selectedTaskIds.value.has(id)) {
    selectedTaskIds.value.delete(id)
  } else {
    selectedTaskIds.value.add(id)
  }
  selectedTaskIds.value = new Set(selectedTaskIds.value)
}

function isIssueSelected(iss: Issue): boolean {
  const tasks = iss.tasks || []
  if (tasks.length === 0) return false
  return tasks.every(t => selectedTaskIds.value.has(t.id))
}

function isIssuePartialSelected(iss: Issue): boolean {
  const tasks = iss.tasks || []
  const count = tasks.filter(t => selectedTaskIds.value.has(t.id)).length
  return count > 0 && count < tasks.length
}

function toggleSelectIssue(iss: Issue) {
  const tasks = iss.tasks || []
  const allSelected = isIssueSelected(iss)
  for (const t of tasks) {
    if (allSelected) {
      selectedTaskIds.value.delete(t.id)
    } else {
      selectedTaskIds.value.add(t.id)
    }
  }
  selectedTaskIds.value = new Set(selectedTaskIds.value)
}

function clearSelection() {
  selectedTaskIds.value.clear()
  selectedTaskIds.value = new Set(selectedTaskIds.value)
}

// Bulk Actions
async function applyBulkStatus() {
  if (!bulkStatusId.value || selectedTaskIds.value.size === 0) return
  isBulkUpdating.value = true
  const ids = Array.from(selectedTaskIds.value)
  try {
    await Promise.all(
      ids.map(id =>
        apiFetch(`/projects/${props.projectId}/tasks/${id}`, {
          method: 'PUT',
          body: JSON.stringify({ status_id: bulkStatusId.value })
        })
      )
    )
    notifyToast(`${ids.length}件のタスクのステータスを一括変更しました`, 'success')
    clearSelection()
    bulkStatusId.value = ''
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || '一括更新に失敗しました', 'error')
  } finally {
    isBulkUpdating.value = false
  }
}

async function applyBulkDelete() {
  const count = selectedTaskIds.value.size
  if (count === 0) return
  if (!confirm(`選択した ${count} 件のタスクを削除しますか？`)) return
  isBulkUpdating.value = true
  const ids = Array.from(selectedTaskIds.value)
  try {
    await Promise.all(
      ids.map(id =>
        apiFetch(`/projects/${props.projectId}/tasks/${id}`, {
          method: 'DELETE'
        })
      )
    )
    notifyToast(`${count}件のタスクを削除しました`, 'success')
    clearSelection()
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || '一括削除に失敗しました', 'error')
  } finally {
    isBulkUpdating.value = false
  }
}

// Inline Fast Editing
function toggleStatusDropdown(taskId: string) {
  openStatusTaskId.value = openStatusTaskId.value === taskId ? null : taskId
}

function toggleAssigneeDropdown(taskId: string) {
  openAssigneeTaskId.value = openAssigneeTaskId.value === taskId ? null : taskId
}

async function inlineUpdateTaskStatus(task: Task, newStatusId: string) {
  openStatusTaskId.value = null
  if (task.status_id === newStatusId) return
  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ status_id: newStatusId })
    })
    notifyToast(`ステータスを変更しました`, 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'ステータス更新に失敗しました', 'error')
  }
}

async function inlineUpdateTaskAssignee(task: Task, newUserId: string | null) {
  openAssigneeTaskId.value = null
  const newAssignees: SubjectRef[] = newUserId ? [{ type: 'user', id: newUserId }] : []
  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ assignees: newAssignees })
    })
    notifyToast('担当者を更新しました', 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || '担当者更新に失敗しました', 'error')
  }
}

async function onNumberFieldBlur(
  task: Task,
  field: 'progress' | 'planned_hours' | 'actual_hours' | 'story_points',
  event: Event
) {
  const target = event.target as HTMLInputElement
  const raw = target.value.trim()
  const currentVal = task[field]

  if (raw === '' && (field === 'planned_hours' || field === 'actual_hours' || field === 'story_points')) {
    if (currentVal === null || currentVal === undefined) return
    await saveField(task, field, null)
    return
  }

  const num = Number(raw)
  if (isNaN(num) || num < 0) {
    target.value = String(currentVal ?? '')
    return
  }

  if ((field === 'planned_hours' || field === 'actual_hours') && !hasAtMostTwoDecimals(num)) {
    notifyToast('工数は小数点第2位までで入力してください', 'error')
    target.value = String(currentVal ?? '')
    return
  }

  if (field === 'progress') {
    const clamped = Math.min(100, Math.max(0, Math.round(num)))
    if (currentVal === clamped) return
    await saveField(task, field, clamped)
    return
  }

  if (currentVal === num) return
  await saveField(task, field, num)
}

async function saveField(task: Task, field: string, value: any) {
  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ [field]: value })
    })
    notifyToast('更新を保存しました', 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || '保存に失敗しました', 'error')
  }
}
</script>
