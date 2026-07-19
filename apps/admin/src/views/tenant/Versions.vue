<!--
  版本管理（开发者）- 响应式
-->
<template>
  <div class="versions-page">
    <PageHeader title="版本管理" subtitle="应用版本发布与强制更新">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建版本</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.app_id" placeholder="应用" clearable style="width: 160px" @change="loadList">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-select v-model="filter.channel" placeholder="渠道" clearable style="width: 120px" @change="loadList">
          <el-option label="稳定版" value="stable" />
          <el-option label="测试版" value="beta" />
          <el-option label="内测版" value="alpha" />
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
        <el-table-column prop="app_name" label="应用" min-width="120" />
        <el-table-column prop="version" label="版本号" min-width="120">
          <template #default="{ row }: any">
            <span class="mono">{{ row.version }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="channel" label="渠道" width="100">
          <template #default="{ row }: any">
            <el-tag :type="channelTag(row.channel)" size="small">{{ channelText(row.channel) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="download_url" label="下载 URL" min-width="200">
          <template #default="{ row }: any">
            <el-link v-if="row.download_url" type="primary" :href="row.download_url" target="_blank" rel="noopener">
              <span class="mono">{{ truncate(row.download_url, 40) }}</span>
            </el-link>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="min_version" label="最低版本" width="110">
          <template #default="{ row }: any">
            <span class="mono">{{ row.min_version || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="force_update" label="强制更新" width="100">
          <template #default="{ row }: any">
            <el-tag :type="row.force_update ? 'danger' : 'info'" size="small">
              {{ row.force_update ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="published" label="已发布" width="90">
          <template #default="{ row }: any">
            <el-tag :type="row.published ? 'success' : 'info'" size="small">
              {{ row.published ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="published_at" label="发布时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.published_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="{ row }: any">
            <el-button type="danger" link size="small" @click="deleteVersion(row)">删除</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="danger" size="small" @click="deleteVersion(item)">删除</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 新建对话框 -->
    <el-dialog v-model="dialogVisible" title="新建版本" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="所属应用" prop="app_id">
          <el-select v-model="form.app_id" placeholder="选择应用">
            <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="版本号" prop="version">
          <el-input v-model="form.version" placeholder="如：1.2.0" />
        </el-form-item>
        <el-form-item label="渠道" prop="channel">
          <el-select v-model="form.channel" placeholder="选择渠道">
            <el-option label="稳定版" value="stable" />
            <el-option label="测试版" value="beta" />
            <el-option label="内测版" value="alpha" />
          </el-select>
        </el-form-item>
        <el-form-item label="下载 URL" prop="download_url">
          <el-input v-model="form.download_url" placeholder="https://..." />
        </el-form-item>
        <el-form-item label="更新日志">
          <el-input v-model="form.update_log" type="textarea" :rows="4" placeholder="本次更新内容" />
        </el-form-item>
        <el-form-item label="最低版本">
          <el-input v-model="form.min_version" placeholder="可选，如：1.0.0" />
        </el-form-item>
        <el-form-item label="强制更新">
          <el-switch v-model="form.force_update" />
          <span class="hint">开启后低于最低版本的客户端将被强制更新</span>
        </el-form-item>
        <el-form-item label="立即发布">
          <el-switch v-model="form.published" />
          <span class="hint">关闭则以草稿状态保存</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submit">创建</el-button>
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
  listTenantVersionsApi,
  createTenantVersionApi,
  deleteTenantVersionApi,
  type TenantVersion,
  type VersionChannel
} from '@/api/tenant'
import { listAppsApi, type App } from '@/api/apps'

const list = ref<TenantVersion[]>([])
const total = ref(0)
const loading = ref(false)
const apps = ref<App[]>([])

const filter = reactive({
  app_id: undefined as number | undefined,
  channel: undefined as VersionChannel | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'app_name', label: '应用' },
  { prop: 'version', label: '版本号' },
  { prop: 'channel', label: '渠道', formatter: (v: string) => channelText(v) },
  { prop: 'force_update', label: '强制更新', formatter: (v: boolean) => v ? '是' : '否' },
  { prop: 'published_at', label: '发布', formatter: (v: string) => formatDate(v) }
]

const dialogVisible = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  app_id: undefined as number | undefined,
  version: '',
  channel: 'stable' as VersionChannel,
  download_url: '',
  update_log: '',
  min_version: '',
  force_update: false,
  published: true
})

const rules = {
  app_id: [{ required: true, message: '请选择应用', trigger: 'change' }],
  version: [{ required: true, message: '请输入版本号', trigger: 'blur' }],
  channel: [{ required: true, message: '请选择渠道', trigger: 'change' }],
  download_url: [{ required: true, message: '请输入下载 URL', trigger: 'blur' }]
}

const channelTag = (c: string): any => {
  const map: Record<string, any> = {
    stable: 'success',
    beta: 'warning',
    alpha: 'info'
  }
  return map[c] || 'info'
}

const channelText = (c: string) => {
  const map: Record<string, string> = {
    stable: '稳定版',
    beta: '测试版',
    alpha: '内测版'
  }
  return map[c] || c || '-'
}

const truncate = (s: string, n: number) => {
  if (!s) return '-'
  return s.length > n ? s.slice(0, n) + '…' : s
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
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
    const resp = await listTenantVersionsApi({
      app_id: filter.app_id,
      channel: filter.channel,
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
  Object.assign(form, {
    app_id: undefined,
    version: '',
    channel: 'stable',
    download_url: '',
    update_log: '',
    min_version: '',
    force_update: false,
    published: true
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
      await createTenantVersionApi({
        app_id: form.app_id,
        version: form.version,
        channel: form.channel,
        download_url: form.download_url,
        update_log: form.update_log,
        min_version: form.min_version,
        force_update: form.force_update,
        published: form.published
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

const deleteVersion = (row: any) => {
  ElMessageBox.confirm(`确定要删除版本「${row.version}」吗？删除后不可恢复`, '危险操作', {
    type: 'error',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await deleteTenantVersionApi(row.id)
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

.versions-page {
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
}
</style>
