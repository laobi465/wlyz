<!--
  代理提现审核（开发者）- 响应式 H5
  - 列表：代理用户名 / 提现金额 / 收款方式 / 收款账号 / 审核备注 / 状态 / 时间
  - 操作：标记已打款（可填写打款流水号）/ 驳回（退回余额，必填原因）
  v0.3.2 已交付 list / pay / reject API。
-->
<template>
  <div class="withdrawal-review-page">
    <PageHeader title="提现审核" subtitle="审核代理提现申请并标记打款" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="onFilterChange">
          <el-option label="待审核" value="pending" />
          <el-option label="已打款" value="paid" />
          <el-option label="已驳回" value="rejected" />
        </el-select>
        <el-input
          v-model="filter.keyword"
          placeholder="用户名/收款账号"
          clearable
          style="width: 220px"
          @keyup.enter="onFilterChange"
        />
        <el-button @click="onFilterChange">查询</el-button>
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
        <el-table-column prop="agent_username" label="代理" min-width="140">
          <template #default="{ row }">
            <div>{{ row.agent_username || '-' }}</div>
            <div class="cell-sub">{{ row.agent_phone || '' }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="amount" label="提现金额" width="120">
          <template #default="{ row }">
            <span class="text-danger">¥{{ formatMoney(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="pay_method" label="收款方式" width="100">
          <template #default="{ row }">{{ methodText(row.pay_method) }}</template>
        </el-table-column>
        <el-table-column prop="pay_account" label="收款账号" min-width="180">
          <template #default="{ row }">
            <div>{{ row.pay_account || '-' }}</div>
            <div v-if="row.audit_remark" class="cell-sub">{{ row.audit_remark }}</div>
          </template>
        </el-table-column>
        <el-table-column prop="pay_trade_no" label="打款流水号" min-width="160">
          <template #default="{ row }">
            <span v-if="row.pay_trade_no">{{ row.pay_trade_no }}</span>
            <span v-else class="cell-sub">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="paid_at" label="打款时间" width="160">
          <template #default="{ row }">{{ formatDate(row.paid_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="申请时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button type="success" link size="small" @click="openPay(row as TenantWithdrawal)">打款</el-button>
              <el-button type="danger" link size="small" @click="openReject(row as TenantWithdrawal)">驳回</el-button>
            </template>
            <span v-else class="cell-sub">已处理</span>
          </template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 打款对话框 -->
    <el-dialog v-model="payVisible" title="确认打款" width="480px">
      <el-form ref="payFormRef" :model="payForm" label-position="top">
        <el-form-item label="代理">
          <el-input :model-value="currentRow?.agent_username" disabled />
        </el-form-item>
        <el-form-item label="提现金额">
          <el-input :model-value="'¥' + formatMoney(currentRow?.amount || 0)" disabled />
        </el-form-item>
        <el-form-item label="收款方式">
          <el-input :model-value="currentRow ? methodText(currentRow.pay_method) : ''" disabled />
        </el-form-item>
        <el-form-item label="收款账号">
          <el-input :model-value="currentRow?.pay_account" disabled />
        </el-form-item>
        <el-form-item label="打款流水号">
          <el-input v-model="payForm.pay_trade_no" maxlength="128" placeholder="可选，第三方支付流水号" />
        </el-form-item>
        <el-form-item label="审核备注">
          <el-input v-model="payForm.remark" type="textarea" :rows="3" maxlength="255" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="payVisible = false">取消</el-button>
        <el-button type="success" :loading="payLoading" @click="confirmPay">确认已打款</el-button>
      </template>
    </el-dialog>

    <!-- 驳回对话框 -->
    <el-dialog v-model="rejectVisible" title="审核驳回" width="480px">
      <el-form ref="rejectFormRef" :model="rejectForm" :rules="rejectRules" label-position="top">
        <el-form-item label="代理">
          <el-input :model-value="currentRow?.agent_username" disabled />
        </el-form-item>
        <el-form-item label="提现金额">
          <el-input :model-value="'¥' + formatMoney(currentRow?.amount || 0)" disabled />
        </el-form-item>
        <el-alert type="warning" :closable="false" show-icon>
          驳回后该笔金额将退回代理余额
        </el-alert>
        <el-form-item label="驳回原因" prop="reason" style="margin-top: 12px">
          <el-input v-model="rejectForm.reason" type="textarea" :rows="3" maxlength="255" show-word-limit placeholder="请填写驳回原因" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="rejectVisible = false">取消</el-button>
        <el-button type="danger" :loading="rejectLoading" @click="confirmReject">确认驳回</el-button>
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
  listTenantWithdrawalsApi,
  payTenantWithdrawalApi,
  rejectTenantWithdrawalApi,
  type TenantWithdrawal
} from '@/api/tenant'

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const formatMoney = (n: number) => {
  return (n || 0).toFixed(2)
}

const list = ref<TenantWithdrawal[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  status: 'pending',
  keyword: '',
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'id', label: 'ID' },
  { prop: 'agent_username', label: '代理' },
  { prop: 'amount', label: '金额', formatter: (v: number) => '¥' + (v || 0).toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) }
]

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listTenantWithdrawalsApi({
      status: filter.status,
      keyword: filter.keyword,
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

const methodText = (m: string) => {
  const map: Record<string, string> = { alipay: '支付宝', wechat: '微信', bank: '银行卡' }
  return map[m] || m || '-'
}

const statusTag = (s: string): any => {
  const map: Record<string, string> = { pending: 'warning', paid: 'success', rejected: 'danger', approved: 'primary', failed: 'info' }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = { pending: '待审核', paid: '已打款', rejected: '已驳回', approved: '审核通过', failed: '打款失败' }
  return map[s] || s
}

// ============== 打款 ==============
const payVisible = ref(false)
const payLoading = ref(false)
const payFormRef = ref<FormInstance>()
const currentRow = ref<TenantWithdrawal | null>(null)
const payForm = reactive({
  pay_trade_no: '',
  remark: ''
})

const openPay = (row: TenantWithdrawal) => {
  currentRow.value = row
  payForm.pay_trade_no = ''
  payForm.remark = ''
  payVisible.value = true
}

const confirmPay = async () => {
  if (!currentRow.value) return
  payLoading.value = true
  try {
    await payTenantWithdrawalApi(currentRow.value.id, {
      pay_trade_no: payForm.pay_trade_no,
      remark: payForm.remark
    })
    ElMessage.success('已标记为打款成功')
    payVisible.value = false
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    payLoading.value = false
  }
}

// ============== 驳回 ==============
const rejectVisible = ref(false)
const rejectLoading = ref(false)
const rejectFormRef = ref<FormInstance>()
const rejectForm = reactive({ reason: '' })
const rejectRules = {
  reason: [{ required: true, message: '请填写驳回原因', trigger: 'blur' }]
}

const openReject = (row: TenantWithdrawal) => {
  currentRow.value = row
  rejectForm.reason = ''
  rejectVisible.value = true
}

const confirmReject = async () => {
  if (!currentRow.value) return
  if (!rejectFormRef.value) return
  try {
    await rejectFormRef.value.validate()
  } catch {
    return
  }
  rejectLoading.value = true
  try {
    await rejectTenantWithdrawalApi(currentRow.value.id, { reason: rejectForm.reason })
    ElMessage.success('已驳回，金额已退回代理余额')
    rejectVisible.value = false
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    rejectLoading.value = false
  }
}

onMounted(loadList)
</script>

<style scoped lang="scss">
.withdrawal-review-page {
  padding: 0;
}
.app-card {
  background: var(--el-bg-color);
  border-radius: 8px;
  padding: 16px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}
.search-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 12px;
}
.cell-sub {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.text-danger {
  color: var(--el-color-danger);
  font-weight: 600;
}

// v0.5.0 响应式：统一走 mobile mixin（$bp-mobile=768px，max-width: 767px）
@include mobile {
  .app-card {
    padding: 8px;
  }
  .search-bar {
    flex-direction: column;
  }
  .search-bar .el-select,
  .search-bar .el-input,
  .search-bar .el-button {
    width: 100%;
  }
}
</style>
