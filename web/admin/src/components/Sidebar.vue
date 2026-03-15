<template>
  <aside class="sidebar" :class="{ collapsed }">
    <div class="sidebar-header">
      <h2 class="sidebar-title" :class="{ hidden: collapsed }">{{ t.sidebar.title }}</h2>
      <p v-if="!collapsed" class="sidebar-subtitle">{{ t.sidebar.subtitle }}</p>
    </div>

    <nav class="sidebar-nav">
      <router-link
        v-for="item in menuItems"
        :key="item.id"
        :to="'/' + item.id"
        class="nav-item"
        active-class="active"
      >
        <span class="nav-icon"><Icon :name="item.icon" :size="20" /></span>
        <span v-if="!collapsed" class="nav-text">{{ item.label }}</span>
      </router-link>
    </nav>

    <div class="sidebar-footer">
      <div v-if="!collapsed" class="footer-controls">
        <LanguageSwitcher />
        <button @click="toggleTheme" class="theme-toggle" :title="isDark ? t.sidebar.lightMode : t.sidebar.darkMode">
          <Icon :name="isDark ? 'sun' : 'moon'" :size="16" />
        </button>
      </div>
      <button @click="$emit('logout')" class="btn-secondary btn-full">
        <span class="logout-icon"><Icon name="log-out" :size="18" /></span>
        <span v-if="!collapsed">{{ t.common.logout }}</span>
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useTheme } from '../theme'
import LanguageSwitcher from './LanguageSwitcher.vue'
import Icon from './Icon.vue'

defineProps<{
  collapsed?: boolean
}>()

defineEmits<{
  logout: []
}>()

const { t } = useI18n()
const { isDark, toggleTheme } = useTheme()

const menuItems = computed(() => [
  { id: 'apps', icon: 'smartphone', label: t.value.sidebar.apps },
  { id: 'upload', icon: 'upload', label: t.value.sidebar.upload },
  { id: 'stats', icon: 'bar-chart', label: t.value.sidebar.stats },
  { id: 'events', icon: 'clipboard', label: t.value.sidebar.events },
  { id: 'udid', icon: 'lock', label: t.value.sidebar.udid },
  { id: 'export', icon: 'save', label: t.value.sidebar.export },
  { id: 'settings', icon: 'settings', label: t.value.sidebar.settings }
])
</script>

<style scoped>
.sidebar {
  width: 260px;
  background: var(--bg-card);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  position: sticky;
  top: 0;
  height: 100vh;
  transition: width 0.3s;
}

.sidebar.collapsed {
  width: 80px;
}

.sidebar-header {
  padding: 24px 20px;
  border-bottom: 1px solid var(--border);
}

.sidebar-title {
  margin: 0;
  font-size: 20px;
  font-weight: 700;
  color: var(--text-bright);
  transition: opacity 0.3s;
}

.sidebar-title.hidden {
  opacity: 0;
}

.sidebar-subtitle {
  margin: 4px 0 0 0;
  font-size: 12px;
  color: var(--text-dim);
}

.sidebar-nav {
  flex: 1;
  padding: 16px 0;
  overflow-y: auto;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 12px 20px;
  color: var(--text-muted);
  cursor: pointer;
  transition: all 0.2s;
  border-left: 3px solid transparent;
  text-decoration: none;
}

.nav-item:hover {
  background: var(--bg-deepest);
  color: var(--text-body);
}

.nav-item.active {
  background: var(--bg-deepest);
  color: var(--text-bright);
  border-left-color: var(--purple-dark);
}

.nav-icon {
  margin-right: 12px;
  display: flex;
  align-items: center;
}

.nav-text {
  font-size: 14px;
  font-weight: 500;
}

.sidebar-footer {
  padding: 16px;
  border-top: 1px solid var(--border);
}

.footer-controls {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.theme-toggle {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: var(--border);
  color: var(--text-body);
  border: none;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.theme-toggle:hover {
  background: var(--bg-hover);
  transform: translateY(-1px);
}

.btn-full {
  width: 100%;
  padding: 10px;
  font-size: 14px;
}

.logout-icon {
  margin-right: 8px;
}
</style>
