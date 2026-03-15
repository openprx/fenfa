<template>
  <!-- Login Page -->
  <LoginPage
    v-if="!isLoggedIn"
    @login="handleLogin"
  />

  <!-- Main Layout -->
  <div v-else class="main-layout">
    <Sidebar
      @logout="handleLogout"
    />

    <main class="main-content">
      <div class="content-wrapper">
        <div v-if="isDev" class="dev-banner">
          <Icon name="wrench" :size="16" style="margin-right: 6px" /> {{ t.common.devMode }}
        </div>

        <router-view />
      </div>
    </main>

    <Toast ref="toastRef" />
    <ConfirmDialog ref="confirmRef" />
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useI18n } from './i18n'
import { useAuth } from './composables/useAuth'
import { useProducts } from './composables/useProducts'
import { provideToast } from './composables/useToast'
import { provideConfirmDialog } from './composables/useConfirmDialog'
import Icon from './components/Icon.vue'
import Toast from './components/Toast.vue'
import ConfirmDialog from './components/ConfirmDialog.vue'
import LoginPage from './pages/LoginPage.vue'
import Sidebar from './components/Sidebar.vue'

const { t } = useI18n()
const toastRef = ref<InstanceType<typeof Toast>>()
const confirmRef = ref<InstanceType<typeof ConfirmDialog>>()
provideToast(toastRef as any)
provideConfirmDialog(confirmRef as any)

const { token, isLoggedIn, handleLogin: authLogin, doLogout, handleAuthError } = useAuth()
const { fetchProducts, clearProducts } = useProducts()

const isDev = import.meta.env.DEV

function handleLogin(newToken: string) {
  authLogin(newToken)
  fetchProducts()
}

async function handleLogout() {
  const confirmed = await confirmRef.value?.show({
    title: t.value.common.logout,
    message: t.value.messages.confirmLogout,
    confirmText: t.value.common.logout,
    cancelText: t.value.common.cancel,
    type: 'danger'
  })
  if (!confirmed) return
  doLogout()
}

watch(token, (v) => {
  if (v) fetchProducts()
  else clearProducts()
})

onMounted(() => {
  if (token.value) {
    fetchProducts()
  }
})
</script>

<style scoped>
.main-layout {
  display: flex;
  min-height: 100vh;
}

.main-content {
  flex: 1;
  overflow-y: auto;
  background: var(--bg-deepest);
}

.content-wrapper {
  max-width: 1400px;
  margin: 0 auto;
  padding: 32px;
}

.dev-banner {
  margin-bottom: 20px;
  padding: 12px 16px;
  border-radius: 8px;
  background: var(--badge-bg);
  color: var(--blue-light);
  border: 1px solid var(--blue);
  font-size: 14px;
}
</style>
