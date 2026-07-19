<!--
  设备管理（开发者）- 响应式
-->
<template>
  <div class="devices-page">
    <PageHeader title="设备管理" subtitle="在线设备与绑定设备管理" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.app_id" placeholder="应用" clearable style="width: 160px" @change="loadList">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-input v-model="filter.keyword" placeholder="设备名/设备ID/IP" clearable style="width: 200px" @change="loadList" />
        <el-select v-model="filter.online" placeholder="在线状态" clearable style="width: 120px" @change="loadList">
          <el-option label="在线" :value="true" />
          <el-option label="离线" :value="false" />
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
        <el-table-column prop="card_key" label="卡密" min-width="160">
          <template #default="{ row }: any">
            <span class="mono">{{ maskKey(row.card_key) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="device_name" label="设备名" min-width="120" />
        <el-table-column prop="device_id" label="设备 ID" min-width="160">
          <template #default="{ row }: any">
            <span class="mono">{{ truncate(row.device_id, 16) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="ip" label="IP" width="130" />
        <el-table-column prop="location" label="位置" min-width="120" />
        <el-table-column prop="heartbeat_at" label="心跳时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.heartbeat_at) }}</template>
        </el-table-column>
        <el-table-column prop="is_online" label="状态" width="80">
          <template #default="{ row }: any">
            <el-tag :type="row.is_online ? 'success' : 'info'" size="small">
              {{ row.is_online ? '在线' : '离线' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="120" fixed="right">
          <template #default="{ row }: any">
            <el-button type="danger" link size="small" @click="kickDevice(row)">强制下线</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="danger" size="small" @click="kickDevice(item)">强制下线</el-button>
        </template>
      </ResponsiveTable>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { listTenantDevicesApi, kickTenantDeviceApi, type TenantDevice } from '@/api/tenant'
import { listAppsApi, type App } from '@/api/apps'

const list = ref<TenantDevice[]>([])
const total = ref(0)
const loading = ref(false)
const apps = ref<App[]>([])

const filter = reactive({
  app_id: undefined as number | undefined,
  keyword: '',
  online: undefined as boolean | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'app_name', label: '应用' },
  { prop: 'card_key', label: '卡密', formatter: (v: string) => maskKey(v) },
  { prop: 'device_name', label: '设备名' },
  { prop: 'is_online', label: '状态', formatter: (v: boolean) => v ? '在线' : '离线' },
  { prop: 'heartbeat_at', label: '心跳', formatter: (v: string) => formatDate(v) }
]

const maskKey = (key: string) => {
  if (!key) return '-'
  if (key.length <= 8) return key.slice(0, 4) + '****'
  return key.slice(0, 6) + '****' + key.slice(-4)
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
    const resp = await listTenantDevicesApi({
      app_id: filter.app_id,
      keyword: filter.keyword,
      online: filter.online,
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

const kickDevice = (row: any) => {
  ElMessageBox.confirm(
    `确定要将设备「${row.device_name || row.device_id}」强制下线吗？`,
    '强制下线',
    {
      type: 'warning',
      confirmButtonText: '确认下线',
      cancelButtonText: '取消'
    }
  ).then(async () => {
    try {
      await kickTenantDeviceApi(row.id)
      ElMessage.success('已强制下线')
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

.devices-page {
  .mono {
    font-family: monospace;
    font-size: 13px;
    color: $color-text-primary;
  }
}
</style>
