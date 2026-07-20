<!--
  H5 我的卡密（v0.4.0 收尾项 C）
  - 列表分页展示
  - 绑定卡密弹窗
  - 解绑卡片
  - 点击卡片跳详情
-->
<template>
  <div class="h5-my-cards">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">我的卡密</span>
      <el-button type="primary" text class="bind-btn" @click="openBindDialog">
        <el-icon><Plus /></el-icon>绑定
      </el-button>
    </div>

    <div v-loading="loading">
      <div v-if="cards.length === 0 && !loading" class="empty-card">
        <el-empty description="暂无绑定的卡密" :image-size="80">
          <el-button type="primary" @click="openBindDialog">绑定卡密</el-button>
        </el-empty>
      </div>

      <div
        v-for="card in cards"
        :key="card.id"
        class="card-item"
        @click="goCardDetail(card)"
      >
        <div class="card-head">
          <span class="card-key">{{ maskKey(card.card_key) }}</span>
          <el-tag :type="statusTagType(card.status)" size="small">{{ statusText(card.status) }}</el-tag>
        </div>
        <div class="card-meta">
          <span v-if="card.app_name" class="meta-text">应用：{{ card.app_name }}</span>
          <span class="meta-text">类型：{{ card.card_type }}</span>
        </div>
        <div class="card-meta">
          <span class="meta-text">绑定：{{ formatTime(card.bound_at) }}</span>
          <span v-if="card.expires_at" class="meta-text">到期：{{ formatTime(card.expires_at) }}</span>
        </div>
        <div class="card-actions" @click.stop>
          <el-button text size="small" @click="copyKey(card.card_key)">复制</el-button>
          <el-button text size="small" type="danger" @click="unbind(card)">解绑</el-button>
        </div>
      </div>
    </div>

    <div v-if="hasMore" class="load-more">
      <el-button :loading="loadingMore" @click="loadMore">加载更多</el-button>
    </div>

    <!-- 绑定卡密弹窗 -->
    <el-dialog v-model="bindDialogVisible" title="绑定卡密" width="90%" :close-on-click-modal="false">
      <el-input v-model="bindCardKey" placeholder="请输入卡密" clearable />
      <template #footer>
        <el-button @click="bindDialogVisible = false">取消</el-button>
        <el-button type="primary" :loading="binding" @click="bindCard">绑定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowLeft, Plus } from '@element-plus/icons-vue'
import {
  endUserListMyCardsApi,
  endUserBindCardApi,
  endUserUnbindCardApi,
  type EndUserCard
} from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()

const cards = ref<EndUserCard[]>([])
const page = ref(1)
const pageSize = 20
const total = ref(0)
const loading = ref(false)
const loadingMore = ref(false)

const bindDialogVisible = ref(false)
const bindCardKey = ref('')
const binding = ref(false)

const hasMore = computed(() => cards.value.length < total.value)

const loadFirst = async () => {
  loading.value = true
  page.value = 1
  try {
    const resp = await endUserListMyCardsApi(page.value, pageSize)
    cards.value = resp.list || []
    total.value = resp.total || 0
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const loadMore = async () => {
  if (!hasMore.value) return
  loadingMore.value = true
  page.value++
  try {
    const resp = await endUserListMyCardsApi(page.value, pageSize)
    cards.value.push(...(resp.list || []))
    total.value = resp.total || 0
  } catch {
    page.value--
  } finally {
    loadingMore.value = false
  }
}

const openBindDialog = () => {
  bindCardKey.value = ''
  bindDialogVisible.value = true
}

const bindCard = async () => {
  if (!bindCardKey.value) {
    ElMessage.warning('请输入卡密')
    return
  }
  binding.value = true
  try {
    const card = await endUserBindCardApi(bindCardKey.value)
    ElMessage.success('卡密绑定成功')
    bindDialogVisible.value = false
    cards.value.unshift(card)
    total.value++
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    binding.value = false
  }
}

const unbind = async (card: EndUserCard) => {
  try {
    await ElMessageBox.confirm(`确定要解绑卡密 ${maskKey(card.card_key)} 吗？`, '提示', {
      type: 'warning',
      confirmButtonText: '解绑',
      cancelButtonText: '取消'
    })
  } catch {
    return
  }
  try {
    await endUserUnbindCardApi(card.id)
    ElMessage.success('已解绑')
    cards.value = cards.value.filter((c) => c.id !== card.id)
    total.value--
  } catch {
    // 错误已由 http 拦截器处理
  }
}

const goCardDetail = (card: EndUserCard) => {
  router.push(`/h5/card/${encodeURIComponent(card.card_key)}`)
}

const copyKey = (key: string) => {
  navigator.clipboard.writeText(key).then(() => {
    ElMessage.success('已复制')
  }).catch(() => {
    ElMessage.error('复制失败，请手动长按复制')
  })
}

const maskKey = (key: string) => {
  if (!key) return ''
  if (key.length <= 8) return key
  return key.slice(0, 4) + '****' + key.slice(-4)
}

const statusTagType = (s: string): any => {
  const map: Record<string, any> = {
    unused: 'info',
    active: 'success',
    expired: 'warning',
    banned: 'danger',
    disabled: 'info',
    bound: 'success'
  }
  return map[s] || 'info'
}

const statusText = (s: string) => {
  const map: Record<string, string> = {
    unused: '未使用',
    active: '正常',
    expired: '已过期',
    banned: '已封禁',
    disabled: '已禁用',
    bound: '已绑定'
  }
  return map[s] || s
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
  loadFirst()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-my-cards {
  max-width: 640px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  align-items: center;
  padding: $spacing-sm 0 $spacing-md;
  position: relative;
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-md;

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
  .bind-btn {
    margin-left: auto;
  }
}

.empty-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
}

.card-item {
  background: #fff;
  border: 1px solid $color-border-lighter;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-sm;
  cursor: pointer;
  transition: all 0.2s;

  &:active {
    border-color: $color-primary;
    background: $color-primary-light;
  }

  .card-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-sm;

    .card-key {
      font-family: monospace;
      font-size: 14px;
      font-weight: 600;
      color: $color-text-primary;
      word-break: break-all;
    }
  }

  .card-meta {
    display: flex;
    gap: $spacing-md;
    flex-wrap: wrap;
    margin-bottom: 4px;
    .meta-text {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }

  .card-actions {
    display: flex;
    justify-content: flex-end;
    gap: $spacing-sm;
    margin-top: $spacing-sm;
    border-top: 1px solid $color-border-lighter;
    padding-top: $spacing-sm;
  }
}

.load-more {
  text-align: center;
  margin-top: $spacing-md;
  :deep(.el-button) {
    width: 100%;
  }
}
</style>
