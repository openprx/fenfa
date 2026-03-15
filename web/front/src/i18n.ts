import { computed } from 'vue'

function detectLanguage(): 'zh' | 'en' {
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh' : 'en'
}

const translations = {
  zh: {
    loading: '加载中...',
    error: {
      title: '页面暂时不可用',
      loadFailed: '加载产品信息失败'
    },
    hero: {
      productCenter: '多平台分发中心',
      legacyPage: '兼容分发页',
      subtitle: '同一页面展示全部可用平台版本，按你的设备优先推荐最合适的下载项。',
      recommended: '推荐'
    },
    actions: {
      install: '安装',
      installing: '正在安装...',
      download: '下载',
      bindDevice: '绑定设备',
      scan: '扫码访问',
      unavailable: '暂不可用'
    },
    variant: {
      identifier: '标识',
      version: '版本',
      build: '构建',
      minOS: '最低系统',
      updatedAt: '更新时间',
      allVersions: '可用版本',
      iosHint: 'iOS 首次安装前需要先绑定当前设备 UDID。'
    },
    time: {
      justNow: '刚刚',
      minutesAgo: '分钟前',
      hoursAgo: '小时前',
      daysAgo: '天前',
      weeksAgo: '周前',
      monthsAgo: '个月前'
    }
  },
  en: {
    loading: 'Loading...',
    error: {
      title: 'This page is temporarily unavailable',
      loadFailed: 'Failed to load product information'
    },
    hero: {
      productCenter: 'Multi-platform Distribution',
      legacyPage: 'Compatibility Page',
      subtitle: 'All platform builds live on one page, with a recommendation based on the current device.',
      recommended: 'Recommended'
    },
    actions: {
      install: 'Install',
      installing: 'Installing...',
      download: 'Download',
      bindDevice: 'Bind Device',
      scan: 'Scan QR Code',
      unavailable: 'Unavailable'
    },
    variant: {
      identifier: 'Identifier',
      version: 'Version',
      build: 'Build',
      minOS: 'Minimum OS',
      updatedAt: 'Updated',
      allVersions: 'Available Versions',
      iosHint: 'On iOS, the device must be registered before the first install.'
    },
    time: {
      justNow: 'Just now',
      minutesAgo: 'min ago',
      hoursAgo: 'h ago',
      daysAgo: 'd ago',
      weeksAgo: 'w ago',
      monthsAgo: 'mo ago'
    }
  }
}

export type Language = 'zh' | 'en'

let currentLanguage: Language = detectLanguage()

export function useI18n() {
  const t = computed(() => translations[currentLanguage])

  const setLanguage = (lang: Language) => {
    currentLanguage = lang
  }

  const getLanguage = () => currentLanguage

  return { t, setLanguage, getLanguage }
}
