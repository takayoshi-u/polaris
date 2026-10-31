<template>
  <div style="display: flex; flex-direction: column; gap: 18px;">
    <!-- Page Header -->
    <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; border-bottom: 1px solid var(--border-color); pb-3;">
      <div>
        <h1 style="font-size: 20px; font-weight: 700;">システム全体設定</h1>
        <p style="font-size: 12px; color: var(--text-muted); margin-top: 2px;">
          ユーザー管理、直接登録、メール送信、全体グループ、デフォルト設定をカテゴリーごとに管理します。
        </p>
      </div>
    </div>

    <!-- Category Tabs Navigation -->
    <div style="display: flex; flex-wrap: wrap; gap: 6px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'users' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('users')"
      >
        👤 ユーザー管理・招待 ({{ users.length }})
      </button>
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'general' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('general')"
      >
        ⚙️ 基本設定・ドメイン制限
      </button>
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'smtp' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('smtp')"
      >
        ✉️ メール送信・SMTP
      </button>
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'groups_perms' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('groups_perms')"
      >
        👥 グループ ＆ システム権限
      </button>
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'defaults' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('defaults')"
      >
        🏷️ ステータス・属性・ラベル
      </button>
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'templates' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('templates')"
      >
        📋 システム標準テンプレート
      </button>
      <button
        type="button"
        class="btn btn-sm"
        :class="activeTab === 'webhooks' ? 'btn-primary' : 'btn-secondary'"
        @click="switchTab('webhooks')"
      >
        🔗 全体Webhook連携
      </button>
    </div>

    <!-- ========================================== -->
    <!-- TAB 1: USERS & INVITATIONS                -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'users'" style="display: flex; flex-direction: column; gap: 16px;">
      <!-- Registration & Invite Panels -->
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(380px, 1fr)); gap: 14px;">
        <!-- Direct Account Registration -->
        <div class="card" style="display: flex; flex-direction: column; gap: 10px;">
          <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
            <div style="font-size: 13px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
              <span>👤</span><span>アカウントを直接登録</span>
            </div>
            <span class="badge badge-green" style="font-size: 10px;">パスワード設定</span>
          </div>
          <form style="display: flex; flex-direction: column; gap: 8px;" @submit.prevent="createDirectUser">
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px;">
              <div>
                <label class="field-label">メールアドレス *</label>
                <input v-model="directUserForm.email" type="email" required placeholder="user@example.com" style="width: 100%;" />
              </div>
              <div>
                <label class="field-label">表示名 *</label>
                <input v-model="directUserForm.display_name" type="text" required placeholder="山田 太郎" style="width: 100%;" />
              </div>
            </div>
            <div>
              <label class="field-label">初期パスワード（8文字以上） *</label>
              <div style="display: flex; gap: 8px;">
                <div style="position: relative; flex: 1; display: flex; align-items: center;">
                  <input
                    v-model="directUserForm.password"
                    :type="showDirectPassword ? 'text' : 'password'"
                    minlength="8"
                    required
                    placeholder="初期パスワードを入力..."
                    style="width: 100%; padding-right: 38px;"
                  />
                  <button
                    type="button"
                    class="btn btn-ghost btn-sm"
                    style="position: absolute; right: 4px; padding: 4px 6px; font-size: 13px;"
                    title="パスワードの表示/非表示を切り替え"
                    @click="showDirectPassword = !showDirectPassword"
                  >
                    {{ showDirectPassword ? '🙈' : '👁️' }}
                  </button>
                </div>
                <button type="submit" class="btn btn-primary btn-sm" :disabled="submittingUser">
                  {{ submittingUser ? '処理中...' : '＋ 直接登録' }}
                </button>
              </div>
            </div>
          </form>
        </div>

        <!-- Email Invitation -->
        <div class="card" style="display: flex; flex-direction: column; gap: 10px;">
          <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
            <div style="font-size: 13px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
              <span>✉️</span><span>招待メールで登録</span>
            </div>
            <div style="display: flex; align-items: center; gap: 6px;">
              <span class="badge badge-amber" style="font-size: 10px;">72時間有効</span>
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                style="padding: 2px 8px; font-size: 11px;"
                @click="showBulkInviteModal = true"
              >
                📥 リストから一括招待
              </button>
            </div>
          </div>
          <form style="display: flex; flex-direction: column; gap: 8px; justify-content: space-between; flex: 1;" @submit.prevent="inviteNewUser">
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px;">
              <div>
                <label class="field-label">招待先メールアドレス *</label>
                <input v-model="inviteForm.email" type="email" required placeholder="newcomer@example.com" style="width: 100%;" />
              </div>
              <div>
                <label class="field-label">表示名 *</label>
                <input v-model="inviteForm.display_name" type="text" required placeholder="佐藤 健" style="width: 100%;" />
              </div>
            </div>
            <div style="display: flex; justify-content: space-between; align-items: center; padding-top: 4px;">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                @click="showBulkInviteModal = true"
              >
                📥 リストから一括招待
              </button>
              <button type="submit" class="btn btn-secondary btn-sm" :disabled="submittingUser">
                {{ submittingUser ? '送信中...' : '✉️ 招待メールを送信' }}
              </button>
            </div>
          </form>
        </div>
      </div>

      <!-- Action Feedback Banner -->
      <div
        v-if="userActionFeedback.text"
        :style="{
          padding: '10px 14px',
          borderRadius: '8px',
          border: `1px solid ${userActionFeedback.type === 'error' ? '#ef4444' : '#10b981'}`,
          backgroundColor: userActionFeedback.type === 'error' ? 'rgba(239, 68, 68, 0.14)' : 'rgba(16, 185, 129, 0.14)',
          fontSize: '12px',
          display: 'flex',
          flexDirection: 'column',
          gap: '6px'
        }"
      >
        <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
          <strong :style="{ color: userActionFeedback.type === 'error' ? 'var(--danger)' : 'var(--success)' }">
            {{ userActionFeedback.text }}
          </strong>
          <button type="button" class="btn btn-ghost btn-sm" style="padding: 2px 6px;" @click="userActionFeedback.text = ''; userActionFeedback.inviteUrl = ''">
            閉じる
          </button>
        </div>
        <div v-if="userActionFeedback.inviteUrl" style="display: flex; flex-wrap: wrap; align-items: center; gap: 8px; margin-top: 2px;">
          <span style="color: var(--text-muted);">招待リンク（手動共有用）:</span>
          <code style="padding: 3px 8px; border-radius: 4px; background: var(--bg-elevated); border: 1px solid var(--border-color); color: var(--text-main); font-size: 11px; word-break: break-all;">
            {{ userActionFeedback.inviteUrl }}
          </code>
          <button type="button" class="btn btn-secondary btn-sm" style="padding: 2px 8px; font-size: 11px;" @click="copyInviteUrl(userActionFeedback.inviteUrl)">
            URLをコピー
          </button>
        </div>
      </div>

      <!-- User List (Full Width Table) -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px; padding: 0; overflow: hidden;">
        <div style="padding: 12px 16px; border-bottom: 1px solid var(--border-color); display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px;">
          <div style="display: flex; align-items: center; gap: 10px; flex-wrap: wrap;">
            <h3 style="font-size: 14px; font-weight: 700;">登録ユーザー一覧</h3>
            <div style="display: flex; border: 1px solid var(--border-color); border-radius: 6px; overflow: hidden;">
              <button
                type="button"
                class="btn btn-sm"
                :class="userStatusFilter === 'all' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px; font-size: 12px;"
                @click="userStatusFilter = 'all'"
              >
                全員 ({{ users.length }})
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="userStatusFilter === 'active' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px; font-size: 12px;"
                @click="userStatusFilter = 'active'"
              >
                有効 ({{ users.filter(u => u.status === 'active').length }})
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="userStatusFilter === 'invited' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px; font-size: 12px;"
                @click="userStatusFilter = 'invited'"
              >
                招待中 ({{ users.filter(u => u.status === 'invited').length }})
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="userStatusFilter === 'disabled' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px; font-size: 12px;"
                @click="userStatusFilter = 'disabled'"
              >
                無効 ({{ users.filter(u => u.status === 'disabled').length }})
              </button>
            </div>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              @click="showBulkInviteModal = true"
            >
              📥 リストから一括招待
            </button>
            <input
              v-model="userSearchQuery"
              type="text"
              placeholder="表示名・メールで絞り込み..."
              style="width: 220px; font-size: 13px;"
            />
          </div>
        </div>

        <div style="overflow-x: auto;">
          <table style="width: 100%; border-collapse: collapse; font-size: 13px; text-align: left;">
            <thead>
              <tr style="background-color: var(--bg-input); border-bottom: 1px solid var(--border-color); color: var(--text-muted);">
                <th style="padding: 8px 16px; font-weight: 600;">ステータス</th>
                <th style="padding: 8px 16px; font-weight: 600;">表示名</th>
                <th style="padding: 8px 16px; font-weight: 600;">メールアドレス</th>
                <th style="padding: 8px 16px; font-weight: 600;">システム権限</th>
                <th style="padding: 8px 16px; font-weight: 600; text-align: right;">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="u in paginatedUsers"
                :key="u.id"
                style="border-bottom: 1px solid var(--border-color);"
              >
                <td style="padding: 10px 16px;">
                  <span
                    :class="[
                      'badge',
                      u.status === 'active' ? 'badge-green' : u.status === 'invited' ? 'badge-amber' : 'badge-red'
                    ]"
                  >
                    {{ u.status === 'active' ? '有効 (ACTIVE)' : u.status === 'invited' ? '招待中 (INVITED)' : '無効 (DISABLED)' }}
                  </span>
                </td>
                <td style="padding: 10px 16px; font-weight: 700;">
                  {{ u.display_name }}
                </td>
                <td style="padding: 10px 16px; color: var(--text-muted);">
                  {{ u.email }}
                </td>
                <td style="padding: 10px 16px;">
                  <div v-if="u.permissions && u.permissions.length > 0" style="display: flex; flex-wrap: wrap; gap: 4px;">
                    <span v-for="p in u.permissions" :key="p" class="badge badge-amber" style="font-size: 11px;">
                      {{ getPermLabel(p) }}
                    </span>
                  </div>
                  <span v-else style="color: var(--text-muted);">-</span>
                </td>
                <td style="padding: 10px 16px; text-align: right;">
                  <div style="display: inline-flex; align-items: center; gap: 6px;">
                    <button
                      v-if="u.status === 'invited'"
                      class="btn btn-secondary btn-sm"
                      @click="resendInvite(u.id)"
                    >
                      招待再送
                    </button>
                    <button
                      class="btn btn-secondary btn-sm"
                      @click="promptAdminEmailChange(u)"
                    >
                      メール変更
                    </button>
                    <button
                      v-if="u.status !== 'disabled'"
                      class="btn btn-danger btn-sm"
                      :disabled="u.id === currentUser?.id"
                      :title="u.id === currentUser?.id ? '自身のアカウントは無効化できません' : ''"
                      @click="toggleUserStatus(u.id, 'disabled')"
                    >
                      無効化
                    </button>
                    <button
                      v-else
                      class="btn btn-primary btn-sm"
                      @click="toggleUserStatus(u.id, 'active')"
                    >
                      有効化
                    </button>
                  </div>
                </td>
              </tr>
              <tr v-if="filteredUsers.length === 0">
                <td colspan="5" style="padding: 24px; text-align: center; color: var(--text-muted);">
                  該当するユーザーは見つかりませんでした
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination Controls -->
        <div
          v-if="filteredUsers.length > userPageSize"
          style="display: flex; align-items: center; justify-content: space-between; padding: 10px 16px; border-top: 1px solid var(--border-color); font-size: 12px; color: var(--text-muted);"
        >
          <div>
            {{ (userPage - 1) * userPageSize + 1 }} - {{ Math.min(userPage * userPageSize, filteredUsers.length) }} / 全 {{ filteredUsers.length }} 件
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="userPage <= 1"
              @click="userPage--"
            >
              前へ
            </button>
            <span style="font-weight: 600; color: var(--text-main);">
              {{ userPage }} / {{ totalUserPages }}
            </span>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              :disabled="userPage >= totalUserPages"
              @click="userPage++"
            >
              次へ
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- TAB 2: GENERAL SETTINGS & DOMAIN RESTRICT -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'general'" style="display: flex; flex-direction: column; gap: 16px;">
      <div class="card" style="display: flex; flex-direction: column; gap: 16px; max-width: 800px;">
        <div style="border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <h2 style="font-size: 15px; font-weight: 700;">システム基本設定 ＆ 許可メールドメイン制限</h2>
          <p style="font-size: 12px; color: var(--text-muted); margin-top: 2px;">
            1日あたりの基準時間初期値と、ユーザー登録を許可するメールドメインを設定します。
          </p>
        </div>

        <form style="display: flex; flex-direction: column; gap: 14px;" @submit.prevent="saveSystemSettings">
          <div>
            <label class="field-label">1日あたりのデフォルト基準時間</label>
            <div style="display: flex; align-items: center; gap: 8px;">
              <input v-model.number="sysForm.default_hours_per_day" type="number" min="1" max="24" step="0.01" required style="width: 128px;" />
              <span style="font-size: 12px; color: var(--text-muted);">時間 / 日</span>
            </div>
          </div>

          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
            <div>
              <label class="field-label">デフォルト課題テンプレート</label>
              <select v-model="sysForm.default_issue_template_id" style="width: 100%;">
                <option v-for="t in sysTemplates.filter(x => x.target_type === 'issue')" :key="t.id" :value="t.id">{{ t.name }}</option>
              </select>
            </div>
            <div>
              <label class="field-label">デフォルトタスクテンプレート</label>
              <select v-model="sysForm.default_task_template_id" style="width: 100%;">
                <option v-for="t in sysTemplates.filter(x => x.target_type === 'task')" :key="t.id" :value="t.id">{{ t.name }}</option>
              </select>
            </div>
          </div>

          <div>
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px;">
              <label class="field-label" style="margin-bottom: 0;">
                許可メールドメイン制限 ({{ allowedDomainsList.length }} / 10件)
              </label>
              <span style="font-size: 11px; color: var(--text-muted);">空の場合は全ドメイン許可</span>
            </div>

            <!-- Domain Tag Pills -->
            <div style="display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; min-height: 36px; padding: 6px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); align-items: center;">
              <span
                v-for="(dom, idx) in allowedDomainsList"
                :key="dom"
                class="badge badge-slate"
                style="display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; font-size: 12px; font-family: monospace;"
              >
                {{ dom }}
                <button
                  type="button"
                  style="background: none; border: none; cursor: pointer; color: var(--danger); font-size: 12px; line-height: 1; padding: 0 2px;"
                  title="ドメインを削除"
                  @click="removeAllowedDomain(idx)"
                >✕</button>
              </span>
              <span v-if="allowedDomainsList.length === 0" style="color: var(--text-muted); font-size: 12px; padding: 2px 4px;">
                ドメイン制限なし（すべてのメールドメインからの登録を許可）
              </span>
            </div>

            <!-- Add Domain Input -->
            <div style="display: flex; gap: 8px; max-width: 420px;">
              <input
                v-model="newDomainInput"
                type="text"
                placeholder="例: example.com (Enterで追加)"
                :disabled="allowedDomainsList.length >= 10"
                style="flex: 1;"
                @keydown.enter.prevent="addAllowedDomain"
              />
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="allowedDomainsList.length >= 10 || !newDomainInput.trim()"
                @click="addAllowedDomain"
              >
                ＋ 追加
              </button>
            </div>
          </div>

          <div style="padding-top: 4px;">
            <button type="submit" class="btn btn-primary btn-sm">基本設定を保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- TAB 3: SMTP / MAIL DELIVERY               -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'smtp'" style="display: flex; flex-direction: column; gap: 16px;">
      <div class="card" style="display: flex; flex-direction: column; gap: 16px; max-width: 800px;">
        <div style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div>
            <h2 style="font-size: 15px; font-weight: 700;">メール送信・SMTP設定</h2>
            <p style="font-size: 12px; color: var(--text-muted); margin-top: 2px;">
              招待メールやパスワード再設定メールの送信元およびリレーサーバーを設定します。
            </p>
          </div>
          <span :class="['badge', smtpForm.enabled ? 'badge-blue' : 'badge-green']">
            {{ smtpForm.enabled ? '外部SMTPサーバー使用中' : '標準メール送信（内蔵）' }}
          </span>
        </div>

        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="saveSMTPSettings">
          <label style="display: flex; align-items: center; gap: 8px; font-size: 13px; font-weight: 600; cursor: pointer;">
            <input v-model="smtpForm.enabled" type="checkbox" />
            <span>外部 SMTP サーバーを使用する</span>
          </label>

          <div style="display: grid; grid-template-columns: 2fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">SMTP ホスト</label>
              <input v-model="smtpForm.host" type="text" style="width: 100%;" placeholder="smtp.example.com" />
            </div>
            <div>
              <label class="field-label">ポート</label>
              <input v-model.number="smtpForm.port" type="number" style="width: 100%;" />
            </div>
          </div>

          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">送信元メールアドレス (From)</label>
              <input v-model="smtpForm.from_email" type="email" style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">送信元表示名</label>
              <input v-model="smtpForm.from_name" type="text" style="width: 100%;" />
            </div>
          </div>

          <label style="display: flex; align-items: center; gap: 8px; font-size: 12px; cursor: pointer;">
            <input v-model="smtpForm.use_auth" type="checkbox" />
            <span>SMTP AUTH 認証を使用する</span>
          </label>

          <div v-if="smtpForm.use_auth" style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <label class="field-label">SMTP ユーザー名</label>
              <input v-model="smtpForm.username" type="text" style="width: 100%;" />
            </div>
            <div>
              <label class="field-label">SMTP パスワード（変更時のみ入力）</label>
              <input v-model="smtpForm.password" type="password" style="width: 100%;" :placeholder="smtpForm.has_password ? '設定済み' : ''" />
            </div>
          </div>

          <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px; border-top: 1px solid var(--border-color); padding-top: 12px;">
            <button type="submit" class="btn btn-primary btn-sm">メール設定を保存</button>
            <div style="display: flex; gap: 6px;">
              <input v-model="testEmailTo" type="email" placeholder="テスト送信先メール..." style="width: 220px;" />
              <button type="button" class="btn btn-secondary btn-sm" :disabled="isSendingTestEmail" @click="sendTestMail">
                <span v-if="isSendingTestEmail">⏳ 送信中...</span>
                <span v-else>テスト送信</span>
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- TAB 4: GROUPS & SYSTEM PERMISSIONS         -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'groups_perms'" style="display: flex; flex-direction: column; gap: 16px;">
      <!-- Global Groups -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
        <div style="border-bottom: 1px solid var(--border-color); padding-bottom: 8px; display: flex; align-items: center; justify-content: space-between;">
          <div>
            <h3 style="font-size: 14px; font-weight: 700;">全体グループ管理（全プロジェクト横断）</h3>
            <p style="font-size: 11px; color: var(--text-muted);">全プロジェクトから直接参照・参加可能なシステム全体グループです。</p>
          </div>
        </div>

        <form style="display: flex; gap: 8px; max-width: 480px;" @submit.prevent="createGlobalGroup">
          <input v-model="groupForm.name" type="text" required style="flex: 1;" placeholder="新しいグループ名を入力..." />
          <button type="submit" class="btn btn-primary btn-sm">＋ 作成</button>
        </form>

        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(320px, 1fr)); gap: 10px;">
          <div
            v-for="g in globalGroups"
            :key="g.id"
            style="padding: 10px 12px; border-radius: 8px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; flex-direction: column; gap: 8px; font-size: 12px;"
          >
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div style="display: flex; align-items: center; gap: 6px;">
                <strong>👥 {{ g.name }}</strong>
                <span style="color: var(--text-muted); font-size: 11px;">({{ g.member_ids.length }}名)</span>
                <button
                  type="button"
                  class="btn btn-ghost btn-sm"
                  style="padding: 1px 4px; font-size: 12px;"
                  title="グループ名を変更"
                  @click="openRenameGroupModal(g)"
                >
                  ✏️
                </button>
              </div>
              <button class="btn btn-danger btn-sm" @click="deleteGlobalGroup(g.id)">削除</button>
            </div>
            <!-- Tag/Chip List of Current Members -->
            <div style="display: flex; flex-wrap: wrap; gap: 6px; max-height: 110px; overflow-y: auto; padding: 2px 0;">
              <span
                v-for="mid in g.member_ids"
                :key="mid"
                class="badge badge-slate"
                style="display: inline-flex; align-items: center; gap: 4px; padding: 2px 8px; font-size: 11px;"
              >
                <span>👤 {{ getUserName(mid) }}</span>
                <button
                  type="button"
                  title="メンバーから除外"
                  style="background: none; border: none; cursor: pointer; color: var(--danger); font-size: 11px; padding: 0 2px; line-height: 1;"
                  @click="toggleGroupMember(g, mid)"
                >
                  ✕
                </button>
              </span>
              <span v-if="g.member_ids.length === 0" style="color: var(--text-muted); font-size: 11px; padding: 2px 0;">
                所属メンバーなし
              </span>
            </div>

            <!-- Autocomplete Search Input to Add Member -->
            <div style="position: relative;">
              <input
                v-model="groupSearchMap[g.id]"
                type="text"
                placeholder="＋ ユーザー名・メールで検索して追加..."
                style="width: 100%; font-size: 11px; padding: 5px 8px;"
                @focus="activeGroupSearchId = g.id"
              />
              <div
                v-if="activeGroupSearchId === g.id && getAvailableGroupUsers(g, groupSearchMap[g.id]).length > 0 && groupSearchMap[g.id]?.trim()"
                style="position: absolute; top: 100%; left: 0; right: 0; max-height: 140px; overflow-y: auto; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 6px; box-shadow: 0 4px 12px rgba(0,0,0,0.12); z-index: 25; margin-top: 2px;"
              >
                <div
                  v-for="u in getAvailableGroupUsers(g, groupSearchMap[g.id])"
                  :key="u.id"
                  style="padding: 6px 10px; font-size: 11px; cursor: pointer; display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border-color);"
                  @click="addGroupMember(g, u.id)"
                >
                  <span>👤 {{ u.display_name }} ({{ u.email }})</span>
                  <span style="color: var(--primary); font-weight: 700;">＋ 追加</span>
                </div>
              </div>
            </div>
          </div>
          <div v-if="globalGroups.length === 0" style="padding: 16px; color: var(--text-muted); font-size: 12px;">
            全体グループはまだ作成されていません
          </div>
        </div>
      </div>

      <!-- System Permissions Matrix -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px; padding: 0; overflow: hidden;">
        <div style="padding: 12px 16px; border-bottom: 1px solid var(--border-color); display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px;">
          <div>
            <h3 style="font-size: 14px; font-weight: 700;">システム権限マトリクス付与</h3>
            <p style="font-size: 11px; color: var(--text-muted); margin-top: 2px;">
              縦に対象（人・グループ）、横にシステム権限チェックボックスを配置した一覧です。クリックで即時切り替わります。
            </p>
          </div>
          <div style="display: flex; align-items: center; gap: 8px;">
            <div style="display: flex; border: 1px solid var(--border-color); border-radius: 6px; overflow: hidden;">
              <button
                type="button"
                class="btn btn-sm"
                :class="permMatrixFilter === 'all' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px;"
                @click="permMatrixFilter = 'all'"
              >
                全員 ({{ users.length + globalGroups.length }})
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="permMatrixFilter === 'user' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px;"
                @click="permMatrixFilter = 'user'"
              >
                👤 ユーザー ({{ users.length }})
              </button>
              <button
                type="button"
                class="btn btn-sm"
                :class="permMatrixFilter === 'group' ? 'btn-primary' : 'btn-ghost'"
                style="border-radius: 0; padding: 4px 10px;"
                @click="permMatrixFilter = 'group'"
              >
                👥 グループ ({{ globalGroups.length }})
              </button>
            </div>
            <input
              v-model="permMatrixSearch"
              type="text"
              placeholder="対象名・メールで検索..."
              style="width: 200px; font-size: 13px;"
            />
          </div>
        </div>

        <div style="overflow-x: auto;">
          <table style="width: 100%; border-collapse: collapse; font-size: 13px; text-align: left;">
            <thead>
              <tr style="background-color: var(--bg-input); border-bottom: 1px solid var(--border-color); color: var(--text-muted);">
                <th style="padding: 10px 16px; font-weight: 600; width: 280px;">対象ユーザー / グループ</th>
                <th style="padding: 10px 16px; font-weight: 600; width: 90px; text-align: center;">種別</th>
                <th style="padding: 10px 16px; font-weight: 600; text-align: center; width: 140px;">
                  <div>プロジェクト作成</div>
                  <div style="font-size: 11px; color: var(--text-muted); font-weight: 400;">create_project</div>
                </th>
                <th style="padding: 10px 16px; font-weight: 600; text-align: center; width: 140px;">
                  <div>プロジェクト削除</div>
                  <div style="font-size: 11px; color: var(--text-muted); font-weight: 400;">delete_project</div>
                </th>
                <th style="padding: 10px 16px; font-weight: 600; text-align: center; width: 140px;">
                  <div>システム全体管理</div>
                  <div style="font-size: 11px; color: var(--text-muted); font-weight: 400;">manage_system</div>
                </th>
                <th style="padding: 10px 16px; font-weight: 600; text-align: right;">権限サマリー</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="sub in matrixSubjects"
                :key="`${sub.type}-${sub.id}`"
                style="border-bottom: 1px solid var(--border-color);"
              >
                <!-- Subject Info -->
                <td style="padding: 10px 16px;">
                  <div style="display: flex; align-items: center; gap: 8px;">
                    <div
                      style="width: 30px; height: 30px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 12px; flex-shrink: 0;"
                      :style="{
                        backgroundColor: sub.type === 'user' ? '#2563eb' : '#4f46e5',
                        color: '#fff'
                      }"
                    >
                      {{ sub.name.slice(0, 2) }}
                    </div>
                    <div>
                      <div style="font-weight: 700; color: var(--text-main);">{{ sub.name }}</div>
                      <div v-if="sub.type === 'user'" style="color: var(--text-muted); font-size: 12px;">
                        {{ sub.subText }}
                      </div>
                      <div v-else style="display: flex; align-items: center; gap: 6px; margin-top: 2px;">
                        <span style="font-size: 12px; color: var(--text-muted);">{{ sub.memberCount }}名所属</span>
                        <button
                          type="button"
                          class="btn btn-ghost btn-sm"
                          style="padding: 1px 6px; font-size: 11px; color: var(--primary);"
                          @click="previewGroup = globalGroups.find(g => g.id === sub.id) || null"
                        >
                          ℹ️ メンバー確認
                        </button>
                      </div>
                    </div>
                  </div>
                </td>

                <!-- Type Badge -->
                <td style="padding: 10px 16px; text-align: center;">
                  <span
                    :class="['badge', sub.type === 'user' ? 'badge-slate' : 'badge-blue']"
                    style="font-size: 11px;"
                  >
                    {{ sub.type === 'user' ? '👤 ユーザー' : '👥 グループ' }}
                  </span>
                </td>

                <!-- Checkbox 1: create_project -->
                <td style="padding: 10px 16px; text-align: center;">
                  <input
                    type="checkbox"
                    :checked="hasPerm(sub.type, sub.id, 'create_project')"
                    style="cursor: pointer; width: 16px; height: 16px;"
                    @change="toggleSystemPerm(sub.type, sub.id, 'create_project')"
                  />
                </td>

                <!-- Checkbox 2: delete_project -->
                <td style="padding: 10px 16px; text-align: center;">
                  <input
                    type="checkbox"
                    :checked="hasPerm(sub.type, sub.id, 'delete_project')"
                    style="cursor: pointer; width: 16px; height: 16px;"
                    @change="toggleSystemPerm(sub.type, sub.id, 'delete_project')"
                  />
                </td>

                <!-- Checkbox 3: manage_system -->
                <td style="padding: 10px 16px; text-align: center;">
                  <input
                    type="checkbox"
                    :checked="hasPerm(sub.type, sub.id, 'manage_system')"
                    :disabled="sub.type === 'user' && sub.id === currentUser?.id"
                    :title="sub.type === 'user' && sub.id === currentUser?.id ? '自身のアカウントのシステム管理者権限は変更できません' : ''"
                    style="cursor: pointer; width: 16px; height: 16px;"
                    @change="toggleSystemPerm(sub.type, sub.id, 'manage_system')"
                  />
                </td>

                <!-- Summary Badge -->
                <td style="padding: 10px 16px; text-align: right;">
                  <div style="display: inline-flex; align-items: center; justify-content: flex-end; gap: 4px;">
                    <span
                      v-if="sub.type === 'user' && getUserEffectivePerms(sub.id).hasInherited"
                      class="badge badge-blue"
                      style="font-size: 11px;"
                    >
                      継承あり
                    </span>
                    <span
                      :class="[
                        'badge',
                        (sub.type === 'user' ? getUserEffectivePerms(sub.id).perms.has('manage_system') : hasPerm(sub.type, sub.id, 'manage_system'))
                          ? 'badge-amber'
                          : (sub.type === 'user' ? getUserEffectivePerms(sub.id).perms.has('create_project') : hasPerm(sub.type, sub.id, 'create_project'))
                          ? 'badge-green'
                          : 'badge-slate'
                      ]"
                    >
                      {{ getSubjectPermSummary(sub.type, sub.id) }}
                    </span>
                  </div>
                </td>
              </tr>
              <tr v-if="matrixSubjects.length === 0">
                <td colspan="6" style="padding: 24px; text-align: center; color: var(--text-muted);">
                  対象が見つかりませんでした
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- TAB 5: DEFAULTS (STATUSES, LABELS, FIELDS) -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'defaults'" style="display: flex; flex-direction: column; gap: 16px;">
      <!-- System Default Statuses -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
        <div style="border-bottom: 1px solid var(--border-color); padding-bottom: 8px; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px;">
          <div>
            <h3 style="font-size: 14px; font-weight: 700;">システムデフォルトステータス（1〜10）</h3>
            <p style="font-size: 11px; color: var(--text-muted);">新規プロジェクト作成時に初期設定されるステータスです。</p>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            style="font-size: 11px; font-weight: 600;"
            @click="resetSystemStatuses"
          >
            🔄 初期設定にリセット
          </button>
        </div>

        <form style="display: flex; gap: 6px; max-width: 480px;" @submit.prevent="createSystemStatus">
          <input v-model.number="sysStatusForm.display_order" type="number" min="1" max="10" required style="width: 64px;" />
          <input v-model="sysStatusForm.name" type="text" required style="flex: 1;" placeholder="ステータス名" />
          <input v-model="sysStatusForm.color" type="color" style="width: 42px; padding: 2px;" />
          <button type="submit" class="btn btn-primary btn-sm" :disabled="sysStatuses.length >= 10">追加</button>
        </form>

        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 8px;">
          <div
            v-for="st in sysStatuses"
            :key="st.id"
            style="padding: 8px 12px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; font-size: 12px;"
          >
            <span style="display: flex; align-items: center; gap: 6px;">
              <span :style="{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: st.color }"></span>
              {{ st.display_order }}. {{ st.name }}
            </span>
            <button v-if="sysStatuses.length > 1" class="btn btn-danger btn-sm" @click="deleteSystemStatus(st.id)">削除</button>
          </div>
        </div>
      </div>

      <!-- System Common Labels -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
        <div style="border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 14px; font-weight: 700;">システム共通ラベル（全プロジェクト共有）</h3>
          <p style="font-size: 11px; color: var(--text-muted);">どのプロジェクトからでも利用できる共通ラベルです。</p>
        </div>

        <form style="display: flex; gap: 6px; max-width: 420px;" @submit.prevent="createSystemLabel">
          <input v-model="sysLabelForm.name" type="text" required style="flex: 1;" placeholder="ラベル名" />
          <input v-model="sysLabelForm.color" type="color" style="width: 42px; padding: 2px;" />
          <button type="submit" class="btn btn-primary btn-sm">追加</button>
        </form>

        <div style="display: flex; flex-wrap: wrap; gap: 6px;">
          <div
            v-for="lb in sysLabels"
            :key="lb.id"
            style="padding: 4px 10px; border-radius: 6px; border: 1px solid var(--border-color); background: var(--bg-input); display: flex; align-items: center; gap: 6px; font-size: 12px;"
          >
            <span :style="{ color: lb.color, fontWeight: 700 }">● {{ lb.name }}</span>
            <button class="btn btn-danger btn-sm" style="padding: 1px 4px; font-size: 10px;" @click="deleteSystemLabel(lb.id)">✕</button>
          </div>
          <div v-if="sysLabels.length === 0" style="color: var(--text-muted); font-size: 12px;">
            共通ラベルはまだ登録されていません
          </div>
        </div>
      </div>

      <!-- System Default Custom Fields -->
      <div class="card" style="display: flex; flex-direction: column; gap: 12px;">
        <div style="border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 14px; font-weight: 700;">システムデフォルトカスタム属性</h3>
          <p style="font-size: 11px; color: var(--text-muted);">新規プロジェクト作成時にテンプレートへ組み込める属性の初期定義です。</p>
        </div>

        <form style="display: flex; flex-wrap: wrap; gap: 6px; max-width: 600px;" @submit.prevent="createSystemCustomField">
          <select v-model="sysCfForm.target_type">
            <option value="issue">課題</option>
            <option value="task">タスク</option>
          </select>
          <input v-model="sysCfForm.name" type="text" required style="flex: 1; min-width: 140px;" placeholder="項目名" />
          <select v-model="sysCfForm.field_type">
            <option value="text_single">1行テキスト</option>
            <option value="text_multi">複数行テキスト</option>
            <option value="markdown">Markdown</option>
            <option value="number">数値</option>
            <option value="date">日付</option>
            <option value="select">プルダウン</option>
            <option value="radio">ラジオ</option>
            <option value="checkbox">チェックボックス</option>
            <option value="user_group">ユーザー/グループ</option>
          </select>
          <button type="submit" class="btn btn-primary btn-sm">追加</button>
        </form>

        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 8px;">
          <div
            v-for="cf in sysCustomFields"
            :key="cf.id"
            style="padding: 8px 12px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); display: flex; justify-content: space-between; align-items: center; font-size: 12px;"
          >
            <span>
              <span :class="['badge', cf.target_type === 'issue' ? 'badge-green' : 'badge-blue']" style="margin-right: 4px; font-size: 10px;">
                {{ cf.target_type === 'issue' ? '課題' : 'タスク' }}
              </span>
              <strong>{{ cf.name }}</strong> ({{ cf.field_type }})
            </span>
            <button class="btn btn-danger btn-sm" @click="deleteSystemCustomField(cf.id)">削除</button>
          </div>
          <div v-if="sysCustomFields.length === 0" style="color: var(--text-muted); font-size: 12px;">
            デフォルトカスタム属性はありません
          </div>
        </div>
      </div>
    </div>

    <!-- ========================================== -->
    <!-- TAB 6: DEFAULT TEMPLATES BUILDER           -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'templates'">
      <TemplateBuilder
        title="システム全体デフォルトテンプレート ＆ 1〜3カラムレイアウト・項目別権限ビルダー"
        :templates="sysTemplates"
        :custom-fields="sysCustomFields"
        :users="users"
        :groups="globalGroups"
        @save="saveSystemTemplate"
        @delete="deleteSystemTemplate"
      />
    </div>

    <!-- ========================================== -->
    <!-- TAB 7: SYSTEM-WIDE OUTBOUND WEBHOOKS       -->
    <!-- ========================================== -->
    <div v-if="activeTab === 'webhooks'" style="display: flex; flex-direction: column; gap: 16px;">
      <div class="card" style="display: flex; flex-direction: column; gap: 14px;">
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 10px; border-bottom: 1px solid var(--border-color); padding-bottom: 10px;">
          <div>
            <h3 style="font-size: 15px; font-weight: 700;">システム全体 Outbound Webhook 連携</h3>
            <p style="font-size: 12px; color: var(--text-muted);">
              全プロジェクト横断の重要イベント（タスク・課題の作成やステータス変更等）をリアルタイムに外部システムへ通知します。
            </p>
          </div>
          <button class="btn btn-primary btn-sm" @click="showSysWebhookModal = true">＋ Webhook を追加</button>
        </div>

        <div style="display: flex; flex-direction: column; gap: 10px;">
          <div
            v-for="wh in sysWebhooks"
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
              <button class="btn btn-secondary btn-sm" style="font-size: 12px;" @click="openEditSysWebhookModal(wh)">編集</button>
              <button class="btn btn-secondary btn-sm" style="font-size: 12px;" @click="testSysWebhook(wh.id)">テスト送信 (Ping)</button>
              <button class="btn btn-secondary btn-sm" style="font-size: 12px;" @click="openSysWebhookDeliveries(wh)">送信ログ</button>
              <button class="btn btn-danger btn-sm" style="font-size: 12px;" @click="deleteSysWebhook(wh.id)">削除</button>
            </div>
          </div>
          <div v-if="sysWebhooks.length === 0" style="padding: 20px; text-align: center; color: var(--text-muted); font-size: 13px;">
            登録された全体Webhookはありません
          </div>
        </div>
      </div>
    </div>

    <!-- System Webhook Create Modal -->
    <div v-if="showSysWebhookModal" class="modal-backdrop" @click.self="showSysWebhookModal = false">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">新規全体 Webhook の登録</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="createSysWebhook">
          <div>
            <label class="field-label">Webhook 名 *</label>
            <input v-model="newSysWebhookForm.name" type="text" required style="width: 100%;" placeholder="例: 監査ログ受信システム" />
          </div>
          <div>
            <label class="field-label">送信先 URL (HTTP/HTTPS) *</label>
            <input v-model="newSysWebhookForm.url" type="url" required style="width: 100%;" placeholder="https://..." />
          </div>
          <div>
            <label class="field-label">シークレット鍵 (HMAC-SHA256署名用・空欄なら自動生成)</label>
            <input v-model="newSysWebhookForm.secret" type="text" style="width: 100%; font-family: monospace;" placeholder="whsec_..." />
          </div>
          <div>
            <label class="field-label">購読イベント</label>
            <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; font-size: 12px;">
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="task.created" />
                <span>task.created (タスク作成)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="task.updated" />
                <span>task.updated (タスク更新)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="task.status_changed" />
                <span>task.status_changed (タスク状態変更)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="issue.created" />
                <span>issue.created (課題作成)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="issue.state_changed" />
                <span>issue.state_changed (課題状態変更)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="comment.created" />
                <span>comment.created (コメント投稿)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px;">
                <input v-model="newSysWebhookForm.events" type="checkbox" value="attachment.created" />
                <span>attachment.created (添付ファイル追加)</span>
              </label>
            </div>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
            <button type="button" class="btn btn-secondary" @click="showSysWebhookModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary">作成する</button>
          </div>
        </form>
      </div>
    </div>

    <!-- System Webhook Deliveries Modal -->
    <div v-if="selectedSysWebhookForLogs" class="modal-backdrop" @click.self="selectedSysWebhookForLogs = null">
      <div class="modal-panel" style="max-width: 680px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 15px; font-weight: 700;">配信ログ: {{ selectedSysWebhookForLogs.name }}</h3>
          <button class="btn btn-secondary btn-sm" @click="selectedSysWebhookForLogs = null">閉じる</button>
        </div>
        <div style="display: flex; flex-direction: column; gap: 8px; max-height: 420px; overflow-y: auto;">
          <div
            v-for="log in sysWebhookDeliveries"
            :key="log.id"
            style="padding: 10px 12px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); font-size: 12px; display: flex; flex-direction: column; gap: 8px; cursor: pointer;"
            @click="expandedSysDeliveryId = expandedSysDeliveryId === log.id ? null : log.id"
          >
            <div style="display: flex; justify-content: space-between; align-items: center;">
              <div>
                <span :class="['badge', log.status_code && log.status_code >= 200 && log.status_code < 300 ? 'badge-green' : 'badge-red']" style="margin-right: 6px;">
                  {{ log.status_code || 'ERR' }}
                </span>
                <strong>{{ log.event }}</strong>
                <span style="color: var(--text-muted); margin-left: 6px;">({{ log.duration_ms }}ms)</span>
                <span style="color: var(--primary); margin-left: 8px; font-size: 11px;">
                  {{ expandedSysDeliveryId === log.id ? '▲ 閉じる' : '▼ 詳細' }}
                </span>
                <div v-if="log.error_message" style="color: var(--danger); font-size: 11px; margin-top: 2px;">
                  {{ log.error_message }}
                </div>
              </div>
              <span style="font-size: 11px; color: var(--text-muted);">{{ log.created_at.slice(0, 19).replace('T', ' ') }}</span>
            </div>

            <!-- Accordion Details (Request & Response) -->
            <div v-if="expandedSysDeliveryId === log.id" style="border-top: 1px dashed var(--border-color); padding-top: 8px; display: flex; flex-direction: column; gap: 6px;" @click.stop>
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
          <div v-if="sysWebhookDeliveries.length === 0" style="padding: 24px; text-align: center; color: var(--text-muted); font-size: 12px;">
            配信履歴はありません
          </div>
        </div>
      </div>
    </div>

    <!-- System Webhook Edit Modal -->
    <div v-if="editingSysWebhook" class="modal-backdrop" @click.self="editingSysWebhook = null">
      <div class="modal-panel" style="max-width: 520px;">
        <h3 style="font-size: 16px; font-weight: 700; margin-bottom: 12px;">全体 Webhook の編集</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="updateSysWebhook">
          <div>
            <label class="field-label">Webhook 名 *</label>
            <input v-model="editSysWebhookForm.name" type="text" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">送信先 URL (HTTP/HTTPS) *</label>
            <input v-model="editSysWebhookForm.url" type="url" required style="width: 100%;" />
          </div>
          <div>
            <label class="field-label">シークレット鍵 (空欄のままなら変更なし)</label>
            <input v-model="editSysWebhookForm.secret" type="text" style="width: 100%; font-family: monospace;" placeholder="更新時のみ入力" />
          </div>
          <label style="display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 13px;">
            <input v-model="editSysWebhookForm.is_active" type="checkbox" />
            <span>有効にする</span>
          </label>
          <div>
            <label class="field-label">購読イベント</label>
            <div style="display: grid; grid-template-columns: repeat(2, 1fr); gap: 6px; font-size: 12px;">
              <label v-for="ev in ['task.created', 'task.updated', 'task.status_changed', 'issue.created', 'issue.state_changed', 'comment.created', 'attachment.created']" :key="ev" style="display: flex; align-items: center; gap: 4px;">
                <input v-model="editSysWebhookForm.events" type="checkbox" :value="ev" />
                <span>{{ ev }}</span>
              </label>
            </div>
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 6px;">
            <button type="button" class="btn btn-secondary" @click="editingSysWebhook = null">キャンセル</button>
            <button type="submit" class="btn btn-primary">更新を保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- SMTP Error Detail Modal -->
    <div v-if="smtpErrorDetail" class="modal-backdrop" @click.self="smtpErrorDetail = null">
      <div class="modal-panel" style="max-width: 500px;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <h3 style="font-size: 15px; font-weight: 700; color: var(--danger); display: flex; align-items: center; gap: 6px;">
            <span>⚠️</span><span>SMTP テスト送信エラー</span>
          </h3>
          <button class="btn btn-secondary btn-sm" @click="smtpErrorDetail = null">閉じる</button>
        </div>
        <div style="display: flex; flex-direction: column; gap: 10px; font-size: 13px;">
          <p style="color: var(--text-muted); margin: 0;">
            SMTP サーバーへの接続または認証に失敗しました。
          </p>
          <pre style="background: var(--bg-input); padding: 10px; border-radius: 6px; border: 1px solid var(--border-color); font-size: 12px; color: var(--danger); white-space: pre-wrap; word-break: break-all; margin: 0;">{{ smtpErrorDetail }}</pre>
          <div style="font-size: 12px; color: var(--text-muted); background: var(--bg-input); padding: 8px 12px; border-radius: 6px; border: 1px solid var(--border-color);">
            <strong>トラブルシューティング:</strong>
            <ul style="margin: 4px 0 0 16px; padding: 0;">
              <li>ホスト名とポート番号が正しいか確認（STARTTLS: 587, SSL/TLS: 465）</li>
              <li>認証情報（ユーザー名・パスワード）が有効か確認</li>
              <li>外部ネットワークへの通信がファイアウォールでブロックされていないか確認</li>
            </ul>
          </div>
        </div>
        <div style="display: flex; justify-content: flex-end; margin-top: 14px;">
          <button class="btn btn-secondary btn-sm" @click="smtpErrorDetail = null">閉じる</button>
        </div>
      </div>
    </div>

    <!-- Group Rename Modal -->
    <div v-if="renamingGroup" class="modal-backdrop" @click.self="renamingGroup = null">
      <div class="modal-panel" style="max-width: 400px;">
        <h3 style="font-size: 15px; font-weight: 700; margin-bottom: 12px;">グループ名の変更</h3>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitRenameGroup">
          <div>
            <label class="field-label">新しいグループ名 *</label>
            <input v-model="renamingGroupName" type="text" required style="width: 100%;" />
          </div>
          <div style="display: flex; justify-content: flex-end; gap: 8px;">
            <button type="button" class="btn btn-secondary btn-sm" @click="renamingGroup = null">キャンセル</button>
            <button type="submit" class="btn btn-primary btn-sm">保存</button>
          </div>
        </form>
      </div>
    </div>

    <!-- Group Member Preview Modal -->
    <div
      v-if="previewGroup"
      class="modal-backdrop"
      style="z-index: 250;"
      @click.self="previewGroup = null"
    >
      <div
        class="card"
        style="max-width: 440px; width: 100%; display: flex; flex-direction: column; gap: 12px; box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);"
      >
        <div style="display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <div>
            <h3 style="font-size: 14px; font-weight: 700; display: flex; align-items: center; gap: 6px;">
              <span>👥</span>
              <span>{{ previewGroup.name }} の所属メンバー</span>
            </h3>
            <div style="font-size: 11px; color: var(--text-muted); margin-top: 2px;">
              全プロジェクト横断・{{ previewGroup.member_ids.length }}名所属
            </div>
          </div>
          <button
            type="button"
            class="btn btn-ghost btn-sm"
            style="padding: 2px 6px;"
            @click="previewGroup = null"
          >
            ✕
          </button>
        </div>

        <input
          v-model="previewGroupSearch"
          type="text"
          placeholder="メンバー名・メールで絞り込み..."
          style="width: 100%; font-size: 12px;"
        />

        <div style="max-height: 240px; overflow-y: auto; display: flex; flex-direction: column; gap: 6px; padding-right: 4px;">
          <div
            v-for="m in filteredPreviewMembers"
            :key="m.id"
            style="display: flex; align-items: center; justify-content: space-between; padding: 6px 10px; border-radius: 6px; background: var(--bg-input); border: 1px solid var(--border-color); font-size: 12px;"
          >
            <div style="display: flex; align-items: center; gap: 8px;">
              <div style="width: 24px; height: 24px; border-radius: 50%; background: #2563eb; color: #fff; display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 10px;">
                {{ m.display_name.slice(0, 2) }}
              </div>
              <div>
                <div style="font-weight: 700;">{{ m.display_name }}</div>
                <div style="font-size: 10px; color: var(--text-muted);">{{ m.email }}</div>
              </div>
            </div>
            <span
              :class="[
                'badge',
                m.status === 'active' ? 'badge-green' : m.status === 'disabled' ? 'badge-red' : 'badge-amber'
              ]"
              style="font-size: 10px;"
            >
              {{ m.status === 'active' ? '有効' : m.status === 'disabled' ? '無効' : '招待中' }}
            </span>
          </div>

          <div v-if="filteredPreviewMembers.length === 0" style="padding: 16px; text-align: center; color: var(--text-muted); font-size: 12px;">
            所属メンバーはいません
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; border-top: 1px solid var(--border-color); padding-top: 8px;">
          <button type="button" class="btn btn-secondary btn-sm" @click="previewGroup = null">
            閉じる
          </button>
        </div>
      </div>
    </div>

    <!-- Bulk User Invite Modal -->
    <BulkInviteModal
      v-if="showBulkInviteModal"
      :allowed-domains="allowedDomainsList"
      :existing-users="users"
      :groups="globalGroups"
      @close="showBulkInviteModal = false"
      @invited="loadAdminData"
    />
  </div>
