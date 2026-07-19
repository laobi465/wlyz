<!--
  H5 购卡首页 - 终端用户
  - 输入应用 AppKey 或选择应用
  - 显示卡类列表（从后端拉取，无数据时显示空状态）
  - 选择卡类 + 数量 + 支付方式
  - 提交下单 → 跳转易支付 → 回跳到 /h5/pay/:orderNo
-->
<template>
  <div class="h5-home">
    <!-- 应用选择 -->
    <div class="app-card">
      <p class="section-label">应用 AppKey</p>
      <el-input
        v-model="appKey"
        placeholder="请输入开发者提供的 AppKey"
        clearable
        @change="loadCardTypes"
      />
      <p v-if="appInfo" class="app-info">
        {{ appInfo.name }} <span v-if="appInfo.description">· {{ appInfo.description }}</span>
      </p>
    </div>

    <!-- 卡类列表 -->
    <div v-if="appKey" class="card-types">
      <p class="section-label">选择套餐</p>
      <div v-loading="loading">
        <div v-if="cardTypes.length === 0 && !loading" class="empty-card">
          <el-empty description="暂无可购买套餐" :image-size="80" />
        </div>

        <div
          v-for="ct in cardTypes"
          :key="ct.id"
          class="card-type-item"
          :class="{ active: selectedCardType?.id === ct.id }"
          @click="selectCardType(ct)"
        >
          <div class="ct-header">
            <span class="ct-name">{{ ct.name }}</span>
            <span class="ct-price">¥{{ ct.price.toFixed(2) }}</span>
          </div>
          <div class="ct-meta">
            <el-tag size="small" type="info">{{ typeLabel(ct.type) }}</el-tag>
            <span v-if="ct.duration_seconds > 0" class="meta-text">时长 {{ formatDuration(ct.duration_seconds) }}</span>
            <span v-else-if="ct.duration_seconds === -1" class="meta-text">永久</span>
            <span v-if="ct.max_uses > 1" class="meta-text">可用 {{ ct.max_uses }} 次</span>
          </div>
        </div>
      </div>
    </div>

    <!-- 数量 + 支付方式 -->
    <div v-if="selectedCardType" class="order-form">
      <p class="section-label">购买数量</p>
      <div class="qty-row">
        <el-input-number v-model="quantity" :min="1" :max="99" />
        <span class="qty-total">合计：¥{{ totalAmount }}</span>
      </div>

      <p class="section-label">支付方式</p>
      <el-radio-group v-model="payType" class="pay-options">
        <el-radio value="alipay" label="alipay">支付宝</el-radio>
        <el-radio value="wxpay" label="wxpay">微信支付</el-radio>
        <el-radio value="qqpay" label="qqpay">QQ 钱包</el-radio>
      </el-radio-group>

      <p class="section-label">联系方式（可选）</p>
      <el-input v-model="buyerContact" placeholder="邮箱或手机号，便于接收卡密" />

      <div class="submit-row">
        <el-button type="primary" size="large" :loading="submitting" @click="submitOrder">立即支付 ¥{{ totalAmount }}</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { createPayOrderApi, type PayType } from '@/api/pay'
import { request } from '@/api/http'
import type { CardType } from '@/api/cards'

const router = useRouter()

const appKey = ref('')
const appInfo = ref<{ id: number; name: string; description: string } | null>(null)
const cardTypes = ref<CardType[]>([])
const selectedCardType = ref<CardType | null>(null)
const quantity = ref(1)
const payType = ref<PayType>('alipay')
const buyerContact = ref('')
const loading = ref(false)
const submitting = ref(false)

const totalAmount = computed(() => {
  if (!selectedCardType.value) return '0.00'
  return (selectedCardType.value.price * quantity.value).toFixed(2)
})

// 加载卡类列表（公开接口，待核实：后端是否提供 public/card_types?app_key=xxx）
const loadCardTypes = async () => {
  if (!appKey.value) {
    cardTypes.value = []
    appInfo.value = null
    return
  }
  loading.value = true
  try {
    // 待核实：当前后端未提供公开的应用信息查询接口，暂用客户端 API 反向获取
    // 此处调用一个待实现的公共接口 /public/apps/info?app_key=xxx
    const info = await request.get<{ id: number; name: string; description: string }>('/public/apps/info', { app_key: appKey.value })
    appInfo.value = info
    const list = await request.get<{ list: CardType[] }>('/public/card_types', { app_id: info.id, status: 'active' })
    cardTypes.value = list.list || []
  } catch (e: any) {
    // 接口未实现时给用户友好提示
    ElMessage.error('应用不存在或已禁用')
    cardTypes.value = []
    appInfo.value = null
  } finally {
    loading.value = false
  }
}

const selectCardType = (ct: CardType) => {
  selectedCardType.value = ct
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

const submitOrder = async () => {
  if (!selectedCardType.value || !appInfo.value) {
    ElMessage.warning('请选择套餐')
    return
  }
  submitting.value = true
  try {
    const resp = await createPayOrderApi({
      app_id: appInfo.value.id,
      card_type_id: selectedCardType.value.id,
      quantity: quantity.value,
      pay_type: payType.value,
      buyer_contact: buyerContact.value
    })
    // 跳转到易支付收银台
    window.location.href = resp.pay_url
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-home {
  max-width: 640px;
  margin: 0 auto;
}

.section-label {
  font-size: 13px;
  color: $color-text-secondary;
  margin: $spacing-md 0 $spacing-sm;
}

.app-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;

  .app-info {
    margin: $spacing-sm 0 0;
    font-size: 13px;
    color: $color-text-regular;
  }
}

.card-types {
  margin-bottom: $spacing-md;
}

.empty-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
}

.card-type-item {
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-sm;
  cursor: pointer;
  transition: all 0.2s;

  &:active, &.active {
    border-color: $color-primary;
    background: $color-primary-light;
  }

  .ct-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-sm;

    .ct-name {
      font-size: 15px;
      font-weight: 600;
      color: $color-text-primary;
    }
    .ct-price {
      font-size: 18px;
      font-weight: 700;
      color: $color-danger;
    }
  }

  .ct-meta {
    display: flex;
    gap: $spacing-sm;
    align-items: center;
    flex-wrap: wrap;
    .meta-text {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }
}

.order-form {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;
}

.qty-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: $spacing-sm;
  .qty-total {
    font-size: 16px;
    font-weight: 600;
    color: $color-danger;
  }
}

.pay-options {
  display: flex;
  flex-direction: column;
  gap: $spacing-sm;
  margin-bottom: $spacing-sm;

  :deep(.el-radio) {
    margin-right: 0;
    height: 36px;
  }
}

.submit-row {
  margin-top: $spacing-lg;
  :deep(.el-button) {
    width: 100%;
  }
}
</style>
