<!--
  H5 我的订单（v0.4.x 残留项 1：U-11）
  - 状态 tab 筛选：全部 / 待支付 / 已支付 / 已关闭 / 已退款
  - 列表分页展示
  - 点击订单跳详情
-->
<template>
  <div class="h5-orders">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">我的订单</span>
    </div>

    <!-- 状态 tab -->
    <div class="status-tabs">
      <div
        v-for="t in statusTabs"
        :key="t.value"
        class="tab-item"
        :class="{ active: activeStatus === t.value }"
        @click="switchTab(t.value)"
      >
        {{ t.label }}
      </div>
    </div>

    <div v-loading="loading">
      <div v-if="orders.length === 0 && !loading" class="empty-card">
        <el-empty :description="emptyText" :image-size="80" />
      </div>

      <div
        v-for="o in orders"
        :key="o.id"
        class="order-item"
        @click="goDetail(o.order_no)"
      >
        <div class="order-head">
          <span class="order-no">{{ o.order_no }}</span>
          <el-tag :type="statusTagType(o.pay_status)" size="small">{{ statusText(o.pay_status) }}</el-tag>
        </div>
        <div class="order-meta">
          <span class="meta-text">数量：{{ o.quantity }}</span>
          <span class="meta-text">单价：¥{{ o.unit_price.toFixed(2) }}</span>
        </div>
        <div class="order-meta">
          <span class="meta-text">支付通道：{{ o.pay_channel || '-' }}</span>
          <span class="meta-text">下单：{{ formatTime(o.created_at) }}</span>
        </div>
        <div class="order-foot">
          <span class="amount">¥{{ o.total_amount.toFixed(2) }}</span>
          <el-button text size="small" type="primary" @click.stop="goDetail(o.order_no)">查看详情</el-button>
        </div>
      </div>
    </div>

    <div v-if="hasMore" class="load-more">
      <el-button :loading="loadingMore" @click="loadMore">加载更多</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import {
  endUserListOrdersApi,
  type EndUserOrder,
  type EndUserOrderStatus
} from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()

type StatusFilter = '' | EndUserOrderStatus

const statusTabs: { label: string; value: StatusFilter }[] = [
  { label: '全部', value: '' },
  { label: '待支付', value: 'pending' },
  { label: '已支付', value: 'paid' },
  { label: '已关闭', value: 'closed' },
  { label: '已退款', value: 'refunded' }
]

const activeStatus = ref<StatusFilter>('')
const orders = ref<EndUserOrder[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const loadingMore = ref(false)

const hasMore = computed(() => orders.value.length < total.value)
const emptyText = computed(() => {
  if (activeStatus.value === '') return '暂无订单'
  return `暂无${statusTabs.find((t) => t.value === activeStatus.value)?.label || ''}订单`
})

const loadFirst = async () => {
  loading.value = true
  page.value = 1
  try {
    const resp = await endUserListOrdersApi({
      page: page.value,
      page_size: pageSize,
      status: activeStatus.value
    })
    orders.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const loadMore = async () => {
  if (!hasMore.value) return
  loadingMore.value = true
  page.value++
  try {
    const resp = await endUserListOrdersApi({
      page: page.value,
      page_size: pageSize,
      status: activeStatus.value
    })
    orders.value.push(...(resp.list || []))
    total.value = resp.total || 0
  } catch {
    page.value--
  } finally {
    loadingMore.value = false
  }
}

const switchTab = (v: StatusFilter) => {
  if (activeStatus.value === v) return
  activeStatus.value = v
  loadFirst()
}

const goDetail = (orderNo: string) => {
  router.push(`/h5/orders/${encodeURIComponent(orderNo)}`)
}

const statusTagType = (s: string): any => {
  const map: Record<string, any> = {
    pending: 'warning',
    paid: 'success',
    closed: 'info',
    refunded: 'danger'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    pending: '待支付',
    paid: '已支付',
    closed: '已关闭',
    refunded: '已退款'
  }
  return map[s] || s
}

const formatTime = (t: string) => {
  if (!t) return '-'
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return t
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch {
    return t
  }
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/profile')
  }
}

onMounted(() => {
  endUserStore.restore()
  if (!endUserStore.isLoggedIn) {
    router.replace('/h5/login')
    return
  }
  loadFirst()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-orders {
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

.status-tabs {
  display: flex;
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xs;
  margin-bottom: $spacing-md;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;

  .tab-item {
    flex: 1;
    text-align: center;
    padding: $spacing-sm $spacing-xs;
    font-size: 13px;
    color: $color-text-secondary;
    cursor: pointer;
    border-radius: $radius-sm;
    transition: all 0.2s;
    white-space: nowrap;
    min-width: 64px;

    &.active {
      background: $color-primary;
      color: #fff;
    }
  }
}

.empty-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
}

.order-item {
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-sm;
  cursor: pointer;
  transition: all 0.2s;

  &:active {
    border-color: $color-primary;
    background: $color-primary-light;
  }

  .order-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-sm;

    .order-no {
      font-family: monospace;
      font-size: 13px;
      font-weight: 600;
      color: $color-text-primary;
      word-break: break-all;
      flex: 1;
      margin-right: $spacing-sm;
    }
  }

  .order-meta {
    display: flex;
    gap: $spacing-md;
    flex-wrap: wrap;
    margin-bottom: 4px;
    .meta-text {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }

  .order-foot {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: $spacing-sm;
    border-top: 1px solid $color-border-lighter;
    padding-top: $spacing-sm;

    .amount {
      font-size: 16px;
      font-weight: 700;
      color: $color-danger;
    }
  }
}

.load-more {
  text-align: center;
  margin-top: $spacing-md;
  :deep(.el-button) {
    width: 100%;
  }
}
</style>
