<template>
  <div v-if="isPublicRoute">
    <slot />
  </div>

  <div v-else class="app-shell">
    <!-- Left Navigation Sidebar -->
    <aside class="app-sidebar">
      <div style="display: flex; flex-direction: column; gap: 20px;">
        <!-- Brand Header -->
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <NuxtLink to="/" style="display: flex; align-items: center; gap: 10px; text-decoration: none; color: inherit;">
            <div style="width: 34px; height: 34px; border-radius: 8px; background: #2563eb; color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; font-size: 16px;">
              P
            </div>
            <div>
              <div style="font-size: 16px; font-weight: 700;">Polaris</div>
              <div style="font-size: 12px; color: var(--text-muted);">Project Management</div>
            </div>
          </NuxtLink>
          <button
            v-if="hasSystemPerm('create_project')"
            class="btn btn-primary btn-sm"
            title="新規プロジェクト作成"
            @click="showNewProjectModal = true"
          >
            <Plus :size="14" /> 新規
          </button>
        </div>

        <!-- Current Project Selector -->
        <div>
          <div style="font-size: 12px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 6px;">
            プロジェクト切替
          </div>
          <select
            :value="activeProjectId"
            style="width: 100%; font-weight: 600;"
            @change="onSelectProject(($event.target as HTMLSelectElement).value)"
          >
            <option value="">-- プロジェクト一覧 --</option>
            <option v-for="p in projects" :key="p.id" :value="p.id">
              {{ p.name }} ({{ formatHours(p.hours_per_day) }}h/日・{{ p.is_member ? p.current_user_role : '未参加' }})
            </option>
          </select>

          <div
            v-if="activeProject"
            style="margin-top: 8px; padding: 10px; border-radius: 8px; background: var(--bg-card); border: 1px solid var(--border-color);"
          >
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 6px;">
              <span style="font-size: 14px; font-weight: 700;">{{ activeProject.name }}</span>
              <span :class="['badge', activeProject.is_member ? 'badge-blue' : 'badge-amber']">
                {{ activeProject.is_member ? activeProject.current_user_role.toUpperCase() : '未参加' }}
              </span>
            </div>
            <div style="font-size: 13px; color: var(--text-muted); margin-top: 4px;">
              基準時間: {{ formatHours(activeProject.hours_per_day) }}h / 日
            </div>
          </div>
        </div>

        <!-- Navigation Menu -->
        <nav style="display: flex; flex-direction: column; gap: 4px;">
          <div style="font-size: 12px; font-weight: 600; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 4px;">
            ナビゲーション
          </div>
          <NuxtLink
            to="/"
            class="btn btn-ghost"
            style="justify-content: flex-start; width: 100%; font-size: 14px;"
          >
            <FolderKanban :size="16" /> プロジェクト一覧
          </NuxtLink>
          <NuxtLink
            v-if="activeProject"
            :to="`/projects/${activeProject.id}`"
            class="btn btn-ghost"
            style="justify-content: flex-start; width: 100%; font-size: 14px;"
          >
            <LayoutGrid :size="16" /> ワークスペースを開く
          </NuxtLink>
          <NuxtLink
            v-if="hasSystemPerm('manage_system')"
            to="/admin"
            class="btn btn-ghost"
            style="justify-content: flex-start; width: 100%; font-size: 14px;"
          >
            <ShieldCheck :size="16" /> システム全体設定
            <span class="badge badge-amber" style="margin-left: auto; font-size: 11px;">Admin</span>
          </NuxtLink>
        </nav>
      </div>

      <!-- User Footer & Notification Bell -->
      <div v-if="currentUser" style="border-top: 1px solid var(--border-color); padding-top: 12px; display: flex; flex-direction: column; gap: 10px;">
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
          <div
            style="display: flex; align-items: center; gap: 8px; cursor: pointer; overflow: hidden;"
            title="アカウント設定を開く"
            @click="openAccountModal"
          >
            <div style="width: 34px; height: 34px; border-radius: 999px; background: #4f46e5; color: #fff; font-weight: 700; font-size: 13px; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
              {{ currentUser.display_name.slice(0, 2).toUpperCase() }}
            </div>
            <div style="overflow: hidden;">
              <div style="font-size: 13px; font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                {{ currentUser.display_name }}
              </div>
              <div style="font-size: 12px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                {{ currentUser.email }}
              </div>
            </div>
          </div>

          <div style="display: flex; align-items: center; gap: 4px; flex-shrink: 0;">
            <button ref="notificationBellRef" class="btn btn-secondary btn-sm" title="通知一覧" @click="toggleNotifications">
              <Bell :size="14" />
              <span>{{ unreadCount }}</span>
            </button>
            <button class="btn btn-ghost btn-sm" title="アカウント設定" @click="openAccountModal">
              <Settings :size="14" />
            </button>
          </div>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <div class="app-main">
      <!-- Notification Drawer Popover -->
      <div
        v-if="showNotifications"
        ref="notificationPopoverRef"
        class="card"
        style="position: fixed; bottom: 72px; left: 268px; width: 390px; max-height: 460px; overflow-y: auto; z-index: 150; box-shadow: 0 10px 25px rgba(0,0,0,0.15);"
      >
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; padding-bottom: 8px; border-bottom: 1px solid var(--border-color);">
          <span style="font-weight: 700; font-size: 13px;">🔔 通知一覧（未読 {{ unreadCount }}件）</span>
          <div style="display: flex; gap: 6px;">
            <button class="btn btn-secondary btn-sm" @click="markAllRead">すべて既読</button>
            <button class="btn btn-ghost btn-sm" @click="showNotifications = false">閉じる</button>
          </div>
        </div>

        <!-- Filter Tab (C-3) -->
        <div style="display: flex; gap: 4px; margin-bottom: 8px; border-bottom: 1px solid var(--border-color); padding-bottom: 6px;">
          <button
            type="button"
            class="btn btn-sm"
            :class="notificationFilter === 'all' ? 'btn-primary' : 'btn-secondary'"
            style="padding: 2px 8px; font-size: 11px;"
            @click="notificationFilter = 'all'"
          >
            すべて ({{ notifications.length }})
          </button>
          <button
            type="button"
            class="btn btn-sm"
            :class="notificationFilter === 'unread' ? 'btn-primary' : 'btn-secondary'"
            style="padding: 2px 8px; font-size: 11px;"
            @click="notificationFilter = 'unread'"
          >
            未読のみ ({{ unreadCount }})
          </button>
        </div>

        <div v-if="filteredNotifications.length === 0" style="font-size: 12px; color: var(--text-muted); padding: 16px 0; text-align: center;">
          {{ notificationFilter === 'unread' ? '未読の通知はありません' : '通知はありません' }}
        </div>
        <div style="display: flex; flex-direction: column; gap: 8px;">
          <div
            v-for="n in filteredNotifications"
            :key="n.id"
            style="padding: 10px; border-radius: 8px; border: 1px solid var(--border-color); cursor: pointer;"
            :style="{ backgroundColor: n.is_read ? 'var(--bg-card)' : 'rgba(37, 99, 235, 0.08)' }"
            @click="onClickNotification(n)"
          >
            <div style="display: flex; justify-content: space-between; align-items: center; font-size: 11px; color: var(--text-muted); margin-bottom: 4px;">
              <div style="display: flex; align-items: center; gap: 6px;">
                <span>{{ n.notification_type === 'mention' ? '💬 メンション' : '👤 アサイン' }}</span>
                <span v-if="!n.is_read" class="badge badge-blue" style="font-size: 10px; padding: 1px 5px;">未読</span>
              </div>
              <div style="display: flex; align-items: center; gap: 6px;">
                <span>{{ formatJSTDateTime(n.created_at) }}</span>
                <!-- Inline Mark Read Button (C-3) -->
                <button
                  v-if="!n.is_read"
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="padding: 1px 6px; font-size: 11px; color: var(--primary);"
                  title="この通知を既読にする"
                  @click.stop="markSingleRead(n)"
                >
                  ✓ 既読化
                </button>
              </div>
            </div>
            <div style="font-size: 12px; font-weight: 500;">{{ n.message }}</div>
          </div>
        </div>
      </div>

      <div class="app-content">
        <slot />
      </div>
    </div>

    <!-- New Project Modal -->
    <div v-if="showNewProjectModal" class="modal-backdrop" @click.self="showNewProjectModal = false">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 14px;">新規プロジェクト作成</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="createNewProject">
          <div>
            <label class="field-label">プロジェクト名 *</label>
            <input v-model="newProjForm.name" type="text" required style="width: 100%;" placeholder="例: 次世代EC基盤開発" />
          </div>
          <div>
            <label class="field-label">説明</label>
            <textarea v-model="newProjForm.description" rows="3" style="width: 100%;" placeholder="プロジェクトの目的や概要"></textarea>
          </div>
          <div>
            <label class="field-label">1日あたりの基準時間</label>
            <input v-model.number="newProjForm.hours_per_day" type="number" min="1" max="24" step="0.01" required style="width: 100%;" />
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px;">
            <button type="button" class="btn btn-secondary" @click="showNewProjectModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary">作成する</button>
          </div>
        </form>
      </div>
    </div>

    <!-- Account Settings Modal -->
    <div v-if="showAccountModal && currentUser" class="modal-backdrop" @click.self="showAccountModal = false">
      <div class="modal-panel" style="max-width: 560px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div style="display: flex; align-items: center; gap: 12px;">
            <h3 style="font-size: 16px; font-weight: 700;">アカウント設定</h3>
            <div style="display: flex; gap: 4px; padding: 2px; border-radius: 6px; background: var(--bg-elevated); border: 1px solid var(--border-color);">
              <button
                type="button"
                class="btn btn-sm"
                :class="accountTab === 'profile' ? 'btn-primary' : 'btn-ghost'"
                style="padding: 2px 8px; font-size: 11px;"
                @click="accountTab = 'profile'"
              >
                基本設定
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="accountTab === 'pat' ? 'btn-primary' : 'btn-ghost'"
                style="padding: 2px 8px; font-size: 11px;"
                @click="accountTab = 'pat'; loadPATs()"
              >
                APIトークン (PAT)
              </button>
            </div>
          </div>
          <button class="btn btn-danger btn-sm" @click="logout">ログアウト</button>
        </div>

        <!-- TAB 1: PROFILE & CREDENTIALS -->
        <div v-if="accountTab === 'profile'" style="display: flex; flex-direction: column; gap: 18px;">
          <!-- Profile -->
          <form style="display: flex; flex-direction: column; gap: 10px;" @submit.prevent="saveProfile">
            <h4 style="font-size: 13px; font-weight: 700;">基本プロフィール</h4>
            <div>
              <label class="field-label">表示名</label>
              <input v-model="accountForm.display_name" type="text" required style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">アイコンURL</label>
              <input v-model="accountForm.avatar_url" type="text" style="width: 100%;" placeholder="https://..." />
            </div>
            <div>
              <button type="submit" class="btn btn-primary btn-sm">プロフィールを保存</button>
            </div>
          </form>

          <!-- Password Change / Initial Setup -->
          <form style="display: flex; flex-direction: column; gap: 10px; border-top: 1px solid var(--border-color); padding-top: 14px;" @submit.prevent="savePassword">
            <h4 style="font-size: 13px; font-weight: 700;">
              {{ currentUser.has_password ? 'パスワード変更' : 'パスワード新規設定（Google連携アカウント用）' }}
            </h4>
            <div v-if="currentUser.has_password">
              <label class="field-label">現在のパスワード</label>
              <input v-model="passwordForm.current_password" type="password" required style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">新しいパスワード（8文字以上）</label>
              <input v-model="passwordForm.new_password" type="password" minlength="8" required style="width: 100%;" />
            </div>
            <div>
              <button type="submit" class="btn btn-primary btn-sm">パスワードを更新</button>
            </div>
          </form>

          <!-- Self Email Change Request -->
          <form style="display: flex; flex-direction: column; gap: 10px; border-top: 1px solid var(--border-color); padding-top: 14px;" @submit.prevent="requestEmailChange">
            <h4 style="font-size: 13px; font-weight: 700;">ログインメールアドレスの変更（確認メール送信）</h4>
            <div>
              <label class="field-label">新しいメールアドレス</label>
              <input v-model="emailChangeForm.new_email" type="email" required style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">現在のパスワード（本人確認）</label>
              <input v-model="emailChangeForm.current_password" type="password" required style="width: 100%;" />
            </div>
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <button type="submit" class="btn btn-primary btn-sm">確認メールを送信</button>
              <button type="button" class="btn btn-secondary btn-sm" @click="showAccountModal = false">閉じる</button>
            </div>
          </form>
        </div>

        <!-- TAB 2: PERSONAL ACCESS TOKENS (API KEYS) -->
        <div v-else-if="accountTab === 'pat'" style="display: flex; flex-direction: column; gap: 14px;">
          <p style="font-size: 13px; color: var(--text-muted);">
            CI/CD、スクリプト、外部Bot等からPolaris APIをセキュアに操作するための認証トークン（PAT）を発行・管理できます。
          </p>

          <!-- Newly Created Token Alert -->
          <div v-if="newPlainToken" style="padding: 12px; border-radius: 8px; background: var(--success-soft); border: 1px solid var(--success); display: flex; flex-direction: column; gap: 8px;">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div style="font-size: 13px; font-weight: 700; color: var(--success);">
                ✓ 新しいトークンが生成されました。このトークンは二度と表示されません。安全な場所に保存してください。
              </div>
              <button
                type="button"
                class="btn btn-ghost btn-sm"
                style="padding: 0 4px; font-size: 14px; line-height: 1; color: var(--text-muted);"
                title="閉じる"
                @click="newPlainToken = ''"
              >
                ✕
              </button>
            </div>
            <div style="display: flex; gap: 8px;">
              <input :value="newPlainToken" readonly style="flex: 1; font-family: monospace; font-size: 13px;" />
              <button type="button" class="btn btn-primary btn-sm" @click="copyToken(newPlainToken)">コピー</button>
            </div>
          </div>

          <!-- Token List -->
          <div style="display: flex; flex-direction: column; gap: 8px; max-height: 200px; overflow-y: auto;">
            <div
              v-for="tok in patList"
              :key="tok.id"
              style="padding: 10px 12px; border-radius: 8px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; align-items: center; justify-content: space-between; gap: 8px;"
            >
              <div style="display: flex; flex-direction: column; gap: 2px;">
                <div style="display: flex; align-items: center; gap: 8px;">
                  <strong style="font-size: 14px;">{{ tok.name }}</strong>
                  <code style="font-size: 12px; color: var(--primary);">{{ tok.token_prefix }}...</code>
                  <span v-if="tok.revoked_at" class="badge badge-red" style="font-size: 11px;">失効済み</span>
                  <span v-else-if="tok.expires_at && new Date(tok.expires_at) < new Date()" class="badge badge-amber" style="font-size: 11px;">期限切れ</span>
                  <span v-else class="badge badge-green" style="font-size: 11px;">有効</span>
                </div>
                <div style="font-size: 12px; color: var(--text-muted);">
                  作成: {{ tok.created_at.slice(0, 10) }} ・ 最終利用: {{ tok.last_used_at ? tok.last_used_at.slice(0, 10) : '未使用' }}
                  ・ 有効期限: {{ tok.expires_at ? tok.expires_at.slice(0, 10) : '無期限' }}
                </div>
              </div>
              <button
                v-if="!tok.revoked_at && (!tok.expires_at || new Date(tok.expires_at) >= new Date())"
                type="button"
                class="btn btn-danger btn-sm"
                style="padding: 3px 8px; font-size: 12px;"
                @click="revokePAT(tok.id)"
              >
                失効
              </button>
            </div>
            <div v-if="patList.length === 0" style="padding: 16px; text-align: center; color: var(--text-muted); font-size: 13px;">
              発行済みのAPIトークンはありません
            </div>
          </div>

          <!-- Create Token Form -->
          <form style="display: flex; flex-direction: column; gap: 10px; border-top: 1px solid var(--border-color); padding-top: 12px;" @submit.prevent="createPAT">
            <h4 style="font-size: 14px; font-weight: 700;">＋ 新規APIトークンを発行</h4>
            <div style="display: grid; grid-template-columns: 1fr 140px; gap: 8px;">
              <div>
                <label class="field-label">トークン名 *</label>
                <input v-model="newPatForm.name" type="text" required style="width: 100%;" placeholder="例: GitHub Actions CI" />
              </div>
              <div>
                <label class="field-label">有効期限</label>
                <select v-model="newPatForm.expires_in_days" style="width: 100%;">
                  <option :value="30">30日</option>
                  <option :value="60">60日</option>
                  <option :value="90">90日</option>
                  <option :value="365">1年間</option>
                  <option :value="0">無期限</option>
                </select>
              </div>
            </div>

            <div>
              <label class="field-label">スコープ（権限範囲）</label>
              <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; font-size: 13px;">
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="read:projects" />
                  <span>read:projects (プロジェクト閲覧)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="write:projects" />
                  <span>write:projects (プロジェクト編集)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="read:tasks" />
                  <span>read:tasks (タスク閲覧)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="write:tasks" />
                  <span>write:tasks (タスク更新)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="read:issues" />
                  <span>read:issues (課題閲覧)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="write:issues" />
                  <span>write:issues (課題起票・更新)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="read:comments" />
                  <span>read:comments (コメント閲覧)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="write:comments" />
                  <span>write:comments (コメント投稿)</span>
                </label>
                <label style="display: flex; align-items: center; gap: 4px; grid-column: span 2;">
                  <input v-model="newPatForm.scopes" type="checkbox" value="admin:system" />
                  <span style="font-weight: 500; color: #b91c1c;">admin:system (システム管理・全権)</span>
                </label>
              </div>
            </div>

            <div style="display: flex; justify-content: flex-end; gap: 8px;">
              <button type="submit" class="btn btn-primary btn-sm">トークンを発行</button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Bell, FolderKanban, LayoutGrid, Plus, Settings, ShieldCheck } from 'lucide-vue-next'
