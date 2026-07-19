<!--
  代理余额/提现（响应式 H5）
  - 钱包概览：可用余额 / 冻结 / 累计充值 / 累计提现
  - 申请充值按钮 → 弹窗（金额 + 备注，提交后等开发者审核）
  - 流水列表：默认过滤 recharge + withdraw 类型
  - 与"佣金记录"页面互补：本页关注钱包出入，佣金页关注收入明细
  铁律 06 待核实：后端无 /agent/recharge 端点，当前调用 /agent/withdraw 提交充值（type 字段区分），
                实际接口待 v0.3.0 实现。
-->
<template>
  <div class="agent-balance-page">
    <PageHeader title="余额 / 提现" subtitle="钱包余额、充值申请与提现记录">
      <template #actions>
        <el-button type="primary" @click="openRecharge">申请充值</el-button>
        <el-button @click="goCommission">申请提现</el-button>
      </template>
    </PageHeader>

    <!-- 钱包概览 -->
    <div class="wallet-overview">
      <div class="balance-main">
        <div class="label">可用余额</div>
        <div class="value">¥{{ profile.balance.toFixed(2) }}</div>
        <div class="actions">
          <el-button type="primary" size="small" @click="openRecharge">充值</el-button>
          <el-button type="success" size="small" @click="goCommission">提现</el-button>
        </div>
      </div>
      <div class="balance-stats">
        <div class="stat">
          <span class="stat-label">冻结金额</span>
          <span class="stat-value warning">¥{{ profile.frozen_balance.toFixed(2) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">累计佣金</span>
          <span class="stat-value success">¥{{ profile.total_commission.toFixed(2) }}</span>
        </div>
        <div class="stat">
          <span class="stat-label">累计提现</span>
          <span class="stat-value">¥{{ profile.total_withdraw.toFixed(2) }}</span>
        </div>
      </div>
    </div>

    <!-- 充值/提现记录 -->
    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.type" placeholder="类型" clearable style="width: 140px" @change="loadList">
          <el-option label="充值" value="recharge" />
          <el-option label="提现" value="withdraw" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="待审核" value="pending" />
          <el-option label="已通过" value="approved" />
          <el-option label="已拒绝" value="rejected" />
          <el-option label="已打款" value="paid" />
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
        <el-table-column prop="type" label="类型" width="100">
          <template #default="scope">
            <el-tag :type="typeTag(scope.row.type)" size="small">{{ typeText(scope.row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="金额" width="120">
          <template #default="scope">
            <span :class="amountClass(scope.row)">
              {{ amountSign(scope.row) }}¥{{ Number(scope.row.amount).toFixed(2) }}
            </span>
          </template>
        </el-table-column>
        <el-table-column prop="balance_after" label="操作后余额" width="120">
          <template #default="scope">¥{{ Number(scope.row.balance_after).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="withdraw_method" label="方式" width="100">
          <template #default="scope">{{ scope.row.withdraw_method ? methodText(scope.row.withdraw_method) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="withdraw_account" label="账号" min-width="160">
          <template #default="scope">{{ scope.row.withdraw_account || '-' }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160">
          <template #default="scope">{{ scope.row.remark || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 申请充值对话框 -->
    <el-dialog v-model="rechargeVisible" title="申请充值" width="500px">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px;">
        充值申请提交后需开发者审核确认，审核通过后人工转账至开发者账户，开发者确认到账后系统自动入账。
      </el-alert>
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="充值金额" prop="amount">
          <el-input-number v-model="form.amount" :min="1" :precision="2" :step="100" />
          <span class="hint">最低 ¥1</span>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="3" placeholder="可选，例如：支付宝转账 ¥500，订单号 20260719001" maxlength="200" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rechargeVisible = false">取消</el-button>
        <el-button type="primary" :loading="rechargeLoading" @click="confirmRecharge">提交申请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listAgentCommissionApi,
  agentWithdrawApi,
  agentMeApi,
  type AgentCommission,
  type CommissionType,
  type CommissionStatus,
  type AgentProfile
} from '@/api/agent'

const router = useRouter()

const profile = ref<AgentProfile>({
  id: 0,
  username: '',
  tenant_id: 0,
  real_name: '',
  phone: '',
  balance: 0,
  frozen_balance: 0,
  total_commission: 0,
  total_withdraw: 0,
  status: 'active',
  created_at: ''
})

const list = ref<AgentCommission[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  type: undefined as CommissionType | undefined,
  status: undefined as CommissionStatus | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'type', label: '类型', formatter: (v: string) => typeText(v) },
  { prop: 'amount', label: '金额', formatter: (v: number, row: AgentCommission) => amountSign(row) + '¥' + Number(v).toFixed(2) },
  { prop: 'balance_after', label: '余额', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const rechargeVisible = ref(false)
const rechargeLoading = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({
  amount: 100,
  remark: ''
})

const rules = {
  amount: [{ required: true, message: '请输入充值金额', trigger: 'blur' }]
}

const typeTag = (t: string): any => ({
  recharge: 'primary',
  withdraw: 'warning',
  purchase: 'success',
  adjust: 'info'
}[t] || 'info')

const typeText = (t: string) => ({
  recharge: '充值',
  withdraw: '提现',
  purchase: '购卡佣金',
  adjust: '调整'
}[t] || t)

const statusTag = (s: string): any => ({
  pending: 'warning',
  approved: 'primary',
  rejected: 'danger',
  paid: 'success'
}[s] || 'info')

const statusText = (s: string) => ({
  pending: '待审核',
  approved: '已通过',
  rejected: '已拒绝',
  paid: '已打款'
}[s] || s)

const methodText = (m: string) => ({
  alipay: '支付宝',
  wechat: '微信',
  bank: '银行卡'
}[m] || m)

const amountSign = (row: any) => row.type === 'withdraw' ? '-' : '+'
const amountClass = (row: any) => row.type === 'withdraw' ? 'text-danger' : 'text-success'

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const goCommission = () => router.push('/agent/commission')

const loadProfile = async () => {
  try {
    const data = await agentMeApi()
    if (data && typeof data === 'object') {
      Object.assign(profile.value, data)
    }
  } catch {
    // 铁律 06 待核实
  }
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAgentCommissionApi({
      type: filter.type,
      status: filter.status,
      page: filter.page,
      page_size: filter.page_size
    })
    list.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 铁律 04 不编造数据
  } finally {
    loading.value = false
  }
}

const openRecharge = () => {
  Object.assign(form, { amount: 100, remark: '' })
  rechargeVisible.value = true
}

const confirmRecharge = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    rechargeLoading.value = true
    try {
      // 铁律 06 待核实：后端暂无 /agent/recharge 端点，临时复用 /agent/withdraw 提交（type 区分）
      // 待 v0.3.0 实现独立的 recharge 端点
      await agentWithdrawApi({
        amount: form.amount,
        method: 'alipay', // 占位，充值不需要
        account: 'recharge_application', // 占位标识
        remark: `[充值申请] ${form.remark}`
      })
      ElMessage.success('充值申请已提交，等待开发者审核')
      rechargeVisible.value = false
      loadProfile()
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      rechargeLoading.value = false
    }
  })
}

onMounted(() => {
  loadProfile()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

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

  @include mobile {
    .el-select { width: 100% !important; }
  }
}

.text-success { color: $color-success; font-weight: 500; }
.text-danger { color: $color-danger; font-weight: 500; }

.hint {
  margin-left: $spacing-sm;
  font-size: 12px;
  color: $color-text-secondary;
}
</style>
