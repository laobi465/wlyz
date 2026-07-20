<!--
  H5 会话管理（v0.4.0 收尾项 C）
  - 列出当前账号的活跃会话
  - 当前会话标记 + 不可踢
  - 其它会话可踢下线
-->
<template>
  <div class="h5-sessions">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">会话管理</span>
    </div>

    <div v-loading="loading">
      <div v-if="sessions.length === 0 && !loading" class="empty-card">
        <el-empty description="暂无活跃会话" :image-size="80" />
      </div>

      <div
        v-for="s in sessions"
        :key="s.jti"
        class="session-item"
      >
        <div class="session-head">
          <span class="user-agent">{{ s.user_agent || '未知设备' }}</span>
          <el-tag v-if="s.is_current" type="success" size="small">当前</el-tag>
        </div>
        <div class="session-meta">
          <span class="meta-text">IP：{{ s.ip || '-' }}</span>
          <span class="meta-text">登录：{{ formatTime(s.created_at) }}</span>
        </div>
        <div class="session-meta">
          <span class="meta-text">到期：{{ formatTime(s.expires_at) }}</span>
        </div>
        <div class="session-actions">
          <el-button
            text
            size="small"
            type="danger"
            :disabled="s.is_current"
            :loading="kickingJti === s.jti"
            @click="kick(s)"
          >
            {{ s.is_current ? '当前会话' : '踢下线' }}
          </el-button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { endUserListSessionsApi, endUserKickSessionApi, type EndUserSession } from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()

const sessions = ref<EndUserSession[]>([])
const loading = ref(false)
const kickingJti = ref<string>('')

const load = async () => {
  loading.value = true
  try {
    const resp = await endUserListSessionsApi()
    sessions.value = resp.list || []
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const kick = async (s: EndUserSession) => {
  if (s.is_current) return
  try {
    await ElMessageBox.confirm('确定要踢该会话下线吗？', '提示', {
      type: 'warning',
      confirmButtonText: '踢下线',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  kickingJti.value = s.jti
  try {
    await endUserKickSessionApi(s.jti)
    ElMessage.success('已踢下线')
    sessions.value = sessions.value.filter((x) => x.jti !== s.jti)
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    kickingJti.value = ''
  }
}

const formatTime = (t: string) => {
  if (!t) return '-'
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return t
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch {
    return t
  }
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/profile')
  }
}

onMounted(() => {
  endUserStore.restore()
  if (!endUserStore.isLoggedIn) {
    router.replace('/h5/login')
    return
  }
  load()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-sessions {
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

.session-item {
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-sm;

  .session-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-sm;

    .user-agent {
      font-size: 14px;
      font-weight: 600;
      color: $color-text-primary;
      word-break: break-all;
      flex: 1;
      margin-right: $spacing-sm;
    }
  }

  .session-meta {
    display: flex;
    gap: $spacing-md;
    flex-wrap: wrap;
    margin-bottom: 4px;
    .meta-text {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }

  .session-actions {
    display: flex;
    justify-content: flex-end;
    margin-top: $spacing-sm;
    border-top: 1px solid $color-border-lighter;
    padding-top: $spacing-sm;
  }
}
</style>
