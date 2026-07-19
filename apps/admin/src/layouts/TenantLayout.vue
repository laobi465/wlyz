<template>
  <el-container class="layout">
    <!-- 三层级公告同时显示：平台公告 + 开发者公告 -->
    <PlatformNoticeBanner v-if="sysConfig.noticeBannerEnabled" />
    <DeveloperNoticeBanner v-if="devNotice" :notice="devNotice" />

    <el-container>
      <!-- 侧边栏 -->
      <el-aside :width="collapsed ? '64px' : '220px'" class="layout-aside">
        <div class="logo">
          <img src="@/assets/logo.svg" alt="logo" v-if="!collapsed" />
          <span v-if="!collapsed" class="logo-text">{{ sysConfig.platformName }}</span>
          <img src="@/assets/logo.svg" alt="logo" v-else class="logo-mini" />
        </div>
        <el-menu
          :default-active="route.path"
          :collapse="collapsed"
          :collapse-transition="false"
          router
          background-color="#fff"
          text-color="#303133"
          active-text-color="#1677ff"
        >
          <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <el-container>
        <!-- 顶部 -->
        <el-header class="layout-header">
          <div class="header-left">
            <el-icon class="collapse-btn" @click="collapsed = !collapsed">
              <Fold v-if="!collapsed" />
              <Expand v-else />
            </el-icon>
            <el-breadcrumb separator="/">
              <el-breadcrumb-item :to="{ path: '/tenant/dashboard' }">开发者中心</el-breadcrumb-item>
              <el-breadcrumb-item>{{ route.meta.title }}</el-breadcrumb-item>
            </el-breadcrumb>
          </div>
          <div class="header-right">
            <el-tag size="small" type="success">{{ packageName || '免费版' }}</el-tag>
            <el-dropdown @command="handleCommand">
              <span class="user-info">
                <el-avatar :size="28" icon="UserFilled" />
                <span class="username">{{ auth.username }}</span>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="profile" @click.stop="router.push('/tenant/profile')">账号设置</el-dropdown-item>
                  <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </el-header>

        <!-- 内容区 -->
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
import DeveloperNoticeBanner from '@/components/DeveloperNoticeBanner.vue'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sysConfig = useSysConfigStore()

const collapsed = ref(false)
const packageName = ref('')
const devNotice = ref<{ title: string; content: string } | null>(null)

const menus = computed(() => {
  const tenantRoute = router.getRoutes().find(r => r.path === '/tenant')
  if (!tenantRoute?.children) return []
  return tenantRoute.children
    .filter(child => !child.meta?.public)
    .map(child => ({
      path: `/tenant/${child.path}`,
      title: (child.meta?.title as string) || '',
      icon: (child.meta?.icon as string) || 'Menu'
    }))
})

onMounted(async () => {
  await sysConfig.load()
  // 待实现：加载开发者套餐信息与开发者公告
  // try {
  //   const profile = await getTenantProfile()
  //   packageName.value = profile.package_name
  //   const notices = await listActiveNotices({ level: 'developer', tenant_id: auth.tenantId })
  //   devNotice.value = notices[0] || null
  // } catch (e) { /* 静默 */ }
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
  background: #fff;
  border-right: 1px solid #e4e7ed;
  transition: width 0.2s;
  overflow: hidden;
  .logo {
    height: 56px;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 0 16px;
    img { width: 28px; height: 28px; }
    .logo-mini { width: 32px; height: 32px; margin: 0 auto; }
    .logo-text { font-size: 16px; font-weight: 600; white-space: nowrap; color: #303133; }
  }
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
