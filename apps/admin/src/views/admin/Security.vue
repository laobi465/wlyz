<!--
  安全防护（超管）- 响应式 H5
  - 顶部 4 个统计卡 + 最近封禁 IP + IP 黑名单管理
  - 铁律 06：后端 501 时静默降级，不编造数据
-->
<template>
  <div class="security-page">
    <PageHeader title="安全防护" subtitle="IP 黑名单与登录封禁监控" />

    <!-- 顶部统计卡 -->
    <el-row :gutter="20" class="stat-row">
      <el-col :xs="12" :sm="6">
        <div class="stat-card">
          <div class="stat-title">IP 黑名单总数</div>
          <div class="stat-value">{{ stats.ip_blacklist_count ?? 0 }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6">
        <div class="stat-card">
          <div class="stat-title">生效中黑名单</div>
          <div class="stat-value">{{ stats.ip_blacklist_active ?? 0 }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6">
        <div class="stat-card">
          <div class="stat-title">今日登录失败</div>
          <div class="stat-value">{{ stats.failed_login_today ?? 0 }}</div>
        </div>
      </el-col>
      <el-col :xs="12" :sm="6">
        <div class="stat-card">
          <div class="stat-title">今日封禁 IP</div>
          <div class="stat-value">{{ stats.failed_login_blocked ?? 0 }}</div>
        </div>
      </el-col>
    </el-row>

    <!-- 下方两列布局 -->
    <el-row :gutter="20" class="main-row">
      <el-col :xs="24" :sm="12">
        <div class="app-card">
          <div class="card-header">
            <h3>最近封禁 IP</h3>
            <el-button text @click="loadStats">刷新</el-button>
          </div>
          <el-empty v-if="!recentBlocked.length" description="暂无数据" :image-size="60" />
          <div v-else class="block-list">
            <div v-for="(item, idx) in recentBlocked" :key="idx" class="block-item">
              <div class="block-ip">{{ item.ip || '-' }}</div>
              <div class="block-reason">{{ item.reason || '-' }}</div>
              <div class="block-time">{{ formatDate(item.blocked_at) }}</div>
            </div>
          </div>
        </div>
      </el-col>

      <el-col :xs="24" :sm="12">
        <div class="app-card">
          <div class="card-header">
            <h3>IP 黑名单管理</h3>
            <el-button type="primary" size="small" @click="openAdd">新增 IP</el-button>
          </div>
          <ResponsiveTable
            :data="blacklist"
            :loading="loading"
            :total="total"
            v-model:page="filter.page"
            v-model:pageSize="filter.page_size"
            :mobile-fields="mobileFields"
            @page-change="loadBlacklist"
            @size-change="loadBlacklist"
          >
            <el-table-column prop="id" label="ID" width="70" />
            <el-table-column prop="ip" label="IP" width="140" />
            <el-table-column prop="reason" label="原因" min-width="160" show-overflow-tooltip />
            <el-table-column prop="expire_at" label="过期时间" width="170">
              <template #default="{ row }">
                <span :class="{ 'expire-forever': !row.expire_at }">
                  {{ row.expire_at ? formatDate(row.expire_at) : '永久' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column prop="created_by" label="创建者" width="120" />
            <el-table-column prop="created_at" label="创建时间" width="170">
              <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
            </el-table-column>
            <el-table-column label="操作" width="90" fixed="right">
              <template #default="{ row }">
                <el-button type="danger" link size="small" @click="removeIp(row)">删除</el-button>
              </template>
            </el-table-column>

            <template #mobile-actions="{ item }">
              <el-button type="danger" size="small" @click="removeIp(item)">删除</el-button>
            </template>
          </ResponsiveTable>
        </div>
      </el-col>
    </el-row>

    <!-- 新增 IP 黑名单对话框 -->
    <el-dialog v-model="addDialogVisible" title="新增 IP 黑名单" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="IP 地址" prop="ip">
          <el-input v-model="form.ip" placeholder="如 1.2.3.4 或 CIDR 10.0.0.0/24" />
        </el-form-item>
        <el-form-item label="封禁原因" prop="reason">
          <el-input v-model="form.reason" type="textarea" :rows="3" placeholder="可选" />
        </el-form-item>
        <el-form-item label="过期小时数（0=永久）" prop="expire_hours">
          <el-input-number v-model="form.expire_hours" :min="0" :max="87600" :step="1" />
          <span class="hint">0 表示永久封禁</span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="addLoading" @click="confirmAdd">确认加入</el-button>
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
  adminSecurityStatsApi,
  listIpBlacklistApi,
  addIpBlacklistApi,
  removeIpBlacklistApi,
  type AdminSecurityStats,
  type IpBlacklistItem
} from '@/api/admin'

const stats = ref<AdminSecurityStats>({} as AdminSecurityStats)
const recentBlocked = ref<Array<{ ip: string; reason: string; blocked_at: string }>>([])

const blacklist = ref<IpBlacklistItem[]>([])
const total = ref(0)
const loading = ref(false)

const filter = reactive({
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'ip', label: 'IP' },
  { prop: 'reason', label: '原因' },
  { prop: 'expire_at', label: '过期', formatter: (v: string) => v ? formatDate(v) : '永久' }
]

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadStats = async () => {
  try {
    const resp = await adminSecurityStatsApi()
    stats.value = resp || ({} as AdminSecurityStats)
    recentBlocked.value = resp?.recent_blocked_ips || []
  } catch {
    // 错误已由 http 拦截器处理（后端 501 时静默降级，不编造数据）
  }
}

const loadBlacklist = async () => {
  loading.value = true
  try {
    const resp = await listIpBlacklistApi({
      page: filter.page,
      page_size: filter.page_size
    })
    blacklist.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

// ============== 新增 IP 黑名单 ==============
const addDialogVisible = ref(false)
const addLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  ip: '',
  reason: '',
  expire_hours: 24
})

const rules = {
  ip: [{ required: true, message: '请输入 IP 地址', trigger: 'blur' }]
}

const openAdd = () => {
  form.ip = ''
  form.reason = ''
  form.expire_hours = 24
  addDialogVisible.value = true
}

const confirmAdd = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (!form.ip) {
      ElMessage.warning('请输入 IP 地址')
      return
    }
    addLoading.value = true
    try {
      await addIpBlacklistApi({
        ip: form.ip,
        reason: form.reason,
        expire_hours: form.expire_hours
      })
      ElMessage.success('已加入黑名单')
      addDialogVisible.value = false
      loadBlacklist()
      loadStats()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      addLoading.value = false
    }
  })
}

