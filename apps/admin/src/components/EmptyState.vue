<!--
  EmptyState 空状态组件
-->
<template>
  <div class="empty-state">
    <el-empty :description="description" :image-size="imageSize">
      <template v-if="$slots.action" #default>
        <slot name="action" />
      </template>
    </el-empty>
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
  description?: string
  imageSize?: number
}>(), {
  description: '暂无数据',
  imageSize: 100
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.empty-state {
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
  text-align: center;
}
</style>
