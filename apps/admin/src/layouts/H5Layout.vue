<!--
  H5Layout 终端用户 H5 布局
  - 移动优先设计
  - 顶部 Logo + 底部导航
  - 不需要登录态
-->
<template>
  <div class="h5-layout">
    <header class="h5-header">
      <div class="brand" @click="router.push('/h5')">
        <img src="@/assets/logo.svg" alt="logo" />
        <span>{{ sysConfig.platformName || 'KeyAuth' }}</span>
      </div>
    </header>

    <main class="h5-main">
      <router-view v-slot="{ Component }">
        <transition name="slide" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>

    <nav class="h5-tabbar">
      <router-link to="/h5" class="tab-item" :class="{ active: route.path === '/h5' }">
        <el-icon><HomeFilled /></el-icon>
        <span>购卡</span>
      </router-link>
      <router-link to="/h5/query" class="tab-item" :class="{ active: route.path.startsWith('/h5/query') }">
        <el-icon><Search /></el-icon>
        <span>查卡</span>
      </router-link>
    </nav>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { HomeFilled, Search } from '@element-plus/icons-vue'
import { useSysConfigStore } from '@/stores/sysConfig'

const route = useRoute()
const router = useRouter()
const sysConfig = useSysConfigStore()

onMounted(() => {
  sysConfig.load()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: $color-bg-page;
  max-width: 768px;
  margin: 0 auto;
  position: relative;
}

.h5-header {
  background: #fff;
  border-bottom: 1px solid $color-border-lighter;
  height: 48px;
  display: flex;
  align-items: center;
  padding: 0 $spacing-md;
  position: sticky;
  top: 0;
  z-index: 10;

  .brand {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    cursor: pointer;
    img { width: 24px; height: 24px; }
    span {
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }
}

.h5-main {
  flex: 1;
  padding: $spacing-md;
  padding-bottom: 72px; // 给 tabbar 留位置
  overflow-y: auto;
}

.h5-tabbar {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  max-width: 768px;
  margin: 0 auto;
  background: #fff;
  border-top: 1px solid $color-border-lighter;
  display: flex;
  height: 56px;
  z-index: 20;

  .tab-item {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    color: $color-text-secondary;
    text-decoration: none;
    font-size: 12px;
    gap: 2px;

    .el-icon { font-size: 22px; }

    &.active {
      color: $color-primary;
    }
  }
}

.slide-enter-active, .slide-leave-active {
  transition: all 0.2s;
}
.slide-enter-from { opacity: 0; transform: translateX(10px); }
.slide-leave-to { opacity: 0; transform: translateX(-10px); }

// 桌面端访问 H5 时也以移动端样式呈现
@media (min-width: $bp-mobile) {
  .h5-layout {
    border-left: 1px solid $color-border-lighter;
    border-right: 1px solid $color-border-lighter;
    box-shadow: 0 0 16px rgba(0, 0, 0, 0.04);
    min-height: 100vh;
    margin-top: 0;
  }
}
</style>
