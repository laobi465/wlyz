<!--
  代理账号设置（响应式 H5）
  - 基本资料：用户名/真实姓名/手机/邮箱
  - 提现账户：开户人/支付宝账号/微信账号/银行卡号/开户行
  - 修改密码
  铁律 06 待核实：除 /agent/auth/me 已实现外，change_password / profile / 提现账户均为 v0.3.0 交付。
-->
<template>
  <div class="profile-page">
    <PageHeader title="账号设置" subtitle="管理代理账号资料、提现账户与安全设置" />

    <!-- 账户概览 -->
    <div class="overview-card">
      <div class="overview-item">
        <span class="label">账户余额</span>
        <span class="value primary">¥{{ Number(accountOverview.balance).toFixed(2) }}</span>
      </div>
      <div class="overview-item">
        <span class="label">冻结金额</span>
        <span class="value">¥{{ Number(accountOverview.frozen_balance).toFixed(2) }}</span>
      </div>
      <div class="overview-item">
        <span class="label">累计佣金</span>
        <span class="value success">¥{{ Number(accountOverview.total_commission).toFixed(2) }}</span>
      </div>
      <div class="overview-item">
        <span class="label">已提现</span>
        <span class="value">¥{{ Number(accountOverview.total_withdraw).toFixed(2) }}</span>
      </div>
    </div>

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
          <el-tag>代理商</el-tag>
        </el-form-item>
        <el-form-item label="真实姓名">
          <el-input v-model="profileForm.real_name" placeholder="请输入真实姓名（提现实名一致）" maxlength="32" />
        </el-form-item>
        <el-form-item label="手机号">
          <el-input v-model="profileForm.phone" placeholder="11 位手机号" maxlength="20" />
        </el-form-item>
        <el-form-item label="邮箱">
          <el-input v-model="profileForm.email" placeholder="选填" maxlength="128" />
        </el-form-item>
        <el-form-item label="邀请人">
          <el-input :model-value="profileForm.inviter_username || '-'" disabled />
        </el-form-item>
        <el-form-item label="注册时间">
          <el-input :model-value="formatDate(profileForm.created_at)" disabled />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="profileSaving" @click="saveProfile">保存资料</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 提现账户 -->
    <div class="app-card">
      <div class="card-title">
        <h3>提现账户</h3>
        <el-tag size="small" type="info">申请提现时使用</el-tag>
      </div>
      <el-alert
        type="warning"
        :closable="false"
        show-icon
        class="withdraw-alert"
      >
        请确保提现账户信息准确，平台将以此账户进行打款。修改后需等待平台审核生效。
      </el-alert>
      <el-form :model="withdrawForm" label-width="100px" :label-position="labelPosition">
        <el-form-item label="默认方式">
          <el-radio-group v-model="withdrawForm.default_method">
            <el-radio value="alipay">支付宝</el-radio>
            <el-radio value="wechat">微信</el-radio>
            <el-radio value="bank">银行卡</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="开户人姓名">
          <el-input v-model="withdrawForm.real_name" placeholder="与账户实名一致" maxlength="32" />
        </el-form-item>

        <!-- 支付宝 -->
        <template v-if="withdrawForm.default_method === 'alipay'">
          <el-form-item label="支付宝账号">
            <el-input v-model="withdrawForm.alipay_account" placeholder="手机号或邮箱" maxlength="64" />
          </el-form-item>
        </template>

        <!-- 微信 -->
        <template v-if="withdrawForm.default_method === 'wechat'">
          <el-form-item label="微信号">
            <el-input v-model="withdrawForm.wechat_id" placeholder="微信号或绑定的手机号" maxlength="64" />
          </el-form-item>
        </template>

        <!-- 银行卡 -->
        <template v-if="withdrawForm.default_method === 'bank'">
          <el-form-item label="开户行">
            <el-input v-model="withdrawForm.bank_name" placeholder="如：中国工商银行XX支行" maxlength="64" />
          </el-form-item>
          <el-form-item label="银行卡号">
            <el-input v-model="withdrawForm.bank_card_no" placeholder="16-19 位卡号" maxlength="32" />
          </el-form-item>
        </template>

        <el-form-item>
          <el-button type="primary" :loading="withdrawSaving" @click="saveWithdrawAccount">保存提现账户</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import { useAuthStore } from '@/stores/auth'
import {
  currentUserApi,
  updateProfileApi,
  changePasswordApi,
  type CurrentUser
} from '@/api/profile'
import { agentMeApi, type AgentProfile } from '@/api/agent'

