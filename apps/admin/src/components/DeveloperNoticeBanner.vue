<template>
  <div class="dev-notice-banner">
    <span class="banner-label">开发者公告</span>
    <span class="banner-content">{{ notice.title }}</span>
    <el-icon class="banner-more" @click="dialogVisible = true">
      <InfoFilled />
    </el-icon>
  </div>

  <el-dialog v-model="dialogVisible" :title="notice.title" width="600px">
    <div v-html="notice.content"></div>
    <template #footer>
      <el-button type="primary" @click="dialogVisible = false">已读</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'

defineProps<{
  notice: { title: string; content: string }
}>()

const dialogVisible = ref(false)
</script>

<style scoped lang="scss">
.dev-notice-banner {
  background: #e6f4ff;
  color: #0958d9;
  padding: 8px 16px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  font-weight: 500;
  .banner-label {
    background: #0958d9;
    color: #fff;
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 12px;
  }
  .banner-content {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .banner-more { cursor: pointer; &:hover { opacity: 0.7; } }
}
</style>
