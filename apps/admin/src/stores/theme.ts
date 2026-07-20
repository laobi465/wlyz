// v0.5.0 多主题 store
//
// 主题列表：light / dark / blue / purple / green / auto（跟随系统）
// 切换方式：调用 setMode(mode) 后，document.documentElement 上的 data-theme 属性会被更新
// 持久化：通过 pinia-plugin-persistedstate 写入 localStorage（key=keyauth-theme）
//
// 铁律 04：所有主题值集中定义在 themes.scss，业务代码使用 var(--xxx)
// 铁律 05：用户偏好属纯前端状态，不进 sys_config
// 铁律 06：不编造主题值，所有可选值见 ThemeMode 类型

import { defineStore } from 'pinia'

export type ThemeMode = 'light' | 'dark' | 'blue' | 'purple' | 'green' | 'auto'

export const THEME_OPTIONS: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'light', label: '明亮', icon: 'Sunny' },
  { value: 'dark', label: '暗黑', icon: 'Moon' },
  { value: 'blue', label: '深蓝', icon: 'Water Cup' },
  { value: 'purple', label: '紫罗兰', icon: 'Magic Stick' },
  { value: 'green', label: '森林绿', icon: 'Aim' },
  { value: 'auto', label: '跟随系统', icon: 'Monitor' }
]

interface ThemeState {
  /** 当前主题模式 */
  mode: ThemeMode
}

export const useThemeStore = defineStore('theme', {
  state: (): ThemeState => ({
    mode: 'light'
  }),
  getters: {
    /** 当前生效的实际主题（auto 解析为 light 或 dark） */
    resolvedMode: (state): 'light' | 'dark' | 'blue' | 'purple' | 'green' => {
      if (state.mode !== 'auto') return state.mode
      // auto：通过 prefers-color-scheme 解析（themes.scss 中已通过媒体查询应用 dark 变量）
      if (typeof window !== 'undefined' && window.matchMedia) {
        return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
      }
      return 'light'
    },
    /** 是否为暗黑模式（dark 或 auto+系统暗黑），用于切换 EP dark class */
    isDark: (state): boolean => {
      if (state.mode === 'dark') return true
      if (state.mode === 'auto' && typeof window !== 'undefined' && window.matchMedia) {
        return window.matchMedia('(prefers-color-scheme: dark)').matches
      }
      return false
    }
  },
  actions: {
    /** 设置主题模式并应用 data-theme 属性 */
    setMode(mode: ThemeMode) {
      this.mode = mode
      this.applyToDocument()
    },
    /** 在 light / dark 之间快速切换（其他主题不变） */
    toggleLightDark() {
      this.setMode(this.mode === 'dark' ? 'light' : 'dark')
    },
    /** 将当前 mode 应用到 document.documentElement
     *  - data-theme 属性：触发 themes.scss 中对应主题的 CSS 变量
     *  - html.dark class：触发 Element Plus dark css-vars（仅 dark/auto+系统暗黑时）
     */
    applyToDocument() {
      if (typeof document === 'undefined') return
      document.documentElement.setAttribute('data-theme', this.mode)
      // EP 通过 html.dark 选择器应用暗黑样式
      document.documentElement.classList.toggle('dark', this.isDark)
    },
    /** 初始化：从持久化恢复 + 应用到 DOM + 监听系统主题变化（auto 模式生效） */
    init() {
      this.applyToDocument()
      if (typeof window === 'undefined' || !window.matchMedia) return
      const mql = window.matchMedia('(prefers-color-scheme: dark)')
      const handler = () => {
        // auto 模式下系统主题变化：重新应用 DOM（同步 html.dark class + data-theme 触发的 CSS 媒体查询）
        if (this.mode === 'auto') {
          this.applyToDocument()
        }
      }
      // addEventListener 在现代浏览器可用， Safari < 14 走 addListener 兼容
      if (mql.addEventListener) {
        mql.addEventListener('change', handler)
      } else if ((mql as any).addListener) {
        (mql as any).addListener(handler)
      }
    }
  },
  persist: {
    key: 'keyauth-theme',
    storage: localStorage,
    paths: ['mode']
  }
})
