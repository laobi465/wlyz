// 全局类型声明：放宽 Element Plus 表格 slot props 类型
// 避免 vue-tsc 在每个 el-table-column #default="{ row }" 都报类型错误
declare module 'element-plus' {
  interface DefaultRow {
    [key: string]: any
  }
}

// 让所有业务页面的 row slot prop 默认为 any
declare module 'vue' {
  interface ComponentCustomProperties {
    row: any
  }
}

export {}
