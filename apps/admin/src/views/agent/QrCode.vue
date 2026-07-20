<!--
  AgentQrCode 代理扫码购卡（v0.4.x 残留项 3 P-10）
  - 路径：/agent/qrcode
  - 调用后端 GET /agent/portal/qrcode 获取 portal_url + qrcode_api
  - 渲染二维码图片 + 提供复制链接 + 下载二维码
  - 移动优先响应式设计
  - 鉴权：仅代理角色可访问
-->
<template>
  <div class="agent-qrcode-page">
    <PageHeader title="扫码购卡" subtitle="用户扫码直接进入您的代理门户购卡">
      <template #actions>
        <el-button @click="loadQrCode" :loading="loading">刷新</el-button>
      </template>
    </PageHeader>

    <div v-loading="loading" class="qr-content">
      <!-- 错误状态 -->
      <EmptyState v-if="errorMsg && !loading" :description="errorMsg" />

      <template v-else-if="qrInfo">
        <!-- 子域名绑定状态提示 -->
        <el-alert
          v-if="qrInfo.subdomain_status === 'none'"
          title="尚未绑定子域名"
          type="info"
          :closable="false"
          show-icon
          class="status-alert"
        >
          当前使用 agent_id 路径模式；绑定专属子域名后可使用更短、更专业的二维码 URL。
        </el-alert>
        <el-alert
          v-else-if="qrInfo.subdomain_status === 'pending'"
          title="子域名申请审核中"
          type="warning"
          :closable="false"
          show-icon
          class="status-alert"
        >
          当前仍使用 agent_id 路径模式；审核通过后自动切换为子域名 URL。
        </el-alert>
        <el-alert
          v-else-if="qrInfo.subdomain_status === 'approved'"
          title="子域名已生效"
          type="success"
          :closable="false"
          show-icon
          class="status-alert"
        >
          已为该代理启用专属子域名 {{ qrInfo.subdomain }}，二维码 URL 自动使用子域名。
        </el-alert>
        <el-alert
          v-else-if="qrInfo.subdomain_status === 'rejected'"
          title="子域名申请被驳回"
          type="error"
          :closable="false"
          show-icon
          class="status-alert"
        >
          当前使用 agent_id 路径模式；可重新申请不同的子域名。
        </el-alert>

        <!-- 二维码展示 -->
        <div class="qr-card">
          <div class="qr-image-wrapper">
            <img
              v-if="qrInfo.qrcode_api"
              :src="qrInfo.qrcode_api"
              alt="代理门户购卡二维码"
              class="qr-image"
              referrerpolicy="no-referrer"
            />
            <div v-else class="qr-placeholder">
              <el-icon size="48"><Picture /></el-icon>
              <span>二维码生成失败</span>
            </div>
          </div>
          <div class="qr-info">
            <div class="info-row">
              <span class="info-label">代理 ID</span>
              <span class="info-value">{{ qrInfo.agent_id }}</span>
            </div>
            <div class="info-row" v-if="qrInfo.subdomain">
              <span class="info-label">子域名</span>
              <span class="info-value">{{ qrInfo.subdomain }}</span>
            </div>
            <div class="info-row">
              <span class="info-label">购卡 URL</span>
              <span class="info-value url-text" :title="qrInfo.portal_url">
                {{ qrInfo.portal_url }}
              </span>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="qr-actions">
            <el-button type="primary" plain @click="copyUrl">
              <el-icon><CopyDocument /></el-icon>
              复制链接
            </el-button>
            <el-button type="primary" plain @click="downloadQr">
              <el-icon><Download /></el-icon>
              下载二维码
            </el-button>
            <el-button type="success" plain @click="openPortal">
              <el-icon><Link /></el-icon>
              打开门户
            </el-button>
          </div>
        </div>

        <!-- 使用说明 -->
        <div class="usage-tips">
          <div class="tips-title">使用说明</div>
          <ol class="tips-list">
            <li>用户使用手机扫码后，将直接进入您的代理门户 H5 页面。</li>
            <li>用户在门户页选择卡类并完成支付，订单将自动归属到您的代理账户。</li>
            <li>支付走开发者官方支付通道，订单按平台抽成规则结算佣金。</li>
            <li>绑定子域名后，二维码 URL 将自动更新为更短的子域名形式。</li>
          </ol>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  CopyDocument,
  Download,
  Link,
  Picture
} from '@element-plus/icons-vue'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import { agentPortalQrCodeApi, type AgentPortalQrCode } from '@/api/agent'

