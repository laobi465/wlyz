<!--
  AgentPortal 代理独立门户 H5 页面（v0.4.x 残留项 2 P-06）
  - 路径：/h5/portal/:agentId
  - 展示代理公开信息 + 可售卡类列表
  - 点击卡类跳转到 /h5/portal/:agentId/buy/:cardTypeId 完成下单
  - 移动优先响应式设计，复用 H5Layout
  - 公开页面：无需登录即可访问
-->
<template>
  <div class="agent-portal" v-loading="loading">
    <!-- 代理信息卡片 -->
    <div v-if="agentInfo" class="agent-card">
      <div class="agent-header">
        <el-avatar :size="56">{{ agentPlaceholder }}</el-avatar>
        <div class="agent-meta">
          <div class="agent-name">
            {{ agentInfo.real_name || agentInfo.username }}
          </div>
          <div class="agent-sub">
            <el-tag size="small" type="info">{{ agentInfo.tenant_name }}</el-tag>
            <span v-if="agentInfo.subdomain" class="subdomain-badge">
              {{ agentInfo.subdomain }}
            </span>
          </div>
        </div>
      </div>
      <div class="agent-tip">
        通过本代理门户购卡，享受开发者官方支付通道保障
      </div>
    </div>

    <!-- 错误状态 -->
    <div v-if="errorMsg" class="error-card">
      <el-empty :description="errorMsg" :image-size="80" />
      <el-button type="primary" plain @click="loadPortal">重试</el-button>
    </div>

    <!-- 卡类列表 -->
    <div v-if="cardTypes.length > 0" class="card-types">
      <div class="section-title">选择套餐</div>
      <div
        v-for="ct in cardTypes"
        :key="ct.id"
        class="card-type-item"
        @click="goBuy(ct)"
      >
        <div class="ct-header">
          <div class="ct-title">
            <span class="ct-name">{{ ct.name }}</span>
            <el-tag size="small" type="info">{{ typeLabel(ct.type) }}</el-tag>
          </div>
          <span class="ct-price">¥{{ ct.price.toFixed(2) }}</span>
        </div>
        <div class="ct-meta">
          <span class="meta-app">
            <el-icon><Cellphone /></el-icon>
            {{ ct.app_name }}
          </span>
          <span v-if="ct.duration_seconds > 0" class="meta-text">
            时长 {{ formatDuration(ct.duration_seconds) }}
          </span>
          <span v-else-if="ct.duration_seconds === -1" class="meta-text">永久</span>
          <span v-if="ct.max_uses > 1" class="meta-text">可用 {{ ct.max_uses }} 次</span>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-if="!loading && !errorMsg && cardTypes.length === 0" class="empty-card">
      <el-empty description="该代理暂无可售套餐" :image-size="80" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { Cellphone } from '@element-plus/icons-vue'
import { getPortalInfoApi, type PortalAgentInfo, type PortalCardType } from '@/api/portal'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const errorMsg = ref('')
const agentInfo = ref<PortalAgentInfo | null>(null)
const cardTypes = ref<PortalCardType[]>([])

const agentPlaceholder = computed(() => {
  const name = agentInfo.value?.real_name || agentInfo.value?.username || '?'
  return name.charAt(0).toUpperCase()
})

const loadPortal = async () => {
  const agentId = Number(route.params.agentId)
  if (!agentId || Number.isNaN(agentId)) {
    errorMsg.value = '代理 ID 无效'
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    const resp = await getPortalInfoApi(agentId)
    agentInfo.value = resp.agent
    cardTypes.value = resp.card_types || []
  } catch (e: any) {
    errorMsg.value = e?.message || '代理不存在或已禁用'
    agentInfo.value = null
    cardTypes.value = []
  } finally {
    loading.value = false
  }
}

const goBuy = (ct: PortalCardType) => {
  const agentId = route.params.agentId
  router.push({
    name: 'AgentPortalBuy',
    params: { agentId, cardTypeId: ct.id }
  })
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
  loadPortal()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.agent-portal {
  max-width: 640px;
  margin: 0 auto;
}

.agent-card {
  background: linear-gradient(135deg, #fff 0%, #f5f7fa 100%);
  border-radius: $radius-md;
  padding: $spacing-lg $spacing-md;
  margin-bottom: $spacing-md;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.04);
}

.agent-header {
  display: flex;
  align-items: center;
  gap: $spacing-md;
}

.agent-meta {
  flex: 1;
  min-width: 0;

  .agent-name {
    font-size: 18px;
    font-weight: 600;
    color: $color-text-primary;
    margin-bottom: 4px;
  }

  .agent-sub {
    display: flex;
    align-items: center;
    gap: $spacing-xs;
    flex-wrap: wrap;
  }

  .subdomain-badge {
    font-size: 12px;
    color: $color-primary;
    background: rgba(64, 158, 255, 0.1);
    padding: 2px 6px;
    border-radius: 4px;
  }
}

.agent-tip {
  margin-top: $spacing-md;
  padding: $spacing-sm $spacing-md;
  background: rgba(64, 158, 255, 0.06);
  border-radius: $radius-sm;
  font-size: 12px;
  color: $color-text-secondary;
  text-align: center;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: $color-text-regular;
  margin: $spacing-md 0 $spacing-sm;
}

.card-type-item {
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-sm;
  cursor: pointer;
  transition: all 0.2s;

  &:active {
    background: $color-bg-page;
  }
}

.ct-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  margin-bottom: $spacing-sm;
}

.ct-title {
  display: flex;
  align-items: center;
  gap: $spacing-xs;
  flex: 1;
  min-width: 0;

  .ct-name {
    font-size: 15px;
    font-weight: 600;
    color: $color-text-primary;
  }
}

.ct-price {
  font-size: 18px;
  font-weight: 600;
  color: $color-danger;
}

.ct-meta {
  display: flex;
  align-items: center;
  gap: $spacing-md;
  font-size: 12px;
  color: $color-text-secondary;
  flex-wrap: wrap;

  .meta-app {
    display: flex;
    align-items: center;
    gap: 4px;
  }
}

.error-card,
.empty-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
  text-align: center;
}
</style>