import type { NotificationItem, PersonalAccessToken } from '~/types/polaris'
import { formatHours, formatJSTDateTime, hasAtMostTwoDecimals } from '~/utils/format'

const route = useRoute()
const router = useRouter()
const {
  apiFetch,
  currentUser,
  projects,
  notifications,
  notifyToast,
  refreshCurrentUser,
  refreshProjects,
  refreshNotifications,
  hasSystemPerm
} = useApi()
const { registerPushNotifications } = useFcm()

const isPublicRoute = computed(() => {
  return (
    route.path.startsWith('/login') ||
    route.path.startsWith('/setup') ||
    route.path.startsWith('/auth/')
  )
})

const activeProjectId = computed(() => {
  return (route.params.id as string) || ''
})

const activeProject = computed(() => {
  return projects.value.find(p => p.id === activeProjectId.value) || null
})

const unreadCount = computed(() => notifications.value.filter(n => !n.is_read).length)

const showNotifications = ref(false)
const showNewProjectModal = ref(false)
const showAccountModal = ref(false)

const newProjForm = reactive({
  name: '',
  description: '',
  hours_per_day: 8.0
})

const accountForm = reactive({
  display_name: '',
  avatar_url: ''
})

const passwordForm = reactive({
  current_password: '',
  new_password: ''
})

