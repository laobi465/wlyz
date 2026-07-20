<!--
  邀请码（开发者）- 响应式
  代理通过邀请码注册，注册费从开发者余额扣款或由代理自付。
  铁律 06 待核实：后端 /tenant/invite_codes、/tenant/agents/invite_codes 当前为 501 占位（v0.3.0 交付），调用失败时静默降级。
-->
<template>
  <div class="invite-codes-page">
    <PageHeader title="邀请码" subtitle="代理注册邀请码生成与管理">
      <template #actions>
        <el-button type="primary" @click="openGenerate">生成邀请码</el-button>
      </template>
    </PageHeader>

    <el-alert
      type="info"
      :closable="false"
      show-icon
      title="代理通过邀请码注册，注册费从开发者余额扣款或由代理自付"
      style="margin-bottom: 16px"
    />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="可用" value="active" />
          <el-option label="已用尽" value="exhausted" />
          <el-option label="已过期" value="expired" />
          <el-option label="已禁用" value="disabled" />
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
        <el-table-column prop="code" label="邀请码" min-width="200">
          <template #default="{ row }">
            <span class="mono">{{ row.code }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="used_by_username" label="使用人" width="120">
          <template #default="{ row }">{{ row.used_by_username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="used_at" label="使用时间" width="160">
          <template #default="{ row }">{{ formatDate(row.used_at) }}</template>
        </el-table-column>
        <el-table-column prop="expire_at" label="过期时间" width="160">
          <template #default="{ row }">{{ formatDate(row.expire_at) }}</template>
        </el-table-column>
        <el-table-column prop="remark" label="备注" min-width="120">
          <template #default="{ row }">{{ row.remark || '-' }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button v-if="row.status === 'active'" type="danger" link size="small" @click="disable(row)">禁用</el-button>
            <el-button type="primary" link size="small" @click="copy(row.code)">复制</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button v-if="item.status === 'active'" type="danger" size="small" @click="disable(item)">禁用</el-button>
          <el-button type="primary" size="small" @click="copy(item.code)">复制</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <el-dialog v-model="generateVisible" title="生成邀请码" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="生成数量" prop="count">
          <el-input-number v-model="form.count" :min="1" :max="100" />
          <span class="hint">最多 100 个/次</span>
        </el-form-item>
        <el-form-item label="有效天数" prop="expire_days">
          <el-input-number v-model="form.expire_days" :min="1" :max="365" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="2" placeholder="可选" maxlength="128" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="generateVisible = false">取消</el-button>
        <el-button type="primary" :loading="generateLoading" @click="confirmGenerate">生成</el-button>
      </template>
    </el-dialog>

    <!-- 生成结果 -->
    <el-dialog v-model="resultVisible" title="生成成功" width="500px">
      <el-alert type="success" :closable="false" show-icon>
        共生成 {{ generatedCodes.length }} 个邀请码
      </el-alert>
      <div class="codes-list">
        <div v-for="(c, idx) in generatedCodes" :key="idx" class="code-row">
          <span class="code-text mono">{{ c.code }}</span>
          <el-button text size="small" @click="copy(c.code)">复制</el-button>
        </div>
      </div>
      <div class="result-actions">
        <el-button @click="copyAll">复制全部</el-button>
        <el-button type="primary" @click="resultVisible = false">完成</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listTenantInviteCodesApi, genTenantInviteCodeApi, disableTenantInviteCodeApi,
  type TenantInviteCode
} from '@/api/tenant'

const list = ref<TenantInviteCode[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'code', label: '邀请码' },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'used_by_username', label: '使用人', formatter: (v: string) => v || '-' },
  { prop: 'expire_at', label: '过期时间', formatter: (v: string) => formatDate(v) }
]

const generateVisible = ref(false)
const generateLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  count: 1,
  expire_days: 30,
  remark: ''
})

const rules = {
  count: [{ required: true, message: '请输入数量', trigger: 'blur' }],
  expire_days: [{ required: true, message: '请输入有效天数', trigger: 'blur' }]
}

const resultVisible = ref(false)
const generatedCodes = ref<TenantInviteCode[]>([])

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    active: 'success',
    exhausted: 'info',
    expired: 'info',
    disabled: 'danger'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    active: '可用',
    exhausted: '已用尽',
    expired: '已过期',
    disabled: '已禁用'
  }
  return map[s] || s
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listTenantInviteCodesApi({
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

const openGenerate = () => {
  Object.assign(form, { count: 1, expire_days: 30, remark: '' })
  generateVisible.value = true
}

const confirmGenerate = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    generateLoading.value = true
    try {
      const resp = await genTenantInviteCodeApi({
        count: form.count,
        expire_days: form.expire_days,
        remark: form.remark
      })
      generatedCodes.value = resp.codes || []
      generateVisible.value = false
      resultVisible.value = true
      ElMessage.success(`成功生成 ${generatedCodes.value.length} 个邀请码`)
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      generateLoading.value = false
    }
  })
}

const disable = (row: any) => {
  ElMessageBox.confirm('确定要禁用此邀请码吗？禁用后不可恢复', '禁用邀请码', {
    type: 'warning',
    confirmButtonText: '确认禁用',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await disableTenantInviteCodeApi(row.id)
      ElMessage.success('已禁用')
      loadList()
    } catch {}
  }).catch(() => {})
}

const copy = (text: string) => {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

const copyAll = () => {
  const text = generatedCodes.value.map(c => c.code).join('\n')
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制全部')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.invite-codes-page {
  .mono {
    font-family: monospace;
    font-size: 13px;
    color: $color-text-primary;
  }
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
  .codes-list {
    max-height: 360px;
    overflow-y: auto;
    margin-top: $spacing-md;
    .code-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: $spacing-sm $spacing-md;
      background: $color-primary-light;
      border-radius: $radius-sm;
      margin-bottom: $spacing-sm;
      .code-text {
        font-family: monospace;
        font-size: 13px;
        color: $color-text-primary;
        word-break: break-all;
        flex: 1;
        margin-right: $spacing-sm;
      }
    }
  }
  .result-actions {
    display: flex;
    justify-content: flex-end;
    gap: $spacing-sm;
    margin-top: $spacing-md;
  }
}
</style>
