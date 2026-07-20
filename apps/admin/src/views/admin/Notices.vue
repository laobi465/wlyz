<!--
  平台公告（超管）- 响应式
  - 列表展示所有公告
  - 新建/编辑/删除公告
  - 注：后端 /admin/notices 当前 501，列表/统计保持空状态（铁律 04/06）
-->
<template>
  <div class="admin-notices-page">
    <PageHeader title="平台公告" subtitle="管理平台向开发者/代理下发的公告">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建公告</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.type" placeholder="类型" clearable style="width: 140px" @change="loadList">
          <el-option label="平台公告" value="platform" />
          <el-option label="开发者公告" value="developer" />
          <el-option label="应用公告" value="app" />
          <el-option label="代理通知" value="agent_notify" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="草稿" value="draft" />
          <el-option label="已发布" value="published" />
          <el-option label="已下线" value="offline" />
        </el-select>
        <el-input v-model="filter.keyword" placeholder="标题关键词" clearable style="width: 200px" @change="loadList" />
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
        <el-table-column prop="type" label="类型" width="120">
          <template #default="scope">
            <el-tag :type="typeTag(scope.row.type)" size="small" effect="plain">{{ typeText(scope.row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="pinned" label="置顶" width="80">
          <template #default="scope">
            <el-tag v-if="scope.row.pinned" type="warning" size="small">是</el-tag>
            <span v-else class="text-secondary">否</span>
          </template>
        </el-table-column>
        <el-table-column prop="sort" label="排序" width="80" />
        <el-table-column prop="publish_at" label="发布时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.publish_at) }}</template>
        </el-table-column>
        <el-table-column prop="expire_at" label="过期时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.expire_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="scope">
            <el-button type="primary" link size="small" @click="openEdit(scope.row)">编辑</el-button>
            <el-button type="danger" link size="small" @click="removeRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
          <el-button type="danger" size="small" @click="removeRow(item)">删除</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 新建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogMode === 'create' ? '新建公告' : '编辑公告'" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="类型" prop="type">
          <el-select v-model="form.type" style="width: 100%">
            <el-option label="平台公告" value="platform" />
            <el-option label="开发者公告" value="developer" />
            <el-option label="应用公告" value="app" />
            <el-option label="代理通知" value="agent_notify" />
          </el-select>
        </el-form-item>
        <el-form-item label="标题" prop="title">
          <el-input v-model="form.title" placeholder="公告标题" maxlength="128" />
        </el-form-item>
        <el-form-item label="内容" prop="content">
          <el-input v-model="form.content" type="textarea" :rows="5" placeholder="公告正文" maxlength="5000" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" style="width: 100%">
            <el-option label="草稿" value="draft" />
            <el-option label="已发布" value="published" />
            <el-option label="已下线" value="offline" />
          </el-select>
        </el-form-item>
        <el-form-item label="置顶">
          <el-switch v-model="form.pinned" />
        </el-form-item>
        <el-form-item label="排序">
          <el-input-number v-model="form.sort" :min="0" :max="9999" />
          <span class="hint">数字越小越靠前</span>
        </el-form-item>
        <el-form-item label="发布时间">
          <el-date-picker
            v-model="form.publish_at"
            type="datetime"
            placeholder="可选，留空则立即发布"
            value-format="YYYY-MM-DDTHH:mm:ss"
            style="width: 100%"
          />
        </el-form-item>
        <el-form-item label="过期时间">
          <el-date-picker
            v-model="form.expire_at"
            type="datetime"
            placeholder="可选，过期后自动隐藏"
            value-format="YYYY-MM-DDTHH:mm:ss"
            clearable
            style="width: 100%"
          />
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
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listAdminNoticesApi, createAdminNoticeApi, updateAdminNoticeApi, deleteAdminNoticeApi,
  type AdminNotice, type NoticeType, type NoticeStatus
} from '@/api/admin'

const list = ref<AdminNotice[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  type: undefined as NoticeType | undefined,
  status: undefined as NoticeStatus | undefined,
  keyword: '',
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'title', label: '标题' },
  { prop: 'type', label: '类型', formatter: (v: string) => typeText(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'publish_at', label: '发布', formatter: (v: string) => formatDate(v) }
]

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  type: 'platform' as NoticeType,
  title: '',
  content: '',
  status: 'draft' as NoticeStatus,
  pinned: false,
  sort: 0,
  publish_at: '',
  expire_at: ''
})

const rules = {
  type: [{ required: true, message: '请选择类型', trigger: 'change' }],
  title: [{ required: true, message: '请输入标题', trigger: 'blur' }],
  content: [{ required: true, message: '请输入内容', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }]
}

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    draft: 'info',
    published: 'success',
    offline: 'info'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    draft: '草稿',
    published: '已发布',
    offline: '已下线'
  }
  return map[s] || s
}

const typeTag = (t: string): any => {
  const map: Record<string, any> = {
    platform: 'danger',
    developer: 'primary',
    app: 'warning',
    agent_notify: 'success'
  }
  return map[t] || 'info'
}

const typeText = (t: string) => {
  const map: Record<string, string> = {
    platform: '平台公告',
    developer: '开发者',
    app: '应用',
    agent_notify: '代理通知'
  }
  return map[t] || t
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAdminNoticesApi({
      type: filter.type,
      status: filter.status,
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
    type: 'platform',
    title: '',
    content: '',
    status: 'draft',
    pinned: false,
    sort: 0,
    publish_at: '',
    expire_at: ''
  })
}

const openCreate = () => {
  dialogMode.value = 'create'
  editingId.value = null
  resetForm()
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  dialogMode.value = 'edit'
  editingId.value = row.id
  Object.assign(form, {
    type: row.type || 'platform',
    title: row.title || '',
    content: row.content || '',
    status: row.status || 'draft',
    pinned: !!row.pinned,
    sort: row.sort ?? 0,
    publish_at: row.publish_at || '',
    expire_at: row.expire_at || ''
  })
  dialogVisible.value = true
}

const confirmSubmit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      const payload = {
        type: form.type,
        title: form.title,
        content: form.content,
        status: form.status,
        pinned: form.pinned,
        sort: form.sort,
        publish_at: form.publish_at || undefined,
        expire_at: form.expire_at || undefined
      }
      if (dialogMode.value === 'create') {
        await createAdminNoticeApi(payload)
        ElMessage.success('已创建')
      } else if (editingId.value) {
        await updateAdminNoticeApi(editingId.value, payload as any)
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

const removeRow = (row: any) => {
  ElMessageBox.confirm(`确定要删除公告「${row.title}」吗？删除后不可恢复`, '危险操作', {
    type: 'error',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await deleteAdminNoticeApi(row.id)
      ElMessage.success('已删除')
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    }
  }).catch(() => {})
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.admin-notices-page {
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
  .text-secondary { color: $color-text-secondary; font-size: 13px; }
}
</style>
