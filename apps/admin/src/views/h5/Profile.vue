<!--
  H5 终端用户个人中心（v0.4.0 收尾项 C）
  - 顶部：用户头像 + 昵称 + 状态
  - 统计卡：我的卡密数 + 会话数
  - 菜单：我的卡密 / 会话管理 / 编辑资料 / 修改密码 / 退出登录
-->
<template>
  <div class="h5-profile">
    <!-- 用户信息卡 -->
    <div class="user-card">
      <div class="avatar">
        <el-avatar v-if="user?.avatar" :src="user.avatar" :size="64" />
        <el-avatar v-else :size="64">{{ avatarPlaceholder }}</el-avatar>
      </div>
      <div class="user-meta">
        <div class="nickname">{{ user?.nickname || user?.username || '终端用户' }}</div>
        <div class="username">@{{ user?.username }}</div>
        <el-tag :type="statusTagType" size="small">{{ statusText }}</el-tag>
      </div>
    </div>

    <!-- 统计卡 -->
    <div class="stats-card">
      <div class="stat-item">
        <span class="num">{{ stats.cardCount }}</span>
        <span class="label">我的卡密</span>
      </div>
      <div class="stat-divider"></div>
      <div class="stat-item">
        <span class="num">{{ stats.sessionCount }}</span>
        <span class="label">活跃会话</span>
      </div>
    </div>

    <!-- 菜单 -->
    <div class="menu-card">
      <div class="menu-item" @click="router.push('/h5/my-cards')">
        <el-icon><Key /></el-icon>
        <span class="label">我的卡密</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
      <div class="menu-item" @click="router.push('/h5/orders')">
        <el-icon><List /></el-icon>
        <span class="label">我的订单</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
      <div class="menu-item" @click="router.push('/h5/sessions')">
        <el-icon><Monitor /></el-icon>
        <span class="label">会话管理</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
      <div class="menu-item" @click="router.push('/h5/edit-profile')">
        <el-icon><EditPen /></el-icon>
        <span class="label">编辑资料</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
      <div class="menu-item" @click="router.push('/h5/change-password')">
        <el-icon><Lock /></el-icon>
        <span class="label">修改密码</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
    </div>

    <!-- 公共服务菜单（v0.4.x 残留项 2-4） -->
    <div class="menu-card">
      <div class="menu-item" @click="goNoticeList">
        <el-icon><Bell /></el-icon>
        <span class="label">平台公告</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
      <div class="menu-item" @click="router.push('/h5/help')">
        <el-icon><QuestionFilled /></el-icon>
        <span class="label">帮助中心</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
      <div class="menu-item" @click="router.push('/h5/contact')">
        <el-icon><Headset /></el-icon>
        <span class="label">联系客服</span>
        <el-icon class="arrow"><ArrowRight /></el-icon>
      </div>
    </div>

    <div class="logout-row">
      <el-button type="danger" plain :loading="loggingOut" @click="logout">退出登录</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Key, Monitor, EditPen, Lock, List, Bell, QuestionFilled, Headset, ArrowRight } from '@element-plus/icons-vue'
import {
  endUserMeApi,
  endUserLogoutApi,
  endUserListMyCardsApi,
  endUserListSessionsApi,
  endUserListPlatformNoticesApi
} from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()
const noticeLoading = ref(false)

const user = ref(endUserStore.user)
const stats = ref({ cardCount: 0, sessionCount: 0 })
const loggingOut = ref(false)

const avatarPlaceholder = computed(() => {
  const name = user.value?.nickname || user.value?.username || '?'
  return name.charAt(0).toUpperCase()
})

const statusTagType = computed(() => {
  const map: Record<string, any> = {
    active: 'success',
    disabled: 'info',
    banned: 'danger'
  }
  return map[user.value?.status || ''] || 'info'
})

const statusText = computed(() => {
  const map: Record<string, string> = {
    active: '正常',
    disabled: '已禁用',
    banned: '已封禁'
  }
  return map[user.value?.status || ''] || user.value?.status || '未知'
})

const loadProfile = async () => {
  try {
    const info = await endUserMeApi()
    user.value = info
    endUserStore.setUser(info)
  } catch {
    // 错误已由 http 拦截器处理
  }
}

