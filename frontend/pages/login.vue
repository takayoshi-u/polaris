<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;">
    <!-- Standard Login Form -->
    <div v-if="!isMfaStep" class="card" style="width: 100%; max-width: 420px; padding: 28px;">
      <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 20px;">
        <div style="width: 36px; height: 36px; border-radius: 8px; background: #2563eb; color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; font-size: 18px;">
          P
        </div>
        <div>
          <h1 style="font-size: 18px; font-weight: 700;">Polaris ログイン</h1>
          <p style="font-size: 12px; color: var(--text-muted);">プロジェクト管理ワークスペース</p>
        </div>
      </div>

      <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitLogin">
        <div>
          <label class="field-label">メールアドレス</label>
          <input v-model="form.email" type="email" required style="width: 100%;" placeholder="you@example.com" />
        </div>
        <div>
          <label class="field-label">パスワード</label>
          <input v-model="form.password" type="password" required style="width: 100%;" />
        </div>
        <button type="submit" class="btn btn-primary" style="width: 100%; padding: 9px;" :disabled="loading">
          {{ loading ? 'ログイン中...' : 'メールアドレスでログイン' }}
        </button>
      </form>

      <div v-if="googleAuthUrl" style="margin-top: 14px; padding-top: 14px; border-top: 1px solid var(--border-color);">
        <a :href="googleAuthUrl" class="btn btn-secondary" style="width: 100%; padding: 9px; text-decoration: none;">
          Google アカウントでログイン
        </a>
      </div>

      <div style="margin-top: 16px; text-align: center;">
        <button class="btn btn-ghost btn-sm" @click="showResetModal = true">
          パスワードをお忘れですか？（再設定メール送信）
        </button>
      </div>
    </div>

    <!-- 2-Factor Authentication (MFA) Challenge View -->
    <div v-else class="card" style="width: 100%; max-width: 440px; padding: 28px; display: flex; flex-direction: column; gap: 16px;">
      <div style="display: flex; align-items: center; gap: 12px;">
        <div style="width: 38px; height: 38px; border-radius: 8px; background: #2563eb; color: #fff; display: flex; align-items: center; justify-content: center;">
          <ShieldCheck :size="22" />
        </div>
        <div>
          <h1 style="font-size: 17px; font-weight: 700;">2要素認証 (MFA) 確認</h1>
          <p style="font-size: 12px; color: var(--text-muted);">アカウント保護のため追加認証を行ってください</p>
        </div>
      </div>

      <!-- MFA Method Switch Tabs -->
      <div style="display: flex; gap: 4px; padding: 3px; border-radius: 8px; background: var(--bg-elevated); border: 1px solid var(--border-color);">
        <button
          v-if="mfaState.has_webauthn"
          type="button"
          class="btn btn-sm"
          :class="mfaTab === 'webauthn' ? 'btn-primary' : 'btn-ghost'"
          style="flex: 1; font-size: 12px; padding: 5px 8px;"
          @click="mfaTab = 'webauthn'"
        >
          <Key :size="13" /> セキュリティキー
        </button>
        <button
          v-if="mfaState.has_totp"
          type="button"
          class="btn btn-sm"
          :class="mfaTab === 'totp' ? 'btn-primary' : 'btn-ghost'"
          style="flex: 1; font-size: 12px; padding: 5px 8px;"
          @click="mfaTab = 'totp'"
        >
          <Smartphone :size="13" /> 認証アプリ
        </button>
        <button
          type="button"
          class="btn btn-sm"
          :class="mfaTab === 'backup_code' ? 'btn-primary' : 'btn-ghost'"
          style="flex: 1; font-size: 12px; padding: 5px 8px;"
          @click="mfaTab = 'backup_code'"
        >
          🎫 救済コード
        </button>
      </div>

      <!-- Tab 1: WebAuthn Security Key -->
      <div v-if="mfaTab === 'webauthn'" style="display: flex; flex-direction: column; gap: 14px; text-align: center; padding: 8px 0;">
        <div style="font-size: 13px; color: var(--text-secondary); line-height: 1.6;">
          PCまたはスマートフォンに接続されたセキュリティキー（YubiKey、Touch ID、Windows Hello等）にタッチしてください。
        </div>
        <button
          type="button"
          class="btn btn-primary"
          style="padding: 12px; font-size: 14px; display: flex; align-items: center; justify-content: center; gap: 8px;"
          :disabled="isAuthenticatingMfa"
          @click="authenticateWebAuthn"
        >
          <Key :size="16" />
          <span>{{ isAuthenticatingMfa ? 'キー操作を待機中...' : '🔑 セキュリティキーにタッチして認証' }}</span>
        </button>
      </div>

      <!-- Tab 2: TOTP Passcode -->
      <form v-else-if="mfaTab === 'totp'" style="display: flex; flex-direction: column; gap: 14px;" @submit.prevent="authenticateTotp">
        <div>
          <label class="field-label" style="text-align: center; display: block;">認証アプリの6桁コードを入力</label>
          <input
            v-model="totpCode"
            type="text"
            maxlength="6"
            pattern="[0-9]*"
            inputmode="numeric"
            placeholder="123456"
            required
            autofocus
            style="width: 100%; text-align: center; font-size: 22px; font-family: monospace; letter-spacing: 0.25em; padding: 10px;"
          />
        </div>
        <button
          type="submit"
          class="btn btn-primary"
          style="width: 100%; padding: 9px;"
          :disabled="isAuthenticatingMfa || totpCode.length !== 6"
        >
          {{ isAuthenticatingMfa ? '認証中...' : '認証してログイン' }}
        </button>
      </form>

      <!-- Tab 3: Backup Code -->
      <form v-else-if="mfaTab === 'backup_code'" style="display: flex; flex-direction: column; gap: 14px;" @submit.prevent="authenticateBackupCode">
        <div>
          <label class="field-label" style="text-align: center; display: block;">保存したバックアップコードを入力</label>
          <input
            v-model="backupCode"
            type="text"
            placeholder="例: A1B2C3D4"
            required
            autofocus
            style="width: 100%; text-align: center; font-size: 16px; font-family: monospace; letter-spacing: 0.15em; padding: 10px;"
          />
        </div>
        <p style="font-size: 11px; color: var(--text-muted); text-align: center; margin: 0;">
          ※ 一度使用したバックアップコードは無効化されます。
        </p>
        <button
          type="submit"
          class="btn btn-primary"
          style="width: 100%; padding: 9px;"
          :disabled="isAuthenticatingMfa || !backupCode.trim()"
        >
          {{ isAuthenticatingMfa ? '認証中...' : 'バックアップコードでログイン' }}
        </button>
      </form>

      <!-- Back to login button -->
      <div style="border-top: 1px solid var(--border-color); padding-top: 12px; text-align: center;">
        <button type="button" class="btn btn-ghost btn-sm" @click="cancelMfa">
          <ArrowLeft :size="14" /> 別の方法でログイン / 戻る
        </button>
      </div>
    </div>

    <!-- Password Reset Request Modal -->
    <div v-if="showResetModal" class="modal-backdrop" @click.self="showResetModal = false">
      <div class="modal-panel" style="max-width: 420px;">
        <h3 style="font-size: 15px; font-weight: 700; margin-bottom: 10px;">パスワード再設定メールの送信</h3>
        <p style="font-size: 12px; color: var(--text-muted); margin-bottom: 12px;">
          登録済みメールアドレスを入力してください。1時間有効なパスワード再設定用リンクを送信します。
        </p>
        <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitResetRequest">
          <input v-model="resetEmail" type="email" required style="width: 100%;" placeholder="you@example.com" />
          <div style="display: flex; justify-content: flex-end; gap: 8px;">
            <button type="button" class="btn btn-secondary" :disabled="isSubmittingReset" @click="showResetModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary" :disabled="isSubmittingReset">
              {{ isSubmittingReset ? '送信中...' : '送信する' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ArrowLeft, Key, ShieldCheck, Smartphone } from 'lucide-vue-next'
import { isWebAuthnSupported, performWebAuthnLogin } from '~/utils/webauthn'

const router = useRouter()
const { apiFetch, notifyToast, refreshCurrentUser } = useApi()

const loading = ref(false)
const googleAuthUrl = ref('')
const showResetModal = ref(false)
const resetEmail = ref('')
const isSubmittingReset = ref(false)

const form = reactive({
  email: '',
  password: ''
})

// MFA Challenge State
const isMfaStep = ref(false)
const mfaTab = ref<'webauthn' | 'totp' | 'backup_code'>('totp')
const totpCode = ref('')
const backupCode = ref('')
const isAuthenticatingMfa = ref(false)

const mfaState = reactive({
  temp_token: '',
  has_totp: false,
  has_webauthn: false,
  allowed_methods: [] as string[]
})

onMounted(async () => {
  try {
    const st = await apiFetch<{ setup_required?: boolean; needs_setup?: boolean }>('/setup/status')
    if (st.setup_required || st.needs_setup) {
      router.replace('/setup')
      return
    }
    const g = await apiFetch<{ enabled: boolean; url: string }>('/auth/google/url')
    if (g.enabled && g.url) {
      googleAuthUrl.value = g.url
    }
  } catch {
    // ignore
  }
})

async function submitLogin() {
  loading.value = true
  try {
    const res = await apiFetch<{
      user?: any
      session_token?: string
      mfa_required?: boolean
      temp_token?: string
      has_totp?: boolean
      has_webauthn?: boolean
      allowed_methods?: string[]
    }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify(form)
    })

    if (res.mfa_required && res.temp_token) {
      mfaState.temp_token = res.temp_token
      mfaState.has_totp = Boolean(res.has_totp)
      mfaState.has_webauthn = Boolean(res.has_webauthn)
      mfaState.allowed_methods = res.allowed_methods || []
      totpCode.value = ''
      backupCode.value = ''

      if (res.has_webauthn && isWebAuthnSupported()) {
        mfaTab.value = 'webauthn'
      } else if (res.has_totp) {
        mfaTab.value = 'totp'
      } else {
        mfaTab.value = 'backup_code'
      }
      isMfaStep.value = true
      return
    }

    await refreshCurrentUser()
    notifyToast('ログインしました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    loading.value = false
  }
}

