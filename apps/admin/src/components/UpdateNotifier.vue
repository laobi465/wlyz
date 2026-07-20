<!--
  UpdateNotifier 管理员更新弹窗通知组件（v0.4.0）
  - 仅在 AdminLayout 中挂载
  - 按 update.poll.interval_seconds 间隔轮询 /admin/update/poll
  - localStorage 记录 last_known_commit，检测到变化弹窗提示管理员刷新
  - 严格遵循铁律 04/05：轮询开关 / 间隔 全部由后端 sys_config 控制
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

const timer = ref<ReturnType<typeof setInterval> | null>(null)
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

  // 启动定时器：使用首次返回的 interval，后续每次轮询会动态调整
  scheduleNext(interval)
}

/** 调度下一次轮询（自适应间隔） */
const scheduleNext = (intervalSec: number) => {
  timer.value = setInterval(async () => {
    const next = await pollOnce()
    if (next === 0) {
      // 后端关闭弹窗通知：停止定时器
      stopPolling()
      return
    }
    if (next !== intervalSec) {
      // 后端调整了间隔：重置定时器
      stopPolling()
      scheduleNext(next)
    }
  }, intervalSec * 1000)
}

/** 停止轮询 */
const stopPolling = () => {
  if (timer.value) {
    clearInterval(timer.value)
    timer.value = null
  }
}

onMounted(() => {
  startPolling()
})

onBeforeUnmount(() => {
  stopPolling()
})

// 暴露给父组件（可选：手动触发一次轮询）
defineExpose({
  pollOnce,
  stopPolling
})
</script>
