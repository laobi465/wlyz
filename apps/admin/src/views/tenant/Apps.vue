<!--
  应用管理（开发者）- 响应式
-->
<template>
  <div class="apps-page">
    <PageHeader title="应用管理" subtitle="管理你的应用与密钥">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建应用</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-input v-model="filter.keyword" placeholder="搜索应用名称或 AppKey" clearable style="width: 280px" @change="loadList" />
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 140px" @change="loadList">
          <el-option label="启用" value="active" />
          <el-option label="禁用" value="disabled" />
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
        <el-table-column prop="name" label="应用名称" min-width="160" />
        <el-table-column prop="app_key" label="AppKey" min-width="200">
          <template #default="scope">
            <span class="mono">{{ scope.row.app_key }}</span>
            <el-button text size="small" @click="copy(scope.row.app_key)">复制</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="max_devices" label="最大设备数" width="100" />
        <el-table-column prop="status" label="状态" width="80">
          <template #default="scope">
            <el-tag :type="scope.row.status === 'active' ? 'success' : 'info'" size="small">
              {{ scope.row.status === 'active' ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="scope">
            <el-button type="primary" link size="small" @click="openDetail(scope.row)">详情</el-button>
            <el-button type="warning" link size="small" @click="openResetKey(scope.row)">重置密钥</el-button>
            <el-button type="danger" link size="small" @click="confirmDelete(scope.row)">删除</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openDetail(item)">详情</el-button>
          <el-button type="warning" size="small" @click="openResetKey(item)">重置</el-button>
          <el-button type="danger" size="small" @click="confirmDelete(item)">删除</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 新建/编辑应用对话框 -->
    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑应用' : '新建应用'" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="应用名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入应用名称" />
        </el-form-item>
        <el-form-item label="应用描述">
          <el-input v-model="form.description" type="textarea" :rows="3" />
        </el-form-item>
        <el-form-item label="最大设备数" prop="max_devices">
          <el-input-number v-model="form.max_devices" :min="1" :max="100" />
        </el-form-item>
        <el-form-item label="心跳间隔（秒）">
          <el-input-number v-model="form.heartbeat_interval" :min="10" :max="3600" />
        </el-form-item>
        <el-form-item label="心跳超时（秒）">
          <el-input-number v-model="form.heartbeat_timeout" :min="60" :max="86400" />
        </el-form-item>
        <el-form-item label="离线宽限期（秒）">
          <el-input-number v-model="form.offline_grace" :min="0" :max="604800" />
        </el-form-item>
        <el-form-item label="解绑扣时长（秒）">
          <el-input-number v-model="form.unbind_deduct_seconds" :min="0" :max="604800" />
        </el-form-item>
        <el-form-item label="代理佣金模式">
          <el-radio-group v-model="form.agent_commission_mode">
            <el-radio value="percentage">按比例</el-radio>
            <el-radio value="diff">按差价</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="submitLoading" @click="submit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 应用详情对话框 -->
    <el-dialog v-model="detailVisible" title="应用详情" width="600px">
      <div v-if="currentApp" class="detail-content">
        <div class="detail-row"><span class="label">应用名称</span><span class="value">{{ currentApp.name }}</span></div>
        <div class="detail-row"><span class="label">AppKey</span><span class="value mono">{{ currentApp.app_key }}</span></div>
        <div class="detail-row"><span class="label">AppSecret</span><span class="value mono">********<el-button text size="small" @click="showSecret">显示</el-button></span></div>
        <div class="detail-row"><span class="label">SignSecret</span><span class="value mono">********<el-button text size="small" @click="showSign">显示</el-button></span></div>
        <div class="detail-row"><span class="label">状态</span><span class="value">{{ currentApp.status }}</span></div>
        <div class="detail-row"><span class="label">最大设备数</span><span class="value">{{ currentApp.max_devices }}</span></div>
        <div class="detail-row"><span class="label">心跳间隔</span><span class="value">{{ currentApp.heartbeat_interval }} 秒</span></div>
        <div class="detail-row"><span class="label">心跳超时</span><span class="value">{{ currentApp.heartbeat_timeout }} 秒</span></div>
        <div class="detail-row"><span class="label">离线宽限期</span><span class="value">{{ currentApp.offline_grace }} 秒</span></div>
        <div class="detail-row"><span class="label">解绑扣时长</span><span class="value">{{ currentApp.unbind_deduct_seconds }} 秒</span></div>
      </div>
    </el-dialog>

    <!-- 重置密钥对话框 -->
    <el-dialog v-model="resetKeyVisible" title="重置密钥" width="500px">
      <el-alert type="warning" :closable="false" show-icon>
        重置密钥后，旧密钥将立即失效，所有使用旧密钥的客户端将无法验证。请谨慎操作！
      </el-alert>
      <el-form label-position="top" style="margin-top: 16px">
        <el-form-item label="重置范围">
          <el-radio-group v-model="resetType">
            <el-radio value="all">全部（AppKey/AppSecret/SignSecret）</el-radio>
            <el-radio value="app_key">仅 AppKey</el-radio>
            <el-radio value="app_secret">仅 AppSecret</el-radio>
            <el-radio value="sign_secret">仅 SignSecret（轮换）</el-radio>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetKeyVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetLoading" @click="confirmResetKey">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { listAppsApi, createAppApi, updateAppApi, deleteAppApi, resetAppKeyApi, getAppApi, type App } from '@/api/apps'

const list = ref<App[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  keyword: '',
  status: undefined as string | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'name', label: '应用' },
  { prop: 'app_key', label: 'AppKey' },
  { prop: 'max_devices', label: '设备数', formatter: (v: number) => String(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => v === 'active' ? '启用' : '禁用' },
  { prop: 'created_at', label: '创建', formatter: (v: string) => formatDate(v) }
]

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  name: '',
  description: '',
  max_devices: 1,
  heartbeat_interval: 60,
  heartbeat_timeout: 180,
  offline_grace: 86400,
  unbind_deduct_seconds: 86400,
  agent_commission_mode: 'percentage' as 'percentage' | 'diff'
})

