<!--
  开发者工作台（响应式 H5）
  - 顶部核心指标：应用/卡密/设备/订单/收入/待结算/代理
  - 快捷入口：应用管理 / 生成卡密 / 订单查询 / 代理邀请
  - 收入趋势：近 7 日（待 v0.3.0）
  - 热门应用 Top5 + 最近订单
  铁律 06 待核实：后端 /tenant/dashboard 当前为 501 占位（v0.3.0 交付），调用失败时显示 0/空。
-->
<template>
  <div class="tenant-dashboard">
    <PageHeader title="工作台" subtitle="应用、卡密、订单、收入核心指标一览">
      <template #actions>
        <el-button type="primary" @click="$router.push('/tenant/card-types')">新增卡类</el-button>
        <el-button @click="loadDashboard">刷新</el-button>
      </template>
    </PageHeader>

    <!-- 核心指标 -->
    <div class="stat-grid">
      <div class="stat-card apps">
        <div class="stat-icon"><el-icon><Cellphone /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">应用总数</div>
          <div class="stat-value">{{ stats.app_total }}</div>
          <div class="stat-extra">活跃 {{ stats.app_active }}</div>
        </div>
      </div>
      <div class="stat-card cards">
        <div class="stat-icon"><el-icon><Key /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">卡密总数</div>
          <div class="stat-value">{{ stats.card_total }}</div>
          <div class="stat-extra">已用 {{ stats.card_used }} / 活跃 {{ stats.card_active }}</div>
        </div>
      </div>
      <div class="stat-card devices">
        <div class="stat-icon"><el-icon><Monitor /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">在线设备</div>
          <div class="stat-value">{{ stats.device_online }}</div>
          <div class="stat-extra">总计 {{ stats.device_total }}</div>
        </div>
      </div>
      <div class="stat-card orders">
        <div class="stat-icon"><el-icon><List /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">今日订单</div>
          <div class="stat-value">{{ stats.order_today }}</div>
          <div class="stat-extra">-</div>
        </div>
      </div>
      <div class="stat-card revenue-today">
        <div class="stat-icon"><el-icon><Money /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">今日收入</div>
          <div class="stat-value">¥{{ stats.revenue_today.toFixed(2) }}</div>
          <div class="stat-extra">本月 ¥{{ stats.revenue_month.toFixed(2) }}</div>
        </div>
      </div>
      <div class="stat-card settlement">
        <div class="stat-icon"><el-icon><Wallet /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">待结算</div>
          <div class="stat-value">{{ stats.settlement_pending }}</div>
          <div class="stat-extra">¥{{ stats.settlement_amount.toFixed(2) }}</div>
        </div>
      </div>
      <div class="stat-card agents">
        <div class="stat-icon"><el-icon><UserFilled /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">代理总数</div>
          <div class="stat-value">{{ stats.agent_total }}</div>
          <div class="stat-extra">-</div>
        </div>
      </div>
      <div class="stat-card quick-action" @click="$router.push('/tenant/invite-codes')">
        <div class="stat-icon"><el-icon><Promotion /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">代理邀请</div>
          <div class="stat-value-text">生成邀请码</div>
          <div class="stat-extra">扩展分销</div>
        </div>
      </div>
    </div>

    <!-- 快捷入口 -->
    <div class="quick-entry">
      <div class="entry-item" @click="$router.push('/tenant/apps')">
        <el-icon><Cellphone /></el-icon>
        <span>应用管理</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/cards')">
        <el-icon><Key /></el-icon>
        <span>卡密管理</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/orders')">
        <el-icon><List /></el-icon>
        <span>订单管理</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/devices')">
        <el-icon><Monitor /></el-icon>
        <span>设备管理</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/agents')">
        <el-icon><UserFilled /></el-icon>
        <span>代理管理</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/cloud-vars')">
        <el-icon><Coin /></el-icon>
        <span>云变量</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/versions')">
        <el-icon><Upload /></el-icon>
        <span>版本管理</span>
      </div>
      <div class="entry-item" @click="$router.push('/tenant/profile')">
        <el-icon><Setting /></el-icon>
        <span>账号设置</span>
      </div>
    </div>

    <!-- 收入趋势 + 热门应用 -->
    <div class="row-2col">
      <div class="app-card">
        <div class="card-header">
          <h3>收入趋势（近 7 日）</h3>
          <el-tag size="small" type="info">待 v0.3.0</el-tag>
        </div>
        <EmptyState v-if="!revenueTrend.length" description="暂无趋势数据" :image-size="80" />
        <div v-else class="trend-chart">
          <div
            v-for="point in revenueTrend"
            :key="point.date"
            class="trend-bar"
            :style="{ height: barHeight(point.amount) + '%' }"
          >
            <span class="bar-value">¥{{ point.amount.toFixed(0) }}</span>
            <span class="bar-date">{{ point.date.slice(5) }}</span>
          </div>
        </div>
      </div>

      <div class="app-card">
        <div class="card-header">
          <h3>热门应用 Top 5</h3>
          <el-tag size="small" type="info">待 v0.3.0</el-tag>
        </div>
        <EmptyState v-if="!topApps.length" description="暂无数据" :image-size="80" />
        <ul v-else class="top-apps">
          <li v-for="(app, idx) in topApps" :key="app.id" class="app-item">
            <span class="rank" :class="'rank-' + (idx + 1)">{{ idx + 1 }}</span>
            <span class="app-name">{{ app.name }}</span>
            <span class="app-card-count">{{ app.card_count }} 张</span>
            <span class="app-revenue">¥{{ Number(app.revenue).toFixed(2) }}</span>
          </li>
        </ul>
      </div>
    </div>

    <!-- 最近订单 -->
    <div class="app-card">
      <div class="card-header">
        <h3>最近订单</h3>
        <el-button link type="primary" @click="$router.push('/tenant/orders')">查看全部</el-button>
      </div>
      <EmptyState v-if="!recentOrders.length" description="暂无订单数据（待 v0.3.0）" :image-size="80" />
      <ResponsiveTable
        v-else
        :data="recentOrders"
        :show-pagination="false"
        :mobile-fields="orderMobileFields"
      >
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column prop="amount" label="金额" width="120">
          <template #default="scope">¥{{ Number(scope.row.amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="下单时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
      </ResponsiveTable>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Cellphone, Key, Monitor, List, Money, Wallet, UserFilled, Promotion, Coin, Upload, Setting } from '@element-plus/icons-vue'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { tenantDashboardApi, type TenantDashboardData } from '@/api/tenant'

