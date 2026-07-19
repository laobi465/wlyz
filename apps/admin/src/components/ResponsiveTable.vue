<!--
  ResponsiveTable 响应式表格组件
  - 桌面端：显示完整 el-table
  - 移动端：自动切换为卡片列表
-->
<template>
  <div class="responsive-table">
    <!-- 桌面端表格 -->
    <el-table
      v-if="!isMobile"
      :data="data"
      v-loading="loading"
      stripe
      style="width: 100%"
      @selection-change="onSelectionChange"
    >
      <el-table-column v-if="selectable" type="selection" width="48" />
      <slot />
      <template #empty>
        <el-empty :description="emptyText" :image-size="80" />
      </template>
    </el-table>

    <!-- 移动端卡片列表 -->
    <div v-else class="card-list" v-loading="loading">
      <div v-for="(item, idx) in data" :key="idx" class="card-item">
        <slot name="mobile-card" :item="item" :index="idx">
          <!-- 默认卡片渲染：从 mobileFields 读取字段 -->
          <div v-for="field in mobileFields" :key="field.prop" class="card-row">
            <span class="label">{{ field.label }}</span>
            <span class="value">{{ formatValue(item, field) }}</span>
          </div>
        </slot>
        <!-- 卡片操作按钮 -->
        <div v-if="$slots['mobile-actions']" class="card-actions">
          <slot name="mobile-actions" :item="item" :index="idx" />
        </div>
      </div>
      <el-empty v-if="!loading && !data.length" :description="emptyText" :image-size="80" />
    </div>

    <!-- 分页 -->
    <div v-if="showPagination && total > 0" class="pagination-wrap">
      <el-pagination
        v-model:current-page="currentPage"
        v-model:page-size="pageSizeRef"
        :total="total"
        :page-sizes="pageSizes"
        :layout="isMobile ? 'prev, pager, next' : 'total, sizes, prev, pager, next, jumper'"
        background
        @current-change="onPageChange"
        @size-change="onSizeChange"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'

export interface MobileField {
  prop: string
  label: string
  formatter?: (value: any, row: any) => string
}

const props = withDefaults(defineProps<{
  data: any[]
  loading?: boolean
  total?: number
  page?: number
  pageSize?: number
  pageSizes?: number[]
  showPagination?: boolean
  selectable?: boolean
  emptyText?: string
  /** 移动端卡片显示字段（默认渲染） */
  mobileFields?: MobileField[]
}>(), {
  loading: false,
  total: 0,
  page: 1,
  pageSize: 20,
  pageSizes: () => [10, 20, 50, 100],
  showPagination: true,
  selectable: false,
  emptyText: '暂无数据',
  mobileFields: () => []
})

const emit = defineEmits<{
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', size: number): void
  (e: 'page-change', page: number): void
  (e: 'size-change', size: number): void
  (e: 'selection-change', rows: any[]): void
}>()

const isMobile = ref(false)
const currentPage = ref(props.page)
const pageSizeRef = ref(props.pageSize)

watch(() => props.page, (v) => { currentPage.value = v })
watch(() => props.pageSize, (v) => { pageSizeRef.value = v })

const checkMobile = () => { isMobile.value = window.innerWidth < 768 }
onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})
onBeforeUnmount(() => window.removeEventListener('resize', checkMobile))

const onPageChange = (p: number) => {
  currentPage.value = p
  emit('update:page', p)
  emit('page-change', p)
}
const onSizeChange = (s: number) => {
  pageSizeRef.value = s
  emit('update:pageSize', s)
  emit('size-change', s)
}
const onSelectionChange = (rows: any[]) => emit('selection-change', rows)

const formatValue = (item: any, field: MobileField) => {
  const value = item?.[field.prop]
  if (field.formatter) return field.formatter(value, item)
  if (value === null || value === undefined || value === '') return '-'
  return String(value)
}
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.responsive-table {
  width: 100%;
}

.pagination-wrap {
  margin-top: $spacing-md;
  display: flex;
  justify-content: flex-end;

  @include mobile {
    justify-content: center;
    :deep(.el-pagination) {
      .el-pagination__sizes { display: none; }
      .el-pagination__jump { display: none; }
    }
  }
}

.card-list {
  .card-item {
    background: $color-bg-card;
    border: 1px solid $color-border-lighter;
    border-radius: $radius-md;
    padding: $spacing-md;
    margin-bottom: $spacing-sm;

    .card-row {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 6px 0;
      font-size: 13px;
      border-bottom: 1px solid $color-border-lighter;

      &:last-child { border-bottom: none; }
      .label { color: $color-text-secondary; }
      .value {
        color: $color-text-primary;
        font-weight: 500;
        text-align: right;
        max-width: 60%;
        word-break: break-all;
      }
    }
    .card-actions {
      display: flex;
      gap: $spacing-sm;
      justify-content: flex-end;
      margin-top: $spacing-sm;
      padding-top: $spacing-sm;
      border-top: 1px solid $color-border-lighter;
      flex-wrap: wrap;
    }
  }
}
</style>