const loadStats = async () => {
  try {
    const [cardsResp, sessionsResp] = await Promise.allSettled([
      endUserListMyCardsApi(1, 1),
      endUserListSessionsApi()
    ])
    if (cardsResp.status === 'fulfilled') {
      stats.value.cardCount = cardsResp.value.total ?? 0
    }
    if (sessionsResp.status === 'fulfilled') {
      stats.value.sessionCount = sessionsResp.value.total ?? (sessionsResp.value.list?.length ?? 0)
    }
  } catch {
    // 静默失败：统计只是辅助信息
  }
}

// 平台公告入口：拉取最新平台公告，跳转详情页
// 若无公告，提示用户；若有多条，跳转置顶/最新的一条
// v0.4.x 简化实现：仅跳转最新一条；v0.5.x 可改为独立的公告列表页
const goNoticeList = async () => {
  if (noticeLoading.value) return
  noticeLoading.value = true
  try {
    const resp = await endUserListPlatformNoticesApi()
    const list = resp.list || []
    if (list.length === 0) {
      ElMessage.info('暂无平台公告')
      return
    }
    // is_pinned 优先（后端已按 is_pinned DESC, start_at DESC 排序，取首条即可）
    router.push(`/h5/notices/${list[0].id}`)
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    noticeLoading.value = false
  }
}

const logout = async () => {
  try {
    await ElMessageBox.confirm('确定要退出登录吗？', '提示', {
      type: 'warning',
      confirmButtonText: '退出',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  loggingOut.value = true
  try {
    try {
      await endUserLogoutApi()
    } catch {
      // 静默失败：本地仍要清空
    }
    endUserStore.clear()
    ElMessage.success('已退出登录')
    router.replace('/h5')
  } finally {
    loggingOut.value = false
  }
}

onMounted(() => {
  endUserStore.restore()
  if (!endUserStore.isLoggedIn) {
    router.replace('/h5/login')
    return
  }
  user.value = endUserStore.user
  loadProfile()
  loadStats()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-profile {
  max-width: 640px;
  margin: 0 auto;
}

.user-card {
  // v0.5.0 多主题：lighten() 无法处理 var()，改用 CSS color-mix()
  background: linear-gradient(135deg, $color-primary 0%, color-mix(in srgb, $color-primary 92%, white) 100%);
  border-radius: $radius-md;
  padding: $spacing-lg $spacing-md;
  margin-bottom: $spacing-md;
  display: flex;
  align-items: center;
  gap: $spacing-md;
  color: #fff;

  .avatar {
    flex-shrink: 0;
  }
  .user-meta {
    flex: 1;
    min-width: 0;
    .nickname {
      font-size: 18px;
      font-weight: 600;
      margin-bottom: 4px;
      word-break: break-all;
    }
    .username {
      font-size: 12px;
      opacity: 0.9;
      margin-bottom: $spacing-sm;
      word-break: break-all;
    }
    :deep(.el-tag) {
      background: rgba(255, 255, 255, 0.2);
      border-color: rgba(255, 255, 255, 0.3);
      color: #fff;
    }
  }
}

.stats-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;
  display: flex;
  align-items: center;

  .stat-item {
    flex: 1;
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 4px;

    .num {
      font-size: 24px;
      font-weight: 700;
      color: $color-text-primary;
    }
    .label {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }
  .stat-divider {
    width: 1px;
    height: 32px;
    background: $color-border-lighter;
  }
}

.menu-card {
  background: #fff;
  border-radius: $radius-md;
  overflow: hidden;
  margin-bottom: $spacing-md;
}

.menu-item {
  display: flex;
  align-items: center;
  padding: $spacing-md;
  cursor: pointer;
  border-bottom: 1px solid $color-border-lighter;
  transition: background 0.2s;

  &:last-child { border-bottom: none; }
  &:active { background: $color-bg-hover; }

  > .el-icon:first-child {
    font-size: 20px;
    color: $color-primary;
    margin-right: $spacing-md;
  }
  .label {
    flex: 1;
    font-size: 15px;
    color: $color-text-primary;
  }
  .arrow {
    color: $color-text-placeholder;
    font-size: 14px;
  }
}

.logout-row {
  margin-top: $spacing-lg;
  :deep(.el-button) {
    width: 100%;
  }
}
</style>
