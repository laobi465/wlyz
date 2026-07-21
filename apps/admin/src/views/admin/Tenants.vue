<!--
  开发者管理（超管）- 响应式
  - 列表展示平台所有开发者
  - 新建/编辑开发者，启用/禁用
  - 注：后端 /admin/tenants 当前 501，列表/统计保持空状态（铁律 04/06）
-->
<template>
  <div class="admin-tenants-page">
    <PageHeader title="开发者管理" subtitle="管理平台所有开发者账号与套餐">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建开发者</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-input v-model="filter.keyword" placeholder="用户名/邮箱/公司" clearable style="width: 220px" @change="onFilterChange" />
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="onFilterChange">
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
        <el-table-column prop="email" label="邮箱" min-width="180">
          <template #default="scope">{{ scope.row.email || '-' }}</template>
        </el-table-column>
        <el-table-column prop="company" label="公司" min-width="140">
          <template #default="scope">{{ scope.row.company || '-' }}</template>
        </el-table-column>
        <el-table-column prop="package_name" label="套餐" width="120">
          <template #default="scope">{{ scope.row.package_name || '-' }}</template>
        </el-table-column>
        <el-table-column prop="app_count" label="应用数" width="90" />
        <el-table-column prop="card_count" label="卡密数" width="90" />
        <el-table-column prop="balance" label="余额" width="110">
          <template #default="scope">¥{{ Number(scope.row.balance || 0).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="expired_at" label="到期时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.expired_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="scope">
            <el-button type="primary" link size="small" @click="openEdit(scope.row)">编辑</el-button>
            <el-button v-if="scope.row.status !== 'active'" type="success" link size="small" @click="toggleStatus(scope.row, 'active')">启用</el-button>
            <el-button v-if="scope.row.status === 'active'" type="warning" link size="small" @click="toggleStatus(scope.row, 'disabled')">禁用</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
          <el-button v-if="item.status !== 'active'" type="success" size="small" @click="toggleStatus(item, 'active')">启用</el-button>
          <el-button v-if="item.status === 'active'" type="warning" size="small" @click="toggleStatus(item, 'disabled')">禁用</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新建开发者' : '编辑开发者'" :width="isMobile ? '92%' : '500px'">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item v-if="dialogMode === 'create'" label="用户名" prop="username">
          <el-input v-model="form.username" placeholder="登录用户名" maxlength="32" />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'create'" label="密码" prop="password">
          <el-input v-model="form.password" type="password" placeholder="初始密码" show-password maxlength="64" />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'create'" label="邮箱">
          <el-input v-model="form.email" placeholder="可选" maxlength="128" />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'create'" label="手机">
          <el-input v-model="form.phone" placeholder="可选" maxlength="20" />
        </el-form-item>
        <el-form-item v-if="dialogMode === 'create'" label="公司">
          <el-input v-model="form.company" placeholder="可选" maxlength="128" />
        </el-form-item>
        <el-form-item label="套餐" prop="package_id">
          <el-select v-model="form.package_id" placeholder="选择套餐" clearable style="width: 100%">
            <el-option v-for="p in packages" :key="p.id" :label="p.name" :value="p.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="dialogMode === 'create' ? '试用天数' : '延长天数'">
          <el-input-number v-model="form.expire_days" :min="0" :max="3650" />
          <span class="hint">{{ dialogMode === 'create' ? '0 表示不设到期' : '从当前到期时间延长 N 天，0 不变' }}</span>
        </el-form-item>
        <el-form-item v-if="dialogMode === 'edit'" label="状态">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="正常" value="active" />
            <el-option label="已禁用" value="disabled" />
            <el-option label="待审核" value="pending" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="dialogMode === 'edit'" label="重置密码">
          <el-input v-model="form.password" type="password" placeholder="留空则不重置" show-password maxlength="64" />
        </el-form-item>
        <el-form-item label="备注">
          <el-input v-model="form.remark" type="textarea" :rows="3" placeholder="可选" maxlength="500" />
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
  listAdminTenantsApi, createAdminTenantApi, updateAdminTenantApi,
  listAdminPackagesApi,
  type AdminTenant, type AdminPackage
} from '@/api/admin'

const list = ref<AdminTenant[]>([])
const total = ref(0)
const loading = ref(false)
const packages = ref<AdminPackage[]>([])

// v0.7.0 修复：dialog 响应式宽度
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }

const filter = reactive({
  keyword: '',
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'username', label: '用户名' },
  { prop: 'company', label: '公司', formatter: (v: string) => v || '-' },
  { prop: 'package_name', label: '套餐', formatter: (v: string) => v || '-' },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'expired_at', label: '到期', formatter: (v: string) => formatDate(v) }
]

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  username: '',
  password: '',
  email: '',
  phone: '',
  company: '',
  package_id: undefined as number | undefined,
  expire_days: 0,
  status: 'active' as 'active' | 'disabled' | 'pending',
  remark: ''
})

const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, message: '请输入密码', trigger: 'blur' }]
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

const formatDate = (s: string | null) => {
  if (!s) return '-'
  const d = new Date(s)
  // v0.7.0 修复：Invalid Date 兜底
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN')
}

const loadPackages = async () => {
  try {
    const resp = await listAdminPackagesApi({ page: 1, page_size: 100 })
    packages.value = resp.list || []
  } catch {
    // 错误已由 http 拦截器处理
  }
}

// v0.7.0 修复：筛选变更重置分页
const onFilterChange = () => {
  filter.page = 1
  loadList()
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAdminTenantsApi({
      keyword: filter.keyword,
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

const resetForm = () => {
  Object.assign(form, {
    username: '', password: '', email: '', phone: '', company: '',
    package_id: undefined, expire_days: 0, status: 'active', remark: ''
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
    username: row.username,
    password: '',
    email: row.email || '',
    phone: row.phone || '',
    company: row.company || '',
    package_id: row.package_id,
    expire_days: 0,
    status: row.status,
    remark: row.remark || ''
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
      if (dialogMode.value === 'create') {
        await createAdminTenantApi({
          username: form.username,
          password: form.password,
          email: form.email,
          phone: form.phone,
          company: form.company,
          package_id: form.package_id,
          expire_days: form.expire_days,
          remark: form.remark
        })
        ElMessage.success('已创建')
      } else if (editingId.value) {
        const data: any = {
          package_id: form.package_id,
          expire_days: form.expire_days,
          status: form.status,
          remark: form.remark
        }
        if (form.password) data.password = form.password
        await updateAdminTenantApi(editingId.value, data)
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

const toggleStatus = async (row: any, status: 'active' | 'disabled') => {
  try {
    await updateAdminTenantApi(row.id, { status })
    ElMessage.success('已更新状态')
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  }
}

onMounted(() => {
  // v0.7.0 修复：dialog 响应式宽度
  checkMobile()
  window.addEventListener('resize', checkMobile)
  loadPackages()
  loadList()
})

// v0.7.0 修复：dialog 响应式宽度
onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.admin-tenants-page {
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
}
</style>
