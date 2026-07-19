<!--
  我的公告（开发者）- 响应式
  向代理/H5 终端用户发布公告。
  铁律 06 待核实：后端 /tenant/notices 当前为 501 占位（v0.3.0 交付），调用失败时静默降级。
  注：tenant.ts 仅 createTenantNoticeApi，无 update/delete API；
      删除按钮当前 disabled，待 v0.3.0 补全 delete/update API（铁律 06）。
-->
<template>
  <div class="notices-page">
    <PageHeader title="我的公告" subtitle="向代理/H5 终端用户发布公告">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建公告</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.type" placeholder="类型" clearable style="width: 140px" @change="loadList">
          <el-option label="开发者" value="tenant" />
          <el-option label="代理" value="agent" />
          <el-option label="H5 终端" value="h5" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="草稿" value="draft" />
          <el-option label="已发布" value="published" />
          <el-option label="已归档" value="archived" />
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
        <el-table-column prop="type" label="类型" width="100">
          <template #default="{ row }">
            <el-tag size="small" :type="typeTag(row.type)">{{ typeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="180" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="pinned" label="置顶" width="80">
          <template #default="{ row }">
            <el-tag v-if="row.pinned" type="warning" size="small">是</el-tag>
            <span v-else>否</span>
          </template>
        </el-table-column>
        <el-table-column prop="publish_at" label="发布时间" width="160">
          <template #default="{ row }">{{ formatDate(row.publish_at) }}</template>
        </el-table-column>
        <el-table-column prop="expire_at" label="过期时间" width="160">
          <template #default="{ row }">{{ formatDate(row.expire_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }">
            <!-- 待核实 v0.3.0 补全 delete/update API（铁律 06），暂不可用 -->
            <el-button type="danger" link size="small" disabled>删除</el-button>
          </template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <el-dialog v-model="dialogVisible" title="新建公告" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="类型" prop="type">
          <el-select v-model="form.type" placeholder="选择类型">
            <el-option label="开发者" value="tenant" />
            <el-option label="代理" value="agent" />
            <el-option label="H5 终端" value="h5" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" placeholder="公告标题" maxlength="128" />
        </el-form-item>
        <el-form-item label="内容" prop="content">
          <el-input v-model="form.content" type="textarea" :rows="5" placeholder="公告内容" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" placeholder="选择状态">
            <el-option label="草稿" value="draft" />
            <el-option label="已发布" value="published" />
            <el-option label="已归档" value="archived" />
          </el-select>
        </el-form-item>
        <el-form-item label="置顶">
          <el-switch v-model="form.pinned" />
        </el-form-item>
        <el-form-item label="发布时间">
          <el-date-picker
            v-model="form.publish_at"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="留空则立即发布"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expire_at"
            type="datetime"
            value-format="YYYY-MM-DD HH:mm:ss"
            placeholder="可选"
            style="width: 100%"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submit">发布</el-button>
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
  listTenantNoticesApi, createTenantNoticeApi, type TenantNotice
} from '@/api/tenant'

const list = ref<TenantNotice[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  type: undefined as string | undefined,
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'title', label: '标题' },
  { prop: 'type', label: '类型', formatter: (v: string) => typeText(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'publish_at', label: '发布时间', formatter: (v: string) => formatDate(v) }
]

const dialogVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  type: 'agent',
  title: '',
  content: '',
  status: 'published',
  pinned: false,
  publish_at: '',
  expire_at: ''
})

const rules = {
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  content: [{ required: true, message: '请输入内容', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }]
}

const typeTag = (t: string): any => {
  const map: Record<string, any> = {
    tenant: 'primary',
    agent: 'success',
    h5: 'warning'
  }
  return map[t] || 'info'
}

const typeText = (t: string) => {
  const map: Record<string, string> = {
    tenant: '开发者',
    agent: '代理',
    h5: 'H5 终端'
  }
  return map[t] || t
}

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    published: 'success',
    draft: 'warning',
    archived: 'info'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    draft: '草稿',
    published: '已发布',
    archived: '已归档'
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
    const resp = await listTenantNoticesApi({
      type: filter.type,
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

const openCreate = () => {
  Object.assign(form, {
    type: 'agent',
    title: '',
    content: '',
    status: 'published',
    pinned: false,
    publish_at: '',
    expire_at: ''
  })
  dialogVisible.value = true
}

const submit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      await createTenantNoticeApi({
        type: form.type,
        title: form.title,
        content: form.content,
        status: form.status,
        pinned: form.pinned,
        publish_at: form.publish_at || undefined,
        expire_at: form.expire_at || undefined
      })
      ElMessage.success('创建成功')
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

.notices-page {
  // 依赖全局 .app-card / .search-bar 样式
}
</style>
