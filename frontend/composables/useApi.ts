import type { User, Project, NotificationItem } from '~/types/polaris'

export interface ToastMessage {
  id: number
  type: 'info' | 'success' | 'error' | 'warning'
  text: string
}

export function usePolarisState() {
  const currentUser = useState<User | null>('polaris_current_user', () => null)
  const projects = useState<Project[]>('polaris_projects', () => [])
  const notifications = useState<NotificationItem[]>('polaris_notifications', () => [])
  const toasts = useState<ToastMessage[]>('polaris_toasts', () => [])

  function notifyToast(text: string, type: ToastMessage['type'] = 'info') {
    const id = Date.now() + Math.floor(Math.random() * 1000)
    toasts.value.push({ id, type, text })
    if (typeof window !== 'undefined') {
      setTimeout(() => {
        toasts.value = toasts.value.filter(t => t.id !== id)
      }, 4500)
    }
  }

  return {
    currentUser,
    projects,
    notifications,
    toasts,
    notifyToast
  }
}

export function useApi() {
  const config = useRuntimeConfig()
  const baseUrl = config.public.apiBaseUrl as string
  const { currentUser, projects, notifications, notifyToast } = usePolarisState()

  async function apiFetch<T = any>(path: string, options: RequestInit = {}): Promise<T> {
    const url = path.startsWith('http') ? path : `${baseUrl}${path}`
    const headers = new Headers(options.headers || {})
    if (!(options.body instanceof FormData) && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }
    const res = await fetch(url, {
      ...options,
      headers,
      credentials: 'include'
    })
    if (res.status === 204) {
      return undefined as unknown as T
    }
    const contentType = res.headers.get('Content-Type') || ''
    if (contentType.includes('text/csv') || contentType.includes('application/octet-stream')) {
      if (!res.ok) {
        throw new Error(`API Error (${res.status})`)
      }
      const blob = await res.blob()
      return blob as unknown as T
    }
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      const err: any = new Error(data?.error || `API Error (${res.status})`)
      err.status = res.status
      err.data = data
      throw err
    }
    return data as T
  }

  async function refreshCurrentUser(): Promise<User | null> {
    try {
      const u = await apiFetch<User>('/auth/me')
      currentUser.value = u
      return u
    } catch {
      currentUser.value = null
      return null
    }
  }

  async function refreshProjects(): Promise<Project[]> {
    try {
      const list = await apiFetch<Project[]>('/projects')
      projects.value = list || []
      return projects.value
    } catch {
      projects.value = []
      return []
    }
  }

  async function refreshNotifications(): Promise<NotificationItem[]> {
    try {
      const list = await apiFetch<NotificationItem[]>('/notifications')
      notifications.value = list || []
      return notifications.value
    } catch {
      notifications.value = []
      return []
    }
  }

  function hasSystemPerm(perm: 'create_project' | 'delete_project' | 'manage_system'): boolean {
    return Boolean(currentUser.value?.permissions?.includes(perm))
  }

  return {
    baseUrl,
    apiFetch,
    currentUser,
    projects,
    notifications,
    notifyToast,
    refreshCurrentUser,
    refreshProjects,
    refreshNotifications,
    hasSystemPerm
  }
}

