<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;">
    <div class="card" style="width: 100%; max-width: 440px; padding: 28px;">
      <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 18px;">
        <div style="width: 36px; height: 36px; border-radius: 8px; background: #2563eb; color: #fff; font-weight: 700; display: flex; align-items: center; justify-content: center; font-size: 18px;">
          P
        </div>
        <div>
          <h1 style="font-size: 18px; font-weight: 700;">Polaris 初期管理者セットアップ</h1>
          <p style="font-size: 12px; color: var(--text-muted);">最初のシステム管理者アカウント（全システム権限付与）を作成します</p>
        </div>
      </div>

      <form style="display: flex; flex-direction: column; gap: 14px;" @submit.prevent="submitSetup">
        <div>
          <label class="field-label">表示名 *</label>
          <input v-model="form.display_name" type="text" required style="width: 100%;" placeholder="System Admin" />
        </div>
        <div>
          <label class="field-label">メールアドレス *</label>
          <input v-model="form.email" type="email" required style="width: 100%;" placeholder="admin@example.com" />
        </div>
        <div>
          <div style="display: flex; justify-content: space-between; align-items: center;">
            <label class="field-label">パスワード（8文字以上） *</label>
            <button
              type="button"
              class="btn btn-ghost btn-sm"
              style="font-size: 11px; padding: 1px 6px;"
              @click="showPassword = !showPassword"
            >
              {{ showPassword ? '🙈 隠す' : '👁️ 表示' }}
            </button>
          </div>
          <input
            v-model="form.password"
            :type="showPassword ? 'text' : 'password'"
            minlength="8"
            required
            style="width: 100%;"
          />
        </div>
        <div>
          <label class="field-label">パスワード（確認用） *</label>
          <input
            v-model="passwordConfirm"
            :type="showPassword ? 'text' : 'password'"
            minlength="8"
            required
            style="width: 100%;"
          />
          <div v-if="passwordConfirm && form.password !== passwordConfirm" style="color: var(--danger); font-size: 11px; margin-top: 4px;">
            ⚠️ パスワードが一致しません
          </div>
        </div>
        <button
          type="submit"
          class="btn btn-primary"
          style="width: 100%; padding: 10px;"
          :disabled="loading || (Boolean(passwordConfirm) && form.password !== passwordConfirm)"
        >
          {{ loading ? '作成中...' : '初期管理者を作成して開始' }}
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
const router = useRouter()
const { apiFetch, notifyToast, refreshCurrentUser } = useApi()

const loading = ref(false)
const showPassword = ref(false)
const passwordConfirm = ref('')
const form = reactive({
  display_name: 'System Admin',
  email: '',
  password: ''
})

onMounted(async () => {
  try {
    const st = await apiFetch<{ setup_required?: boolean; needs_setup?: boolean }>('/setup/status')
    if (!st.setup_required && !st.needs_setup) {
      router.replace('/login')
    }
  } catch {
    // ignore
  }
})

async function submitSetup() {
  if (passwordConfirm.value && form.password !== passwordConfirm.value) {
    notifyToast('パスワードが一致しません', 'error')
    return
  }
  loading.value = true
  try {
    await apiFetch('/setup/init', {
      method: 'POST',
      body: JSON.stringify(form)
    })
    await refreshCurrentUser()
    notifyToast('初期管理者アカウントを作成しました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  } finally {
    loading.value = false
  }
}
</script>

