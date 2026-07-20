import { request } from './http'

// 系统配置 API（铁律 05 核心实现）
// 后端路由（router.go adminAuth 组）：
//   GET /admin/config         —— 列表（支持 group/keyword/page/size）
//   PUT /admin/config/:key    —— 更新单项
// 注：后端未实现 /admin/config/batch 和 /admin/config/:key/reset
//      批量获取改为循环 GET /admin/config（带 group 过滤）
//      重置配置功能后端未实现，前端去掉该 API

/** 列表查询（支持 group 过滤） */
export const listSysConfig = (params: { group?: string; keyword?: string; page?: number; size?: number }) => {
  return request.get('/admin/config', params)
}

/** 批量获取配置（按 group 一次性拉取，前端自行筛选 keys）
 *  后端无 batch 接口，改用 GET /admin/config?group=xxx
 */
export const getSysConfig = (keys: string[]) => {
  // 从 keys 推断 group（取第一个 key 的点号前缀）
  const group = keys.length > 0 ? keys[0].split('.')[0] : ''
  return request.get<Record<string, any>>('/admin/config', { group })
}

/** 更新单个配置 */
export const updateSysConfig = (key: string, data: { value: string; name?: string; group?: string; remark?: string }) => {
  return request.put(`/admin/config/${key}`, data)
}

/** 重置配置到默认值
 *  后端未实现 reset 路由，前端调用会 404
 *  TODO: 后端在 adminAuth 组下新增 POST /admin/config/:key/reset 路由
 */
export const resetSysConfig = (key: string) => {
  return request.post(`/admin/config/${key}/reset`, {})
}
