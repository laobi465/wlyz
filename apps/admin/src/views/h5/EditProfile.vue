<!--
  H5 编辑资料（v0.4.0 收尾项 C）
  - 修改昵称 + 头像 URL + 邮箱 + 手机
-->
<template>
  <div class="h5-edit-profile">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">编辑资料</span>
    </div>

    <div class="form-card">
      <p class="section-label">头像 URL</p>
      <div class="avatar-row">
        <el-avatar v-if="form.avatar" :src="form.avatar" :size="56" />
        <el-avatar v-else :size="56">{{ avatarPlaceholder }}</el-avatar>
        <el-input v-model="form.avatar" placeholder="请输入头像 URL" clearable />
      </div>

      <p class="section-label">昵称</p>
      <el-input v-model="form.nickname" placeholder="请输入昵称" clearable maxlength="32" show-word-limit />

      <p class="section-label">邮箱</p>
      <el-input v-model="form.email" placeholder="请输入邮箱" clearable />

      <p class="section-label">手机号</p>
      <el-input v-model="form.phone" placeholder="请输入手机号" clearable />

      <div class="submit-row">
        <el-button type="primary" size="large" :loading="saving" @click="save">保存</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { endUserMeApi, endUserUpdateProfileApi } from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()

const form = reactive({
  nickname: '',
  avatar: '',
  email: '',
  phone: ''
})
const saving = ref(false)

const avatarPlaceholder = computed(() => {
  const name = form.nickname || endUserStore.user?.username || '?'
  return name.charAt(0).toUpperCase()
})

const loadProfile = async () => {
  try {
    const info = await endUserMeApi()
    form.nickname = info.nickname || ''
    form.avatar = info.avatar || ''
    form.email = info.email || ''
    form.phone = info.phone || ''
    endUserStore.setUser(info)
  } catch {
    // 错误已由 http 拦截器处理
  }
}

const save = async () => {
  if (form.email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
    ElMessage.warning('邮箱格式不正确')
    return
  }
  if (form.phone && !/^\d{6,15}$/.test(form.phone)) {
    ElMessage.warning('手机号格式不正确')
    return
  }

  saving.value = true
  try {
    const info = await endUserUpdateProfileApi({
      nickname: form.nickname,
      avatar: form.avatar,
      email: form.email,
      phone: form.phone
    })
    endUserStore.setUser(info)
    ElMessage.success('保存成功')
    router.back()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    saving.value = false
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
  const u = endUserStore.user
  if (u) {
    form.nickname = u.nickname || ''
    form.avatar = u.avatar || ''
    form.email = u.email || ''
    form.phone = u.phone || ''
  }
  loadProfile()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-edit-profile {
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

.section-label {
  font-size: 13px;
  color: $color-text-secondary;
  margin: $spacing-md 0 $spacing-sm;
}

.form-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;
}

.avatar-row {
  display: flex;
  align-items: center;
  gap: $spacing-md;
  :deep(.el-input) { flex: 1; }
}

.submit-row {
  margin-top: $spacing-lg;
  :deep(.el-button) { width: 100%; }
}
</style>
