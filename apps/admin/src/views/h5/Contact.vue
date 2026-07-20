<!--
  H5 联系客服（v0.4.x 残留项 4：U-14）
  - 从后端 sys_config 读取：QQ 群 / 微信 / 邮箱 / 电话
  - 每项支持「复制」+「跳转」（QQ 群跳加群链接，邮箱跳 mailto，电话跳 tel）
  - 4 项均可空（后端留空时前端不展示对应渠道）
-->
<template>
  <div class="h5-contact">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">联系客服</span>
    </div>

    <div v-loading="loading">
      <div v-if="!hasAnyChannel && !loading" class="empty-card">
        <el-empty description="暂未配置客服联系方式" :image-size="80">
          <el-button type="primary" @click="goHome">返回首页</el-button>
        </el-empty>
      </div>

      <template v-if="hasAnyChannel">
        <div v-if="info.qq_group" class="channel-card">
          <div class="channel-head">
            <el-icon class="channel-icon" color="#12B7F5"><ChatDotRound /></el-icon>
            <div class="channel-meta">
              <div class="channel-name">QQ 群</div>
              <div class="channel-value">{{ info.qq_group }}</div>
            </div>
          </div>
          <div class="channel-actions">
            <el-button text size="small" @click="copy(info.qq_group, 'QQ 群号')">复制</el-button>
            <el-button text size="small" type="primary" @click="openQQGroup">加群</el-button>
          </div>
        </div>

        <div v-if="info.wechat" class="channel-card">
          <div class="channel-head">
            <el-icon class="channel-icon" color="#07C160"><ChatLineRound /></el-icon>
            <div class="channel-meta">
              <div class="channel-name">微信</div>
              <div class="channel-value">{{ info.wechat }}</div>
            </div>
          </div>
          <div class="channel-actions">
            <el-button text size="small" @click="copy(info.wechat, '微信号')">复制</el-button>
          </div>
        </div>

        <div v-if="info.email" class="channel-card">
          <div class="channel-head">
            <el-icon class="channel-icon" color="#1677ff"><Message /></el-icon>
            <div class="channel-meta">
              <div class="channel-name">邮箱</div>
              <div class="channel-value">{{ info.email }}</div>
            </div>
          </div>
          <div class="channel-actions">
            <el-button text size="small" @click="copy(info.email, '邮箱')">复制</el-button>
            <el-button text size="small" type="primary" @click="openEmail">发送邮件</el-button>
          </div>
        </div>

        <div v-if="info.phone" class="channel-card">
          <div class="channel-head">
            <el-icon class="channel-icon" color="#FF4D4F"><Phone /></el-icon>
            <div class="channel-meta">
              <div class="channel-name">电话</div>
              <div class="channel-value">{{ info.phone }}</div>
            </div>
          </div>
          <div class="channel-actions">
            <el-button text size="small" @click="copy(info.phone, '电话')">复制</el-button>
            <el-button text size="small" type="primary" @click="openPhone">立即拨打</el-button>
          </div>
        </div>

        <div class="tip-card">
          <p>客服工作时间：周一至周日 9:00 - 22:00</p>
          <p>非工作时间请留言，客服上线后会尽快回复。</p>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft, ChatDotRound, ChatLineRound, Message, Phone } from '@element-plus/icons-vue'
import { getContactInfoApi, type ContactInfo } from '@/api/enduser'

const router = useRouter()

const info = ref<ContactInfo>({ qq_group: '', wechat: '', email: '', phone: '' })
const loading = ref(false)

const hasAnyChannel = computed(() =>
  !!(info.value.qq_group || info.value.wechat || info.value.email || info.value.phone)
)

const load = async () => {
  loading.value = true
  try {
    const resp = await getContactInfoApi()
    info.value = {
      qq_group: resp.qq_group || '',
      wechat: resp.wechat || '',
      email: resp.email || '',
      phone: resp.phone || ''
    }
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const copy = (text: string, label: string) => {
  if (!text) return
  navigator.clipboard.writeText(text).then(() => {
    ElMessage.success(`${label}已复制`)
  }).catch(() => {
    ElMessage.error('复制失败，请手动长按复制')
  })
}

/**
 * 打开 QQ 加群链接
 * 铁律 06：群号格式校验（仅数字），避免恶意 URL 注入
 * v0.4.x 简化实现：直接跳腾讯官方加群 URL，由用户浏览器处理
 */
const openQQGroup = () => {
  const q = info.value.qq_group.replace(/[^\d]/g, '')
  if (!q) {
    ElMessage.warning('QQ 群号格式错误')
    return
  }
  // 腾讯官方加群链接（仅支持数字群号）
  window.open(`https://qm.qq.com/q/${q}`, '_blank')
}

const openEmail = () => {
  const e = info.value.email.trim()
  if (!e) return
  window.location.href = `mailto:${e}`
}

const openPhone = () => {
  const p = info.value.phone.replace(/[^\d+-]/g, '')
  if (!p) {
    ElMessage.warning('电话号码格式错误')
    return
  }
  window.location.href = `tel:${p}`
}

const goHome = () => router.push('/h5')

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/profile')
  }
}

onMounted(() => {
  load()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-contact {
  max-width: 640px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  align-items: center;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-md;
  background: #fff;
  border-radius: $radius-md;
  position: relative;

  .back-btn {
    padding: 0 $spacing-sm;
  }
  .title {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    font-size: 16px;
    font-weight: 600;
    color: $color-text-primary;
  }
}

.empty-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
}

.channel-card {
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-sm;

  .channel-head {
    display: flex;
    align-items: center;
    gap: $spacing-md;

    .channel-icon {
      font-size: 28px;
      flex-shrink: 0;
    }

    .channel-meta {
      flex: 1;
      min-width: 0;

      .channel-name {
        font-size: 13px;
        color: $color-text-secondary;
        margin-bottom: 4px;
      }
      .channel-value {
        font-size: 15px;
        font-weight: 600;
        color: $color-text-primary;
        word-break: break-all;
      }
    }
  }

  .channel-actions {
    display: flex;
    justify-content: flex-end;
    gap: $spacing-sm;
    margin-top: $spacing-sm;
    border-top: 1px solid $color-border-lighter;
    padding-top: $spacing-sm;
  }
}

.tip-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-top: $spacing-md;

  p {
    font-size: 12px;
    color: $color-text-secondary;
    line-height: 1.6;
    margin: 0;
  }
}
</style>
