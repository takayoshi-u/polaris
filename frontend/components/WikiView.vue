<template>
  <div class="card" style="display: flex; flex-direction: column; gap: 0; padding: 0; min-height: 600px; border-radius: 8px; overflow: hidden;">
    <!-- Top Action Toolbar -->
    <div style="padding: 10px 16px; border-bottom: 1px solid var(--border-color); background: var(--bg-elevated); display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px;">
      <div style="display: flex; align-items: center; gap: 8px;">
        <h2 style="font-size: 15px; font-weight: 700; display: flex; align-items: center; gap: 6px; margin: 0;">
          <span>📖 プロジェクト Wiki</span>
          <span class="badge badge-slate" style="font-size: 11px;">{{ flatPages.length }}ページ</span>
        </h2>
      </div>

      <div style="display: flex; align-items: center; gap: 8px;">
        <button
          v-if="canEdit"
          type="button"
          class="btn btn-primary btn-sm"
          @click="startNewPage(null)"
        >
          ＋ 新規ページ作成
        </button>
      </div>
    </div>

    <!-- Main 2-Pane Layout -->
    <div style="display: grid; grid-template-columns: 280px 1fr; flex: 1; min-height: 550px;">
      <!-- Left Pane: Page Tree & Search -->
      <div style="border-right: 1px solid var(--border-color); background: var(--bg-card); display: flex; flex-direction: column; gap: 10px; padding: 12px;">
        <!-- Search Filter -->
        <input
          v-model="searchQuery"
          type="text"
          placeholder="ページを検索..."
          style="width: 100%; font-size: 12px; padding: 5px 8px;"
        />

        <!-- Page Tree List -->
        <div style="flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 2px;">
          <div v-if="filteredPages.length === 0" style="padding: 20px 8px; text-align: center; color: var(--text-muted); font-size: 12px;">
            {{ searchQuery ? '該当するページがありません' : 'ページがまだありません' }}
          </div>

          <template v-for="item in filteredPages" :key="item.page.id">
            <div
              :style="{
                padding: '6px 8px',
                paddingLeft: `${8 + item.depth * 14}px`,
                borderRadius: '6px',
                cursor: 'pointer',
                fontSize: '13px',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                gap: '6px',
                backgroundColor: selectedPageId === item.page.id ? 'var(--bg-elevated)' : 'transparent',
                fontWeight: selectedPageId === item.page.id ? '700' : 'normal',
                color: selectedPageId === item.page.id ? 'var(--primary)' : 'var(--text-main)'
              }"
              @click="selectPage(item.page.id)"
            >
              <div style="display: flex; align-items: center; gap: 6px; overflow: hidden;">
                <span style="color: var(--text-muted); font-size: 12px;">📄</span>
                <span style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">{{ item.page.title }}</span>
              </div>

              <!-- Quick subpage add -->
              <button
                v-if="canEdit"
                type="button"
                class="btn btn-ghost btn-sm"
                style="padding: 1px 4px; font-size: 10px; opacity: 0.6;"
                title="子ページを追加"
                @click.stop="startNewPage(item.page.id)"
              >
                ＋
              </button>
            </div>
          </template>
        </div>
      </div>

      <!-- Right Pane: Viewer / Editor -->
      <div style="display: flex; flex-direction: column; background: var(--bg-main); overflow-y: auto;">
        <!-- Empty State -->
        <div v-if="!currentPage && !isCreating" style="padding: 60px 20px; text-align: center; color: var(--text-muted);">
          <p style="font-size: 15px; font-weight: 600;">ページを選択するか、新しく作成してください</p>
        </div>

        <!-- Mode: View or Edit -->
        <div v-else style="padding: 20px 24px; display: flex; flex-direction: column; gap: 16px;">
          <!-- Top Page Meta & Mode Toggle -->
          <div style="display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 12px;">
            <div style="display: flex; flex-direction: column; gap: 4px;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <span v-if="currentPage?.slug" class="badge badge-slate" style="font-size: 11px;">/{{ currentPage.slug }}</span>
                <span v-if="currentPage?.parent_id" class="badge badge-blue" style="font-size: 11px;">
                  親: {{ getParentTitle(currentPage.parent_id) }}
                </span>
                <span v-if="currentPage?.updated_at" style="font-size: 11px; color: var(--text-muted);">
                  最終更新: {{ formatRelativeTime(currentPage.updated_at) }}
                  <span v-if="currentPage.updated_by">({{ getUserName(currentPage.updated_by) }})</span>
                </span>
              </div>
              <h1 v-if="!isEditing && !isCreating" style="font-size: 22px; font-weight: 700; margin: 0;">
                {{ currentPage?.title }}
              </h1>
            </div>

            <!-- Header Action Buttons -->
            <div style="display: flex; align-items: center; gap: 8px;">
              <template v-if="!isEditing && !isCreating">
                <button
                  v-if="canEdit"
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="startEditCurrentPage"
                >
                  ✏️ 編集
                </button>
                <button
                  v-if="canEdit"
                  type="button"
                  class="btn btn-danger btn-sm"
                  @click="deleteCurrentPage"
                >
                  🗑 削除
                </button>
              </template>
              <template v-else>
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  @click="cancelEdit"
                >
                  キャンセル
                </button>
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="isSaving"
                  @click="savePage"
                >
                  {{ isSaving ? '保存中...' : 'ページを保存' }}
                </button>
              </template>
            </div>
          </div>

          <!-- VIEW MODE -->
          <div
            v-if="!isEditing && !isCreating && currentPage"
            class="wiki-content-view markdown-body"
            style="min-height: 300px; padding: 4px 0;"
            v-html="renderedWikiHtml"
            @click="handleContentClick"
          ></div>

          <!-- EDIT / CREATE MODE -->
          <div v-else style="display: flex; flex-direction: column; gap: 14px;">
            <!-- Title & Slug Input -->
            <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 12px;">
              <div>
                <label class="field-label">ページタイトル *</label>
                <input
                  v-model="pageForm.title"
                  type="text"
                  required
                  placeholder="例: プロジェクト仕様書 / 開発ガイドライン"
                  style="width: 100%; font-size: 14px; font-weight: 600;"
                  @input="onTitleInput"
                />
              </div>
              <div>
                <label class="field-label">スラッグ (URL用英数)</label>
                <input
                  v-model="pageForm.slug"
                  type="text"
                  placeholder="例: dev-guide"
                  style="width: 100%; font-family: monospace;"
                />
              </div>
            </div>

            <!-- Parent Page Selector -->
            <div>
              <label class="field-label">親ページ</label>
              <select v-model="pageForm.parent_id" style="width: 100%; max-width: 360px;">
                <option :value="null">(ルート階層 / 親なし)</option>
                <option
                  v-for="p in availableParentPages"
                  :key="p.id"
                  :value="p.id"
                >
                  {{ p.title }}
                </option>
              </select>
            </div>

            <!-- Editor / Preview Tabs -->
            <div style="display: flex; flex-direction: column; gap: 8px;">
              <div style="display: flex; align-items: center; justify-content: space-between;">
                <div style="display: flex; gap: 4px; padding: 2px; border-radius: 6px; background: var(--bg-card); border: 1px solid var(--border-color);">
                  <button
                    type="button"
                    class="btn btn-sm"
                    :class="editorTab === 'write' ? 'btn-primary' : 'btn-ghost'"
                    style="font-size: 12px;"
                    @click="editorTab = 'write'"
                  >
                    ✏️ 編集 (Markdown)
                  </button>
                  <button
                    type="button"
                    class="btn btn-sm"
                    :class="editorTab === 'preview' ? 'btn-primary' : 'btn-ghost'"
                    style="font-size: 12px;"
                    @click="editorTab = 'preview'"
                  >
                    👁 プレビュー (タスク埋込展開)
                  </button>
                </div>

                <div style="font-size: 11px; color: var(--text-muted);">
                  💡 <code style="padding: 1px 4px; background: var(--bg-input); border-radius: 3px;">task:UUID</code> または <code style="padding: 1px 4px; background: var(--bg-input); border-radius: 3px;">#タスク名</code> でタスクカードを埋め込めます
                </div>
              </div>

              <!-- Write Mode Textarea -->
              <div v-show="editorTab === 'write'">
                <textarea
                  v-model="pageForm.content"
                  rows="18"
                  style="width: 100%; font-family: monospace; font-size: 13px; line-height: 1.6; padding: 12px;"
                  placeholder="# プロジェクトWiki&#10;&#10;ここにMarkdown形式でドキュメントを入力...&#10;&#10;関連タスク: task:xxxx-xxxx または #タスク名"
                ></textarea>
              </div>

              <!-- Preview Mode -->
              <div
                v-show="editorTab === 'preview'"
                class="wiki-content-view markdown-body"
                style="min-height: 360px; padding: 14px; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 6px;"
                v-html="renderedEditorPreviewHtml"
                @click="handleContentClick"
              ></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Status, Task, User, WikiPage } from '~/types/polaris'
