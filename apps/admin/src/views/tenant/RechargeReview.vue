<!--
  代理充值审核（开发者）- 响应式 H5
  - 列表：代理用户名 / 金额 / 付款方式 / 凭证 / 备注 / 状态 / 时间
  - 操作：通过（可调整实际到账金额）/ 驳回（必填原因）
  v0.3.2 已交付 list / approve / reject API。
-->
<template>
  <div class="recharge-review-page">
    <PageHeader title="充值审核" subtitle="审核代理提交的充值申请" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="onFilterChange">
          <el-option label="待审核" value="pending" />
          <el-option label="已通过" value="settled" />
          <el-option label="已驳回" value="rejected" />
        </el-select>
        <el-input
          v-model="filter.keyword"
          placeholder="用户名/凭证/备注"
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
        <el-table-column prop="amount" label="申请金额" width="120">
          <template #default="{ row }">
            <span class="text-primary">¥{{ formatMoney(row.amount) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="pay_method" label="付款方式" width="120">
          <template #default="{ row }">{{ methodText(row.pay_method) }}</template>
        </el-table-column>
        <el-table-column prop="pay_voucher" label="付款凭证" min-width="160">
          <template #default="{ row }">
            <span v-if="row.pay_voucher">{{ row.pay_voucher }}</span>
            <span v-else class="cell-sub">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="160">
          <template #default="{ row }">
            <span v-if="row.remark">{{ row.remark }}</span>
            <span v-else class="cell-sub">-</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="提交时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <template v-if="row.status === 'pending'">
              <el-button type="primary" link size="small" @click="openApprove(row as TenantRechargeRequest)">通过</el-button>
              <el-button type="danger" link size="small" @click="openReject(row as TenantRechargeRequest)">驳回</el-button>
            </template>
            <span v-else class="cell-sub">已处理</span>
          </template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 通过对话框 -->
    <el-dialog v-model="approveVisible" title="审核通过" width="480px">
      <el-form ref="approveFormRef" :model="approveForm" label-position="top">
        <el-form-item label="代理">
          <el-input :model-value="currentRow?.agent_username" disabled />
        </el-form-item>
        <el-form-item label="申请金额">
          <el-input :model-value="'¥' + formatMoney(currentRow?.amount || 0)" disabled />
        </el-form-item>
        <el-form-item label="实际到账金额">
          <el-input-number
            v-model="approveForm.actual_amount"
            :min="0.01"
            :precision="2"
            :step="100"
            style="width: 100%"
            placeholder="缺省按申请金额"
          />
          <div class="form-hint">若实际到账与申请金额不同，请填写实际金额</div>
        </el-form-item>
        <el-form-item label="审核备注">
          <el-input v-model="approveForm.remark" type="textarea" :rows="3" maxlength="255" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="approveVisible = false">取消</el-button>
        <el-button type="primary" :loading="approveLoading" @click="confirmApprove">确认通过</el-button>
      </template>
    </el-dialog>

    <!-- 驳回对话框 -->
    <el-dialog v-model="rejectVisible" title="审核驳回" width="480px">
      <el-form ref="rejectFormRef" :model="rejectForm" :rules="rejectRules" label-position="top">
        <el-form-item label="代理">
          <el-input :model-value="currentRow?.agent_username" disabled />
        </el-form-item>
        <el-form-item label="申请金额">
          <el-input :model-value="'¥' + formatMoney(currentRow?.amount || 0)" disabled />
        </el-form-item>
        <el-form-item label="驳回原因" prop="reason">
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
  listTenantRechargeRequestsApi,
  approveTenantRechargeApi,
  rejectTenantRechargeApi,
  type TenantRechargeRequest
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

const list = ref<TenantRechargeRequest[]>([])
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
    const resp = await listTenantRechargeRequestsApi({
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
  const map: Record<string, string> = { alipay: '支付宝', wechat: '微信', bank: '银行转账', manual: '人工' }
  return map[m] || m || '-'
}

const statusTag = (s: string): any => {
  const map: Record<string, string> = { pending: 'warning', settled: 'success', rejected: 'danger' }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = { pending: '待审核', settled: '已通过', rejected: '已驳回' }
  return map[s] || s
}

// ============== 通过 ==============
const approveVisible = ref(false)
const approveLoading = ref(false)
const approveFormRef = ref<FormInstance>()
const currentRow = ref<TenantRechargeRequest | null>(null)
const approveForm = reactive({
  actual_amount: undefined as number | undefined,
  remark: ''
})

const openApprove = (row: TenantRechargeRequest) => {
  currentRow.value = row
  approveForm.actual_amount = row.amount
  approveForm.remark = ''
  approveVisible.value = true
}

const confirmApprove = async () => {
  if (!currentRow.value) return
  approveLoading.value = true
  try {
    await approveTenantRechargeApi(currentRow.value.id, {
      actual_amount: approveForm.actual_amount,
      remark: approveForm.remark
    })
    ElMessage.success('已通过审核，代理余额已到账')
    approveVisible.value = false
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    approveLoading.value = false
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

const openReject = (row: TenantRechargeRequest) => {
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
    await rejectTenantRechargeApi(currentRow.value.id, { reason: rejectForm.reason })
    ElMessage.success('已驳回该充值申请')
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

<style scoped>
.recharge-review-page {
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
.text-primary {
  color: var(--el-color-primary);
  font-weight: 600;
}
.form-hint {
  color: var(--el-text-color-secondary);
  font-size: 12px;
  margin-top: 4px;
}

@media (max-width: 768px) {
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
