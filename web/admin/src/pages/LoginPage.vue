<template>
  <div class="login-container">
    <div class="login-card">
      <div class="header-top">
        <h1 class="login-title">{{ t.login.title }}</h1>
        <LanguageSwitcher />
      </div>
      <p class="login-subtitle">{{ t.login.subtitle }}</p>

      <div class="form-group">
        <label class="label">{{ t.login.tokenLabel }}</label>
        <input
          v-model="localToken"
          type="password"
          :placeholder="t.login.tokenPlaceholder"
          @keyup.enter="handleLogin"
          class="input"
        >
      </div>

      <button @click="handleLogin" class="btn-primary btn-full" :disabled="isLoggingIn">
        {{ isLoggingIn ? (t.login.loggingIn || 'Logging in...') : t.login.loginButton }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import LanguageSwitcher from '../components/LanguageSwitcher.vue'

const emit = defineEmits<{
  login: [token: string]
}>()

const { t } = useI18n()
const { token } = useAuth()
const localToken = ref(token.value)
const isLoggingIn = ref(false)

async function handleLogin() {
  if (!localToken.value.trim()) {
    alert(t.value.login.pleaseEnterToken)
    return
  }

  // Verify token by calling a protected API
  isLoggingIn.value = true
  try {
    const res = await fetch('/admin/api/products?q=', {
      headers: { 'X-Auth-Token': localToken.value }
    })

    if (res.status === 401 || res.status === 403) {
      alert(t.value.login.invalidToken || 'Invalid token')
      return
    }

    if (!res.ok) {
      alert(t.value.login.loginFailed || 'Login failed')
      return
    }

    // Token is valid, proceed with login
    emit('login', localToken.value)
  } catch (e) {
    alert(t.value.login.loginFailed || 'Login failed: ' + e)
  } finally {
    isLoggingIn.value = false
  }
}
</script>

<style scoped>
.login-container {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, #1e1b4b 0%, #312e81 50%, #1e1b4b 100%);
}

.login-card {
  background: var(--bg-card);
  padding: 40px;
  border-radius: 16px;
  box-shadow: var(--modal-shadow);
  width: 100%;
  max-width: 400px;
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.login-title {
  margin: 0;
  font-size: 28px;
  color: var(--text-bright);
}

.login-subtitle {
  margin: 0 0 32px 0;
  text-align: center;
  color: var(--text-muted);
  font-size: 14px;
}

.btn-full {
  width: 100%;
  padding: 12px;
  font-size: 16px;
  font-weight: 600;
}

</style>
