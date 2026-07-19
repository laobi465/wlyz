<template>
  <el-container class="layout">
    <!-- 代理可见平台公告 + 来自开发者的通知（agent_notify 类型） -->
    <PlatformNoticeBanner v-if="sysConfig.noticeBannerEnabled" />
    <AgentNotifyBanner v-if="agentNotify" :notice="agentNotify" />

    <el-container>
      <el-aside :width="collapsed ? '64px' : '220px'" class="layout-aside">
        <div class="logo">
          <img src="@/assets/logo.svg" alt="logo" v-if="!collapsed" />
          <span v-if="!collapsed" class="logo-text">代理中心</span>
          <img src="@/assets/logo.svg" alt="logo" v-else class="logo-mini" />
        </div>
        <el-menu
          :default-active="route.path"
          :collapse="collapsed"
          :collapse-transition="false"
          router
          background-color="#1f2937"
          text-color="#cbd5e1"
          active-text-color="#fff"
        >
          <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-container>
        <el-header class="layout-header">
          <div class="header-left">
            <el-icon class="collapse-btn" @click="collapsed = !collapsed">
              <Fold v-if="!collapsed" />
              <Expand v-else />
            </el-icon>
            <el-breadcrumb separator="/">
              <el-breadcrumb-item :to="{ path: '/agent/dashboard' }">代理中心</el-breadcrumb-item>
              <el-breadcrumb-item>{{ route.meta.title }}</el-breadcrumb-item>
            </el-breadcrumb>
          </div>
          <div class="header-right">
            <el-tag size="small" type="warning">余额 ¥ {{ balance.toFixed(2) }}</el-tag>
            <el-dropdown @command="handleCommand">
              <span class="user-info">
                <el-avatar :size="28" icon="UserFilled" />
                <span class="username">{{ auth.username }}</span>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="profile" @click.stop="router.push('/agent/profile')">账号设置</el-dropdown-item>
                  <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-header>

        <el-main class="layout-main">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </el-main>
      </el-container>
    </el-container>
  </el-container>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useAuthStore } from '@/stores/auth'
import { useSysConfigStore } from '@/stores/sysConfig'
import PlatformNoticeBanner from '@/components/PlatformNoticeBanner.vue'
import AgentNotifyBanner from '@/components/AgentNotifyBanner.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sysConfig = useSysConfigStore()

const collapsed = ref(false)
const balance = ref(0)
const agentNotify = ref<{ title: string; content: string } | null>(null)

const menus = computed(() => {
  const agentRoute = router.getRoutes().find(r => r.path === '/agent')
  if (!agentRoute?.children) return []
  return agentRoute.children
    .filter(child => !child.meta?.public)
    .map(child => ({
      path: `/agent/${child.path}`,
      title: (child.meta?.title as string) || '',
      icon: (child.meta?.icon as string) || 'Menu'
    }))
})

onMounted(async () => {
  await sysConfig.load()
  // 待实现：加载代理余额 + agent_notify 公告（开发者切换支付方式时下发）
})

const handleCommand = async (cmd: string) => {
  if (cmd === 'logout') {
    await ElMessageBox.confirm('确定退出登录吗？', '提示', { type: 'warning' })
    auth.logout()
    router.push('/login')
  }
}
</script>

<style scoped lang="scss">
.layout { height: 100vh; }
.layout-aside {
  background: #1f2937;
  transition: width 0.2s;
  overflow: hidden;
  .logo {
    height: 56px;
    display: flex; align-items: center; gap: 8px; padding: 0 16px;
    img { width: 28px; height: 28px; }
    .logo-mini { width: 32px; height: 32px; margin: 0 auto; }
    .logo-text { font-size: 16px; font-weight: 600; white-space: nowrap; color: #fff; }
  }
  :deep(.el-menu) { border-right: none; }
}
.layout-header {
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  display: flex; align-items: center; justify-content: space-between;
  padding: 0 16px; height: 56px;
  .header-left {
    display: flex; align-items: center; gap: 16px;
    .collapse-btn { font-size: 20px; cursor: pointer; &:hover { color: var(--el-color-primary); } }
  }
  .header-right {
    display: flex; align-items: center; gap: 12px;
    .user-info {
      display: flex; align-items: center; gap: 8px; cursor: pointer;
      .username { font-size: 14px; color: #303133; }
    }
  }
}
.layout-main { background: #f5f7fa; padding: 16px; overflow-y: auto; }
.fade-enter-active, .fade-leave-active { transition: opacity 0.15s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
