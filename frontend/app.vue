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
          fontSize: '13px'
        }"
      >
        {{ t.text }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const { toasts } = usePolarisState()
</script>

