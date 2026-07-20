<!--
  BasicLayout 通用响应式布局
  - 桌面端：固定侧边栏 + 顶部 + 内容区
  - 平板：可折叠侧边栏
  - 移动端：抽屉式侧边栏 + 简化顶栏
  - v0.5.0 多主题支持：顶栏内嵌 ThemeSwitcher，6 主题可切换（light/dark/blue/purple/green/auto）
-->
<template>
  <div class="basic-layout">
    <!-- 平台公告横幅（由 sysConfig 控制） -->
    <PlatformNoticeBanner v-if="sysConfig.noticeBannerEnabled" />

    <!-- 二级公告插槽（开发者公告 / 代理通知） -->
    <slot name="secondary-banner" />

    <div class="layout-body">
      <!-- 桌面/平板：固定侧边栏 -->
      <aside
        v-if="!isMobile"
        class="layout-aside"
        :class="{ collapsed }"
      >
        <div class="logo">
          <img src="@/assets/logo.svg" alt="logo" />
          <span v-if="!collapsed" class="logo-text">{{ logoText }}</span>
        </div>
        <el-menu
          :default-active="route.path"
          :collapse="collapsed"
          :collapse-transition="false"
          router
          class="aside-menu"
        >
          <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </el-menu>
      </aside>

      <!-- 移动端：抽屉式侧边栏 -->
      <el-drawer
        v-if="isMobile"
        v-model="drawerVisible"
        direction="ltr"
        :size="260"
        :with-header="false"
      >
        <div class="drawer-logo">
          <img src="@/assets/logo.svg" alt="logo" />
          <span class="logo-text">{{ logoText }}</span>
        </div>
        <el-menu
          :default-active="route.path"
          router
          class="drawer-menu"
          @select="onMenuSelect"
        >
          <el-menu-item v-for="item in menus" :key="item.path" :index="item.path">
            <el-icon><component :is="item.icon" /></el-icon>
            <template #title>{{ item.title }}</template>
          </el-menu-item>
        </el-menu>
      </el-drawer>

      <div class="layout-main-wrap">
        <!-- 顶部栏 -->
        <header class="layout-header">
          <div class="header-left">
            <!-- 移动端：菜单按钮 -->
            <el-icon v-if="isMobile" class="menu-btn" @click="drawerVisible = true">
              <Menu />
            </el-icon>
            <!-- 桌面端：折叠按钮 -->
            <el-icon v-else class="collapse-btn" @click="collapsed = !collapsed">
              <Fold v-if="!collapsed" />
              <Expand v-else />
            </el-icon>

            <el-breadcrumb separator="/" class="hidden-mobile">
              <el-breadcrumb-item :to="{ path: homePath }">{{ homeTitle }}</el-breadcrumb-item>
              <el-breadcrumb-item>{{ currentRouteTitle }}</el-breadcrumb-item>
            </el-breadcrumb>
            <span class="page-title-mobile visible-mobile-only">{{ currentRouteTitle }}</span>
          </div>

          <div class="header-right">
            <!-- 头部右侧插槽（套餐标签、余额等） -->
            <slot name="header-extra" />

            <!-- v0.5.0 多主题切换器 -->
            <ThemeSwitcher />

            <!-- v0.5.0 语言切换器 -->
            <LanguageSwitcher />

            <el-dropdown @command="handleCommand">
              <span class="user-info">
                <el-avatar :size="28" icon="UserFilled" />
                <span class="username hidden-mobile">{{ auth.username || t('layout.user') }}</span>
              </span>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="profile">{{ t('layout.profile') }}</el-dropdown-item>
                  <el-dropdown-item command="logout" divided>{{ t('layout.logout') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </header>

        <!-- 内容区 -->
        <main class="layout-main">
          <router-view v-slot="{ Component }">
            <transition name="fade" mode="out-in">
              <component :is="Component" />
            </transition>
          </router-view>
        </main>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { Menu, Fold, Expand } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useSysConfigStore } from '@/stores/sysConfig'
import PlatformNoticeBanner from '@/components/PlatformNoticeBanner.vue'

interface MenuItem {
  path: string
  title: string
  icon: string
}

const props = defineProps<{
  /** 路由前缀，如 /admin /tenant /agent */
  routePrefix: string
  /** Logo 文字 */
  logoText?: string
  /** 首页路径（面包屑用） */
  homePath: string
  /** 首页标题（面包屑用） */
  homeTitle: string
  /** 自定义菜单（不传则自动从路由表读取） */
  customMenus?: MenuItem[]
  /** 退出登录回调（不传则使用默认行为） */
  onLogout?: () => Promise<void>
}>()

const { t, te } = useI18n()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sysConfig = useSysConfigStore()

const collapsed = ref(false)
const drawerVisible = ref(false)
const isMobile = ref(false)

const defaultLogo = props.logoText || 'KeyAuth SaaS'

/**
 * v0.5.0 国际化：路由标题翻译辅助
 * - 若 meta.titleKey 存在，用 i18n 翻译
 * - 否则回退到 meta.title（向后兼容）
 */
const translateRouteTitle = (meta: any): string => {
  if (meta?.titleKey && te(meta.titleKey)) {
    return t(meta.titleKey)
  }
  return (meta?.title as string) || ''
}

const menus = computed<MenuItem[]>(() => {
  if (props.customMenus?.length) return props.customMenus
  const target = router.getRoutes().find(r => r.path === props.routePrefix)
  if (!target?.children) return []
  return target.children
    .filter(child => !child.meta?.public)
    .map(child => ({
      path: `${props.routePrefix}/${child.path}`,
      title: translateRouteTitle(child.meta),
      icon: (child.meta?.icon as string) || 'Menu'
    }))
})

// 当前路由标题（响应式：i18n locale 变化时自动更新）
const currentRouteTitle = computed(() => translateRouteTitle(route.meta))

const handleCommand = async (cmd: string) => {
  if (cmd === 'logout') {
    try {
      await ElMessageBox.confirm(t('layout.confirmLogout'), t('common.tip'), { type: 'warning' })
    } catch {
      return
    }
    if (props.onLogout) {
      await props.onLogout()
    } else {
      // 默认行为：尝试调用后端登出（失败也强制清除本地态）
      try {
        const { logoutApi } = await import('@/api/auth')
        await logoutApi(props.routePrefix.slice(1) as 'admin' | 'tenant' | 'agent')
      } catch { /* 静默 */ }
      auth.logout()
      router.push('/login')
    }
  } else if (cmd === 'profile') {
    router.push(`${props.routePrefix}/profile`)
  }
}

const onMenuSelect = () => {
  drawerVisible.value = false
}

// 响应式：监听窗口尺寸
const checkMobile = () => {
  isMobile.value = window.innerWidth < 768
  if (!isMobile.value) drawerVisible.value = false
}

const defaultLogoText = computed(() => sysConfig.platformName || defaultLogo)

// 路由标题国际化：动态更新 document.title
watch([currentRouteTitle, () => route.meta.title], () => {
  const title = currentRouteTitle.value || (route.meta.title as string) || ''
  document.title = `${title} - KeyAuth SaaS`
}, { immediate: true })

onMounted(async () => {
  await sysConfig.load()
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})

// 路由切换时关闭抽屉
watch(() => route.path, () => {
  if (isMobile.value) drawerVisible.value = false
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.basic-layout {
  height: 100vh;
  display: flex;
  flex-direction: column;
  background: $color-bg-page;
}

.layout-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

// ============== 桌面侧边栏 ==============
.layout-aside {
  width: $layout-sidebar-width;
  background: $color-bg-sidebar;
  border-right: 1px solid $color-border-light;
  transition: width 0.2s;
  overflow: hidden;
  flex-shrink: 0;

  &.collapsed {
    width: $layout-sidebar-collapsed-width;
  }

  .logo {
    height: $layout-header-height;
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    padding: 0 $spacing-md;
    border-bottom: 1px solid $color-border-lighter;
    img { width: 28px; height: 28px; }
    .logo-text {
      font-size: 15px;
      font-weight: 600;
      color: $color-text-primary;
      white-space: nowrap;
    }
  }

  :deep(.aside-menu) {
    border-right: none;
    background: $color-bg-sidebar;

    .el-menu-item {
      color: $color-text-regular;
      &:hover {
        background: $color-bg-hover;
        color: $color-primary;
      }
      &.is-active {
        background: $color-bg-sidebar-active;
        color: $color-primary;
        font-weight: 500;
      }
    }
  }
}

// ============== 移动端抽屉 ==============
.drawer-logo {
  height: $layout-header-height;
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  padding: 0 $spacing-md;
  border-bottom: 1px solid $color-border-lighter;
  img { width: 28px; height: 28px; }
  .logo-text {
    font-size: 16px;
    font-weight: 600;
    color: $color-text-primary;
  }
}
:deep(.drawer-menu) {
  border-right: none;
  .el-menu-item {
    height: 48px;
    line-height: 48px;
    &.is-active {
      background: $color-bg-sidebar-active;
      color: $color-primary;
    }
  }
}

// ============== 主体区域 ==============
.layout-main-wrap {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.layout-header {
  background: $color-bg-header;
  border-bottom: 1px solid $color-border-light;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 $spacing-md;
  height: $layout-header-height;
  flex-shrink: 0;

  .header-left {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    .collapse-btn, .menu-btn {
      font-size: 20px;
      cursor: pointer;
      color: $color-text-regular;
      &:hover { color: $color-primary; }
    }
    .page-title-mobile {
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    .user-info {
      display: flex;
      align-items: center;
      gap: $spacing-sm;
      cursor: pointer;
      .username {
        font-size: 14px;
        color: $color-text-primary;
      }
    }
  }

  @include mobile {
    padding: 0 $spacing-sm;
    .header-left { gap: $spacing-sm; }
  }
}

.layout-main {
  flex: 1;
  background: $color-bg-page;
  padding: $spacing-md;
  overflow-y: auto;

  @include mobile {
    padding: $spacing-sm;
  }
}

// ============== 路由切换动画 ==============
.fade-enter-active, .fade-leave-active {
  transition: opacity 0.15s;
}
.fade-enter-from, .fade-leave-to {
  opacity: 0;
}
</style>
