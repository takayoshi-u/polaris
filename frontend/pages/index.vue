<template>
  <div style="display: flex; flex-direction: column; gap: 20px;">
    <!-- Top Header & Action Toolbar -->
    <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px;">
      <div>
        <h1 style="font-size: 20px; font-weight: 700;">プロジェクト一覧</h1>
        <p style="font-size: 13px; color: var(--text-muted);">
          参加中のプロジェクト（または削除権限保持プロジェクト）の一覧です。
        </p>
      </div>

      <button
        v-if="hasSystemPerm('create_project')"
        type="button"
        class="btn btn-primary"
        style="display: flex; align-items: center; gap: 6px; font-weight: 700;"
        @click="showCreateProjectModal = true"
      >
        <span>＋</span>
        <span>新規プロジェクト作成</span>
      </button>
    </div>

    <!-- Filter & Search Toolbar (P-5) -->
    <div class="card" style="padding: 12px 16px; display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px;">
      <!-- Search Input -->
      <div style="display: flex; align-items: center; gap: 8px; flex: 1 1 260px; max-width: 400px;">
        <span style="font-size: 14px; color: var(--text-muted);">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="プロジェクト名・説明文で検索..."
          style="width: 100%; font-size: 13px; padding: 6px 10px;"
        />
      </div>

      <!-- Role Filter Tabs -->
      <div style="display: flex; align-items: center; gap: 4px; padding: 3px; border-radius: 8px; background: var(--bg-main); border: 1px solid var(--border-color);">
        <button
          v-for="roleTab in roleTabOptions"
          :key="roleTab.value"
          type="button"
          class="btn btn-sm"
          :class="selectedRoleFilter === roleTab.value ? 'btn-primary' : 'btn-ghost'"
          style="padding: 3px 10px; font-size: 12px;"
          @click="selectedRoleFilter = roleTab.value"
        >
          {{ roleTab.label }}
        </button>
      </div>
    </div>

    <div v-if="filteredProjects.length === 0" class="card" style="text-align: center; padding: 48px 20px;">
      <p style="font-size: 14px; font-weight: 600; margin-bottom: 6px;">条件に一致するプロジェクトがありません</p>
      <p style="font-size: 12px; color: var(--text-muted);">
        検索条件を変更するか、右上の「＋ 新規プロジェクト作成」ボタンから新しいプロジェクトを作成してください。
      </p>
    </div>

    <!-- Projects Grid -->
    <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px;">
      <div
        v-for="p in filteredProjects"
        :key="p.id"
        class="card"
        style="display: flex; flex-direction: column; justify-content: space-between; gap: 14px;"
      >
        <div>
          <div style="display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; margin-bottom: 6px;">
            <div>
              <span style="font-size: 16px; font-weight: 700;">{{ p.name }}</span>
              <div v-if="projectStatsMap[p.id]?.overdueCount" style="margin-top: 4px;">
                <span class="badge badge-amber" style="color: var(--danger); border-color: var(--danger); font-size: 11px; font-weight: 700;">
                  ⚠️ 遅延タスク: {{ projectStatsMap[p.id].overdueCount }}件
                </span>
              </div>
            </div>
            <span :class="['badge', p.is_member ? 'badge-blue' : 'badge-amber']" style="flex-shrink: 0;">
              {{ p.is_member ? `Role: ${p.current_user_role.toUpperCase()}` : '未参加（削除権限のみ）' }}
            </span>
          </div>
          <p style="font-size: 13px; color: var(--text-muted); min-height: 38px; line-height: 1.5;">
            {{ p.description || '説明なし' }}
          </p>

          <!-- Weighted Progress Bar (P-5) -->
          <div v-if="p.is_member && projectStatsMap[p.id]" style="margin-top: 10px; display: flex; flex-direction: column; gap: 4px;">
            <div style="display: flex; justify-content: space-between; font-size: 12px; color: var(--text-secondary);">
              <span>進捗率 (加重平均)</span>
              <strong>{{ projectStatsMap[p.id].weightedProgress }}%</strong>
            </div>
            <div style="height: 6px; border-radius: 999px; background: var(--border-color); overflow: hidden;">
              <div
                :style="{
                  width: `${projectStatsMap[p.id].weightedProgress}%`,
                  height: '100%',
                  backgroundColor: projectStatsMap[p.id].overdueCount ? 'var(--warning)' : 'var(--primary)'
                }"
              ></div>
            </div>
          </div>
        </div>

        <div style="display: flex; align-items: center; justify-content: space-between; border-top: 1px solid var(--border-color); padding-top: 12px;">
          <div style="display: flex; flex-direction: column; gap: 2px; font-size: 12px; color: var(--text-secondary);">
            <span>1日基準: <strong>{{ formatHours(p.hours_per_day) }}h</strong></span>
            <span>コスト(予/実): <strong>{{ formatCost(p.total_planned_cost) }} / {{ formatCost(p.total_actual_cost) }}</strong></span>
          </div>
          <div style="display: flex; gap: 8px;">
            <button
              v-if="hasSystemPerm('delete_project')"
              class="btn btn-danger btn-sm"
              @click="openDeleteSafetyModal(p)"
            >
              削除
            </button>
            <NuxtLink
              v-if="p.is_member"
              :to="`/projects/${p.id}`"
              class="btn btn-primary btn-sm"
              style="text-decoration: none;"
            >
              開く →
            </NuxtLink>
          </div>
        </div>
      </div>
    </div>

    <!-- Create Project Modal (P-9) -->
    <div v-if="showCreateProjectModal" class="modal-backdrop" @click.self="showCreateProjectModal = false">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 14px;">新規プロジェクト作成</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitCreateProject">
          <div>
            <label class="field-label">プロジェクト名 *</label>
            <input v-model="newProjForm.name" type="text" required style="width: 100%;" placeholder="例: 次世代EC基盤開発" />
          </div>
          <div>
            <label class="field-label">説明</label>
            <textarea v-model="newProjForm.description" rows="3" style="width: 100%;" placeholder="プロジェクトの目的や概要"></textarea>
          </div>
          <div>
            <label class="field-label">1日あたりの基準時間 (h)</label>
            <input v-model.number="newProjForm.hours_per_day" type="number" min="1" max="24" step="0.01" required style="width: 100%;" />
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
            <button type="button" class="btn btn-secondary" :disabled="isCreatingProject" @click="showCreateProjectModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary" :disabled="isCreatingProject">
              {{ isCreatingProject ? '⏳ 作成中...' : '作成して開く' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Safety Confirmation Modal with Key/Name Input & Impact Summary (A-2) -->
    <div v-if="deleteSafetyProject" class="modal-backdrop" @click.self="deleteSafetyProject = null">
      <div class="modal-panel" style="max-width: 500px;">
        <h3 style="font-size: 16px; font-weight: 700; color: var(--danger); margin-bottom: 10px;">
          ⚠️ プロジェクトの安全削除確認
        </h3>
        <div style="font-size: 13px; line-height: 1.6; display: flex; flex-direction: column; gap: 10px; margin-bottom: 16px;">
          <p>
            プロジェクト「<strong>{{ deleteSafetyProject.name }}</strong>」を削除しようとしています。<br />
            この操作は取り消すことができず、配下のすべてのデータが完全に削除されます。
          </p>
          <!-- Impact Summary (A-2) -->
          <div style="padding: 10px 12px; border-radius: 6px; background: var(--bg-main); border: 1px solid var(--border-color);">
            <div style="font-weight: 700; margin-bottom: 4px; font-size: 12px; color: var(--text-secondary);">削除されるデータの影響サマリー:</div>
            <ul style="padding-left: 20px; font-size: 12px; color: var(--text-main);">
              <li>登録課題数: <strong>{{ deleteImpactSummary.issueCount }}</strong> 件</li>
              <li>登録タスク数: <strong>{{ deleteImpactSummary.taskCount }}</strong> 件</li>
              <li>関連するコメント、添付ファイル、ステータス設定、Webhooks等すべて</li>
            </ul>
          </div>
          <div>
            <label class="field-label" style="color: var(--danger);">
              安全確認のため、プロジェクト名「<strong>{{ deleteSafetyProject.name }}</strong>」を入力してください:
            </label>
            <input
              v-model="deleteSafetyInput"
              type="text"
              style="width: 100%; font-size: 13px;"
              placeholder="プロジェクト名を入力"
            />
          </div>
        </div>
        <div style="display: flex; justify-content: flex-end; gap: 8px;">
          <button class="btn btn-secondary" @click="deleteSafetyProject = null">キャンセル</button>
          <button
            class="btn btn-danger"
            :disabled="deleteSafetyInput !== deleteSafetyProject.name"
            @click="executeSafetyDelete"
          >
            プロジェクトを完全に削除
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Issue, Project } from '~/types/polaris'
import { formatCost, formatHours } from '~/utils/format'

const router = useRouter()
const { apiFetch, projects, refreshProjects, notifyToast, hasSystemPerm } = useApi()

// P-5 Search & Filter State
const searchQuery = ref('')
const selectedRoleFilter = ref<string>('all')

const roleTabOptions = [
  { label: 'すべて', value: 'all' },
  { label: 'Admin', value: 'admin' },
  { label: 'Editor', value: 'editor' },
  { label: 'Viewer', value: 'viewer' },
  { label: '未参加', value: 'non_member' }
]

const filteredProjects = computed(() => {
  return projects.value.filter((p) => {
    // Role filter
    if (selectedRoleFilter.value === 'non_member') {
      if (p.is_member) return false
    } else if (selectedRoleFilter.value !== 'all') {
      if (!p.is_member || p.current_user_role !== selectedRoleFilter.value) return false
    }

    // Search query
    if (searchQuery.value.trim()) {
      const q = searchQuery.value.trim().toLowerCase()
      const matchName = p.name.toLowerCase().includes(q)
      const matchDesc = (p.description || '').toLowerCase().includes(q)
      if (!matchName && !matchDesc) return false
    }
    return true
  })
})

// P-5 Weighted Progress & Overdue Stats
interface ProjectStats {
  issueCount: number
  taskCount: number
  weightedProgress: number
  overdueCount: number
}
const projectStatsMap = ref<Record<string, ProjectStats>>({})

async function loadProjectsStats() {
  const todayStr = new Date().toISOString().slice(0, 10)
  const promises = projects.value
    .filter(p => p.is_member)
    .map(async (p) => {
      try {
        const issues = await apiFetch<Issue[]>(`/projects/${p.id}/issues`)
        let totalWeight = 0
        let totalProg = 0
        let taskCount = 0
        let overdueCount = 0

        for (const iss of issues || []) {
          for (const t of iss.tasks || []) {
            taskCount++
            const w = t.planned_hours || 1
            totalWeight += w
            totalProg += (t.progress || 0) * w
            if (t.planned_end_date && t.planned_end_date < todayStr && (t.progress || 0) < 100) {
              overdueCount++
            }
          }
        }
        const weighted = totalWeight > 0 ? totalProg / totalWeight : 0
        projectStatsMap.value[p.id] = {
          issueCount: issues?.length || 0,
          taskCount,
          weightedProgress: Math.min(100, Math.round(weighted)),
          overdueCount
        }
      } catch {
        // ignore
      }
    })
  await Promise.all(promises)
}

watch(projects, () => {
  loadProjectsStats()
}, { immediate: true })

// P-9 Create Project Modal
const showCreateProjectModal = ref(false)
const isCreatingProject = ref(false)
const newProjForm = reactive({
  name: '',
  description: '',
  hours_per_day: 8.0
})

async function submitCreateProject() {
  if (isCreatingProject.value) return
  if (!newProjForm.name.trim()) {
    notifyToast('プロジェクト名を入力してください', 'error')
    return
  }
  isCreatingProject.value = true
  try {
    const created = await apiFetch<Project>('/projects', {
      method: 'POST',
      body: JSON.stringify({
        ...newProjForm,
        name: newProjForm.name.trim()
      })
    })
    showCreateProjectModal.value = false
    newProjForm.name = ''
    newProjForm.description = ''
    newProjForm.hours_per_day = 8.0
    await refreshProjects()
    notifyToast(`プロジェクト「${created.name}」を作成しました`, 'success')
    router.push(`/projects/${created.id}`)
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    isCreatingProject.value = false
  }
}

// A-2 Safety Confirmation Modal with Project Name Input & Impact Summary
const deleteSafetyProject = ref<Project | null>(null)
const deleteSafetyInput = ref('')
const deleteImpactSummary = reactive({
  issueCount: 0,
  taskCount: 0
})

async function openDeleteSafetyModal(p: Project) {
  deleteSafetyProject.value = p
  deleteSafetyInput.value = ''
  deleteImpactSummary.issueCount = projectStatsMap.value[p.id]?.issueCount || 0
  deleteImpactSummary.taskCount = projectStatsMap.value[p.id]?.taskCount || 0

  // If stats not loaded or non-member, try loading issues count
  try {
    const issues = await apiFetch<Issue[]>(`/projects/${p.id}/issues`)
    let tCount = 0
    for (const iss of issues || []) {
      tCount += (iss.tasks || []).length
    }
    deleteImpactSummary.issueCount = issues?.length || 0
    deleteImpactSummary.taskCount = tCount
  } catch {
    // ignore
  }
}

async function executeSafetyDelete() {
  if (!deleteSafetyProject.value) return
  const p = deleteSafetyProject.value
  try {
    const query = !p.is_member ? '?confirm_non_member=true' : ''
    await apiFetch(`/projects/${p.id}${query}`, { method: 'DELETE' })
    deleteSafetyProject.value = null
    await refreshProjects()
    notifyToast(`プロジェクト「${p.name}」を削除しました`, 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}
</script>

