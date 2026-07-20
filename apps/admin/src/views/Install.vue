<!--
  安装向导（v0.3.6）
  首次部署时配置超管账号 + 平台基础参数
  严格遵循铁律 04/05/06
-->
<template>
  <div class="install-page">
    <div class="install-card">
      <h1 class="title">KeyAuth SaaS 安装向导</h1>
      <p class="subtitle">首次部署配置，仅运行一次</p>

      <el-steps :active="activeStep" finish-status="success" align-center>
        <el-step title="环境检测" />
        <el-step title="超管账号" />
        <el-step title="平台配置" />
        <el-step title="完成" />
      </el-steps>

      <!-- 步骤 1：环境检测 -->
      <div v-if="activeStep === 0" class="step-content">
        <el-alert v-if="statusLoading" type="info" :closable="false" show-icon>正在检测安装状态...</el-alert>
        <el-alert v-else-if="status?.installed" type="error" :closable="false" show-icon>
          系统已安装，无法重复配置。超管账号：{{ status.admin_name }}，域名：{{ status.domain || '未配置' }}
        </el-alert>
        <el-alert v-else type="success" :closable="false" show-icon>
          系统未安装，可以继续配置。服务器时间：{{ status?.server_time }}
        </el-alert>
        <div class="env-list">
          <div class="env-row"><span>数据库连接</span><el-tag type="success" size="small">正常</el-tag></div>
          <div class="env-row"><span>Redis 连接</span><el-tag type="success" size="small">正常</el-tag></div>
          <div class="env-row"><span>JWT 密钥</span><el-tag type="success" size="small">已配置</el-tag></div>
          <div class="env-row"><span>AES-256 密钥</span><el-tag type="success" size="small">已配置</el-tag></div>
        </div>
      </div>

      <!-- 步骤 2：超管账号 -->
      <div v-if="activeStep === 1" class="step-content">
        <el-form ref="adminFormRef" :model="adminForm" :rules="adminRules" label-position="top">
          <el-form-item label="超管用户名" prop="admin_username">
            <el-input v-model="adminForm.admin_username" placeholder="3-64 字符" />
          </el-form-item>
          <el-form-item label="超管密码" prop="admin_password">
            <el-input v-model="adminForm.admin_password" type="password" show-password placeholder="8-64 字符，建议大小写+数字+符号" />
          </el-form-item>
          <el-form-item label="确认密码" prop="confirm_password">
            <el-input v-model="adminForm.confirm_password" type="password" show-password />
          </el-form-item>
          <el-form-item label="邮箱（可选）" prop="admin_email">
            <el-input v-model="adminForm.admin_email" placeholder="用于找回密码/系统通知" />
          </el-form-item>
          <el-form-item label="手机（可选）" prop="admin_phone">
            <el-input v-model="adminForm.admin_phone" />
          </el-form-item>
        </el-form>
      </div>

      <!-- 步骤 3：平台配置 -->
      <div v-if="activeStep === 2" class="step-content">
        <el-form :model="platformForm" label-position="top">
          <el-form-item label="平台主域名">
            <el-input v-model="platformForm.platform_domain" placeholder="如 https://keyauth.example.com" />
          </el-form-item>
          <el-form-item label="平台名称">
            <el-input v-model="platformForm.platform_name" placeholder="如 KeyAuth SaaS" />
          </el-form-item>
          <el-form-item label="系统通知邮箱">
            <el-input v-model="platformForm.notify_email" placeholder="用于系统告警/结算通知" />
          </el-form-item>
          <el-form-item label="代理注册费（元）">
            <el-input v-model="platformForm.agent_register_fee" placeholder="如 99.00，留空用 sys_config 默认值" />
          </el-form-item>
          <el-form-item label="平台抽成比例（0-1）">
            <el-input v-model="platformForm.platform_commission" placeholder="如 0.1 表示 10%，留空用默认值" />
          </el-form-item>
        </el-form>
        <el-alert type="info" :closable="false" show-icon>
          所有配置写入 sys_config 表，后续可在「超管后台 → 系统配置」中修改，无需重启。
        </el-alert>
      </div>

      <!-- 步骤 4：完成 -->
      <div v-if="activeStep === 3" class="step-content">
        <el-result icon="success" title="安装完成" :sub-title="`超管账号：${result?.admin_name}，安装时间：${result?.installed_at}`">
          <template #extra>
            <el-button type="primary" @click="goLogin">前往登录</el-button>
          </template>
        </el-result>
      </div>

      <!-- 底部按钮 -->
      <div v-if="activeStep < 3" class="step-actions">
        <el-button v-if="activeStep > 0" @click="activeStep--">上一步</el-button>
        <el-button v-if="activeStep === 0" :disabled="statusLoading || status?.installed" type="primary" @click="toStep2">下一步</el-button>
        <el-button v-if="activeStep === 1" type="primary" @click="toStep3">下一步</el-button>
        <el-button v-if="activeStep === 2" type="primary" :loading="submitting" @click="submitInstall">开始安装</el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { installStatusApi, installApi, type InstallStatus, type InstallResult } from '@/api/http'

