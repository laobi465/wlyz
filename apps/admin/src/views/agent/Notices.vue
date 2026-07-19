<!--
  代理消息通知（响应式 H5）
  - 类型筛选 + 仅未读开关 + 刷新
  - 顶部统计小卡：未读消息数（前端计算 list.filter(!read).length）
  - 列表：ID / 类型 / 标题 / 置顶 / 已读 / 发布时间 / 操作
  - 查看对话框：完整内容（标题 + 类型 + 发布时间 + 内容 textarea readonly）
  - 标为已读：仅未读状态显示，调用 readAgentNoticeApi
  v0.3.1 已交付 /agent/notices + /agent/notices/:id/read。
-->
<template>
  <div class="notices-page">
    <PageHeader title="消息通知" subtitle="平台公告与开发者通知" />

    <!-- 未读统计小卡 -->
    <div class="stat-grid">
      <div class="stat-card unread">
        <div class="stat-label">未读消息</div>
        <div class="stat-value">{{ unreadCount }}</div>
        <div class="stat-extra">当前页共 {{ list.length }} 条</div>
      </div>
    </div>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.type" placeholder="类型" clearable style="width: 140px" @change="loadList">
          <el-option label="平台公告" value="platform" />
          <el-option label="租户通知" value="tenant" />
          <el-option label="代理通知" value="agent" />
        </el-select>
        <el-switch v-model="filter.unread_only" active-text="仅未读" @change="loadList" />
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
          <template #default="{ row }: { row: any }">
            <el-tag :type="typeTag(row.type)" size="small">{{ typeText(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="title" label="标题" min-width="200" />
        <el-table-column prop="pinned" label="置顶" width="80">
          <template #default="{ row }: { row: any }">
            <el-tag v-if="row.pinned" type="danger" size="small">置顶</el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="read" label="已读" width="90">
          <template #default="{ row }: { row: any }">
            <el-tag :type="row.read ? 'info' : 'warning'" size="small">{{ row.read ? '已读' : '未读' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="publish_at" label="发布时间" width="170">
          <template #default="{ row }: { row: any }">{{ formatDate(row.publish_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }: { row: any }">
            <el-button type="primary" link size="small" @click="openDetail(row)">查看</el-button>
            <el-button v-if="!row.read" type="success" link size="small" @click="markRead(row)">标为已读</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button size="small" @click="openDetail(item)">查看</el-button>
          <el-button v-if="!item.read" type="success" size="small" @click="markRead(item)">标为已读</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 查看通知对话框 -->
    <el-dialog v-model="detailVisible" title="通知详情" width="500px">
      <el-form label-position="top">
        <el-form-item label="标题">
          <span class="detail-text">{{ current?.title || '-' }}</span>
        </el-form-item>
        <el-form-item label="类型">
          <el-tag :type="typeTag(current?.type)" size="small">{{ typeText(current?.type) }}</el-tag>
        </el-form-item>
        <el-form-item label="发布时间">
          <span class="detail-text">{{ formatDate(current?.publish_at) }}</span>
        </el-form-item>
        <el-form-item label="内容">
          <el-input
            type="textarea"
            :model-value="current?.content || ''"
            readonly
            :rows="6"
            resize="none"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listAgentNoticesApi,
  readAgentNoticeApi,
  type AgentNotice
} from '@/api/agent'

const list = ref<AgentNotice[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  type: undefined as string | undefined,
  unread_only: false,
  page: 1,
  page_size: 20
})

const unreadCount = computed(() => list.value.filter(n => !n.read).length)

const mobileFields = [
  { prop: 'title', label: '标题' },
  { prop: 'type', label: '类型', formatter: (v: string) => typeText(v) },
  { prop: 'read', label: '已读', formatter: (v: boolean) => (v ? '已读' : '未读') },
  { prop: 'publish_at', label: '发布时间', formatter: (v: string) => formatDate(v) }
]

const detailVisible = ref(false)
const current = ref<AgentNotice | null>(null)

const typeTag = (s?: string): any => ({
  platform: 'primary',
  tenant: 'success',
  agent: 'warning'
}[s || ''] || 'info')

const typeText = (s?: string) => ({
  platform: '平台公告',
  tenant: '租户通知',
  agent: '代理通知'
}[s || ''] || (s || '-'))

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listAgentNoticesApi({
      type: filter.type,
      unread_only: filter.unread_only,
      page: filter.page,
      page_size: filter.page_size
    })
    list.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    /* 错误已由 http 拦截器处理 */
  } finally {
    loading.value = false
  }
}

const openDetail = (row: AgentNotice) => {
  current.value = row
  detailVisible.value = true
}

const markRead = async (row: AgentNotice) => {
  try {
    await readAgentNoticeApi(row.id)
    row.read = true
    ElMessage.success('已标记为已读')
  } catch {
    /* 错误已由 http 拦截器处理 */
  }
}

onMounted(loadList)
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.notices-page {
  .stat-grid {
    display: grid;
    grid-template-columns: repeat(1, 1fr);
    gap: $spacing-md;
    margin-bottom: $spacing-lg;

    @include mobile {
      gap: $spacing-sm;
    }
  }

  .stat-card {
    background: $color-bg-card;
    border-radius: $radius-md;
    padding: $spacing-md $spacing-lg;
    box-shadow: $shadow-card;
    border-left: 4px solid $color-warning;

    .stat-label {
      font-size: 13px;
      color: $color-text-secondary;
    }
    .stat-value {
      font-size: 24px;
      font-weight: 600;
      color: $color-text-primary;
      margin: 6px 0 4px;
      font-family: 'SF Mono', 'Menlo', monospace;
    }
    .stat-extra {
      font-size: 12px;
      color: $color-text-secondary;
    }

    @include mobile {
      padding: $spacing-sm $spacing-md;
      .stat-value { font-size: 18px; }
    }
  }

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
    align-items: center;
    flex-wrap: wrap;

    @include mobile {
      .el-select { width: 100% !important; }
    }
  }

  .detail-text {
    color: $color-text-primary;
    font-weight: 500;
  }
}
</style>
