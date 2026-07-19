<!--
  TenantLayout 开发者后台布局
  包装 BasicLayout，注入开发者专属信息（套餐标签、开发者公告）
-->
<template>
  <BasicLayout
    route-prefix="/tenant"
    logo-text="开发者中心"
    home-path="/tenant/dashboard"
    home-title="开发者中心"
  >
    <template #header-extra>
      <el-tag v-if="packageName" size="small" type="success" class="hidden-mobile">{{ packageName }}</el-tag>
    </template>
  </BasicLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import BasicLayout from './BasicLayout.vue'

const packageName = ref('')

onMounted(() => {
  // TODO: 待对接 /tenant/auth/me 接口获取套餐名
  // 当前使用占位（铁律 04：不编造数据）
})
</script>
