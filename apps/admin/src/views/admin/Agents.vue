<!--
  代理管理（超管）- 响应式
  - 列表展示平台所有代理
  - 编辑代理状态/佣金模式/佣金比例/余额
  - 注：后端 /admin/agents 当前 501，列表/统计保持空状态（铁律 04/06）
-->
<template>
  <div class="admin-agents-page">
    <PageHeader title="代理管理" subtitle="管理平台所有代理账号与佣金配置" />

    <div class="app-card">
      <div class="search-bar">
        <el-input v-model="filter.keyword" placeholder="用户名/真实姓名/手机" clearable style="width: 220px" @change="loadList" />
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="正常" value="active" />
          <el-option label="已禁用" value="disabled" />
          <el-option label="待审核" value="pending" />
        </el-select>
        <el-select v-model="filter.tenant_id" placeholder="所属开发者" clearable filterable style="width: 200px" @change="loadList">
          <el-option v-for="t in tenants" :key="t.id" :label="t.username" :value="t.id" />
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
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="real_name" label="真实姓名" min-width="120">
          <template #default="scope">{{ scope.row.real_name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="phone" label="手机" width="130">
          <template #default="scope">{{ scope.row.phone || '-' }}</template>
        </el-table-column>
        <el-table-column prop="tenant_name" label="所属开发者" min-width="120">
          <template #default="scope">{{ scope.row.tenant_name || ('#' + scope.row.tenant_id) }}</template>
        </el-table-column>
        <el-table-column prop="balance" label="余额" width="110">
          <template #default="scope">¥{{ Number(scope.row.balance || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="frozen_balance" label="冻结" width="110">
          <template #default="scope">¥{{ Number(scope.row.frozen_balance || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="total_commission" label="累计佣金" width="120">
          <template #default="scope">¥{{ Number(scope.row.total_commission || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="total_withdraw" label="累计提现" width="120">
          <template #default="scope">¥{{ Number(scope.row.total_withdraw || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="commission_mode" label="佣金模式" width="100">
          <template #default="scope">{{ commissionModeText(scope.row.commission_mode) }}</template>
        </el-table-column>
        <el-table-column prop="commission_rate" label="佣金比例" width="100">
          <template #default="scope">{{ scope.row.commission_rate }}%</template>
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
            <el-button type="primary" link size="small" @click="openEdit(scope.row)">编辑</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 编辑对话框 -->
    <el-dialog v-model="dialogVisible" title="编辑代理" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="正常" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="待审核" value="pending" />
          </el-select>
        </el-form-item>
        <el-form-item label="佣金模式" prop="commission_mode">
          <el-select v-model="form.commission_mode" style="width: 100%">
            <el-option label="比例分成" value="percentage" />
            <el-option label="差价模式" value="diff" />
          </el-select>
        </el-form-item>
        <el-form-item label="佣金比例（%）" prop="commission_rate">
          <el-input-number v-model="form.commission_rate" :min="0" :max="100" :precision="2" :step="1" />
          <span class="hint">仅比例分成模式生效</span>
        </el-form-item>
        <el-form-item label="余额（元）" prop="balance">
          <el-input-number v-model="form.balance" :min="0" :precision="2" :step="100" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="confirmSubmit">确认</el-button>
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
  listAdminAgentsApi, updateAdminAgentApi,
  listAdminTenantsApi,
  type AdminAgent, type AdminTenant
} from '@/api/admin'

const list = ref<AdminAgent[]>([])
const total = ref(0)
const loading = ref(false)
const tenants = ref<Array<Pick<AdminTenant, 'id' | 'username'>>>([])

const filter = reactive({
  keyword: '',
  status: undefined as string | undefined,
  tenant_id: undefined as number | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'username', label: '用户名' },
  { prop: 'tenant_name', label: '开发者', formatter: (v: string, row: any) => v || ('#' + row.tenant_id) },
  { prop: 'balance', label: '余额', formatter: (v: number) => '¥' + Number(v || 0).toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) }
]

const dialogVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  status: 'active' as 'active' | 'disabled' | 'pending',
  commission_mode: 'percentage' as 'percentage' | 'diff',
  commission_rate: 0,
  balance: 0
})

const rules = {
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
  commission_mode: [{ required: true, message: '请选择佣金模式', trigger: 'change' }],
  commission_rate: [{ required: true, message: '请输入佣金比例', trigger: 'blur' }],
  balance: [{ required: true, message: '请输入余额', trigger: 'blur' }]
}

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    active: 'success',
    disabled: 'danger',
    pending: 'warning'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    active: '正常',
    disabled: '已禁用',
    pending: '待审核'
  }
  return map[s] || s
}

const commissionModeText = (m: string) => {
  const map: Record<string, string> = {
    percentage: '比例分成',
    diff: '差价模式'
  }
  return map[m] || m
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadTenants = async () => {
  try {
    const resp = await listAdminTenantsApi({ page: 1, page_size: 200 })
    tenants.value = (resp.list || []).map(t => ({ id: t.id, username: t.username }))
  } catch {
    // 错误已由 http 拦截器处理
  }
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAdminAgentsApi({
      keyword: filter.keyword,
      status: filter.status,
      tenant_id: filter.tenant_id,
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

const openEdit = (row: any) => {
  editingId.value = row.id
  Object.assign(form, {
    status: row.status || 'active',
    commission_mode: row.commission_mode || 'percentage',
    commission_rate: row.commission_rate ?? 0,
    balance: row.balance ?? 0
  })
  dialogVisible.value = true
}

const confirmSubmit = async () => {
  if (!formRef.value) return
  const id = editingId.value
  if (!id) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      await updateAdminAgentApi(id, {
        status: form.status,
        commission_mode: form.commission_mode,
        commission_rate: form.commission_rate,
        balance: form.balance
      })
      ElMessage.success('已更新')
      dialogVisible.value = false
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      submitLoading.value = false
    }
  })
}

onMounted(() => {
  loadTenants()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.admin-agents-page {
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
}
</style>
