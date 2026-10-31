<template>
  <div style="display: flex; flex-direction: column; gap: 16px;">
    <!-- Top Bar: Sprint Selection & Actions -->
    <div class="card" style="padding: 12px 16px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
      <div style="display: flex; align-items: center; gap: 10px;">
        <h2 style="font-size: 16px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
          <span>🏃 スプリント管理 ＆ バーンダウン</span>
          <span class="badge badge-slate" style="font-size: 11px;">{{ sprints.length }}スプリント</span>
        </h2>
      </div>

      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
        <!-- Tab Mode: Planning vs Burndown -->
        <div style="display: flex; gap: 4px; padding: 2px; border-radius: 6px; background: var(--bg-card); border: 1px solid var(--border-color);">
          <button
            type="button"
            class="btn btn-sm"
            :class="subTab === 'planning' ? 'btn-primary' : 'btn-ghost'"
            @click="subTab = 'planning'"
          >
            📋 スプリント計画
          </button>
          <button
            type="button"
            class="btn btn-sm"
            :class="subTab === 'burndown' ? 'btn-primary' : 'btn-ghost'"
            @click="subTab = 'burndown'"
          >
            📈 バーンダウンチャート
          </button>
        </div>

        <button
          v-if="canEdit"
          type="button"
          class="btn btn-primary btn-sm"
          @click="openCreateModal"
        >
          ＋ 新規スプリント作成
        </button>
      </div>
    </div>

    <!-- SUBVIEW 1: SPRINT PLANNING AREA -->
    <div v-if="subTab === 'planning'" style="display: flex; flex-direction: column; gap: 16px;">
      <!-- Active Sprint Highlight Banner (if any) -->
      <div
        v-if="activeSprint"
        class="card"
        style="padding: 14px 18px; border-left: 4px solid var(--success); background: var(--bg-card); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 14px;"
      >
        <div style="display: flex; flex-direction: column; gap: 4px;">
          <div style="display: flex; align-items: center; gap: 8px;">
            <span class="badge badge-green" style="font-size: 11px; font-weight: 700;">⚡ 現在のアクティブスプリント</span>
            <strong style="font-size: 15px;">{{ activeSprint.name }}</strong>
            <span v-if="activeSprint.start_date || activeSprint.end_date" style="font-size: 12px; color: var(--text-muted);">
              ({{ activeSprint.start_date || '-' }} 〜 {{ activeSprint.end_date || '-' }})
            </span>
          </div>
          <p v-if="activeSprint.goal" style="font-size: 13px; color: var(--text-muted); margin: 0;">
            目標: {{ activeSprint.goal }}
          </p>
        </div>

        <div style="display: flex; align-items: center; gap: 12px;">
          <div style="display: flex; gap: 8px; font-size: 12px;">
            <span class="badge badge-purple">{{ activeSprintStoryPoints }} SP</span>
            <span class="badge badge-blue">{{ activeSprintTasks.length }} タスク</span>
            <span class="badge badge-slate">{{ formatHours(activeSprintPlannedHours) }}h</span>
          </div>
          <button
            v-if="canEdit"
            type="button"
            class="btn btn-secondary btn-sm"
            @click="closeSprint(activeSprint)"
          >
            ✓ スプリント完了
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            @click="viewSprintBurndown(activeSprint.id)"
          >
            📈 バーンダウンを見る
          </button>
        </div>
      </div>

      <!-- Two-Column Planning Board: Left = Sprints, Right = Unassigned Backlog -->
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 16px; align-items: start;">
        <!-- Column 1: Sprints (Active / Planned / Closed) -->
        <div style="display: flex; flex-direction: column; gap: 14px;">
          <h3 style="font-size: 14px; font-weight: 700; color: var(--text-muted); display: flex; align-items: center; justify-content: space-between;">
            <span>スプリント一覧</span>
            <span style="font-size: 12px; font-weight: normal;">{{ sprints.length }}件</span>
          </h3>

          <div v-if="sprints.length === 0" class="card" style="padding: 24px; text-align: center; color: var(--text-muted);">
            スプリントがまだありません。「新規スプリント作成」から計画を作成してください。
          </div>

          <div
            v-for="sp in sprints"
            :key="sp.id"
            class="card"
            :style="{
              padding: '14px 16px',
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
              transition: 'all 0.15s ease',
              border: dragOverSprintId === sp.id ? '2px dashed var(--primary)' : '1px solid var(--border-color)',
              backgroundColor: dragOverSprintId === sp.id ? 'var(--primary-soft)' : 'var(--bg-card)'
            }"
            @dragover.prevent="onDragOverSprint(sp.id)"
            @dragleave="onDragLeaveSprint(sp.id)"
            @drop="onDropToSprint(sp)"
          >
            <!-- Sprint Card Header -->
            <div style="display: flex; align-items: flex-start; justify-content: space-between; gap: 8px;">
              <div>
                <div style="display: flex; align-items: center; gap: 8px;">
                  <span
                    :class="[
                      'badge',
                      sp.status === 'active' ? 'badge-green' : sp.status === 'closed' ? 'badge-slate' : 'badge-blue'
                    ]"
                    style="font-size: 10px;"
                  >
                    {{ sp.status === 'active' ? 'アクティブ' : sp.status === 'closed' ? '完了' : '計画中' }}
                  </span>
                  <strong style="font-size: 14px;">{{ sp.name }}</strong>
                </div>
                <div v-if="sp.goal" style="font-size: 12px; color: var(--text-muted); margin-top: 2px;">
                  {{ sp.goal }}
                </div>
                <div style="font-size: 11px; color: var(--text-muted); margin-top: 2px;">
                  期間: {{ sp.start_date || '未定' }} 〜 {{ sp.end_date || '未定' }}
                </div>
              </div>

              <!-- Sprint Header Actions -->
              <div style="display: flex; align-items: center; gap: 6px;">
                <button
                  v-if="canEdit && sp.status === 'planned'"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  style="font-size: 11px; padding: 2px 7px;"
                  @click="startSprint(sp)"
                >
                  開始
                </button>
                <button
                  v-else-if="canEdit && sp.status === 'active'"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  style="font-size: 11px; padding: 2px 7px;"
                  @click="closeSprint(sp)"
                >
                  完了
                </button>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="font-size: 11px; padding: 2px 5px;"
                  title="バーンダウンチャート"
                  @click="viewSprintBurndown(sp.id)"
                >
                  📈
                </button>
                <button
                  v-if="canEdit"
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="font-size: 11px; padding: 2px 5px;"
                  title="スプリント編集"
                  @click="openEditModal(sp)"
                >
                  ✏️
                </button>
                <button
                  v-if="canEdit"
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="font-size: 11px; padding: 2px 5px; color: var(--danger);"
                  title="スプリント削除"
                  @click="deleteSprint(sp)"
                >
                  🗑
                </button>
              </div>
            </div>

            <!-- Sprint Metrics Bar -->
            <div style="display: flex; flex-wrap: wrap; gap: 8px; font-size: 11px; padding: 6px 10px; background: var(--bg-input); border-radius: 6px;">
              <span>タスク: <strong>{{ getTasksForSprint(sp.id).length }}</strong> 件</span>
              <span>・</span>
              <span>SP: <strong>{{ getSprintTotalSP(sp.id) }}</strong></span>
              <span>・</span>
              <span>予定工数: <strong>{{ formatHours(getSprintTotalPlannedHours(sp.id)) }}</strong>h</span>
            </div>

            <!-- Tasks in this sprint -->
            <div style="display: flex; flex-direction: column; gap: 6px; max-height: 320px; overflow-y: auto;">
              <div v-if="getTasksForSprint(sp.id).length === 0" style="font-size: 12px; color: var(--text-muted); padding: 8px 0; text-align: center;">
                割当タスクはありません（タスクをここにドラッグ＆ドロップ）
              </div>
              <div
                v-for="t in getTasksForSprint(sp.id)"
                :key="t.id"
                :draggable="canEdit"
                :style="{
                  padding: '8px 10px',
                  borderRadius: '6px',
                  border: '1px solid var(--border-color)',
                  background: 'var(--bg-main)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '8px',
                  fontSize: '12px',
                  cursor: canEdit ? 'grab' : 'default',
                  opacity: draggedTaskId === t.id ? 0.4 : 1
                }"
                @dragstart="onDragStart($event, t)"
                @dragend="onDragEnd"
              >
                <div style="display: flex; align-items: center; gap: 6px; overflow: hidden;">
                  <span
                    :style="{
                      width: '8px',
                      height: '8px',
                      borderRadius: '50%',
                      backgroundColor: getStatusColor(t.status_id),
                      flexShrink: 0
                    }"
                  ></span>
                  <span
                    style="font-weight: 500; cursor: pointer; color: var(--text-main); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"
                    @click="$emit('open-task', t)"
                  >
                    {{ t.title }}
                  </span>
                  <span v-if="t.story_points" class="badge badge-purple" style="font-size: 10px; padding: 1px 4px; flex-shrink: 0;">
                    {{ t.story_points }} SP
                  </span>
                </div>

                <div style="display: flex; align-items: center; gap: 6px; flex-shrink: 0;">
                  <button
                    v-if="canEdit"
                    type="button"
                    class="btn btn-ghost btn-sm"
                    style="font-size: 10px; padding: 1px 4px; color: var(--text-muted);"
                    title="バックログに戻す"
                    @click="unassignTask(t)"
                  >
                    バックログへ戻す
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Column 2: Unassigned Backlog -->
        <div style="display: flex; flex-direction: column; gap: 14px;">
          <h3 style="font-size: 14px; font-weight: 700; color: var(--text-muted); display: flex; align-items: center; justify-content: space-between;">
            <span>未割当バックログ</span>
            <span style="font-size: 12px; font-weight: normal;">{{ backlogTasks.length }}タスク ({{ backlogStoryPoints }} SP)</span>
          </h3>

          <div
            class="card"
            :style="{
              padding: '14px 16px',
              display: 'flex',
              flexDirection: 'column',
              gap: '10px',
              transition: 'all 0.15s ease',
              border: dragOverBacklog ? '2px dashed var(--primary)' : '1px solid var(--border-color)',
              backgroundColor: dragOverBacklog ? 'var(--primary-soft)' : 'var(--bg-card)'
            }"
            @dragover.prevent="onDragOverBacklog"
            @dragleave="onDragLeaveBacklog"
            @drop="onDropToBacklog"
          >
            <div v-if="backlogTasks.length === 0" style="padding: 30px; text-align: center; color: var(--text-muted); font-size: 13px;">
              未割当のタスクはありません（スプリントからここにドロップして解除）
            </div>

            <div
              v-for="t in backlogTasks"
              :key="t.id"
              :draggable="canEdit"
              :style="{
                padding: '10px 12px',
                borderRadius: '6px',
                border: '1px solid var(--border-color)',
                background: 'var(--bg-main)',
                display: 'flex',
                flexDirection: 'column',
                gap: '6px',
                cursor: canEdit ? 'grab' : 'default',
                opacity: draggedTaskId === t.id ? 0.4 : 1
              }"
              @dragstart="onDragStart($event, t)"
              @dragend="onDragEnd"
            >
              <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
                <div style="display: flex; align-items: center; gap: 6px; overflow: hidden;">
                  <span
                    :style="{
                      width: '8px',
                      height: '8px',
                      borderRadius: '50%',
                      backgroundColor: getStatusColor(t.status_id),
                      flexShrink: 0
                    }"
                  ></span>
                  <span
                    style="font-weight: 600; cursor: pointer; color: var(--text-main); font-size: 13px;"
                    @click="$emit('open-task', t)"
                  >
                    {{ t.title }}
                  </span>
                </div>

                <div style="display: flex; align-items: center; gap: 6px; flex-shrink: 0;">
                  <span v-if="t.story_points" class="badge badge-purple" style="font-size: 11px;">
                    {{ t.story_points }} SP
                  </span>
                  <span class="badge badge-slate" style="font-size: 11px;">
                    {{ formatHours(t.planned_hours) }}h
                  </span>
                </div>
              </div>

              <!-- Quick Assign to Sprint dropdown -->
              <div v-if="canEdit && sprints.length > 0" style="display: flex; align-items: center; justify-content: flex-end; gap: 6px; font-size: 11px;">
                <span style="color: var(--text-muted);">スプリントへ割当:</span>
                <select
                  style="font-size: 11px; padding: 2px 6px;"
                  @change="onAssignDropdownChange(t, $event)"
                >
                  <option value="">選択してください...</option>
                  <option v-for="sp in sprints.filter(s => s.status !== 'closed')" :key="sp.id" :value="sp.id">
                    {{ sp.status === 'active' ? '⚡ ' : '📅 ' }}{{ sp.name }}
                  </option>
                </select>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- SUBVIEW 2: BURNDOWN CHART AREA -->
    <div v-else-if="subTab === 'burndown'" class="card" style="padding: 16px; display: flex; flex-direction: column; gap: 16px;">
      <!-- Chart Controls Header -->
      <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
        <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 10px;">
          <span style="font-size: 13px; font-weight: 600; color: var(--text-muted);">対象スプリント:</span>
          <select v-model="selectedSprintId" style="font-size: 13px; padding: 4px 10px;" @change="loadBurndown">
            <option v-for="sp in sprints" :key="sp.id" :value="sp.id">
              {{ sp.status === 'active' ? '⚡ ' : sp.status === 'closed' ? '✓ ' : '📅 ' }}{{ sp.name }} ({{ sp.status }})
            </option>
          </select>

          <!-- Metric Basis Toggle: SP vs Hours -->
          <div style="display: flex; gap: 4px; padding: 2px; border-radius: 6px; background: var(--bg-card); border: 1px solid var(--border-color);">
            <button
              type="button"
              class="btn btn-sm"
              :class="burndownMetric === 'sp' ? 'btn-primary' : 'btn-ghost'"
              style="font-size: 11px;"
              @click="burndownMetric = 'sp'"
            >
              ストーリーポイント (SP)
            </button>
            <button
              type="button"
              class="btn btn-sm"
              :class="burndownMetric === 'hours' ? 'btn-primary' : 'btn-ghost'"
              style="font-size: 11px;"
              @click="burndownMetric = 'hours'"
            >
              予定・残余時間 (h)
            </button>
          </div>
        </div>

        <!-- Legend -->
        <div style="display: flex; align-items: center; gap: 14px; font-size: 12px;">
          <div style="display: flex; align-items: center; gap: 6px;">
            <span style="display: inline-block; width: 18px; height: 0; border-top: 2px dashed #94a3b8;"></span>
            <span style="color: var(--text-muted);">理想バーンダウン線</span>
          </div>
          <div style="display: flex; align-items: center; gap: 6px;">
            <span style="display: inline-block; width: 18px; height: 3px; background-color: var(--primary); border-radius: 2px;"></span>
            <span style="font-weight: 600; color: var(--primary);">残余実績線</span>
          </div>
        </div>
      </div>

      <div v-if="isLoadingBurndown" style="padding: 40px; text-align: center; color: var(--text-muted);">
        バーンダウンデータを読み込み中...
      </div>

      <div v-else-if="!burndownData || burndownData.points.length === 0" style="padding: 40px; text-align: center; color: var(--text-muted);">
        このスプリントのバーンダウンデータがありません。開始日と終了日を設定し、タスクを割り当ててください。
      </div>

      <!-- SVG Burndown Chart Viewport -->
      <div v-else style="display: flex; flex-direction: column; gap: 10px;">
        <div style="overflow-x: auto; width: 100%;">
          <svg
            :viewBox="`0 0 ${chartWidth} ${chartHeight}`"
            style="width: 100%; height: auto; min-width: 600px; max-height: 380px; background: var(--bg-card); border-radius: 8px; border: 1px solid var(--border-color);"
          >
            <!-- Background Grid Lines -->
            <g>
              <line
                v-for="(tick, idx) in yTicks"
                :key="`ytick-${idx}`"
                :x1="paddingLeft"
                :y1="tick.y"
                :x2="chartWidth - paddingRight"
                :y2="tick.y"
                stroke="var(--border-color)"
                stroke-width="1"
                stroke-dasharray="2,2"
              />
              <text
                v-for="(tick, idx) in yTicks"
                :key="`ytext-${idx}`"
                :x="paddingLeft - 8"
                :y="tick.y + 4"
                text-anchor="end"
                font-size="10"
                fill="var(--text-muted)"
              >
                {{ tick.val }}
              </text>
            </g>

            <!-- X Axis Grid & Date Labels -->
            <g>
              <line
                :x1="paddingLeft"
                :y1="chartHeight - paddingBottom"
                :x2="chartWidth - paddingRight"
                :y2="chartHeight - paddingBottom"
                stroke="var(--border-color)"
                stroke-width="1.5"
              />
              <text
                v-for="(pt, idx) in chartPoints"
                :key="`xtext-${idx}`"
                :x="pt.x"
                :y="chartHeight - paddingBottom + 16"
                text-anchor="middle"
                font-size="10"
                fill="var(--text-muted)"
              >
                {{ pt.date.slice(5) }}
              </text>
            </g>

            <!-- Ideal Line (Dashed) -->
            <path
              :d="idealLinePath"
              fill="none"
              stroke="#94a3b8"
              stroke-width="2"
              stroke-dasharray="5,5"
            />

            <!-- Remaining Line (Solid Primary) -->
            <path
              :d="remainingLinePath"
              fill="none"
              stroke="var(--primary)"
              stroke-width="3"
              stroke-linejoin="round"
              stroke-linecap="round"
            />

            <!-- Data Point Dots -->
            <g>
              <circle
                v-for="(pt, idx) in chartPoints"
                :key="`dot-${idx}`"
                :cx="pt.x"
                :cy="pt.yRemaining"
                r="4.5"
                fill="#fff"
                stroke="var(--primary)"
                stroke-width="2.5"
                style="cursor: pointer;"
              >
                <title>{{ pt.date }}: 残余 {{ pt.valRemaining }} (理想: {{ pt.valIdeal }})</title>
              </circle>
            </g>
          </svg>
        </div>

        <div style="display: flex; justify-content: space-between; font-size: 12px; color: var(--text-muted); padding: 0 4px;">
          <span>開始時総計: <strong>{{ burndownMetric === 'sp' ? (burndownData.total_story_points || 0) + ' SP' : formatHours(burndownData.total_hours) + 'h' }}</strong></span>
          <span>最終残余: <strong>{{ lastRemainingValue }}</strong></span>
        </div>
      </div>
    </div>

    <!-- Create / Edit Sprint Modal -->
    <div
      v-if="modalOpen"
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
      @click.self="modalOpen = false"
    >
      <div class="card" style="width: 100%; max-width: 480px; padding: 20px; display: flex; flex-direction: column; gap: 14px;">
        <h3 style="font-size: 16px; font-weight: 700;">
          {{ isEditing ? 'スプリント編集' : '新規スプリント作成' }}
        </h3>

        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="saveSprint">
          <div>
            <label class="field-label">スプリント名 *</label>
            <input v-model="sprintForm.name" type="text" required style="width: 100%;" placeholder="例: Sprint 1 (Sprint Goal)" />
          </div>

          <div>
            <label class="field-label">スプリントゴール (目標)</label>
            <input v-model="sprintForm.goal" type="text" style="width: 100%;" placeholder="例: コア機能MVPの完成" />
          </div>

          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">開始日</label>
              <input v-model="sprintForm.start_date" type="date" style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">終了日</label>
              <input v-model="sprintForm.end_date" type="date" style="width: 100%;" />
            </div>
          </div>

          <div>
            <label class="field-label">状態</label>
            <select v-model="sprintForm.status" style="width: 100%;">
              <option value="planned">計画中 (planned)</option>
              <option value="active">アクティブ (active)</option>
              <option value="closed">完了 (closed)</option>
            </select>
          </div>

          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px;">
            <button type="button" class="btn btn-secondary" @click="modalOpen = false">キャンセル</button>
            <button type="submit" class="btn btn-primary" :disabled="isSavingSprint">
              {{ isSavingSprint ? '保存中...' : '保存' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Sprint, SprintBurndown, Status, Task } from '~/types/polaris'
import { formatHours } from '~/utils/format'

const props = defineProps<{
  projectId: string
  sprints: Sprint[]
  allTasks: Task[]
  statuses: Status[]
  canEdit: boolean
}>()

const emit = defineEmits<{
  (e: 'open-task', task: Task): void
  (e: 'refresh'): void
}>()

const { apiFetch, notifyToast } = useApi()

const subTab = ref<'planning' | 'burndown'>('planning')
const selectedSprintId = ref<string>('')
const burndownMetric = ref<'sp' | 'hours'>('sp')
const isLoadingBurndown = ref(false)
const burndownData = ref<SprintBurndown | null>(null)

// Modal State
const modalOpen = ref(false)
const isEditing = ref(false)
const editingSprintId = ref<string | null>(null)
const isSavingSprint = ref(false)

const sprintForm = reactive({
  name: '',
  goal: '',
  start_date: '',
  end_date: '',
  status: 'planned' as 'planned' | 'active' | 'closed'
})

const activeSprint = computed(() => {
  return props.sprints.find(s => s.status === 'active') || null
})

const activeSprintTasks = computed(() => {
  if (!activeSprint.value) return []
  return props.allTasks.filter(t => t.sprint_id === activeSprint.value!.id)
})

const activeSprintStoryPoints = computed(() => {
  return activeSprintTasks.value.reduce((acc, t) => acc + (Number(t.story_points) || 0), 0)
})

const activeSprintPlannedHours = computed(() => {
  return activeSprintTasks.value.reduce((acc, t) => acc + (Number(t.planned_hours) || 0), 0)
})

const backlogTasks = computed(() => {
  return props.allTasks.filter(t => !t.sprint_id)
})

const backlogStoryPoints = computed(() => {
  return backlogTasks.value.reduce((acc, t) => acc + (Number(t.story_points) || 0), 0)
})

function getTasksForSprint(sprintId: string): Task[] {
  return props.allTasks.filter(t => t.sprint_id === sprintId)
}

function getSprintTotalSP(sprintId: string): number {
  return getTasksForSprint(sprintId).reduce((acc, t) => acc + (Number(t.story_points) || 0), 0)
}

function getSprintTotalPlannedHours(sprintId: string): number {
  return getTasksForSprint(sprintId).reduce((acc, t) => acc + (Number(t.planned_hours) || 0), 0)
}

function getStatusColor(statusId: string): string {
  return props.statuses.find(s => s.id === statusId)?.color || '#94a3b8'
}

// Sprint Actions
function openCreateModal() {
  isEditing.value = false
  editingSprintId.value = null
  sprintForm.name = `Sprint ${props.sprints.length + 1}`
  sprintForm.goal = ''
  sprintForm.start_date = new Date().toISOString().slice(0, 10)
  const d = new Date()
  d.setDate(d.getDate() + 14)
  sprintForm.end_date = d.toISOString().slice(0, 10)
  sprintForm.status = 'planned'
  modalOpen.value = true
}

function openEditModal(sp: Sprint) {
  isEditing.value = true
  editingSprintId.value = sp.id
  sprintForm.name = sp.name
  sprintForm.goal = sp.goal || ''
  sprintForm.start_date = sp.start_date || ''
  sprintForm.end_date = sp.end_date || ''
  sprintForm.status = sp.status
  modalOpen.value = true
}

async function saveSprint() {
  if (!sprintForm.name.trim()) return
  isSavingSprint.value = true
  try {
    if (isEditing.value && editingSprintId.value) {
      await apiFetch(`/sprints/${editingSprintId.value}`, {
        method: 'PUT',
        body: JSON.stringify(sprintForm)
      })
      notifyToast('スプリントを更新しました', 'success')
    } else {
      await apiFetch(`/projects/${props.projectId}/sprints`, {
        method: 'POST',
        body: JSON.stringify(sprintForm)
      })
      notifyToast('スプリントを作成しました', 'success')
    }
    modalOpen.value = false
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'スプリントの保存に失敗しました', 'error')
  } finally {
    isSavingSprint.value = false
  }
}

