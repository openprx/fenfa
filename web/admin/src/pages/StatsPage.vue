<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t.stats.title }}</h1>
      <p class="page-subtitle">{{ t.stats.subtitle }}</p>
    </div>

    <div class="card">
      <div class="filter-bar">
        <div class="form-group">
          <label class="label">{{ t.apps.name }}</label>
          <select v-model="selectedProductId" class="input">
            <option value="">{{ t.upload.selectProduct }}</option>
            <option v-for="p in products" :key="p.id" :value="p.id">{{ p.name }}</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">{{ t.apps.platform }}</label>
          <select v-model="selectedVariantId" class="input" :disabled="!selectedProductId || loadingVariants">
            <option value="">{{ loadingVariants ? t.common.loading : t.upload.selectVariant }}</option>
            <option v-for="v in variantOptions" :key="v.id" :value="v.id">
              {{ v.platform }} · {{ v.display_name || v.identifier }}
            </option>
          </select>
        </div>
      </div>

      <div v-if="loading" class="loading-state">{{ t.common.loading }}</div>

      <div v-else-if="statsData">
        <div class="app-info">
          <div class="app-info-name">{{ statsData.variant?.display_name || statsData.product?.name }}</div>
          <div class="app-info-meta">
            <span class="badge">{{ statsData.variant?.platform }}</span>
            <span v-if="statsData.variant?.identifier" class="app-info-id">{{ statsData.variant.identifier }}</span>
            <span v-if="statsData.product?.name" class="app-info-product">{{ statsData.product.name }}</span>
          </div>
        </div>

        <div class="table-container">
          <table class="table">
            <thead>
              <tr>
                <th>{{ t.stats.version }}</th>
                <th>{{ t.stats.build }}</th>
                <th>{{ t.stats.downloadCount }}</th>
                <th>{{ t.stats.createdAt }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="r in statsData.releases" :key="r.id">
                <td class="version">{{ r.version }}</td>
                <td>{{ r.build }}</td>
                <td><span class="download-count">{{ r.download_count }}</span></td>
                <td class="date">{{ new Date(r.created_at).toLocaleString() }}</td>
              </tr>
              <tr v-if="!statsData.releases?.length">
                <td colspan="4" class="empty-state">{{ t.stats.noReleases }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-else class="empty-state">
        <p v-if="!selectedProductId">{{ t.upload.selectProductFirst }}</p>
        <p v-else-if="!selectedVariantId">{{ t.upload.selectVariantFirst }}</p>
        <p v-else>{{ t.stats.emptyState }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import { useProducts } from '../composables/useProducts'
import { useProductVariantSelector } from '../composables/useProductVariantSelector'

const props = defineProps<{
  variantId?: string
}>()

const route = useRoute()
const { t } = useI18n()
const { token, handleAuthError } = useAuth()
const { products } = useProducts()

const {
  selectedProductId, selectedVariantId, loadingVariants,
  variantOptions, initFromVariantId
} = useProductVariantSelector(products, token, handleAuthError)

const loading = ref(false)
const statsData = ref<any>(null)

watch(selectedVariantId, (value) => {
  if (value) fetchStats(value)
  else statsData.value = null
})

async function fetchStats(variantId: string) {
  loading.value = true
  statsData.value = null
  try {
    const res = await fetch(`/admin/api/variants/${variantId}/stats`, {
      headers: { 'X-Auth-Token': token.value }
    })
    if (handleAuthError(res)) return
    const payload = await res.json()
    if (res.ok && payload.ok) {
      statsData.value = payload.data
    }
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

// Handle route param for variant ID
const initVariantId = props.variantId || (route.params.variantId as string)
if (initVariantId) {
  initFromVariantId(initVariantId).then(data => {
    if (data) statsData.value = data
  })
}
</script>

<style scoped>
.filter-bar {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 24px;
}

.loading-state {
  text-align: center;
  padding: 40px;
  color: var(--text-muted);
}

.app-info {
  padding: 16px;
  background: var(--bg-deepest);
  border-radius: 8px;
  margin-bottom: 20px;
}

.app-info-name {
  font-size: 18px;
  font-weight: 600;
  margin-bottom: 4px;
}

.app-info-meta {
  font-size: 14px;
  color: var(--text-muted);
  display: flex;
  align-items: center;
  gap: 8px;
}

.app-info-id {
  font-family: monospace;
}

.app-info-product {
  color: var(--text-dim);
}

.table-container {
  overflow-x: auto;
}

.version {
  font-weight: 600;
}

.download-count {
  color: var(--green);
  font-weight: 600;
}

.date {
  color: var(--text-muted);
}

.empty-state {
  text-align: center;
  padding: 48px 20px;
  color: var(--text-dim);
}

@media (max-width: 640px) {
  .filter-bar {
    grid-template-columns: 1fr;
  }
}
</style>
