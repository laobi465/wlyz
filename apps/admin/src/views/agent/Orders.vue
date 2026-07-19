<!--
  代理订单（响应式 H5）
  - 状态筛选 + 列表 + 分页
  - 每条订单显示：订单号 / 卡类 / 数量 / 金额 / 佣金 / 状态 / 时间
  铁律 06 待核实：后端 /agent/orders 当前为 501 占位（v0.3.0 交付）。
-->
<template>
  <div class="agent-orders-page">
    <PageHeader title="我的订单" subtitle="查看所有购卡订单及佣金" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.status" placeholder="订单状态" clearable style="width: 140px" @change="loadList">
          <el-option label="已支付" value="paid" />
          <el-option label="待支付" value="pending" />
          <el-option label="已关闭" value="closed" />
          <el-option label="已退款" value="refunded" />
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
        <el-table-column prop="order_no" label="订单号" min-width="180" />
        <el-table-column prop="card_type_name" label="卡类" min-width="140" />
        <el-table-column prop="app_name" label="应用" min-width="120" />
        <el-table-column prop="quantity" label="数量" width="80" />
        <el-table-column prop="total_amount" label="订单金额" width="120">
          <template #default="scope">¥{{ Number(scope.row.total_amount).toFixed(2) }}</template>
        </el-table-column>
        <el-table-column prop="commission_amount" label="佣金" width="100">
          <template #default="scope">
            <span class="text-success">¥{{ Number(scope.row.commission_amount).toFixed(2) }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="pay_status" label="状态" width="100">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.pay_status)" size="small">{{ statusText(scope.row.pay_status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="pay_channel" label="渠道" width="100" />
        <el-table-column prop="created_at" label="下单时间" width="170">
          <template #default="scope">{{ formatDate(scope.row.created_at) }}</template>
        </el-table-column>
        <el-table-column prop="paid_at" label="支付时间" width="170">
          <template #default="scope">{{ formatDate(scope.row.paid_at) }}</template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button size="small" @click="copyOrderNo(item.order_no)">复制订单号</el-button>
        </template>
      </ResponsiveTable>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { listAgentOrdersApi, type AgentOrder, type AgentOrderStatus } from '@/api/agent'

const list = ref<AgentOrder[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  status: undefined as AgentOrderStatus | undefined,
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'order_no', label: '订单号' },
  { prop: 'card_type_name', label: '卡类' },
  { prop: 'quantity', label: '数量' },
  { prop: 'total_amount', label: '金额', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'commission_amount', label: '佣金', formatter: (v: number) => '¥' + Number(v).toFixed(2) },
  { prop: 'pay_status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'created_at', label: '下单', formatter: (v: string) => formatDate(v) }
]

const statusTag = (s: string): any => ({
  paid: 'success',
  pending: 'warning',
  closed: 'info',
  refunded: 'danger'
}[s] || 'info')

const statusText = (s: string) => ({
  paid: '已支付',
  pending: '待支付',
  closed: '已关闭',
  refunded: '已退款'
}[s] || s)

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAgentOrdersApi({
      status: filter.status,
      page: filter.page,
      page_size: filter.page_size
    })
    list.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 铁律 04 不编造数据
  } finally {
    loading.value = false
  }
}

const copyOrderNo = (orderNo: string) => {
  navigator.clipboard.writeText(orderNo).then(() => ElMessage.success('已复制订单号'))
}

onMounted(loadList)
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.app-card {
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-md;
  box-shadow: $shadow-card;
}

.search-bar {
  display: flex;
  gap: $spacing-sm;
  margin-bottom: $spacing-md;
  flex-wrap: wrap;

  @include mobile {
    .el-select { width: 100% !important; }
  }
}

.text-success {
  color: $color-success;
  font-weight: 500;
}
</style>
