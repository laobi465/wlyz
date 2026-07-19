<!--
  AdminLayout 平台超管布局
  包装 BasicLayout，注入超管专属菜单与角色标识
-->
<template>
  <BasicLayout
    route-prefix="/admin"
    logo-text="KeyAuth 管理后台"
    home-path="/admin/dashboard"
    home-title="管理后台"
  />
</template>

<script setup lang="ts">
import BasicLayout from './BasicLayout.vue'
</script>
