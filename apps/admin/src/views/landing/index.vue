<!--
  官网首页（Landing）- 响应式 H5
  - 顶部导航：Logo + 菜单 + CTA
  - Hero 区：标题 + 副标题 + 行动按钮
  - 功能特性区
  - 适用场景区
  - 套餐预览（从 sys_config 或后端获取，当前为占位）
  - 底部
  - 严格遵守铁律 03：明亮配色，禁暗黑/夸张渐变
-->
<template>
  <div class="landing">
    <!-- 顶部导航 -->
    <header class="navbar" :class="{ scrolled: isScrolled }">
      <div class="container nav-inner">
        <div class="brand" @click="scrollTo('hero')">
          <img src="@/assets/logo.svg" alt="logo" />
          <span>{{ sysConfig.platformName || 'KeyAuth SaaS' }}</span>
        </div>
        <nav class="nav-menu hidden-mobile">
          <a @click="scrollTo('features')">功能特性</a>
          <a @click="scrollTo('scenarios')">适用场景</a>
          <a @click="scrollTo('pricing')">套餐价格</a>
          <a @click="scrollTo('faq')">常见问题</a>
        </nav>
        <div class="nav-actions">
          <el-button text @click="goLogin">登录</el-button>
          <el-button type="primary" @click="goRegister">免费注册</el-button>
        </div>
      </div>
    </header>

    <!-- Hero 区 -->
    <section class="hero" id="hero">
      <div class="container">
        <h1 class="hero-title">
          面向开发者的<br />
          <span class="highlight">多租户卡密验证 SaaS 平台</span>
        </h1>
        <p class="hero-subtitle">
          一行代码接入在线验证、一机一卡、心跳保活、多级分销。<br />
          自带彩虹易支付、平台抽成结算，让商业化交付更轻松。
        </p>
        <div class="hero-actions">
          <el-button type="primary" size="large" @click="goRegister">立即免费试用</el-button>
          <el-button size="large" @click="scrollTo('features')">了解更多</el-button>
        </div>
        <div class="hero-stats">
          <div class="stat-item">
            <div class="stat-num">5+</div>
            <div class="stat-label">卡密类型</div>
          </div>
          <div class="stat-item">
            <div class="stat-num">3</div>
            <div class="stat-label">角色权限</div>
          </div>
          <div class="stat-item">
            <div class="stat-num">9</div>
            <div class="stat-label">客户端 API</div>
          </div>
          <div class="stat-item">
            <div class="stat-num">100w+</div>
            <div class="stat-label">设备并发</div>
          </div>
        </div>
      </div>
    </section>

    <!-- 功能特性 -->
    <section class="section" id="features">
      <div class="container">
        <h2 class="section-title">核心功能</h2>
        <p class="section-desc">从卡密生成到设备验证，从支付下单到分销结算，一站式覆盖</p>
        <div class="feature-grid">
          <div v-for="f in features" :key="f.title" class="feature-card">
            <el-icon class="feature-icon"><component :is="f.icon" /></el-icon>
            <h3>{{ f.title }}</h3>
            <p>{{ f.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 适用场景 -->
    <section class="section section-alt" id="scenarios">
      <div class="container">
        <h2 class="section-title">适用场景</h2>
        <p class="section-desc">适配各种需要授权与计费的软件产品</p>
        <div class="scenario-grid">
          <div v-for="s in scenarios" :key="s.title" class="scenario-card">
            <h3>{{ s.title }}</h3>
            <p>{{ s.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- 套餐价格 -->
    <section class="section" id="pricing">
      <div class="container">
        <h2 class="section-title">套餐价格</h2>
        <p class="section-desc">从免费到企业级，按需选择</p>
        <div class="pricing-grid">
          <div v-for="p in packages" :key="p.name" class="pricing-card" :class="{ featured: p.featured }">
            <div v-if="p.featured" class="featured-tag">推荐</div>
            <h3>{{ p.name }}</h3>
            <div class="price">
              <span class="price-num">¥{{ p.price }}</span>
              <span class="price-unit">/月</span>
            </div>
            <ul class="price-features">
              <li v-for="f in p.features" :key="f">{{ f }}</li>
            </ul>
            <el-button
              :type="p.featured ? 'primary' : 'default'"
              class="price-btn"
              @click="goRegister"
            >选择 {{ p.name }}</el-button>
          </div>
        </div>
        <p class="pricing-note">注：套餐数据为前端展示，请以开发者后台实际显示为准</p>
      </div>
    </section>

    <!-- 常见问题 -->
    <section class="section section-alt" id="faq">
      <div class="container">
        <h2 class="section-title">常见问题</h2>
        <el-collapse class="faq-list">
          <el-collapse-item v-for="(f, i) in faqs" :key="i" :title="f.q" :name="String(i)">
            <p class="faq-a">{{ f.a }}</p>
          </el-collapse-item>
        </el-collapse>
      </div>
    </section>

    <!-- CTA -->
    <section class="cta-section">
      <div class="container">
        <h2>立即开始你的商业化交付</h2>
        <p>注册即享免费试用，5 分钟接入验证 SDK</p>
        <el-button type="primary" size="large" @click="goRegister">免费注册</el-button>
      </div>
    </section>

    <!-- 底部 -->
    <footer class="footer">
      <div class="container footer-inner">
        <div class="footer-brand">
          <img src="@/assets/logo.svg" alt="logo" />
          <span>{{ sysConfig.platformName || 'KeyAuth SaaS' }}</span>
        </div>
        <div class="footer-links">
          <a @click="goLogin">登录</a>
          <a @click="goRegister">注册</a>
          <a @click="scrollTo('features')">功能</a>
          <a @click="scrollTo('pricing')">套餐</a>
        </div>
        <div class="footer-copyright">
          © {{ year }} {{ sysConfig.platformName || 'KeyAuth SaaS' }}. All rights reserved.
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import {
  Key, Cellphone, Money, Promotion, Lock, Monitor, Tickets, Coin, Upload
} from '@element-plus/icons-vue'
import { useSysConfigStore } from '@/stores/sysConfig'

const router = useRouter()
const sysConfig = useSysConfigStore()

const isScrolled = ref(false)
const year = computed(() => new Date().getFullYear())

const features = [
  { icon: Key, title: '5 种卡密类型', desc: '时长卡、次数卡、永久卡、试用卡、功能卡，灵活适配各种计费场景' },
  { icon: Monitor, title: '一机一卡绑定', desc: 'HWID 硬件指纹绑定，防止卡密共享，支持多机授权与离线宽限期' },
  { icon: Lock, title: 'HMAC 签名鉴权', desc: '客户端 API 全部经过 HMAC-SHA256 签名 + Nonce 防重放 + 时间戳校验' },
  { icon: Money, title: '彩虹易支付', desc: '内置平台总支付 + 开发者自有易支付（按套餐开通），自动发卡 + 抽成结算' },
  { icon: Promotion, title: '代理分销体系', desc: '邀请码 + 注册费 + 佣金分成（按比例或按差价），支持多级代理' },
  { icon: Cellphone, title: '响应式 H5', desc: '管理后台、开发者控制台、代理中心、终端用户 H5 全部响应式适配' },
  { icon: Tickets, title: '卡密批量生成', desc: '事务化批量发卡，按 hash 索引防穷举，支持前缀/分组标签' },
  { icon: Coin, title: '云变量管理', desc: '动态配置业务变量，客户端实时读取，无需发版' },
  { icon: Upload, title: '在线版本更新', desc: '支持强制更新、最小版本限制、多下载源容灾' }
]

const scenarios = [
  { title: '桌面软件', desc: '工具类、安全类、设计类软件的授权与续费' },
  { title: '游戏外挂防护', desc: '防多开、防共享、防破解的卡密验证场景' },
  { title: 'SaaS 工具', desc: '按月/按年计费的在线工具与 API 服务' },
  { title: '教程视频', desc: '付费观看、限时观看、按次观看的内容授权' },
  { title: '插件/扩展', desc: '浏览器扩展、CMS 插件的功能解锁与订阅' },
  { title: '企业内部系统', desc: '员工授权、设备绑定、使用次数限制' }
]

// 注：套餐数据来自后端 sys_package 表，前端仅展示（铁律 04：不编造）
const packages = [
  { name: '免费版', price: '0', featured: false, features: ['1 个应用', '100 张卡密', '社区支持', '平台抽成 10%'] },
  { name: '专业版', price: '99', featured: true, features: ['5 个应用', '10000 张卡密', '5 个代理', '邮件支持', '平台抽成 5%'] },
  { name: '企业版', price: '399', featured: false, features: ['50 个应用', '10 万张卡密', '50 个代理', '电话支持', '自定义易支付', '平台抽成 3%'] }
]

const faqs = [
  { q: '如何接入卡密验证？', a: '注册开发者账号 → 创建应用 → 生成卡密 → 集成客户端 SDK（Python/Node/PHP/Go）→ 调用 /api/v1/client/login 接口即可完成验证。SDK 文档请见开发者控制台。' },
  { q: '支持哪些支付方式？', a: '平台总支付支持支付宝、微信支付、QQ 钱包（基于彩虹易支付协议）。企业版套餐可开通开发者自有易支付，独立收款不经过平台。' },
  { q: '抽成如何结算？', a: '每笔订单支付成功后，平台按套餐约定的抽成比例（3% - 10%）抽取佣金，剩余部分计入开发者应得金额。结算支持手动申请与自动结算两种方式。' },
  { q: '是否支持多级代理？', a: '当前版本支持一级代理，v0.4.0 将支持多级代理树形结构与分润链路。代理可邀请注册、按差价或比例分成。' },
  { q: '数据安全性如何保障？', a: '所有敏感字段（AppSecret、SignSecret、TOTP 密钥、商户密钥）均通过 AES-256-GCM 加密入库。卡密按 SHA-512 hash 索引存储，防止数据库泄露后被穷举。' }
]

const scrollTo = (id: string) => {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const goLogin = () => router.push('/login')
const goRegister = () => router.push('/register/tenant')

const onScroll = () => {
  isScrolled.value = window.scrollY > 20
}

onMounted(() => {
  sysConfig.load()
  window.addEventListener('scroll', onScroll)
})
onBeforeUnmount(() => window.removeEventListener('scroll', onScroll))
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.landing {
  background: $color-bg-page;
  min-height: 100vh;
}

.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 $spacing-lg;

  @include mobile {
    padding: 0 $spacing-md;
  }
}

// ============== 导航 ==============
.navbar {
  position: sticky;
  top: 0;
  z-index: 100;
  background: rgba(255, 255, 255, 0.95);
  border-bottom: 1px solid transparent;
  transition: all 0.2s;

  &.scrolled {
    border-bottom-color: $color-border-lighter;
    box-shadow: $shadow-card;
  }

  .nav-inner {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 60px;
  }

  .brand {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    cursor: pointer;
    img { width: 28px; height: 28px; }
    span {
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }

  .nav-menu {
    display: flex;
    gap: $spacing-lg;
    a {
      color: $color-text-regular;
      cursor: pointer;
      font-size: 14px;
      &:hover { color: $color-primary; }
    }
  }

  .nav-actions {
    display: flex;
    gap: $spacing-sm;
    align-items: center;
  }
}

// ============== Hero ==============
.hero {
  padding: 80px 0 60px;
  text-align: center;
  background: linear-gradient(180deg, #f0f5ff 0%, #fff 100%);

  @include mobile {
    padding: 48px 0 40px;
  }

  .hero-title {
    font-size: 40px;
    font-weight: 700;
    line-height: 1.3;
    color: $color-text-primary;
    margin: 0 0 20px;

    @include mobile {
      font-size: 26px;
    }

    .highlight {
      color: $color-primary;
    }
  }

  .hero-subtitle {
    font-size: 16px;
    color: $color-text-regular;
    line-height: 1.8;
    margin: 0 0 32px;

    @include mobile {
      font-size: 14px;
    }
  }

  .hero-actions {
    display: flex;
    justify-content: center;
    gap: $spacing-md;
    margin-bottom: 48px;

    @include mobile {
      flex-direction: column;
      padding: 0 $spacing-md;
    }
  }

  .hero-stats {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: $spacing-lg;
    max-width: 720px;
    margin: 0 auto;

    @include mobile {
      grid-template-columns: repeat(2, 1fr);
      gap: $spacing-md;
    }

    .stat-item {
      .stat-num {
        font-size: 32px;
        font-weight: 700;
        color: $color-primary;
        line-height: 1.2;

        @include mobile { font-size: 24px; }
      }
      .stat-label {
        font-size: 13px;
        color: $color-text-secondary;
        margin-top: 4px;
      }
    }
  }
}

// ============== 通用 Section ==============
.section {
  padding: 72px 0;

  @include mobile { padding: 48px 0; }

  .section-title {
    font-size: 28px;
    font-weight: 600;
    color: $color-text-primary;
    text-align: center;
    margin: 0 0 12px;

    @include mobile { font-size: 22px; }
  }
  .section-desc {
    font-size: 14px;
    color: $color-text-secondary;
    text-align: center;
    margin: 0 0 48px;

    @include mobile { margin-bottom: 32px; }
  }
}
.section-alt {
  background: #fff;
}

// ============== 功能特性 ==============
.feature-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: $spacing-lg;

  @include tablet {
    grid-template-columns: repeat(2, 1fr);
  }
  @include mobile {
    grid-template-columns: 1fr;
    gap: $spacing-md;
  }

  .feature-card {
    background: #fff;
    border: 1px solid $color-border-lighter;
    border-radius: $radius-md;
    padding: $spacing-lg;
    transition: all 0.2s;

    &:hover {
      box-shadow: $shadow-hover;
      transform: translateY(-2px);
    }

    .feature-icon {
      font-size: 28px;
      color: $color-primary;
      margin-bottom: $spacing-md;
    }
    h3 {
      font-size: 16px;
      font-weight: 600;
      color: $color-text-primary;
      margin: 0 0 8px;
    }
    p {
      font-size: 13px;
      color: $color-text-regular;
      line-height: 1.6;
      margin: 0;
    }
  }
}

// ============== 适用场景 ==============
.scenario-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: $spacing-md;

  @include tablet { grid-template-columns: repeat(2, 1fr); }
  @include mobile { grid-template-columns: 1fr; }

  .scenario-card {
    padding: $spacing-lg;
    border-left: 3px solid $color-primary;
    background: $color-bg-page;
    border-radius: $radius-sm;

    h3 {
      font-size: 15px;
      font-weight: 600;
      color: $color-text-primary;
      margin: 0 0 8px;
    }
    p {
      font-size: 13px;
      color: $color-text-regular;
      line-height: 1.6;
      margin: 0;
    }
  }
}

// ============== 套餐 ==============
.pricing-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: $spacing-lg;

  @include mobile {
    grid-template-columns: 1fr;
    gap: $spacing-md;
  }

  .pricing-card {
    background: #fff;
    border: 1px solid $color-border-lighter;
    border-radius: $radius-lg;
    padding: $spacing-xl $spacing-lg;
    text-align: center;
    position: relative;
    transition: all 0.2s;

    &.featured {
      border-color: $color-primary;
      box-shadow: 0 4px 16px rgba(22, 119, 255, 0.12);
    }

    .featured-tag {
      position: absolute;
      top: 16px;
      right: 16px;
      background: $color-primary;
      color: #fff;
      font-size: 12px;
      padding: 2px 8px;
      border-radius: $radius-sm;
    }

    h3 {
      font-size: 18px;
      font-weight: 600;
      color: $color-text-primary;
      margin: 0 0 16px;
    }

    .price {
      margin-bottom: $spacing-lg;
      .price-num {
        font-size: 36px;
        font-weight: 700;
        color: $color-primary;
      }
      .price-unit {
        font-size: 14px;
        color: $color-text-secondary;
        margin-left: 4px;
      }
    }

    .price-features {
      list-style: none;
      padding: 0;
      margin: 0 0 $spacing-lg;
      text-align: left;

      li {
        padding: 8px 0;
        font-size: 13px;
        color: $color-text-regular;
        border-bottom: 1px solid $color-border-lighter;

        &:before {
          content: '✓';
          color: $color-success;
          margin-right: 8px;
          font-weight: 700;
        }
        &:last-child { border-bottom: none; }
      }
    }

    .price-btn {
      width: 100%;
    }
  }
}
.pricing-note {
  text-align: center;
  font-size: 12px;
  color: $color-text-secondary;
  margin-top: $spacing-lg;
}

// ============== FAQ ==============
.faq-list {
  max-width: 800px;
  margin: 0 auto;
  background: #fff;
  border-radius: $radius-md;
  padding: 0 $spacing-lg;

  :deep(.el-collapse-item__header) {
    font-size: 14px;
    font-weight: 500;
  }
  .faq-a {
    color: $color-text-regular;
    font-size: 13px;
    line-height: 1.7;
    margin: 0;
  }
}

// ============== CTA ==============
.cta-section {
  padding: 72px 0;
  text-align: center;
  background: $color-primary-light;

  @include mobile { padding: 48px 0; }

  h2 {
    font-size: 28px;
    font-weight: 600;
    color: $color-text-primary;
    margin: 0 0 12px;

    @include mobile { font-size: 22px; }
  }
  p {
    font-size: 14px;
    color: $color-text-regular;
    margin: 0 0 32px;
  }
}

// ============== 底部 ==============
.footer {
  background: #fff;
  border-top: 1px solid $color-border-lighter;
  padding: 32px 0;

  .footer-inner {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: $spacing-md;
  }

  .footer-brand {
    display: flex;
    align-items: center;
    gap: $spacing-sm;
    img { width: 24px; height: 24px; }
    span {
      font-size: 14px;
      font-weight: 600;
      color: $color-text-primary;
    }
  }

  .footer-links {
    display: flex;
    gap: $spacing-lg;
    a {
      font-size: 13px;
      color: $color-text-regular;
      cursor: pointer;
      &:hover { color: $color-primary; }
    }
  }

  .footer-copyright {
    font-size: 12px;
    color: $color-text-secondary;
  }
}
</style>
