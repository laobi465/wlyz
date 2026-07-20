<!--
  H5 帮助中心（v0.4.x 残留项 3：U-13）
  - 用 el-collapse 折叠分类列表展示 FAQ
  - v0.4.x 简化实现：FAQ 硬编码在前端（4 分类 × 3-5 条）
  - v0.5.x 可改为后端 API（如 /public/help/faq）
-->
<template>
  <div class="h5-help">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">帮助中心</span>
    </div>

    <div class="search-card">
      <el-input v-model="keyword" placeholder="搜索常见问题" clearable>
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>
    </div>

    <div class="faq-card">
      <el-collapse v-model="activeNames">
        <template v-for="cat in filteredCategories" :key="cat.name">
          <el-collapse-item :title="`${cat.name}（${cat.items.length}）`" :name="cat.name">
            <div
              v-for="(item, idx) in cat.items"
              :key="idx"
              class="faq-item"
            >
              <div class="faq-q">Q：{{ item.q }}</div>
              <div class="faq-a">A：{{ item.a }}</div>
            </div>
          </el-collapse-item>
        </template>
      </el-collapse>

      <div v-if="filteredCategories.length === 0" class="empty-faq">
        <el-empty :description="`未找到包含「${keyword}」的问题`" :image-size="80" />
      </div>
    </div>

    <div class="contact-tip">
      <p>未找到答案？</p>
      <el-button type="primary" plain @click="goContact">联系客服</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Search } from '@element-plus/icons-vue'

const router = useRouter()

interface FaqItem { q: string; a: string }
interface FaqCategory { name: string; items: FaqItem[] }

// v0.4.x 简化实现：FAQ 硬编码在前端
// v0.5.x 可改为后端 API（如 /public/help/faq）
const categories: FaqCategory[] = [
  {
    name: '账号相关',
    items: [
      { q: '如何注册终端用户账号？', a: '在 H5 首页点击「登录」→「注册账号」，输入开发者提供的 AppKey + 用户名 + 密码即可完成注册。' },
      { q: '忘记密码怎么办？', a: '在登录页点击「忘记密码」，通过绑定的邮箱或手机号接收验证码后重置密码。' },
      { q: '如何修改个人信息？', a: '进入「我的」→「编辑资料」，可修改昵称、头像、邮箱、手机号等。' },
      { q: '账号被禁用怎么办？', a: '账号被禁用通常由于违规操作，请联系平台客服申诉解禁。' }
    ]
  },
  {
    name: '卡密相关',
    items: [
      { q: '如何购买卡密？', a: '在 H5 首页输入 AppKey 后选择套餐，按提示完成支付即可获得卡密。' },
      { q: '如何绑定已购买的卡密？', a: '进入「我的」→「我的卡密」→「绑定」，输入卡密即可绑定到当前账号。' },
      { q: '一张卡密可以绑几个账号？', a: '一张卡密只能绑定一个终端用户账号，绑定时如已被他人绑定将提示错误。' },
      { q: '卡密过期了还能用吗？', a: '卡密过期后无法继续使用，请购买新的卡密。永久卡不会过期。' },
      { q: '如何解绑卡密？', a: '在「我的卡密」列表中找到对应卡密，点击「解绑」即可解除绑定关系。' }
    ]
  },
  {
    name: '订单与支付',
    items: [
      { q: '支持哪些支付方式？', a: '当前支持支付宝、微信支付、QQ 钱包三种支付方式（由开发者配置的易支付通道决定）。' },
      { q: '支付成功但没收到卡密？', a: '请先在「我的订单」中查看订单状态。如已支付但卡密未显示，请等待 1 分钟后刷新；仍未显示请联系客服。' },
      { q: '订单长时间未支付会怎样？', a: '订单创建后 30 分钟内未支付将自动关闭，关闭后无法继续支付，请重新下单。' },
      { q: '可以申请退款吗？', a: '卡密类虚拟商品一经售出原则上不支持退款；如遇卡密问题请联系平台客服处理。' }
    ]
  },
  {
    name: '会话与安全',
    items: [
      { q: '如何踢出其他设备的登录？', a: '进入「我的」→「会话管理」，可查看当前所有活跃会话，点击「踢下线」即可让对应设备退出。' },
      { q: '修改密码后其它设备会自动退出吗？', a: '不会自动退出。如需让其它设备下线，请前往「会话管理」主动踢下线。' },
      { q: '账号被盗怎么办？', a: '请立即修改密码，并前往「会话管理」踢出未知设备。如无法登录请联系平台客服。' }
    ]
  }
]

const activeNames = ref<string[]>([categories[0].name])
const keyword = ref('')

const filteredCategories = computed(() => {
  const kw = keyword.value.trim().toLowerCase()
  if (!kw) return categories
  return categories
    .map((cat) => ({
      name: cat.name,
      items: cat.items.filter(
        (it) => it.q.toLowerCase().includes(kw) || it.a.toLowerCase().includes(kw)
      )
    }))
    .filter((cat) => cat.items.length > 0)
})

const goContact = () => router.push('/h5/contact')

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/profile')
  }
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-help {
  max-width: 640px;
  margin: 0 auto;
}

.page-head {
  display: flex;
  align-items: center;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-md;
  background: #fff;
  border-radius: $radius-md;
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

.search-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-md;
}

.faq-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-sm $spacing-md;
  margin-bottom: $spacing-md;

  :deep(.el-collapse-item__header) {
    font-size: 14px;
    font-weight: 600;
    color: $color-text-primary;
  }

  :deep(.el-collapse-item__content) {
    padding-bottom: $spacing-sm;
  }
}

.faq-item {
  padding: $spacing-sm 0;
  border-bottom: 1px solid $color-border-lighter;

  &:last-child { border-bottom: none; }

  .faq-q {
    font-size: 14px;
    font-weight: 600;
    color: $color-text-primary;
    margin-bottom: $spacing-xs;
    line-height: 1.5;
  }
  .faq-a {
    font-size: 13px;
    color: $color-text-regular;
    line-height: 1.6;
    word-break: break-word;
  }
}

.empty-faq {
  padding: $spacing-lg 0;
}

.contact-tip {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  text-align: center;

  p {
    font-size: 13px;
    color: $color-text-secondary;
    margin: 0 0 $spacing-sm;
  }
  :deep(.el-button) {
    width: 100%;
  }
}
</style>
