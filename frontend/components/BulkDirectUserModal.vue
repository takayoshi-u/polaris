<template>
  <div
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
    v-backdrop-dismiss="() => $emit('close')"
  >
    <div
      class="card"
      style="
        width: 100%;
        max-width: 720px;
        max-height: 90vh;
        overflow-y: auto;
        padding: 24px;
        display: flex;
        flex-direction: column;
        gap: 16px;
      "
    >
      <!-- Header -->
      <div style="display: flex; align-items: center; justify-content: space-between; border-bottom: 1px solid var(--border-color); padding-bottom: 12px;">
        <div style="display: flex; align-items: center; gap: 8px;">
          <span style="font-size: 20px;">📥</span>
          <div>
            <h3 style="font-size: 16px; font-weight: 700; margin: 0;">リストからユーザーを一括直接登録</h3>
            <p style="font-size: 11px; color: var(--text-muted); margin: 2px 0 0 0;">
              CSV/TSV/テキストから複数ユーザーを仮パスワード付きで直接登録します (形式: email,display_name,[password])。
            </p>
          </div>
        </div>
        <button type="button" class="btn btn-ghost btn-sm" @click="$emit('close')">✕</button>
      </div>

      <!-- RESULT VIEW (After submission) -->
      <div v-if="directResult" style="display: flex; flex-direction: column; gap: 16px;">
        <!-- Summary Banner -->
        <div
          :style="{
            padding: '16px',
            borderRadius: '8px',
            background: directResult.succeeded > 0 ? 'rgba(16, 185, 129, 0.12)' : 'rgba(239, 68, 68, 0.12)',
            border: `1px solid ${directResult.succeeded > 0 ? '#10b981' : '#ef4444'}`
          }"
        >
          <div style="font-size: 15px; font-weight: 700;" :style="{ color: directResult.succeeded > 0 ? 'var(--success, #10b981)' : 'var(--danger, #ef4444)' }">
            {{ directResult.succeeded > 0 ? `🎉 ${directResult.succeeded} 件 のアカウントを登録しました` : '一括直接登録を実行できませんでした' }}
          </div>
          <div style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
            全 {{ directResult.total }} 件中 / 成功: {{ directResult.succeeded }} 件 / 失敗・スキップ: {{ directResult.failed }} 件
          </div>
        </div>

        <!-- Failure Details Table (if any) -->
        <div v-if="directResult.failures && directResult.failures.length > 0" style="display: flex; flex-direction: column; gap: 6px;">
          <div style="font-size: 12px; font-weight: 700; color: var(--danger, #ef4444);">
            ⚠️ 登録に失敗したアドレス ({{ directResult.failures.length }}件):
          </div>
          <div style="max-height: 180px; overflow-y: auto; border: 1px solid var(--border-color); border-radius: 6px; background: var(--bg-input);">
            <table style="width: 100%; border-collapse: collapse; font-size: 12px;">
              <thead>
                <tr style="border-bottom: 1px solid var(--border-color); color: var(--text-muted); background: var(--bg-card);">
                  <th style="padding: 6px 10px; text-align: left;">メールアドレス</th>
                  <th style="padding: 6px 10px; text-align: left;">表示名</th>
                  <th style="padding: 6px 10px; text-align: left;">失敗・スキップ理由</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(f, idx) in directResult.failures" :key="idx" style="border-bottom: 1px solid var(--border-color);">
                  <td style="padding: 6px 10px; font-family: monospace;">{{ f.email }}</td>
                  <td style="padding: 6px 10px;">{{ f.display_name }}</td>
                  <td style="padding: 6px 10px; color: var(--danger, #ef4444);">{{ f.reason }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Created Users with Temporary Passwords & Export Actions -->
        <div v-if="directResult.created_users && directResult.created_users.length > 0" style="display: flex; flex-direction: column; gap: 8px;">
          <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 8px;">
            <div style="font-size: 13px; font-weight: 700; color: var(--text-main);">
              📋 発行された仮認証情報（{{ directResult.created_users.length }}件）:
            </div>
            <div style="display: flex; gap: 8px;">
              <button type="button" class="btn btn-secondary btn-sm" @click="copyCredentials">
                📋 クリップボードにコピー
              </button>
              <button type="button" class="btn btn-secondary btn-sm" @click="downloadCredentialsCSV">
                📥 CSVダウンロード
              </button>
            </div>
          </div>

          <div style="max-height: 240px; overflow-y: auto; border: 1px solid var(--border-color); border-radius: 6px; background: var(--bg-card);">
            <table style="width: 100%; border-collapse: collapse; font-size: 12px; text-align: left;">
              <thead>
                <tr style="background: var(--bg-input); border-bottom: 1px solid var(--border-color); color: var(--text-muted);">
                  <th style="padding: 6px 10px; width: 36px;">#</th>
                  <th style="padding: 6px 10px;">メールアドレス</th>
                  <th style="padding: 6px 10px;">表示名</th>
                  <th style="padding: 6px 10px;">仮パスワード</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(u, idx) in directResult.created_users" :key="u.id || idx" style="border-bottom: 1px solid var(--border-color);">
                  <td style="padding: 6px 10px; color: var(--text-muted);">{{ idx + 1 }}</td>
                  <td style="padding: 6px 10px; font-family: monospace;">{{ u.email }}</td>
                  <td style="padding: 6px 10px;">{{ u.display_name }}</td>
                  <td style="padding: 6px 10px;">
                    <div style="display: flex; align-items: center; gap: 6px;">
                      <code style="font-family: monospace; font-size: 12px; background: var(--bg-input); padding: 2px 6px; border-radius: 4px; font-weight: 600;">
                        {{ u.temp_password }}
                      </code>
                      <button
                        type="button"
                        class="btn btn-ghost btn-sm"
                        style="padding: 2px 6px; font-size: 11px;"
                        title="パスワードをコピー"
                        @click="copySinglePassword(u.temp_password)"
                      >
                        コピー
                      </button>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
          <div style="font-size: 11px; color: var(--text-muted);">
            ※ 仮パスワードはこの画面を閉じると二度と表示されません。速やかにダウンロードまたはコピーして対象者に安全に配布してください。
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 8px;">
          <button type="button" class="btn btn-primary" @click="finish">
            完了して閉じる
          </button>
        </div>
      </div>

      <!-- INPUT & PREVIEW VIEW -->
      <div v-else style="display: flex; flex-direction: column; gap: 14px;">
        <!-- Input Mode Switcher Tabs -->
        <div style="display: flex; gap: 6px; border-bottom: 1px solid var(--border-color); padding-bottom: 8px;">
          <button
            type="button"
            class="btn btn-sm"
            :class="inputMode === 'file' ? 'btn-primary' : 'btn-secondary'"
            @click="inputMode = 'file'"
          >
            📁 ファイルから読み込み (.csv / .txt / .tsv)
          </button>
          <button
            type="button"
            class="btn btn-sm"
            :class="inputMode === 'text' ? 'btn-primary' : 'btn-secondary'"
            @click="inputMode = 'text'"
          >
            ✍️ テキスト直接貼り付け
          </button>
        </div>

        <!-- Mode 1: File Drop Zone -->
        <div v-if="inputMode === 'file'" style="display: flex; flex-direction: column; gap: 8px;">
          <div
            style="
              border: 2px dashed var(--border-color);
              border-radius: 8px;
              padding: 24px;
              text-align: center;
              cursor: pointer;
              background: var(--bg-input);
              display: flex;
              flex-direction: column;
              align-items: center;
              gap: 8px;
            "
            @dragover.prevent="isDragging = true"
            @dragleave.prevent="isDragging = false"
            @drop.prevent="onFileDrop"
            @click="triggerFileInput"
          >
            <input
              ref="fileInputRef"
              type="file"
              accept=".csv,.txt,.tsv,text/csv,text/plain,text/tab-separated-values"
              style="display: none;"
              @change="onFileSelected"
            />
            <span style="font-size: 28px;">📄</span>
            <div v-if="selectedFileName" style="font-weight: 600; font-size: 13px; color: var(--primary);">
              {{ selectedFileName }} ({{ formatFileSize(selectedFileSize) }}) - クリックで再選択
            </div>
            <div v-else style="font-size: 13px; color: var(--text-secondary);">
              CSV・テキストファイルをここにドラッグ＆ドロップ、またはクリックして選択
            </div>
            <div style="font-size: 11px; color: var(--text-muted);">
              形式: <code>email,display_name,[password]</code>（ヘッダー行有無どちらも対応）
            </div>
          </div>
        </div>

        <!-- Mode 2: Direct Textarea Input -->
        <div v-if="inputMode === 'text'" style="display: flex; flex-direction: column; gap: 6px;">
          <label class="field-label" style="display: flex; justify-content: space-between;">
            <span>登録ユーザーリスト（改行またはカンマ区切り）</span>
            <span style="font-size: 11px; color: var(--text-muted);">形式: email,display_name,[password]</span>
          </label>
          <textarea
            v-model="rawText"
            rows="5"
            placeholder="alice@example.com, 山田 アリス, Secret1234!&#10;bob@example.com, 鈴木 ボブ&#10;charlie@example.com"
            style="width: 100%; font-family: monospace; font-size: 12px; line-height: 1.5; padding: 10px;"
          ></textarea>
        </div>

        <!-- Batch Direct User Options -->
        <div style="display: flex; flex-direction: column; gap: 10px; padding: 12px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-input);">
          <div style="font-size: 12px; font-weight: 700;">🔐 仮パスワード設定オプション</div>
          <div style="display: grid; grid-template-columns: 1fr; gap: 8px;">
            <div>
              <label class="field-label">一括デフォルト仮パスワード（未指定行に適用、空欄時はランダム自動生成）</label>
              <input
                v-model="batchDefaultPassword"
                type="text"
                placeholder="例: InitPass2026!（空欄の場合は12文字の英数記号を自動生成）"
                style="width: 100%; font-size: 12px;"
              />
            </div>
            <label style="display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 12px; margin-top: 4px;">
              <input v-model="mustChangePassword" type="checkbox" />
              <span style="font-weight: 600;">初回ログイン時にパスワード変更を強制する (仮パスワード)</span>
            </label>
          </div>
        </div>

        <!-- Real-time Pre-validation & Preview Area -->
        <div v-if="parsedRows.length > 0" style="display: flex; flex-direction: column; gap: 8px;">
          <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 8px;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <strong style="font-size: 13px;">リアルタイム検出プレビュー</strong>
              <span class="badge badge-blue" style="font-size: 11px;">検出: {{ parsedRows.length }}件</span>
              <span v-if="validCount > 0" class="badge badge-green" style="font-size: 11px;">登録可能: {{ validCount }}件</span>
              <span v-if="warningCount > 0" class="badge badge-red" style="font-size: 11px;">要確認: {{ warningCount }}件</span>
            </div>
            <div v-if="allowedDomains && allowedDomains.length > 0" style="font-size: 11px; color: var(--text-muted);">
              許可ドメイン: {{ allowedDomains.join(', ') }}
            </div>
          </div>

          <!-- Preview Table -->
          <div style="max-height: 180px; overflow-y: auto; border: 1px solid var(--border-color); border-radius: 6px; background: var(--bg-card);">
            <table style="width: 100%; border-collapse: collapse; font-size: 12px; text-align: left;">
              <thead>
                <tr style="background: var(--bg-input); border-bottom: 1px solid var(--border-color); color: var(--text-muted);">
                  <th style="padding: 6px 10px;">#</th>
                  <th style="padding: 6px 10px;">メールアドレス</th>
                  <th style="padding: 6px 10px;">表示名</th>
                  <th style="padding: 6px 10px;">パスワード設定</th>
                  <th style="padding: 6px 10px; text-align: center;">判定</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="(row, idx) in parsedRows"
                  :key="idx"
                  style="border-bottom: 1px solid var(--border-color);"
                  :style="{ backgroundColor: !row.isDomainAllowed || row.isDuplicateInBatch || row.isAlreadyRegistered || !row.isValidEmailFormat ? 'rgba(239, 68, 68, 0.05)' : 'transparent' }"
                >
                  <td style="padding: 6px 10px; color: var(--text-muted); width: 32px;">{{ idx + 1 }}</td>
                  <td style="padding: 6px 10px; font-family: monospace;">{{ row.email }}</td>
                  <td style="padding: 6px 10px;">{{ row.displayName }}</td>
                  <td style="padding: 6px 10px;">
                    <span v-if="row.passwordMode === 'custom'" class="badge badge-purple" style="font-size: 10px;">個別指定</span>
                    <span v-else-if="row.passwordMode === 'batch_default'" class="badge badge-blue" style="font-size: 10px;">一括指定 (仮)</span>
                    <span v-else class="badge badge-secondary" style="font-size: 10px;">自動生成 (仮)</span>
                  </td>
                  <td style="padding: 6px 10px; text-align: center;">
                    <span v-if="!row.isValidEmailFormat" class="badge badge-red">❌ 書式不正</span>
                    <span v-else-if="!row.isDomainAllowed" class="badge badge-red">⚠️ 許可ドメイン外</span>
                    <span v-else-if="row.isDuplicateInBatch" class="badge badge-amber">⚠️ 重複</span>
                    <span v-else-if="row.isAlreadyRegistered" class="badge badge-amber">⚠️ 既存登録済</span>
                    <span v-else class="badge badge-green">✓ OK</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Initial Group & Permission Configuration -->
        <div style="display: flex; flex-direction: column; gap: 10px; padding: 12px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-input);">
          <div style="font-size: 12px; font-weight: 700;">⚙️ 初期所属グループ ＆ システム権限（任意）</div>

          <!-- Group Checkboxes -->
          <div>
            <div style="font-size: 11px; color: var(--text-muted); margin-bottom: 4px;">初期所属グループ:</div>
            <div style="display: flex; flex-wrap: wrap; gap: 8px; max-height: 80px; overflow-y: auto;">
              <label
                v-for="g in groups"
                :key="g.id"
                style="display: flex; align-items: center; gap: 4px; font-size: 12px; cursor: pointer; padding: 3px 8px; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 4px;"
              >
                <input v-model="selectedGroupIds" type="checkbox" :value="g.id" />
                <span>👥 {{ g.name }}</span>
              </label>
              <span v-if="groups.length === 0" style="color: var(--text-muted); font-size: 11px;">全体グループはありません</span>
            </div>
          </div>

          <!-- System Permission Checkboxes -->
          <div>
            <div style="font-size: 11px; color: var(--text-muted); margin-bottom: 4px;">初期システム権限:</div>
            <div style="display: flex; flex-wrap: wrap; gap: 8px;">
              <label style="display: flex; align-items: center; gap: 4px; font-size: 12px; cursor: pointer; padding: 3px 8px; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 4px;">
                <input v-model="selectedPermissions" type="checkbox" value="create_project" />
                <span>プロジェクト作成 (create_project)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px; font-size: 12px; cursor: pointer; padding: 3px 8px; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 4px;">
                <input v-model="selectedPermissions" type="checkbox" value="delete_project" />
                <span>プロジェクト削除 (delete_project)</span>
              </label>
              <label style="display: flex; align-items: center; gap: 4px; font-size: 12px; cursor: pointer; padding: 3px 8px; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 4px;">
                <input v-model="selectedPermissions" type="checkbox" value="manage_system" />
                <span>システム全体管理 (manage_system)</span>
              </label>
            </div>
          </div>
        </div>

        <!-- Submit & Cancel Actions -->
        <div style="display: flex; justify-content: flex-end; gap: 8px; margin-top: 4px;">
          <button type="button" class="btn btn-secondary" :disabled="isSubmitting" @click="$emit('close')">
            キャンセル
          </button>
          <button
            type="button"
            class="btn btn-primary"
            :disabled="validCount === 0 || isSubmitting"
            @click="executeBulkDirect"
          >
            {{ isSubmitting ? '直接登録を実行中...' : `👤 ${validCount} 件のユーザーを一括直接登録` }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { BulkDirectUserItem, BulkDirectUserResult, Group, User } from '~/types/polaris'
import { parseDirectUserText } from '~/utils/format'

const props = defineProps<{
  allowedDomains: string[]
  existingUsers: User[]
  groups: Group[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'created'): void
}>()

const { apiFetch, notifyToast } = useApi()

const inputMode = ref<'file' | 'text'>('file')
const rawText = ref('')
const selectedFileName = ref('')
const selectedFileSize = ref(0)
const isDragging = ref(false)
const fileInputRef = ref<HTMLInputElement | null>(null)

const batchDefaultPassword = ref('')
const mustChangePassword = ref(true)
const selectedGroupIds = ref<string[]>([])
const selectedPermissions = ref<string[]>([])
const isSubmitting = ref(false)
const directResult = ref<BulkDirectUserResult | null>(null)

const existingEmailsSet = computed(() => {
  const set = new Set<string>()
  for (const u of props.existingUsers) {
    if (u.email) {
      set.add(u.email.toLowerCase().trim())
    }
  }
  return set
})

const parsedRows = computed(() => {
  return parseDirectUserText(
    rawText.value,
    props.allowedDomains,
    existingEmailsSet.value,
    batchDefaultPassword.value
  )
})

const validCount = computed(() => {
  return parsedRows.value.filter(
    r => r.isValidEmailFormat && r.isDomainAllowed && !r.isDuplicateInBatch && !r.isAlreadyRegistered
  ).length
})

const warningCount = computed(() => {
  return parsedRows.value.filter(
    r => !r.isValidEmailFormat || !r.isDomainAllowed || r.isDuplicateInBatch || r.isAlreadyRegistered
  ).length
})

function triggerFileInput() {
  fileInputRef.value?.click()
}

function onFileSelected(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    handleFile(file)
  }
}

function onFileDrop(event: DragEvent) {
  isDragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) {
    handleFile(file)
  }
}

