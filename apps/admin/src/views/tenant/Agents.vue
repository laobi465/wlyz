<!--
  代理管理（开发者）- 响应式
  铁律 06 待核实：后端 /tenant/agents 当前为 501 占位（v0.3.0 交付），调用失败时静默降级为空列表。
  铁律 04：顶部 4 项聚合数据后端暂不返回，显示 0，不编造，待 v0.3.0 补全。
-->
<template>
  <div class="agents-page">
    <PageHeader title="代理管理" subtitle="开发者下属代理与佣金配置" />

    <!-- 顶部数据卡：聚合数据后端暂不返回，显示 0（铁律 04），待 v0.3.0 补全 -->
    <div class="stat-grid">
      <div class="stat-card">
        <div class="stat-info">
          <div class="stat-label">代理总数</div>
          <div class="stat-value">{{ agentTotal }}</div>
          <div class="stat-extra">活跃 {{ activeTotal }}</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-info">
          <div class="stat-label">活跃代理</div>
          <div class="stat-value">{{ activeTotal }}</div>
          <div class="stat-extra">-</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-info">
          <div class="stat-label">累计佣金</div>
          <div class="stat-value">¥{{ totalCommission.toFixed(2) }}</div>
          <div class="stat-extra">-</div>
        </div>
      </div>
      <div class="stat-card">
        <div class="stat-info">
          <div class="stat-label">累计提现</div>
          <div class="stat-value">¥{{ totalWithdraw.toFixed(2) }}</div>
          <div class="stat-extra">-</div>
        </div>
      </div>
    </div>

    <div class="app-card">
      <div class="search-bar">
        <el-input v-model="filter.keyword" placeholder="用户名/真实姓名/手机" clearable style="width: 220px" @change="loadList" />
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="正常" value="active" />
          <el-option label="已禁用" value="disabled" />
          <el-option label="待审核" value="pending" />
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
        <el-table-column prop="real_name" label="真实姓名" min-width="100" />
        <el-table-column prop="phone" label="手机" width="130" />
        <el-table-column prop="balance" label="余额" width="100">
          <template #default="{ row }">
            <span>¥{{ Number(row.balance).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="frozen_balance" label="冻结" width="100">
          <template #default="{ row }">
            <span>¥{{ Number(row.frozen_balance).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="total_commission" label="累计佣金" width="110">
          <template #default="{ row }">
            <span>¥{{ Number(row.total_commission).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="total_withdraw" label="累计提现" width="110">
          <template #default="{ row }">
            <span>¥{{ Number(row.total_withdraw).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="commission_mode" label="佣金模式" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="row.commission_mode === 'percentage' ? 'primary' : 'warning'">
              {{ commissionModeText(row.commission_mode) }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="commission_rate" label="佣金比例" width="100">
          <template #default="{ row }">
            {{ row.commission_mode === 'percentage' ? (row.commission_rate + '%') : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="last_active_at" label="最近活跃" width="160">
          <template #default="{ row }">{{ formatDate(row.last_active_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="90" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openEdit(row)">编辑</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <el-dialog v-model="dialogVisible" title="编辑代理" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" placeholder="选择状态">
            <el-option label="正常" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="待审核" value="pending" />
          </el-select>
        </el-form-item>
        <el-form-item label="佣金模式" prop="commission_mode">
          <el-radio-group v-model="form.commission_mode">
            <el-radio value="percentage">按比例</el-radio>
            <el-radio value="diff">按差价</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.commission_mode === 'percentage'" label="佣金比例（%）" prop="commission_rate">
          <el-input-number v-model="form.commission_rate" :min="0" :max="100" :precision="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submit">保存</el-button>
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
  listTenantAgentsApi, updateTenantAgentApi, type TenantAgent
} from '@/api/tenant'

const list = ref<TenantAgent[]>([])
const total = ref(0)
const loading = ref(false)

// 顶部聚合数据：后端暂不返回，显示 0（铁律 04 不编造），待 v0.3.0 补全
const agentTotal = ref(0)
const activeTotal = ref(0)
const totalCommission = ref(0)
const totalWithdraw = ref(0)

const filter = reactive({
  keyword: '',
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'username', label: '用户名' },
  { prop: 'real_name', label: '真实姓名' },
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
  commission_rate: 0
})

const rules = {
  status: [{ required: true, message: '请选择状态', trigger: 'change' }],
  commission_mode: [{ required: true, message: '请选择佣金模式', trigger: 'change' }]
}

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    active: 'success',
    pending: 'warning',
    disabled: 'danger'
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
  return m === 'percentage' ? '按比例' : (m === 'diff' ? '按差价' : m)
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listTenantAgentsApi({
      keyword: filter.keyword,
      status: filter.status,
      page: filter.page,
      page_size: filter.page_size
    })
    list.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 后端 501 占位时静默降级（铁律 06），不编造数据
  } finally {
    loading.value = false
  }
}

const openEdit = (row: any) => {
  editingId.value = row.id
  Object.assign(form, {
    status: row.status,
    commission_mode: row.commission_mode,
    commission_rate: Number(row.commission_rate) || 0
  })
  dialogVisible.value = true
}

const submit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (!editingId.value) return
    submitLoading.value = true
    try {
      await updateTenantAgentApi(editingId.value, {
        status: form.status,
        commission_mode: form.commission_mode,
        commission_rate: form.commission_rate
      })
      ElMessage.success('保存成功')
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
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.agents-page {
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

    .stat-card {
      background: $color-bg-card;
      border-radius: $radius-md;
      padding: $spacing-md $spacing-lg;
      box-shadow: $shadow-card;
      border-left: 4px solid $color-primary;

      .stat-info {
        .stat-label {
          font-size: 12px;
          color: $color-text-secondary;
        }
        .stat-value {
          font-size: 22px;
          font-weight: 600;
          color: $color-text-primary;
          font-family: 'SF Mono', 'Menlo', monospace;
          line-height: 1.4;
        }
        .stat-extra {
          font-size: 12px;
          color: $color-text-secondary;
          margin-top: 2px;
        }
      }

      @include mobile {
        padding: $spacing-sm $spacing-md;
        .stat-info .stat-value { font-size: 16px; }
      }
    }
  }
}
</style>