import { formatRelativeTime, renderWikiMarkdownWithTaskEmbeds } from '~/utils/format'

const props = defineProps<{
  projectId: string
  tasks: Task[]
  statuses: Status[]
  users: User[]
  canEdit: boolean
}>()

const emit = defineEmits<{
  (e: 'open-task', task: Task): void
  (e: 'refresh'): void
}>()

const { apiFetch, notifyToast } = useApi()

const wikiPages = ref<WikiPage[]>([])
const selectedPageId = ref<string | null>(null)
const searchQuery = ref('')

const isEditing = ref(false)
const isCreating = ref(false)
const isSaving = ref(false)
const editorTab = ref<'write' | 'preview'>('write')

const pageForm = reactive({
  title: '',
  slug: '',
  content: '',
  parent_id: null as string | null,
  sort_order: 0
})

const statusMap = computed(() => {
  const map = new Map<string, { name: string; color: string }>()
  for (const s of props.statuses) {
    map.set(s.id, { name: s.name, color: s.color })
  }
  return map
})

// Flattened list helper for tree hierarchy display
interface TreeFlattenedItem {
  page: WikiPage
  depth: number
}

function flattenWikiTree(pages: WikiPage[], depth = 0): TreeFlattenedItem[] {
  const items: TreeFlattenedItem[] = []
  for (const p of pages) {
    items.push({ page: p, depth })
    if (p.children && p.children.length > 0) {
      items.push(...flattenWikiTree(p.children, depth + 1))
    }
  }
  return items
}