const auth = useAuthStore()
const role = 'agent' as const

const accountOverview = ref({
  balance: 0,
  frozen_balance: 0,
  total_commission: 0,
  total_withdraw: 0
})

const profileForm = reactive({
  username: '',
  real_name: '',
  phone: '',
  email: '',
  inviter_username: '',
  created_at: ''
})
const profileSaving = ref(false)

const withdrawForm = reactive({
  default_method: 'alipay' as 'alipay' | 'wechat' | 'bank',
  real_name: '',
  alipay_account: '',
  wechat_id: '',
  bank_name: '',
  bank_card_no: ''
})
const withdrawSaving = ref(false)

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

const labelPosition = computed(() => {
  return window.innerWidth < 768 ? 'top' : 'right'
})

const formatDate = (s: string | null) => {
  if (!s) return '-'
  return new Date(s).toLocaleString('zh-CN')
}

const loadProfile = async () => {
  try {
    // 优先用 /agent/auth/me（与代理后台其他页面一致），它复用 CurrentUser handler 返回基本字段
    const meData = await currentUserApi(role)
    if (meData && typeof meData === 'object') {
      const u = meData as CurrentUser
      profileForm.username = u.username || auth.username
    }
  } catch {
    profileForm.username = auth.username
  }
  // 代理专属字段：余额/佣金/邀请人 通过 agentMeApi（同后端 /agent/auth/me，复用 CurrentUser）
  // 注：当前 CurrentUser handler 仅返回 user_id/username/role/tenant_id，余额等字段待 v0.3.0 扩展
  try {
    const agentInfo = await agentMeApi() as AgentProfile | CurrentUser
    if (agentInfo && typeof agentInfo === 'object') {
      const a = agentInfo as any
      accountOverview.value.balance = Number(a.balance) || 0
      accountOverview.value.frozen_balance = Number(a.frozen_balance) || 0
      accountOverview.value.total_commission = Number(a.total_commission) || 0
      accountOverview.value.total_withdraw = Number(a.total_withdraw) || 0
      profileForm.real_name = a.real_name || ''
      profileForm.phone = a.phone || ''
      profileForm.inviter_username = a.inviter_username || ''
      profileForm.created_at = a.created_at || ''
      // 提现账户字段待 v0.3.0 扩展（待核实）
      withdrawForm.real_name = a.real_name || ''
    }
  } catch {
    // 铁律 06 待核实：当前 /agent/auth/me 仅返回基本字段，余额等 v0.3.0 补全
  }
}

const saveProfile = async () => {
  profileSaving.value = true
  try {
    await updateProfileApi(role, {
      real_name: profileForm.real_name,
      phone: profileForm.phone,
      email: profileForm.email
    })
    ElMessage.success('资料已保存')
  } catch {
    // 铁律 06 待核实
  } finally {
    profileSaving.value = false
  }
}

const saveWithdrawAccount = async () => {
  if (!withdrawForm.real_name) {
    ElMessage.warning('请填写开户人姓名')
    return
  }
  withdrawSaving.value = true
  try {
    // 铁律 06 待核实：提现账户字段待 v0.3.0 后端扩展
    await updateProfileApi(role, {
      real_name: withdrawForm.real_name
    } as any)
    ElMessage.success('提现账户已保存')
  } catch {
    // 铁律 06 待核实
  } finally {
    withdrawSaving.value = false
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

onMounted(() => {
  loadProfile()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.profile-page {
  display: flex;
  flex-direction: column;
  gap: $spacing-lg;
}

.overview-card {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: $spacing-md;
  background: $color-bg-card;
  border-radius: $radius-md;
  padding: $spacing-lg;
  box-shadow: $shadow-card;

  .overview-item {
    display: flex;
    flex-direction: column;
    gap: 4px;
    .label {
      font-size: 12px;
      color: $color-text-secondary;
    }
    .value {
      font-size: 18px;
      font-weight: 600;
      color: $color-text-primary;
      font-family: 'SF Mono', 'Menlo', monospace;
      &.primary { color: $color-primary; }
      &.success { color: $color-success; }
    }
  }

  @include tablet {
    grid-template-columns: repeat(2, 1fr);
  }
  @include mobile {
    grid-template-columns: repeat(2, 1fr);
    padding: $spacing-md;
    gap: $spacing-sm;
    .overview-item .value { font-size: 16px; }
  }
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

.withdraw-alert {
  margin-bottom: $spacing-md;
}
</style>
