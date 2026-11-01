<template>
  <div class="item-detail-page-view" style="display: flex; flex-direction: column; gap: 16px; width: 100%;">
    <!-- Top Breadcrumb / Back Navigation Bar -->
    <div class="card" style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; padding: 12px 16px;">
      <div style="display: flex; align-items: center; gap: 12px; font-size: 13px;">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          style="display: flex; align-items: center; gap: 6px; font-weight: 700;"
          @click="confirmClose"
        >
          <span>←</span>
          <span>ワークスペースに戻る</span>
        </button>
        <span style="height: 16px; border-left: 1px solid var(--border-color);"></span>
        <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 6px; color: var(--text-muted); font-size: 13px;">
          <span
            style="color: var(--primary); font-weight: 600; cursor: pointer; text-decoration: underline;"
            title="ワークスペースに戻る"
            @click="confirmClose"
          >
            {{ projectName || 'プロジェクト' }}
          </span>
          <span>›</span>
          <span
            v-if="targetType === 'task' && parentIssueTitle"
            style="color: var(--primary); font-weight: 600; cursor: pointer; text-decoration: underline;"
            title="親課題に移動"
            @click="goToParentIssue"
          >
            {{ parentIssueTitle }}
          </span>
          <span v-if="targetType === 'task' && parentIssueTitle">›</span>
          <span style="font-weight: 700; color: var(--text-main);">
            {{ targetType === 'issue' ? '課題詳細' : 'タスク詳細' }}
          </span>
        </div>
      </div>

      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
        <!-- Issue Open/Closed State -->
        <div v-if="targetType === 'issue'" style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
          <span style="color: var(--text-muted);">状態:</span>
          <select v-model="form.state" :disabled="!canEditAny">
            <option value="open">OPEN (オープン)</option>
            <option value="closed">CLOSED (クローズ)</option>
          </select>
        </div>

        <!-- Owner & Transfer -->
        <div style="display: flex; align-items: center; gap: 6px; font-size: 13px; padding: 4px 10px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input);">
          <span style="color: var(--text-muted);">オーナー:</span>
          <select
            ref="ownerSelectRef"
            :value="ownerUserId"
            :disabled="!canTransferOwner"
            style="padding: 2px 6px; font-size: 13px;"
            @change="onOwnerSelectChange(($event.target as HTMLSelectElement).value)"
          >
            <option v-for="u in users" :key="u.id" :value="u.id">
              {{ u.display_name }}
            </option>
          </select>
        </div>

        <!-- Type Switcher -->
        <div style="display: flex; align-items: center; gap: 6px; font-size: 13px; padding: 4px 10px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input);">
          <span style="color: var(--text-muted);">種別:</span>
          <select v-model="form.type_id" :disabled="!canEditAny" style="padding: 2px 6px; font-size: 13px;">
            <option value="">(未設定)</option>
            <option v-for="tp in types || []" :key="tp.id" :value="tp.id">
              {{ tp.name }}
            </option>
          </select>
          <span
            v-if="selectedType"
            class="badge"
            :style="{ backgroundColor: selectedType.color, color: '#fff', fontSize: '11px', fontWeight: 'bold' }"
          >
            {{ selectedType.name }}
          </span>
        </div>

        <!-- Template Switcher -->
        <div style="display: flex; align-items: center; gap: 6px; font-size: 13px; padding: 4px 10px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input);">
          <span style="color: var(--text-muted);">テンプレート:</span>
          <select v-model="form.template_id" :disabled="!canEditAny" style="padding: 2px 6px; font-size: 13px;">
            <option v-for="tpl in applicableTemplates" :key="tpl.id" :value="tpl.id">
              {{ tpl.name }}
            </option>
          </select>
        </div>

        <button type="button" class="btn btn-secondary btn-sm" @click="copyItemUrl">URLをコピー</button>
        <button v-if="canEditAny" type="button" class="btn btn-danger btn-sm" @click="deleteItem">削除</button>
        <button
          type="button"
          class="btn btn-primary btn-sm"
          :class="{ 'btn-dirty': isDirty }"
          :disabled="!canEditAny || isSaving"
          @click="saveItem"
        >
          <span v-if="isSaving">⏳ 保存中...</span>
          <span v-else-if="saveSuccess">✓ 保存完了</span>
          <span v-else>変更を保存</span>
        </button>
      </div>
    </div>

    <!-- Main Detail Content Card -->
    <div class="card" style="display: flex; flex-direction: column; gap: 18px; padding: 20px;">
      <!-- Title Card Header -->
      <div style="display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 14px;">
        <div style="display: flex; flex-direction: column; gap: 4px;">
          <div style="display: flex; align-items: center; gap: 8px; font-size: 13px; color: var(--text-muted);">
            <span :class="['badge', targetType === 'issue' ? 'badge-green' : 'badge-blue']">
              {{ targetType === 'issue' ? '課題 (Issue)' : 'タスク (Task)' }}
            </span>
            <span
              v-if="selectedType"
              class="badge"
              :style="{ backgroundColor: selectedType.color, color: '#fff', fontSize: '11px', fontWeight: 'bold' }"
            >
              {{ selectedType.name }}
            </span>
            <span v-if="targetType === 'task' && parentIssueTitle">
              親課題: <strong>{{ parentIssueTitle }}</strong>
            </span>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              style="padding: 2px 7px; font-size: 12px;"
              @click="copyItemId"
            >
              IDをコピー
            </button>
          </div>
          <h2 style="font-size: 20px; font-weight: 700;">{{ form.title || '(無題)' }}</h2>
        </div>

        <!-- Tab Switcher: Details vs History -->
        <div style="display: flex; gap: 4px; padding: 2px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color);">
          <button
            type="button"
            class="btn btn-sm"
            :class="modalTab === 'details' ? 'btn-primary' : 'btn-ghost'"
            style="font-size: 12px;"
            @click="modalTab = 'details'"
          >
            📝 詳細項目
          </button>
          <button
            type="button"
            class="btn btn-sm"
            :class="modalTab === 'history' ? 'btn-primary' : 'btn-ghost'"
            style="font-size: 12px;"
            @click="modalTab = 'history'"
          >
            ⏱️ 履歴 (Activity)
          </button>
        </div>
      </div>

      <!-- HISTORY TAB CONTENT -->
      <div v-if="modalTab === 'history'" style="padding: 4px 0;">
        <ActivityLogView
          :project-id="projectId"
          :entity-type="targetType"
          :entity-id="targetId"
          :users="users"
        />
      </div>

      <!-- DETAILS TAB CONTENT -->
      <template v-else>
      <!-- Issue Rollup Summary Banner (When viewing an Issue) -->
      <div
        v-if="targetType === 'issue' && issue?.rollup"
        style="padding: 12px 14px; border-radius: 8px; background: rgba(37, 99, 235, 0.1); border: 1px solid rgba(59, 130, 246, 0.35); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; font-size: 13px;"
      >
        <div>
          <span style="color: var(--text-muted);">自動集計予定期間:</span>
          <strong style="margin-left: 6px;">{{ issue.rollup.planned_start_date || '-' }} 〜 {{ issue.rollup.planned_end_date || '-' }}</strong>
        </div>
        <div>
          <span style="color: var(--text-muted);">自動集計実績期間:</span>
          <strong style="margin-left: 6px;">{{ issue.rollup.actual_start_date || '-' }} 〜 {{ issue.rollup.actual_end_date || '-' }}</strong>
        </div>
        <div>
          <span style="color: var(--text-muted);">合計予定/実績工数:</span>
          <strong style="margin-left: 6px;">{{ formatHours(issue.rollup.total_planned_hours) }}h / {{ formatHours(issue.rollup.total_actual_hours) }}h</strong>
        </div>
        <div>
          <span style="color: var(--text-muted);">合計予定/実績コスト:</span>
          <strong style="margin-left: 6px;">{{ formatCost(issue.rollup.total_planned_cost) }} / {{ formatCost(issue.rollup.total_actual_cost) }}</strong>
        </div>
        <div style="display: flex; align-items: center; gap: 8px;">
          <span style="color: var(--text-muted);">加重平均進捗 ({{ issue.rollup.task_count }}タスク):</span>
          <strong style="color: var(--primary);">{{ issue.rollup.weighted_progress.toFixed(1) }}%</strong>
        </div>
      </div>

      <!-- Dynamic 1..3 Column Template Layout -->
      <div style="display: flex; flex-direction: column; gap: 12px;">
        <div style="font-size: 12px; font-weight: 700; color: var(--text-muted); text-transform: uppercase;">
          項目入力
        </div>

        <div
          v-for="(row, rIdx) in layoutRows"
          :key="rIdx"
          :class="`tpl-row-${row.columns_count}`"
        >
          <div
            v-for="(field, fIdx) in row.fields"
            :key="fIdx"
            :class="isFieldViewable(field) ? 'field-slot' : 'field-slot-masked'"
          >
            <div class="field-label">
              <span>{{ getFieldLabel(field) }}</span>
              <span v-if="!isFieldViewable(field)" class="badge badge-amber" style="font-size: 11px;">非表示</span>
              <span v-else-if="!isFieldEditable(field)" class="badge badge-slate" style="font-size: 11px;">閲覧のみ</span>
            </div>

            <!-- Non-viewable Masked Slot -->
            <div v-if="!isFieldViewable(field)" style="font-size: 13px; font-weight: 600; color: var(--text-muted); padding: 6px 0;">
              {{ getFieldPlaceholder(field) }}
            </div>

            <!-- Standard Field: title -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'title'">
              <input v-model="form.title" type="text" :disabled="!isFieldEditable(field)" style="width: 100%;" />
            </div>

            <!-- Standard Field: type -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'type'" style="display: flex; align-items: center; gap: 8px;">
              <select v-model="form.type_id" :disabled="!isFieldEditable(field)" style="flex: 1; max-width: 280px;">
                <option value="">-- 未設定 --</option>
                <option v-for="tp in types || []" :key="tp.id" :value="tp.id">
                  ● {{ tp.name }}
                </option>
              </select>
              <span
                v-if="selectedType"
                class="badge"
                :style="{ backgroundColor: selectedType.color, color: '#fff', fontSize: '11px', fontWeight: 'bold' }"
              >
                {{ selectedType.name }}
              </span>
            </div>

            <!-- Standard Field: description -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'description'" style="display: flex; flex-direction: column; gap: 6px;">
              <textarea
                v-if="isFieldEditable(field)"
                v-model="form.description"
                rows="3"
                style="width: 100%;"
                placeholder="タスクの説明を入力..."
              ></textarea>
              <div
                v-if="form.description"
                class="markdown-body"
                style="padding: 8px; border-radius: 6px; background: var(--bg-main); border: 1px solid var(--border-color);"
                v-html="renderSafeMarkdown(form.description)"
              ></div>
            </div>

            <!-- Standard Field: status (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'status'">
              <select v-model="form.status_id" :disabled="!isFieldEditable(field)" style="width: 100%;">
                <option v-for="st in statuses" :key="st.id" :value="st.id">
                  {{ st.display_order }}. {{ st.name }}
                </option>
              </select>
            </div>

            <!-- Standard Field: progress (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'progress'" style="display: flex; flex-direction: column; gap: 6px;">
              <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
                <input
                  v-model.number="form.progress"
                  type="number"
                  min="0"
                  max="100"
                  step="1"
                  :disabled="!isFieldEditable(field)"
                  style="width: 75px;"
                />
                <span>%</span>
                <input
                  v-model.number="form.progress"
                  type="range"
                  min="0"
                  max="100"
                  step="1"
                  :disabled="!isFieldEditable(field)"
                  style="flex: 1 1 120px; max-width: 180px; cursor: pointer;"
                />
                <div style="display: inline-flex; gap: 4px;">
                  <button
                    v-for="pVal in [0, 25, 50, 75, 100]"
                    :key="pVal"
                    type="button"
                    class="btn btn-sm"
                    :class="form.progress === pVal ? 'btn-primary' : 'btn-secondary'"
                    :disabled="!isFieldEditable(field)"
                    style="padding: 2px 6px; font-size: 11px;"
                    @click="form.progress = pVal"
                  >
                    {{ pVal }}%
                  </button>
                </div>
              </div>
              <div v-if="form.progress === 100 && !form.actual_end_date && isFieldEditable(field)">
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  style="font-size: 11px; padding: 2px 8px; color: var(--primary);"
                  @click="setTodayAsActualEnd"
                >
                  🎯 進捗100%: 本日 ({{ todayStr }}) を実績終了日にセット
                </button>
              </div>
            </div>

            <!-- Standard Field: planned_dates (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'planned_dates'" style="display: flex; flex-wrap: wrap; align-items: center; gap: 6px;">
              <input v-model="form.planned_start_date" type="date" :disabled="!isFieldEditable(field)" style="flex: 1 1 128px; min-width: 128px;" />
              <span style="flex-shrink: 0; text-align: center;">〜</span>
              <input v-model="form.planned_end_date" type="date" :disabled="!isFieldEditable(field)" style="flex: 1 1 128px; min-width: 128px;" />
            </div>

            <!-- Standard Field: actual_dates (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'actual_dates'" style="display: flex; flex-wrap: wrap; align-items: center; gap: 6px;">
              <input v-model="form.actual_start_date" type="date" :disabled="!isFieldEditable(field)" style="flex: 1 1 128px; min-width: 128px;" />
              <span style="flex-shrink: 0; text-align: center;">〜</span>
              <input v-model="form.actual_end_date" type="date" :disabled="!isFieldEditable(field)" style="flex: 1 1 128px; min-width: 128px;" />
            </div>

            <!-- Standard Field: planned_hours (Task, up to 2 decimal places) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'planned_hours'" style="display: flex; align-items: center; gap: 6px;">
              <input
                v-model="form.planned_hours"
                type="number"
                min="0"
                step="0.01"
                :disabled="!isFieldEditable(field)"
                style="width: 120px;"
                placeholder="0.00"
                @blur="formatHoursOnBlur('planned_hours')"
              />
              <span>h (小数第2位まで)</span>
            </div>

            <!-- Standard Field: actual_hours (Task, up to 2 decimal places) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'actual_hours'" style="display: flex; flex-direction: column; gap: 6px;">
              <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
                <input
                  v-model="form.actual_hours"
                  type="number"
                  min="0"
                  step="0.01"
                  :disabled="!isFieldEditable(field)"
                  style="width: 120px;"
                  placeholder="0.00"
                  @blur="formatHoursOnBlur('actual_hours')"
                />
                <span>h (小数第2位まで)</span>
                <div v-if="isFieldEditable(field)" style="display: inline-flex; gap: 4px;">
                  <button
                    v-for="hAdd in [0.5, 1.0, 2.0]"
                    :key="hAdd"
                    type="button"
                    class="btn btn-secondary btn-sm"
                    style="padding: 2px 7px; font-size: 11px;"
                    @click="addActualHours(hAdd)"
                  >
                    +{{ hAdd.toFixed(1) }}h
                  </button>
                </div>
              </div>
              <!-- Planned vs Actual Bar -->
              <div v-if="plannedHoursNum > 0" style="display: flex; flex-direction: column; gap: 3px; max-width: 360px;">
                <div style="display: flex; justify-content: space-between; font-size: 11px; color: var(--text-muted);">
                  <span>予実比: {{ hoursBurnRatio.toFixed(0) }}%</span>
                  <span v-if="hoursBurnRatio > 100" style="color: var(--danger); font-weight: 700;">
                    ⚠️ 工数超過 (+{{ (actualHoursNum - plannedHoursNum).toFixed(2) }}h)
                  </span>
                </div>
                <div style="height: 6px; border-radius: 3px; background: var(--border-color); overflow: hidden;">
                  <div
                    :style="{
                      width: `${Math.min(hoursBurnRatio, 100)}%`,
                      height: '100%',
                      backgroundColor: hoursBurnRatio > 100 ? 'var(--danger)' : 'var(--primary)'
                    }"
                  ></div>
                </div>
              </div>
            </div>

            <!-- Standard Field: planned_cost (Task, up to 2 decimal places) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'planned_cost'" style="display: flex; align-items: center; gap: 6px;">
              <input
                v-model="form.planned_cost"
                type="number"
                min="0"
                step="0.01"
                :disabled="!isFieldEditable(field)"
                style="width: 140px;"
                placeholder="0.00"
              />
              <span>(小数第2位まで)</span>
            </div>

            <!-- Standard Field: actual_cost (Task, up to 2 decimal places) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'actual_cost'" style="display: flex; align-items: center; gap: 6px;">
              <input
                v-model="form.actual_cost"
                type="number"
                min="0"
                step="0.01"
                :disabled="!isFieldEditable(field)"
                style="width: 140px;"
                placeholder="0.00"
              />
              <span>(小数第2位まで)</span>
            </div>

            <!-- Standard Field: story_points (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'story_points'" style="display: flex; align-items: center; gap: 8px;">
              <input
                v-model="form.story_points"
                type="number"
                min="0"
                step="0.1"
                :disabled="!isFieldEditable(field)"
                style="width: 140px;"
                placeholder="SP (例: 3)"
              />
              <span style="font-size: 12px; color: var(--text-muted);">(0以上、小数第1位まで)</span>
            </div>

            <!-- Standard Field: sprint_id (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'sprint_id'">
              <select v-model="form.sprint_id" :disabled="!isFieldEditable(field)" style="width: 100%;">
                <option value="">(未割当バックログ)</option>
                <option v-for="sp in projectSprints" :key="sp.id" :value="sp.id">
                  {{ sp.status === 'active' ? '⚡ ' : '📅 ' }}{{ sp.name }} ({{ sp.status === 'active' ? 'アクティブ' : sp.status === 'closed' ? '完了' : '計画中' }})
                </option>
              </select>
            </div>

            <!-- Standard Field: assignees (Task) -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'assignees'">
              <div style="display: flex; flex-wrap: wrap; gap: 6px;">
                <label
                  v-for="sub in subjectOptions"
                  :key="`${sub.type}:${sub.id}`"
                  style="display: inline-flex; align-items: center; gap: 4px; font-size: 12px; padding: 2px 8px; border-radius: 6px; border: 1px solid var(--border-color); cursor: pointer;"
                >
                  <input
                    type="checkbox"
                    :disabled="!isFieldEditable(field)"
                    :checked="hasAssignee(sub.type, sub.id)"
                    @change="toggleAssignee(sub.type, sub.id)"
                  />
                  <span>{{ sub.label }}</span>
                </label>
              </div>
            </div>

            <!-- Standard Field: labels -->
            <div v-else-if="field.field_source === 'standard' && field.field_key === 'labels'">
              <div style="display: flex; flex-wrap: wrap; gap: 6px;">
                <label
                  v-for="lb in labels"
                  :key="lb.id"
                  style="display: inline-flex; align-items: center; gap: 4px; font-size: 12px; padding: 2px 8px; border-radius: 6px; border: 1px solid var(--border-color); cursor: pointer;"
                >
                  <input
                    type="checkbox"
                    :disabled="!isFieldEditable(field)"
                    :checked="form.label_ids.includes(lb.id)"
                    @change="toggleLabel(lb.id)"
                  />
                  <span :style="{ color: lb.color, fontWeight: 600 }">● {{ lb.name }}</span>
                </label>
              </div>
            </div>

            <!-- Custom Field (9 types) -->
            <div v-else-if="field.field_source === 'custom'">
              <template v-if="getCustomDef(field.field_key)?.field_type === 'text_Multi' || getCustomDef(field.field_key)?.field_type === 'text_multi'">
                <textarea v-model="form.custom_values[field.field_key]" rows="2" :disabled="!isFieldEditable(field)" style="width: 100%;"></textarea>
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'markdown'">
                <textarea v-model="form.custom_values[field.field_key]" rows="2" :disabled="!isFieldEditable(field)" style="width: 100%;"></textarea>
                <div
                  v-if="form.custom_values[field.field_key]"
                  class="markdown-body"
                  style="margin-top: 4px;"
                  v-html="renderSafeMarkdown(String(form.custom_values[field.field_key]))"
                ></div>
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'number'">
                <input v-model.number="form.custom_values[field.field_key]" type="number" step="any" :disabled="!isFieldEditable(field)" style="width: 100%;" />
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'date'">
                <input v-model="form.custom_values[field.field_key]" type="date" :disabled="!isFieldEditable(field)" style="width: 100%;" />
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'select'">
                <select v-model="form.custom_values[field.field_key]" :disabled="!isFieldEditable(field)" style="width: 100%;">
                  <option value="">-- 選択 --</option>
                  <option v-for="opt in getCustomDef(field.field_key)?.options || []" :key="opt" :value="opt">{{ opt }}</option>
                </select>
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'radio'">
                <div style="display: flex; flex-wrap: wrap; gap: 12px; padding: 4px 0;">
                  <label
                    v-for="opt in getCustomDef(field.field_key)?.options || []"
                    :key="opt"
                    style="display: inline-flex; align-items: center; gap: 4px; font-size: 13px; cursor: pointer;"
                  >
                    <input
                      type="radio"
                      :name="`radio-${field.field_key}`"
                      :value="opt"
                      :disabled="!isFieldEditable(field)"
                      v-model="form.custom_values[field.field_key]"
                    />
                    <span>{{ opt }}</span>
                  </label>
                </div>
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'checkbox'">
                <div style="display: flex; flex-wrap: wrap; gap: 8px;">
                  <label v-for="opt in getCustomDef(field.field_key)?.options || []" :key="opt" style="display: inline-flex; align-items: center; gap: 4px; font-size: 12px;">
                    <input
                      type="checkbox"
                      :disabled="!isFieldEditable(field)"
                      :checked="Array.isArray(form.custom_values[field.field_key]) && form.custom_values[field.field_key].includes(opt)"
                      @change="toggleCustomCheckbox(field.field_key, opt)"
                    />
                    <span>{{ opt }}</span>
                  </label>
                </div>
              </template>
              <template v-else-if="getCustomDef(field.field_key)?.field_type === 'user_group'">
                <select v-model="form.custom_values[field.field_key]" :disabled="!isFieldEditable(field)" style="width: 100%;">
                  <option value="">-- 選択 --</option>
                  <option v-for="sub in subjectOptions" :key="`${sub.type}:${sub.id}`" :value="`${sub.type}:${sub.id}`">
                    {{ sub.label }}
                  </option>
                </select>
              </template>
              <template v-else>
                <input v-model="form.custom_values[field.field_key]" type="text" :disabled="!isFieldEditable(field)" style="width: 100%;" />
              </template>
            </div>
          </div>
        </div>
      </div>

      <!-- Agile & Sprint Attributes (Task Only) -->
      <div v-if="targetType === 'task'" style="border-top: 1px solid var(--border-color); padding-top: 14px; display: flex; flex-direction: column; gap: 12px;">
        <h3 style="font-size: 14px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
          <span>🏃 スプリント ＆ ストーリーポイント設定</span>
        </h3>
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 14px; background: var(--bg-input); padding: 12px; border-radius: 8px; border: 1px solid var(--border-color);">
          <div>
            <label class="field-label" style="font-size: 12px; font-weight: 700; color: var(--text-muted); margin-bottom: 4px; display: block;">ストーリーポイント (SP)</label>
            <input
              v-model="form.story_points"
              type="number"
              min="0"
              step="0.1"
              :disabled="!canEditAny"
              style="width: 100%;"
              placeholder="例: 3, 5, 8"
            />
            <span style="font-size: 11px; color: var(--text-muted);">0以上の数値（0.1単位）</span>
          </div>
          <div>
            <label class="field-label" style="font-size: 12px; font-weight: 700; color: var(--text-muted); margin-bottom: 4px; display: block;">所属スプリント</label>
            <select v-model="form.sprint_id" :disabled="!canEditAny" style="width: 100%;">
              <option value="">(未割当バックログ)</option>
              <option v-for="sp in projectSprints" :key="sp.id" :value="sp.id">
                {{ sp.status === 'active' ? '⚡ ' : '📅 ' }}{{ sp.name }} ({{ sp.status === 'active' ? 'アクティブ' : sp.status === 'closed' ? '完了' : '計画中' }})
              </option>
            </select>
            <span style="font-size: 11px; color: var(--text-muted);">バックログまたは対象スプリントを選択</span>
          </div>
        </div>
      </div>

      <!-- Task Dependencies Section (Task Only) -->
      <div v-if="targetType === 'task'" style="border-top: 1px solid var(--border-color); padding-top: 14px; display: flex; flex-direction: column; gap: 12px;">
        <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px;">
          <h3 style="font-size: 14px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
            <span>🔗 タスク依存関係（FS / SS / FF / SF）</span>
            <span class="badge badge-slate" style="font-size: 11px;">先行 {{ predecessorDeps.length }}件 / 後続 {{ successorDeps.length }}件</span>
          </h3>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); gap: 14px;">
          <!-- Predecessor Tasks -->
          <div style="border: 1px solid var(--border-color); border-radius: 8px; padding: 12px; background: var(--bg-input);">
            <div style="font-weight: 700; font-size: 13px; margin-bottom: 8px; color: var(--text-main); display: flex; align-items: center; justify-content: space-between;">
              <span>◀ 先行タスク（前提タスク）</span>
              <span class="badge badge-blue" style="font-size: 10px;">{{ predecessorDeps.length }}件</span>
            </div>
            <div v-if="predecessorDeps.length === 0" style="font-size: 12px; color: var(--text-muted); padding: 4px 0;">
              先行タスクはありません
            </div>
            <div v-else style="display: flex; flex-direction: column; gap: 6px; margin-bottom: 10px;">
              <div
                v-for="dep in predecessorDeps"
                :key="dep.id"
                style="display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; background: var(--bg-main); border: 1px solid var(--border-color); border-radius: 6px; font-size: 12px;"
              >
                <div style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-right: 8px;">
                  <span class="badge badge-blue" style="font-size: 10px; margin-right: 4px;">{{ dep.dependency_type || 'FS' }}</span>
                  <span v-if="dep.lag_days" class="badge badge-slate" style="font-size: 10px; margin-right: 4px;">+{{ dep.lag_days }}d</span>
                  <strong>{{ getTaskTitle(dep.predecessor_id) }}</strong>
                </div>
                <button
                  v-if="canEditAny"
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="color: var(--danger); padding: 2px 6px; font-size: 11px;"
                  title="依存関係を解除"
                  @click="removeDependency(dep.id)"
                >
                  解除
                </button>
              </div>
            </div>

            <!-- Add Predecessor Form -->
            <div v-if="canEditAny" style="display: flex; flex-direction: column; gap: 6px; border-top: 1px dashed var(--border-color); padding-top: 8px;">
              <div style="font-size: 11px; font-weight: 600; color: var(--text-muted);">先行タスクを追加:</div>
              <div style="display: flex; gap: 6px; flex-wrap: wrap;">
                <select v-model="newPredecessorId" style="flex: 1; min-width: 140px; font-size: 12px;">
                  <option value="">-- タスクを選択 --</option>
                  <option
                    v-for="t in availablePredecessorTasks"
                    :key="t.id"
                    :value="t.id"
                  >
                    {{ t.title }}
                  </option>
                </select>
                <select v-model="newDepType" style="width: 70px; font-size: 12px;" title="依存タイプ">
                  <option value="FS">FS</option>
                  <option value="SS">SS</option>
                  <option value="FF">FF</option>
                  <option value="SF">SF</option>
                </select>
                <input
                  v-model.number="newDepLag"
                  type="number"
                  placeholder="Lag"
                  title="ラグ日数"
                  style="width: 50px; font-size: 12px;"
                />
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="!newPredecessorId || isAddingDep"
                  @click="createDependency('predecessor')"
                >
                  追加
                </button>
              </div>
            </div>
          </div>

          <!-- Successor Tasks -->
          <div style="border: 1px solid var(--border-color); border-radius: 8px; padding: 12px; background: var(--bg-input);">
            <div style="font-weight: 700; font-size: 13px; margin-bottom: 8px; color: var(--text-main); display: flex; align-items: center; justify-content: space-between;">
              <span>▶ 後続タスク（依存されるタスク）</span>
              <span class="badge badge-blue" style="font-size: 10px;">{{ successorDeps.length }}件</span>
            </div>
            <div v-if="successorDeps.length === 0" style="font-size: 12px; color: var(--text-muted); padding: 4px 0;">
              後続タスクはありません
            </div>
            <div v-else style="display: flex; flex-direction: column; gap: 6px; margin-bottom: 10px;">
              <div
                v-for="dep in successorDeps"
                :key="dep.id"
                style="display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; background: var(--bg-main); border: 1px solid var(--border-color); border-radius: 6px; font-size: 12px;"
              >
                <div style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; margin-right: 8px;">
                  <span class="badge badge-blue" style="font-size: 10px; margin-right: 4px;">{{ dep.dependency_type || 'FS' }}</span>
                  <span v-if="dep.lag_days" class="badge badge-slate" style="font-size: 10px; margin-right: 4px;">+{{ dep.lag_days }}d</span>
                  <strong>{{ getTaskTitle(dep.successor_id) }}</strong>
                </div>
                <button
                  v-if="canEditAny"
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="color: var(--danger); padding: 2px 6px; font-size: 11px;"
                  title="依存関係を解除"
                  @click="removeDependency(dep.id)"
                >
                  解除
                </button>
              </div>
            </div>

            <!-- Add Successor Form -->
            <div v-if="canEditAny" style="display: flex; flex-direction: column; gap: 6px; border-top: 1px dashed var(--border-color); padding-top: 8px;">
              <div style="font-size: 11px; font-weight: 600; color: var(--text-muted);">後続タスクを追加:</div>
              <div style="display: flex; gap: 6px; flex-wrap: wrap;">
                <select v-model="newSuccessorId" style="flex: 1; min-width: 140px; font-size: 12px;">
                  <option value="">-- タスクを選択 --</option>
                  <option
                    v-for="t in availableSuccessorTasks"
                    :key="t.id"
                    :value="t.id"
                  >
                    {{ t.title }}
                  </option>
                </select>
                <select v-model="newDepType" style="width: 70px; font-size: 12px;" title="依存タイプ">
                  <option value="FS">FS</option>
                  <option value="SS">SS</option>
                  <option value="FF">FF</option>
                  <option value="SF">SF</option>
                </select>
                <input
                  v-model.number="newDepLag"
                  type="number"
                  placeholder="Lag"
                  title="ラグ日数"
                  style="width: 50px; font-size: 12px;"
                />
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="!newSuccessorId || isAddingDep"
                  @click="createDependency('successor')"
                >
                  追加
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Unified Attachments Section (Max 100 files, Max 1GB) -->
      <div style="border-top: 1px solid var(--border-color); padding-top: 14px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px;">
          <div>
            <h3 style="font-size: 14px; font-weight: 700;">
              📎 添付ファイル一覧（{{ attachments.length }} / 100件 ・ 1ファイル最大1GB）
            </h3>
            <p style="font-size: 12px; color: var(--text-muted);">
              ※コメント投稿時に追加したファイルもすべてこの{{ targetType === 'issue' ? '課題' : 'タスク' }}直下に集約して表示されます
            </p>
          </div>
          <label v-if="canEditAny && attachments.length < 100" class="btn btn-secondary btn-sm" style="cursor: pointer;">
            ＋ ファイルを追加
            <input type="file" style="display: none;" @change="onUploadFile" />
          </label>
        </div>

        <!-- DnD Drop Zone -->
        <div
          v-if="canEditAny && attachments.length < 100"
          class="file-drop-zone"
          :class="{ active: isDraggingFiles }"
          @dragover.prevent="isDraggingFiles = true"
          @dragleave.prevent="isDraggingFiles = false"
          @drop.prevent="onDropFiles"
        >
          <div style="font-size: 13px; color: var(--text-secondary);">
            ここにファイルをドラッグ＆ドロップ、またはクリップボードから画像をペースト（Ctrl+V）して追加
          </div>
        </div>

        <div v-if="attachments.length === 0" style="font-size: 13px; color: var(--text-muted);">
          添付ファイルはありません
        </div>
        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 8px;">
          <div
            v-for="att in attachments"
            :key="att.id"
            style="padding: 10px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input); display: flex; align-items: center; justify-content: space-between; gap: 8px; font-size: 13px;"
          >
            <div style="display: flex; align-items: center; gap: 10px; overflow: hidden;">
              <!-- Image Thumbnail with Lightbox trigger -->
              <img
                v-if="isImageAttachment(att)"
                :src="getAttachmentDownloadUrl(att.id)"
                :alt="att.file_name"
                style="width: 44px; height: 44px; object-fit: cover; border-radius: 4px; cursor: pointer; flex-shrink: 0; border: 1px solid var(--border-color);"
                title="クリックして拡大表示"
                @click="openLightbox(att)"
              />
              <div style="overflow: hidden;">
                <div style="font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">{{ att.file_name }}</div>
                <div style="font-size: 12px; color: var(--text-muted);">
                  {{ formatFileSize(att.size_bytes) }} ・ {{ formatJSTDateTime(att.created_at) }}
                </div>
              </div>
            </div>
            <div style="display: flex; gap: 6px; flex-shrink: 0;">
              <a
                :href="getAttachmentDownloadUrl(att.id)"
                target="_blank"
                class="btn btn-secondary btn-sm"
                style="text-decoration: none;"
              >
                DL
              </a>
              <button class="btn btn-danger btn-sm" @click="deleteAttachment(att.id)">削除</button>
            </div>
          </div>
        </div>
      </div>

      <!-- Comments & Mentions Section -->
      <div style="border-top: 1px solid var(--border-color); padding-top: 14px; display: flex; flex-direction: column; gap: 12px;">
        <h3 style="font-size: 14px; font-weight: 700;">💬 コメント・メンション（Markdown対応 / ファイル同時添付可）</h3>

        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div
            v-for="c in comments"
            :key="c.id"
            style="padding: 10px 12px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input);"
          >
            <div style="display: flex; justify-content: space-between; align-items: center; font-size: 12px; color: var(--text-muted); margin-bottom: 4px;">
              <strong>{{ getUserName(c.author_user_id) }}</strong>
              <div style="display: flex; align-items: center; gap: 8px;">
                <span>{{ formatJSTDateTime(c.created_at) }}</span>
                <button
                  v-if="(c.author_user_id === currentUser?.id || currentUserRole === 'admin') && editingCommentId !== c.id"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  style="padding: 2px 7px; font-size: 11px;"
                  @click="startEditComment(c)"
                >
                  編集
                </button>
                <button
                  v-if="c.author_user_id === currentUser?.id || currentUserRole === 'admin'"
                  class="btn btn-danger btn-sm"
                  style="padding: 2px 7px; font-size: 11px;"
                  @click="removeComment(c.id)"
                >
                  削除
                </button>
              </div>
            </div>

            <!-- Inline Edit Form -->
            <div v-if="editingCommentId === c.id" style="display: flex; flex-direction: column; gap: 8px; margin-top: 6px;">
              <textarea
                v-model="editingCommentBody"
                rows="3"
                style="width: 100%;"
              ></textarea>
              <div style="display: flex; align-items: center; gap: 8px;">
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="isSavingComment"
                  @click="saveEditComment(c.id)"
                >
                  <span v-if="isSavingComment">保存中...</span>
                  <span v-else>更新を保存</span>
                </button>
                <button type="button" class="btn btn-secondary btn-sm" @click="cancelEditComment">キャンセル</button>
              </div>
            </div>
            <!-- Rendered Body -->
            <div v-else class="markdown-body" v-html="renderSafeMarkdown(c.body)"></div>
          </div>
        </div>

        <!-- New Comment Form -->
        <form v-if="canEditAny" style="display: flex; flex-direction: column; gap: 8px;" @submit.prevent="postComment">
          <!-- Write / Preview Tabs -->
          <div style="display: flex; gap: 6px; border-bottom: 1px solid var(--border-color); padding-bottom: 4px;">
            <button
              type="button"
              class="btn btn-sm"
              :class="commentTab === 'write' ? 'btn-primary' : 'btn-secondary'"
              style="padding: 3px 10px; font-size: 12px;"
              @click="commentTab = 'write'"
            >
              編集中 (Write)
            </button>
            <button
              type="button"
              class="btn btn-sm"
              :class="commentTab === 'preview' ? 'btn-primary' : 'btn-secondary'"
              style="padding: 3px 10px; font-size: 12px;"
              @click="commentTab = 'preview'"
            >
              プレビュー (Preview)
            </button>
          </div>

          <div v-if="commentTab === 'write'" style="position: relative;">
            <textarea
              ref="commentTextareaRef"
              v-model="newCommentBody"
              rows="3"
              placeholder="コメントをMarkdownで入力... (@でメンション補完 / Ctrl+Enterで投稿)"
              style="width: 100%;"
              @input="onCommentInput"
              @keydown.ctrl.enter="postComment"
              @keydown.meta.enter="postComment"
            ></textarea>
            <!-- Mention suggestions dropdown -->
            <div
              v-if="showMentionSuggest && filteredMentionSubjects.length > 0"
              class="mention-suggest-popover"
              :style="mentionSuggestStyle"
            >
              <div
                v-for="sub in filteredMentionSubjects"
                :key="`suggest-${sub.type}:${sub.id}`"
                class="mention-suggest-item"
                @mousedown.prevent="selectMentionSuggest(sub)"
              >
                {{ sub.label }}
              </div>
            </div>
          </div>

          <div
            v-else
            class="markdown-body"
            style="padding: 10px; border-radius: 6px; background: var(--bg-main); border: 1px solid var(--border-color); min-height: 80px;"
            v-html="renderSafeMarkdown(newCommentBody || '*(プレビューする内容がありません)*')"
          ></div>

          <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px;">
            <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 6px; font-size: 13px;">
              <span style="color: var(--text-muted);">メンション通知先:</span>
              <select style="font-size: 13px;" @change="addCommentMention(($event.target as HTMLSelectElement).value); ($event.target as HTMLSelectElement).value = ''">
                <option value="">＋ ユーザー/グループを追加...</option>
                <option v-for="sub in subjectOptions" :key="`m-${sub.type}:${sub.id}`" :value="`${sub.type}:${sub.id}`">
                  {{ sub.label }}
                </option>
              </select>
              <span
                v-for="(m, idx) in newCommentMentions"
                :key="idx"
                class="badge badge-blue"
                style="cursor: pointer;"
                @click="newCommentMentions.splice(idx, 1)"
              >
                @{{ getSubjectLabel(m) }} ✕
              </span>
            </div>

            <div style="display: flex; align-items: center; gap: 8px;">
              <label class="btn btn-secondary btn-sm" style="cursor: pointer;">
                📎 コメントにファイルを添付（本体へ集約）
                <input type="file" style="display: none;" @change="onUploadFileFromComment" />
              </label>
              <button type="submit" class="btn btn-primary btn-sm">コメント投稿</button>
            </div>
          </div>
        </form>
      </div>
      </template>
    </div>

    <!-- Lightbox Modal -->
    <div v-if="lightboxAtt" class="lightbox-backdrop" @click="lightboxAtt = null">
      <div class="lightbox-container" @click.stop>
        <img :src="getAttachmentDownloadUrl(lightboxAtt.id)" :alt="lightboxAtt.file_name" />
        <div style="color: white; margin-top: 10px; display: flex; align-items: center; justify-content: space-between; width: 100%; font-size: 13px;">
          <span>{{ lightboxAtt.file_name }} ({{ formatFileSize(lightboxAtt.size_bytes) }})</span>
          <button type="button" class="btn btn-secondary btn-sm" @click="lightboxAtt = null">✕ 閉じる</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type {
  Attachment,
  Comment,
  CustomFieldDefinition,
  DependencyType,
  FieldPermissionState,
  Group,
  Issue,
  ItemType,
  Label,
  ProjectRole,
  Sprint,
  Status,
  SubjectRef,
  SubjectType,
  TargetType,
  Task,
  TaskDependency,
  Template,
  TemplateField,
  TemplateRow,
  User
} from '~/types/polaris'
import { formatCost, formatFileSize, formatHours, formatJSTDateTime, hasAtMostTwoDecimals, renderSafeMarkdown } from '~/utils/format'

const props = defineProps<{
  projectId: string
  projectName?: string
  targetType: TargetType
  issue?: Issue | null
  task?: Task | null
  parentIssueTitle?: string
  currentUserRole: ProjectRole
  statuses: Status[]
  labels: Label[]
  customFields: CustomFieldDefinition[]
  templates: Template[]
  users: User[]
  groups: Group[]
  sprints?: Sprint[]
  allTasks?: Task[]
  types?: ItemType[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'updated'): void
}>()

const router = useRouter()
const route = useRoute()
const { baseUrl, apiFetch, currentUser, notifyToast } = useApi()

const targetId = computed(() => (props.targetType === 'issue' ? props.issue?.id || '' : props.task?.id || ''))
const targetPathPlural = computed(() => (props.targetType === 'issue' ? 'issues' : 'tasks'))
const ownerUserId = computed(() => (props.targetType === 'issue' ? props.issue?.owner_user_id || '' : props.task?.owner_user_id || ''))

const canEditAny = computed(() => props.currentUserRole === 'admin' || props.currentUserRole === 'editor')
const canTransferOwner = computed(() => {
  return props.currentUserRole === 'admin' || (currentUser.value && currentUser.value.id === ownerUserId.value)
})

const modalTab = ref<'details' | 'history'>('details')

const applicableTemplates = computed(() => props.templates.filter(t => t.target_type === props.targetType))

const isSaving = ref(false)
const saveSuccess = ref(false)
const initialSnapshot = ref('')
const isDirty = computed(() => {
  if (!initialSnapshot.value) return false
  return JSON.stringify(form) !== initialSnapshot.value
})

function confirmClose() {
  if (isDirty.value && !confirm('未保存の変更があります。破棄して閉じますか？')) {
    return
  }
  emit('close')
}

function goToParentIssue() {
  if (!props.task?.issue_id) return
  if (isDirty.value && !confirm('未保存の変更があります。破棄して親課題へ移動しますか？')) {
    return
  }
  router.push({ query: { ...route.query, item: `issue:${props.task.issue_id}` } })
}

const ownerSelectRef = ref<HTMLSelectElement | null>(null)
async function onOwnerSelectChange(newOwnerId: string) {
  if (!newOwnerId || newOwnerId === ownerUserId.value) return
  const targetUser = props.users.find(u => u.id === newOwnerId)
  const targetName = targetUser?.display_name || newOwnerId
  if (!confirm(`オーナーを「${targetName}」に移譲しますか？権限が変更される場合があります。`)) {
    if (ownerSelectRef.value) {
      ownerSelectRef.value.value = ownerUserId.value
    }
    return
  }
  await transferOwner(newOwnerId)
}

const todayStr = computed(() => new Date().toISOString().slice(0, 10))
function setTodayAsActualEnd() {
  form.actual_end_date = todayStr.value
}

const plannedHoursNum = computed(() => {
  const v = form.planned_hours
  return (v !== '' && v !== null && !isNaN(Number(v))) ? Number(v) : 0
})
const actualHoursNum = computed(() => {
  const v = form.actual_hours
  return (v !== '' && v !== null && !isNaN(Number(v))) ? Number(v) : 0
})
const hoursBurnRatio = computed(() => {
  if (plannedHoursNum.value <= 0) return 0
  return (actualHoursNum.value / plannedHoursNum.value) * 100
})

function addActualHours(delta: number) {
  const cur = actualHoursNum.value
  const newVal = Math.round((cur + delta) * 100) / 100
  form.actual_hours = newVal
}

function formatHoursOnBlur(field: 'planned_hours' | 'actual_hours') {
  const val = form[field]
  if (val !== '' && val !== null && !isNaN(Number(val))) {
    form[field] = Number(Number(val).toFixed(2))
  }
}

async function copyItemUrl() {
  try {
    const url = new URL(window.location.href)
    url.searchParams.set('item', `${props.targetType}:${targetId.value}`)
    await navigator.clipboard.writeText(url.toString())
    notifyToast('詳細画面URLをコピーしました', 'success')
  } catch {
    notifyToast('URLコピーに失敗しました', 'error')
  }
}

const form = reactive({
  title: '',
  description: '',
  state: 'open' as 'open' | 'closed',
  template_id: '' as string | null,
  type_id: '' as string,
  status_id: '',
  progress: 0,
  planned_start_date: '',
  planned_end_date: '',
  actual_start_date: '',
  actual_end_date: '',
  planned_hours: '' as string | number,
  actual_hours: '' as string | number,
  planned_cost: '' as string | number,
  actual_cost: '' as string | number,
  story_points: '' as string | number,
  sprint_id: '' as string,
  assignees: [] as SubjectRef[],
  label_ids: [] as string[],
  custom_values: {} as Record<string, any>
})

const selectedType = computed<ItemType | null>(() => {
  if (!form.type_id) return null
  return (props.types || []).find(t => t.id === form.type_id) || null
})

const fieldPermissions = computed<Record<string, FieldPermissionState>>(() => {
  if (props.targetType === 'issue') {
    return props.issue?.field_permissions || {}
  }
  return props.task?.field_permissions || {}
})

const activeTemplate = computed<Template | null>(() => {
  return applicableTemplates.value.find(t => t.id === form.template_id) || applicableTemplates.value[0] || null
})

function isFieldHidden(f: TemplateField): boolean {
  if (f.field_source === 'standard' && f.field_key === 'title') return false
  if (f.is_hidden) return true
  return Boolean(fieldPermissions.value[f.field_key]?.is_hidden)
}

const layoutRows = computed<TemplateRow[]>(() => {
  let baseRows: TemplateRow[] = []
  if (activeTemplate.value && activeTemplate.value.rows?.length) {
    baseRows = activeTemplate.value.rows
  } else {
    const stdKeys =
      props.targetType === 'issue'
        ? ['title', 'type', 'labels', 'description']
        : ['title', 'type', 'status', 'progress', 'planned_dates', 'actual_dates', 'planned_hours', 'actual_hours', 'planned_cost', 'actual_cost', 'assignees', 'labels', 'description']
    baseRows = stdKeys.map((k, idx) => ({
      row_order: idx + 1,
      columns_count: 1,
      fields: [
        {
          column_order: 1,
          field_source: 'standard',
          field_key: k,
          is_hidden: false,
          is_restricted: false,
          allow_owner_view: true,
          allow_owner_edit: true,
          view_subjects: [],
          edit_subjects: []
        }
      ]
    }))
  }
  return baseRows
    .map((r) => {
      const visibleFields = (r.fields || []).filter(f => !isFieldHidden(f))
      return {
        ...r,
        columns_count: Math.min(r.columns_count, Math.max(visibleFields.length, 1)),
        fields: visibleFields
      }
    })
    .filter(r => r.fields.length > 0)
})

const subjectOptions = computed(() => {
  const list: { type: SubjectType; id: string; label: string }[] = []
  for (const u of props.users) {
    list.push({ type: 'user', id: u.id, label: `👤 ${u.display_name}` })
  }
  const seenGroupIds = new Set<string>()
  for (const g of props.groups) {
    if (!seenGroupIds.has(g.id)) {
      seenGroupIds.add(g.id)
      list.push({ type: 'group', id: g.id, label: `👥 ${g.name}` })
    }
  }
  return list
})

const attachments = ref<Attachment[]>([])
const comments = ref<Comment[]>([])
const newCommentBody = ref('')
const newCommentMentions = ref<SubjectRef[]>([])

watch(
  () => [props.issue, props.task],
  () => {
    initForm()
    loadAttachmentsAndComments()
  },
  { immediate: true }
)

function initForm() {
  if (props.targetType === 'issue' && props.issue) {
    form.title = props.issue.title
    form.description = props.issue.description || ''
    form.state = props.issue.state
    form.template_id = props.issue.template_id || applicableTemplates.value[0]?.id || null
    form.type_id = props.issue.type_id || ''
    form.label_ids = [...(props.issue.label_ids || [])]
    form.custom_values = JSON.parse(JSON.stringify(props.issue.custom_values || {}))
  } else if (props.targetType === 'task' && props.task) {
    form.title = props.task.title
    form.description = props.task.description || ''
    form.template_id = props.task.template_id || applicableTemplates.value[0]?.id || null
    form.type_id = props.task.type_id || ''
    form.status_id = props.task.status_id
    form.progress = props.task.progress
    form.planned_start_date = props.task.planned_start_date || ''
    form.planned_end_date = props.task.planned_end_date || ''
    form.actual_start_date = props.task.actual_start_date || ''
    form.actual_end_date = props.task.actual_end_date || ''
    form.planned_hours = props.task.planned_hours ?? ''
    form.actual_hours = props.task.actual_hours ?? ''
    form.planned_cost = props.task.planned_cost ?? ''
    form.actual_cost = props.task.actual_cost ?? ''
    form.story_points = props.task.story_points ?? ''
    form.sprint_id = props.task.sprint_id ?? ''
    form.assignees = JSON.parse(JSON.stringify(props.task.assignees || []))
    form.label_ids = [...(props.task.label_ids || [])]
    form.custom_values = JSON.parse(JSON.stringify(props.task.custom_values || {}))
  }
  nextTick(() => {
    initialSnapshot.value = JSON.stringify(form)
  })
}

async function loadAttachmentsAndComments() {
  if (!targetId.value) return
  try {
    const [atts, comms] = await Promise.all([
      apiFetch<Attachment[]>(`/projects/${props.projectId}/${targetPathPlural.value}/${targetId.value}/attachments`),
      apiFetch<Comment[]>(`/projects/${props.projectId}/${targetPathPlural.value}/${targetId.value}/comments`)
    ])
    attachments.value = atts || []
    comments.value = comms || []
  } catch {
    // ignore
  }
}

function isFieldViewable(f: TemplateField): boolean {
  const perm = fieldPermissions.value[f.field_key]
  if (!perm) return true
  return perm.can_view
}

function isKeyEditable(key: string): boolean {
  if (!canEditAny.value) return false
  const perm = fieldPermissions.value[key]
  if (!perm) return true
  return perm.can_edit
}

function isFieldEditable(f: TemplateField): boolean {
  return isKeyEditable(f.field_key)
}

function getFieldPlaceholder(f: TemplateField): string {
  return fieldPermissions.value[f.field_key]?.placeholder || '🔒 閲覧権限がありません'
}

function getCustomDef(fieldId: string): CustomFieldDefinition | undefined {
  return props.customFields.find(c => c.id === fieldId)
}

function getFieldLabel(f: TemplateField): string {
  if (f.field_source === 'custom') {
    return getCustomDef(f.field_key)?.name || 'カスタム項目'
  }
  const names: Record<string, string> = {
    title: 'タイトル',
    type: '種別 (Type)',
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
  return names[f.field_key] || f.field_key
}

function hasAssignee(type: SubjectType, id: string): boolean {
  return form.assignees.some(a => a.type === type && a.id === id)
}

function toggleAssignee(type: SubjectType, id: string) {
  const idx = form.assignees.findIndex(a => a.type === type && a.id === id)
  if (idx >= 0) {
    form.assignees.splice(idx, 1)
  } else {
    form.assignees.push({ type, id })
  }
}

function toggleLabel(labelId: string) {
  const idx = form.label_ids.indexOf(labelId)
  if (idx >= 0) {
    form.label_ids.splice(idx, 1)
  } else {
    form.label_ids.push(labelId)
  }
}

function toggleCustomCheckbox(fieldKey: string, opt: string) {
  if (!Array.isArray(form.custom_values[fieldKey])) {
    form.custom_values[fieldKey] = []
  }
  const arr = form.custom_values[fieldKey] as string[]
  const idx = arr.indexOf(opt)
  if (idx >= 0) {
    arr.splice(idx, 1)
  } else {
    arr.push(opt)
  }
}

async function saveItem() {
  if (isSaving.value) return
  isSaving.value = true
  try {
    if (props.targetType === 'issue') {
      const payload: Record<string, any> = {}
      if (isKeyEditable('title')) payload.title = form.title
      if (isKeyEditable('description')) payload.description = form.description
      if (isKeyEditable('state')) payload.state = form.state
      if (form.template_id) payload.template_id = form.template_id
      payload.type_id = form.type_id || null
      if (isKeyEditable('labels')) payload.label_ids = form.label_ids

      const filteredCustom: Record<string, any> = {}
      for (const [k, v] of Object.entries(form.custom_values)) {
        if (isKeyEditable(k)) {
          filteredCustom[k] = v
        }
      }
      if (Object.keys(filteredCustom).length > 0) {
        payload.custom_values = filteredCustom
      }

      await apiFetch(`/projects/${props.projectId}/issues/${targetId.value}`, {
        method: 'PUT',
        body: JSON.stringify(payload)
      })
    } else {
      const payload: Record<string, any> = {}
      if (isKeyEditable('title')) payload.title = form.title
      if (isKeyEditable('description')) payload.description = form.description
      if (form.template_id) payload.template_id = form.template_id
      payload.type_id = form.type_id || null
      if (isKeyEditable('status')) payload.status_id = form.status_id
      if (isKeyEditable('progress')) payload.progress = Number(form.progress)

      if (isKeyEditable('planned_dates')) {
        payload.planned_start_date = form.planned_start_date || ''
        payload.planned_end_date = form.planned_end_date || ''
      }
      if (isKeyEditable('actual_dates')) {
        payload.actual_start_date = form.actual_start_date || ''
        payload.actual_end_date = form.actual_end_date || ''
      }

      if (isKeyEditable('planned_hours')) {
        const pHours = form.planned_hours === '' || form.planned_hours === null ? null : Number(form.planned_hours)
        if (pHours !== null && !hasAtMostTwoDecimals(pHours)) {
          notifyToast('予定工数は小数点第2位までで入力してください', 'error')
          return
        }
        payload.planned_hours = pHours
      }

      if (isKeyEditable('actual_hours')) {
        const aHours = form.actual_hours === '' || form.actual_hours === null ? null : Number(form.actual_hours)
        if (aHours !== null && !hasAtMostTwoDecimals(aHours)) {
          notifyToast('実績工数は小数点第2位までで入力してください', 'error')
          return
        }
        payload.actual_hours = aHours
      }

      if (isKeyEditable('planned_cost')) {
        const pCost = form.planned_cost === '' || form.planned_cost === null ? null : Number(form.planned_cost)
        if (pCost !== null && !hasAtMostTwoDecimals(pCost)) {
          notifyToast('予定コストは小数点第2位までで入力してください', 'error')
          return
        }
        payload.planned_cost = pCost
      }

      if (isKeyEditable('actual_cost')) {
        const aCost = form.actual_cost === '' || form.actual_cost === null ? null : Number(form.actual_cost)
        if (aCost !== null && !hasAtMostTwoDecimals(aCost)) {
          notifyToast('実績コストは小数点第2位までで入力してください', 'error')
          return
        }
        payload.actual_cost = aCost
      }

      if (form.story_points !== '' && form.story_points !== null) {
        payload.story_points = Number(form.story_points)
      } else {
        payload.story_points = null
      }
      payload.sprint_id = form.sprint_id || null

      if (isKeyEditable('assignees')) payload.assignees = form.assignees
      if (isKeyEditable('labels')) payload.label_ids = form.label_ids

      const filteredCustom: Record<string, any> = {}
      for (const [k, v] of Object.entries(form.custom_values)) {
        if (isKeyEditable(k)) {
          filteredCustom[k] = v
        }
      }
      if (Object.keys(filteredCustom).length > 0) {
        payload.custom_values = filteredCustom
      }

      await apiFetch(`/projects/${props.projectId}/tasks/${targetId.value}`, {
        method: 'PUT',
        body: JSON.stringify(payload)
      })
    }
    initialSnapshot.value = JSON.stringify(form)
    saveSuccess.value = true
    setTimeout(() => {
      saveSuccess.value = false
    }, 2000)
    notifyToast('変更を保存しました', 'success')
    emit('updated')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    isSaving.value = false
  }
}

async function transferOwner(newOwnerId: string) {
  if (!newOwnerId || newOwnerId === ownerUserId.value) return
  try {
    const endpoint =
      props.targetType === 'issue'
        ? `/projects/${props.projectId}/issues/${targetId.value}/owner`
        : `/projects/${props.projectId}/tasks/${targetId.value}/owner`
    await apiFetch(endpoint, {
      method: 'PUT',
      body: JSON.stringify({ owner_user_id: newOwnerId })
    })
    notifyToast('オーナーを委譲しました', 'success')
    emit('updated')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteItem() {
  if (props.targetType === 'issue') {
    const taskCount = props.issue?.rollup?.task_count || 0
    const msg =
      taskCount > 0
        ? `この課題には配下タスクが ${taskCount} 件あります。課題を削除すると配下のタスクとコメント・添付もすべて削除されます。本当に削除しますか？`
        : 'この課題を削除しますか？'
    if (!confirm(msg)) return
    try {
      await apiFetch(`/projects/${props.projectId}/issues/${targetId.value}`, { method: 'DELETE' })
      notifyToast('課題を削除しました', 'success')
      emit('updated')
      emit('close')
    } catch (e: any) {
      notifyToast(e.message, 'error')
    }
  } else {
    if (!confirm('このタスクを削除しますか？')) return
    try {
      await apiFetch(`/projects/${props.projectId}/tasks/${targetId.value}`, { method: 'DELETE' })
      notifyToast('タスクを削除しました', 'success')
      emit('updated')
      emit('close')
    } catch (e: any) {
      notifyToast(e.message, 'error')
    }
  }
}

// Attachments logic
const isDraggingFiles = ref(false)
const lightboxAtt = ref<Attachment | null>(null)

function isImageAttachment(att: Attachment): boolean {
  return /\.(png|jpe?g|gif|webp|svg)$/i.test(att.file_name)
}

function getAttachmentDownloadUrl(attId: string): string {
  return `${baseUrl}/projects/${props.projectId}/attachments/${attId}/download`
}

function openLightbox(att: Attachment) {
  lightboxAtt.value = att
}

async function uploadAttachmentFile(file: File): Promise<Attachment | null> {
  const fd = new FormData()
  fd.append('file', file)
  try {
    const created = await apiFetch<Attachment>(
      `/projects/${props.projectId}/${targetPathPlural.value}/${targetId.value}/attachments`,
      {
        method: 'POST',
        body: fd
      }
    )
    await loadAttachmentsAndComments()
    notifyToast(`ファイル「${file.name}」を添付しました`, 'success')
    return created
  } catch (e: any) {
    notifyToast(e.message, 'error')
    return null
  }
}

async function onUploadFile(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  await uploadAttachmentFile(file)
  input.value = ''
}

async function onDropFiles(e: DragEvent) {
  isDraggingFiles.value = false
  const files = e.dataTransfer?.files
  if (!files || files.length === 0) return
  for (let i = 0; i < files.length; i++) {
    await uploadAttachmentFile(files[i])
  }
}

function onWindowPaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items
  if (!items) return
  for (let i = 0; i < items.length; i++) {
    if (items[i].type.startsWith('image/')) {
      const file = items[i].getAsFile()
      if (file) {
        e.preventDefault()
        uploadAttachmentFile(file)
        break
      }
    }
  }
}

async function onUploadFileFromComment(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  const att = await uploadAttachmentFile(file)
  if (att) {
    const dlUrl = getAttachmentDownloadUrl(att.id)
    newCommentBody.value += `\n[📎 ${att.file_name}](${dlUrl})`
  }
  input.value = ''
}

async function deleteAttachment(attachmentId: string) {
  if (!confirm('この添付ファイルを削除しますか？')) return
  try {
    await apiFetch(`/projects/${props.projectId}/attachments/${attachmentId}`, { method: 'DELETE' })
    await loadAttachmentsAndComments()
    notifyToast('添付ファイルを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

// Comments logic
const commentTab = ref<'write' | 'preview'>('write')
const editingCommentId = ref<string | null>(null)
const editingCommentBody = ref('')
const isSavingComment = ref(false)

function startEditComment(c: Comment) {
  editingCommentId.value = c.id
  editingCommentBody.value = c.body
}

function cancelEditComment() {
  editingCommentId.value = null
  editingCommentBody.value = ''
}

async function saveEditComment(commentId: string) {
  if (!editingCommentBody.value.trim()) return
  isSavingComment.value = true
  try {
    await apiFetch(`/projects/${props.projectId}/comments/${commentId}`, {
      method: 'PUT',
      body: JSON.stringify({
        body: editingCommentBody.value,
        mentions: []
      })
    })
    editingCommentId.value = null
    editingCommentBody.value = ''
    await loadAttachmentsAndComments()
    notifyToast('コメントを更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    isSavingComment.value = false
  }
}

// Mention inline suggest
const commentTextareaRef = ref<HTMLTextAreaElement | null>(null)
const showMentionSuggest = ref(false)
const mentionQuery = ref('')
const mentionSuggestStyle = ref<Record<string, string>>({ top: '38px', left: '10px' })

const filteredMentionSubjects = computed(() => {
  if (!mentionQuery.value) return subjectOptions.value.slice(0, 8)
  const q = mentionQuery.value.toLowerCase()
  return subjectOptions.value.filter(s => s.label.toLowerCase().includes(q)).slice(0, 8)
})

function onCommentInput() {
  const el = commentTextareaRef.value
  if (!el) return
  const textBeforeCursor = newCommentBody.value.slice(0, el.selectionStart || 0)
  const match = textBeforeCursor.match(/@([^\s@]*)$/)
  if (match) {
    showMentionSuggest.value = true
    mentionQuery.value = match[1]
  } else {
    showMentionSuggest.value = false
    mentionQuery.value = ''
  }
}

function selectMentionSuggest(sub: { type: SubjectType; id: string; label: string }) {
  const el = commentTextareaRef.value
  if (!el) return
  const curPos = el.selectionStart || 0
  const textBeforeCursor = newCommentBody.value.slice(0, curPos)
  const textAfterCursor = newCommentBody.value.slice(curPos)
  const match = textBeforeCursor.match(/@([^\s@]*)$/)
  if (match) {
    const replaceFrom = match.index!
    const cleanLabel = sub.label.replace(/^[👤👥]\s*/, '')
    const inserted = `@${cleanLabel} `
    newCommentBody.value = textBeforeCursor.slice(0, replaceFrom) + inserted + textAfterCursor
    if (!newCommentMentions.value.some(m => m.type === sub.type && m.id === sub.id)) {
      newCommentMentions.value.push({ type: sub.type, id: sub.id })
    }
    showMentionSuggest.value = false
    mentionQuery.value = ''
    nextTick(() => {
      const newPos = replaceFrom + inserted.length
      el.setSelectionRange(newPos, newPos)
      el.focus()
    })
  }
}

function addCommentMention(val: string) {
  if (!val) return
  const [t, id] = val.split(':')
  if ((t === 'user' || t === 'group') && id) {
    if (!newCommentMentions.value.some(m => m.type === t && m.id === id)) {
      newCommentMentions.value.push({ type: t, id })
      const label = getSubjectLabel({ type: t, id })
      newCommentBody.value += ` @${label} `
    }
  }
}

const draftStorageKey = computed(() => targetId.value ? `polaris_comment_draft_${targetId.value}` : null)

watch(draftStorageKey, (key) => {
  if (typeof window !== 'undefined' && key) {
    const saved = localStorage.getItem(key)
    if (saved) {
      newCommentBody.value = saved
    } else {
      newCommentBody.value = ''
    }
  }
}, { immediate: true })

watch(newCommentBody, (val) => {
  if (typeof window !== 'undefined' && draftStorageKey.value) {
    if (val.trim()) {
      localStorage.setItem(draftStorageKey.value, val)
    } else {
      localStorage.removeItem(draftStorageKey.value)
    }
  }
})

function onKeydown(e: KeyboardEvent) {
  // Shortcut: Ctrl+S or Cmd+S
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
    e.preventDefault()
    saveItem()
    return
  }

  // Escape to close
  if (e.key === 'Escape') {
    const activeEl = document.activeElement as HTMLElement | null
    const isEditing = activeEl && (
      activeEl.tagName === 'INPUT' ||
      activeEl.tagName === 'TEXTAREA' ||
      activeEl.tagName === 'SELECT' ||
      activeEl.isContentEditable
    )
    if (isEditing) {
      activeEl.blur()
      e.preventDefault()
      return
    }
    confirmClose()
  }
}

function onBeforeUnload(e: BeforeUnloadEvent) {
  if (isDirty.value) {
    e.preventDefault()
    e.returnValue = ''
  }
}

onBeforeRouteLeave((to, from, next) => {
  if (isDirty.value) {
    const ok = window.confirm('未保存の変更があります。破棄して移動しますか？')
    if (!ok) {
      next(false)
      return
    }
  }
  next()
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('paste', onWindowPaste)
  window.addEventListener('beforeunload', onBeforeUnload)
})

onUnmounted(() => {
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('paste', onWindowPaste)
  window.removeEventListener('beforeunload', onBeforeUnload)
})

async function copyItemId() {
  if (!targetId.value) return
  try {
    await navigator.clipboard.writeText(targetId.value)
    notifyToast(`ID「${targetId.value}」をクリップボードにコピーしました`, 'success')
  } catch {
    notifyToast('IDのコピーに失敗しました', 'error')
  }
}

async function postComment() {
  if (!newCommentBody.value.trim()) return
  try {
    await apiFetch(`/projects/${props.projectId}/${targetPathPlural.value}/${targetId.value}/comments`, {
      method: 'POST',
      body: JSON.stringify({
        body: newCommentBody.value,
        mentions: newCommentMentions.value
      })
    })
    newCommentBody.value = ''
    newCommentMentions.value = []
    if (typeof window !== 'undefined' && draftStorageKey.value) {
      localStorage.removeItem(draftStorageKey.value)
    }
    await loadAttachmentsAndComments()
    notifyToast('コメントを投稿しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function removeComment(commentId: string) {
  if (!confirm('このコメントを削除しますか？')) return
  try {
    await apiFetch(`/projects/${props.projectId}/comments/${commentId}`, { method: 'DELETE' })
    await loadAttachmentsAndComments()
    notifyToast('コメントを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function getUserName(uid: string): string {
  return props.users.find(u => u.id === uid)?.display_name || uid.slice(0, 8)
}

function getSubjectLabel(s: SubjectRef): string {
  if (s.type === 'user') return getUserName(s.id)
  return props.groups.find(g => g.id === s.id)?.name || s.id.slice(0, 8)
}

// Sprint and Dependency State
const projectSprints = ref<Sprint[]>([])
const projectDependencies = ref<TaskDependency[]>([])
const allTasksList = ref<Task[]>([])
const newPredecessorId = ref('')
const newSuccessorId = ref('')
const newDepType = ref<DependencyType>('FS')
const newDepLag = ref(0)
const isAddingDep = ref(false)

async function loadSprintsAndDependencies() {
  if (!props.projectId) return
  try {
    const [sps, deps] = await Promise.all([
      props.sprints ? Promise.resolve(props.sprints) : apiFetch<Sprint[]>(`/projects/${props.projectId}/sprints`).catch(() => []),
      apiFetch<TaskDependency[]>(`/projects/${props.projectId}/dependencies`).catch(() => [])
    ])
    projectSprints.value = sps || []
    projectDependencies.value = deps || []
  } catch {}
}

async function loadAllProjectTasks() {
  if (props.allTasks && props.allTasks.length > 0) {
    allTasksList.value = props.allTasks
    return
  }
  try {
    const issuesList = await apiFetch<Issue[]>(`/projects/${props.projectId}/issues?state=all`).catch(() => [])
    const tasks: Task[] = []
    for (const iss of issuesList || []) {
      for (const t of iss.tasks || []) {
        tasks.push(t)
      }
    }
    allTasksList.value = tasks
  } catch {}
}

watch(
  () => [props.targetType, props.task?.id, props.projectId],
  () => {
    if (props.targetType === 'task') {
      loadSprintsAndDependencies()
      loadAllProjectTasks()
    }
  },
  { immediate: true }
)

const predecessorDeps = computed(() => {
  if (props.targetType !== 'task' || !targetId.value) return []
  return projectDependencies.value.filter(d => d.successor_id === targetId.value)
})

const successorDeps = computed(() => {
  if (props.targetType !== 'task' || !targetId.value) return []
  return projectDependencies.value.filter(d => d.predecessor_id === targetId.value)
})

const availablePredecessorTasks = computed(() => {
  return allTasksList.value.filter(t => t.id !== targetId.value && !predecessorDeps.value.some(d => d.predecessor_id === t.id))
})

const availableSuccessorTasks = computed(() => {
  return allTasksList.value.filter(t => t.id !== targetId.value && !successorDeps.value.some(d => d.successor_id === t.id))
})

function getTaskTitle(taskId: string): string {
  const found = allTasksList.value.find(t => t.id === taskId)
  return found ? found.title : taskId.slice(0, 8)
}

async function createDependency(direction: 'predecessor' | 'successor') {
  if (!targetId.value || isAddingDep.value) return
  const predId = direction === 'predecessor' ? newPredecessorId.value : targetId.value
  const succId = direction === 'predecessor' ? targetId.value : newSuccessorId.value
  if (!predId || !succId) return

  isAddingDep.value = true
  try {
    await apiFetch(`/projects/${props.projectId}/dependencies`, {
      method: 'POST',
      body: JSON.stringify({
        predecessor_id: predId,
        successor_id: succId,
        dependency_type: newDepType.value,
        lag_days: Number(newDepLag.value) || 0
      })
    })
    notifyToast('依存関係を登録しました', 'success')
    if (direction === 'predecessor') {
      newPredecessorId.value = ''
    } else {
      newSuccessorId.value = ''
    }
    newDepLag.value = 0
    await loadSprintsAndDependencies()
    emit('updated')
  } catch (e: any) {
    notifyToast(e.message || '依存関係の作成に失敗しました（循環依存の可能性があります）', 'error')
  } finally {
    isAddingDep.value = false
  }
}

async function removeDependency(depId: string) {
  if (!confirm('この依存関係を解除しますか？')) return
  try {
    await apiFetch(`/projects/${props.projectId}/dependencies/${depId}`, {
      method: 'DELETE'
    })
    notifyToast('依存関係を解除しました', 'success')
    await loadSprintsAndDependencies()
    emit('updated')
  } catch (e: any) {
    notifyToast(e.message || '依存関係の解除に失敗しました', 'error')
  }
}
</script>

