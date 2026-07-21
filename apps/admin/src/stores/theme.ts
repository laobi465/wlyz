// v0.8.0 单一明亮主题 store
//
// 历史背景：
//   v0.5.0 引入多主题（light/dark/blue/purple/green/auto）
//   v0.8.0 应产品要求去除多主题与暗黑模式，仅保留明亮主题
//
// 兼容性处理：
//   - 旧用户 localStorage 中可能存在 keyauth-theme=dark/blue/purple/green/auto
//   - init() 时强制清理旧值并设置为 light，移除 html.dark class
//   - 保留 store 是为了让其他可能引用 useThemeStore 的地方不报错（极简桩）
//
// 铁律 04：所有主题值集中定义在 themes.scss（v0.8.0 起只有 :root 默认明亮）
// 铁律 05：用户偏好属纯前端状态，不进 sys_config

import { defineStore } from 'pinia'

export type ThemeMode = 'light'

export const THEME_OPTIONS: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'light', label: '明亮', icon: 'Sunny' }
]

interface ThemeState {
  /** 当前主题模式（v0.8.0 起固定为 light） */
  mode: ThemeMode
}

export const useThemeStore = defineStore('theme', {
  state: (): ThemeState => ({
    mode: 'light'
  }),
  actions: {
    /** 设置主题模式（v0.8.0 起仅接受 light，传入其他值会被忽略） */
    setMode(mode: ThemeMode) {
      this.mode = 'light'
      this.applyToDocument()
    },
    /** 将 light 主题应用到 document.documentElement
     *  v0.8.0：清除旧 data-theme 属性 + 移除 html.dark class（兼容旧暗黑用户）
     */
    applyToDocument() {
      if (typeof document === 'undefined') return
      // 清除旧的 data-theme 属性（dark/blue/purple/green/auto）
      document.documentElement.removeAttribute('data-theme')
      // 移除暗黑模式 class（EP 通过 html.dark 应用暗黑样式）
      document.documentElement.classList.remove('dark')
    },
    /** 初始化：清理旧主题数据 + 应用明亮主题
     *  v0.8.0：移除 matchMedia 监听器（不再需要响应系统主题变化）
     *  v0.8.0：清理 localStorage 中的旧主题值（dark/blue/purple/green/auto）
     */
    init() {
      this.applyToDocument()
      // v0.8.0：清理旧用户遗留的主题数据
      if (typeof window !== 'undefined' && window.localStorage) {
        const old = window.localStorage.getItem('keyauth-theme')
        if (old && old !== '{"mode":"light"}') {
          // 旧值可能是 '{"mode":"dark"}' 等，清理掉
          window.localStorage.removeItem('keyauth-theme')
        }
      }
    }
  },
  persist: {
    key: 'keyauth-theme',
    storage: localStorage,
    paths: ['mode']
  }
})