async function startSprint(sp: Sprint) {
  if (!confirm(`スプリント「${sp.name}」を開始（アクティブ化）しますか？`)) return
  try {
    await apiFetch(`/sprints/${sp.id}`, {
      method: 'PUT',
      body: JSON.stringify({ status: 'active' })
    })
    notifyToast(`スプリント「${sp.name}」を開始しました`, 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'スプリントの開始に失敗しました', 'error')
  }
}

async function closeSprint(sp: Sprint) {
  if (!confirm(`スプリント「${sp.name}」を完了（クローズ）しますか？`)) return
  try {
    await apiFetch(`/sprints/${sp.id}`, {
      method: 'PUT',
      body: JSON.stringify({ status: 'closed' })
    })
    notifyToast(`スプリント「${sp.name}」を完了しました`, 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'スプリントの完了に失敗しました', 'error')
  }
}

async function deleteSprint(sp: Sprint) {
  if (!confirm(`スプリント「${sp.name}」を削除しますか？配下のタスクは未割当バックログに戻ります。`)) return
  try {
    await apiFetch(`/sprints/${sp.id}`, {
      method: 'DELETE'
    })
    notifyToast('スプリントを削除しました', 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'スプリントの削除に失敗しました', 'error')
  }
}

async function unassignTask(task: Task) {
  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ sprint_id: null })
    })
    notifyToast('タスクをバックログに戻しました', 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'タスクの更新に失敗しました', 'error')
  }
}

