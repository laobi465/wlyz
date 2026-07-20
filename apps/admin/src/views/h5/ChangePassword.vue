<!--
  H5 修改密码（v0.4.0 收尾项 C）
  - 旧密码 + 新密码 + 确认新密码
-->
<template>
  <div class="h5-change-password">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">修改密码</span>
    </div>

    <div class="form-card">
      <p class="section-label">旧密码</p>
      <el-input v-model="form.oldPassword" type="password" placeholder="请输入旧密码" show-password />

      <p class="section-label">新密码</p>
      <el-input v-model="form.newPassword" type="password" placeholder="请输入新密码（至少 6 位）" show-password />

      <p class="section-label">确认新密码</p>
      <el-input v-model="form.confirmPassword" type="password" placeholder="请再次输入新密码" show-password />

      <div class="submit-row">
        <el-button type="primary" size="large" :loading="saving" @click="save">保存</el-button>
      </div>

      <p class="tip">修改密码后，其它设备的会话不会自动失效；如需让其它设备下线，请前往「会话管理」踢下线。</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { endUserChangePasswordApi } from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()

const form = reactive({
  oldPassword: '',
  newPassword: '',
  confirmPassword: ''
})
const saving = ref(false)

const save = async () => {
  if (!form.oldPassword) {
    ElMessage.warning('请输入旧密码')
    return
  }
  if (!form.newPassword) {
    ElMessage.warning('请输入新密码')
    return
  }
  if (form.newPassword.length < 6) {
    ElMessage.warning('新密码至少 6 位')
    return
  }
  if (form.newPassword !== form.confirmPassword) {
    ElMessage.warning('两次新密码不一致')
    return
  }
  if (form.newPassword === form.oldPassword) {
    ElMessage.warning('新密码不能与旧密码相同')
    return
  }

  saving.value = true
  try {
    await endUserChangePasswordApi({
      old_password: form.oldPassword,
      new_password: form.newPassword
    })
    ElMessage.success('密码修改成功')
    router.replace('/h5/profile')
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
  }
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-change-password {
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

.submit-row {
  margin-top: $spacing-lg;
  :deep(.el-button) { width: 100%; }
}

.tip {
  margin-top: $spacing-md;
  font-size: 12px;
  color: $color-text-secondary;
  line-height: 1.6;
}
</style>