const flatPages = computed(() => {
  return flattenWikiTree(wikiPages.value)
})

const filteredPages = computed(() => {
  if (!searchQuery.value.trim()) return flatPages.value
  const q = searchQuery.value.toLowerCase().trim()
  return flatPages.value.filter(
    item =>
      item.page.title.toLowerCase().includes(q) ||
      (item.page.slug && item.page.slug.toLowerCase().includes(q))
  )
})

const currentPage = computed(() => {
  if (!selectedPageId.value) return null
  return findPageById(wikiPages.value, selectedPageId.value)
})

const availableParentPages = computed(() => {
  const all: WikiPage[] = []
  function collect(list: WikiPage[]) {
    for (const p of list) {
      // Exclude self if editing
      if (!currentPage.value || p.id !== currentPage.value.id) {
        all.push(p)
      }
      if (p.children) collect(p.children)
    }
  }
  collect(wikiPages.value)
  return all
})

function findPageById(list: WikiPage[], id: string): WikiPage | null {
  for (const p of list) {
    if (p.id === id) return p
    if (p.children) {
      const found = findPageById(p.children, id)
      if (found) return found
    }
  }
  return null
}

function getParentTitle(parentId: string): string {
  const p = findPageById(wikiPages.value, parentId)
  return p ? p.title : parentId.slice(0, 8)
}

function getUserName(userId?: string | null): string {
  if (!userId) return '-'
  return props.users.find(u => u.id === userId)?.display_name || userId.slice(0, 8)
}

// Markdown Rendering with Task Embeds
const renderedWikiHtml = computed(() => {
  if (!currentPage.value) return ''
  return renderWikiMarkdownWithTaskEmbeds(
    currentPage.value.content,
    props.tasks,
    statusMap.value
  )
})

