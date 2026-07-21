<template>
  <div class="platform-notice-banner" :style="{ background: sysConfig.noticeBannerBg, color: sysConfig.noticeBannerTextColor }">
    <span class="banner-label">平台公告</span>
    <span class="banner-content">{{ latestNotice?.title || '欢迎使用 ' + sysConfig.platformName }}</span>
    <el-icon v-if="latestNotice" class="banner-more" @click="showDetail">
      <InfoFilled />
    </el-icon>
  </div>

  <el-dialog v-model="dialogVisible" :title="latestNotice?.title" width="600px">
    <div v-html="sanitizedContent"></div>
    <template #footer>
      <el-button type="primary" @click="dialogVisible = false">已读</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import DOMPurify from 'dompurify'
import { useSysConfigStore } from '@/stores/sysConfig'
// import { listActiveNotices } from '@/api/notice'  // 待实现

const sysConfig = useSysConfigStore()
const latestNotice = ref<{ title: string; content: string } | null>(null)
const dialogVisible = ref(false)

// P1-01: 对后端返回的 HTML 内容做净化，防止 <script> 等 XSS 注入
const sanitizedContent = computed(() =>
  DOMPurify.sanitize(latestNotice.value?.content || '', { USE_PROFILES: { html: true } })
)

onMounted(async () => {
  // 待实现：加载最新平台公告
  // try {
  //   const notices = await listActiveNotices({ type: 'platform' })
  //   latestNotice.value = notices[0] || null
  // } catch (e) { /* 静默 */ }
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
    background: currentColor;
    color: #fff;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 12px;
    // 反色让文字可读
    &::selection { background: transparent; }
  }
  .banner-content {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .banner-more {
    cursor: pointer;
    &:hover { opacity: 0.7; }
  }
}
// 修正 label 内部颜色
.platform-notice-banner .banner-label {
  color: #fff !important;
  background: attr(color);
}
</style>
