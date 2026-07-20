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
        <p>{{ t('login.subtitle') }}</p>
      </div>

      <!-- v0.5.0 国际化：语言切换器（右上角） -->
      <div class="lang-toggle">
        <LanguageSwitcher />
      </div>

      <el-tabs v-model="activeRole" class="login-tabs" stretch>
        <el-tab-pane :label="t('login.tabAdmin')" name="admin" />
        <el-tab-pane :label="t('login.tabTenant')" name="tenant" />
        <el-tab-pane :label="t('login.tabAgent')" name="agent" />
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
        <el-form-item :label="t('login.account')" prop="username">
          <el-input v-model="form.username" :placeholder="t('login.pleaseInputAccount')" :prefix-icon="User" autocomplete="username" />
        </el-form-item>
        <el-form-item :label="t('login.password')" prop="password">
          <el-input
            v-model="form.password"
            type="password"
            show-password
            :placeholder="t('login.pleaseInputPassword')"
            :prefix-icon="Lock"
            autocomplete="current-password"
            @keyup.enter="handleLogin"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" class="login-btn" @click="handleLogin">{{ t('login.submit') }}</el-button>
        </el-form-item>
      </el-form>

      <!-- 阶段 2：2FA 验证码 -->
      <el-form v-else label-position="top" @submit.prevent="handleTotpVerify">
        <el-form-item :label="t('login.totpLabel')">
          <el-input
            v-model="totpCode"
            :placeholder="t('login.totpPlaceholder')"
            :prefix-icon="Key"
            maxlength="6"
            @keyup.enter="handleTotpVerify"
          />
          <p class="totp-hint">{{ t('login.totpHint') }}</p>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" class="login-btn" @click="handleTotpVerify">{{ t('login.totpSubmit') }}</el-button>
        </el-form-item>
        <div class="totp-back">
          <el-link type="info" :underline="false" @click="cancelTotp">{{ t('login.backToLogin') }}</el-link>
        </div>
      </el-form>

      <div class="login-footer">
        <el-link v-if="activeRole === 'tenant'" type="primary" :underline="false" @click="goRegister">{{ t('login.registerTenant') }}</el-link>
        <el-link v-if="activeRole === 'agent'" type="warning" :underline="false" @click="goAgentRegister">{{ t('login.registerAgent') }}</el-link>
        <el-link type="info" :underline="false" @click="goHome">{{ t('login.backHome') }}</el-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, watch, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { User, Lock, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useSysConfigStore } from '@/stores/sysConfig'
import { loginApi, type UserRole } from '@/api/auth'

const { t } = useI18n()
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

// v0.5.0 国际化：表单校验规则响应式跟随 locale
const rules = computed(() => ({
  username: [{ required: true, message: t('login.pleaseInputAccount'), trigger: 'blur' }],
  password: [
    { required: true, message: t('login.pleaseInputPassword'), trigger: 'blur' },
    { min: 8, message: t('login.passwordMinLength'), trigger: 'blur' }
  ]
}))

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
        password: form.password,
        // 单阶段 2FA：如果用户已输入 totp_code，一起提交
        // 后端 doLogin 逻辑：已绑定 2FA 的账号必须传 totp_code，否则返回 1007
        ...(totpCode.value ? { totp_code: totpCode.value } : {})
      })

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
      ElMessage.success(t('login.success'))

      const redirect = (route.query.redirect as string) || auth.homePath
      router.replace(redirect)
    } catch (e: any) {
      // 后端返回 1007 = 动态验证码错误或已过期（账号已绑定 2FA 但未传 totp_code）
      // 显示 TOTP 输入框，让用户输入后重新提交（不需要 temp_token，单阶段流程）
      if (e?.code === 1007 && !totpRequired.value) {
        totpRequired.value = true
        ElMessage.info(t('login.totpRequired'))
      }
      // 其他错误已由 http 拦截器处理
    } finally {
      loading.value = false
    }
  })
}

const handleTotpVerify = async () => {
  if (!totpCode.value || totpCode.value.length !== 6) {
    ElMessage.warning(t('login.totpInvalid'))
    return
  }
  // 单阶段：直接调 handleLogin，它会带上 totp_code 重新提交
  await handleLogin()
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
  position: relative; // v0.5.0 国际化：为 lang-toggle 定位锚点
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
.lang-toggle {
  position: absolute;
  top: $spacing-md;
  right: $spacing-md;
  // 弱化视觉权重，避免抢占登录主流程注意力
  opacity: 0.8;
  &:hover { opacity: 1; }
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
