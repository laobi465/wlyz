<template>
  <div class="placeholder-view">
    <el-result icon="info" :title="title" :sub-title="subTitle">
      <template #extra>
        <el-tag type="warning">接口待实现</el-tag>
        <p class="hint">
          当前页面为前端骨架占位，对应后端 API（<code>{{ apiPath }}</code>）尚未实现。<br/>
          完成后端开发后此页面将自动接入真实数据。
        </p>
      </template>
    </el-result>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()

const title = computed(() => (route.meta.title as string) || '页面')
const subTitle = computed(() => `${title.value} - 功能开发中`)
const apiPath = computed(() => {
  const path = route.path
  // /admin/dashboard -> /api/v1/admin/dashboard
  return `/api/v1${path}`
})
</script>

<style scoped lang="scss">
.placeholder-view {
  background: #fff;
  border-radius: 8px;
  padding: 48px;
  text-align: center;
}
.hint {
  margin-top: 16px;
  font-size: 13px;
  color: #909399;
  line-height: 1.8;
  code {
    background: #f5f7fa;
    padding: 2px 6px;
    border-radius: 3px;
    color: #cf1322;
    font-family: 'Courier New', monospace;
  }
}
</style>
