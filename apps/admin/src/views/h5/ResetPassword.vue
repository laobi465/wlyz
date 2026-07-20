<!--
  H5 终端用户重置密码页（v0.4.0 收尾项 C）
  - AppKey + 邮箱/手机 + 验证码 + 新密码 + 确认新密码
-->
<template>
  <div class="h5-reset">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">重置密码</span>
    </div>

    <div class="form-card">
      <p class="section-label">应用 AppKey</p>
      <el-input v-model="form.appKey" placeholder="请输入开发者提供的 AppKey" clearable />

      <p class="section-label">账号</p>
      <el-input v-model="form.username" placeholder="请输入要重置密码的账号" clearable />

      <p class="section-label">联系方式</p>
      <el-radio-group v-model="contactType" class="contact-type">
        <el-radio value="email" label="email">邮箱</el-radio>
        <el-radio value="phone" label="phone">手机号</el-radio>
      </el-radio-group>
      <el-input v-if="contactType === 'email'" v-model="form.email" placeholder="请输入注册邮箱" clearable />
      <el-input v-else v-model="form.phone" placeholder="请输入注册手机号" clearable />

      <p class="section-label">验证码</p>
      <div class="code-row">
        <el-input v-model="form.verifyCode" placeholder="请输入验证码" clearable />
        <el-button :disabled="counting > 0 || sending" :loading="sending" @click="sendCode">
          {{ counting > 0 ? `${counting}s` : '发送验证码' }}
        </el-button>
      </div>

      <p class="section-label">新密码</p>
      <el-input v-model="form.newPassword" type="password" placeholder="请输入新密码" show-password />

      <p class="section-label">确认新密码</p>
      <el-input v-model="form.confirmPassword" type="password" placeholder="请再次输入新密码" show-password />

      <div class="submit-row">
        <el-button type="primary" size="large" :loading="submitting" @click="submit">重置密码</el-button>
      </div>

      <div class="links">
        <router-link to="/h5/login" class="link">返回登录</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { endUserResetPasswordApi, endUserSendVerifyCodeApi } from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const router = useRouter()
const endUserStore = useEndUserStore()

const contactType = ref<'email' | 'phone'>('email')

const form = reactive({
  appKey: '',
  username: '',
  email: '',
  phone: '',
  verifyCode: '',
  newPassword: '',
  confirmPassword: ''
})

const submitting = ref(false)
const sending = ref(false)
const counting = ref(0)
let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  endUserStore.restore()
  if (endUserStore.appKey) {
    form.appKey = endUserStore.appKey
  }
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
})

const validateContact = (): string => {
  if (contactType.value === 'email') {
    if (!form.email) {
      ElMessage.warning('请输入邮箱')
      return ''
    }
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email)) {
      ElMessage.warning('邮箱格式不正确')
      return ''
    }
    return form.email
  } else {
    if (!form.phone) {
      ElMessage.warning('请输入手机号')
      return ''
    }
    if (!/^\d{6,15}$/.test(form.phone)) {
      ElMessage.warning('手机号格式不正确')
      return ''
    }
    return form.phone
  }
}

const sendCode = async () => {
  if (!form.appKey) {
    ElMessage.warning('请先填写 AppKey')
    return
  }
  const target = validateContact()
  if (!target) return

  sending.value = true
  try {
    // P0 高危 11/12：后端 H5SendVerifyCode 接收 channel（sms/email）+ recipient
    await endUserSendVerifyCodeApi({
      app_key: form.appKey,
      channel: contactType.value === 'email' ? 'email' : 'sms',
      recipient: target,
      purpose: 'reset_password'
    })
    ElMessage.success('验证码已发送，请注意查收')
    counting.value = 60
    timer = setInterval(() => {
      counting.value--
      if (counting.value <= 0 && timer) {
        clearInterval(timer)
        timer = null
      }
    }, 1000)
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    sending.value = false
  }
}

const submit = async () => {
  if (!form.appKey) {
    ElMessage.warning('请填写 AppKey')
    return
  }
  if (!form.username) {
    ElMessage.warning('请输入账号')
    return
  }
  const target = validateContact()
  if (!target) return
  if (!form.verifyCode) {
    ElMessage.warning('请输入验证码')
    return
  }
  if (!form.newPassword) {
    ElMessage.warning('请输入新密码')
    return
  }
  if (form.newPassword.length < 6) {
    ElMessage.warning('密码至少 6 位')
    return
  }
  if (form.newPassword !== form.confirmPassword) {
    ElMessage.warning('两次密码不一致')
    return
  }

  submitting.value = true
  try {
    // P0 高危 12：后端 H5ResetPassword 接收 username + password + channel + recipient + verify_code
    await endUserResetPasswordApi({
      app_key: form.appKey,
      username: form.username,
      password: form.newPassword,
      channel: contactType.value === 'email' ? 'email' : 'sms',
      recipient: target,
      verify_code: form.verifyCode
    })
    ElMessage.success('密码已重置，请使用新密码登录')
    router.replace('/h5/login')
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    submitting.value = false
  }
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/login')
  }
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-reset {
  max-width: 640px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  align-items: center;
  padding: $spacing-sm 0 $spacing-md;
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

.contact-type {
  margin-bottom: $spacing-sm;
  :deep(.el-radio) {
    margin-right: $spacing-md;
  }
}

.code-row {
  display: flex;
  gap: $spacing-sm;
  :deep(.el-input) { flex: 1; }
  :deep(.el-button) { flex-shrink: 0; }
}

.submit-row {
  margin-top: $spacing-lg;
  :deep(.el-button) { width: 100%; }
}

.links {
  display: flex;
  justify-content: center;
  margin-top: $spacing-md;

  .link {
    font-size: 13px;
    color: $color-primary;
    text-decoration: none;
  }
}
</style>
