<template>
  <div class="agent-register-container">
    <div class="register-box">
      <div class="register-header">
        <img src="@/assets/logo.svg" alt="logo" />
        <h1>代理注册</h1>
        <p>通过开发者邀请码注册成为代理</p>
      </div>

      <el-steps :active="activeStep" finish-status="success" align-center>
        <el-step title="填写邀请码" />
        <el-step title="支付注册费" />
        <el-step title="完成注册" />
      </el-steps>

      <div class="step-content">
        <!-- Step 1: 邀请码 -->
        <div v-if="activeStep === 0">
          <el-form ref="inviteFormRef" :model="inviteForm" :rules="inviteRules" label-position="top">
            <el-form-item label="开发者邀请码" prop="invite_code">
              <el-input v-model="inviteForm.invite_code" placeholder="请输入开发者提供的代理邀请码" :prefix-icon="Promotion" />
            </el-form-item>
            <el-form-item label="代理账号" prop="username">
              <el-input v-model="inviteForm.username" placeholder="请设置代理登录账号" :prefix-icon="User" />
            </el-form-item>
            <el-form-item label="登录密码" prop="password">
              <el-input v-model="inviteForm.password" type="password" show-password placeholder="至少 8 位" :prefix-icon="Lock" />
            </el-form-item>
            <el-form-item label="确认密码" prop="confirm_password">
              <el-input v-model="inviteForm.confirm_password" type="password" show-password placeholder="再次输入密码" :prefix-icon="Lock" />
            </el-form-item>
            <el-form-item label="联系方式" prop="contact">
              <el-input v-model="inviteForm.contact" placeholder="QQ/邮箱/手机号" :prefix-icon="Phone" />
            </el-form-item>
            <el-alert
              type="warning"
              :closable="false"
              title="注册需支付代理注册费（金额从 sys_config agent.register.fee 读取）"
              style="margin-bottom: 16px;"
            />
            <el-button type="primary" class="next-btn" :loading="loading" @click="verifyInviteCode">
              下一步：支付注册费
            </el-button>
          </el-form>
        </div>

        <!-- Step 2: 支付 -->
        <div v-else-if="activeStep === 1">
          <el-card class="pay-card">
            <div class="pay-amount">
              <span class="label">注册费：</span>
              <span class="value">¥ {{ registerFee.toFixed(2) }}</span>
            </div>
            <el-divider />
            <p class="pay-tip">请选择支付方式完成支付，支付成功后自动进入下一步</p>
            <div class="pay-methods">
              <el-button v-for="m in payMethods" :key="m.value" :type="selectedMethod === m.value ? 'primary' : 'default'" @click="selectedMethod = m.value">
                {{ m.label }}
              </el-button>
            </div>
            <el-button type="primary" class="next-btn" :loading="loading" @click="startPay">前往支付</el-button>
            <el-button text @click="activeStep = 0">返回上一步</el-button>
          </el-card>
        </div>

        <!-- Step 3: 完成 -->
        <div v-else>
          <el-result icon="success" title="注册成功" sub-title="您已成为该开发者的代理，现在可以登录代理中心">
            <template #extra>
              <el-button type="primary" @click="$router.push('/login')">前往登录</el-button>
            </template>
          </el-result>
        </div>
      </div>

      <div class="register-footer">
        <el-link type="primary" :underline="false" @click="$router.push('/login')">返回登录</el-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, type FormInstance } from 'element-plus'
import { Promotion, User, Lock, Phone } from '@element-plus/icons-vue'
import { useSysConfigStore } from '@/stores/sysConfig'

const sysConfig = useSysConfigStore()
const activeStep = ref(0)
const loading = ref(false)

const inviteFormRef = ref<FormInstance>()
const inviteForm = reactive({
  invite_code: '',
  username: '',
  password: '',
  confirm_password: '',
  contact: ''
})

const inviteRules = {
  invite_code: [{ required: true, message: '请输入邀请码', trigger: 'blur' }],
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 位', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, callback: any) => {
        if (value !== inviteForm.password) callback(new Error('两次密码不一致'))
        else callback()
      },
      trigger: 'blur'
    }
  ]
}

// 注册费从 sys_config 读取（铁律 05：禁止硬编码）
const registerFee = ref(99.00)
// 支付方式从 sys_config pay.platform.methods 读取
const payMethods = ref([
  { label: '支付宝', value: 'alipay' },
  { label: '微信支付', value: 'wxpay' },
  { label: 'QQ 支付', value: 'qqpay' }
])
const selectedMethod = ref('alipay')

onMounted(async () => {
  await sysConfig.load()
  // TODO: 从 sys_config 真实读取注册费与支付方式
  // registerFee.value = await getSysConfigValue('agent.register.fee', 99.00)
})

const verifyInviteCode = async () => {
  if (!inviteFormRef.value) return
  await inviteFormRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      // TODO: 调用后端校验邀请码 + 创建预支付订单
      // await request.post('/agent/register/verify', inviteForm)
      ElMessage.error('邀请码校验接口待实现')
      return
    } finally {
      loading.value = false
    }
  })
}

const startPay = async () => {
  loading.value = true
  try {
    // TODO: 调用后端发起支付，跳转到易支付网关
    // const resp = await request.post('/agent/register/pay', { ...inviteForm, method: selectedMethod.value })
    // window.location.href = resp.pay_url
    ElMessage.error('支付接口待实现')
  } finally {
    loading.value = false
  }
}
</script>

<style scoped lang="scss">
.agent-register-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #fff7e6 0%, #fff 100%);
  padding: 24px;
}
.register-box {
  width: 540px;
  max-width: 100%;
  padding: 32px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0,0,0,0.08);
}
.register-header {
  text-align: center;
  margin-bottom: 24px;
  img { width: 56px; height: 56px; }
  h1 { margin: 12px 0 4px; font-size: 22px; color: #d46b08; }
  p { margin: 0; color: #909399; font-size: 13px; }
}
.step-content {
  margin: 32px 0;
  min-height: 280px;
}
.next-btn {
  width: 100%;
  margin-top: 12px;
}
.pay-card {
  text-align: center;
  .pay-amount {
    .label { font-size: 14px; color: #606266; }
    .value { font-size: 32px; font-weight: 600; color: #cf1322; }
  }
  .pay-tip { color: #909399; font-size: 13px; margin: 16px 0; }
  .pay-methods {
    display: flex; gap: 12px; justify-content: center; margin-bottom: 16px;
  }
}
.register-footer {
  text-align: center;
  margin-top: 16px;
}
</style>
