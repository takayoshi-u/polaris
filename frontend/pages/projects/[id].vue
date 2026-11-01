<template>
  <div style="display: flex; flex-direction: column; gap: 18px;">
    <!-- DEDICATED DETAIL PAGE TRANSITION VIEW -->
    <ItemDetailModal
      v-if="detailModalState.open"
      :project-id="projectId"
      :project-name="project?.name"
      :target-type="detailModalState.targetType"
      :issue="detailModalState.issue"
      :task="detailModalState.task"
      :parent-issue-title="detailModalState.parentIssueTitle"
      :current-user-role="project?.current_user_role || 'viewer'"
      :statuses="statuses"
      :labels="allLabels"
      :custom-fields="customFields"
      :templates="templates"
      :users="users"
      :groups="allGroups"
      :sprints="sprints"
      :all-tasks="allTasks"
      :types="projectTypes"
      @close="closeDetailView"
      @updated="onDetailUpdated"
    />

    <!-- WORKSPACE VIEWS -->
    <template v-else>
      <!-- Top Project Workspace Header (Basic Info & Actions/Settings) -->
      <div class="card" style="padding: 14px 18px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 14px;">
        <div style="display: flex; align-items: center; gap: 12px;">
          <div>
            <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
              <h1 style="font-size: 17px; font-weight: 700;">{{ project?.name || 'プロジェクト' }}</h1>
              <span v-if="project" class="badge badge-blue">
                基準: {{ formatHours(project.hours_per_day) }}h / 日 ・ Role: {{ project.current_user_role.toUpperCase() }}
              </span>
              <span v-if="project" class="badge badge-green">
                PJ総コスト(予/実): {{ formatCost(project.total_planned_cost) }} / {{ formatCost(project.total_actual_cost) }}
              </span>
            </div>
            <p v-if="project?.description" style="font-size: 12px; color: var(--text-muted); margin-top: 4px; margin-bottom: 0;">{{ project.description }}</p>
          </div>
        </div>

        <!-- Project Actions & Settings Buttons -->
        <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
          <button class="btn btn-secondary btn-sm" title="タスクデータをCSVでエクスポート" @click="exportProjectCSV">
            📤 CSV エクスポート
          </button>
          <button v-if="canEditProjectItems" class="btn btn-secondary btn-sm" title="CSVからタスクを一括インポート" @click="showCSVImportModal = true">
            📥 CSV インポート
          </button>
          <button
            v-if="canManageProjectSettings"
            class="btn btn-sm"
            :class="activeTab === 'settings' ? 'btn-primary' : 'btn-secondary'"
            title="プロジェクト設定"
            @click="activeTab = 'settings'"
          >
            ⚙️ プロジェクト設定
          </button>
        </div>
      </div>

      <!-- View Switcher Tabs Bar -->
      <div class="card" style="padding: 8px 12px; display: flex; flex-wrap: wrap; align-items: center; gap: 6px;">
        <button
          class="btn"
          :class="activeTab === 'kanban' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'kanban'"
        >
          📋 カンバン
        </button>
        <button
          class="btn"
          :class="activeTab === 'gantt' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'gantt'"
        >
          📊 ガント
        </button>
        <button
          class="btn"
          :class="activeTab === 'list' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'list'"
        >
          📑 リスト / テーブル
        </button>
        <button
          class="btn"
          :class="activeTab === 'sprint' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'sprint'"
        >
          🏃 スプリント
        </button>
        <button
          class="btn"
          :class="activeTab === 'workload' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'workload'"
        >
          👥 リソース負荷
        </button>
        <button
          class="btn"
          :class="activeTab === 'wiki' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'wiki'"
        >
          📖 Wiki
        </button>
        <button
          class="btn"
          :class="activeTab === 'activity' ? 'btn-primary' : 'btn-secondary'"
          @click="activeTab = 'activity'"
        >
          ⏱️ アクティビティ
        </button>
      </div>

    <!-- Common Filter Bar (Kanban / Gantt / List) -->
    <div
      v-if="activeTab !== 'settings' && activeTab !== 'sprint' && activeTab !== 'workload' && activeTab !== 'wiki' && activeTab !== 'activity'"
      class="card"
      style="padding: 12px 16px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;"
    >
      <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 10px;">
        <input
          v-model="filters.keyword"
          type="text"
          placeholder="キーワード検索..."
          style="width: 220px;"
          @keyup.enter="loadIssues"
        />

        <div style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">課題状態:</span>
          <select v-model="filters.issue_state" @change="loadIssues">
            <option value="open">オープン</option>
            <option value="closed">クローズ</option>
            <option value="all">すべて</option>
          </select>
        </div>

        <div style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">種別:</span>
          <select v-model="filters.type_id" @change="loadIssues">
            <option value="">すべての種別</option>
            <option v-for="tp in projectTypes" :key="tp.id" :value="tp.id">
              {{ tp.name }}
            </option>
          </select>
        </div>

        <div style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">ステータス:</span>
          <select v-model="filters.status_id" @change="loadIssues">
            <option value="">すべて</option>
            <option v-for="st in statuses" :key="st.id" :value="st.id">
              {{ st.display_order }}. {{ st.name }}
            </option>
          </select>
        </div>

        <div style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">担当者:</span>
          <select v-model="filters.assignee_id" @change="loadIssues">
            <option value="">すべて</option>
            <optgroup label="ユーザー">
              <option v-for="u in users" :key="u.id" :value="u.id">👤 {{ u.display_name }}</option>
            </optgroup>
            <optgroup label="グループ">
              <option v-for="g in allGroups" :key="g.id" :value="g.id">👥 {{ g.name }}</option>
            </optgroup>
          </select>
        </div>

        <div style="display: flex; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">スプリント:</span>
          <select v-model="filters.sprint_id" @change="loadIssues">
            <option value="">すべて</option>
            <option value="active">⚡ アクティブスプリントのみ</option>
            <option value="unassigned">未割当 (バックログ)</option>
            <optgroup v-if="sprints.length > 0" label="各スプリント">
              <option v-for="sp in sprints" :key="sp.id" :value="sp.id">
                {{ sp.status === 'active' ? '⚡ ' : sp.status === 'closed' ? '✓ ' : '📅 ' }}{{ sp.name }}
              </option>
            </optgroup>
          </select>
        </div>

        <!-- AND Label Multi-Select Pills -->
        <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 4px; font-size: 12px;">
          <span style="color: var(--text-muted);">ラベル(AND):</span>
          <button
            v-for="lb in allLabels"
            :key="lb.id"
            type="button"
            class="btn btn-sm"
            :style="{
              padding: '2px 8px',
              fontSize: '11px',
              borderColor: filters.label_ids.includes(lb.id) ? lb.color : 'var(--border-color)',
              backgroundColor: filters.label_ids.includes(lb.id) ? 'rgba(59,130,246,0.2)' : 'var(--bg-input)',
              color: lb.color
            }"
            @click="toggleFilterLabel(lb.id)"
          >
            ● {{ lb.name }}
          </button>
        </div>

        <button class="btn btn-secondary btn-sm" @click="loadIssues">検索</button>
      </div>

      <div v-if="canEditProjectItems" style="display: flex; gap: 8px;">
        <button class="btn btn-primary" @click="openNewIssueModal">
          ＋ 新規課題作成
        </button>
      </div>
    </div>

    <!-- VIEW 1: MATRIX KANBAN BOARD -->
    <section v-if="activeTab === 'kanban'" style="display: flex; flex-direction: column; gap: 14px;">
      <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px;">
        <div>
          <h2 style="font-size: 16px; font-weight: 700;">
            マトリクス・カンバンボード
          </h2>
          <p style="font-size: 13px; color: var(--text-muted);">
            ドラッグ＆ドロップでステータス変更・親課題の付け替え・表示順序の調整が可能です
          </p>
        </div>
        <div style="display: flex; gap: 8px; align-items: center; flex-wrap: wrap;">
          <!-- Status Column Quick Jump -->
          <div v-if="statuses.length > 0" style="display: flex; align-items: center; gap: 6px;">
            <select
              style="padding: 4px 8px; font-size: 12px; height: 28px; border-radius: 6px;"
              aria-label="列へジャンプ"
              @change="onJumpSelect($event)"
            >
              <option value="" disabled selected>📍 列へ移動...</option>
              <option v-for="(st, idx) in statuses" :key="st.id" :value="idx">
                {{ st.display_order }}. {{ st.name }} ({{ countTasksByStatus(st.id) }}件)
              </option>
            </select>
          </div>

          <!-- Horizontal Scroll Navigation Buttons (1 column / ~280px smooth scroll) -->
          <div v-if="kanbanIsOverflowing" style="display: inline-flex; align-items: center; gap: 4px;">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              style="padding: 2px 8px; height: 28px; display: inline-flex; align-items: center; justify-content: center; font-size: 12px;"
              :disabled="!kanbanCanScrollLeft"
              title="左へスクロール"
              aria-label="左へスクロール"
              @click="scrollKanban('left')"
            >
              ◀
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              style="padding: 2px 8px; height: 28px; display: inline-flex; align-items: center; justify-content: center; font-size: 12px;"
              :disabled="!kanbanCanScrollRight"
              title="右へスクロール"
              aria-label="右へスクロール"
              @click="scrollKanban('right')"
            >
              ▶
            </button>
          </div>

          <button type="button" class="btn btn-secondary btn-sm" @click="expandAllIssues">すべて展開</button>
          <button type="button" class="btn btn-secondary btn-sm" @click="collapseAllIssues">すべて折りたたむ</button>
        </div>
      </div>

      <!-- Top Synchronized Scrollbar Container -->
      <div
        v-show="kanbanIsOverflowing"
        ref="kanbanTopScrollRef"
        class="kanban-top-scroll-wrapper"
        style="overflow-x: auto; overflow-y: hidden; width: 100%; height: 10px; margin-bottom: -6px; z-index: 15; position: sticky; top: 0; background: var(--bg-main);"
        @scroll="onTopScroll"
      >
        <div :style="{ minWidth: `${Math.max(statuses.length, 1) * 260}px`, height: '1px' }"></div>
      </div>

      <!-- Scrollable Unified Kanban Container -->
      <div
        ref="kanbanScrollContainerRef"
        class="kanban-scroll-wrapper"
        style="overflow-x: auto; width: 100%; padding-bottom: 12px;"
        @scroll="onMainScroll"
      >
        <div :style="{ minWidth: `${Math.max(statuses.length, 1) * 260}px`, display: 'flex', flexDirection: 'column', gap: '14px' }">
          <!-- Status Column Headers (Sticky Top) -->
          <div
            :style="{
              display: 'grid',
              gridTemplateColumns: `repeat(${Math.max(statuses.length, 1)}, minmax(240px, 1fr))`,
              gap: '12px',
              position: 'sticky',
              top: '0',
              zIndex: '10',
              background: 'var(--bg-main)',
              paddingBottom: '4px'
            }"
          >
            <div
              v-for="st in statuses"
              :key="st.id"
              class="card"
              style="padding: 10px 14px; display: flex; align-items: center; justify-content: space-between;"
            >
              <span style="font-size: 14px; font-weight: 700; display: flex; align-items: center; gap: 8px;">
                <span :style="{ width: '10px', height: '10px', borderRadius: '999px', backgroundColor: st.color }"></span>
                {{ st.display_order }}. {{ st.name }}
              </span>
              <span style="font-size: 13px; color: var(--text-muted);">
                {{ countTasksByStatus(st.id) }}件
              </span>
            </div>
          </div>

          <div v-if="issues.length === 0" class="card" style="text-align: center; padding: 40px;">
            <p style="font-size: 15px; font-weight: 600;">該当する課題がありません</p>
          </div>

          <!-- Swimlanes per Issue -->
          <div v-for="iss in issues" :key="iss.id" class="kanban-swimlane">
            <!-- Issue Rollup Header -->
            <div class="kanban-swimlane-header">
              <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="padding: 2px 6px; font-size: 13px; font-weight: 700;"
                  :title="collapsedIssueIds.has(iss.id) ? '展開' : '折りたたむ'"
                  @click="toggleIssueCollapse(iss.id)"
                >
                  {{ collapsedIssueIds.has(iss.id) ? '▶' : '▼' }}
                </button>
                <span :class="['badge', iss.state === 'open' ? 'badge-green' : 'badge-slate']">
                  {{ iss.state.toUpperCase() }}
                </span>
                <span
                  v-if="getType(iss.type_id)"
                  class="badge"
                  :style="{
                    backgroundColor: getType(iss.type_id)?.color,
                    color: '#fff',
                    fontSize: '11px',
                    fontWeight: '700'
                  }"
                >
                  {{ getType(iss.type_id)?.name }}
                </span>
                <span
                  style="font-size: 15px; font-weight: 700; cursor: pointer; color: var(--primary);"
                  @click="openIssueDetail(iss)"
                >
                  {{ iss.title }}
                </span>
                <span
                  v-for="lid in iss.label_ids"
                  :key="lid"
                  class="badge badge-slate"
                  :style="{ color: getLabel(lid)?.color || '#cbd5e1' }"
                >
                  ● {{ getLabel(lid)?.name }}
                </span>
                <span style="font-size: 13px; color: var(--text-muted);">
                  オーナー: {{ getUserName(iss.owner_user_id) }}
                </span>
              </div>

              <!-- Auto-aggregated Rollup Metrics -->
              <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 14px; font-size: 13px;">
                <div>
                  <span style="color: var(--text-muted);">自動集計予定:</span>
                  <strong style="margin-left: 4px;">
                    {{ iss.rollup.planned_start_date || '-' }} 〜 {{ iss.rollup.planned_end_date || '-' }}
                  </strong>
                </div>
                <div>
                  <span style="color: var(--text-muted);">工数集計(予/実):</span>
                  <strong style="margin-left: 4px;">
                    {{ formatHours(iss.rollup.total_planned_hours) }}h / {{ formatHours(iss.rollup.total_actual_hours) }}h
                  </strong>
                </div>
                <div>
                  <span style="color: var(--text-muted);">コスト集計(予/実):</span>
                  <strong style="margin-left: 4px;">
                    {{ formatCost(iss.rollup.total_planned_cost) }} / {{ formatCost(iss.rollup.total_actual_cost) }}
                  </strong>
                </div>
                <div>
                  <span style="color: var(--text-muted);">SP集計(完/計):</span>
                  <strong style="margin-left: 4px; color: #7c3aed;">
                    {{ iss.rollup.completed_story_points || 0 }} / {{ iss.rollup.total_story_points || 0 }} SP
                  </strong>
                </div>
                <div style="display: flex; align-items: center; gap: 6px;">
                  <span style="color: var(--text-muted);">加重平均進捗:</span>
                  <div style="width: 80px; height: 8px; border-radius: 999px; background: var(--border-color); overflow: hidden;">
                    <div :style="{ width: `${iss.rollup.weighted_progress}%`, height: '100%', backgroundColor: '#2563eb' }"></div>
                  </div>
                  <strong style="color: var(--primary);">{{ iss.rollup.weighted_progress.toFixed(1) }}%</strong>
                </div>
                <button
                  v-if="canEditProjectItems"
                  class="btn btn-secondary btn-sm"
                  @click="openNewTaskModal(iss)"
                >
                  ＋ 子タスク追加
                </button>
              </div>
            </div>

            <!-- Status Cells for this Issue (Collapsible) -->
            <div
              v-if="!collapsedIssueIds.has(iss.id)"
              :style="{
                display: 'grid',
                gridTemplateColumns: `repeat(${Math.max(statuses.length, 1)}, minmax(240px, 1fr))`,
                gap: '12px',
                padding: '12px'
              }"
            >
              <div
                v-for="st in statuses"
                :key="st.id"
                class="kanban-cell"
                :class="{ 'drag-over': dragOverKey === `${iss.id}:${st.id}` }"
                @dragover.prevent="dragOverKey = `${iss.id}:${st.id}`"
                @dragleave="dragOverKey = ''"
                @drop.prevent="onDropTaskCard(iss.id, st.id, getTasksForCell(iss, st.id).length + 1)"
              >
                <template
                  v-for="(t, tIdx) in getTasksForCell(iss, st.id)"
                  :key="t.id"
                >
                  <div v-if="dragOverCardId === t.id && dragOverPosition === 'before'" class="kanban-drop-indicator"></div>
                  <div
                    class="kanban-card"
                    :draggable="canEditProjectItems"
                    @dragstart="onDragStartTask(t)"
                    @dragover.prevent.stop="onCardDragOver($event, t.id)"
                    @dragleave="onCardDragLeave(t.id)"
                    @drop.prevent.stop="onDropTaskCard(iss.id, st.id, dragOverPosition === 'after' ? tIdx + 2 : tIdx + 1)"
                    @click="openTaskDetail(t, iss)"
                  >
                    <div style="display: flex; align-items: flex-start; justify-content: space-between; gap: 6px; margin-bottom: 6px;">
                      <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap;">
                        <span
                          v-if="getType(t.type_id)"
                          class="badge"
                          :style="{
                            backgroundColor: getType(t.type_id)?.color,
                            color: '#fff',
                            fontSize: '10px',
                            fontWeight: '700',
                            padding: '1px 6px'
                          }"
                        >
                          {{ getType(t.type_id)?.name }}
                        </span>
                        <span style="font-size: 14px; font-weight: 700;">{{ t.title }}</span>
                      </div>
                      <div style="display: flex; align-items: center; gap: 4px; flex-shrink: 0;">
                        <span v-if="t.story_points !== null && t.story_points !== undefined && t.story_points !== ''" class="badge badge-purple" style="font-size: 11px;">
                          {{ t.story_points }} SP
                        </span>
                        <span class="badge badge-blue">{{ t.progress }}%</span>
                      </div>
                    </div>

                    <div style="font-size: 12px; color: var(--text-muted); margin-bottom: 6px;">
                      <div>予定: {{ t.planned_start_date || '-' }}〜{{ t.planned_end_date || '-' }} ({{ formatHours(t.planned_hours) }}h)</div>
                      <div v-if="t.actual_start_date || t.actual_hours !== null">
                        実績: {{ t.actual_start_date || '-' }}〜{{ t.actual_end_date || '-' }} ({{ formatHours(t.actual_hours) }}h)
                      </div>
                      <div v-if="t.planned_cost !== null && t.planned_cost !== undefined || t.actual_cost !== null && t.actual_cost !== undefined">
                        コスト(予/実): {{ formatCost(t.planned_cost) }} / {{ formatCost(t.actual_cost) }}
                      </div>
                    </div>

                    <div style="width: 100%; height: 5px; border-radius: 999px; background: var(--border-color); overflow: hidden; margin-bottom: 8px;">
                      <div :style="{ width: `${t.progress}%`, height: '100%', backgroundColor: st.color || '#3b82f6' }"></div>
                    </div>

                    <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 4px;">
                      <div style="display: flex; flex-wrap: wrap; gap: 4px;">
                        <span
                          v-for="(a, aIdx) in t.assignees"
                          :key="aIdx"
                          class="badge badge-slate"
                          style="font-size: 11px;"
                        >
                          {{ a.type === 'user' ? `👤 ${getUserName(a.id)}` : `👥 ${getGroupName(a.id)}` }}
                        </span>
                      </div>
                    </div>
                  </div>
                  <div v-if="dragOverCardId === t.id && dragOverPosition === 'after'" class="kanban-drop-indicator"></div>
                </template>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- VIEW 2: DUAL-BAR GANTT CHART -->
    <section v-if="activeTab === 'gantt'" style="display: flex; flex-direction: column; gap: 14px;">
      <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
        <div>
          <h2 style="font-size: 15px; font-weight: 700;">
            ガントチャート
          </h2>
          <p style="font-size: 12px; color: var(--text-muted);">
            上段: 予定期間 (進捗塗) ／ 下段: 実績期間 (両端リサイズ対応)
          </p>
        </div>

        <div style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px;">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            title="階層ペインの表示/非表示を切り替え"
            @click="isGanttLeftCollapsed = !isGanttLeftCollapsed"
          >
            {{ isGanttLeftCollapsed ? '▶ 階層を表示' : '◀ 階層を隠す' }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="shiftTimeline(-7)">◀ -7日</button>
          <button class="btn btn-secondary btn-sm" @click="resetTimelineToday">今日へ</button>
          <button class="btn btn-primary btn-sm" @click="scrollToToday">📍 今日</button>
          <button class="btn btn-secondary btn-sm" @click="shiftTimeline(7)">+7日 ▶</button>

          <!-- Auto-scheduling Toggle -->
          <label style="display: flex; align-items: center; gap: 4px; font-size: 11px; cursor: pointer; padding: 2px 6px; border-radius: 6px; background: var(--bg-card); border: 1px solid var(--border-color);">
            <input type="checkbox" v-model="autoScheduleEnabled" />
            <span :style="{ color: autoScheduleEnabled ? 'var(--primary)' : 'var(--text-muted)', fontWeight: autoScheduleEnabled ? '700' : 'normal' }">
              ⚡ オートスケジューリング
            </span>
          </label>

          <!-- Dependencies Line Toggle -->
          <label style="display: flex; align-items: center; gap: 4px; font-size: 11px; cursor: pointer; padding: 2px 6px; border-radius: 6px; background: var(--bg-card); border: 1px solid var(--border-color);">
            <input type="checkbox" v-model="showDependencyLines" />
            <span>🔗 依存関係線 ({{ ganttDependencyLines.length }})</span>
          </label>

          <div style="display: flex; gap: 4px; padding: 3px; border-radius: 8px; background: var(--bg-card); border: 1px solid var(--border-color);">
            <button class="btn btn-sm" :class="ganttScale === 'day' ? 'btn-primary' : 'btn-ghost'" @click="ganttScale = 'day'">日</button>
            <button class="btn btn-sm" :class="ganttScale === 'week' ? 'btn-primary' : 'btn-ghost'" @click="ganttScale = 'week'">週</button>
            <button class="btn btn-sm" :class="ganttScale === 'month' ? 'btn-primary' : 'btn-ghost'" @click="ganttScale = 'month'">月</button>
          </div>
        </div>
      </div>

      <div class="gantt-container">
        <!-- Left Tree Pane -->
        <div v-show="!isGanttLeftCollapsed" class="gantt-left">
          <div class="gantt-row" style="height: 44px; background: var(--bg-elevated); padding: 0 12px; font-size: 13px; font-weight: 700;">
            <div style="flex: 1;">課題 / タスク階層</div>
            <div style="width: 96px; text-align: right;">予定/実績(h)</div>
            <div style="width: 54px; text-align: right;">進捗</div>
          </div>
          <!-- Sprint Period Left Label Row -->
          <div
            v-if="sprintsWithDates.length > 0"
            class="gantt-row"
            style="height: 24px; background: rgba(124, 58, 237, 0.06); padding: 0 12px; font-size: 11px; font-weight: 700; color: #7c3aed;"
          >
            <div>🏃 スプリント期間</div>
          </div>

          <template v-for="iss in issues" :key="`g-left-${iss.id}`">
            <!-- Parent Issue Row -->
            <div
              class="gantt-row"
              style="padding: 0 12px; background: var(--bg-card-hover); cursor: pointer;"
              @click="openIssueDetail(iss)"
            >
              <div style="flex: 1; font-size: 13px; font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--primary);">
                ▼ {{ iss.title }}
              </div>
              <div style="width: 96px; text-align: right; font-size: 12px; font-weight: 600;">
                {{ formatHours(iss.rollup.total_planned_hours) }} / {{ formatHours(iss.rollup.total_actual_hours) }}
              </div>
              <div style="width: 54px; text-align: right; font-size: 13px; font-weight: 700; color: var(--primary);">
                {{ iss.rollup.weighted_progress.toFixed(0) }}%
              </div>
            </div>

            <!-- Child Task Rows -->
            <div
              v-for="t in iss.tasks || []"
              :key="`g-left-t-${t.id}`"
              class="gantt-row"
              style="padding: 0 12px 0 24px; cursor: pointer;"
              @click="openTaskDetail(t, iss)"
            >
              <div style="flex: 1; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; display: flex; align-items: center;">
                <span>├ {{ t.title }}</span>
                <span v-if="isTaskOverdue(t)" class="badge badge-amber" style="font-size: 11px; margin-left: 6px; color: #dc2626; border-color: #dc2626; font-weight: 700;">⚠️ 遅延</span>
              </div>
              <div style="width: 96px; text-align: right; font-size: 12px; color: var(--text-muted);">
                {{ formatHours(t.planned_hours) }} / {{ formatHours(t.actual_hours) }}
              </div>
              <div style="width: 54px; text-align: right; font-size: 13px; font-weight: 600;">
                {{ t.progress }}%
              </div>
            </div>
          </template>
        </div>

        <!-- Right Timeline Pane -->
        <div ref="timelinePaneRef" class="gantt-right">
          <div :style="{ width: `${timelineDays.length * ganttDayWidth}px`, minWidth: '100%', position: 'relative' }">
            <!-- Today Indicator Line -->
            <div
              v-if="getBarPos(todayYMD, todayYMD).visible"
              :style="{
                position: 'absolute',
                top: '0',
                bottom: '0',
                left: `${getBarPos(todayYMD, todayYMD).leftPx + (ganttDayWidth / 2)}px`,
                width: '2px',
                backgroundColor: '#ef4444',
                zIndex: 10,
                pointerEvents: 'none'
              }"
              title="今日 (Today)"
            >
              <div
                style="
                  position: absolute;
                  top: 2px;
                  left: 50%;
                  transform: translateX(-50%);
                  background: #ef4444;
                  color: #fff;
                  font-size: 10px;
                  font-weight: 700;
                  padding: 1px 4px;
                  border-radius: 4px;
                  white-space: nowrap;
                "
              >
                今日
              </div>
            </div>

            <!-- SVG Dependency Arrows Overlay Layer -->
            <svg
              v-if="showDependencyLines && ganttDependencyLines.length > 0"
              style="
                position: absolute;
                top: 0;
                left: 0;
                width: 100%;
                height: 100%;
                pointer-events: none;
                z-index: 5;
              "
            >
              <defs>
                <marker
                  id="gantt-dep-arrow"
                  viewBox="0 0 10 10"
                  refX="6"
                  refY="5"
                  markerWidth="6"
                  markerHeight="6"
                  orient="auto-start-reverse"
                >
                  <path d="M 0 1.5 L 8 5 L 0 8.5 z" fill="#2563eb" />
                </marker>
              </defs>
              <g v-for="depLine in ganttDependencyLines" :key="depLine.id">
                <path
                  :d="depLine.path"
                  fill="none"
                  stroke="#2563eb"
                  stroke-width="1.8"
                  marker-end="url(#gantt-dep-arrow)"
                >
                  <title>{{ depLine.tooltip }}</title>
                </path>
              </g>
            </svg>

            <!-- 2-Tier Date Header -->
            <div class="gantt-row" style="height: 44px; background: var(--bg-elevated); display: flex; flex-direction: column; padding: 0;">
              <!-- Upper Tier: Month / Year -->
              <div style="display: flex; height: 22px; border-bottom: 1px solid var(--border-color);">
                <div
                  v-for="(ug, uIdx) in ganttHeaderGroups.upperGroups"
                  :key="ug.key || uIdx"
                  :style="{
                    width: `${ug.widthPx}px`,
                    height: '100%',
                    borderRight: '1px solid var(--border-color)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '11px',
                    fontWeight: '700',
                    color: 'var(--text-main)',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap'
                  }"
                >
                  {{ ug.label }}
                </div>
              </div>
              <!-- Lower Tier: Day / Week / Month Buckets -->
              <div style="display: flex; height: 22px;">
                <div
                  v-for="lg in ganttHeaderGroups.lowerGroups"
                  :key="lg.key"
                  :style="{
                    width: `${lg.widthPx}px`,
                    height: '100%',
                    borderRight: '1px solid var(--border-color)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    fontSize: '10px',
                    background: lg.isWeekend ? 'rgba(100, 116, 139, 0.08)' : 'transparent',
                    color: (todayYMD >= lg.startYMD && todayYMD <= lg.endYMD) ? 'var(--primary)' : lg.isWeekend ? 'var(--text-muted)' : 'var(--text-main)',
                    fontWeight: (todayYMD >= lg.startYMD && todayYMD <= lg.endYMD) ? '700' : '400',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap'
                  }"
                >
                  {{ lg.label }}
                </div>
              </div>
            </div>

            <!-- Sprint Period Overlay Band Row -->
            <div
              v-if="sprintsWithDates.length > 0"
              class="gantt-row"
              style="height: 24px; position: relative; background: rgba(124, 58, 237, 0.04); padding: 0;"
            >
              <template v-for="sp in sprintsWithDates" :key="`gantt-sp-${sp.id}`">
                <div
                  v-if="getBarPos(sp.start_date, sp.end_date).visible"
                  :style="{
                    position: 'absolute',
                    top: '2px',
                    height: '19px',
                    left: `${getBarPos(sp.start_date, sp.end_date).leftPx}px`,
                    width: `${getBarPos(sp.start_date, sp.end_date).widthPx}px`,
                    borderRadius: '4px',
                    background: sp.status === 'active' ? 'rgba(16, 185, 129, 0.22)' : 'rgba(124, 58, 237, 0.18)',
                    border: sp.status === 'active' ? '1px solid #10b981' : '1px solid #8b5cf6',
                    color: sp.status === 'active' ? '#047857' : '#6d28d9',
                    fontSize: '10px',
                    fontWeight: '700',
                    display: 'flex',
                    alignItems: 'center',
                    padding: '0 6px',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    zIndex: 3
                  }"
                  :title="`🏃 ${sp.name} (${sp.start_date} 〜 ${sp.end_date})`"
                >
                  🏃 {{ sp.name }} ({{ sp.start_date?.slice(5) }}〜{{ sp.end_date?.slice(5) }})
                </div>
              </template>
            </div>

            <template v-for="iss in issues" :key="`g-right-${iss.id}`">
              <!-- Parent Issue Rollup Bar Row -->
              <div class="gantt-row" style="position: relative; background: var(--bg-card-hover);">
                <div
                  v-if="getBarPos(iss.rollup.planned_start_date, iss.rollup.planned_end_date).visible"
                  class="gantt-bar-planned"
                  :style="{
                    left: `${getBarPos(iss.rollup.planned_start_date, iss.rollup.planned_end_date).leftPx}px`,
                    width: `${getBarPos(iss.rollup.planned_start_date, iss.rollup.planned_end_date).widthPx}px`,
                    cursor: 'default'
                  }"
                >
                  <div class="gantt-bar-planned-progress" :style="{ width: `${iss.rollup.weighted_progress}%` }"></div>
                </div>
                <div
                  v-if="getBarPos(iss.rollup.actual_start_date, iss.rollup.actual_end_date).visible"
                  class="gantt-bar-actual"
                  :style="{
                    left: `${getBarPos(iss.rollup.actual_start_date, iss.rollup.actual_end_date).leftPx}px`,
                    width: `${getBarPos(iss.rollup.actual_start_date, iss.rollup.actual_end_date).widthPx}px`,
                    cursor: 'default'
                  }"
                ></div>
              </div>

              <!-- Child Task Timeline Row -->
              <div
                v-for="t in iss.tasks || []"
                :key="`g-right-t-${t.id}`"
                class="gantt-row"
                style="position: relative;"
              >
                <!-- Background Date Cells (supports drag-to-create when dates unset) -->
                <div
                  v-for="lg in ganttHeaderGroups.lowerGroups"
                  :key="`cell-${t.id}-${lg.key}`"
                  :style="{
                    width: `${lg.widthPx}px`,
                    height: '100%',
                    borderRight: '1px solid var(--border-color)',
                    background: lg.isWeekend ? 'rgba(100, 116, 139, 0.04)' : 'transparent',
                    cursor: !t.planned_start_date ? 'crosshair' : 'default'
                  }"
                  :title="!t.planned_start_date ? `${lg.startYMD} を起点にドラッグして予定期間を作成` : lg.label"
                  @mousedown="onCellMouseDownCreate(t, lg.startYMD)"
                  @mouseup="onCellMouseUpCreate(t, lg.endYMD)"
                ></div>

                <!-- Planned Bar (Upper, Draggable & Resizable) -->
                <div
                  v-if="getBarPos(t.planned_start_date, t.planned_end_date).visible"
                  class="gantt-bar-planned"
                  :class="{ 'task-overdue': isTaskOverdue(t) }"
                  :style="{
                    left: `${getBarPos(t.planned_start_date, t.planned_end_date).leftPx}px`,
                    width: `${getBarPos(t.planned_start_date, t.planned_end_date).widthPx}px`
                  }"
                  @mousedown.stop="startGanttBarDrag($event, t, 'planned', 'move')"
                  @click.stop="onGanttBarClick(t, iss)"
                >
                  <div v-if="canEditProjectItems" class="gantt-resize-handle" @mousedown.stop="startGanttBarDrag($event, t, 'planned', 'resize-start')"></div>
                  <div class="gantt-bar-planned-progress" :style="{ width: `${t.progress}%` }"></div>
                  <span style="position: relative; z-index: 2; font-size: 11px; font-weight: 700; padding: 0 4px; white-space: nowrap;">
                    予定 {{ t.planned_start_date?.slice(5) }}〜{{ t.planned_end_date?.slice(5) }}
                  </span>
                  <div v-if="canEditProjectItems" style="margin-left: auto;" class="gantt-resize-handle" @mousedown.stop="startGanttBarDrag($event, t, 'planned', 'resize-end')"></div>
                </div>

                <!-- Actual Bar (Lower, Draggable & Resizable) -->
                <div
                  v-if="getBarPos(t.actual_start_date, t.actual_end_date).visible"
                  class="gantt-bar-actual"
                  :style="{
                    left: `${getBarPos(t.actual_start_date, t.actual_end_date).leftPx}px`,
                    width: `${getBarPos(t.actual_start_date, t.actual_end_date).widthPx}px`
                  }"
                  @mousedown.stop="startGanttBarDrag($event, t, 'actual', 'move')"
                  @click.stop="onGanttBarClick(t, iss)"
                >
                  <div v-if="canEditProjectItems" class="gantt-resize-handle" @mousedown.stop="startGanttBarDrag($event, t, 'actual', 'resize-start')"></div>
                  <span style="position: relative; z-index: 2; font-size: 11px; color: #fff; padding: 0 4px; white-space: nowrap;">
                    実績 {{ t.actual_start_date?.slice(5) }}〜{{ t.actual_end_date?.slice(5) }}
                  </span>
                  <div v-if="canEditProjectItems" style="margin-left: auto;" class="gantt-resize-handle" @mousedown.stop="startGanttBarDrag($event, t, 'actual', 'resize-end')"></div>
                </div>
                <button
                  v-else-if="canEditProjectItems && t.planned_start_date"
                  class="btn btn-ghost btn-sm"
                  style="position: absolute; top: 31px; left: 8px; padding: 1px 6px; font-size: 11px; color: var(--success); border: 1px dashed var(--success);"
                  @click.stop="initActualFromPlanned(t)"
                >
                  ＋ 実績バー作成
                </button>
              </div>
            </template>
          </div>
        </div>
      </div>
    </section>

    <!-- VIEW 3: SPREADSHEET LIST / TABLE VIEW -->
    <section v-if="activeTab === 'list'">
      <TaskTableView
        :project-id="projectId"
        :issues="issues"
        :statuses="statuses"
        :users="users"
        :can-edit="canEditProjectItems"
        :types="projectTypes"
        @open-issue="openIssueDetail"
        @open-task="openTaskDetail"
        @refresh="refreshAllWorkspace"
      />
    </section>

    <!-- VIEW 4: SPRINT MANAGEMENT VIEW -->
    <section v-if="activeTab === 'sprint'">
      <SprintBoardView
        :project-id="projectId"
        :sprints="sprints"
        :all-tasks="allTasks"
        :statuses="statuses"
        :can-edit="canEditProjectItems"
        @open-task="openTaskDetailStandalone"
        @refresh="refreshAllWorkspace"
      />
    </section>

    <!-- VIEW 5: RESOURCE WORKLOAD VIEW -->
    <section v-if="activeTab === 'workload'">
      <ResourceWorkloadView
        :project-id="projectId"
        :hours-per-day="project?.hours_per_day || 8.0"
        :all-tasks="allTasks"
        :users="users"
        :members="members"
        :issues="issues"
        @open-task="openTaskDetail"
      />
    </section>

    <!-- VIEW 6: WIKI VIEW -->
    <section v-if="activeTab === 'wiki'">
      <WikiView
        :project-id="projectId"
        :tasks="allTasks"
        :statuses="statuses"
        :can-edit="canEditProjectItems"
        @open-task="openTaskDetailStandalone"
      />
    </section>

    <!-- VIEW 7: ACTIVITY LOG VIEW -->
    <section v-if="activeTab === 'activity'">
      <ActivityLogView
        :project-id="projectId"
        :users="users"
        :statuses="statuses"
        @open-task="openTargetFromQuery"
        @open-issue="openTargetFromQuery"
      />
    </section>

    <!-- VIEW 8: PROJECT SETTINGS & TEMPLATE BUILDER (Admin Role) -->
    <section v-if="activeTab === 'settings' && canManageProjectSettings && project" style="display: flex; flex-direction: column; gap: 20px;">
      <!-- 1. Project General Settings -->
      <div class="card">
        <h3 style="font-size: 15px; font-weight: 700; margin-bottom: 12px;">プロジェクト基本設定</h3>
        <form style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 12px; align-items: end;" @submit.prevent="saveProjectSettings">
          <div>
            <label class="field-label">プロジェクト名 *</label>
            <input v-model="projForm.name" type="text" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">説明</label>
            <input v-model="projForm.description" type="text" style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">1日あたりの基準時間 (h)</label>
            <input v-model.number="projForm.hours_per_day" type="number" min="1" max="24" step="0.01" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">デフォルト課題テンプレート</label>
            <select v-model="projForm.default_issue_template_id" style="width: 100%;">
              <option v-for="t in templates.filter(x => x.target_type === 'issue')" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </div>
          <div>
            <label class="field-label">デフォルトタスクテンプレート</label>
            <select v-model="projForm.default_task_template_id" style="width: 100%;">
              <option v-for="t in templates.filter(x => x.target_type === 'task')" :key="t.id" :value="t.id">{{ t.name }}</option>
            </select>
          </div>
          <div>
            <button type="submit" class="btn btn-primary">基本設定を保存</button>
          </div>
        </form>
      </div>

      <!-- 2. Project Members & Roles Permissions Matrix -->
      <div class="card" style="display: flex; flex-direction: column; gap: 14px;">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div>
            <h3 style="font-size: 15px; font-weight: 700;">参加メンバー・グループ権限マトリクス</h3>
            <p style="font-size: 12px; color: var(--text-muted); margin-top: 2px; margin-bottom: 0;">
              プロジェクトへのアクセス権限をユーザー・グループ単位で設定できます。
            </p>
          </div>
          <form style="display: flex; gap: 8px; flex-wrap: wrap; align-items: center;" @submit.prevent="addProjectMember">
            <select v-model="memberForm.subject_combined" required style="min-width: 220px; font-size: 13px;">
              <option value="">-- ユーザーまたはグループを選択 --</option>
              <optgroup label="ユーザー">
                <option v-for="u in users" :key="u.id" :value="`user:${u.id}`">👤 {{ u.display_name }} ({{ u.email }})</option>
              </optgroup>
              <optgroup label="グループ">
                <option v-for="g in allGroups" :key="g.id" :value="`group:${g.id}`">👥 {{ g.name }}</option>
              </optgroup>
            </select>
            <select v-model="memberForm.role" style="font-size: 13px;">
              <option value="admin">Admin (管理者)</option>
              <option value="editor">Editor (編集者)</option>
              <option value="viewer">Viewer (閲覧者)</option>
            </select>
            <button type="submit" class="btn btn-primary btn-sm">＋ メンバー・グループを追加</button>
          </form>
        </div>

        <div style="overflow-x: auto;">
          <table style="width: 100%; border-collapse: collapse; font-size: 13px;">
            <thead>
              <tr style="border-bottom: 1px solid var(--border-color); color: var(--text-muted); font-size: 12px;">
                <th style="padding: 10px 12px; text-align: left; font-weight: 600;">対象</th>
                <th style="padding: 10px 12px; text-align: center; width: 110px; font-weight: 600;">Admin</th>
                <th style="padding: 10px 12px; text-align: center; width: 110px; font-weight: 600;">Editor</th>
                <th style="padding: 10px 12px; text-align: center; width: 110px; font-weight: 600;">Viewer</th>
                <th style="padding: 10px 12px; text-align: right; width: 90px; font-weight: 600;">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="m in members"
                :key="m.id"
                style="border-bottom: 1px solid var(--border-color);"
              >
                <td style="padding: 10px 12px;">
                  <div style="display: flex; align-items: center; gap: 10px;">
                    <div
                      style="width: 30px; height: 30px; border-radius: 9999px; display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 700; flex-shrink: 0;"
                      :style="m.subject_type === 'user' ? 'background: rgba(59, 130, 246, 0.15); color: #2563eb;' : 'background: rgba(139, 92, 246, 0.15); color: #7c3aed;'"
                    >
                      {{ m.subject_type === 'user' ? (getUserName(m.subject_id)?.[0] || 'U') : (getGroupName(m.subject_id)?.[0] || 'G') }}
                    </div>
                    <div>
                      <div style="font-weight: 600; display: flex; align-items: center; gap: 6px;">
                        <span>{{ getMemberSubjectName(m) }}</span>
                        <span :class="['badge', m.subject_type === 'user' ? 'badge-blue' : 'badge-purple']" style="font-size: 10px; padding: 1px 6px;">
                          {{ m.subject_type === 'user' ? 'ユーザー' : 'グループ' }}
                        </span>
                        <span v-if="isLastAdmin(m)" class="badge badge-amber" style="font-size: 10px; padding: 1px 6px;">
                          唯一の管理者
                        </span>
                      </div>
                      <div v-if="m.subject_type === 'user'" style="font-size: 11px; color: var(--text-muted); margin-top: 1px;">
                        {{ users.find(u => u.id === m.subject_id)?.email }}
                      </div>
                    </div>
                  </div>
                </td>
                <td style="padding: 10px 12px; text-align: center;">
                  <label style="cursor: pointer; display: inline-flex; align-items: center; justify-content: center; width: 100%; height: 100%;">
                    <input
                      type="radio"
                      :name="`role-${m.id}`"
                      value="admin"
                      :checked="m.role === 'admin'"
                      style="cursor: pointer; width: 16px; height: 16px;"
                      @change="updateMemberRole(m, 'admin')"
                    />
                  </label>
                </td>
                <td style="padding: 10px 12px; text-align: center;">
                  <label
                    :style="isLastAdmin(m) ? 'cursor: not-allowed; opacity: 0.4;' : 'cursor: pointer;'"
                    :title="isLastAdmin(m) ? '最後の管理者は降格できません' : ''"
                    style="display: inline-flex; align-items: center; justify-content: center; width: 100%; height: 100%;"
                  >
                    <input
                      type="radio"
                      :name="`role-${m.id}`"
                      value="editor"
                      :checked="m.role === 'editor'"
                      :disabled="isLastAdmin(m)"
                      :style="isLastAdmin(m) ? 'cursor: not-allowed;' : 'cursor: pointer;'"
                      style="width: 16px; height: 16px;"
                      @change="updateMemberRole(m, 'editor')"
                    />
                  </label>
                </td>
                <td style="padding: 10px 12px; text-align: center;">
                  <label
                    :style="isLastAdmin(m) ? 'cursor: not-allowed; opacity: 0.4;' : 'cursor: pointer;'"
                    :title="isLastAdmin(m) ? '最後の管理者は降格できません' : ''"
                    style="display: inline-flex; align-items: center; justify-content: center; width: 100%; height: 100%;"
                  >
                    <input
                      type="radio"
                      :name="`role-${m.id}`"
                      value="viewer"
                      :checked="m.role === 'viewer'"
                      :disabled="isLastAdmin(m)"
                      :style="isLastAdmin(m) ? 'cursor: not-allowed;' : 'cursor: pointer;'"
                      style="width: 16px; height: 16px;"
                      @change="updateMemberRole(m, 'viewer')"
                    />
                  </label>
                </td>
                <td style="padding: 10px 12px; text-align: right;">
                  <button
                    class="btn btn-danger btn-sm"
                    :disabled="isLastAdmin(m)"
                    :title="isLastAdmin(m) ? '最後の管理者は削除できません' : '解除'"
                    @click="removeProjectMember(m)"
                  >
                    解除
                  </button>
                </td>
              </tr>
              <tr v-if="members.length === 0">
                <td colspan="5" style="text-align: center; padding: 20px; color: var(--text-muted);">
                  参加メンバー・グループが登録されていません
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <p style="font-size: 12px; color: var(--text-muted); margin: 0;">
          ※ 各メンバーの権限列（Admin / Editor / Viewer）をクリックして即時変更できます。最後の管理者は降格・削除できません。
        </p>
      </div>

      <!-- 3. Project Statuses & Labels -->
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr)); gap: 16px;">
        <!-- Project Statuses (1..10, migration on delete) -->
        <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
          <h3 style="font-size: 15px; font-weight: 700;">ステータス設定</h3>
          <form style="display: flex; gap: 8px; flex-wrap: wrap;" @submit.prevent="createStatus">
            <input v-model.number="statusForm.display_order" type="number" min="1" max="10" required style="width: 70px;" placeholder="順序" />
            <input v-model="statusForm.name" type="text" required style="flex: 1;" placeholder="ステータス名" />
            <input v-model="statusForm.color" type="color" style="width: 44px; padding: 2px;" />
            <button type="submit" class="btn btn-primary btn-sm" :disabled="statuses.length >= 10">＋ 追加</button>
          </form>

          <div style="display: flex; flex-direction: column; gap: 6px;">
            <div
              v-for="st in statuses"
              :key="st.id"
              style="padding: 8px 10px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; font-size: 12px;"
            >
              <span style="display: flex; align-items: center; gap: 8px; font-weight: 600;">
                <span :style="{ width: '10px', height: '10px', borderRadius: '999px', backgroundColor: st.color }"></span>
                {{ st.display_order }}. {{ st.name }}
              </span>
              <div v-if="statuses.length > 1" style="display: flex; align-items: center; gap: 6px;">
                <select v-model="migrateTargetMap[st.id]" style="font-size: 11px; padding: 2px 6px;">
                  <option value="">移行先ステータス...</option>
                  <option v-for="other in statuses.filter(x => x.id !== st.id)" :key="other.id" :value="other.id">
                    → {{ other.display_order }}. {{ other.name }}
                  </option>
                </select>
                <button class="btn btn-danger btn-sm" @click="deleteStatus(st.id)">削除</button>
              </div>
            </div>
          </div>
        </div>

        <!-- Project固有ラベル -->
        <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
          <h3 style="font-size: 15px; font-weight: 700;">プロジェクト固有ラベル</h3>
          <form style="display: flex; gap: 8px;" @submit.prevent="createProjectLabel">
            <input v-model="labelForm.name" type="text" required style="flex: 1;" placeholder="ラベル名" />
            <input v-model="labelForm.color" type="color" style="width: 44px; padding: 2px;" />
            <button type="submit" class="btn btn-primary btn-sm">＋ 追加</button>
          </form>
          <div style="display: flex; flex-wrap: wrap; gap: 8px;">
            <div
              v-for="lb in projectLabels"
              :key="lb.id"
              style="padding: 4px 10px; border-radius: 6px; border: 1px solid var(--border-color); background: var(--bg-input); display: flex; align-items: center; gap: 8px; font-size: 12px;"
            >
              <span :style="{ color: lb.color, fontWeight: 700 }">● {{ lb.name }}</span>
              <button class="btn btn-danger btn-sm" style="padding: 1px 5px; font-size: 10px;" @click="deleteProjectLabel(lb.id)">✕</button>
            </div>
          </div>
        </div>
      </div>

      <!-- 4. Custom Field Definitions -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
        <h3 style="font-size: 15px; font-weight: 700;">カスタム属性定義</h3>
        <form style="display: flex; flex-wrap: wrap; gap: 8px;" @submit.prevent="createCustomField">
          <select v-model="cfForm.target_type">
            <option value="issue">課題用</option>
            <option value="task">タスク用</option>
          </select>
          <input v-model="cfForm.name" type="text" required style="flex: 1;" placeholder="項目名（例: 外注見積金額）" />
          <select v-model="cfForm.field_type">
            <option value="text_single">1行テキスト</option>
            <option value="text_multi">複数行テキスト</option>
            <option value="markdown">Markdown</option>
            <option value="number">数値</option>
            <option value="date">日付</option>
            <option value="select">プルダウン</option>
            <option value="radio">ラジオボタン</option>
            <option value="checkbox">チェックボックス</option>
            <option value="user_group">ユーザー/グループ</option>
          </select>
          <input
            v-if="['select', 'radio', 'checkbox'].includes(cfForm.field_type)"
            v-model="cfForm.options_csv"
            type="text"
            style="width: 100%;"
            placeholder="選択肢（カンマ区切り: A, B, C）"
          />
          <button type="submit" class="btn btn-primary btn-sm">＋ 追加</button>
        </form>

        <div style="display: flex; flex-direction: column; gap: 6px;">
          <div
            v-for="cf in customFields"
            :key="cf.id"
            style="padding: 6px 10px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; font-size: 12px;"
          >
            <span>
              <span :class="['badge', cf.target_type === 'issue' ? 'badge-green' : 'badge-blue']" style="margin-right: 6px;">
                {{ cf.target_type === 'issue' ? '課題' : 'タスク' }}
              </span>
              <strong>{{ cf.name }}</strong> ({{ cf.field_type }})
            </span>
            <button class="btn btn-danger btn-sm" @click="deleteCustomField(cf.id)">削除</button>
          </div>
        </div>
      </div>

      <!-- 4. Project Types Management -->
      <div class="card" style="display: flex; flex-direction: column; gap: 14px;">
        <div style="border-bottom: 1px solid var(--border-color); padding-bottom: 8px; display: flex; align-items: center; justify-content: space-between;">
          <div>
            <h3 style="font-size: 15px; font-weight: 700;">種別設定 (Types)</h3>
            <p style="font-size: 12px; color: var(--text-muted); margin-top: 2px;">
              課題およびタスクに設定できる種別を管理します。システム共通種別に加え、プロジェクト固有の種別を追加できます。
            </p>
          </div>
          <span class="badge badge-blue">合計 {{ projectTypes.length }}件</span>
        </div>

        <!-- Add Project Specific Type Form -->
        <form style="display: flex; flex-direction: column; gap: 10px; max-width: 680px;" @submit.prevent="createProjectType">
          <div style="display: grid; grid-template-columns: 80px 1fr 140px; gap: 8px; align-items: end;">
            <div>
              <label class="field-label">表示順 *</label>
              <input v-model.number="projectTypeForm.display_order" type="number" min="1" required style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">種別名 *</label>
              <input v-model="projectTypeForm.name" type="text" required placeholder="例: リファクタリング, QA確認" style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">カラー</label>
              <div style="display: flex; align-items: center; gap: 8px;">
                <input v-model="projectTypeForm.color" type="color" style="width: 44px; height: 34px; padding: 2px; cursor: pointer;" />
                <span class="badge" :style="{ backgroundColor: projectTypeForm.color, color: '#fff', fontSize: '11px', fontWeight: 'bold' }">
                  プレビュー
                </span>
              </div>
            </div>
          </div>

          <!-- Color Presets -->
          <div style="display: flex; flex-direction: column; gap: 4px;">
            <label class="field-label">プリセットカラー</label>
            <div style="display: flex; flex-wrap: wrap; gap: 6px;">
              <button
                v-for="c in PRESET_COLORS"
                :key="c"
                type="button"
                style="width: 22px; height: 22px; border-radius: 4px; cursor: pointer; border: 2px solid transparent;"
                :style="{
                  backgroundColor: c,
                  borderColor: projectTypeForm.color.toLowerCase() === c.toLowerCase() ? 'var(--text-main)' : 'transparent'
                }"
                :title="c"
                @click="projectTypeForm.color = c"
              />
            </div>
          </div>

          <div>
            <button type="submit" class="btn btn-primary btn-sm">＋ プロジェクト固有種別を追加</button>
          </div>
        </form>

        <!-- Types List -->
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div style="font-size: 13px; font-weight: 700; color: var(--text-muted); margin-top: 6px;">利用可能な種別一覧</div>
          <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 8px;">
            <div
              v-for="tp in sortedProjectTypes"
              :key="tp.id"
              style="padding: 8px 12px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; font-size: 12px;"
            >
              <div style="display: flex; align-items: center; gap: 8px;">
                <span
                  class="badge"
                  :style="{ backgroundColor: tp.color, color: '#fff', fontSize: '11px', fontWeight: 'bold' }"
                >
                  {{ tp.name }}
                </span>
                <span v-if="!tp.project_id" class="badge badge-slate" style="font-size: 10px;">共通</span>
                <span v-else class="badge badge-blue" style="font-size: 10px;">固有</span>
                <span style="font-size: 11px; color: var(--text-muted);">順序: {{ tp.display_order }}</span>
              </div>

              <!-- Actions for Project Specific Types -->
              <div v-if="tp.project_id" style="display: flex; align-items: center; gap: 6px;">
                <button type="button" class="btn btn-secondary btn-sm" style="padding: 2px 6px; font-size: 11px;" @click="openEditProjectTypeModal(tp)">編集</button>
                <button type="button" class="btn btn-danger btn-sm" style="padding: 2px 6px; font-size: 11px;" @click="deleteProjectType(tp)">削除</button>
              </div>
              <div v-else style="font-size: 11px; color: var(--text-muted);">
                参照のみ
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- 5. Template Layout & Field Permission Builder -->
      <TemplateBuilder
        title="プロジェクトテンプレート ＆ レイアウトビルダー"
        :templates="templates"
        :custom-fields="customFields"
        :users="users"
        :groups="allGroups"
        @save="saveProjectTemplate"
        @delete="deleteProjectTemplate"
      />

      <!-- 5. Project Outbound Webhooks -->
      <div class="card" style="display: flex; flex-direction: column; gap: 14px;">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div>
            <h3 style="font-size: 15px; font-weight: 700;">プロジェクト Outbound Webhook 連携</h3>
            <p style="font-size: 12px; color: var(--text-muted);">
              タスクや課題の作成・更新・ステータス変更イベントを外部URL（Slack, Discord, Teams, 自社サーバー等）へリアルタイム送信します。
            </p>
          </div>
          <button class="btn btn-primary btn-sm" @click="showNewWebhookModal = true">＋ Webhook を追加</button>
        </div>

        <div style="display: flex; flex-direction: column; gap: 10px;">
          <div
            v-for="wh in projectWebhooks"
            :key="wh.id"
            style="padding: 12px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input); display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px;"
          >
            <div style="display: flex; flex-direction: column; gap: 4px;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <span :class="['badge', wh.is_active ? 'badge-green' : 'badge-slate']">
                  {{ wh.is_active ? '有効' : '停止中' }}
                </span>
                <strong style="font-size: 14px;">{{ wh.name }}</strong>
                <code style="font-size: 12px;">{{ wh.url }}</code>
              </div>
              <div style="display: flex; flex-wrap: wrap; gap: 4px; align-items: center;">
                <span style="font-size: 12px; color: var(--text-muted);">購読イベント:</span>
                <span v-for="ev in wh.events" :key="ev" class="badge badge-blue" style="font-size: 11px;">
                  {{ ev }}
                </span>
              </div>
            </div>

            <div style="display: flex; align-items: center; gap: 8px;">
              <button class="btn btn-secondary btn-sm" style="font-size: 12px;" @click="openEditWebhook(wh)">編集</button>
              <button class="btn btn-secondary btn-sm" style="font-size: 12px;" @click="testProjectWebhook(wh.id)">テスト送信 (Ping)</button>
              <button class="btn btn-secondary btn-sm" style="font-size: 12px;" @click="openWebhookDeliveries(wh)">送信ログ</button>
              <button class="btn btn-danger btn-sm" style="font-size: 12px;" @click="deleteProjectWebhook(wh.id)">削除</button>
            </div>
          </div>
          <div v-if="projectWebhooks.length === 0" style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 13px;">
            登録されたWebhookはありません
          </div>
        </div>
      </div>

      <!-- 6. Inbound Webhook 連携（外部タスク自動起票） -->
      <div class="card" style="display: flex; flex-direction: column; gap: 14px;">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div>
            <h3 style="font-size: 15px; font-weight: 700;">Inbound Webhook 連携（外部タスク自動起票）</h3>
            <p style="font-size: 12px; color: var(--text-muted);">
              CI/CD（GitHub Actions、CircleCI等）や外部監視ツールからHTTP POSTを受信し、本プロジェクト内にタスクを自動起票します。
            </p>
          </div>
          <button class="btn btn-primary btn-sm" @click="openNewInboundWebhookModal">＋ Inbound Webhook を作成</button>
        </div>

        <div style="display: flex; flex-direction: column; gap: 10px;">
          <div
            v-for="wh in inboundWebhooks"
            :key="wh.id"
            style="padding: 12px; border-radius: 8px; border: 1px solid var(--border-color); background: var(--bg-input); display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px;"
          >
            <div style="display: flex; flex-direction: column; gap: 4px;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <span :class="['badge', wh.is_active ? 'badge-green' : 'badge-slate']">
                  {{ wh.is_active ? '有効' : '無効' }}
                </span>
                <strong style="font-size: 14px;">{{ wh.name }}</strong>
              </div>
              <div style="display: flex; flex-wrap: wrap; gap: 12px; font-size: 12px; color: var(--text-muted);">
                <span v-if="wh.default_issue_id">
                  デフォルト課題: {{ issues.find(i => i.id === wh.default_issue_id)?.title || wh.default_issue_id }}
                </span>
                <span v-if="wh.default_status_id">
                  デフォルトステータス: {{ statuses.find(s => s.id === wh.default_status_id)?.name || wh.default_status_id }}
                </span>
              </div>
            </div>

            <div style="display: flex; align-items: center; gap: 8px;">
              <button class="btn btn-danger btn-sm" style="font-size: 12px;" @click="deleteInboundWebhook(wh.id)">削除</button>
            </div>
          </div>

          <div v-if="inboundWebhooks.length === 0" style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 13px;">
            登録された Inbound Webhook はありません
          </div>
        </div>
      </div>
    </section>

    <!-- New Issue Modal (with create_initial_task toggle) -->
    <div v-if="showNewIssueModal" class="modal-backdrop" v-backdrop-dismiss="() => showNewIssueModal = false">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">新規課題（Issue）作成</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitNewIssue">
          <div>
            <label class="field-label">課題タイトル *</label>
            <input v-model="newIssueForm.title" type="text" required style="width: 100%;" placeholder="例: 認証・招待・ドメイン制限基盤の構築" />
          </div>
          <div>
            <label class="field-label">説明 (Markdown)</label>
            <textarea v-model="newIssueForm.description" rows="3" style="width: 100%;"></textarea>
          </div>
          <label style="display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; cursor: pointer; padding: 8px 10px; border-radius: 8px; background: rgba(37,99,235,0.12); border: 1px solid rgba(59,130,246,0.4);">
            <input v-model="newIssueForm.create_initial_task" type="checkbox" />
            <span>同名の初期タスクを同時に1つ作成する（最も若いステータス列へ自動配置）</span>
          </label>
          <div style="display: flex; justify-content: flex-end; gap: 8px;">
            <button type="button" class="btn btn-secondary" :disabled="isSubmittingIssue" @click="showNewIssueModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary" :disabled="isSubmittingIssue">
              {{ isSubmittingIssue ? '⏳ 作成中...' : '作成する' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- New Task Modal -->
    <div v-if="showNewTaskModal" class="modal-backdrop" v-backdrop-dismiss="() => showNewTaskModal = false">
      <div class="modal-panel" style="max-width: 560px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">
          新規子タスク作成（親課題: {{ newTaskParentIssue?.title }}）
        </h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitNewTask">
          <div>
            <label class="field-label">タスクタイトル *</label>
            <input v-model="newTaskForm.title" type="text" required style="width: 100%;" />
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">予定開始日</label>
              <input v-model="newTaskForm.planned_start_date" type="date" style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">予定終了日</label>
              <input v-model="newTaskForm.planned_end_date" type="date" style="width: 100%;" />
            </div>
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">予定工数 (h)</label>
              <input v-model="newTaskForm.planned_hours" type="number" min="0" step="0.01" style="width: 100%;" placeholder="0.00" />
            </div>
            <div>
              <label class="field-label">実績工数 (h)</label>
              <input v-model="newTaskForm.actual_hours" type="number" min="0" step="0.01" style="width: 100%;" placeholder="0.00" />
            </div>
            <div>
              <label class="field-label">初期進捗率 (0〜100%)</label>
              <input v-model.number="newTaskForm.progress" type="number" min="0" max="100" style="width: 100%;" />
            </div>
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">予定コスト (小数第2位まで)</label>
              <input v-model="newTaskForm.planned_cost" type="number" min="0" step="0.01" style="width: 100%;" placeholder="0.00" />
            </div>
            <div>
              <label class="field-label">実績コスト (小数第2位まで)</label>
              <input v-model="newTaskForm.actual_cost" type="number" min="0" step="0.01" style="width: 100%;" placeholder="0.00" />
            </div>
          </div>
          <div>
            <label class="field-label">説明 (Markdown)</label>
            <textarea v-model="newTaskForm.description" rows="2" style="width: 100%;"></textarea>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px;">
            <button type="button" class="btn btn-secondary" :disabled="isSubmittingTask" @click="showNewTaskModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary" :disabled="isSubmittingTask">
              {{ isSubmittingTask ? '⏳ 作成中...' : 'タスクを作成' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- New Webhook Modal -->
    <div v-if="showNewWebhookModal" class="modal-backdrop" v-backdrop-dismiss="() => showNewWebhookModal = false">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">新規 Webhook の登録</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="createProjectWebhook">
          <div>
            <label class="field-label">Webhook 名 *</label>
            <input v-model="newWebhookForm.name" type="text" required style="width: 100%;" placeholder="例: Slack #dev-notify" />
          </div>
          <div>
            <label class="field-label">送信先 URL (HTTP/HTTPS) *</label>
            <input v-model="newWebhookForm.url" type="url" required style="width: 100%;" placeholder="https://hooks.slack.com/services/..." />
          </div>
          <div>
            <label class="field-label">シークレット鍵 (HMAC-SHA256署名用・空欄なら自動生成)</label>
            <input v-model="newWebhookForm.secret" type="text" style="width: 100%; font-family: monospace;" placeholder="whsec_..." />
          </div>
          <div>
            <label class="field-label">購読イベント</label>
            <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; font-size: 12px;">
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="task.created" />
                <span>task.created (タスク作成)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="task.updated" />
                <span>task.updated (タスク更新)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="task.status_changed" />
                <span>task.status_changed (タスク状態変更)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="issue.created" />
                <span>issue.created (課題作成)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="issue.state_changed" />
                <span>issue.state_changed (課題状態変更)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="comment.created" />
                <span>comment.created (コメント投稿)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newWebhookForm.events" type="checkbox" value="attachment.created" />
                <span>attachment.created (添付ファイル追加)</span>
              </label>
            </div>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
            <button type="button" class="btn btn-secondary" @click="showNewWebhookModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary">作成する</button>
          </div>
        </form>
      </div>
    </div>

    <!-- Webhook Deliveries Modal -->
    <div v-if="selectedWebhookForLogs" class="modal-backdrop" v-backdrop-dismiss="() => selectedWebhookForLogs = null">
      <div class="modal-panel" style="max-width: 680px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 15px; font-weight: 700;">配信ログ: {{ selectedWebhookForLogs.name }}</h3>
          <button class="btn btn-secondary btn-sm" @click="selectedWebhookForLogs = null">閉じる</button>
        </div>
        <div style="display: flex; flex-direction: column; gap: 8px; max-height: 420px; overflow-y: auto;">
          <div
            v-for="log in webhookDeliveries"
            :key="log.id"
            style="padding: 10px 12px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); font-size: 12px; display: flex; flex-direction: column; gap: 8px; cursor: pointer;"
            @click="expandedDeliveryId = expandedDeliveryId === log.id ? null : log.id"
          >
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div>
                <span :class="['badge', log.status_code && log.status_code >= 200 && log.status_code < 300 ? 'badge-green' : 'badge-red']" style="margin-right: 6px;">
                  {{ log.status_code || 'ERR' }}
                </span>
                <strong>{{ log.event }}</strong>
                <span style="color: var(--text-muted); margin-left: 6px;">({{ log.duration_ms }}ms)</span>
                <span style="color: var(--primary); margin-left: 8px; font-size: 11px;">
                  {{ expandedDeliveryId === log.id ? '▲ 閉じる' : '▼ 詳細' }}
                </span>
                <div v-if="log.error_message" style="color: var(--danger); font-size: 11px; margin-top: 2px;">
                  {{ log.error_message }}
                </div>
              </div>
              <span style="font-size: 11px; color: var(--text-muted);">{{ log.created_at.slice(0, 19).replace('T', ' ') }}</span>
            </div>

            <!-- Accordion Details (Request & Response) -->
            <div v-if="expandedDeliveryId === log.id" style="border-top: 1px dashed var(--border-color); padding-top: 8px; display: flex; flex-direction: column; gap: 6px;" @click.stop>
              <div>
                <strong style="color: var(--text-muted); font-size: 11px;">リクエスト本文 (JSON):</strong>
                <pre style="margin-top: 4px; padding: 6px; background: var(--bg-main); border: 1px solid var(--border-color); border-radius: 4px; font-family: monospace; font-size: 11px; max-height: 160px; overflow: auto;">{{ JSON.stringify(log.request_body, null, 2) }}</pre>
              </div>
              <div v-if="log.response_body">
                <strong style="color: var(--text-muted); font-size: 11px;">レスポンス本文:</strong>
                <pre style="margin-top: 4px; padding: 6px; background: var(--bg-main); border: 1px solid var(--border-color); border-radius: 4px; font-family: monospace; font-size: 11px; max-height: 120px; overflow: auto;">{{ log.response_body }}</pre>
              </div>
            </div>
          </div>
          <div v-if="webhookDeliveries.length === 0" style="padding: 24px; text-align: center; color: var(--text-muted); font-size: 12px;">
            配信履歴はありません
          </div>
        </div>
      </div>
    </div>

    <!-- Edit Webhook Modal -->
    <div v-if="editingWebhook" class="modal-backdrop" v-backdrop-dismiss="() => editingWebhook = null">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">Webhook の編集</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="updateProjectWebhook">
          <div>
            <label class="field-label">Webhook 名 *</label>
            <input v-model="editingWebhookForm.name" type="text" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">送信先 URL (HTTP/HTTPS) *</label>
            <input v-model="editingWebhookForm.url" type="url" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">シークレット鍵 (空欄のままなら変更なし)</label>
            <input v-model="editingWebhookForm.secret" type="text" style="width: 100%; font-family: monospace;" placeholder="更新時のみ入力" />
          </div>
          <label style="display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 13px;">
            <input v-model="editingWebhookForm.is_active" type="checkbox" />
            <span>有効にする</span>
          </label>
          <div>
            <label class="field-label">購読イベント</label>
            <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; font-size: 12px;">
              <label v-for="ev in ['task.created', 'task.updated', 'task.status_changed', 'issue.created', 'issue.state_changed', 'comment.created', 'attachment.created']" :key="ev" style="display: flex; align-items: center; gap: 4px;">
                <input v-model="editingWebhookForm.events" type="checkbox" :value="ev" />
                <span>{{ ev }}</span>
              </label>
            </div>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
            <button type="button" class="btn btn-secondary" @click="editingWebhook = null">キャンセル</button>
            <button type="submit" class="btn btn-primary">更新を保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- New Inbound Webhook Modal -->
    <div v-if="showNewInboundWebhookModal" class="modal-backdrop" v-backdrop-dismiss="() => showNewInboundWebhookModal = false">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">新規 Inbound Webhook の登録</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="createInboundWebhook">
          <div>
            <label class="field-label">Webhook 名 *</label>
            <input v-model="newInboundWebhookForm.name" type="text" required style="width: 100%;" placeholder="例: GitHub Actions CI Failure" />
          </div>
          <div>
            <label class="field-label">デフォルト起票先課題 (任意)</label>
            <select v-model="newInboundWebhookForm.default_issue_id" style="width: 100%;">
              <option value="">指定なし（リクエスト本文で issue_id を指定）</option>
              <option v-for="iss in issues" :key="iss.id" :value="iss.id">{{ iss.title }}</option>
            </select>
          </div>
          <div>
            <label class="field-label">デフォルトステータス (任意)</label>
            <select v-model="newInboundWebhookForm.default_status_id" style="width: 100%;">
              <option :value="undefined">指定なし（初期ステータスを使用）</option>
              <option v-for="st in statuses" :key="st.id" :value="st.id">{{ st.display_order }}. {{ st.name }}</option>
            </select>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
            <button type="button" class="btn btn-secondary" @click="showNewInboundWebhookModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary">作成してトークン発行</button>
          </div>
        </form>
      </div>
    </div>

    <!-- Inbound Webhook Created Dialog (One-time token & secret display) -->
    <div v-if="createdInboundWebhook" class="modal-backdrop" v-backdrop-dismiss="() => createdInboundWebhook = null">
      <div class="modal-panel" style="max-width: 620px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 16px; font-weight: 700; color: #10b981;">
            ✅ Inbound Webhook を作成しました
          </h3>
          <button class="btn btn-secondary btn-sm" @click="createdInboundWebhook = null">閉じる</button>
        </div>

        <div style="padding: 10px 12px; border-radius: 6px; background: rgba(245, 158, 11, 0.12); border: 1px solid rgba(245, 158, 11, 0.4); margin-bottom: 16px; font-size: 13px; color: #d97706;">
          ⚠️ <strong>注意:</strong> 生成されたトークンおよびシークレットは<strong>この画面を閉じると二度と表示されません</strong>。安全な場所に保存してください。
        </div>

        <div style="display: flex; flex-direction: column; gap: 14px;">
          <div>
            <label class="field-label">受信エンドポイント URL (公開POST受信用)</label>
            <div style="display: flex; gap: 8px;">
              <input :value="inboundWebhookUrl" readonly style="flex: 1; font-family: monospace; font-size: 12px; background: var(--bg-input);" />
              <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(inboundWebhookUrl, 'URLをコピー')">コピー</button>
            </div>
          </div>

          <div>
            <label class="field-label">認証トークン (Path Token)</label>
            <div style="display: flex; gap: 8px;">
              <input :value="createdInboundWebhook.token" readonly style="flex: 1; font-family: monospace; font-size: 12px; background: var(--bg-input);" />
              <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(createdInboundWebhook.token, 'トークンをコピー')">コピー</button>
            </div>
          </div>

          <div v-if="createdInboundWebhook.secret">
            <label class="field-label">署名検証シークレット (Secret)</label>
            <div style="display: flex; gap: 8px;">
              <input :value="createdInboundWebhook.secret" readonly style="flex: 1; font-family: monospace; font-size: 12px; background: var(--bg-input);" />
              <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(createdInboundWebhook.secret, 'シークレットをコピー')">コピー</button>
            </div>
          </div>

          <div>
            <label class="field-label">連携例 (curl / GitHub Actions / CI)</label>
            <div style="position: relative;">
              <pre style="padding: 10px 12px; background: var(--bg-main); border: 1px solid var(--border-color); border-radius: 6px; font-family: monospace; font-size: 12px; overflow-x: auto; white-space: pre-wrap;">{{ inboundCurlExample }}</pre>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                style="position: absolute; top: 8px; right: 8px; font-size: 11px;"
                @click="copyToClipboard(inboundCurlExample, 'curlコマンドをコピー')"
              >
                コピー
              </button>
            </div>
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; margin-top: 18px;">
          <button class="btn btn-primary" @click="createdInboundWebhook = null">保存したので閉じる</button>
        </div>
      </div>
    </div>

    <!-- Edit Project Type Modal -->
    <div v-if="editingProjectType" class="modal-backdrop" v-backdrop-dismiss="() => editingProjectType = null">
      <div class="modal-panel" style="max-width: 460px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 15px; font-weight: 700;">プロジェクト種別の編集</h3>
          <button class="btn btn-secondary btn-sm" @click="editingProjectType = null">✕</button>
        </div>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="updateProjectType">
          <div>
            <label class="field-label">表示順 *</label>
            <input v-model.number="editProjectTypeForm.display_order" type="number" min="1" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">種別名 *</label>
            <input v-model="editProjectTypeForm.name" type="text" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">カラー</label>
            <div style="display: flex; align-items: center; gap: 8px;">
              <input v-model="editProjectTypeForm.color" type="color" style="width: 44px; height: 34px; padding: 2px; cursor: pointer;" />
              <span class="badge" :style="{ backgroundColor: editProjectTypeForm.color, color: '#fff', fontSize: '11px', fontWeight: 'bold' }">
                プレビュー: {{ editProjectTypeForm.name || '種別' }}
              </span>
            </div>
          </div>
          <div style="display: flex; flex-direction: column; gap: 4px;">
            <label class="field-label">プリセットカラー</label>
            <div style="display: flex; flex-wrap: wrap; gap: 6px;">
              <button
                v-for="c in PRESET_COLORS"
                :key="c"
                type="button"
                style="width: 22px; height: 22px; border-radius: 4px; cursor: pointer; border: 2px solid transparent;"
                :style="{
                  backgroundColor: c,
                  borderColor: editProjectTypeForm.color.toLowerCase() === c.toLowerCase() ? 'var(--text-main)' : 'transparent'
                }"
                :title="c"
                @click="editProjectTypeForm.color = c"
              />
            </div>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 12px; border-top: 1px solid var(--border-color); padding-top: 10px;">
            <button type="button" class="btn btn-secondary" @click="editingProjectType = null">キャンセル</button>
            <button type="submit" class="btn btn-primary">変更を保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- CSV Import Modal -->
    <CSVImportModal
      v-if="showCSVImportModal"
      :project-id="projectId"
      @close="showCSVImportModal = false"
      @imported="refreshAllWorkspace"
    />

    </template>
  </div>
</template>

<script setup lang="ts">
import type {
  CustomFieldDefinition,
  Group,
  InboundWebhook,
  Issue,
  ItemType,
  Label,
  Project,
  ProjectMember,
  Sprint,
  Status,
  TargetType,
  Task,
  TaskDependency,
  Template,
  TemplateRow,
  User,
  Webhook,
  WebhookDelivery
} from '~/types/polaris'
import {
  addDaysYMD,
  buildGanttHeaderGroups,
  calcGanttBarPosition,
  calculateAutoScheduleUpdates,
  calculateGanttDependencyPath,
  formatCost,
  formatHours,
  formatLocalYMD,
  generateDateRangeYMD,
  getGanttScaleConfig,
  hasAtMostTwoDecimals,
  isTaskCompleted
} from '~/utils/format'
import {
  calculateColumnOffset,
  calculateNextScrollLeft,
  getScrollBounds,
  isOverflowing
} from '~/utils/kanban_scroll'

const PRESET_COLORS = [
  '#3b82f6', // blue
  '#ef4444', // red
  '#10b981', // green
  '#f59e0b', // amber
  '#8b5cf6', // purple
  '#ec4899', // pink
  '#6366f1', // indigo
  '#64748b', // slate
  '#06b6d4', // cyan
  '#f97316'  // orange
]

const route = useRoute()
const router = useRouter()
const { apiFetch, notifyToast, refreshProjects, hasSystemPerm } = useApi()

const projectId = computed(() => route.params.id as string)

type ProjectTab = 'kanban' | 'gantt' | 'list' | 'sprint' | 'workload' | 'wiki' | 'activity' | 'settings'
const validTabs: ProjectTab[] = ['kanban', 'gantt', 'list', 'sprint', 'workload', 'wiki', 'activity', 'settings']
const initialTab = (route.query.tab as ProjectTab)
const activeTab = ref<ProjectTab>(validTabs.includes(initialTab) ? initialTab : 'kanban')

watch(activeTab, (newTab) => {
  if (route.query.tab !== newTab) {
    router.replace({ query: { ...route.query, tab: newTab } })
  }
})

watch(() => route.query.tab, (tab) => {
  if (tab && validTabs.includes(tab as ProjectTab) && activeTab.value !== tab) {
    activeTab.value = tab as ProjectTab
  }
})

const isSubmittingIssue = ref(false)
const isSubmittingTask = ref(false)
const isGanttLeftCollapsed = ref(false)

const inboundWebhooks = ref<InboundWebhook[]>([])
const showNewInboundWebhookModal = ref(false)
const newInboundWebhookForm = reactive({
  name: '',
  default_issue_id: '',
  default_status_id: undefined as number | undefined
})
const createdInboundWebhook = ref<InboundWebhook | null>(null)
const showCSVImportModal = ref(false)

const inboundWebhookUrl = computed(() => {
  if (!createdInboundWebhook.value) return ''
  const origin = typeof window !== 'undefined' ? window.location.origin : ''
  return `${origin}/api/v1/inbound-webhooks/${createdInboundWebhook.value.token}`
})

const inboundCurlExample = computed(() => {
  if (!createdInboundWebhook.value) return ''
  return `curl -X POST "${inboundWebhookUrl.value}" \\
  -H "Content-Type: application/json" \\
  -d '{"title": "CIテスト失敗の自動報告", "description": "ビルド #123 でテストが失敗しました"}'`
})

const project = ref<Project | null>(null)
const statuses = ref<Status[]>([])
const projectLabels = ref<Label[]>([])
const systemLabels = ref<Label[]>([])
const customFields = ref<CustomFieldDefinition[]>([])
const templates = ref<Template[]>([])
const members = ref<ProjectMember[]>([])
const projectGroups = ref<Group[]>([])
const globalGroups = ref<Group[]>([])
const users = ref<User[]>([])
const issues = ref<Issue[]>([])
const allProjectIssues = ref<Issue[]>([])
const sprints = ref<Sprint[]>([])
const dependencies = ref<TaskDependency[]>([])
const projectTypes = ref<ItemType[]>([])
const sortedProjectTypes = computed(() => [...projectTypes.value].sort((a, b) => a.display_order - b.display_order))

function getType(typeId?: string | null): ItemType | undefined {
  if (!typeId) return undefined
  return projectTypes.value.find(t => t.id === typeId)
}

const autoScheduleEnabled = ref(true)
const showDependencyLines = ref(true)

const allTasks = computed<Task[]>(() => {
  const sourceIssues = allProjectIssues.value.length > 0 ? allProjectIssues.value : issues.value
  const list: Task[] = []
  for (const iss of sourceIssues) {
    for (const t of iss.tasks || []) {
      list.push(t)
    }
  }
  return list
})

const sprintsWithDates = computed(() => {
  return sprints.value.filter(sp => sp.start_date && sp.end_date)
})

const ganttDependencyLines = computed(() => {
  if (!showDependencyLines.value || dependencies.value.length === 0) return []

  const headerOffset = sprintsWithDates.value.length > 0 ? 68 : 44
  let currentRowIndex = 0
  const taskRowYMap = new Map<string, number>()

  for (const iss of issues.value) {
    currentRowIndex++
    for (const t of iss.tasks || []) {
      const topY = headerOffset + (currentRowIndex * 62)
      taskRowYMap.set(t.id, topY + 20)
      currentRowIndex++
    }
  }

  const lines: { id: string; path: string; tooltip: string }[] = []
  for (const dep of dependencies.value) {
    const fromY = taskRowYMap.get(dep.predecessor_id)
    const toY = taskRowYMap.get(dep.successor_id)
    if (fromY === undefined || toY === undefined) continue

    const predTask = allTasks.value.find(t => t.id === dep.predecessor_id)
    const succTask = allTasks.value.find(t => t.id === dep.successor_id)
    if (!predTask || !succTask) continue
    if (!predTask.planned_start_date || !predTask.planned_end_date) continue
    if (!succTask.planned_start_date || !succTask.planned_end_date) continue

    const predPos = getBarPos(predTask.planned_start_date, predTask.planned_end_date)
    const succPos = getBarPos(succTask.planned_start_date, succTask.planned_end_date)
    if (!predPos.visible && !succPos.visible) continue

    const fromX = predPos.leftPx + predPos.widthPx
    const toX = succPos.leftPx

    const path = calculateGanttDependencyPath(fromX, fromY, toX, toY)
    const tooltip = `${predTask.title} ➔ ${succTask.title} (${dep.dependency_type || 'FS'}${dep.lag_days ? ` +${dep.lag_days}d` : ''})`
    lines.push({
      id: dep.id,
      path,
      tooltip
    })
  }
  return lines
})

async function triggerAutoSchedule(changedTaskId: string, newEndDate: string) {
  if (!autoScheduleEnabled.value || dependencies.value.length === 0) return
  const updates = calculateAutoScheduleUpdates(changedTaskId, newEndDate, dependencies.value, allTasks.value)
  if (updates.length === 0) return

  try {
    await Promise.all(
      updates.map(u =>
        apiFetch(`/projects/${projectId.value}/tasks/${u.taskId}/gantt-dates`, {
          method: 'PATCH',
          body: JSON.stringify({
            planned_start_date: u.planned_start_date,
            planned_end_date: u.planned_end_date
          })
        })
      )
    )
    notifyToast(`⚡ オートスケジューリング: 後続 ${updates.length} 件の予定期間を自動スライド調整しました`, 'info')
  } catch (err: any) {
    console.error('Auto schedule update error:', err)
  }
}

function openTaskDetailStandalone(t: Task) {
  let parentIss =
    issues.value.find(i => (i.tasks || []).some(x => x.id === t.id)) ||
    allProjectIssues.value.find(i => (i.tasks || []).some(x => x.id === t.id))
  if (!parentIss && t.issue_id) {
    parentIss =
      issues.value.find(i => i.id === t.issue_id) ||
      allProjectIssues.value.find(i => i.id === t.issue_id)
  }
  openTaskDetail(t, parentIss || ({ id: t.issue_id, title: '親課題' } as any))
}

async function refreshAllWorkspace() {
  await Promise.all([loadIssues(), loadWorkspace(), refreshProjects()])
}

const canManageProjectSettings = computed(() => {
  return project.value?.current_user_role === 'admin' || hasSystemPerm('manage_system')
})

const canEditProjectItems = computed(() => {
  return project.value?.current_user_role === 'admin' || project.value?.current_user_role === 'editor'
})

watch([activeTab, canManageProjectSettings], ([newTab, canManage]) => {
  if (newTab === 'settings' && !canManage) {
    activeTab.value = 'kanban'
  }
})

const allLabels = computed(() => [...systemLabels.value, ...projectLabels.value])
const allGroups = computed(() => {
  const map = new Map<string, Group>()
  for (const g of globalGroups.value) {
    map.set(g.id, g)
  }
  for (const g of projectGroups.value) {
    map.set(g.id, g)
  }
  return Array.from(map.values())
})

const filters = reactive({
  issue_state: 'open',
  type_id: '',
  status_id: '',
  assignee_id: '',
  sprint_id: '',
  keyword: '',
  label_ids: [] as string[]
})

// Kanban State
const draggingTask = ref<Task | null>(null)
const dragOverKey = ref('')
const dragOverCardId = ref<string | null>(null)
const dragOverPosition = ref<'before' | 'after'>('before')
const collapsedIssueIds = ref<Set<string>>(new Set())

function toggleIssueCollapse(id: string) {
  if (collapsedIssueIds.value.has(id)) {
    collapsedIssueIds.value.delete(id)
  } else {
    collapsedIssueIds.value.add(id)
  }
  collapsedIssueIds.value = new Set(collapsedIssueIds.value)
}

function expandAllIssues() {
  collapsedIssueIds.value = new Set()
}

function collapseAllIssues() {
  collapsedIssueIds.value = new Set(issues.value.map(i => i.id))
}

function onCardDragOver(e: DragEvent, cardId: string) {
  dragOverCardId.value = cardId
  const target = e.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const offsetY = e.clientY - rect.top
  dragOverPosition.value = offsetY > rect.height / 2 ? 'after' : 'before'
}

function onCardDragLeave(cardId: string) {
  if (dragOverCardId.value === cardId) {
    dragOverCardId.value = null
  }
}

// Gantt State
const timelinePaneRef = ref<HTMLElement | null>(null)
const timelinePaneWidth = ref(0)
let timelineResizeObserver: ResizeObserver | null = null

const ganttScale = ref<'day' | 'week' | 'month'>('day')
const baseDayWidth = computed(() => {
  return getGanttScaleConfig(ganttScale.value).baseDayWidth
})

const ganttDayWidth = computed(() => {
  const count = timelineDays.value.length
  if (!count) return baseDayWidth.value
  const minTotal = count * baseDayWidth.value
  if (timelinePaneWidth.value > minTotal) {
    return timelinePaneWidth.value / count
  }
  return baseDayWidth.value
})
const todayYMD = formatLocalYMD()
const timelineStartYMD = ref(addDaysYMD(todayYMD, -5))
const timelineDays = computed(() => {
  const count = getGanttScaleConfig(ganttScale.value).count
  return generateDateRangeYMD(timelineStartYMD.value, count)
})
const createDragStartYMD = ref<string | null>(null)

const ganttHeaderGroups = computed(() => {
  return buildGanttHeaderGroups(timelineDays.value, ganttScale.value, ganttDayWidth.value)
})

function isWeekend(ymd: string): boolean {
  const d = new Date(ymd).getDay()
  return d === 0 || d === 6
}

function scrollToToday() {
  if (!timelinePaneRef.value) return
  const pos = getBarPos(todayYMD, todayYMD)
  if (pos.visible) {
    const paneWidth = timelinePaneRef.value.clientWidth
    timelinePaneRef.value.scrollTo({
      left: Math.max(0, pos.leftPx - paneWidth / 2 + ganttDayWidth.value / 2),
      behavior: 'smooth'
    })
  } else {
    resetTimelineToday()
  }
}

function isTaskOverdue(t: Task): boolean {
  if (!t.planned_end_date) return false
  if (isTaskCompleted(t, statuses.value)) return false
  return t.planned_end_date < todayYMD
}

// Modals
const showNewIssueModal = ref(false)
const newIssueForm = reactive({
  title: '',
  description: '',
  create_initial_task: true
})

const showNewTaskModal = ref(false)
const newTaskParentIssue = ref<Issue | null>(null)
const newTaskForm = reactive({
  title: '',
  description: '',
  planned_start_date: '',
  planned_end_date: '',
  planned_hours: '' as string | number,
  actual_hours: '' as string | number,
  planned_cost: '' as string | number,
  actual_cost: '' as string | number,
  progress: 0
})
const wasDragging = ref(false)

const detailModalState = reactive({
  open: false,
  targetType: 'issue' as TargetType,
  issue: null as Issue | null,
  task: null as Task | null,
  parentIssueTitle: ''
})

// Settings Forms
const projForm = reactive({
  name: '',
  description: '',
  hours_per_day: 8.0,
  default_issue_template_id: '' as string | null,
  default_task_template_id: '' as string | null
})

const memberForm = reactive({
  subject_combined: '',
  role: 'editor'
})

const statusForm = reactive({
  display_order: 5,
  name: '',
  color: '#3b82f6'
})
const migrateTargetMap = reactive<Record<string, string>>({})

const labelForm = reactive({
  name: '',
  color: '#3b82f6'
})

const cfForm = reactive({
  target_type: 'issue' as TargetType,
  name: '',
  field_type: 'text_single',
  options_csv: ''
})

const projectTypeForm = reactive({
  name: '',
  color: '#3b82f6',
  display_order: 1
})

const editingProjectType = ref<ItemType | null>(null)
const editProjectTypeForm = reactive({
  name: '',
  color: '#3b82f6',
  display_order: 1
})

function updateTimelinePaneWidth() {
  if (timelinePaneRef.value) {
    timelinePaneWidth.value = timelinePaneRef.value.clientWidth
  }
}

watch(activeTab, (tab) => {
  if (tab === 'gantt') {
    nextTick(() => {
      updateTimelinePaneWidth()
      if (timelinePaneRef.value && !timelineResizeObserver) {
        timelineResizeObserver = new ResizeObserver(entries => {
          for (const entry of entries) {
            timelinePaneWidth.value = entry.contentRect.width
          }
        })
        timelineResizeObserver.observe(timelinePaneRef.value)
      }
    })
  }
})

watch(isGanttLeftCollapsed, () => {
  nextTick(() => {
    updateTimelinePaneWidth()
  })
})

function onProjectGlobalKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    if (createdInboundWebhook.value) {
      createdInboundWebhook.value = null
      return
    }
    if (showNewInboundWebhookModal.value) {
      showNewInboundWebhookModal.value = false
      return
    }
    if (showCSVImportModal.value) {
      showCSVImportModal.value = false
      return
    }
    if (showNewIssueModal.value) {
      showNewIssueModal.value = false
      return
    }
    if (showNewTaskModal.value) {
      showNewTaskModal.value = false
      return
    }
    if (selectedWebhookForLogs.value) {
      selectedWebhookForLogs.value = null
      return
    }
    if (editingWebhook.value) {
      editingWebhook.value = null
      return
    }
  }
}

// Kanban Horizontal Scroll Assistant
const kanbanScrollContainerRef = ref<HTMLElement | null>(null)
const kanbanTopScrollRef = ref<HTMLElement | null>(null)
const kanbanIsOverflowing = ref(false)
const kanbanCanScrollLeft = ref(false)
const kanbanCanScrollRight = ref(false)
let isSyncingScroll = false
let kanbanResizeObserver: ResizeObserver | null = null

function updateKanbanScrollState() {
  const el = kanbanScrollContainerRef.value
  if (!el) {
    kanbanIsOverflowing.value = false
    kanbanCanScrollLeft.value = false
    kanbanCanScrollRight.value = false
    return
  }
  kanbanIsOverflowing.value = isOverflowing(el.scrollWidth, el.clientWidth)
  const bounds = getScrollBounds(el.scrollLeft, el.clientWidth, el.scrollWidth)
  kanbanCanScrollLeft.value = bounds.canScrollLeft
  kanbanCanScrollRight.value = bounds.canScrollRight
}

function onMainScroll() {
  if (isSyncingScroll) return
  if (kanbanScrollContainerRef.value && kanbanTopScrollRef.value) {
    isSyncingScroll = true
    kanbanTopScrollRef.value.scrollLeft = kanbanScrollContainerRef.value.scrollLeft
    requestAnimationFrame(() => {
      isSyncingScroll = false
    })
  }
  updateKanbanScrollState()
}

function onTopScroll() {
  if (isSyncingScroll) return
  if (kanbanTopScrollRef.value && kanbanScrollContainerRef.value) {
    isSyncingScroll = true
    kanbanScrollContainerRef.value.scrollLeft = kanbanTopScrollRef.value.scrollLeft
    requestAnimationFrame(() => {
      isSyncingScroll = false
    })
  }
  updateKanbanScrollState()
}

function scrollKanban(direction: 'left' | 'right') {
  const el = kanbanScrollContainerRef.value
  if (!el) return
  const maxScroll = Math.max(0, el.scrollWidth - el.clientWidth)
  const target = calculateNextScrollLeft(el.scrollLeft, direction, 280, maxScroll)
  el.scrollTo({ left: target, behavior: 'smooth' })
}

function scrollToColumn(index: number) {
  const el = kanbanScrollContainerRef.value
  if (!el) return
  const maxScroll = Math.max(0, el.scrollWidth - el.clientWidth)
  const target = calculateColumnOffset(index, statuses.value.length, el.scrollWidth, maxScroll)
  el.scrollTo({ left: target, behavior: 'smooth' })
}

function onJumpSelect(event: Event) {
  const target = event.target as HTMLSelectElement
  const idx = parseInt(target.value, 10)
  if (!Number.isNaN(idx)) {
    scrollToColumn(idx)
  }
  target.value = ''
}

onMounted(async () => {
  window.addEventListener('keydown', onProjectGlobalKeydown)
  if (typeof window !== 'undefined') {
    window.addEventListener('resize', updateKanbanScrollState)
  }
  await loadWorkspace()
  nextTick(() => {
    updateTimelinePaneWidth()
    if (timelinePaneRef.value) {
      timelineResizeObserver = new ResizeObserver(entries => {
        for (const entry of entries) {
          timelinePaneWidth.value = entry.contentRect.width
        }
      })
      timelineResizeObserver.observe(timelinePaneRef.value)
    }
    updateKanbanScrollState()
    if (kanbanScrollContainerRef.value && typeof ResizeObserver !== 'undefined') {
      kanbanResizeObserver = new ResizeObserver(() => {
        updateKanbanScrollState()
      })
      kanbanResizeObserver.observe(kanbanScrollContainerRef.value)
    }
  })
})

onUnmounted(() => {
  window.removeEventListener('keydown', onProjectGlobalKeydown)
  if (typeof window !== 'undefined') {
    window.removeEventListener('resize', updateKanbanScrollState)
  }
  if (timelineResizeObserver) {
    timelineResizeObserver.disconnect()
    timelineResizeObserver = null
  }
  if (kanbanResizeObserver) {
    kanbanResizeObserver.disconnect()
    kanbanResizeObserver = null
  }
})

watch([() => statuses.value.length, () => activeTab.value, () => issues.value.length], () => {
  nextTick(() => {
    updateKanbanScrollState()
  })
})

watch(() => route.params.id, async () => {
  await loadWorkspace()
})

watch(
  () => [route.query.item, route.query.openTarget],
  async ([itemVal, openTargetVal]) => {
    const raw = itemVal || openTargetVal
    if (raw) {
      await openTargetFromQuery(String(raw))
    } else {
      detailModalState.open = false
    }
  }
)

async function openTargetFromQuery(raw: string) {
  if (route.query.item !== raw && route.query.openTarget !== raw) {
    router.push({ query: { ...route.query, item: raw } })
  }
  const [targetType, targetId] = raw.split(':')
  if (!targetType || !targetId) return
  if (targetType === 'issue') {
    let found = issues.value.find(i => i.id === targetId)
    if (!found) {
      try {
        found = await apiFetch<Issue>(`/projects/${projectId.value}/issues/${targetId}`)
      } catch (e) {}
    }
    if (found) {
      detailModalState.targetType = 'issue'
      detailModalState.issue = found
      detailModalState.task = null
      detailModalState.parentIssueTitle = ''
      detailModalState.open = true
    }
  } else if (targetType === 'task') {
    let foundTask: Task | undefined
    let parentIss: Issue | undefined
    for (const iss of issues.value) {
      const found = (iss.tasks || []).find(t => t.id === targetId)
      if (found) {
        foundTask = found
        parentIss = iss
        break
      }
    }
    if (!foundTask) {
      try {
        const fetchedTask = await apiFetch<Task>(`/projects/${projectId.value}/tasks/${targetId}`)
        if (fetchedTask) {
          foundTask = fetchedTask
          if (fetchedTask.issue_id) {
            parentIss = issues.value.find(i => i.id === fetchedTask.issue_id)
            if (!parentIss) {
              try {
                parentIss = await apiFetch<Issue>(`/projects/${projectId.value}/issues/${fetchedTask.issue_id}`)
              } catch (e) {}
            }
          }
        }
      } catch (e) {}
    } else if (foundTask && !parentIss && foundTask.issue_id) {
      parentIss = issues.value.find(i => i.id === foundTask.issue_id)
      if (!parentIss) {
        try {
          parentIss = await apiFetch<Issue>(`/projects/${projectId.value}/issues/${foundTask.issue_id}`)
        } catch (e) {}
      }
    }
    if (foundTask) {
      detailModalState.targetType = 'task'
      detailModalState.task = foundTask
      detailModalState.issue = null
      detailModalState.parentIssueTitle = parentIss ? parentIss.title : ''
      detailModalState.open = true
    }
  }
}

async function loadWorkspace() {
  if (!projectId.value) return
  try {
    const [p, sts, pLabels, sLabels, cfs, tpls, mems, pGrps, gGrps, usrs, sps, deps, pts] = await Promise.all([
      apiFetch<Project>(`/projects/${projectId.value}`),
      apiFetch<Status[]>(`/projects/${projectId.value}/statuses`),
      apiFetch<Label[]>(`/projects/${projectId.value}/labels`),
      apiFetch<Label[]>('/labels'),
      apiFetch<CustomFieldDefinition[]>(`/projects/${projectId.value}/custom-fields`),
      apiFetch<Template[]>(`/projects/${projectId.value}/templates`),
      apiFetch<ProjectMember[]>(`/projects/${projectId.value}/members`),
      apiFetch<Group[]>(`/projects/${projectId.value}/groups`),
      apiFetch<Group[]>('/groups'),
      apiFetch<User[]>('/users'),
      apiFetch<Sprint[]>(`/projects/${projectId.value}/sprints`).catch(() => []),
      apiFetch<TaskDependency[]>(`/projects/${projectId.value}/dependencies`).catch(() => []),
      apiFetch<ItemType[]>(`/projects/${projectId.value}/types`).catch(() => [])
    ])
    project.value = p
    statuses.value = (sts || []).sort((a, b) => a.display_order - b.display_order)
    projectLabels.value = pLabels || []
    systemLabels.value = sLabels || []
    customFields.value = cfs || []
    templates.value = tpls || []
    members.value = mems || []
    projectGroups.value = pGrps || []
    globalGroups.value = gGrps || []
    users.value = usrs || []
    sprints.value = sps || []
    dependencies.value = deps || []
    projectTypes.value = pts || []
    if (!projectTypeForm.name) {
      projectTypeForm.display_order = Math.max(...projectTypes.value.map(t => t.display_order), 0) + 1
    }

    projForm.name = p.name
    projForm.description = p.description
    projForm.hours_per_day = p.hours_per_day
    projForm.default_issue_template_id = p.default_issue_template_id || null
    projForm.default_task_template_id = p.default_task_template_id || null

    await loadIssues()
    if (project.value?.current_user_role === 'admin') {
      await loadProjectWebhooks()
      await loadInboundWebhooks()
    }
    const targetQuery = route.query.item || route.query.openTarget
    if (targetQuery) {
      await openTargetFromQuery(String(targetQuery))
    }
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function loadIssues() {
  if (!projectId.value) return
  const params = new URLSearchParams()
  if (filters.issue_state) params.set('state', filters.issue_state)
  if (filters.type_id) params.set('type_id', filters.type_id)
  if (filters.status_id) params.set('status_id', filters.status_id)
  if (filters.assignee_id) params.set('assignee_id', filters.assignee_id)
  if (filters.sprint_id) params.set('sprint_id', filters.sprint_id)
  if (filters.keyword.trim()) params.set('keyword', filters.keyword.trim())
  if (filters.label_ids.length > 0) params.set('label_ids', filters.label_ids.join(','))

  const hasTaskFilters = Boolean(
    filters.type_id ||
    filters.status_id ||
    filters.assignee_id ||
    filters.sprint_id ||
    filters.keyword.trim() ||
    filters.label_ids.length > 0
  )
  const isUnfilteredAll = !hasTaskFilters && filters.issue_state === 'all'

  try {
    if (isUnfilteredAll) {
      const list = await apiFetch<Issue[]>(`/projects/${projectId.value}/issues?${params.toString()}`)
      issues.value = list || []
      allProjectIssues.value = issues.value
    } else {
      const [list, allList] = await Promise.all([
        apiFetch<Issue[]>(`/projects/${projectId.value}/issues?${params.toString()}`),
        apiFetch<Issue[]>(`/projects/${projectId.value}/issues?state=all`).catch(() => null)
      ])
      issues.value = list || []
      allProjectIssues.value = allList !== null ? allList : issues.value
    }
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function toggleFilterLabel(id: string) {
  const idx = filters.label_ids.indexOf(id)
  if (idx >= 0) {
    filters.label_ids.splice(idx, 1)
  } else {
    filters.label_ids.push(id)
  }
  loadIssues()
}

function getTasksForCell(iss: Issue, statusId: string): Task[] {
  return (iss.tasks || [])
    .filter(t => t.status_id === statusId)
    .sort((a, b) => a.kanban_order - b.kanban_order)
}

function countTasksByStatus(statusId: string): number {
  let count = 0
  for (const iss of issues.value) {
    count += (iss.tasks || []).filter(t => t.status_id === statusId).length
  }
  return count
}

function onDragStartTask(t: Task) {
  if (!canEditProjectItems.value) return
  draggingTask.value = t
}

async function onTaskDropKanban(targetIssueId: string, targetStatusId: string, targetIndexOrOrder: number) {
  dragOverKey.value = ''
  if (!draggingTask.value || !canEditProjectItems.value) return
  const t = draggingTask.value
  draggingTask.value = null

  try {
    const targetIss = issues.value.find(i => i.id === targetIssueId)
    const existing = (targetIss ? getTasksForCell(targetIss, targetStatusId) : []).filter(item => item.id !== t.id)
    const insertIdx = Math.max(0, Math.min(targetIndexOrOrder - 1, existing.length))
    const reordered = [...existing]
    reordered.splice(insertIdx, 0, t)

    const updates: Promise<any>[] = []
    reordered.forEach((taskItem, idx) => {
      const cleanOrder = idx + 1
      if (taskItem.id === t.id) {
        updates.push(
          apiFetch(`/projects/${projectId.value}/tasks/${t.id}/kanban-move`, {
            method: 'PATCH',
            body: JSON.stringify({
              issue_id: targetIssueId,
              status_id: targetStatusId,
              kanban_order: cleanOrder
            })
          })
        )
      } else if (taskItem.kanban_order !== cleanOrder) {
        taskItem.kanban_order = cleanOrder
        updates.push(
          apiFetch(`/projects/${projectId.value}/tasks/${taskItem.id}/kanban-move`, {
            method: 'PATCH',
            body: JSON.stringify({
              issue_id: targetIssueId,
              status_id: targetStatusId,
              kanban_order: cleanOrder
            })
          })
        )
      }
    })

    await Promise.all(updates)
    await loadIssues()
    if (project.value?.current_user_role === 'admin') {
      await loadProjectWebhooks()
      await loadInboundWebhooks()
    }
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}
const onDropTaskCard = onTaskDropKanban

// Gantt Helpers & Drag/Resize
function getBarPos(startYMD?: string | null, endYMD?: string | null) {
  return calcGanttBarPosition(timelineStartYMD.value, startYMD, endYMD, ganttDayWidth.value)
}

function shiftTimeline(days: number) {
  timelineStartYMD.value = addDaysYMD(timelineStartYMD.value, days)
}

function resetTimelineToday() {
  timelineStartYMD.value = addDaysYMD(todayYMD, -3)
}

function startGanttBarDrag(
  ev: MouseEvent,
  task: Task,
  barType: 'planned' | 'actual',
  mode: 'move' | 'resize-start' | 'resize-end'
) {
  if (!canEditProjectItems.value) return
  const startX = ev.clientX
  const origStart = barType === 'planned' ? task.planned_start_date : task.actual_start_date
  const origEnd = barType === 'planned' ? task.planned_end_date : task.actual_end_date
  if (!origStart || !origEnd) return

  const onMouseMove = (moveEv: MouseEvent) => {
    if (Math.abs(moveEv.clientX - startX) > 3) {
      wasDragging.value = true
    }
    const deltaDays = Math.round((moveEv.clientX - startX) / ganttDayWidth.value)
    if (deltaDays === 0) return
    let nextStart = origStart
    let nextEnd = origEnd
    if (mode === 'move') {
      nextStart = addDaysYMD(origStart, deltaDays)
      nextEnd = addDaysYMD(origEnd, deltaDays)
    } else if (mode === 'resize-start') {
      nextStart = addDaysYMD(origStart, deltaDays)
      if (nextStart > nextEnd) nextStart = nextEnd
    } else if (mode === 'resize-end') {
      nextEnd = addDaysYMD(origEnd, deltaDays)
      if (nextEnd < nextStart) nextEnd = nextStart
    }
    if (barType === 'planned') {
      task.planned_start_date = nextStart
      task.planned_end_date = nextEnd
    } else {
      task.actual_start_date = nextStart
      task.actual_end_date = nextEnd
    }
  }

  const onMouseUp = async () => {
    window.removeEventListener('mousemove', onMouseMove)
    window.removeEventListener('mouseup', onMouseUp)
    setTimeout(() => {
      wasDragging.value = false
    }, 100)
    try {
      const payload: Record<string, any> = {}
      if (barType === 'actual') {
        payload.actual_start_date = task.actual_start_date
        payload.actual_end_date = task.actual_end_date
      } else {
        payload.planned_start_date = task.planned_start_date
        payload.planned_end_date = task.planned_end_date
      }
      await apiFetch(`/projects/${projectId.value}/tasks/${task.id}/gantt-dates`, {
        method: 'PATCH',
        body: JSON.stringify(payload)
      })
      if (barType === 'planned' && autoScheduleEnabled.value && task.planned_end_date) {
        await triggerAutoSchedule(task.id, task.planned_end_date)
      }
      await loadIssues()
    } catch (e: any) {
      notifyToast(e.message, 'error')
      await loadIssues()
    }
  }

  window.addEventListener('mousemove', onMouseMove)
  window.addEventListener('mouseup', onMouseUp)
}

function onCellMouseDownCreate(task: Task, ymd: string) {
  if (!canEditProjectItems.value || task.planned_start_date) return
  createDragStartYMD.value = ymd
}

async function onCellMouseUpCreate(task: Task, ymd: string) {
  if (!createDragStartYMD.value || task.planned_start_date) return
  const s = createDragStartYMD.value <= ymd ? createDragStartYMD.value : ymd
  const e = createDragStartYMD.value <= ymd ? ymd : createDragStartYMD.value
  createDragStartYMD.value = null
  try {
    await apiFetch(`/projects/${projectId.value}/tasks/${task.id}/gantt-dates`, {
      method: 'PATCH',
      body: JSON.stringify({
        planned_start_date: s,
        planned_end_date: e
      })
    })
    notifyToast(`予定期間 (${s} 〜 ${e}) を作成しました`, 'success')
    if (autoScheduleEnabled.value) {
      await triggerAutoSchedule(task.id, e)
    }
    await loadIssues()
  } catch (err: any) {
    notifyToast(err.message, 'error')
  }
}

async function initActualFromPlanned(task: Task) {
  const s = task.planned_start_date || todayYMD
  const e = task.planned_end_date || s
  try {
    await apiFetch(`/projects/${projectId.value}/tasks/${task.id}/gantt-dates`, {
      method: 'PATCH',
      body: JSON.stringify({
        actual_start_date: s,
        actual_end_date: e
      })
    })
    await loadIssues()
  } catch (err: any) {
    notifyToast(err.message, 'error')
  }
}

// Create Issue / Task Modals
function openNewIssueModal() {
  newIssueForm.title = ''
  newIssueForm.description = ''
  newIssueForm.create_initial_task = true
  showNewIssueModal.value = true
}

async function submitNewIssue() {
  if (isSubmittingIssue.value) return
  if (!newIssueForm.title.trim()) {
    notifyToast('課題タイトルを入力してください', 'error')
    return
  }
  isSubmittingIssue.value = true
  try {
    await apiFetch(`/projects/${projectId.value}/issues`, {
      method: 'POST',
      body: JSON.stringify({
        ...newIssueForm,
        title: newIssueForm.title.trim()
      })
    })
    showNewIssueModal.value = false
    await loadIssues()
    if (project.value?.current_user_role === 'admin') {
      await loadProjectWebhooks()
      await loadInboundWebhooks()
    }
    notifyToast('課題を作成しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    isSubmittingIssue.value = false
  }
}

function openNewTaskModal(iss: Issue) {
  newTaskParentIssue.value = iss
  newTaskForm.title = ''
  newTaskForm.description = ''
  newTaskForm.planned_start_date = ''
  newTaskForm.planned_end_date = ''
  newTaskForm.planned_hours = ''
  newTaskForm.actual_hours = ''
  newTaskForm.planned_cost = ''
  newTaskForm.actual_cost = ''
  newTaskForm.progress = 0
  showNewTaskModal.value = true
}

async function submitNewTask() {
  if (!newTaskParentIssue.value || isSubmittingTask.value) return
  if (!newTaskForm.title.trim()) {
    notifyToast('タスクタイトルを入力してください', 'error')
    return
  }
  const pHours = newTaskForm.planned_hours === '' ? null : Number(newTaskForm.planned_hours)
  const aHours = newTaskForm.actual_hours === '' ? null : Number(newTaskForm.actual_hours)
  const pCost = newTaskForm.planned_cost === '' ? null : Number(newTaskForm.planned_cost)
  const aCost = newTaskForm.actual_cost === '' ? null : Number(newTaskForm.actual_cost)
  if (pHours !== null && !hasAtMostTwoDecimals(pHours)) {
    notifyToast('予定工数は小数点第2位までで入力してください', 'error')
    return
  }
  if (aHours !== null && !hasAtMostTwoDecimals(aHours)) {
    notifyToast('実績工数は小数点第2位までで入力してください', 'error')
    return
  }
  if (pCost !== null && !hasAtMostTwoDecimals(pCost)) {
    notifyToast('予定コストは小数点第2位までで入力してください', 'error')
    return
  }
  if (aCost !== null && !hasAtMostTwoDecimals(aCost)) {
    notifyToast('実績コストは小数点第2位までで入力してください', 'error')
    return
  }
  isSubmittingTask.value = true
  try {
    await apiFetch(`/projects/${projectId.value}/tasks`, {
      method: 'POST',
      body: JSON.stringify({
        issue_id: newTaskParentIssue.value.id,
        title: newTaskForm.title.trim(),
        description: newTaskForm.description,
        planned_start_date: newTaskForm.planned_start_date || null,
        planned_end_date: newTaskForm.planned_end_date || null,
        planned_hours: pHours,
        actual_hours: aHours,
        planned_cost: pCost,
        actual_cost: aCost,
        progress: newTaskForm.progress
      })
    })
    showNewTaskModal.value = false
    await Promise.all([loadIssues(), loadWorkspace()])
    notifyToast('子タスクを作成しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    isSubmittingTask.value = false
  }
}

function onGanttBarClick(t: Task, parentIss: Issue) {
  if (wasDragging.value) return
  openTaskDetail(t, parentIss)
}

function openIssueDetail(iss: Issue) {
  detailModalState.targetType = 'issue'
  detailModalState.issue = iss
  detailModalState.task = null
  detailModalState.parentIssueTitle = ''
  detailModalState.open = true
  router.push({ query: { ...route.query, item: `issue:${iss.id}` } })
}

function openTaskDetail(t: Task, parentIss: Issue) {
  detailModalState.targetType = 'task'
  detailModalState.task = t
  detailModalState.issue = null
  detailModalState.parentIssueTitle = parentIss.title
  detailModalState.open = true
  router.push({ query: { ...route.query, item: `task:${t.id}` } })
}

function closeDetailView() {
  detailModalState.open = false
  if (route.query.item || route.query.openTarget) {
    if (typeof window !== 'undefined' && window.history.state && window.history.state.back) {
      router.back()
    } else {
      const q = { ...route.query }
      delete q.item
      delete q.openTarget
      router.push({ query: q })
    }
  }
}

async function onDetailUpdated() {
  await Promise.all([loadIssues(), loadWorkspace(), refreshProjects()])
  if (detailModalState.targetType === 'task' && detailModalState.task?.planned_end_date && autoScheduleEnabled.value) {
    await triggerAutoSchedule(detailModalState.task.id, detailModalState.task.planned_end_date)
    await loadIssues()
  }
  if (detailModalState.targetType === 'issue' && detailModalState.issue) {
    const updated = issues.value.find(i => i.id === detailModalState.issue?.id)
    if (updated) detailModalState.issue = updated
  } else if (detailModalState.targetType === 'task' && detailModalState.task) {
    let found = false
    for (const iss of issues.value) {
      const x = (iss.tasks || []).find(t => t.id === detailModalState.task?.id)
      if (x) {
        detailModalState.task = x
        detailModalState.parentIssueTitle = iss.title
        found = true
        break
      }
    }
    if (!found) {
      try {
        const fetched = await apiFetch<Task>(`/projects/${projectId.value}/tasks/${detailModalState.task.id}`)
        if (fetched) {
          detailModalState.task = fetched
          if (fetched.issue_id && !detailModalState.parentIssueTitle) {
            const pIss = issues.value.find(i => i.id === fetched.issue_id) || await apiFetch<Issue>(`/projects/${projectId.value}/issues/${fetched.issue_id}`).catch(() => null)
            if (pIss) detailModalState.parentIssueTitle = pIss.title
          }
        }
      } catch (e) {}
    }
  }
}

// Project Settings Actions
async function saveProjectSettings() {
  if (!hasAtMostTwoDecimals(projForm.hours_per_day)) {
    notifyToast('1日の基準時間は小数点第2位までで入力してください', 'error')
    return
  }
  try {
    await apiFetch(`/projects/${projectId.value}`, {
      method: 'PUT',
      body: JSON.stringify(projForm)
    })
    await Promise.all([loadWorkspace(), refreshProjects()])
    notifyToast('プロジェクト基本設定を保存しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

const adminMembersCount = computed(() => members.value.filter(m => m.role === 'admin').length)

function isLastAdmin(m: ProjectMember): boolean {
  return m.role === 'admin' && adminMembersCount.value <= 1
}

function getMemberSubjectName(m: ProjectMember): string {
  return m.subject_type === 'user' ? getUserName(m.subject_id) : getGroupName(m.subject_id)
}

async function updateMemberRole(m: ProjectMember, newRole: 'admin' | 'editor' | 'viewer') {
  if (m.role === newRole) return
  if (isLastAdmin(m) && newRole !== 'admin') {
    notifyToast('最後の管理者は降格できません', 'error')
    return
  }
  try {
    await apiFetch(`/projects/${projectId.value}/members`, {
      method: 'POST',
      body: JSON.stringify({
        subject_type: m.subject_type,
        subject_id: m.subject_id,
        role: newRole
      })
    })
    await loadWorkspace()
    notifyToast(`「${getMemberSubjectName(m)}」の権限を更新しました`, 'success')
  } catch (e: any) {
    await loadWorkspace()
    notifyToast(e.message, 'error')
  }
}

async function addProjectMember() {
  const [subject_type, subject_id] = memberForm.subject_combined.split(':')
  if (!subject_type || !subject_id) return
  try {
    await apiFetch(`/projects/${projectId.value}/members`, {
      method: 'POST',
      body: JSON.stringify({
        subject_type,
        subject_id,
        role: memberForm.role
      })
    })
    memberForm.subject_combined = ''
    await loadWorkspace()
    notifyToast('メンバー権限を更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function removeProjectMember(m: ProjectMember) {
  if (isLastAdmin(m)) {
    notifyToast('最後の管理者は削除できません', 'error')
    return
  }
  const name = getMemberSubjectName(m)
  if (!confirm(`「${name}」をプロジェクトメンバーから解除しますか？`)) return
  try {
    await apiFetch(`/projects/${projectId.value}/members/${m.id}`, { method: 'DELETE' })
    await loadWorkspace()
    notifyToast(`「${name}」を解除しました`, 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createStatus() {
  try {
    await apiFetch(`/projects/${projectId.value}/statuses`, {
      method: 'POST',
      body: JSON.stringify(statusForm)
    })
    statusForm.name = ''
    await loadWorkspace()
    notifyToast('ステータスを追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteStatus(statusId: string) {
  const migrateTo = migrateTargetMap[statusId] || ''
  const query = migrateTo ? `?migrate_to_status_id=${migrateTo}` : ''
  try {
    await apiFetch(`/projects/${projectId.value}/statuses/${statusId}${query}`, { method: 'DELETE' })
    await loadWorkspace()
    notifyToast('ステータスを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createProjectLabel() {
  try {
    await apiFetch(`/projects/${projectId.value}/labels`, {
      method: 'POST',
      body: JSON.stringify(labelForm)
    })
    labelForm.name = ''
    await loadWorkspace()
    notifyToast('ラベルを追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteProjectLabel(labelId: string) {
  try {
    await apiFetch(`/projects/${projectId.value}/labels/${labelId}`, { method: 'DELETE' })
    await loadWorkspace()
    notifyToast('ラベルを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createProjectType() {
  if (!projectTypeForm.name.trim()) return
  try {
    await apiFetch(`/projects/${projectId.value}/types`, {
      method: 'POST',
      body: JSON.stringify({
        name: projectTypeForm.name.trim(),
        color: projectTypeForm.color,
        display_order: projectTypeForm.display_order
      })
    })
    projectTypeForm.name = ''
    await loadWorkspace()
    notifyToast('プロジェクト種別を追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function openEditProjectTypeModal(tp: ItemType) {
  editingProjectType.value = tp
  editProjectTypeForm.name = tp.name
  editProjectTypeForm.color = tp.color
  editProjectTypeForm.display_order = tp.display_order
}

async function updateProjectType() {
  if (!editingProjectType.value || !editProjectTypeForm.name.trim()) return
  try {
    await apiFetch(`/projects/${projectId.value}/types/${editingProjectType.value.id}`, {
      method: 'PUT',
      body: JSON.stringify({
        name: editProjectTypeForm.name.trim(),
        color: editProjectTypeForm.color,
        display_order: editProjectTypeForm.display_order
      })
    })
    editingProjectType.value = null
    await loadWorkspace()
    notifyToast('プロジェクト種別を更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteProjectType(tp: ItemType) {
  if (!confirm(`種別「${tp.name}」を削除してもよろしいですか？`)) return
  try {
    await apiFetch(`/projects/${projectId.value}/types/${tp.id}`, { method: 'DELETE' })
    await loadWorkspace()
    notifyToast('プロジェクト種別を削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createCustomField() {
  const options = cfForm.options_csv
    .split(',')
    .map(s => s.trim())
    .filter(Boolean)
  try {
    await apiFetch(`/projects/${projectId.value}/custom-fields`, {
      method: 'POST',
      body: JSON.stringify({
        target_type: cfForm.target_type,
        name: cfForm.name,
        field_type: cfForm.field_type,
        options
      })
    })
    cfForm.target_type = 'issue'
    cfForm.name = ''
    cfForm.options_csv = ''
    await loadWorkspace()
    notifyToast('カスタム属性を追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteCustomField(fieldId: string) {
  try {
    await apiFetch(`/projects/${projectId.value}/custom-fields/${fieldId}`, { method: 'DELETE' })
    await loadWorkspace()
    notifyToast('カスタム属性を削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function saveProjectTemplate(payload: { id?: string; target_type: TargetType; name: string; rows: TemplateRow[] }) {
  try {
    if (payload.id) {
      await apiFetch(`/projects/${projectId.value}/templates/${payload.id}`, {
        method: 'PUT',
        body: JSON.stringify({ name: payload.name, rows: payload.rows })
      })
    } else {
      await apiFetch(`/projects/${projectId.value}/templates`, {
        method: 'POST',
        body: JSON.stringify(payload)
      })
    }
    await loadWorkspace()
    notifyToast('テンプレートを保存しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteProjectTemplate(tplId: string) {
  try {
    await apiFetch(`/projects/${projectId.value}/templates/${tplId}`, { method: 'DELETE' })
    await loadWorkspace()
    notifyToast('テンプレートを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function getLabel(id: string): Label | undefined {
  return allLabels.value.find(l => l.id === id)
}

function getStatusName(id: string): string {
  return statuses.value.find(s => s.id === id)?.name || '-'
}

function getUserName(id: string): string {
  return users.value.find(u => u.id === id)?.display_name || id.slice(0, 8)
}

function getGroupName(id: string): string {
  return allGroups.value.find(g => g.id === id)?.name || id.slice(0, 8)
}

// Webhook State & Methods
const projectWebhooks = ref<Webhook[]>([])
const showNewWebhookModal = ref(false)
const selectedWebhookForLogs = ref<Webhook | null>(null)
const webhookDeliveries = ref<WebhookDelivery[]>([])
const expandedDeliveryId = ref<string | null>(null)

const newWebhookForm = reactive({
  name: '',
  url: '',
  secret: '',
  events: ['task.created', 'task.updated', 'issue.created', 'comment.created']
})

const editingWebhook = ref<Webhook | null>(null)
const editingWebhookForm = reactive({
  name: '',
  url: '',
  secret: '',
  is_active: true,
  events: [] as string[]
})

function openEditWebhook(wh: Webhook) {
  editingWebhook.value = wh
  editingWebhookForm.name = wh.name
  editingWebhookForm.url = wh.url
  editingWebhookForm.secret = ''
  editingWebhookForm.is_active = wh.is_active
  editingWebhookForm.events = [...wh.events]
}

async function updateProjectWebhook() {
  if (!editingWebhook.value) return
  try {
    const payload: Record<string, any> = {
      name: editingWebhookForm.name,
      url: editingWebhookForm.url,
      events: editingWebhookForm.events,
      is_active: editingWebhookForm.is_active
    }
    if (editingWebhookForm.secret) {
      payload.secret = editingWebhookForm.secret
    }
    await apiFetch(`/projects/${projectId.value}/webhooks/${editingWebhook.value.id}`, {
      method: 'PUT',
      body: JSON.stringify(payload)
    })
    editingWebhook.value = null
    await loadProjectWebhooks()
    notifyToast('Webhook を更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function loadProjectWebhooks() {
  try {
    projectWebhooks.value = await apiFetch<Webhook[]>(`/projects/${projectId.value}/webhooks`)
  } catch (e: any) {
    // ignore
  }
}

async function createProjectWebhook() {
  try {
    await apiFetch(`/projects/${projectId.value}/webhooks`, {
      method: 'POST',
      body: JSON.stringify(newWebhookForm)
    })
    showNewWebhookModal.value = false
    newWebhookForm.name = ''
    newWebhookForm.url = ''
    newWebhookForm.secret = ''
    await loadProjectWebhooks()
    notifyToast('Webhook を登録しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function testProjectWebhook(webhookId: string) {
  try {
    const res = await apiFetch<WebhookDelivery>(`/projects/${projectId.value}/webhooks/${webhookId}/test`, {
      method: 'POST'
    })
    if (res.status_code && res.status_code >= 200 && res.status_code < 300) {
      notifyToast(`テスト送信成功 (HTTP ${res.status_code} / ${res.duration_ms}ms)`, 'success')
    } else {
      notifyToast(`テスト送信失敗 (${res.error_message || `HTTP ${res.status_code}`})`, 'error')
    }
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function openWebhookDeliveries(wh: Webhook) {
  selectedWebhookForLogs.value = wh
  expandedDeliveryId.value = null
  try {
    webhookDeliveries.value = await apiFetch<WebhookDelivery[]>(`/projects/${projectId.value}/webhooks/${wh.id}/deliveries`)
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteProjectWebhook(webhookId: string) {
  if (!confirm('この Webhook を削除しますか？')) return
  try {
    await apiFetch(`/projects/${projectId.value}/webhooks/${webhookId}`, { method: 'DELETE' })
    await loadProjectWebhooks()
    notifyToast('Webhook を削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

// CSV Export
async function exportProjectCSV() {
  try {
    const blob = await apiFetch<Blob>(`/projects/${projectId.value}/export/csv`)
    const url = window.URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    const pName = (project.value?.name || 'project').replace(/[^\w\s-]/gi, '_')
    const today = new Date().toISOString().slice(0, 10).replace(/-/g, '')
    a.download = `polaris_${pName}_${today}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    window.URL.revokeObjectURL(url)
    notifyToast('CSVをエクスポートしました', 'success')
  } catch (e: any) {
    notifyToast(e.message || 'CSVエクスポートに失敗しました', 'error')
  }
}

// Inbound Webhooks
async function loadInboundWebhooks() {
  try {
    inboundWebhooks.value = await apiFetch<InboundWebhook[]>(`/projects/${projectId.value}/inbound-webhooks`)
  } catch (e: any) {
    // ignore
  }
}

function openNewInboundWebhookModal() {
  newInboundWebhookForm.name = ''
  newInboundWebhookForm.default_issue_id = ''
  newInboundWebhookForm.default_status_id = undefined
  showNewInboundWebhookModal.value = true
}

async function createInboundWebhook() {
  try {
    const payload: { name: string; default_issue_id?: string; default_status_id?: number } = {
      name: newInboundWebhookForm.name
    }
    if (newInboundWebhookForm.default_issue_id) {
      payload.default_issue_id = newInboundWebhookForm.default_issue_id
    }
    if (newInboundWebhookForm.default_status_id) {
      payload.default_status_id = Number(newInboundWebhookForm.default_status_id)
    }
    const created = await apiFetch<InboundWebhook>(`/projects/${projectId.value}/inbound-webhooks`, {
      method: 'POST',
      body: JSON.stringify(payload)
    })
    showNewInboundWebhookModal.value = false
    createdInboundWebhook.value = created
    await loadInboundWebhooks()
    notifyToast('Inbound Webhook を作成しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteInboundWebhook(id: string) {
  if (!confirm('この Inbound Webhook を削除しますか？\n削除するとこのトークンを使用した自動起票はすべて拒否されます。')) return
  try {
    await apiFetch(`/inbound-webhooks/${id}`, { method: 'DELETE' })
    await loadInboundWebhooks()
    notifyToast('Inbound Webhook を削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function copyToClipboard(text: string, label: string = 'コピー') {
  try {
    await navigator.clipboard.writeText(text)
    notifyToast(`${label}しました`, 'success')
  } catch (e) {
    notifyToast('クリップボードへのコピーに失敗しました', 'error')
  }
}
</script>

