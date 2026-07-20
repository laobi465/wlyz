<!--
  CountUp 数字滚动动效（v0.5.0）
  - 基于 requestAnimationFrame + easeOutCubic 缓动
  - 零依赖（不引入 gsap/countup.js）
  - props：
      value        目标数值（必填）
      duration     动画时长 ms（默认 800）
      decimals     小数位数（默认 0）
      prefix       前缀（默认 ''，如 '¥'）
      suffix       后缀（默认 ''，如 ' 元'）
      separator    千分位分隔符（默认 ''，如 ','）
      autoplay     是否自动播放（默认 true，false 时需调用 start()）
  - 行为：
      - value 变化时自动从当前显示值过渡到新值
      - duration=0 时直接显示终值（无动画）
      - 组件卸载时自动取消 raf，避免内存泄漏
  - 铁律 06：所有数值来自父组件传入，组件本身不编造任何数据
-->
<template>
  <span class="count-up">{{ displayText }}</span>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onBeforeUnmount, computed } from 'vue'

const props = withDefaults(defineProps<{
  value: number
  duration?: number
  decimals?: number
  prefix?: string
  suffix?: string
  separator?: string
  autoplay?: boolean
}>(), {
  duration: 800,
  decimals: 0,
  prefix: '',
  suffix: '',
  separator: '',
  autoplay: true
})

// 当前显示的数值（动画进行中的中间值）
const current = ref(0)
// 上一次的起始值（用于 value 变化时的平滑过渡）
const fromValue = ref(0)
let rafId: number | null = null
let startTime = 0

// 缓动函数：easeOutCubic（开始快 → 末尾慢，更适合数字滚动）
const easeOutCubic = (t: number) => 1 - Math.pow(1 - t, 3)

// 格式化数值：处理小数位 + 千分位分隔符
const format = (n: number): string => {
  // 修正浮点精度问题（如 0.1 + 0.2）
  const fixed = n.toFixed(props.decimals)
  if (!props.separator) return fixed
  const [intPart, decPart] = fixed.split('.')
  const withSep = intPart.replace(/\B(?=(\d{3})+(?!\d))/g, props.separator)
  return decPart !== undefined ? `${withSep}.${decPart}` : withSep
}

const displayText = computed(() => `${props.prefix}${format(current.value)}${props.suffix}`)

const cancelAnim = () => {
  if (rafId !== null) {
    cancelAnimationFrame(rafId)
    rafId = null
  }
}

const animate = (to: number) => {
  cancelAnim()
  // 铁律 06：duration=0 时直接显示终值
  if (props.duration <= 0) {
    current.value = to
    return
  }
  fromValue.value = current.value
  startTime = performance.now()

  const step = (now: number) => {
    const elapsed = now - startTime
    const progress = Math.min(elapsed / props.duration, 1)
    const eased = easeOutCubic(progress)
    current.value = fromValue.value + (to - fromValue.value) * eased

    if (progress < 1) {
      rafId = requestAnimationFrame(step)
    } else {
      current.value = to
      rafId = null
    }
  }
  rafId = requestAnimationFrame(step)
}

const start = () => animate(props.value)

watch(() => props.value, (newVal) => {
  animate(newVal)
})

onMounted(() => {
  if (props.autoplay) {
    // 首次挂载：从 0 滚动到目标值
    current.value = 0
    animate(props.value)
  } else {
    current.value = props.value
  }
})

onBeforeUnmount(() => {
  cancelAnim()
})

defineExpose({ start })
</script>

<style scoped lang="scss">
.count-up {
  // 继承父级样式（字体大小、颜色等由调用方控制）
  display: inline;
  font-variant-numeric: tabular-nums; // 等宽数字，避免滚动抖动
}
</style>
