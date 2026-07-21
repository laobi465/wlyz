<!--
  超管账号设置（响应式 H5）
  - 基本资料：用户名/邮箱/手机/真实姓名（用户名只读，其余可编辑）
  - 修改密码
  - 2FA 两步验证（TOTP）
  - 登录设备管理
  铁律 06 待核实：除 /admin/auth/me 已实现外，change_password / profile / 2fa / devices 均为 v0.3.0 交付。
-->
<template>
  <div class="profile-page">
    <PageHeader title="账号设置" subtitle="管理超管账号资料、密码与安全设置" />

    <!-- 基本资料 -->
    <div class="app-card">
      <div class="card-title">
        <h3>基本资料</h3>
      </div>
      <el-form :model="profileForm" label-width="100px" :label-position="labelPosition" class="profile-form">
        <el-form-item label="用户名">
          <el-input :model-value="profileForm.username" disabled />
        </el-form-item>
        <el-form-item label="角色">
          <el-tag>平台超管</el-tag>
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
          ? '两步验证已启用，登录时需输入动态验证码。建议妥善保存恢复码。'
          : '启用两步验证后，登录时需输入 Google Authenticator 等动态验证码，可显著提升账号安全性。' }}
      </el-alert>

      <!-- 启用流程 -->
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
          <p class="bc-title">恢复码（请妥善保存，丢失后无法找回）：</p>
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

    <!-- 登录设备 -->
    <div class="app-card">
      <div class="card-title">
        <h3>登录设备</h3>
        <el-button link type="primary" @click="loadDevices">刷新</el-button>
      </div>
      <EmptyState v-if="!devices.length && !devicesLoading" description="暂无设备记录（待 v0.3.0 后端实现）" />
      <ResponsiveTable
        v-else
        :data="devices"
        :loading="devicesLoading"
        :total="devices.length"
        :show-pagination="false"
        :mobile-fields="deviceMobileFields"
      >
        <el-table-column prop="device_name" label="设备名称" min-width="140" />
        <el-table-column prop="ip" label="IP 地址" width="140" />
        <el-table-column prop="location" label="归属地" width="160" />
        <el-table-column prop="last_active_at" label="最后活跃" width="180">
          <template #default="scope">{{ formatDate(scope.row.last_active_at) }}</template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="scope">
            <el-tag v-if="scope.row.current" type="success" size="small">当前</el-tag>
            <el-tag v-else type="info" size="small">在线</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100" fixed="right">
          <template #default="scope">
            <el-button
              v-if="!scope.row.current"
              type="danger"
              link
              size="small"
              @click="kickDevice(scope.row)"
            >踢下线</el-button>
            <span v-else class="text-secondary">-</span>
          </template>
        </el-table-column>
      </ResponsiveTable>
    </div>

    <!-- 关闭 2FA 对话框 -->
    <el-dialog v-model="disableDialogVisible" title="关闭两步验证" :width="isMobile ? '92%' : '440px'">
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
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage, ElMessageBox, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import EmptyState from '@/components/EmptyState.vue'
import ResponsiveTable from '@/components/ResponsiveTable.vue'
import { useAuthStore } from '@/stores/auth'
import {
  currentUserApi,
  updateProfileApi,
  changePasswordApi,
  setup2FAApi,
  verify2FAApi,
  disable2FAApi,
  listLoginDevicesApi,
  kickDeviceApi,
  type CurrentUser,
  type TwoFASetupResp,
  type LoginDevice
} from '@/api/profile'

const auth = useAuthStore()
const role = 'admin' as const

const profileForm = reactive({
  username: '',
  real_name: '',
  email: '',
  phone: ''
})
const profileSaving = ref(false)

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

// 2FA
const totpEnabled = ref(false)
const settingUp = ref(false)
const setupData = ref<TwoFASetupResp | null>(null)
const verifyForm = reactive({ code: '' })
const verifying = ref(false)

const disableDialogVisible = ref(false)
const disableForm = reactive({ password: '', code: '' })
const disabling = ref(false)

