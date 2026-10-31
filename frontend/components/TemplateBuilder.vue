<template>
  <div class="card" style="display: flex; flex-direction: column; gap: 16px;">
    <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px; border-bottom: 1px solid var(--border-color); padding-bottom: 12px;">
      <div>
        <h3 style="font-size: 15px; font-weight: 700;">
          {{ title || 'テンプレート ＆ レイアウトビルダー' }}
        </h3>
        <p style="font-size: 13px; color: var(--text-muted);">
          行ごとに1〜3カラムを設定し、標準必須項目とカスタム項目を配置します。骨格項目は常時閲覧可（編集制限のみ可）です。
        </p>
      </div>
      <div style="display: flex; gap: 8px;">
        <button class="btn btn-secondary btn-sm" @click="startNewTemplate('issue')">＋ 新規課題テンプレート</button>
        <button class="btn btn-secondary btn-sm" @click="startNewTemplate('task')">＋ 新規タスクテンプレート</button>
      </div>
    </div>

    <!-- Existing Templates List -->
    <div style="display: flex; flex-wrap: wrap; gap: 8px;">
      <div
        v-for="t in templates"
        :key="t.id"
        style="display: flex; align-items: center; gap: 8px; padding: 6px 12px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input); cursor: pointer;"
        :style="{ borderColor: editingTemplate?.id === t.id ? '#3b82f6' : 'var(--border-color)' }"
        @click="selectTemplate(t)"
      >
        <span :class="['badge', t.target_type === 'issue' ? 'badge-green' : 'badge-blue']">
          {{ t.target_type === 'issue' ? '課題' : 'タスク' }}
        </span>
        <span style="font-size: 13px; font-weight: 600;">{{ t.name }}</span>
        <button
          v-if="templates.filter(x => x.target_type === t.target_type).length > 1"
          class="btn btn-danger btn-sm"
          style="padding: 2px 6px; font-size: 12px;"
          @click.stop="emit('delete', t.id)"
        >
          削除
        </button>
      </div>
    </div>

    <!-- Active Template Editor -->
    <div v-if="editingTemplate" style="border-top: 1px solid var(--border-color); padding-top: 14px; display: flex; flex-direction: column; gap: 14px;">
      <div style="display: grid; grid-template-columns: 1fr 180px auto; gap: 12px; align-items: end;">
        <div>
          <label class="field-label">テンプレート名 *</label>
          <input v-model="editingTemplate.name" type="text" style="width: 100%;" />
        </div>
        <div>
          <label class="field-label">対象種別</label>
          <input :value="editingTemplate.target_type === 'issue' ? '課題 (Issue)' : 'タスク (Task)'" disabled style="width: 100%;" />
        </div>
        <div style="display: flex; gap: 8px;">
          <button class="btn btn-secondary" @click="addRow">＋ 行を追加</button>
          <button class="btn btn-primary" :disabled="missingRequiredFields.length > 0" @click="saveCurrentTemplate">テンプレートを保存</button>
        </div>
      </div>

      <!-- Missing Required Standard Fields Warning -->
      <div v-if="missingRequiredFields.length > 0" style="padding: 10px 12px; border-radius: 8px; background: var(--danger-soft); border: 1px solid var(--danger); font-size: 13px; color: var(--danger);">
        ⚠️ 必須の標準項目が未配置です: <strong>{{ missingRequiredFields.map(fieldDisplayName).join(', ') }}</strong>
      </div>
      <div v-else style="padding: 8px 12px; border-radius: 8px; background: var(--success-soft); font-size: 13px; color: var(--success);">
        ✓ 必須の標準フィールドがすべてレイアウト内に配置されています
      </div>

      <!-- Rows -->
      <div style="display: flex; flex-direction: column; gap: 12px;">
        <div
          v-for="(row, rIdx) in editingTemplate.rows"
          :key="rIdx"
          style="padding: 12px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input); display: flex; flex-direction: column; gap: 10px;"
          @dragover.prevent
          @drop="onRowDrop(rIdx)"
        >
          <div style="display: flex; align-items: center; justify-content: space-between;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <span
                style="cursor: grab; font-size: 16px; user-select: none; color: var(--text-muted); padding: 2px 4px;"
                title="行をドラッグして並び替え"
                draggable="true"
                @dragstart="onRowDragStart(rIdx)"
              >
                ⠿
              </span>
              <span style="font-size: 13px; font-weight: 700;">Row {{ rIdx + 1 }} （{{ row.columns_count }} カラム構成）</span>
            </div>
            <div style="display: flex; align-items: center; gap: 6px;">
              <span style="font-size: 12px; color: var(--text-muted);">カラム数:</span>
              <button
                v-for="c in [1, 2, 3]"
                :key="c"
                type="button"
                class="btn btn-sm"
                :class="row.columns_count === c ? 'btn-primary' : 'btn-secondary'"
                @click="setRowColumns(row, c)"
              >
                {{ c }}列
              </button>
              <button
                v-if="editingTemplate.rows.length > 1"
                type="button"
                class="btn btn-danger btn-sm"
                @click="removeRow(rIdx)"
              >
                行削除
              </button>
            </div>
          </div>

          <div :class="`tpl-row-${row.columns_count}`">
            <div
              v-for="(f, fIdx) in row.fields"
              :key="fIdx"
              style="padding: 10px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-card); display: flex; flex-direction: column; gap: 8px;"
            >
              <div style="display: flex; justify-content: space-between; align-items: center;">
                <span style="font-size: 12px; font-weight: 700; color: var(--text-muted);">列 {{ fIdx + 1 }}</span>
                <span v-if="isSkeleton(f)" class="badge badge-blue" style="font-size: 11px;">骨格項目(常時閲覧可)</span>
              </div>

              <!-- Field Selection -->
              <select
                :value="`${f.field_source}:${f.field_key}`"
                style="width: 100%;"
                @change="onChangeFieldKey(f, ($event.target as HTMLSelectElement).value)"
              >
                <optgroup label="標準フィールド">
                  <option
                    v-for="stdKey in availableStandardFields"
                    :key="stdKey"
                    :value="`standard:${stdKey}`"
                  >
                    [標準] {{ fieldDisplayName(stdKey) }}
                  </option>
                </optgroup>
                <optgroup v-if="filteredCustomFields.length > 0" label="カスタムフィールド">
                  <option
                    v-for="cf in filteredCustomFields"
                    :key="cf.id"
                    :value="`custom:${cf.id}`"
                  >
                    [カスタム] {{ cf.name }} ({{ cf.field_type }})
                  </option>
                </optgroup>
              </select>

              <!-- Visibility & Permission Mode -->
              <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px; font-size: 13px;">
                <label
                  v-if="!(f.field_source === 'standard' && f.field_key === 'title')"
                  style="display: flex; align-items: center; gap: 6px; cursor: pointer;"
                >
                  <input v-model="f.is_hidden" type="checkbox" />
                  <span>👁️ 非表示にする</span>
                </label>
                <span v-else style="font-size: 12px; color: var(--text-muted);">※タイトルは常時表示</span>

                <label style="display: flex; align-items: center; gap: 6px; cursor: pointer;">
                  <input v-model="f.is_restricted" type="checkbox" />
                  <span>🔒 制限モードを有効化</span>
                </label>
              </div>

              <div v-if="f.is_restricted" style="display: flex; flex-direction: column; gap: 8px; padding-top: 6px; border-top: 1px dashed var(--border-color); font-size: 12px;">
                <div style="display: flex; gap: 12px;">
                  <label v-if="!isSkeleton(f)" style="display: flex; align-items: center; gap: 4px;">
                    <input v-model="f.allow_owner_view" type="checkbox" />
                    <span>オーナー閲覧可</span>
                  </label>
                  <label style="display: flex; align-items: center; gap: 4px;">
                    <input v-model="f.allow_owner_edit" type="checkbox" />
                    <span>オーナー編集可</span>
                  </label>
                </div>

                <!-- View Subjects Tag Selection (P-6) -->
                <div v-if="!isSkeleton(f)">
                  <div style="color: var(--text-muted); margin-bottom: 4px;">閲覧許可対象（ユーザー / グループ）:</div>
                  <div style="display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 4px;">
                    <span
                      v-for="(sub, sIdx) in f.view_subjects"
                      :key="`vsub-${sub.type}:${sub.id}`"
                      class="badge badge-blue"
                      style="font-size: 11px; cursor: pointer;"
                      @click="removeSubject(f, 'view', sIdx)"
                    >
                      {{ getSubjectLabel(sub) }} ✕
                    </span>
                  </div>
                  <select
                    style="width: 100%; font-size: 12px; padding: 4px 8px;"
                    @change="addSubject(f, 'view', ($event.target as HTMLSelectElement).value); ($event.target as HTMLSelectElement).value = ''"
                  >
                    <option value="">＋ 閲覧許可対象を追加...</option>
                    <option
                      v-for="sub in allSubjectOptions"
                      :key="`v-${sub.type}-${sub.id}`"
                      :value="`${sub.type}:${sub.id}`"
                      :disabled="hasSubject(f.view_subjects, sub.type, sub.id)"
                    >
                      {{ sub.label }}
                    </option>
                  </select>
                </div>

                <!-- Edit Subjects Tag Selection (P-6) -->
                <div>
                  <div style="color: var(--text-muted); margin-bottom: 4px;">編集許可対象（ユーザー / グループ）:</div>
                  <div style="display: flex; flex-wrap: wrap; gap: 4px; margin-bottom: 4px;">
                    <span
                      v-for="(sub, sIdx) in f.edit_subjects"
                      :key="`esub-${sub.type}:${sub.id}`"
                      class="badge badge-blue"
                      style="font-size: 11px; cursor: pointer;"
                      @click="removeSubject(f, 'edit', sIdx)"
                    >
                      {{ getSubjectLabel(sub) }} ✕
                    </span>
                  </div>
                  <select
                    style="width: 100%; font-size: 12px; padding: 4px 8px;"
                    @change="addSubject(f, 'edit', ($event.target as HTMLSelectElement).value); ($event.target as HTMLSelectElement).value = ''"
                  >
                    <option value="">＋ 編集許可対象を追加...</option>
                    <option
                      v-for="sub in allSubjectOptions"
                      :key="`e-${sub.type}-${sub.id}`"
                      :value="`${sub.type}:${sub.id}`"
                      :disabled="hasSubject(f.edit_subjects, sub.type, sub.id)"
                    >
                      {{ sub.label }}
                    </option>
                  </select>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { CustomFieldDefinition, Group, SubjectRef, SubjectType, TargetType, Template, TemplateField, TemplateRow, User } from '~/types/polaris'

