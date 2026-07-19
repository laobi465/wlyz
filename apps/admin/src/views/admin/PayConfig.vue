<!--
  支付配置（超管）- 响应式 H5
  - 平台总支付（彩虹易支付）参数配置
  - 所有参数走 sys_config 表，键前缀 pay.platform.*（铁律 05：可变参数后台化）
  - 铁律 04：禁止硬编码密钥/token；铁律 06：后端 501 时静默降级，不编造数据
-->
<template>
  <div class="pay-config-page">
    <PageHeader title="支付配置" subtitle="平台总支付（彩虹易支付）参数配置" />

    <div class="app-card">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="所有支付参数均存储于 sys_config 表，键前缀 pay.platform.*（铁律 05：可变参数后台化）"
        description="敏感字段（商户密钥）默认隐藏，点击「显示」可查看。保存将逐项写入 sys_config。"
      />

      <div class="form-actions">
        <el-button type="primary" :loading="saveLoading" @click="saveAll">保存</el-button>
        <el-button :loading="testLoading" @click="testPay">测试支付配置</el-button>
        <el-button @click="loadList">刷新</el-button>
      </div>

      <el-form v-loading="loading" label-position="top" class="pay-form">
        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="商户 ID (pid)">
              <el-input v-model="form.pid" placeholder="如 10000" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="商户密钥 (key)">
              <el-input
                v-model="form.key"
                :type="showKey ? 'text' : 'password'"
                placeholder="商户密钥"
              >
                <template #append>
                  <el-button text @click="showKey = !showKey">{{ showKey ? '隐藏' : '显示' }}</el-button>
                </template>
              </el-input>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="网关地址 (api_url)">
              <el-input v-model="form.api_url" placeholder="https://example.com/submit.php" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="签名类型 (sign_type)">
              <el-select v-model="form.sign_type" placeholder="选择签名类型">
                <el-option label="MD5" value="MD5" />
                <el-option label="SHA1" value="SHA1" />
                <!-- 待核实：彩虹易支付其他签名类型 -->
              </el-select>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="异步通知路径 (notify_path)">
              <el-input v-model="form.notify_path" placeholder="/api/v1/pay/notify" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="同步跳转路径 (return_path)">
              <el-input v-model="form.return_path" placeholder="/pay/return" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="前端回跳地址 (return_front_url)">
              <el-input v-model="form.return_front_url" placeholder="https://example.com/pay/result" />
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="订单名前缀 (order_name_prefix)">
              <el-input v-model="form.order_name_prefix" placeholder="如 KA-" maxlength="32" />
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="默认抽成比例 (commission_default, %)">
              <el-input-number v-model="form.commission_default" :min="0" :max="100" :step="1" :precision="2" />
              <span class="hint">0~100，单位 %</span>
            </el-form-item>
          </el-col>
          <el-col :xs="24" :sm="12">
            <el-form-item label="结算最低金额 (settlement.min_amount, ¥)">
              <el-input-number v-model="form.settlement_min_amount" :min="0" :step="1" :precision="2" />
              <span class="hint">单位 元</span>
            </el-form-item>
          </el-col>
        </el-row>

        <el-row :gutter="20">
          <el-col :xs="24" :sm="12">
            <el-form-item label="自动结算 (settlement.auto_enabled)">
              <el-switch v-model="form.settlement_auto_enabled" active-text="开启" inactive-text="关闭" />
              <span class="hint">开启后满足条件自动结算</span>
            </el-form-item>
          </el-col>
        </el-row>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import PageHeader from '@/components/PageHeader.vue'
import { listSysConfig, updateSysConfig } from '@/api/sysConfig'
import { testPayConfigApi } from '@/api/pay'

interface SysConfigItem {
  id: number
  config_key: string
  config_value: string
  config_type: string
  config_name: string
  config_group: string
  remark: string
}

// 表单字段 -> sys_config key 映射
const keyMap: Record<string, string> = {
  pid: 'pay.platform.pid',
  key: 'pay.platform.key',
  api_url: 'pay.platform.api_url',
  sign_type: 'pay.platform.sign_type',
  notify_path: 'pay.platform.notify_path',
  return_path: 'pay.platform.return_path',
  return_front_url: 'pay.platform.return_front_url',
  order_name_prefix: 'pay.platform.order_name_prefix',
  commission_default: 'pay.platform.commission_default',
  settlement_min_amount: 'pay.platform.settlement.min_amount',
  settlement_auto_enabled: 'pay.platform.settlement.auto_enabled'
}