async function onAssignDropdownChange(task: Task, event: Event) {
  const target = event.target as HTMLSelectElement
  const sprintId = target.value
  if (!sprintId) return
  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ sprint_id: sprintId })
    })
    notifyToast('タスクをスプリントに割り当てました', 'success')
    target.value = ''
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'タスクの割当に失敗しました', 'error')
  }
}

// Drag and Drop (DnD) Support
const draggedTaskId = ref<string | null>(null)
const draggedTask = ref<Task | null>(null)
const dragOverSprintId = ref<string | null>(null)
const dragOverBacklog = ref(false)

function onDragStart(event: DragEvent, task: Task) {
  if (!props.canEdit) return
  draggedTaskId.value = task.id
  draggedTask.value = task
  if (event.dataTransfer) {
    event.dataTransfer.setData('text/plain', task.id)
    event.dataTransfer.effectAllowed = 'move'
  }
}

function onDragEnd() {
  draggedTaskId.value = null
  draggedTask.value = null
  dragOverSprintId.value = null
  dragOverBacklog.value = false
}

function onDragOverSprint(sprintId: string) {
  if (!props.canEdit || !draggedTask.value) return
  dragOverSprintId.value = sprintId
}

function onDragLeaveSprint(sprintId: string) {
  if (dragOverSprintId.value === sprintId) {
    dragOverSprintId.value = null
  }
}