const props = defineProps<{
  title?: string
  templates: Template[]
  customFields: CustomFieldDefinition[]
  users: User[]
  groups: Group[]
}>()

const emit = defineEmits<{
  (e: 'save', payload: { id?: string; target_type: TargetType; name: string; rows: TemplateRow[] }): void
  (e: 'delete', templateId: string): void
}>()

const editingTemplate = ref<{ id?: string; target_type: TargetType; name: string; rows: TemplateRow[] } | null>(null)

const issueStdFields = ['title', 'description', 'labels']
const taskStdFields = [
  'title',
  'status',
  'progress',
  'planned_dates',
  'actual_dates',
  'planned_hours',
  'actual_hours',
  'planned_cost',
  'actual_cost',
  'assignees',
  'labels',
  'description'
]
const requiredTaskStdFields = [
  'title',
  'status',
  'progress',
  'planned_dates',
  'actual_dates',
  'planned_hours',
  'actual_hours',
  'assignees',
  'labels',
  'description'
]
const skeletonKeys = new Set([
  'title',
  'status',
  'planned_dates',
  'actual_dates',
  'planned_hours',
  'actual_hours',
  'progress'
])

const availableStandardFields = computed(() => {
  if (!editingTemplate.value) return []
  return editingTemplate.value.target_type === 'issue' ? issueStdFields : taskStdFields
})

