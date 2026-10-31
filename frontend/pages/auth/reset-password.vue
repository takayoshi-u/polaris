<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;">
    <div class="card" style="width: 100%; max-width: 420px; padding: 28px;">
      <h1 style="font-size: 17px; font-weight: 700; margin-bottom: 8px;">パスワード再設定</h1>
      <p style="font-size: 12px; color: var(--text-muted); margin-bottom: 16px;">
        新しいパスワードを入力してください（トークン有効期限: 1時間）。
      </p>
      <form style="display: flex; flex-direction: column; gap: 12px;" @submit.prevent="submitReset">
        <div>
          <label class="field-label">再設定トークン</label>
          <input v-model="form.token" type="text" required style="width: 100%;" />
        </div>
        <div>
          <label class="field-label">新しいパスワード（8文字以上）</label>
          <input v-model="form.new_password" type="password" minlength="8" required style="width: 100%;" />
        </div>
        <button type="submit" class="btn btn-primary" style="width: 100%; padding: 9px;">パスワードを変更する</button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const { apiFetch, notifyToast } = useApi()

const form = reactive({
  token: (route.query.token as string) || '',
  new_password: ''
})

async function submitReset() {
  try {
    await apiFetch('/auth/password-reset/confirm', {
      method: 'POST',
      body: JSON.stringify(form)
    })
    notifyToast('パスワードを再設定しました。ログインしてください', 'success')
    router.replace('/login')
  } catch (e: any) {
    notifyToast(e.message, 'error')
  }
}
</script>

