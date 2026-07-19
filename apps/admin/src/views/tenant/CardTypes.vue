<!--
  卡类管理（开发者）- 响应式
-->
<template>
  <div class="card-types-page">
    <PageHeader title="卡类管理" subtitle="管理应用的卡密套餐">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建卡类</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.app_id" placeholder="选择应用" clearable style="width: 200px" @change="loadList">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
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
        <el-table-column prop="name" label="卡类名称" min-width="140" />
        <el-table-column prop="app_id" label="应用" min-width="120">
          <template #default="scope">{{ getAppName(scope.row.app_id) }}</template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="100">
          <template #default="scope">
            <el-tag size="small" :type="typeTag(scope.row.type)">{{ typeLabel(scope.row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="duration_seconds" label="时长" width="120">
          <template #default="scope">{{ formatDuration(scope.row.duration_seconds) }}</template>
        </el-table-column>
        <el-table-column prop="max_uses" label="可用次数" width="100" />
        <el-table-column prop="price" label="售价" width="100">
          <template #default="scope">¥{{ Number(scope.row.price).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="agent_base_price" label="代理底价" width="100">
          <template #default="scope">¥{{ Number(scope.row.agent_base_price).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="80">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'active' ? 'success' : 'info'" size="small">
              {{ scope.row.status === 'active' ? '启用' : '禁用' }}
            </el-tag>
          </template>
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

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑卡类' : '新建卡类'" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="所属应用" prop="app_id">
          <el-select v-model="form.app_id" placeholder="选择应用" :disabled="isEdit">
            <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡类名称" prop="name">
          <el-input v-model="form.name" placeholder="如：月卡 / 季卡 / 永久卡" />
        </el-form-item>
        <el-form-item label="类型" prop="type">
          <el-radio-group v-model="form.type">
            <el-radio value="duration">时长卡</el-radio>
            <el-radio value="count">次数卡</el-radio>
            <el-radio value="permanent">永久卡</el-radio>
            <el-radio value="trial">试用卡</el-radio>
            <el-radio value="feature">功能卡</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="form.type === 'duration'" label="时长（秒）">
          <el-input-number v-model="form.duration_seconds" :min="60" />
          <span class="hint">如 2592000 = 30 天</span>
        </el-form-item>
        <el-form-item v-if="form.type === 'count' || form.type === 'trial'" label="可用次数">
          <el-input-number v-model="form.max_uses" :min="1" />
        </el-form-item>
        <el-form-item label="售价（元）" prop="price">
          <el-input-number v-model="form.price" :min="0" :precision="2" />
        </el-form-item>
        <el-form-item label="代理底价（元）">
          <el-input-number v-model="form.agent_base_price" :min="0" :precision="2" />
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
import { listCardTypesApi, createCardTypeApi, updateCardTypeApi, type CardType, type CardTypeKind } from '@/api/cards'
import { listAppsApi, type App } from '@/api/apps'

const list = ref<CardType[]>([])
const total = ref(0)
const loading = ref(false)
const apps = ref<App[]>([])

const filter = reactive({
  app_id: undefined as number | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'name', label: '卡类' },
  { prop: 'type', label: '类型', formatter: (v: string) => typeLabel(v) },
  { prop: 'price', label: '售价', formatter: (v: number) => '¥' + v.toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => v === 'active' ? '启用' : '禁用' }
]

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  app_id: undefined as number | undefined,
  name: '',
  type: 'duration' as CardTypeKind,
  duration_seconds: 2592000,
  max_uses: 1,
  price: 0,
  agent_base_price: 0
})

const rules = {
  app_id: [{ required: true, message: '请选择应用', trigger: 'change' }],
  name: [{ required: true, message: '请输入卡类名称', trigger: 'blur' }],
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  price: [{ required: true, message: '请输入售价', trigger: 'blur' }]
}

const typeLabel = (t: string) => {
  const map: Record<string, string> = {
    duration: '时长卡',
    count: '次数卡',
    permanent: '永久卡',
    trial: '试用卡',
    feature: '功能卡'
  }
  return map[t] || t
}

const typeTag = (t: string): any => {
  const map: Record<string, any> = {
    duration: 'primary',
    count: 'success',
    permanent: 'warning',
    trial: 'info',
    feature: ''
  }
  return map[t] || 'info'
}

const formatDuration = (s: number) => {
  if (s === -1) return '永久'
  if (s === 0) return '-'
  if (s >= 86400) return `${Math.floor(s / 86400)} 天`
  if (s >= 3600) return `${Math.floor(s / 3600)} 小时`
  return `${s} 秒`
}

const getAppName = (id: number) => {
  const a = apps.value.find(x => x.id === id)
  return a ? a.name : `#${id}`
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
    const resp = await listCardTypesApi({
      app_id: filter.app_id,
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
  editingId.value = null
  Object.assign(form, {
    app_id: undefined,
    name: '',
    type: 'duration',
    duration_seconds: 2592000,
    max_uses: 1,
    price: 0,
    agent_base_price: 0
  })
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  isEdit.value = true
  editingId.value = row.id
  Object.assign(form, {
    app_id: row.app_id,
    name: row.name,
    type: row.type,
    duration_seconds: row.duration_seconds,
    max_uses: row.max_uses,
    price: row.price,
    agent_base_price: row.agent_base_price
  })
  dialogVisible.value = true
}

const submit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      if (isEdit.value && editingId.value) {
        await updateCardTypeApi(editingId.value, { ...form } as any)
        ElMessage.success('保存成功')
      } else {
        if (!form.app_id) {
          ElMessage.warning('请选择应用')
          return
        }
        await createCardTypeApi({ ...form, app_id: form.app_id } as any)
        ElMessage.success('创建成功')
      }
      dialogVisible.value = false
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      submitLoading.value = false
    }
  })
}

onMounted(async () => {
  await loadApps()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.card-types-page {
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
}
</style>
