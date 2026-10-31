<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;">
    <div class="card" style="width: 100%; max-width: 420px; padding: 28px;">
      <h1 style="font-size: 17px; font-weight: 700; margin-bottom: 8px;">招待の受諾・初期アカウント設定</h1>
      <p style="font-size: 12px; color: var(--text-muted); margin-bottom: 16px;">
        表示名とパスワードを設定してアカウントを有効化します（招待トークン有効期限: 72時間）。
      </p>
      <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitAccept">
        <div>
          <label class="field-label">招待トークン</label>
          <input v-model="form.token" type="text" required style="width: 100%;" />
        </div>
        <div>
          <label class="field-label">表示名</label>
          <input v-model="form.display_name" type="text" required style="width: 100%;" />
        </div>
        <div>
          <label class="field-label">初期パスワード（8文字以上）</label>
          <input v-model="form.password" type="password" minlength="8" required style="width: 100%;" />
        </div>
        <button type="submit" class="btn btn-primary" style="width: 100%; padding: 9px;">アカウントを有効化してログイン</button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const { apiFetch, notifyToast, refreshCurrentUser } = useApi()

const form = reactive({
  token: (route.query.token as string) || '',
  display_name: '',
  password: ''
})

async function submitAccept() {
  try {
    await apiFetch('/auth/invite/accept', {
      method: 'POST',
      body: JSON.stringify(form)
    })
    await refreshCurrentUser()
    notifyToast('招待を受諾し、アカウントを有効化しました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}
</script>

