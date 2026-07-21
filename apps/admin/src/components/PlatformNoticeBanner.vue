<template>
  <div class="platform-notice-banner" :style="{ background: sysConfig.noticeBannerBg, color: sysConfig.noticeBannerTextColor }">
    <span class="banner-label">平台公告</span>
    <span class="banner-content">{{ latestNotice?.title || '欢迎使用 ' + sysConfig.platformName }}</span>
    <el-icon v-if="latestNotice" class="banner-more" @click="showDetail">
      <InfoFilled />
    </el-icon>
  </div>

  <el-dialog v-model="dialogVisible" :title="latestNotice?.title" :width="isMobile ? '92%' : '600px'">
    <div v-html="sanitizedContent"></div>
    <template #footer>
      <el-button type="primary" @click="dialogVisible = false">已读</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import DOMPurify from 'dompurify'
import { useSysConfigStore } from '@/stores/sysConfig'
// import { listActiveNotices } from '@/api/notice'  // 待实现

const sysConfig = useSysConfigStore()
const latestNotice = ref<{ title: string; content: string } | null>(null)
const dialogVisible = ref(false)

// v0.7.0 修复：isMobile 响应式检测（与 BasicLayout 一致）
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }

// P1-01: 对后端返回的 HTML 内容做净化，防止 <script> 等 XSS 注入
const sanitizedContent = computed(() =>
  DOMPurify.sanitize(latestNotice.value?.content || '', { USE_PROFILES: { html: true } })
)

onMounted(async () => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  // 待实现：加载最新平台公告
  // try {
  //   const notices = await listActiveNotices({ type: 'platform' })
  //   latestNotice.value = notices[0] || null
  // } catch (e) { /* 静默 */ }
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})

const showDetail = () => {
  dialogVisible.value = true
}
</script>

<style scoped lang="scss">
.platform-notice-banner {
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  font-weight: 500;
  .banner-label {
    // v0.7.0 修复：原 background: currentColor + color:#fff 导致白底白字不可见
    // 改用半透明白色背景 + 白色文字，确保在任何横幅背景色上都可读
    background: rgba(255, 255, 255, 0.25);
    color: #fff;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 12px;
    flex-shrink: 0;
    &::selection { background: transparent; }
  }
  .banner-content {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .banner-more {
    cursor: pointer;
    flex-shrink: 0;
    &:hover { opacity: 0.7; }
  }
}
</style>
