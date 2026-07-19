<!--
  登录页 - 响应式 H5
  - 三角色切换（admin/tenant/agent）
  - 用户名 + 密码
  - 2FA TOTP 二阶段（后端返回 totp_required 时显示）
  - 登录失败锁定提示
-->
<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <img src="@/assets/logo.svg" alt="logo" />
        <h1>{{ sysConfig.platformName || 'KeyAuth SaaS' }}</h1>
        <p>多租户卡密验证平台</p>
      </div>

      <el-tabs v-model="activeRole" class="login-tabs" stretch>
        <el-tab-pane label="平台管理员" name="admin" />
        <el-tab-pane label="开发者" name="tenant" />
        <el-tab-pane label="代理" name="agent" />
      </el-tabs>

      <!-- 阶段 1：账号密码 -->
      <el-form
        v-if="!totpRequired"
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleLogin"
      >
        <el-form-item label="账号" prop="username">
          <el-input v-model="form.username" placeholder="请输入账号" :prefix-icon="User" autocomplete="username" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            placeholder="请输入密码"
            :prefix-icon="Lock"
            autocomplete="current-password"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" class="login-btn" @click="handleLogin">登 录</el-button>
        </el-form-item>
      </el-form>

      <!-- 阶段 2：2FA 验证码 -->
      <el-form v-else label-position="top" @submit.prevent="handleTotpVerify">
        <el-form-item label="动态验证码">
          <el-input
            v-model="totpCode"
            placeholder="请输入 6 位动态验证码"
            :prefix-icon="Key"
            maxlength="6"
            @keyup.enter="handleTotpVerify"
          />
          <p class="totp-hint">请打开身份验证器 App（如 Google Authenticator）输入 6 位数字</p>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" class="login-btn" @click="handleTotpVerify">验 证</el-button>
        </el-form-item>
        <div class="totp-back">
          <el-link type="info" :underline="false" @click="cancelTotp">返回登录</el-link>
        </div>
      </el-form>

      <div class="login-footer">
        <el-link v-if="activeRole === 'tenant'" type="primary" :underline="false" @click="goRegister">开发者注册</el-link>
        <el-link v-if="activeRole === 'agent'" type="warning" :underline="false" @click="goAgentRegister">代理注册</el-link>
        <el-link type="info" :underline="false" @click="goHome">返回首页</el-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { User, Lock, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useSysConfigStore } from '@/stores/sysConfig'
import { loginApi, type UserRole } from '@/api/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sysConfig = useSysConfigStore()

const formRef = ref<FormInstance>()
const activeRole = ref<UserRole>('admin')
const loading = ref(false)
const totpRequired = ref(false)
const tempToken = ref('')
const totpCode = ref('')

const form = reactive({
  username: '',
  password: ''
})

const rules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 位', trigger: 'blur' }
  ]
}

// 切换角色时重置 2FA 状态
watch(activeRole, () => {
  totpRequired.value = false
  tempToken.value = ''
  totpCode.value = ''
})

const handleLogin = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      const resp = await loginApi(activeRole.value, {
        username: form.username,
        password: form.password
      })

      // 二阶段：后端要求 2FA
      if (resp.totp_required && resp.temp_token) {
        tempToken.value = resp.temp_token
        totpRequired.value = true
        ElMessage.info('请输入动态验证码')
        return
      }

      // 登录成功
      auth.setAuth({
        access_token: resp.access_token,
        refresh_token: resp.refresh_token,
        role: activeRole.value,
        userId: resp.user?.id,
        username: resp.user?.username,
        tenantId: resp.user?.tenant_id,
        expires_at: resp.expires_at
      })
      ElMessage.success('登录成功')

      const redirect = (route.query.redirect as string) || auth.homePath
      router.replace(redirect)
    } catch (e: any) {
      // 错误已由 http 拦截器处理
    } finally {
      loading.value = false
    }
  })
}

const handleTotpVerify = async () => {
  if (!totpCode.value || totpCode.value.length !== 6) {
    ElMessage.warning('请输入 6 位动态验证码')
    return
  }
  loading.value = true
  try {
    const resp = await loginApi(activeRole.value, {
      username: form.username,
      password: form.password,
      temp_token: tempToken.value,
      totp_code: totpCode.value
    })

    auth.setAuth({
      access_token: resp.access_token,
      refresh_token: resp.refresh_token,
      role: activeRole.value,
      userId: resp.user?.id,
      username: resp.user?.username,
      tenantId: resp.user?.tenant_id,
      expires_at: resp.expires_at
    })
    ElMessage.success('登录成功')

    const redirect = (route.query.redirect as string) || auth.homePath
    router.replace(redirect)
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const cancelTotp = () => {
  totpRequired.value = false
  tempToken.value = ''
  totpCode.value = ''
}

const goRegister = () => {
  router.push('/register/tenant')
}

const goAgentRegister = () => {
  router.push('/agent/register')
}

const goHome = () => {
  router.push('/')
}

// 加载平台配置
sysConfig.load()
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #f0f5ff 0%, #fff 100%);
  padding: $spacing-md;
}
.login-box {
  width: 100%;
  max-width: 400px;
  padding: $spacing-xl;
  background: #fff;
  border-radius: $radius-lg;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);

  @include mobile {
    padding: $spacing-lg;
    box-shadow: none;
    border-radius: 0;
    max-width: 100%;
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    justify-content: center;
  }
}
.login-header {
  text-align: center;
  margin-bottom: $spacing-lg;
  img { width: 56px; height: 56px; }
  h1 {
    margin: 12px 0 4px;
    font-size: 24px;
    color: $color-primary;
    @include mobile { font-size: 20px; }
  }
  p { margin: 0; color: $color-text-secondary; font-size: 13px; }
}
.login-tabs {
  margin-bottom: $spacing-md;
}
.totp-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: $color-text-secondary;
  line-height: 1.6;
}
.totp-back {
  text-align: center;
  margin-top: $spacing-sm;
}
.login-btn {
  width: 100%;
}
.login-footer {
  display: flex;
  justify-content: space-between;
  margin-top: $spacing-md;
  flex-wrap: wrap;
  gap: $spacing-sm;
}
</style>
