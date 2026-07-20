<template>
  <!-- ElConfigProvider 响应式切换 EP 内置组件语言（随 i18n locale 同步） -->
  <el-config-provider :locale="epLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import elZhCn from 'element-plus/es/locale/lang/zh-cn'
import elEnUs from 'element-plus/es/locale/lang/en'

const { locale } = useI18n()
const epLocale = computed(() => (locale.value === 'en-US' ? elEnUs : elZhCn))
</script>

<style lang="scss">
html, body, #app {
  height: 100%;
  margin: 0;
  padding: 0;
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
}
</style>