async function onDropToSprint(sprint: Sprint) {
  if (!props.canEdit || !draggedTask.value) return
  const task = draggedTask.value
  dragOverSprintId.value = null
  if (task.sprint_id === sprint.id) return

  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ sprint_id: sprint.id })
    })
    notifyToast(`タスク「${task.title}」を「${sprint.name}」に割り当てました`, 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'タスクの割当に失敗しました', 'error')
  } finally {
    onDragEnd()
  }
}

function onDragOverBacklog() {
  if (!props.canEdit || !draggedTask.value) return
  dragOverBacklog.value = true
}

function onDragLeaveBacklog() {
  dragOverBacklog.value = false
}

async function onDropToBacklog() {
  if (!props.canEdit || !draggedTask.value) return
  const task = draggedTask.value
  dragOverBacklog.value = false
  if (!task.sprint_id) return

  try {
    await apiFetch(`/projects/${props.projectId}/tasks/${task.id}`, {
      method: 'PUT',
      body: JSON.stringify({ sprint_id: null })
    })
    notifyToast(`タスク「${task.title}」を未割当バックログに戻しました`, 'success')
    emit('refresh')
  } catch (e: any) {
    notifyToast(e.message || 'バックログへの移動に失敗しました', 'error')
  } finally {
    onDragEnd()
  }
}

