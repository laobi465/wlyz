<!--
  更新管理（超管）- v0.9.0 新增
  - 顶部：GitHub Release 检查更新（当前版本 / 最新版本 / 检查按钮 / 立即更新按钮 / Release Notes）
  - 中部：当前部署状态（commit / 锁状态 / 自动开关 / 最近一次审计）
  - 下部：更新历史审计日志（分页列表）
  - 严格遵循铁律 04/05/06：所有配置走 sys_config；网络错误显式提示不编造数据；token 不回显
-->
<template>
  <div class="update-page">
    <PageHeader title="更新管理" subtitle="GitHub Release 检查更新 / 在线更新审计日志">
      <template #actions>
        <el-button :icon="Refresh" @click="loadAll">刷新</el-button>
      </template>
    </PageHeader>

    <!-- ============== 1. GitHub Release 检查更新 ============== -->
    <div class="app-card section-card">
      <div class="card-header">
        <h3>GitHub Release 检查</h3>
        <div class="card-actions">
          <el-button
            type="primary"
            size="small"
            :loading="checkLoading"
            :icon="Search"
            @click="checkUpdate"
          >
            检查更新
          </el-button>
          <el-button
            v-if="checkResult?.has_update"
            type="success"
            size="small"
            :icon="Upload"
            :loading="triggerLoading"
            :disabled="checkResult?.is_locked"
            @click="triggerUpdate"
          >
            立即更新
          </el-button>
        </div>
      </div>

      <!-- 配置缺失提示 -->
      <el-alert
        v-if="configError"
        :title="configError"
        type="warning"
        :closable="false"
        show-icon
        class="config-alert"
      >
        <template #default>
          <p>请到「系统配置 > update.github.*」配置以下参数后重试：</p>
          <ul class="config-hint">
            <li><code>update.github.owner</code> — GitHub 仓库 owner（如 laobi465）</li>
            <li><code>update.github.repo</code> — GitHub 仓库名（如 wlyz）</li>
            <li><code>update.github.current_version</code> — 当前部署版本号（如 v0.9.0）</li>
            <li><code>update.github.token</code> — GitHub Personal Access Token（可选，避免匿名限流）</li>
          </ul>
        </template>
      </el-alert>

      <!-- 检查结果 -->
      <div v-if="checkResult" class="check-result">
        <el-row :gutter="20">
          <el-col :xs="12" :sm="6">
            <div class="info-block">
              <div class="info-label">当前版本</div>
              <div class="info-value">
                <el-tag size="large">{{ checkResult.current_version || '未配置' }}</el-tag>
              </div>
            </div>
          </el-col>
          <el-col :xs="12" :sm="6">
            <div class="info-block">
              <div class="info-label">最新版本</div>
              <div class="info-value">
                <el-tag size="large" :type="checkResult.has_update ? 'success' : 'info'">
                  {{ checkResult.latest_version || '-' }}
                </el-tag>
              </div>
            </div>
          </el-col>
          <el-col :xs="12" :sm="6">
            <div class="info-block">
              <div class="info-label">是否有更新</div>
              <div class="info-value">
                <el-tag v-if="checkResult.has_update" type="warning" size="large">有新版本</el-tag>
                <el-tag v-else type="success" size="large">已是最新</el-tag>
              </div>
            </div>
          </el-col>
          <el-col :xs="12" :sm="6">
            <div class="info-block">
              <div class="info-label">仓库</div>
              <div class="info-value">
                <el-link
                  v-if="checkResult.repo_owner && checkResult.repo_name"
                  :href="`https://github.com/${checkResult.repo_owner}/${checkResult.repo_name}`"
                  target="_blank"
                  type="primary"
                >
                  {{ checkResult.repo_owner }}/{{ checkResult.repo_name }}
                </el-link>
                <span v-else>-</span>
              </div>
            </div>
          </el-col>
        </el-row>

        <el-row :gutter="20" class="meta-row">
          <el-col :xs="24" :sm="8">
            <div class="info-block small">
              <div class="info-label">发布时间</div>
              <div class="info-value">{{ formatDate(checkResult.published_at) }}</div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="8">
            <div class="info-block small">
              <div class="info-label">发布者</div>
              <div class="info-value">{{ checkResult.author || '-' }}</div>
            </div>
          </el-col>
          <el-col :xs="24" :sm="8">
            <div class="info-block small">
              <div class="info-label">检查时间</div>
              <div class="info-value">{{ formatTimestamp(checkResult.checked_at) }}</div>
            </div>
          </el-col>
        </el-row>

        <!-- 当前部署 commit -->
        <div v-if="checkResult.current_commit" class="commit-block">
          <span class="commit-label">当前部署 commit：</span>
          <code class="commit-hash">{{ checkResult.current_commit.slice(0, 12) }}</code>
          <el-tag v-if="checkResult.is_locked" type="danger" size="small" class="lock-tag">
            更新进行中（已锁定）
          </el-tag>
        </div>

        <!-- Release Notes -->
        <div v-if="checkResult.release_notes" class="release-notes">
          <div class="notes-title">
            更新内容
            <el-link
              v-if="checkResult.release_url"
              :href="checkResult.release_url"
              target="_blank"
              type="primary"
              class="notes-link"
            >
              查看 GitHub Release
            </el-link>
          </div>
          <pre class="notes-content">{{ checkResult.release_notes }}</pre>
        </div>
      </div>

      <!-- 未检查过 + 无错误时的初始提示 -->
      <el-empty
        v-else-if="!configError"
        description="点击「检查更新」从 GitHub 拉取最新 Release 信息"
        :image-size="80"
      />
    </div>

    <!-- ============== 2. 当前部署状态 ============== -->
    <div class="app-card section-card">
      <div class="card-header">
        <h3>当前部署状态</h3>
        <el-button text @click="loadStatus">刷新</el-button>
      </div>
      <el-row :gutter="20">
        <el-col :xs="12" :sm="6">
          <div class="info-block">
            <div class="info-label">当前 commit</div>
            <div class="info-value">
              <code v-if="status.current_commit">{{ status.current_commit.slice(0, 12) }}</code>
              <span v-else>-</span>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="6">
          <div class="info-block">
            <div class="info-label">更新锁</div>
            <div class="info-value">
              <el-tag v-if="status.is_locked" type="danger" size="small">锁定中</el-tag>
              <el-tag v-else type="success" size="small">空闲</el-tag>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="6">
          <div class="info-block">
            <div class="info-label">自动更新</div>
            <div class="info-value">
              <el-tag v-if="status.auto_update" type="success" size="small">已开启</el-tag>
              <el-tag v-else type="info" size="small">已关闭</el-tag>
            </div>
          </div>
        </el-col>
        <el-col :xs="12" :sm="6">
          <div class="info-block">
            <div class="info-label">目标分支</div>
            <div class="info-value">{{ status.branch || '-' }}</div>
          </div>
        </el-col>
      </el-row>

      <!-- 最近一次审计 -->
      <div v-if="status.latest_log" class="latest-log">
        <div class="log-line">
          <span class="log-label">最近一次更新：</span>
          <el-tag :type="getStatusTagType(status.latest_log.status)" size="small">
            {{ status.latest_log.status }}
          </el-tag>
          <span class="log-text">
            {{ status.latest_log.trigger_source }} ·
            {{ formatDate(status.latest_log.created_at) }} ·
            耗时 {{ (status.latest_log.duration_ms / 1000).toFixed(1) }}s
          </span>
        </div>
        <div v-if="status.latest_log.error_message" class="log-error">
          错误：{{ status.latest_log.error_message }}
        </div>
      </div>

      <!-- 成功/失败统计 -->
      <div class="stat-row">
        <el-tag type="success" size="small">成功 {{ status.success_count ?? 0 }} 次</el-tag>
        <el-tag type="danger" size="small">失败 {{ status.failed_count ?? 0 }} 次</el-tag>
      </div>
    </div>

    <!-- ============== 3. 更新历史 ============== -->
    <div class="app-card section-card">
      <div class="card-header">
        <h3>更新历史</h3>
        <el-button text @click="loadHistory">刷新</el-button>
      </div>
      <ResponsiveTable
        :data="history"
        :loading="historyLoading"
        :total="historyTotal"
        v-model:page="historyFilter.page"
        v-model:pageSize="historyFilter.page_size"
        :mobile-fields="historyMobileFields"
        @page-change="loadHistory"
        @size-change="loadHistory"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="status" label="状态" width="110">
          <template #default="{ row }">
            <el-tag :type="getStatusTagType(row.status)" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="trigger_source" label="触发源" width="100" />
        <el-table-column prop="branch" label="分支" width="120" show-overflow-tooltip />
        <el-table-column prop="commit_before" label="原 commit" width="130" show-overflow-tooltip>
          <template #default="{ row }">
            <code v-if="row.commit_before">{{ String(row.commit_before).slice(0, 8) }}</code>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="commit_after" label="新 commit" width="130" show-overflow-tooltip>
          <template #default="{ row }">
            <code v-if="row.commit_after">{{ String(row.commit_after).slice(0, 8) }}</code>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="duration_ms" label="耗时" width="100">
          <template #default="{ row }">
            {{ row.duration_ms ? (row.duration_ms / 1000).toFixed(1) + 's' : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="error_message" label="错误信息" min-width="180" show-overflow-tooltip />
        <el-table-column prop="created_at" label="时间" width="170">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button size="small" @click="viewLogDetail(item)">详情</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- ============== 详情对话框 ============== -->
    <el-dialog
      v-model="detailDialogVisible"
      title="更新日志详情"
      :width="isMobile ? '92%' : '720px'"
      destroy-on-close
    >
      <div v-if="currentLog" class="log-detail">
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="ID">{{ currentLog.id }}</el-descriptions-item>
          <el-descriptions-item label="状态">
            <el-tag :type="getStatusTagType(currentLog.status)" size="small">{{ currentLog.status }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="触发源">{{ currentLog.trigger_source }}</el-descriptions-item>
          <el-descriptions-item label="分支">{{ currentLog.branch || '-' }}</el-descriptions-item>
          <el-descriptions-item label="触发者 ID">{{ currentLog.trigger_by || '-' }}</el-descriptions-item>
          <el-descriptions-item label="触发 IP">{{ currentLog.trigger_ip || '-' }}</el-descriptions-item>
          <el-descriptions-item label="原 commit">
            <code v-if="currentLog.commit_before">{{ currentLog.commit_before.slice(0, 12) }}</code>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="新 commit">
            <code v-if="currentLog.commit_after">{{ currentLog.commit_after.slice(0, 12) }}</code>
            <span v-else>-</span>
          </el-descriptions-item>
          <el-descriptions-item label="耗时">
            {{ currentLog.duration_ms ? (currentLog.duration_ms / 1000).toFixed(1) + 's' : '-' }}
          </el-descriptions-item>
          <el-descriptions-item label="时间">{{ formatDate(currentLog.created_at) }}</el-descriptions-item>
        </el-descriptions>

        <div v-if="currentLog.error_message" class="detail-error">
          <strong>错误信息：</strong>{{ currentLog.error_message }}
        </div>

        <div v-if="currentLog.log_text" class="detail-log-text">
          <strong>执行日志：</strong>
          <pre>{{ currentLog.log_text }}</pre>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Refresh, Search, Upload } from '@element-plus/icons-vue'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  checkUpdateApi,
  updateStatusApi,
  triggerUpdateApi,
  listUpdateHistoryApi,
  getUpdateLogApi,
  type CheckUpdateResult,
  type UpdateStatus,
  type UpdateLog
} from '@/api/update'

// v0.7.0 修复：dialog 响应式宽度
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }

