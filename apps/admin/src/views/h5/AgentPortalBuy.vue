<!--
  AgentPortalBuy 代理门户购卡结算页（v0.4.x 残留项 2 P-06）
  - 路径：/h5/portal/:agentId/buy/:cardTypeId
  - 展示所选卡类信息 + 数量选择 + 支付方式 + 联系方式
  - 提交下单：调用代理门户公开下单接口，跳转易支付收银台
  - 移动优先响应式设计，复用 H5Layout
  - 公开页面：无需登录即可访问
-->
<template>
  <div class="portal-buy" v-loading="loading">
    <!-- 顶部返回 -->
    <div class="back-row">
      <el-button text @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
        返回代理门户
      </el-button>
    </div>

    <!-- 错误状态 -->
    <div v-if="errorMsg" class="error-card">
      <el-empty :description="errorMsg" :image-size="80" />
      <el-button type="primary" plain @click="loadData">重试</el-button>
    </div>

    <template v-else-if="cardType">
      <!-- 卡类信息卡片 -->
      <div class="ct-card">
        <div class="ct-name">{{ cardType.name }}</div>
        <div class="ct-meta">
          <el-tag size="small" type="info">{{ typeLabel(cardType.type) }}</el-tag>
          <span class="meta-app">
            <el-icon><Cellphone /></el-icon>
            {{ cardType.app_name }}
          </span>
          <span v-if="cardType.duration_seconds > 0" class="meta-text">
            时长 {{ formatDuration(cardType.duration_seconds) }}
          </span>
          <span v-else-if="cardType.duration_seconds === -1" class="meta-text">永久</span>
          <span v-if="cardType.max_uses > 1" class="meta-text">可用 {{ cardType.max_uses }} 次</span>
        </div>
        <div class="ct-price-row">
          <span class="price-label">单价</span>
          <span class="price-value">¥{{ cardType.price.toFixed(2) }}</span>
        </div>
      </div>

      <!-- 购买数量 -->
      <div class="form-card">
        <div class="form-label">购买数量</div>
        <div class="qty-row">
          <el-input-number v-model="quantity" :min="1" :max="99" />
          <span class="qty-total">合计：¥{{ totalAmount }}</span>
        </div>
      </div>

      <!-- 支付方式 -->
      <div class="form-card">
        <div class="form-label">支付方式</div>
        <el-radio-group v-model="payType" class="pay-options">
          <el-radio value="alipay" label="alipay">支付宝</el-radio>
          <el-radio value="wxpay" label="wxpay">微信支付</el-radio>
          <el-radio value="qqpay" label="qqpay">QQ 钱包</el-radio>
        </el-radio-group>
      </div>

      <!-- 联系方式 -->
      <div class="form-card">
        <div class="form-label">联系方式（可选）</div>
        <el-input v-model="buyerContact" placeholder="邮箱或手机号，便于接收卡密" clearable />
      </div>

      <!-- 提交按钮 -->
      <div class="submit-row">
        <el-button
          type="primary"
          size="large"
          :loading="submitting"
          @click="submitOrder"
        >
          立即支付 ¥{{ totalAmount }}
        </el-button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Cellphone } from '@element-plus/icons-vue'
import {
  getPortalInfoApi,
  createPortalOrderApi,
  type PortalCardType,
  type PortalAgentInfo
} from '@/api/portal'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const submitting = ref(false)
const errorMsg = ref('')
const agentInfo = ref<PortalAgentInfo | null>(null)
const cardType = ref<PortalCardType | null>(null)
const quantity = ref(1)
const payType = ref<'alipay' | 'wxpay' | 'qqpay'>('alipay')
const buyerContact = ref('')

const totalAmount = computed(() => {
  if (!cardType.value) return '0.00'
  return (cardType.value.price * quantity.value).toFixed(2)
})

const loadData = async () => {
  const agentId = Number(route.params.agentId)
  const cardTypeId = Number(route.params.cardTypeId)
  if (!agentId || !cardTypeId || Number.isNaN(agentId) || Number.isNaN(cardTypeId)) {
    errorMsg.value = '参数无效'
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    const resp = await getPortalInfoApi(agentId)
    agentInfo.value = resp.agent
    const found = (resp.card_types || []).find((ct) => ct.id === cardTypeId)
    if (!found) {
      errorMsg.value = '卡类不存在或已下架'
      return
    }
    cardType.value = found
  } catch (e: any) {
    errorMsg.value = e?.message || '代理不存在或已禁用'
  } finally {
    loading.value = false
  }
}

const submitOrder = async () => {
  if (!cardType.value || !agentInfo.value) return
  submitting.value = true
  try {
    const resp = await createPortalOrderApi(agentInfo.value.agent_id, {
      app_id: cardType.value.app_id,
      card_type_id: cardType.value.id,
      quantity: quantity.value,
      pay_type: payType.value,
      buyer_contact: buyerContact.value
    })
    // 跳转易支付收银台
    window.location.href = resp.pay_url
  } catch {
    // 错误由 http 拦截器统一处理
  } finally {
    submitting.value = false
  }
}

const goBack = () => {
  const agentId = route.params.agentId
  router.push({ name: 'AgentPortal', params: { agentId } })
}

const typeLabel = (type: string) => {
  const map: Record<string, string> = {
    duration: '时长卡',
    count: '次数卡',
    permanent: '永久卡',
    trial: '试用卡',
    feature: '功能卡'
  }
  return map[type] || type
}

const formatDuration = (seconds: number) => {
  if (seconds >= 86400) return `${Math.floor(seconds / 86400)} 天`
  if (seconds >= 3600) return `${Math.floor(seconds / 3600)} 小时`
  return `${seconds} 秒`
}

onMounted(() => {
  loadData()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.portal-buy {
  max-width: 640px;
  margin: 0 auto;
}

.back-row {
  margin-bottom: $spacing-sm;

  .el-button {
    padding: 0;
    color: $color-text-secondary;
  }
}

.ct-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;

  .ct-name {
    font-size: 18px;
    font-weight: 600;
    color: $color-text-primary;
    margin-bottom: $spacing-sm;
  }

  .ct-meta {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    font-size: 12px;
    color: $color-text-secondary;
    margin-bottom: $spacing-md;
    flex-wrap: wrap;

    .meta-app {
      display: flex;
      align-items: center;
      gap: 4px;
    }
  }

  .ct-price-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding-top: $spacing-sm;
    border-top: 1px dashed $color-border-lighter;

    .price-label {
      font-size: 13px;
      color: $color-text-secondary;
    }

    .price-value {
      font-size: 20px;
      font-weight: 600;
      color: $color-danger;
    }
  }
}

.form-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;

  .form-label {
    font-size: 13px;
    color: $color-text-secondary;
    margin-bottom: $spacing-sm;
  }
}

.qty-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: $spacing-md;

  .qty-total {
    font-size: 16px;
    font-weight: 600;
    color: $color-danger;
  }
}

.pay-options {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
}

.submit-row {
  margin-top: $spacing-lg;

  .el-button {
    width: 100%;
    font-size: 16px;
    height: 48px;
  }
}

.error-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
  text-align: center;
}
</style>
