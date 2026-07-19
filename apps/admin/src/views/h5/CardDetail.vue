<!--
  H5 卡密详情页
  - 通过路由参数 cardKey 携带卡密明文（已下单后跳转）
  - 显示卡密详情 + 设备信息
-->
<template>
  <div class="card-detail">
    <div class="card-info">
      <p class="section-label">卡密</p>
      <div class="key-row">
        <span class="key-text">{{ cardKey || '未提供' }}</span>
        <el-button v-if="cardKey" text size="small" @click="copy">复制</el-button>
      </div>
    </div>
    <div class="empty">
      <el-empty description="请前往查询页输入卡密查看详情" :image-size="100">
        <el-button type="primary" @click="goQuery">前往查询</el-button>
      </el-empty>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()

const cardKey = computed(() => route.params.cardKey as string)

const copy = () => {
  navigator.clipboard.writeText(cardKey.value).then(() => {
    ElMessage.success('已复制')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

const goQuery = () => router.push('/h5/query')
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.card-detail { max-width: 640px; margin: 0 auto; }

.card-info {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;
}

.section-label {
  font-size: 13px;
  color: $color-text-secondary;
  margin: 0 0 $spacing-sm;
}

.key-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: $color-primary-light;
  border-radius: $radius-sm;
  padding: $spacing-sm $spacing-md;

  .key-text {
    font-family: monospace;
    font-size: 14px;
    color: $color-text-primary;
    font-weight: 600;
    word-break: break-all;
    flex: 1;
  }
}

.empty {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
  text-align: center;
}
</style>
