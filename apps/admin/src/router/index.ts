import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'
import { useAuthStore } from '@/stores/auth'

NProgress.configure({ showSpinner: false })

// 三套布局
import AdminLayout from '@/layouts/AdminLayout.vue'
import TenantLayout from '@/layouts/TenantLayout.vue'
import AgentLayout from '@/layouts/AgentLayout.vue'
import H5Layout from '@/layouts/H5Layout.vue'

// 占位组件
import PlaceholderView from '@/components/PlaceholderView.vue'

// 懒加载辅助
const lazy = (loader: () => Promise<any>) => loader

const routes: RouteRecordRaw[] = [
  // 官网首页
  {
    path: '/',
    name: 'Landing',
    component: lazy(() => import('@/views/landing/index.vue')),
    meta: { title: '首页', public: true }
  },

  // 登录
  {
    path: '/login',
    name: 'Login',
    component: lazy(() => import('@/views/login/index.vue')),
    meta: { title: '登录', public: true }
  },

  // 开发者注册
  {
    path: '/register/tenant',
    name: 'TenantRegister',
    component: lazy(() => import('@/views/register/TenantRegister.vue')),
    meta: { title: '开发者注册', public: true }
  },

  // ---------- 终端用户 H5 ----------
  {
    path: '/h5',
    component: H5Layout,
    meta: { public: true },
    children: [
      { path: '', name: 'H5Home', component: lazy(() => import('@/views/h5/Home.vue')), meta: { title: '购卡', public: true } },
      { path: 'pay/:orderNo', name: 'H5PayResult', component: lazy(() => import('@/views/h5/PayResult.vue')), meta: { title: '支付结果', public: true } },
      { path: 'query', name: 'H5Query', component: lazy(() => import('@/views/h5/Query.vue')), meta: { title: '卡密查询', public: true } },
      { path: 'card/:cardKey', name: 'H5CardDetail', component: lazy(() => import('@/views/h5/CardDetail.vue')), meta: { title: '卡密详情', public: true } }
    ]
  },

  // ---------- 平台管理员后台 ----------
  {
    path: '/admin',
    component: AdminLayout,
    redirect: '/admin/dashboard',
    meta: { role: 'admin', requiresAuth: true },
    children: [
      { path: 'dashboard',   name: 'AdminDashboard',  component: PlaceholderView, meta: { title: '概览',     icon: 'Odometer' } },
      { path: 'tenants',     name: 'AdminTenants',    component: PlaceholderView, meta: { title: '开发者管理', icon: 'User' } },
      { path: 'packages',    name: 'AdminPackages',   component: PlaceholderView, meta: { title: '套餐管理',  icon: 'Box' } },
      { path: 'agents',      name: 'AdminAgents',     component: PlaceholderView, meta: { title: '代理管理',  icon: 'UserFilled' } },
      { path: 'notices',     name: 'AdminNotices',    component: PlaceholderView, meta: { title: '平台公告',  icon: 'Bell' } },
      { path: 'pay-config',  name: 'AdminPayConfig',  component: PlaceholderView, meta: { title: '支付配置',  icon: 'Money' } },
      { path: 'settlements', name: 'AdminSettlements',component: lazy(() => import('@/views/admin/Settlements.vue')), meta: { title: '结算管理', icon: 'Wallet' } },
      { path: 'sys-config',  name: 'AdminSysConfig',  component: lazy(() => import('@/views/admin/SysConfig.vue')), meta: { title: '系统配置', icon: 'Setting' } },
      { path: 'logs',        name: 'AdminLogs',       component: PlaceholderView, meta: { title: '日志审计',  icon: 'Document' } },
      { path: 'security',    name: 'AdminSecurity',   component: PlaceholderView, meta: { title: '安全防护',  icon: 'Lock' } },
      { path: 'profile',     name: 'AdminProfile',    component: PlaceholderView, meta: { title: '账号设置',  icon: 'Setting' } }
    ]
  },

  // ---------- 开发者后台 ----------
  {
    path: '/tenant',
    component: TenantLayout,
    redirect: '/tenant/dashboard',
    meta: { role: 'tenant', requiresAuth: true },
    children: [
      { path: 'dashboard',     name: 'TenantDashboard',   component: PlaceholderView, meta: { title: '概览',     icon: 'Odometer' } },
      { path: 'apps',          name: 'TenantApps',        component: lazy(() => import('@/views/tenant/Apps.vue')), meta: { title: '应用管理', icon: 'Cellphone' } },
      { path: 'card-types',    name: 'TenantCardTypes',   component: lazy(() => import('@/views/tenant/CardTypes.vue')), meta: { title: '卡类管理', icon: 'Tickets' } },
      { path: 'cards',         name: 'TenantCards',       component: lazy(() => import('@/views/tenant/Cards.vue')), meta: { title: '卡密管理', icon: 'Key' } },
      { path: 'devices',       name: 'TenantDevices',     component: PlaceholderView, meta: { title: '设备管理',  icon: 'Monitor' } },
      { path: 'orders',        name: 'TenantOrders',      component: PlaceholderView, meta: { title: '订单管理',  icon: 'List' } },
      { path: 'cloud-vars',    name: 'TenantCloudVars',   component: PlaceholderView, meta: { title: '云变量',    icon: 'Coin' } },
      { path: 'versions',      name: 'TenantVersions',    component: PlaceholderView, meta: { title: '版本管理',  icon: 'Upload' } },
      { path: 'agents',        name: 'TenantAgents',      component: PlaceholderView, meta: { title: '代理管理',  icon: 'UserFilled' } },
      { path: 'invite-codes',  name: 'TenantInviteCodes', component: PlaceholderView, meta: { title: '邀请码',    icon: 'Promotion' } },
      { path: 'pay-config',    name: 'TenantPayConfig',   component: PlaceholderView, meta: { title: '支付配置',  icon: 'Money' } },
      { path: 'notices',       name: 'TenantNotices',     component: PlaceholderView, meta: { title: '我的公告',  icon: 'Bell' } },
      { path: 'profile',       name: 'TenantProfile',     component: PlaceholderView, meta: { title: '账号设置',  icon: 'Setting' } }
    ]
  },

  // ---------- 代理后台 ----------
  {
    path: '/agent',
    component: AgentLayout,
    redirect: '/agent/dashboard',
    meta: { role: 'agent', requiresAuth: true },
    children: [
      { path: 'dashboard',   name: 'AgentDashboard',  component: lazy(() => import('@/views/agent/Dashboard.vue')), meta: { title: '概览',     icon: 'Odometer' } },
      { path: 'register',    name: 'AgentRegister',   component: lazy(() => import('@/views/agent/Register.vue')), meta: { title: '注册代理', icon: 'Plus', public: true } },
      { path: 'cards',       name: 'AgentCards',      component: lazy(() => import('@/views/agent/Cards.vue')), meta: { title: '购卡',     icon: 'Key' } },
      { path: 'orders',      name: 'AgentOrders',     component: lazy(() => import('@/views/agent/Orders.vue')), meta: { title: '我的订单',  icon: 'List' } },
      { path: 'balance',     name: 'AgentBalance',    component: lazy(() => import('@/views/agent/Balance.vue')), meta: { title: '余额/提现', icon: 'Wallet' } },
      { path: 'commission',  name: 'AgentCommission', component: lazy(() => import('@/views/agent/Commission.vue')), meta: { title: '佣金记录',  icon: 'GoldMedal' } },
      { path: 'notices',     name: 'AgentNotices',    component: PlaceholderView, meta: { title: '消息通知',  icon: 'Bell' } },
      { path: 'profile',     name: 'AgentProfile',    component: PlaceholderView, meta: { title: '账号设置',  icon: 'Setting' } }
    ]
  },

  // 404
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: lazy(() => import('@/views/error/404.vue')),
    meta: { public: true, title: '页面不存在' }
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
  document.title = `${to.meta.title || ''} - KeyAuth SaaS`

  const auth = useAuthStore()

  if (to.meta.public) {
    next()
    return
  }

  if (to.meta.requiresAuth && !auth.isLoggedIn) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  const requiredRole = to.meta.role as string | undefined
  if (requiredRole && auth.role !== requiredRole) {
    next({ path: `/${auth.role}/dashboard` })
    return
  }

  next()
})

router.afterEach(() => {
  NProgress.done()
})

export default router
