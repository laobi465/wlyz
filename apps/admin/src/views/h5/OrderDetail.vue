<!--
  H5 订单详情（v0.4.x 残留项 1：U-11）
  - 展示订单基本信息
  - 已支付订单展示卡密列表（支持复制）
  - 待支付订单提供「前往支付」入口
-->
<template>
  <div class="h5-order-detail">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">订单详情</span>
    </div>

    <div v-loading="loading">
      <template v-if="order">
        <!-- 状态卡 -->
        <div class="status-card" :class="order.pay_status">
          <el-icon v-if="order.pay_status === 'paid'" class="status-icon"><CircleCheck /></el-icon>
          <el-icon v-else-if="order.pay_status === 'pending'" class="status-icon"><Loading /></el-icon>
          <el-icon v-else class="status-icon"><CircleClose /></el-icon>
          <h2>{{ statusText(order.pay_status) }}</h2>
          <p class="order-no">订单号：{{ order.order_no }}</p>
        </div>

        <!-- 订单信息 -->
        <div class="info-card">
          <p class="section-label">订单信息</p>
          <div class="info-row">
            <span class="label">支付通道</span>
            <span class="value">{{ order.pay_channel || '-' }}</span>
          </div>
          <div class="info-row">
            <span class="label">购买数量</span>
            <span class="value">{{ order.quantity }}</span>
          </div>
          <div class="info-row">
            <span class="label">单价</span>
            <span class="value">¥{{ order.unit_price.toFixed(2) }}</span>
          </div>
          <div class="info-row">
            <span class="label">合计金额</span>
            <span class="value amount">¥{{ order.total_amount.toFixed(2) }}</span>
          </div>
          <div v-if="order.pay_trade_no" class="info-row">
            <span class="label">交易号</span>
            <span class="value">{{ order.pay_trade_no }}</span>
          </div>
          <div v-if="order.paid_at" class="info-row">
            <span class="label">支付时间</span>
            <span class="value">{{ formatTime(order.paid_at) }}</span>
          </div>
          <div class="info-row">
            <span class="label">下单时间</span>
            <span class="value">{{ formatTime(order.created_at) }}</span>
          </div>
          <div v-if="order.buyer_contact" class="info-row">
            <span class="label">联系方式</span>
            <span class="value">{{ order.buyer_contact }}</span>
          </div>
        </div>

        <!-- 卡密列表（已支付时展示） -->
        <div v-if="order.pay_status === 'paid' && order.cards.length" class="cards-card">
          <p class="section-label">卡密列表（{{ order.cards.length }} 张）</p>
          <div v-for="card in order.cards" :key="card.id" class="card-item">
            <div class="card-head">
              <span class="card-key">{{ card.card_key }}</span>
              <el-tag :type="cardStatusTagType(card.status)" size="small">{{ cardStatusText(card.status) }}</el-tag>
            </div>
            <div class="card-meta">
              <span v-if="card.duration_seconds > 0" class="meta-text">时长：{{ formatDuration(card.duration_seconds) }}</span>
              <span v-else-if="card.duration_seconds === -1" class="meta-text">永久</span>
              <span class="meta-text">使用：{{ card.used_count }} / {{ card.max_uses }}</span>
            </div>
            <div v-if="card.expires_at" class="card-meta">
              <span class="meta-text">到期：{{ formatTime(card.expires_at) }}</span>
            </div>
            <div class="card-actions">
              <el-button text size="small" @click="copyKey(card.card_key)">复制卡密</el-button>
            </div>
          </div>
        </div>

        <!-- 操作按钮 -->
        <div class="actions-row">
          <el-button v-if="order.pay_status === 'pending'" type="primary" @click="goPay">前往支付</el-button>
          <el-button v-if="order.pay_status === 'paid'" type="primary" plain @click="copyAll">复制全部卡密</el-button>
          <el-button @click="goOrders">返回订单列表</el-button>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, CircleCheck, CircleClose, Loading } from '@element-plus/icons-vue'
import { endUserGetOrderApi, type EndUserOrderDetail } from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const route = useRoute()
const router = useRouter()
const endUserStore = useEndUserStore()