// 登录设备
const devices = ref<LoginDevice[]>([])
const devicesLoading = ref(false)
const deviceMobileFields = [
  { prop: 'device_name', label: '设备' },
  { prop: 'ip', label: 'IP' },
  { prop: 'location', label: '归属地' },
  { prop: 'last_active_at', label: '最后活跃', formatter: (v: string) => formatDate(v) }
]

// v0.7.0 修复 P0-5：原 computed 读 window.innerWidth 非响应式数据，resize 不重算
// 改为响应式 ref + resize 监听器，确保窗口尺寸变化时 labelPosition 自动更新
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }

const labelPosition = computed<'top' | 'right'>(() => isMobile.value ? 'top' : 'right')

const formatDate = (s: string | null) => {
  if (!s) return '-'
  const d = new Date(s)
  // v0.7.0 修复：Invalid Date 兜底
  if (isNaN(d.getTime())) return '-'
  return d.toLocaleString('zh-CN')
}

const loadProfile = async () => {
  try {
    const data = await currentUserApi(role)
    if (data && typeof data === 'object') {
      const u = data as CurrentUser
      profileForm.username = u.username || auth.username
      profileForm.real_name = u.real_name || ''
      profileForm.email = u.email || ''
      profileForm.phone = u.phone || ''
      totpEnabled.value = !!u.totp_enabled
    }
  } catch {
    // /auth/me 已实现，失败时回退到 store 中的用户名
    profileForm.username = auth.username
  }
}

const saveProfile = async () => {
  // v0.7.0 修复：防抖守卫，避免重复点击产生重复请求
  if (profileSaving.value) return
  profileSaving.value = true
  try {
    await updateProfileApi(role, {
      real_name: profileForm.real_name,
      email: profileForm.email,
      phone: profileForm.phone
    })
    ElMessage.success('资料已保存')
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    profileSaving.value = false
  }
}

const changePassword = async () => {
  if (!pwdFormRef.value) return
  // v0.7.0 修复：防抖守卫
  if (pwdSaving.value) return
  // v0.7.0 修复：原 validate callback + async 混用易导致 pwdSaving 状态错乱
  // 改为 await validate 的 Promise 形式
  try {
    await pwdFormRef.value.validate()
  } catch {
    return
  }
  pwdSaving.value = true
  try {
    await changePasswordApi(role, { ...pwdForm })
    ElMessage.success('密码修改成功，请重新登录')
    pwdForm.old_password = ''
    pwdForm.new_password = ''
    pwdForm.confirm_password = ''
    // 密码修改后建议重新登录
    setTimeout(() => {
      auth.logout()
      location.href = '/login'
    }, 1500)
  } catch {
    // 铁律 06 待核实：POST /admin/auth/change_password 待 v0.3.0 实现
  } finally {
    pwdSaving.value = false
  }
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

const loadDevices = async () => {
  devicesLoading.value = true
  try {
    const data = await listLoginDevicesApi(role)
    devices.value = data?.list || []
  } catch {
    // 错误已由 http 拦截器处理
    devices.value = []
  } finally {
    devicesLoading.value = false
  }
}

const kickDevice = async (row: any) => {
  try {
    await ElMessageBox.confirm(`确定踢该设备下线（${row.device_name} / ${row.ip}）？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await kickDeviceApi(role, row.id)
    ElMessage.success('已踢下线')
    loadDevices()
  } catch {
    // 铁律 06 待核实
  }
}

onMounted(() => {
  // v0.7.0 修复 P0-5：注册 resize 监听，让 labelPosition 响应窗口尺寸变化
  checkMobile()
  window.addEventListener('resize', checkMobile)
  loadProfile()
  loadDevices()
})

onBeforeUnmount(() => {
  // v0.7.0 修复 P0-5：清理 resize 监听，避免内存泄漏
  window.removeEventListener('resize', checkMobile)
})
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

.profile-form {
  max-width: 520px;
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

.text-secondary {
  color: $color-text-secondary;
  font-size: 12px;
}
</style>
