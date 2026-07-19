<!--
  开发者结算记录（响应式 H5）- v0.3.4 新增
  - 余额概览卡片：可用余额 / 冻结金额 / 累计已结 / 累计已提现 / 待审核提现
  - 双 Tab：
    1) 结算记录：自己的 platform_settlement，含 pending_sum/settled_sum 汇总
    2) 余额流水：tenant_balance_log，含 type/status 筛选
-->
<template>
  <div class="tenant-settlements-page">
    <PageHeader title="结算记录" subtitle="查看我的订单结算与余额流水" />

    <!-- 余额概览 -->
    <div class="wallet-overview">
      <div class="balance-main">
        <div class="label">可用余额</div>
        <div class="value">¥{{ formatMoney(overview.balance) }}</div>
        <div class="actions">
          <el-button type="primary" size="small" @click="goWithdraw">申请提现</el-button>
        </div>
      </div>
      <div class="balance-stats">
        <div class="stat">
          <span class="stat-label">冻结金额</span>
          <span class="stat-value warning">¥{{ formatMoney(overview.frozen_balance) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">累计已结</span>
          <span class="stat-value success">¥{{ formatMoney(overview.settled_total) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">累计已提现</span>
          <span class="stat-value">¥{{ formatMoney(overview.withdrawn_total) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">待审核提现</span>
          <span class="stat-value warning">¥{{ formatMoney(overview.pending_withdraw) }}</span>
        </div>
      </div>
    </div>

    <!-- 双 Tab -->
    <div class="app-card">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <!-- ===== Tab 1: 结算记录 ===== -->
        <el-tab-pane label="结算记录" name="settlements">
          <div class="search-bar">
            <el-select v-model="settleFilter.status" placeholder="状态" clearable style="width: 140px" @change="loadSettlements">
              <el-option label="待结算" value="pending" />
              <el-option label="已结算" value="settled" />
              <el-option label="已拒绝" value="rejected" />
            </el-select>
            <el-input
              v-model="settleFilter.order_no"
              placeholder="订单号搜索"
              clearable
              style="width: 220px"
              @keyup.enter="onSettleFilterChange"
            />
            <el-date-picker
              v-model="settleDateRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width: 260px"
              @change="onSettleFilterChange"
            />
            <el-button @click="onSettleFilterChange">查询</el-button>
            <el-button @click="loadSettlements">刷新</el-button>
            <span v-if="settleSum.pending_sum > 0" class="sum-hint warning">
              待结 ¥{{ formatMoney(settleSum.pending_sum) }}
            </span>
            <span v-if="settleSum.settled_sum > 0" class="sum-hint success">
              已结 ¥{{ formatMoney(settleSum.settled_sum) }}
            </span>
          </div>

          <ResponsiveTable
            :data="settleList"
            :loading="settleLoading"
            :total="settleTotal"
            v-model:page="settleFilter.page"
            v-model:pageSize="settleFilter.page_size"
            :mobile-fields="settleMobileFields"
            @page-change="loadSettlements"
            @size-change="loadSettlements"
          >
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="order_no" label="订单号" min-width="180" />
            <el-table-column prop="gross_amount" label="订单金额" width="120">
              <template #default="{ row }">¥{{ formatMoney(row.gross_amount) }}</template>
            </el-table-column>
            <el-table-column prop="commission_rate" label="抽成比例" width="100">
              <template #default="{ row }">{{ row.commission_rate }}%</template>
            </el-table-column>
            <el-table-column prop="commission_amount" label="平台抽成" width="120">
              <template #default="{ row }">¥{{ formatMoney(row.commission_amount) }}</template>
            </el-table-column>
            <el-table-column prop="net_amount" label="应得金额" width="120">
              <template #default="{ row }">
                <span class="text-success">¥{{ formatMoney(row.net_amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="settle_batch_no" label="结算批次号" min-width="160">
              <template #default="{ row }">{{ row.settle_batch_no || '-' }}</template>
            </el-table-column>
            <el-table-column prop="settled_at" label="结算时间" width="160">
              <template #default="{ row }">{{ formatDate(row.settled_at) }}</template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" width="160">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </el-table-column>
          </ResponsiveTable>
        </el-tab-pane>

        <!-- ===== Tab 2: 余额流水 ===== -->
        <el-tab-pane label="余额流水" name="balance_logs">
          <div class="search-bar">
            <el-select v-model="logFilter.type" placeholder="类型" clearable style="width: 140px" @change="onLogFilterChange">
              <el-option label="结算入账" value="settle" />
              <el-option label="提现申请" value="withdraw" />
              <el-option label="提现驳回退款" value="refund" />
              <el-option label="手动调整" value="adjust" />
            </el-select>
            <el-select v-model="logFilter.status" placeholder="状态" clearable style="width: 140px" @change="onLogFilterChange">
              <el-option label="待处理" value="pending" />
              <el-option label="已完成" value="settled" />
              <el-option label="已驳回" value="rejected" />
            </el-select>
            <el-date-picker
              v-model="logDateRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width: 260px"
              @change="onLogFilterChange"
            />
            <el-button @click="onLogFilterChange">查询</el-button>
            <el-button @click="loadBalanceLogs">刷新</el-button>
          </div>

          <ResponsiveTable
            :data="logList"
            :loading="logLoading"
            :total="logTotal"
            v-model:page="logFilter.page"
            v-model:pageSize="logFilter.page_size"
            :mobile-fields="logMobileFields"
            @page-change="loadBalanceLogs"
            @size-change="loadBalanceLogs"
          >
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="type" label="类型" width="140">
              <template #default="{ row }">
                <el-tag :type="logTypeTag(row.type)" size="small">{{ logTypeText(row.type) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="amount" label="金额变动" width="120">
              <template #default="{ row }">
                <span :class="logAmountClass(row)">
                  {{ logAmountSign(row) }}¥{{ formatMoney(row.amount) }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="balance_after" label="操作后余额" width="120">
              <template #default="{ row }">¥{{ formatMoney(row.balance_after) }}</template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="logStatusTag(row.status)" size="small">{{ logStatusText(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="settle_batch_no" label="批次号" min-width="160">
              <template #default="{ row }">{{ row.settle_batch_no || '-' }}</template>
            </el-table-column>
            <el-table-column prop="pay_method" label="方式" width="100">
              <template #default="{ row }">{{ row.pay_method ? methodText(row.pay_method) : '-' }}</template>
            </el-table-column>
            <el-table-column prop="remark" label="备注" min-width="160">
              <template #default="{ row }">{{ row.remark || '-' }}</template>
            </el-table-column>
            <el-table-column prop="created_at" label="时间" width="160">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </el-table-column>
          </ResponsiveTable>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listTenantSettlementsApi,
  tenantBalanceOverviewApi,
  listTenantBalanceLogsApi,
  type PlatformSettlement,
  type TenantBalanceLog,
  type TenantBalanceOverview
} from '@/api/tenantFinance'

const router = useRouter()

// ============== 通用工具 ==============
const formatMoney = (n: number) => (Number(n) || 0).toFixed(2)

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const methodText = (m: string) => ({
  alipay: '支付宝',
  wechat: '微信',
  bank: '银行卡',
  manual: '手动转账'
}[m] || m || '-')

const goWithdraw = () => router.push('/tenant/withdrawal')

// ============== 余额概览 ==============
const overview = ref<TenantBalanceOverview>({
  balance: 0,
  frozen_balance: 0,
  settled_total: 0,
  withdrawn_total: 0,
  pending_withdraw: 0,
  updated_at: ''
})

const loadOverview = async () => {
  try {
    const data = await tenantBalanceOverviewApi()
    Object.assign(overview.value, data)
  } catch {
    // 错误已由 http 拦截器处理
  }
}

// ============== Tab 切换 ==============
const activeTab = ref<'settlements' | 'balance_logs'>('settlements')

const onTabChange = (name: string | number) => {
  if (name === 'settlements') {
    loadSettlements()
  } else if (name === 'balance_logs') {
    loadBalanceLogs()
  }
}

// ============== Tab 1: 结算记录 ==============
const settleList = ref<PlatformSettlement[]>([])
const settleTotal = ref(0)
const settleLoading = ref(false)
const settleSum = reactive({ pending_sum: 0, settled_sum: 0 })
const settleDateRange = ref<[string, string] | null>(null)

const settleFilter = reactive({
  status: undefined as string | undefined,
  order_no: '',
  start_date: undefined as string | undefined,
  end_date: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const settleMobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'net_amount', label: '应得', formatter: (v: number) => '¥' + formatMoney(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const statusTag = (s: string): any => ({
  pending: 'warning',
  settled: 'success',
  rejected: 'info'
}[s] || 'info')

const statusText = (s: string) => ({
  pending: '待结算',
  settled: '已结算',
  rejected: '已拒绝'
}[s] || s)

const loadSettlements = async () => {
  settleLoading.value = true
  try {
    if (settleDateRange.value && settleDateRange.value.length === 2) {
      settleFilter.start_date = settleDateRange.value[0]
      settleFilter.end_date = settleDateRange.value[1]
    } else {
      settleFilter.start_date = undefined
      settleFilter.end_date = undefined
    }
    const resp = await listTenantSettlementsApi({
      status: settleFilter.status,
      order_no: settleFilter.order_no || undefined,
      start_date: settleFilter.start_date,
      end_date: settleFilter.end_date,
      page: settleFilter.page,
      page_size: settleFilter.page_size
    })
    settleList.value = resp.list || []
    settleTotal.value = resp.total || 0
    settleSum.pending_sum = resp.pending_sum || 0
    settleSum.settled_sum = resp.settled_sum || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    settleLoading.value = false
  }
}

const onSettleFilterChange = () => {
  settleFilter.page = 1
  loadSettlements()
}

// ============== Tab 2: 余额流水 ==============
const logList = ref<TenantBalanceLog[]>([])
const logTotal = ref(0)
const logLoading = ref(false)
const logDateRange = ref<[string, string] | null>(null)

const logFilter = reactive({
  type: undefined as string | undefined,
  status: undefined as string | undefined,
  start_date: undefined as string | undefined,
  end_date: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const logMobileFields = [
  { prop: 'type', label: '类型', formatter: (v: string) => logTypeText(v) },
  { prop: 'amount', label: '金额', formatter: (v: number, row: any) => logAmountSign(row) + '¥' + formatMoney(v) },
  { prop: 'balance_after', label: '余额', formatter: (v: number) => '¥' + formatMoney(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => logStatusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const logTypeTag = (t: string): any => ({
  settle: 'success',
  withdraw: 'warning',
  refund: 'primary',
  adjust: 'info'
}[t] || 'info')

const logTypeText = (t: string) => ({
  settle: '结算入账',
  withdraw: '提现申请',
  refund: '提现退款',
  adjust: '手动调整'
}[t] || t)

const logStatusTag = (s: string): any => ({
  pending: 'warning',
  settled: 'success',
  rejected: 'danger'
}[s] || 'info')

const logStatusText = (s: string) => ({
  pending: '待处理',
  settled: '已完成',
  rejected: '已驳回'
}[s] || s)

// 金额符号：settle/refund 为入账（+），withdraw 为出账（-），adjust 视金额正负
const logAmountSign = (row: any) => {
  if (row.type === 'settle' || row.type === 'refund') return '+'
  if (row.type === 'withdraw') return '-'
  return row.amount >= 0 ? '+' : ''
}
const logAmountClass = (row: any) => {
  if (row.type === 'settle' || row.type === 'refund') return 'text-success'
  if (row.type === 'withdraw') return 'text-danger'
  return row.amount >= 0 ? 'text-success' : 'text-danger'
}

const loadBalanceLogs = async () => {
  logLoading.value = true
  try {
    if (logDateRange.value && logDateRange.value.length === 2) {
      logFilter.start_date = logDateRange.value[0]
      logFilter.end_date = logDateRange.value[1]
    } else {
      logFilter.start_date = undefined
      logFilter.end_date = undefined
    }
    const resp = await listTenantBalanceLogsApi({
      type: logFilter.type,
      status: logFilter.status,
      start_date: logFilter.start_date,
      end_date: logFilter.end_date,
      page: logFilter.page,
      page_size: logFilter.page_size
    })
    logList.value = resp.list || []
    logTotal.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    logLoading.value = false
  }
}

const onLogFilterChange = () => {
  logFilter.page = 1
  loadBalanceLogs()
}

onMounted(() => {
  loadOverview()
  loadSettlements()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.tenant-settlements-page {
  .app-card {
    background: $color-bg-card;
    border-radius: $radius-md;
    padding: $spacing-md;
    box-shadow: $shadow-card;
  }

  .search-bar {
    display: flex;
    gap: $spacing-sm;
    margin-bottom: $spacing-md;
    flex-wrap: wrap;
    align-items: center;

    @include mobile {
      flex-direction: column;
      .el-select, .el-input, .el-button, .el-date-editor { width: 100% !important; }
    }
  }

  .sum-hint {
    font-size: 13px;
    font-weight: 600;
    &.warning { color: $color-warning; }
    &.success { color: $color-success; }
  }

  .text-success { color: $color-success; font-weight: 500; }
  .text-danger { color: $color-danger; font-weight: 500; }
}

.wallet-overview {
  display: grid;
  grid-template-columns: 1fr 2fr;
  gap: $spacing-md;
  margin-bottom: $spacing-lg;

  @include mobile {
    grid-template-columns: 1fr;
  }
}

.balance-main {
  background: linear-gradient(135deg, $color-primary-light, $color-bg-card);
  border: 1px solid $color-primary-light;
  border-radius: $radius-md;
  padding: $spacing-lg;
  box-shadow: $shadow-card;
  text-align: center;

  .label {
    font-size: 13px;
    color: $color-text-secondary;
  }
  .value {
    font-size: 32px;
    font-weight: 700;
    color: $color-primary;
    margin: $spacing-sm 0 $spacing-md;
    font-family: 'SF Mono', 'Menlo', monospace;
  }
  .actions {
    display: flex;
    gap: $spacing-sm;
    justify-content: center;
  }

  @include mobile {
    padding: $spacing-md;
    .value { font-size: 24px; }
  }
}

.balance-stats {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: $spacing-md;

  .stat {
    background: $color-bg-card;
    border-radius: $radius-md;
    padding: $spacing-md;
    box-shadow: $shadow-card;
    display: flex;
    flex-direction: column;
    gap: $spacing-sm;

    .stat-label {
      font-size: 13px;
      color: $color-text-secondary;
    }
    .stat-value {
      font-size: 20px;
      font-weight: 600;
      color: $color-text-primary;
      font-family: 'SF Mono', 'Menlo', monospace;
      &.success { color: $color-success; }
      &.warning { color: $color-warning; }
    }
  }

  @include mobile {
    grid-template-columns: repeat(2, 1fr);
    gap: $spacing-sm;
    .stat {
      padding: $spacing-sm;
      .stat-value { font-size: 14px; }
      .stat-label { font-size: 11px; }
    }
  }
}
</style>
