<!--
  ThemeSwitcher 多主题切换器
  - 下拉菜单：light / dark / blue / purple / green / auto
  - 当前主题图标显示在触发按钮上
  - 切换时立即生效，并通过 theme store 持久化到 localStorage
-->
<template>
  <el-dropdown trigger="click" @command="onCommand">
    <span class="theme-switcher" :title="'切换主题（当前：' + currentLabel + '）'">
      <el-icon><component :is="currentIcon" /></el-icon>
      <span class="label hidden-mobile">{{ currentLabel }}</span>
    </span>
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item
          v-for="opt in THEME_OPTIONS"
          :key="opt.value"
          :command="opt.value"
          :class="{ 'is-active': opt.value === themeStore.mode }"
        >
          <el-icon><component :is="opt.icon" /></el-icon>
          <span>{{ opt.label }}</span>
          <el-icon v-if="opt.value === themeStore.mode" class="check-icon"><Check /></el-icon>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { Check } from '@element-plus/icons-vue'
import { useThemeStore, THEME_OPTIONS, type ThemeMode } from '@/stores/theme'

const themeStore = useThemeStore()

const currentOption = computed(() =>
  THEME_OPTIONS.find(o => o.value === themeStore.mode) || THEME_OPTIONS[0]
)
const currentLabel = computed(() => currentOption.value.label)
const currentIcon = computed(() => currentOption.value.icon)

const onCommand = (cmd: ThemeMode) => {
  themeStore.setMode(cmd)
}

onMounted(() => {
  // 应用持久化的主题到 DOM（防止刷新后未应用）
  themeStore.init()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.theme-switcher {
  display: inline-flex;
  align-items: center;
  gap: $spacing-xs;
  padding: 0 $spacing-sm;
  height: 32px;
  border-radius: $radius-md;
  cursor: pointer;
  color: $color-text-regular;
  transition: background 0.2s, color 0.2s;

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
  margin-left: auto;
  margin-left: $spacing-md;
  color: $color-primary;
}
</style>
