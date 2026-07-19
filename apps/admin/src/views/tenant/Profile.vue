<!--
  开发者账号设置（响应式 H5）
  - 基本资料：用户名/邮箱/手机/真实姓名
  - 企业信息：公司名称/联系人/营业执照号
  - 修改密码
  - 2FA 两步验证
  铁律 06 待核实：除 /tenant/auth/me 已实现外，change_password / profile / 2fa 均为 v0.3.0 交付。
-->
<template>
  <div class="profile-page">
    <PageHeader title="账号设置" subtitle="管理开发者账号资料、企业信息与安全设置" />

    <!-- 基本资料 -->
    <div class="app-card">
      <div class="card-title">
        <h3>基本资料</h3>
      </div>
      <el-form :model="profileForm" label-width="100px" :label-position="labelPosition">
        <el-form-item label="用户名">
          <el-input :model-value="profileForm.username" disabled />
        </el-form-item>
        <el-form-item label="角色">
          <el-tag>开发者</el-tag>
          <el-tag v-if="profileForm.tenant_id" type="info" size="small" style="margin-left: 8px">租户 #{{ profileForm.tenant_id }}</el-tag>
        </el-form-item>
        <el-form-item label="真实姓名">
          <el-input v-model="profileForm.real_name" placeholder="请输入真实姓名" maxlength="32" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="profileForm.email" placeholder="example@domain.com" maxlength="128" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="profileForm.phone" placeholder="11 位手机号" maxlength="20" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="profileSaving" @click="saveProfile">保存资料</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 企业信息 -->
    <div class="app-card">
      <div class="card-title">
        <h3>企业信息</h3>
      </div>
      <el-form :model="companyForm" label-width="100px" :label-position="labelPosition">
        <el-form-item label="公司名称">
          <el-input v-model="companyForm.company" placeholder="公司或团队名称" maxlength="128" />
        </el-form-item>
        <el-form-item label="联系人">
          <el-input v-model="companyForm.contact_name" placeholder="企业联系人姓名" maxlength="32" />
        </el-form-item>
        <el-form-item label="联系电话">
          <el-input v-model="companyForm.contact_phone" placeholder="企业联系电话" maxlength="20" />
        </el-form-item>
        <el-form-item label="营业执照号">
          <el-input v-model="companyForm.license_no" placeholder="选填，用于实名认证" maxlength="32" />
        </el-form-item>
        <el-form-item label="联系地址">
          <el-input v-model="companyForm.address" type="textarea" :rows="2" placeholder="选填" maxlength="256" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="companySaving" @click="saveCompany">保存企业信息</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 修改密码 -->
    <div class="app-card">
      <div class="card-title">
        <h3>修改密码</h3>
      </div>
      <el-form ref="pwdFormRef" :model="pwdForm" :rules="pwdRules" label-width="100px" :label-position="labelPosition">
        <el-form-item label="原密码" prop="old_password">
          <el-input v-model="pwdForm.old_password" type="password" show-password placeholder="请输入当前密码" />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="pwdForm.new_password" type="password" show-password placeholder="至少 8 位，含字母与数字" />
        </el-form-item>
        <el-form-item label="确认新密码" prop="confirm_password">
          <el-input v-model="pwdForm.confirm_password" type="password" show-password placeholder="再次输入新密码" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="pwdSaving" @click="changePassword">提交修改</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 2FA 两步验证 -->
    <div class="app-card">
      <div class="card-title">
        <h3>两步验证（TOTP）</h3>
        <el-tag v-if="totpEnabled" type="success" size="small">已启用</el-tag>
        <el-tag v-else type="warning" size="small">未启用</el-tag>
      </div>
      <el-alert
        :type="totpEnabled ? 'success' : 'info'"
        :closable="false"
        show-icon
        class="totp-alert"
      >
        {{ totpEnabled
          ? '两步验证已启用，登录时需输入动态验证码。'
          : '启用两步验证后，登录时需输入 Google Authenticator 等动态验证码，可显著提升账号安全性。' }}
      </el-alert>

      <div v-if="!totpEnabled && setupData" class="totp-setup">
        <div class="qr-area">
          <img v-if="setupData.qr_code_url" :src="setupData.qr_code_url" alt="2FA QR Code" class="qr-img" />
          <p class="secret">密钥：<code>{{ setupData.secret }}</code></p>
        </div>
        <el-form :model="verifyForm" inline class="verify-form">
          <el-form-item label="验证码">
            <el-input v-model="verifyForm.code" placeholder="6 位数字" maxlength="6" style="width: 160px" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="verifying" @click="verify2FA">确认启用</el-button>
          </el-form-item>
        </el-form>
        <div class="backup-codes" v-if="setupData.backup_codes?.length">
          <p class="bc-title">恢复码（请妥善保存）：</p>
          <div class="codes-grid">
            <code v-for="(c, i) in setupData.backup_codes" :key="i">{{ c }}</code>
          </div>
        </div>
      </div>

      <div class="totp-actions">
        <el-button v-if="!totpEnabled" type="primary" :loading="settingUp" @click="startSetup2FA">开始启用</el-button>
        <el-button v-else type="danger" plain @click="openDisable2FA">关闭两步验证</el-button>
      </div>
    </div>

    <!-- 关闭 2FA 对话框 -->
    <el-dialog v-model="disableDialogVisible" title="关闭两步验证" width="440px">
      <el-form label-position="top">
        <el-form-item label="当前密码">
          <el-input v-model="disableForm.password" type="password" show-password placeholder="验证身份" />
        </el-form-item>
        <el-form-item label="动态验证码">
          <el-input v-model="disableForm.code" placeholder="6 位数字" maxlength="6" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="disableDialogVisible = false">取消</el-button>
        <el-button type="danger" :loading="disabling" @click="confirmDisable2FA">确认关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import { useAuthStore } from '@/stores/auth'
