<!--
  UpdateNotifier 管理员更新弹窗通知组件（v0.4.0）
  - 仅在 AdminLayout 中挂载
  - 按 update.poll.interval_seconds 间隔轮询 /admin/update/poll
  - localStorage 记录 last_known_commit，检测到变化弹窗提示管理员刷新
  - 严格遵循铁律 04/05：轮询开关 / 间隔 全部由后端 sys_config 控制
  - v0.6.8：修复 setInterval 累积导致 CPU 100% / 内存泄漏 / 页面卡死
            改用 setTimeout 自递归（每次 pollOnce 完成后再调度下一次，从根上消除并发）
            添加 isUnmounted 标志位防止卸载后回调执行
-->
<template>
  <!-- 无 UI，仅逻辑组件 -->
  <span style="display: none" aria-hidden="true" />
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { ElMessageBox } from 'element-plus'
import { pollUpdateApi, type UpdatePoll } from '@/api/update'

const STORAGE_KEY = 'keyauth_admin_last_known_commit'

// v0.6.8：timer 类型改为 setTimeout（自递归调度，不再用 setInterval）
const timer = ref<ReturnType<typeof setTimeout> | null>(null)
// v0.6.8：组件卸载标志位，防止 onBeforeUnmount 后排队的 setTimeout 回调继续执行
let isUnmounted = false
const notifiedCommit = ref<string>('') // 已弹窗过的 commit，避免重复弹窗

/** 从 localStorage 读取上次已知 commit */
const loadLastKnownCommit = (): string => {
  try {
    return localStorage.getItem(STORAGE_KEY) || ''
  } catch {
    return ''
  }
}

/** 写入 localStorage */
const saveLastKnownCommit = (commit: string) => {
  try {
    localStorage.setItem(STORAGE_KEY, commit)
  } catch {
    // 隐私模式 localStorage 不可用，静默降级
  }
}

/** 弹窗提示管理员刷新 */
const showRefreshDialog = (newCommit: string) => {
  // 防止重复弹窗同一 commit（用户选「稍后提醒」后本会话不再打扰）
  if (notifiedCommit.value === newCommit) return
  notifiedCommit.value = newCommit

  ElMessageBox.confirm(
    '系统检测到新版本已部署，建议立即刷新页面以应用最新版本。',
    '系统已更新',
    {
      confirmButtonText: '立即刷新',
      cancelButtonText: '稍后提醒',
      type: 'success',
      closeOnClickModal: false,
      closeOnPressEscape: false
    }
  )
    .then(() => {
      // 用户确认：刷新页面（location.reload 强制重新加载所有资源）
      window.location.reload()
    })
    .catch(() => {
      // 用户取消：保持 notifiedCommit 标记，本会话内不再重复弹窗
    })
}

/** 单次轮询，返回后端建议的轮询间隔（秒），异常返回 0 */
const pollOnce = async (): Promise<number> => {
  try {
    const data: UpdatePoll = await pollUpdateApi()

    // 后端关闭弹窗通知：返回 0 信号让外层停止定时器
    if (!data.enabled) return 0

    const currentCommit = data.current_commit || ''
    if (currentCommit) {
      const lastKnown = loadLastKnownCommit()
      if (lastKnown && lastKnown !== currentCommit) {
        // 检测到新 commit：弹窗提示
        showRefreshDialog(currentCommit)
      }
      // 不论是否弹窗，都更新 last_known_commit（避免下次重复触发）
      if (lastKnown !== currentCommit) {
        saveLastKnownCommit(currentCommit)
      }
    }

    // 返回后端建议的间隔（强制下限 10 秒，与后端 PollIntervalMin 对齐）
    return Math.max(data.interval_seconds || 30, 10)
  } catch {
    // 网络错误 / 接口异常：静默忽略，返回默认间隔让外层重试
    return 30
  }
}

/** 启动轮询（自适应间隔：每次轮询后用响应中的 interval_seconds 重置定时器） */
const startPolling = async () => {
  // 首次轮询
  const interval = await pollOnce()
  if (interval === 0) return // 后端关闭弹窗通知

  // v0.6.8：首次 await 期间组件可能已卸载，防御性检查
  if (isUnmounted) return

  // 启动定时器：使用首次返回的 interval，后续每次轮询会动态调整
  scheduleNext(interval)
}

/**
 * 调度下一次轮询（自适应间隔）
 * v0.6.8：改用 setTimeout 自递归（不再用 setInterval）
 *  - setInterval 不等待 async 回调完成 → pollOnce 耗时超过 interval 时多次回调并发执行
 *  - interval 变化时 stopPolling + scheduleNext 创建新 setInterval，但排队中的旧回调仍会执行并创建更多 setInterval → 引用覆盖泄漏
 *  - setTimeout 自递归保证：每次 pollOnce 完成后才调度下一次，从根上消除并发
 *  - isUnmounted 标志位防止组件卸载后排队的 setTimeout 回调继续执行
 */
const scheduleNext = (intervalSec: number) => {
  timer.value = setTimeout(async () => {
    // 卸载后不再执行（防御：onBeforeUnmount 已 clearTimeout，此处双保险）
    if (isUnmounted) return

    const next = await pollOnce()

    // await 期间组件可能已卸载：再次检查
    if (isUnmounted) return

    if (next === 0) {
      // 后端关闭弹窗通知：停止定时器（无需再调度）
      timer.value = null
      return
    }

    // 不论 next 是否等于 intervalSec，都重新调度（自递归）
    // - 间隔未变：继续按原间隔调度
    // - 间隔变化：自动用新间隔调度（无需 stopPolling + scheduleNext 二步操作）
    scheduleNext(next)
  }, intervalSec * 1000)
}

/** 停止轮询 */
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
  // v0.6.8：先标记卸载，再清除定时器
  // - 标记 isUnmounted 后，已在事件循环中排队但尚未执行的 setTimeout 回调开头会直接 return
  // - clearTimeout 取消尚未触发的定时器
  isUnmounted = true
  stopPolling()
})

// 暴露给父组件（可选：手动触发一次轮询）
defineExpose({
  pollOnce,
  stopPolling
})
</script>
