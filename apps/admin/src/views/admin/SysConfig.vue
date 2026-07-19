<!--
  系统配置（超管）- 响应式
  - 按 group 分组显示所有 sys_config
  - 支持编辑、保存
  - 铁律 05：所有可变参数后台化的可视化入口
-->
<template>
  <div class="sys-config-page">
    <PageHeader title="系统配置" subtitle="管理所有可变参数（铁律 05：后台化）" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="activeGroup" placeholder="选择配置分组" @change="loadList">
          <el-option label="全部分组" value="" />
          <el-option v-for="g in groups" :key="g" :label="g" :value="g" />
        </el-select>
        <el-input v-model="keyword" placeholder="搜索配置项" clearable style="width: 240px" />
        <el-button @click="loadList">刷新</el-button>
      </div>

      <el-tabs v-model="activeGroup" @tab-change="loadList">
        <el-tab-pane label="全部" name="" />
        <el-tab-pane v-for="g in groups" :key="g" :label="groupLabel(g)" :name="g" />
      </el-tabs>

      <ResponsiveTable
        :data="filteredList"
        :loading="loading"
        :show-pagination="false"
        :mobile-fields="mobileFields"
      >
        <el-table-column prop="config_key" label="配置项 Key" min-width="200" />
        <el-table-column prop="config_name" label="名称" min-width="160" />
        <el-table-column prop="config_value" label="当前值" min-width="200">
          <template #default="scope">
            <span v-if="!isSensitive(scope.row.config_key)" class="value-text">{{ scope.row.config_value || '(空)' }}</span>
            <el-tag v-else type="warning" size="small">敏感配置</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="config_type" label="类型" width="80">
          <template #default="scope">
            <el-tag size="small">{{ scope.row.config_type }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="config_group" label="分组" width="120" />
        <el-table-column prop="remark" label="说明" min-width="200" show-overflow-tooltip />
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="scope">
            <el-button type="primary" link size="small" @click="openEdit(scope.row)">编辑</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openEdit(item)">编辑</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 编辑对话框 -->
    <el-dialog v-model="editDialogVisible" title="编辑配置" width="500px">
      <el-form label-position="top">
        <el-form-item label="配置 Key">
          <el-input :model-value="currentRow?.config_key" disabled />
        </el-form-item>
        <el-form-item label="名称">
          <el-input v-model="editForm.name" />
        </el-form-item>
        <el-form-item label="值">
          <el-input
            v-if="currentRow?.config_type !== 'json'"
            v-model="editForm.value"
            :type="isSensitive(currentRow?.config_key) ? 'password' : 'text'"
            show-password
            :rows="2"
          />
          <el-input
            v-else
            v-model="editForm.value"
            type="textarea"
            :rows="6"
            placeholder="JSON 格式"
          />
        </el-form-item>
        <el-form-item label="说明">
          <el-input v-model="editForm.remark" type="textarea" :rows="2" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="editLoading" @click="saveConfig">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { request } from '@/api/http'

interface SysConfigItem {
  id: number
  config_key: string
  config_value: string
  config_type: string
  config_name: string
  config_group: string
  remark: string
}

const list = ref<SysConfigItem[]>([])
const loading = ref(false)
const activeGroup = ref('')
const keyword = ref('')

const groups = ['system', 'security', 'jwt', 'totp', 'app', 'card', 'verify', 'pay', 'admin', 'tenant', 'agent']

const groupLabels: Record<string, string> = {
  system: '系统',
  security: '安全',
  jwt: 'JWT',
  totp: '2FA',
  app: '应用',
  card: '卡密',
  verify: '验证',
  pay: '支付',
  admin: '超管',
  tenant: '开发者',
  agent: '代理'
}

const groupLabel = (g: string) => groupLabels[g] || g

const filteredList = computed(() => {
  let arr = list.value
  if (keyword.value) {
    const k = keyword.value.toLowerCase()
    arr = arr.filter(item =>
      item.config_key.toLowerCase().includes(k) ||
      item.config_name.toLowerCase().includes(k) ||
      (item.remark || '').toLowerCase().includes(k)
    )
  }
  return arr
})

const mobileFields = [
  { prop: 'config_key', label: 'Key' },
  { prop: 'config_name', label: '名称' },
  { prop: 'config_value', label: '值', formatter: (v: string, row: any) => isSensitive(row.config_key) ? '敏感' : (v || '(空)') },
  { prop: 'config_group', label: '分组', formatter: (v: string) => groupLabel(v) }
]

const isSensitive = (key?: string) => {
  if (!key) return false
  const patterns = ['secret', 'password', 'private', 'token', 'aes_key']
  return patterns.some(p => key.toLowerCase().includes(p))
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await request.get<{ list: SysConfigItem[] }>('/admin/config', {
      group: activeGroup.value
    })
    list.value = resp.list || []
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const editDialogVisible = ref(false)
const editLoading = ref(false)
const currentRow = ref<SysConfigItem | null>(null)
const editForm = reactive({
  value: '',
  name: '',
  remark: ''
})

const openEdit = (row: any) => {
  currentRow.value = row
  editForm.value = row.config_value || ''
  editForm.name = row.config_name || ''
  editForm.remark = row.remark || ''
  editDialogVisible.value = true
}

const saveConfig = async () => {
  if (!currentRow.value) return
  editLoading.value = true
  try {
    await request.put(`/admin/config/${currentRow.value.config_key}`, {
      value: editForm.value,
      name: editForm.name,
      remark: editForm.remark
    })
    ElMessage.success('保存成功')
    editDialogVisible.value = false
    loadList()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    editLoading.value = false
  }
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.sys-config-page {
  .value-text {
    font-family: monospace;
    font-size: 13px;
    color: $color-text-primary;
    word-break: break-all;
  }
}
</style>