const accountTab = ref<'profile' | 'pat'>('profile')
const patList = ref<PersonalAccessToken[]>([])
const newPlainToken = ref('')
const newPatForm = reactive({
  name: '',
  expires_in_days: 90,
  scopes: ['read:projects', 'read:tasks', 'write:tasks', 'read:issues', 'write:issues']
})

async function loadPATs() {
  try {
    patList.value = await apiFetch<PersonalAccessToken[]>('/user/tokens')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createPAT() {
  try {
    const res = await apiFetch<{ token: string; pat: PersonalAccessToken }>('/user/tokens', {
      method: 'POST',
      body: JSON.stringify({
        name: newPatForm.name,
        expires_in_days: newPatForm.expires_in_days === 0 ? null : newPatForm.expires_in_days,
        scopes: newPatForm.scopes
      })
    })
    newPlainToken.value = res.token
    newPatForm.name = ''
    await loadPATs()
    notifyToast('APIトークンを発行しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function revokePAT(id: string) {
  if (!confirm('このAPIトークンを失効させますか？失効すると元に戻せません。')) return
  try {
    await apiFetch(`/user/tokens/${id}`, { method: 'DELETE' })
    await loadPATs()
    notifyToast('APIトークンを失効させました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function copyToken(text: string) {
  if (navigator?.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(() => {
      notifyToast('トークンをクリップボードにコピーしました', 'success')
    }).catch(() => {
      notifyToast('クリップボードへのコピーに失敗しました', 'error')
    })
  } else {
    notifyToast('クリップボードAPI非対応の環境です', 'error')
  }
}

const emailChangeForm = reactive({
  new_email: '',
  current_password: ''
})

onMounted(async () => {
  if (isPublicRoute.value) return
  try {
    const setupStatus = await apiFetch<{ setup_required?: boolean; needs_setup?: boolean }>('/setup/status')
    if (setupStatus.setup_required || setupStatus.needs_setup) {
      router.replace('/setup')
      return
    }
  } catch {
    // ignore
  }
  const u = await refreshCurrentUser()
  if (!u) {
    router.replace('/login')
    return
  }
  await Promise.all([refreshProjects(), refreshNotifications()])
  registerPushNotifications()
})

watch(() => route.path, async (newPath) => {
  if (!newPath.startsWith('/login') && !newPath.startsWith('/setup') && !newPath.startsWith('/auth/')) {
    if (!currentUser.value) {
      const u = await refreshCurrentUser()
      if (!u) {
        router.replace('/login')
        return
      }
      await Promise.all([refreshProjects(), refreshNotifications()])
    }
  }
})

function onSelectProject(pid: string) {
  if (!pid) {
    router.push('/')
  } else {
    router.push(`/projects/${pid}`)
  }
}

async function createNewProject() {
  if (!hasAtMostTwoDecimals(newProjForm.hours_per_day)) {
    notifyToast('1日の基準時間は小数点第2位までで入力してください', 'error')
    return
  }
  try {
    const created = await apiFetch('/projects', {
      method: 'POST',
      body: JSON.stringify(newProjForm)
    })
    showNewProjectModal.value = false
    newProjForm.name = ''
    newProjForm.description = ''
    await refreshProjects()
    notifyToast('プロジェクトを作成しました', 'success')
    router.push(`/projects/${created.id}`)
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

const notificationPopoverRef = ref<HTMLElement | null>(null)
const notificationBellRef = ref<HTMLElement | null>(null)
const notificationFilter = ref<'all' | 'unread'>('all')

const filteredNotifications = computed(() => {
  if (notificationFilter.value === 'unread') {
    return notifications.value.filter(n => !n.is_read)
  }
  return notifications.value
})

function onDocumentClick(e: MouseEvent) {
  if (!showNotifications.value) return
  const target = e.target as HTMLElement
  if (
    notificationPopoverRef.value?.contains(target) ||
    notificationBellRef.value?.contains(target)
  ) {
    return
  }
  showNotifications.value = false
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
})

function toggleNotifications() {
  showNotifications.value = !showNotifications.value
  if (showNotifications.value) {
    refreshNotifications()
  }
}

async function markAllRead() {
  try {
    await apiFetch('/notifications/read-all', { method: 'PUT' })
    await refreshNotifications()
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function markSingleRead(n: NotificationItem) {
  try {
    await apiFetch(`/notifications/${n.id}/read`, { method: 'PUT' })
    await refreshNotifications()
    notifyToast('通知を既読にしました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function onClickNotification(n: NotificationItem) {
  try {
    if (!n.is_read) {
      await apiFetch(`/notifications/${n.id}/read`, { method: 'PUT' })
      await refreshNotifications()
    }
    showNotifications.value = false
    router.push(`/projects/${n.project_id}?openTarget=${n.target_type}:${n.target_id}`)
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function openAccountModal() {
  if (!currentUser.value) return
  accountForm.display_name = currentUser.value.display_name
  accountForm.avatar_url = currentUser.value.avatar_url || ''
  passwordForm.current_password = ''
  passwordForm.new_password = ''
  emailChangeForm.new_email = ''
  emailChangeForm.current_password = ''
  showAccountModal.value = true
}

async function saveProfile() {
  try {
    await apiFetch('/auth/me/profile', {
      method: 'PUT',
      body: JSON.stringify(accountForm)
    })
    await refreshCurrentUser()
    notifyToast('プロフィールを更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function savePassword() {
  try {
    await apiFetch('/auth/me/password', {
      method: 'PUT',
      body: JSON.stringify(passwordForm)
    })
    passwordForm.current_password = ''
    passwordForm.new_password = ''
    await refreshCurrentUser()
    notifyToast('パスワードを更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function requestEmailChange() {
  try {
    await apiFetch('/auth/me/email-change', {
      method: 'POST',
      body: JSON.stringify(emailChangeForm)
    })
    emailChangeForm.new_email = ''
    emailChangeForm.current_password = ''
    notifyToast('確認メールを新しいメールアドレス宛に送信しました（1時間有効）', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function logout() {
  try {
    await apiFetch('/auth/logout', { method: 'POST' })
  } finally {
    currentUser.value = null
    showAccountModal.value = false
    router.push('/login')
  }
}
</script>

