<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;">
    <div class="card" style="width: 100%; max-width: 420px; padding: 28px;">
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
            <button type="button" class="btn btn-secondary" @click="showResetModal = false">キャンセル</button>
            <button type="submit" class="btn btn-primary">送信する</button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const router = useRouter()
const { apiFetch, notifyToast, refreshCurrentUser } = useApi()

const loading = ref(false)
const googleAuthUrl = ref('')
const showResetModal = ref(false)
const resetEmail = ref('')

const form = reactive({
  email: '',
  password: ''
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
    await apiFetch('/auth/login', {
      method: 'POST',
      body: JSON.stringify(form)
    })
    await refreshCurrentUser()
    notifyToast('ログインしました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    loading.value = false
  }
}

async function submitResetRequest() {
  try {
    await apiFetch('/auth/password-reset/request', {
      method: 'POST',
      body: JSON.stringify({ email: resetEmail.value })
    })
    showResetModal.value = false
    resetEmail.value = ''
    notifyToast('パスワード再設定メールを送信しました（1時間有効）', 'success')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}
</script>