</template>

<script setup lang="ts">
import type {
  CustomFieldDefinition,
  Group,
  Label,
  SMTPSettings,
  Status,
  SystemPermission,
  SystemSettings,
  TargetType,
  Template,
  TemplateRow,
  User,
  Webhook,
  WebhookDelivery
} from '~/types/polaris'
import { hasAtMostTwoDecimals } from '~/utils/format'

type AdminTab = 'users' | 'general' | 'smtp' | 'groups_perms' | 'defaults' | 'templates' | 'webhooks'

const route = useRoute()
const router = useRouter()
const { apiFetch, notifyToast, currentUser } = useApi()

const activeTab = ref<AdminTab>((route.query.tab as AdminTab) || 'users')

watch(() => route.query.tab, (t) => {
  if (t && ['users', 'general', 'smtp', 'groups_perms', 'defaults', 'templates', 'webhooks'].includes(String(t))) {
    activeTab.value = t as AdminTab
  }
})

function switchTab(t: AdminTab) {
  activeTab.value = t
  router.replace({ query: { ...route.query, tab: t } })
}

const users = ref<User[]>([])
const globalGroups = ref<Group[]>([])
const sysPerms = ref<SystemPermission[]>([])
const sysLabels = ref<Label[]>([])
const sysStatuses = ref<Status[]>([])
const sysCustomFields = ref<CustomFieldDefinition[]>([])
const sysTemplates = ref<Template[]>([])