const filteredCustomFields = computed(() => {
  if (!editingTemplate.value) return []
  return props.customFields.filter(c => c.target_type === editingTemplate.value?.target_type)
})

const allSubjectOptions = computed(() => {
  const list: { type: SubjectType; id: string; label: string }[] = []
  for (const g of props.groups) {
    list.push({ type: 'group', id: g.id, label: `👥 [グループ] ${g.name}` })
  }
  for (const u of props.users) {
    list.push({ type: 'user', id: u.id, label: `👤 ${u.display_name} (${u.email})` })
  }
  return list
})

const missingRequiredFields = computed(() => {
  if (!editingTemplate.value) return []
  const placed = new Set<string>()
  for (const r of editingTemplate.value.rows) {
    for (const f of r.fields) {
      if (f.field_source === 'standard') {
        placed.add(f.field_key)
      }
    }
  }
  const reqs = editingTemplate.value.target_type === 'issue' ? issueStdFields : requiredTaskStdFields
  return reqs.filter(k => !placed.has(k))
})

watch(
  () => props.templates,
  (list) => {
    if (!editingTemplate.value && list.length > 0) {
      selectTemplate(list[0])
    }
  },
  { immediate: true }
)

function fieldDisplayName(key: string): string {
  const names: Record<string, string> = {
    title: 'タイトル',
    description: '説明 (Markdown)',
    labels: 'ラベル',
    status: 'ステータス',
    progress: '進捗率 (%)',
    planned_dates: '予定開始・終了日',
    actual_dates: '実績開始・終了日',
    planned_hours: '予定工数 (h)',
    actual_hours: '実績工数 (h)',
    planned_cost: '予定コスト',
    actual_cost: '実績コスト',
    assignees: '担当者'
  }
  return names[key] || key
}

