<template>
  <div class="login-container">
    <div class="login-box">
      <div class="login-header">
        <img src="@/assets/logo.svg" alt="logo" />
        <h1>KeyAuth SaaS</h1>
        <p>多租户卡密验证平台</p>
      </div>

      <el-tabs v-model="activeRole" class="login-tabs" stretch>
        <el-tab-pane label="平台管理员" name="admin" />
        <el-tab-pane label="开发者" name="tenant" />
        <el-tab-pane label="代理" name="agent" />
      </el-tabs>

      <el-form ref="formRef" :model="form" :rules="rules" label-position="top" @submit.prevent="handleLogin">
        <el-form-item label="账号" prop="username">
          <el-input v-model="form.username" placeholder="请输入账号" :prefix-icon="User" />
        </el-form-item>
        <el-form-item label="密码" prop="password">
          <el-input v-model="form.password" type="password" show-password placeholder="请输入密码" :prefix-icon="Lock" @keyup.enter="handleLogin" />
        </el-form-item>
        <el-form-item v-if="activeRole === 'tenant'" label="验证码" prop="captcha">
          <div class="captcha-row">
            <el-input v-model="form.captcha" placeholder="请输入验证码" :prefix-icon="Key" />
            <!-- 待实现：图形验证码组件 -->
            <div class="captcha-placeholder">图形验证码占位</div>
          </div>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="loading" class="login-btn" @click="handleLogin">登 录</el-button>
        </el-form-item>
      </el-form>

      <div class="login-footer">
        <el-link v-if="activeRole === 'tenant'" type="primary" :underline="false" @click="goRegister">开发者注册</el-link>
        <el-link v-if="activeRole === 'agent'" type="warning" :underline="false" @click="goAgentRegister">代理注册</el-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, type FormInstance } from 'element-plus'
import { User, Lock, Key } from '@element-plus/icons-vue'
import { useAuthStore } from '@/stores/auth'
import { useSysConfigStore } from '@/stores/sysConfig'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const sysConfig = useSysConfigStore()

const formRef = ref<FormInstance>()
const activeRole = ref<'admin' | 'tenant' | 'agent'>('admin')
const loading = ref(false)

const form = reactive({
  username: '',
  password: '',
  captcha: ''
})

const rules = {
  username: [{ required: true, message: '请输入账号', trigger: 'blur' }],
  password: [
    { required: true, message: '请输入密码', trigger: 'blur' },
    { min: 8, message: '密码至少 8 位', trigger: 'blur' }
  ]
}

const handleLogin = async () => {
  if (!formRef.value) return
  await formRef.value.validate(async (valid) => {
    if (!valid) return
    loading.value = true
    try {
      // TODO: 调用后端登录接口
      // const resp = await request.post(`/${activeRole.value}/login`, form)
      // auth.setAuth({ token: resp.token, role: activeRole.value, userId: resp.user_id, username: resp.username, tenantId: resp.tenant_id })

      // 占位：模拟登录失败（铁律 04：不编造假数据）
      ElMessage.error('登录接口待实现，请等待后端开发完成')
    } catch (e) {
      // 已由 http 拦截器处理
    } finally {
      loading.value = false
    }
  })
}

const goRegister = () => {
  ElMessage.info('开发者注册页面待实现')
}

const goAgentRegister = () => {
  router.push('/agent/register')
}

// 加载平台配置
sysConfig.load()
</script>

<style scoped lang="scss">
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #f0f5ff 0%, #fff 100%);
}
.login-box {
  width: 400px;
  padding: 32px;
  background: #fff;
  border-radius: 12px;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.08);
}
.login-header {
  text-align: center;
  margin-bottom: 24px;
  img { width: 56px; height: 56px; }
  h1 { margin: 12px 0 4px; font-size: 24px; color: #1677ff; }
  p { margin: 0; color: #909399; font-size: 13px; }
}
.login-tabs {
  margin-bottom: 16px;
}
.captcha-row {
  display: flex;
  gap: 8px;
  width: 100%;
  .el-input { flex: 1; }
  .captcha-placeholder {
    width: 120px;
    height: 32px;
    background: #f5f7fa;
    border: 1px dashed #dcdfe6;
    border-radius: 4px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 12px;
    color: #909399;
  }
}
.login-btn {
  width: 100%;
}
.login-footer {
  display: flex;
  justify-content: space-between;
  margin-top: 12px;
}
</style>
