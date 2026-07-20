<!--
  H5 终端用户登录页（v0.4.0 收尾项 C）
  - AppKey + 用户名 + 密码
  - 登录成功后跳 /h5/profile（或 redirect 参数指定的地址）
-->
<template>
  <div class="h5-login">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">登录</span>
    </div>

    <div class="form-card">
      <p class="section-label">应用 AppKey</p>
      <el-input v-model="form.appKey" placeholder="请输入开发者提供的 AppKey" clearable />

      <p class="section-label">用户名</p>
      <el-input v-model="form.username" placeholder="请输入用户名" clearable />

      <p class="section-label">密码</p>
      <el-input v-model="form.password" type="password" placeholder="请输入密码" show-password @keyup.enter="submit" />

      <div class="submit-row">
        <el-button type="primary" size="large" :loading="submitting" @click="submit">登录</el-button>
      </div>

      <div class="links">
        <router-link to="/h5/register" class="link">注册账号</router-link>
        <router-link to="/h5/reset-password" class="link">忘记密码</router-link>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { endUserLoginApi } from '@/api/enduser'
import { useEndUserStore } from '@/stores/enduser'

const route = useRoute()
const router = useRouter()
const endUserStore = useEndUserStore()

const form = reactive({
  appKey: '',
  username: '',
  password: ''
})
const submitting = ref(false)

onMounted(() => {
  endUserStore.restore()
  if (endUserStore.appKey) {
    form.appKey = endUserStore.appKey
  }
})

const submit = async () => {
  if (!form.appKey || !form.username || !form.password) {
    ElMessage.warning('请填写完整信息')
    return
  }
  submitting.value = true
  try {
    const resp = await endUserLoginApi({
      app_key: form.appKey,
      username: form.username,
      password: form.password
    })
    endUserStore.setAppKey(form.appKey)
    endUserStore.setLogin({
      access_token: resp.access_token,
      refresh_token: resp.refresh_token,
      // P0 高危 10：后端返回 expires_in（相对秒数），store 内部存绝对时间戳（ms）
      expires_at: Date.now() + resp.expires_in * 1000,
      user: resp.user
    })
    ElMessage.success('登录成功')
    const redirect = route.query.redirect
    if (typeof redirect === 'string' && redirect.startsWith('/h5/')) {
      router.replace(redirect)
    } else {
      router.replace('/h5/profile')
    }
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    submitting.value = false
  }
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5')
  }
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-login {
  max-width: 640px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  align-items: center;
  padding: $spacing-sm 0 $spacing-md;
  position: relative;

  .back-btn {
    padding: 0 $spacing-sm;
  }
  .title {
    position: absolute;
    left: 50%;
    transform: translateX(-50%);
    font-size: 16px;
    font-weight: 600;
    color: $color-text-primary;
  }
}

.section-label {
  font-size: 13px;
  color: $color-text-secondary;
  margin: $spacing-md 0 $spacing-sm;
}

.form-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;
}

.submit-row {
  margin-top: $spacing-lg;
  :deep(.el-button) { width: 100%; }
}

.links {
  display: flex;
  justify-content: space-between;
  margin-top: $spacing-md;

  .link {
    font-size: 13px;
    color: $color-primary;
    text-decoration: none;
  }
}
</style>
