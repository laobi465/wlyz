<!--
  卡密管理（开发者）- 响应式
-->
<template>
  <div class="cards-page">
    <PageHeader title="卡密管理" subtitle="批量生成与管理卡密">
      <template #actions>
        <el-button type="primary" @click="openGenerate">批量生成卡密</el-button>
      </template>
    </PageHeader>

    <div class="app-card">
      <div class="search-bar">
        <el-select v-model="filter.app_id" placeholder="应用" clearable style="width: 160px" @change="loadList">
          <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
        </el-select>
        <el-select v-model="filter.card_type_id" placeholder="卡类" clearable style="width: 160px" @change="loadList">
          <el-option v-for="ct in cardTypes" :key="ct.id" :label="ct.name" :value="ct.id" />
        </el-select>
        <el-select v-model="filter.status" placeholder="状态" clearable style="width: 120px" @change="loadList">
          <el-option label="未使用" value="unused" />
          <el-option label="正常" value="active" />
          <el-option label="已过期" value="expired" />
          <el-option label="已封禁" value="banned" />
          <el-option label="已禁用" value="disabled" />
        </el-select>
        <el-input v-model="filter.batch_no" placeholder="批次号" clearable style="width: 160px" @change="loadList" />
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
        <el-table-column prop="card_key" label="卡密" min-width="200">
          <template #default="scope">
            <span class="mono">{{ maskKey(scope.row.card_key) }}</span>
            <el-button text size="small" @click="copy(scope.row.card_key)">复制</el-button>
          </template>
        </el-table-column>
        <el-table-column prop="batch_no" label="批次" width="140" />
        <el-table-column prop="status" label="状态" width="80">
          <template #default="scope">
            <el-tag :type="statusTag(scope.row.status)" size="small">{{ statusText(scope.row.status) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="used_count" label="已用/最大" width="100">
          <template #default="scope">{{ scope.row.used_count }}/{{ scope.row.max_uses }}</template>
        </el-table-column>
        <el-table-column prop="activated_at" label="激活时间" width="160">
          <template #default="scope">{{ formatDate(scope.row.activated_at) }}</template>
        </el-table-column>
        <el-table-column prop="expires_at" label="过期时间" width="160">
          <template #default="scope">{{ formatDate(scope.row.expires_at) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="scope">
            <el-button v-if="scope.row.status !== 'banned'" type="warning" link size="small" @click="banCard(scope.row)">封禁</el-button>
            <el-button v-if="scope.row.status === 'banned'" type="success" link size="small" @click="unbanCard(scope.row)">解禁</el-button>
            <el-button type="danger" link size="small" @click="deleteCard(scope.row)">删除</el-button>
          </template>
        </el-table-column>

        <template #mobile-actions="{ item }">
          <el-button v-if="item.status !== 'banned'" type="warning" size="small" @click="banCard(item)">封禁</el-button>
          <el-button v-if="item.status === 'banned'" type="success" size="small" @click="unbanCard(item)">解禁</el-button>
          <el-button type="danger" size="small" @click="deleteCard(item)">删除</el-button>
        </template>
      </ResponsiveTable>
    </div>

    <!-- 批量生成对话框 -->
    <el-dialog v-model="generateVisible" title="批量生成卡密" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="所属应用" prop="app_id">
          <el-select v-model="form.app_id" placeholder="选择应用" @change="onAppChange">
            <el-option v-for="a in apps" :key="a.id" :label="a.name" :value="a.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="卡类" prop="card_type_id">
          <el-select v-model="form.card_type_id" placeholder="选择卡类">
            <el-option v-for="ct in cardTypesByApp" :key="ct.id" :label="ct.name + ' ¥' + ct.price" :value="ct.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="生成数量" prop="quantity">
          <el-input-number v-model="form.quantity" :min="1" :max="1000" />
          <span class="hint">最多 1000 张/次</span>
        </el-form-item>
        <el-form-item label="前缀">
          <el-input v-model="form.prefix" placeholder="可选，如 VIP-" maxlength="16" />
        </el-form-item>
        <el-form-item label="分组标签">
          <el-input v-model="form.group_tag" placeholder="可选" maxlength="64" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="generateVisible = false">取消</el-button>
        <el-button type="primary" :loading="generateLoading" @click="confirmGenerate">生成</el-button>
      </template>
    </el-dialog>

    <!-- 生成结果对话框 -->
    <el-dialog v-model="resultVisible" title="生成成功" width="600px">
      <el-alert type="success" :closable="false" show-icon>
        共生成 {{ generatedKeys.length }} 张卡密，批次号：{{ generatedBatch }}
      </el-alert>
      <div class="keys-list">
        <div v-for="(key, idx) in generatedKeys" :key="idx" class="key-row">
          <span class="key-text">{{ key }}</span>
          <el-button text size="small" @click="copy(key)">复制</el-button>
        </div>
      </div>
      <div class="result-actions">
        <el-button @click="copyAll">复制全部</el-button>
        <el-button type="primary" @click="resultVisible = false">完成</el-button>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import {
  listCardsApi, generateCardsApi, banCardApi, unbanCardApi, deleteCardApi,
  type Card, type CardStatus, type CardTypeKind
} from '@/api/cards'
import { listAppsApi, type App } from '@/api/apps'
import { listCardTypesApi, type CardType } from '@/api/cards'

const list = ref<Card[]>([])
const total = ref(0)
const loading = ref(false)
const apps = ref<App[]>([])
const cardTypes = ref<CardType[]>([])

const filter = reactive({
  app_id: undefined as number | undefined,
  card_type_id: undefined as number | undefined,
  status: undefined as CardStatus | undefined,
  batch_no: '',
  page: 1,
  page_size: 20
})

const mobileFields = [
  { prop: 'card_key', label: '卡密', formatter: (v: string) => maskKey(v) },
  { prop: 'batch_no', label: '批次' },
  { prop: 'status', label: '状态', formatter: (v: string) => statusText(v) },
  { prop: 'used_count', label: '使用', formatter: (v: number, row: Card) => `${v}/${row.max_uses}` },
  { prop: 'expires_at', label: '过期', formatter: (v: string) => formatDate(v) }
]

const generateVisible = ref(false)
const generateLoading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  app_id: undefined as number | undefined,
  card_type_id: undefined as number | undefined,
  quantity: 10,
  prefix: '',
  group_tag: ''
})