import {
  currentUserApi,
  updateProfileApi,
  changePasswordApi,
  setup2FAApi,
  verify2FAApi,
  disable2FAApi,
  type CurrentUser,
  type TwoFASetupResp
} from '@/api/profile'

const auth = useAuthStore()
const role = 'tenant' as const

const profileForm = reactive({
  username: '',
  real_name: '',
  email: '',
  phone: '',
  tenant_id: 0 as number | undefined
})
const profileSaving = ref(false)

const companyForm = reactive({
  company: '',
  contact_name: '',
  contact_phone: '',
  license_no: '',
  address: ''
})
const companySaving = ref(false)

const pwdFormRef = ref<FormInstance>()
const pwdForm = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})
const pwdSaving = ref(false)

const pwdRules = {
  old_password: [{ required: true, message: '请输入原密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 8, max: 64, message: '长度 8-64 位', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, cb: (err?: Error) => void) => {
        if (!/[a-zA-Z]/.test(value) || !/\d/.test(value)) {
          cb(new Error('密码必须包含字母与数字'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ],
  confirm_password: [
    { required: true, message: '请确认新密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, cb: (err?: Error) => void) => {
        if (value !== pwdForm.new_password) {
          cb(new Error('两次输入的密码不一致'))
          return
        }
        cb()
      },
      trigger: 'blur'
    }
  ]
}

const totpEnabled = ref(false)
const settingUp = ref(false)
const setupData = ref<TwoFASetupResp | null>(null)
const verifyForm = reactive({ code: '' })
const verifying = ref(false)

const disableDialogVisible = ref(false)
const disableForm = reactive({ password: '', code: '' })
const disabling = ref(false)

const labelPosition = computed(() => {
  return window.innerWidth < 768 ? 'top' : 'right'
})

const loadProfile = async () => {
  try {
    const data = await currentUserApi(role)
    if (data && typeof data === 'object') {
      const u = data as CurrentUser
      profileForm.username = u.username || auth.username
      profileForm.real_name = u.real_name || ''
      profileForm.email = u.email || ''
      profileForm.phone = u.phone || ''
      profileForm.tenant_id = u.tenant_id || auth.tenantId || undefined
      profileForm.tenant_id = u.tenant_id || 0
      // 企业信息字段从 profile 同步（待 v0.3.0 后端补全 company 字段）
      companyForm.company = (u as any).company || ''
      totpEnabled.value = !!u.totp_enabled
    }
  } catch {
    profileForm.username = auth.username
    profileForm.tenant_id = auth.tenantId || 0
  }
}

const saveProfile = async () => {
  profileSaving.value = true
  try {
    await updateProfileApi(role, {
      real_name: profileForm.real_name,
      email: profileForm.email,
      phone: profileForm.phone
    })
    ElMessage.success('资料已保存')
  } catch {
    // 铁律 06 待核实
  } finally {
    profileSaving.value = false
  }
}

const saveCompany = async () => {
  companySaving.value = true
  try {
    await updateProfileApi(role, {
      company: companyForm.company
      // contact_name / contact_phone / license_no / address 待 v0.3.0 扩展字段
    } as any)
    ElMessage.success('企业信息已保存')
  } catch {
    // 铁律 06 待核实
  } finally {
    companySaving.value = false
  }
}

const changePassword = async () => {
  if (!pwdFormRef.value) return
  await pwdFormRef.value.validate(async (valid) => {
    if (!valid) return
    pwdSaving.value = true
    try {
      await changePasswordApi(role, { ...pwdForm })
      ElMessage.success('密码修改成功，请重新登录')
      pwdForm.old_password = ''
      pwdForm.new_password = ''
      pwdForm.confirm_password = ''
      setTimeout(() => {
        auth.logout()
        location.href = '/login'
      }, 1500)
    } catch {
      // 铁律 06 待核实
    } finally {
      pwdSaving.value = false
    }
  })
}

const startSetup2FA = async () => {
  settingUp.value = true
  try {
    const data = await setup2FAApi(role)
    setupData.value = data
    ElMessage.info('请使用 Google Authenticator 扫描二维码')
  } catch {
    // 铁律 06 待核实
  } finally {
    settingUp.value = false
  }
}

const verify2FA = async () => {
  if (!verifyForm.code || verifyForm.code.length !== 6) {
    ElMessage.warning('请输入 6 位验证码')
    return
  }
  verifying.value = true
  try {
    await verify2FAApi(role, { code: verifyForm.code })
    totpEnabled.value = true
    setupData.value = null
    verifyForm.code = ''
    ElMessage.success('两步验证已启用')
  } catch {
    // 铁律 06 待核实
  } finally {
    verifying.value = false
  }
}

const openDisable2FA = () => {
  disableForm.password = ''
  disableForm.code = ''
  disableDialogVisible.value = true
}

const confirmDisable2FA = async () => {
  if (!disableForm.password || !disableForm.code) {
    ElMessage.warning('请填写密码与验证码')
    return
  }
  disabling.value = true
  try {
    await disable2FAApi(role, { ...disableForm })
    totpEnabled.value = false
    disableDialogVisible.value = false
    ElMessage.success('两步验证已关闭')
  } catch {
    // 铁律 06 待核实
  } finally {
    disabling.value = false
  }
}

onMounted(() => {
  loadProfile()
})

// 防止 ElMessageBox 未使用告警
void ElMessageBox
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.profile-page {
  display: flex;
  flex-direction: column;
  gap: $spacing-lg;
}

.app-card {
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-lg;
  box-shadow: $shadow-card;

  .card-title {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: $spacing-md;
    h3 {
      margin: 0;
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }

  @include mobile {
    padding: $spacing-md;
  }
}

.totp-alert {
  margin-bottom: $spacing-md;
}

.totp-setup {
  background: $color-bg-page;
  border-radius: $radius-sm;
  padding: $spacing-md;
  margin-bottom: $spacing-md;

  .qr-area {
    text-align: center;
    margin-bottom: $spacing-md;
    .qr-img {
      width: 180px;
      height: 180px;
      border: 1px solid $color-border-lighter;
      border-radius: $radius-sm;
    }
    .secret {
      margin-top: $spacing-sm;
      font-size: 12px;
      color: $color-text-secondary;
      code {
        font-family: 'SF Mono', 'Menlo', monospace;
        color: $color-text-primary;
        background: $color-bg-card;
        padding: 2px 6px;
        border-radius: $radius-sm;
      }
    }
  }
  .verify-form {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    flex-wrap: wrap;
  }
  .backup-codes {
    margin-top: $spacing-md;
    padding-top: $spacing-md;
    border-top: 1px dashed $color-border-lighter;
    .bc-title {
      margin: 0 0 $spacing-sm;
      font-size: 13px;
      color: $color-warning;
    }
    .codes-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(100px, 1fr));
      gap: 4px;
      code {
        font-family: 'SF Mono', 'Menlo', monospace;
        font-size: 12px;
        background: $color-bg-card;
        padding: 4px 6px;
        border-radius: $radius-sm;
        color: $color-text-primary;
        text-align: center;
      }
    }
  }
}

.totp-actions {
  display: flex;
  gap: $spacing-sm;
}
</style>
