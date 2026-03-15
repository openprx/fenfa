<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t.events.title }}</h1>
      <p class="page-subtitle">{{ t.events.subtitle }}</p>
    </div>

    <div class="card">
      <div class="filter-grid">
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
        <div class="form-group">
          <label class="label">{{ t.events.eventType }}</label>
          <select v-model="eventType" class="input">
            <option value="">{{ t.events.all }}</option>
            <option value="visit">{{ t.events.visit }}</option>
            <option value="click">{{ t.events.click }}</option>
            <option value="download">{{ t.events.download }}</option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">{{ t.events.startDate }}</label>
          <input v-model="from" type="date" class="input">
        </div>
        <div class="form-group">
          <label class="label">{{ t.events.endDate }}</label>
          <input v-model="to" type="date" class="input">
        </div>
      </div>

      <button
        @click="queryFromFirstPage()"
        :disabled="loading"
        class="btn-primary btn-query"
      >
        <Icon name="search" :size="14" />
        {{ loading ? t.common.loading : t.events.queryButton }}
      </button>

      <Pagination :total="totalItems" v-model:page="currentPage" :page-size="pageSize" />

      <div class="table-container">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t.events.time }}</th>
              <th>{{ t.events.type }}</th>
              <th>{{ t.events.variantId }}</th>
              <th>{{ t.events.releaseId }}</th>
              <th>{{ t.events.ipAddress }}</th>
              <th>{{ t.events.userAgent }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in events" :key="e.ts + e.ip">
              <td class="event-time">{{ new Date(e.ts).toLocaleString() }}</td>
              <td><span class="badge">{{ e.type }}</span></td>
              <td class="event-id">{{ e.variant_id }}</td>
              <td class="event-id">{{ e.release_id }}</td>
              <td class="event-ip">{{ e.ip }}</td>
              <td class="event-ua">
                <template v-if="parseExtra(e.extra)?.ua">
                  {{ parseExtra(e.extra)?.ua?.browser?.name }} {{ parseExtra(e.extra)?.ua?.browser?.version }} ·
                  {{ parseExtra(e.extra)?.ua?.os?.name }} {{ parseExtra(e.extra)?.ua?.os?.version }}
                </template>
                <template v-else>{{ e.ua }}</template>
              </td>
            </tr>
            <tr v-if="events.length === 0">
              <td colspan="6" class="empty-state">
                {{ loading ? t.common.loading : t.events.noData }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <Pagination :total="totalItems" v-model:page="currentPage" :page-size="pageSize" />
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
import Icon from '../components/Icon.vue'
import Pagination from '../components/Pagination.vue'

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
const events = ref<any[]>([])
const eventType = ref('')
const from = ref('')
const to = ref('')
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 50

watch([selectedProductId, selectedVariantId, eventType, from, to], () => { currentPage.value = 1 })
watch(currentPage, () => fetchEvents())

function queryFromFirstPage() {
  if (currentPage.value === 1) fetchEvents()
  else currentPage.value = 1 // watch will trigger fetchEvents
}

async function fetchEvents() {
  loading.value = true
  try {
    const p = new URLSearchParams()
    if (selectedVariantId.value) p.set('variant_id', selectedVariantId.value)
    if (eventType.value) p.set('type', eventType.value)
    if (from.value) p.set('from', from.value)
    if (to.value) p.set('to', to.value)
    p.set('limit', String(pageSize))
    p.set('offset', String((currentPage.value - 1) * pageSize))
    const res = await fetch('/admin/api/events?' + p.toString(), {
      headers: { 'X-Auth-Token': token.value }
    })
    if (handleAuthError(res)) return
    const payload = await res.json()
    events.value = payload?.data?.items || []
    totalItems.value = payload?.data?.total || 0
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

function parseExtra(s: string) {
  try { return JSON.parse(s) } catch { return null }
}

// Handle route param for variant ID
const initVariantId = props.variantId || (route.params.variantId as string)
if (initVariantId) {
  initFromVariantId(initVariantId).then(() => {
    fetchEvents()
  })
}
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.btn-query {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 12px 24px;
  margin-bottom: 20px;
}

.table-container {
  overflow-x: auto;
}

.event-time {
  color: var(--text-muted);
  font-size: 13px;
  white-space: nowrap;
}

.event-id {
  font-family: monospace;
  font-size: 13px;
  color: var(--text-dim);
}

.event-ip {
  font-family: monospace;
  font-size: 13px;
}

.event-ua {
  font-size: 13px;
  color: var(--text-muted);
}

.empty-state {
  text-align: center;
  padding: 40px;
  color: var(--text-dim);
}
</style>
