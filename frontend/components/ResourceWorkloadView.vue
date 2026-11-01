<template>
  <div class="card" style="display: flex; flex-direction: column; gap: 16px; padding: 16px;">
    <!-- Top Header and Week Navigation -->
    <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 10px;">
        <h2 style="font-size: 16px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
          <span>👥 メンバー別リソース負荷（キャパシティ）</span>
        </h2>
        <span class="badge badge-blue">
          基準: {{ formatHours(hoursPerDay) }}h/日 (週キャパシティ: {{ formatHours(weeklyCapacity) }}h)
        </span>
      </div>

      <!-- Navigation & Shift Controls -->
      <div style="display: flex; align-items: center; gap: 8px;">
        <button type="button" class="btn btn-secondary btn-sm" @click="shiftWeeks(-2)">◀ -2週</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="resetToCurrentWeek">今週へ</button>
        <button type="button" class="btn btn-secondary btn-sm" @click="shiftWeeks(2)">+2週 ▶</button>
      </div>
    </div>

    <!-- Color Legend -->
    <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 14px; font-size: 12px; padding: 6px 10px; background: var(--bg-elevated); border-radius: 6px;">
      <span style="font-weight: 600; color: var(--text-muted);">凡例:</span>
      <div style="display: flex; align-items: center; gap: 5px;">
        <span style="width: 14px; height: 14px; border-radius: 3px; background: #e2e8f0; border: 1px solid #cbd5e1;"></span>
        <span>余裕あり (&lt; 80%)</span>
      </div>
      <div style="display: flex; align-items: center; gap: 5px;">
        <span style="width: 14px; height: 14px; border-radius: 3px; background: #dcfce7; border: 1px solid #86efac;"></span>
        <span style="font-weight: 600; color: #15803d;">適正負荷 (80%〜100%)</span>
      </div>
      <div style="display: flex; align-items: center; gap: 5px;">
        <span style="width: 14px; height: 14px; border-radius: 3px; background: #fee2e2; border: 1px solid #fca5a5;"></span>
        <span style="font-weight: 700; color: #b91c1c;">⚠️ 過負荷警告 (&gt; 100%)</span>
      </div>
      <span style="margin-left: auto; color: var(--text-muted); font-size: 11px;">※セルをクリックすると担当タスク詳細を表示します</span>
    </div>

    <!-- Workload Matrix Table Container -->
    <div style="overflow-x: auto; width: 100%; border: 1px solid var(--border-color); border-radius: 8px;">
      <table style="width: 100%; border-collapse: collapse; font-size: 12px; min-width: 780px;">
        <thead>
          <tr style="background: var(--bg-elevated); border-bottom: 2px solid var(--border-color); height: 42px;">
            <th style="padding: 8px 12px; width: 180px; text-align: left; position: sticky; left: 0; background: var(--bg-elevated); z-index: 10; border-right: 1px solid var(--border-color);">
              メンバー ({{ membersList.length }}名)
            </th>
            <th
              v-for="wk in weeks"
              :key="wk.start"
              style="padding: 8px 6px; text-align: center; border-right: 1px solid var(--border-color); min-width: 120px;"
            >
              <div style="font-weight: 700; color: var(--text-main);">{{ wk.label }}</div>
              <div style="font-size: 10px; color: var(--text-muted); font-weight: normal;">
                {{ wk.start.slice(5) }} 〜 {{ wk.end.slice(5) }}
              </div>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="membersList.length === 0">
            <td :colspan="weeks.length + 1" style="text-align: center; padding: 30px; color: var(--text-muted);">
              プロジェクトメンバーが登録されていません
            </td>
          </tr>

          <tr
            v-for="u in membersList"
            :key="u.id"
            style="border-bottom: 1px solid var(--border-color); height: 56px;"
          >
            <!-- Member Header Cell (Sticky Left) -->
            <td style="padding: 8px 12px; position: sticky; left: 0; background: var(--bg-card); z-index: 5; border-right: 1px solid var(--border-color);">
              <div style="display: flex; align-items: center; gap: 8px;">
                <div style="width: 26px; height: 26px; border-radius: 50%; background: #e2e8f0; display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 700;">
                  {{ u.display_name.slice(0, 1) }}
                </div>
                <div style="overflow: hidden;">
                  <div style="font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">{{ u.display_name }}</div>
                  <div style="font-size: 10px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">{{ u.email }}</div>
                </div>
              </div>
            </td>

            <!-- Week Cells for Member -->
            <td
              v-for="cell in getMemberWeeks(u.id)"
              :key="cell.week.start"
              :style="{
                padding: '6px 8px',
                textAlign: 'center',
                borderRight: '1px solid var(--border-color)',
                cursor: 'pointer',
                backgroundColor: getCellBg(cell),
                color: getCellTextColor(cell),
                transition: 'background-color 0.15s ease'
              }"
              @click="openDetailPopover(u, cell)"
            >
              <div style="display: flex; flex-direction: column; align-items: center; gap: 2px;">
                <div style="display: flex; align-items: center; gap: 4px; font-weight: 700; font-size: 12px;">
                  <span v-if="cell.status === 'overload'">⚠️</span>
                  <span>{{ formatHours(cell.allocatedHours) }}h</span>
                  <span style="font-size: 10px; font-weight: normal; opacity: 0.85;">/ {{ formatHours(weeklyCapacity) }}h</span>
                </div>

                <!-- Mini Progress Bar -->
                <div style="width: 80%; height: 5px; border-radius: 3px; background: rgba(0,0,0,0.1); overflow: hidden; margin-top: 2px;">
                  <div
                    :style="{
                      width: `${Math.min(cell.ratioPercent, 100)}%`,
                      height: '100%',
                      backgroundColor: getBarColor(cell)
                    }"
                  ></div>
                </div>

                <div style="font-size: 10px; font-weight: 600; margin-top: 1px;">
                  {{ cell.ratioPercent }}%
                </div>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Task Breakdown Popover / Modal -->
    <div
      v-if="selectedCellModal"
      style="
        position: fixed;
        inset: 0;
        z-index: 1000;
        background: rgba(0, 0, 0, 0.45);
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 16px;
      "
      v-backdrop-dismiss="() => selectedCellModal = null"
    >
      <div class="card" style="width: 100%; max-width: 520px; padding: 20px; display: flex; flex-direction: column; gap: 14px;">
        <!-- Popover Header -->
        <div style="display: flex; align-items: flex-start; justify-content: space-between; gap: 8px;">
          <div>
            <div style="display: flex; align-items: center; gap: 8px;">
              <span class="badge badge-blue">期間: {{ selectedCellModal.cell.week.start }} 〜 {{ selectedCellModal.cell.week.end }}</span>
              <span
                :class="[
                  'badge',
                  selectedCellModal.cell.status === 'overload'
                    ? 'badge-amber'
                    : selectedCellModal.cell.status === 'optimal'
                    ? 'badge-green'
                    : 'badge-slate'
                ]"
              >
                負荷率: {{ selectedCellModal.cell.ratioPercent }}%
              </span>
            </div>
            <h3 style="font-size: 16px; font-weight: 700; margin-top: 6px;">
              👤 {{ selectedCellModal.user.display_name }} の週間担当タスク
            </h3>
          </div>
          <button type="button" class="btn btn-ghost btn-sm" @click="selectedCellModal = null">✕</button>
        </div>

        <!-- Metric summary -->
        <div style="display: flex; justify-content: space-between; font-size: 12px; padding: 8px 12px; background: var(--bg-input); border-radius: 6px;">
          <span>週間合計割当工数: <strong>{{ formatHours(selectedCellModal.cell.allocatedHours) }}h</strong></span>
          <span>週間キャパシティ: <strong>{{ formatHours(selectedCellModal.cell.capacityHours) }}h</strong></span>
        </div>

        <!-- Task list for this member & week -->
        <div style="display: flex; flex-direction: column; gap: 8px; max-height: 280px; overflow-y: auto;">
          <div v-if="selectedCellModal.cell.contributions.length === 0" style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 13px;">
            この期間に担当している工数割当タスクはありません
          </div>

          <div
            v-for="c in selectedCellModal.cell.contributions"
            :key="c.taskId"
            style="padding: 10px 12px; border-radius: 6px; border: 1px solid var(--border-color); background: var(--bg-main); display: flex; align-items: center; justify-content: space-between; gap: 10px; cursor: pointer;"
            @click="onTaskClick(c.taskId)"
          >
            <div style="overflow: hidden;">
              <div style="font-weight: 600; font-size: 13px; color: var(--primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                {{ c.taskTitle }}
              </div>
              <div style="font-size: 11px; color: var(--text-muted); margin-top: 2px;">
                予定期間: {{ c.plannedStartDate || '-' }} 〜 {{ c.plannedEndDate || '-' }}
              </div>
            </div>

            <div style="display: flex; flex-direction: column; align-items: flex-end; gap: 2px; flex-shrink: 0;">
              <span style="font-size: 13px; font-weight: 700; color: var(--text-main);">
                {{ formatHours(c.allocatedHours) }}h
              </span>
              <span style="font-size: 10px; color: var(--text-muted);">
                (タスク総工数: {{ formatHours(c.totalPlannedHours) }}h)
              </span>
            </div>
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end;">
          <button type="button" class="btn btn-secondary" @click="selectedCellModal = null">閉じる</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { MemberWeekWorkload, WeekPeriod } from '~/utils/format'
