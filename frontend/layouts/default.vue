<template>
  <div v-if="isPublicRoute">
    <slot />
  </div>

  <div v-else class="app-shell">
    <!-- Backdrop overlay for mobile drawer -->
    <div
      v-if="isMobileSidebarOpen"
      class="sidebar-backdrop"
      @click="isMobileSidebarOpen = false"
    ></div>

    <!-- Left Navigation Sidebar -->
    <aside class="app-sidebar" :class="{ 'mobile-open': isMobileSidebarOpen }">
      <div style="display: flex; flex-direction: column; gap: 20px;">
        <!-- Brand Header -->
        <div style="display: flex; align-items: center; justify-content: space-between;">
          <NuxtLink to="/" style="display: flex; align-items: center; gap: 10px; text-decoration: none; color: inherit;" @click="isMobileSidebarOpen = false">
            <div style="width: 34px; height: 34px; border-radius: 8px; background: #2563eb; color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; font-size: 16px;">
              P
            </div>
            <div>
              <div style="font-size: 16px; font-weight: 700;">Polaris</div>
              <div style="font-size: 12px; color: var(--text-muted);">Project Management</div>
            </div>
          </NuxtLink>
          <div style="display: flex; align-items: center; gap: 4px;">
            <button
              v-if="hasSystemPerm('create_project')"
              class="btn btn-primary btn-sm"
              title="新規プロジェクト作成"
              @click="showNewProjectModal = true; isMobileSidebarOpen = false"
            >
              <Plus :size="14" /> 新規
            </button>
            <button
              type="button"
              class="btn btn-ghost btn-sm mobile-sidebar-close-btn"
              title="サイドバーを閉じる"
              @click="isMobileSidebarOpen = false"
            >
              ✕
            </button>
          </div>
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

      <!-- User Footer & Actions (2-Tier Layout) -->
      <div v-if="currentUser" style="border-top: 1px solid var(--border-color); padding-top: 12px; display: flex; flex-direction: column; gap: 8px;">
        <!-- 上段（ユーザー情報カード） -->
        <div
          style="display: flex; align-items: center; gap: 10px; padding: 8px 10px; background: var(--bg-hover); border: 1px solid var(--border-color); border-radius: 8px; cursor: default; overflow: hidden; width: 100%; box-sizing: border-box;"
        >
          <div style="width: 34px; height: 34px; border-radius: 999px; background: #4f46e5; color: #fff; font-weight: 700; font-size: 13px; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
            {{ currentUser.display_name.slice(0, 2).toUpperCase() }}
          </div>
          <div style="overflow: hidden; flex: 1; min-width: 0;">
            <div
              :title="currentUser.display_name"
              style="font-size: 13px; font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis;"
            >
              {{ currentUser.display_name }}
            </div>
            <div
              :title="currentUser.email"
              style="font-size: 11px; color: var(--text-muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; margin-top: 1px;"
            >
              {{ currentUser.email }}
            </div>
          </div>
        </div>

        <!-- 下段（アクションボタングループ） -->
        <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px;">
          <button
            ref="notificationBellRef"
            type="button"
            class="btn btn-secondary btn-sm"
            style="display: flex; align-items: center; justify-content: center; gap: 3px; padding: 6px 2px; font-size: 11px; width: 100%; white-space: nowrap;"
            title="通知一覧"
            @click="toggleNotifications"
          >
            <Bell :size="14" />
            <span>通知</span>
            <span
              v-if="unreadCount > 0"
              class="badge badge-amber"
              style="font-size: 9px; padding: 0 4px; line-height: 13px; height: 13px; border-radius: 999px;"
            >
              {{ unreadCount }}
            </span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            style="display: flex; align-items: center; justify-content: center; gap: 3px; padding: 6px 2px; font-size: 11px; width: 100%; white-space: nowrap;"
            title="アカウント設定"
            @click="openAccountModal"
          >
            <Settings :size="14" />
            <span>設定</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            style="display: flex; align-items: center; justify-content: center; gap: 3px; padding: 6px 2px; font-size: 11px; width: 100%; white-space: nowrap; color: var(--danger);"
            title="ログアウト"
            @click="openLogoutModal"
          >
            <LogOut :size="14" />
            <span>ログアウト</span>
          </button>
        </div>
      </div>
    </aside>

    <!-- Main Content -->
    <div class="app-main">
      <!-- Mobile Top Bar (< 768px) -->
      <header class="mobile-header">
        <button
          type="button"
          class="btn btn-ghost btn-sm"
          style="padding: 6px 8px;"
          title="メニューを開く"
          aria-label="メニューを開く"
          @click="isMobileSidebarOpen = true"
        >
          <Menu :size="20" />
        </button>
        <div style="display: flex; align-items: center; gap: 8px; font-weight: 700; font-size: 15px;">
          <div style="width: 24px; height: 24px; border-radius: 6px; background: #2563eb; color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; font-size: 12px;">
            P
          </div>
          <span>Polaris</span>
        </div>
        <div style="display: flex; align-items: center; gap: 6px;">
          <button class="btn btn-secondary btn-sm" style="padding: 4px 8px;" title="通知一覧" @click="toggleNotifications">
            <Bell :size="14" />
            <span v-if="unreadCount > 0">{{ unreadCount }}</span>
          </button>
        </div>
      </header>

      <!-- Notification Drawer Popover -->
      <div
        v-if="showNotifications"
        ref="notificationPopoverRef"
        class="card notification-popover"
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
    <div v-if="showNewProjectModal" class="modal-backdrop" v-backdrop-dismiss="() => showNewProjectModal = false">
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
    <div v-if="showAccountModal && currentUser" class="modal-backdrop" v-backdrop-dismiss="() => showAccountModal = false">
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
              <button
                type="button"
                class="btn btn-sm"
                :class="accountTab === 'mfa' ? 'btn-primary' : 'btn-ghost'"
                style="padding: 2px 8px; font-size: 11px;"
                @click="accountTab = 'mfa'; loadMFAStatus()"
              >
                🔐 2要素認証 (MFA)
              </button>
            </div>
          </div>
          <button type="button" class="btn btn-ghost btn-sm" style="padding: 2px 6px;" title="閉じる" @click="showAccountModal = false">✕</button>
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

        <!-- TAB 3: 2-FACTOR AUTHENTICATION (MFA) -->
        <div v-else-if="accountTab === 'mfa'" style="display: flex; flex-direction: column; gap: 16px;">
          <!-- MFA Status Banner -->
          <div
            style="padding: 12px 14px; border-radius: 8px; border: 1px solid var(--border-color); display: flex; align-items: center; justify-content: space-between; gap: 10px;"
            :style="{ backgroundColor: mfaStatus?.mfa_enabled ? 'rgba(22, 163, 74, 0.08)' : 'rgba(245, 158, 11, 0.08)' }"
          >
            <div style="display: flex; align-items: center; gap: 8px;">
              <ShieldCheck v-if="mfaStatus?.mfa_enabled" :size="20" style="color: var(--success);" />
              <ShieldAlert v-else :size="20" style="color: var(--warning);" />
              <div>
                <div style="font-size: 13px; font-weight: 700;">
                  {{ mfaStatus?.mfa_enabled ? '2要素認証は有効です' : '2要素認証は未設定です' }}
                </div>
                <div style="font-size: 11px; color: var(--text-muted);">
                  {{ (mfaStatus?.mfa_enforced || mfaStatus?.system_mfa_required) ? '⚠️ セキュリティポリシーによりMFAが必須化されています' : 'ログイン時のセキュリティを高めるためMFAの設定を推奨します' }}
                </div>
              </div>
            </div>
            <span :class="['badge', mfaStatus?.mfa_enabled ? 'badge-green' : 'badge-amber']" style="font-size: 11px;">
              {{ mfaStatus?.mfa_enabled ? '有効' : '未設定' }}
            </span>
          </div>

          <!-- Section 1: TOTP (Authenticator App) -->
          <div class="card" style="padding: 14px; display: flex; flex-direction: column; gap: 10px;">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div style="display: flex; align-items: center; gap: 6px;">
                <Smartphone :size="16" style="color: var(--primary);" />
                <h4 style="font-size: 13px; font-weight: 700;">認証アプリ (TOTP)</h4>
              </div>
              <span v-if="mfaStatus?.has_totp" class="badge badge-green" style="font-size: 10px;">設定済み ✅</span>
              <span v-else class="badge badge-slate" style="font-size: 10px;">未設定</span>
            </div>

            <p style="font-size: 12px; color: var(--text-muted); margin: 0;">
              Google Authenticator、1Password等の認証アプリで生成される6桁のワンタイムパスコードを利用します。
            </p>

            <!-- Not Setup: Button to start setup -->
            <div v-if="!mfaStatus?.has_totp && !showTotpSetupCard">
              <button type="button" class="btn btn-primary btn-sm" @click="startTOTPSetup">
                認証アプリを設定する
              </button>
            </div>

            <!-- Setup Card in progress -->
            <div
              v-else-if="!mfaStatus?.has_totp && showTotpSetupCard"
              style="padding: 12px; border-radius: 8px; background: var(--bg-elevated); border: 1px solid var(--border-color); display: flex; flex-direction: column; gap: 12px;"
            >
              <div style="font-size: 12px; color: var(--text-secondary); line-height: 1.5;">
                1. 認証アプリで以下のQRコードをスキャンするか、シークレットキーを手動入力してください。
              </div>

              <!-- QR Code Image -->
              <div style="display: flex; justify-content: center;">
                <img
                  :src="'https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=' + encodeURIComponent(totpSetupData.otpauth_url)"
                  alt="TOTP QR Code"
                  style="width: 170px; height: 170px; border-radius: 8px; border: 1px solid var(--border-color); background: #fff; padding: 6px;"
                />
              </div>

              <!-- Monospace Secret Key -->
              <div style="display: flex; flex-direction: column; align-items: center; gap: 6px;">
                <div style="font-size: 11px; color: var(--text-muted);">シークレットキー（手動入力用）:</div>
                <div style="display: flex; align-items: center; gap: 6px;">
                  <code style="font-size: 13px; font-family: monospace; letter-spacing: 0.1em; background: var(--bg-card); padding: 4px 10px; border-radius: 6px; border: 1px solid var(--border-color);">
                    {{ totpSetupData.secret }}
                  </code>
                  <button type="button" class="btn btn-secondary btn-sm" style="padding: 3px 8px; font-size: 11px;" @click="copyToken(totpSetupData.secret)">
                    コピー
                  </button>
                </div>
              </div>

              <!-- Step 2: Verification Code -->
              <div style="display: flex; flex-direction: column; gap: 6px; border-top: 1px solid var(--border-color); padding-top: 10px;">
                <div style="font-size: 12px; font-weight: 600;">2. 認証アプリに表示されている6桁コードを入力して有効化:</div>
                <div style="display: flex; gap: 8px;">
                  <input
                    v-model="totpConfirmCode"
                    type="text"
                    maxlength="6"
                    pattern="[0-9]*"
                    inputmode="numeric"
                    placeholder="123456"
                    style="flex: 1; text-align: center; font-size: 18px; font-family: monospace; letter-spacing: 0.2em;"
                  />
                  <button
                    type="button"
                    class="btn btn-primary btn-sm"
                    :disabled="totpConfirmCode.length !== 6 || isEnablingTotp"
                    @click="confirmEnableTOTP"
                  >
                    {{ isEnablingTotp ? '有効化中...' : '有効化する' }}
                  </button>
                  <button type="button" class="btn btn-secondary btn-sm" @click="showTotpSetupCard = false">
                    キャンセル
                  </button>
                </div>
              </div>
            </div>

            <!-- Already Setup: Disable Button -->
            <div v-else-if="mfaStatus?.has_totp" style="display: flex; align-items: center; justify-content: space-between; gap: 10px;">
              <span style="font-size: 12px; color: var(--text-muted);">
                {{ (mfaStatus?.mfa_enforced || mfaStatus?.system_mfa_required) ? '※ 管理者ポリシーによりMFAが必須化されているため無効化できません' : '認証アプリによる2要素認証が有効です' }}
              </span>
              <button
                type="button"
                class="btn btn-danger btn-sm"
                :disabled="Boolean(mfaStatus?.mfa_enforced || mfaStatus?.system_mfa_required)"
                @click="showDisableTotpModal = true"
              >
                無効化
              </button>
            </div>
          </div>

          <!-- Section 2: WebAuthn / Security Key (FIDO2) -->
          <div class="card" style="padding: 14px; display: flex; flex-direction: column; gap: 10px;">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div style="display: flex; align-items: center; gap: 6px;">
                <Key :size="16" style="color: var(--primary);" />
                <h4 style="font-size: 13px; font-weight: 700;">セキュリティキー / FIDO2 (WebAuthn)</h4>
              </div>
              <span class="badge badge-slate" style="font-size: 10px;">{{ mfaStatus?.webauthn_credentials?.length || 0 }}件登録</span>
            </div>

            <p style="font-size: 12px; color: var(--text-muted); margin: 0;">
              YubiKey等の物理セキュリティキーや、Touch ID、Windows Hello等の生体認証を利用して素早くログインできます。
            </p>

            <!-- Register New Security Key Form -->
            <div style="display: flex; gap: 8px;">
              <input
                v-model="newWebAuthnKeyName"
                type="text"
                placeholder="キーの名称（例: MacBook Touch ID, YubiKey 5C）"
                style="flex: 1; font-size: 13px;"
              />
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="!newWebAuthnKeyName.trim() || isRegisteringWebAuthn"
                @click="registerWebAuthnKey"
              >
                {{ isRegisteringWebAuthn ? 'キー待機中...' : '＋ セキュリティキーを登録' }}
              </button>
            </div>

            <!-- Registered Keys List -->
            <div v-if="mfaStatus?.webauthn_credentials && mfaStatus.webauthn_credentials.length > 0" style="display: flex; flex-direction: column; gap: 6px; margin-top: 4px;">
              <div
                v-for="k in mfaStatus.webauthn_credentials"
                :key="k.id"
                style="padding: 8px 10px; border-radius: 6px; background: var(--bg-elevated); border: 1px solid var(--border-color); display: flex; align-items: center; justify-content: space-between; gap: 8px;"
              >
                <div style="display: flex; align-items: center; gap: 8px;">
                  <Key :size="14" style="color: var(--primary);" />
                  <strong style="font-size: 13px;">{{ k.name }}</strong>
                  <span style="font-size: 11px; color: var(--text-muted);">
                    登録: {{ k.created_at.slice(0, 10) }} {{ k.last_used_at ? '・ 最終利用: ' + k.last_used_at.slice(0, 10) : '' }}
                  </span>
                </div>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="color: var(--danger); padding: 2px 6px;"
                  title="セキュリティキーを削除"
                  @click="deleteWebAuthnKey(k)"
                >
                  <Trash2 :size="14" />
                </button>
              </div>
            </div>
            <div v-else style="font-size: 12px; color: var(--text-muted); text-align: center; padding: 8px 0;">
              登録されているセキュリティキーはありません
            </div>
          </div>

          <!-- Section 3: Backup Codes Management -->
          <div class="card" style="padding: 14px; display: flex; flex-direction: column; gap: 10px;">
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div style="font-size: 13px; font-weight: 700;">🎫 バックアップコード管理</div>
              <span class="badge badge-purple" style="font-size: 10px;">残り {{ mfaStatus?.remaining_backup_codes ?? 0 }} 件</span>
            </div>
            <p style="font-size: 12px; color: var(--text-muted); margin: 0;">
              端末の故障・紛失時にアカウントを復旧するための使い捨てコードです。安全な場所に保存してください。
            </p>
            <div style="display: flex; justify-content: flex-end;">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                @click="regenerateBackupCodes"
              >
                バックアップコードを再生成
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Backup Codes Display Modal -->
    <div v-if="showBackupCodesModal" class="modal-backdrop" v-backdrop-dismiss="() => showBackupCodesModal = false">
      <div class="modal-panel" style="max-width: 480px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 8px;">🎫 バックアップコードが発行されました</h3>
        <p style="font-size: 12px; color: var(--danger); margin-bottom: 12px; font-weight: 600;">
          ⚠️ これらのコードは二度と表示されません。今すぐコピーして安全なパスワードマネージャー等に保存してください。
        </p>

        <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 8px; margin-bottom: 14px; background: var(--bg-elevated); padding: 12px; border-radius: 8px; border: 1px solid var(--border-color);">
          <div
            v-for="(c, idx) in newlyGeneratedBackupCodes"
            :key="idx"
            style="font-family: monospace; font-size: 14px; letter-spacing: 0.1em; text-align: center; background: var(--bg-card); padding: 6px; border-radius: 4px; border: 1px solid var(--border-color);"
          >
            {{ c }}
          </div>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center;">
          <button type="button" class="btn btn-secondary btn-sm" @click="copyBackupCodes">
            コード一覧をコピー
          </button>
          <button type="button" class="btn btn-primary btn-sm" @click="showBackupCodesModal = false">
            安全に保存しました
          </button>
        </div>
      </div>
    </div>

    <!-- Disable TOTP Password Prompt Modal -->
    <div v-if="showDisableTotpModal" class="modal-backdrop" v-backdrop-dismiss="() => showDisableTotpModal = false">
      <div class="modal-panel" style="max-width: 420px;">
        <h3 style="font-size: 15px; font-weight: 700; margin-bottom: 8px;">認証アプリ (TOTP) の無効化</h3>
        <p style="font-size: 12px; color: var(--text-muted); margin-bottom: 12px;">
          本人確認のため、現在のログインパスワードを入力してください。
        </p>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="disableTOTP">
          <input
            v-model="disableTotpPassword"
            type="password"
            required
            placeholder="現在のパスワード"
            style="width: 100%;"
          />
          <div style="display: flex; justify-content: flex-end; gap: 8px;">
            <button type="button" class="btn btn-secondary btn-sm" @click="showDisableTotpModal = false">
              キャンセル
            </button>
            <button type="submit" class="btn btn-danger btn-sm" :disabled="isDisablingTotp">
              {{ isDisablingTotp ? '無効化中...' : '無効化を実行' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Mandatory MFA Enforcement Blocking Modal (Requirement 4) -->
    <div
      v-if="isMfaEnforcedPending"
      class="modal-backdrop"
      style="z-index: 9999; background: rgba(15, 23, 42, 0.88); backdrop-filter: blur(4px);"
    >
      <div class="modal-panel" style="max-width: 560px; box-shadow: 0 20px 40px rgba(0,0,0,0.5);">
        <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div style="width: 38px; height: 38px; border-radius: 8px; background: #dc2626; color: #fff; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
            <ShieldAlert :size="22" />
          </div>
          <div>
            <h3 style="font-size: 16px; font-weight: 700; color: var(--danger);">⚠️ 2要素認証（MFA）の設定が必要です</h3>
            <p style="font-size: 12px; color: var(--text-muted); margin: 0;">
              セキュリティポリシーにより、2要素認証の設定が義務付けられています。設定を完了するまで操作が制限されます。
            </p>
          </div>
        </div>

        <div style="display: flex; flex-direction: column; gap: 14px;">
          <!-- Choice: Setup TOTP or WebAuthn -->
          <div style="display: flex; gap: 6px; padding: 2px; border-radius: 6px; background: var(--bg-elevated); border: 1px solid var(--border-color);">
            <button
              type="button"
              class="btn btn-sm"
              :class="enforcedSetupTab === 'totp' ? 'btn-primary' : 'btn-ghost'"
              style="flex: 1; font-size: 12px;"
              @click="enforcedSetupTab = 'totp'"
            >
              <Smartphone :size="13" /> 認証アプリ (TOTP)
            </button>
            <button
              type="button"
              class="btn btn-sm"
              :class="enforcedSetupTab === 'webauthn' ? 'btn-primary' : 'btn-ghost'"
              style="flex: 1; font-size: 12px;"
              @click="enforcedSetupTab = 'webauthn'"
            >
              <Key :size="13" /> セキュリティキー
            </button>
          </div>

          <!-- Enforced TOTP Tab -->
          <div v-if="enforcedSetupTab === 'totp'" style="display: flex; flex-direction: column; gap: 10px;">
            <div v-if="!showTotpSetupCard">
              <p style="font-size: 12px; color: var(--text-secondary);">
                Google Authenticator、1Password等のアプリで二段階認証を設定します。
              </p>
              <button type="button" class="btn btn-primary btn-sm" @click="startTOTPSetup">
                認証アプリを設定する
              </button>
            </div>
            <div v-else style="display: flex; flex-direction: column; gap: 10px;">
              <div style="display: flex; justify-content: center;">
                <img
                  :src="'https://api.qrserver.com/v1/create-qr-code/?size=180x180&data=' + encodeURIComponent(totpSetupData.otpauth_url)"
                  alt="TOTP QR Code"
                  style="width: 160px; height: 160px; border-radius: 8px; border: 1px solid var(--border-color); background: #fff; padding: 6px;"
                />
              </div>
              <div style="display: flex; justify-content: center; align-items: center; gap: 6px;">
                <code style="font-size: 12px; font-family: monospace; background: var(--bg-elevated); padding: 3px 8px; border-radius: 4px;">{{ totpSetupData.secret }}</code>
                <button type="button" class="btn btn-secondary btn-sm" style="padding: 2px 6px; font-size: 11px;" @click="copyToken(totpSetupData.secret)">コピー</button>
              </div>
              <div style="display: flex; gap: 8px;">
                <input
                  v-model="totpConfirmCode"
                  type="text"
                  maxlength="6"
                  pattern="[0-9]*"
                  inputmode="numeric"
                  placeholder="6桁コード"
                  style="flex: 1; text-align: center; font-size: 18px; font-family: monospace; letter-spacing: 0.2em;"
                />
                <button
                  type="button"
                  class="btn btn-primary btn-sm"
                  :disabled="totpConfirmCode.length !== 6 || isEnablingTotp"
                  @click="confirmEnableTOTP"
                >
                  {{ isEnablingTotp ? '有効化中...' : '有効化して開始' }}
                </button>
              </div>
            </div>
          </div>

          <!-- Enforced WebAuthn Tab -->
          <div v-else-if="enforcedSetupTab === 'webauthn'" style="display: flex; flex-direction: column; gap: 10px;">
            <p style="font-size: 12px; color: var(--text-secondary);">
              YubiKey、Touch ID等のセキュリティキーを登録して二段階認証を有効化します。
            </p>
            <div style="display: flex; gap: 8px;">
              <input
                v-model="newWebAuthnKeyName"
                type="text"
                placeholder="キーの名称（例: MacBook Touch ID）"
                style="flex: 1; font-size: 13px;"
              />
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="!newWebAuthnKeyName.trim() || isRegisteringWebAuthn"
                @click="registerWebAuthnKey"
              >
                {{ isRegisteringWebAuthn ? 'キー待機中...' : '登録して有効化' }}
              </button>
            </div>
          </div>
        </div>

        <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--border-color); padding-top: 10px; margin-top: 14px;">
          <span style="font-size: 11px; color: var(--text-muted);">設定を完了するとこの画面は自動的に閉じられます</span>
          <button type="button" class="btn btn-danger btn-sm" @click="logout">ログアウト</button>
        </div>
      </div>
    </div>

    <!-- Forced Password Change Blocking Modal (Requirement 3) -->
    <div
      v-if="currentUser?.must_change_password"
      class="modal-backdrop"
      style="z-index: 9999; background: rgba(15, 23, 42, 0.88); backdrop-filter: blur(4px);"
    >
      <div class="modal-panel" style="max-width: 480px; box-shadow: 0 20px 40px rgba(0,0,0,0.5);">
        <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div style="width: 38px; height: 38px; border-radius: 8px; background: var(--primary); color: #fff; display: flex; align-items: center; justify-content: center; flex-shrink: 0; font-size: 18px;">
            🔐
          </div>
          <div>
            <h3 style="font-size: 16px; font-weight: 700; margin: 0;">初期パスワードの変更が必要です</h3>
            <p style="font-size: 12px; color: var(--text-muted); margin: 2px 0 0 0;">
              管理者によって仮パスワードで登録されています。セキュリティ保護のため、新しいパスワードを設定してください。
            </p>
          </div>
        </div>

        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitForcedPasswordChange">
          <div>
            <label class="field-label">現在の仮パスワード *</label>
            <input
              v-model="forcedPwdForm.current_password"
              type="password"
              required
              style="width: 100%;"
              placeholder="仮パスワードを入力"
            />
          </div>

          <div>
            <label class="field-label">新しいパスワード（8文字以上） *</label>
            <input
              v-model="forcedPwdForm.new_password"
              type="password"
              minlength="8"
              required
              style="width: 100%;"
              placeholder="新しいパスワード（8文字以上）"
            />
          </div>

          <div>
            <label class="field-label">新しいパスワード（確認） *</label>
            <input
              v-model="forcedPwdForm.confirm_password"
              type="password"
              minlength="8"
              required
              style="width: 100%;"
              placeholder="確認のため再入力"
            />
          </div>

          <div v-if="forcedPwdError" style="font-size: 12px; color: var(--danger); font-weight: 600;">
            {{ forcedPwdError }}
          </div>

          <div style="margin-top: 4px;">
            <button
              type="submit"
              class="btn btn-primary"
              style="width: 100%; justify-content: center;"
              :disabled="isSubmittingForcedPwd"
            >
              {{ isSubmittingForcedPwd ? '変更中...' : 'パスワードを変更して利用開始' }}
            </button>
          </div>
        </form>

        <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--border-color); padding-top: 10px; margin-top: 14px;">
          <span style="font-size: 11px; color: var(--text-muted);">変更完了まで他の操作は行えません</span>
          <button type="button" class="btn btn-secondary btn-sm" style="color: var(--danger);" @click="openLogoutModal">
            <LogOut :size="14" />
            <span>ログアウト</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Logout Confirmation Modal (Requirement 3) -->
    <div
      v-if="showLogoutModal"
      class="modal-backdrop"
      style="z-index: 10000;"
      v-backdrop-dismiss="() => !isLoggingOut && (showLogoutModal = false)"
    >
      <div class="modal-panel" style="max-width: 400px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 15px; font-weight: 700; margin: 0; display: flex; align-items: center; gap: 6px;">
            <LogOut :size="16" style="color: var(--danger);" />
            ログアウトの確認
          </h3>
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            style="padding: 2px 6px;"
            :disabled="isLoggingOut"
            @click="showLogoutModal = false"
          >
            ✕
          </button>
        </div>

        <p style="font-size: 13px; color: var(--text-main); margin-bottom: 20px;">
          ログアウトしてログイン画面に戻りますか？
        </p>

        <div style="display: flex; justify-content: flex-end; gap: 8px;">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            :disabled="isLoggingOut"
            @click="showLogoutModal = false"
          >
            キャンセル
          </button>
          <button
            type="button"
            class="btn btn-danger btn-sm"
            :disabled="isLoggingOut"
            @click="executeLogout"
          >
            <LogOut :size="14" />
            <span>{{ isLoggingOut ? 'ログアウト中...' : 'ログアウト' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Bell, FolderKanban, Key, LogOut, Menu, Plus, Settings, ShieldAlert, ShieldCheck, Smartphone, Trash2 } from 'lucide-vue-next'
import type { MFAStatus, NotificationItem, PersonalAccessToken, WebAuthnCredential } from '~/types/polaris'
import { formatHours, formatJSTDateTime, hasAtMostTwoDecimals } from '~/utils/format'
import { isWebAuthnSupported, performWebAuthnRegister } from '~/utils/webauthn'

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

// 1. 直前のプロジェクトIDを記憶するグローバルステート
const lastActiveProjectId = useState<string | null>('polaris_last_active_project_id', () => null)

// 2. ワークスペースを開いた時にIDを記憶
watch(() => route.params.id, (newId) => {
  if (newId && typeof newId === 'string') {
    lastActiveProjectId.value = newId
  }
}, { immediate: true })

// 3. activeProjectId を現在のルートID または 直前のアクティブIDから解決
const activeProjectId = computed(() => {
  return (route.params.id as string) || lastActiveProjectId.value || ''
})

const activeProject = computed(() => {
  return projects.value.find(p => p.id === activeProjectId.value) || null
})

const unreadCount = computed(() => notifications.value.filter(n => !n.is_read).length)

const isMobileSidebarOpen = ref(false)
const showNotifications = ref(false)
const showNewProjectModal = ref(false)
const showAccountModal = ref(false)
const showLogoutModal = ref(false)
const isLoggingOut = ref(false)

watch(() => route.fullPath, () => {
  isMobileSidebarOpen.value = false
})

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

const accountTab = ref<'profile' | 'pat' | 'mfa'>('profile')
const patList = ref<PersonalAccessToken[]>([])
const newPlainToken = ref('')
const newPatForm = reactive({
  name: '',
  expires_in_days: 90,
  scopes: ['read:projects', 'read:tasks', 'write:tasks', 'read:issues', 'write:issues']
})

// MFA Management State
const mfaStatus = ref<MFAStatus | null>(null)
const isLoadingMfa = ref(false)
const showTotpSetupCard = ref(false)
const totpSetupData = reactive({ secret: '', otpauth_url: '' })
const totpConfirmCode = ref('')
const isEnablingTotp = ref(false)
const showDisableTotpModal = ref(false)
const disableTotpPassword = ref('')
const isDisablingTotp = ref(false)
const newWebAuthnKeyName = ref('')
const isRegisteringWebAuthn = ref(false)
const showBackupCodesModal = ref(false)
const newlyGeneratedBackupCodes = ref<string[]>([])
const enforcedSetupTab = ref<'totp' | 'webauthn'>('totp')

const isMfaEnforcedPending = computed(() => {
  if (isPublicRoute.value || !currentUser.value) return false
  const enforced = Boolean(currentUser.value.mfa_enforced || mfaStatus.value?.system_mfa_required || mfaStatus.value?.mfa_enforced)
  return enforced && !currentUser.value.mfa_enabled && !mfaStatus.value?.mfa_enabled
})

async function loadMFAStatus() {
  if (isPublicRoute.value || !currentUser.value) return
  isLoadingMfa.value = true
  try {
    mfaStatus.value = await apiFetch<MFAStatus>('/auth/mfa/status')
  } catch {
    mfaStatus.value = null
  } finally {
    isLoadingMfa.value = false
  }
}

async function startTOTPSetup() {
  try {
    const res = await apiFetch<{ secret: string; otpauth_url: string }>('/auth/mfa/totp/setup', { method: 'POST' })
    totpSetupData.secret = res.secret
    totpSetupData.otpauth_url = res.otpauth_url
    totpConfirmCode.value = ''
    showTotpSetupCard.value = true
  } catch (e: any) {
    notifyToast(e.message || '認証アプリのセットアップ開始に失敗しました', 'error')
  }
}

async function confirmEnableTOTP() {
  if (totpConfirmCode.value.length !== 6 || isEnablingTotp.value) return
  isEnablingTotp.value = true
  try {
    const res = await apiFetch<{ status: string; backup_codes: string[] }>('/auth/mfa/totp/enable', {
      method: 'POST',
      body: JSON.stringify({
        secret: totpSetupData.secret,
        code: totpConfirmCode.value.trim()
      })
    })
    showTotpSetupCard.value = false
    totpConfirmCode.value = ''
    notifyToast('認証アプリ (TOTP) を有効化しました', 'success')
    await refreshCurrentUser()
    await loadMFAStatus()
    if (res.backup_codes && res.backup_codes.length > 0) {
      newlyGeneratedBackupCodes.value = res.backup_codes
      showBackupCodesModal.value = true
    }
  } catch (e: any) {
    notifyToast(e.message || '認証コードが正しくありません', 'error')
  } finally {
    isEnablingTotp.value = false
  }
}

async function disableTOTP() {
  if (!disableTotpPassword.value || isDisablingTotp.value) return
  isDisablingTotp.value = true
  try {
    await apiFetch('/auth/mfa/totp/disable', {
      method: 'POST',
      body: JSON.stringify({ password: disableTotpPassword.value })
    })
    showDisableTotpModal.value = false
    disableTotpPassword.value = ''
    notifyToast('認証アプリ (TOTP) を無効化しました', 'success')
    await refreshCurrentUser()
    await loadMFAStatus()
  } catch (e: any) {
    notifyToast(e.message || '無効化に失敗しました。パスワードをご確認ください', 'error')
  } finally {
    isDisablingTotp.value = false
  }
}

async function registerWebAuthnKey() {
  if (!newWebAuthnKeyName.value.trim() || isRegisteringWebAuthn.value) return
  isRegisteringWebAuthn.value = true
  try {
    const opts = await apiFetch<any>('/auth/mfa/webauthn/register-options', { method: 'POST' })
    const attestation = await performWebAuthnRegister(opts)
    const res = await apiFetch<{ status: string; backup_codes?: string[] }>('/auth/mfa/webauthn/register-verify', {
      method: 'POST',
      body: JSON.stringify({
        name: newWebAuthnKeyName.value.trim(),
        ...attestation
      })
    })
    newWebAuthnKeyName.value = ''
    notifyToast('セキュリティキーを登録しました', 'success')
    await refreshCurrentUser()
    await loadMFAStatus()
    if (res.backup_codes && res.backup_codes.length > 0) {
      newlyGeneratedBackupCodes.value = res.backup_codes
      showBackupCodesModal.value = true
    }
  } catch (e: any) {
    notifyToast(e.message || 'セキュリティキーの登録に失敗しました', 'error')
  } finally {
    isRegisteringWebAuthn.value = false
  }
}

async function deleteWebAuthnKey(key: WebAuthnCredential) {
  if (!confirm(`セキュリティキー「${key.name}」を削除しますか？`)) return
  try {
    await apiFetch(`/auth/mfa/webauthn/keys/${key.id}`, { method: 'DELETE' })
    notifyToast('セキュリティキーを削除しました', 'success')
    await refreshCurrentUser()
    await loadMFAStatus()
  } catch (e: any) {
    notifyToast(e.message || 'セキュリティキーの削除に失敗しました', 'error')
  }
}

async function regenerateBackupCodes() {
  if (!confirm('新しいバックアップコード8件を再生成しますか？\n（以前のコードはすべて無効化されます）')) return
  try {
    const res = await apiFetch<{ backup_codes: string[] }>('/auth/mfa/backup-codes/regenerate', {
      method: 'POST',
      body: JSON.stringify({ password: '' })
    })
    await loadMFAStatus()
    if (res.backup_codes && res.backup_codes.length > 0) {
      newlyGeneratedBackupCodes.value = res.backup_codes
      showBackupCodesModal.value = true
    }
    notifyToast('バックアップコードを再生成しました', 'success')
  } catch (e: any) {
    notifyToast(e.message || 'バックアップコードの再生成に失敗しました', 'error')
  }
}

function copyBackupCodes() {
  if (!newlyGeneratedBackupCodes.value.length) return
  const text = newlyGeneratedBackupCodes.value.join('\n')
  copyToken(text)
}

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
  await Promise.all([refreshProjects(), refreshNotifications(), loadMFAStatus()])
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
      await Promise.all([refreshProjects(), refreshNotifications(), loadMFAStatus()])
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

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && showLogoutModal.value && !isLoggingOut.value) {
    showLogoutModal.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', onDocumentClick)
  window.addEventListener('keydown', onKeydown)
})