function cancelMfa() {
  isMfaStep.value = false
  mfaState.temp_token = ''
  totpCode.value = ''
  backupCode.value = ''
}

async function authenticateTotp() {
  if (!totpCode.value.trim() || isAuthenticatingMfa.value) return
  isAuthenticatingMfa.value = true
  try {
    await apiFetch('/auth/mfa/verify', {
      method: 'POST',
      body: JSON.stringify({
        temp_token: mfaState.temp_token,
        method: 'totp',
        code: totpCode.value.trim()
      })
    })
    await refreshCurrentUser()
    notifyToast('2要素認証に成功しました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message || '認証コードが正しくありません', 'error')
  } finally {
    isAuthenticatingMfa.value = false
  }
}

async function authenticateBackupCode() {
  if (!backupCode.value.trim() || isAuthenticatingMfa.value) return
  isAuthenticatingMfa.value = true
  try {
    await apiFetch('/auth/mfa/verify', {
      method: 'POST',
      body: JSON.stringify({
        temp_token: mfaState.temp_token,
        method: 'backup_code',
        code: backupCode.value.trim()
      })
    })
    await refreshCurrentUser()
    notifyToast('バックアップコードで認証しました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message || 'バックアップコードが無効です', 'error')
  } finally {
    isAuthenticatingMfa.value = false
  }
}

async function authenticateWebAuthn() {
  if (isAuthenticatingMfa.value) return
  isAuthenticatingMfa.value = true
  try {
    const opts = await apiFetch<any>('/auth/mfa/webauthn/login-options', {
      method: 'POST',
      body: JSON.stringify({ temp_token: mfaState.temp_token })
    })

    const assertion = await performWebAuthnLogin(opts)

    await apiFetch('/auth/mfa/webauthn/login-verify', {
      method: 'POST',
      body: JSON.stringify({
        temp_token: mfaState.temp_token,
        ...assertion
      })
    })

    await refreshCurrentUser()
    notifyToast('セキュリティキーで認証しました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message || 'セキュリティキー認証に失敗しました', 'error')
  } finally {
    isAuthenticatingMfa.value = false
  }
}

async function submitResetRequest() {
  if (isSubmittingReset.value) return
  if (!resetEmail.value.trim()) {
    notifyToast('メールアドレスを入力してください', 'error')
    return
  }
  isSubmittingReset.value = true
  try {
    await apiFetch('/auth/password-reset/request', {
      method: 'POST',
      body: JSON.stringify({ email: resetEmail.value.trim() })
    })
    showResetModal.value = false
    resetEmail.value = ''
    notifyToast('パスワード再設定メールを送信しました（1時間有効）', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    isSubmittingReset.value = false
  }
}
</script>


