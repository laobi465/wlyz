<!--
  开发者提现申请（响应式 H5）- v0.3.4 新增
  - 余额概览：可用余额 / 冻结金额 / 累计已提现 / 待审核提现
  - 提现表单：金额 + 收款方式 + 收款账号 + 备注
  - 提现记录列表：自己的 tenant_withdraw
-->
<template>
  <div class="tenant-withdrawal-page">
    <PageHeader title="提现申请" subtitle="申请提现并查看提现记录" />

    <!-- 余额概览 -->
    <div class="wallet-overview">
      <div class="balance-main">
        <div class="label">可用余额</div>
        <div class="value">¥{{ formatMoney(overview.balance) }}</div>
        <div class="actions">
          <el-button type="primary" size="small" @click="scrollToForm">立即提现</el-button>
          <el-button size="small" @click="goSettlements">查看结算</el-button>
        </div>
      </div>
      <div class="balance-stats">
        <div class="stat">
          <span class="stat-label">冻结金额</span>
          <span class="stat-value warning">¥{{ formatMoney(overview.frozen_balance) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">累计已提现</span>
          <span class="stat-value success">¥{{ formatMoney(overview.withdrawn_total) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">待审核提现</span>
          <span class="stat-value warning">¥{{ formatMoney(overview.pending_withdraw) }}</span>
        </div>
      </div>
    </div>

    <!-- 提现表单 + 提现记录 双栏 -->
    <div class="content-grid">
      <!-- 左：提现表单 -->
      <div class="app-card form-card" ref="formCardRef">
        <h3 class="card-title">发起提现申请</h3>
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
          提现申请提交后，平台超管将审核打款；审核期间对应金额将被冻结。
        </el-alert>
        <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
          <el-form-item label="可提现余额">
            <el-input :model-value="'¥' + formatMoney(overview.balance)" disabled />
          </el-form-item>
          <el-form-item label="提现金额" prop="amount">
            <el-input-number
              v-model="form.amount"
              :min="0.01"
              :max="overview.balance"
              :precision="2"
              :step="100"
              style="width: 100%"
            />
            <div class="quick-amounts">
              <el-button text size="small" @click="form.amount = overview.balance">全部提现</el-button>
              <el-button text size="small" @click="form.amount = Math.floor(overview.balance / 2) * 1">提现一半</el-button>
            </div>
          </el-form-item>
          <el-form-item label="收款方式" prop="pay_method">
            <el-radio-group v-model="form.pay_method">
              <el-radio value="alipay">支付宝</el-radio>
              <el-radio value="wechat">微信</el-radio>
              <el-radio value="bank">银行卡</el-radio>
            </el-radio-group>
          </el-form-item>
          <el-form-item label="收款账号" prop="pay_account">
            <el-input
              v-model="form.pay_account"
              :placeholder="accountPlaceholder"
              maxlength="128"
              show-word-limit
            />
          </el-form-item>
          <el-form-item label="备注">
            <el-input
              v-model="form.remark"
              type="textarea"
              :rows="3"
              placeholder="可选，例如：收款人姓名 / 开户行名称"
              maxlength="255"
              show-word-limit
            />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="submitLoading" @click="confirmSubmit" style="width: 100%">
              提交提现申请
            </el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 右：提现记录 -->
      <div class="app-card">
        <div class="search-bar">
          <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="onFilterChange">
            <el-option label="待审核" value="pending" />
            <el-option label="已打款" value="paid" />
            <el-option label="已驳回" value="rejected" />
            <el-option label="打款失败" value="failed" />
          </el-select>
          <el-button @click="loadList">刷新</el-button>
        </div>

        <ResponsiveTable
          :data="list"
          :loading="loading"
          :total="total"
          v-model:page="filter.page"
          v-model:pageSize="filter.page_size"
          :mobile-fields="mobileFields"
          @page-change="loadList"
          @size-change="loadList"
        >
          <el-table-column prop="id" label="ID" width="80" />
          <el-table-column prop="amount" label="提现金额" width="120">
            <template #default="{ row }">
              <span class="text-danger">¥{{ formatMoney(row.amount) }}</span>
            </template>
          </el-table-column>
          <el-table-column prop="pay_method" label="方式" width="100">
            <template #default="{ row }">{{ methodText(row.pay_method) }}</template>
          </el-table-column>
          <el-table-column prop="pay_account" label="收款账号" min-width="180">
            <template #default="{ row }">{{ row.pay_account || '-' }}</template>
          </el-table-column>
          <el-table-column prop="status" label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column prop="audit_remark" label="审核备注" min-width="160">
            <template #default="{ row }">{{ row.audit_remark || '-' }}</template>
          </el-table-column>
          <el-table-column prop="pay_trade_no" label="打款流水号" min-width="160">
            <template #default="{ row }">{{ row.pay_trade_no || '-' }}</template>
          </el-table-column>
          <el-table-column prop="paid_at" label="打款时间" width="160">
            <template #default="{ row }">{{ formatDate(row.paid_at) }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="申请时间" width="160">
            <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
          </el-table-column>
        </ResponsiveTable>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  tenantBalanceOverviewApi,
  listTenantOwnWithdrawalsApi,
  tenantWithdrawApi,
  type TenantWithdrawal,
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
  bank: '银行卡'
}[m] || m || '-')

const statusTag = (s: string): any => ({
  pending: 'warning',
  paid: 'success',
  rejected: 'danger',
  failed: 'info'
}[s] || 'info')

const statusText = (s: string) => ({
  pending: '待审核',
  paid: '已打款',
  rejected: '已驳回',
  failed: '打款失败'
}[s] || s)

const goSettlements = () => router.push('/tenant/settlements')

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

// ============== 提现表单 ==============
const formRef = ref<FormInstance>()
const formCardRef = ref<HTMLElement>()
const submitLoading = ref(false)

const form = reactive({
  amount: 0,
  pay_method: 'alipay' as 'alipay' | 'wechat' | 'bank',
  pay_account: '',
  remark: ''
})

const accountPlaceholder = computed(() => {
  switch (form.pay_method) {
    case 'alipay': return '支付宝账号（邮箱/手机号）'
    case 'wechat': return '微信号'
    case 'bank': return '银行卡号'
    default: return '收款账号'
  }
})

const rules = {
  amount: [
    { required: true, message: '请输入提现金额', trigger: 'blur' },
    {
      validator: (_rule: any, value: number, callback: any) => {
        if (!value || value <= 0) {
          callback(new Error('提现金额必须大于 0'))
        } else if (value > overview.value.balance) {
          callback(new Error('提现金额不能超过可提现余额'))
        } else {
          callback()
        }
      },
      trigger: 'blur'
    }
  ],
  pay_method: [{ required: true, message: '请选择收款方式', trigger: 'change' }],
  pay_account: [
    { required: true, message: '请填写收款账号', trigger: 'blur' },
    { min: 4, max: 128, message: '账号长度 4-128 字符', trigger: 'blur' }
  ]
}

const scrollToForm = () => {
  if (formCardRef.value) {
    formCardRef.value.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }
}

const confirmSubmit = async () => {
  if (!formRef.value) return
  try {
    await formRef.value.validate()
  } catch {
    return
  }
  submitLoading.value = true
  try {
    const resp = await tenantWithdrawApi({
      amount: form.amount,
      pay_method: form.pay_method,
      pay_account: form.pay_account,
      remark: form.remark
    })
    ElMessage.success(resp.message || '提现申请已提交，等待平台审核')
    // 重置表单
    form.amount = 0
    form.pay_account = ''
    form.remark = ''
    // 刷新数据
    loadOverview()
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    submitLoading.value = false
  }
}

// ============== 提现记录 ==============
const list = ref<TenantWithdrawal[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'amount', label: '金额', formatter: (v: number) => '¥' + formatMoney(v) },
  { prop: 'pay_method', label: '方式', formatter: (v: string) => methodText(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listTenantOwnWithdrawalsApi({
      status: filter.status,
      page: filter.page,
      page_size: filter.page_size
    })
    list.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const onFilterChange = () => {
  filter.page = 1
  loadList()
}

onMounted(() => {
  loadOverview()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.tenant-withdrawal-page {
  .app-card {
    background: $color-bg-card;
    border-radius: $radius-md;
    padding: $spacing-md;
    box-shadow: $shadow-card;
  }

  .card-title {
    margin: 0 0 $spacing-md;
    font-size: 16px;
    font-weight: 600;
    color: $color-text-primary;
  }

  .search-bar {
    display: flex;
    gap: $spacing-sm;
    margin-bottom: $spacing-md;
    flex-wrap: wrap;

    @include mobile {
      flex-direction: column;
      .el-select, .el-button { width: 100% !important; }
    }
  }

  .quick-amounts {
    margin-top: $spacing-xs;
    display: flex;
    gap: $spacing-sm;
  }

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
  grid-template-columns: repeat(3, 1fr);
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
    grid-template-columns: repeat(3, 1fr);
    gap: $spacing-sm;
    .stat {
      padding: $spacing-sm;
      .stat-value { font-size: 14px; }
      .stat-label { font-size: 11px; }
    }
  }
}

.content-grid {
  display: grid;
  grid-template-columns: 1fr 1.5fr;
  gap: $spacing-md;

  @include mobile {
    grid-template-columns: 1fr;
  }
}
</style>
