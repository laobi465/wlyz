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
        <!-- Step 1: 邀请码 + 账号 -->
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
            <el-form-item label="联系方式" prop="phone">
              <el-input v-model="inviteForm.phone" placeholder="QQ/邮箱/手机号（可选）" :prefix-icon="Phone" />
            </el-form-item>
            <el-alert
              v-if="!configLoading"
              :type="config.pay_enabled ? 'warning' : 'error'"
              :closable="false"
              :title="config.pay_enabled
                ? `注册需支付代理注册费 ¥${config.register_fee.toFixed(2)}（从 sys_config agent.register.fee 读取）`
                : '平台支付未启用，无法完成代理注册，请联系平台管理员'"
              style="margin-bottom: 16px;"
            />
            <el-button type="primary" class="next-btn" :loading="loading" :disabled="!config.pay_enabled" @click="submitRegister">
              下一步：前往支付
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
            <p class="pay-tip">请选择支付方式，点击下方按钮跳转到支付页面完成付款</p>
            <div class="pay-methods">
              <el-button
                v-for="m in payMethods"
                :key="m.value"
                :type="selectedMethod === m.value ? 'primary' : 'default'"
                @click="selectedMethod = (m.value as 'alipay' | 'wxpay' | 'qqpay')"
              >
                {{ m.label }}
              </el-button>
            </div>
            <el-alert
              type="info"
              :closable="false"
              title="支付成功后代理账号将自动创建，请勿关闭本页面"
              style="margin-bottom: 16px;"
            />
            <el-button type="primary" class="next-btn" @click="goToPay">前往支付页面</el-button>
            <el-button text @click="pollOrderStatus">我已完成支付，查询状态</el-button>
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
import {
  agentRegisterConfigApi,
  agentRegisterApi,
  agentRegisterOrderStatusApi,
  type AgentRegisterConfig
} from '@/api/agent'

const activeStep = ref(0)
const loading = ref(false)
const configLoading = ref(true)

const inviteFormRef = ref<FormInstance>()
const inviteForm = reactive({
  invite_code: '',
  username: '',
  password: '',
  confirm_password: '',
  phone: ''
})

const inviteRules = {
  invite_code: [{ required: true, message: '请输入邀请码', trigger: 'blur' }],
  username: [
    { required: true, message: '请输入账号', trigger: 'blur' },
    { min: 3, max: 64, message: '账号长度 3-64 字符', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, max: 64, message: '密码 8-64 位', trigger: 'blur' }
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

// 配置（铁律 05：从 sys_config 后台读取，不硬编码）
const config = reactive<AgentRegisterConfig>({
  register_fee: 99.00,
  pay_enabled: true,
  pay_methods: ['alipay', 'wxpay', 'qqpay'],
  order_expire_seconds: 1800
})

// 注册费与支付方式由后端 AgentRegisterConfig 接口统一返回
const registerFee = ref(99.00)
const payMethods = ref([
  { label: '支付宝', value: 'alipay' },
  { label: '微信支付', value: 'wxpay' },
  { label: 'QQ 支付', value: 'qqpay' }
])
const selectedMethod = ref<'alipay' | 'wxpay' | 'qqpay'>('alipay')

// 当前注册订单号（Step 1 提交后回填，Step 2 查询用）
const currentOrderNo = ref('')
// 支付 URL（Step 1 提交后回填，Step 2 跳转用）
const currentPayURL = ref('')

// 支付方式标签映射
const payMethodLabel = (v: string): string => {
  if (v === 'alipay') return '支付宝'
  if (v === 'wxpay') return '微信支付'
  if (v === 'qqpay') return 'QQ 支付'
  return v
}

onMounted(async () => {
  configLoading.value = true
  try {
    const data = await agentRegisterConfigApi()
    config.register_fee = data.register_fee
    config.pay_enabled = data.pay_enabled
    config.pay_methods = data.pay_methods
    config.order_expire_seconds = data.order_expire_seconds
    registerFee.value = data.register_fee
    // 按后端配置重建支付方式列表（铁律 04：不硬编码支付方式）
    payMethods.value = data.pay_methods.map((v: string) => ({
      label: payMethodLabel(v),
      value: v
    }))
    if (payMethods.value.length > 0) {
      selectedMethod.value = payMethods.value[0].value as 'alipay' | 'wxpay' | 'qqpay'
    }
  } catch (e: any) {
    // 铁律 06：不编造数据，使用默认值并提示
    ElMessage.warning('注册配置加载失败，使用默认值')
  } finally {
    configLoading.value = false
  }
})

// Step 1 提交：调 AgentRegister 创建预支付订单 + 返回 pay_url
const submitRegister = async () => {
  if (!inviteFormRef.value) return
  // v0.9.0 修复：原 callback 风格 `await inviteFormRef.value.validate(async (valid) => {...})`
  // 中 await 立即 resolve，callback 内 async 操作不被等待，finally 立即执行
  // 表现为"点击下一步按钮没反应"——按钮 loading 一闪而过，注册请求未真正发出
  // 改为 Promise 风格：validate 失败抛异常被 catch 捕获后直接 return
  try {
    await inviteFormRef.value.validate()
  } catch {
    return // 校验失败
  }
  loading.value = true
  try {
    const resp = await agentRegisterApi({
      invite_code: inviteForm.invite_code,
      username: inviteForm.username,
      password: inviteForm.password,
      phone: inviteForm.phone || undefined,
      pay_type: selectedMethod.value
    })
    currentOrderNo.value = resp.order_no
    currentPayURL.value = resp.pay_url
    registerFee.value = resp.amount
    activeStep.value = 1
    ElMessage.success('订单已创建，请前往支付')
  } catch (e: any) {
    // 错误信息由 http 拦截器统一处理
  } finally {
    loading.value = false
  }
}

// Step 2 跳转到支付页面（新窗口，避免丢失原页面状态）
const goToPay = () => {
  if (!currentPayURL.value) {
    ElMessage.error('支付 URL 为空，请返回上一步重新提交')
    return
  }
  window.open(currentPayURL.value, '_blank')
}

// Step 2 查询订单状态（用户支付完成跳回后点击）
const pollOrderStatus = async () => {
  if (!currentOrderNo.value) {
    ElMessage.error('订单号为空')
    return
  }
  loading.value = true
  try {
    const order = await agentRegisterOrderStatusApi(currentOrderNo.value)
    if (order.pay_status === 'paid' && order.agent_id) {
      ElMessage.success('支付成功，代理账号已创建')
      activeStep.value = 2
    } else if (order.pay_status === 'pending') {
      ElMessage.warning('订单尚未支付，请在新打开的页面完成支付后再次查询')
    } else {
      ElMessage.error('订单状态异常：' + order.pay_status)
    }
  } catch (e: any) {
    // 错误信息由 http 拦截器统一处理
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
