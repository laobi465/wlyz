import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { useAuthStore } from '@/stores/auth'
import { useEndUserStore } from '@/stores/enduser'

NProgress.configure({ showSpinner: false })

// 三套布局
import AdminLayout from '@/layouts/AdminLayout.vue'
import TenantLayout from '@/layouts/TenantLayout.vue'
import AgentLayout from '@/layouts/AgentLayout.vue'
import H5Layout from '@/layouts/H5Layout.vue'

// 懒加载辅助
const lazy = (loader: () => Promise<any>) => loader

// v0.5.0 国际化：每条路由同时提供 title（向后兼容兜底）和 titleKey（i18n 翻译键）
// BasicLayout 优先使用 titleKey 翻译，回退到 title
const routes: RouteRecordRaw[] = [
  // 官网首页
  {
    path: '/',
    name: 'Landing',
    component: lazy(() => import('@/views/landing/index.vue')),
    meta: { title: '首页', titleKey: 'route.landing', public: true }
  },

  // 登录
  {
    path: '/login',
    name: 'Login',
    component: lazy(() => import('@/views/login/index.vue')),
    meta: { title: '登录', titleKey: 'route.login', public: true }
  },

  // 安装向导（v0.3.6，首次部署用）
  {
    path: '/install',
    name: 'Install',
    component: lazy(() => import('@/views/Install.vue')),
    meta: { title: '安装向导', titleKey: 'route.install', public: true }
  },

  // 开发者注册
  {
    path: '/register/tenant',
    name: 'TenantRegister',
    component: lazy(() => import('@/views/register/TenantRegister.vue')),
    meta: { title: '开发者注册', titleKey: 'route.tenantRegister', public: true }
  },

  // ---------- 终端用户 H5 ----------
  {
    path: '/h5',
    component: H5Layout,
    meta: { public: true },
    children: [
      { path: '', name: 'H5Home', component: lazy(() => import('@/views/h5/Home.vue')), meta: { title: '购卡', titleKey: 'route.h5Home', public: true } },
      { path: 'pay/:orderNo', name: 'H5PayResult', component: lazy(() => import('@/views/h5/PayResult.vue')), meta: { title: '支付结果', titleKey: 'route.h5PayResult', public: true } },
      { path: 'query', name: 'H5Query', component: lazy(() => import('@/views/h5/Query.vue')), meta: { title: '卡密查询', titleKey: 'route.h5Query', public: true } },
      { path: 'card/:cardKey', name: 'H5CardDetail', component: lazy(() => import('@/views/h5/CardDetail.vue')), meta: { title: '卡密详情', titleKey: 'route.h5CardDetail', public: true } },
      // v0.4.0 收尾项 C：H5 终端用户中心
      { path: 'login', name: 'H5Login', component: lazy(() => import('@/views/h5/Login.vue')), meta: { title: '登录', titleKey: 'route.h5Login', public: true, guestOnly: true } },
      { path: 'register', name: 'H5Register', component: lazy(() => import('@/views/h5/Register.vue')), meta: { title: '注册', titleKey: 'route.h5Register', public: true, guestOnly: true } },
      { path: 'reset-password', name: 'H5ResetPassword', component: lazy(() => import('@/views/h5/ResetPassword.vue')), meta: { title: '重置密码', titleKey: 'route.h5ResetPassword', public: true, guestOnly: true } },
      { path: 'profile', name: 'H5Profile', component: lazy(() => import('@/views/h5/Profile.vue')), meta: { title: '我的', titleKey: 'route.h5Profile', role: 'enduser' } },
      { path: 'my-cards', name: 'H5MyCards', component: lazy(() => import('@/views/h5/MyCards.vue')), meta: { title: '我的卡密', titleKey: 'route.h5MyCards', role: 'enduser' } },
      { path: 'sessions', name: 'H5Sessions', component: lazy(() => import('@/views/h5/Sessions.vue')), meta: { title: '会话管理', titleKey: 'route.h5Sessions', role: 'enduser' } },
      { path: 'edit-profile', name: 'H5EditProfile', component: lazy(() => import('@/views/h5/EditProfile.vue')), meta: { title: '编辑资料', titleKey: 'route.h5EditProfile', role: 'enduser' } },
      { path: 'change-password', name: 'H5ChangePassword', component: lazy(() => import('@/views/h5/ChangePassword.vue')), meta: { title: '修改密码', titleKey: 'route.h5ChangePassword', role: 'enduser' } },

      // v0.4.x 残留项 1-4：H5 终端用户订单/公告/帮助/客服
      // 订单相关需 enduser 鉴权；公告详情/帮助/客服为公开端点
      { path: 'orders', name: 'H5Orders', component: lazy(() => import('@/views/h5/Orders.vue')), meta: { title: '我的订单', titleKey: 'route.h5Orders', role: 'enduser' } },
      { path: 'orders/:orderNo', name: 'H5OrderDetail', component: lazy(() => import('@/views/h5/OrderDetail.vue')), meta: { title: '订单详情', titleKey: 'route.h5OrderDetail', role: 'enduser' } },
      { path: 'notices/:id', name: 'H5NoticeDetail', component: lazy(() => import('@/views/h5/NoticeDetail.vue')), meta: { title: '平台公告', titleKey: 'route.h5NoticeDetail', public: true } },
      { path: 'help', name: 'H5Help', component: lazy(() => import('@/views/h5/Help.vue')), meta: { title: '帮助中心', titleKey: 'route.h5Help', public: true } },
      { path: 'contact', name: 'H5Contact', component: lazy(() => import('@/views/h5/Contact.vue')), meta: { title: '联系客服', titleKey: 'route.h5Contact', public: true } },

      // v0.4.x 残留项 2（P-06）：代理独立门户 H5 页面（公开访问）
      { path: 'portal/:agentId', name: 'AgentPortal', component: lazy(() => import('@/views/h5/AgentPortal.vue')), meta: { title: '代理门户', titleKey: 'route.agentPortal', public: true } },
      { path: 'portal/:agentId/buy/:cardTypeId', name: 'AgentPortalBuy', component: lazy(() => import('@/views/h5/AgentPortalBuy.vue')), meta: { title: '购卡结算', titleKey: 'route.agentPortalBuy', public: true } }
    ]
  },

  // ---------- 平台管理员后台 ----------
  {
    path: '/admin',
    component: AdminLayout,
    redirect: '/admin/dashboard',
    meta: { role: 'admin', requiresAuth: true },
    children: [
      { path: 'dashboard',   name: 'AdminDashboard',  component: lazy(() => import('@/views/admin/Dashboard.vue')), meta: { title: '概览',     titleKey: 'route.adminDashboard', icon: 'Odometer' } },
      { path: 'tenants',     name: 'AdminTenants',    component: lazy(() => import('@/views/admin/Tenants.vue')),    meta: { title: '开发者管理', titleKey: 'route.adminTenants', icon: 'User' } },
      { path: 'packages',    name: 'AdminPackages',   component: lazy(() => import('@/views/admin/Packages.vue')),   meta: { title: '套餐管理',  titleKey: 'route.adminPackages', icon: 'Box' } },
      { path: 'agents',      name: 'AdminAgents',     component: lazy(() => import('@/views/admin/Agents.vue')),     meta: { title: '代理管理',  titleKey: 'route.adminAgents', icon: 'UserFilled' } },
      { path: 'notices',     name: 'AdminNotices',    component: lazy(() => import('@/views/admin/Notices.vue')),    meta: { title: '平台公告',  titleKey: 'route.adminNotices', icon: 'Bell' } },
      { path: 'pay-config',  name: 'AdminPayConfig',  component: lazy(() => import('@/views/admin/PayConfig.vue')),  meta: { title: '支付配置',  titleKey: 'route.adminPayConfig', icon: 'Money' } },
      { path: 'settlements', name: 'AdminSettlements',component: lazy(() => import('@/views/admin/Settlements.vue')), meta: { title: '结算管理', titleKey: 'route.adminSettlements', icon: 'Wallet' } },
      { path: 'tenant-withdrawal-review', name: 'AdminTenantWithdrawalReview', component: lazy(() => import('@/views/admin/TenantWithdrawalReview.vue')), meta: { title: '开发者提现审核', titleKey: 'route.adminTenantWithdrawalReview', icon: 'CreditCard' } },
      { path: 'sys-config',  name: 'AdminSysConfig',  component: lazy(() => import('@/views/admin/SysConfig.vue')), meta: { title: '系统配置', titleKey: 'route.adminSysConfig', icon: 'Setting' } },
      { path: 'logs',        name: 'AdminLogs',       component: lazy(() => import('@/views/admin/Logs.vue')),       meta: { title: '日志审计',  titleKey: 'route.adminLogs', icon: 'Document' } },
      { path: 'security',    name: 'AdminSecurity',   component: lazy(() => import('@/views/admin/Security.vue')),   meta: { title: '安全防护',  titleKey: 'route.adminSecurity', icon: 'Lock' } },
      { path: 'profile',     name: 'AdminProfile',    component: lazy(() => import('@/views/admin/Profile.vue')), meta: { title: '账号设置',  titleKey: 'route.adminProfile', icon: 'Setting' } }
    ]
  },

  // ---------- 开发者后台 ----------
  {
    path: '/tenant',
    component: TenantLayout,
    redirect: '/tenant/dashboard',
    meta: { role: 'tenant', requiresAuth: true },
    children: [
      { path: 'dashboard',     name: 'TenantDashboard',   component: lazy(() => import('@/views/tenant/Dashboard.vue')), meta: { title: '概览',     titleKey: 'route.tenantDashboard', icon: 'Odometer' } },
      { path: 'apps',          name: 'TenantApps',        component: lazy(() => import('@/views/tenant/Apps.vue')), meta: { title: '应用管理', titleKey: 'route.tenantApps', icon: 'Cellphone' } },
      { path: 'card-types',    name: 'TenantCardTypes',   component: lazy(() => import('@/views/tenant/CardTypes.vue')), meta: { title: '卡类管理', titleKey: 'route.tenantCardTypes', icon: 'Tickets' } },
      { path: 'cards',         name: 'TenantCards',       component: lazy(() => import('@/views/tenant/Cards.vue')), meta: { title: '卡密管理', titleKey: 'route.tenantCards', icon: 'Key' } },
      { path: 'devices',       name: 'TenantDevices',     component: lazy(() => import('@/views/tenant/Devices.vue')),     meta: { title: '设备管理',  titleKey: 'route.tenantDevices', icon: 'Monitor' } },
      { path: 'orders',        name: 'TenantOrders',      component: lazy(() => import('@/views/tenant/Orders.vue')),      meta: { title: '订单管理',  titleKey: 'route.tenantOrders', icon: 'List' } },
      { path: 'settlements',   name: 'TenantSettlements', component: lazy(() => import('@/views/tenant/Settlements.vue')), meta: { title: '结算记录',  titleKey: 'route.tenantSettlements', icon: 'Wallet' } },
      { path: 'withdrawal',    name: 'TenantWithdrawal',  component: lazy(() => import('@/views/tenant/Withdrawal.vue')),  meta: { title: '提现申请',  titleKey: 'route.tenantWithdrawal', icon: 'CreditCard' } },
      { path: 'cloud-vars',    name: 'TenantCloudVars',   component: lazy(() => import('@/views/tenant/CloudVars.vue')),   meta: { title: '云变量',    titleKey: 'route.tenantCloudVars', icon: 'Coin' } },
      { path: 'versions',      name: 'TenantVersions',    component: lazy(() => import('@/views/tenant/Versions.vue')),    meta: { title: '版本管理',  titleKey: 'route.tenantVersions', icon: 'Upload' } },
      { path: 'agents',        name: 'TenantAgents',      component: lazy(() => import('@/views/tenant/Agents.vue')),      meta: { title: '代理管理',  titleKey: 'route.tenantAgents', icon: 'UserFilled' } },
      { path: 'invite-codes',  name: 'TenantInviteCodes', component: lazy(() => import('@/views/tenant/InviteCodes.vue')), meta: { title: '邀请码',    titleKey: 'route.tenantInviteCodes', icon: 'Promotion' } },
      { path: 'recharge-review', name: 'TenantRechargeReview', component: lazy(() => import('@/views/tenant/RechargeReview.vue')), meta: { title: '充值审核', titleKey: 'route.tenantRechargeReview', icon: 'WalletFilled' } },
      { path: 'withdrawal-review', name: 'TenantWithdrawalReview', component: lazy(() => import('@/views/tenant/WithdrawalReview.vue')), meta: { title: '提现审核', titleKey: 'route.tenantWithdrawalReview', icon: 'CreditCard' } },
      { path: 'pay-config',    name: 'TenantPayConfig',   component: lazy(() => import('@/views/tenant/PayConfig.vue')),   meta: { title: '支付配置',  titleKey: 'route.tenantPayConfig', icon: 'Money' } },
      { path: 'notices',       name: 'TenantNotices',     component: lazy(() => import('@/views/tenant/Notices.vue')),     meta: { title: '我的公告',  titleKey: 'route.tenantNotices', icon: 'Bell' } },
      { path: 'profile',       name: 'TenantProfile',     component: lazy(() => import('@/views/tenant/Profile.vue')), meta: { title: '账号设置',  titleKey: 'route.tenantProfile', icon: 'Setting' } }
    ]
  },

  // ---------- 代理后台 ----------
  {
    path: '/agent',
    component: AgentLayout,
    redirect: '/agent/dashboard',
    meta: { role: 'agent', requiresAuth: true },
    children: [
      { path: 'dashboard',   name: 'AgentDashboard',  component: lazy(() => import('@/views/agent/Dashboard.vue')), meta: { title: '概览',     titleKey: 'route.agentDashboard', icon: 'Odometer' } },
      { path: 'register',    name: 'AgentRegister',   component: lazy(() => import('@/views/agent/Register.vue')), meta: { title: '注册代理', titleKey: 'route.agentRegister', icon: 'Plus', public: true } },
      { path: 'cards',       name: 'AgentCards',      component: lazy(() => import('@/views/agent/Cards.vue')), meta: { title: '购卡',     titleKey: 'route.agentCards', icon: 'Key' } },
      { path: 'orders',      name: 'AgentOrders',     component: lazy(() => import('@/views/agent/Orders.vue')), meta: { title: '我的订单',  titleKey: 'route.agentOrders', icon: 'List' } },
      { path: 'balance',     name: 'AgentBalance',    component: lazy(() => import('@/views/agent/Balance.vue')), meta: { title: '余额/提现', titleKey: 'route.agentBalance', icon: 'Wallet' } },
      { path: 'commission',  name: 'AgentCommission', component: lazy(() => import('@/views/agent/Commission.vue')), meta: { title: '佣金记录',  titleKey: 'route.agentCommission', icon: 'GoldMedal' } },
      { path: 'notices',     name: 'AgentNotices',    component: lazy(() => import('@/views/agent/Notices.vue')), meta: { title: '消息通知',  titleKey: 'route.agentNotices', icon: 'Bell' } },
      // v0.4.x 残留项 3（P-10）：代理扫码购卡页面
      { path: 'qrcode',      name: 'AgentQrCode',     component: lazy(() => import('@/views/agent/QrCode.vue')), meta: { title: '扫码购卡',  titleKey: 'route.agentQrCode', icon: 'Iphone' } },
      { path: 'profile',     name: 'AgentProfile',    component: lazy(() => import('@/views/agent/Profile.vue')), meta: { title: '账号设置',  titleKey: 'route.agentProfile', icon: 'Setting' } }
    ]
  },

  // 404
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: lazy(() => import('@/views/error/404.vue')),
    meta: { public: true, title: '页面不存在', titleKey: 'route.notFound' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 })
})

