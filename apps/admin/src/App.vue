<template>
  <!-- ElConfigProvider 响应式切换 EP 内置组件语言（随 i18n locale 同步） -->
  <el-config-provider :locale="epLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import elZhCn from 'element-plus/es/locale/lang/zh-cn'
import elEnUs from 'element-plus/es/locale/lang/en'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'

const { locale } = useI18n()
const epLocale = computed(() => (locale.value === 'en-US' ? elEnUs : elZhCn))

// v0.9.0 修复：刷新页面后 persist 仅恢复 state 字段，_refreshTimer 定时器丢失
// 需要在 app 启动时主动重建 access token 自动续期定时器，否则 token 过期后无法续期
// 触发场景：管理员登录后刷新页面，定时器丢失 → token 过期 → 任何 API 401 → refresh 失败 → logout
onMounted(() => {
  const auth = useAuthStore()
  const theme = useThemeStore()
  theme.init()
  if (auth.isLoggedIn && auth.refreshToken) {
    auth.scheduleRefresh()
  }
})
</script>

<style lang="scss">
html, body, #app {
  height: 100%;
  margin: 0;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
</style>
