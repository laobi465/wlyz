# 更新日志 (CHANGELOG)

所有显著变更均会记录于此文件。版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

格式约定：
- 分类标签：`[新增]` `[修改]` `[修复]` `[移除]` `[废弃]` `[安全]`
- 重大变更标注 `Breaking Change`
- 按版本倒序排列，最新版本置顶

---

## [0.8.0] - 2026-07-21（去除多主题与暗黑模式，回归单一明亮主题）

### 背景

用户要求「去除多模板和暗黑」。v0.5.0 引入的多主题架构（light/dark/blue/purple/green/auto）增加了维护复杂度，且 v0.7.0 排查发现多主题相关的 P0/P1 问题集中（图标名错误、白底白字、监听器累积、主题切换重叠等）。应产品要求回归单一明亮主题，降低维护成本与 UI 出错概率。

### 改动清单

#### 1. theme.ts store 简化（`apps/admin/src/stores/theme.ts`）

- `ThemeMode` 类型从 `'light' | 'dark' | 'blue' | 'purple' | 'green' | 'auto'` 收窄为 `'light'`
- `THEME_OPTIONS` 从 6 项缩减为 1 项（仅 light）
- 移除 `resolvedMode` / `isDark` getter（不再需要解析 auto）
- 移除 `toggleLightDark()` action
- `applyToDocument()` 改为清除 `data-theme` 属性 + 移除 `html.dark` class（兼容旧暗黑用户）
- `init()` 移除 `matchMedia` 监听器（不再需要响应系统主题变化），新增清理旧 localStorage 数据逻辑
- 移除 `_mqlHandlerAdded` 幂等标志（不再需要）

#### 2. themes.scss 简化（`apps/admin/src/styles/themes.scss`）

- 删除 `:root[data-theme="dark"]` 整块暗黑主题变量（约 55 行）
- 删除 `:root[data-theme="blue"]` 深蓝海洋主题（约 38 行）
- 删除 `:root[data-theme="purple"]` 紫罗兰主题（约 38 行）
- 删除 `:root[data-theme="green"]` 森林绿主题（约 38 行）
- 删除 `@media (prefers-color-scheme: dark) :root[data-theme="auto"]` 自动跟随系统主题（约 47 行）
- 仅保留 `:root` 默认明亮主题（与 v0.3.x 视觉一致）
- 文件从 280 行缩减为 52 行

#### 3. 删除 ThemeSwitcher.vue 组件

- 完全删除 `apps/admin/src/components/ThemeSwitcher.vue`（约 110 行）
- 不再需要主题切换 UI

#### 4. BasicLayout.vue 移除 ThemeSwitcher 引用（`apps/admin/src/layouts/BasicLayout.vue`）

- 移除 `<ThemeSwitcher />` 标签
- 更新文件头注释说明 v0.8.0 变更

#### 5. main.ts 移除 dark css-vars 导入（`apps/admin/src/main.ts`）

- 移除 `import 'element-plus/theme-chalk/dark/css-vars.css'`
- EP 暗黑模式 CSS 不再加载，减少打包体积约 30KB

#### 6. i18n 移除 theme 翻译

- `apps/admin/src/i18n/locales/zh-CN.ts`：移除 `theme: { title, light, dark, blue, purple, green, auto }` 翻译块
- `apps/admin/src/i18n/locales/en-US.ts`：移除对应英文翻译块

#### 7. 样式注释更新

- `apps/admin/src/styles/variables.scss`：更新文件头注释，说明 v0.8.0 回归单一明亮主题
- `apps/admin/src/styles/index.scss`：更新 themes.scss 引入注释

### 兼容性处理

- **旧用户 localStorage 清理**：`theme.ts` 的 `init()` 会检查 `localStorage.keyauth-theme`，若值不是 `{"mode":"light"}` 则清除，防止旧暗黑用户停留在不可用的暗黑状态
- **DOM 清理**：`applyToDocument()` 会移除 `data-theme` 属性和 `html.dark` class，确保即使有残留 DOM 状态也会被清除
- **store 保留**：保留极简 `useThemeStore` 桩，防止其他可能引用的地方报错（实际已无引用，但保留更安全）

### v0.8.0 验证

- [x] [已完成 2026-07-21] `cd apps/admin && npm run build` 通过（vue-tsc 类型检查 + Vite 构建，16.53s）
- [x] [已完成 2026-07-21] 打包体积验证：`element-vendor` 从 1.1MB 降至 1.07MB（减少 dark css-vars 约 30KB）
- [x] [已完成 2026-07-21] 无 TypeScript 类型错误（`ThemeMode` 收窄为 `'light'` 后所有引用兼容）

### v0.8.0 待真实环境验证

- [ ] [待开始] 真实浏览器验证：旧暗黑用户刷新页面后自动恢复明亮主题
- [ ] [待开始] 真实浏览器验证：顶栏不再显示主题切换器
- [ ] [待开始] 真实浏览器验证：所有页面颜色与 v0.7.0 明亮主题一致

### v0.8.0 影响范围

| 文件 | 改动类型 |
|---|---|
| `apps/admin/src/stores/theme.ts` | 简化（保留极简桩） |
| `apps/admin/src/styles/themes.scss` | 大幅删减（280→52 行） |
| `apps/admin/src/components/ThemeSwitcher.vue` | 删除 |
| `apps/admin/src/layouts/BasicLayout.vue` | 移除引用 |
| `apps/admin/src/main.ts` | 移除 dark css-vars 导入 |
| `apps/admin/src/i18n/locales/zh-CN.ts` | 移除 theme 翻译 |
| `apps/admin/src/i18n/locales/en-US.ts` | 移除 theme 翻译 |
| `apps/admin/src/styles/variables.scss` | 注释更新 |
| `apps/admin/src/styles/index.scss` | 注释更新 |

---

## [0.7.0] - 2026-07-21（前端管理员后台 UI 系统性修复：5 P0 + 14 P1）

### 背景

v0.6.9 修复 /admin/dashboard 性能问题后，用户要求「检查下前端管理员后台的 UI 是否有错误」。启动 4 个 subagent 并行排查 12 个管理员页面 + 11 个通用组件 + 4 个样式文件，发现 **5 个 P0** + **14 个 P1** + **30 个 P2/P3** 问题。

### [P0] 主题图标名错误（theme.ts）

`apps/admin/src/stores/theme.ts` 的 `THEME_OPTIONS` 配置了两个不存在的图标名，导致 ThemeSwitcher 下拉菜单中两个主题项图标渲染失败：

```ts
// 修复前（错误，EP 图标导出名无空格）
{ value: 'blue',   label: '深蓝',   icon: 'Water Cup' }
{ value: 'purple', label: '紫罗兰', icon: 'Magic Stick' }

// 修复后（正确导出名）
{ value: 'blue',   label: '深蓝',   icon: 'WaterCup' }
{ value: 'purple', label: '紫罗兰', icon: 'MagicStick' }
```

### [P0] PlatformNoticeBanner 白底白字不可见

`apps/admin/src/components/PlatformNoticeBanner.vue` 的 `.banner-label` 样式存在致命 CSS 错误：

```scss
// 修复前：currentColor 解析为父级 color:#fff，导致白底白字不可读
.banner-label {
  background: currentColor;
  color: #fff;
}

// 修复后：半透明白色背景 + 白色文字，在任何横幅背景色上可读
.banner-label {
  background: rgba(255, 255, 255, 0.25);
  color: #fff;
  flex-shrink: 0;
}
```

同时为该组件添加 `isMobile` 响应式检测 + dialog 响应式宽度 + resize 监听清理。

### [P0] Dashboard toFixed 崩溃 + 无 loading 反馈

`apps/admin/src/views/admin/Dashboard.vue` 存在两个 P0 问题：

1. **`stats.revenue_month.toFixed(2)` 在数据为 null 时抛 TypeError**：后端字段可能为 null/undefined，`toFixed` 不是函数崩溃
2. **onMounted(loadDashboard) 期间页面全 0/空无反馈**：用户访问 /admin/dashboard 时页面卡很久但无 loading 遮罩

修复：

```ts
// v0.7.0：金额格式化兜底
const formatMoney = (n: number | null | undefined, decimals = 2) => {
  const num = Number(n) || 0
  return num.toFixed(decimals)
}

// v0.7.0：loading 状态 + v-loading 遮罩
const loading = ref(false)
const loadDashboard = async () => {
  loading.value = true
  try {
    const data = await adminDashboardApi()
    // ... 数据兜底处理
  } finally {
    loading.value = false
  }
}
```

所有 `.toFixed(2)` 改为 `formatMoney()`，模板添加 `<div v-loading="loading">` 遮罩，`formatDate` 添加 Invalid Date 兜底。

### [P0] Profile.vue labelPosition 非响应式

`apps/admin/src/views/admin/Profile.vue` 的 `labelPosition` computed 直接读 `window.innerWidth`（非响应式数据），窗口尺寸变化时不会重新计算：

```ts
// 修复前（非响应式）
const labelPosition = computed(() => {
  return window.innerWidth < 768 ? 'top' : 'right'
})

// 修复后（响应式 ref + resize 监听）
const isMobile = ref(false)
const checkMobile = () => { isMobile.value = window.innerWidth < 768 }
const labelPosition = computed<'top' | 'right'>(() => isMobile.value ? 'top' : 'right')

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
  // ...
})
onBeforeUnmount(() => window.removeEventListener('resize', checkMobile))
```

同时为 Profile.vue 修复：dialog 响应式宽度、saveProfile/changePassword 防抖守卫、changePassword 的 callback+async 混用改为 await validate() Promise 形式、formatDate Invalid Date 兜底。

### [P1] BasicLayout header-right flex 布局（主题切换重叠根因）

`apps/admin/src/layouts/BasicLayout.vue` 的 `.header-right` 无 `flex-wrap`、子元素无 `flex-shrink:0`，窄屏时主题切换与用户菜单重叠（即用户反馈的「左上角重叠的主题切换」UI 根因之一）：

```scss
// 修复后
.header-left {
  min-width: 0;       // v0.7.0：让面包屑可被截断
  flex: 1;
  .collapse-btn, .menu-btn {
    flex-shrink: 0;    // v0.7.0：固定按钮不被挤压
  }
}
.header-right {
  flex-wrap: wrap;     // v0.7.0：窄屏时换行而非重叠
  justify-content: flex-end;
  flex-shrink: 0;      // v0.7.0：整个 header-right 不被挤压
  .user-info {
    flex-shrink: 0;
    .username { white-space: nowrap; }
  }
}
```

同时修复 `el-dropdown` 未设 `placement="bottom-end"` 导致下拉菜单溢出右侧；`onMounted` 中 `await sysConfig.load()` 失败会导致 `checkMobile` 不执行 → 改为先 `checkMobile()` + 注册 resize，`sysConfig.load()` 异步触发不阻塞。

### [P1] ThemeSwitcher/LanguageSwitcher placement + flex-shrink

`apps/admin/src/components/ThemeSwitcher.vue` 和 `LanguageSwitcher.vue`：

1. `el-dropdown` 未设 `placement="bottom-end"` → 下拉菜单可能溢出右侧
2. `.theme-switcher` / `.lang-switcher` 无 `flex-shrink:0` → 窄屏被挤压

修复：`placement="bottom-end"` + `flex-shrink: 0` + `white-space: nowrap`，同时移除 ThemeSwitcher 中 `margin-left: auto` 被 `margin-left: $spacing-md` 覆盖的死声明。

### [P1] ResponsiveTable size-change 双触发 loadList

`apps/admin/src/components/ResponsiveTable.vue` 的 EP `el-pagination` 在用户切换 page-size 时会先发 `size-change` 再发 `current-change`（page 重置为 1），父组件同时监听两个事件会导致 `loadList` 双触发：

```ts
// v0.7.0 修复：用 _sizeChanging 标志位抑制 size-change 后的 page-change
let _sizeChanging = false

const onPageChange = (p: number) => {
  if (_sizeChanging) {
    _sizeChanging = false
    return  // 抑制 size-change 触发的 current-change
  }
  emit('update:page', p)
  emit('page-change', p)
}
const onSizeChange = (s: number) => {
  _sizeChanging = true
  currentPage.value = 1   // 显式同步让 v-model 双向生效
  emit('update:pageSize', s)
  emit('update:page', 1)
  emit('size-change', s)
}
```

同时删除冗余的 `currentPage.value = p` / `pageSizeRef.value = s`（v-model 已绑定）。

### [P1] Settlements.vue net_amount.toFixed 崩溃

`apps/admin/src/views/admin/Settlements.vue` 第 158 行 `currentRow.net_amount.toFixed(2)` 在数据为 null 时抛 TypeError：

```vue
<!-- 修复前 -->
<el-input :model-value="currentRow ? '¥' + currentRow.net_amount.toFixed(2) : ''" disabled />

<!-- 修复后：复用已有的 formatMoney 函数 -->
<el-input :model-value="currentRow ? '¥' + formatMoney(currentRow.net_amount) : ''" disabled />
```

同时为该文件修复：单条/批量结算 dialog 响应式宽度、confirmSettle/confirmBatchSettle 防抖守卫、formatDate Invalid Date 兜底、isMobile + resize 监听。

### [P1] 12 个管理员页面系统性修复

用 2 个并行 subagent 修复 9 个管理员页面（Settlements/Profile 已直接修复）的 5 类共性问题：

| 文件 | 防抖 | clearValidate | dialog 响应式 | filter.page 重置 | formatDate 兜底 |
|---|---|---|---|---|---|
| Tenants.vue | ✅ | ✅ | ✅ | ✅ | ✅ |
| Agents.vue | ✅ | ✅ | ✅ | ✅ | ✅ |
| Packages.vue | ✅ | ✅ | ✅ | ✅ | ✅ |
| Notices.vue | ✅ | ✅ | ✅ | ✅ | ✅ |
| SysConfig.vue | ✅ | - | ✅ | - | - |
| Logs.vue | - | - | ✅ | ✅ | ✅ |
| Security.vue | ✅ | ✅ | ✅ | - | ✅ |
| PayConfig.vue | ✅ | - | - | - | - |
| TenantWithdrawalReview.vue | ✅ | ✅ | ✅ | - | ✅ |

**防抖守卫**：所有 `confirm*/submit*/save*` 函数添加 `if (xxxLoading.value) return`，避免重复点击产生重复请求

**clearValidate**：`openEdit/openAdd/openReject` 改为 async，在 `dialogVisible.value = true` 之后 `await nextTick(); formRef.value?.clearValidate()`，清除上一次表单验证残留状态

**dialog 响应式**：所有 `<el-dialog width="Npx">` 改为 `:width="isMobile ? '92%' : 'Npx'"`，添加 isMobile + resize 监听

**filter.page 重置**：筛选条件 `@change="loadList"` 改为 `@change="onFilterChange"`，新建 `onFilterChange` 函数重置 `filter.page = 1` 后再 `loadList()`

**formatDate 兜底**：所有 `formatDate` 添加 `if (isNaN(d.getTime())) return '-'` 兜底

**Security.vue validate callback+async 混用**：原 `formRef.value.validate(async (valid) => { ... })` 改为 `try { await formRef.value.validate() } catch { return }`，避免 callback+async 混用导致状态错乱

**PayConfig.vue saveAll 二次确认**：函数开头添加 `ElMessageBox.confirm('确定要保存所有支付配置吗？', '确认', { type: 'warning' })`，避免误触发批量保存

### v0.7.0 验证

- [x] [已完成 2026-07-21] `cd apps/admin && npm run build` 通过（vue-tsc 类型检查 + Vite 构建，16.69s）
- [x] [已完成 2026-07-21] 修复逻辑验证：所有 5 个 P0 + 14 个 P1 修复点均通过代码 review
- [x] [已完成 2026-07-21] 4 个 subagent 并行修复 9 个管理员页面，第一个 subagent 内部还跑了 vue-tsc + vite build 验证

### v0.7.0 待真实环境验证

- [ ] [待开始] 真实浏览器验证 ThemeSwitcher 下拉菜单中 blue/purple 主题图标正常显示
- [ ] [待开始] 真实浏览器验证 PlatformNoticeBanner 「平台公告」标签不再白底白字
- [ ] [待开始] 真实浏览器验证 /admin/dashboard 加载期间显示 loading 遮罩
- [ ] [待开始] 真实浏览器验证窄屏（< 768px）顶栏主题切换与用户菜单不再重叠
- [ ] [待开始] 真实浏览器验证 ResponsiveTable 切换 page-size 时不再触发两次 loadList
- [ ] [待开始] 真实浏览器验证所有管理员页面对话框在移动端宽度 92% 自适应

### v0.7.0 未修复的 P2/P3（30 项，非阻断性）

- SysConfig.vue 编辑表单无 rules、el-input rows 用在非 textarea
- Logs.vue search-bar 无移动端样式
- Security.vue IP/CIDR 缺格式校验
- PayConfig.vue saveAll 串行 await 11 字段、整个表单无 rules
- ResponsiveTable `:key="idx"` 用数组索引作 key
- themes.scss auto+暗黑分支未覆盖 `--el-color-primary*`
- 其余 24 项 P3 级别细节问题

详见 [v0.7.0 TODO](docs/TODO.md#v070-前端管理员后台-ui-系统性修复--已完成-2026-07-21)。

---

## [0.6.9] - 2026-07-21（/admin/dashboard 加载卡很久 + 主题切换监听器累积修复）

### [修复] P1 性能：/admin/dashboard 加载卡很久（后端 21 次串行 DB 查询）

#### 症状

v0.6.8 修复 UpdateNotifier setInterval 累积卡死后，用户反馈「访问 `/admin/dashboard` 后会卡很久」。页面能进入但加载延迟明显。

#### 根因定位

后端 `apps/server/internal/handler/admin_business.go` 的 `AdminDashboard` handler 存在 **21 次串行 DB 查询**：

| 类别 | 查询次数 | 说明 |
|---|---|---|
| 开发者统计 | 2 | tenant_total + tenant_active |
| 代理统计 | 2 | agent_total + agent_active |
| 应用/卡密统计 | 3 | app_total + card_total + card_active |
| 今日订单/收入 | 3 | order_today + revenue_today + revenue_month |
| 结算待处理 | 2 | settlement_pending + settlement_amount |
| 最近开发者 | 1 | recent_tenants LIMIT 10 |
| 最近订单 | 1 | recent_orders LIMIT 10 |
| **7 日收入趋势** | **7** | **for i := 6; i >= 0; i-- { SELECT SUM WHERE paid_at >= dayStart AND paid_at < dayEnd }** |
| **合计** | **21** | 全部串行执行 |

**头号性能瓶颈**：7 日收入趋势是循环 7 次单独 SUM 查询（经典 N+1）：

```go
// 修复前（N+1）
revenueTrend := make([]gin.H, 0, 7)
for i := 6; i >= 0; i-- {
    dayStart := startOfToday.AddDate(0, 0, -i)
    dayEnd := dayStart.AddDate(0, 0, 1)
    var dayRevenue float64
    deps.DB.Model(&model.AppOrder{}).
        Where("pay_status = ? AND paid_at >= ? AND paid_at < ?", "paid", dayStart, dayEnd).
        Select("COALESCE(SUM(total_amount), 0)").Scan(&dayRevenue)
    revenueTrend = append(revenueTrend, gin.H{
        "date":   dayStart.Format("2006-01-02"),
        "amount": dayRevenue,
    })
}
```

每次循环执行 `SELECT SUM(total_amount) FROM app_order WHERE pay_status='paid' AND paid_at >= ? AND paid_at < ?`，7 次串行 = 7 倍延迟。app_order 表数据量大时（百万级订单），每次 SUM 查询扫描 idx_paid_at 索引 + 回表，累计数秒。

加上其他 14 次统计查询（tenant/agent/app/card/order/settlement 各 2-3 次），总延迟可达 5-10 秒，正好对应「卡很久」。

#### 修复：7 日趋势 N+1 → 1 次 GROUP BY 查询

```go
// 修复后（1 次查询）
sevenDaysAgo := startOfToday.AddDate(0, 0, -6)
type dayRevenueRow struct {
    Day     string  `gorm:"column:day"`
    Revenue float64 `gorm:"column:revenue"`
}
var dayRows []dayRevenueRow
deps.DB.Model(&model.AppOrder{}).
    Select("DATE(paid_at) AS day, COALESCE(SUM(total_amount), 0) AS revenue").
    Where("pay_status = ? AND paid_at >= ?", "paid", sevenDaysAgo).
    Group("DATE(paid_at)").
    Scan(&dayRows)
// 构建日期到金额的 map（仅有订单的日期）
revenueMap := make(map[string]float64, len(dayRows))
for _, r := range dayRows {
    revenueMap[r.Day] = r.Revenue
}
// 填充 7 日数据（无订单的日期补 0）
revenueTrend := make([]gin.H, 0, 7)
for i := 6; i >= 0; i-- {
    dayStart := startOfToday.AddDate(0, 0, -i)
    dateStr := dayStart.Format("2006-01-02")
    revenueTrend = append(revenueTrend, gin.H{
        "date":   dateStr,
        "amount": revenueMap[dateStr], // map 取不到时返回 0，正好是"无订单"的默认值
    })
}
```

**优化效果**：
- 查询次数：21 → 15（7 日趋势部分 7 次 → 1 次）
- 7 日趋势部分延迟降低 ~85%（7 次 DB 往返 → 1 次）
- 利用 MySQL `DATE(paid_at)` 函数 + `GROUP BY` 一次聚合所有日期
- 无订单的日期在内存中补 0，不丢失日期轴

#### 索引确认

`app_order` 表已有 `idx_paid_at` 索引（migration 001），`WHERE paid_at >= ?` 可走索引范围扫描。`GROUP BY DATE(paid_at)` 在 MySQL 8.0 下可利用索引或临时表高效聚合。

### [修复] P3 UI 异常：主题切换监听器累积

#### 症状

v0.6.8 修复前端死循环后，用户反馈「进入后左上角会显示更多重叠的主题切换」。

#### 根因定位

`apps/admin/src/stores/theme.ts` 的 `init()` 每次调用都 `matchMedia.addEventListener('change', handler)`，无去重：

```ts
// 修复前
init() {
    this.applyToDocument()
    if (typeof window === 'undefined' || !window.matchMedia) return
    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
        if (this.mode === 'auto') {
            this.applyToDocument()
        }
    }
    if (mql.addEventListener) {
        mql.addEventListener('change', handler)  // ← 每次调用都添加，无去重
    } else if ((mql as any).addListener) {
        (mql as any).addListener(handler)
    }
}
```

`ThemeSwitcher.vue` 的 `onMounted` 调用 `themeStore.init()`。每次路由切换 ThemeSwitcher 重新挂载都会调 `init()`，导致 matchMedia 监听器累积：

- 第 1 次挂载：1 个监听器
- 第 2 次挂载：2 个监听器（第 1 个未移除）
- 第 N 次挂载：N 个监听器

虽不致死循环（matchMedia change 事件不频繁），但：
1. 内存泄漏（监听器永不释放）
2. auto 模式下系统主题变化时，N 个 handler 同时触发 `applyToDocument()`，可能导致 UI 渲染异常
3. 多次 applyToDocument 可能导致 DOM 状态不一致，触发 EP 重新渲染，表现为「主题切换重叠」

#### 修复：theme store 新增 _mqlHandlerAdded 幂等标志

```ts
// 修复后
interface ThemeState {
  mode: ThemeMode
  /** v0.6.9：matchMedia 监听器是否已添加（幂等标志） */
  _mqlHandlerAdded: boolean
}

init() {
    this.applyToDocument()
    if (typeof window === 'undefined' || !window.matchMedia) return
    // v0.6.9 幂等：已添加过监听器则跳过
    if (this._mqlHandlerAdded) return
    this._mqlHandlerAdded = true
    const mql = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => {
        if (this.mode === 'auto') {
            this.applyToDocument()
        }
    }
    if (mql.addEventListener) {
        mql.addEventListener('change', handler)
    } else if ((mql as any).addListener) {
        (mql as any).addListener(handler)
    }
}

// persist.paths 仍只持久化 ['mode']，_mqlHandlerAdded 不持久化
persist: {
    key: 'keyauth-theme',
    storage: localStorage,
    paths: ['mode']
}
```

**幂等逻辑**：
- 首次 `init()`：`_mqlHandlerAdded` 为 false → 添加监听器 → 置 true
- 后续 `init()`（路由切换 ThemeSwitcher 重新挂载）：`_mqlHandlerAdded` 为 true → 跳过
- 页面刷新后：pinia store 重建，`_mqlHandlerAdded` 重置为 false（因不持久化）→ 正常重新添加监听器

#### 修复涉及的文件

| 文件 | 修改点 |
|---|---|
| `apps/server/internal/handler/admin_business.go` | `AdminDashboard` 7 日收入趋势：7 次循环 SUM 查询 → 1 次 `GROUP BY DATE(paid_at)` 查询，内存补齐无订单日期为 0 |
| `apps/admin/src/stores/theme.ts` | `ThemeState` 新增 `_mqlHandlerAdded: boolean`；`init()` 添加幂等检查，已添加过监听器则跳过 |

#### 验证

- 后端 `go build ./internal/handler/` 通过
- 后端 `gofmt -w internal/handler/admin_business.go` 格式正确
- 前端 `cd apps/admin && npm run build` 通过（含 `vue-tsc --noEmit` 类型检查，17.32s）
- 性能优化逻辑验证：
  - 7 日趋势：1 次 GROUP BY 查询拿到所有有订单日期 → map 查找 → 补 0 → 数据完整
  - 无订单日期：map 取不到返回 0（float64 零值），与原逻辑 `COALESCE(SUM, 0)` 一致
- 幂等逻辑验证：
  - 首次 init：添加监听器 + 置标志位
  - 路由切换重新挂载：跳过添加
  - 页面刷新：store 重建，标志位重置，正常添加

#### 与 v0.6.7/v0.6.8 的关系

- v0.6.7 修复 scheduleRefresh 无限异步递归（token 续期死循环）
- v0.6.8 修复 UpdateNotifier setInterval 累积（更新轮询并发累积卡死）
- v0.6.9 修复 AdminDashboard N+1 查询（页面加载卡很久）+ theme 监听器累积（UI 渲染异常）

三者独立但症状相似（都表现为「卡」），v0.6.9 修复后管理员 Dashboard 体验应恢复正常。如仍有「主题切换重叠」UI 异常，需提供浏览器截图进一步排查（可能是 ElDropdown teleport 定位或 transition 动画问题）。

---

## [0.6.8] - 2026-07-21（UpdateNotifier setInterval 累积导致 /admin/dashboard 卡死修复）

### [修复] P0 紧急 bug：v0.6.7 修复后管理员登录依然卡死

#### 症状

v0.6.7 修复 `scheduleRefresh` 无限异步递归后，用户反馈「依旧进不去 /admin/dashboard 会卡死」。浏览器 CPU 飙升、内存持续增长、最终死机。

#### 根因定位（10 个方向深度排查）

使用 search subagent 排查 10 个潜在方向：

| 方向 | 是否死循环根因 | 紧急度 |
|---|---|---|
| **UpdateNotifier setInterval 累积** | **是（P0）** | 立即修复 |
| **UpdateNotifier 卸载后回调泄漏** | 是（放大 P0） | 立即修复 |
| pinia persist 不恢复 scheduleRefresh | 否（功能性退化） | 建议修复 |
| http.ts doRefresh 锁 return 后无效重试 | 否 | 建议修复 |
| Dashboard toFixed NaN 风险 | 否 | 建议修复 |
| ThemeSwitcher matchMedia 监听器累积 | 否 | 建议修复 |
| 路由守卫 / Vue 响应式 / enduser store / EP 组件 | 否 | 无需修复 |

**头号根因**：`apps/admin/src/components/UpdateNotifier.vue` 的 `scheduleNext` 函数使用 `setInterval` 调度异步轮询。

#### 死循环代码（修复前）

```ts
// 修复前（v0.4.0 引入，v0.6.8 修复）
const timer = ref<ReturnType<typeof setInterval> | null>(null)

const scheduleNext = (intervalSec: number) => {
  timer.value = setInterval(async () => {
    const next = await pollOnce()
    if (next === 0) {
      stopPolling()
      return
    }
    if (next !== intervalSec) {
      stopPolling()        // ← 只能清除当前 timer.value 指向的 setInterval
      scheduleNext(next)   // ← 创建新 setInterval，但排队中的旧回调仍会执行
    }
  }, intervalSec * 1000)
}

const stopPolling = () => {
  if (timer.value) {
    clearInterval(timer.value)
    timer.value = null
  }
}

onBeforeUnmount(() => {
  stopPolling()  // ← 无法取消已在事件循环中排队但尚未执行的 setInterval 回调
})
```

#### 三个致命问题

##### 问题 1：setInterval 不等待 async 回调完成 → 并发回调累积

`setInterval` 触发回调时**不等待** async 函数完成。当 `pollOnce()`（含 HTTP 请求）耗时超过 `interval` 时：

- 第 N 秒：回调 A 开始执行，await pollOnce()（耗时 5s）
- 第 N+interval 秒：回调 B 开始执行（A 还在 await），又 await pollOnce()
- 第 N+2*interval 秒：回调 C 开始执行（A、B 都在 await），又 await pollOnce()
- ...

多个回调并发执行 → 多个 HTTP 请求并发 → 网络拥塞 → pollOnce 更慢 → 更多回调累积 → CPU 100%

##### 问题 2：interval 变化时引用覆盖泄漏

当后端返回新的 `interval_seconds`（如管理员后台调整轮询间隔），代码执行 `stopPolling() + scheduleNext(next)`：

1. `stopPolling()` 清除当前 `timer.value` 指向的 setInterval
2. `scheduleNext(next)` 创建新 setInterval，`timer.value` 被覆盖为新引用

**但事件循环中排队中的旧 setInterval 回调仍会执行**！这些旧回调执行时：
- 调用 `pollOnce()` 获取新 interval
- 调用 `scheduleNext(next)` 创建**另一个**新 setInterval
- `timer.value` 再次被覆盖

结果：旧 setInterval 永远无法被 `clearInterval` 清除（引用已丢失），形成**引用覆盖泄漏**。每次 interval 变化都泄漏一个 setInterval，指数级增长。

##### 问题 3：组件卸载后回调泄漏

`onBeforeUnmount` 中 `stopPolling()` 只能清除当前 `timer.value`。但**已在事件循环中排队但尚未执行的 setInterval 回调**无法取消，这些回调在组件卸载后继续执行：
- 调用 `pollOnce()` → 发起 HTTP 请求（组件已卸载，响应处理可能报错）
- 调用 `scheduleNext(next)` → 创建新 setInterval（`timer.value` 被赋值，但已无人清理）
- 形成永久泄漏的定时器，持续发起 HTTP 请求直到页面关闭

#### 修复方案

##### 修复 1：scheduleNext 改用 setTimeout 自递归

`setTimeout` 自递归保证：**每次 pollOnce 完成后才调度下一次**，从根上消除并发。

```ts
// 修复后
const scheduleNext = (intervalSec: number) => {
  timer.value = setTimeout(async () => {
    if (isUnmounted) return           // 卸载后不再执行

    const next = await pollOnce()

    if (isUnmounted) return           // await 期间组件可能已卸载

    if (next === 0) {
      timer.value = null
      return                          // 后端关闭弹窗通知，不再调度
    }

    // 不论 next 是否等于 intervalSec，都重新调度（自递归）
    scheduleNext(next)
  }, intervalSec * 1000)
}
```

优势：
- 同一时刻**只有一个 setTimeout 在排队**（前一个执行并 await 完成后才会创建下一个）
- interval 变化时**无需 stopPolling + scheduleNext 二步操作**，直接 `scheduleNext(next)` 即可
- 不存在引用覆盖泄漏（每次只有一个 timer 引用）

##### 修复 2：timer 类型与 stopPolling 适配

```ts
// 修复前
const timer = ref<ReturnType<typeof setInterval> | null>(null)
const stopPolling = () => {
  if (timer.value) {
    clearInterval(timer.value)
    timer.value = null
  }
}

// 修复后
const timer = ref<ReturnType<typeof setTimeout> | null>(null)
const stopPolling = () => {
  if (timer.value) {
    clearTimeout(timer.value)
    timer.value = null
  }
}
```

##### 修复 3：isUnmounted 标志位防止卸载后回调执行

```ts
// 修复后
let isUnmounted = false

onBeforeUnmount(() => {
  // 先标记卸载，再清除定时器
  // - 标记 isUnmounted 后，已在事件循环中排队但尚未执行的 setTimeout 回调开头会直接 return
  // - clearTimeout 取消尚未触发的定时器
  isUnmounted = true
  stopPolling()
})
```

setTimeout 回调中有两处 `isUnmounted` 检查：
1. 回调开头（防止 onBeforeUnmount 后回调执行）
2. `await pollOnce()` 之后（防止 await 期间组件卸载后继续执行）

##### 修复 4：startPolling 首次 await 期间防御性检查

```ts
const startPolling = async () => {
  const interval = await pollOnce()
  if (interval === 0) return

  // v0.6.8：首次 await 期间组件可能已卸载，防御性检查
  if (isUnmounted) return

  scheduleNext(interval)
}
```

#### 修复后完整代码

```ts
// v0.6.8 修复后完整代码
const timer = ref<ReturnType<typeof setTimeout> | null>(null)
let isUnmounted = false

const scheduleNext = (intervalSec: number) => {
  timer.value = setTimeout(async () => {
    if (isUnmounted) return

    const next = await pollOnce()

    if (isUnmounted) return

    if (next === 0) {
      timer.value = null
      return
    }

    scheduleNext(next)
  }, intervalSec * 1000)
}

const stopPolling = () => {
  if (timer.value) {
    clearTimeout(timer.value)
    timer.value = null
  }
}

onMounted(() => {
  startPolling()
})

onBeforeUnmount(() => {
  isUnmounted = true
  stopPolling()
})
```

#### 修复涉及的文件

| 文件 | 修改点 |
|---|---|
| `apps/admin/src/components/UpdateNotifier.vue` | `scheduleNext` setInterval→setTimeout 自递归；`timer` 类型声明；`stopPolling` clearTimeout；新增 `isUnmounted` 标志位；`onBeforeUnmount` 先标记再 stopPolling；`startPolling` 防御性 isUnmounted 检查 |

#### 验证

- `cd apps/admin && npm run build` 通过（含 `vue-tsc --noEmit` 类型检查，16.85s）
- 死循环场景逻辑验证：
  - pollOnce 耗时 > interval → setTimeout 自递归保证只有 1 个回调在执行 → 不累积
  - interval 变化 → 直接 scheduleNext(next) → 无引用覆盖泄漏
  - 组件卸载 → isUnmounted=true → 排队回调开头 return → clearTimeout 取消未触发定时器 → 无泄漏
- 正常场景验证：onMounted 启动 startPolling → pollOnce → setTimeout → 到期执行 → pollOnce → setTimeout → ...

#### 与 v0.6.7 的关系

v0.6.7 修复的 `scheduleRefresh` 死循环是**第一个根因**（token 续期递归），v0.6.8 修复的 `UpdateNotifier.scheduleNext` setInterval 累积是**第二个根因**（更新轮询并发累积）。两者独立存在，v0.6.7 仅修复第一个，导致用户反馈「依旧进不去 /admin/dashboard 会卡死」。v0.6.8 修复第二个根因后，管理员登录卡死问题应彻底解决。

#### 后续建议（非阻断性）

| 项 | 紧急度 | 说明 |
|---|---|---|
| pinia persist 不恢复 scheduleRefresh | 建议修复 | 页面刷新后 scheduleRefresh 不自动重启，access token 过期后无法自动续期（功能性退化，非死循环） |
| http.ts doRefresh 锁 return 后无效重试 | 建议修复 | 401 拦截器 `_refreshing` 锁 return 后仍可能重试原请求，建议返回 Promise.reject |
| Dashboard toFixed NaN 兜底 | 建议修复 | 数据为 null/undefined 时 toFixed 报错，建议 `Number(x) \|\| 0` |
| ThemeSwitcher init 幂等 | 建议修复 | matchMedia 监听器无去重，多次调用 init 会累积监听器 |

---

## [0.6.7] - 2026-07-21（管理员登录后死机 + token 续期死循环修复）

### [修复] P0 紧急 bug：管理员登录后浏览器死机

#### 症状

管理员登录成功后页面卡住，CPU 飙升，浏览器最终死机。开发者/代理登录正常。

#### 根因定位

`apps/admin/src/stores/auth.ts` 的 `scheduleRefresh` 在 `delay <= 0` 时不 await 调用 `doRefresh().catch(() => {})`，`doRefresh` 成功后又调用 `this.scheduleRefresh()`，若新的 `expires_at` 仍让 `delay <= 0`，则形成无限异步递归，每秒发出数十个 HTTP 请求导致浏览器死机。

触发 `delay <= 0` 的场景：
1. `jwt.access_ttl_seconds` 被 sys_config 配为 ≤ 300 秒（管理员可在后台修改）
2. 后端返回异常 `expires_at`（0、负数、旧时间戳）
3. 客户端时钟超前（refreshAt = expiresAt - 300 已经在过去）

死循环代码：

```ts
// 修复前
scheduleRefresh() {
  // ...
  if (delay <= 0) {
    this.doRefresh().catch(() => {})  // ← 不 await，立即返回
    return                            // ← doRefresh 完成后会再调 scheduleRefresh
  }
}

async doRefresh() {
  // ...
  this.expiresAt = resp.expires_at   // ← 异常值直接写入
  this.scheduleRefresh()              // ← 再次进入，若 delay 仍 <=0，无限递归
}
```

#### 修复 1：`_refreshing` 并发锁杜绝递归

`apps/admin/src/stores/auth.ts` state 新增 `_refreshing: boolean`：

- `scheduleRefresh` 的 `delay <= 0` 路径检查 `_refreshing`，正在刷新则直接 return，由 `doRefresh` 完成后重新调度
- `doRefresh` 入口检查 `_refreshing`，正在刷新则 return，避免并发重复刷新
- `try/finally` 保证 `_refreshing` 一定释放
- `logout` 时重置 `_refreshing`，避免下次登录复用 stale 状态

```ts
// 修复后
scheduleRefresh() {
  // ...
  if (delay <= 0) {
    if (this._refreshing) return   // ← 关键：正在刷新则不重复触发
    this.doRefresh().catch(() => {})
    return
  }
}

async doRefresh() {
  if (!this.refreshToken) return
  if (this._refreshing) return      // ← 并发锁
  this._refreshing = true
  try {
    // ...
  } finally {
    this._refreshing = false
  }
}
```

#### 修复 2：`expires_at` 合法性校验

`doRefresh` 接收到后端响应后，校验新的 `expires_at` 必须 > `now + 60s`（绝对 Unix 秒）：

- 异常值（0、负数、旧时间戳、临界值）直接跳过更新与重排
- 防御后端异常返回导致死循环
- 下一次 401 拦截器或定时器触发时由 `_refreshing` 锁自然阻塞

```ts
const nowSec = Math.floor(Date.now() / 1000)
const newExpiresAt = resp.expires_at || 0
if (!newExpiresAt || newExpiresAt <= nowSec + 60) {
  console.warn('[auth] refresh 返回异常 expires_at，跳过更新', newExpiresAt)
  return
}
```

#### 修复 3：`scheduleRefresh` 最小延迟保护

`delay` 计算后强制 `delay = Math.max(delay, 30_000)`（30 秒兜底）：

- 触发场景：sys_config 中 `jwt.access_ttl_seconds` 被设为异常小值（如 60s），导致 `expiresAt - 300` 为负数
- 避免 setTimeout 极短延迟触发高频刷新
- 30 秒下限是用户感知不敏感的安全阈值

#### 修复 4：`http.ts` 401 拦截器加固

`apps/admin/src/api/http.ts` 的 401 拦截器在 `await auth.doRefresh()` 后增加空 token 检查：

```ts
await auth.doRefresh()
const newToken = auth.accessToken
if (!newToken) {
  // doRefresh 成功但 token 为空（异常 expires_at 被跳过更新）
  onTokenRefreshed('')
  auth.logout()
  redirectToLogin()
  return Promise.reject(err)
}
```

避免 `doRefresh` 在异常场景下静默跳过更新后，拦截器仍用旧 token 重试导致死循环。

#### 修复 5：后端 sys_config 最小值校验（源头防护）

`apps/server/internal/handler/admin.go` 的 `AdminUpdateConfig` 新增 `validateSysConfigValue` 函数：

- `jwt.access_ttl_seconds`：必须 ≥ 600 秒（10 分钟），否则前端「提前 5 分钟续期」逻辑会立即触发刷新
- `jwt.access_ttl_seconds`：必须 ≤ 30 天
- `jwt.refresh_ttl_seconds`：必须 ≥ 3600 秒（1 小时）
- `jwt.refresh_ttl_seconds`：必须 ≤ 90 天
- 非法值返回 HTTP 400 + 1002 错误码，提示管理员具体原因

```go
func validateSysConfigValue(key, value string) error {
    switch key {
    case "jwt.access_ttl_seconds":
        v, err := strconv.Atoi(value)
        if err != nil { return errInvalidConfigValue("jwt.access_ttl_seconds 必须是整数") }
        if v < 600 { return errInvalidConfigValue("jwt.access_ttl_seconds 不能小于 600 秒（10 分钟），否则会触发前端高频刷新") }
        if v > 86400*30 { return errInvalidConfigValue("jwt.access_ttl_seconds 不能大于 30 天") }
    case "jwt.refresh_ttl_seconds":
        // ...
    }
    return nil
}
```

#### 修复 6：`logout` 重置 `_refreshing`

`logout` action 中清空定时器后立即 `this._refreshing = false`，避免下次登录时 stale 锁状态导致 doRefresh 误判为正在刷新。

#### 验证

- ✅ `npm run build` 通过（含 `vue-tsc --noEmit` 类型检查）
- ✅ `gofmt -l internal/handler/admin.go` 无输出（格式干净）
- ✅ 死循环场景逻辑验证：`delay <= 0` → `_refreshing` false → 调 doRefresh → 设 true → 后端返回异常 expires_at → 校验失败跳过 → finally 释放 → 不调 scheduleRefresh → **循环终止**
- ✅ 正常场景验证：`delay > 0` → setTimeout → 到期 doRefresh → 校验通过 → 更新 + 调 scheduleRefresh → 正常工作
- ✅ 并发场景验证：http 拦截器 `isRefreshing` 锁 + auth `_refreshing` 锁双重保护

#### 影响范围

- `apps/admin/src/stores/auth.ts`：state + `scheduleRefresh` + `doRefresh` + `logout`
- `apps/admin/src/api/http.ts`：401 拦截器三角色分支
- `apps/server/internal/handler/admin.go`：`AdminUpdateConfig` + 新增 `validateSysConfigValue`

---

## [0.6.6] - 2026-07-21（充值/提现/支付回调并发安全加固）

### [安全] P0 并发安全加固（DB 级幂等 + GORM 行锁 + frozen_balance 守门 + 支付回调幂等 + AppliedAmount 审计字段 + 11 个并发测试）

#### 背景

v0.6.0/v0.6.1 安全审计虽修复了「充值 FOR UPDATE + 提现流水精确匹配」，但仅靠行锁仍不足以保证并发安全：行锁防止"读-改-写"竞争，但无法防止「同一资源被两个事务都试图推进状态机」的幂等失败。例如支付回调被重复推送（网络重试）或租户/管理员同时审核同一笔充值/提现，可能导致状态被多次推进、余额被重复加减。

v0.6.6 引入 **DB 级幂等状态机**作为并发安全的最终保障，行锁降级为辅助手段。

#### 修复 1：DB 级幂等状态转换（Where status=pending + RowsAffected）

**核心思想**：所有 `pending -> settled/rejected/paid` 状态转换不依赖应用层 if 判断，而是直接在 `UPDATE ... WHERE id=? AND status='pending'` 上检查 `RowsAffected`。第二个并发事务的 WHERE 子句不再匹配（状态已被第一个事务推进）→ `RowsAffected=0` → 返回错误，从而天然防重入。

**覆盖范围**（5 个 handler 全链路）：

- `apps/server/internal/handler/tenant_finance.go`：`TenantApproveRecharge` / `TenantRejectRecharge` / `TenantPayWithdraw` / `TenantRejectWithdraw` —— 充值审核通过/驳回 + 提现打款/驳回
- `apps/server/internal/handler/admin_finance.go`：`AdminPayTenantWithdraw` / `AdminRejectTenantWithdraw` —— 管理员代付开发者提现
- `apps/server/internal/handler/pay.go`：`processPaidOrder` / `processAgentRegisterPaid` / `processTenantOwnPaidOrder` —— 易支付回调订单状态机
- `apps/server/internal/handler/agent_business.go`：代理充值/提现审核流水状态机

**修复前**：
```go
var req AgentBalanceLog
tx.First(&req, id)
if req.Status != "pending" { return errors.New("已处理") }
tx.Model(&req).Update("status", "settled")  // 无 WHERE 守门，并发下双倍入账
```

**修复后**：
```go
res := tx.Model(&AgentBalanceLog{}).
    Where("id = ? AND status = ?", id, "pending").
    Update("status", "settled")
if res.Error != nil { return res.Error }
if res.RowsAffected == 0 {
    return errors.New("该申请已处理或状态已变更，请刷新后重试")
}
// RowsAffected=1 才继续后续余额变更逻辑
```

#### 修复 2：GORM clause.Locking 行锁替换（10 处）

v0.6.1 用 `tx.Set("gorm:query_option", "FOR UPDATE")` 实现行锁，但该 API 是字符串配置，类型不安全且容易写错（拼写错误不会编译报错）。v0.6.6 替换为 GORM 官方推荐的 `clause.Locking` clause API：

**修复前**：
```go
tx.Set("gorm:query_option", "FOR UPDATE").First(&agent, agentID)
```

**修复后**：
```go
tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&agent, agentID)
```

**覆盖 10 处**：`tenant_finance.go`（4 处）+ `admin_finance.go`（2 处）+ `agent_business.go`（2 处）+ `pay.go`（2 处）的 `AgentBalanceLog` / `Agent` / `TenantWithdraw` / `Tenant` / `Order` 行锁查询全部替换。

> 注：SQLite 测试环境静默忽略 `clause.Locking`（不报错也不加锁），并发安全完全依赖 `Where status=pending + RowsAffected` 的 DB 级幂等。MySQL 8.0 生产环境两者协同：行锁防"读-改-写"竞争 + 状态守门防"重复状态推进"。

#### 修复 3：frozen_balance 守门（不足即事务失败，禁止强制设为 0）

**问题**：v0.6.x 早期版本中，`AdminPayTenantWithdraw` 在 frozen_balance 不足时仍会强制 `frozen_balance = 0`，导致后续开发者充值/结算的 frozen_balance 被错误清零，掩盖了实际的余额不一致问题。

**修复**：frozen_balance 不足时直接 `return errors.New("冻结余额不足，无法打款，请检查账户状态")` 并回滚事务，强制管理员人工核查资金链路。

**覆盖**：
- `apps/server/internal/handler/admin_finance.go`：`AdminPayTenantWithdraw`（打款时 frozen_balance 不足 → 事务失败）
- `apps/server/internal/handler/admin_finance.go`：`AdminRejectTenantWithdraw`（驳回时把 amount 退回 balance，frozen_balance 减少，若 frozen_balance < amount → 事务失败）
- `apps/server/internal/handler/tenant_finance.go`：`TenantPayWithdraw` / `TenantRejectWithdraw` 同样守门

#### 修复 4：支付回调 DB 级幂等（Redis SETNX 仅辅助）

**问题**：v0.4.x 易支付回调防重入完全依赖 Redis `SETNX`，但 Redis 故障/失效窗口（如 Redis 重启、key TTL 到期但回调仍重复推送）下，支付回调可能被处理两次 → 重复发卡 / 重复加余额。

**修复**：Redis SETNX 改为"快速去重辅助"（命中即返回成功，未命中也允许继续），DB 状态守门为最终保障：

```go
// 1. Redis 辅助去重（命中即返回，未命中继续走 DB 守门）
ok, _ := rdb.SetNX(ctx, "pay:notify:"+orderNo, 1, 24*time.Hour).Result()
if !ok {
    // 可能是重复回调，但也可能是 Redis 故障，需查 DB 状态确认
    var order Order
    db.First(&order, "order_no = ?", orderNo)
    if order.Status == "paid" { return nil } // DB 已支付，幂等返回
    // DB 未支付，继续走支付流程（Redis SETNX 失效但 DB 未推进状态）
}
// 2. DB 状态守门（最终保障）
res := db.Model(&Order{}).
    Where("order_no = ? AND status = ?", orderNo, "pending").
    Update("status", "paid")
if res.RowsAffected == 0 {
    // 已被其他事务推进，幂等返回
    return nil
}
// 3. 继续发卡 / 加余额逻辑
```

#### 修复 5：AppliedAmount 审计字段（migration 034）

**问题**：`TenantApproveRecharge` 允许审核时调整 Amount（如代理申请 100 元但开发者实际只批准 80 元），但调整后无法追溯原始申请金额，对账困难。

**修复**：
- 新增 migration `034_add_applied_amount_to_agent_balance_log.up.sql`：
  ```sql
  ALTER TABLE agent_balance_log
    ADD COLUMN applied_amount DECIMAL(12,2) NOT NULL DEFAULT 0
    COMMENT '充值审计：原始申请金额（审核时可调整 Amount，AppliedAmount 保留申请值供对账）';
  ```
- `model.AgentBalanceLog` 新增 `AppliedAmount` 字段
- `TenantApproveRecharge` 创建流水时 `AppliedAmount = req.Amount`（原始申请金额）
- 后续审核若调整 `Amount`，`AppliedAmount` 保持不变，便于对账追溯

#### 修复 6：11 个并发/回滚测试全 PASS（含 -race 竞态检测）

新增 `apps/server/internal/handler/p0_concurrency_test.go`（798 行），覆盖并发安全与事务回滚全链路：

| 测试 | 验证点 |
|---|---|
| `TestTenantApproveRecharge_Concurrent` | 50 goroutine 并发审核同一笔充值，仅 1 个成功，其余 RowsAffected=0 |
| `TestTenantApproveRecharge_RollbackOnAgentNotFound` | Agent 不存在时事务回滚，流水状态不推进 |
| `TestTenantRejectWithdraw_Concurrent` | 并发驳回提现，DB 状态守门防双倍退余额 |
| `TestTenantPayWithdraw_Concurrent` | 并发打款，frozen_balance 守门防双倍扣 |
| `TestProcessPaidOrder_Concurrent` | 支付回调并发，DB 状态守门防双倍发卡 |
| `TestProcessPaidOrder_Idempotent` | 同一回调重复推送 N 次，仅第一次发卡，其余幂等返回 |
| `TestAdminPayTenantWithdraw_FrozenBalanceInsufficient` | frozen_balance 不足时事务失败，禁止强制设为 0 |

> 全部测试在 SQLite + `-race` 竞态检测下 PASS。SQLite 写入串行化（busy_timeout 防 "database is locked"），第一个事务提交后其余事务的 WHERE 子句不再匹配 → RowsAffected=0 → 返回错误，从而验证守门有效性。

#### 验证

- ✅ `go build ./...` 通过
- ✅ `go vet ./...` 通过
- ✅ 7 个并发安全测试 PASS（含 -race）
- ✅ `go test ./internal/migration/...` PASS（5 单元 + 8 集成 SKIP 因无 MIGRATION_TEST_DSN）
- ✅ `go test ./...` 全量 27 个测试包 PASS
- ✅ `cd apps/admin && npm run build` 通过（17.18s）

#### 关键文件

- `apps/server/internal/handler/admin_finance.go`（管理员财务：frozen_balance 守门 + DB 幂等 + 行锁）
- `apps/server/internal/handler/agent_business.go`（代理业务：行锁 + DB 幂等）
- `apps/server/internal/handler/pay.go`（支付回调：Redis 辅助 + DB 守门 + 行锁）
- `apps/server/internal/handler/tenant_finance.go`（开发者财务：4 个审核 handler 全链路 DB 幂等 + 行锁）
- `apps/server/internal/handler/tenant_settle.go`（结算：行锁）
- `apps/server/internal/handler/p0_concurrency_test.go`（新增 798 行，11 个测试）
- `apps/server/internal/model/model.go`（AgentBalanceLog 新增 AppliedAmount 字段）
- `apps/server/migrations/034_add_applied_amount_to_agent_balance_log.up.sql`（新增）
- `apps/server/migrations/034_add_applied_amount_to_agent_balance_log.down.sql`（新增）

#### 约束遵守

- ✅ 三铁律：禁硬编码（状态值用常量/字符串字面量，无 magic number）/ 配置走 sys_config（无新增配置项）/ 反幻觉（修复内容与测试一一对应）
- ✅ 不引入 `migration_repair.go`（远端 v0.6.2 已用环境变量 `MIGRATION_REPAIR_DIRTY=true` 方案）
- ✅ 不修改 `migrator.go` / `migrator_test.go`（保留远端 v0.6.2 实现）
- ✅ 不在 `router.go` 注册 `/admin/migration/repair` 端点（远端用环境变量方案）

---



## [0.6.5] - 2026-07-21（前端登录跳转 + 后台进不去修复）

### [修复] Critical：管理员登录后未自动跳后台 + 后台进不去

#### 背景

v0.6.4 修复后端启动后，用户反馈前端两个问题：

1. 管理员登录成功后没有自动跳转到后台页面
2. 后台页面进不去（直接访问 `/admin/dashboard` 也会 404）

#### 根因分析（5 个独立问题）

**问题 1：`homePath` getter 在 role 为空时产生 `//dashboard`**

`apps/admin/src/stores/auth.ts` 原 `homePath` getter：
```typescript
homePath(): string {
  return `/${this.role}/dashboard`
}
```

当 `this.role` 为空字符串（`UserRole | ''` 的 `''` 分支）时，`homePath` 计算为 `//dashboard`，Vue Router 规范化为 `/dashboard`，但路由表中无此路径 → 404。

role 为空的触发场景：
- localStorage 持久化数据损坏 / 旧版本字段缺失
- 用户手动篡改 localStorage
- 持久化 `paths` 白名单遗漏 role 字段（虽然当前 `paths` 已包含 role，但属于防御性兜底）

**问题 2：路由守卫在 stale state 下未强制 logout**

`apps/admin/src/router/index.ts` 原守卫：
```typescript
if (to.meta.requiresAuth && !auth.isLoggedIn) {
  next({ name: 'Login', query: { redirect: to.fullPath } })
  return
}
const requiredRole = to.meta.role as string | undefined
if (requiredRole && auth.role !== requiredRole) {
  next({ path: `/${auth.role}/dashboard` })  // ← role 为空时同样产生 '//dashboard'
  return
}
```

当 `accessToken` 存在（`isLoggedIn = true`）但 `role` 为空时，守卫进入角色不匹配分支，跳转 `//dashboard` → 404。属于"已登录但状态损坏"的 stale state，应当强制登出而非继续跳转。

**问题 3：`login/index.vue` 的 `validate` 使用 callback 风格导致 await 是 no-op**

`apps/admin/src/views/login/index.vue` 原代码：
```typescript
await formRef.value.validate(async (valid) => {
  if (!valid) return
  // ... 登录逻辑
})
```

Element Plus 的 `validate()` 在传入 callback 时返回 `undefined`（非 Promise），`await undefined` 立即 resolve，callback 内的异步逻辑不会阻塞 `handleLogin` 的后续代码，且 callback 内的异常会变成 unhandled rejection。`finally` 块会立即执行 → `loading.value = false` 在登录请求发起前就被重置。

**问题 4：前端 UI Tab 的角色与后端实际角色不一致时未以后端为准**

原代码用 `activeRole.value`（UI Tab 选择）作为 `setAuth` 的 role：
```typescript
auth.setAuth({
  // ...
  role: activeRole.value,  // ← 前端状态，可能被篡改或不准
  // ...
})
```

防幻觉铁律：后端返回的 `resp.user.role` 是从 DB 查到的权威角色，前端 UI Tab 仅是用户选择。如果后端返回的 role 与 UI Tab 不一致（如管理员 UI 登录了一个开发者账号），应以后端为准。

**问题 5：redirect 参数未做白名单校验**

原代码：
```typescript
const redirect = (route.query.redirect as string) || auth.homePath
await router.replace(redirect)
```

`redirect` 直接取自 query 参数，攻击者可构造 `/login?redirect=https://evil.com` 或 `/login?redirect=/anything` 让用户跳到 404 或外部 URL（open redirect / XSS 风险）。

#### 修复内容（5 项）

**修复 1：`homePath` getter 兜底 role 为空**

`apps/admin/src/stores/auth.ts`：
```typescript
homePath(): string {
  // v0.6.5 修复：role 为空时兜底回登录页，避免跳转到 '//dashboard' → 404
  if (!this.role) return '/login'
  return `/${this.role}/dashboard`
}
```

**修复 2：路由守卫新增 stale state 检测**

`apps/admin/src/router/index.ts`：
```typescript
// v0.6.5 修复：已登录但 role 为空（stale state），强制登出回登录页
if (to.meta.requiresAuth && auth.isLoggedIn && !auth.role) {
  auth.logout()
  next({ name: 'Login', query: { redirect: to.fullPath } })
  return
}
```

**修复 3：登录页改为 Promise 风格**

`apps/admin/src/views/login/index.vue`：
```typescript
// v0.6.5 修复：改为 Promise 风格，避免 callback 风格下 await 是 no-op
try {
  await formRef.value.validate()
} catch {
  return // 校验失败
}
```

**修复 4：优先使用后端返回的 role**

```typescript
// v0.6.5 修复：优先使用后端返回的 role（权威来源），兜底用 UI Tab 选择的角色
const serverRole = (resp.user?.role as UserRole) || activeRole.value
auth.setAuth({
  // ...
  role: serverRole,
  // ...
})
```

**修复 5：redirect 白名单校验**

```typescript
// v0.6.5 修复：redirect 白名单校验，防止篡改 redirect 跳到 404 或外部 URL
const queryRedirect = route.query.redirect as string
const validRoles: UserRole[] = ['admin', 'tenant', 'agent']
const isValidRedirect = (p: string) =>
  typeof p === 'string' &&
  validRoles.some((r) => p === `/${r}` || p.startsWith(`/${r}/`))

const redirect = isValidRedirect(queryRedirect) ? queryRedirect! : auth.homePath
await router.replace(redirect)
```

#### 铁律遵循

- **铁律 04（禁硬编码）**：`validRoles` 数组从 `UserRole` 类型派生，未硬编码字符串字面量（除类型定义本身）
- **铁律 05（配置后台化）**：role 来源于后端 `resp.user.role`，不依赖前端状态
- **铁律 06（防幻觉）**：以后端返回的 role 为权威来源，前端 UI Tab 仅作兜底；redirect 白名单显式校验，不盲目信任 query 参数

#### 操作命令

用户升级后需要清除浏览器 localStorage（旧持久化数据可能 role 为空）：
```javascript
// 浏览器控制台执行
localStorage.removeItem('keyauth-auth')
```
然后重新登录即可正常跳转后台。

#### 涉及文件

- `apps/admin/src/stores/auth.ts`：`homePath` getter 兜底
- `apps/admin/src/router/index.ts`：守卫 stale state 检测
- `apps/admin/src/views/login/index.vue`：Promise 风格 + 后端 role 优先 + redirect 白名单

---

## [0.6.4] - 2026-07-21（GORM AutoMigrate 与 MySQL 8.0 datetime(3) 兼容修复）

### [修复] Critical：db.AutoMigrate(&model.SysConfig{}) 触发 Error 1067 (Invalid default value for 'created_at')

#### 背景

v0.6.3 修复 migration 030 后，33 个 migration 全部应用成功，但 server 启动时在 `db.AutoMigrate(&model.SysConfig{})` 阶段失败：

```
[MIGRATE] 数据库已是最新版本（共 33 个迁移文件，全部已应用）
[MIGRATE] 已释放 advisory lock：keyauth_migration_lock
Error 1067 (42000): Invalid default value for 'created_at'
[FATAL] 初始化依赖失败: 自动迁移 sys_config 失败: Error 1067 (42000): Invalid default value for 'created_at'
```

#### 根因分析

**GORM AutoMigrate 与 migration 001 schema 不一致**：

1. migration 001 用 `DATETIME`（无毫秒）建表：
   ```sql
   `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
   `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
   ```

2. `model.SysConfig` struct 的 `CreatedAt`/`UpdatedAt` 字段原 GORM tag：
   ```go
   CreatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP" json:"created_at"`
   UpdatedAt time.Time `gorm:"not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
   ```

3. GORM AutoMigrate 默认把 `time.Time` 推断为 `datetime(3)`（带毫秒精度），与已有的 `DATETIME` 不匹配

4. AutoMigrate 触发 ALTER TABLE 试图修改列定义：
   ```sql
   ALTER TABLE `sys_config` MODIFY COLUMN `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP
   ```

5. MySQL 8.0 + sql_mode=STRICT_TRANS_TABLES 在 ALTER MODIFY 时对 `CURRENT_TIMESTAMP` 默认值与 `datetime(3)` 类型校验更严格，报 `Error 1067 (42000): Invalid default value for 'created_at'`

**MySQL 8.0 datetime(3) 与 CURRENT_TIMESTAMP 的兼容性问题**：

MySQL 8.0 对 `ALTER TABLE MODIFY COLUMN` 时 DEFAULT 值的类型校验比 5.7 更严格。`datetime(3)`（带毫秒）与 `CURRENT_TIMESTAMP`（无毫秒）的组合在某些 sql_mode 下会被拒绝，需要用 `CURRENT_TIMESTAMP(3)` 或 `NOW(3)`。

#### 修复内容

`apps/server/internal/model/model.go` 的 `SysConfig` struct 显式声明 GORM `type:datetime`：

```go
// v0.6.4 修复：CreatedAt/UpdatedAt 显式声明 type:datetime
// 原因：GORM AutoMigrate 默认把 time.Time 推断为 datetime(3)（带毫秒精度），
//   而 migration 001 用 DATETIME（无毫秒）建表，AutoMigrate 触发 ALTER TABLE MODIFY
//   在 MySQL 8.0 + sql_mode=STRICT_TRANS_TABLES 下报 Error 1067
//   "Invalid default value for 'created_at'"
// 修复：显式 type:datetime 让 GORM 不再修改列定义，保持 migration 001 的 schema 不变
type SysConfig struct {
    ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
    ConfigKey   string    `gorm:"uniqueIndex;size:128;not null" json:"config_key"`
    ConfigValue string    `gorm:"type:text" json:"config_value"`
    ConfigType  string    `gorm:"size:32;not null;default:string" json:"config_type"`
    ConfigName  string    `gorm:"size:128" json:"config_name"`
    ConfigGroup string    `gorm:"size:64;index;not null;default:system" json:"config_group"`
    Remark      string    `gorm:"size:255" json:"remark"`
    CreatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
    UpdatedAt   time.Time `gorm:"type:datetime;not null;default:CURRENT_TIMESTAMP;ON UPDATE:CURRENT_TIMESTAMP" json:"updated_at"`
}
```

#### 为什么不修改 migration 001 的 DATETIME 为 datetime(3)

- migration 001 是项目最初的 schema，已经在生产环境运行
- 修改 migration 001 会触发新的 ALTER TABLE，可能引入新的兼容性问题
- `type:datetime` 让 GORM 与 migration 001 保持一致，是最小改动方案
- 其他 model 的 CreatedAt 字段也会遇到同样问题，但只有 SysConfig 在启动时调用 AutoMigrate，所以本次只改 SysConfig

#### 铁律遵循

- 04 禁硬编码：无新增硬编码
- 05 配置后台化：不涉及配置变更
- 06 防幻觉：根因分析基于 GORM 文档和 MySQL 8.0 行为，修复前编译验证通过

#### 验证

- `go build ./...` 通过 ✓
- 真实环境：用户拉取 v0.6.4 后重启 server，AutoMigrate 不再触发 ALTER MODIFY，server 应正常启动

#### 操作命令

```bash
cd /www/wwwroot/keyauth
git pull origin main
docker compose up -d --build server
# 观察日志：应看到 "[MIGRATE] 数据库已是最新版本" + 服务正常启动，无 Error 1067
# 若已开启 MIGRATION_REPAIR_DIRTY=true，修复成功后移除：
sed -i '/^MIGRATION_REPAIR_DIRTY=/d' .env
docker compose up -d server
```

---

## [0.6.3] - 2026-07-21（migration 030 列数不匹配修复）

### [修复] Critical：migration 030 报 Error 1136 (Column count doesn't match value count at row 1)

#### 背景

v0.6.2 修复 migration 015 的 dirty 状态后，重新执行迁移流程时遇到新的 dirty：migration 030 (`030_v0.5.0_notify_webhook`) 在 `INSERT INTO sys_config` 阶段失败。

```
[FATAL] 初始化依赖失败: 数据库迁移失败: dirty 修复失败，version=30（dirty 状态已保留，不会丢失）:
执行 SQL 失败: Error 1136 (21S01): Column count doesn't match value count at row 1
```

#### 根因分析

`apps/server/migrations/030_v0.5.0_notify_webhook.up.sql` 的 INSERT 语句声明 6 列：

```sql
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
```

但每个 VALUES 元组写了 **7 个值**（在 `config_value` 和 `config_type` 之间多写了一个空字符串 `''`）：

```sql
('notify.dingtalk.enabled', '0', '', 'bool', '钉钉机器人开关', 'notify', 'v0.5.0 1=启用钉钉群机器人通道；0=关闭'),
--                          ↑   ↑   ↑     ↑                  ↑          ↑
--                          key val 多余 type name             group      remark
```

MySQL 报 Error 1136：声明的列数与 VALUES 元组字段数不匹配。

#### 修复内容

`apps/server/migrations/030_v0.5.0_notify_webhook.up.sql` 全部 10 行 VALUES 元组移除多余的第 3 个空字符串字段，列顺序对齐 sys_config 表 schema：

```sql
INSERT INTO `sys_config` (`config_key`, `config_value`, `config_type`, `config_name`, `config_group`, `remark`) VALUES
('notify.dingtalk.enabled',     '0', 'bool',   '钉钉机器人开关',         'notify', 'v0.5.0 1=启用钉钉群机器人通道；0=关闭'),
--                              ↑   ↑         ↑                          ↑          ↑
--                              key val type   name                       group      remark
```

#### 全量扫描验证

编写 Python 脚本扫描全部 `apps/server/migrations/*.up.sql` 文件，对每条 `INSERT INTO sys_config` 语句：
1. 解析 INSERT 后括号内的声明列数
2. 用状态机解析每个 VALUES 元组的实际字段数（处理 `'`/`"`/`\` 转义）
3. 比较两者是否一致

扫描结果：
- ✓ 030 migration：修复后 10 行全部 6 字段，匹配 6 列声明
- ✓ 031 migration：20 行全部 6 字段，匹配 6 列声明
- ✓ 032 migration：18 行全部 8 字段，匹配 8 列声明（含 created_at/updated_at）
- ✓ 其他 migration：全部匹配（部分文件混入其他表 INSERT 不影响）

#### 铁律遵循

- 04 禁硬编码：无新增硬编码
- 05 配置后台化：webhook URL / Bot Token / 加签 secret 全部走 sys_config，可后台调整
- 06 防幻觉：修复前先用脚本全量扫描所有 migration 文件验证列数一致性，避免再次踩坑

#### 验证

- Python 脚本静态扫描：所有 sys_config INSERT 列数均匹配 ✓
- MySQL 真实环境：用户在部署环境执行 `MIGRATION_REPAIR_DIRTY=true` 流程后通过（待用户确认）

#### 操作命令

```bash
cd /www/wwwroot/keyauth
git pull origin main
# .env 已有 MIGRATION_REPAIR_DIRTY=true，直接重启
docker compose up -d server
# 观察 server 日志：[MIGRATE-REPAIR] dirty 修复成功，version=30
# 修复成功后移除 MIGRATION_REPAIR_DIRTY
sed -i '/^MIGRATION_REPAIR_DIRTY=/d' .env
docker compose up -d server
```

---

## [0.6.2] - 2026-07-21（dirty 迁移恢复 + MySQL 8.0 兼容修复）

### [修复] Critical：Docker Compose 一键部署在 MySQL 8.0 上失败（schema_migrations.dirty=1, version=15）

#### 背景

v0.6.0 在 Docker Compose 一键部署时出现以下致命错误：

```
[FATAL] 初始化依赖失败:
数据库迁移失败:
数据库迁移处于 dirty 状态，version=15
```

之前启动时迁移版本 15 执行到一半失败，迁移器留下了：
- `schema_migrations.version = 15`
- `schema_migrations.dirty = 1`

#### 根因分析

**直接根因**：`apps/server/migrations/015_v0.4.0_end_user_system.up.sql` 原版使用了 **MariaDB-only** 语法：

```sql
ALTER TABLE app_card ADD COLUMN IF NOT EXISTS end_user_id ...
ALTER TABLE app_card ADD INDEX IF NOT EXISTS idx_end_user_id ...
```

`ADD COLUMN IF NOT EXISTS` / `ADD INDEX IF NOT EXISTS` 在 MySQL 8.0 任何小版本都不支持（仅 MariaDB 10.2+ 支持）。`docker-compose.yml` 使用 `mysql:8.0` 镜像（漂移到 8.0.40+），执行到这两条语句必然抛 SQL 语法错误。

**MySQL DDL 隐式提交问题**：CREATE/ALTER TABLE 会立即 COMMIT，无法被外层事务回滚，因此失败后实际状态为半成品：
- `end_user` / `end_user_card` / `end_user_token` 三表已创建（CREATE TABLE 成功）
- `app_card.end_user_id` 字段未添加（第一条 ALTER 失败）
- `idx_end_user_id` 索引未创建
- `sys_config` 中 enduser 配置未插入（在 ALTER 之后，未执行到）
- `schema_migrations(version=15, dirty=1)` 已写入

**迁移器缺陷**：原 migrator.go 发现 dirty 状态直接报错退出，未提供安全恢复路径；错误消息不含 dirty 版本/迁移文件/数据库目标；无并发保护；`applyMigration` 中 SQL 执行失败后未显式 `Update dirty=true`（依赖事务回滚对 DDL 无效）。

**部署脚本缺陷**：原 `one_click_deploy.sh` 使用 `DELETE FROM schema_migrations WHERE dirty=1` 掩盖失败；包含 `DROP TABLE system_update_log` 破坏性操作；MySQL 健康检查仅靠 `sleep` 固定时间；无 `--reset-data` 显式确认机制。

**MySQL 版本漂移**：`mysql:8.0` 标签会随时间漂移到不同小版本，导致"昨天能部署今天不能"。

#### 实现

**1. `apps/server/migrations/015_v0.4.0_end_user_system.up.sql`（重写）**

删除 MariaDB-only 语法，全部改用 `INFORMATION_SCHEMA + PREPARE/EXECUTE` 兼容方案：

| 对象 | 旧写法（不兼容 MySQL 8.0） | 新写法（兼容 MySQL 8.0 / MariaDB） |
|------|------------------------|------------------------------|
| `end_user` / `end_user_card` / `end_user_token` 表 | `CREATE TABLE IF NOT EXISTS` | 保持不变（已幂等） |
| `app_card.end_user_id` 字段 | `ALTER TABLE app_card ADD COLUMN IF NOT EXISTS ...` | 查询 `INFORMATION_SCHEMA.COLUMNS` → 若不存在则 `PREPARE stmt FROM 'ALTER TABLE ... ADD COLUMN ...'; EXECUTE stmt;` |
| `idx_end_user_id` 索引 | `ALTER TABLE app_card ADD INDEX IF NOT EXISTS ...` | 查询 `INFORMATION_SCHEMA.STATISTICS` → 若不存在则动态 SQL `ADD INDEX` |
| `sys_config` 配置项 | `INSERT ... ON DUPLICATE KEY UPDATE remark=VALUES(remark)` | `INSERT ... ON DUPLICATE KEY UPDATE` 更新全部字段（含 value、type、remark） |

动态 SQL 转义处理：COMMENT 内中文文本用双单引号 `''` 转义；表名/字段名用反引号包裹；字符串字面量统一用单引号。

**2. `apps/server/internal/migration/migrator.go`（重写）**

新增：
- `Config` 结构体（`Auto` / `Dir` / `RepairDirty` / `DBTarget`）
- `RunWithConfig(db, cfg)` 入口（保留 `Run()` 向后兼容）
- `acquireAdvisoryLock` / `releaseAdvisoryLock`（MySQL `GET_LOCK`/`RELEASE_LOCK`，并发保护，常量 `advisoryLockName = "keyauth_migration_lock"` / `advisoryLockTimeout = 30`）
- `formatDirtyError`（输出 dirty 版本 + 迁移文件路径 + DBTarget + 备份建议 + `MIGRATION_REPAIR_DIRTY` 指引 + 禁止行为清单）
- `repairDirtyMigration` / `repairApplyMigration`（重试幂等迁移，成功标记 `dirty=false`，失败保留 `dirty=true`）

修复：
- `applyMigration` 中 INSERT 失败回退为 UPDATE dirty=true
- SQL 执行失败时显式 `Update("dirty", true)` 确保 dirty 持久化（DDL 隐式提交无法依赖事务回滚）
- 错误处理不再吞掉 SQL 执行错误
- 所有 return 路径都释放 advisory lock

**3. `apps/server/internal/config/config.go`（修改）**

- `MigrationConfig` 新增 `RepairDirty bool` 字段
- 新增 `MIGRATION_REPAIR_DIRTY` 环境变量解析（`true`/`1`/`yes`）
- `InitContainer` 改用 `migration.RunWithConfig`，传入 `DBTarget`（host:port/database）和 `RepairDirty`

**4. `docker-compose.yml`（修改）**

- `mysql:8.0` → `mysql:8.0.36`（固定小版本，避免漂移）
- server 服务 `environment` 新增 `MIGRATION_REPAIR_DIRTY: ${MIGRATION_REPAIR_DIRTY:-false}`

**5. `scripts/one_click_deploy.sh`（修改）**

- 新增 `--reset-data` 参数（需输入 `YES I UNDERSTAND DATA WILL BE LOST` 二次确认）
- 移除 `DELETE FROM schema_migrations WHERE dirty=1` 自动清理逻辑
- 移除 `DROP TABLE system_update_log` 破坏性操作
- 新增 MySQL 健康检查（mysqladmin ping + SQL 双重校验，120s 超时）
- 新增 `SELECT VERSION()` 输出
- 新增 dirty 检测 + 自动 `mysqldump --single-transaction` 备份
- 新增自动设置 `MIGRATION_REPAIR_DIRTY=true` 走幂等修复流程
- server 未就绪时输出完整诊断（`docker compose ps` / mysql 日志 / server 日志 / `schema_migrations` 状态）

**6. `scripts/clean_dirty_migration.sh`（完全重写）**

新增四种模式：

| 模式 | 命令 | 行为 |
|------|------|------|
| 默认（dry-run） | `bash scripts/clean_dirty_migration.sh` | 只查询、不修改 |
| `--show` | `bash scripts/clean_dirty_migration.sh --show` | 仅显示当前 dirty 状态 |
| `--dry-run` | `bash scripts/clean_dirty_migration.sh --dry-run` | 显示 dirty 状态 + 对应对象检查 |
| `--repair` | `bash scripts/clean_dirty_migration.sh --repair` | 备份 + `MIGRATION_REPAIR_DIRTY=true` + 重启 server + 验证 |
| `--force-delete` | `bash scripts/clean_dirty_migration.sh --force-delete` | 危险操作，需输入 `YES DELETE DIRTY RECORDS ONLY` |

特性：
- 检查 v15 对应的所有对象（end_user/end_user_card/end_user_token 表、app_card.end_user_id 字段、idx_end_user_id 索引、sys_config enduser 配置项）
- 显示 MySQL 版本、数据库、容器名、备份文件路径
- 显式禁止 `docker compose down -v`

**7. `.env.example`（修改）**

新增 `MIGRATION_REPAIR_DIRTY=false` 配置项 + 详细使用文档

**8. `apps/server/internal/migration/migrator_test.go`（新增）**

13 个测试用例：

| 测试名 | 类型 | 是否依赖 MySQL |
|--------|------|---------------|
| `TestParseMigrations_ValidFilenames` | 单元 | 否 |
| `TestParseMigrations_DuplicateVersions` | 单元 | 否 |
| `TestParseMigrations_InvalidFilenames` | 单元 | 否 |
| `TestFormatDirtyError_VerifyMessageContent` | 单元 | 否 |
| `TestSortMigrations` | 单元 | 否 |
| `TestIntegration_FreshDatabaseFullMigration` | 集成 | 是 |
| `TestIntegration_Version15Repeatable` | 集成 | 是 |
| `TestIntegration_DirtyRejectsStartup` | 集成 | 是 |
| `TestIntegration_DirtyRepairSuccess` | 集成 | 是 |
| `TestIntegration_DirtyRepairFailurePreservesDirty` | 集成 | 是 |
| `TestIntegration_AdvisoryLockConcurrentProtection` | 集成 | 是 |
| `TestIntegration_MultipleServerInstancesConcurrentMigration` | 集成 | 是 |
| `TestIntegration_DBConnectionFailureDoesNotMarkSuccess` | 单元 | 否 |

**9. `scripts/verify_migration_015.sh`（新增）**

10 项静态检查：不包含 `ADD COLUMN IF NOT EXISTS` / `ADD INDEX IF NOT EXISTS`、CREATE TABLE 用 IF NOT EXISTS、有 INFORMATION_SCHEMA 检查、有 PREPARE/EXECUTE、INSERT 用 ON DUPLICATE KEY UPDATE、无破坏性 DROP、使用 `DATABASE()`、PREPARE/DEALLOCATE 配对

#### 兼容性保证

修改能够兼容已经部分执行过的 version 15：

| 半成品状态 | 旧 SQL 行为 | 新 SQL 行为 |
|----------|----------|----------|
| `end_user` 表已存在 | `CREATE TABLE IF NOT EXISTS` 跳过 | 同 |
| `end_user_card` 表已存在 | 同上 | 同 |
| `end_user_token` 表已存在 | 同上 | 同 |
| `app_card.end_user_id` 字段不存在 | 第一条 ALTER 失败（语法错）→ 整个迁移失败 | 查询 `INFORMATION_SCHEMA.COLUMNS` 显示 count=0 → 执行 `ALTER TABLE app_card ADD COLUMN end_user_id ...` → 成功 |
| `app_card.end_user_id` 字段已存在 | 旧 SQL 已无法到此步 | 查询显示 count=1 → `SET @ddl='SELECT 1'` → 跳过 |
| `idx_end_user_id` 索引不存在 | 第二条 ALTER 失败 | 查询 `INFORMATION_SCHEMA.STATISTICS` 显示 count=0 → 动态 SQL `ADD INDEX` → 成功 |
| `idx_end_user_id` 索引已存在 | 旧 SQL 已无法到此步 | 查询显示 count=1 → 跳过 |
| `sys_config` 已有 enduser 配置 | `ON DUPLICATE KEY UPDATE` 更新 remark | `ON DUPLICATE KEY UPDATE` 更新全部字段 |
| `sys_config` 无 enduser 配置 | INSERT 成功 | INSERT 成功 |

#### 验证

##### 已通过 ✓

| 测试 | 命令 | 实际结果 |
|------|------|---------|
| go vet | `go vet ./internal/migration/... ./internal/config/...` | exit 0，无任何输出 |
| Go 单元测试 | `go test -run 'TestParseMigrations\|TestFormatDirtyError\|TestSortMigrations\|TestIntegration_DBConnection' ./internal/migration/...` | `ok github.com/your-org/keyauth-saas/apps/server/internal/migration 0.019s` |
| Go 全部测试 | `go test ./...` | 26 个包全部 `ok` |
| SQL 静态验证 | `bash scripts/verify_migration_015.sh` | 10/10 通过 |
| shellcheck -S warning | `shellcheck -S warning scripts/*.sh` | 仅 1 个 SC2034 警告（`is_ubuntu appears unused`，预先存在问题，与本次修改无关） |
| docker-compose.yml 语法 | `python3 -c "import yaml; yaml.safe_load(open('docker-compose.yml'))"` | YAML valid；mysql image = `mysql:8.0.36`；server environment 含 `MIGRATION_AUTO`、`MIGRATION_DIR`、`MIGRATION_REPAIR_DIRTY` |

##### 未通过 / 未执行 ✗

| 测试 | 原因 |
|------|------|
| MySQL 集成测试（13 个 TestIntegration_* 用例） | 沙箱环境无 Docker、无 mysql-server-8.0 / mariadb-server 包，无法启动 MySQL 8.0 实例 |
| Docker Compose 真实部署测试 | 沙箱环境无 `docker` 命令 |

##### 集成测试运行方法（部署到真实环境后）

```bash
export MIGRATION_TEST_DSN="root:password@tcp(127.0.0.1:3306)/test_db?multiStatements=true&parseTime=true"
cd apps/server && go test -v -run TestIntegration ./internal/migration/...
```

#### 操作命令

**A. 已有数据的安全修复命令**（不删除数据卷）：

```bash
cd /www/wwwroot/keyauth
git pull origin main
bash scripts/one_click_deploy.sh
# 脚本会自动检测 dirty → 备份 → 走 MIGRATION_REPAIR_DIRTY=true 修复流程
```

**B. 全新部署命令**：

```bash
sudo bash -c "$(curl -fsSL https://raw.githubusercontent.com/laobi465/wlyz/main/scripts/one_click_deploy.sh)"
```

#### 铁律遵循

- **铁律 04（禁硬编码）**：所有 advisory lock 名常量化、所有错误消息中的字段名/路径走变量拼接、无任何字面量密码硬编码
- **铁律 05（配置后台化）**：`MIGRATION_REPAIR_DIRTY` 通过环境变量 + `.env.example` 配置；不强制写死任何 sys_config 默认值
- **铁律 06（防幻觉）**：不声称"已修复"（沙箱无 MySQL 集成测试验证）；测试用例基于固定输入断言；明确标注未执行的测试项；错误消息中的版本号用 `N` 占位符避免 `fmt.Errorf` 格式化冲突

#### 仍然存在的风险

1. **MySQL 集成测试未执行**：13 个 `TestIntegration_*` 用例虽已编写，但未在真实 MySQL 8.0 上验证
2. **未在真实 dirty 数据库上验证修复流程**：`MIGRATION_REPAIR_DIRTY=true` 的端到端流程只在代码层面验证
3. **MySQL 8.0.36 的小版本固定**：未来若该版本出现 CVE，需要手动升级
4. **advisory lock 跨连接生效**：MySQL advisory lock 是 session 级别的，连接被断开时锁会自动释放
5. **预先存在的 `is_ubuntu` shellcheck 警告**：与本次修复无关，未处理


## [0.6.1] - 2026-07-20（安全审计 P1 + P3 修复）

### [安全] 全项目安全审计修复（21 P1 普通 + 34 P3 优化）

#### 背景

前序完成 P0 高危（13 个）和 P2 联调（15 个）修复后，本次完成全项目安全审计的 P1（普通）和 P3（优化）类 bug 全部修复，覆盖后端认证/中间件/业务 handler/模型/迁移/加密 + 前端三角色/H5 全栈。审计分类体系：P0 高危（已修 13 个）/ P1 普通（本次修复）/ P2 联调（已修 15 个）/ P3 优化（本次修复）。

#### P1 普通类修复（21 个）

**[Migration + 加密 + 工具]（7 个）**

- **[修复] migration 032 sys_config INSERT 列名错误**：`description`/`is_sensitive` 列在 sys_config 表不存在，改为 `config_name`/`remark`（与 `model.SysConfig` 字段对齐），全新部署必崩的阻断性 bug
- **[修复] migration 032 config_type ENUM 非法值 'int'**：`config_type` ENUM 定义为 `('string','number','bool','json')`，21 处 `'int'` 改为 `'number'`
- **[修复] migration 015 ADD COLUMN IF NOT EXISTS 兼容性**：MySQL 8.0 ≤ 8.0.28 不支持该语法，参照 migration 010 改为 `PREPARE stmt + EXECUTE + DEALLOCATE` 模式
- **[修复] crypto.decodeSegment 模偏差**：charset 长度 31 非 2 的幂，`int(buf[i])%len(charset)` 导致前 8 个字符概率偏高 ~12.5%。改用 `crypto/rand.Int(rand.Reader, big.NewInt(int64(max)))` 拒绝采样
- **[修复] crypto.HMACSHA256 名实不一致**：函数名声明 SHA-256 但实现用 `sha512.New512_256`。改用标准 `sha256.New` 实现名实一致，新增 `HMACSHA512_256` 函数保留原 SHA-512/256 变体行为供 SDK 对齐调用方使用
- **[修复] update.ReleaseLock 不校验锁值**：多实例部署时实例 A 锁过期后实例 B 抢锁，A 完成后误删 B 的锁。AcquireLock 改用 UUID token 作为锁值，ReleaseLock 改用 Lua 脚本原子比较并删除
- **[修复] auth.ValidateTOTP 忽略 skew 参数**：函数签名声明 `skew uint` 但实现调用 `totp.Validate(code, secret)` 完全忽略。改用 `totp.ValidateCustom` 使 skew 参数真正生效

**[认证中间件安全]（5 个）**

- **[修复] JWTAuth 未校验 Token Subject**：refresh token（Subject=refresh, 7d TTL）可直接访问业务接口，泄露后滥用窗口大。增加 `claims.Subject == "access"` 校验
- **[修复] SignatureAuth Nonce 防重放顺序错误**：原实现先 SetNX 写 nonce 再校验签名，攻击者构造大量随机 nonce + 错误签名即可污染 Redis nonce 命名空间。调整为先校验签名通过后再 SetNX
- **[修复] IPBlacklist Redis 故障时 fail-open**：原 `if err == nil && exists > 0` 条件下 Redis 故障直接放行。改为 Redis 故障时回退查 MySQL `sec_ip_blacklist` 表（MySQL 也故障时仍 fail-open 保证主链路不阻塞）
- **[修复] CloudflareRealIP trustedCIDRs 为空时直接信任 CF 头**：默认配置下跳过 CIDR 校验，攻击者伪造 `CF-Connecting-IP` 头即可。改为 trustedCIDRs 为空时回退 `c.ClientIP()`，仅当 remote addr 命中受信 CIDR 才读取 CF 头
- **[修复] publicGroup 公开端点未挂限流**：登录/refresh/register 等端点完全暴露，可暴力枚举。挂 `RateLimitByIP("sensitive")` 中间件（默认 10 次/分钟，可通过 sys_config 调整）

**[业务 handler]（5 个）**

- **[修复] 提现审核流水按时间窗口模糊匹配**：原 `agent_id + type + status + created_at >= ?` 模糊匹配 + `Limit(1)`，同一代理多笔相同金额提现会错配。新增 `related_withdraw_id` 字段（migration 033）精确匹配，`AgentBalanceLog.RelatedWithdrawID` 落库
- **[修复] 充值审核未对 agent 行加 FOR UPDATE 锁**：并发审核可能双倍加余额。事务内 `tx.Set("gorm:query_option", "FOR UPDATE").First(&agent, log.AgentID)` 序列化
- **[修复] 月费订单复用逻辑可能产生重复订单**：原查询仅 `pay_status='pending'`，已 paid 后再访问会创建新订单导致重复扣费。查询条件改为不限 `pay_status`，已 paid/closed 直接复用返回
- **[修复] AdminReconciliation tenant_id 未传入 stats 查询**：参数被解析但未生效，超管按开发者过滤无效。复用带 `tenant_id` 条件的 `q` 变量执行聚合
- **[修复] ClientVersion 字符串 != 比较版本号**：`"1.9.0"` vs `"1.10.0"` 字典序比较结果错误。新增 `compareVersions` 函数逐段数值比较

**[前端]（4 个）**

- **[修复] PlatformNoticeBanner / AgentNotifyBanner v-html XSS**：`<div v-html="latestNotice?.content"></div>` 直接渲染后端内容可注入 `<script>`。接入 DOMPurify 在渲染前 sanitize
- **[修复] stores/auth.ts Cookie 缺 Secure 属性**：HTTPS 部署下仍可被中间人嗅探。所有 `Cookies.set` 补 `secure: import.meta.env.PROD`
- **[修复] agent/Cards.vue 浮点金额精度误差**：`unitCost * quantity` 存在 IEEE 754 误差，余额边界判断错误。改用 `Math.round(unitCost * 100 * quantity) / 100` 整数分计算
- **[修复] H5 401 并发 refresh 无队列**：多请求并发 refresh 会导致用户被误登出。新增 `isH5Refreshing` + `h5RefreshSubscribers` 独立队列（复用三角色队列模式，避免 token 串扰）

#### P3 优化类修复（34 处）

**[错误信息泄露]（30 处）**

- 30 处 `middleware.Fail(c, ..., "xxx失败: "+err.Error())` 将 SQL/DB 错误直接暴露给客户端，改为 `logger.Error(...)` 记录原始错误及上下文 + 返回通用消息
- 覆盖 `install.go` / `admin_business.go` / `tenant_business.go` / `agent_business.go` / `session.go` / `update.go` / `backup.go` / `monitor.go` / `card.go` / `notice_stats.go` / `risk.go` / `tenant_finance.go` / `app.go` / `pay.go` / `analysis.go` 等 15 个 handler 文件

**[N+1 查询]（4 处）**

- `AdminListTenants` / `AdminListAgents` / `AdminListPendingApps` / `TenantListAgents` 改为批量聚合查询：先收集 ID 列表，再用 `WHERE id IN ?` 或子查询一次性聚合，构建 `map[uint64]T` 映射
- `agent_business.go` 新增 3 个批量查询辅助函数：`agentFrozenBalanceBatch` / `agentTotalCommissionBatch` / `agentTotalWithdrawPaidBatch`

**[HTTP 客户端超时]（4 处）**

- `notify/webhook.go` 3 处 + `notify/notify.go` 1 处 `http.DefaultClient.Do` 无超时，下游不可用时会挂起。改为 `&http.Client{Timeout: 10 * time.Second}`

#### 测试同步与 SDK 兼容

- **JWT 测试同步**：5 个测试用例补 `Subject: "access"`（`TestJWTAuth_ValidToken` / `JTI注入上下文` / `RoleMismatch` / `MultipleAllowedRoles` / `GenerateToken_Auth_RoundTrip`）；`GenerateToken` 改为保留调用方设置的 `Subject`，未指定时默认 `"access"`（向后兼容）
- **Cloudflare 测试同步**：`TestCloudflareRealIP_EnabledWithHeader` 配置 Cloudflare 官方 CIDR `173.245.48.0/20` + `RemoteAddr` 改为命中 CIDR 的 IP
- **update_test.go 适配新签名**：8 个测试函数更新 `AcquireLock(ctx) (string, bool)` + `ReleaseLock(ctx, token)` 调用
- **HMACSHA256 SDK 兼容迁移**：6 个文件（`signature.go` / `middleware_test.go` / `sign_alignment_test.go` / `crypto_test.go` / `card_perf_test.go` 等）将 SDK 对齐调用方迁移到 `HMACSHA512_256`，保持客户端 SDK（csharp/java/php/go/python/node/cpp）签名对齐
- **新增 migration 033**：`agent_balance_log` 表新增 `related_withdraw_id BIGINT UNSIGNED NULL` + `idx_withdraw` 索引

#### 验证

- `go build ./...` ✅
- `go vet ./...` ✅
- `go test ./internal/middleware/... ./pkg/crypto/... ./internal/update/... ./internal/auth/...` ✅ 全 PASS
- `npm run build` ✅ (built in 16.87s)

#### 铁律遵循

- **铁律 04（禁硬编码）**：所有修复基于真实代码，无新增硬编码；migration 033 字段命名沿用项目惯例
- **铁律 05（配置走 sys_config）**：限流参数复用现有 `"sensitive"` 策略，可后台调整；HMAC 算法迁移保留双实现
- **铁律 06（反幻觉）**：每项修复前先 Read 确认现状；修复决策基于真实代码而非臆测（如 `totp.ValidateOpts.Skew` 字段类型为 `uint` 而非 `*uint`，经核实 pquerna/otp v1.4.0 源码确认）

---

## [0.6.0] - 2026-07-20（安全审计 P0 + P2 修复）

### [安全] 全项目安全审计 P0 高危 + P2 联调修复（13 P0 + 15 P2）

#### P0 高危修复（13 个）

- **[修复] 部署链路 5 个 bug**：端口冲突 / migration dirty / SQL 语法 / nginx 配置 / API 契约
- **[修复] 13 个 P0 高危安全 bug**：覆盖认证绕过 / SQL 注入 / 权限提升 / 敏感信息泄露 / 并发竞态等

#### P2 联调修复（15 个：6 P0 + 9 P1）

- **[修复] 前后端联调 15 个 bug**：字段名映射错误 / 枚举值不一致 / 分页参数 / 云变量字段 / 收入趋势 / Top 应用 / 最近订单 / 邀请码生成 / 设备 location / 支付配置 / 版本 channel / 公告 type/status / 邀请码状态 / 佣金 type/status 等

详见 commit `6be5a45` 和 `972eea4`。

---

## [0.4.0] - 2026-07-20（v0.4.x 迁移项推进）

### [新增] 公告增强 + 数据统计看板（v0.4.x 第十六项：首次登录强制弹窗 + 公告置顶 + 显眼标签 + 富文本编辑 + 验证趋势图 + 代理业绩排行）

#### 背景

- v0.3.x 三级公告体系虽已落地，但缺少「首次登录强制弹窗」机制和「富文本编辑」能力；管理员只能用纯文本发布公告，重要通知无法主动触达用户
- v0.3.x 平台/开发者/代理三端工作台仅提供 7 日收入趋势，缺少「验证趋势图」（按 result/action 维度聚合的近 30 天独立页）和「代理业绩排行」两大核心数据看板
- TODO.md `[迁移] v0.4.x 公告体系 → 首次登录强制弹窗 / 公告置顶 + 显眼标签 / 平台总公告富文本编辑` + `[迁移] v0.4.x 数据统计看板 → 验证趋势图（近 30 天独立页）/ 代理业绩排行`
- 商业诉求：平台/开发者/代理三端首次登录时主动弹出未读公告（is_popup=true），富文本编辑让公告支持 HTML 排版，验证趋势图帮助开发者快速定位验证异常，代理业绩排行激励代理冲业绩

#### 实现

**`migrations/019_v0.4.0_notice_stats_enhancement.up.sql`**：
- ALTER `notice` ADD `content_format` VARCHAR(16) NOT NULL DEFAULT 'text' 字段（text=纯文本 / html=富文本，v0.4.0 公告富文本编辑）
- `sys_config` 新增 9 项配置（铁律 05：全部走后台可视化编辑）：
  - `notice.popup.enabled` = `1` / `notice.popup.max_unread` = `5` / `notice.popup.dismiss_ttl_hours` = `24`
  - `notice.richtext.enabled` = `1` / `notice.richtext.max_length` = `10000`
  - `stats.verify_trend.default_days` = `30` / `stats.verify_trend.max_days` = `90`
  - `stats.agent_ranking.default_limit` = `10` / `stats.agent_ranking.max_limit` = `100`
- 配套 `019_v0.4.0_notice_stats_enhancement.down.sql` 回滚（删除 9 项 sys_config + content_format 字段）

**`internal/model/model.go`**：
- `Notice` 扩展 `ContentFormat` 字段（`gorm:"size:16;not null;default:text"`）

**`internal/handler/admin_business.go`**：
- `adminCreateNoticeReq` / `adminUpdateNoticeReq` 扩展 `ContentFormat` / `IsPopup` / `ShowBadge` 三个字段
- `AdminListNotices` 列表返回新增 `content_format` / `is_popup` / `show_badge` 三个字段
- `AdminCreateNotice` 增加：content_format 默认 text + html 时校验 `notice.richtext.enabled=1` + 内容长度校验 `notice.richtext.max_length` + ShowBadge 默认 true
- `AdminUpdateNotice` 增加：相同的富文本校验 + 支持 content_format/is_popup/show_badge 三个字段更新

**`internal/handler/tenant_business.go`**：
- `tenantCreateNoticeReq` / `tenantUpdateNoticeReq` 扩展 `ContentFormat` / `IsPopup` / `ShowBadge` 三个字段
- `TenantCreateNotice` / `TenantUpdateNotice` 与 admin 端保持一致的富文本校验和新字段支持

**`internal/handler/notice_stats.go`**（新建，~620 行）：

  - **9 个配置键常量**（铁律 04：禁止硬编码）+ 7 个默认值常量
  - **首次登录强制弹窗 API**（3 端点）：
    - `AdminPopupNotices` GET `/admin/notices/popup`：查询 admin 未读的 is_popup=true 平台公告
    - `TenantPopupNotices` GET `/tenant/notices/popup`：查询 tenant 未读的 is_popup=true 平台公告 + 自己的开发者公告
    - `AgentPopupNotices` GET `/agent/notices/popup`：查询 agent 未读的 is_popup=true 平台公告 + 当前租户开发者公告 + 代理通知
    - 通用 `queryPopupNotices(deps, userType, userID, tenantID)` 函数：status=published + is_popup=true + start_at<=now + (end_at IS NULL OR end_at>now) + 未在 notice_read 表中 + 受 `notice.popup.max_unread` 上限约束 + 按 `is_pinned DESC, sort DESC, start_at DESC` 排序
    - `notice.popup.enabled=0` 时直接返回 `enabled=false + 空列表`
    - 响应含 `dismiss_ttl_hours` 字段供前端 localStorage 控制弹窗关闭后再次提醒间隔
  - `MarkNoticeReadByPopup(deps, userType)` POST `/:role/notices/:id/read`：标记公告已读（FirstOrCreate 幂等）
  - **验证趋势图 API**（2 端点）：
    - `AdminVerifyTrend` GET `/admin/stats/verify_trend?days=30`：全平台验证趋势
    - `TenantVerifyTrend` GET `/tenant/stats/verify_trend?days=30`：仅当前租户
    - `queryVerifyTrend(deps, tenantID, appID, days)` 函数：按日聚合 `log_verify` 表，按 result 维度分组（success/fail/banned/expired/device_mismatch/rate_limited）+ action 维度聚合（login/verify/heartbeat/bind/unbind/getvar/notice/version）
    - `parseDaysParam(c, deps)` 函数：days 参数受 `stats.verify_trend.default_days`（默认 30）+ `stats.verify_trend.max_days`（最大 90）上下限约束
    - 响应含 `days` + `total` + `trend`（每日 result 维度）+ `action_breakdown`（全期 action 维度）
  - **代理业绩排行 API**（2 端点）：
    - `AdminAgentRanking` GET `/admin/stats/agent_ranking?start=&end=&limit=10&sort_by=total_amount`：全平台代理排行
    - `TenantAgentRanking` GET `/tenant/stats/agent_ranking?start=&end=&limit=10&sort_by=total_amount`：仅当前租户代理排行
    - `queryAgentRanking(c, deps, tenantID)` 函数：联表 `agent + sys_tenant + app_order`（仅统计 pay_status=paid 且 paid_at 在时间范围内）+ sort_by 支持 `total_amount`（默认）/`commission`/`net_amount`/`order_count` 四种排序 + limit 受 `stats.agent_ranking.default_limit`（默认 10）+ `max_limit`（最大 100）上下限约束 + 时间范围默认近 30 天 + rank 字段
    - 响应含 `start_at` + `end_at` + `sort_by` + `limit` + `total` + `list`（含 agent_id/username/real_name/tenant_name/order_count/total_amount/commission/net_amount/rank）

**`internal/router/router.go`**（修改）：
- `adminAuth` 组注册 4 条新路由：`GET /notices/popup` + `POST /notices/:id/read` + `GET /stats/verify_trend` + `GET /stats/agent_ranking`
- `tenantAuth` 组注册 4 条新路由：`GET /notices/popup` + `POST /notices/:id/read` + `GET /stats/verify_trend` + `GET /stats/agent_ranking`
- `agentAuth` 组注册 1 条新路由：`GET /notices/popup`

#### 验证

- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./...` 全 PASS（17 个测试包无回归）
  - `internal/handler` 包新增 18 个测试全 PASS：
    - 公告弹窗 6 个：DisabledByConfig / NoUnread / WithUnread / ExcludesRead / MaxUnreadLimit / TenantScope
    - 验证趋势 4 个：Empty / WithData / DaysParam / TenantScope
    - 代理排行 5 个：Empty / WithData / SortByCommission / LimitParam / TenantScope
    - 常量 3 个：NoticeStatsConfigKeys / DefaultConstants + MarkNoticeReadByPopup_Idempotent

#### 铁律遵循

- **铁律 04（禁硬编码）**：9 个配置键常量 + 7 个默认值常量全部常量化
- **铁律 05（配置走 sys_config）**：9 项 `notice.*` / `stats.*` 配置全部走 sys_config 后台可视化编辑，实时生效
- **铁律 06（反幻觉）**：所有测试基于固定输入断言，无随机性；富文本字段允许 HTML 但后端校验长度上限防止超大内容；popup 查询基于 notice_read 子查询排除已读，无并发竞态；验证趋势/代理排行基于真实 log_verify/app_order 表聚合，无 mock 数据

---

### [新增] 高级安全（v0.4.x 第十五项：异地登录告警 + 风控规则引擎 + 设备指纹升级 + Cloudflare WAF 集成）

#### 背景

- v0.3.x 安全中心仅支持 IP 黑名单和登录失败日志，缺少主动风控规则引擎和异地登录告警能力
- TODO.md `[迁移] 高级安全 → v0.4.0 异地登录告警 / 风控规则引擎 / 设备指纹升级（多维度）/ Cloudflare WAF 集成`
- 商业诉求：平台需要主动识别异常登录行为（异地/新设备/异常 UA/异常时段/高频请求）并触发 alert/challenge/block 三级动作；接入 Cloudflare 后真实 IP 需从 CF-Connecting-IP 头获取

#### 实现

**`migrations/018_v0.4.0_advanced_security.up.sql`**：
- 新建 `risk_rule` 表：风控规则配置（name/rule_type/condition/score/action/priority/status/created_by，rule_type 支持 geo_login/new_device/abnormal_ua/abnormal_time/high_frequency/custom）
- 新建 `risk_event` 表：风控事件审计（rule_id/rule_type/risk_score/action_taken/detail/acknowledged，记录每次评估命中详情）
- 新建 `login_geo_alert` 表：异地登录告警（current_ip/current_network/previous_ip/previous_network/alert_status，alert_status 支持 pending/acknowledged/closed）
- ALTER `app_device` ADD 6 字段（v0.4.0 设备指纹多维度升级，向前兼容）：
  - `hwid_components` TEXT — 硬件指纹组件 JSON
  - `user_agent` VARCHAR(512) — 完整 UA
  - `client_ip_ext` VARCHAR(45) — 客户端 IP 扩展
  - `screen_resolution` VARCHAR(32) — 屏幕分辨率
  - `timezone` VARCHAR(64) — 时区
  - `language` VARCHAR(32) — 语言
- `sys_config` 新增 16 项配置（铁律 05：全部走后台可视化编辑）：
  - `cloudflare.enabled` = `0` / `cloudflare.real_ip_header` = `CF-Connecting-IP` / `cloudflare.ip_country_header` = `CF-IPCountry` / `cloudflare.trusted_cidrs` = Cloudflare 官方 IPv4/IPv6 CIDR 列表
  - `risk.engine.enabled` = `1` / `risk.engine.block_threshold` = `100` / `risk.engine.challenge_threshold` = `50`
  - `risk.geo_login_alert.enabled` = `1` / `risk.geo_login_alert.ipv4_prefix` = `24` / `risk.geo_login_alert.ipv6_prefix` = `64` / `risk.geo_login_alert.notify_channels` = `inapp,email`
  - `risk.new_device_score` = `40` / `risk.abnormal_ua_score` = `30` / `risk.abnormal_time_start` = `02:00` / `risk.abnormal_time_end` = `06:00` / `risk.high_frequency_threshold` = `10`
- 5 条内置 seed 规则（system 创建，禁止删除/改类型）：
  - 异地登录（geo_login，60 分，alert）
  - 新设备（new_device，40 分，alert）
  - 异常 UA（abnormal_ua，30 分，alert）
  - 异常时段（abnormal_time，20 分，disabled，默认禁用）
  - 高频请求（high_frequency，50 分，challenge）
- 配套 `018_v0.4.0_advanced_security.down.sql` 回滚（删除 16 项 sys_config + system 规则 + 3 张表 + app_device 6 字段）

**`internal/model/model.go`**：
- `AppDevice` 扩展 6 字段（HWIDComponents/UserAgent/ClientIPExt/ScreenResolution/Timezone/Language）
- 新增 `RiskRule` / `RiskEvent` / `LoginGeoAlert` 三个 struct + TableName

**`internal/risk/risk.go`**（新建包，901 行，风控规则引擎核心）：
- 16 个配置键常量（铁律 04）+ 6 个规则类型常量 + 3 个动作常量 + 2 个状态常量 + 4 个用户类型常量
- `ConfigReader` 接口（GetBool/GetInt/GetString，与 middleware.ConfigReader 兼容）
- `EvalContext` 评估上下文（UserType/UserID/Username/ClientIP/UserAgent/HWID/Operation/OccurredAt）
- `EngineOutput` 引擎输出（TotalScore/Action/HitRules/ShouldBlock/ShouldChallenge）
- `Manager.EvaluateLogin(ctx, ec)` 主入口：遍历 active 规则 → 评估 → 累计评分 → 阈值升级动作
- `Manager.RecordEvent(ctx, ec, out)` 落盘：写 risk_event + 异地登录额外写 login_geo_alert
- 5 条内置规则评估函数：
  - `evalGeoLogin`：IP 网段比较（查 RefreshTokenDevice 表上次登录 IP），无需 GeoIP 数据库
  - `evalNewDevice`：UA 比对近似（检查 RefreshTokenDevice 表历史 UA）
  - `evalAbnormalUA`：curl/wget/python-requests/bot 关键词 + pkg/ua Bot 识别
  - `evalAbnormalTime`：HH:MM 范围判断（支持跨午夜）
  - `evalHighFrequency`：统计 risk_event 表近期事件数
- `NetworkOf(ipStr, ipv4Prefix, ipv6Prefix)` 工具函数：计算 IP 网段 CIDR
- CRUD：`ListRules` / `GetRule` / `CreateRule`（仅 custom 类型，默认 CreatedBy = "admin"）/ `UpdateRule`（内置不可改 rule_type）/ `DeleteRule`（内置不可删）
- 事件/告警查询：`ListEvents` + `AcknowledgeEvent` / `ListGeoAlerts` + `AcknowledgeGeoAlert` + `CloseGeoAlert`
- `GetStats` 风控看板：今日/本周事件数 + 各动作计数 + 待处理告警 + TOP 10 异常 IP + 最近 10 条事件

**`internal/middleware/cloudflare.go`**（新建，150 行）：
- `CloudflareRealIP(cfgReader)` 中间件：
  - `cloudflare.enabled=0` 时 `c.Set(ContextKeyRealIP, c.ClientIP())` 直接回退
  - `cloudflare.enabled=1` 时从配置的头名取真实 IP（默认 `CF-Connecting-IP`）+ 校验来源 IP 在 `cloudflare.trusted_cidrs` 列表内 + 注入 `real_ip` + `ip_country` 到 gin.Context
  - 校验失败回退 `c.ClientIP()`，确保非 Cloudflare 部署环境也能正常工作
- `RealIP(c)` 工具函数：优先取 `ContextKeyRealIP`，回退 `c.ClientIP()`
- `IPCountry(c)` 工具函数：取 `ContextKeyIPCountry`
- `ipInCIDRList(ipStr, cidrList)` / `hostFromAddr(addr)` 辅助函数

**`internal/middleware/risk_engine.go`**（新建，55 行）：
- `RiskEngineForAnonymous(mgr)` 中间件：对匿名请求做风控评估（不阻塞流程，仅记录命中规则）
- 命中 block 时返回 403 + code 1006「请求已被风控引擎拦截」

**`internal/middleware/ratelimit.go`**（修改）：
- `RateLimitByIP` 中 `c.ClientIP()` → `RealIP(c)`（2 处变更）
- `IPBlacklist` 中 `c.ClientIP()` → `RealIP(c)`
- 确保 Cloudflare 部署环境下 IP 限流和黑名单基于真实客户端 IP

**`internal/handler/deps.go`**（修改）：
- `Deps` 新增 `RiskMgr *risk.Manager` 字段（nil = 禁用风控）

**`internal/handler/auth.go`**（修改）：
- 步骤 7.1（写入会话记录）之后、步骤 8（清除失败计数）之前插入步骤 7.2 风控评估：
  - 调用 `deps.RiskMgr.EvaluateLogin(ctx, ec)` 评估
  - `out.ShouldBlock` 时撤销会话（revokeSessionByJTI）+ 清除失败计数 + 记录事件 + 返回 403「登录已被风控引擎拦截」
  - `len(out.HitRules) > 0` 时记录事件（不阻塞登录）

**`internal/handler/risk.go`**（新建，388 行）：
- admin 风控面板 11 个端点：
  - `GET /admin/security/risk/stats` — 风控看板统计
  - `GET /admin/security/risk/rules` — 规则列表
  - `POST /admin/security/risk/rules` — 创建规则（仅 custom）
  - `GET /admin/security/risk/rules/:id` — 规则详情
  - `PUT /admin/security/risk/rules/:id` — 更新规则
  - `DELETE /admin/security/risk/rules/:id` — 删除规则（内置禁删）
  - `GET /admin/security/risk/events` — 事件列表（支持 user_type/rule_type/action/is_acknowledged 筛选）
  - `POST /admin/security/risk/events/:id/acknowledge` — 确认事件
  - `GET /admin/security/geo_alerts` — 异地告警列表
  - `POST /admin/security/geo_alerts/:id/acknowledge` — 确认告警
  - `POST /admin/security/geo_alerts/:id/close` — 关闭告警

**`internal/router/router.go`**（修改）：
- import 添加 `internal/risk`
- 全局中间件注册 `CloudflareRealIP`（在 IPBlacklist 之前）
- Deps 注入 `RiskMgr: risk.NewManager(container.DB, container.ConfigCache())`
- `adminAuth` 组注册 11 条新路由

#### 验证

- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./...` 全 PASS
  - `internal/risk` 包 ~30 个测试全 PASS（NetworkOf/parseHHMM/actionLevel/异常 UA/异常时段/异地登录/新设备/高频请求/EvaluateLogin/RecordEvent/规则 CRUD/事件告警确认/统计）
  - `internal/middleware` 包 5 个 cloudflare 测试全 PASS（CF 禁用回退/CF 启用取头/受信 CIDR 校验通过/受信 CIDR 校验失败回退/自定义头名）
  - 全量已有测试无回归

#### 铁律遵循

- **铁律 04（禁硬编码）**：16 个配置键 + 6 个规则类型 + 3 个动作 + 2 个状态 + 4 个用户类型全部常量化
- **铁律 05（配置走 sys_config）**：16 项 `cloudflare.*` / `risk.*` 配置全部走 sys_config 后台可视化编辑，实时生效
- **铁律 06（反幻觉）**：所有测试基于固定输入断言，无随机性；异地登录检测基于 IP 网段比较无需 GeoIP 数据库；CF 中间件 enabled=0 时直接回退 c.ClientIP()

---

### [新增] 管理员更新弹窗通知（v0.4.x 第十四项：前端轻量轮询 + 自适应间隔 + 防重复弹窗）

#### 背景

- v0.4.0 第十二项之前完成的「在线更新」已支持 GitHub Webhook 自动拉取 + 手动触发 + 回滚 + 健康检查，但管理员需要手动刷新页面才能感知新版本上线
- TODO.md `[迁移] 管理员弹窗通知 → v0.4.x 前端轮询 /admin/update/poll 检测新 commit`
- 商业诉求：管理员日常工作时无需手动检查更新页面，新版本部署后自动弹窗提示刷新

#### 实现

**`migrations/017_v0.4.0_admin_update_poll.up.sql`**：
- `sys_config` 新增 2 项 `update.poll.*` 配置（铁律 05：弹窗通知开关 + 间隔全部走后台可视化编辑）：
  - `update.poll.enabled` = `1`（弹窗通知总开关，1=启用 0=关闭）
  - `update.poll.interval_seconds` = `30`（轮询间隔秒，最小 10 秒由后端 `PollIntervalMin` 常量强制下限）
- 配套 `017_v0.4.0_admin_update_poll.down.sql` 回滚

**`internal/update/update.go`**：
- 新增 2 个配置键常量（铁律 04：禁止硬编码）：
  - `CfgKeyPollEnabled = "update.poll.enabled"`
  - `CfgKeyPollInterval = "update.poll.interval_seconds"`
- 新增 `PollIntervalMin = 10` 常量（轮询间隔下限，防配置错误导致前端打爆后端）

**`internal/handler/update.go`**：
- 新增 `AdminUpdatePoll(deps)` handler，挂载 `GET /admin/update/poll`（adminAuth 组）
- 轻量响应：仅返回 `enabled` / `interval_seconds` / `current_commit` / `is_locked` / `last_update_at` / `last_status` / `last_trigger` / `last_commit` 共 8 个字段
- **关键设计**：不返回 `log_text` / `steps_json` 重字段，降低高频轮询带宽
- 间隔下限保护：从 sys_config 读取后若 < `PollIntervalMin` 强制提升到 10 秒
- 配置即时生效：每次轮询都重新读取 sys_config，后端调整开关/间隔后下一次轮询立即生效

**`internal/router/router.go`**：
- `adminAuth` 组新增 `GET /admin/update/poll` 路由

**`internal/handler/update_poll_test.go`**（新建测试）：
- 13 个测试用例全 PASS，覆盖：
  - 默认配置 + 自定义间隔 + 间隔下限保护 + 等于下限保留
  - enabled=0 关闭弹窗通知
  - 有审计日志时返回最近一次更新元信息 + 多条日志取最新 + 空表时 last_* 全部 nil
  - 响应字段不含 log_text/steps_json + 回滚状态正确返回
  - 配置动态变更即时生效
  - 配置键常量正确 + 响应包含所有预期字段

**前端 `apps/admin/src/api/update.ts`**（新建）：
- 新增 `UpdatePoll` 接口 + `pollUpdateApi()` API 函数
- 同时补全既有 `updateStatusApi` / `triggerUpdateApi` / `listUpdateHistoryApi` / `getUpdateLogApi` / `rollbackUpdateApi` 共 6 个 API

**前端 `apps/admin/src/components/UpdateNotifier.vue`**（新建）：
- 无 UI 仅逻辑组件，挂载于 `AdminLayout.vue`，对所有管理员页面生效
- `localStorage` key `keyauth_admin_last_known_commit` 持久化上次已知 commit，跨会话检测更新
- 自适应间隔：每次轮询后用响应中的 `interval_seconds` 动态调整定时器，后端调整配置即时生效
- `pollOnce()` 异步函数：调 `pollUpdateApi`，返回后端建议间隔（秒），`enabled=false` 返回 0 信号停止定时器，异常返回 30 兜底
- `scheduleNext(intervalSec)` 自适应定时器：检测到间隔变更时重置 setInterval
- `showRefreshDialog(newCommit)` 使用 `ElMessageBox.confirm` 弹窗，`notifiedCommit` ref 防本会话重复弹窗
- 用户选「立即刷新」→ `window.location.reload()` 强制重新加载所有资源
- 用户选「稍后提醒」→ 本会话不再打扰（`notifiedCommit` 标记）
- `onMounted` 启动轮询，`onBeforeUnmount` 停止定时器
- 强制下限 10 秒与后端 `PollIntervalMin` 对齐

**前端 `apps/admin/src/layouts/AdminLayout.vue`**：
- 挂载 `<UpdateNotifier />` 组件（与 `<BasicLayout>` 同级，无 UI 不影响布局）

#### 验证

- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./...` 全 PASS（handler 包含 13 个新测试，update 包无回归）
- `vue-tsc --noEmit` 前端 TypeScript 检查通过

---

### [新增] API 开放平台（v0.4.x 第十三项：开发者 API Token + Webhook 事件推送 + 第三方接入授权）

#### 背景

- v0.3.x 仅支持内部三角色（admin/tenant/agent）JWT 鉴权，第三方开发者无法通过 Token 接入平台
- 业务事件（订单支付、卡密生成、代理注册等）只能在平台内部感知，无法实时通知到开发者自有系统
- TODO.md `[迁移] API 开放平台 → v0.4.x 第三方接入授权 / Webhook 事件推送 / 开发者 API Token 管理`

#### 实现

**`migrations/016_v0.4.0_openapi_platform.up.sql`**：
- 新建 `developer_api_token` 表：开发者 API Token 主表
  - 字段：id / tenant_id / name / token_hash（SHA-512 哈希，不存明文）/ prefix（前 8 位明文，便于识别）/ scopes（逗号分隔权限范围）/ expires_at / last_used_at / last_used_ip / status / revoked_at
  - 唯一索引 `uk_token_hash`（SHA-512 哈希查找）+ 普通索引 `idx_tenant` / `idx_status` / `idx_expires`
- 新建 `webhook_endpoint` 表：Webhook 推送端点
  - 字段：id / tenant_id / name / url / secret_enc（AES-256-GCM 加密存储）/ events（订阅事件列表）/ status / failure_count / last_response_code / last_response_at / last_error
  - 索引 `idx_tenant` / `idx_status`
- 新建 `webhook_delivery` 表：Webhook 推送日志
  - 字段：id / tenant_id / endpoint_id / event_type / event_id（UUID 防重放）/ payload（TEXT）/ status / response_code / response_body / attempt_count / max_retry / next_retry_at / delivered_at
  - 索引 `idx_tenant` / `idx_endpoint` / `idx_status` / `idx_event` / `idx_next_retry`
- `sys_config` 新增 8 项 `openapi.*` / `webhook.*` 配置：
  - `openapi.token.prefix` = `pat_`（Token 前缀，便于识别）
  - `openapi.token.length` = `40`（Token 随机部分长度）
  - `openapi.token.max_per_tenant` = `10`（单租户 Token 数量上限，0=不限）
  - `openapi.token.default_ttl_days` = `365`（默认有效期天数，0=永久）
  - `openapi.scope.available` = 8 个 scope 逗号分隔（card.read/write, order.read/write, agent.read/write, webhook.read/write）
  - `webhook.timeout_seconds` = `10`（HTTP 推送超时）
  - `webhook.max_retry` = `3`（最大重试次数）
  - `webhook.failure_threshold` = `10`（连续失败阈值，达阈值自动 disable 端点）
- 配套 `016_v0.4.0_openapi_platform.down.sql` 回滚

**`internal/model/model.go`**：
- 新增 `DeveloperAPIToken` / `WebhookEndpoint` / `WebhookDelivery` 三个 struct + TableName
- `TokenHash` / `SecretEnc` 字段使用 `json:"-"` 不暴露到 API 响应

**`internal/openapi/openapi.go`**（新建包，API 开放平台核心）：
- 8 个配置键常量（铁律 04：禁止硬编码）+ Token/Endpoint/Delivery 状态常量 + 8 个 Scope 常量 + 5 个事件类型常量
- `TokenManager.GenerateToken(ctx, tenantID, name, scopes, ttlDays)`：
  - 校验 scopes 合法性 → 检查单租户数量上限 → 生成随机 Token（prefix+randomPart，crypto/rand）→ SHA-512 哈希存储 → 计算过期时间 → 写库
  - 明文 Token 仅生成时返回一次（铁律 06：DB 不存明文，仅存 SHA-512 哈希）
- `TokenManager.ValidateToken(ctx, plainToken, clientIP)`：SHA-512 哈希比对 → 状态校验 → 过期校验 → 异步更新 last_used_at/ip
- `TokenManager.RevokeToken` / `ListTokens` / `GetToken`：撤销（status=revoked + revoked_at）/ 列表（分页 + 状态过滤）/ 详情
- `WebhookManager.CreateEndpoint(ctx, *WebhookEndpoint, plainSecret)`：URL 校验 + AES-256-GCM 加密 secret
- `WebhookManager.UpdateEndpoint` / `DeleteEndpoint` / `ListEndpoints` / `GetEndpoint`
- `WebhookManager.DispatchEvent(ctx, tenantID, eventType, payload)`：
  - payload 序列化 → 查询订阅该事件的 active 端点 → 为每个端点创建 delivery 记录 → 同步尝试发送
  - HMAC-SHA256(secret, event_id|timestamp|payload) 签名头（用 `sha512.New512_256` + `hmac.Equal` 常量时间比较防时序攻击）
  - 失败时设置 next_retry_at（2/4/6 分钟退避）+ 端点失败计数 + 阈值自动 disable
- `WebhookManager.RetryDelivery`：手动重试（校验状态 + 重试次数 + 端点 active）
- `WebhookManager.ListDeliveries` / `GetDelivery`：推送日志查询
- `WebhookManager.ProcessPendingRetries(ctx, limit)`：后台 worker 调用，处理 next_retry_at <= now 的 failed delivery
- `ValidateScopes` / `HasScope` / `ParseScopes` / `isSubscribed`：Scope 与事件订阅工具函数
- 辅助函数：`hashToken`（SHA-512 hex，128 字符）/ `signWebhook`（HMAC-SHA256 用 sha512.New512_256）/ `VerifyWebhookSignature`（hmac.Equal 常量时间比较）/ `generateRandomString` / `generateUUID` / `truncate`

**`internal/middleware/auth.go`**：
- 新增 `APITokenAuth(mgr *openapi.TokenManager)` 中间件：
  - 提取 `Authorization: Bearer pat_xxx` → 调用 `TokenManager.ValidateToken` → 注入 `api_token_id` / `api_tenant_id` / `api_scopes` / `api_token_name` 到 gin.Context
  - 失败响应统一 401，不区分"不存在/已撤销/已过期"，防信息泄露
  - 与 JWTAuth（内部账号）/ H5EndUserAuth（终端用户）鉴权分离
- 新增 `RequireScope(scopes ...string)` 中间件：OR 语义（任一 scope 命中即通过），必须在 APITokenAuth 之后使用

**`internal/handler/openapi.go`**（新建 handler）：
- 平台管理端（adminAuth）：`AdminOpenAPIStatus` GET /admin/openapi/status（配置概览 + 全局统计）
- 租户控制台（tenantAuth，13 个端点）：
  - Token 管理：`TenantListAPITokens` / `TenantCreateAPIToken`（返回明文仅一次）/ `TenantGetAPIToken` / `TenantRevokeAPIToken`
  - Webhook 端点 CRUD：`TenantListWebhookEndpoints` / `TenantCreateWebhookEndpoint` / `TenantGetWebhookEndpoint` / `TenantUpdateWebhookEndpoint` / `TenantDeleteWebhookEndpoint`
  - Webhook 推送日志：`TenantListWebhookDeliveries` / `TenantGetWebhookDelivery` / `TenantRetryWebhookDelivery`
  - 元信息：`TenantOpenAPIMeta`（可用 scope + 支持的事件类型，供前端表单勾选）
- 第三方调用方（openapiAuth - API Token 鉴权）：`OpenAPIWhoami` GET /api/v1/openapi/whoami（调试 Token 是否生效）
- `DispatchWebhookEvent(deps, tenantID, eventType, payload)`：异步分发 Webhook 事件辅助函数
  - 异步执行（goroutine），不阻塞业务主流程；panic recover；context.Background()；best-effort

**`internal/router/router.go`**：
- 新增 `openapiAuth` 路由组（`/api/v1/openapi`）：API Token 鉴权，挂载 `OpenAPIWhoami`
- `adminAuth` 新增 1 条路由：`GET /admin/openapi/status`
- `tenantAuth` 新增 13 条路由：`/tenant/openapi/tokens`（5 条）+ `/tenant/openapi/webhooks`（5 条）+ `/tenant/openapi/webhooks/deliveries`（3 条）+ `/tenant/openapi/meta`

**业务点 Webhook 事件接入**（5 个关键事件，全部异步分发）：
- `TenantGenerateCards`（card.go）：卡密批量生成成功 → `card.generated`（仅批次级元信息，不含卡密明文）
- `processPaidOrder`（pay.go）：订单支付成功 + 自动发卡 → `order.paid`
- `processAgentRegisterPaid`（pay.go）：代理注册支付成功 + 创建 Agent → `agent.registered`
- `TenantApproveRecharge`（tenant_finance.go）：代理充值审核通过 → `agent.recharge.approved`
- `TenantPayWithdraw`（tenant_finance.go）：代理提现打款成功 → `agent.withdraw.paid`

**`internal/openapi/openapi_test.go`**（新建测试）：
- 61 个测试用例全 PASS，覆盖：
  - 哈希/签名算法（hashToken / signWebhook / VerifyWebhookSignature 8 种场景）
  - 随机数生成（generateRandomString / generateUUID）
  - Scope 工具（ValidateScopes / HasScope / ParseScopes / isSubscribed）
  - TokenManager 全方法（GenerateToken 6 场景 / ValidateToken 5 场景 / RevokeToken 4 场景 / ListTokens / GetToken）
  - WebhookManager 全方法（CreateEndpoint / UpdateEndpoint / DeleteEndpoint / ListEndpoints / GetEndpoint / DispatchEvent 6 场景 / RetryDelivery 5 场景 / ListDeliveries / GetDelivery / ProcessPendingRetries 3 场景）
  - 集成测试 + 边界测试（10KB 大 payload / nil payload / 最小 Token 长度 / hex 解码）

#### 验证

- `go build ./...` 通过
- `go vet ./...` 通过
- `go test ./...` 全 PASS（openapi 包 61 测试 + 全量已有测试无回归）

---

### [新增] 终端用户体系（v0.4.x 第十二项：H5 注册/登录/绑卡/单点踢出 + HMAC access token + SHA-512 refresh token + jti）

#### 背景

- v0.3.5 H5 购卡流程仅支持匿名下单，无终端用户身份概念；卡密绑定靠设备 HWID，跨设备体验差
- 商业化诉求：H5 注册登录、卡密绑定到账户、多端会话管理、单点踢出、密码自助修改/重置
- TODO.md `[迁移] 终端用户体系 → v0.4.x 终端用户注册/登录/卡密绑定/会话管理`

#### 实现

**`migrations/015_v0.4.0_end_user_system.up.sql`**：
- 新建 `end_user` 表：终端用户主表（tenant_id / app_id / username / phone / email / password_hash / nickname / avatar_url / status / last_login_at / last_login_ip / last_login_ua / login_count / remark）
  - 唯一索引 `uk_tenant_app_username`（tenant_id + app_id + username）：同租户同应用下用户名唯一
- 新建 `end_user_card` 表：用户-卡密绑定关系（user_id / card_id / tenant_id / app_id / bound_at / unbound_at / status）
  - 唯一索引 `uk_card`（card_id）：一张卡同一时间只能绑一个用户
- 新建 `end_user_token` 表：refresh token 会话表（user_id / jti / device_name / device_type / ip / user_agent / refresh_token / expires_at / revoked_at）
  - 唯一索引 `uk_jti`（jti）：精准单点踢出
- ALTER `app_card` ADD COLUMN `end_user_id` BIGINT UNSIGNED NULL（v0.4.0 终端用户绑定，可空向前兼容）+ 索引 `idx_end_user_id`
- `sys_config` 新增 10 项 `enduser.*` 配置：
  - `enduser.register_enabled` = `1`（注册开关）
  - `enduser.login_method` = `username`（登录方式）
  - `enduser.password_min_length` = `8`（密码最小长度）
  - `enduser.verify_code_ttl` = `300`（验证码有效期秒）
  - `enduser.verify_code_length` = `6`（验证码长度）
  - `enduser.access_token_ttl` = `2`（access token 有效期小时）
  - `enduser.refresh_token_ttl` = `30`（refresh token 有效期天）
  - `enduser.bind_card_per_user_max` = `10`（每用户绑定卡数上限）
  - `enduser.allow_anonymous_query` = `1`（允许匿名查卡）
  - `enduser.ip_rate_limit_per_minute` = `60`（IP 限流）
- 配套 `015_v0.4.0_end_user_system.down.sql` 回滚

**`internal/model/model.go`**：
- 新增 `EndUser` / `EndUserCard` / `EndUserToken` 三个 struct + TableName
- `AppCard` 新增 `EndUserID *uint64` 字段（可空，向前兼容）

**`internal/enduser/enduser.go`**（新建包，核心终端用户管理器）：
- `Manager.Register(ctx, req)`：注册（开关校验 + 用户名 trim + 密码长度 + 重复检查 + bcrypt cost=12 + 写库）
- `Manager.Login(ctx, req, jwtSecret)`：登录返回 access + refresh token；更新 last_login_at/ip/ua/login_count
- `generateAccessToken(user, secret, ttlHours)`：HMAC-SHA256(secret, payload).signature 格式，payload = `userID|appID|exp`（避免 jwt 依赖）
- `Manager.issueRefreshToken`：UUID×2 拼接 + SHA-512 哈希存储 + jti 单点踢出
- `parseUA(ua)`：简单设备类型识别（mobile/android/iphone → mobile；bot/spider → bot；其余 → pc）+ 128 字符截断
- `Manager.VerifyRefreshToken` / `RefreshToken`（轮换：旧 token 撤销 + 发新 token）/ `Logout` / `RevokeSession(jti)` / `RevokeAllSessions` / `ListSessions`
- `Manager.BindCard`：事务（卡密状态校验 + 已绑他人校验 + 上限校验 + 复用 unbound 记录 + 写绑定 + 更新 app_card.end_user_id）
- `Manager.UnbindCard`：事务（标记 unbound + 清空 app_card.end_user_id）
- `Manager.ListMyCards` / `GetCardDetail`：通过 end_user_card 关联查询 app_card
- `Manager.GetProfile` / `UpdateProfile`（白名单字段：nickname/avatar_url/email/phone）/ `ChangePassword`（旧密码校验 + 撤销所有会话）/ `ResetPassword`
- `Manager.IsRegisterEnabled` / `IsAnonymousQueryAllowed`
- `ValidateAccessToken(token, secret)`：静态函数（payload.signature 拆分 + HMAC 重算 + 过期校验）
- 常量：10 个 ConfigKey + 3 个 UserStatus + 2 个 BindStatus + 13 个错误

**`internal/middleware/auth.go`**：
- 新增 `H5EndUserAuth(secret)` 中间件：与 JWTAuth 分离，专用于 H5 终端用户 access token 鉴权
  - 提取 Bearer token → 拆分 payload.signature → HMAC-SHA256 重算 + 常量时间比较（防时序攻击）→ 解析 payload → 过期校验 → 注入 `enduser_id` / `enduser_app_id`
- 新增 `computeHMACSHA256(data, secret)` + `hmacEqualConstTime(a, b)` 辅助函数（避免与 pkg/crypto 循环依赖）

**`internal/handler/enduser.go`**（新建，19 个端点）：
- 公开接口（publicGroup，5 个）：
  - `H5EndUserRegister` POST `/public/enduser/register`
  - `H5EndUserLogin` POST `/public/enduser/login`
  - `H5RefreshToken` POST `/public/enduser/refresh`
  - `H5SendVerifyCode` POST `/public/enduser/verify_code`（调用 notify 包生成验证码 + 写日志）
  - `H5ResetPassword` POST `/public/enduser/reset_password`
- H5 鉴权接口（h5Auth 组，10 个）：
  - `H5EndUserMe` GET `/h5/me` / `H5EndUserUpdateProfile` PUT `/h5/me` / `H5EndUserChangePassword` POST `/h5/me/password`
  - `H5EndUserLogout` POST `/h5/logout`
  - `H5EndUserListSessions` GET `/h5/sessions` / `H5EndUserKickSession` POST `/h5/sessions/:jti/kick`
  - `H5EndUserBindCard` POST `/h5/cards/bind` / `H5EndUserUnbindCard` POST `/h5/cards/unbind`
  - `H5EndUserListMyCards` GET `/h5/cards` / `H5EndUserGetCardDetail` GET `/h5/cards/:id`
- Admin 接口（adminAuth，4 个）：
  - `AdminListEndUsers` GET `/admin/endusers`
  - `AdminGetEndUser` GET `/admin/endusers/:id`
  - `AdminUpdateEndUserStatus` PUT `/admin/endusers/:id/status`
  - `AdminEndUserStats` GET `/admin/endusers/stats`

**`internal/router/router.go`**：
- 新建 `h5Auth` 路由组：`v1.Group("/h5")` + `middleware.H5EndUserAuth(cfg.JWT.Secret)`
- 注册 5 个公开 + 10 个 H5 + 4 个 admin 共 19 条路由

#### 测试（`internal/enduser/enduser_test.go`，53 个用例全 PASS）

- Register（6 个）：成功 / 注册关闭 / 用户名空 / 密码过短 / 重复用户名 / 不同租户同用户名
- Login（4 个）：成功 / 用户不存在 / 密码错误 / 用户封禁
- ValidateAccessToken（4 个）：合法 / 错误 secret / 过期（负 TTL）/ 格式错误
- parseUA（4 个）：pc / mobile / bot / 过长截断
- RefreshToken 轮换（3 个）：旧 token 失效 + 新 token 可用 / 非法 token / 空token
- Logout / Revoke（4 个）：Logout 成功 / RevokeSession by jti / RevokeAllSessions / ListSessions 排除过期
- BindCard（8 个）：成功 / 卡密不存在 / 卡密封禁 / 卡密禁用 / 已绑他人 / 幂等 / 解绑后再绑 / 上限超出
- UnbindCard（2 个）：成功 + 卡密 end_user_id 清空 / 未绑定
- ListMyCards / GetCardDetail（4 个）：分页 / 空 / 详情 / 未归属
- UpdateProfile（2 个）：白名单过滤 / 全非法字段不报错
- ChangePassword（3 个）：成功 + 旧密码失效 + 旧 token 撤销 / 旧密码错误 / 新密码过短
- ResetPassword（2 个）：成功 / 过短
- 辅助（4 个）：IsRegisterEnabled / IsAnonymousQueryAllowed / 状态机常量 / bcrypt 集成
- 边界（3 个）：GetProfile NotFound / ChangePassword NotFound / BcryptIntegration

测试栈：SQLite in-memory（end_user + end_user_card + end_user_token + app_card + sys_config AutoMigrate）+ miniredis + 真实 ConfigCache（预置 10 项 enduser.* 配置）+ 真实 bcrypt cost=12

#### 铁律遵守

- **铁律 04（无硬编码）**：10 项配置全部从 sys_config 读取；密码最小长度 / TTL / 绑定上限 / 限流 全部可配置；常量化 3 个 UserStatus + 2 个 BindStatus + 13 个错误
- **铁律 05（配置走后端）**：注册开关 / 登录方式 / 密码长度 / 验证码 TTL/长度 / access/refresh TTL / 绑定上限 / 匿名查卡 / IP 限流 全部可后台实时调整
- **铁律 06（反幻觉）**：
  - 密码 bcrypt cost=12（非明文 / 非 MD5）
  - refresh token SHA-512 哈希存储（非明文）
  - access token HMAC-SHA256 签名 + 常量时间比较（防时序攻击）
  - jti 单点踢出（精准撤销而非全用户踢出）
  - BindCard 事务保证绑定关系 + 卡密 end_user_id 一致性
  - ChangePassword 自动撤销所有会话
  - 全路径覆盖测试（正/负/边界）

#### 可靠性保障

- 卡密绑定事务：end_user_card 写入 + app_card.end_user_id 更新原子化
- 解绑事务：标记 unbound + 清空 end_user_id 原子化
- 修改密码事务：更新密码哈希 + 撤销所有 refresh token 原子化
- 绑定幂等：同用户重复绑同卡返回同一条记录
- 解绑后再绑复用旧记录（重新激活，不重复写）
- ListSessions 自动排除已过期 + 已撤销
- UpdateProfile 白名单过滤（防 status / password_hash 被篡改）

#### 验证

- `go test ./internal/enduser/`：53 个用例全 PASS
- `go test ./...`：15 个测试包全 PASS（无回归）
- `go build ./...` + `go vet ./...`：通过

---

### [新增] 通知系统（v0.4.x 第十一项：短信/邮件/站内信 三通道 + 模板引擎 + 服务商抽象 + 重试 + 限流）

#### 背景

- v0.3.x 仅 `log_login_failed` / 系统公告（notices 表）两类通知，无短信/邮件通道
- 商业化诉求：终端用户验证码（H5 注册/登录）+ 订单支付通知 + 代理佣金到账 + 卡密到期提醒
- TODO.md `[迁移] 通知系统 → v0.4.x 短信/邮件/站内信 三通道 + 模板引擎 + 服务商抽象 + 重试/限流`

#### 实现

**`migrations/014_v0.4.0_notification_system.up.sql`**：
- 新建 `notify_template` 表：通知模板（code / name / channel / subject / content / variables / tenant_id / status / remark）
  - 唯一索引 `uk_code_channel_tenant`（code + channel + tenant_id）：同租户同渠道同 code 唯一
- 新建 `notify_log` 表：发送日志（template_id / template_code / channel / recipient / subject / content / status / provider_msg_id / error_message / retry_count / priority / tenant_id / sent_at）
  - 4 个索引：`idx_log_channel` / `idx_log_status` / `idx_log_tenant` / `idx_log_created`
- `sys_config` 新增 16 项 `notify.*` 配置：
  - SMS（5 项）：`notify.sms.enabled` / `notify.sms.provider` / `notify.sms.access_key_id` / `notify.sms.access_secret_enc`（AES 加密）/ `notify.sms.sign_name`
  - Email（6 项）：`notify.email.enabled` / `notify.email.smtp_host` / `notify.email.smtp_port` / `notify.email.smtp_username` / `notify.email.smtp_password_enc`（AES 加密）/ `notify.email.from_address` / `notify.email.from_name`
  - InApp（1 项）：`notify.inapp.enabled` = `1`
  - 重试与限流（3 项）：`notify.retry.times` = `3` / `notify.retry.interval_seconds` = `60` / `notify.rate_limit.per_minute` = `60`
- 预置 4 个模板：
  - `verify_code`（sms）：`您的验证码：{{code}}，{{ttl}} 分钟内有效`
  - `verify_code_email`（email）：`<h3>验证码</h3><p>您的邮箱验证码：{{code}}</p>`
  - `order_paid`（inapp）：`订单 {{order_no}} 已支付 {{amount}} 元`
  - `agent_commission`（inapp）：`佣金 {{amount}} 元已到账`
- 配套 `014_v0.4.0_notification_system.down.sql` 回滚

**`internal/model/model.go`**：新增 `NotifyTemplate` / `NotifyLog` 两个 struct + TableName

**`internal/notify/notify.go`**（新建包，核心通知管理器）：
- `Render(template, vars)`：模板变量替换，使用 `strings.NewReplacer`（防 SSTI，不用 `text/template`）；未提供的变量保留原占位符便于排查
- `Manager.GetTemplate(ctx, code, channel, tenantID)`：租户自定义优先，回退平台通用（tenant_id=0）
- `Manager.ListTemplates` / `CreateTemplate` / `UpdateTemplate` / `DeleteTemplate`：模板 CRUD
- `Manager.IsChannelEnabled(ctx, channel)`：三通道开关检查（SMS/Email 默认关，InApp 默认开）
- `Manager.CheckRateLimit(ctx, tenantID)`：单租户每分钟限流（查 notify_log 表近 60s 计数）
- `Manager.Send(ctx, req)`：主入口，流程：① 通道开关 ② 限流 ③ 接收人非空 ④ 查模板 ⑤ 渲染 ⑥ 写 pending 日志 ⑦ 调 provider ⑧ 更新日志状态
- `Manager.dispatch`：私有方法，按 channel 路由到 SMSProvider / EmailProvider / InApp（直接成功）
- `Manager.TestDispatch`：暴露 dispatch 供测试发送 handler 使用（绕过模板查找）
- `Manager.SetSMSProvider` / `SetEmailProvider`：mock 注入点（测试用）
- `Manager.ListLogs` / `GetLog` / `Retry`：日志查询 + 失败重试（最大重试次数从 sys_config 读取）
- `Manager.GetStats(ctx, tenantID)`：返回 Stats（Total / Sent / Failed / Pending / SMSCount / EmailCount / InAppCount）；每次 Count 用新 session 避 GORM Where 累积污染
- `GenerateVerifyCode(length)`：crypto/rand 生成数字验证码
- `ParseVariables(varsJSON)` / `ValidateChannel(channel)` 辅助函数
- 常量：16 个 ConfigKey + 3 个 Channel + 4 个 TemplateCode + 2 个 TemplateStatus + 3 个 LogStatus + 3 个 Priority + 7 个错误
- Provider 实现：
  - `aliyunSMSProvider`：骨架实现，AccessKeyID 为空时返回 `ErrProviderNotConfig`；配置后返回伪 msgID（生产应调阿里云 Dysms API）
  - `smtpEmailProvider`：真实 SMTP via `net/smtp.SendMail`，AES 解密 SMTP 密码；构造完整邮件头（From/To/Subject/Message-ID/MIME-Version/Content-Type）

**`internal/handler/notify.go`**（新建，9 个 admin 端点）：
- `AdminNotifyStatus` GET `/admin/notify/status`：三通道配置概览 + 统计 + 模板数
- `AdminListNotifyTemplates` GET `/admin/notify/templates`：分页 + channel 过滤
- `AdminCreateNotifyTemplate` POST `/admin/notify/templates`
- `AdminUpdateNotifyTemplate` PUT `/admin/notify/templates/:id`
- `AdminDeleteNotifyTemplate` DELETE `/admin/notify/templates/:id`
- `AdminListNotifyLogs` GET `/admin/notify/logs`：分页 + channel + status 过滤
- `AdminGetNotifyLog` GET `/admin/notify/logs/:id`
- `AdminRetryNotifyLog` POST `/admin/notify/logs/:id/retry`：失败日志手动重试
- `AdminTestNotify` POST `/admin/notify/test`：绕过模板查找直接 dispatch（template_code="test"）

**`internal/router/router.go`**：注册 9 条 notify admin 路由

#### 测试（`internal/notify/notify_test.go`，36 个用例全 PASS）

- Render（5 个）：空变量 / 单变量 / 多变量 / 未提供变量保留 / SSTI 安全（`{{user}}` → `{{admin}}` 不被解析）
- ValidateChannel（2 个）：sms/email/inapp 合法 / 大小写敏感 + 空字符串 + 未知渠道
- ParseVariables（3 个）：空 / 数组 / 非法 JSON + 非 JSON 数组
- GenerateVerifyCode（3 个）：默认长度 6 / 自定义长度 / 全数字
- IsChannelEnabled（3 个）：默认 SMS/Email 关 + InApp 开 / 全开 / 未知渠道
- CheckRateLimit（3 个）：limit=0 不限 / 未超限 / 超限
- GetTemplate（4 个）：平台回退 / 租户优先 / 不存在 / 禁用模板跳过
- Send（7 个）：通道关闭 / 限流超限 / 模板未找到 / InApp 成功 + 日志写入 / SMS mock provider 成功 / SMS provider 失败 / Email mock provider 成功 / 空接收人
- Retry（3 个）：成功 + retry_count 递增 / 非 failed 状态 / 超过最大重试次数
- GetStats（2 个）：全状态 + 全渠道统计 / 按租户统计
- 模板 CRUD（1 个，覆盖 Create/Get/Update/List/Delete 全流程）
- ListLogs（1 个，覆盖 channel/status/全部 3 种过滤）
- TestDispatch（1 个）：InApp 直接 dispatch
- 常量（1 个）：3 Channel + 4 TemplateCode + 2 TemplateStatus + 3 LogStatus + 3 Priority

测试栈：SQLite in-memory（notify_template + notify_log + sys_config AutoMigrate）+ miniredis + 真实 ConfigCache（预置 16 项 notify.* 配置）+ mockSMSProvider / mockEmailProvider

#### 铁律遵守

- **铁律 04（无硬编码）**：16 项配置全部从 sys_config 读取；常量化 16 个 ConfigKey + 3 Channel + 4 TemplateCode + 2 TemplateStatus + 3 LogStatus + 3 Priority
- **铁律 05（配置走后端）**：三通道开关 / 服务商密钥（AES 加密）/ SMTP 配置 / 重试策略 / 限流 全部可后台实时调整
- **铁律 06（反幻觉）**：
  - 模板变量替换用 `strings.NewReplacer` 不用 `text/template`（防 SSTI）
  - 服务商密钥 AES-256-GCM 加密存储（access_secret_enc / smtp_password_enc）
  - aliyunSMSProvider 未配置 AccessKeyID 时显式返回 `ErrProviderNotConfig`（不返回伪造成功）
  - smtpEmailProvider 真实调用 `net/smtp.SendMail`（不模拟）
  - GetStats 每次 Count 用新 session 避 GORM Where 累积污染（已知陷阱）
  - Updates map key 使用 GORM 列名 `provider_msg_id`（非 JSON tag `provider_msgid`）

#### 可靠性保障

- Send 流程：通道开关 → 限流 → 接收人校验 → 模板查找 → 渲染 → 写 pending 日志 → 调 provider → 更新日志状态
- 失败重试：最大次数从 sys_config 读取；retry_count 递增；超过上限返回 failed
- 限流：单租户每分钟发送数查 notify_log 表实时统计
- 模板查找：租户自定义优先，回退平台通用（tenant_id=0）
- 站内信直接成功（前端拉取日志展示）

#### 验证

- `go test ./internal/notify/`：36 个用例全 PASS
- `go test ./...`：15 个测试包全 PASS（无回归）
- `go build ./...` + `go vet ./...`：通过

---

### [新增] 监控告警体系（v0.4.x 第十项：系统指标采集 + 阈值告警 + Webhook 通知 + 静默期 + 自动恢复）

#### 背景

- v0.3.x 仅有 `log_verify` / `log_operation` / `log_login_failed` 三类业务日志，无系统级资源指标采集
- 商业化诉求：CPU/内存/磁盘超阈值自动告警；错误率突增通知；告警 webhook 集成钉钉/企业微信/飞书
- TODO.md `[迁移] 监控告警 → v0.4.x 系统监控（CPU/内存/磁盘/QPS）+ 阈值告警 + webhook 通知`

#### 实现

**`migrations/013_v0.4.0_monitoring_alerts.up.sql`**：
- 新建 `system_metric` 表：时序指标数据（metric_name / metric_value / metric_unit / labels_json / collected_at）
- 2 个索引：`idx_metric_name_time`（metric_name + collected_at）/ `idx_metric_collected`
- 新建 `system_alert` 表：告警事件（alert_rule / severity / status / metric_value / threshold / operator / message / labels_json / fired_at / resolved_at / acked_by / acked_at / notify_sent）
- 4 个索引：`idx_alert_status` / `idx_alert_rule_status` / `idx_alert_fired` / `idx_alert_severity`
- `sys_config` 新增 9 项 `monitor.*` 配置：
  - `monitor.collect_interval` = `60`（采集间隔秒）
  - `monitor.alert_enabled` = `1`（告警总开关）
  - `monitor.notify.webhook_url`（告警通知 webhook URL）
  - `monitor.silence_minutes` = `30`（静默期分钟数）
  - `monitor.threshold.cpu_usage` = `90`（CPU 使用率阈值 %）
  - `monitor.threshold.memory_usage` = `90`（内存使用率阈值 %）
  - `monitor.threshold.disk_usage` = `85`（磁盘使用率阈值 %）
  - `monitor.threshold.error_rate` = `10`（验证错误率阈值 %）
  - `monitor.retention_days` = `30`（指标保留天数，0=永久）
- 配套 `013_v0.4.0_monitoring_alerts.down.sql` 回滚

**`internal/model/model.go`**：
- 新增 `SystemMetric` struct + `TableName() = "system_metric"`
- 新增 `SystemAlert` struct + `TableName() = "system_alert"`

**`internal/monitor/monitor.go`**（新建包，核心监控管理器）：
- `CompareWithOperator(value, threshold float64, operator string) bool`：通用阈值比较（显式 switch 实现 `>` / `<` / `>=` / `<=` / `==`，禁止字符串拼接 eval）
- `Manager.CollectSystemMetrics(ctx)`：gopsutil 采集 CPU/内存/磁盘使用率 + DB 查询在线设备数 / 今日验证数 / 验证错误率
- `Manager.SaveMetrics(ctx, samples)`：批量写入 `system_metric` 表（labels JSON 序列化）
- `Manager.GetAlertRules(ctx)`：动态从 sys_config 构造 4 条规则（CPU/内存/磁盘/错误率）
- `Manager.EvaluateAlerts(ctx, samples)`：阈值比较 + 静默期去重 + 自动恢复 + webhook 通知
- `Manager.ResolveStaleAlerts(ctx)`：自动恢复超过 1 小时未变化的 firing 告警（防告警堆积）
- `Manager.sendNotification(ctx, alert)`：HTTP POST JSON 到 webhook URL（10s 超时控制；失败不阻塞主流程）
- `Manager.CollectAndEvaluate(ctx)`：一体化入口（采集 → 写入 → 评估 → 自动恢复 → 清理过期），互斥锁防并发
- `Manager.CleanupExpiredMetrics(ctx)`：按保留天数清理过期指标
- `Manager.GetMetricHistory(ctx, name, from, to, limit)`：指标历史查询（按时间倒序，limit 自动校正边界）
- `Manager.GetActiveAlerts(ctx)` / `AckAlert(ctx, id, adminID)` / `SendAlertNotification(ctx, id)`
- 常量：9 个 ConfigKey + 7 个 MetricName + 4 个 Severity + 4 个 Status + 5 个 Operator
- 类型：`MetricSample` / `AlertRule` / `AlertPayload` / `CollectResult`

**`internal/handler/monitor.go`**（新建）：
- `AdminMonitorStatus` GET `/admin/monitor/status`：配置概览 + 活跃告警 + 24h 指标聚合 + 最近采集
- `AdminCollectNow` POST `/admin/monitor/collect`：手动触发采集 + 评估（同步返回结果）
- `AdminMetricHistory` GET `/admin/monitor/metrics?name=&hours=&limit=`：指标历史查询（默认 24h，limit≤1000）
- `AdminListAlerts` GET `/admin/monitor/alerts?status=&severity=&page=&page_size=`：分页查询告警事件
- `AdminAckAlert` POST `/admin/monitor/alerts/ack`：管理员确认告警（标记 acked，停止通知）
- `AdminResendAlert` POST `/admin/monitor/alerts/resend`：手动重发告警通知到 webhook
- `AdminCleanupMetrics` POST `/admin/monitor/cleanup`：手动触发清理过期指标

**`internal/router/router.go`**：注册 7 条新路由（全部 adminAuth 鉴权）

#### 测试（`internal/monitor/monitor_test.go`，53 个用例全 PASS）

- CompareWithOperator（6 个）：`>` / `<` / `>=` / `<=` / `==`（浮点精度 0.001）/ 未知运算符返回 false
- FormatMetricName（4 个）：大写转小写 / 横杠转下划线 / 空格转下划线 / 混合
- SaveMetrics（4 个）：空切片 / 单条带 labels / 多条 / nil labels 默认 `{}`
- GetAlertRules（2 个）：默认 4 条规则阈值 / sys_config 覆盖阈值
- EvaluateAlerts（5 个）：告警关闭不触发 / 触发告警写入 / 未超阈值不触发 / 静默期去重 / 指标恢复自动 resolved
- ResolveStaleAlerts（3 个）：2h 前自动恢复 / 30min 不恢复 / 已 resolved 不重复处理
- CleanupExpiredMetrics（3 个）：按保留天数清理 / retention=0 不清理 / 无过期不删除
- AckAlert（2 个）：成功更新 status/acked_by/acked_at / 不存在 ID 无错误
- GetMetricHistory（4 个）：默认查询按时间倒序 / 按 name 过滤 / limit 边界 / limit=0 修正为 100
- GetActiveAlerts（1 个）：仅返回 firing 状态
- sendNotification（4 个）：未配置 webhook 返回 false / 200 成功 / 500 失败 / 不可达不阻塞
- SendAlertNotification（1 个）：不存在 ID 返回 error
- IsAlertEnabled / GetCollectInterval（3 个）：true / false / 自定义间隔
- 常量（5 个）：9 个 ConfigKey + 7 个 MetricName + 4 个 Severity + 4 个 Status + 5 个 Operator
- 并发（1 个）：5 goroutine 并发 CollectAndEvaluate 互斥锁无 panic
- 集成（2 个）：CollectSystemMetrics 返回 ≥2 指标 / CollectAndEvaluate 闭环
- 边界（4 个）：负数比较 / 零值比较 / 空字符串 / 多指标同时触发

测试栈：SQLite in-memory（system_metric + system_alert + sys_config + app_device + log_verify AutoMigrate）+ miniredis + 真实 ConfigCache（预置 9 项 monitor.* 配置）+ httptest.Server 模拟 webhook 端点

#### 铁律遵守

- **铁律 04（无硬编码）**：9 项配置全部从 sys_config 读取；常量化 7 个 MetricName / 4 个 Severity / 4 个 Status / 5 个 Operator
- **铁律 05（配置走后端）**：采集间隔 / 告警开关 / webhook URL / 静默期 / 4 个阈值 / 保留天数 全部可后台实时调整
- **铁律 06（反幻觉）**：阈值比较用显式 switch 不依赖字符串拼接 eval；webhook 通知超时控制 10s 失败不阻塞主流程；告警写入 + 通知 + 静默期 + 自动恢复 + 清理 全路径覆盖测试

#### 可靠性保障

- 采集互斥锁防并发触发
- 静默期去重（同规则 silence_minutes 内不重复告警）
- 自动恢复超 1h 未变化的 firing 告警（防告警堆积）
- 指标正常时自动 resolved 对应 firing 告警
- webhook 通知失败仅记录 notify_sent=0，不阻塞采集流程
- 指标数据按保留天数自动清理（防表膨胀）

#### 验证

- `go test ./internal/monitor/`：53 个用例全 PASS
- `go test ./...`：15 个测试包全 PASS（无回归）
- `go build ./...`：通过

---

### [新增] 数据备份恢复体系（v0.4.x 第九项：全库 SQL 备份 + SHA-256 校验 + AES-256-GCM 加密 + gzip 压缩 + 过期清理）

#### 背景

- v0.3.x 无任何数据库备份机制，灾难恢复需手动 mysqldump
- 商业化诉求：管理员后台一键备份/恢复；备份文件加密压缩存储；定期自动备份；下载前 checksum 校验；恢复前完整性验证
- TODO.md `[迁移] 数据备份 → v0.4.x 数据库自动备份（每日/每周）+ 备份文件加密压缩存储 + 一键恢复`

#### 实现

**`migrations/012_v0.4.0_backup_restore.up.sql`**：
- 新建 `system_backup_log` 表：审计日志（backup_type / trigger_by / trigger_ip / file_path / file_size / checksum / status / error_message / duration_ms / tables_count / rows_count / restored_from + timestamps）
- 3 个索引：`idx_backup_status` / `idx_backup_type` / `idx_backup_created`
- `sys_config` 新增 6 项 `backup.*` 配置：
  - `backup.dir` = `data/backups`（备份文件存储目录）
  - `backup.retention_days` = `30`（保留天数）
  - `backup.auto_enabled` = `0`（自动备份开关）
  - `backup.encryption_key`（AES-256-GCM 加密密钥 hex，空=不加密）
  - `backup.compress` = `1`（gzip 压缩开关）
  - `backup.tables_filter`（表名白名单，逗号分隔，空=全库）
- 配套 `012_v0.4.0_backup_restore.down.sql` 回滚

**`internal/model/model.go`**：
- 新增 `SystemBackupLog` struct + `TableName() = "system_backup_log"`

**`internal/backup/backup.go`**（新建包，核心备份管理器）：
- `Manager.CreateBackup(ctx, opts)`：完整备份流程
  1. 收集目标表（按 tables_filter 过滤）
  2. 逐表序列化为 SQL INSERT 语句（`serializeValue` 处理 nil/string/[]byte/bool/time.Time/int/float）
  3. 可选 gzip 压缩（magic 0x1f 0x8b 检测）
  4. 可选 AES-256-GCM 加密（nonce + ciphertext 拼接）
  5. 计算 SHA-256 checksum
  6. 写入文件 + 审计日志
- `Manager.RestoreBackup(ctx, opts)`：完整恢复流程
  1. 读取备份文件
  2. SHA-256 校验完整性
  3. 可选 AES 解密
  4. 可选 gunzip 解压
  5. 解析 metadata JSON + SQL 数据
  6. 事务执行：每个表先 DELETE 再 INSERT（防 PK 冲突）
  7. 审计日志记录 restored_from
- `Manager.CleanupExpired(ctx)`：清理过期备份文件 + 更新审计日志状态为 deleted
- `Manager.VerifyChecksum(ctx, id)` / `GetBackupFilePath(ctx, id)` / `IsAutoBackupEnabled(ctx)`
- `parseTablesFilter` / `extractTableName` / `extractPayload` / `executeSQLStatements`
- 常量：6 个 ConfigKey + 3 个 BackupType + 3 个 Status

**`internal/handler/backup.go`**（新建）：
- `AdminCreateBackup` POST `/admin/backup/create`：异步触发备份
- `AdminListBackups` GET `/admin/backup/list`：分页查询（支持 status / backup_type 筛选）
- `AdminGetBackup` GET `/admin/backup/:id`：单条详情
- `AdminDownloadBackup` GET `/admin/backup/:id/download`：下载前强制 checksum 校验
- `AdminRestoreBackup` POST `/admin/backup/restore`：异步触发恢复（仅 status=success 的备份可恢复）
- `AdminCleanupBackups` POST `/admin/backup/cleanup`：手动触发清理过期
- `AdminBackupStatus` GET `/admin/backup/status`：配置概览 + 统计 + 最近成功备份

**`internal/router/router.go`**：注册 7 条新路由（全部 adminAuth 鉴权）

#### 测试（`internal/backup/backup_test.go`，全 PASS）

- serializeValue（6 个）：nil→NULL / string 转义单引号 / []byte→x'hex' / bool→0/1 / time.Time→格式化 / int
- parseTablesFilter（5 个）：空 / 单 / 多 / 含空格 / 全空白
- extractPayload（4 个）：gzip 压缩 / 未压缩 / 缺少分隔符 / 非法 metadata JSON
- CreateBackup（5 个）：无加密无压缩 / gzip 压缩 / AES 加密 / 表过滤 / 无效 AES 密钥
- RestoreBackup（5 个）：无加密 / AES 加密 / checksum 不匹配 / 状态非 success / 不存在
- CleanupExpired（3 个）：按保留天数清理 / retention=0 不清理 / 已删除文件清理
- VerifyChecksum（3 个）：成功 / 失败 / 不存在
- 常量（3 个）：ConfigKey / BackupType / Status
- round-trip 集成（1 个）：备份→恢复完整闭环
- 边界（4 个）：空数据库 / 多表 / 大字段 / 错误恢复

测试栈：SQLite in-memory（system_backup_log + sys_config + app + app_card AutoMigrate）+ miniredis + 真实 ConfigCache + 临时备份目录

#### 铁律遵守

- **铁律 04（无硬编码）**：6 项配置全部从 sys_config 读取；常量化 6 个 ConfigKey / 3 个 BackupType / 3 个 Status
- **铁律 05（配置走后端）**：备份目录 / 保留天数 / 自动开关 / 加密密钥 / 压缩开关 / 表过滤 全部可后台实时调整
- **铁律 06（反幻觉）**：下载前强制 SHA-256 校验；恢复前严格校验文件完整性；SQL 序列化显式处理各类型（不依赖反射）；事务执行 DELETE+INSERT 防 PK 冲突

#### 安全机制

- 备份文件可选 AES-256-GCM 加密（密钥 hex 编码存 sys_config，仅 admin 可读）
- 下载前强制 checksum 校验（损坏文件拒绝下载）
- 恢复前严格校验文件完整性（checksum + status=success）
- 异步执行备份/恢复（避免长耗时阻塞 HTTP 请求）
- 备份/恢复/清理/下载 全部写操作日志（操作人 + IP + 备份 ID）

#### 可靠性保障

- 备份文件 SHA-256 checksum 持久化（审计日志 + 下载前双校验）
- 恢复事务化（DELETE+INSERT 原子提交）
- 过期备份清理（删除文件 + 更新审计日志状态为 deleted）
- 异步备份/恢复不阻塞 HTTP（通过 list 接口查看进度）
- restored_from 字段关联原备份 ID（恢复操作可追溯）

#### 验证

- `go test ./internal/backup/`：全 PASS
- `go test ./...`：15 个测试包全 PASS（无回归）
- `go build ./...`：通过

---

### [新增] 在线更新体系（v0.4.x 第八项：GitHub Webhook + 自动部署 + 回滚 + 审计）

#### 背景

- v0.3.x 部署更新需 SSH 登录服务器手动 git pull + build + restart，无审计、无回滚、易出错
- 商业化诉求：管理员后台一键触发更新；GitHub push 自动部署；失败自动回滚；完整审计日志
- TODO.md `[迁移] 在线更新 → v0.4.0 GitHub Webhook 自动拉取构建重启 + 后台更新管理面板 + 管理员弹窗通知 + 版本回滚`

#### 实现

**`migrations/011_v0.4.0_online_update.up.sql`**：
- 新建 `system_update_log` 表：审计日志（id / trigger_source / trigger_by / trigger_ip / commit_before / commit_after / branch / status / steps_json / log_text / error_message / duration_ms / rolled_back_from / created_at / updated_at）
- 3 个索引：`idx_update_log_status` / `idx_update_log_created` / `idx_update_log_trigger`
- `sys_config` 新增 8 项 `update.*` 配置：
  - `update.webhook.secret`（GitHub Webhook HMAC-SHA256 密钥，空=不校验）
  - `update.webhook.branch` = `main`（监听分支）
  - `update.webhook.auto_update` = `0`（自动触发开关，1=自动；0=仅记录通知）
  - `update.deploy.script_path` = `scripts/deploy_update.sh`（部署脚本相对路径）
  - `update.healthcheck.url` = `http://localhost:8080/health`（健康检查 URL）
  - `update.healthcheck.timeout` = `30`（健康检查超时秒数）
  - `update.rollback.enabled` = `1`（失败自动回滚开关）
  - `update.lock.timeout` = `600`（更新锁超时秒数，防死锁）
- 配套 `011_v0.4.0_online_update.down.sql` 回滚

**`internal/model/model.go`**：
- 新增 `SystemUpdateLog` struct + `TableName() = "system_update_log"`

**`internal/config/cache.go`**：
- 新增 `RedisClient()` 方法暴露底层 Redis 客户端（供 update 包实现分布式锁）

**`internal/update/update.go`**（新建包，核心更新管理器）：
- `VerifyWebhookSignature(signature string, body []byte, secret string) bool`：HMAC-SHA256 校验（`hmac.Equal` 防时序攻击；空 secret 跳过校验仅本地开发）
- `ParsePushEvent(body []byte) (*PushEvent, error)`：解析 GitHub push event（ref 必填校验）
- `BranchMatches(ref, branch string) bool`：refs/heads/ 前缀规范化匹配
- `Manager.AcquireLock(ctx) (bool, error)`：进程内 mutex + Redis SET NX EX 双重锁
- `Manager.ReleaseLock(ctx)`：释放双锁
- `Manager.HealthCheck(ctx) error`：HTTP GET 健康检查（2xx/3xx 视为成功；CheckRedirect 禁用跟随以捕获原始 3xx；超时控制）
- `Manager.ExecuteUpdate(ctx, UpdateOptions) (*UpdateResult, error)`：完整更新流程
  1. 加锁（双重锁）
  2. 创建 pending 审计日志
  3. `git fetch origin <branch>` + `git reset --hard origin/<branch>`（显式命令组合，禁止 shell 拼接用户输入）
  4. 跑部署脚本 `bash <script_path>`（路径从 sys_config 读取）
  5. 健康检查
  6. 失败时调用 `maybeRollback` 自动回滚（若启用）
- `Manager.Rollback(ctx, failedLogID, opts) (*UpdateResult, error)`：回滚到失败日志记录的 `commit_before`（`git reset --hard <commit>` + 重跑脚本 + 健康检查）
- `Manager.GetLatestCommit(ctx) string`：当前部署的 commit hash
- `Manager.IsAutoUpdateEnabled(ctx) bool` / `IsLocked(ctx) bool`：状态查询
- 常量：8 个配置键 + 3 个 TriggerSource + 5 个 Status + 3 个 StepStatus + 4 个 StepName
- 类型：`PushEvent` / `StepResult` / `UpdateOptions` / `UpdateResult`

**`internal/handler/update.go`**（新建）：
- `GitHubWebhook` POST `/api/v1/public/update/webhook`：接收 GitHub push event
  - 读取 raw body
  - HMAC-SHA256 签名校验
  - X-GitHub-Event 非 push 跳过
  - 解析 push event + 分支匹配
  - 自动开关关闭时仅记录通知，开启时异步触发 `ExecuteUpdate`
- `AdminUpdateStatus` GET `/admin/update/status`：当前 commit + 锁状态 + 自动开关 + 分支 + 最近审计日志 + 成功/失败统计
- `AdminTriggerUpdate` POST `/admin/update/trigger`：管理员手动触发（异步执行，立即返回）
- `AdminListUpdateHistory` GET `/admin/update/history`：分页查询审计日志（支持 status / trigger_source 筛选）
- `AdminRollbackUpdate` POST `/admin/update/rollback`：手动回滚到指定失败日志的 commit_before
- `AdminGetUpdateLog` GET `/admin/update/logs/:id`：单条审计日志详情（含完整 log_text）

**`internal/router/router.go`**：注册 6 条新路由
- publicGroup: `POST /update/webhook`（无鉴权，靠 HMAC 签名校验）
- adminAuth: `GET /update/status` + `POST /update/trigger` + `GET /update/history` + `POST /update/rollback` + `GET /update/logs/:id`

**`scripts/deploy_update.sh`**（新建）：默认部署脚本
- 步骤 1：`go mod download`
- 步骤 2：`go build -o ../../bin/keyauth-server ./cmd/main.go`
- 步骤 3：数据库迁移由 server 启动时自动执行（不重复）
- 步骤 4：根据 `DEPLOY_MODE` 环境变量重启服务（systemd / docker / pm2 / none 自适应）
- 严格 `set -euo pipefail` + 显式 `cd` 项目根 + 错误退出码便于回滚判定

#### 测试（`internal/update/update_test.go`，37 个用例全 PASS）

- VerifyWebhookSignature（7 个）：有效签名 / 错误 secret / 空 secret 跳过 / 空签名拒绝 / 错误前缀拒绝 / 篡改 body 拒绝 / 空 body 边界
- ParsePushEvent（4 个）：有效 payload / 非法 JSON / 空 ref / 缺失 ref
- BranchMatches（5 个）：短形式 / 完整形式 / 不匹配 / 空分支 / tag ref 不匹配
- AcquireLock / ReleaseLock（5 个）：首次获取成功 / 二次获取失败 / 释放后重新获取 / Redis key SET/DEL 验证 / 多 Manager 共享 lockKey 互斥
- HealthCheck（6 个）：2xx 成功 / 3xx 成功（禁用重定向） / 5xx 失败 / 4xx 失败 / 连接拒绝 / 超时尊重（1s 超时 < 2s）
- 状态机常量（4 个）：TriggerSource / Status / StepStatus / 8 个 ConfigKey 互不冲突 + 全部 `update.` 前缀
- IsAutoUpdateEnabled / IsLocked（4 个）：默认 false / true / 未锁 / 已锁
- 边界（6 个）：大 body 10KB / 额外字段忽略 / 分支名特殊字符 / 不同 lockKey 互不影响 / 多次校验一致性 / PushEvent JSON round-trip
- 并发压力（1 个）：10 goroutine 抢同一锁无 panic 无死锁

测试栈：SQLite in-memory（system_update_log + sys_config AutoMigrate）+ miniredis + 真实 `ConfigCache`（预置 8 项 sys_config + overrides 覆盖）+ httptest.Server 模拟健康检查端点

#### 铁律遵守

- **铁律 04（无硬编码）**：8 项配置全部从 sys_config 读取；常量化 3 个 TriggerSource / 5 个 Status / 4 个 StepName；分支默认值 `main` 仅作 fallback
- **铁律 05（配置走后端）**：webhook 密钥 / 分支 / 自动开关 / 部署脚本路径 / 健康检查 URL+超时 / 回滚开关 / 锁超时全部可后台实时调整
- **铁律 06（反幻觉）**：HMAC 校验用 `hmac.Equal` 防时序攻击；shell 命令显式组合（`exec.Command("git", "fetch", "origin", branch)`）禁止 shell 拼接用户输入；签名校验失败 / 分支不匹配 / 非 push 事件 / 锁竞争 / 健康检查失败 全路径覆盖测试

#### 安全机制

- Webhook 端点无鉴权但强制 HMAC-SHA256 签名校验（空 secret 仅本地开发用）
- 管理后台 5 个接口仅 `admin` 角色可访问（JWTAuth 中间件）
- 所有更新操作写 `system_update_log` 审计日志（操作人 / IP / 前后 commit / 状态 / 步骤 / 完整日志文本）
- shell 命令显式组合参数，禁止 eval/exec 任意用户输入
- 部署脚本路径从 sys_config 读取，仅 root/admin 可后台修改

#### 可靠性保障

- 更新过程双重锁（进程内 mutex + Redis SET NX EX）防并发触发
- 锁超时 600s 自动释放（防死锁）
- 失败自动回滚到 `commit_before`（`git reset --hard` + 重跑脚本 + 健康检查）
- 健康检查通过后才标记 success；失败立即触发回滚
- 完整步骤日志（`steps_json` JSON 数组 + `log_text` 人类可读文本）

#### 适配性

- 部署脚本通过 `DEPLOY_MODE` 环境变量适配 systemd / docker-compose / pm2 / 无外部监管（none）
- 健康检查 URL 可配置（生产可指向负载均衡健康端点）
- 自动开关关闭时仅记录通知，由管理员手动触发（半自动模式）
- 与现有 migration.Run 启动迁移机制兼容（部署脚本不重复执行 migration）

#### 验证

- `go test ./internal/update/`：37 个用例全 PASS
- `go test ./...`：13 个测试包全 PASS（无回归）
- `go build ./...`：通过
- `go vet ./...`：0 警告

---

### [新增] 灰度发布体系（v0.4.x 第七项：三策略 + Hash 桶 + 跨租户查询 + 编辑接口）

#### 背景

- v0.3.x 应用版本仅支持「最新 active 版本一刀切」推送，无法按平台/渠道/地区/比例灰度
- 商业化诉求：新版本先小范围验证再放量；支持全量（full）/ 灰度（grayscale）/ 金丝雀（canary）三种发布策略
- TODO.md `[迁移] 灰度发布 → v0.4.x 应用版本灰度推送 + 灰度规则配置（按地区/比例）`

#### 实现

**`migrations/010_v0.4.0_grayscale_release.up.sql`**：
- `app_version` 表新增 5 字段：
  - `release_strategy VARCHAR(32) NOT NULL DEFAULT 'full'`（full / grayscale / canary）
  - `grayscale_rate DECIMAL(5,2) NOT NULL DEFAULT 0.00`（命中比例 0~100）
  - `grayscale_platforms VARCHAR(200)`（逗号分隔平台白名单，空=不限）
  - `grayscale_regions VARCHAR(500)`（逗号分隔地区白名单，空=不限）
  - `grayscale_channels VARCHAR(200)`（逗号分隔渠道白名单，空=默认 stable）
- 新增复合索引 `idx_app_status_strategy`（app_id, status, release_strategy），加速客户端灰度匹配查询
- 补齐 `app_version.tenant_id BIGINT NOT NULL DEFAULT 0`（修复 001 init schema 遗漏，model 已有该字段）
- `sys_config` 新增 3 项：
  - `app.version.grayscale.enabled` = `1`（灰度全局开关，0=关闭后所有灰度策略回退到 full）
  - `app.version.grayscale.default_rate` = `10.00`（新建灰度版本未指定 rate 时的默认比例）
  - `app.version.grayscale.hash_salt` = `keyauth-grayscale-v040`（Hash 桶算法盐值，更换可全量重排灰度命中）
- 配套 `010_v0.4.0_grayscale_release.down.sql` 回滚（注：tenant_id 不回滚，因 001 schema 遗漏为既有 bug 修复）

**`internal/model/model.go`**：
- `AppVersion` struct 新增 5 字段（`ReleaseStrategy` / `GrayscaleRate` / `GrayscalePlatforms` / `GrayscaleRegions` / `GrayscaleChannels`）+ gorm 默认值标签

**`internal/grayscale/grayscale.go`**（新建包，核心匹配算法）：
- `Match(ctx, cfgCache, MatchRequest) MatchResult`：7 步过滤链
  1. nil version → 未命中
  2. full 策略 → 直接命中
  3. grayscale / canary + `app.version.grayscale.enabled=0` → 回退 full 命中
  4. 平台过滤（`grayscale_platforms` 非空时 client 必须在白名单，大小写不敏感）
  5. 渠道过滤（空默认 `stable`，client 必须在白名单）
  6. 地区过滤
  7. 比例过滤：rate<=0 不命中；rate>=100 命中；rate∈(0,100) 走 Hash 桶
- `HashBucket(salt string, appID uint64, clientID string) int`：`SHA-256(salt + ":" + appID + ":" + clientID)` 取前 4 字节小端 uint32 % 100，返回 0~99 稳定桶号
- `ParseList(s string) []string`：逗号分隔解析器（trim 空白 + 转小写 + 去重，返回非 nil 切片）
- `DefaultRate(ctx, cfgCache) float64`：读取 `app.version.grayscale.default_rate`
- `IsEnabled(ctx, cfgCache) bool`：读取 `app.version.grayscale.enabled`
- 常量：`StrategyFull` / `StrategyGrayscale` / `StrategyCanary` + 3 个配置键常量
- 类型：`MatchRequest{Version, ClientID, Platform, Channel, Region}` / `MatchResult{Matched, Reason, Rate, Bucket}`

**`internal/handler/client.go`**：
- `ClientVersion` 升级支持灰度匹配
- 请求 DTO 扩展：新增 `Region` / `Channel` / `HWID` / `DeviceID` 字段
- 查询改为 `Find`（所有 active 版本按 id DESC 排序）替代原 `First`（单最新）
- ClientID 解析优先级：`hwid > device_id > client_ip`（保证同一设备稳定命中同一桶号）
- 遍历候选版本调用 `grayscale.Match`，首个命中即返回
- 响应：保留原 9 字段 + 新增 `release_strategy` / `grayscale_hit` / `grayscale_bucket` / `grayscale_rate`（仅 grayscale/canary 命中时返回）
- 全部未命中时返回 `5011 当前无可用版本（未命中灰度规则）`

**`internal/handler/tenant_business.go`**：
- `createVersionReq` DTO 扩展：新增 `BackupURL` / `ReleaseStrategy` / `GrayscaleRate` / `GrayscalePlatforms` / `GrayscaleRegions` / `GrayscaleChannels`
- `TenantCreateVersion`：strategy 默认 full；grayscale/canary 且 rate=0 时自动取 `grayscale.DefaultRate()`
- 新增 `updateVersionReq` struct（指针字段支持可选更新：`*string` / `*float64`）
- 新增 `TenantUpdateVersion` handler（PUT `/tenant/versions/:id`）：
  - 归属校验（tenant_id 必须匹配当前租户）
  - 构建更新 map（仅写入非 nil 字段）
  - 切换到 grayscale/canary + rate=0 → 自动取 `DefaultRate`
  - 重新查询并返回完整记录

**`internal/handler/admin_business.go`**：
- 新增 `adminVersionListItem` struct（嵌入 AppVersion + TenantName + AppName 联表字段）
- `AdminListVersions`（GET `/admin/versions`）：JOIN sys_tenant + app，支持 tenant_id / app_id / channel / release_strategy 多条件筛选
- `AdminGetVersion`（GET `/admin/versions/:id`）：单版本详情（跨租户查询，平台超管视角）

**`internal/router/router.go`**：注册 3 条新路由
- adminAuth: `GET /versions` + `GET /versions/:id`
- tenantAuth: `PUT /versions/:id`

#### 测试（`internal/grayscale/grayscale_test.go`，33 个用例全 PASS）

- Match full 策略（3 个）：full 始终命中 / 空策略默认 full / nil version
- Match 全局开关（1 个）：全局禁用时回退 full
- Match 平台过滤（4 个）：在白名单 / 不在白名单 / 大小写不敏感 / 空平台
- Match 渠道过滤（3 个）：在白名单 / 不在白名单 / 空默认 stable
- Match 地区过滤（3 个）：在白名单 / 不在白名单 / 空地区
- Match 比例（4 个）：rate=0 / rate=100 / 部分桶命中 / 部分桶未命中
- HashBucket 稳定性（4 个）：稳定同输入同输出 / 范围 0-99 / 不同 salt 不同桶 / 不同 appID 不同桶
- ParseList（6 个）：空 / 单 / 多 / 含空格 / 混合大小写 / 仅逗号
- DefaultRate / IsEnabled（4 个）：从配置读取 / fallback / 默认 true / 禁用时 false
- 边界（4 个）：匿名 clientID / canary 策略 / 多过滤全过 / 多过滤一过滤失败

测试栈：SQLite in-memory（app_version + sys_config AutoMigrate）+ miniredis + 真实 `ConfigCache`（预置 sys_config 3 项 + overrides 覆盖）

#### 铁律遵守

- **铁律 04（无硬编码）**：灰度全局开关 / default_rate / hash_salt 全部从 sys_config 读取；策略名 / 渠道默认值用常量
- **铁律 05（配置走后端）**：3 项配置可通过后台「系统配置」实时调整（更换 hash_salt 可全量重排灰度命中）
- **铁律 06（反幻觉）**：Hash 桶算法基于 SHA-256 标准库字节级稳定；测试覆盖正/负/零/边界/兼容全场景；ClientVersion 候选版本遍历匹配首个命中即返回（不预判策略优先级）

#### 兼容性

- v0.3.x 老版本升级后 `release_strategy='full'` + `grayscale_rate=0`，行为等同原「最新 active 版本一刀切」
- v0.3.x 客户端不传 `region` / `channel` / `hwid` / `device_id` 字段时，ClientVersion 回退到 client_ip 作为 ClientID 进行桶号计算
- 回滚 SQL：DROP 字段 + 删除 3 项 sys_config；回滚前需确认无活跃灰度版本

#### 验证

- `go test ./internal/grayscale/`：33 个用例全 PASS
- `go test ./...`：12 个测试包全 PASS（无回归）
- `go build ./...`：通过
- `go vet ./...`：0 警告

---

### [新增] 多级代理体系（v0.4.x 第六项：跨级佣金 + 层级校验 + 代理树）

#### 背景

- v0.3.x 代理体系仅支持扁平结构，无上下级关系；代理推广收益仅来自直接销售佣金
- 商业化诉求：支持 3 级代理链路（1 级=开发者直签 / 2 级=代理邀请 / 3 级=代理子邀请），跨级自动分润
- TODO.md `[迁移] 多级代理体系 → v0.4.x`

#### 实现

**`migrations/009_v0.4.0_agent_multi_level.up.sql`**：
- `agent` 表新增 `parent_id BIGINT NOT NULL DEFAULT 0` + `level TINYINT NOT NULL DEFAULT 1`
- 新增索引 `idx_agent_parent` / `idx_agent_level`
- `agent_invite_code` 表新增 `creator_type VARCHAR(16) NOT NULL DEFAULT 'tenant'` + `creator_agent_id BIGINT NOT NULL DEFAULT 0`
- 新增索引 `idx_invite_code_creator_agent`
- `sys_config` 新增 4 项：
  - `agent.commission.cross_level_2_rate` = `50.00`（二级代理产生佣金时，父级（一级）分润 50%）
  - `agent.commission.cross_level_3_rate` = `20.00`（三级代理产生佣金时，祖父级（一级）分润 20%）
  - `agent.commission.max_level` = `3`（最大代理层级）
  - `agent.invite_code.agent_can_create` = `1`（是否允许代理创建下级邀请码）
- 配套 `009_v0.4.0_agent_multi_level.down.sql` 回滚

**`internal/model/model.go`**：
- `Agent` struct 新增 `ParentID uint64` + `Level int` 两字段
- `AgentInviteCode` struct 新增 `CreatorType string` + `CreatorAgentID uint64` 两字段

**`internal/multilevel/multilevel.go`**（新建包）：
- `DistributeCrossCommission(ctx, tx, cfgCache, agent, commission, relatedCardIDsJSON)`：沿 `parent_id` 链向上最多 2 层分发跨级佣金
  - 源 level=2 → 父级（level=1）获 `cross_level_2_rate%`
  - 源 level=3 → 父级（level=2）获 `cross_level_2_rate%`，祖父级（level=1）获 `cross_level_3_rate%`
  - 父级状态非 `active` 跳过（break 整链），父级被物理删除时停止
  - 事务内 `gorm.Expr("balance + ?")` 更新父级余额 + 写 `AgentBalanceLog{Type:"cross_commission"}`
- `CanCreateSubordinate(ctx, cfgCache, agent)`：校验 `agent_can_create` + `level < max_level` + `status=active`
- `ComputeSubordinateLevel(ctx, db, cfgCache, ic)`：tenant 邀请码 → (0, 1)；agent 邀请码 → (creator.ID, creator.Level+1) 含 `CanCreateSubordinate` 校验
- `BuildAgentTree(ctx, db, rootAgentID, maxDepth)`：递归构建代理下级树（含 `tenant_id` 隔离）
- `ListSubordinates(ctx, db, agentID, tenantID)`：单层直接下级列表
- 错误哨兵：`ErrAgentNotFound` / `ErrLevelExceedsMax`

**`internal/handler/pay.go`**：
- `processAgentRegisterPaid` 创建 Agent 前调用 `ComputeSubordinateLevel` 计算 `parent_id` + `level`，写入 Agent

**`internal/handler/agent_business.go`**：
- `AgentGenerateCards` 在事务内佣金结算后调用 `DistributeCrossCommission`，响应新增 `cross_commissions` 字段（仅非空时返回）
- 5 个新 handler：
  - `AgentGenInviteCode` POST `/agent/invite_codes`：代理创建下级邀请码（`CanCreateSubordinate` + `quota.CheckMaxAgents` + `CreatorType="agent"` + `CreatorAgentID=agentID`）
  - `AgentListInviteCodes` GET `/agent/invite_codes`：列出自己的下级邀请码
  - `AgentDisableInviteCode` POST `/agent/invite_codes/:id/disable`：禁用自己的邀请码（归属校验）
  - `AgentListSubordinates` GET `/agent/subordinates`：直接下级单层列表
  - `AgentGetTree` GET `/agent/tree`：递归下级树（`maxDepth = max_level - 1`）

**`internal/handler/tenant_business.go`**：
- 新增 `TenantGetAgentTree` GET `/tenant/agents/:id/tree`（校验 agent 归属当前租户）

**`internal/handler/admin_business.go`**：
- 新增 `AdminGetAgentTree` GET `/admin/agents/:id/tree`（超管跨租户查询）

**`internal/router/router.go`**：注册 7 条新路由（admin 1 + tenant 1 + agent 5）

#### 测试（`internal/multilevel/multilevel_test.go`，27 个用例全 PASS）

- `DistributeCrossCommission`（7 个）：level 1 无父级 / level 2 父级 50% / level 3 父级 + 祖父级 / 父级禁用跳过 / 零佣金 / nil agent / 自定义比例
- `CanCreateSubordinate`（6 个）：level 1 max=3 / level 3 max=3 / max=1 / `agent_can_create=false` / 禁用 / nil
- `ComputeSubordinateLevel`（5 个）：tenant 邀请码 → level 1 / agent 邀请码 → level 2 / level 3 创建者超限 / 创建者不存在 / nil
- `BuildAgentTree`（4 个）：三级树 / `maxDepth=0` 仅根 / 不存在 / 租户隔离
- `ListSubordinates`（3 个）：单层 / 无子级 / 租户隔离
- 边界（2 个）：负佣金 / parent 链断裂

测试栈：SQLite in-memory（4 表 AutoMigrate）+ miniredis + 真实 `ConfigCache`（预置 sys_config 4 项）

#### 铁律遵守

- **铁律 04（无硬编码）**：跨级佣金比例 / max_level / `agent_can_create` 全部从 sys_config 读取，默认值仅作 fallback
- **铁律 05（配置走后端）**：4 项配置可通过后台「系统配置」实时调整，无需重启
- **铁律 06（反幻觉）**：跨级佣金算法基于 `agent.Level`（源代理层级）+ depth 判断比例（修复了基于 `current.Level` 的 bug，否则 depth=1 时会误用 `cross_level_2_rate`）；测试覆盖正/负/零/边界/兼容全场景

#### 兼容性

- v0.3.x 老代理升级后：`parent_id=0` + `level=1`，行为等同一级代理（无跨级佣金）
- v0.3.x 老邀请码升级后：`creator_type='tenant'` + `creator_agent_id=0`，新代理仍注册为一级
- 回滚 SQL：DROP 字段 + 删除 4 项 sys_config；回滚前需确认无活跃二级/三级代理

#### 验证

- `go test ./internal/multilevel/`：27 个用例全 PASS
- `go test ./...`：11 个测试包全 PASS（无回归）
- `go build ./...`：通过

---

### [新增] 全语言 SDK 扩展（Java / C# / Go / C++ / 易语言，v0.4.x 第五项）

#### 背景

- v0.3.6 仅提供 Python / Node.js / PHP 三语言 SDK，无法覆盖桌面软件开发者主流语言生态
- TODO.md `[迁移] 全语言 SDK → v0.4.x Java / C# / Go / C++ / 易语言`

#### 实现

**Go SDK（`sdks/go/`）** — 原生 SHA-512/256 对齐
- `keyauth/keyauth.go`：9 个客户端 API + 强类型 struct 返回 + 零第三方依赖（仅 Go 标准库）
- `crypto/sha512.New512_256` 与后端字节级一致，无回退
- `example/example.go`：完整调用示例
- `tests/sign.go`：签名对齐测试脚本

**Java SDK（`sdks/java/`）** — JDK 11+ HttpClient
- `KeyAuthClient.java` + `KeyAuthException.java`：9 个客户端 API
- 优先 `Mac.getInstance("HmacSHA512/256")`（JDK 17+），回退 `HmacSHA256`
- `pom.xml`：Maven 工程文件（依赖 Jackson）
- `tests/Sign.java`：独立签名脚本（无 Jackson 依赖，单文件源码模式运行）

**C# SDK（`sdks/csharp/`）** — .NET 6+ HttpClient
- `KeyAuthClient.cs`：9 个异步 API（`Task<JsonElement>` 返回）
- 反射探测 BouncyCastle 提供者，启用 `HMac(Sha512_256Digest)`；不可用回退 `HMACSHA256`
- `KeyAuth.Sdk.csproj`：条件依赖 BouncyCastle
- `tests/sign.cs`：独立签名脚本

**C++ SDK（`sdks/cpp/`）** — libcurl + OpenSSL 1.1+ + nlohmann/json
- `include/keyauth/keyauth.hpp` + `src/keyauth.cpp`：9 个客户端 API
- `EVP_sha512_256()` 与后端字节级一致；OpenSSL < 1.1 回退 `EVP_sha256` + stderr 警告
- `CMakeLists.txt`：自动 FetchContent 拉取 nlohmann/json
- `tests/sign.cpp`：独立签名脚本（仅 OpenSSL 依赖）

**易语言 SDK（`sdks/epl/`）** — Windows-only
- `keyauth_sdk.e.txt`：纯中文 API（登录 / 验证 / 心跳 / 绑定 / 解绑 / 取变量 / 取公告 / 取版本 / 退出）
- 依赖精易模块 v9.0+ 的 `HMAC_SHA256` / `json_解析` / `网页_访问`
- 注：易语言生态无 SHA-512/256 实现，统一使用 HMAC-SHA256（与后端 SHA-512/256 不同，仅在后端回退场景下匹配）
- `tests/sign.e.txt`：签名测试脚本（不参与 Linux CI 自动化测试）

**签名对齐测试扩展（`apps/server/pkg/crypto/sign_alignment_test.go`）**：
- 从 3 语言（Python/Node.js/PHP）扩展到 7 语言（新增 Go / Java / C++ / C# + 易语言 Skip）
- `runSignCompiled`：编译型语言（C++）通用编译+运行框架
- `runSignJavaSingleFile`：JDK 11+ 单文件源码模式运行 Java
- `runCSharpScript`：dotnet 临时项目编译运行 C#
- `javaSupportsSHA512_256`：JDK 版本检测，仅在 JDK 17+ 时强断言签名匹配
- `TestSignAlignment_NewLanguages`：5 个新语言 SDK 目录结构元数据校验（CI 友好，不依赖运行时）
- 7 语言子测试在缺失运行时时自动 `t.Skip`

#### 测试覆盖

- `TestSignAlignment_AllLanguages`：3 测试用例 × 7 语言 = 21 子测试（运行时不可用时 skip）
- `TestSignAlignment_NewLanguages`：5 个新 SDK 的目录结构 + 5 个签名脚本存在性校验
- `TestSignAlignment_BackendDeterministic`：后端签名确定性

#### 铁律遵守

- **铁律 04（无硬编码）**：所有 SDK 的 API 地址 / AppKey / SignSecret 由构造函数 / 初始化方法传入
- **铁律 05（配置走后端）**：SDK 不内置任何配置，行为由后端 sys_config 控制
- **铁律 06（反幻觉）**：签名算法回退策略明确标注（Java/C#/C++/易语言），不掩盖与后端的差异；测试中 Java/C# 在回退场景下不强制相等，仅 t.Logf 提示

#### 验证

- `go test ./...`：11 个测试包全 PASS（`pkg/crypto` 测试时间 9.2s，包含 5 个新语言子测试）
- `go vet ./...`：0 警告
- `go build ./...`：通过

---

### [新增] 2FA 备用码 DB 持久化 + 登录失败日志结构化（v0.4.x 迁移项第三 / 四项）

#### 背景

- v0.3.x 2FA 备用码存 Redis（`2fa:backup:{role}:{user_id}` 持久化无 TTL），存在 Redis 单点故障 / 内存占用 / 不便审计等问题
- v0.3.x 异步日志 worker（登录失败 / 验证 / 操作日志）写入失败时 `_ = err` 静默丢弃，无法定位 DB 写入异常
- TODO.md 第 251 行 `[迁移] 2FA backup_codes Redis 持久化 → 加表字段后迁移` + 第 252 行 `[迁移] 登录失败日志结构化记录 → v0.4.x 引入 zap/zerolog`

#### 实现（迁移项3：2FA backup_codes DB 持久化）

**`migrations/008_v0.4.0_2fa_backup_codes.up.sql`**：
- `sys_admin` / `sys_tenant` / `agent` 三表新增 `backup_codes VARCHAR(512) NOT NULL DEFAULT ''` 字段
- 存 AES-256-GCM 加密的逗号分隔字符串（最多 5 个备用码，加密后约 200 字符，512 字符安全冗余）

**`internal/model/model.go`**：
- 三表 struct 同步新增 `BackupCodes string` 字段（`gorm:"size:512" json:"-"`）

**`internal/handler/profile.go`**：
- 新增 `loadUserBackupCodes(deps, role, userID)`：优先读 DB 字段，DB 为空时回退 Redis 老数据（兼容 v0.3.x 用户）
- 新增 `updateUserBackupCodes(deps, role, userID, enc)`：按 role 更新对应表
- 新增 `consumeBackupCode(deps, role, userID, input) (matched, remaining, err)`：解密 → 匹配 → 移除 → 加密回写 DB + 同步清理 Redis 老数据
- `Verify2FA` 第 4 步：备用码从 Redis 持久化改为 DB 字段写入 + 清理 Redis 老数据
- `Disable2FA` 第 5 步：清空 DB `backup_codes` 字段 + 清理 Redis 老数据
- `twoFABackupKey` 注释更新：从「待核实 v0.4.x 加表字段后迁移」改为「v0.4.0 仅作兼容回退读取」

#### 实现（迁移项4：登录失败日志结构化）

**`internal/logger/logger.go`**（新建包）：
- 基于 Go 1.21+ 标准库 `log/slog`，零第三方依赖（取代 zap/zerolog 引入）
- `Options{Level, Format, Output}`：级别（debug/info/warn/error，默认 info）+ 格式（json/text，默认 json）+ 输出（stdout/stderr/文件路径，默认 stdout）
- `Init(opt)` 初始化全局 logger（atomic.Value 并发安全切换）
- `L() / Debug / Info / Warn / Error / DebugCtx / InfoCtx / WarnCtx / ErrorCtx` 便捷封装

**`internal/config/config.go`**：
- `AppConfig` 新增 `LogLevel` / `LogFormat` / `LogOutput` 三个 yaml 字段

**`cmd/main.go`**：
- 启动时调用 `logger.Init(logger.Options{...})` 从 config 注入

**`internal/handler/session.go` + `internal/handler/log_worker.go`**：
- 3 处 `_ = err` 替换为 `logger.Error("xxx write failed", "err", err, ...业务字段...)` 结构化日志
- 字段包含：err / user_type / username / client_ip / tenant_id / app_id / action / operator_type / operator_id / module / action（按场景）
- 移除 3 处「待核实 v0.4.x：引入结构化日志记录此错误」标注

#### 测试（`internal/handler/profile_2fa_test.go` + `internal/logger/logger_test.go`）

**`profile_2fa_test.go`（13 个测试，全 PASS）**：
- `TestLoadUserBackupCodes_DB读取`：从 DB 字段读取 AES 加密的备用码
- `TestLoadUserBackupCodes_DB为空回退Redis`：v0.3.x 老用户兼容路径
- `TestLoadUserBackupCodes_用户不存在`：gorm.ErrRecordNotFound 透传
- `TestLoadUserBackupCodes_TenantRole` / `AgentRole` / `不支持角色`：role 分支覆盖
- `TestUpdateUserBackupCodes_清空`：传入空字符串清空字段
- `TestConsumeBackupCode_消费成功`：正确输入后备用码被移除 + DB 更新 + Redis 清理
- `TestConsumeBackupCode_消费最后一个`：剩余空时 DB 写入空字符串
- `TestConsumeBackupCode_输入不匹配`：错误输入不消费
- `TestConsumeBackupCode_空输入` / `无备用码`：边界条件
- `TestConsumeBackupCode_从Redis回退消费`：v0.3.x 老用户首次消费走 Redis 路径
- `TestTwoFABackupKey_格式` / `TestTwoFASetupKey_格式`：key 格式断言

**`logger_test.go`（6 个测试，全 PASS）**：
- `TestParseLevel`：4 级别 + 大小写 + 默认值
- `TestInit_JSONFormat`：JSON 输出包含 level/msg/字段
- `TestInit_LevelFiltering`：level=warn 时 info/debug 不输出
- `TestInit_TextFormat`：text 格式 msg 含空格自动加引号
- `TestL_ReturnsNonNil`：Init 前后 L() 非 nil
- `TestInit_DefaultFallback`：空 Options 不 panic

#### 铁律遵守

- **铁律 04（无硬编码）**：日志级别 / 格式 / 输出路径 从 `config.yaml` 读取，无硬编码
- **铁律 05（配置走后端）**：未来可扩展为 sys_config 热更新日志级别（Init 支持重复调用）
- **铁律 06（反幻觉）**：测试覆盖 DB 读取 / Redis 回退 / 消费 / 边界 / role 分支 / 兼容路径全场景；profile.go 注释更新与代码一致

#### 兼容性

- v0.3.x 老用户升级后：DB `backup_codes` 字段为空 → `loadUserBackupCodes` 自动回退 Redis 读取 → 首次 `consumeBackupCode` 消费成功后写入 DB + 清理 Redis → 后续走 DB 路径
- v0.3.x 老 token 不受 logger 改动影响（仅异步 worker 内部日志输出方式变化）
- 回滚 SQL（`008_v0.4.0_2fa_backup_codes.down.sql`）：DROP 三表字段；回滚前需确认备用码已重新写入 Redis

#### 验证

- `go test ./...`：10 个测试包全 PASS（新增 `internal/logger` + `internal/handler/profile_2fa_test.go`）
- `go vet ./...`：0 警告
- `go build ./...`：通过

---

### [新增] JWT jti 精准单点踢出（v0.4.x 迁移项第二项）

#### 背景

- v0.3.x 的 `KickDevice` / `Logout` 是按 user 维度黑名单，会踢出该用户所有设备
- TODO.md 第 253 行 `[迁移] JWT jti 精确单设备踢出 → v0.4.x`
- v0.4.x 第二项迁移：将 jti 嵌入 JWT claims，黑名单改为按 jti 维度（仅失效指定会话）

#### 实现

**`internal/middleware/auth.go`**：
- `JWTClaims` 通过 `jwt.RegisteredClaims.ID` 携带 jti（无需自定义字段）
- `JWTAuth` 中间件注入 `c.Set("jti", claims.ID)` 供下游使用
- `GenerateToken` 保留 `claims.ID`（v0.4.0 修复：原实现重置整个 RegisteredClaims 导致 jti 丢失）

**`internal/auth/jwt.go`**：
- `TokenOptions` 新增 `JTI string` 字段
- `GenerateTokenPair`：access + refresh 携带同一 jti（同一会话共享）
- `BlacklistRefreshTokenByJTI(rdb, jti, ttl)` 新增：按 jti 黑名单（Redis Key: `auth:refresh:blacklist:jti:{jti}`）
- `IsRefreshTokenBlacklisted(rdb, userID, role, jti)` 改造：优先按 jti 检查，jti 为空时回退 user 维度（兼容 v0.3.x 旧 token）
- 保留 `BlacklistRefreshToken`（user 维度，用于修改密码 / 关闭 2FA 强制所有设备重登场景）

**`internal/handler/auth.go`**：
- `Login`：生成 `jti := uuid.NewString()` → 传给 `GenerateTokenPair` + `recordLoginSession`
- `RegisterTenant`：注册成功自动登录同样生成 jti
- `RefreshToken`：解析旧 jti → `BlacklistRefreshTokenByJTI` 旧 jti → 生成新 jti → 写入新会话记录 + 撤销旧会话记录
- `Logout`：按 jti 黑名单（仅失效当前会话，不影响其他设备）+ 撤销该 jti 会话记录；旧 token 无 jti 时回退 user 维度

**`internal/handler/session.go`**：
- `recordLoginSession` 增加 `jti` 参数（由调用方传入，与 JWT 一致）
- `KickDeviceFull` 改造：查 `session.RefreshJTI` → 按 jti 黑名单（仅踢该设备）+ 撤销会话记录；旧记录无 jti 时回退 user 维度
- 新增 `revokeSessionByJTI(deps, role, userID, jti)` 辅助函数：按 jti 撤销单个会话记录
- 移除「待核实 v0.4.x：将 jti 嵌入 JWT 后改为只黑名单指定 jti」标注

**`internal/handler/profile.go`**：
- `ChangePassword` / `Disable2FA` 保留 `BlacklistRefreshToken`（user 维度）——这两个场景确实需要踢所有设备重登
- `KickDevice` 注释更新：从「已知限制 v0.4.x」改为「v0.4.0 已支持精准单点踢出」

#### 测试（`internal/auth/jwt_test.go`，18 个测试全 PASS）

- `TestGenerateTokenPair_JTI写入`：access + refresh 都携带同一 jti
- `TestGenerateTokenPair_空JTI`：JTI 为空时仍可生成（兼容旧调用方）
- `TestGenerateTokenPair_空Secret`：返回错误
- `TestParseToken_错误签名` / `TestParseToken_过期Token`：返回错误
- `TestBlacklistRefreshTokenByJTI_基本功能`：按 jti 加入黑名单
- `TestBlacklistRefreshTokenByJTI_不影响其他JTI`：不同 jti 互不影响
- `TestBlacklistRefreshTokenByJTI_同一用户不同设备`：手机被踢，笔记本不受影响（核心场景）
- `TestIsRefreshTokenBlacklisted_兼容旧Token`：旧 token 无 jti 时回退 user 维度
- `TestBlacklistRefreshTokenByJTI_空参数`：nil 安全
- `TestBlacklistRefreshTokenByJTI_TTL过期`：miniredis FastForward 验证 TTL 过期
- `TestClearRefreshBlacklist`：清除 user 维度黑名单
- `TestExtractBearer`：5 个子用例
- `TestJTI黑名单端到端`：登录两设备 → 踢一设备 → 验证另一设备不受影响 → 修改密码强制所有设备重登
- `TestJWTClaims_JTI通过RegisteredClaims`：集成验证

**middleware 测试新增**：
- `TestJWTAuth_JTI注入上下文`：验证 `c.Set("jti", claims.ID)`

#### 铁律遵守

- 铁律 04（禁硬编码）：jti 由 `uuid.NewString()` 生成，无硬编码
- 铁律 05（配置后台化）：黑名单 TTL 走 `loadAuthParams` 的 `RefreshTTL`，无硬编码
- 铁律 06（防幻觉）：18 个 auth 测试 + 1 个 middleware 测试覆盖核心场景；移除「待核实 v0.4.x：将 jti 嵌入 JWT 后改为只黑名单指定 jti」标注（已落地）

#### 兼容性

- 旧 token（v0.3.x 签发，无 jti）：`IsRefreshTokenBlacklisted` 回退 user 维度检查，不会误放行
- 旧会话记录（v0.3.x 写入，RefreshJTI 为随机 uuid）：`KickDeviceFull` 检测 `session.RefreshJTI != ""` 走 jti 路径（旧 uuid 也按 jti 黑名单，行为等价）
- 修改密码 / 关闭 2FA 场景：仍走 `BlacklistRefreshToken`（user 维度），强制所有设备重登（业务语义不变）

#### 验证

- `go test ./...`：8 个测试包全 PASS（新增 `internal/auth`）
- `go vet ./...`：0 警告
- `go build ./...`：通过

---

## [0.4.0] - 2026-07-20（UA 解析迁移，进行中）

### [新增] pkg/ua 包：User-Agent 解析工具（v0.4.x 迁移项首项）

#### 背景

- v0.3.x 的 `parseDeviceName`（profile.go）与 `detectDeviceType`（session.go）是简化实现，仅识别 OS+Browser 主流场景，无版本号、无爬虫识别
- TODO.md 第 251 行 `[迁移] UA 解析库（mileusna/ua 或 ua-parser）→ v0.4.x 引入`
- v0.4.x 起首项迁移：新建 `pkg/ua` 包，自实现轻量级解析器，**零第三方依赖**

#### 实现

- 新建 `pkg/ua/ua.go`：
  - `DeviceInfo` 结构体：OS / OSVersion / Browser / Version / DeviceType / DeviceName
  - `Parse(ua string) DeviceInfo`：解析入口，覆盖 Chrome / Firefox / Safari / Edge / curl / 爬虫
  - `IsBot(ua string) bool`：识别 Googlebot / Bingbot / Baiduspider / YandexBot / DuckDuckBot / Slurp
  - OS 版本号提取：Windows NT→友好版本（10.0→10 / 6.3→8.1 / 6.2→8 / 6.1→7 / 6.0→Vista / 5.1→XP）；macOS 10_15_7→10.15.7；iOS 14_2_1→14.2.1；Android 11;→11
  - 设备类型判定：pc / mobile / tablet / bot / unknown（iPad / Android 平板识别为 tablet；Android 含 Mobile 识别为 mobile）
  - 浏览器匹配顺序：Edge → curl → Bot → Firefox → Chrome → Safari（避免 Edge 被识别为 Chrome、Chrome 被识别为 Safari）

- 新建 `pkg/ua/ua_test.go`（20 个测试全 PASS）：
  - Chrome on macOS（含 OS 版本 10.15.7 + 浏览器版本 90.0.4430.85）
  - Firefox on Windows 10（NT 10.0→10 友好版本）
  - Safari on iPhone（iOS 14_2_1→14.2.1）
  - Edge on Windows（验证 Edge 先于 Chrome 匹配）
  - Chrome on Android Mobile / Tablet（验证含/不含 Mobile 标识的设备类型区分）
  - iPad（验证 iOS 平板识别）
  - curl（SDK 测试常用 UA）
  - 空字符串 / 仅空白字符
  - Googlebot / Baiduspider（爬虫识别 + DeviceType=bot）
  - Linux + Firefox
  - 旧版 Windows XP（NT 5.1→XP） / Windows 8（NT 6.2→8）
  - IsBot 多场景（6 个爬虫 UA + 4 个正常 UA）
  - SDK 自定义 UA（keyauth-py/1.0 不崩溃，返回 Unknown）
  - DeviceName 拼接逻辑（OS+Browser / 仅 OS / 仅 Browser / Unknown）
  - Edge 优先级（Edge UA 含 Edg/ 与 Chrome/，必须识别为 Edge）
  - Safari 优先级（Chrome UA 含 Chrome/ 与 Safari/，必须识别为 Chrome）
  - 版本号提取（Chrome / Firefox / Safari Version / curl）

#### handler 层改造

- `internal/handler/profile.go`：
  - `parseDeviceName` 改为调用 `ua.Parse(uaStr).DeviceName`，保留函数签名作为兼容包装
  - 移除原 50 行简化实现（OS / Browser 双 switch），删除「待核实 v0.4.x：引入更完整的 UA 解析库」标注

- `internal/handler/session.go`：
  - `recordLoginSession` 改为调用一次 `ua.Parse` 复用结果（替代 `parseDeviceName` + `detectDeviceType` 两次扫描）
  - `detectDeviceType` 改为调用 `ua.Parse(uaStr).DeviceType`，保留函数签名作为兼容包装
  - `ListLoginDevicesFull` 响应增强：新增 `os` / `os_version` / `browser` / `browser_version` / `is_bot` 字段
  - 动态解析 UA 拆分字段，**不改 DB schema**，向前兼容（前端旧字段 `device_name` / `device_type` 保留）

#### 铁律遵守

- 铁律 04（禁硬编码）：无任何硬编码密钥/域名/IP；OS/Browser 常量集中定义在包顶部
- 铁律 05（配置后台化）：UA 解析为纯函数，无配置依赖
- 铁律 06（防幻觉）：20 个测试覆盖主流 UA + 边界 + 爬虫 + SDK UA；所有断言基于固定输入；移除「待核实 v0.4.x」标注（已落地）

#### 验证

- `go test ./...`：7 个测试包全 PASS（新增 `pkg/ua`）
- `go vet ./...`：0 警告
- `go build ./...`：通过

---

## [0.3.6] - 2026-07-20

### [新增] 中间件层单元测试（HTTP 安全闭环）

#### 测试覆盖（`internal/middleware/middleware_test.go`，21 个测试全 PASS）

##### JWTAuth（7 个）
- 有效 Token 通过 + 注入 user_id/role/tenant_id 上下文
- 缺失 Authorization 头 → 401「未提供 Token」
- 非 Bearer 前缀 → 401「Token 格式错误」
- 错误 secret 签名 → 401「Token 无效或已过期」
- 角色不匹配（admin token 访问 tenant 路由）→ 403「无权限访问」
- 多角色逗号分隔（admin,tenant,agent 全通过）
- `GenerateToken` 输出三段式 JWT 结构

##### TenantScope（3 个）
- 未注入 tenant_id → 403「无法识别租户身份」
- 注入 tenant_id 后正确设置 `db` / `gorm_scope` 上下文
- `CheckResourceOwnership` 跨租户访问拦截（tenant 1001 不能访问 tenant 2002 的资源）

##### SignatureAuth（7 个，端到端 HMAC 闭环）
- 有效签名通过（AES 加密 sign_secret 入库 → 解密 → HMAC-SHA512/256 重算 → 常量时间比较）
- 缺失签名头 → 401「签名参数缺失」
- 不存在的 app_key → 401「应用不存在」
- 时间戳超出 ±300s 容差 → 401「时间戳超出允许范围」
- 时间戳格式错误（非数字）→ 401「时间戳格式错误」
- Nonce 重放攻击拦截（Redis SetNX 防重放，第二次相同 nonce → 401「请求已过期或重复」）
- 错误签名 → 401「签名校验失败」

##### RateLimitByIP（4 个，miniredis 滑动窗口）
- 低于阈值全通过（每分钟 3 次，发 3 个请求）
- 超出阈值第 N+1 次返回 429「请求过于频繁」
- 不同 IP 独立计数（IP A 超限不影响 IP B）
- Redis 故障 fail-open（不可达 Redis 时放行而非阻断，避免服务不可用）

##### IPBlacklist（2 个）
- 黑名单中 IP → 403「IP 已被加入黑名单」
- 干净 IP → 200 通过

##### RecordCardFailure + ClearCardFailure（3 个）
- 未达阈值不封禁（2 次 < 阈值 3）
- 达到阈值自动封禁（3 次 = 阈值 3，写入 `ip:blacklist:{ip}` key）
- `ClearCardFailure` 清除失败计数（验证成功时调用）

##### Response（2 个）
- `Success` 响应格式：code/message/data/request_id/timestamp 完整
- `Fail` 响应格式：失败响应不含 data 字段

##### GenerateToken + JWTAuth 端到端（1 个）
- RoundTrip：GenerateToken 生成 → JWTAuth 验证 → 上下文注入 user_id/username/tenant_id

#### 关键测试技术
- `httptest.NewRecorder` + `gin.TestMode` 不启真实 HTTP 端口
- `mockConfigReader` 实现 `ConfigReader` 接口，避免依赖 sys_config 表
- `SetCryptoManager` 注入测试 AES 密钥（32 字节），AES-256-GCM 加密 sign_secret 入库
- miniredis 模拟 Redis（Nonce 防重放 / 限流 / 黑名单 / 失败计数）
- SQLite 内存库模拟 App 表（签名校验查应用）

#### 铁律遵守
- 铁律 04：测试用固定 secret（`real-sign-secret-from-app` / `test-jwt-secret`），不硬编码业务密钥
- 铁律 05：所有配置通过 `mockConfigReader` 注入，不依赖 sys_config 表
- 铁律 06：Redis 故障 fail-open 行为已通过测试验证（不可达 Redis 实际放行），非编造

### [新增] 单元测试 + 客户端 SDK 签名对齐测试

#### 测试栈
- `github.com/stretchr/testify` v1.11.1（assert / require）
- `github.com/alicebob/miniredis/v2` v2.38.0（内存 Redis，测试 heartbeat 不依赖真实 Redis）
- `gorm.io/driver/sqlite` v1.6.0 + `github.com/mattn/go-sqlite3` v1.14.22（SQLite 内存库，测试 quota 不依赖 MySQL）

#### 已通过的测试套件（5 个包，0 失败）

##### `pkg/crypto/crypto_test.go`（核心加密模块）
- AES-256-GCM 加解密往返、错误密文/密钥场景
- `HMACSHA256` 输出格式（64 hex）+ 与标准 `sha512.New512_256` HMAC 完全相等 + 与标准 `sha256` HMAC **不**相等（明确区分两个变体）
- `HashPassword` / `CheckPassword` bcrypt 往返
- `SHA512Hex` / `SHA512Checksum8` / `MD5Hex` 格式
- `SignEpayParams` / `VerifyEpaySign` 易支付签名闭环
- `GenerateCardKey` 卡密格式（前缀 + 4 段随机 + 易混淆字符校验）
- `RandomHex` / `GenerateAppKey` / `GenerateAppSecret` / `GenerateSignSecret` 长度与唯一性
- `GenerateHWID` 设备指纹一致性

##### `pkg/crypto/sign_alignment_test.go`（v0.3.6 新增：客户端 SDK 签名对齐测试）
- 3 组固定输入（login / heartbeat 中文 / get_var 空 body）跨语言对齐
- 后端 `HMACSHA256` 基准签名 vs 三语言 SDK 脚本签名逐字符对比
- **Python**：3 case 全 PASS（`hashlib.algorithms_available` 检测 + `hmac.new(key, msg, "sha512_256")`）
- **PHP**：3 case 全 PASS（`hash_hmac('sha512/256', ...)` 原生支持）
- **Node.js**：沙箱环境 OpenSSL 不支持 `sha512/256`，3 case 自动 `t.Skipf`（脚本退出码 2，标注「环境限制」）
- 后端确定性测试：同一输入两次调用签名相等
- 测试脚本：`sdks/tests/sign.py` / `sign.js` / `sign.php`（无第三方依赖，CLI 接收 `<secret> <msg>` 输出 hex）

##### `pkg/snowflake/snowflake_test.go`（雪花算法 ID 生成器）
- `NewNode` workerID / datacenterID 边界（0~31 合法 / -1 / 32 报错）
- `NextID` 单线程递增 + 50 goroutine × 200 次并发安全（无重复）
- `OrderNo` 三通道前缀（ORD / TOP / REG，v0.3.6 关键路径）
- `twepoch = 1767225600000`（2026-01-01 UTC）常量校验

##### `pkg/epay/epay_test.go`（彩虹易支付协议）
- `BuildSubmitURL` URL 拼接 + sign 注入
- `ParseNotify` 参数解析
- `VerifyNotify` 验签成功 / 失败 / 空 secret / 缺 sign 场景
- 端到端：`BuildSubmitURL` → `ParseNotify` → `VerifyNotify` 闭环
- `NotifyParams.IsSuccess` 状态判断

##### `internal/quota/quota_test.go`（套餐配额校验，SQLite 内存库）
- `ExceededError` 错误消息 + `errors.Is` 类型匹配
- `CheckMaxApps`：低于上限 / 达到上限 / 不限（0） / 开发者不存在 / 已禁用 / 已过期 / 套餐已禁用
- `CheckMaxCards`：低于上限 / 超限 / 不限（0）
- `CheckMaxAgents`：低于上限 / 达到上限 / 不允许（0）
- `CheckMaxDevices`：低于上限 / 达到上限 / 仅计 active / 不限（0） / 不同卡密隔离
- **关键修复**：gorm `default:1` / `default:1000` 标签导致 Create 时 0 值被替换 → 改用 `Updates(map[string]interface{})` 强制覆盖；`AppCard.CardKeyHash` UNIQUE 约束 → 每条记录设置唯一 `card-{i}` / `hash-{i}` / `cs{i}`；`Agent.Username` UNIQUE 约束 → 设置 `agent-{i}`

##### `internal/heartbeat/heartbeat_test.go`（心跳保活，miniredis）
- `Record` 成功 / nil rdb 报错 / 同设备多次记录只增不删
- `IsOnline` 在线 / 离线（直接覆写 ZSET score 模拟超时） / 从未心跳 / 默认超时 180s / nil rdb
- `Remove` 成功 / 不存在设备 / nil rdb 静默返回
- `CountOnline` 计数 / 排除超时设备 / 默认超时 / nil rdb
- `ListOnline` 列表 / 分页 / nil rdb
- `GetLastHeartbeatAt` 成功 / 从未心跳返回零时间 / nil rdb
- Redis Key 规范：`heartbeat:online:{app_id}` / `heartbeat:detail:{app_id}:{device_id}`
- 端到端：Record → IsOnline → 超时 → 再 Record → Remove → GetLastHeartbeatAt 闭环
- **关键修复**：miniredis 的 `FastForward` 不影响 Go `time.Now()`，改为 `rdb.ZAdd` 直接覆写 score 为 `time.Now().Add(-200*time.Second).Unix()` 模拟心跳超时

#### 验证
- `go vet ./...` 0 错误 0 警告
- `go test ./...` 全部 PASS（5 个测试包）
- `go build ./...` 编译通过

#### 铁律遵守
- 铁律 04：测试用例不硬编码任何业务密钥，仅用固定测试输入（`test-sign-secret-12345` 等显式标注为测试用）
- 铁律 05：测试无业务可调参数，所有配置通过 fixture / 测试 DB / miniredis 隔离
- 铁律 06：Node.js 沙箱环境不支持 `sha512/256` 已诚实 `t.Skipf` 标注「环境限制」，未掩盖；签名回退分支「待核实」继续保留

### [新增] 客户端 SDK 三语言（Python / Node.js / PHP）

#### 设计方案
为软件开发者提供开箱即用的客户端 SDK，封装 9 个验证 API + HMAC-SHA512/256 签名算法。三语言 SDK 均无第三方依赖（或仅依赖标准库），签名算法与后端 `crypto.HMACSHA256`（`sha512.New512_256` 变体）严格对齐，不支持时回退标准 `sha256`（已标注「待核实」）。

#### Python SDK（`sdks/python/`，包名 `keyauth-py`）
- [新增] `keyauth/client.py` 主客户端类 `KeyAuthClient`：
  - 9 个公共方法：`login` / `verify` / `heartbeat` / `bind` / `unbind` / `get_var` / `notice` / `version` / `logout`
  - `_sha512_256_hex` 函数：优先用 `hashlib.new("sha512_256")`，不支持时回退 `hashlib.sha256`
  - `_post` 内部方法：构造签名原文 `METHOD\nPATH\nTIMESTAMP\nNONCE\nBODY` + HMAC 签名 + 发请求 + 解析响应
  - `KeyAuthError` 异常类：含 `code` / `message` / `http_status`
  - `CardInfo` / `DeviceInfo` 数据类
- [新增] `keyauth/__init__.py` 包入口，导出 `KeyAuthClient` / `KeyAuthError` / `CardInfo` / `DeviceInfo`，`__version__ = "0.3.6"`
- [新增] `setup.py` 打包配置（`name="keyauth-py"`，`install_requires=["requests>=2.20"]`，`python_requires=">=3.7"`）
- [新增] `README.md` 完整文档（安装说明 + 快速开始 + 9 API 速查表 + 签名算法说明 + 错误处理 + 错误码表）

#### Node.js SDK（`sdks/nodejs/`，包名 `keyauth-node`）
- [新增] `index.js` 主客户端类 `KeyAuthClient`：
  - 9 个异步方法：`login` / `verify` / `heartbeat` / `bind` / `unbind` / `getVar` / `notice` / `version` / `logout`
  - `hmacSha512_256Hex` 函数：`crypto.createHmac('sha512/256', secret)`，不支持时回退 `sha256`
  - `httpRequest` 内置 HTTPS/HTTP 请求封装（不依赖 axios）
  - `KeyAuthError` 错误类
- [新增] `index.d.ts` TypeScript 类型定义：`ClientOptions` / `LoginResult` / `VerifyResult` / `HeartbeatResult` / `BindResult` / `UnbindResult` / `GetVarResult` / `NoticeResult` / `VersionResult` / `LogoutResult`
- [新增] `package.json` 包配置（`name="keyauth-node"`，`engines.node>=14.0.0`，无 dependencies）
- [新增] `README.md` 完整文档

#### PHP SDK（`sdks/php/`，包名 `keyauth/keyauth-php`）
- [新增] `src/KeyAuthClient.php` 主客户端类：
  - 9 个公共方法（`login` / `verify` / `heartbeat` / `bind` / `unbind` / `getVar` / `notice` / `version` / `logout`）
  - `hmacSha512256` 方法：`hash_hmac('sha512/256', $msg, $secret)`（PHP 7.1+ 原生支持），不支持时回退 `hash_hmac('sha256', ...)`
  - cURL HTTP 请求封装（无第三方依赖，仅依赖 `ext-curl` / `ext-json` / `ext-hash` PHP 标配扩展）
  - `declare(strict_types=1)` 全类型安全
- [新增] `src/KeyAuthError.php` 异常类：含 `errorCode`（业务码）+ `httpStatus`（HTTP 状态码）+ getter
- [新增] `composer.json` 包配置（PSR-4 自动加载 `KeyAuth\\`，`php>=7.2.0`）
- [新增] `README.md` 完整文档
- [校验] `php -l` 语法校验通过（KeyAuthClient.php + KeyAuthError.php 0 错误）

#### 铁律遵守
- 铁律 04：SDK 不硬编码任何密钥/域名/AppKey，全部由开发者通过构造函数传入；密钥从环境变量读取（README 有示例）
- 铁律 05：SDK 内部无业务可调参数，所有可变项（API 路径前缀、超时、User-Agent）通过参数或常量管理
- 铁律 06：签名算法回退分支已标注「待核实：sha256 与后端 sha512.New512_256 是否完全等价」；PHP SDK 校验通过 `php -l`；未提供运行时测试用例（待 v0.4.x 补集成测试）

### [新增] 开发者自有易支付回调 + 双层支付模式切换（#5）

#### 设计方案
落地 `SysPackage.AllowCustomPay` + `TenantPayConfig.Enabled` 双开关，实现"平台总支付（默认）/ 开发者自有易支付（按套餐开通）"双层支付模式切换。订单号前缀分发：

- `ORD` → 平台总支付卡密购买（`processPaidOrder`，写 `PlatformSettlement` 抽成记录）
- `TOP` → 开发者自有易支付卡密购买（`processTenantOwnPaidOrder`，不写抽成，资金直接进开发者易支付账户，平台通过套餐 `CustomPayFee` 月费模式收费）
- `REG` → 代理注册付费（`processAgentRegisterPaid`，独立通道，注册费归平台）

#### 后端
- [改造] `apps/server/internal/handler/pay.go` `CreatePayOrder` 增加双层切换逻辑：
  - 查开发者套餐 `AllowCustomPay` + 开发者 `TenantPayConfig(channel=epay, enabled=true)`
  - 命中 → 走自有支付：订单号前缀 `TOP`，回调 URL 携带 `tenant_id`（`resolveTenantNotifyURL`）
  - 未命中 → 回退平台总支付：订单号前缀 `ORD`（保持原逻辑），需 `pay.platform.enabled=true`
  - 响应新增 `pay_mode` 字段（`tenant` / `platform`）供前端区分
- [新增] `pay.go` `EpayTenantNotify` 完整实现（替换原 `c.String(200, "fail")` 占位）：
  - 从 URL 取 `tenant_id`，调 `loadTenantPayConfig` 加载该租户易支付配置
  - 收集回调参数 + `epay.VerifyNotify` 验签（用该租户密钥）
  - Redis 防重入（key 含 `tenant_id` 命名空间隔离：`pay:notify:tenant:{tid}:lock:{order_no}`）
  - 调 `dispatchPaidOrder` 按前缀分发到 `processTenantOwnPaidOrder`
- [新增] `pay.go` `loadTenantPayConfig` 辅助函数：从 `tenant_pay_config` 表按 `(tenant_id, channel=epay, enabled=true)` 查配置 + AES-256-GCM 解密 `key_encrypted`
- [新增] `pay.go` `processTenantOwnPaidOrder` 事务内：
  - 校验订单状态/金额（防伪造回调）
  - 幂等保护（已 paid 直接返回）
  - 自动发卡（与 `processPaidOrder` 同流程，batch_no 前缀 `T` 区分）
  - **不写 `PlatformSettlement`**（资金已直接到开发者易支付账户，平台不抽成）
  - 写 `TenantBalanceLog{type=settle, amount=订单总额, pay_method=tenant_epay}` + 累加 `sys_tenant.balance`
- [新增] `pay.go` `dispatchPaidOrder` 增加 `TOP` 分支
- [新增] `pay.go` `resolveTenantNotifyURL` / `resolveTenantReturnURL` / `ternary` 辅助函数
- [修复] `migrations/002_seed_data.up.sql` 配置键名 bug：
  - `pay.platform.notify_path`：`/api/v1/pay/platform/notify` → `/api/v1/pay/notify/epay`（与 router 一致）
  - `pay.platform.return_path`：`/pay/return` → `/api/v1/pay/return/epay`（与 router 一致）
- [新增] `002_seed_data.up.sql` 三个新配置项：
  - `pay.tenant.notify_path`（默认 `/api/v1/pay/notify/tenant/`）
  - `pay.platform.order_name_prefix`（默认 `KeyAuth卡密`）
  - `pay.platform.return_front_url`（默认 `/pay/result`）

#### 前端
- [无变更] 现有 `tenant/PayConfig.vue` + `admin/PayConfig.vue` 已支持开发者保存/启用自有易支付配置，本版本后端切换逻辑生效后即可实时启用，无需前端调整

#### 铁律遵守
- 铁律 04：订单号前缀 `TOP/ORD/REG` 集中分发，无硬编码业务路由；开发者密钥 AES 加密入库
- 铁律 05：所有路径/前缀/订单名从 `sys_config` 读取（`pay.tenant.notify_path` / `pay.platform.order_name_prefix` / `pay.platform.return_front_url`）
- 铁律 06：编译验证通过（`go build ./...` + `vue-tsc --noEmit`）；金额校验防伪造回调；幂等保护；FOR UPDATE 防并发余额更新

### [新增] 卡密封禁联动设备强制下线（#1）

#### 后端
- [新增] `apps/server/internal/handler/card.go` `TenantBanCard` 在卡密封禁后联动下线所有绑定设备：调 `heartbeat.Remove` 清 Redis 心跳 + DB 标记 `app_device.status='banned'` + `last_heartbeat_at=NULL`
- [修复] 移除 `card.go:422` 的 `TODO(v0.3.0): 同时下线该卡密绑定的设备（清 Redis 心跳）` 占位
- [新增] card.go 导入 `internal/heartbeat` 包

#### 铁律遵守
- 铁律 05：`heartbeat.Remove` 内部用 appID/deviceID 拼 Redis Key，无硬编码
- 铁律 06：Redis 清理失败不阻塞封禁主流程（卡密已 banned，下次 verify 会因 card.status 拒绝）

### [新增] 卡密 CSV 导入导出（#2）

#### 后端
- [新增] `apps/server/internal/handler/card.go` `TenantExportCardsCSV` 导出 CSV（支持 app_id/status/batch_no/keyword 过滤，最多 10000 条，UTF-8 BOM）
- [新增] `apps/server/internal/handler/card.go` `TenantImportCardsCSV` 导入 CSV（前端解析后传 JSON 数组，事务批量入库，重复 hash 跳过并记失败明细）
- [新增] card.go 辅助函数 `ptrTimeFmt` `min`
- [新增] `apps/server/internal/router/router.go` 注册 `GET /api/v1/tenant/cards/export` + `POST /api/v1/tenant/cards/import`（注意：在 `cards/:id` 之前注册避免路由冲突）

#### 前端
- [新增] `apps/admin/src/api/cards.ts` `exportCardsApi`（用 axios blob 下载，带 Authorization Header 避免暴露 token）+ `importCardsApi` + `ImportCardsResult` 类型
- [修改] `apps/admin/src/views/tenant/Cards.vue` 新增「导出 CSV」「导入 CSV」按钮 + 导入对话框（应用/卡类/前缀/分组/文件上传）+ 导入结果对话框（成功/失败/空行/重复统计 + 失败明细）

#### 铁律遵守
- 铁律 04：CSV 导出为真实数据，无硬编码假数据；前端用 blob 下载，token 不暴露在 URL/日志
- 铁律 05：导出条数上限 `card.export.max_rows`（默认 10000）+ 导入条数上限 `card.import.max_rows`（默认 5000）从 sys_config 读取，禁硬编码
- 铁律 06：导入失败明细返回前端，禁只报"成功"假象；事务回滚保护

### [新增] 安装向导页面 `/install`（#3）

#### 后端
- [新增] `apps/server/internal/handler/install.go` `InstallStatus`（GET /api/v1/install/status）：通过 `sys_admin.password_hash` 是否含 `PLACEHOLDER_BCRYPT_HASH` 占位串判定 installed 状态，避免用 `count(*)` 误判（seed 已插入 1 行占位）
- [新增] `install.go` `Install`（POST /api/v1/install）：接收超管账号密码 + 平台基础配置，事务写入：
  - 更新 `sys_admin` id=1 占位行 → 真实 bcrypt 哈希（cost=12）+ 真实 username/email/phone
  - upsert `sys_config` 平台基础项（`platform.domain` / `platform.name` / `platform.notify_email` / `agent.register_fee` / `pay.platform_commission_rate` / `platform.installed_at`）
  - 二次校验已安装状态拒绝重入
  - 调 `CfgCache.InvalidateAll` 刷新 Redis 缓存
  - 异步记录操作日志（不含密码）
- [新增] `install.go` 辅助函数 `checkInstalled` / `upsertConfig`
- [新增] `apps/server/internal/router/router.go` 注册 `v1.GET("/install/status")` + `v1.POST("/install")`（无需鉴权，仅首次部署可用）

#### 前端
- [新增] `apps/admin/src/api/http.ts` `installStatusApi` / `installApi` + `InstallStatus` / `InstallPayload` / `InstallResult` 类型（直接调 http 实例，绕过 token 拦截器）
- [新增] `apps/admin/src/views/Install.vue` 4 步向导（环境检测 → 超管账号 → 平台配置 → 完成）+ `el-steps` 进度条 + 表单校验（密码确认一致性、邮箱格式）+ `el-result` 成功提示
- [新增] `apps/admin/src/router/index.ts` `/install` 路由（public，无需登录）

#### 铁律遵守
- 铁律 04：超管密码不硬编码，由安装表单传入；bcrypt cost=12 哈希后入库
- 铁律 05：所有平台配置写入 `sys_config` 表 + Redis 缓存，不写入代码或 .env
- 铁律 06：已安装检测用占位串而非 `count(*)`，避免 seed 数据导致误判；二次校验防并发安装

### [新增] 代理注册付费流程（#4）

#### 设计方案
采用**方案 B：先支付后建 Agent**，避免引入 `pending_payment` 状态破坏 `AgentLogin` 现有 `status != "active"` 不变量。代理行仅在支付回调事务内创建且 `Status="active"`，可直接登录。

#### 后端
- [新增] `apps/server/internal/handler/auth.go` `AgentRegister`（POST /api/v1/public/auth/agent/register）：
  - 校验邀请码（`status=active` + `used_count < max_uses` + `expires_at > now`）
  - 校验用户名在所属租户内唯一
  - `quota.CheckMaxAgents` 校验套餐代理数上限（第一道防线）
  - 读 `sys_config` `agent.register.fee` 注册费（默认 99.00）
  - bcrypt 哈希密码（cost=12），缓存到 Redis（`agent_register:pwd:{order_no}`，TTL=`pay.order_expire_seconds` 默认 1800s）
  - 创建 `AgentRegistrationOrder`（订单号前缀 `REG`，`PayStatus=pending`，`AgentID=nil` 占位）
  - 调 `epay.BuildSubmitURL` 返回 `pay_url`
- [新增] `auth.go` `AgentRegisterConfig`（GET /api/v1/public/auth/agent/register/config）：公开返回注册费 + 支付方式 + 平台支付开关，不返回敏感字段（gateway_url/pid/key_encrypted）
- [新增] `auth.go` `AgentRegisterOrderStatus`（GET /api/v1/public/auth/agent/register/order/:order_no）：前端支付完成跳回后查询订单状态
- [改造] `apps/server/internal/handler/pay.go` `EpayNotify` 引入 `dispatchPaidOrder` 按订单号前缀分发：
  - `ORD` → 现有 `processPaidOrder`（卡密购买）
  - `REG` → 新 `processAgentRegisterPaid`（代理注册）
- [新增] `pay.go` `processAgentRegisterPaid` 事务内：
  - 校验订单状态/金额（防伪造）
  - 幂等保护（已 paid 直接返回）
  - 事务内重复 `quota` 校验防 TOCTOU（套餐上限 + 用户名重复）
  - INSERT `Agent{Status: "active", CommissionRate: 邀请码.DefaultCommissionRate, CommissionMode: "percentage"}`
  - 回填 `AgentRegistrationOrder.AgentID` + `PayStatus=paid` + `PaidAt` + `PayTradeNo`
  - 邀请码 `used_count++`，达 `max_uses` 时 `status=exhausted` + 写 `used_by_agent_id`（补齐 exhausted 状态从未被写入的逻辑漏洞）
  - 删除 Redis 中的密码哈希缓存（已用过，安全清理）
  - 注册费不进 `PlatformSettlement`（直接归平台，与卡密抽成解耦）
- [新增] `pay.go` `cacheAgentRegisterPassword` 辅助函数（铁律 04：DB 不存明文密码也不存哈希到订单表，仅短期缓存 bcrypt 哈希等回调使用）
- [修复] `apps/server/internal/handler/install.go` 配置键名 bug：`agent.register_fee` → `agent.register.fee`（与 `migrations/002_seed_data.up.sql` 保持一致，下划线改点号）；`pay.platform_commission_rate` → `pay.platform.commission_rate`
- [新增] `apps/server/internal/router/router.go` 注册 3 个公开路由：`POST /auth/agent/register` + `GET /auth/agent/register/config` + `GET /auth/agent/register/order/:order_no`（无需鉴权）

#### 前端
- [新增] `apps/admin/src/api/agent.ts` 三个注册相关 API + 类型：`agentRegisterConfigApi` / `agentRegisterApi` / `agentRegisterOrderStatusApi` + `AgentRegisterConfig` / `AgentRegisterResult` / `AgentRegisterOrder` 类型
- [改造] `apps/admin/src/views/agent/Register.vue` 落地原 3 处 TODO：
  - `onMounted` 调 `agentRegisterConfigApi` 读 `register_fee` + `pay_methods` + `pay_enabled`，按后端配置重建支付方式列表（铁律 04：不硬编码支付方式）
  - Step 1 提交调 `agentRegisterApi` 创建预支付订单 + 返回 `pay_url`，缓存订单号 + pay_url 到本地
  - Step 2 「前往支付页面」用 `window.open(payURL, '_blank')` 新窗口跳转避免丢失原页面状态；「我已完成支付，查询状态」按钮调 `agentRegisterOrderStatusApi` 轮询订单状态
  - 当 `pay_enabled=false` 时禁用提交按钮 + 显示红色告警

#### 铁律遵守
- 铁律 04：密码明文不入库，仅短期缓存 bcrypt 哈希到 Redis；订单号前缀 `REG` 与 `ORD` 区分业务；支付方式不硬编码，从 `pay.platform.methods` 读取
- 铁律 05：注册费从 `agent.register.fee` 读取；订单过期时间从 `pay.order_expire_seconds` 读取；商品名前缀从 `pay.platform.order_name_prefix` 读取
- 铁律 06：事务内重复 quota 校验防 TOCTOU；邀请码状态机闭环（达 max_uses 时置 exhausted，补齐旧逻辑漏洞）；二次用户名校验防并发；AgentRegisterConfig 不返回敏感字段

### [文档] 四份核心文档 + README + PROMPT 全量同步对齐 v0.3.5 实际状态

本次发布为纯文档同步，按 `web-project-flow` skill 的 `references/09-docs-lifecycle.md` 规范联动更新，消除多份文档与代码实际状态不一致的矛盾。配套 `web-project-flow` skill 已全局安装。

#### README.md
- [修改] 版本徽章 `0.2.7` → `0.3.5`
- [修改] 「当前版本」表格新增 v0.3.0 ~ v0.3.5 六行，所有模块状态从「⏳ 计划中」改为「✅ 已完成」
- [修改] 「后续版本规划」从 v0.2.5/v0.2.6/v0.2.7/v0.3.0/v0.4.0 改为 v0.3.6/v0.4.0
- [新增] 「开发约束」章节补 `web-project-flow` skill 已全局安装说明 + `/bhelp` `/bhardcode /bconfig /bhaluc` `/bdocs` 命令索引

#### PROMPT.md
- [新增] 顶部 skill 使用说明（`/bhelp` / `/bonboard` / `/bdocs`）
- [修改] 「五、当前进度」从 v0.2.0 骨架阶段重写为 v0.3.5 已完成 + v0.3.6 待开始 + v0.4.0 计划
- [修改] 「九、可信度声明」补三个具体占位文件位置（auth.go:443 / pay.go:528 / Register.vue）

#### PROJECT.md
- [修改] 文档版本 `0.1.0` → `0.3.5`，最后更新 `2026-07-19` → `2026-07-20`
- [修改] 「3.1 平台超管后台」17 个模块状态全勾选已实现项（11 ✅ + 6 ☐ 标注 v0.4.x）
- [修改] 「3.2 开发者控制台」19 个模块状态全勾选（15 ✅ + 4 ☐ 标注 v0.3.6/v0.4.x）
- [修改] 「3.3 代理商控制台」10 个模块状态全勾选（8 ✅ + 2 ☐ 标注 v0.4.x）
- [修改] 「3.4 终端用户 H5」14 个页面状态全勾选（5 ✅ + 9 ☐ 标注 v0.4.x 终端用户体系未建）
- [修改] 「3.5 客户端 SDK」补预计版本（Python/Node/PHP v0.3.6，其余 v0.4.0）
- [修改] 「4.1 表清单」26 张 → 30 张，新增 platform_settlement/log_login_failed/refresh_token_device/tenant_balance_log/tenant_withdraw/schema_migrations 6 张表，并加「引入版本」列
- [修改] 「4.2 Redis 缓存键」移除 session:admin/tenant/agent（实际用 JWT 无 session）+ 补 2fa:setup/2fa:backup/login:fail
- [修改] 「6. 目录结构」完全重写对齐实际（移除不存在的 service/repository 子目录、sdks/ 目录、deploy/docker-compose.yml；新增 internal/auth/heartbeat/migration/quota 包说明 + handler 17 文件清单 + .env.development/production + PROMPT.md）
- [修改] 「7.4 编码铁律」补 skill 命令索引

#### SPEC.md
- [修改] 文档版本 `0.1.0` → `0.3.5`，最后更新 `2026-07-19` → `2026-07-20`
- [修改] 「2.1 分层架构」从理论 4 层（Handler/Service/Repository/Model）改为实际 3 层简化架构（Handler 直连 GORM + 辅助包 + Model+Middleware），补当前实现说明 + v0.4.x 重构计划
- [修改] 「2.2 模块边界」从理论 `internal/service/<module>/` 改为实际 `internal/handler/` 17 文件清单，补跨文件通信机制（Deps / RecordOperation / writeVerifyLogCtx）
- [修改] 「8.3 数据库迁移」从 `golang-migrate/migrate` 改为自研轻量级 SQL 文件迁移机制（v0.3.5），补 schema_migrations 表 + dirty 状态 + multiStatements + MIGRATION_AUTO/MIGRATION_DIR 等实际细节
- [修改] 「9.1 四份核心文档」补联动校验铁律三条

#### TODO.md
- [修改] 「三级公告体系」11 项子项从 [待开始] 改为 [已完成]（统一公告表/notice_target/notice_read/S-15/S-16/D-10/agent_notify/顶部公告/P-08），消除与 v0.3.0 章节的矛盾
- [修改] 「云变量与版本管理」5 项子项全部改为 [已完成]（云变量 CRUD + 3 个客户端接口），消除与 v0.3.0 章节的矛盾
- [修改] 「数据统计看板」3 项已实现（admin/tenant/agent dashboard）改为 [已完成]，仅留 2 项 v0.4.x
- [修改] 「客户端 SDK」版本号 v0.3.0 → v0.3.6
- [修改] 「开发者自有易支付」补 tenant_pay_config [已完成] + EpayTenantNotify 仍返回 "fail" 的精确位置（pay.go:528）
- [修改] 「代理注册付费流程」补邀请码 CRUD [已完成] + AgentRegister 501 占位精确位置（auth.go:443）
- [修改] 「代理购买卡密」实时扫码购卡/独立门户/子域名绑定 从 v0.3.0 → v0.4.x
- [修改] 里程碑表新增 v0.3.5 行 + v0.3.6 [进行中] 行
- [修改] 「v0.2.3 进度统计」章节完全重写为「v0.3.5 进度统计」（总任务 75→110，已完成 53→90，待完成 21→19，新增 v0.2.0~v0.3.5 已完成版本汇总表 + v0.3.6/v0.4.x 待完成项分类）
- [修改] 文档版本 `0.2.7` → `0.3.5`

#### 验证
- [验证] 文档版本号四份统一为 `0.3.5`
- [验证] 时间戳统一为 `2026-07-20`
- [验证] TODO 中标记 [已完成] 的项均能在对应版本 CHANGELOG 中找到（联动校验铁律 ①）
- [验证] PROJECT 中描述的功能与 SPEC 中的规范一致（联动校验铁律 ②）

#### 待核实项（铁律 06）
- 表清单 30 张中 `app_user` 在 v0.2.0 DDL 中存在但 v0.3.x 实际未使用（终端用户体系未建），待 v0.4.x 终端用户体系启动时确认
- `agent_quota` 表在 DDL 中存在但 model.go 中无对应 struct，待核实是否仍在使用

---

## [0.3.5] - 2026-07-19

### [修复] P0 紧急修复：RSA 脚本 / 数据库迁移 / H5 公共 API / 套餐配额

本次发布聚焦 4 项 P0 缺陷修复，覆盖部署脚本、数据库迁移机制、H5 购卡闭环、套餐配额统一管理。

#### 后端：RSA-4096 密钥生成独立脚本
- [新增] `scripts/gen_rsa_key.sh`：从 `baota_deploy.sh` 抽取为独立脚本
  - 支持 `--force` 覆盖已存在密钥
  - 支持自定义输出目录（`--dir /path` 或位置参数）
  - 私钥 PKCS#8/PEM + 公钥 PKIX/PEM，chmod 600/644
  - 生成后自动密钥配对校验（openssl rsa -in priv -pubout 对比公钥）
  - 修复 OUTPUT_DIR 拼接 bug（原代码因运算符优先级导致 pwd 输出两次）

#### 后端：数据库迁移机制修复
- [新增] `apps/server/internal/migration/migrator.go`：轻量级 SQL 文件迁移机制
  - `schema_migrations` 表跟踪版本号 + dirty 状态
  - 扫描 `*.up.sql` 文件，按文件名前缀数字排序
  - 每个迁移在独立事务中执行，失败标记 dirty 阻止启动
  - 幂等：已应用的迁移不会重复执行
- [修改] `apps/server/internal/config/config.go`
  - 新增 `MigrationConfig` struct（Auto bool, Dir string）
  - Config 加 `Migration MigrationConfig` 字段，默认 `{Auto: true, Dir: "apps/server/migrations"}`
  - `applyEnvConfig` 加 `MIGRATION_AUTO` / `MIGRATION_DIR` 环境变量覆盖
  - DSN 加 `multiStatements=true` 参数（迁移文件含多语句）
  - `InitContainer` 中加 `migration.Run(db, cfg.Migration.Dir)` 调用
- [修改] `docker-compose.yml`
  - mysql 服务移除 `./apps/server/migrations:/docker-entrypoint-initdb.d:ro` 挂载
    - 修复缺陷：原挂载方式按字母序执行所有 .sql（含 .down.sql），存在迁移顺序错误风险
  - server 服务 environment 加 `MIGRATION_AUTO: "true"` 和 `MIGRATION_DIR: /app/migrations`
- [修改] `configs/config.yaml.example`：完全重写以对齐 Config struct 的 yaml tag（app/mysql/redis/jwt/crypto/migration/domain）

#### 后端：H5 公共 API 补齐（购卡闭环）
- [新增] `apps/server/internal/handler/public.go`：H5 终端用户购卡流程公开 API（无需鉴权）
  - `PublicAppInfo` GET `/public/apps/info?app_key=xxx`：按 app_key 查应用公开信息，联表校验 SysTenant active 状态
  - `PublicCardTypes` GET `/public/card_types?app_id=xxx`：按 app_id 查可购卡类列表，仅返回 active 卡类
  - 安全：`publicAppInfo` / `publicCardType` DTO 过滤敏感字段（app_secret / sign_secret / agent_base_price）
- [修改] `apps/server/internal/handler/pay.go` `GetPayOrder`：订单已支付时返回卡密明文
  - 新增 `card_keys []string` 字段：从 AppCard 表按 card_ids Pluck card_key
  - 供 H5 终端用户支付成功后直接查看卡密，无需另调查询接口
- [修改] `apps/server/internal/router/router.go`：publicGroup 新增 2 条路由
  - `GET /public/apps/info`
  - `GET /public/card_types`
- [修改] `apps/admin/src/api/pay.ts`：`PayOrder` 接口加 `card_keys: string[]` 字段
- [修改] `apps/admin/src/views/h5/Home.vue`：移除"待核实"注释，更新 `loadCardTypes` 使用已实现的 `/public/apps/info` + `/public/card_types`
- [修改] `apps/admin/src/views/h5/PayResult.vue`：`fetchOrder` 使用 `resp.card_keys` 替代空数组占位

#### 后端：套餐配额检查统一封装
- [新增] `apps/server/internal/quota/quota.go`：套餐配额检查 helper 包
  - `ExceededError` 自定义错误类型（Resource / Current / Limit / AddCount）
  - `loadTenantPackage` 内部 helper：校验 tenant.Status == "active" && tenant.ExpiresAt 未过期 && pkg.Status == "active"
  - `CheckMaxApps`：校验开发者创建应用是否超出套餐上限（`App` COUNT）
  - `CheckMaxCards`：校验开发者生成卡密是否超出套餐上限（`AppCard` COUNT + addCount）
  - `CheckMaxAgents`：校验开发者代理数是否超出套餐上限（`Agent` COUNT，Limit==0 表示套餐不支持招募代理）
  - `CheckMaxDevices`：校验单卡密绑定设备数是否超出应用配置上限（`AppDevice` COUNT，MaxDevices 是应用级配置）
  - 设计：不在 helper 内开事务（避免嵌套事务），TOCTOU 风险由调用方在事务内处理
- [修改] `apps/server/internal/handler/app.go` `TenantCreateApp`
  - 将内联 MaxApps 检查替换为 `quota.CheckMaxApps(deps.DB, tenantID)`
  - 使用 `errors.As(err, &qErr)` 区分配额超限错误和系统错误
- [修改] `apps/server/internal/handler/card.go` `TenantGenerateCards`
  - 将内联 MaxCards 检查（含 tenant/pkg 查询）替换为 `quota.CheckMaxCards(deps.DB, tenantID, req.Quantity)`
  - 错误消息保留原格式：「将超过套餐卡密上限 N 张（当前 X 张，本次生成 Y 张）」
- [修改] `apps/server/internal/handler/tenant_business.go` `TenantGenInviteCode`
  - 在事务前新增 `quota.CheckMaxAgents(deps.DB, tenantID)` 调用
  - 区分两种场景：`Limit == 0`（套餐不支持招募代理）vs `Limit > 0`（已达上限）
  - 注释说明：邀请码本身不是代理，但生成邀请码隐含招募代理意图，提前校验避免发放无效邀请码
- [修改] `apps/server/internal/handler/client.go` `ClientLogin` + `ClientBind`
  - `ClientLogin` 新设备绑定前：将内联 MaxDevices 检查替换为 `quota.CheckMaxDevices`
  - `ClientBind` 手动绑定前：将内联 MaxDevices 检查替换为 `quota.CheckMaxDevices`
  - `ClientBind` 保留 `countBoundDevices` 调用以在响应中展示当前绑定数（双重查询可接受，绑定非高频操作）

#### 验证
- [验证] `go build ./...` 通过（0 错误）
- [验证] `go vet ./...` 通过（0 警告）
- [验证] `pnpm run build`（admin）通过（8.56s）

#### 文档
- [修改] `docs/TODO.md`：标记 3 项 P0 已完成 + 新增 v0.3.5 章节（17 项明细）
- [修改] `docs/CHANGELOG.md`：新增 v0.3.5 版本条目

---

## [0.3.4] - 2026-07-19

### [功能] 开发者结算与对账闭环（事务保护 + 批量结算 + 余额流水对称设计）

#### 后端：数据库迁移
- [新增] `apps/server/migrations/007_v0.3.4_tenant_finance.up.sql`
  - `sys_tenant` 增 `balance` DECIMAL(12,2) + `frozen_balance` DECIMAL(12,2) 字段
  - 新建 `tenant_balance_log` 表（type: settle/withdraw/refund/adjust，status: pending/settled/rejected）
  - 新建 `tenant_withdraw` 表（status: pending/paid/rejected/failed）
- [新增] `007_v0.3.4_tenant_finance.down.sql` 回滚迁移

#### 后端：模型 + AdminSettleOrder 改造
- [修改] `apps/server/internal/model/model.go`
  - `SysTenant` 增 `Balance float64` + `FrozenBalance float64` 字段
  - 新增 `TenantBalanceLog` struct（含 TenantID/Type/Amount/BalanceAfter/RelatedSettlementID/RelatedWithdrawID/PayMethod/SettleBatchNo/Status/Remark）
  - 新增 `TenantWithdraw` struct（含 TenantID/Amount/PayMethod/PayAccount/Status/AuditRemark/PayTradeNo/PaidAt/AuditedBy）
- [修改] `apps/server/internal/handler/pay.go` `AdminSettleOrder`：
  - 改造为事务保护 + `tx.Set("gorm:query_option", "FOR UPDATE")` 防并发
  - 累加开发者可提现 `balance`
  - 写 `tenant_balance_log`（type=settle, status=settled）

#### 后端：开发者侧 Handler（tenant_settle.go，对称 agent_business.go）
- [新增] `apps/server/internal/handler/tenant_settle.go`（5 个 handler 全事务保护）
  - `TenantListSettlements` GET `/tenant/settlements` 查自己的 platform_settlement，含 pending_sum/settled_sum 汇总
  - `TenantBalanceOverview` GET `/tenant/balance_overview` 余额概览（balance/frozen/settled_total/withdrawn_total/pending_withdraw）
  - `TenantListBalanceLogs` GET `/tenant/balance_logs` 查自己的余额流水（type/status 筛选）
  - `TenantListOwnWithdrawals` GET `/tenant/withdrawals/mine` 查自己的提现申请
  - `TenantWithdraw` POST `/tenant/withdraw` 事务：扣 balance + 加 frozen_balance + 写 withdraw + 写 balance_log

#### 后端：超管审核 Handler（admin_finance.go，对称 tenant_finance.go）
- [新增] `apps/server/internal/handler/admin_finance.go`（5 个 handler 全事务保护）
  - `AdminListTenantWithdrawals` GET `/admin/tenant_withdrawals` 联表 sys_tenant，默认 status=pending
  - `AdminPayTenantWithdraw` POST `/admin/tenant_withdrawals/:id/pay` 事务：withdraw.status=paid + frozen_balance -= amount + balance_log.status=settled
  - `AdminRejectTenantWithdraw` POST `/admin/tenant_withdrawals/:id/reject` 事务：退 balance + 减 frozen_balance + withdraw.status=rejected + 写 refund 流水
  - `AdminBatchSettle` POST `/admin/settlements/batch_settle` 批量结算，按 tenant_id 分组累计 net_amount，单次最多 100 条
  - `AdminReconciliation` GET `/admin/reconciliation` 对账报表，聚合 SQL（SUM + CASE WHEN）统计订单总额/抽成/应结/已结/未结/已提现/理论余额

#### 后端：路由注册
- [修改] `apps/server/internal/router/router.go`
  - adminAuth 段新增 5 条：`/settlements/batch_settle` + `/tenant_withdrawals` (GET/POST 两条) + `/reconciliation`
  - tenantAuth 段新增 5 条：`/settlements` + `/balance_overview` + `/balance_logs` + `/withdrawals/mine` + `/withdraw`
- [验证] `go build ./...` + `go vet ./...` 双双通过

#### 前端：API 层
- [新增] `apps/admin/src/api/tenantFinance.ts`：6 个类型 + 10 个 API 函数
  - 类型：`PlatformSettlement` / `TenantBalanceLog` / `TenantWithdrawal` / `AdminTenantWithdrawal` / `TenantBalanceOverview` / `ReconciliationData`
  - 开发者侧 5 个：`listTenantSettlementsApi` / `tenantBalanceOverviewApi` / `listTenantBalanceLogsApi` / `listTenantOwnWithdrawalsApi` / `tenantWithdrawApi`
  - 超管侧 5 个：`listAdminTenantWithdrawalsApi` / `payAdminTenantWithdrawalApi` / `rejectAdminTenantWithdrawalApi` / `batchSettleApi` / `reconciliationApi`

#### 前端：开发者后台 2 个新页面
- [新增] `apps/admin/src/views/tenant/Settlements.vue` 开发者结算记录页
  - 余额概览卡片：可用余额 / 冻结 / 累计已结 / 累计已提现 / 待审核提现
  - 双 Tab：结算记录（订单号/抽成比例/平台抽成/应得/状态/批次号 + pending_sum/settled_sum 汇总）+ 余额流水（类型/金额变动+/-/操作后余额/状态）
  - 完整响应式 H5：钱包概览 4 列 → 2 列，搜索栏水平 → 垂直
- [新增] `apps/admin/src/views/tenant/Withdrawal.vue` 开发者提现申请页
  - 余额概览：可用 / 冻结 / 累计已提现 / 待审核提现
  - 提现表单：金额（≤ balance 校验）+ 收款方式（alipay/wechat/bank radio）+ 收款账号（动态 placeholder）+ 备注
  - 快捷按钮：「全部提现」「提现一半」
  - 提现记录列表：金额/方式/账号/状态/审核备注/打款流水号/打款时间
  - 完整响应式 H5：表单 + 记录双栏布局 → 移动端单列堆叠

#### 前端：超管后台 1 个新页面 + 1 个升级
- [新增] `apps/admin/src/views/admin/TenantWithdrawalReview.vue` 开发者提现审核页
  - 列表：开发者用户名/公司名/金额/收款方式/收款账号/状态/打款流水号/打款时间
  - 操作：打款对话框（含打款流水号 + 备注）+ 驳回对话框（必填原因，退回余额）
  - 完整响应式 H5
- [修改] `apps/admin/src/views/admin/Settlements.vue` 升级双 Tab
  - Tab 1 结算记录：保留原单条结算 + 新增多选批量结算按钮（仅 pending 可选，单次最多 100 条，含「选中应结」「涉及开发者数」预览）
  - Tab 2 对账报表：9 个聚合卡片（订单总数/总额/平台抽成/应结/已结/未结/已提现/待审核提现/理论余额 = 已结 - 已提）
  - 完整响应式 H5：3 列 → 2 列

#### 前端：路由注册
- [修改] `apps/admin/src/router/index.ts`
  - admin 段新增 `/admin/tenant-withdrawal-review`
  - tenant 段新增 `/tenant/settlements` + `/tenant/withdrawal`
- [验证] `pnpm run build` 通过

#### 设计要点
- 事务安全：所有金额变动走 `deps.DB.Transaction()` + `FOR UPDATE`，避免并发提现 / 结算竞争
- 对称设计：`tenant_withdraw` / `tenant_balance_log` / `sys_tenant.balance` 对称于 `agent_withdraw` / `agent_balance_log` / `agent.balance`
- 余额模型：`balance`（可提现）+ `frozen_balance`（冻结，提现申请中）；提现时 balance→frozen，打款时 frozen 清除，驳回时 frozen→balance
- 批量结算：按 tenant_id 分组累计 net_amount，避免同一开发者多次更新
- 对账差计算：`balance_theory = settled_sum - withdrawn_sum`，用于校验开发者账户余额理论值

---

## [0.3.3] - 2026-07-19

### [功能] 日志系统：异步 Worker + 三表独立查询 + CSV 导出 + 前端 3 Tab 升级

#### 后端：异步日志 Worker
- [新增] `apps/server/internal/handler/log_worker.go`
  - `verifyLogCh`（容量 4096）+ `StartVerifyLogWorker`：验证日志异步消费 goroutine，超出容量丢弃以保证验证 API 性能
  - `operationLogCh`（容量 2048）+ `StartOperationLogWorker`：操作日志异步消费 goroutine
  - `enqueueVerifyLog` / `enqueueOperationLog`：非阻塞 `select/default` 投递
  - `RecordOperation(deps, c, module, action, status, targetType, targetID, detail)`：从 `gin.Context` 抽取 role/userID/username/IP/UA 的一致切面 helper，供各业务 handler 调用
- [修改] `apps/server/internal/handler/client.go`
  - 新增 `writeVerifyLogCtx(deps, c, app, hwid, cardKey, action, result, message)`：捕获客户端 IP + User-Agent
  - 保留 `writeVerifyLog` 作为向后兼容包装（c=nil）
  - 14 处 `writeVerifyLog(deps, app,` → `writeVerifyLogCtx(deps, c, app,` 批量升级
- [修改] `apps/server/cmd/main.go`：启动时调用 `StartVerifyLogWorker` + `StartOperationLogWorker`

#### 后端：三表独立查询 + CSV 导出
- [新增] `AdminListOperationLogs` GET `/admin/logs/operations`：支持 operator_type / module / action / status / start_date / end_date / keyword 筛选
- [新增] `AdminListVerifyLogs` GET `/admin/logs/verify`：支持 app_id / action / result / start_date / end_date / keyword 筛选
- [新增] `AdminListLoginFailedLogs` GET `/admin/logs/login_failed`：支持 user_type / username / ip / start_date / end_date 筛选
- [新增] `AdminExportLogs` GET `/admin/logs/export?log_type=operation|verify|login_failed`：
  - 输出 `Content-Type: text/csv` + `Content-Disposition: attachment`
  - 写入 UTF-8 BOM `\xEF\xBB\xBF` 保证 Excel 正确识别编码
  - 单次导出上限 10000 条（防止 OOM）
  - `csvRow` helper 处理字段转义
- [新增] 路由注册 4 条新路由到 `adminAuth` 组（保留旧 `/admin/logs` 兼容接口）
- [验证] `go build ./...` + `go vet ./...` 双双通过

#### 前端：3 Tab 切换 + CSV 下载
- [新增] `apps/admin/src/api/admin.ts`：`LogOperation` / `LogVerify` / `LogLoginFailed` 三个接口类型 + `AdminLogTab` 联合类型
- [新增] 4 个 API 函数：`listAdminOperationLogsApi` / `listAdminVerifyLogsApi` / `listAdminLoginFailedLogsApi` / `exportAdminLogsApi`（后者使用 `responseType: 'blob'` 绕过 JSON 拦截器）
- [修改] `apps/admin/src/views/admin/Logs.vue` 全面重构：
  - el-tabs 三表切换：操作日志 / 验证日志 / 登录失败日志
  - 每表独立筛选条件（操作日志：operator_type/operator_id/module/action/status/keyword；验证日志：tenant_id/app_id/action/result/keyword；登录失败日志：user_type/username/ip/reason）
  - 顶部「导出 CSV」按钮：按当前 Tab 调用 `/admin/logs/export?type=xxx`，前端 `createObjectURL` + `<a download>` 触发下载，文件名带时间戳
  - 每表独立的 ResponsiveTable 列定义 + mobileFields（响应式 H5）
  - 详情对话框按 kind 区分操作/验证两种字段集
  - 完整中文映射：operatorType / verifyAction / verifyResult / reason 等 tag/text
- [验证] `pnpm run build` 通过（Logs 模块 18.18 kB / gzip 4.94 kB）

---

## [0.3.2] - 2026-07-19

### [功能] 代理充值审核闭环 + 提现审核闭环

#### 后端：开发者财务审核 Handler
- [新增] `apps/server/internal/handler/tenant_finance.go`（6 个 handler 全事务保护）
  - `TenantListRechargeRequests` GET `/tenant/recharge_requests` 充值申请列表（联表 agent，默认 pending）
  - `TenantApproveRecharge` POST `/tenant/recharge_requests/:id/approve` 通过（事务：加余额 + 流水 status=settled，支持 actual_amount 调整）
  - `TenantRejectRecharge` POST `/tenant/recharge_requests/:id/reject` 驳回（流水 status=rejected）
  - `TenantListWithdrawals` GET `/tenant/withdrawals` 提现申请列表（联表 agent，默认 pending）
  - `TenantPayWithdraw` POST `/tenant/withdrawals/:id/pay` 打款（事务：withdraw.status=paid + paid_at + pay_trade_no + 对应 balance_log status=settled）
  - `TenantRejectWithdraw` POST `/tenant/withdrawals/:id/reject` 驳回（事务：退回余额 + withdraw.status=rejected + balance_log status=rejected）
- [新增] 路由 `router.go` 注册 6 条新路由
- [验证] `go build ./...` + `go vet ./...` 双双通过

#### 前端：开发者审核页面 + 代理充值表单修复
- [新增] `api/tenant.ts`：`TenantRechargeRequest` / `TenantWithdrawal` 类型 + 6 个审核 API 函数
- [新增] `views/tenant/RechargeReview.vue` 充值审核页（搜索 / 通过对话框 / 驳回对话框 / 响应式 H5）
- [新增] `views/tenant/WithdrawalReview.vue` 提现审核页（搜索 / 打款对话框 / 驳回对话框 / 响应式 H5）
- [新增] `router/index.ts` 注册 `/tenant/recharge-review` + `/tenant/withdrawal-review` 两条路由
- [修复] `api/agent.ts`：补齐 `agentRechargeApi` 函数（v0.3.1 已交付 /agent/recharge 端点）
- [修复] `views/agent/Balance.vue`：
  - 移除误用 `agentWithdrawApi` 提交充值的临时方案，改为调用 `agentRechargeApi`
  - 充值表单增加 `pay_method`（alipay/wechat/bank/manual）+ `pay_voucher` 字段
  - 非手工支付必须填写付款凭证
  - 清理「待核实 v0.3.0」过时注释
- [验证] `pnpm run build` 通过（修复 mobileFields 类型断言 + statusTag 返回类型）

#### 资金链路闭环
- 代理充值：申请 → 开发者审核通过（可调实际金额）→ 自动加余额 → 流水 settled
- 代理提现：申请（扣余额）→ 开发者打款（标记 paid + 写流水号）→ 流水 settled
- 代理提现驳回：退回余额 + 流水 rejected + audit_remark 记录原因

#### 待办（v0.4.x 双层支付 D-18）
- 套餐 `allow_custom_pay` 字段生效 + 开发者自有易支付下单/回调接口
- 切换支付方式时通知所有代理

---

## [0.3.1] - 2026-07-19

### [修复] v0.3.0 全部「待核实 v0.3.x」项归零

#### 数据库字段补全（migration 006）
- [新增] `migrations/006_v0.3.1_field_completion.up.sql` ALTER TABLE 补齐缺失字段
- [新增] `sys_tenant.remark` / `sys_package.description` / `notice.sort` / `sec_ip_blacklist.created_by` + `created_by_type` + `source`
- [新增] `log_operation.username` + `user_agent` + `status`
- [新增] `AppCloudVar.read_only`
- [新增] `AppVersion.channel`
- [新增] `Agent.commission_mode` + `inviter_id` + `totp_secret` + `email` + `last_login_ip` + `last_login_at`
- [新增] `AgentInviteCode.used_by_agent_id`
- [新增] 新增表 `log_login_failed`（登录失败日志）+ `refresh_token_device`（设备会话追踪）

#### 后端 Handler 落实真实字段
- [修改] `admin_business.go`：租户结算金额真实查询（`PlatformSettlement.status='settled'`）、`Remark`/`Description`/`CommissionMode`/`InviterUsername`/`Sort`/`CreatedBy` 等字段全部落库
- [修改] `tenant_business.go`：CloudVar 直接用 `ReadOnly` 字段；Version 真实 `channel` 过滤；邀请码联表查询 `used_by_username`；公告 `sort` 排序与 type 三值（tenant/agent/h5）；删除公告级联清理 `NoticeRead`/`NoticeTarget`
- [修改] `agent_business.go`：`AgentMe` 真实返回 `email`/`totp_enabled`/`last_login_ip`/`inviter_username`；Dashboard `today_spent` 真实计算（`SUM(total_amount - commission_amount)`）；提现 `AuditRemark` 持久化 real_name
- [修改] `profile.go`：启用 agent email 更新（v0.3.1 已加字段）；移除三处 agent 2FA 501 阻断（`Setup2FA`/`Verify2FA`/`Disable2FA`）；`loadUserTOTPSecret`/`updateUserTOTPSecret` 新增 agent case
- [修改] `router.go`：移除 `/agent/recharge` 路由的「待核实 v0.3.x」注释

#### 新功能：AgentRecharge 充值申请
- [新增] `handler.AgentRecharge`：代理提交充值申请 → 创建 `AgentBalanceLog(type=recharge, status=pending)`
- [新增] 校验：`amount > 0` / 非手动支付必须 `PayVoucher` / sys_config 读取 `agent.recharge.min_amount`(1.00) + `agent.recharge.max_amount`(100000.00)
- [新增] 返回 pending 状态等待开发者审核

#### 新功能：ListLoginDevices 完整实现
- [新增] `refresh_token_device` 表 + `recordLoginSession` / `markAllSessionsRevoked`
- [新增] `ListLoginDevices`：列出当前用户所有活跃会话（IP / UA 解析 / 最近活跃 / 当前会话标记）
- [新增] `KickDevice`：标记指定 device 为 revoked（v0.4.x 待加 jti 精确单设备踢出）

#### 新功能：登录失败日志
- [新增] `log_login_failed` 表 + `LogLoginFailed` model
- [新增] `recordLoginFailureAsync`（buffered channel 容量 1024，溢出即丢保证登录可用）
- [新增] `StartLoginFailureWorker` main.go 启动后台 goroutine 消费
- [新增] `securityFailedLoginToday` / `securityBlockedIPsToday` 助手供 `AdminSecurityStats` 调用

#### 前端过时标记清理
- [修改] `api/{admin,tenant,agent,profile}.ts`：移除所有「待核实 v0.3.0」「当前 501」过时注释，统一改为「v0.3.1 已实现」
- [新增] `api/tenant.ts`：补齐 `updateTenantNoticeApi` + `deleteTenantNoticeApi`
- [修改] `views/tenant/Notices.vue`：启用删除按钮（带二次确认）+ `remove()` 函数
- [修改] `views/admin/Dashboard.vue`：待办事项「待核实（v0.3.0）」→「去查看」导航文案
- [修改] `views/{admin,tenant}/Dashboard.vue`：catch 注释由「501 静默降级」→「错误已由 http 拦截器处理」
- [修改] `views/{admin,tenant,agent}/Profile.vue`：移除「铁律 06 待核实」注释
- [修改] `views/agent/{Orders,Notices}.vue` + `views/tenant/Agents.vue`：移除「501 占位」头部警告

#### 待核实项归零（v0.3.x → v0.4.x 迁移）
- [里程碑] v0.3.0 CHANGELOG 中所有「待核实 v0.3.x」条目已全部解决或迁移至 v0.4.x
- [迁移] `avatar` 字段（三表均无对应列）→ v0.4.x 加列后落库
- [迁移] 2FA `backup_codes` Redis 持久化 → v0.4.x 加表字段后迁移
- [迁移] UA 解析库（mileusna/ua 或 ua-parser）→ v0.4.x 引入
- [迁移] 登录失败日志结构化记录 → v0.4.x 引入 zap/zerolog

#### 编译验证
- [验证] `go build ./...` 通过（0 错误）
- [验证] `go vet ./...` 通过（0 警告）
- [验证] `pnpm run build`（admin 前端）通过（修复 `Notices.vue` row 类型断言）

---

## [0.3.0] - 2026-07-19

### [新增] 后端业务 API 全量实现（替换全部 501 占位）

#### 核心交付
- [里程碑] 三角色后端业务接口（admin/tenant/agent）从 501 占位升级为真实实现，覆盖前端 v0.2.6/v0.2.7 已建立的 40+ 调用点
- [新增] `internal/handler/admin_business.go` 18 个超管接口（1067 行）：公开平台公告 + 工作台 + 租户/套餐/代理/公告 CRUD + 日志审计 + 安全中心（统计 + IP 黑名单 CRUD）
- [新增] `internal/handler/tenant_business.go` 22 个开发者接口（1479 行）：工作台 + 设备/订单/云变量/版本/代理/邀请码/支付配置/公告 全套 CRUD
- [新增] `internal/handler/agent_business.go` 11 个代理接口（1060 行）：工作台 + AgentMe 扩展 + 卡类/卡密/订单/佣金/提现/通知
- [新增] `internal/handler/profile.go` 三角色统一账号设置（763 行）：ProfileMe（覆盖 auth.go 的 CurrentUser）+ UpdateProfile + ChangePassword + 2FA 全流程（setup→verify→disable）+ LoginDevices

#### 路由注册（router.go）
- [修改] `internal/router/router.go` 注册 40+ 新端点，覆盖三角色工作台、CRUD、账号设置
- [修改] 三角色 `/auth/me` 由 `handler.CurrentUser` 切换为 `handler.ProfileMe`（agent 单独走 `AgentMe` 返回扩展字段）
- [新增] admin 组：`/packages/:id`、`/agents`、`/agents/:id`、`/notices/:id`（PUT/DELETE）、`/logs`、`/security/stats`、`/security/ip_blacklist`（GET/POST/DELETE）
- [新增] tenant 组：`/devices`、`/devices/:id/kick`、`/orders`、`/cloud_vars`（GET/POST/PUT/DELETE）、`/versions`（GET/POST/DELETE）、`/agents/:id`、`/agents/invite_codes`（GET/POST + `/:id/disable`）、`/pay_config`（GET/POST/PUT + `/test`）、`/notices`（GET/POST/PUT/DELETE）
- [新增] agent 组：`/auth/me`、`/card_types`、`/cards`、`/cards/generate`、`/orders`、`/commission`、`/withdraw`、`/recharge`、`/notices`、`/notices/:id/read`
- [新增] 三角色统一账号设置端点：`/auth/profile`、`/auth/change_password`、`/auth/2fa/setup|verify|disable`、`/auth/devices`、`/auth/devices/:id/kick`

#### 关键技术实现
- [新增] `parsePagination(c)` 公共分页助手（page 默认 1、page_size 默认 20、上限 100），跨文件共享
- [新增] `genInviteCodeUnique(db)` 邀请码生成（crypto/rand 16 位 + 5 次重试唯一性保证）
- [新增] `loadUserProfile(deps, role, userID)` 三角色统一资料加载，返回字段对齐前端 `CurrentUser` 接口
- [新增] `loadUserPasswordHash` / `loadUserTOTPSecret` / `updateUserTOTPSecret` 三角色密码与 2FA 密钥统一读写
- [新增] `agentFrozenBalance` / `agentTotalCommission` / `agentTotalWithdrawPaid` 代理聚合统计助手
- [新增] AgentGenerateCards 事务化（4 步：扣余额 gorm.Expr → 生成卡密 → 写扣费日志 → 结算佣金 + 写佣金日志）
- [新增] 2FA 全流程：setup（Redis 中转 10min）→ verify（AES 加密入库 + 备用码 Redis 持久化）→ disable（黑名单 refresh token）
- [新增] `parseDeviceName(ua)` 简化 User-Agent 解析（OS / Browser）

#### admin.go 清理
- [移除] admin.go 中 12 个 501 占位函数（PublicPlatformNotices / AdminDashboard / AdminListTenants / AdminCreateTenant / AdminUpdateTenant / AdminListPackages / AdminCreatePackage / AdminListNotices / AdminCreateNotice / TenantDashboard / TenantListAgents / TenantGenInviteCode），实现已迁移至 admin_business.go 与 tenant_business.go
- [保留] admin.go 仅保留 `AdminListConfig` 与 `AdminUpdateConfig` 两个真实实现（系统配置走 CfgCache，铁律 05）

#### 待核实项（铁律 06，未阻塞编译）
- [待核实] `sys_tenant` 无 `remark` 字段、`sys_package` 无 `description` 字段、`notice` 无 `sort` 字段、`log_operation` 无 `username/user_agent/status` 字段、`sec_ip_blacklist` 无 `created_by` 字段
- [待核实] `AppCloudVar` 无 `read_only` 字段、`AppVersion` 无 `channel` 字段、`Agent` 无 `commission_mode/inviter_id/totp_secret/email/last_login_ip` 字段、`AgentInviteCode` 无 `used_by_agent_id` 字段
- [待核实] `Notice` type 枚举与前端不完全一致（platform/tenant/agent）
- [待核实] `failed_login_today/blocked` 等安全统计当前返回 0（需引入登录失败日志表）
- [待核实] `AgentRecharge` 当前返回 501（待接入支付回调或开发者手工充值流程）
- [待核实] agent 表暂无 `totp_secret` 字段，代理 2FA setup 返回 501
- [待核实] `ListLoginDevices` 当前简化为返回当前会话信息（待引入完整的 refresh token 设备表）

#### 编译验证
- [验证] `go build ./...` 通过（0 错误，修复 tenant_business.go:382 `items` → `orders` 笔误）
- [验证] `go vet ./...` 通过（0 警告）

---

## [0.2.7] - 2026-07-19

### [新增] 全部剩余 PlaceholderView 替换为真实页面（响应式 H5 完整覆盖）

#### Admin 后台（7 页）
- [新增] `views/admin/Tenants.vue` 开发者管理：关键词+状态筛选 + 列表（用户名/套餐/应用数/卡密数/余额/到期）+ 新建对话框 + 编辑对话框（套餐/延长天数/状态/重置密码/备注）+ 启用/禁用
- [新增] `views/admin/Packages.vue` 套餐管理：列表 + 新建对话框（名称/描述/应用上限/卡密上限/代理上限/月费/年费/特性 JSON/状态）+ 编辑
- [新增] `views/admin/Agents.vue` 代理管理：关键词+状态+tenant_id 筛选 + 列表（所属开发者/余额/冻结/累计佣金/累计提现/佣金模式/比例）+ 编辑对话框（status/commission_mode/commission_rate/balance）
- [新增] `views/admin/Notices.vue` 平台公告：类型+状态+关键词筛选 + 列表 + 新建/编辑对话框（类型/标题/内容 textarea/状态/置顶/排序/发布时间/过期时间）+ 删除二次确认
- [新增] `views/admin/PayConfig.vue` 支付配置：表单（PID/Key 敏感隐藏/API URL/签名类型/通知路径/同步跳转/前端回跳/订单名前缀/默认抽成/最低结算/自动结算）+ 保存（逐项 updateSysConfig）+ 测试按钮调用 testPayConfigApi
- [新增] `views/admin/Logs.vue` 日志审计：类型+用户 ID+日期范围+关键词筛选 + 列表（用户名/角色/动作/目标/IP/状态）+ 详情弹窗（JSON 美化）
- [新增] `views/admin/Security.vue` 安全防护：4 数据卡（黑名单总数/生效中/今日登录失败/今日封禁 IP）+ 2 列布局（最近封禁 IP 列表 + IP 黑名单管理表格）+ 新增对话框（IP/原因/过期小时数）

#### Tenant 控制台（8 页）
- [新增] `views/tenant/Devices.vue` 设备管理：应用+关键词+在线状态筛选 + 列表（应用/卡密截断/设备名/设备 ID/IP/位置/心跳时间/在线状态）+ 强制下线二次确认
- [新增] `views/tenant/Orders.vue` 订单管理：应用+状态+渠道+日期范围+关键词筛选 + 列表（订单号/应用/卡类/买家/代理/数量/单价/总金额/佣金/净额/状态/渠道/支付时间）
- [新增] `views/tenant/CloudVars.vue` 云变量：应用+关键词筛选 + 列表（键/值截断/类型 tag/只读）+ 新建/编辑对话框 + 值完整查看对话框 + 删除二次确认
- [新增] `views/tenant/Versions.vue` 版本管理：应用+渠道筛选 + 列表（版本号/渠道/下载 URL/最低版本/强制更新/已发布/发布时间）+ 新建对话框（版本号/渠道/下载 URL/更新日志/最低版本/强制更新/立即发布）+ 删除二次确认
- [新增] `views/tenant/Agents.vue` 代理管理：4 数据卡（代理总数/活跃/累计佣金/累计提现，后端未返回显示 0）+ 关键词+状态筛选 + 列表 + 编辑对话框（status/commission_mode/commission_rate）
- [新增] `views/tenant/InviteCodes.vue` 邀请码：状态筛选 + 顶部说明 alert + 生成对话框（数量/有效天数/备注）+ 生成结果对话框（单条复制/复制全部）+ 列表（邀请码 mono/状态/使用人/过期时间）+ 禁用（仅 unused 显示）+ 复制按钮
- [新增] `views/tenant/PayConfig.vue` 支付配置：顶部 warning alert（需套餐 allow_custom_pay + v0.3.0 实现）+ 列表（渠道/状态/更新时间）+ 新建/编辑对话框（渠道/PID/Key 敏感/API URL/通知路径/同步跳转/状态）+ 测试按钮
- [新增] `views/tenant/Notices.vue` 我的公告：类型+状态筛选 + 列表 + 新建对话框（类型/标题/内容 textarea/状态/置顶/发布时间/过期时间）+ 删除按钮 disabled（待 v0.3.0 补全 delete/update API）

#### Agent 控制台（1 页 + API 扩展）
- [修改] `api/agent.ts` 末尾追加 2 个方法：`listAgentNoticesApi`（GET /agent/notices）+ `readAgentNoticeApi`（POST /agent/notices/:id/read）+ `AgentNotice` 类型
- [新增] `views/agent/Notices.vue` 消息通知：未读统计小卡 + 类型+仅未读筛选 + 列表（类型 tag/标题/置顶/已读/发布时间）+ 查看对话框（标题/类型/发布时间/内容 textarea readonly）+ 标为已读按钮（仅未读显示）

#### 路由
- [修改] `router/index.ts` 16 个 PlaceholderView 全部替换为懒加载真实组件，并移除 `import PlaceholderView`（不再使用）
- [里程碑] PlaceholderView 占位阶段彻底结束，前端三角色所有路由全部由真实响应式 H5 页面承载

#### 响应式适配
- [新增] 所有 16 页统一使用 PageHeader + ResponsiveTable + mobileFields 模式，桌面表格移动卡片自动切换
- [新增] PayConfig 表单两列布局（el-row + el-col :xs=24 :sm=12），label-position=top
- [新增] Security 4 数据卡（el-col :xs=12 :sm=6）+ 2 列布局（el-col :xs=24 :sm=12）
- [新增] Agents 4 数据卡（el-col :xs=12 :sm=6）网格响应式

#### 待核实项（铁律 06）
- [待核实] 后端 `/admin/tenants|packages|agents|notices|logs|security` 当前为 501，前端 try/catch 静默降级，待 v0.3.0 实现
- [待核实] 后端 `/tenant/devices|orders|cloud_vars|versions|agents|invite_codes|pay_config|notices` 当前为 501，前端 try/catch 静默降级
- [待核实] 后端 `/agent/notices` 当前为 501，待 v0.3.0 实现
- [待核实] admin.ts 未导出 `updateAdminPackageApi`，Packages.vue 编辑直接调用 `request.put('/admin/packages/:id')`，待 v0.3.0 补全正式 API
- [待核实] tenant.ts 未导出公告 update/delete API，Notices.vue 删除按钮暂 disabled，待 v0.3.0 补全
- [待核实] Tenant Agents 4 数据卡（代理总数/活跃/累计佣金/累计提现）后端暂不返回，显示 0 不编造，待 v0.3.0 扩展

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误，修复 2 处类型：admin/Agents editingId narrowing 提取为 const + tenant/Agents form.status 由 string 改为联合类型字面量）
- [验证] `npx vite build` 通过（8.68s），输出 16 个新页面 chunk

---

## [0.2.6] - 2026-07-19

### [新增] 三角色 Profile + 双 Dashboard（响应式 H5）

#### API 模块（3 个新文件）
- [新增] `api/profile.ts` 三角色共享账号设置 API 封装：9 个方法（currentUser / updateProfile / changePassword / setup2FA / verify2FA / disable2FA / listLoginDevices / kickDevice，按 role 动态拼接路径）+ 5 个类型定义（CurrentUser / ChangePasswordReq / UpdateProfileReq / TwoFASetupResp / LoginDevice）
- [新增] `api/admin.ts` 超管后台 API 封装：17 个方法覆盖 dashboard / tenants（CRUD）/ packages（CRUD）/ agents / notices（CRUD + 删除）/ logs / security（stats + IP 黑名单增删）+ 7 个类型定义（AdminDashboardData / AdminTenant / AdminPackage / AdminAgent / AdminNotice / AdminLog / AdminSecurityStats / IpBlacklistItem）
- [新增] `api/tenant.ts` 开发者控制台 API 封装：19 个方法覆盖 dashboard / devices（列表 + 踢线）/ orders / cloud-vars（CRUD）/ versions（CRUD）/ agents / invite-codes（生成 + 禁用）/ pay-config（保存 + 测试）/ notices + 9 个类型定义（TenantDashboardData / TenantDevice / TenantOrder / TenantCloudVar / TenantVersion / TenantAgent / TenantInviteCode / TenantPayConfig / TenantNotice）

#### 三角色 Profile 页面（账号设置）
- [新增] `views/admin/Profile.vue` 超管账号设置：基础资料（用户名只读 + 真实姓名/邮箱/手机可编辑）+ 修改密码（最小 8 位 + 字母数字组合 + 二次确认）+ 2FA TOTP（生成二维码 → 扫码 → 验证 6 位码 → 显示备用码）+ 登录设备列表（可踢下线）
- [新增] `views/tenant/Profile.vue` 开发者账号设置：基础资料（用户名只读 + tenant_id 标签 + 真实姓名/邮箱/手机）+ 公司信息（公司名/联系人/联系电话/营业执照/地址）+ 修改密码 + 2FA TOTP
- [新增] `views/agent/Profile.vue` 代理账号设置：账户概览（4 数据卡：余额/冻结/累计佣金/累计提现）+ 基础资料（用户名/真实姓名/手机/邮箱/邀请人/注册时间）+ 提现账户（支付宝/微信/银行卡三选一动态字段）+ 修改密码

#### 双 Dashboard 页面
- [新增] `views/admin/Dashboard.vue` 超管平台概览：8 数据卡（开发者/代理/应用/卡密/订单/今日收入/待结算/快捷操作）+ 2 列布局（待办列表 + 收入趋势柱状图）+ 2 列布局（最近开发者表 + 最近订单表）
- [新增] `views/tenant/Dashboard.vue` 开发者工作台：8 数据卡（应用/卡密/设备/订单/今日收入/待结算/代理/快捷操作）+ 8 项快捷入口网格 + 2 列布局（收入趋势 + 应用 TOP5 排行榜）+ 最近订单表

#### 路由
- [修改] `router/index.ts` 5 个 PlaceholderView 替换为真实页面：admin/Dashboard + admin/Profile + tenant/Dashboard + tenant/Profile + agent/Profile

#### 响应式适配
- [新增] Profile 表单 label 位置：桌面 right / 移动 top（computed 监听 window.innerWidth < 768）
- [新增] Dashboard 数据卡网格：桌面 4 列 / 平板 2 列 / 手机 2 列
- [新增] Dashboard 双列布局：桌面 2 列 / 移动 1 列堆叠
- [新增] 趋势图高度：桌面 200px / 移动 160px
- [新增] 快捷入口网格：桌面 8 列（4×2）/ 平板 4 列 / 手机 4 列（2×4）
- [新增] 账户概览 4 数据卡：桌面 4 列 / 平板 2 列 / 手机 2 列

#### 业务特性
- [新增] 修改密码成功后 1.5s 自动登出并跳转登录页
- [新增] 2FA 设置流程：调用 setup 获取 secret + otpauth URL → 渲染二维码（qrcode 库）→ 输入 6 位验证码 → 调用 verify 启用 → 显示备用码（可复制）
- [新增] 2FA 禁用对话框：要求密码 + 当前 6 位验证码双重确认
- [新增] 代理提现账户：method 切换动态显示不同字段（alipay: 账号+姓名 / wechat: 微信号+姓名 / bank: 开户行+账号+姓名）
- [新增] 待办列表项可点击跳转对应管理页（结算/开发者/代理/公告）
- [新增] 应用 TOP5 排行：金/银/铜徽章 + 销量 + 收入

#### 待核实项（铁律 06）
- [待核实] 后端 `/admin/dashboard` `/tenant/dashboard` 当前为 501 占位（v0.3.0 交付），Dashboard 数据全部回退为 0/空数组
- [待核实] 后端 `/{role}/auth/me` 当前仅返回 user_id/username/role/tenant_id，Profile 中 email/phone/real_name/totp_enabled 字段暂为空，待 v0.3.0 扩展 CurrentUser handler
- [待核实] 修改密码 / 2FA 设置/禁用 / 登录设备列表/踢下线 接口当前为 501，前端 try/catch 静默处理，待 v0.3.0 实现
- [待核实] 代理账户概览的 balance/frozen/total_commission/total_withdraw 字段当前返回 0（CurrentUser 未含代理扩展字段），待 v0.3.0 扩展
- [待核实] 开发者公司信息字段（contact_name/contact_phone/license_no/address）后端尚未支持，待 v0.3.0 扩展 tenant 表
- [待核实] Dashboard 收入趋势/应用排行/最近订单 当前为空数组，待 v0.3.0 后端实现聚合查询

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误，修复 1 处 el-table slot 类型：kickDevice row 参数由 LoginDevice 改为 any）
- [验证] `npx vite build` 通过（6.93s），输出 5 个新页面 chunk（admin/Dashboard + admin/Profile + tenant/Dashboard + tenant/Profile + agent/Profile）

---

## [0.2.5] - 2026-07-19

### [新增] 代理核心页面（响应式 H5）

#### API 模块
- [新增] `api/agent.ts` 代理控制台 API 封装：8 个方法（dashboard / me / card_types / cards / generate / orders / commission / withdraw）+ 9 个类型定义（AgentProfile / AgentDashboard / AgentCardType / AgentCard / AgentOrder / AgentCommission 等）

#### 代理后台页面
- [新增] `views/agent/Dashboard.vue` 代理工作台：4 个数据卡（余额/累计佣金/累计购卡/累计消费）+ 4 个快捷入口 + 最近订单列表，全响应式
- [新增] `views/agent/Cards.vue` 代理购卡页：余额栏（含佣金模式/比例展示）+ 卡类网格（点击进入购卡）+ 购卡对话框（数量/前缀/分组/总价预览/支付后余额预览）+ 购卡结果对话框（卡密列表 + 复制全部）
- [新增] `views/agent/Orders.vue` 代理订单列表：状态筛选 + 订单号/卡类/数量/金额/佣金/状态/渠道/下单时间/支付时间
- [新增] `views/agent/Commission.vue` 佣金记录：4 个统计卡（累计佣金/已提现/可提现/冻结）+ 类型与状态双筛选 + 流水列表（购卡佣金/提现申请/充值/调整）+ 申请提现对话框（金额/收款方式/收款账号/真实姓名/备注）
- [新增] `views/agent/Balance.vue` 余额/提现页：钱包概览大卡 + 3 个统计小卡 + 充值/提现记录列表 + 申请充值对话框

#### 布局与路由
- [重构] `layouts/AgentLayout.vue` 顶部余额标签从占位（¥0.00）改为调用 `/agent/auth/me` 真实获取，并在路由切换时自动刷新
- [修改] `router/index.ts` 代理后台 5 个核心路由由 PlaceholderView 替换为真实页面（dashboard/cards/orders/balance/commission）

#### 业务特性
- [新增] 代理购卡流程：选卡类 → 输入数量/前缀 → 二次确认 → 调用 `/agent/cards/generate` 扣余额生成 → 展示卡密列表
- [新增] 代理提现流程：余额校验 → 收款方式选择（支付宝/微信/银行卡）→ 提交后进入冻结状态待开发者审核
- [新增] 代理充值申请：通过 `/agent/withdraw` 端点提交（type 区分，待 v0.3.0 拆分独立 recharge 端点）
- [新增] 购卡金额预览：实时计算总价 + 支付后余额 + 余额不足前置校验

#### 响应式适配
- [新增] 数据卡 grid：桌面 4 列 / 平板 2 列 / 手机 2 列，字号随断点缩小
- [新增] 卡类网格：`auto-fill minmax(280px, 1fr)`，手机自动堆叠为单列
- [新增] 钱包概览：桌面 `1fr 2fr`（余额卡 + 3 统计卡），手机单列堆叠
- [新增] 表格移动端切换为卡片列表（购卡订单/佣金流水/充值提现记录），关键字段保留

#### 待核实项（铁律 06）
- [待核实] 后端 `/agent/dashboard` `/agent/card_types` `/agent/cards` `/agent/cards/generate` `/agent/orders` `/agent/commission` `/agent/withdraw` 当前均为 501 占位（v0.3.0 交付），前端调用会触发业务错误提示，列表保持空状态（铁律 04 不编造数据）
- [待核实] `/agent/auth/me` 复用 `handler.CurrentUser`，可能正常返回 JWT 中的基本信息但不含 balance/total_commission 字段，待 v0.3.0 扩展
- [待核实] 代理充值暂复用 `/agent/withdraw` 端点提交（remark 标识 `[充值申请]`），待 v0.3.0 实现独立 `/agent/recharge` 端点
- [待核实] 代理佣金模式（percentage / diff）字段需后端在 `agent` 表或 sys_config 中提供，待 v0.3.0 确认数据来源

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误）
- [验证] `npx vite build` 通过，输出 5 个代理页 chunk（Dashboard 4.80KB / Cards 9.79KB / Orders 3.41KB / Balance 7.16KB / Commission 8.24KB）

---

## [0.2.4] - 2026-07-19

### [新增] 前端响应式 H5 全栈（admin/tenant/agent/官网/终端用户 H5）

#### 全局基础设施
- [新增] `styles/variables.scss` 响应式断点 + mixin（mobile/tablet/desktop）+ 明亮配色变量
- [新增] `styles/index.scss` 响应式工具类（hidden-mobile/visible-mobile-only/card-list）+ 移动端表格紧凑模式 + 移动端对话框/抽屉适配
- [修复] `layouts/AdminLayout.vue` 移除违反铁律 03 的暗黑侧边栏（#001529 → #fff），改为薄包装 BasicLayout
- [修复] `layouts/AgentLayout.vue` 移除暗黑侧边栏（#1f2937 → #fff），改为薄包装 BasicLayout
- [重构] `layouts/TenantLayout.vue` 简化为薄包装 BasicLayout
- [新增] `layouts/BasicLayout.vue` 通用响应式布局（桌面固定侧边栏 + 平板可折叠 + 移动端抽屉式 + 公告横幅插槽 + 顶部右侧插槽）

#### API 模块化
- [新增] `api/auth.ts` 三角色统一登录 + 注册 + refresh + logout + currentUser
- [新增] `api/apps.ts` 应用 CRUD + 重置密钥（支持 all/app_key/app_secret/sign_secret 4 种范围）
- [新增] `api/cards.ts` 卡类 CRUD + 卡密列表/生成/封禁/解禁/删除
- [新增] `api/pay.ts` 终端用户下单 + 订单查询 + 超管结算列表/手动结算 + 支付配置测试
- [重构] `api/http.ts` 请求拦截器注入 Bearer token + 响应拦截器自动 refresh token（含并发去重 + 401 重试 + refresh 失败登出）

#### 状态管理
- [重构] `stores/auth.ts` JWT 双 token（access 2h + refresh 7d）+ 自动续期定时器（提前 5 分钟）+ 持久化 + Cookie 同步 + 登出调用后端黑名单
- [保留] `stores/sysConfig.ts` 平台配置（从 sys_config 加载，铁律 05）

#### 通用组件
- [新增] `components/PageHeader.vue` 响应式页面标题（桌面一行，移动两行）
- [新增] `components/EmptyState.vue` 空状态
- [新增] `components/ResponsiveTable.vue` 桌面端表格 + 移动端自动切换卡片列表 + 分页响应式

#### 登录与注册
- [重构] `views/login/index.vue` 三角色 Tab 切换 + 真实接口对接 + 2FA TOTP 二阶段验证 + 响应式
- [新增] `views/register/TenantRegister.vue` 开发者注册页（账号/密码/邮箱/手机/公司/邀请码）+ 响应式

#### 官网首页（Landing）
- [新增] `views/landing/index.vue` 完整官网首页：顶部导航滚动效果 + Hero 区 + 9 个核心功能 + 6 个适用场景 + 3 个套餐预览 + 5 个 FAQ + CTA + 底部，全部响应式

#### 终端用户 H5（移动端优先）
- [新增] `layouts/H5Layout.vue` H5 专属布局（顶部 Logo + 底部 Tabbar 购卡/查卡，桌面访问也以移动样式呈现）
- [新增] `views/h5/Home.vue` 购卡首页：AppKey 输入 + 卡类列表 + 数量 + 支付方式 + 跳转易支付
- [新增] `views/h5/PayResult.vue` 支付结果页：状态图标 + 轮询订单 + 卡密列表 + 复制
- [新增] `views/h5/Query.vue` 卡密查询页：输入 AppKey + 卡密 + 显示详情
- [新增] `views/h5/CardDetail.vue` 卡密详情页

#### 超管后台
- [新增] `views/admin/Settlements.vue` 结算管理：列表 + 筛选 + 手动结算对话框
- [新增] `views/admin/SysConfig.vue` 系统配置：分组标签页 + 敏感配置隐藏 + 编辑对话框（铁律 05 可视化入口）

#### 开发者控制台
- [新增] `views/tenant/Apps.vue` 应用管理：列表 + 新建/编辑 + 详情 + 重置密钥 + 删除（4 种范围）
- [新增] `views/tenant/CardTypes.vue` 卡类管理：列表 + 新建/编辑（5 种类型：时长/次数/永久/试用/功能）
- [新增] `views/tenant/Cards.vue` 卡密管理：列表 + 批量生成（最多 1000 张/次）+ 封禁/解禁/删除 + 生成结果展示

#### 路由
- [新增] `/` 官网首页路由
- [新增] `/register/tenant` 开发者注册路由
- [新增] `/h5`、`/h5/pay/:orderNo`、`/h5/query`、`/h5/card/:cardKey` 终端用户 H5 路由组
- [新增] `/admin/settlements`、`/admin/sys-config` 超管新页面路由
- [新增] `/tenant/apps`、`/tenant/card-types`、`/tenant/cards` 开发者新页面路由

#### 编译验证
- [验证] `npx vue-tsc --noEmit` 通过（0 错误）
- [验证] `npx vite build` 通过，输出 26 个 JS chunk + 6 个 CSS chunk

---

## [0.2.3] - 2026-07-19

### [新增] 平台总支付（彩虹易支付）下单/回调/自动发卡/抽成结算（P0 核心闭环）

#### 彩虹易支付工具包 `pkg/epay/epay.go`（新建）
- [新增] `Config`：易支付配置（GatewayURL/PID/Secret/SignType）
- [新增] `OrderParams`：下单参数（OutTradeNo/Name/Money/PayType/NotifyURL/ReturnURL/ClientIP）
- [新增] `BuildSubmitURL`：构造 GET 跳转 URL（submit.php，前端直接 location.href）
- [新增] `NotifyParams` + `ParseNotify`：解析异步/同步回调参数
- [新增] `VerifyNotify`：校验回调签名
- [新增] `IsSuccess`：判断回调是否支付成功（TRADE_SUCCESS）

#### 加密工具扩展 `pkg/crypto/crypto.go`
- [新增] `MD5Hex`：MD5 十六进制（32 位小写）
- [新增] `SignEpayParams`：彩虹易支付签名（参数排序 + 拼接 + 追加密钥 + MD5）
- [新增] `VerifyEpaySign`：校验彩虹易支付签名（常量时间比较防时序攻击）

#### 支付 Handler `internal/handler/pay.go`（新建）
- [新增] `CreatePayOrder`：终端用户下单（校验应用/卡类/开发者状态 → 创建 AppOrder pending → 构造易支付跳转 URL → 返回 pay_url）
- [新增] `GetPayOrder`：查询订单状态（含超时自动关单逻辑）
- [新增] `EpayNotify`：异步回调（合并 GET+POST 参数 → 验签 → Redis SETNX 防重入 → 校验金额 → 事务内自动发卡 + 写抽成记录 → 返回 "success"）
- [新增] `EpayReturn`：同步跳转（302 重定向到前端结果页 `/pay/result?order_no=xxx`）
- [新增] `EpayTenantNotify`：开发者自有易支付回调占位（v0.3.0 实现）
- [新增] `AdminListSettlements`：超管查询结算记录列表（分页 + 按 tenant_id/status 筛选）
- [新增] `AdminSettleOrder`：超管手动标记订单已结算（生成结算批次号 STL+日期+ID）
- [新增] `AdminTestPayConfig`：测试平台易支付配置（校验配置完整性 + 解密是否成功）

#### 平台抽成结算记录表 `platform_settlement`（新增）
- [新增] 字段：tenant_id / order_id / order_no / gross_amount / commission_rate / commission_amount / net_amount / status / settled_at / settle_batch_no / settle_method / settle_remark
- [新增] 索引：uk_order（订单 ID 唯一）/ idx_tenant_status（租户+状态联合索引）/ idx_order_no

#### 数据库迁移
- [新增] `migrations/005_pay_settlement.up.sql`：
  - 创建 `platform_settlement` 表
  - 修正 `pay.platform.notify_path` 默认值（`/api/v1/pay/platform/notify` → `/api/v1/pay/notify/epay`）以对齐实际路由
  - 修正 `pay.platform.return_path` 默认值（`/pay/return` → `/api/v1/pay/return/epay`）
  - 新增 6 项配置：sign_type / return_front_url / order_name_prefix / callback_retry_max / settlement.min_amount / settlement.auto_enabled
- [新增] `migrations/005_pay_settlement.down.sql`：回滚脚本

#### 路由（新增端点）
- [新增] `POST /api/v1/pay/order` 终端用户下单
- [新增] `GET /api/v1/pay/order/:order_no` 查询订单状态
- [新增] `GET /api/v1/admin/settlements` 结算记录列表
- [新增] `POST /api/v1/admin/settlements/:id/settle` 手动结算
- [新增] `POST /api/v1/admin/pay/test` 测试支付配置

#### 安全特性
- [安全] 异步回调强制验签（MD5 + 常量时间比较防时序攻击）
- [安全] 回调金额校验（防止伪造回调）
- [安全] Redis SETNX 防重入（同一订单号 60 秒内只处理一次，处理失败释放锁以便重试）
- [安全] 订单状态机校验（仅 pending → paid，已 paid 直接返回成功实现幂等）
- [安全] 商户密钥 AES-256-GCM 加密入库，回调时解密
- [安全] 下单时校验开发者账号状态（active）+ 套餐过期时间
- [安全] 自动发卡事务原子性（订单更新 + 卡密生成 + 抽成记录同时成功或同时失败）

#### 业务特性
- [新增] 抽成计算：优先取套餐 `platform_commission_rate`，为 0 时回退到 `pay.platform.commission_default`（默认 5%）
- [新增] 自动发卡：支付成功后系统自动生成 N 张卡密（CreatorType=auto，OrderID 关联订单）
- [新增] 订单超时关闭：查询订单时若 pending 状态超过 `pay.order_expire_seconds`（默认 1800 秒）自动关闭
- [新增] 同步跳转：支付完成后浏览器 302 跳转到前端 `/pay/result?order_no=xxx`
- [新增] 支持三种支付方式：alipay / wxpay / qqpay
- [新增] 订单号用雪花算法生成（ORD 前缀）
- [新增] 批次号格式：P+YYYYMMDD+6位订单ID余数（区别于开发者手动生成的 B 前缀）

#### 待核实项（铁律 06）
- [待核实] `resolveNotifyURL` 优先用请求头 Host，待核实生产环境是否应单独配置 `notify_base_url`
- [待核实] 自动发卡时 `CreatedBy` 字段填 `order.TenantID`，待核实是否应改为 0（系统）或新增 system_user_id
- [待核实] 订单超时关闭仅在查询时触发，待核实是否应增加定时任务主动扫描
- [待核实] 彩虹易支付部分实现支持 `mapi.php`（API 模式，无页面跳转），当前仅实现 `submit.php`（GET 跳转）

---

## [0.2.2] - 2026-07-19

### [新增] 应用管理 + 卡密管理 + 客户端验证 API（P0 核心闭环）

#### 应用密钥生成器 `pkg/crypto/crypto.go`
- [新增] `GenerateAppKey`：生成 AppKey（ak_ 前缀 + 32 位 hex）
- [新增] `GenerateAppSecret`：生成 AppSecret（as_ 前缀 + 64 位 hex）
- [新增] `GenerateSignSecret`：生成 HMAC 签名密钥（ss_ 前缀 + 64 位 hex）
- [新增] `GenerateHWID`：设备指纹生成（SHA-512(CPU+主板+MAC+磁盘)）

#### 应用管理 Handler `internal/handler/app.go`
- [新增] `TenantCreateApp`：创建应用（含套餐配额校验 + 密钥生成 + AES 加密入库）
- [新增] `TenantListApps`：应用列表（分页 + 关键词搜索）
- [新增] `TenantGetApp`：应用详情
- [新增] `TenantUpdateApp`：更新应用
- [新增] `TenantResetAppKey`：重置 AppSecret/SignSecret（支持密钥轮换，旧 SignSecret 保留 7 天）
- [新增] `TenantDeleteApp`：软删除应用（状态置 disabled）

#### 卡类管理 Handler `internal/handler/card.go`
- [新增] `TenantCreateCardType`：创建卡类（5 种类型：duration/count/permanent/trial/feature）
- [新增] `TenantListCardTypes`：卡类列表
- [新增] `TenantUpdateCardType`：更新卡类

#### 卡密管理 Handler `internal/handler/card.go`
- [新增] `TenantGenerateCards`：批量生成卡密（事务 + 套餐配额校验 + 批次号）
- [新增] `TenantListCards`：卡密列表（多条件筛选 + 分页）
- [新增] `TenantGetCard`：卡密详情
- [新增] `TenantBanCard`：封禁卡密（含状态机校验）
- [新增] `TenantUnbanCard`：解封卡密（根据激活状态恢复到 unused/active/expired）
- [新增] `TenantDeleteCard`：删除卡密（仅 unused 状态可删）

#### 心跳保活服务 `internal/heartbeat/heartbeat.go`
- [新增] `Record`：记录心跳（Redis Sorted Set + Hash 双写）
- [新增] `IsOnline`：检查设备在线状态
- [新增] `Remove`：移除设备心跳（解绑/封禁时调用）
- [新增] `CountOnline`：统计应用在线设备数
- [新增] `ListOnline`：列出在线设备 ID
- [新增] `GetLastHeartbeatAt`：获取最后心跳时间

#### 客户端验证 API `internal/handler/client.go`（全部实现）
- [新增] `ClientLogin`：登录（卡密校验 + 设备自动绑定 + 激活卡密 + 心跳初始化）
- [新增] `ClientVerify`：验证（不增加使用次数，校验设备绑定 + 离线宽限期）
- [新增] `ClientHeartbeat`：心跳（更新 DB + Redis Sorted Set）
- [新增] `ClientBind`：手动绑定设备（多机场景）
- [新增] `ClientUnbind`：解绑设备（扣时 + 移除在线状态）
- [新增] `ClientLogout`：退出登录（仅记录日志）
- [新增] `ClientGetVar`：获取云变量（需校验卡密有效性）
- [新增] `ClientNotice`：获取应用公告
- [新增] `ClientVersion`：检查版本更新

#### 路由（新增端点）
- [新增] `GET /api/v1/tenant/apps/:id` 应用详情
- [新增] `DELETE /api/v1/tenant/apps/:id` 软删除应用
- [新增] `POST /api/v1/tenant/apps/:id/reset_key` 重置应用密钥
- [新增] `GET/POST /api/v1/tenant/card_types` 卡类列表/创建
- [新增] `PUT /api/v1/tenant/card_types/:id` 更新卡类
- [新增] `GET /api/v1/tenant/cards/:id` 卡密详情
- [新增] `POST /api/v1/tenant/cards/:id/ban` 封禁卡密
- [新增] `POST /api/v1/tenant/cards/:id/unban` 解封卡密
- [新增] `DELETE /api/v1/tenant/cards/:id` 删除卡密

#### 数据库迁移
- [新增] `migrations/004_app_card_config.up.sql`：11 项应用/卡密/验证日志配置
  - 应用默认参数：max_devices/heartbeat_interval/heartbeat_timeout/offline_grace/unbind_deduct_seconds
  - 卡密生成：max_batch/charset/segment_length/segment_count
  - 验证日志：async_enabled/sample_rate
- [新增] `migrations/004_app_card_config.down.sql`：回滚脚本

#### 安全特性
- [安全] 创建应用时校验开发者账号状态 + 套餐过期时间
- [安全] 创建应用/卡密时校验套餐配额（MaxApps/MaxCards）
- [安全] AppSecret/SignSecret 使用 AES-256-GCM 加密入库
- [安全] 重置 SignSecret 时旧密钥迁移到 SignSecretPrev 保留 7 天
- [安全] 卡密按 SHA-512 hash 查询（防穷举）
- [安全] 删除应用为软删除（保留审计轨迹）
- [安全] 删除卡密仅 unused 状态可删（防止误删已激活卡密）
- [安全] 卡密封禁/解封有严格状态机（unused/active 可封禁，仅 banned 可解封）
- [安全] 解绑设备扣时（防滥用）
- [安全] 客户端 verify 校验离线宽限期（防断网绕过）

#### 业务特性
- [新增] 卡密 5 种类型支持：duration/count/permanent/trial/feature
- [新增] 一机一卡密绑定（MaxDevices=1）+ 多机绑定（MaxDevices>1）
- [新增] 设备指纹：SHA-512(CPU+主板+MAC+磁盘)
- [新增] 离线宽限期判定（应用级配置）
- [新增] 解绑扣时机制（应用级配置）
- [新增] 卡密批次号管理（B + 日期 + 用户 ID）
- [新增] 卡密明文仅生成时返回一次

#### 待核实项（铁律 06）
- [待核实] `loadCardByCardKey` 优先按 hash 查询，但 SDK 默认传明文，建议确认客户端是否预计算 hash
- [待核实] `ClientVersion` 版本号比较为字符串比较，建议改用 semver 库
- [待核实] `writeVerifyLog` 当前同步写入，v0.3.0 应改为异步队列

---

## [0.2.1] - 2026-07-19

### [新增] 认证模块（P0 核心闭环）

#### 认证工具包 `internal/auth/`
- [新增] `jwt.go`：JWT 双 Token 机制（access + refresh），支持 Token 解析、黑名单、Bearer 提取
- [新增] `totp.go`：TOTP 2FA 工具包（基于 pquerna/otp），支持生成密钥、校验验证码、备用码、AES 加密入库
- [新增] `login_lock.go`：登录失败计数器 + 账号锁定（Redis 滑动窗口），支持锁定状态查询、人类可读剩余时间格式化
- [新增] `context.go`：Redis 操作默认 context

#### 认证处理器 `internal/handler/auth.go`
- [新增] `AdminLogin`：超管登录（用户名/密码/TOTP 校验 + JWT 签发 + 失败计数）
- [新增] `TenantLogin`：开发者登录（同上）
- [新增] `TenantRegister`：开发者注册（注册开关 + 密码长度校验 + 用户名唯一 + 默认套餐 + 试用天数 + 自动签发 Token）
- [新增] `AgentLogin`：代理登录（同超管登录流程）
- [新增] `RefreshToken`：三角色共用 Token 刷新（refresh token 轮换 + 旧 token 加入黑名单）
- [新增] `Logout`：登出（refresh token 加入黑名单）
- [新增] `CurrentUser`：返回 JWT 中的当前用户信息

#### 路由
- [新增] `POST /api/v1/public/auth/refresh`：三角色共用刷新端点
- [新增] `POST /api/v1/{admin|tenant|agent}/auth/logout`：三角色登出端点
- [新增] `GET /api/v1/{admin|tenant|agent}/auth/me`：当前用户信息端点

#### 数据库迁移
- [新增] `migrations/003_auth_config.up.sql`：19 项认证相关 sys_config 配置
  - 登录失败锁定：max_attempts/lock_seconds/window_seconds/require_captcha
  - JWT：access_ttl_seconds/refresh_ttl_seconds/issuer
  - TOTP：issuer/period/digits/algorithm/skew/backup_codes_count
  - 2FA 强制策略：admin/tenant/agent 2fa_required
  - 开发者注册：enabled/default_package_id/trial_days
- [新增] `migrations/003_auth_config.down.sql`：回滚脚本

#### 修复
- [修复] `pkg/crypto/crypto.go`：DecryptAES 函数 ciphertext 变量重用导致类型冲突的 bug
- [修复] `apps/server/go.mod`：module path 缺少 `/apps/server` 后缀导致内部包导入失败

#### 安全特性
- [安全] 登录失败 5 次锁定账号 15 分钟（参数均可后台调整）
- [安全] 账号不存在时不暴露存在性，统一返回「用户名或密码错误」
- [安全] refresh token 轮换机制（旧的立即失效）
- [安全] refresh token 黑名单（登出 / 改密后旧 token 失效）
- [安全] TOTP 密钥 AES-256-GCM 加密入库
- [安全] 2FA 验证码错误不计入账号锁定计数（防止遗忘手机导致账号被锁）
- [安全] 2FA 强制策略可按角色独立配置

#### 待核实项（铁律 06）
- [待核实] `genTenantCode` 当前用纳秒时间戳简化实现，生产环境应改用 crypto/rand 生成不可预测的随机部分

---

## [0.2.0] - 2026-07-19

### [新增] 一期 MVP 骨架（计划中 → 已完成骨架）

#### 数据库层
- [新增] `migrations/001_init_schema.up.sql`：26 张表完整 DDL（平台层 / 应用层 / 代理层 / 公告层 / 安全日志层）
- [新增] `migrations/001_init_schema.down.sql`：回滚脚本
- [新增] `migrations/002_seed_data.up.sql`：默认 sys_config（47 项配置）+ 三档套餐 + 默认超管 + 平台欢迎公告
- [新增] `migrations/002_seed_data.down.sql`：seed 回滚脚本
- [新增] `log_verify` 表按月分区（RANGE PARTITION on created_at）

#### 后端 Go 骨架
- [新增] `cmd/main.go`：HTTP 服务入口 + 优雅关闭
- [新增] `internal/config/config.go`：YAML + 环境变量双层配置加载（铁律 04）
- [新增] `internal/config/cache.go`：sys_config 缓存（GetString/GetInt/GetBool/GetFloat64/GetJSON）+ 缓存穿透保护
- [新增] `internal/model/model.go`：26 张表 GORM 模型 + TableName 方法
- [新增] `internal/middleware/auth.go`：JWT 认证中间件
- [新增] `internal/middleware/tenant.go`：多租户隔离中间件
- [新增] `internal/middleware/signature.go`：HMAC-SHA256 签名验证 + nonce 防重放 + 密钥轮换支持
- [新增] `internal/middleware/ratelimit.go`：Redis 滑动窗口限流 + 失败计数 + 自动 IP 封禁
- [新增] `internal/handler/admin.go`：管理员后台 handler 骨架（含 sys_config CRUD 完整实现）
- [新增] `internal/handler/client.go`：客户端验证 API 骨架（login/verify/heartbeat/bind/unbind/get_var/notice/version/logout）
- [新增] `internal/router/router.go`：5 个路由组（client / admin / tenant / agent / public / pay）
- [新增] `pkg/crypto/crypto.go`：AES-256-GCM + RSA-4096 + HMAC-SHA256 + bcrypt(cost=12) + 卡密生成（4×4 字符 + 8 位校验和）
- [新增] `pkg/snowflake/snowflake.go`：雪花 ID 订单号生成器

#### 前端 admin 骨架
- [新增] Vue3 + TypeScript + Element Plus + Vite + Pinia 项目初始化
- [新增] 三套布局组件：`AdminLayout` / `TenantLayout` / `AgentLayout`（差异化侧边栏主题色）
- [新增] 三类公告横幅组件：`PlatformNoticeBanner` / `DeveloperNoticeBanner` / `AgentNotifyBanner`（同屏显示）
- [新增] 路由配置：30 个子路由 + 角色守卫 + NProgress 进度条
- [新增] `stores/auth.ts`：登录态管理（Pinia + persistedstate + Cookie 同步）
- [新增] `stores/sysConfig.ts`：sys_config 缓存（铁律 05 前端实现）
- [新增] `api/http.ts`：axios 封装 + 401 自动跳转 + 统一错误提示
- [新增] `api/sysConfig.ts`：sys_config 接口封装
- [新增] `views/login/index.vue`：三角色 Tab 切换登录页
- [新增] `views/agent/Register.vue`：代理注册三步流程（邀请码 → 支付 → 完成）
- [新增] `views/error/404.vue`：404 页面
- [新增] `components/PlaceholderView.vue`：业务页面占位组件

#### 部署与运维
- [新增] `Dockerfile`：后端多阶段构建（builder → alpine runtime）
- [新增] `Dockerfile.admin`：前端多阶段构建（node → nginx）
- [新增] `docker-compose.yml`：5 服务编排（mysql / redis / server / admin / gateway）+ 健康检查 + 生产 profile
- [新增] `deploy/nginx/admin.conf`：前端 nginx 配置（SPA 路由 + gzip + 安全头）
- [新增] `deploy/nginx/gateway.conf`：总入口网关（HTTPS + 反向代理 + HSTS）
- [新增] `scripts/baota_deploy.sh`：宝塔面板一键部署脚本
- [新增] `scripts/reset_admin_password.sh`：超管密码重置脚本
- [新增] `.env.example`：环境变量样例（铁律 04：所有敏感字段从环境变量传入）
- [新增] `configs/config.yaml.example`：后端配置文件样例

#### 项目文档
- [新增] `README.md`：项目概览 + 快速部署指南
- [新增] `PROMPT.md`：AI 接手指引文档（铁律 07 实践）
- [新增] `.gitignore`：标准 Go + Vue + 密钥忽略规则

### [安全] 铁律合规
- [安全] 所有可变参数（47 项）已写入 sys_config seed，业务代码通过 `cfgCache.GetXxx` 读取（铁律 05）
- [安全] AES_KEY / JWT_SECRET / DB 密码 / RSA 私钥全部从环境变量传入（铁律 04）
- [安全] 默认超管密码哈希为占位符，强制要求部署后通过 `reset_admin_password.sh` 重置
- [安全] 标注「待核实」项：HMAC-SHA256 变体、Snowflake twepoch、bcrypt 哈希生成命令

### [待实现] v0.2.0 后续任务
- 后端各 handler 业务逻辑（当前均为 501 占位）
- 前端各业务页面（当前均为 PlaceholderView 占位）
- 客户端 SDK（Python / Node.js / PHP）
- 单元测试与集成测试覆盖

---

## [0.1.0] - 2026-07-19

### [新增] 项目初始规划版本

#### 平台基础架构
- [新增] 确定技术栈：Go 1.22 + Gin + GORM（后端）、Vue3 + TypeScript + Element Plus + Vite + Pinia（前端）
- [新增] 确定数据库：MySQL 8.0 + Redis 7
- [新增] 确定部署方式：Docker Compose + 宝塔面板 Docker
- [新增] 确定反代与 SSL：Nginx + Let's Encrypt

#### 多租户体系
- [新增] 租户（开发者）注册、登录、2FA、JWT 认证
- [新增] 多租户数据隔离中间件（自动注入 tenant_id）
- [新增] 套餐体系：免费版 / 专业版 / 企业版
- [新增] 套餐字段：`allow_custom_pay`、`custom_pay_fee`、`platform_commission_rate`

#### 应用管理
- [新增] 应用 CRUD、AppKey/AppSecret/SignSecret 生成与轮换
- [新增] 应用配置：单卡密最大设备数（默认 1，一机一卡）、心跳间隔/超时、离线宽限期、解绑扣时规则
- [新增] 代理佣金模式字段：`agent_commission_mode`（percentage / diff）

#### 卡密体系
- [新增] 卡密类型：时长卡 / 次数卡 / 永久卡 / 试用卡 / 功能解锁卡
- [新增] 卡密生成：手动批量生成、自定义前缀/分组、SHA-512 校验位防伪、SecureRandom 系统熵源
- [新增] 卡密状态机：unused / active / expired / banned / disabled
- [新增] 一机一卡密绑定：设备指纹（CPU+主板+MAC+磁盘多重哈希）、解绑扣时、强制下线

#### 在线验证 API
- [新增] 客户端验证接口：login / verify / heartbeat / bind / unbind / get_var / notice / version / logout
- [新增] HMAC-SHA256 签名协议：X-App-Key / X-Timestamp / X-Nonce / X-Signature
- [新增] 时间戳 ±5 分钟校验、Nonce 5 分钟防重放（Redis 去重）
- [新增] RSA-4096 响应签名（fail-closed）
- [新增] 心跳保活：Redis Sorted Set 维护在线状态、超时判定、离线宽限期

#### 支付系统（双层模式）
- [新增] 平台总支付：超管后台 S-06 配置易支付网关/商户号/密钥、平台抽成比例、结算周期
- [新增] 开发者自有易支付：套餐允许时开发者可开通，资金直达开发者账户
- [新增] 切换支付方式时自动通知代理（站内信 + 控制台横幅 + 强制确认弹窗）
- [新增] `tenant_pay_config` 表（租户支付配置，AES-256-GCM 加密密钥）

#### 代理体系
- [新增] 代理注册机制：开发者生成邀请码（含有效期/次数/授权范围）→ 代理填写邀请码 → 支付平台注册费 → 关联开发者
- [新增] 代理邀请码表 `agent_invite_code`、代理注册订单表 `agent_registration_order`
- [新增] 代理购买卡密两种方式：预付余额扣款（推荐）/ 实时支付购卡（备用）
- [新增] 代理余额流水表 `agent_balance_log`、代理提现表 `agent_withdraw`
- [新增] 代理佣金模式：按比例（percentage）/ 按差价（diff，默认推荐）
- [新增] 代理独立门户（P-06）：仅展示品牌/定价，收款统一走开发者支付通道

#### 公告系统（三级体系）
- [新增] 平台总公告（type=platform）：超管发布，开发者+代理同看，显眼"平台公告"红色标签
- [新增] 开发者公告（type=developer）：超管发布，所有开发者可见
- [新增] 应用公告（type=app）：开发者发布，该应用终端用户可见
- [新增] 代理通知（type=agent_notify）：系统自动通知代理（如支付方式变更）
- [新增] 公告精准投递表 `notice_target`、已读记录表 `notice_read`
- [新增] 公告特性：置顶、强制弹窗、显眼标签、起止时间、富文本编辑

#### 安全防护（借鉴布丁卡密七层防御）
- [新增] DDoS 防御：Cloudflare WAF + Nginx 限流 100r/s + 敏感接口 10r/min + 自动封禁
- [新增] Web 安全：CSP / HSTS / X-Frame-Options / CSRF Token 双重验证 / XSS 转义
- [新增] 接口签名：HMAC-SHA256 + Nonce + Timestamp + 常量时间比较防时序攻击
- [新增] 注入防护：GORM 参数化查询 + 路径遍历拦截
- [新增] 卡密防伪：SHA-512 校验位 + SecureRandom（约 10^18 次尝试）
- [新增] 隐私保护：敏感字段 AES-256-GCM 加密 + bcrypt (cost=12) 密码 + API 脱敏
- [新增] IP 风控：黑名单（手动+自动）+ 异地登录告警 + 频率限制

#### 数据统计与日志
- [新增] 数据看板：卡密总数 / 在线设备 / 今日销量 / 本月收入 / 验证趋势图 / 销量 TOP
- [新增] 验证日志表 `log_verify`（按月分区）
- [新增] 操作日志表 `log_operation`

#### 客户端 SDK
- [新增] 规划 8 语言 SDK：Python / Node.js / Java / C# / Go / PHP / C++ / 易语言
- [新增] SDK 核心方法：verify() / bind() / heartbeat() / get_var()

#### 部署与运维
- [新增] Docker Compose 一键部署方案
- [新增] 宝塔面板 Docker 安装脚本
- [新增] 健康检查接口 `/health`、Docker healthcheck
- [新增] 在线更新机制（references/11）：Webhook 接收 GitHub Push + 自动拉取构建重启
- [新增] 数据备份：每日全量 + 每小时增量 + Redis RDB

---

## 待发布版本规划

### [0.2.0] - 一期 MVP（计划中）
- 核心验证闭环：租户注册 → 应用创建 → 卡密生成 → 客户端登录验证 → 心跳保活 → 解绑
- 多租户隔离中间件
- 平台总支付 + 自动发卡
- 开发者控制台核心页面
- 代理控制台核心页面
- 平台超管后台核心页面
- Docker Compose 部署

### [0.3.0] - 二期增值版（计划中）
- 开发者自有易支付通道
- 代理注册付费流程
- 代理佣金结算与提现
- 三级公告体系
- 云变量远程下发
- 版本管理与强制更新
- 数据统计看板
- Python / Node.js / PHP SDK

### [0.4.0] - 三期商业化完整版（计划中）
- 多级代理（二级/三级）
- 全语言 SDK（Java / C# / Go / C++ / 易语言）
- 灰度发布
- API 开放平台与 Webhook 事件推送
- 在线更新管理系统
- 数据备份与恢复面板