function isSkeleton(f: TemplateField): boolean {
  return f.field_source === 'standard' && skeletonKeys.has(f.field_key)
}

function selectTemplate(t: Template) {
  editingTemplate.value = JSON.parse(JSON.stringify({
    id: t.id,
    target_type: t.target_type,
    name: t.name,
    rows: t.rows
  }))
}

function startNewTemplate(targetType: TargetType) {
  const stds = targetType === 'issue' ? issueStdFields : taskStdFields
  const rows: TemplateRow[] = stds.map((key, idx) => ({
    row_order: idx + 1,
    columns_count: 1,
    fields: [
      {
        column_order: 1,
        field_source: 'standard',
        field_key: key,
        is_restricted: false,
        allow_owner_view: true,
        allow_owner_edit: true,
        view_subjects: [],
        edit_subjects: []
      }
    ]
  }))
  editingTemplate.value = {
    target_type: targetType,
    name: targetType === 'issue' ? '新規課題テンプレート' : '新規タスクテンプレート',
    rows
  }
}

function pickNextAvailableField(): { source: 'standard' | 'custom'; key: string } {
  if (!editingTemplate.value) return { source: 'standard', key: 'title' }
  const placed = new Set<string>()
  for (const r of editingTemplate.value.rows) {
    for (const f of r.fields) {
      if (f.field_source === 'standard') {
        placed.add(`standard:${f.field_key}`)
      } else {
        placed.add(`custom:${f.field_key}`)
      }
    }
  }
  for (const k of missingRequiredFields.value) {
    return { source: 'standard', key: k }
  }
  const stds = availableStandardFields.value
  for (const k of stds) {
    if (!placed.has(`standard:${k}`)) {
      return { source: 'standard', key: k }
    }
  }
  for (const cf of filteredCustomFields.value) {
    if (!placed.has(`custom:${cf.id}`)) {
      return { source: 'custom', key: cf.id }
    }
  }
  return { source: 'standard', key: stds[0] || 'title' }
}

