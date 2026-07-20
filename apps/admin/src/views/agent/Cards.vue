<!--
  代理购卡（响应式 H5）
  - 顶部：余额展示 + 刷新
  - 卡类网格：展示授权可购买的卡类（含售价/结算价/佣金）
  - 点击卡类 → 打开购卡对话框（数量/前缀/分组/总价预览）
  - 生成结果对话框：显示卡密列表 + 复制全部
  铁律 06 待核实：后端 /agent/card_types 与 /agent/cards/generate 当前为 501 占位（v0.3.0 交付）。
-->
<template>
  <div class="agent-cards-page">
    <PageHeader title="购卡" subtitle="从授权卡类中购买卡密，扣账户余额">
      <template #actions>
        <el-button @click="loadCardTypes">刷新</el-button>
      </template>
    </PageHeader>

    <!-- 余额展示 -->
    <div class="balance-bar">
      <div class="balance-item">
        <span class="label">账户余额</span>
        <span class="value primary">¥{{ profile.balance.toFixed(2) }}</span>
      </div>
      <div class="balance-item">
        <span class="label">冻结</span>
        <span class="value">¥{{ profile.frozen_balance.toFixed(2) }}</span>
      </div>
      <div class="balance-item">
        <span class="label">佣金模式</span>
        <span class="value">{{ commissionModeText }}</span>
      </div>
      <div class="balance-item" v-if="profile.commission_mode === 'percentage'">
        <span class="label">佣金比例</span>
        <span class="value">{{ profile.commission_rate || 0 }}%</span>
      </div>
    </div>

    <!-- 卡类网格 -->
    <div v-loading="loading">
      <EmptyState v-if="!cardTypes.length && !loading" description="暂无可购买的卡类，请联系开发者授权" />
      <div v-else class="card-type-grid">
        <div
          v-for="ct in cardTypes"
          :key="ct.id"
          class="card-type-item"
          :class="{ disabled: ct.status !== 'active' }"
          @click="openPurchase(ct)"
        >
          <div class="ct-header">
            <span class="ct-name">{{ ct.name }}</span>
            <el-tag size="small" :type="typeTag(ct.type)">{{ typeText(ct.type) }}</el-tag>
          </div>
          <div class="ct-app">{{ ct.app_name || '应用 #' + ct.app_id }}</div>
          <div class="ct-meta">
            <div class="meta-row">
              <span class="meta-label">售价</span>
              <span class="meta-value">¥{{ Number(ct.price).toFixed(2) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">结算价</span>
              <span class="meta-value danger">¥{{ Number(ct.agent_base_price).toFixed(2) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">佣金</span>
              <span class="meta-value success">¥{{ Number(ct.agent_commission).toFixed(2) }}</span>
            </div>
          </div>
          <el-button
            type="primary"
            size="small"
            class="ct-buy-btn"
            :disabled="ct.status !== 'active'"
          >立即购买</el-button>
        </div>
      </div>
    </div>

    <!-- 购卡对话框 -->
    <el-dialog v-model="purchaseVisible" title="代理购卡" width="500px">
      <el-form ref="formRef" :model="form" :rules="rules" label-position="top">
        <el-form-item label="卡类">
          <el-input :model-value="currentCardType ? currentCardType.name + ' (' + (currentCardType.app_name || '应用#' + currentCardType.app_id) + ')' : ''" disabled />
        </el-form-item>
        <el-form-item label="结算单价">
          <el-input :model-value="currentCardType ? '¥' + Number(currentCardType.agent_base_price).toFixed(2) + ' / 张' : ''" disabled />
        </el-form-item>
        <el-form-item label="购买数量" prop="quantity">
          <el-input-number v-model="form.quantity" :min="1" :max="1000" />
          <span class="hint">最多 1000 张/次</span>
        </el-form-item>
        <el-form-item label="卡密前缀">
          <el-input v-model="form.prefix" placeholder="可选，如 VIP-" maxlength="16" />
        </el-form-item>
        <el-form-item label="分组标签">
          <el-input v-model="form.group_tag" placeholder="可选" maxlength="64" />
        </el-form-item>
        <div class="cost-summary">
          <div class="summary-row">
            <span>单价</span>
            <span>¥{{ unitCost.toFixed(2) }}</span>
          </div>
          <div class="summary-row">
            <span>数量</span>
            <span>× {{ form.quantity }}</span>
          </div>
          <div class="summary-row total">
            <span>合计扣款</span>
            <span class="danger">¥{{ totalCost.toFixed(2) }}</span>
          </div>
          <div class="summary-row">
            <span>支付后余额</span>
            <span>¥{{ balanceAfter.toFixed(2) }}</span>
          </div>
        </div>
      </el-form>
      <template #footer>
        <el-button @click="purchaseVisible = false">取消</el-button>
        <el-button type="primary" :loading="purchaseLoading" @click="confirmPurchase">确认购卡</el-button>
      </template>
    </el-dialog>

    <!-- 购卡结果对话框 -->
    <el-dialog v-model="resultVisible" title="购卡成功" width="600px">
      <el-alert type="success" :closable="false" show-icon>
        共生成 {{ generatedKeys.length }} 张卡密，扣款 ¥{{ generatedCost.toFixed(2) }}，批次号：{{ generatedBatch }}
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
import EmptyState from '@/components/EmptyState.vue'
import {
  listAgentCardTypesApi,
  agentGenerateCardsApi,
  agentMeApi,
  type AgentCardType,
  type AgentProfile
} from '@/api/agent'

const profile = ref<AgentProfile>({
  agent_id: 0,
  username: '',
  tenant_id: 0,
  real_name: '',
  phone: '',
  balance: 0,
  frozen_balance: 0,
  total_commission: 0,
  total_withdraw: 0,
  status: 'active',
  created_at: ''
})

const cardTypes = ref<AgentCardType[]>([])
const loading = ref(false)

const purchaseVisible = ref(false)
const purchaseLoading = ref(false)
const currentCardType = ref<AgentCardType | null>(null)
const formRef = ref<FormInstance>()

const form = reactive({
  quantity: 1,
  prefix: '',
  group_tag: ''
})

const rules = {
  quantity: [{ required: true, message: '请输入数量', trigger: 'blur' }]
}

const resultVisible = ref(false)
const generatedKeys = ref<string[]>([])
const generatedBatch = ref('')
const generatedCost = ref(0)

const commissionModeText = computed(() => {
  if (profile.value.commission_mode === 'percentage') return '按比例'
  if (profile.value.commission_mode === 'diff') return '按差价'
  return '-'
})

const unitCost = computed(() => {
  return currentCardType.value ? Number(currentCardType.value.agent_base_price) : 0
})

const totalCost = computed(() => unitCost.value * form.quantity)

const balanceAfter = computed(() => {
  return Math.max(0, profile.value.balance - totalCost.value)
})

const typeTag = (t: string): any => ({
  duration: 'primary',
  count: 'success',
  permanent: 'danger',
  trial: 'info',
  feature: 'warning'
}[t] || 'info')

const typeText = (t: string) => ({
  duration: '时长',
  count: '次数',
  permanent: '永久',
  trial: '试用',
  feature: '功能'
}[t] || t)

const loadProfile = async () => {
  try {
    const data = await agentMeApi()
    if (data && typeof data === 'object') {
      Object.assign(profile.value, data)
    }
  } catch {
    // 铁律 06 待核实：后端 /agent/auth/me 复用 CurrentUser handler，可能正常返回基本信息
  }
}

const loadCardTypes = async () => {
  loading.value = true
  try {
    const resp = await listAgentCardTypesApi({ page: 1, page_size: 100 })
    cardTypes.value = resp.list || []
  } catch {
    // 铁律 04 不编造数据
  } finally {
    loading.value = false
  }
}

const openPurchase = (ct: AgentCardType) => {
  if (ct.status !== 'active') {
    ElMessage.warning('该卡类已下架')
    return
  }
  currentCardType.value = ct
  Object.assign(form, { quantity: 1, prefix: '', group_tag: '' })
  purchaseVisible.value = true
}

const confirmPurchase = async () => {
  if (!formRef.value || !currentCardType.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    if (totalCost.value > profile.value.balance) {
      ElMessage.error('余额不足，请先充值')
      return
    }
    try {
      await ElMessageBox.confirm(
        `将扣款 ¥${totalCost.value.toFixed(2)} 购买 ${form.quantity} 张卡密，确认继续？`,
        '购卡确认',
        { type: 'warning' }
      )
    } catch {
      return
    }
    purchaseLoading.value = true
    try {
      const ctId = currentCardType.value?.id
      if (!ctId) {
        ElMessage.error('卡类信息丢失，请重新选择')
        return
      }
      const resp = await agentGenerateCardsApi({
        card_type_id: ctId,
        quantity: form.quantity,
        prefix: form.prefix,
        group_tag: form.group_tag
      })
      generatedKeys.value = resp.card_keys || []
      generatedBatch.value = resp.batch_no
      generatedCost.value = resp.cost_total || totalCost.value
      purchaseVisible.value = false
      resultVisible.value = true
      ElMessage.success(`成功生成 ${resp.quantity} 张卡密`)
      // 刷新余额与卡类列表
      loadProfile()
      loadCardTypes()
    } catch {
      // 错误已由 http 拦截器处理
    } finally {
      purchaseLoading.value = false
    }
  })
}

const copy = (text: string) => {
  navigator.clipboard.writeText(text).then(() => ElMessage.success('已复制'))
}

const copyAll = () => {
  navigator.clipboard.writeText(generatedKeys.value.join('\n')).then(() => ElMessage.success('已复制全部'))
}

onMounted(() => {
  loadProfile()
  loadCardTypes()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.balance-bar {
  display: flex;
  gap: $spacing-lg;
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-md $spacing-lg;
  margin-bottom: $spacing-lg;
  box-shadow: $shadow-card;
  flex-wrap: wrap;

  .balance-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    .label {
      font-size: 12px;
      color: $color-text-secondary;
    }
    .value {
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
      &.primary { color: $color-primary; }
    }
  }

  @include mobile {
    gap: $spacing-md;
    padding: $spacing-sm $spacing-md;
    .balance-item .value { font-size: 14px; }
  }
}

.card-type-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: $spacing-md;

  @include mobile {
    grid-template-columns: 1fr;
    gap: $spacing-sm;
  }
}

.card-type-item {
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-md;
  box-shadow: $shadow-card;
  border: 1px solid $color-border-lighter;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  flex-direction: column;

  &:hover {
    box-shadow: $shadow-hover;
    border-color: $color-primary;
  }
  &.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .ct-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-sm;
    .ct-name {
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }
  .ct-app {
    font-size: 12px;
    color: $color-text-secondary;
    margin-bottom: $spacing-md;
  }
  .ct-meta {
    flex: 1;
    .meta-row {
      display: flex;
      justify-content: space-between;
      padding: 4px 0;
      font-size: 13px;
      border-bottom: 1px dashed $color-border-lighter;
      &:last-child { border-bottom: none; }
      .meta-label { color: $color-text-secondary; }
      .meta-value {
        font-weight: 500;
        color: $color-text-primary;
        &.danger { color: $color-danger; }
        &.success { color: $color-success; }
      }
    }
  }
  .ct-buy-btn {
    margin-top: $spacing-md;
    width: 100%;
  }
}

.hint {
  margin-left: $spacing-sm;
  font-size: 12px;
  color: $color-text-secondary;
}

.cost-summary {
  background: $color-bg-page;
  border-radius: $radius-sm;
  padding: $spacing-md;
  margin-top: $spacing-sm;

  .summary-row {
    display: flex;
    justify-content: space-between;
    padding: 4px 0;
    font-size: 13px;
    color: $color-text-regular;

    &.total {
      font-size: 15px;
      font-weight: 600;
      color: $color-text-primary;
      border-top: 1px solid $color-border-lighter;
      margin-top: $spacing-sm;
      padding-top: $spacing-sm;
    }
    .danger { color: $color-danger; font-size: 16px; }
  }
}

.keys-list {
  max-height: 320px;
  overflow-y: auto;
  margin: $spacing-md 0;
  background: $color-bg-page;
  border-radius: $radius-sm;
  padding: $spacing-sm;

  .key-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px $spacing-sm;
    border-bottom: 1px solid $color-border-lighter;
    &:last-child { border-bottom: none; }
    .key-text {
      font-family: 'SF Mono', 'Menlo', monospace;
      font-size: 13px;
      color: $color-text-primary;
      word-break: break-all;
    }
  }
}

.result-actions {
  display: flex;
  gap: $spacing-sm;
  justify-content: flex-end;
}
</style>