const userSearchQuery = ref('')
const userStatusFilter = ref<'all' | 'active' | 'invited' | 'disabled'>('all')
const userPage = ref(1)
const userPageSize = 10
const showBulkInviteModal = ref(false)

const filteredUsers = computed(() => {
  let list = users.value
  if (userStatusFilter.value !== 'all') {
    list = list.filter(u => u.status === userStatusFilter.value)
  }
  const q = userSearchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(u =>
      u.display_name.toLowerCase().includes(q) ||
      u.email.toLowerCase().includes(q)
    )
  }
  return list
})

const totalUserPages = computed(() => Math.ceil(filteredUsers.value.length / userPageSize) || 1)
const paginatedUsers = computed(() => {
  const start = (userPage.value - 1) * userPageSize
  return filteredUsers.value.slice(start, start + userPageSize)
})

watch([userSearchQuery, userStatusFilter], () => {
  userPage.value = 1
})

const allowedDomainsList = ref<string[]>([])
const newDomainInput = ref('')

function addAllowedDomain() {
  let d = newDomainInput.value.trim().toLowerCase()
  if (!d) return
  d = d.replace(/^https?:\/\//, '').replace(/^@/, '').split('/')[0].trim()
  if (!d) return
  if (allowedDomainsList.value.includes(d)) {
    notifyToast(`ドメイン「${d}」は既に追加されています`, 'error')
    return
  }
  if (allowedDomainsList.value.length >= 10) {
    notifyToast('許可ドメインは最大10件までです', 'error')
    return
  }
  allowedDomainsList.value.push(d)
  newDomainInput.value = ''
}

function removeAllowedDomain(idx: number) {
  allowedDomainsList.value.splice(idx, 1)
}

const sysForm = reactive({
  default_hours_per_day: 8.0,
  default_issue_template_id: '' as string | null,
  default_task_template_id: '' as string | null,
  allowed_domains_text: ''
})

const smtpForm = reactive({
  enabled: false,
  host: '',
  port: 587,
  use_auth: false,
  username: '',
  password: '',
  has_password: false,
  from_email: 'noreply@localhost',
  from_name: 'Polaris'
})
const testEmailTo = ref('')
const isSendingTestEmail = ref(false)
const smtpErrorDetail = ref<string | null>(null)

const renamingGroup = ref<Group | null>(null)
const renamingGroupName = ref('')

function openRenameGroupModal(g: Group) {
  renamingGroup.value = g
  renamingGroupName.value = g.name
}

async function submitRenameGroup() {
  if (!renamingGroup.value || !renamingGroupName.value.trim()) return
  try {
    await apiFetch(`/groups/${renamingGroup.value.id}`, {
      method: 'PUT',
      body: JSON.stringify({
        name: renamingGroupName.value.trim(),
        member_ids: renamingGroup.value.member_ids
      })
    })
    renamingGroup.value = null
    await loadAdminData()
    notifyToast('グループ名を変更しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

const submittingUser = ref(false)
const userActionFeedback = reactive({
  type: 'success' as 'success' | 'error',
  text: '',
  inviteUrl: ''
})

const directUserForm = reactive({
  email: '',
  display_name: '',
  password: ''
})
const showDirectPassword = ref(false)

const inviteForm = reactive({
  email: '',
  display_name: ''
})

const groupForm = reactive({
  name: ''
})

const permMatrixFilter = ref<'all' | 'user' | 'group'>('all')
const permMatrixSearch = ref('')
const previewGroup = ref<Group | null>(null)
const previewGroupSearch = ref('')

const matrixSubjects = computed(() => {
  const list: Array<{
    type: 'user' | 'group'
    id: string
    name: string
    subText?: string
    memberCount?: number
    members?: User[]
  }> = []

  if (permMatrixFilter.value === 'all' || permMatrixFilter.value === 'user') {
    for (const u of users.value) {
      list.push({
        type: 'user',
        id: u.id,
        name: u.display_name,
        subText: u.email
      })
    }
  }

  if (permMatrixFilter.value === 'all' || permMatrixFilter.value === 'group') {
    for (const g of globalGroups.value) {
      list.push({
        type: 'group',
        id: g.id,
        name: g.name,
        memberCount: g.member_ids.length,
        members: users.value.filter(u => g.member_ids.includes(u.id))
      })
    }
  }

  if (!permMatrixSearch.value.trim()) return list
  const q = permMatrixSearch.value.toLowerCase().trim()
  return list.filter(item => item.name.toLowerCase().includes(q) || (item.subText && item.subText.toLowerCase().includes(q)))
})

const filteredPreviewMembers = computed(() => {
  if (!previewGroup.value) return []
  const groupMembers = users.value.filter(u => previewGroup.value!.member_ids.includes(u.id))
  if (!previewGroupSearch.value.trim()) return groupMembers
  const q = previewGroupSearch.value.toLowerCase().trim()
  return groupMembers.filter(u => u.display_name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q))
})

function hasPerm(subjectType: 'user' | 'group', subjectId: string, perm: string): boolean {
  return sysPerms.value.some(sp => sp.subject_type === subjectType && sp.subject_id === subjectId && sp.permission === perm)
}

function getUserEffectivePerms(userId: string): { perms: Set<string>; hasInherited: boolean } {
  const perms = new Set<string>()
  for (const sp of sysPerms.value) {
    if (sp.subject_type === 'user' && sp.subject_id === userId) {
      perms.add(sp.permission)
    }
  }
  let hasInherited = false
  const userGroups = globalGroups.value.filter(g => g.member_ids?.includes(userId))
  for (const g of userGroups) {
    for (const sp of sysPerms.value) {
      if (sp.subject_type === 'group' && sp.subject_id === g.id) {
        if (!perms.has(sp.permission)) {
          hasInherited = true
        }
        perms.add(sp.permission)
      }
    }
  }
  return { perms, hasInherited }
}

function getSubjectPermSummary(subjectType: 'user' | 'group', subjectId: string): string {
  if (subjectType === 'group') {
    const perms = sysPerms.value.filter(sp => sp.subject_type === 'group' && sp.subject_id === subjectId).map(sp => sp.permission)
    if (perms.includes('manage_system')) return '全権限 (システム管理者)'
    if (perms.includes('create_project') && perms.includes('delete_project')) return '作成 ＆ 削除権限'
    if (perms.includes('create_project')) return 'プロジェクト作成権限'
    if (perms.includes('delete_project')) return 'プロジェクト削除権限'
    return '権限なし（一般）'
  }

  const { perms, hasInherited } = getUserEffectivePerms(subjectId)
  const tag = hasInherited ? ' [継承含む]' : ''
  if (perms.has('manage_system')) return `全権限 (システム管理者)${tag}`
  if (perms.has('create_project') && perms.has('delete_project')) return `作成 ＆ 削除権限${tag}`
  if (perms.has('create_project')) return `プロジェクト作成権限${tag}`
  if (perms.has('delete_project')) return `プロジェクト削除権限${tag}`
  return '権限なし（一般）'
}

async function toggleSystemPerm(subjectType: 'user' | 'group', subjectId: string, perm: string) {
  if (perm === 'manage_system') {
    if (subjectType === 'user' && subjectId === currentUser.value?.id) {
      notifyToast('自身のアカウントのシステム管理者権限は変更できません', 'error')
      await loadAdminData()
      return
    }
    const isGranting = !hasPerm(subjectType, subjectId, perm)
    const actionText = isGranting ? '付与' : '剥奪'
    if (!confirm(`システム管理者権限（manage_system）を${actionText}しようとしています。本当によろしいですか？`)) {
      await loadAdminData()
      return
    }
  }

  const existing = sysPerms.value.find(sp => sp.subject_type === subjectType && sp.subject_id === subjectId && sp.permission === perm)
  if (existing) {
    await revokeSystemPerm(existing.id)
  } else {
    try {
      await apiFetch('/system/permissions', {
        method: 'POST',
        body: JSON.stringify({
          subject_type: subjectType,
          subject_id: subjectId,
          permission: perm
        })
      })
      await loadAdminData()
      notifyToast('システム権限を付与しました', 'success')
    } catch (e: any) {
      notifyToast(e.message, 'error')
    }
  }
}

const sysLabelForm = reactive({
  name: '',
  color: '#3b82f6'
})

const sysStatusForm = reactive({
  display_order: 5,
  name: '',
  color: '#64748b'
})

const sysCfForm = reactive({
  target_type: 'issue' as TargetType,
  name: '',
  field_type: 'text_single'
})

onMounted(async () => {
  await loadAdminData()
})

async function loadAdminData() {
  try {
    const [st, smtp, usrs, grps, perms, lbls, sts, cfs, tpls, whs] = await Promise.all([
      apiFetch<SystemSettings>('/system/settings'),
      apiFetch<SMTPSettings>('/system/smtp'),
      apiFetch<User[]>('/users'),
      apiFetch<Group[]>('/groups'),
      apiFetch<SystemPermission[]>('/system/permissions'),
      apiFetch<Label[]>('/labels'),
      apiFetch<Status[]>('/system/statuses'),
      apiFetch<CustomFieldDefinition[]>('/system/custom-fields'),
      apiFetch<Template[]>('/system/templates'),
      apiFetch<Webhook[]>('/system/webhooks')
    ])

    sysForm.default_hours_per_day = st.default_hours_per_day
    sysForm.default_issue_template_id = st.default_issue_template_id || null
    sysForm.default_task_template_id = st.default_task_template_id || null
    sysForm.allowed_domains_text = (st.allowed_domains || []).join('\n')
    allowedDomainsList.value = [...(st.allowed_domains || [])]

    smtpForm.enabled = smtp.enabled
    smtpForm.host = smtp.host
    smtpForm.port = smtp.port
    smtpForm.use_auth = smtp.use_auth
    smtpForm.username = smtp.username
    smtpForm.has_password = smtp.has_password
    smtpForm.password = ''
    smtpForm.from_email = smtp.from_email
    smtpForm.from_name = smtp.from_name

    users.value = usrs || []
    globalGroups.value = grps || []
    sysPerms.value = perms || []
    sysLabels.value = lbls || []
    sysStatuses.value = (sts || []).sort((a, b) => a.display_order - b.display_order)
    sysCustomFields.value = cfs || []
    sysTemplates.value = tpls || []
    sysWebhooks.value = whs || []
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function saveSystemSettings() {
  if (!hasAtMostTwoDecimals(sysForm.default_hours_per_day)) {
    notifyToast('1日のデフォルト基準時間は小数点第2位までで入力してください', 'error')
    return
  }
  try {
    await apiFetch('/system/settings', {
      method: 'PUT',
      body: JSON.stringify({
        default_hours_per_day: sysForm.default_hours_per_day,
        default_issue_template_id: sysForm.default_issue_template_id,
        default_task_template_id: sysForm.default_task_template_id,
        allowed_domains: allowedDomainsList.value
      })
    })
    await loadAdminData()
    notifyToast('システム基本設定を保存しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function saveSMTPSettings() {
  try {
    const body: any = {
      enabled: smtpForm.enabled,
      host: smtpForm.host,
      port: smtpForm.port,
      use_auth: smtpForm.use_auth,
      username: smtpForm.username,
      from_email: smtpForm.from_email,
      from_name: smtpForm.from_name
    }
    if (smtpForm.password !== '') {
      body.password = smtpForm.password
    }
    await apiFetch('/system/smtp', {
      method: 'PUT',
      body: JSON.stringify(body)
    })
    await loadAdminData()
    notifyToast('メール送信設定を保存しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function sendTestMail() {
  if (!testEmailTo.value) {
    notifyToast('テスト送信先メールアドレスを入力してください', 'error')
    return
  }
  isSendingTestEmail.value = true
  smtpErrorDetail.value = null
  try {
    await apiFetch('/system/smtp/test', {
      method: 'POST',
      body: JSON.stringify({ recipient_email: testEmailTo.value })
    })
    notifyToast(`テストメールを ${testEmailTo.value} に送信しました`, 'success')
  } catch (e: any) {
    smtpErrorDetail.value = e.message || 'テストメール送信に失敗しました'
    notifyToast('メール送信に失敗しました', 'error')
  } finally {
    isSendingTestEmail.value = false
  }
}

async function createDirectUser() {
  submittingUser.value = true
  userActionFeedback.text = ''
  userActionFeedback.inviteUrl = ''
  try {
    const created = await apiFetch<User>('/users', {
      method: 'POST',
      body: JSON.stringify(directUserForm)
    })
    users.value = [...users.value.filter(x => x.id !== created.id), created]
    directUserForm.email = ''
    directUserForm.display_name = ''
    directUserForm.password = ''
    userActionFeedback.type = 'success'
    userActionFeedback.text = `✓ ユーザー「${created.display_name} (${created.email})」を直接登録しました（すぐログイン可能です）`
    notifyToast(`ユーザー「${created.display_name}」を直接登録しました`, 'success')
    await loadAdminData()
  } catch (e: any) {
    userActionFeedback.type = 'error'
    userActionFeedback.text = `登録エラー: ${e.message}`
    notifyToast(e.message, 'error')
  } finally {
    submittingUser.value = false
  }
}

async function inviteNewUser() {
  submittingUser.value = true
  userActionFeedback.text = ''
  userActionFeedback.inviteUrl = ''
  try {
    const invited = await apiFetch<User>('/users/invite', {
      method: 'POST',
      body: JSON.stringify(inviteForm)
    })
    users.value = [...users.value.filter(x => x.id !== invited.id), invited]
    inviteForm.email = ''
    inviteForm.display_name = ''
    userActionFeedback.type = 'success'
    userActionFeedback.text = `✓ ユーザー「${invited.display_name} (${invited.email})」を招待リストに登録し、招待メール（72時間有効）を送信しました`
    userActionFeedback.inviteUrl = invited.invite_url || ''
    notifyToast(`「${invited.display_name}」へ招待メールを送信しました`, 'success')
    await loadAdminData()
  } catch (e: any) {
    userActionFeedback.type = 'error'
    userActionFeedback.text = `招待エラー: ${e.message}`
    notifyToast(e.message, 'error')
  } finally {
    submittingUser.value = false
  }
}

async function resendInvite(userId: string) {
  try {
    const res = await apiFetch<{ status: string; invite_url?: string }>(`/users/${userId}/resend-invite`, { method: 'POST' })
    const target = users.value.find(x => x.id === userId)
    userActionFeedback.type = 'success'
    userActionFeedback.text = `✓ ${target ? `「${target.display_name} (${target.email})」へ` : ''}招待メールを再送しました（72時間有効）`
    userActionFeedback.inviteUrl = res?.invite_url || ''
    notifyToast('招待メールを再送しました', 'success')
  } catch (e: any) {
    userActionFeedback.type = 'error'
    userActionFeedback.text = `再送エラー: ${e.message}`
    notifyToast(e.message, 'error')
  }
}

async function copyInviteUrl(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    notifyToast('招待URLをクリップボードにコピーしました', 'success')
  } catch {
    notifyToast('コピーに失敗しました。URLを選択してコピーしてください', 'error')
  }
}

async function promptAdminEmailChange(u: User) {
  const newEmail = prompt(`ユーザー「${u.display_name}」の新しいメールアドレスを入力してください:`, u.email)
  if (!newEmail || newEmail === u.email) return
  const sendVerification = confirm('確認メールを送信して本人確認後に変更しますか？（キャンセルを選ぶと即時変更します）')
  try {
    await apiFetch(`/users/${u.id}/email`, {
      method: 'PUT',
      body: JSON.stringify({
        new_email: newEmail,
        send_verification: sendVerification
      })
    })
    await loadAdminData()
    notifyToast(sendVerification ? '確認メールを送信しました' : 'メールアドレスを即時変更しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function toggleUserStatus(userId: string, status: 'active' | 'disabled') {
  if (status === 'disabled' && !confirm('このユーザーを無効化してもよろしいですか？')) {
    return
  }
  try {
    await apiFetch(`/users/${userId}/status`, {
      method: 'PUT',
      body: JSON.stringify({ status })
    })
    await loadAdminData()
    notifyToast('ユーザーステータスを更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createGlobalGroup() {
  try {
    await apiFetch('/groups', {
      method: 'POST',
      body: JSON.stringify({ name: groupForm.name, member_ids: [] })
    })
    groupForm.name = ''
    await loadAdminData()
    notifyToast('全体グループを作成しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

const groupSearchMap = reactive<Record<string, string>>({})
const activeGroupSearchId = ref<string | null>(null)

function getAvailableGroupUsers(g: Group, query?: string): User[] {
  const notInGroup = users.value.filter(u => !g.member_ids.includes(u.id))
  if (!query || !query.trim()) return []
  const q = query.trim().toLowerCase()
  return notInGroup.filter(u =>
    u.display_name.toLowerCase().includes(q) ||
    u.email.toLowerCase().includes(q)
  ).slice(0, 10)
}

async function addGroupMember(g: Group, userId: string) {
  if (!g.member_ids.includes(userId)) {
    await toggleGroupMember(g, userId)
  }
  groupSearchMap[g.id] = ''
  activeGroupSearchId.value = null
}

async function toggleGroupMember(g: Group, userId: string) {
  const nextIds = g.member_ids.includes(userId)
    ? g.member_ids.filter(x => x !== userId)
    : [...g.member_ids, userId]
  try {
    await apiFetch(`/groups/${g.id}`, {
      method: 'PUT',
      body: JSON.stringify({ name: g.name, member_ids: nextIds })
    })
    await loadAdminData()
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteGlobalGroup(groupId: string) {
  if (!confirm('このグループを削除してもよろしいですか？')) {
    return
  }
  try {
    await apiFetch(`/groups/${groupId}`, { method: 'DELETE' })
    await loadAdminData()
    notifyToast('グループを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function revokeSystemPerm(id: string) {
  try {
    await apiFetch(`/system/permissions/${id}`, { method: 'DELETE' })
    await loadAdminData()
    notifyToast('システム権限を解除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createSystemLabel() {
  try {
    await apiFetch('/labels', {
      method: 'POST',
      body: JSON.stringify(sysLabelForm)
    })
    sysLabelForm.name = ''
    await loadAdminData()
    notifyToast('共通ラベルを追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteSystemLabel(id: string) {
  if (!confirm('この共通ラベルを削除してもよろしいですか？')) {
    return
  }
  try {
    await apiFetch(`/labels/${id}`, { method: 'DELETE' })
    await loadAdminData()
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createSystemStatus() {
  try {
    await apiFetch('/system/statuses', {
      method: 'POST',
      body: JSON.stringify(sysStatusForm)
    })
    sysStatusForm.name = ''
    await loadAdminData()
    sysStatusForm.display_order = Math.max(...sysStatuses.value.map(s => s.display_order), 0) + 1
    notifyToast('デフォルトステータスを追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function resetSystemStatuses() {
  if (!confirm('システムデフォルトステータスを初期設定（1.未着手、2.進行中、3.レビュー中、4.完了）にリセットしますか？\n※既存プロジェクトのステータスには影響しません。')) {
    return
  }
  try {
    await apiFetch('/system/statuses/reset', { method: 'POST' })
    await loadAdminData()
    notifyToast('デフォルトステータスを初期設定にリセットしました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteSystemStatus(id: string) {
  if (!confirm('このデフォルトステータスを削除してもよろしいですか？')) {
    return
  }
  try {
    await apiFetch(`/system/statuses/${id}`, { method: 'DELETE' })
    await loadAdminData()
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function createSystemCustomField() {
  try {
    await apiFetch('/system/custom-fields', {
      method: 'POST',
      body: JSON.stringify({
        target_type: sysCfForm.target_type,
        name: sysCfForm.name,
        field_type: sysCfForm.field_type,
        options: []
      })
    })
    sysCfForm.name = ''
    await loadAdminData()
    notifyToast('デフォルトカスタム属性を追加しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteSystemCustomField(id: string) {
  try {
    await apiFetch(`/system/custom-fields/${id}`, { method: 'DELETE' })
    await loadAdminData()
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function saveSystemTemplate(payload: { id?: string; target_type: TargetType; name: string; rows: TemplateRow[] }) {
  try {
    if (payload.id) {
      await apiFetch(`/system/templates/${payload.id}`, {
        method: 'PUT',
        body: JSON.stringify({ name: payload.name, rows: payload.rows })
      })
    } else {
      await apiFetch('/system/templates', {
        method: 'POST',
        body: JSON.stringify(payload)
      })
    }
    await loadAdminData()
    notifyToast('システムデフォルトテンプレートを保存しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteSystemTemplate(id: string) {
  try {
    await apiFetch(`/system/templates/${id}`, { method: 'DELETE' })
    await loadAdminData()
    notifyToast('テンプレートを削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

function getUserName(id: string): string {
  return users.value.find(u => u.id === id)?.display_name || id.slice(0, 8)
}

function getGroupName(id: string): string {
  return globalGroups.value.find(g => g.id === id)?.name || id.slice(0, 8)
}

function getPermLabel(perm: string): string {
  if (perm === 'create_project') return 'プロジェクト作成'
  if (perm === 'delete_project') return 'プロジェクト削除'
  if (perm === 'manage_system') return 'システム全体管理'
  return perm
}

// System Webhooks State & Methods
const sysWebhooks = ref<Webhook[]>([])
const showSysWebhookModal = ref(false)
const selectedSysWebhookForLogs = ref<Webhook | null>(null)
const sysWebhookDeliveries = ref<WebhookDelivery[]>([])
const expandedSysDeliveryId = ref<string | null>(null)

const editingSysWebhook = ref<Webhook | null>(null)
const editSysWebhookForm = reactive({
  name: '',
  url: '',
  secret: '',
  events: [] as string[],
  is_active: true
})

function openEditSysWebhookModal(wh: Webhook) {
  editingSysWebhook.value = wh
  editSysWebhookForm.name = wh.name
  editSysWebhookForm.url = wh.url
  editSysWebhookForm.secret = ''
  editSysWebhookForm.events = [...(wh.events || [])]
  editSysWebhookForm.is_active = wh.is_active
}

async function updateSysWebhook() {
  if (!editingSysWebhook.value) return
  try {
    const payload: any = {
      name: editSysWebhookForm.name,
      url: editSysWebhookForm.url,
      events: editSysWebhookForm.events,
      is_active: editSysWebhookForm.is_active
    }
    if (editSysWebhookForm.secret) {
      payload.secret = editSysWebhookForm.secret
    }
    await apiFetch(`/system/webhooks/${editingSysWebhook.value.id}`, {
      method: 'PUT',
      body: JSON.stringify(payload)
    })
    editingSysWebhook.value = null
    await loadSysWebhooks()
    notifyToast('全体 Webhook を更新しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

const newSysWebhookForm = reactive({
  name: '',
  url: '',
  secret: '',
  events: ['task.created', 'task.updated', 'issue.created', 'comment.created']
})

async function loadSysWebhooks() {
  try {
    sysWebhooks.value = await apiFetch<Webhook[]>('/system/webhooks')
  } catch (e: any) {
    // ignore
  }
}

async function createSysWebhook() {
  try {
    await apiFetch('/system/webhooks', {
      method: 'POST',
      body: JSON.stringify(newSysWebhookForm)
    })
    showSysWebhookModal.value = false
    newSysWebhookForm.name = ''
    newSysWebhookForm.url = ''
    newSysWebhookForm.secret = ''
    await loadSysWebhooks()
    notifyToast('全体 Webhook を登録しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function testSysWebhook(webhookId: string) {
  try {
    const res = await apiFetch<WebhookDelivery>(`/system/webhooks/${webhookId}/test`, {
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

async function openSysWebhookDeliveries(wh: Webhook) {
  selectedSysWebhookForLogs.value = wh
  try {
    sysWebhookDeliveries.value = await apiFetch<WebhookDelivery[]>(`/system/webhooks/${wh.id}/deliveries`)
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}

async function deleteSysWebhook(webhookId: string) {
  if (!confirm('この全体 Webhook を削除しますか？')) return
  try {
    await apiFetch(`/system/webhooks/${webhookId}`, { method: 'DELETE' })
    await loadSysWebhooks()
    notifyToast('全体 Webhook を削除しました', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}
</script>