import type { Issue, ProjectMember, Task, User } from '~/types/polaris'
import { calculateMemberWorkloads, formatHours, generateWeeksList } from '~/utils/format'

const props = defineProps<{
  projectId: string
  hoursPerDay: number
  allTasks: Task[]
  users: User[]
  members: ProjectMember[]
  issues: Issue[]
}>()

const emit = defineEmits<{
  (e: 'open-task', task: Task, parentIssue?: Issue): void
}>()

const weeklyCapacity = computed(() => {
  return Math.round((props.hoursPerDay || 8.0) * 5 * 100) / 100
})

const referenceDateYMD = ref(new Date().toISOString().slice(0, 10))
const weekCount = ref(6)

const weeks = computed<WeekPeriod[]>(() => {
  return generateWeeksList(referenceDateYMD.value, weekCount.value)
})

function shiftWeeks(deltaWeeks: number) {
  const parts = referenceDateYMD.value.split('-').map(Number)
  const dt = new Date(Date.UTC(parts[0], parts[1] - 1, parts[2]))
  dt.setUTCDate(dt.getUTCDate() + deltaWeeks * 7)
  const y = dt.getUTCFullYear()
  const m = String(dt.getUTCMonth() + 1).padStart(2, '0')
  const d = String(dt.getUTCDate()).padStart(2, '0')
  referenceDateYMD.value = `${y}-${m}-${d}`
}

