<!--
  平台看板（响应式 H5）
  - 顶部核心指标：开发者/代理/应用/卡密/订单/收入/待结算
  - 待办事项：待结算订单、新增开发者审核
  - 收入趋势：近 7 日订单金额折线（待 v0.3.0）
  - 最近开发者 / 最近订单
  铁律 06 待核实：后端 /admin/dashboard 当前为 501 占位（v0.3.0 交付），调用失败时显示 0/空。
-->
<template>
  <div class="admin-dashboard">
    <PageHeader title="平台看板" subtitle="平台核心指标、待办事项与最近动态一览">
      <template #actions>
        <el-button @click="loadDashboard">刷新</el-button>
      </template>
    </PageHeader>

    <!-- 核心指标卡 -->
    <div class="stat-grid">
      <div class="stat-card tenants">
        <div class="stat-icon"><el-icon><User /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">开发者总数</div>
          <div class="stat-value"><CountUp :value="stats.tenant_total" /></div>
          <div class="stat-extra">活跃 {{ stats.tenant_active }}</div>
        </div>
      </div>
      <div class="stat-card agents">
        <div class="stat-icon"><el-icon><UserFilled /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">代理总数</div>
          <div class="stat-value"><CountUp :value="stats.agent_total" /></div>
          <div class="stat-extra">活跃 {{ stats.agent_active }}</div>
        </div>
      </div>
      <div class="stat-card apps">
        <div class="stat-icon"><el-icon><Cellphone /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">应用总数</div>
          <div class="stat-value"><CountUp :value="stats.app_total" /></div>
          <div class="stat-extra">-</div>
        </div>
      </div>
      <div class="stat-card cards">
        <div class="stat-icon"><el-icon><Key /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">卡密总数</div>
          <div class="stat-value"><CountUp :value="stats.card_total" /></div>
          <div class="stat-extra">活跃 {{ stats.card_active }}</div>
        </div>
      </div>
      <div class="stat-card orders">
        <div class="stat-icon"><el-icon><List /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">今日订单</div>
          <div class="stat-value"><CountUp :value="stats.order_today" /></div>
          <div class="stat-extra">-</div>
        </div>
      </div>
      <div class="stat-card revenue-today">
        <div class="stat-icon"><el-icon><Money /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">今日收入</div>
          <div class="stat-value"><CountUp :value="stats.revenue_today" :decimals="2" prefix="¥" /></div>
          <div class="stat-extra">本月 ¥{{ stats.revenue_month.toFixed(2) }}</div>
        </div>
      </div>
      <div class="stat-card settlement">
        <div class="stat-icon"><el-icon><Wallet /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">待结算</div>
          <div class="stat-value"><CountUp :value="stats.settlement_pending" /></div>
          <div class="stat-extra">¥{{ stats.settlement_amount.toFixed(2) }}</div>
        </div>
      </div>
      <div class="stat-card quick-action" @click="$router.push('/admin/sys-config')">
        <div class="stat-icon"><el-icon><Setting /></el-icon></div>
        <div class="stat-info">
          <div class="stat-label">系统配置</div>
          <div class="stat-value-text">前往配置</div>
          <div class="stat-extra">参数管理</div>
        </div>
      </div>
    </div>

    <!-- 待办事项 + 收入趋势 -->
    <div class="row-2col">
      <div class="app-card todo-card">
        <div class="card-header">
          <h3>待办事项</h3>
        </div>
        <ul class="todo-list">
          <li class="todo-item" @click="$router.push('/admin/settlements')">
            <span class="todo-label">待结算订单</span>
            <span class="todo-value danger">{{ stats.settlement_pending }} 笔 / ¥{{ stats.settlement_amount.toFixed(2) }}</span>
            <el-icon class="todo-arrow"><ArrowRight /></el-icon>
          </li>
          <li class="todo-item" @click="$router.push('/admin/tenants')">
            <span class="todo-label">开发者审核</span>
            <span class="todo-value">去查看</span>
            <el-icon class="todo-arrow"><ArrowRight /></el-icon>
          </li>
          <li class="todo-item" @click="$router.push('/admin/agents')">
            <span class="todo-label">代理审核</span>
            <span class="todo-value">去查看</span>
            <el-icon class="todo-arrow"><ArrowRight /></el-icon>
          </li>
          <li class="todo-item" @click="$router.push('/admin/notices')">
            <span class="todo-label">公告管理</span>
            <span class="todo-value">去查看</span>
            <el-icon class="todo-arrow"><ArrowRight /></el-icon>
          </li>
        </ul>
      </div>

      <div class="app-card trend-card">
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
    </div>

    <!-- 最近开发者 + 最近订单 -->
    <div class="row-2col">
      <div class="app-card">
        <div class="card-header">
          <h3>最近注册开发者</h3>
          <el-button link type="primary" @click="$router.push('/admin/tenants')">查看全部</el-button>
        </div>
        <EmptyState v-if="!recentTenants.length" description="暂无数据（待 v0.3.0）" :image-size="80" />
        <ResponsiveTable
          v-else
          :data="recentTenants"
          :show-pagination="false"
          :mobile-fields="tenantMobileFields"
        >
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="username" label="用户名" min-width="140" />
          <el-table-column prop="status" label="状态" width="100">
            <template #default="scope">
              <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="注册时间" width="180">
            <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
          </el-table-column>
        </ResponsiveTable>
      </div>

      <div class="app-card">
        <div class="card-header">
          <h3>最近订单</h3>
          <el-button link type="primary" @click="$router.push('/admin/settlements')">查看全部</el-button>
        </div>
        <EmptyState v-if="!recentOrders.length" description="暂无数据（待 v0.3.0）" :image-size="80" />
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
              <el-tag :type="orderStatusTag(scope.row.status)" size="small">{{ orderStatusText(scope.row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="下单时间" width="180">
            <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
          </el-table-column>
        </ResponsiveTable>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { User, UserFilled, Cellphone, Key, List, Money, Wallet, Setting, ArrowRight } from '@element-plus/icons-vue'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { adminDashboardApi, type AdminDashboardData } from '@/api/admin'

const stats = ref<AdminDashboardData>({
  tenant_total: 0,
  tenant_active: 0,
  agent_total: 0,
  agent_active: 0,
  app_total: 0,
  card_total: 0,
  card_active: 0,
  order_today: 0,
  revenue_today: 0,
  revenue_month: 0,
  settlement_pending: 0,
  settlement_amount: 0
})

const recentTenants = ref<NonNullable<AdminDashboardData['recent_tenants']>>([])
const recentOrders = ref<NonNullable<AdminDashboardData['recent_orders']>>([])
const revenueTrend = ref<NonNullable<AdminDashboardData['revenue_trend']>>([])

const tenantMobileFields = [
  { prop: 'username', label: '用户名' },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '注册时间', formatter: (v: string) => formatDate(v) }
]
const orderMobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'amount', label: '金额', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => orderStatusText(v) },
  { prop: 'created_at', label: '下单时间', formatter: (v: string) => formatDate(v) }
]

