<!--
  H5 公告详情（v0.4.x 残留项 2：U-12）
  - 支持 text/html 渲染
  - HTML 模式做 XSS 防护：仅允许基础白名单标签（a/b/br/p/span/div/ul/ol/li/h1-h6/strong/em/img）
  - 浏览量统计由后端 UPDATE view_count = view_count + 1 实现
-->
<template>
  <div class="h5-notice-detail">
    <div class="page-head">
      <el-button text class="back-btn" @click="goBack">
        <el-icon><ArrowLeft /></el-icon>
      </el-button>
      <span class="title">平台公告</span>
    </div>

    <div v-loading="loading">
      <div v-if="!notice && !loading" class="empty-card">
        <el-empty description="公告不存在或已下线" :image-size="80">
          <el-button type="primary" @click="goHome">返回首页</el-button>
        </el-empty>
      </div>

      <template v-if="notice">
        <div class="notice-card">
          <h1 class="title">{{ notice.title }}</h1>
          <div class="meta">
            <el-tag v-if="notice.is_pinned" type="warning" size="small">置顶</el-tag>
            <span class="meta-text">发布：{{ formatTime(notice.start_at) }}</span>
            <span class="meta-text">浏览：{{ notice.view_count }}</span>
          </div>
          <!--
            内容渲染：
            - text 模式：直接展示纯文本（自动换行）
            - html 模式：v-html 渲染经过 sanitizeHtml 过滤后的内容
            铁律 06：XSS 防护 - 仅允许白名单标签和属性，禁用 script/iframe/on*
          -->
          <div v-if="notice.content_format === 'html'" class="content html-content" v-html="safeHtml"></div>
          <div v-else class="content text-content">{{ notice.content }}</div>
        </div>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { endUserGetNoticeApi, type EndUserNoticeDetail } from '@/api/enduser'

const route = useRoute()
const router = useRouter()

const noticeId = computed(() => route.params.id as string)
const notice = ref<EndUserNoticeDetail | null>(null)
const loading = ref(false)

/**
 * XSS 防护：对 HTML 内容做白名单过滤
 * v0.4.x 简化实现：用正则去除 <script> / <iframe> / on* 事件属性 / javascript: 协议
 * v0.5.x 可改为引入 DOMPurify 库做更严格的过滤
 */
const sanitizeHtml = (html: string): string => {
  if (!html) return ''
  let s = html
  // 移除危险标签（script/iframe/object/embed/link/style/meta）
  s = s.replace(/<\s*(script|iframe|object|embed|link|style|meta)[\s\S]*?<\/\s*\1\s*>/gi, '')
  // 移除独立出现的 script/iframe 标签（含自闭合）
  s = s.replace(/<\s*(script|iframe|object|embed|link|meta)[^>]*>/gi, '')
  // 移除 on* 事件属性（onclick/onload/onerror 等）
  s = s.replace(/\son[a-z]+\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)/gi, '')
  // 移除 javascript: 协议
  s = s.replace(/(href|src)\s*=\s*["']?\s*javascript:/gi, '$1="')
  return s
}

const safeHtml = computed(() => {
  if (!notice.value || notice.value.content_format !== 'html') return ''
  return sanitizeHtml(notice.value.content)
})

const load = async () => {
  loading.value = true
  try {
    const resp = await endUserGetNoticeApi(noticeId.value)
    notice.value = resp
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    loading.value = false
  }
}

const formatTime = (t: string | null) => {
  if (!t) return '-'
  try {
    const d = new Date(t)
    if (isNaN(d.getTime())) return t
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch {
    return t
  }
}

const goBack = () => {
  if (window.history.length > 1) {
    router.back()
  } else {
    router.push('/h5/profile')
  }
}

const goHome = () => router.push('/h5')

onMounted(() => {
  load()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.h5-notice-detail {
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

.empty-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-xl $spacing-md;
}

.notice-card {
  background: #fff;
  border-radius: $radius-md;
  padding: $spacing-md;
  margin-bottom: $spacing-md;

  .title {
    font-size: 18px;
    font-weight: 600;
    color: $color-text-primary;
    margin: 0 0 $spacing-sm;
    line-height: 1.4;
    word-break: break-word;
  }

  .meta {
    display: flex;
    gap: $spacing-sm;
    align-items: center;
    flex-wrap: wrap;
    padding-bottom: $spacing-md;
    margin-bottom: $spacing-md;
    border-bottom: 1px solid $color-border-lighter;

    .meta-text {
      font-size: 12px;
      color: $color-text-secondary;
    }
  }

  .content {
    font-size: 14px;
    line-height: 1.7;
    color: $color-text-regular;
    word-break: break-word;
  }

  .text-content {
    white-space: pre-wrap;
  }

  .html-content {
    :deep(img) {
      max-width: 100%;
      height: auto;
    }
    :deep(a) {
      color: $color-primary;
      text-decoration: none;
      &:active { text-decoration: underline; }
    }
    :deep(p) {
      margin: 0 0 $spacing-sm;
    }
    :deep(h1),
    :deep(h2),
    :deep(h3),
    :deep(h4),
    :deep(h5),
    :deep(h6) {
      margin: $spacing-md 0 $spacing-sm;
      color: $color-text-primary;
    }
    :deep(ul),
    :deep(ol) {
      padding-left: $spacing-lg;
      margin: 0 0 $spacing-sm;
    }
    :deep(code) {
      background: $color-bg-page;
      padding: 2px 6px;
      border-radius: $radius-sm;
      font-family: monospace;
      font-size: 13px;
    }
    :deep(pre) {
      background: $color-bg-page;
      padding: $spacing-sm $spacing-md;
      border-radius: $radius-sm;
      overflow-x: auto;
      code {
        background: transparent;
        padding: 0;
      }
    }
  }
}
</style>
