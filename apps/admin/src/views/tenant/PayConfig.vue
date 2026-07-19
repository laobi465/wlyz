<!--
  支付配置（开发者）- 响应式
  开发者自有易支付（按套餐开通）。需套餐 allow_custom_pay 开通，待 v0.3.0 实现。
  铁律 06 待核实：后端 /tenant/pay_config 当前为 501 占位（v0.3.0 交付），调用失败时静默降级。
-->
<template>
  <div class="pay-config-page">
    <PageHeader title="支付配置" subtitle="开发者自有易支付（按套餐开通）">
      <template #actions>
        <el-button type="primary" @click="openCreate">新建配置</el-button>
      </template>
    </PageHeader>

    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="此功能需套餐 allow_custom_pay 开通，待 v0.3.0 实现，当前后端 501"
      style="margin-bottom: 16px"
    />

    <div class="app-card">
      <div class="search-bar">
        <el-button @click="loadList">刷新</el-button>
      </div>

      <ResponsiveTable
        :data="list"
        :loading="loading"
        :total="total"
        :show-pagination="false"
        :mobile-fields="mobileFields"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="channel" label="渠道" min-width="120">
          <template #default="{ row }">
            <el-tag size="small">{{ channelText(row.channel) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'active' ? 'success' : 'info'" size="small">
              {{ row.status === 'active' ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="updated_at" label="更新时间" width="180">
          <template #default="{ row }">{{ formatDate(row.updated_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="160" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openEdit(row)">编辑</el-button>
            <el-button type="success" link size="small" :loading="testingId === row.id" @click="test(row)">测试</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
          <el-button type="success" size="small" :loading="testingId === item.id" @click="test(item)">测试</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <el-dialog v-model="dialogVisible" :title="isEdit ? '编辑支付配置' : '新建支付配置'" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="渠道" prop="channel">
          <el-select v-model="form.channel" placeholder="选择渠道" :disabled="isEdit">
            <el-option label="易支付" value="epay" />
            <el-option label="支付宝" value="alipay" />
            <el-option label="微信" value="wechat" />
            <el-option label="Stripe" value="stripe" />
          </el-select>
        </el-form-item>
        <el-form-item label="PID">
          <el-input v-model="form.pid" placeholder="商户 ID" autocomplete="off" />
        </el-form-item>
        <el-form-item label="Key" prop="key">
          <el-input v-model="form.key" type="password" show-password placeholder="商户密钥（敏感）" autocomplete="off" />
        </el-form-item>
        <el-form-item label="API URL">
          <el-input v-model="form.api_url" placeholder="如 https://pay.example.com" />
        </el-form-item>
        <el-form-item label="通知路径">
          <el-input v-model="form.notify_url" placeholder="如 /api/v1/pay/notify/epay" />
        </el-form-item>
        <el-form-item label="同步跳转路径">
          <el-input v-model="form.return_url" placeholder="如 /pay/return" />
        </el-form-item>
        <el-form-item label="状态" prop="status">
          <el-select v-model="form.status" placeholder="选择状态">
            <el-option label="启用" value="active" />
            <el-option label="禁用" value="disabled" />
          </el-select>
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
  listTenantPayConfigApi, saveTenantPayConfigApi, testTenantPayConfigApi,
  type TenantPayConfig
} from '@/api/tenant'

const list = ref<TenantPayConfig[]>([])
const total = ref(0)
const loading = ref(false)
const testingId = ref<number | null>(null)

const mobileFields = [
  { prop: 'channel', label: '渠道', formatter: (v: string) => channelText(v) },
  { prop: 'status', label: '状态', formatter: (v: string) => v === 'active' ? '启用' : '禁用' },
  { prop: 'updated_at', label: '更新时间', formatter: (v: string) => formatDate(v) }
]

const dialogVisible = ref(false)
const isEdit = ref(false)
const submitLoading = ref(false)
const formRef = ref<FormInstance>()
const editingId = ref<number | null>(null)

const form = reactive({
  channel: 'epay',
  pid: '',
  key: '',
  api_url: '',
  notify_url: '',
  return_url: '',
  status: 'active'
})

const rules = {
  channel: [{ required: true, message: '请选择渠道', trigger: 'change' }],
  key: [{ required: true, message: '请输入密钥', trigger: 'blur' }],
  status: [{ required: true, message: '请选择状态', trigger: 'change' }]
}

const channelText = (c: string) => {
  const map: Record<string, string> = {
    epay: '易支付',
    alipay: '支付宝',
    wechat: '微信',
    stripe: 'Stripe'
  }
  return map[c] || c
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listTenantPayConfigApi()
    list.value = resp.list || []
    total.value = list.value.length
  } catch {
    // 后端 501 占位时静默降级（铁律 06），不编造数据
  } finally {
    loading.value = false
  }
}

const openCreate = () => {
  isEdit.value = false
  editingId.value = null
  Object.assign(form, {
    channel: 'epay', pid: '', key: '', api_url: '',
    notify_url: '', return_url: '', status: 'active'
  })
  dialogVisible.value = true
}

const openEdit = (row: any) => {
  isEdit.value = true
  editingId.value = row.id
  Object.assign(form, {
    channel: row.channel,
    pid: row.config?.pid || '',
    key: row.config?.key || '',
    api_url: row.config?.api_url || '',
    notify_url: row.config?.notify_url || '',
    return_url: row.config?.return_url || '',
    status: row.status
  })
  dialogVisible.value = true
}

const submit = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    submitLoading.value = true
    try {
      await saveTenantPayConfigApi({
        channel: form.channel,
        config: {
          pid: form.pid,
          key: form.key,
          api_url: form.api_url,
          notify_url: form.notify_url,
          return_url: form.return_url
        },
        status: form.status
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

const test = async (row: any) => {
  testingId.value = row.id
  try {
    const resp = await testTenantPayConfigApi(row.id)
    if (resp.success) {
      ElMessage.success(resp.message || '测试成功')
    } else {
      ElMessage.warning(resp.message || '测试失败')
    }
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    testingId.value = null
  }
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.pay-config-page {
  // 依赖全局 .app-card / .search-bar 样式
}
</style>
