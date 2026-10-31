<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px;">
    <div class="card" style="width: 100%; max-width: 420px; padding: 28px; text-align: center;">
      <h1 style="font-size: 17px; font-weight: 700; margin-bottom: 10px;">メールアドレス変更の確認</h1>
      <p style="font-size: 13px; color: var(--text-muted); margin-bottom: 16px;">{{ statusText }}</p>
      <NuxtLink to="/login" class="btn btn-primary">ログイン画面へ移動</NuxtLink>
    </div>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const { apiFetch } = useApi()
const statusText = ref('確認処理を実行しています...')

onMounted(async () => {
  const token = (route.query.token as string) || ''
  if (!token) {
    statusText.value = '確認トークンが指定されていません。'
    return
  }
  try {
    await apiFetch('/auth/email-change/confirm', {
      method: 'POST',
      body: JSON.stringify({ token })
    })
    statusText.value = 'メールアドレスの変更が完了しました。新しいメールアドレスでログインしてください。'
  } catch (e: any) {
    statusText.value = `エラー: ${e.message}`
  }
})
</script>

