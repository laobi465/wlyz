<!--
  H5 卡密查询页
  - 输入卡密 + 应用 AppKey
  - 调用客户端 API 验证（不绑定设备）
  - 显示卡密状态、过期时间、剩余次数等
-->
<template>
  <div class="h5-query">
    <div class="query-card">
      <p class="section-label">应用 AppKey</p>
      <el-input v-model="appKey" placeholder="请输入 AppKey" />

      <p class="section-label">卡密</p>
      <el-input v-model="cardKey" placeholder="请输入卡密" clearable />

      <div class="submit-row">
        <el-button type="primary" :loading="loading" @click="doQuery">查询卡密</el-button>
      </div>
    </div>

    <div v-if="result" class="result-card">
      <p class="section-label">卡密信息</p>
      <div class="info-row">
        <span class="label">状态</span>
        <el-tag :type="statusTagType(result.status)" size="small">{{ statusText(result.status) }}</el-tag>
      </div>
      <div class="info-row">
        <span class="label">类型</span>
        <span class="value">{{ result.type }}</span>
      </div>
      <div v-if="result.expires_at" class="info-row">
        <span class="label">到期时间</span>
        <span class="value">{{ formatTime(result.expires_at) }}</span>
      </div>
      <div class="info-row">
        <span class="label">剩余时长</span>
        <span class="value">{{ formatRemaining(result.remaining_seconds) }}</span>
      </div>
      <div class="info-row">
        <span class="label">已绑定设备</span>
        <span class="value">{{ result.bound_devices }} / {{ result.max_devices }}</span>
      </div>
      <div class="info-row">
        <span class="label">使用次数</span>
        <span class="value">{{ result.used_count }} / {{ result.max_uses }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { request } from '@/api/http'

const appKey = ref('')
const cardKey = ref('')
const loading = ref(false)
const result = ref<any>(null)

const doQuery = async () => {
  if (!appKey.value || !cardKey.value) {
    ElMessage.warning('请填写 AppKey 和卡密')
    return
  }
  loading.value = true
  result.value = null
  try {
    // 注：调用客户端验证接口需要 HMAC 签名
    // 待核实：当前 http.ts 未实现客户端签名，此接口在 H5 端直接调用会失败
    // 建议：后端提供一个 public/card_info?app_key=xxx&card_key=yyy 的只读查询接口
    // 当前先调用 /client/verify 接口（待签名实现后可用）
    const resp = await request.post('/client/verify', {
      card_key: cardKey.value,
      hwid: 'h5-query-' + Date.now()
    }, {
      headers: {
        'X-App-Key': appKey.value
      }
    })
    result.value = resp.card || resp
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const statusTagType = (s: string): any => {
  const map: Record<string, any> = {
    unused: 'info',
    active: 'success',
    expired: 'warning',
    banned: 'danger',
    disabled: 'info'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    unused: '未使用',
    active: '正常',
    expired: '已过期',
    banned: '已封禁',
    disabled: '已禁用'
  }
  return map[s] || s
}

const formatTime = (ts: number) => {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

const formatRemaining = (seconds: number) => {
  if (seconds === -1) return '永久'
  if (seconds <= 0) return '已过期'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  if (d > 0) return `${d} 天 ${h} 小时`
  if (h > 0) return `${h} 小时 ${m} 分钟`
  return `${m} 分钟`
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-query {
  max-width: 640px;
  margin: 0 auto;
}

.section-label {
  font-size: 13px;
  color: $color-text-secondary;
  margin: $spacing-md 0 $spacing-sm;
}

.query-card, .result-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;
}

.submit-row {
  margin-top: $spacing-md;
  :deep(.el-button) { width: 100%; }
}

.info-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: $spacing-sm 0;
  border-bottom: 1px solid $color-border-lighter;

  &:last-child { border-bottom: none; }
  .label {
    font-size: 13px;
    color: $color-text-secondary;
  }
  .value {
    font-size: 13px;
    color: $color-text-primary;
    font-weight: 500;
    text-align: right;
    max-width: 60%;
    word-break: break-all;
  }
}
</style>
