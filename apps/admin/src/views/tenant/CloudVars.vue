<!--
  云变量（开发者）- 响应式
-->
<template>
  <div class="cloud-vars-page">
    <PageHeader title="云变量" subtitle="应用级云变量配置">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建变量</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.app_id" placeholder="应用" clearable style="width: 160px" @change="loadList">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-input v-model="filter.keyword" placeholder="键/描述" clearable style="width: 200px" @change="loadList" />
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
        <el-table-column prop="app_name" label="应用" min-width="120" />
        <el-table-column prop="key" label="键" min-width="140">
          <template #default="{ row }: any">
            <span class="mono">{{ row.key }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="value" label="值" min-width="180">
          <template #default="{ row }: any">
            <el-button text size="small" @click="showValue(row)">
              <span class="mono">{{ truncate(row.value, 30) }}</span>
            </el-button>
          </template>
        </el-table-column>
        <el-table-column prop="value_type" label="类型" width="100">
          <template #default="{ row }: any">
            <el-tag :type="typeTag(row.value_type)" size="small">{{ row.value_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="read_only" label="只读" width="80">
          <template #default="{ row }: any">
            <el-tag :type="row.read_only ? 'warning' : 'info'" size="small">
              {{ row.read_only ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }: any">
            <el-button type="primary" link size="small" @click="openEdit(row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="deleteVar(row)">删除</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
          <el-button type="danger" size="small" @click="deleteVar(item)">删除</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑变量' : '新建变量'" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="所属应用" prop="app_id">
          <el-select v-model="form.app_id" placeholder="选择应用" :disabled="isEdit">
            <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="键（key）" prop="key">
          <el-input v-model="form.key" placeholder="如：notice_text / max_retry" :disabled="isEdit" />
        </el-form-item>
        <el-form-item label="值（value）" prop="value">
          <el-input v-model="form.value" type="textarea" :rows="4" placeholder="变量值" />
        </el-form-item>
        <el-form-item label="值类型" prop="value_type">
          <el-select v-model="form.value_type" placeholder="选择类型">
            <el-option label="string" value="string" />
            <el-option label="number" value="number" />
            <el-option label="json" value="json" />
            <el-option label="bool" value="bool" />
          </el-select>
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" placeholder="可选" maxlength="200" />
        </el-form-item>
        <el-form-item label="只读">
          <el-switch v-model="form.read_only" />
          <span class="hint">只读变量客户端不可修改</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 查看完整值对话框 -->
    <el-dialog v-model="valueDialogVisible" title="变量值" width="500px">
      <div class="value-view">
        <div class="value-row">
          <span class="label">键：</span>
          <span class="mono">{{ currentValue.key }}</span>
        </div>
        <div class="value-row">
          <span class="label">类型：</span>
          <el-tag :type="typeTag(currentValue.value_type)" size="small">{{ currentValue.value_type }}</el-tag>
        </div>
        <div class="value-row">
          <span class="label">值：</span>
        </div>
        <pre class="value-content">{{ currentValue.value }}</pre>
      </div>
      <template #footer>
        <el-button @click="valueDialogVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listTenantCloudVarsApi,
  upsertTenantCloudVarApi,
  deleteTenantCloudVarApi,
  type TenantCloudVar
} from '@/api/tenant'
import { listAppsApi, type App } from '@/api/apps'

const list = ref<TenantCloudVar[]>([])
const total = ref(0)
const loading = ref(false)
const apps = ref<App[]>([])

const filter = reactive({
  app_id: undefined as number | undefined,
  keyword: '',
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'app_name', label: '应用' },
  { prop: 'key', label: '键' },
  { prop: 'value', label: '值', formatter: (v: string) => truncate(v, 30) },
  { prop: 'value_type', label: '类型' }
]

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  app_id: undefined as number | undefined,
  key: '',
  value: '',
  value_type: 'string',
  description: '',
  read_only: false
})

const rules = {
  app_id: [{ required: true, message: '请选择应用', trigger: 'change' }],
  key: [{ required: true, message: '请输入键', trigger: 'blur' }],
  value: [{ required: true, message: '请输入值', trigger: 'blur' }],
  value_type: [{ required: true, message: '请选择类型', trigger: 'change' }]
}

const valueDialogVisible = ref(false)
const currentValue = reactive({
  key: '',
  value: '',
  value_type: 'string'
})

const typeTag = (t: string): any => {
  const map: Record<string, any> = {
    string: 'primary',
    number: 'success',
    json: 'warning',
    bool: 'info'
  }
  return map[t] || 'info'
}

const truncate = (s: string, n: number) => {
  if (!s) return '-'
  return s.length > n ? s.slice(0, n) + '…' : s
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const showValue = (row: any) => {
  Object.assign(currentValue, {
    key: row.key,
    value: row.value,
    value_type: row.value_type
  })
  valueDialogVisible.value = true
}

const loadApps = async () => {
  try {
    const resp = await listAppsApi({ page: 1, page_size: 100 })
    apps.value = resp.list || []
  } catch {}
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listTenantCloudVarsApi({
      app_id: filter.app_id,
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

const openCreate = () => {
  isEdit.value = false
  Object.assign(form, {
    app_id: undefined,
    key: '',
    value: '',
    value_type: 'string',
    description: '',
    read_only: false
  })
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  isEdit.value = true
  Object.assign(form, {
    app_id: row.app_id,
    key: row.key,
    value: row.value,
    value_type: row.value_type,
    description: row.description || '',
    read_only: !!row.read_only
  })
  dialogVisible.value = true
}

const submit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (!form.app_id) {
      ElMessage.warning('请选择应用')
      return
    }
    submitLoading.value = true
    try {
      await upsertTenantCloudVarApi({
        app_id: form.app_id,
        key: form.key,
        value: form.value,
        value_type: form.value_type,
        description: form.description,
        read_only: form.read_only
      })
      ElMessage.success(isEdit.value ? '保存成功' : '创建成功')
      dialogVisible.value = false
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      submitLoading.value = false
    }
  })
}

const deleteVar = (row: any) => {
  ElMessageBox.confirm(`确定要删除变量「${row.key}」吗？删除后不可恢复`, '危险操作', {
    type: 'error',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await deleteTenantCloudVarApi(row.id)
      ElMessage.success('已删除')
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    }
  }).catch(() => {})
}

onMounted(async () => {
  await loadApps()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.cloud-vars-page {
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
  .value-view {
    .value-row {
      display: flex;
      align-items: center;
      padding: $spacing-sm 0;
      font-size: 13px;
      .label {
        color: $color-text-secondary;
        margin-right: $spacing-sm;
      }
    }
    .value-content {
      background: $color-primary-light;
      border-radius: $radius-sm;
      padding: $spacing-md;
      margin: $spacing-sm 0 0;
      font-family: monospace;
      font-size: 13px;
      color: $color-text-primary;
      white-space: pre-wrap;
      word-break: break-all;
      max-height: 320px;
      overflow-y: auto;
    }
  }
}
</style>
