// v0.5.0 i18n 初始化
// - 支持 zh-CN（默认）和 en-US
// - 通过 pinia-plugin-persistedstate 持久化语言选择
// - 业务页面通过 useI18n() 或 $t() 访问翻译
//
// 铁律 04：所有 i18n key 集中在 locales 目录，禁止业务代码硬编码中英文
// 铁律 06：legacy: false 启用 Composition API 模式，避免 Options API 混淆

import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN'
import enUS from './locales/en-US'

export type AppLocale = 'zh-CN' | 'en-US'

export const SUPPORTED_LOCALES: { value: AppLocale; label: string }[] = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'en-US', label: 'English' }
]

/**
 * 从 localStorage 恢复用户选择的语言（keyauth-locale）
 * 首次访问时根据浏览器语言自动判定
 */
export function getDefaultLocale(): AppLocale {
  const stored = localStorage.getItem('keyauth-locale')
  if (stored === 'zh-CN' || stored === 'en-US') return stored
  // 根据浏览器语言自动选择
  const browserLang = navigator.language || (navigator as any).userLanguage || 'zh-CN'
  return browserLang.startsWith('zh') ? 'zh-CN' : 'en-US'
}

const i18n = createI18n({
  legacy: false, // 启用 Composition API 模式
  locale: getDefaultLocale(),
  fallbackLocale: 'zh-CN',
  messages: {
    'zh-CN': zhCN,
    'en-US': enUS
  }
})

export default i18n

/**
 * 切换语言（业务代码调用）
 * 同时更新 localStorage + document.documentElement.lang
 */
export function setLocale(locale: AppLocale) {
  i18n.global.locale.value = locale
  localStorage.setItem('keyauth-locale', locale)
  document.documentElement.setAttribute('lang', locale)
}

/**
 * 应用持久化的语言到 DOM（在 main.ts 启动时调用一次）
 */
export function applyLocale() {
  const locale = i18n.global.locale.value as AppLocale
  document.documentElement.setAttribute('lang', locale)
}
