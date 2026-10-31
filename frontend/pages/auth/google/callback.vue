<template>
  <div style="min-height: 100vh; display: flex; align-items: center; justify-content: center;">
    <div class="card" style="padding: 24px; text-align: center;">
      <p style="font-size: 14px; font-weight: 600;">Google 認証を確認しています...</p>
    </div>
  </div>
</template>

<script setup lang="ts">
const route = useRoute()
const router = useRouter()
const { apiFetch, notifyToast, refreshCurrentUser } = useApi()

onMounted(async () => {
  const code = (route.query.code as string) || ''
  if (!code) {
    router.replace('/login')
    return
  }
  try {
    await apiFetch('/auth/google/callback', {
      method: 'POST',
      body: JSON.stringify({ code })
    })
    await refreshCurrentUser()
    notifyToast('Google アカウントでログインしました', 'success')
    router.replace('/')
  } catch (e: any) {
    notifyToast(e.message, 'error')
    router.replace('/login')
  }
})
</script>

