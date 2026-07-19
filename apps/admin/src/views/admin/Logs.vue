<!--
  日志审计（超管）- 响应式 H5
  - 列表展示平台操作日志（登录/操作/支付/安全/系统）
  - 铁律 06：后端 501 时静默降级，不编造数据
-->
<template>
  <div class="logs-page">
    <PageHeader title="日志审计" subtitle="查询平台所有操作日志" />

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.type" placeholder="日志类型" clearable style="width: 140px" @change="loadList">
          <el-option label="登录" value="login" />
          <el-option label="操作" value="operation" />
          <el-option label="支付" value="pay" />
          <el-option label="安全" value="security" />
          <el-option label="系统" value="system" />
        </el-select>
        <el-input v-model="filter.user_id_input" placeholder="用户 ID" clearable style="width: 140px" @change="loadList" />
        <el-date-picker
          v-model="dateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 280px"
          @change="onDateChange"
        />
        <el-input v-model="filter.keyword" placeholder="关键词（动作/目标）" clearable style="width: 200px" @change="loadList" />
        <el-button @click="loadList">刷新</el-button>
      </div>

      <ResponsiveTable
        :data="sortedList"
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
            <el-tag :type="typeTag(row.type)" size="small">{{ typeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" width="140" />
        <el-table-column prop="role" label="角色" width="100" />
        <el-table-column prop="action" label="动作" min-width="160" show-overflow-tooltip />
        <el-table-column prop="target" label="目标" min-width="160" show-overflow-tooltip />
        <el-table-column prop="ip" label="IP" width="140" />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="详情" width="100" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openDetail(row)">查看</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openDetail(item)">查看详情</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 详情对话框 -->
    <el-dialog v-model="detailVisible" title="日志详情" width="500px">
      <div v-if="currentRow" class="detail-box">
        <div class="detail-row"><span class="label">ID：</span><span>{{ currentRow.id }}</span></div>
        <div class="detail-row"><span class="label">类型：</span><span>{{ typeText(currentRow.type) }}</span></div>
        <div class="detail-row"><span class="label">用户名：</span><span>{{ currentRow.username || '-' }}</span></div>
        <div class="detail-row"><span class="label">角色：</span><span>{{ currentRow.role || '-' }}</span></div>
        <div class="detail-row"><span class="label">动作：</span><span>{{ currentRow.action || '-' }}</span></div>
        <div class="detail-row"><span class="label">目标：</span><span>{{ currentRow.target || '-' }}</span></div>
        <div class="detail-row"><span class="label">IP：</span><span>{{ currentRow.ip || '-' }}</span></div>
        <div class="detail-row"><span class="label">状态：</span>
          <el-tag :type="statusTag(currentRow.status)" size="small">{{ statusText(currentRow.status) }}</el-tag>
        </div>
        <div class="detail-row"><span class="label">UA：</span><span class="mono">{{ currentRow.user_agent || '-' }}</span></div>
        <div class="detail-row"><span class="label">时间：</span><span>{{ formatDate(currentRow.created_at) }}</span></div>
        <div class="detail-row detail-block">
          <span class="label">详情：</span>
          <pre class="detail-pre">{{ formatDetail(currentRow.detail) }}</pre>
        </div>
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { listAdminLogsApi, type AdminLog, type AdminLogType } from '@/api/admin'

const list = ref<AdminLog[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  type: undefined as AdminLogType | undefined,
  user_id: undefined as number | undefined,
  user_id_input: '',
  start_date: '' as string,
  end_date: '' as string,
  keyword: '',
  page: 1,
  page_size: 20
})

const dateRange = ref<[string, string] | null>(null)

const onDateChange = (val: any) => {
  if (val && val.length === 2) {
    filter.start_date = val[0]
    filter.end_date = val[1]
  } else {
    filter.start_date = ''
    filter.end_date = ''
  }
  loadList()
}

// 默认按创建时间倒序（前端排序兜底，后端 501 时为空也安全）
const sortedList = computed(() => {
  return [...list.value].sort((a, b) => {
    const ta = a.created_at ? new Date(a.created_at).getTime() : 0
    const tb = b.created_at ? new Date(b.created_at).getTime() : 0
    return tb - ta
  })
})

const mobileFields = [
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) },
  { prop: 'username', label: '用户名' },
  { prop: 'type', label: '类型', formatter: (v: string) => typeText(v) },
  { prop: 'action', label: '动作' },
  { prop: 'ip', label: 'IP' }
]

const typeTag = (t: string): any => {
  const map: Record<string, any> = {
    login: 'info',
    operation: 'primary',
    pay: 'warning',
    security: 'danger',
    system: 'info'
  }
  return map[t] || 'info'
}

const typeText = (t: string) => {
  const map: Record<string, string> = {
    login: '登录',
    operation: '操作',
    pay: '支付',
    security: '安全',
    system: '系统'
  }
  return map[t] || t || '-'
}

const statusTag = (s: string): any => {
  return s === 'success' ? 'success' : 'danger'
}

const statusText = (s: string) => {
  return s === 'success' ? '成功' : (s === 'fail' ? '失败' : (s || '-'))
}

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const formatDetail = (s: string | null | undefined) => {
  if (!s) return '-'
  // 尝试 JSON 美化
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

const loadList = async () => {
  loading.value = true
  // user_id_input 转 user_id
  if (filter.user_id_input) {
    const n = Number(filter.user_id_input)
    filter.user_id = isNaN(n) ? undefined : n
  } else {
    filter.user_id = undefined
  }
  try {
    const resp = await listAdminLogsApi({
      type: filter.type,
      user_id: filter.user_id,
      start_date: filter.start_date || undefined,
      end_date: filter.end_date || undefined,
      keyword: filter.keyword || undefined,
      page: filter.page,
      page_size: filter.page_size
    })
    list.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理（后端 501 时静默降级，不编造数据）
  } finally {
    loading.value = false
  }
}

const detailVisible = ref(false)
const currentRow = ref<AdminLog | null>(null)

const openDetail = (row: any) => {
  currentRow.value = row
  detailVisible.value = true
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.logs-page {
  .detail-box {
    .detail-row {
      display: flex;
      padding: 6px 0;
      font-size: 13px;
      border-bottom: 1px solid $color-border-lighter;
      word-break: break-all;
      .label {
        width: 70px;
        color: $color-text-secondary;
        flex-shrink: 0;
      }
    }
    .detail-block {
      flex-direction: column;
      .label {
        width: auto;
        margin-bottom: $spacing-sm;
      }
    }
    .detail-pre {
      margin: 0;
      padding: $spacing-sm;
      background: $color-bg-page;
      border-radius: $radius-sm;
      font-family: monospace;
      font-size: 12px;
      color: $color-text-primary;
      max-height: 260px;
      overflow: auto;
      white-space: pre-wrap;
      word-break: break-all;
    }
    .mono {
      font-family: monospace;
      font-size: 12px;
      word-break: break-all;
    }
  }
}
</style>
