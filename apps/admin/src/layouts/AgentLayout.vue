<!--
  AgentLayout 代理后台布局
  包装 BasicLayout，注入代理专属信息（余额标签）
-->
<template>
  <BasicLayout
    route-prefix="/agent"
    logo-text="代理中心"
    home-path="/agent/dashboard"
    home-title="代理中心"
  >
    <template #header-extra>
      <el-tag size="small" type="warning">余额 ¥{{ balance.toFixed(2) }}</el-tag>
    </template>
  </BasicLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import BasicLayout from './BasicLayout.vue'

const balance = ref(0)

onMounted(() => {
  // TODO: 待对接 /agent/auth/me 接口获取余额
  // 当前使用占位（铁律 04：不编造数据）
})
</script>
