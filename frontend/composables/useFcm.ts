import { initializeApp, getApps } from 'firebase/app'
import { getMessaging, getToken, onMessage, isSupported } from 'firebase/messaging'

export function useFcm() {
  const config = useRuntimeConfig()
  const { apiFetch, notifyToast, refreshNotifications } = useApi()

  async function registerPushNotifications() {
    if (typeof window === 'undefined') return
    const apiKey = config.public.firebaseApiKey as string
    const projectId = config.public.firebaseProjectId as string
    const messagingSenderId = config.public.firebaseMessagingSenderId as string
    const appId = config.public.firebaseAppId as string
    const vapidKey = config.public.firebaseVapidKey as string

    if (!apiKey || !projectId || !messagingSenderId || !appId) {
      return
    }

    try {
      const supported = await isSupported()
      if (!supported) return

      const app = getApps().length
        ? getApps()[0]
        : initializeApp({
            apiKey,
            authDomain: config.public.firebaseAuthDomain as string,
            projectId,
            messagingSenderId,
            appId
          })

      const messaging = getMessaging(app)
      const permission = await Notification.requestPermission()
      if (permission !== 'granted') return

      const token = await getToken(messaging, vapidKey ? { vapidKey } : undefined)
      if (token) {
        await apiFetch('/fcm-tokens', {
          method: 'POST',
          body: JSON.stringify({
            token,
            device_name: navigator.userAgent.slice(0, 120)
          })
        })
      }

      onMessage(messaging, (payload) => {
        const body = payload.notification?.body || '新しい通知があります'
        notifyToast(`🔔 ${body}`, 'info')
        refreshNotifications()
      })
    } catch {
      // Silently ignore if browser blocks push or Firebase config is optional
    }
  }

  return {
    registerPushNotifications
  }
}

