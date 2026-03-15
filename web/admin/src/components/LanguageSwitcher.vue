<template>
  <div class="language-switcher">
    <button 
      @click="toggleLanguage" 
      class="lang-button"
      :title="locale === 'zh' ? 'Switch to English' : '切换到中文'"
    >
      <span class="lang-icon"><Icon name="globe" :size="16" /></span>
      <span class="lang-text">{{ locale === 'zh' ? '中文' : 'EN' }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from '../i18n'
import Icon from './Icon.vue'

const { locale, setLocale } = useI18n()

function toggleLanguage() {
  setLocale(locale.value === 'zh' ? 'en' : 'zh')
}
</script>

<style scoped>
.language-switcher {
  display: flex;
  align-items: center;
}

.lang-button {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  background: var(--border);
  color: var(--text-body);
  border: none;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.lang-button:hover {
  background: var(--bg-hover);
  transform: translateY(-1px);
}

.lang-icon {
  display: flex;
  align-items: center;
}

.lang-text {
  font-weight: 600;
}
</style>

