// v0.5.0 后台国际化中文词汇表
// 覆盖范围：路由标题 / 登录注册 / Layout 公共组件 / 通用按钮与状态 / ThemeSwitcher
// 业务页面字符串暂保留中文，可通过 i18n key 逐步替换（向后兼容）
//
// 铁律 04：所有翻译文本集中定义在此文件，禁止业务代码硬编码中英文
// 铁律 06：翻译需符合实际语义，不编造术语

export default {
  common: {
    // 通用按钮
    save: '保存',
    cancel: '取消',
    confirm: '确认',
    ok: '确定',
    delete: '删除',
    edit: '编辑',
    add: '新增',
    create: '创建',
    update: '更新',
    search: '查询',
    reset: '重置',
    submit: '提交',
    back: '返回',
    close: '关闭',
    refresh: '刷新',
    export: '导出',
    import: '导入',
    download: '下载',
    upload: '上传',
    copy: '复制',
    view: '查看',
    detail: '详情',
    operation: '操作',
    actions: '操作',
    expand: '展开',
    collapse: '收起',
    more: '更多',
    all: '全部',
    none: '无',

    // 通用状态
    status: '状态',
    enabled: '启用',
    disabled: '禁用',
    active: '活跃',
    inactive: '未激活',
    pending: '待处理',
    approved: '已通过',
    rejected: '已驳回',
    success: '成功',
    failure: '失败',
    loading: '加载中...',
    empty: '暂无数据',
    yes: '是',
    no: '否',

    // 通用字段
    username: '账号',
    password: '密码',
    email: '邮箱',
    phone: '手机号',
    remark: '备注',
    description: '描述',
    name: '名称',
    title: '标题',
    content: '内容',
    createTime: '创建时间',
    updateTime: '更新时间',

    // 通用提示
    saveSuccess: '保存成功',
    saveFailure: '保存失败',
    deleteSuccess: '删除成功',
    deleteFailure: '删除失败',
    operationSuccess: '操作成功',
    operationFailure: '操作失败',
    confirmDelete: '确定删除吗？',
    tip: '提示',
    pleaseInput: '请输入',
    pleaseSelect: '请选择',
  },

  // v0.8.0：移除 theme 翻译（去除多主题与暗黑模式，不再需要主题切换器）

  // ============== 语言切换器 ==============
  language: {
    title: '切换语言',
    'zh-CN': '简体中文',
    'en-US': 'English',
  },

  // ============== Layout 公共 ==============
  layout: {
    profile: '账号设置',
    logout: '退出登录',
    confirmLogout: '确定退出登录吗？',
    user: '用户',
  },

  // ============== 登录页 ==============
  login: {
    subtitle: '多租户卡密验证平台',
    tabAdmin: '平台管理员',
    tabTenant: '开发者',
    tabAgent: '代理',
    account: '账号',
    password: '密码',
    pleaseInputAccount: '请输入账号',
    pleaseInputPassword: '请输入密码',
    passwordMinLength: '密码至少 8 位',
    submit: '登 录',
    totpLabel: '动态验证码',
    totpPlaceholder: '请输入 6 位动态验证码',
    totpHint: '请打开身份验证器 App（如 Google Authenticator）输入 6 位数字',
    totpSubmit: '验 证',
    backToLogin: '返回登录',
    registerTenant: '开发者注册',
    registerAgent: '代理注册',
    backHome: '返回首页',
    success: '登录成功',
    totpRequired: '请输入动态验证码',
    totpInvalid: '请输入 6 位动态验证码',
  },

  // ============== 注册/安装页 ==============
  register: {
    tenantTitle: '开发者注册',
    installTitle: '安装向导',
  },

  // ============== 路由标题（meta.title）==============
  route: {
    // 公共页面
    landing: '首页',
    login: '登录',
    install: '安装向导',
    tenantRegister: '开发者注册',
    notFound: '页面不存在',

    // H5 终端用户
    h5Home: '购卡',
    h5PayResult: '支付结果',
    h5Query: '卡密查询',
    h5CardDetail: '卡密详情',
    h5Login: '登录',
    h5Register: '注册',
    h5ResetPassword: '重置密码',
    h5Profile: '我的',
    h5MyCards: '我的卡密',
    h5Sessions: '会话管理',
    h5EditProfile: '编辑资料',
    h5ChangePassword: '修改密码',
    h5Orders: '我的订单',
    h5OrderDetail: '订单详情',
    h5NoticeDetail: '平台公告',
    h5Help: '帮助中心',
    h5Contact: '联系客服',
    agentPortal: '代理门户',
    agentPortalBuy: '购卡结算',

    // 平台管理员后台
    adminDashboard: '概览',
    adminTenants: '开发者管理',
    adminPackages: '套餐管理',
    adminAgents: '代理管理',
    adminNotices: '平台公告',
    adminPayConfig: '支付配置',
    adminSettlements: '结算管理',
    adminTenantWithdrawalReview: '开发者提现审核',
    adminSysConfig: '系统配置',
    adminLogs: '日志审计',
    adminSecurity: '安全防护',
    adminProfile: '账号设置',

    // 开发者后台
    tenantDashboard: '概览',
    tenantApps: '应用管理',
    tenantCardTypes: '卡类管理',
    tenantCards: '卡密管理',
    tenantDevices: '设备管理',
    tenantOrders: '订单管理',
    tenantSettlements: '结算记录',
    tenantWithdrawal: '提现申请',
    tenantCloudVars: '云变量',
    tenantVersions: '版本管理',
    tenantAgents: '代理管理',
    tenantInviteCodes: '邀请码',
    tenantRechargeReview: '充值审核',
    tenantWithdrawalReview: '提现审核',
    tenantPayConfig: '支付配置',
    tenantNotices: '我的公告',
    tenantProfile: '账号设置',

    // 代理后台
    agentDashboard: '概览',
    agentRegister: '注册代理',
    agentCards: '购卡',
    agentOrders: '我的订单',
    agentBalance: '余额/提现',
    agentCommission: '佣金记录',
    agentNotices: '消息通知',
    agentQrCode: '扫码购卡',
    agentProfile: '账号设置',
  }
}
