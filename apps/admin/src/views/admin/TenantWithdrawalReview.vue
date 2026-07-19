<!--
  开发者提现审核（超管）- 响应式 H5 - v0.3.4 新增
  - 列表：开发者用户名 / 公司名 / 提现金额 / 收款方式 / 收款账号 / 状态 / 时间
  - 操作：标记已打款（可填写打款流水号）/ 驳回（退回余额，必填原因）
-->
<template>
  <div class="tenant-withdrawal-review-page">
    <PageHeader title="开发者提现审核" subtitle="审核开发者提现申请并标记打款" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="onFilterChange">
          <el-option label="待审核" value="pending" />
          <el-option label="已打款" value="paid" />
          <el-option label="已驳回" value="rejected" />
          <el-option label="打款失败" value="failed" />
        </el-select>
        <el-input
          v-model="filter.keyword"
          placeholder="开发者用户名/收款账号"
          clearable
          style="width: 220px"
          @keyup.enter="onFilterChange"
        />
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 260px"
          @change="onFilterChange"
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
        <el-table-column prop="tenant_username" label="开发者" min-width="140">
          <template #default="{ row }">
            <div>{{ row.tenant_username || '-' }}</div>
            <div v-if="row.tenant_company" class="cell-sub">{{ row.tenant_company }}</div>
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
              <el-button type="success" link size="small" @click="openPay(row as AdminTenantWithdrawal)">打款</el-button>
              <el-button type="danger" link size="small" @click="openReject(row as AdminTenantWithdrawal)">驳回</el-button>
            </template>
            <span v-else class="cell-sub">已处理</span>
          </template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 打款对话框 -->
    <el-dialog v-model="payVisible" title="确认打款" width="480px">
      <el-form ref="payFormRef" :model="payForm" label-position="top">
        <el-form-item label="开发者">
          <el-input :model-value="currentRow?.tenant_username" disabled />
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
        <el-form-item label="开发者">
          <el-input :model-value="currentRow?.tenant_username" disabled />
        </el-form-item>
        <el-form-item label="提现金额">
          <el-input :model-value="'¥' + formatMoney(currentRow?.amount || 0)" disabled />
        </el-form-item>
        <el-alert type="warning" :closable="false" show-icon>
          驳回后该笔金额将退回开发者可提现余额
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
  listAdminTenantWithdrawalsApi,
  payAdminTenantWithdrawalApi,
  rejectAdminTenantWithdrawalApi,
  type AdminTenantWithdrawal
} from '@/api/tenantFinance'

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
}

const formatMoney = (n: number) => (Number(n) || 0).toFixed(2)

const list = ref<AdminTenantWithdrawal[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  status: 'pending',
  keyword: '',
  start_date: undefined as string | undefined,
  end_date: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const dateRange = ref<[string, string] | null>(null)

const mobileFields = [
  { prop: 'id', label: 'ID' },
  { prop: 'tenant_username', label: '开发者' },
  { prop: 'amount', label: '金额', formatter: (v: number) => '¥' + formatMoney(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) }
]

const loadList = async () => {
  loading.value = true
  try {
    if (dateRange.value && dateRange.value.length === 2) {
      filter.start_date = dateRange.value[0]
      filter.end_date = dateRange.value[1]
    } else {
      filter.start_date = undefined
      filter.end_date = undefined
    }
    const resp = await listAdminTenantWithdrawalsApi({
      status: filter.status,
      keyword: filter.keyword || undefined,
      start_date: filter.start_date,
      end_date: filter.end_date,
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
  const map: Record<string, string> = { pending: 'warning', paid: 'success', rejected: 'danger', failed: 'info' }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = { pending: '待审核', paid: '已打款', rejected: '已驳回', failed: '打款失败' }
  return map[s] || s
}

// ============== 打款 ==============
const payVisible = ref(false)
const payLoading = ref(false)
const payFormRef = ref<FormInstance>()
const currentRow = ref<AdminTenantWithdrawal | null>(null)
const payForm = reactive({
  pay_trade_no: '',
  remark: ''
})

const openPay = (row: AdminTenantWithdrawal) => {
  currentRow.value = row
  payForm.pay_trade_no = ''
  payForm.remark = ''
  payVisible.value = true
}

const confirmPay = async () => {
  if (!currentRow.value) return
  payLoading.value = true
  try {
    await payAdminTenantWithdrawalApi(currentRow.value.id, {
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

const openReject = (row: AdminTenantWithdrawal) => {
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
    await rejectAdminTenantWithdrawalApi(currentRow.value.id, { reason: rejectForm.reason })
    ElMessage.success('已驳回，金额已退回开发者余额')
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
@use '@/styles/variables.scss' as *;

.tenant-withdrawal-review-page {
  .app-card {
    background: $color-bg-card;
    border-radius: $radius-md;
    padding: $spacing-md;
    box-shadow: $shadow-card;
  }

  .search-bar {
    display: flex;
    flex-wrap: wrap;
    gap: $spacing-sm;
    margin-bottom: $spacing-md;
    align-items: center;

    @include mobile {
      flex-direction: column;
      .el-select, .el-input, .el-button, .el-date-editor { width: 100% !important; }
    }
  }

  .cell-sub {
    color: $color-text-secondary;
    font-size: 12px;
  }
  .text-danger {
    color: $color-danger;
    font-weight: 600;
  }
}
</style>
