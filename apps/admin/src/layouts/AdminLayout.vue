<!--
  AdminLayout 平台超管布局
  包装 BasicLayout，注入超管专属菜单与角色标识
  v0.4.0：挂载 UpdateNotifier 组件，启用更新弹窗通知轮询
-->
<template>
  <div class="admin-layout-wrap">
    <BasicLayout
      route-prefix="/admin"
      logo-text="KeyAuth 管理后台"
      home-path="/admin/dashboard"
      home-title="管理后台"
    />
    <!-- v0.4.0 更新弹窗通知（轻量轮询 /admin/update/poll，无 UI 仅逻辑） -->
    <UpdateNotifier />
  </div>
</template>

<script setup lang="ts">
import BasicLayout from './BasicLayout.vue'
import UpdateNotifier from '@/components/UpdateNotifier.vue'
</script>

<style scoped>
.admin-layout-wrap {
  height: 100%;
}
</style>