const maxRevenue = computed(() => {
  return Math.max(1, ...revenueTrend.value.map(p => p.amount))
})

const barHeight = (amount: number) => {
  return Math.max(8, (amount / maxRevenue.value) * 100)
}

const statusTag = (s: string): any => ({
  active: 'success',
  disabled: 'danger',
  pending: 'warning'
}[s] || 'info')

const statusText = (s: string) => ({
  active: '正常',
  disabled: '已禁用',
  pending: '待审核'
}[s] || s)

const orderStatusTag = (s: string): any => ({
  paid: 'success',
  pending: 'warning',
  closed: 'info',
  refunded: 'danger'
}[s] || 'info')

const orderStatusText = (s: string) => ({
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
    const data = await adminDashboardApi()
    if (data && typeof data === 'object') {
      Object.assign(stats.value, data)
      recentTenants.value = data.recent_tenants || []
      recentOrders.value = data.recent_orders || []
      revenueTrend.value = data.revenue_trend || []
    }
  } catch {
    // 错误已由 http 拦截器处理
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

  &.tenants { border-left-color: $color-primary; }
  &.agents { border-left-color: $color-success; }
  &.apps { border-left-color: $color-warning; }
  &.cards { border-left-color: $color-info; }
  &.orders { border-left-color: $color-primary; }
  &.revenue-today { border-left-color: $color-success; }
  &.settlement { border-left-color: $color-danger; }
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

.todo-list {
  list-style: none;
  margin: 0;
  padding: 0;

  .todo-item {
    display: flex;
    align-items: center;
    padding: $spacing-md $spacing-sm;
    border-bottom: 1px solid $color-border-lighter;
    cursor: pointer;
    transition: background 0.2s;

    &:last-child { border-bottom: none; }
    &:hover { background: $color-bg-hover; }

    .todo-label {
      flex: 1;
      font-size: 14px;
      color: $color-text-primary;
    }
    .todo-value {
      font-size: 13px;
      color: $color-text-secondary;
      &.danger { color: $color-danger; font-weight: 500; }
    }
    .todo-arrow {
      margin-left: $spacing-sm;
      color: $color-text-placeholder;
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
    // v0.5.0 多主题：$color-primary 现为 var()，无法在 SCSS lighten() 中使用
    // 改用 CSS color-mix() 实现"主色 + 20% 白色"渐变（现代浏览器原生支持）
    background: linear-gradient(to top, $color-primary, color-mix(in srgb, $color-primary 80%, white));
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

.trend-card {
  padding-bottom: $spacing-xl;
}
</style>
