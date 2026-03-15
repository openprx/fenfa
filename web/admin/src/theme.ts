import { ref, computed } from 'vue'

type Theme = 'dark' | 'light'

const THEME_KEY = 'fenfa_admin_theme'

const savedTheme = localStorage.getItem(THEME_KEY)
const currentTheme = ref<Theme>(savedTheme === 'light' ? 'light' : 'dark')

// Apply on load
if (currentTheme.value !== 'dark') {
  document.documentElement.dataset.theme = currentTheme.value
}

export function useTheme() {
  const isDark = computed(() => currentTheme.value === 'dark')

  function setTheme(t: Theme) {
    currentTheme.value = t
    if (t === 'dark') {
      delete document.documentElement.dataset.theme
    } else {
      document.documentElement.dataset.theme = t
    }
    try {
      localStorage.setItem(THEME_KEY, t)
    } catch (e) {
      console.warn('Failed to save theme:', e)
    }
  }

  function toggleTheme() {
    setTheme(currentTheme.value === 'dark' ? 'light' : 'dark')
  }

  return { theme: currentTheme, isDark, setTheme, toggleTheme }
}