function resetToCurrentWeek() {
  referenceDateYMD.value = new Date().toISOString().slice(0, 10)
}

const membersList = computed<User[]>(() => {
  // Members registered in project
  const memberUserIds = new Set(
    props.members
      .filter(m => m.subject_type === 'user')
      .map(m => m.subject_id)
  )
  if (memberUserIds.size === 0) {
    return props.users
  }
  const filtered = props.users.filter(u => memberUserIds.has(u.id))
  return filtered.length > 0 ? filtered : props.users
})

const taskParentMap = computed(() => {
  const map = new Map<string, string>()
  for (const iss of props.issues) {
    for (const t of iss.tasks || []) {
      map.set(t.id, iss.title)
    }
  }
  return map
})

const workloadsMap = computed(() => {
  const uids = membersList.value.map(u => u.id)
  return calculateMemberWorkloads(
    uids,
    props.allTasks,
    weeks.value,
    props.hoursPerDay || 8.0,
    taskParentMap.value
  )
})

function getMemberWeeks(userId: string): MemberWeekWorkload[] {
  return workloadsMap.value.get(userId) || []
}

function getCellBg(cell: MemberWeekWorkload): string {
  if (cell.status === 'overload') {
    return '#fee2e2' // red-100
  }
  if (cell.status === 'optimal') {
    return '#dcfce7' // green-100
  }
  if (cell.allocatedHours > 0) {
    return '#f1f5f9' // slate-100
  }
  return 'transparent'
}

function getCellTextColor(cell: MemberWeekWorkload): string {
  if (cell.status === 'overload') {
    return '#b91c1c' // red-700
  }
  if (cell.status === 'optimal') {
    return '#15803d' // green-700
  }
  return 'var(--text-main)'
}

function getBarColor(cell: MemberWeekWorkload): string {
  if (cell.status === 'overload') return '#ef4444'
  if (cell.status === 'optimal') return '#22c55e'
  return '#3b82f6'
}

// Popover Modal
const selectedCellModal = ref<{ user: User; cell: MemberWeekWorkload } | null>(null)

function openDetailPopover(user: User, cell: MemberWeekWorkload) {
  selectedCellModal.value = { user, cell }
}

function onTaskClick(taskId: string) {
  const t = props.allTasks.find(x => x.id === taskId)
  if (!t) return
  let parentIss: Issue | undefined
  for (const iss of props.issues) {
    if ((iss.tasks || []).some(x => x.id === taskId)) {
      parentIss = iss
      break
    }
  }
  selectedCellModal.value = null
  emit('open-task', t, parentIss)
}
</script>
