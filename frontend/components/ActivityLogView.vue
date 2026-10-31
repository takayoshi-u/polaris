<template>
  <div class="card" style="display: flex; flex-direction: column; gap: 14px; padding: 16px;">
    <!-- Top Filter Header -->
    <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
      <div style="display: flex; align-items: center; gap: 8px;">
        <h2 style="font-size: 15px; font-weight: 700; display: flex; align-items: center; gap: 6px; margin: 0;">
          <span>⏱️ 活動履歴ログ (Activity Log)</span>
          <span class="badge badge-slate" style="font-size: 11px;">{{ logs.length }}件</span>
        </h2>
      </div>

      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
        <!-- Entity Filter (only shown if not scoped to specific entity) -->
        <div v-if="!entityId" style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">対象:</span>
          <select v-model="selectedEntityType" style="font-size: 12px; padding: 3px 8px;" @change="loadLogs">
            <option value="">すべて</option>
            <option value="task">タスク (Task)</option>
            <option value="issue">課題 (Issue)</option>
            <option value="sprint">スプリント (Sprint)</option>
            <option value="wiki">Wiki</option>
            <option value="inbound_webhook">Inbound Webhook</option>
          </select>
        </div>

        <div style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">件数:</span>
          <select v-model="limitCount" style="font-size: 12px; padding: 3px 8px;" @change="loadLogs">
            <option :value="20">20件</option>
            <option :value="50">50件</option>
            <option :value="100">100件</option>
          </select>
        </div>

        <button type="button" class="btn btn-secondary btn-sm" :disabled="isLoading" @click="loadLogs">
          🔄 {{ isLoading ? '更新中...' : '再読み込み' }}
        </button>
      </div>
    </div>

    <!-- Timeline Stream List -->
    <div style="position: relative; display: flex; flex-direction: column; gap: 12px; padding-left: 20px;">
      <!-- Vertical connecting line -->
      <div
        v-if="logs.length > 0"
        style="
          position: absolute;
          top: 8px;
          bottom: 8px;
          left: 28px;
          width: 2px;
          background: var(--border-color);
          z-index: 1;
        "
      ></div>

      <div v-if="isLoading" style="padding: 40px; text-align: center; color: var(--text-muted);">
        活動履歴を読み込み中...
      </div>

      <div v-else-if="logs.length === 0" style="padding: 40px; text-align: center; color: var(--text-muted); font-size: 13px;">
        活動履歴がありません
      </div>

      <div
        v-for="log in logs"
        :key="log.id"
        style="
          position: relative;
          z-index: 2;
          display: flex;
          align-items: flex-start;
          gap: 12px;
          padding: 10px 14px;
          border-radius: 8px;
          background: var(--bg-card);
          border: 1px solid var(--border-color);
          font-size: 13px;
        "
      >
        <!-- Action Avatar / Icon Indicator -->
        <div
          :style="{
            width: '28px',
            height: '28px',
            borderRadius: '50%',
            backgroundColor: getActionColor(log.action) + '22',
            color: getActionColor(log.action),
            border: `1px solid ${getActionColor(log.action)}`,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            fontSize: '12px',
            flexShrink: 0
          }"
        >
          {{ getActionIcon(log.action) }}
        </div>

        <!-- Log Content Body -->
        <div style="display: flex; flex-direction: column; gap: 4px; flex: 1; overflow: hidden;">
          <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px;">
            <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 6px;">
              <strong style="color: var(--text-main);">
                {{ log.actor_name || getUserName(log.actor_id || log.user_id) }}
              </strong>
              <span style="color: var(--text-muted);">が</span>
              <span :class="['badge', getEntityBadgeClass(log.entity_type)]" style="font-size: 10px;">
                {{ getEntityTypeName(log.entity_type) }}
              </span>
              <span style="font-weight: 600;">{{ formatActionText(log.action) }}</span>
            </div>

            <span style="font-size: 11px; color: var(--text-muted); white-space: nowrap;">
              {{ formatRelativeTime(log.created_at) }}
            </span>
          </div>

          <!-- Entity reference or details title -->
          <div v-if="log.details?.title || log.details?.name" style="font-size: 12px; font-weight: 500; color: var(--primary);">
            「{{ log.details?.title || log.details?.name }}」
          </div>

          <!-- Before / After Diff Tags -->
          <div v-if="hasDiffChanges(log)" style="display: flex; flex-wrap: wrap; gap: 6px; margin-top: 4px;">
            <div
              v-for="(diff, key) in extractDiffChanges(log)"
              :key="key"
              style="
                display: inline-flex;
                align-items: center;
                gap: 6px;
                padding: 2px 8px;
                border-radius: 4px;
                background: var(--bg-input);
                border: 1px solid var(--border-color);
                font-size: 11px;
              "
            >
              <span style="color: var(--text-muted); font-weight: 600;">{{ formatFieldName(key) }}:</span>
              <span style="text-decoration: line-through; opacity: 0.7;">{{ formatDiffVal(diff.from) }}</span>
              <span>➔</span>
              <strong style="color: var(--primary);">{{ formatDiffVal(diff.to) }}</strong>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ActivityLog, User } from '~/types/polaris'
