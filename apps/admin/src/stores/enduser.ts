// H5 终端用户登录态（独立于三角色 auth store，避免污染）
// v0.4.0 收尾项 C
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { EndUserInfo } from '@/api/enduser'

const STORAGE_KEY = 'keyauth-enduser'

interface PersistPayload {
  accessToken: string
  refreshToken: string
  expiresAt: number
  user: EndUserInfo | null
  appKey: string
}

export const useEndUserStore = defineStore('enduser', () => {
  const accessToken = ref<string>('')
  const refreshToken = ref<string>('')
  const expiresAt = ref<number>(0)
  const user = ref<EndUserInfo | null>(null)
  // H5 用户需要绑定 app_key（登录、注册、验证码等接口都需要）
  const appKey = ref<string>('')

  const isLoggedIn = computed(() => !!accessToken.value)

  /** 是否已从本地存储恢复过状态 */
  const _restored = ref(false)

  function setLogin(data: {
    access_token: string
    refresh_token: string
    expires_at: number
    user: EndUserInfo
  }) {
    accessToken.value = data.access_token
    refreshToken.value = data.refresh_token
    expiresAt.value = data.expires_at
    user.value = data.user
    persist()
  }

  function setUser(u: EndUserInfo | null) {
    user.value = u
    persist()
  }

  function setAppKey(key: string) {
    appKey.value = key
    persist()
  }

  function clear() {
    accessToken.value = ''
    refreshToken.value = ''
    expiresAt.value = 0
    user.value = null
    // appKey 不清空：方便下次登录自动填充
    persist()
  }

  function clearAll() {
    accessToken.value = ''
    refreshToken.value = ''
    expiresAt.value = 0
    user.value = null
    appKey.value = ''
    localStorage.removeItem(STORAGE_KEY)
  }

  function persist() {
    const payload: PersistPayload = {
      accessToken: accessToken.value,
      refreshToken: refreshToken.value,
      expiresAt: expiresAt.value,
      user: user.value,
      appKey: appKey.value
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(payload))
  }

  function restore() {
    if (_restored.value) return
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      _restored.value = true
      return
    }
    try {
      const data: PersistPayload = JSON.parse(raw)
      accessToken.value = data.accessToken || ''
      refreshToken.value = data.refreshToken || ''
      expiresAt.value = data.expiresAt || 0
      user.value = data.user || null
      appKey.value = data.appKey || ''
    } catch {
      // 数据损坏：忽略
    }
    _restored.value = true
  }

  return {
    accessToken,
    refreshToken,
    expiresAt,
    user,
    appKey,
    isLoggedIn,
    setLogin,
    setUser,
    setAppKey,
    clear,
    clearAll,
    restore
  }
})
