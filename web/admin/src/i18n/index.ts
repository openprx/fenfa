import { ref, computed } from 'vue'
import zh from './zh'
import en from './en'

type Locale = 'zh' | 'en'
type Messages = typeof zh

const messages: Record<Locale, Messages> = {
  zh,
  en
}

const LOCALE_KEY = 'fenfa_admin_locale'

// Get saved locale or default to Chinese
const savedLocale = localStorage.getItem(LOCALE_KEY) as Locale | null
const currentLocale = ref<Locale>(savedLocale || 'zh')

export function useI18n() {
  const locale = computed(() => currentLocale.value)
  
  const t = computed(() => messages[currentLocale.value])
  
  function setLocale(newLocale: Locale) {
    currentLocale.value = newLocale
    try {
      localStorage.setItem(LOCALE_KEY, newLocale)
    } catch (e) {
      console.warn('Failed to save locale:', e)
    }
  }
  
  return {
    locale,
    t,
    setLocale
  }
}

