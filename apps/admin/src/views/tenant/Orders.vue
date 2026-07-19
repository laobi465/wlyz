<!--
  订单管理（开发者）- 响应式
-->
<template>
  <div class="orders-page">
    <PageHeader title="订单管理" subtitle="订单与收入记录" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.app_id" placeholder="应用" clearable style="width: 160px" @change="loadList">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 120px" @change="loadList">
          <el-option label="已支付" value="paid" />
          <el-option label="待支付" value="pending" />
          <el-option label="已关闭" value="closed" />
          <el-option label="已退款" value="refunded" />
        </el-select>
        <el-select v-model="filter.channel" placeholder="渠道" clearable style="width: 120px" @change="loadList">
          <el-option label="H5" value="h5" />
          <el-option label="代理" value="agent" />
          <el-option label="手动" value="manual" />
        </el-select>
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          style="width: 240px"
          value-format="YYYY-MM-DD"
          @change="onDateChange"
        />
        <el-input v-model="filter.keyword" placeholder="订单号/买家" clearable style="width: 180px" @change="loadList" />
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
        <el-table-column prop="order_no" label="订单号" min-width="180">
          <template #default="{ row }: any">
            <span class="mono">{{ row.order_no }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="app_name" label="应用" min-width="120" />
        <el-table-column prop="card_type_name" label="卡类" min-width="120" />
        <el-table-column prop="buyer_username" label="买家" min-width="120" />
        <el-table-column prop="agent_username" label="代理" min-width="120">
          <template #default="{ row }: any">{{ row.agent_username || '-' }}</template>
        </el-table-column>
        <el-table-column prop="quantity" label="数量" width="80" />
        <el-table-column prop="unit_price" label="单价" width="100">
          <template #default="{ row }: any">¥{{ Number(row.unit_price).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="total_amount" label="总金额" width="100">
          <template #default="{ row }: any">¥{{ Number(row.total_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="commission_amount" label="佣金" width="100">
          <template #default="{ row }: any">¥{{ Number(row.commission_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="net_amount" label="净额" width="100">
          <template #default="{ row }: any">¥{{ Number(row.net_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="pay_status" label="状态" width="90">
          <template #default="{ row }: any">
            <el-tag :type="statusTag(row.pay_status)" size="small">{{ statusText(row.pay_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="channel" label="渠道" width="80">
          <template #default="{ row }: any">{{ channelText(row.channel) }}</template>
        </el-table-column>
        <el-table-column prop="paid_at" label="支付时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.paid_at) }}</template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="160">
          <template #default="{ row }: any">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
      </ResponsiveTable>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listTenantOrdersApi,
  type TenantOrder,
  type TenantOrderStatus,
  type TenantOrderChannel
} from '@/api/tenant'
import { listAppsApi, type App } from '@/api/apps'

const list = ref<TenantOrder[]>([])
const total = ref(0)
const loading = ref(false)
const apps = ref<App[]>([])

const filter = reactive({
  app_id: undefined as number | undefined,
  status: undefined as TenantOrderStatus | undefined,
  channel: undefined as TenantOrderChannel | undefined,
  start_date: '',
  end_date: '',
  keyword: '',
  page: 1,
  page_size: 20
})

const dateRange = ref<[string, string] | null>(null)

const mobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'app_name', label: '应用' },
  { prop: 'quantity', label: '数量' },
  { prop: 'total_amount', label: '总金额', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'pay_status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '创建', formatter: (v: string) => formatDate(v) }
]

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    paid: 'success',
    pending: 'warning',
    closed: 'info',
    refunded: 'danger'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    paid: '已支付',
    pending: '待支付',
    closed: '已关闭',
    refunded: '已退款'
  }
  return map[s] || s
}

const channelText = (c: string) => {
  const map: Record<string, string> = {
    h5: 'H5',
    agent: '代理',
    manual: '手动'
  }
  return map[c] || c || '-'
}

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const onDateChange = (val: [string, string] | null) => {
  if (val && val.length === 2) {
    filter.start_date = val[0]
    filter.end_date = val[1]
  } else {
    filter.start_date = ''
    filter.end_date = ''
  }
  loadList()
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
    const resp = await listTenantOrdersApi({
      app_id: filter.app_id,
      status: filter.status,
      channel: filter.channel,
      start_date: filter.start_date,
      end_date: filter.end_date,
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

onMounted(async () => {
  await loadApps()
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.orders-page {
  .mono {
    font-family: monospace;
    font-size: 13px;
    color: $color-text-primary;
  }
}
</style>
