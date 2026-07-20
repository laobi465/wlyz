import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPersist from 'pinia-plugin-persistedstate'
import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'
// v0.5.0 多主题：EP 暗黑模式 css 变量（仅在 data-theme="dark"/"auto" 时生效，
// 因为 EP 通过 html.dark 选择器应用，而我们使用 html[data-theme] 切换，
// themes.scss 已在 dark 主题下覆盖 --el-* 变量，此文件作为兜底补充）
import 'element-plus/theme-chalk/dark/css-vars.css'
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
