<template>
  <div>
    <NuxtLayout>
      <NuxtPage />
    </NuxtLayout>

    <!-- Global Toast Notifications -->
    <div style="position: fixed; bottom: 20px; right: 20px; z-index: 300; display: flex; flex-direction: column; gap: 8px; max-width: 380px;">
      <div
        v-for="t in toasts"
        :key="t.id"
        class="card"
        :style="{
          padding: '10px 14px',
          borderColor: t.type === 'error' ? 'var(--danger)' : t.type === 'success' ? 'var(--success)' : 'var(--primary)',
          backgroundColor: 'var(--bg-card)',
          color: 'var(--text-main)',
          boxShadow: '0 10px 25px rgba(0,0,0,0.12)',
          fontSize: '13px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '10px'
        }"
      >
        <div style="display: flex; align-items: center; gap: 8px; word-break: break-word;">
          <span style="font-size: 14px; flex-shrink: 0;">
            {{ t.type === 'success' ? '✅' : t.type === 'error' ? '❌' : t.type === 'warning' ? '⚠️' : 'ℹ️' }}
          </span>
          <span>{{ t.text }}</span>
        </div>
        <button
          type="button"
          class="btn btn-ghost btn-sm"
          style="padding: 2px 6px; font-size: 12px; line-height: 1; flex-shrink: 0;"
          title="閉じる"
          @click="removeToast(t.id)"
        >
          ✕
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const { toasts, removeToast } = usePolarisState()
</script>