// ============== 1. GitHub Release 检查 ==============
const checkLoading = ref(false)
const checkResult = ref<CheckUpdateResult | null>(null)
const configError = ref<string>('')
const triggerLoading = ref(false)

const checkUpdate = async () => {
  checkLoading.value = true
  configError.value = ''
  try {
    const data = await checkUpdateApi()
    checkResult.value = data
    if (data.has_update) {
      ElMessage.success(`检测到新版本：${data.latest_version}`)
    } else if (data.latest_version) {
      ElMessage.info('当前已是最新版本')
    }
  } catch (err: any) {
    // 后端返回 502 时通常是配置缺失或 GitHub API 异常
    const msg = err?.message || err?.toString() || ''
    if (msg.includes('未配置') || msg.includes('GitHub 仓库')) {
      configError.value = msg
    } else {
      ElMessage.error('检查更新失败：' + msg)
    }
  } finally {
    checkLoading.value = false
  }
}

const triggerUpdate = async () => {
  if (!checkResult.value?.has_update) return
  if (checkResult.value.is_locked) {
    ElMessage.warning('已有更新在进行中，请等待完成')
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认立即触发更新？将拉取最新代码并重新部署，期间服务可能短暂不可用。`,
      '更新确认',
      { type: 'warning', confirmButtonText: '确认更新', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  triggerLoading.value = true
  try {
    const resp = await triggerUpdateApi({})
    ElMessage.success(resp?.message || '更新已触发，请通过历史列表查看进度')
    // 刷新状态与历史
    await Promise.all([loadStatus(), loadHistory()])
  } catch (err: any) {
    ElMessage.error('触发更新失败：' + (err?.message || err?.toString() || ''))
  } finally {
    triggerLoading.value = false
  }
}

// ============== 2. 当前部署状态 ==============
const status = ref<UpdateStatus>({} as UpdateStatus)

const loadStatus = async () => {
  try {
    status.value = await updateStatusApi()
  } catch {
    // 错误已由 http 拦截器处理
  }
}

// ============== 3. 更新历史 ==============
const history = ref<UpdateLog[]>([])
const historyTotal = ref(0)
const historyLoading = ref(false)
const historyFilter = reactive({
  page: 1,
  page_size: 10
})

const historyMobileFields = [
  { prop: 'id', label: 'ID' },
  { prop: 'status', label: '状态' },
  { prop: 'trigger_source', label: '触发源' },
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) }
]

const loadHistory = async () => {
  historyLoading.value = true
  try {
    const resp = await listUpdateHistoryApi({
      page: historyFilter.page,
      page_size: historyFilter.page_size
    })
    history.value = resp.list || []
    historyTotal.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    historyLoading.value = false
  }
}

// ============== 详情对话框 ==============
const detailDialogVisible = ref(false)
const currentLog = ref<UpdateLog | null>(null)

const viewLogDetail = async (row: UpdateLog) => {
  try {
    // 拉取完整详情（含 log_text）
    currentLog.value = await getUpdateLogApi(row.id)
    detailDialogVisible.value = true
  } catch {
    // 错误已由 http 拦截器处理
  }
}

// ============== 辅助函数 ==============
const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  const d = new Date(s)
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN')
}

const formatTimestamp = (ts: number | null | undefined) => {
  if (!ts) return '-'
  return formatDate(new Date(ts * 1000).toISOString())
}

const getStatusTagType = (status: string): 'success' | 'danger' | 'warning' | 'info' => {
  switch (status) {
    case 'success': return 'success'
    case 'failed': return 'danger'
    case 'rolled_back': return 'warning'
    case 'running': return 'warning'
    case 'pending': return 'info'
    default: return 'info'
  }
}

const loadAll = async () => {
  await Promise.all([loadStatus(), loadHistory()])
}

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  loadAll()
  // 不自动调 checkUpdate：避免页面打开就消耗 GitHub API 额度（防限流）
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.update-page {
  display: flex;
  flex-direction: column;
  gap: $spacing-md;
}

.section-card {
  padding: $spacing-md;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: $spacing-md;
  flex-wrap: wrap;
  gap: $spacing-sm;

  h3 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    color: $color-text-primary;
  }

  .card-actions {
    display: flex;
    gap: $spacing-sm;
    flex-wrap: wrap;
  }
}

.config-alert {
  margin-bottom: $spacing-md;

  .config-hint {
    margin: $spacing-sm 0 0;
    padding-left: $spacing-lg;
    font-size: 13px;

    code {
      background: $color-bg-hover;
      padding: 2px 6px;
      border-radius: $radius-sm;
      font-size: 12px;
      color: $color-primary;
    }
  }
}

.check-result {
  display: flex;
  flex-direction: column;
  gap: $spacing-md;

  .meta-row {
    .info-block.small {
      .info-label {
        font-size: 12px;
      }
      .info-value {
        font-size: 13px;
      }
    }
  }
}

.info-block {
  margin-bottom: $spacing-sm;

  .info-label {
    font-size: 13px;
    color: $color-text-secondary;
    margin-bottom: 4px;
  }

  .info-value {
    font-size: 14px;
    color: $color-text-primary;
    display: flex;
    align-items: center;
    gap: $spacing-xs;
    flex-wrap: wrap;

    code {
      background: $color-bg-hover;
      padding: 2px 6px;
      border-radius: $radius-sm;
      font-family: 'Menlo', 'Monaco', monospace;
      font-size: 12px;
      color: $color-primary;
    }
  }
}

.commit-block {
  padding: $spacing-sm $spacing-md;
  background: $color-bg-hover;
  border-radius: $radius-md;
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  flex-wrap: wrap;

  .commit-label {
    font-size: 13px;
    color: $color-text-secondary;
  }

  .commit-hash {
    background: transparent;
    font-family: 'Menlo', 'Monaco', monospace;
    font-size: 13px;
    color: $color-primary;
  }

  .lock-tag {
    margin-left: auto;
  }
}

.release-notes {
  border-top: 1px solid $color-border-lighter;
  padding-top: $spacing-md;

  .notes-title {
    font-size: 14px;
    font-weight: 600;
    color: $color-text-primary;
    margin-bottom: $spacing-sm;
    display: flex;
    align-items: center;
    gap: $spacing-md;

    .notes-link {
      font-size: 13px;
      font-weight: normal;
    }
  }

  .notes-content {
    background: $color-bg-hover;
    padding: $spacing-md;
    border-radius: $radius-md;
    font-family: 'Menlo', 'Monaco', monospace;
    font-size: 13px;
    color: $color-text-regular;
    white-space: pre-wrap;
    word-break: break-word;
    max-height: 400px;
    overflow-y: auto;
    margin: 0;
    line-height: 1.6;
  }
}

.latest-log {
  margin-top: $spacing-md;
  padding: $spacing-sm $spacing-md;
  background: $color-bg-hover;
  border-radius: $radius-md;

  .log-line {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    flex-wrap: wrap;
    font-size: 13px;

    .log-label {
      color: $color-text-secondary;
    }

    .log-text {
      color: $color-text-regular;
    }
  }

  .log-error {
    margin-top: $spacing-sm;
    font-size: 13px;
    color: $color-danger;
  }
}

.stat-row {
  margin-top: $spacing-sm;
  display: flex;
  gap: $spacing-sm;
  flex-wrap: wrap;
}

.log-detail {
  .detail-error {
    margin-top: $spacing-md;
    padding: $spacing-sm $spacing-md;
    background: var(--color-danger-light, #fef0f0);
    border-radius: $radius-md;
    color: $color-danger;
    font-size: 13px;
  }

  .detail-log-text {
    margin-top: $spacing-md;

    strong {
      display: block;
      margin-bottom: $spacing-sm;
      color: $color-text-primary;
      font-size: 13px;
    }

    pre {
      background: $color-bg-hover;
      padding: $spacing-md;
      border-radius: $radius-md;
      font-family: 'Menlo', 'Monaco', monospace;
      font-size: 12px;
      color: $color-text-regular;
      white-space: pre-wrap;
      word-break: break-word;
      max-height: 360px;
      overflow-y: auto;
      margin: 0;
      line-height: 1.5;
    }
  }
}

@include mobile {
  .section-card {
    padding: $spacing-sm;
  }

  .release-notes .notes-content {
    padding: $spacing-sm;
    font-size: 12px;
  }
}
</style>
