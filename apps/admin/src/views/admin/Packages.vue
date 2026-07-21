<!--
  套餐管理（超管）- 响应式
  - 列表展示所有套餐
  - 新建/编辑套餐
  - 注：admin.ts 仅导出 list/create，更新接口待核实（铁律 06），
    暂按 RESTful 约定直接调用 PUT /admin/packages/:id，后端 501 时静默降级（铁律 04/06）
-->
<template>
  <div class="admin-packages-page">
    <PageHeader title="套餐管理" subtitle="管理开发者可选套餐与定价">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建套餐</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-input v-model="filter.keyword" placeholder="名称/描述" clearable style="width: 220px" @change="onFilterChange" />
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
        <el-table-column prop="name" label="名称" min-width="140" />
        <el-table-column prop="description" label="描述" min-width="200">
          <template #default="scope">{{ scope.row.description || '-' }}</template>
        </el-table-column>
        <el-table-column prop="max_apps" label="应用上限" width="100" />
        <el-table-column prop="max_cards" label="卡密上限" width="100" />
        <el-table-column prop="max_agents" label="代理上限" width="100" />
        <el-table-column prop="price_monthly" label="月费" width="100">
          <template #default="scope">¥{{ Number(scope.row.price_monthly || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="price_yearly" label="年费" width="100">
          <template #default="scope">¥{{ Number(scope.row.price_yearly || 0).toFixed(2) }}</template>
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

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新建套餐' : '编辑套餐'" :width="isMobile ? '92%' : '500px'">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="名称" prop="name">
          <el-input v-model="form.name" placeholder="如：标准版" maxlength="64" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" :rows="2" placeholder="可选" maxlength="500" />
        </el-form-item>
        <el-form-item label="最大应用数" prop="max_apps">
          <el-input-number v-model="form.max_apps" :min="0" :max="100000" />
        </el-form-item>
        <el-form-item label="最大卡密数" prop="max_cards">
          <el-input-number v-model="form.max_cards" :min="0" :max="10000000" />
        </el-form-item>
        <el-form-item label="最大代理数" prop="max_agents">
          <el-input-number v-model="form.max_agents" :min="0" :max="100000" />
        </el-form-item>
        <el-form-item label="月费（元）" prop="price_monthly">
          <el-input-number v-model="form.price_monthly" :min="0" :precision="2" :step="10" />
        </el-form-item>
        <el-form-item label="年费（元）" prop="price_yearly">
          <el-input-number v-model="form.price_yearly" :min="0" :precision="2" :step="100" />
        </el-form-item>
        <el-form-item label="特性 JSON">
          <el-input v-model="form.features" type="textarea" :rows="3" placeholder='可选，如 {"api": true, "support": "24h"}' />
          <span class="hint">字符串形式存放，由业务方自行解析</span>
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="启用" value="active" />
            <el-option label="禁用" value="disabled" />
          </el-select>
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
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listAdminPackagesApi, createAdminPackageApi,
  type AdminPackage
} from '@/api/admin'
import { request } from '@/api/http'

const list = ref<AdminPackage[]>([])
const total = ref(0)
const loading = ref(false)

// v0.7.0 修复：dialog 响应式宽度
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }

const filter = reactive({
  keyword: '',
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'name', label: '名称' },
  { prop: 'price_monthly', label: '月费', formatter: (v: number) => '¥' + Number(v || 0).toFixed(2) },
  { prop: 'price_yearly', label: '年费', formatter: (v: number) => '¥' + Number(v || 0).toFixed(2) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) }
]

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  name: '',
  description: '',
  max_apps: 5,
  max_cards: 10000,
  max_agents: 10,
  price_monthly: 0,
  price_yearly: 0,
  features: '',
  status: 'active' as 'active' | 'disabled'
})

const rules = {
  name: [{ required: true, message: '请输入套餐名称', trigger: 'blur' }],
  max_apps: [{ required: true, message: '请输入最大应用数', trigger: 'blur' }],
  max_cards: [{ required: true, message: '请输入最大卡密数', trigger: 'blur' }],
  max_agents: [{ required: true, message: '请输入最大代理数', trigger: 'blur' }],
  price_monthly: [{ required: true, message: '请输入月费', trigger: 'blur' }],
  price_yearly: [{ required: true, message: '请输入年费', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }]
}

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    active: 'success',
    disabled: 'danger'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    active: '启用',
    disabled: '禁用'
  }
  return map[s] || s
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  const d = new Date(s)
  // v0.7.0 修复：Invalid Date 兜底
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN')
}

// v0.7.0 修复：筛选变更重置分页
const onFilterChange = () => {
  filter.page = 1
  loadList()
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAdminPackagesApi({
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

const resetForm = () => {
  Object.assign(form, {
    name: '', description: '',
    max_apps: 5, max_cards: 10000, max_agents: 10,
    price_monthly: 0, price_yearly: 0,
    features: '', status: 'active'
  })
}

const openCreate = async () => {
  dialogMode.value = 'create'
  editingId.value = null
  resetForm()
  dialogVisible.value = true
  // v0.7.0 修复：清除上一次表单验证的残留状态
  await nextTick()
  formRef.value?.clearValidate()
}

const openEdit = async (row: any) => {
  dialogMode.value = 'edit'
  editingId.value = row.id
  Object.assign(form, {
    name: row.name || '',
    description: row.description || '',
    max_apps: row.max_apps ?? 0,
    max_cards: row.max_cards ?? 0,
    max_agents: row.max_agents ?? 0,
    price_monthly: row.price_monthly ?? 0,
    price_yearly: row.price_yearly ?? 0,
    features: row.features || '',
    status: row.status || 'active'
  })
  dialogVisible.value = true
  // v0.7.0 修复：清除上一次表单验证的残留状态
  await nextTick()
  formRef.value?.clearValidate()
}

const confirmSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    // v0.7.0 修复：防抖守卫，避免重复点击产生重复请求
    if (submitLoading.value) return
    submitLoading.value = true
    try {
      const payload = {
        name: form.name,
        description: form.description,
        max_apps: form.max_apps,
        max_cards: form.max_cards,
        max_agents: form.max_agents,
        price_monthly: form.price_monthly,
        price_yearly: form.price_yearly,
        features: form.features,
        status: form.status
      }
      if (dialogMode.value === 'create') {
        await createAdminPackageApi(payload)
        ElMessage.success('已创建')
      } else if (editingId.value) {
        // 待核实：admin.ts 暂未导出 updateAdminPackageApi（v0.3.0 后端补全），
        // 按 RESTful 约定直接调用 PUT /admin/packages/:id
        await request.put<AdminPackage>(`/admin/packages/${editingId.value}`, payload)
        ElMessage.success('已更新')
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

onMounted(() => {
  // v0.7.0 修复：dialog 响应式宽度
  checkMobile()
  window.addEventListener('resize', checkMobile)
  loadList()
})

// v0.7.0 修复：dialog 响应式宽度
onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.admin-packages-page {
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
}
</style>
