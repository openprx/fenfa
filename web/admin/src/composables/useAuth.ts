import { ref, computed, type Ref } from 'vue'

const TOKEN_KEY = 'fenfa_admin_token'
const token = ref<string>(localStorage.getItem(TOKEN_KEY) || '')
const isLoggedIn = computed(() => !!token.value)

export function useAuth() {
  function handleLogin(newToken: string) {
    token.value = newToken
    try { localStorage.setItem(TOKEN_KEY, newToken) } catch {}
  }

  function doLogout() {
    token.value = ''
    try { localStorage.removeItem(TOKEN_KEY) } catch {}
    window.location.hash = '#/'
  }

  function handleAuthError(res: Response): boolean {
    if (res.status === 401 || res.status === 403) {
      doLogout()
      return true
    }
    return false
  }

  return {
    token: token as Ref<string>,
    isLoggedIn,
    handleLogin,
    doLogout,
    handleAuthError,
  }
}
