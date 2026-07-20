<!--
  代理佣金（响应式 H5）
  - 顶部统计卡：累计佣金 / 已提现 / 可提现（=余额）
  - 申请提现按钮 → 弹窗（金额 + 收款方式 + 收款账号 + 真实姓名 + 备注）
  - 流水列表：购卡佣金 / 提现申请 / 充值 / 调整
  铁律 06 待核实：后端 /agent/commission 与 /agent/withdraw 当前为 501 占位（v0.3.0 交付）。
-->
<template>
  <div class="agent-commission-page">
    <PageHeader title="佣金记录" subtitle="佣金流水明细与提现申请">
      <template #actions>
        <el-button type="primary" @click="openWithdraw">申请提现</el-button>
        <el-button @click="loadList">刷新</el-button>
      </template>
    </PageHeader>

    <!-- 统计卡 -->
    <div class="stat-grid">
      <div class="stat-card">
        <div class="label">累计佣金</div>
        <div class="value primary">¥{{ profile.total_commission.toFixed(2) }}</div>
      </div>
      <div class="stat-card">
        <div class="label">已提现</div>
        <div class="value">¥{{ profile.total_withdraw.toFixed(2) }}</div>
      </div>
      <div class="stat-card">
        <div class="label">可提现余额</div>
        <div class="value success">¥{{ profile.balance.toFixed(2) }}</div>
      </div>
      <div class="stat-card">
        <div class="label">冻结金额</div>
        <div class="value warning">¥{{ profile.frozen_balance.toFixed(2) }}</div>
      </div>
    </div>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.type" placeholder="流水类型" clearable style="width: 140px" @change="loadList">
          <el-option label="购卡佣金" value="purchase" />
          <el-option label="提现申请" value="withdraw" />
          <el-option label="充值" value="recharge" />
          <el-option label="调整" value="adjust" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="待审核" value="pending" />
          <el-option label="已通过" value="approved" />
          <el-option label="已拒绝" value="rejected" />
          <el-option label="已打款" value="paid" />
        </el-select>
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
        <el-table-column prop="related_order_no" label="关联订单" min-width="160">
          <template #default="scope">{{ scope.row.related_order_no || '-' }}</template>
        </el-table-column>
        <el-table-column prop="withdraw_method" label="提现方式" width="100">
          <template #default="scope">{{ scope.row.withdraw_method ? methodText(scope.row.withdraw_method) : '-' }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160">
          <template #default="scope">{{ scope.row.remark || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 申请提现对话框 -->
    <el-dialog v-model="withdrawVisible" title="申请提现" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="可提现余额">
          <el-input :model-value="'¥' + profile.balance.toFixed(2)" disabled />
        </el-form-item>
        <el-form-item label="提现金额" prop="amount">
          <el-input-number v-model="form.amount" :min="1" :max="profile.balance" :precision="2" :step="100" />
          <span class="hint">最低 ¥1，最高 ¥{{ profile.balance.toFixed(2) }}</span>
        </el-form-item>
        <el-form-item label="收款方式" prop="method">
          <el-radio-group v-model="form.method">
            <el-radio value="alipay">支付宝</el-radio>
            <el-radio value="wechat">微信</el-radio>
            <el-radio value="bank">银行卡</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="收款账号" prop="account">
          <el-input v-model="form.account" placeholder="请输入收款账号" maxlength="64" />
        </el-form-item>
        <el-form-item label="真实姓名" prop="real_name">
          <el-input v-model="form.real_name" placeholder="请输入收款人真实姓名" maxlength="32" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" placeholder="可选" maxlength="200" />
        </el-form-item>
        <el-alert type="info" :closable="false" show-icon>
          提现申请提交后将进入冻结状态，待开发者审核打款后到账；审核不通过金额会退回余额。
        </el-alert>
      </el-form>
      <template #footer>
        <el-button @click="withdrawVisible = false">取消</el-button>
        <el-button type="primary" :loading="withdrawLoading" @click="confirmWithdraw">提交申请</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
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

const profile = ref<AgentProfile>({
  agent_id: 0,
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
  { prop: 'related_order_no', label: '关联订单' },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

// 提现表单
const withdrawVisible = ref(false)
const withdrawLoading = ref(false)
const formRef = ref<FormInstance>()
const form = reactive({
  amount: 100,
  method: 'alipay' as 'alipay' | 'wechat' | 'bank',
  account: '',
  real_name: '',
  remark: ''
})

const rules = {
  amount: [{ required: true, message: '请输入提现金额', trigger: 'blur' }],
  method: [{ required: true, message: '请选择收款方式', trigger: 'change' }],
  account: [{ required: true, message: '请输入收款账号', trigger: 'blur' }],
  real_name: [{ required: true, message: '请输入真实姓名', trigger: 'blur' }]
}

const typeTag = (t: string): any => ({
  purchase: 'success',
  withdraw: 'warning',
  recharge: 'primary',
  adjust: 'info'
}[t] || 'info')

const typeText = (t: string) => ({
  purchase: '购卡佣金',
  withdraw: '提现申请',
  recharge: '充值',
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

const amountSign = (row: any) => {
  // 提现为负，其他为正
  return row.type === 'withdraw' ? '-' : '+'
}

const amountClass = (row: any) => {
  return row.type === 'withdraw' ? 'text-danger' : 'text-success'
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadProfile = async () => {
  try {
    const data = await agentMeApi()
    if (data && typeof data === 'object') {
      Object.assign(profile.value, data)
    }
  } catch {
    // 铁律 06 待核实：可能正常返回
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

const openWithdraw = () => {
  if (profile.value.balance <= 0) {
    ElMessage.warning('可提现余额为 0')
    return
  }
  Object.assign(form, {
    amount: Math.min(100, profile.value.balance),
    method: 'alipay',
    account: '',
    real_name: profile.value.real_name || '',
    remark: ''
  })
  withdrawVisible.value = true
}

const confirmWithdraw = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (form.amount > profile.value.balance) {
      ElMessage.error('提现金额超过可提现余额')
      return
    }
    withdrawLoading.value = true
    try {
      await agentWithdrawApi({
        amount: form.amount,
        method: form.method,
        account: form.account,
        real_name: form.real_name,
        remark: form.remark
      })
      ElMessage.success('提现申请已提交，等待开发者审核')
      withdrawVisible.value = false
      loadProfile()
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      withdrawLoading.value = false
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

  .label {
    font-size: 13px;
    color: $color-text-secondary;
  }
  .value {
    font-size: 22px;
    font-weight: 600;
    color: $color-text-primary;
    margin-top: 6px;
    font-family: 'SF Mono', 'Menlo', monospace;
    &.primary { color: $color-primary; }
    &.success { color: $color-success; }
    &.warning { color: $color-warning; }
  }

  @include mobile {
    padding: $spacing-sm $spacing-md;
    .value { font-size: 16px; }
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