const orderNo = computed(() => route.params.orderNo as string)
const order = ref<EndUserOrderDetail | null>(null)
const loading = ref(false)

const load = async () => {
  loading.value = true
  try {
    const resp = await endUserGetOrderApi(orderNo.value)
    order.value = resp
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const copyKey = (key: string) => {
  navigator.clipboard.writeText(key).then(() => {
    ElMessage.success('已复制卡密')
  }).catch(() => {
    ElMessage.error('复制失败，请手动长按复制')
  })
}

const copyAll = () => {
  if (!order.value?.card_keys?.length) return
  const text = order.value.card_keys.join('\n')
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(`已复制 ${order.value!.card_keys.length} 张卡密`)
  }).catch(() => {
    ElMessage.error('复制失败，请手动复制')
  })
}

const goPay = () => {
  // 跳到支付结果页轮询状态（用户可重新发起支付在 H5 首页）
  ElMessage.info('如需重新支付，请前往购卡页重新下单')
  router.push('/h5')
}

const goOrders = () => router.push('/h5/orders')

const statusText = (s: string) => {
  const map: Record<string, string> = {
    pending: '等待支付',
    paid: '支付成功',
    closed: '订单已关闭',
    refunded: '已退款'
  }
  return map[s] || s
}

const cardStatusTagType = (s: string): any => {
  const map: Record<string, any> = {
    unused: 'info',
    active: 'success',
    expired: 'warning',
    banned: 'danger',
    disabled: 'info'
  }
  return map[s] || 'info'
}

const cardStatusText = (s: string) => {
  const map: Record<string, string> = {
    unused: '未使用',
    active: '正常',
    expired: '已过期',
    banned: '已封禁',
    disabled: '已禁用'
  }
  return map[s] || s
}

const formatTime = (t: string | null) => {
  if (!t) return '-'
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return t
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch {
    return t
  }
}

const formatDuration = (seconds: number) => {
  if (seconds >= 86400) return `${Math.floor(seconds / 86400)} 天`
  if (seconds >= 3600) return `${Math.floor(seconds / 3600)} 小时`
  return `${seconds} 秒`
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/orders')
  }
}

onMounted(() => {
  endUserStore.restore()
  if (!endUserStore.isLoggedIn) {
    router.replace('/h5/login')
    return
  }
  load()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-order-detail {
  max-width: 640px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  align-items: center;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-md;
  background: #fff;
  border-radius: $radius-md;
  position: relative;

  .back-btn {
    padding: 0 $spacing-sm;
  }
  .title {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    font-size: 16px;
    font-weight: 600;
    color: $color-text-primary;
  }
}

.status-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
  text-align: center;
  margin-bottom: $spacing-md;

  .status-icon {
    font-size: 56px;
    margin-bottom: $spacing-md;
  }
  &.paid .status-icon { color: $color-success; }
  &.pending .status-icon { color: $color-warning; }
  &.closed .status-icon,
  &.refunded .status-icon { color: $color-danger; }

  h2 {
    font-size: 18px;
    font-weight: 600;
    color: $color-text-primary;
    margin: 0 0 $spacing-sm;
  }
  .order-no {
    font-size: 12px;
    color: $color-text-secondary;
    margin: 0;
    word-break: break-all;
  }
}

.info-card,
.cards-card {
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
    &.amount {
      font-size: 16px;
      font-weight: 700;
      color: $color-danger;
    }
  }
}

.card-item {
  border: 1px solid $color-border-lighter;
  border-radius: $radius-sm;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-sm;
  background: $color-primary-light;

  &:last-child { margin-bottom: 0; }

  .card-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-xs;

    .card-key {
      font-family: monospace;
      font-size: 14px;
      font-weight: 600;
      color: $color-text-primary;
      word-break: break-all;
      flex: 1;
      margin-right: $spacing-sm;
    }
  }

  .card-meta {
    display: flex;
    gap: $spacing-md;
    flex-wrap: wrap;
    .meta-text {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }

  .card-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: $spacing-xs;
  }
}

.actions-row {
  display: flex;
  flex-direction: column;
  gap: $spacing-sm;
  margin-top: $spacing-md;

  :deep(.el-button) {
    width: 100%;
  }
}
</style>