const rules = {
  name: [{ required: true, message: '请输入应用名称', trigger: 'blur' }],
  max_devices: [{ required: true, message: '请输入最大设备数', trigger: 'blur' }]
}

const detailVisible = ref(false)
const currentApp = ref<App | null>(null)

const resetKeyVisible = ref(false)
const resetType = ref<'all' | 'app_key' | 'app_secret' | 'sign_secret'>('all')
const resetLoading = ref(false)
const resettingAppId = ref<number | null>(null)

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAppsApi({
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

const openCreate = () => {
  isEdit.value = false
  editingId.value = null
  Object.assign(form, {
    name: '',
    description: '',
    max_devices: 1,
    heartbeat_interval: 60,
    heartbeat_timeout: 180,
    offline_grace: 86400,
    unbind_deduct_seconds: 86400,
    agent_commission_mode: 'percentage'
  })
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  isEdit.value = true
  editingId.value = row.id
  Object.assign(form, {
    name: row.name,
    description: row.description,
    max_devices: row.max_devices,
    heartbeat_interval: row.heartbeat_interval,
    heartbeat_timeout: row.heartbeat_timeout,
    offline_grace: row.offline_grace,
    unbind_deduct_seconds: row.unbind_deduct_seconds,
    agent_commission_mode: row.agent_commission_mode
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
        await updateAppApi(editingId.value, { ...form } as any)
        ElMessage.success('保存成功')
      } else {
        const resp = await createAppApi({ ...form } as any)
        ElMessage.success('创建成功，请妥善保管密钥')
        // 显示密钥
        currentApp.value = resp
        detailVisible.value = true
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

const openDetail = async (row: any) => {
  try {
    const detail = await getAppApi(row.id)
    currentApp.value = detail
    detailVisible.value = true
  } catch {
    // 错误已由 http 拦截器处理
  }
}

const showSecret = async () => {
  if (!currentApp.value) return
  // 待核实：后端应提供显示明文密钥的接口
  ElMessage.info('请联系管理员或调用 reset_key 接口重新生成')
}

const showSign = () => showSecret()

const openResetKey = (row: any) => {
  resettingAppId.value = row.id
  resetType.value = 'all'
  resetKeyVisible.value = true
}

const confirmResetKey = async () => {
  if (!resettingAppId.value) return
  resetLoading.value = true
  try {
    const resp = await resetAppKeyApi(resettingAppId.value, { reset_type: resetType.value })
    ElMessage.success('密钥已重置')
    resetKeyVisible.value = false
    // 显示新密钥
    if (resp.app_key) {
      ElMessageBox.alert(
        `新的 AppKey: ${resp.app_key}\n新的 AppSecret: ${resp.app_secret || '(未返回)'}\n新的 SignSecret: ${resp.sign_secret || '(未返回)'}`,
        '请妥善保存新密钥',
        { confirmButtonText: '我知道了' }
      )
    }
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    resetLoading.value = false
  }
}

const confirmDelete = (row: any) => {
  ElMessageBox.confirm(
    `确定要删除应用「${row.name}」吗？删除后所有卡密将无法验证！`,
    '危险操作',
    { type: 'error', confirmButtonText: '确认删除', cancelButtonText: '取消' }
  ).then(async () => {
    try {
      await deleteAppApi(row.id)
      ElMessage.success('已删除')
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    }
  }).catch(() => {})
}

const copy = (text: string) => {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

const formatDate = (s: string) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.apps-page {
  .mono {
    font-family: monospace;
    font-size: 13px;
    color: $color-text-primary;
  }

  .detail-content {
    .detail-row {
      display: flex;
      padding: $spacing-sm 0;
      border-bottom: 1px solid $color-border-lighter;
      &:last-child { border-bottom: none; }
      .label {
        width: 120px;
        color: $color-text-secondary;
        font-size: 13px;
        flex-shrink: 0;
      }
      .value {
        color: $color-text-primary;
        font-size: 13px;
        flex: 1;
        word-break: break-all;
      }
    }
  }
}
</style>