function setRowColumns(row: TemplateRow, cols: number) {
  row.columns_count = cols
  while (row.fields.length < cols) {
    const next = pickNextAvailableField()
    row.fields.push({
      column_order: row.fields.length + 1,
      field_source: next.source,
      field_key: next.key,
      is_restricted: false,
      allow_owner_view: true,
      allow_owner_edit: true,
      view_subjects: [],
      edit_subjects: []
    })
  }
  if (row.fields.length > cols) {
    row.fields.splice(cols)
  }
  row.fields.forEach((f, idx) => {
    f.column_order = idx + 1
  })
}

function addRow() {
  if (!editingTemplate.value) return
  const next = pickNextAvailableField()
  editingTemplate.value.rows.push({
    row_order: editingTemplate.value.rows.length + 1,
    columns_count: 1,
    fields: [
      {
        column_order: 1,
        field_source: next.source,
        field_key: next.key,
        is_restricted: false,
        allow_owner_view: true,
        allow_owner_edit: true,
        view_subjects: [],
        edit_subjects: []
      }
    ]
  })
}

function removeRow(rIdx: number) {
  if (!editingTemplate.value) return
  editingTemplate.value.rows.splice(rIdx, 1)
  editingTemplate.value.rows.forEach((r, idx) => {
    r.row_order = idx + 1
  })
}

function onChangeFieldKey(f: TemplateField, combined: string) {
  const [source, key] = combined.split(':')
  f.field_source = (source as 'standard' | 'custom') || 'standard'
  f.field_key = key || 'title'
  if (f.field_source === 'standard' && f.field_key === 'title') {
    f.is_hidden = false
  }
}

function hasSubject(list: SubjectRef[] | undefined, type: SubjectType, id: string): boolean {
  return Boolean(list?.some(s => s.type === type && s.id === id))
}

const draggingRowIndex = ref<number | null>(null)
function onRowDragStart(index: number) {
  draggingRowIndex.value = index
}
function onRowDrop(targetIndex: number) {
  if (draggingRowIndex.value === null || !editingTemplate.value) return
  const rows = editingTemplate.value.rows
  const [removed] = rows.splice(draggingRowIndex.value, 1)
  rows.splice(targetIndex, 0, removed)
  rows.forEach((r, idx) => {
    r.row_order = idx + 1
  })
  draggingRowIndex.value = null
}

function getSubjectLabel(s: SubjectRef): string {
  if (s.type === 'user') {
    const u = props.users.find(x => x.id === s.id)
    return u ? `👤 ${u.display_name}` : `👤 ${s.id.slice(0, 8)}`
  }
  const g = props.groups.find(x => x.id === s.id)
  return g ? `👥 ${g.name}` : `👥 ${s.id.slice(0, 8)}`
}

function addSubject(f: TemplateField, mode: 'view' | 'edit', val: string) {
  if (!val) return
  const [type, id] = val.split(':') as [SubjectType, string]
  if (!f.view_subjects) f.view_subjects = []
  if (!f.edit_subjects) f.edit_subjects = []
  const list = mode === 'view' ? f.view_subjects : f.edit_subjects
  if (!list.some(s => s.type === type && s.id === id)) {
    list.push({ type, id })
  }
}

function removeSubject(f: TemplateField, mode: 'view' | 'edit', index: number) {
  const list = mode === 'view' ? f.view_subjects : f.edit_subjects
  list.splice(index, 1)
}

function onSelectSubjects(f: TemplateField, mode: 'view' | 'edit', ev: Event) {
  const sel = ev.target as HTMLSelectElement
  const refs: SubjectRef[] = []
  for (const opt of Array.from(sel.selectedOptions)) {
    const [t, id] = opt.value.split(':')
    if ((t === 'user' || t === 'group') && id) {
      refs.push({ type: t, id })
    }
  }
  if (mode === 'view') {
    f.view_subjects = refs
  } else {
    f.edit_subjects = refs
  }
}

function saveCurrentTemplate() {
  if (!editingTemplate.value) return
  editingTemplate.value.rows.forEach((r, rIdx) => {
    r.row_order = rIdx + 1
    r.fields.forEach((f, fIdx) => {
      f.column_order = fIdx + 1
    })
  })
  emit('save', editingTemplate.value)
}
</script>