const loading = ref(false)
const saveLoading = ref(false)
const testLoading = ref(false)
const showKey = ref(false)

const form = reactive({
  pid: '',
  key: '',
  api_url: '',
  sign_type: 'MD5',
  notify_path: '',
  return_path: '',
  return_front_url: '',
  order_name_prefix: '',
  commission_default: 0,
  settlement_min_amount: 0,
  settlement_auto_enabled: false
})

// 保存原始值用于判断是否变更
const original: Record<string, any> = {}

const parseValue = (k: string, v: string) => {
  if (k === 'commission_default' || k === 'settlement_min_amount') {
    const n = Number(v)
    return isNaN(n) ? 0 : n
  }
  if (k === 'settlement_auto_enabled') {
    return v === 'true' || v === '1' || v === 'on'
  }
  return v || ''
}

const serializeValue = (k: string, v: any): string => {
  if (k === 'settlement_auto_enabled') {
    return v ? 'true' : 'false'
  }
  if (k === 'commission_default' || k === 'settlement_min_amount') {
    return String(v ?? 0)
  }
  return String(v ?? '')
}

const applyItem = (item: SysConfigItem) => {
  // 反向匹配字段
  for (const field of Object.keys(keyMap)) {
    if (item.config_key === keyMap[field]) {
      const parsed = parseValue(field, item.config_value)
      ;(form as any)[field] = parsed
      original[field] = parsed
      break
    }
  }
}

const loadList = async () => {
  loading.value = true
  try {
    // 加载 group=pay 的所有配置，前端再过滤 pay.platform.* 前缀
    const resp = await listSysConfig({ group: 'pay', page: 1, size: 200 }) as any
    const list: SysConfigItem[] = resp?.list || resp?.data?.list || []
    const matched = list.filter(it => it.config_key.startsWith('pay.platform.'))
    if (matched.length === 0) {
      // 待核实：若后端尚未初始化支付配置，保持默认值，不编造数据
      ElMessage.info('暂无支付配置数据，可填写后保存')
    } else {
      matched.forEach(applyItem)
    }
  } catch {
    // 错误已由 http 拦截器处理（后端 501 时静默降级）
  } finally {
    loading.value = false
  }
}

const saveAll = async () => {
  saveLoading.value = true
  let okCount = 0
  let failCount = 0
  for (const field of Object.keys(keyMap)) {
    const cur = (form as any)[field]
    if (original[field] === cur) continue // 跳过未变更
    try {
      await updateSysConfig(keyMap[field], {
        value: serializeValue(field, cur),
        name: field,
        group: 'pay'
      })
      original[field] = cur
      okCount++
    } catch {
      failCount++
    }
  }
  saveLoading.value = false
  if (okCount > 0 && failCount === 0) {
    ElMessage.success(`保存成功（${okCount} 项）`)
  } else if (okCount > 0 && failCount > 0) {
    ElMessage.warning(`部分保存成功：成功 ${okCount} 项，失败 ${failCount} 项`)
  } else if (failCount > 0) {
    ElMessage.error('保存失败')
  } else {
    ElMessage.info('没有需要保存的变更')
  }
}

const testPay = async () => {
  testLoading.value = true
  try {
    const resp = await testPayConfigApi()
    if (resp?.ok) {
      ElMessage.success(`配置测试通过，网关：${resp.gateway_url}，PID：${resp.pid}`)
    } else {
      ElMessage.warning('测试未通过，请检查配置')
    }
  } catch {
    // 错误已由 http 拦截器处理
  } finally {
    testLoading.value = false
  }
}

onMounted(() => {
  loadList()
})
</script>

<style scoped lang="scss">
@use '@/styles/variables.scss' as *;

.pay-config-page {
  .form-actions {
    display: flex;
    gap: $spacing-sm;
    flex-wrap: wrap;
    margin: $spacing-md 0;
  }
  .pay-form {
    margin-top: $spacing-sm;
  }
  .hint {
    font-size: 12px;
    color: $color-text-secondary;
    margin-left: $spacing-sm;
  }
  .el-input-number {
    width: 180px;
  }
  .el-select {
    width: 100%;
  }
}
</style>