const stats = ref<TenantDashboardData>({
  app_total: 0,
  app_active: 0,
  card_total: 0,
  card_active: 0,
  card_used: 0,
  device_online: 0,
  device_total: 0,
  order_today: 0,
  revenue_today: 0,
  revenue_month: 0,
  settlement_pending: 0,
  settlement_amount: 0,
  agent_total: 0
})

const revenueTrend = ref<NonNullable<TenantDashboardData['revenue_trend']>>([])
const topApps = ref<NonNullable<TenantDashboardData['top_apps']>>([])
const recentOrders = ref<NonNullable<TenantDashboardData['recent_orders']>>([])

const orderMobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'amount', label: '金额', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '下单时间', formatter: (v: string) => formatDate(v) }
]

const maxRevenue = computed(() => {
  return Math.max(1, ...revenueTrend.value.map(p => p.amount))
})

const barHeight = (amount: number) => {
  return Math.max(8, (amount / maxRevenue.value) * 100)
}

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
  try {
    const data = await tenantDashboardApi()
    if (data && typeof data === 'object') {
      Object.assign(stats.value, data)
      revenueTrend.value = data.revenue_trend || []
      topApps.value = data.top_apps || []
      recentOrders.value = data.recent_orders || []
    }
  } catch {
    // 铁律 06 待核实：后端 /tenant/dashboard 当前为 501（v0.3.0 交付），保持 0/空（铁律 04 不编造数据）
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
  display: flex;
  align-items: center;
  gap: $spacing-md;
  border-left: 4px solid $color-primary;
  transition: transform 0.2s;

  &:hover { transform: translateY(-2px); box-shadow: $shadow-hover; }

  &.apps { border-left-color: $color-primary; }
  &.cards { border-left-color: $color-success; }
  &.devices { border-left-color: $color-warning; }
  &.orders { border-left-color: $color-info; }
  &.revenue-today { border-left-color: $color-success; }
  &.settlement { border-left-color: $color-danger; }
  &.agents { border-left-color: $color-primary; }
  &.quick-action { border-left-color: $color-warning; cursor: pointer; }

  .stat-icon {
    width: 44px;
    height: 44px;
    border-radius: $radius-md;
    background: $color-primary-light;
    display: flex;
    align-items: center;
    justify-content: center;
    color: $color-primary;
    .el-icon { font-size: 22px; }
  }
  .stat-info {
    flex: 1;
    min-width: 0;
    .stat-label {
      font-size: 12px;
      color: $color-text-secondary;
    }
    .stat-value {
      font-size: 22px;
      font-weight: 600;
      color: $color-text-primary;
      font-family: 'SF Mono', 'Menlo', monospace;
      line-height: 1.4;
    }
    .stat-value-text {
      font-size: 16px;
      font-weight: 600;
      color: $color-primary;
      line-height: 1.4;
    }
    .stat-extra {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }

  @include mobile {
    padding: $spacing-sm $spacing-md;
    .stat-icon { width: 36px; height: 36px; .el-icon { font-size: 18px; } }
    .stat-info .stat-value { font-size: 16px; }
  }
}

.quick-entry {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: $spacing-md;
  margin-bottom: $spacing-lg;
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-lg;
  box-shadow: $shadow-card;

  @include tablet {
    grid-template-columns: repeat(4, 1fr);
  }
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

    &:hover { background: $color-bg-hover; }
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

.row-2col {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: $spacing-lg;
  margin-bottom: $spacing-lg;

  @include tablet {
    grid-template-columns: 1fr;
  }
  @include mobile {
    grid-template-columns: 1fr;
    gap: $spacing-md;
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

.trend-chart {
  display: flex;
  align-items: flex-end;
  gap: $spacing-sm;
  height: 200px;
  padding-top: $spacing-md;

  .trend-bar {
    flex: 1;
    background: linear-gradient(to top, $color-primary, lighten($color-primary, 20%));
    border-radius: $radius-sm $radius-sm 0 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
    position: relative;
    min-height: 30px;
    transition: opacity 0.2s;

    &:hover { opacity: 0.85; }

    .bar-value {
      position: absolute;
      top: -20px;
      font-size: 11px;
      color: $color-text-secondary;
      white-space: nowrap;
    }
    .bar-date {
      position: absolute;
      bottom: -20px;
      font-size: 11px;
      color: $color-text-secondary;
    }
  }

  @include mobile {
    height: 160px;
    .trend-bar .bar-value { display: none; }
  }
}

.top-apps {
  list-style: none;
  margin: 0;
  padding: 0;

  .app-item {
    display: flex;
    align-items: center;
    padding: $spacing-md $spacing-sm;
    border-bottom: 1px solid $color-border-lighter;

    &:last-child { border-bottom: none; }

    .rank {
      width: 24px;
      height: 24px;
      border-radius: 50%;
      background: $color-bg-page;
      color: $color-text-secondary;
      font-size: 12px;
      font-weight: 600;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-right: $spacing-md;

      &.rank-1 { background: $color-warning; color: #fff; }
      &.rank-2 { background: $color-text-placeholder; color: #fff; }
      &.rank-3 { background: $color-warning; color: #fff; opacity: 0.7; }
    }
    .app-name {
      flex: 1;
      font-size: 14px;
      color: $color-text-primary;
    }
    .app-card-count {
      font-size: 12px;
      color: $color-text-secondary;
      margin-right: $spacing-md;
    }
    .app-revenue {
      font-size: 14px;
      font-weight: 600;
      color: $color-success;
      font-family: 'SF Mono', 'Menlo', monospace;
    }
  }
}
</style>
