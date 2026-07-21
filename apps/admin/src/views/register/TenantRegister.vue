<!--
  开发者注册页 - 响应式
-->
<template>
  <div class="register-container">
    <div class="register-box">
      <div class="register-header">
        <img src="@/assets/logo.svg" alt="logo" />
        <h1>开发者注册</h1>
        <p>免费试用，5 分钟接入卡密验证</p>
      </div>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-position="top"
        @submit.prevent="handleRegister"
      >
        <el-form-item label="账号" prop="username">
          <el-input v-model="form.username" placeholder="3-32 位字母数字下划线" :prefix-icon="User" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="至少 8 位" :prefix-icon="Lock" />
        </el-form-item>
        <el-form-item label="确认密码" prop="confirmPassword">
          <el-input v-model="form.confirmPassword" type="password" show-password placeholder="请再次输入密码" :prefix-icon="Lock" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="用于接收通知" :prefix-icon="Message" />
        </el-form-item>
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="form.phone" placeholder="可选" :prefix-icon="Iphone" />
        </el-form-item>
        <el-form-item label="公司 / 团队名称" prop="company">
          <el-input v-model="form.company" placeholder="可选" />
        </el-form-item>
        <el-form-item label="邀请码" prop="invite_code">
          <el-input v-model="form.invite_code" placeholder="可选" />
        </el-form-item>

        <el-form-item>
          <el-checkbox v-model="agree">
            我已阅读并同意 <el-link type="primary" :underline="false">《服务协议》</el-link>
          </el-checkbox>
        </el-form-item>

        <el-form-item>
          <el-button type="primary" :loading="loading" class="register-btn" @click="handleRegister">立即注册</el-button>
        </el-form-item>

        <div class="form-footer">
          已有账号？<el-link type="primary" :underline="false" @click="goLogin">前往登录</el-link>
        </div>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { User, Lock, Message, Iphone } from '@element-plus/icons-vue'
import { tenantRegisterApi } from '@/api/auth'

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)
const agree = ref(false)

const form = reactive({
  username: '',
  password: '',
  confirmPassword: '',
  email: '',
  phone: '',
  company: '',
  invite_code: ''
})

const rules = {
  username: [
    { required: true, message: '请输入账号', trigger: 'blur' },
    { pattern: /^[a-zA-Z0-9_]{3,32}$/, message: '3-32 位字母数字下划线', trigger: 'blur' }
  ],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 位', trigger: 'blur' }
  ],
  confirmPassword: [
    { required: true, message: '请再次输入密码', trigger: 'blur' },
    {
      validator: (_rule: any, value: string, callback: any) => {
        if (value !== form.password) callback(new Error('两次输入的密码不一致'))
        else callback()
      },
      trigger: 'blur'
    }
  ],
  email: [
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' }
  ],
  phone: [
    { pattern: /^1\d{10}$/, message: '手机号格式不正确', trigger: 'blur' }
  ]
}

const handleRegister = async () => {
  if (!formRef.value) return
  // v0.9.0 修复：原 callback 风格 `await formRef.value.validate(async (valid) => {...})`
  // 中 await 立即 resolve，callback 内 async 操作不被等待，finally 立即执行
  // 表现为"点击注册按钮没反应"——按钮 loading 一闪而过，注册请求未真正发出
  // 改为 Promise 风格：validate 失败抛异常被 catch 捕获后直接 return
  try {
    await formRef.value.validate()
  } catch {
    return // 校验失败
  }
  if (!agree.value) {
    ElMessage.warning('请阅读并同意服务协议')
    return
  }
  loading.value = true
  try {
    await tenantRegisterApi({
      username: form.username,
      password: form.password,
      email: form.email,
      phone: form.phone,
      company: form.company,
      invite_code: form.invite_code
    })
    ElMessage.success('注册成功，请登录')
    router.push('/login')
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const goLogin = () => router.push('/login')
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.register-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #f0f5ff 0%, #fff 100%);
  padding: $spacing-md;
}
.register-box {
  width: 100%;
  max-width: 460px;
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
  }
}
.register-header {
  text-align: center;
  margin-bottom: $spacing-lg;
  img { width: 56px; height: 56px; }
  h1 {
    margin: 12px 0 4px;
    font-size: 22px;
    color: $color-primary;
  }
  p { margin: 0; color: $color-text-secondary; font-size: 13px; }
}
.register-btn { width: 100%; }
.form-footer {
  text-align: center;
  font-size: 13px;
  color: $color-text-secondary;
}
</style>