onUnmounted(() => {
  document.removeEventListener('click', onDocumentClick)
  window.removeEventListener('keydown', onKeydown)
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
  loadMFAStatus()
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

const forcedPwdForm = reactive({
  current_password: '',
  new_password: '',
  confirm_password: ''
})
const forcedPwdError = ref('')
const isSubmittingForcedPwd = ref(false)

async function submitForcedPasswordChange() {
  forcedPwdError.value = ''
  if (!forcedPwdForm.current_password) {
    forcedPwdError.value = '現在の仮パスワードを入力してください'
    return
  }
  if (forcedPwdForm.new_password.length < 8) {
    forcedPwdError.value = '新しいパスワードは8文字以上で入力してください'
    return
  }
  if (forcedPwdForm.new_password !== forcedPwdForm.confirm_password) {
    forcedPwdError.value = '新しいパスワードが一致しません'
    return
  }

  isSubmittingForcedPwd.value = true
  try {
    try {
      await apiFetch('/users/me/password', {
        method: 'PUT',
        body: JSON.stringify({
          current_password: forcedPwdForm.current_password,
          new_password: forcedPwdForm.new_password
        })
      })
    } catch (e: any) {
      if (e?.status === 404 || e?.statusCode === 404 || e?.message?.includes('404')) {
        await apiFetch('/auth/me/password', {
          method: 'PUT',
          body: JSON.stringify({
            current_password: forcedPwdForm.current_password,
            new_password: forcedPwdForm.new_password
          })
        })
      } else {
        throw e
      }
    }
    if (currentUser.value) {
      currentUser.value.must_change_password = false
    }
    forcedPwdForm.current_password = ''
    forcedPwdForm.new_password = ''
    forcedPwdForm.confirm_password = ''
    await refreshCurrentUser()
    notifyToast('パスワードを更新しました', 'success')
  } catch (err: any) {
    forcedPwdError.value = err.message || 'パスワードの変更に失敗しました'
    notifyToast(err.message || 'パスワードの変更に失敗しました', 'error')
  } finally {
    isSubmittingForcedPwd.value = false
  }
}

function openLogoutModal() {
  showLogoutModal.value = true
}

function logout() {
  openLogoutModal()
}

async function executeLogout() {
  if (isLoggingOut.value) return
  isLoggingOut.value = true
  try {
    await apiFetch('/auth/logout', { method: 'POST' })
  } finally {
    currentUser.value = null
    showAccountModal.value = false
    showLogoutModal.value = false
    isLoggingOut.value = false
    router.push('/login')
  }
}
</script>

