<!--
  LanguageSwitcher 语言切换器（v0.5.0 国际化批次）
  - 下拉菜单：简体中文 / English
  - 切换时调用 setLocale() 持久化到 localStorage
  - 与 ThemeSwitcher 并排在 BasicLayout 顶栏显示
-->
<template>
  <el-dropdown trigger="click" placement="bottom-end" @command="onCommand">
    <span class="lang-switcher" :title="t('language.title')">
      <el-icon><component :is="currentIcon" /></el-icon>
      <span class="label hidden-mobile">{{ currentLabel }}</span>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="opt in SUPPORTED_LOCALES"
          :key="opt.value"
          :command="opt.value"
          :class="{ 'is-active': opt.value === currentLocale }"
        >
          <span>{{ opt.label }}</span>
          <el-icon v-if="opt.value === currentLocale" class="check-icon"><Check /></el-icon>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check } from '@element-plus/icons-vue'
import { SUPPORTED_LOCALES, setLocale, type AppLocale } from '@/i18n'

const { t, locale } = useI18n()

const currentLocale = computed(() => locale.value as AppLocale)
const currentLabel = computed(() =>
  SUPPORTED_LOCALES.find(o => o.value === currentLocale.value)?.label || '简体中文'
)
// 中文用 ChatDotRound，英文用 English 字面图标（EP 无 English 图标，复用 Comment）
const currentIcon = computed(() => currentLocale.value === 'zh-CN' ? 'ChatDotRound' : 'Comment')

const onCommand = (cmd: AppLocale) => {
  setLocale(cmd)
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.lang-switcher {
  display: inline-flex;
  align-items: center;
  gap: $spacing-xs;
  padding: 0 $spacing-sm;
  height: 32px;
  border-radius: $radius-md;
  cursor: pointer;
  color: $color-text-regular;
  transition: background 0.2s, color 0.2s;
  // v0.7.0 修复 P1-G：flex-shrink:0 防止被父容器挤压
  flex-shrink: 0;
  white-space: nowrap;

  &:hover {
    background: $color-bg-hover;
    color: $color-primary;
  }

  .el-icon {
    font-size: 18px;
  }
  .label {
    font-size: 13px;
  }
}

:deep(.el-dropdown-menu__item.is-active) {
  color: $color-primary;
  background: $color-primary-light;
  font-weight: 500;
}

.check-icon {
  margin-left: $spacing-md;
  color: $color-primary;
}
</style>