const rules = {
  app_id: [{ required: true, message: '请选择应用', trigger: 'change' }],
  card_type_id: [{ required: true, message: '请选择卡类', trigger: 'change' }],
  quantity: [{ required: true, message: '请输入数量', trigger: 'blur' }]
}

const cardTypesByApp = computed(() => {
  if (!form.app_id) return []
  return cardTypes.value.filter(ct => ct.app_id === form.app_id)
})

const resultVisible = ref(false)
const generatedKeys = ref<string[]>([])
const generatedBatch = ref('')

const statusTag = (s: string): any => {
  const map: Record<string, any> = {
    unused: 'info',
    active: 'success',
    expired: 'warning',
    banned: 'danger',
    disabled: 'info'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    unused: '未使用',
    active: '正常',
    expired: '已过期',
    banned: '已封禁',
    disabled: '已禁用'
  }
  return map[s] || s
}

const maskKey = (key: string) => {
  if (!key) return '-'
  if (key.length <= 8) return key.slice(0, 4) + '****'
  return key.slice(0, 6) + '****' + key.slice(-4)
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

const loadCardTypes = async () => {
  try {
    const resp = await listCardTypesApi({ page: 1, page_size: 100 })
    cardTypes.value = resp.list || []
  } catch {}
}

const loadList = async () => {
  loading.value = true
  try {
    const resp = await listCardsApi({
      app_id: filter.app_id,
      card_type_id: filter.card_type_id,
      status: filter.status,
      batch_no: filter.batch_no,
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

const openGenerate = () => {
  Object.assign(form, { app_id: undefined, card_type_id: undefined, quantity: 10, prefix: '', group_tag: '' })
  generateVisible.value = true
}

const onAppChange = () => {
  form.card_type_id = undefined
}

const confirmGenerate = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (!form.app_id || !form.card_type_id) {
      ElMessage.warning('请选择应用和卡类')
      return
    }
    generateLoading.value = true
    try {
      const resp = await generateCardsApi({
        app_id: form.app_id,
        card_type_id: form.card_type_id,
        quantity: form.quantity,
        prefix: form.prefix,
        group_tag: form.group_tag
      })
      generatedKeys.value = resp.card_keys || []
      generatedBatch.value = resp.batch_no
      generateVisible.value = false
      resultVisible.value = true
      ElMessage.success(`成功生成 ${resp.quantity} 张卡密`)
      loadList()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      generateLoading.value = false
    }
  })
}

const banCard = (row: any) => {
  ElMessageBox.prompt('请输入封禁原因', '封禁卡密', {
    confirmButtonText: '确认封禁',
    cancelButtonText: '取消',
    inputType: 'textarea',
    inputPlaceholder: '可选'
  }).then(async ({ value }) => {
    try {
      await banCardApi(row.id, value)
      ElMessage.success('已封禁')
      loadList()
    } catch {}
  }).catch(() => {})
}

const unbanCard = async (row: any) => {
  try {
    await unbanCardApi(row.id)
    ElMessage.success('已解禁')
    loadList()
  } catch {}
}

const deleteCard = (row: any) => {
  ElMessageBox.confirm('确定要删除此卡密吗？删除后不可恢复', '危险操作', {
    type: 'error',
    confirmButtonText: '确认删除',
    cancelButtonText: '取消'
  }).then(async () => {
    try {
      await deleteCardApi(row.id)
      ElMessage.success('已删除')
      loadList()
    } catch {}
  }).catch(() => {})
}

const copy = (text: string) => {
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success('已复制')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

const copyAll = () => {
  navigator.clipboard.writeText(generatedKeys.value.join('\n')).then(() => {
    ElMessage.success('已复制全部')
  }).catch(() => {
    ElMessage.error('复制失败')
  })
}

onMounted(async () => {
  await Promise.all([loadApps(), loadCardTypes()])
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.cards-page {
  .mono {
    font-family: monospace;
    font-size: 13px;
    color: $color-text-primary;
  }
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
  .keys-list {
    max-height: 400px;
    overflow-y: auto;
    margin-top: $spacing-md;
    .key-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: $spacing-sm $spacing-md;
      background: $color-primary-light;
      border-radius: $radius-sm;
      margin-bottom: $spacing-sm;
      .key-text {
        font-family: monospace;
        font-size: 13px;
        color: $color-text-primary;
        word-break: break-all;
        flex: 1;
        margin-right: $spacing-sm;
      }
    }
  }
  .result-actions {
    display: flex;
    justify-content: flex-end;
    gap: $spacing-sm;
    margin-top: $spacing-md;
  }
}
</style>
