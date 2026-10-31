// Firebase Cloud Messaging Service Worker for Polaris
self.addEventListener('push', (event) => {
  if (!event.data) return
  try {
    const payload = event.data.json()
    const title = payload.notification?.title || 'Polaris 通知'
    const options = {
      body: payload.notification?.body || '',
      data: payload.data || {}
    }
    event.waitUntil(self.registration.showNotification(title, options))
  } catch {
    // Fallback text notification
  }
})

