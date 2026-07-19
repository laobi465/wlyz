import { defineStore } from 'pinia'
import { getSysConfig } from '@/api/sysConfig'

interface SysConfigState {
  // 平台基础信息
  platformName: string
  platformUrl: string
  // 公告横幅（铁律 05：从 sys_config 读取）
  noticeBannerEnabled: boolean
  noticeBannerBg: string
  noticeBannerTextColor: string
  // 加载状态
  loaded: boolean
}

export const useSysConfigStore = defineStore('sysConfig', {
  state: (): SysConfigState => ({
    platformName: 'KeyAuth SaaS',
    platformUrl: '',
    noticeBannerEnabled: true,
    noticeBannerBg: '#fff7e6',
    noticeBannerTextColor: '#d46b08',
    loaded: false
  }),
  actions: {
    async load() {
      if (this.loaded) return
      try {
        const keys = [
          'basic.platform_name',
          'basic.platform_url',
          'notice.platform.banner_enabled',
          'notice.platform.banner_color',
          'notice.platform.banner_text_color'
        ]
        const data = await getSysConfig(keys)
        this.platformName = data['basic.platform_name'] || this.platformName
        this.platformUrl = data['basic.platform_url'] || ''
        this.noticeBannerEnabled = data['notice.platform.banner_enabled'] === '1' || data['notice.platform.banner_enabled'] === true
        this.noticeBannerBg = data['notice.platform.banner_color'] || this.noticeBannerBg
        this.noticeBannerTextColor = data['notice.platform.banner_text_color'] || this.noticeBannerTextColor
        this.loaded = true
      } catch (e) {
        // 加载失败使用默认值（铁律 06：不编造数据）
        console.warn('[sysConfig] 加载失败，使用默认值', e)
      }
    },
    reset() {
      this.$reset()
    }
  }
})
