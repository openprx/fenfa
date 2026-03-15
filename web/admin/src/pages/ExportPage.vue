<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t.exportPage.title }}</h1>
      <p class="page-subtitle">{{ t.exportPage.subtitle }}</p>
    </div>

    <div class="card">
      <div class="filter-grid">
        <div class="form-group">
          <label class="label">{{ t.apps.name }} {{ t.exportPage.optional }}</label>
          <select v-model="selectedProductId" class="input">
            <option value="">{{ t.exportPage.allProducts }}</option>
            <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">{{ t.apps.platform }} {{ t.exportPage.optional }}</label>
          <select v-model="selectedVariantId" class="input" :disabled="!selectedProductId || loadingVariants">
            <option value="">{{ loadingVariants ? t.common.loading : t.exportPage.allVariants }}</option>
            <option v-for="v in variantOptions" :key="v.id" :value="v.id">
              {{ v.platform }} · {{ v.display_name || v.identifier }}
            </option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">{{ t.exportPage.startDate }}</label>
          <input v-model="from" type="date" class="input">
        </div>
        <div class="form-group">
          <label class="label">{{ t.exportPage.endDate }}</label>
          <input v-model="to" type="date" class="input">
        </div>
      </div>

      <div class="export-grid">
        <div @click="handleExport('releases')" class="export-card">
          <div class="export-icon"><Icon name="package" :size="28" /></div>
          <div class="export-title">{{ t.exportPage.releases }}</div>
          <div class="export-desc">{{ t.exportPage.releasesDesc }}</div>
        </div>

        <div @click="handleExport('events')" class="export-card">
          <div class="export-icon"><Icon name="clipboard" :size="28" /></div>
          <div class="export-title">{{ t.exportPage.events }}</div>
          <div class="export-desc">{{ t.exportPage.eventsDesc }}</div>
        </div>

        <div @click="handleExport('ios_devices')" class="export-card">
          <div class="export-icon"><Icon name="smartphone" :size="28" /></div>
          <div class="export-title">{{ t.exportPage.iosDevices }}</div>
          <div class="export-desc">{{ t.exportPage.iosDevicesDesc }}</div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import { useProducts } from '../composables/useProducts'
import { useToast } from '../composables/useToast'
import { useProductVariantSelector } from '../composables/useProductVariantSelector'
import Icon from '../components/Icon.vue'

const { t } = useI18n()
const { token, handleAuthError } = useAuth()
const { products } = useProducts()
const toast = useToast()

const {
  selectedProductId, selectedVariantId, loadingVariants, variantOptions
} = useProductVariantSelector(products, token, handleAuthError)

const from = ref('')
const to = ref('')

async function handleExport(type: string) {
  const p = new URLSearchParams()
  if (selectedVariantId.value) p.set('variant_id', selectedVariantId.value)
  if (from.value) p.set('from', from.value)
  if (to.value) p.set('to', to.value)

  const path = `/admin/exports/${type}.csv`
  const url = path + (p.toString() ? ('?' + p.toString()) : '')

  try {
    const r = await fetch(url, { headers: { 'X-Auth-Token': token.value } })
    if (handleAuthError(r)) return
    if (!r.ok) throw new Error('HTTP ' + r.status)
    const blob = await r.blob()
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = `${type}.csv`
    document.body.appendChild(a)
    a.click()
    a.remove()
    setTimeout(() => URL.revokeObjectURL(a.href), 1000)
  } catch (err) {
    toast.error(t.value.exportPage.exportFailed + ': ' + err)
  }
}
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.export-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
}

.export-card {
  background: var(--bg-deepest);
  border: 2px solid var(--border);
  border-radius: 12px;
  padding: 24px;
  text-align: center;
  cursor: pointer;
  transition: all 0.3s;
}

.export-card:hover {
  border-color: var(--purple-dark);
  transform: translateY(-4px);
  box-shadow: 0 8px 16px rgba(102, 126, 234, 0.2);
}

.export-icon {
  margin-bottom: 8px;
  color: var(--text-muted);
}

.export-title {
  font-weight: 600;
  margin-bottom: 4px;
}

.export-desc {
  font-size: 13px;
  color: var(--text-muted);
}
</style>
