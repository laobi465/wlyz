<!--
  登录页 - 响应式 H5
  - v0.9.0：根据路由 meta.loginMode 决定显示哪些角色 Tab
    - /login (loginMode='user')：仅显示 tenant + agent（用户端不显示管理员入口）
    - /admin/login (loginMode='admin')：仅显示 admin（管理员独立登录入口）
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
        <p>{{ isAdminMode ? t('login.subtitleAdmin') : t('login.subtitle') }}</p>
      </div>

      <!-- v0.5.0 国际化：语言切换器（右上角） -->
      <div class="lang-toggle">
        <LanguageSwitcher />
      </div>

      <!-- v0.9.0：管理员模式只显示 admin Tab，用户端模式显示 tenant + agent Tab -->
      <el-tabs v-if="visibleRoles.length > 1" v-model="activeRole" class="login-tabs" stretch>
        <el-tab-pane v-if="showAdminTab" :label="t('login.tabAdmin')" name="admin" />
        <el-tab-pane v-if="showTenantTab" :label="t('login.tabTenant')" name="tenant" />
        <el-tab-pane v-if="showAgentTab" :label="t('login.tabAgent')" name="agent" />
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

// v0.9.0：根据路由 meta.loginMode 决定显示哪些角色 Tab
// - /login (loginMode='user')：用户端，显示 tenant + agent，不显示 admin
// - /admin/login (loginMode='admin')：管理员独立登录入口，只显示 admin
const isAdminMode = computed(() => route.meta.loginMode === 'admin')
const showAdminTab = computed(() => isAdminMode.value)
const showTenantTab = computed(() => !isAdminMode.value)
const showAgentTab = computed(() => !isAdminMode.value)
const visibleRoles = computed<UserRole[]>(() => {
  if (isAdminMode.value) return ['admin']
  return ['tenant', 'agent']
})

const formRef = ref<FormInstance>()
// v0.9.0：默认角色跟随 loginMode（admin 模式默认 admin，用户端默认 tenant）
const activeRole = ref<UserRole>(isAdminMode.value ? 'admin' : 'tenant')
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
  // v0.6.5 修复：改为 Promise 风格，避免 callback 风格下 await 是 no-op
  // 原 `await formRef.value.validate(async (valid) => {...})` 中 await 立即 resolve，
  // callback 内的异常可能变成 unhandled rejection，且 finally 立即执行
  try {
    await formRef.value.validate()
  } catch {
    return // 校验失败
  }
  loading.value = true
  try {
    const resp = await loginApi(activeRole.value, {
      username: form.username,
      password: form.password,
      // 单阶段 2FA：如果用户已输入 totp_code，一起提交
      // 后端 doLogin 逻辑：已绑定 2FA 的账号必须传 totp_code，否则返回 1007
      ...(totpCode.value ? { totp_code: totpCode.value } : {})
    })

    // v0.6.5 修复：优先使用后端返回的 role（权威来源），兜底用 UI Tab 选择的角色
    // 后端 resp.user.role 是从 DB 查到的真实角色，UI Tab 只是用户选择
    // 如果后端未返回 role 或与 UI Tab 不一致，以后端为准（防幻觉：不盲目信任前端状态）
    const serverRole = (resp.user?.role as UserRole) || activeRole.value

    // 登录成功
    auth.setAuth({
      access_token: resp.access_token,
      refresh_token: resp.refresh_token,
      role: serverRole,
      userId: resp.user?.id,
      username: resp.user?.username,
      tenantId: resp.user?.tenant_id,
      expires_at: resp.expires_at
    })
    ElMessage.success(t('login.success'))

    // v0.6.5 修复：redirect 白名单校验，防止篡改 redirect 跳到 404 或外部 URL
    // 只允许 /admin /tenant /agent 开头的相对路径
    const queryRedirect = route.query.redirect as string
    const validRoles: UserRole[] = ['admin', 'tenant', 'agent']
    const isValidRedirect = (p: string) =>
      typeof p === 'string' &&
      validRoles.some((r) => p === `/${r}` || p.startsWith(`/${r}/`))

    const redirect = isValidRedirect(queryRedirect) ? queryRedirect! : auth.homePath
    await router.replace(redirect)
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

// v0.9.0：代理注册已移至 /register/agent（独立顶层路由，不再嵌套 AgentLayout）
const goAgentRegister = () => {
  router.push('/register/agent')
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