// Burndown Chart
function viewSprintBurndown(sprintId: string) {
  selectedSprintId.value = sprintId
  subTab.value = 'burndown'
  loadBurndown()
}

watch(
  () => props.sprints,
  (list) => {
    if (!selectedSprintId.value && list.length > 0) {
      const act = list.find(s => s.status === 'active')
      selectedSprintId.value = act ? act.id : list[0].id
    }
  },
  { immediate: true }
)

async function loadBurndown() {
  if (!selectedSprintId.value) return
  isLoadingBurndown.value = true
  try {
    const data = await apiFetch<SprintBurndown>(`/sprints/${selectedSprintId.value}/burndown`)
    burndownData.value = data
  } catch (e: any) {
    burndownData.value = null
  } finally {
    isLoadingBurndown.value = false
  }
}

watch(subTab, (tab) => {
  if (tab === 'burndown' && !burndownData.value) {
    loadBurndown()
  }
})

// SVG Chart Metrics
const chartWidth = 640
const chartHeight = 320
const paddingLeft = 50
const paddingRight = 30
const paddingTop = 30
const paddingBottom = 40

const maxMetricVal = computed(() => {
  if (!burndownData.value || !burndownData.value.points.length) return 10
  const isSp = burndownMetric.value === 'sp'
  const maxInitial = isSp ? (burndownData.value.total_story_points || 0) : (burndownData.value.total_hours || 0)
  let maxPts = maxInitial
  for (const pt of burndownData.value.points) {
    const v = isSp ? Math.max(pt.ideal_story_points || 0, pt.remaining_story_points || 0) : Math.max(pt.ideal_hours || 0, pt.remaining_hours || 0)
    if (v > maxPts) maxPts = v
  }
  return Math.max(maxPts, 1)
})