// 全局前置守卫
router.beforeEach((to, _from, next) => {
  NProgress.start()
  // v0.5.0 国际化：document.title 由 BasicLayout/H5Layout 响应式更新
  // 此处仅作 fallback（无 Layout 的公开页保持简单标题）
  if (!to.meta.titleKey) {
    document.title = `${to.meta.title || ''} - KeyAuth SaaS`
  }

  const auth = useAuthStore()

  if (to.meta.public) {
    // H5 guestOnly 页面（登录/注册/重置密码）：已登录则跳 profile
    if (to.meta.guestOnly && to.path.startsWith('/h5')) {
      const endUserStore = useEndUserStore()
      endUserStore.restore()
      if (endUserStore.isLoggedIn) {
        next({ name: 'H5Profile' })
        return
      }
    }
    next()
    return
  }

  // H5 终端用户角色（enduser）：独立鉴权，不与三角色混淆
  if (to.meta.role === 'enduser') {
    const endUserStore = useEndUserStore()
    endUserStore.restore()
    if (!endUserStore.isLoggedIn) {
      next({ name: 'H5Login', query: { redirect: to.fullPath } })
      return
    }
    next()
    return
  }

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  // v0.6.5 修复：已登录但 role 为空（stale state），强制登出回登录页
  // 触发场景：localStorage 持久化数据损坏 / 旧版本字段缺失 / 手动篡改
  // 不修复会跳转到 '//dashboard' → 404，导致"后台进不去"
  if (to.meta.requiresAuth && auth.isLoggedIn && !auth.role) {
    auth.logout()
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  const requiredRole = to.meta.role as string | undefined
  if (requiredRole && auth.role !== requiredRole) {
    // role 已校验非空（上面已兜底），安全跳转
    next({ path: `/${auth.role}/dashboard` })
    return
  }

  next()
})

router.afterEach(() => {
  NProgress.done()
})

export default router
