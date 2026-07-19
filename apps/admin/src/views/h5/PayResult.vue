<!--
  H5 支付结果页
  - 从 URL 参数读取订单号
  - 轮询订单状态
  - 支付成功显示卡密列表
  - 失败/超时显示对应提示
-->
<template>
  <div class="pay-result">
    <div class="status-card">
      <el-icon v-if="status === 'paid'" class="status-icon success"><CircleCheck /></el-icon>
      <el-icon v-else-if="status === 'pending'" class="status-icon pending"><Loading /></el-icon>
      <el-icon v-else class="status-icon fail"><CircleClose /></el-icon>

      <h2>{{ statusText }}</h2>
      <p class="order-no">订单号：{{ orderNo }}</p>

      <div v-if="status === 'pending'" class="pending-tip">
        <p>支付完成后将自动刷新</p>
        <el-button text @click="manualRefresh">手动刷新</el-button>
      </div>

      <div v-if="status === 'paid' && cards.length" class="cards-list">
        <p class="cards-label">支付成功！您的卡密如下：</p>
        <div v-for="(card, idx) in cards" :key="idx" class="card-key-row">
          <span class="card-key">{{ card }}</span>
          <el-button text size="small" @click="copyKey(card)">复制</el-button>
        </div>
        <el-button type="primary" class="query-btn" @click="goQuery">前往查询卡密</el-button>
      </div>

      <div v-if="status === 'closed'" class="closed-tip">
        <p>订单已关闭，请重新下单</p>
        <el-button type="primary" @click="goHome">重新购卡</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { CircleCheck, CircleClose, Loading } from '@element-plus/icons-vue'
import { getPayOrderApi, type PayStatus } from '@/api/pay'

const route = useRoute()
const router = useRouter()

const orderNo = computed(() => route.params.orderNo as string)
const status = ref<PayStatus | 'loading'>('loading')
const cards = ref<string[]>([])
let pollTimer: ReturnType<typeof setTimeout> | null = null
let pollCount = 0

const statusText = computed(() => {
  const map: Record<string, string> = {
    paid: '支付成功',
    pending: '等待支付',
    closed: '订单已关闭',
    refunded: '已退款',
    loading: '加载中'
  }
  return map[status.value] || '未知状态'
})

const fetchOrder = async () => {
  try {
    const resp = await getPayOrderApi(orderNo.value)
    status.value = resp.pay_status
    // v0.3.5：后端在订单已支付时直接返回 card_keys 明文数组
    if (resp.pay_status === 'paid' && resp.card_keys?.length) {
      cards.value = resp.card_keys
    }
    if (resp.pay_status === 'pending' && pollCount < 30) {
      // 每 3 秒轮询一次，最多 30 次（90 秒）
      pollTimer = setTimeout(fetchOrder, 3000)
      pollCount++
    }
  } catch {
    // 加载失败不重试
  }
}

const manualRefresh = () => {
  pollCount = 0
  fetchOrder()
}

const copyKey = (key: string) => {
  navigator.clipboard.writeText(key).then(() => {
    ElMessage.success('已复制到剪贴板')
  }).catch(() => {
    ElMessage.error('复制失败，请手动长按复制')
  })
}

const goQuery = () => router.push('/h5/query')
const goHome = () => router.push('/h5')

onMounted(() => {
  fetchOrder()
})
onBeforeUnmount(() => {
  if (pollTimer) clearTimeout(pollTimer)
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.pay-result {
  max-width: 640px;
  margin: 0 auto;
}

.status-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
  text-align: center;
}

.status-icon {
  font-size: 56px;
  margin-bottom: $spacing-md;

  &.success { color: $color-success; }
  &.pending { color: $color-warning; }
  &.fail { color: $color-danger; }
}

h2 {
  font-size: 18px;
  font-weight: 600;
  color: $color-text-primary;
  margin: 0 0 $spacing-sm;
}

.order-no {
  font-size: 12px;
  color: $color-text-secondary;
  margin: 0 0 $spacing-md;
  word-break: break-all;
}

.pending-tip {
  p {
    font-size: 13px;
    color: $color-text-secondary;
    margin: 0 0 $spacing-sm;
  }
}

.cards-list {
  margin-top: $spacing-lg;
  text-align: left;

  .cards-label {
    font-size: 13px;
    color: $color-text-regular;
    margin: 0 0 $spacing-sm;
    text-align: center;
  }

  .card-key-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: $color-primary-light;
    border-radius: $radius-sm;
    padding: $spacing-sm $spacing-md;
    margin-bottom: $spacing-sm;

    .card-key {
      font-family: monospace;
      font-size: 14px;
      color: $color-text-primary;
      font-weight: 600;
      word-break: break-all;
      flex: 1;
      margin-right: $spacing-sm;
    }
  }

  .query-btn {
    width: 100%;
    margin-top: $spacing-md;
  }
}

.closed-tip {
  margin-top: $spacing-lg;
  p {
    font-size: 13px;
    color: $color-text-secondary;
    margin: 0 0 $spacing-md;
  }
}
</style>