const yTicks = computed(() => {
  const max = maxMetricVal.value
  const steps = 4
  const ticks: { val: number; y: number }[] = []
  const plotH = chartHeight - paddingTop - paddingBottom
  for (let i = 0; i <= steps; i++) {
    const val = Math.round((max / steps) * i * 10) / 10
    const ratio = val / max
    const y = chartHeight - paddingBottom - ratio * plotH
    ticks.push({ val, y })
  }
  return ticks
})

const chartPoints = computed(() => {
  if (!burndownData.value || !burndownData.value.points.length) return []
  const pts = burndownData.value.points
  const isSp = burndownMetric.value === 'sp'
  const max = maxMetricVal.value
  const plotW = chartWidth - paddingLeft - paddingRight
  const plotH = chartHeight - paddingTop - paddingBottom
  const denom = Math.max(pts.length - 1, 1)

  return pts.map((p, idx) => {
    const x = paddingLeft + (idx / denom) * plotW
    const valIdeal = isSp ? (p.ideal_story_points ?? 0) : (p.ideal_hours ?? 0)
    const valRemaining = isSp ? (p.remaining_story_points ?? 0) : (p.remaining_hours ?? 0)
    const yIdeal = chartHeight - paddingBottom - (Math.max(valIdeal, 0) / max) * plotH
    const yRemaining = chartHeight - paddingBottom - (Math.max(valRemaining, 0) / max) * plotH
    return {
      date: p.date,
      x,
      yIdeal,
      yRemaining,
      valIdeal,
      valRemaining
    }
  })
})

const idealLinePath = computed(() => {
  if (chartPoints.value.length === 0) return ''
  return chartPoints.value.map((p, idx) => `${idx === 0 ? 'M' : 'L'} ${p.x} ${p.yIdeal}`).join(' ')
})

const remainingLinePath = computed(() => {
  if (chartPoints.value.length === 0) return ''
  return chartPoints.value.map((p, idx) => `${idx === 0 ? 'M' : 'L'} ${p.x} ${p.yRemaining}`).join(' ')
})

const lastRemainingValue = computed(() => {
  if (chartPoints.value.length === 0) return '-'
  const last = chartPoints.value[chartPoints.value.length - 1]
  return burndownMetric.value === 'sp' ? `${last.valRemaining} SP` : `${formatHours(last.valRemaining)}h`
})
</script>
