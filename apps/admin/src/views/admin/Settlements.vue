<!--
  结算管理（超管）- 响应式
  - 列表展示所有结算记录
  - 手动结算操作
-->
<template>
  <div class="settlements-page">
    <PageHeader title="结算管理" subtitle="管理所有开发者的订单结算" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.tenant_id" placeholder="按开发者筛选" clearable style="width: 200px" @change="loadList">
          <el-option v-for="t in tenants" :key="t.id" :label="t.username" :value="t.id" />
        </el-select>
        <el-select v-model="filter.status" placeholder="按状态筛选" clearable style="width: 160px" @change="loadList">
          <el-option label="待结算" value="pending" />
          <el-option label="已结算" value="settled" />
          <el-option label="已拒绝" value="rejected" />
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
        <el-table-column prop="tenant_id" label="开发者" width="120">
          <template #default="scope">{{ getTenantName(scope.row.tenant_id) }}</template>
        </el-table-column>
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column prop="gross_amount" label="订单金额" width="120">
          <template #default="scope">¥{{ Number(scope.row.gross_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="commission_rate" label="抽成比例" width="100">
          <template #default="scope">{{ scope.row.commission_rate }}%</template>
        </el-table-column>
        <el-table-column prop="commission_amount" label="平台抽成" width="120">
          <template #default="scope">¥{{ Number(scope.row.commission_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="net_amount" label="开发者应得" width="120">
          <template #default="scope">¥{{ Number(scope.row.net_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="scope">
            <el-button v-if="scope.row.status === 'pending'" type="primary" link size="small" @click="openSettle(scope.row)">结算</el-button>
            <span v-else class="text-secondary">-</span>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button v-if="item.status === 'pending'" type="primary" size="small" @click="openSettle(item)">结算</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 结算对话框 -->
    <el-dialog v-model="settleDialogVisible" title="手动结算" width="500px">
      <el-form label-position="top">
        <el-form-item label="订单号">
          <el-input :model-value="currentRow?.order_no" disabled />
        </el-form-item>
        <el-form-item label="开发者应得">
          <el-input :model-value="currentRow ? '¥' + currentRow.net_amount.toFixed(2) : ''" disabled />
        </el-form-item>
        <el-form-item label="结算方式">
          <el-select v-model="settleForm.method" placeholder="选择结算方式">
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { listSettlementsApi, settleOrderApi, type Settlement } from '@/api/pay'
import { request } from '@/api/http'

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
  { prop: 'gross_amount', label: '金额', formatter: (v: number) => '¥' + v.toFixed(2) },
  { prop: 'net_amount', label: '应得', formatter: (v: number) => '¥' + v.toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const settleDialogVisible = ref(false)
const settleLoading = ref(false)
const currentRow = ref<Settlement | null>(null)
const settleForm = reactive({
  method: 'manual' as 'manual' | 'alipay' | 'wechat' | 'bank',
  remark: ''
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
    // 待核实：当前后端未提供完整的租户列表 API，先调用现有接口
    // 实际应该用 AdminListTenants 接口
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

const formatDate = (s: string) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

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

onMounted(() => {
  loadTenants()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.settlements-page {
  .text-secondary { color: $color-text-secondary; font-size: 13px; }
}
</style>