function handleFile(file: File) {
  selectedFileName.value = file.name
  selectedFileSize.value = file.size
  const reader = new FileReader()
  reader.onload = (e) => {
    rawText.value = (e.target?.result as string) || ''
  }
  reader.readAsText(file, 'UTF-8')
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

async function executeBulkDirect() {
  if (validCount.value === 0 || isSubmitting.value) return

  const candidates: BulkDirectUserItem[] = parsedRows.value
    .filter(r => r.isValidEmailFormat && !r.isDuplicateInBatch)
    .map(r => ({
      email: r.email,
      display_name: r.displayName,
      password: r.password || (batchDefaultPassword.value.trim() ? batchDefaultPassword.value.trim() : undefined)
    }))

  const payload = {
    users: candidates,
    default_password: batchDefaultPassword.value.trim() || undefined,
    must_change_password: mustChangePassword.value,
    default_group_ids: selectedGroupIds.value.length > 0 ? selectedGroupIds.value : undefined,
    default_permissions: selectedPermissions.value.length > 0 ? selectedPermissions.value : undefined
  }

  isSubmitting.value = true
  try {
    const res = await apiFetch<BulkDirectUserResult>('/users/bulk-direct', {
      method: 'POST',
      body: JSON.stringify(payload)
    })
    directResult.value = res
    if (res.succeeded > 0) {
      notifyToast(`${res.succeeded} 件のユーザーを直接登録しました`, 'success')
    }
  } catch (err: any) {
    notifyToast(err.message || '一括直接登録に失敗しました', 'error')
  } finally {
    isSubmitting.value = false
  }
}

function copySinglePassword(pwd: string) {
  navigator.clipboard.writeText(pwd)
  notifyToast('パスワードをコピーしました', 'info')
}

function copyCredentials() {
  if (!directResult.value || !directResult.value.created_users) return
  const lines = ['email,display_name,temp_password']
  for (const u of directResult.value.created_users) {
    lines.push(`"${u.email}","${u.display_name}","${u.temp_password}"`)
  }
  navigator.clipboard.writeText(lines.join('\n'))
  notifyToast('認証情報（CSV形式）をクリップボードにコピーしました', 'success')
}

function downloadCredentialsCSV() {
  if (!directResult.value || !directResult.value.created_users) return
  const lines = ['email,display_name,temp_password']
  for (const u of directResult.value.created_users) {
    lines.push(`"${u.email}","${u.display_name}","${u.temp_password}"`)
  }
  const csvContent = '\uFEFF' + lines.join('\r\n')
  const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  const now = new Date().toISOString().replace(/[-:T]/g, '').slice(0, 14)
  a.href = url
  a.download = `polaris_credentials_${now}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
  notifyToast('CSVファイルをダウンロードしました', 'success')
}

function finish() {
  emit('created')
  emit('close')
}
</script>