const removeIp = (row: any) => {
  ElMessageBox.confirm(
    `确定要将 ${row.ip} 移出黑名单吗？`,
    '危险操作',
    {
      type: 'warning',
      confirmButtonText: '确认删除',
      cancelButtonText: '取消'
    }
  ).then(async () => {
    try {
      await removeIpBlacklistApi(row.id)
      ElMessage.success('已移出黑名单')
      loadBlacklist()
      loadStats()
    } catch {
      // 错误已由 http 拦截器处理
    }
  }).catch(() => {})
}

onMounted(() => {
  loadStats()
  loadBlacklist()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.security-page {
  .stat-row {
    margin-bottom: $spacing-lg;
    .stat-card {
      background: $color-bg-card;
      border: 1px solid $color-border-lighter;
      border-radius: $radius-md;
      padding: $spacing-md $spacing-lg;
      box-shadow: $shadow-card;
      margin-bottom: $spacing-sm;
      .stat-title {
        font-size: 13px;
        color: $color-text-secondary;
        margin-bottom: $spacing-sm;
      }
      .stat-value {
        font-size: 26px;
        font-weight: 600;
        color: $color-primary;
      }
    }
  }
  .main-row {
    .card-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: $spacing-md;
      h3 {
        margin: 0;
        font-size: 15px;
        font-weight: 600;
        color: $color-text-primary;
      }
    }
    .block-list {
      .block-item {
        display: flex;
        flex-direction: column;
        padding: $spacing-sm 0;
        border-bottom: 1px solid $color-border-lighter;
        &:last-child { border-bottom: none; }
        .block-ip {
          font-family: monospace;
          font-size: 14px;
          font-weight: 600;
          color: $color-text-primary;
        }
        .block-reason {
          font-size: 13px;
          color: $color-text-regular;
          margin-top: 2px;
          word-break: break-all;
        }
        .block-time {
          font-size: 12px;
          color: $color-text-secondary;
          margin-top: 2px;
        }
      }
    }
    .expire-forever {
      color: $color-danger;
      font-weight: 600;
    }
    .hint {
      font-size: 12px;
      color: $color-text-secondary;
      margin-left: $spacing-sm;
    }
    .el-input-number {
      width: 180px;
    }
  }
}
</style>
