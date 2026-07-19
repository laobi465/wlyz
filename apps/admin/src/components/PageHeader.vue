<!--
  PageHeader 页面标题组件
  - 桌面端：标题 + 操作按钮一行
  - 移动端：标题与操作分两行
-->
<template>
  <div class="page-header">
    <div class="header-left">
      <h2>{{ title }}</h2>
      <p v-if="subtitle" class="subtitle">{{ subtitle }}</p>
    </div>
    <div class="page-header-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  title: string
  subtitle?: string
}>()
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: $spacing-lg;
  flex-wrap: wrap;
  gap: $spacing-sm;

  .header-left {
    h2 {
      margin: 0;
      font-size: 18px;
      font-weight: 600;
      color: $color-text-primary;
    }
    .subtitle {
      margin: 4px 0 0;
      font-size: 13px;
      color: $color-text-secondary;
    }
  }

  .page-header-actions {
    display: flex;
    gap: $spacing-sm;
    flex-wrap: wrap;
  }

  @include mobile {
    margin-bottom: $spacing-md;
    .header-left h2 { font-size: 16px; }
  }
}
</style>
