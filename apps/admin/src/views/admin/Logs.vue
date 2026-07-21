<!--
  日志审计（超管）- 响应式 H5 - v0.3.3 升级
  - 三表独立查询：操作日志 / 验证日志 / 登录失败日志
  - 每表独立筛选条件，顶部「导出 CSV」按钮（按当前 Tab 导出，含 UTF-8 BOM）
  - 铁律 06：后端异常时静默降级，不编造数据
-->
<template>
  <div class="logs-page">
    <PageHeader title="日志审计" subtitle="操作日志 / 验证日志 / 登录失败日志" />

    <div class="app-card">
      <el-tabs v-model="activeTab" @tab-change="onTabChange">
        <el-tab-pane label="操作日志" name="operation" />
        <el-tab-pane label="验证日志" name="verify" />
        <el-tab-pane label="登录失败日志" name="login_failed" />
      </el-tabs>

      <!-- 顶部操作条：导出按钮 -->
      <div class="action-bar">
        <el-button type="success" :loading="exporting" @click="exportCsv">
          导出 CSV（当前{{ tabLabel }}，最多 10000 条）
        </el-button>
      </div>

      <!-- 操作日志筛选 -->
      <div v-if="activeTab === 'operation'" class="search-bar">
        <el-select v-model="opFilter.operator_type" placeholder="操作者类型" clearable style="width: 140px" @change="onFilterChange">
          <el-option label="超管" value="admin" />
          <el-option label="开发者" value="tenant" />
          <el-option label="代理" value="agent" />
        </el-select>
        <el-input v-model="opFilter.operator_id_input" placeholder="操作者 ID" clearable style="width: 130px" @change="onFilterChange" />
        <el-input v-model="opFilter.module" placeholder="模块（如 agent/card）" clearable style="width: 180px" @change="onFilterChange" />
        <el-input v-model="opFilter.action" placeholder="动作（如 create/delete）" clearable style="width: 180px" @change="onFilterChange" />
        <el-select v-model="opFilter.status" placeholder="状态" clearable style="width: 110px" @change="onFilterChange">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="fail" />
        </el-select>
        <el-date-picker
          v-model="opDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 280px"
          @change="(v: any) => onDateChange(v, 'operation')"
        />
        <el-input v-model="opFilter.keyword" placeholder="关键词（action/module/username/ip）" clearable style="width: 240px" @change="onFilterChange" />
        <el-button @click="loadList">刷新</el-button>
      </div>

      <!-- 验证日志筛选 -->
      <div v-else-if="activeTab === 'verify'" class="search-bar">
        <el-input v-model="vFilter.tenant_id_input" placeholder="租户 ID" clearable style="width: 120px" @change="onFilterChange" />
        <el-input v-model="vFilter.app_id_input" placeholder="应用 ID" clearable style="width: 120px" @change="onFilterChange" />
        <el-select v-model="vFilter.action" placeholder="动作" clearable style="width: 130px" @change="onFilterChange">
          <el-option label="登录" value="login" />
          <el-option label="验证" value="verify" />
          <el-option label="心跳" value="heartbeat" />
          <el-option label="绑定" value="bind" />
          <el-option label="解绑" value="unbind" />
          <el-option label="取变量" value="getvar" />
        </el-select>
        <el-select v-model="vFilter.result" placeholder="结果" clearable style="width: 130px" @change="onFilterChange">
          <el-option label="成功" value="success" />
          <el-option label="失败" value="fail" />
          <el-option label="已封禁" value="banned" />
          <el-option label="已过期" value="expired" />
          <el-option label="设备不符" value="device_mismatch" />
          <el-option label="限流" value="rate_limited" />
        </el-select>
        <el-date-picker
          v-model="vDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 280px"
          @change="(v: any) => onDateChange(v, 'verify')"
        />
        <el-input v-model="vFilter.keyword" placeholder="关键词（IP/extra）" clearable style="width: 200px" @change="onFilterChange" />
        <el-button @click="loadList">刷新</el-button>
      </div>

      <!-- 登录失败日志筛选 -->
      <div v-else class="search-bar">
        <el-select v-model="lfFilter.user_type" placeholder="用户类型" clearable style="width: 140px" @change="onFilterChange">
          <el-option label="超管" value="admin" />
          <el-option label="开发者" value="tenant" />
          <el-option label="代理" value="agent" />
        </el-select>
        <el-input v-model="lfFilter.username" placeholder="用户名" clearable style="width: 160px" @change="onFilterChange" />
        <el-input v-model="lfFilter.ip" placeholder="IP 地址" clearable style="width: 160px" @change="onFilterChange" />
        <el-select v-model="lfFilter.reason" placeholder="失败原因" clearable style="width: 160px" @change="onFilterChange">
          <el-option label="密码错误" value="wrong_password" />
          <el-option label="账号禁用" value="disabled" />
          <el-option label="账号锁定" value="locked" />
          <el-option label="未知" value="unknown" />
        </el-select>
        <el-date-picker
          v-model="lfDateRange"
          type="daterange"
          range-separator="至"
          start-placeholder="开始日期"
          end-placeholder="结束日期"
          value-format="YYYY-MM-DD"
          style="width: 280px"
          @change="(v: any) => onDateChange(v, 'login_failed')"
        />
        <el-button @click="loadList">刷新</el-button>
      </div>

      <!-- 操作日志表格 -->
      <ResponsiveTable
        v-if="activeTab === 'operation'"
        :data="opList"
        :loading="loading"
        :total="opTotal"
        v-model:page="opFilter.page"
        v-model:pageSize="opFilter.page_size"
        :mobile-fields="opMobileFields"
        @page-change="loadList"
        @size-change="loadList"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="operator_type" label="操作者类型" width="110">
          <template #default="{ row }">
            <el-tag :type="operatorTypeTag(row.operator_type)" size="small">{{ operatorTypeText(row.operator_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" width="140" show-overflow-tooltip />
        <el-table-column prop="operator_id" label="操作者ID" width="100" />
        <el-table-column prop="module" label="模块" width="120" show-overflow-tooltip />
        <el-table-column prop="action" label="动作" min-width="140" show-overflow-tooltip />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="statusTag(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="target_type" label="目标类型" width="120" show-overflow-tooltip />
        <el-table-column prop="target_id" label="目标ID" width="90" />
        <el-table-column prop="operator_ip" label="IP" width="140" />
        <el-table-column label="详情" width="80" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openDetail(row, 'operation')">查看</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openDetail(item, 'operation')">查看详情</el-button>
        </template>
      </ResponsiveTable>

      <!-- 验证日志表格 -->
      <ResponsiveTable
        v-else-if="activeTab === 'verify'"
        :data="vList"
        :loading="loading"
        :total="vTotal"
        v-model:page="vFilter.page"
        v-model:pageSize="vFilter.page_size"
        :mobile-fields="vMobileFields"
        @page-change="loadList"
        @size-change="loadList"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="tenant_id" label="租户ID" width="90" />
        <el-table-column prop="app_id" label="应用ID" width="90" />
        <el-table-column prop="action" label="动作" width="100">
          <template #default="{ row }">{{ verifyActionText(row.action) }}</template>
        </el-table-column>
        <el-table-column prop="result" label="结果" width="110">
          <template #default="{ row }">
            <el-tag :type="verifyResultTag(row.result)" size="small">{{ verifyResultText(row.result) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="card_id" label="卡密ID" width="90" />
        <el-table-column prop="device_id" label="设备ID" width="90" />
        <el-table-column prop="client_ip" label="客户端IP" width="140" />
        <el-table-column prop="user_agent" label="UA" min-width="200" show-overflow-tooltip />
        <el-table-column label="详情" width="80" fixed="right">
          <template #default="{ row }">
            <el-button type="primary" link size="small" @click="openDetail(row, 'verify')">查看</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="时间" width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button type="primary" size="small" @click="openDetail(item, 'verify')">查看详情</el-button>
        </template>
      </ResponsiveTable>

      <!-- 登录失败日志表格 -->
      <ResponsiveTable
        v-else
        :data="lfList"
        :loading="loading"
        :total="lfTotal"
        v-model:page="lfFilter.page"
        v-model:pageSize="lfFilter.page_size"
        :mobile-fields="lfMobileFields"
        @page-change="loadList"
        @size-change="loadList"
      >
        <el-table-column prop="id" label="ID" width="80" />
        <el-table-column prop="user_type" label="用户类型" width="110">
          <template #default="{ row }">
            <el-tag :type="operatorTypeTag(row.user_type)" size="small">{{ operatorTypeText(row.user_type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="username" label="用户名" width="160" show-overflow-tooltip />
        <el-table-column prop="client_ip" label="IP" width="150" />
        <el-table-column prop="reason" label="失败原因" width="140">
          <template #default="{ row }">
            <el-tag :type="reasonTag(row.reason)" size="small">{{ reasonText(row.reason) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="user_agent" label="UA" min-width="200" show-overflow-tooltip />
        <el-table-column prop="created_at" label="时间" width="180">
          <template #default="{ row }">{{ formatDate(row.created_at) }}</template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 详情对话框 -->
    <el-dialog v-model="detailVisible" :title="detailTitle" :width="isMobile ? '92%' : '560px'">
      <div v-if="detailRow" class="detail-box">
        <div class="detail-row"><span class="label">ID：</span><span>{{ detailRow.id }}</span></div>

        <!-- 操作日志 -->
        <template v-if="detailKind === 'operation'">
          <div class="detail-row"><span class="label">操作者类型：</span>{{ operatorTypeText(detailRow.operator_type) }}</div>
          <div class="detail-row"><span class="label">操作者ID：</span>{{ detailRow.operator_id }}</div>
          <div class="detail-row"><span class="label">用户名：</span>{{ detailRow.username || '-' }}</div>
          <div class="detail-row"><span class="label">模块：</span>{{ detailRow.module || '-' }}</div>
          <div class="detail-row"><span class="label">动作：</span>{{ detailRow.action || '-' }}</div>
          <div class="detail-row"><span class="label">状态：</span>
            <el-tag :type="statusTag(detailRow.status)" size="small">{{ statusText(detailRow.status) }}</el-tag>
          </div>
          <div class="detail-row"><span class="label">目标类型：</span>{{ detailRow.target_type || '-' }}</div>
          <div class="detail-row"><span class="label">目标ID：</span>{{ detailRow.target_id || '-' }}</div>
          <div class="detail-row"><span class="label">IP：</span>{{ detailRow.operator_ip || '-' }}</div>
          <div class="detail-row"><span class="label">UA：</span><span class="mono">{{ detailRow.user_agent || '-' }}</span></div>
          <div class="detail-row"><span class="label">时间：</span>{{ formatDate(detailRow.created_at) }}</div>
          <div class="detail-row detail-block">
            <span class="label">详情：</span>
            <pre class="detail-pre">{{ formatJson(detailRow.detail) }}</pre>
          </div>
        </template>

        <!-- 验证日志 -->
        <template v-else-if="detailKind === 'verify'">
          <div class="detail-row"><span class="label">租户ID：</span>{{ detailRow.tenant_id }}</div>
          <div class="detail-row"><span class="label">应用ID：</span>{{ detailRow.app_id }}</div>
          <div class="detail-row"><span class="label">卡密ID：</span>{{ detailRow.card_id || '-' }}</div>
          <div class="detail-row"><span class="label">设备ID：</span>{{ detailRow.device_id || '-' }}</div>
          <div class="detail-row"><span class="label">动作：</span>{{ verifyActionText(detailRow.action) }}</div>
          <div class="detail-row"><span class="label">结果：</span>
            <el-tag :type="verifyResultTag(detailRow.result)" size="small">{{ verifyResultText(detailRow.result) }}</el-tag>
          </div>
          <div class="detail-row"><span class="label">客户端IP：</span>{{ detailRow.client_ip || '-' }}</div>
          <div class="detail-row"><span class="label">UA：</span><span class="mono">{{ detailRow.user_agent || '-' }}</span></div>
          <div class="detail-row"><span class="label">时间：</span>{{ formatDate(detailRow.created_at) }}</div>
          <div class="detail-row detail-block">
            <span class="label">Extra：</span>
            <pre class="detail-pre">{{ formatJson(detailRow.extra) }}</pre>
          </div>
        </template>
      </div>
      <template #footer>
        <el-button @click="detailVisible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listAdminOperationLogsApi,
  listAdminVerifyLogsApi,
  listAdminLoginFailedLogsApi,
  exportAdminLogsApi,
  type LogOperation,
  type LogVerify,
  type LogLoginFailed,
  type AdminLogTab
} from '@/api/admin'

const activeTab = ref<AdminLogTab>('operation')
const loading = ref(false)
const exporting = ref(false)

// v0.7.0 修复：dialog 响应式宽度
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }

const tabLabel = computed(() => {
  return activeTab.value === 'operation' ? '操作日志'
    : activeTab.value === 'verify' ? '验证日志'
    : '登录失败日志'
})

// ============== 操作日志状态 ==============
const opList = ref<LogOperation[]>([])
const opTotal = ref(0)
const opFilter = reactive({
  operator_type: '' as string,
  operator_id_input: '',
  operator_id: undefined as number | undefined,
  module: '',
  action: '',
  status: '' as string,
  start_date: '',
  end_date: '',
  keyword: '',
  page: 1,
  page_size: 20
})
const opDateRange = ref<[string, string] | null>(null)

const opMobileFields = [
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) },
  { prop: 'username', label: '用户名' },
  { prop: 'operator_type', label: '类型', formatter: (v: string) => operatorTypeText(v) },
  { prop: 'action', label: '动作' },
  { prop: 'operator_ip', label: 'IP' }
]

// ============== 验证日志状态 ==============
const vList = ref<LogVerify[]>([])
const vTotal = ref(0)
const vFilter = reactive({
  tenant_id_input: '',
  tenant_id: undefined as number | undefined,
  app_id_input: '',
  app_id: undefined as number | undefined,
  action: '' as string,
  result: '' as string,
  start_date: '',
  end_date: '',
  keyword: '',
  page: 1,
  page_size: 20
})
const vDateRange = ref<[string, string] | null>(null)

const vMobileFields = [
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) },
  { prop: 'app_id', label: '应用ID' },
  { prop: 'action', label: '动作', formatter: (v: string) => verifyActionText(v) },
  { prop: 'result', label: '结果', formatter: (v: string) => verifyResultText(v) },
  { prop: 'client_ip', label: 'IP' }
]

// ============== 登录失败日志状态 ==============
const lfList = ref<LogLoginFailed[]>([])
const lfTotal = ref(0)
const lfFilter = reactive({
  user_type: '' as string,
  username: '',
  ip: '',
  reason: '' as string,
  start_date: '',
  end_date: '',
  page: 1,
  page_size: 20
})
const lfDateRange = ref<[string, string] | null>(null)

const lfMobileFields = [
  { prop: 'created_at', label: '时间', formatter: (v: string) => formatDate(v) },
  { prop: 'user_type', label: '类型', formatter: (v: string) => operatorTypeText(v) },
  { prop: 'username', label: '用户名' },
  { prop: 'client_ip', label: 'IP' },
  { prop: 'reason', label: '原因', formatter: (v: string) => reasonText(v) }
]

// ============== 日期变更 ==============
const onDateChange = (val: [string, string] | null, kind: AdminLogTab) => {
  if (val && val.length === 2) {
    if (kind === 'operation') {
      opFilter.start_date = val[0]
      opFilter.end_date = val[1]
    } else if (kind === 'verify') {
      vFilter.start_date = val[0]
      vFilter.end_date = val[1]
    } else {
      lfFilter.start_date = val[0]
      lfFilter.end_date = val[1]
    }
  } else {
    if (kind === 'operation') {
      opFilter.start_date = ''
      opFilter.end_date = ''
    } else if (kind === 'verify') {
      vFilter.start_date = ''
      vFilter.end_date = ''
    } else {
      lfFilter.start_date = ''
      lfFilter.end_date = ''
    }
  }
  // v0.7.0 修复：筛选变更重置分页
  if (kind === 'operation') opFilter.page = 1
  else if (kind === 'verify') vFilter.page = 1
  else lfFilter.page = 1
  loadList()
}

// ============== Tab 切换 ==============
const onTabChange = (tab: string | number) => {
  activeTab.value = tab as AdminLogTab
  loadList()
}

// v0.7.0 修复：筛选变更重置分页
const onFilterChange = () => {
  if (activeTab.value === 'operation') opFilter.page = 1
  else if (activeTab.value === 'verify') vFilter.page = 1
  else lfFilter.page = 1
  loadList()
}

// ============== 加载列表 ==============
const loadList = async () => {
  loading.value = true
  try {
    if (activeTab.value === 'operation') {
      // 处理 operator_id_input → operator_id
      if (opFilter.operator_id_input) {
        const n = Number(opFilter.operator_id_input)
        opFilter.operator_id = isNaN(n) ? undefined : n
      } else {
        opFilter.operator_id = undefined
      }
      const resp = await listAdminOperationLogsApi({
        operator_type: opFilter.operator_type || undefined,
        operator_id: opFilter.operator_id,
        module: opFilter.module || undefined,
        action: opFilter.action || undefined,
        status: opFilter.status || undefined,
        start_date: opFilter.start_date || undefined,
        end_date: opFilter.end_date || undefined,
        keyword: opFilter.keyword || undefined,
        page: opFilter.page,
        page_size: opFilter.page_size
      })
      opList.value = resp.list || []
      opTotal.value = resp.total || 0
    } else if (activeTab.value === 'verify') {
      if (vFilter.tenant_id_input) {
        const n = Number(vFilter.tenant_id_input)
        vFilter.tenant_id = isNaN(n) ? undefined : n
      } else {
        vFilter.tenant_id = undefined
      }
      if (vFilter.app_id_input) {
        const n = Number(vFilter.app_id_input)
        vFilter.app_id = isNaN(n) ? undefined : n
      } else {
        vFilter.app_id = undefined
      }
      const resp = await listAdminVerifyLogsApi({
        tenant_id: vFilter.tenant_id,
        app_id: vFilter.app_id,
        action: vFilter.action || undefined,
        result: vFilter.result || undefined,
        start_date: vFilter.start_date || undefined,
        end_date: vFilter.end_date || undefined,
        keyword: vFilter.keyword || undefined,
        page: vFilter.page,
        page_size: vFilter.page_size
      })
      vList.value = resp.list || []
      vTotal.value = resp.total || 0
    } else {
      const resp = await listAdminLoginFailedLogsApi({
        user_type: lfFilter.user_type || undefined,
        username: lfFilter.username || undefined,
        ip: lfFilter.ip || undefined,
        reason: lfFilter.reason || undefined,
        start_date: lfFilter.start_date || undefined,
        end_date: lfFilter.end_date || undefined,
        page: lfFilter.page,
        page_size: lfFilter.page_size
      })
      lfList.value = resp.list || []
      lfTotal.value = resp.total || 0
    }
  } catch {
    // 错误已由 http 拦截器处理（铁律 06：不编造数据）
  } finally {
    loading.value = false
  }
}

// ============== 导出 CSV ==============
const exportCsv = async () => {
  exporting.value = true
  try {
    const blob = await exportAdminLogsApi({ type: activeTab.value })
    // 后端已写入 UTF-8 BOM，直接构造下载链接
    const url = window.URL.createObjectURL(blob as unknown as Blob)
    const a = document.createElement('a')
    a.href = url
    const now = new Date()
    const stamp = `${now.getFullYear()}${String(now.getMonth() + 1).padStart(2, '0')}${String(now.getDate()).padStart(2, '0')}_${String(now.getHours()).padStart(2, '0')}${String(now.getMinutes()).padStart(2, '0')}${String(now.getSeconds()).padStart(2, '0')}`
    a.download = `logs_${activeTab.value}_${stamp}.csv`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    window.URL.revokeObjectURL(url)
    ElMessage.success('导出成功')
  } catch {
    // 拦截器已提示
  } finally {
    exporting.value = false
  }
}

// ============== 详情对话框 ==============
const detailVisible = ref(false)
const detailRow = ref<any>(null)
const detailKind = ref<AdminLogTab>('operation')
const detailTitle = computed(() => {
  return detailKind.value === 'operation' ? '操作日志详情'
    : detailKind.value === 'verify' ? '验证日志详情'
    : '登录失败日志详情'
})

const openDetail = (row: any, kind: AdminLogTab) => {
  detailRow.value = row
  detailKind.value = kind
  detailVisible.value = true
}

// ============== 格式化与标签辅助 ==============
const operatorTypeText = (t: string) => {
  const map: Record<string, string> = { admin: '超管', tenant: '开发者', agent: '代理' }
  return map[t] || t || '-'
}

const operatorTypeTag = (t: string): any => {
  const map: Record<string, any> = { admin: 'danger', tenant: 'primary', agent: 'warning' }
  return map[t] || 'info'
}

const statusTag = (s: string): any => s === 'success' ? 'success' : 'danger'
const statusText = (s: string) => s === 'success' ? '成功' : (s === 'fail' ? '失败' : (s || '-'))

const verifyActionText = (a: string) => {
  const map: Record<string, string> = {
    login: '登录', verify: '验证', heartbeat: '心跳', bind: '绑定',
    unbind: '解绑', getvar: '取变量', notice: '公告', version: '版本'
  }
  return map[a] || a || '-'
}

const verifyResultTag = (r: string): any => {
  const map: Record<string, any> = {
    success: 'success', fail: 'danger', banned: 'warning',
    expired: 'info', device_mismatch: 'warning', rate_limited: 'info'
  }
  return map[r] || 'info'
}

const verifyResultText = (r: string) => {
  const map: Record<string, string> = {
    success: '成功', fail: '失败', banned: '已封禁',
    expired: '已过期', device_mismatch: '设备不符', rate_limited: '限流'
  }
  return map[r] || r || '-'
}

const reasonText = (r: string) => {
  const map: Record<string, string> = {
    wrong_password: '密码错误', disabled: '账号禁用',
    locked: '账号锁定', unknown: '未知'
  }
  return map[r] || r || '-'
}

const reasonTag = (r: string): any => {
  const map: Record<string, any> = {
    wrong_password: 'warning', disabled: 'danger',
    locked: 'danger', unknown: 'info'
  }
  return map[r] || 'info'
}

const formatDate = (s: string | null | undefined) => {
  if (!s) return '-'
  const d = new Date(s)
  // v0.7.0 修复：Invalid Date 兜底
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN')
}

const formatJson = (s: string | null | undefined) => {
  if (!s) return '-'
  try {
    return JSON.stringify(JSON.parse(s), null, 2)
  } catch {
    return s
  }
}

onMounted(() => {
  loadList()
  // v0.7.0 修复：dialog 响应式宽度
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

// v0.7.0 修复：dialog 响应式宽度
onBeforeUnmount(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.logs-page {
  .action-bar {
    display: flex;
    justify-content: flex-end;
    margin-bottom: $spacing-sm;
  }

  .detail-box {
    .detail-row {
      display: flex;
      padding: 6px 0;
      font-size: 13px;
      border-bottom: 1px solid $color-border-lighter;
      word-break: break-all;
      .label {
        width: 90px;
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