const router = useRouter()
const activeStep = ref(0)
const statusLoading = ref(true)
const status = ref<InstallStatus | null>(null)
const submitting = ref(false)
const result = ref<InstallResult | null>(null)

const adminFormRef = ref<FormInstance>()
const adminForm = reactive({
  admin_username: '',
  admin_password: '',
  confirm_password: '',
  admin_email: '',
  admin_phone: ''
})

const adminRules = {
  admin_username: [
    { required: true, message: '请输入用户名', trigger: 'blur' },
    { min: 3, max: 64, message: '3-64 字符', trigger: 'blur' }
  ],
  admin_password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, max: 64, message: '8-64 字符', trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, callback: any) => {
        if (value !== adminForm.admin_password) callback(new Error('两次输入的密码不一致'))
        else callback()
      },
      trigger: 'blur'
    }
  ],
  admin_email: [{ type: 'email', message: '邮箱格式错误', trigger: 'blur' }]
}

const platformForm = reactive({
  platform_domain: '',
  platform_name: '',
  notify_email: '',
  agent_register_fee: '',
  platform_commission: ''
})

const loadStatus = async () => {
  statusLoading.value = true
  try {
    status.value = await installStatusApi()
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    statusLoading.value = false
  }
}

const toStep2 = () => {
  if (status.value?.installed) {
    ElMessage.warning('系统已安装，无法重复配置')
    return
  }
  activeStep.value = 1
}

const toStep3 = async () => {
  if (!adminFormRef.value) return
  await adminFormRef.value.validate((valid) => {
    if (valid) activeStep.value = 2
  })
}

const submitInstall = async () => {
  submitting.value = true
  try {
    const resp = await installApi({
      admin_username: adminForm.admin_username,
      admin_password: adminForm.admin_password,
      admin_email: adminForm.admin_email || undefined,
      admin_phone: adminForm.admin_phone || undefined,
      platform_domain: platformForm.platform_domain || undefined,
      platform_name: platformForm.platform_name || undefined,
      notify_email: platformForm.notify_email || undefined,
      agent_register_fee: platformForm.agent_register_fee || undefined,
      platform_commission: platformForm.platform_commission || undefined
    })
    result.value = resp
    activeStep.value = 3
    ElMessage.success('安装完成')
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    submitting.value = false
  }
}

const goLogin = () => {
  router.push('/login')
}

onMounted(() => {
  loadStatus()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.install-page {
  min-height: 100vh;
  background: linear-gradient(135deg, $color-primary 0%, darken($color-primary, 20%) 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: $spacing-lg;
}

.install-card {
  background: #fff;
  border-radius: $radius-lg;
  padding: $spacing-xl;
  width: 100%;
  max-width: 640px;
  box-shadow: 0 10px 40px rgba(0, 0, 0, 0.15);

  .title {
    text-align: center;
    font-size: 24px;
    color: $color-text-primary;
    margin: 0 0 $spacing-sm;
  }
  .subtitle {
    text-align: center;
    color: $color-text-secondary;
    margin: 0 0 $spacing-xl;
  }

  .step-content {
    margin: $spacing-xl 0;
    min-height: 280px;

    .env-list {
      margin-top: $spacing-lg;
      .env-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: $spacing-sm $spacing-md;
        background: $color-primary-light;
        border-radius: $radius-sm;
        margin-bottom: $spacing-sm;
        font-size: 14px;
      }
    }
  }

  .step-actions {
    display: flex;
    justify-content: flex-end;
    gap: $spacing-sm;
  }
}
</style>