const renderedEditorPreviewHtml = computed(() => {
  return renderWikiMarkdownWithTaskEmbeds(
    pageForm.content,
    props.tasks,
    statusMap.value
  )
})

function handleContentClick(event: MouseEvent) {
  const target = (event.target as HTMLElement).closest('.wiki-task-pill')
  if (target) {
    event.preventDefault()
    const taskId = target.getAttribute('data-task-id')
    if (taskId) {
      const found = props.tasks.find(t => t.id === taskId)
      if (found) {
        emit('open-task', found)
      } else {
        notifyToast(`タスク (ID: ${taskId}) が見つかりませんでした`, 'warning')
      }
    }
  }
}

// API interactions
async function loadWikiPages() {
  if (!props.projectId) return
  try {
    const list = await apiFetch<WikiPage[]>(`/projects/${props.projectId}/wiki`)
    wikiPages.value = list || []
    if (!selectedPageId.value && wikiPages.value.length > 0) {
      selectedPageId.value = wikiPages.value[0].id
    }
  } catch (e: any) {
    wikiPages.value = []
  }
}

watch(() => props.projectId, () => loadWikiPages(), { immediate: true })

function selectPage(pageId: string) {
  if (isEditing.value || isCreating.value) {
    if (!confirm('編集中です。変更を破棄して別のページへ移動しますか？')) return
  }
  isEditing.value = false
  isCreating.value = false
  selectedPageId.value = pageId
}

function startNewPage(parentId: string | null = null) {
  isCreating.value = true
  isEditing.value = false
  pageForm.title = ''
  pageForm.slug = ''
  pageForm.content = ''
  pageForm.parent_id = parentId
  pageForm.sort_order = flatPages.value.length + 1
  editorTab.value = 'write'
}

function startEditCurrentPage() {
  if (!currentPage.value) return
  isEditing.value = true
  isCreating.value = false
  pageForm.title = currentPage.value.title
  pageForm.slug = currentPage.value.slug || ''
  pageForm.content = currentPage.value.content || ''
  pageForm.parent_id = currentPage.value.parent_id || null
  pageForm.sort_order = currentPage.value.sort_order || 0
  editorTab.value = 'write'
}

function cancelEdit() {
  isEditing.value = false
  isCreating.value = false
}

function onTitleInput() {
  if (isCreating.value && !pageForm.slug) {
    pageForm.slug = pageForm.title
      .toLowerCase()
      .replace(/[^a-z0-9_-]/g, '-')
      .replace(/-+/g, '-')
      .replace(/^-|-$/g, '')
  }
}

async function savePage() {
  if (!pageForm.title.trim()) {
    notifyToast('タイトルを入力してください', 'error')
    return
  }
  isSaving.value = true
  try {
    if (isEditing.value && currentPage.value) {
      const updated = await apiFetch<WikiPage>(`/wiki/${currentPage.value.id}`, {
        method: 'PUT',
        body: JSON.stringify(pageForm)
      })
      notifyToast('Wikiページを更新しました', 'success')
      await loadWikiPages()
      selectedPageId.value = updated.id
    } else {
      const created = await apiFetch<WikiPage>(`/projects/${props.projectId}/wiki`, {
        method: 'POST',
        body: JSON.stringify(pageForm)
      })
      notifyToast('Wikiページを作成しました', 'success')
      await loadWikiPages()
      selectedPageId.value = created.id
    }
    isEditing.value = false
    isCreating.value = false
  } catch (e: any) {
    notifyToast(e.message || 'ページの保存に失敗しました', 'error')
  } finally {
    isSaving.value = false
  }
}

async function deleteCurrentPage() {
  if (!currentPage.value) return
  if (!confirm(`Wikiページ「${currentPage.value.title}」を削除しますか？`)) return
  try {
    await apiFetch(`/wiki/${currentPage.value.id}`, { method: 'DELETE' })
    notifyToast('ページを削除しました', 'success')
    selectedPageId.value = null
    await loadWikiPages()
  } catch (e: any) {
    notifyToast(e.message || 'ページの削除に失敗しました', 'error')
  }
}
</script>