import { formatRelativeTime } from '~/utils/format'

const props = defineProps<{
  projectId: string
  entityType?: string
  entityId?: string
  users?: User[]
}>()

const { apiFetch } = useApi()

const logs = ref<ActivityLog[]>([])
const isLoading = ref(false)
const selectedEntityType = ref(props.entityType || '')
const limitCount = ref(50)

function getUserName(userId?: string | null): string {
  if (!userId) return 'システム'
  if (!props.users) return userId.slice(0, 8)
  return props.users.find(u => u.id === userId)?.display_name || userId.slice(0, 8)
}

function getEntityTypeName(type: string): string {
  const map: Record<string, string> = {
    task: 'タスク',
    issue: '課題',
    sprint: 'スプリント',
    wiki: 'Wiki',
    dependency: '依存関係',
    inbound_webhook: 'Webhook',
    comment: 'コメント'
  }
  return map[type] || type
}

function getEntityBadgeClass(type: string): string {
  const map: Record<string, string> = {
    task: 'badge-blue',
    issue: 'badge-green',
    sprint: 'badge-purple',
    wiki: 'badge-slate',
    comment: 'badge-amber'
  }
  return map[type] || 'badge-slate'
}

function getActionColor(action: string): string {
  if (action.includes('create') || action.includes('add')) return '#16a34a' // green
  if (action.includes('delete') || action.includes('remove')) return '#dc2626' // red
  if (action.includes('close') || action.includes('complete')) return '#475569' // slate
  return '#2563eb' // blue for update / change
}

function getActionIcon(action: string): string {
  if (action.includes('create') || action.includes('add')) return '＋'
  if (action.includes('delete') || action.includes('remove')) return '🗑'
  if (action.includes('comment')) return '💬'
  if (action.includes('status')) return '●'
  if (action.includes('close')) return '✓'
  return '✏️'
}

function formatActionText(action: string): string {
  const map: Record<string, string> = {
    'created': 'を作成しました',
    'updated': 'を更新しました',
    'deleted': 'を削除しました',
    'status_changed': 'のステータスを変更しました',
    'assignee_changed': 'の担当者を変更しました',
    'comment_added': 'にコメントを投稿しました',
    'dates_changed': 'の期間を変更しました',
    'sprint_started': 'を開始しました',
    'sprint_closed': 'を完了しました'
  }
  return map[action] || action
}

function formatFieldName(key: string): string {
  const map: Record<string, string> = {
    status: 'ステータス',
    progress: '進捗率',
    planned_hours: '予定工数',
    actual_hours: '実績工数',
    story_points: 'SP',
    assignees: '担当者',
    planned_dates: '予定期間',
    title: 'タイトル'
  }
  return map[key] || key
}

function formatDiffVal(val: any): string {
  if (val === null || val === undefined || val === '') return '(なし)'
  if (typeof val === 'boolean') return val ? '有効' : '無効'
  return String(val)
}

function hasDiffChanges(log: ActivityLog): boolean {
  if (!log.details) return false
  if (log.details.changes && Object.keys(log.details.changes).length > 0) return true
  if (log.details.diff && Object.keys(log.details.diff).length > 0) return true
  return false
}

function extractDiffChanges(log: ActivityLog): Record<string, { from: any; to: any }> {
  const d = log.details || {}
  const raw = d.changes || d.diff || {}
  const out: Record<string, { from: any; to: any }> = {}
  for (const [k, v] of Object.entries(raw)) {
    if (v && typeof v === 'object' && ('from' in v || 'to' in v)) {
      out[k] = { from: (v as any).from, to: (v as any).to }
    } else if (v && typeof v === 'object' && ('old' in v || 'new' in v)) {
      out[k] = { from: (v as any).old, to: (v as any).new }
    }
  }
  return out
}

async function loadLogs() {
  if (!props.projectId) return
  isLoading.value = true
  const params = new URLSearchParams()
  const eType = props.entityType || selectedEntityType.value
  if (eType) params.set('entity_type', eType)
  const eId = props.entityId
  if (eId) params.set('entity_id', eId)
  if (limitCount.value) params.set('limit', String(limitCount.value))

  try {
    const list = await apiFetch<ActivityLog[]>(`/projects/${props.projectId}/activity?${params.toString()}`)
    logs.value = list || []
  } catch (e) {
    logs.value = []
  } finally {
    isLoading.value = false
  }
}

watch(() => [props.projectId, props.entityType, props.entityId], () => loadLogs(), { immediate: true })
</script>