const loading = ref(false)
const errorMsg = ref('')
const qrInfo = ref<AgentPortalQrCode | null>(null)

const loadQrCode = async () => {
  loading.value = true
  errorMsg.value = ''
  try {
    qrInfo.value = await agentPortalQrCodeApi()
  } catch (e: any) {
    errorMsg.value = e?.message || '获取二维码失败'
    qrInfo.value = null
  } finally {
    loading.value = false
  }
}

const copyUrl = async () => {
  if (!qrInfo.value?.portal_url) return
  try {
    await navigator.clipboard.writeText(qrInfo.value.portal_url)
    ElMessage.success('链接已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动选择并复制')
  }
}

const downloadQr = () => {
  if (!qrInfo.value?.qrcode_api) {
    ElMessage.warning('二维码 URL 不可用')
    return
  }
  // 通过 a 标签下载（避免 CORS 限制）
  const a = document.createElement('a')
  a.href = qrInfo.value.qrcode_api
  a.download = `agent-portal-${qrInfo.value.agent_id}.png`
  a.target = '_blank'
  a.rel = 'noopener noreferrer'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

const openPortal = () => {
  if (!qrInfo.value?.portal_url) return
  // 如果是相对路径，直接在前端打开；如果是绝对 URL，新窗口打开
  const url = qrInfo.value.portal_url
  if (url.startsWith('http://') || url.startsWith('https://')) {
    window.open(url, '_blank')
  } else {
    window.open(url, '_blank')
  }
}

onMounted(() => {
  loadQrCode()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.agent-qrcode-page {
  max-width: 720px;
  margin: 0 auto;
}

.qr-content {
  margin-top: $spacing-md;
}

.status-alert {
  margin-bottom: $spacing-md;
}

.qr-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-lg $spacing-md;
  margin-bottom: $spacing-md;
  text-align: center;
}

.qr-image-wrapper {
  display: inline-block;
  padding: $spacing-md;
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  margin-bottom: $spacing-md;
}

.qr-image {
  width: 220px;
  height: 220px;
  display: block;
}

.qr-placeholder {
  width: 220px;
  height: 220px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: $spacing-sm;
  color: $color-text-secondary;
  font-size: 13px;
}

.qr-info {
  text-align: left;
  margin-bottom: $spacing-md;
}

.info-row {
  display: flex;
  align-items: flex-start;
  gap: $spacing-md;
  padding: $spacing-xs 0;
  font-size: 13px;

  .info-label {
    flex-shrink: 0;
    width: 80px;
    color: $color-text-secondary;
  }

  .info-value {
    flex: 1;
    color: $color-text-primary;
    word-break: break-all;

    &.url-text {
      font-family: 'Courier New', monospace;
      color: $color-primary;
    }
  }
}

.qr-actions {
  display: flex;
  justify-content: center;
  gap: $spacing-sm;
  flex-wrap: wrap;
}

.usage-tips {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;

  .tips-title {
    font-size: 14px;
    font-weight: 600;
    color: $color-text-regular;
    margin-bottom: $spacing-sm;
  }

  .tips-list {
    margin: 0;
    padding-left: $spacing-lg;
    font-size: 13px;
    color: $color-text-secondary;
    line-height: 1.8;
  }
}

// v0.5.0 响应式：统一走 mobile-sm mixin（$bp-mobile-sm=480px，max-width: 479px）
@include mobile-sm {
  .qr-actions {
    flex-direction: column;

    .el-button {
      width: 100%;
    }
  }
}
</style>
