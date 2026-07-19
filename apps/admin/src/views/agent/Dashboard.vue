<!--
  代理工作台（响应式 H5）
  - 顶部数据卡：余额 / 累计佣金 / 已购卡密 / 待提现
  - 快捷入口：购卡 / 订单 / 佣金
  - 最近订单列表
  铁律 06 待核实：后端 /agent/dashboard 当前为 501 占位（v0.3.0 交付），调用失败时显示 0/空。
-->
<template>
  <div class="agent-dashboard">
    <PageHeader title="工作台" subtitle="代理账户余额、佣金与购卡一览">
      <template #actions>
        <el-button type="primary" @click="$router.push('/agent/cards')">立即购卡</el-button>
        <el-button @click="$router.push('/agent/commission')">申请提现</el-button>
      </template>
    </PageHeader>

    <!-- 数据卡 -->
    <div class="stat-grid">
      <div class="stat-card balance">
        <div class="stat-label">账户余额</div>
        <div class="stat-value">¥{{ stats.balance.toFixed(2) }}</div>
        <div class="stat-extra">冻结 ¥{{ stats.frozen_balance.toFixed(2) }}</div>
      </div>
      <div class="stat-card commission">
        <div class="stat-label">累计佣金</div>
        <div class="stat-value">¥{{ stats.total_commission.toFixed(2) }}</div>
        <div class="stat-extra">已提现 ¥{{ stats.total_withdraw.toFixed(2) }}</div>
      </div>
      <div class="stat-card purchased">
        <div class="stat-label">累计购卡</div>
        <div class="stat-value">{{ stats.total_purchased }}</div>
        <div class="stat-extra">今日 {{ stats.today_purchased }} 张</div>
      </div>
      <div class="stat-card spent">
        <div class="stat-label">累计消费</div>
        <div class="stat-value">¥{{ stats.total_spent.toFixed(2) }}</div>
        <div class="stat-extra">今日 ¥{{ stats.today_spent.toFixed(2) }}</div>
      </div>
    </div>

    <!-- 快捷入口 -->
    <div class="quick-entry">
      <div class="entry-item" @click="$router.push('/agent/cards')">
        <el-icon><Key /></el-icon>
        <span>购卡</span>
      </div>
      <div class="entry-item" @click="$router.push('/agent/orders')">
        <el-icon><List /></el-icon>
        <span>订单</span>
      </div>
      <div class="entry-item" @click="$router.push('/agent/commission')">
        <el-icon><GoldMedal /></el-icon>
        <span>佣金</span>
      </div>
      <div class="entry-item" @click="$router.push('/agent/balance')">
        <el-icon><Wallet /></el-icon>
        <span>提现</span>
      </div>
    </div>

    <!-- 最近订单 -->
    <div class="app-card">
      <div class="card-header">
        <h3>最近订单</h3>
        <el-button link type="primary" @click="$router.push('/agent/orders')">查看全部</el-button>
      </div>
      <EmptyState v-if="!recentOrders.length && !loading" description="暂无订单记录" />
      <ResponsiveTable
        v-else
        :data="recentOrders"
        :loading="loading"
        :total="recentOrders.length"
        :show-pagination="false"
        :mobile-fields="orderMobileFields"
      >
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column prop="card_type_name" label="卡类" min-width="140" />
        <el-table-column prop="quantity" label="数量" width="80" />
        <el-table-column prop="total_amount" label="金额" width="120">
          <template #default="scope">¥{{ Number(scope.row.total_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="commission_amount" label="佣金" width="120">
          <template #default="scope">¥{{ Number(scope.row.commission_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="pay_status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.pay_status)" size="small">{{ statusText(scope.row.pay_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="下单时间" width="170">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
      </ResponsiveTable>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Key, List, GoldMedal, Wallet } from '@element-plus/icons-vue'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import EmptyState from '@/components/EmptyState.vue'
import { agentDashboardApi, type AgentOrder, type AgentDashboard as DashboardData } from '@/api/agent'

const stats = ref<DashboardData>({
  balance: 0,
  frozen_balance: 0,
  today_purchased: 0,
  today_spent: 0,
  total_purchased: 0,
  total_spent: 0,
  total_commission: 0,
  total_withdraw: 0,
  pending_withdraw: 0
})
const recentOrders = ref<AgentOrder[]>([])
const loading = ref(false)

const orderMobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'card_type_name', label: '卡类' },
  { prop: 'quantity', label: '数量' },
  { prop: 'total_amount', label: '金额', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'pay_status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const statusTag = (s: string): any => ({
  paid: 'success',
  pending: 'warning',
  closed: 'info',
  refunded: 'danger'
}[s] || 'info')

const statusText = (s: string) => ({
  paid: '已支付',
  pending: '待支付',
  closed: '已关闭',
  refunded: '已退款'
}[s] || s)

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadDashboard = async () => {
  loading.value = true
  try {
    const data = await agentDashboardApi()
    if (data && typeof data === 'object') {
      Object.assign(stats.value, data)
      recentOrders.value = data.recent_orders || []
    }
  } catch {
    // 铁律 06 待核实：后端接口未实现，保持 0 值显示（铁律 04 不编造数据）
  } finally {
    loading.value = false
  }
}

onMounted(loadDashboard)
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.stat-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: $spacing-md;
  margin-bottom: $spacing-lg;

  @include tablet {
    grid-template-columns: repeat(2, 1fr);
  }
  @include mobile {
    grid-template-columns: repeat(2, 1fr);
    gap: $spacing-sm;
  }
}

.stat-card {
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-md $spacing-lg;
  box-shadow: $shadow-card;
  border-left: 4px solid $color-primary;

  .stat-label {
    font-size: 13px;
    color: $color-text-secondary;
  }
  .stat-value {
    font-size: 24px;
    font-weight: 600;
    color: $color-text-primary;
    margin: 6px 0 4px;
    font-family: 'SF Mono', 'Menlo', monospace;
  }
  .stat-extra {
    font-size: 12px;
    color: $color-text-secondary;
  }

  &.balance { border-left-color: $color-primary; }
  &.commission { border-left-color: $color-success; }
  &.purchased { border-left-color: $color-warning; }
  &.spent { border-left-color: $color-danger; }

  @include mobile {
    padding: $spacing-sm $spacing-md;
    .stat-value { font-size: 18px; }
  }
}

.quick-entry {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: $spacing-md;
  margin-bottom: $spacing-lg;
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-lg;
  box-shadow: $shadow-card;

  @include mobile {
    grid-template-columns: repeat(4, 1fr);
    padding: $spacing-md;
    gap: $spacing-sm;
  }

  .entry-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: $spacing-sm;
    cursor: pointer;
    padding: $spacing-sm;
    border-radius: $radius-md;
    transition: background 0.2s;

    &:hover {
      background: $color-bg-hover;
    }
    .el-icon {
      font-size: 28px;
      color: $color-primary;
    }
    span {
      font-size: 13px;
      color: $color-text-regular;
    }

    @include mobile {
      .el-icon { font-size: 22px; }
      span { font-size: 12px; }
    }
  }
}

.app-card {
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-md;
  box-shadow: $shadow-card;

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-md;
    h3 {
      margin: 0;
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }
}
</style>
