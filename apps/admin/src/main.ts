import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPersist from 'pinia-plugin-persistedstate'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
// v0.8.0：移除 element-plus/theme-chalk/dark/css-vars.css（去除多主题与暗黑模式）
import * as ElementPlusIconsVue from '@element-plus/icons-vue'

import App from './App.vue'
import router from './router'
import i18n, { applyLocale } from './i18n'
import './styles/index.scss'

const app = createApp(App)

// 注册所有 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

const pinia = createPinia()
pinia.use(piniaPersist)

app.use(pinia)
app.use(router)
app.use(i18n)
// Element Plus locale 由 App.vue 内 ElConfigProvider 响应式控制，此处仅注册插件
app.use(ElementPlus)

// 应用持久化的语言到 DOM（lang 属性）
applyLocale()

app.mount('#app')
