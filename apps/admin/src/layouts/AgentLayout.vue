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
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import BasicLayout from './BasicLayout.vue'
import { agentMeApi } from '@/api/agent'

const balance = ref(0)
const route = useRoute()

const loadBalance = async () => {
  try {
    const data = await agentMeApi()
    if (data && typeof data.balance === 'number') {
      balance.value = data.balance
    }
  } catch {
    // 铁律 06 待核实：后端 /agent/auth/me 当前可能正常返回（复用 CurrentUser handler）
  }
}

// 路由切换到关键页面后刷新余额（购卡/提现后实时反映）
watch(() => route.path, () => {
  loadBalance()
})

onMounted(loadBalance)
</script>
