<!--
  结算管理（超管）- 响应式 H5 - v0.3.4 升级
  - 双 Tab：
    1) 结算记录：列表 + 单条结算 + 批量结算（按 tenant_id 分组累计）
    2) 对账报表：聚合统计订单总额/抽成/应结/已结/未结/已提现/理论余额
  v0.2.3 已交付 list / settle API；v0.3.4 新增 batch_settle / reconciliation
-->
<template>
  <div class="settlements-page">
    <PageHeader title="结算管理" subtitle="管理所有开发者的订单结算与对账" />

    <div class="app-card">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <!-- ===== Tab 1: 结算记录 ===== -->
        <el-tab-pane label="结算记录" name="settlements">
          <div class="search-bar">
            <el-select v-model="filter.tenant_id" placeholder="按开发者筛选" clearable style="width: 200px" @change="onFilterChange">
              <el-option v-for="t in tenants" :key="t.id" :label="t.username" :value="t.id" />
            </el-select>
            <el-select v-model="filter.status" placeholder="按状态筛选" clearable style="width: 160px" @change="onFilterChange">
              <el-option label="待结算" value="pending" />
              <el-option label="已结算" value="settled" />
              <el-option label="已拒绝" value="rejected" />
            </el-select>
            <el-button @click="loadList">刷新</el-button>
            <el-button
              type="warning"
              :disabled="selectedRows.length === 0"
              @click="openBatchSettle"
            >
              批量结算（{{ selectedRows.length }}）
            </el-button>
            <span v-if="selectedSum > 0" class="sum-hint success">
              选中应结 ¥{{ formatMoney(selectedSum) }}
            </span>
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
            @selection-change="onSelectionChange"
          >
            <el-table-column type="selection" width="55" :selectable="canSelect" />
            <el-table-column prop="id" label="ID" width="80" />
            <el-table-column prop="tenant_id" label="开发者" width="120">
              <template #default="scope">{{ getTenantName(scope.row.tenant_id) }}</template>
            </el-table-column>
            <el-table-column prop="order_no" label="订单号" min-width="180" />
            <el-table-column prop="gross_amount" label="订单金额" width="120">
              <template #default="scope">¥{{ formatMoney(scope.row.gross_amount) }}</template>
            </el-table-column>
            <el-table-column prop="commission_rate" label="抽成比例" width="100">
              <template #default="scope">{{ scope.row.commission_rate }}%</template>
            </el-table-column>
            <el-table-column prop="commission_amount" label="平台抽成" width="120">
              <template #default="scope">¥{{ formatMoney(scope.row.commission_amount) }}</template>
            </el-table-column>
            <el-table-column prop="net_amount" label="开发者应得" width="120">
              <template #default="scope">
                <span class="text-success">¥{{ formatMoney(scope.row.net_amount) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="status" label="状态" width="100">
              <template #default="scope">
                <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="settle_batch_no" label="结算批次号" min-width="160">
              <template #default="scope">{{ scope.row.settle_batch_no || '-' }}</template>
            </el-table-column>
            <el-table-column prop="created_at" label="创建时间" width="160">
              <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="120" fixed="right">
              <template #default="scope">
                <el-button v-if="scope.row.status === 'pending'" type="primary" link size="small" @click="openSettle(scope.row)">单条结算</el-button>
                <span v-else class="text-secondary">-</span>
              </template>
            </el-table-column>
          </ResponsiveTable>
        </el-tab-pane>

        <!-- ===== Tab 2: 对账报表 ===== -->
        <el-tab-pane label="对账报表" name="reconciliation">
          <div class="search-bar">
            <el-select v-model="reconFilter.tenant_id" placeholder="按开发者筛选" clearable style="width: 200px" @change="loadReconciliation">
              <el-option v-for="t in tenants" :key="t.id" :label="t.username" :value="t.id" />
            </el-select>
            <el-date-picker
              v-model="reconDateRange"
              type="daterange"
              range-separator="至"
              start-placeholder="开始日期"
              end-placeholder="结束日期"
              value-format="YYYY-MM-DD"
              style="width: 260px"
              @change="loadReconciliation"
            />
            <el-button @click="loadReconciliation">刷新</el-button>
          </div>

          <div v-loading="reconLoading" class="recon-grid">
            <div class="recon-stat">
              <span class="recon-label">订单总数</span>
              <span class="recon-value">{{ recon.order_count }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">订单总额</span>
              <span class="recon-value">¥{{ formatMoney(recon.gross_total) }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">平台抽成</span>
              <span class="recon-value danger">¥{{ formatMoney(recon.commission_sum) }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">开发者应结</span>
              <span class="recon-value success">¥{{ formatMoney(recon.net_total) }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">已结算</span>
              <span class="recon-value success">¥{{ formatMoney(recon.settled_sum) }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">未结算</span>
              <span class="recon-value warning">¥{{ formatMoney(recon.pending_sum) }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">已提现</span>
              <span class="recon-value">¥{{ formatMoney(recon.withdrawn_sum) }}</span>
            </div>
            <div class="recon-stat">
              <span class="recon-label">待审核提现</span>
              <span class="recon-value warning">¥{{ formatMoney(recon.pending_withdraw_sum) }}</span>
            </div>
            <div class="recon-stat highlight">
              <span class="recon-label">理论账户余额</span>
              <span class="recon-value primary">¥{{ formatMoney(recon.balance_theory) }}</span>
              <span class="recon-hint">= 已结 - 已提</span>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 单条结算对话框 -->
    <el-dialog v-model="settleDialogVisible" title="手动结算（单条）" width="500px">
      <el-form label-position="top">
        <el-form-item label="订单号">
          <el-input :model-value="currentRow?.order_no" disabled />
        </el-form-item>
        <el-form-item label="开发者应得">
          <el-input :model-value="currentRow ? '¥' + currentRow.net_amount.toFixed(2) : ''" disabled />
        </el-form-item>
        <el-form-item label="结算方式">
          <el-select v-model="settleForm.method" placeholder="选择结算方式" style="width: 100%">
            <el-option label="手动转账" value="manual" />
            <el-option label="支付宝" value="alipay" />
            <el-option label="微信" value="wechat" />
            <el-option label="银行转账" value="bank" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="settleForm.remark" type="textarea" :rows="3" placeholder="可填写交易流水号" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="settleDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="settleLoading" @click="confirmSettle">确认结算</el-button>
      </template>
    </el-dialog>

    <!-- 批量结算对话框 -->
    <el-dialog v-model="batchDialogVisible" title="批量结算" width="560px">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px;">
        系统将按开发者分组累计应结金额，单次最多 100 条；结算后金额自动入账开发者可提现余额。
      </el-alert>
      <el-form label-position="top">
        <el-form-item label="本次结算记录数">
          <el-input :model-value="String(selectedRows.length)" disabled />
        </el-form-item>
        <el-form-item label="累计应结金额">
          <el-input :model-value="'¥' + formatMoney(selectedSum)" disabled />
        </el-form-item>
        <el-form-item label="涉及开发者数">
          <el-input :model-value="String(selectedTenantCount)" disabled />
        </el-form-item>
        <el-form-item label="结算方式">
          <el-select v-model="batchForm.method" placeholder="选择结算方式" style="width: 100%">
            <el-option label="手动转账" value="manual" />
            <el-option label="支付宝" value="alipay" />
            <el-option label="微信" value="wechat" />
            <el-option label="银行转账" value="bank" />
          </el-select>
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="batchForm.remark" type="textarea" :rows="3" placeholder="可填写批次说明" maxlength="255" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="batchDialogVisible = false">取消</el-button>
        <el-button type="warning" :loading="batchLoading" @click="confirmBatchSettle">确认批量结算</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { listSettlementsApi, settleOrderApi, type Settlement } from '@/api/pay'
import { batchSettleApi, reconciliationApi, type ReconciliationData } from '@/api/tenantFinance'
import { request } from '@/api/http'

const formatMoney = (n: number) => (Number(n) || 0).toFixed(2)

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

// ============== Tab 切换 ==============
const activeTab = ref<'settlements' | 'reconciliation'>('settlements')

const onTabChange = (name: string | number) => {
  if (name === 'settlements') {
    loadList()
  } else if (name === 'reconciliation') {
    loadReconciliation()
  }
}

// ============== Tab 1: 结算记录 ==============
const list = ref<Settlement[]>([])
const total = ref(0)
const loading = ref(false)
const tenants = ref<Array<{ id: number; username: string }>>([])

const filter = reactive({
  tenant_id: undefined as number | undefined,
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'gross_amount', label: '金额', formatter: (v: number) => '¥' + formatMoney(v) },
  { prop: 'net_amount', label: '应得', formatter: (v: number) => '¥' + formatMoney(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

// 多选
const selectedRows = ref<Settlement[]>([])

const onSelectionChange = (rows: Settlement[]) => {
  selectedRows.value = rows
}

// 仅 pending 状态可选
const canSelect = (row: Settlement) => row.status === 'pending'

const selectedSum = computed(() => {
  return selectedRows.value.reduce((acc, r) => acc + Number(r.net_amount || 0), 0)
})

const selectedTenantCount = computed(() => {
  const ids = new Set(selectedRows.value.map(r => r.tenant_id))
  return ids.size
})

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listSettlementsApi({
      tenant_id: filter.tenant_id,
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

const loadTenants = async () => {
  try {
    const resp = await request.get<{ list: Array<{ id: number; username: string }> }>('/admin/tenants', { page: 1, page_size: 100 })
    tenants.value = resp.list || []
  } catch {
    // 接口未实现时静默
  }
}

const getTenantName = (id: number) => {
  const t = tenants.value.find(x => x.id === id)
  return t ? t.username : `#${id}`
}

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    pending: 'warning',
    settled: 'success',
    rejected: 'info'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    pending: '待结算',
    settled: '已结算',
    rejected: '已拒绝'
  }
  return map[s] || s
}

const onFilterChange = () => {
  filter.page = 1
  loadList()
}

// ============== 单条结算 ==============
const settleDialogVisible = ref(false)
const settleLoading = ref(false)
const currentRow = ref<Settlement | null>(null)
const settleForm = reactive({
  method: 'manual' as 'manual' | 'alipay' | 'wechat' | 'bank',
  remark: ''
})

const openSettle = (row: any) => {
  currentRow.value = row
  settleForm.method = 'manual'
  settleForm.remark = ''
  settleDialogVisible.value = true
}

const confirmSettle = async () => {
  if (!currentRow.value) return
  settleLoading.value = true
  try {
    await settleOrderApi(currentRow.value.id, {
      method: settleForm.method,
      remark: settleForm.remark
    })
    ElMessage.success('结算成功')
    settleDialogVisible.value = false
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    settleLoading.value = false
  }
}

// ============== 批量结算 ==============
const batchDialogVisible = ref(false)
const batchLoading = ref(false)
const batchForm = reactive({
  method: 'manual' as 'manual' | 'alipay' | 'wechat' | 'bank',
  remark: ''
})

const openBatchSettle = () => {
  if (selectedRows.value.length === 0) {
    ElMessage.warning('请先勾选要结算的记录')
    return
  }
  if (selectedRows.value.length > 100) {
    ElMessage.warning('单次最多批量结算 100 条')
    return
  }
  batchForm.method = 'manual'
  batchForm.remark = ''
  batchDialogVisible.value = true
}

const confirmBatchSettle = async () => {
  if (selectedRows.value.length === 0) return
  batchLoading.value = true
  try {
    const ids = selectedRows.value.map(r => r.id)
    const resp = await batchSettleApi({
      settlement_ids: ids,
      method: batchForm.method,
      remark: batchForm.remark
    })
    ElMessage.success(`批量结算成功，批次号 ${resp.batch_no}，已结算 ${resp.success_count} 条记录，涉及 ${resp.tenant_count} 个开发者`)
    batchDialogVisible.value = false
    selectedRows.value = []
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    batchLoading.value = false
  }
}

// ============== Tab 2: 对账报表 ==============
const reconLoading = ref(false)
const reconDateRange = ref<[string, string] | null>(null)
const reconFilter = reactive({
  tenant_id: undefined as number | undefined,
  start_date: undefined as string | undefined,
  end_date: undefined as string | undefined
})

const recon = ref<ReconciliationData>({
  start_date: '',
  end_date: '',
  order_count: 0,
  gross_total: 0,
  commission_sum: 0,
  net_total: 0,
  settled_sum: 0,
  pending_sum: 0,
  withdrawn_sum: 0,
  pending_withdraw_sum: 0,
  balance_theory: 0
})

const loadReconciliation = async () => {
  reconLoading.value = true
  try {
    if (reconDateRange.value && reconDateRange.value.length === 2) {
      reconFilter.start_date = reconDateRange.value[0]
      reconFilter.end_date = reconDateRange.value[1]
    } else {
      reconFilter.start_date = undefined
      reconFilter.end_date = undefined
    }
    const data = await reconciliationApi({
      tenant_id: reconFilter.tenant_id,
      start_date: reconFilter.start_date,
      end_date: reconFilter.end_date
    })
    Object.assign(recon.value, data)
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    reconLoading.value = false
  }
}

onMounted(() => {
  loadTenants()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.settlements-page {
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
    &.success { color: $color-success; }
    &.warning { color: $color-warning; }
  }

  .text-success { color: $color-success; font-weight: 500; }
  .text-secondary { color: $color-text-secondary; font-size: 13px; }
}

.recon-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: $spacing-md;

  @include mobile {
    grid-template-columns: repeat(2, 1fr);
    gap: $spacing-sm;
  }
}

.recon-stat {
  background: $color-bg-card;
  border: 1px solid $color-border;
  border-radius: $radius-md;
  padding: $spacing-md;
  display: flex;
  flex-direction: column;
  gap: $spacing-xs;

  &.highlight {
    border-color: $color-primary;
    background: linear-gradient(135deg, $color-primary-light, $color-bg-card);
  }

  .recon-label {
    font-size: 13px;
    color: $color-text-secondary;
  }
  .recon-value {
    font-size: 22px;
    font-weight: 700;
    color: $color-text-primary;
    font-family: 'SF Mono', 'Menlo', monospace;

    &.success { color: $color-success; }
    &.warning { color: $color-warning; }
    &.danger { color: $color-danger; }
    &.primary { color: $color-primary; }
  }
  .recon-hint {
    font-size: 11px;
    color: $color-text-secondary;
  }

  @include mobile {
    padding: $spacing-sm;
    .recon-value { font-size: 16px; }
    .recon-label { font-size: 11px; }
  }
}
</style>
