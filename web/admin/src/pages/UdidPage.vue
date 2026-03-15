<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t.udidPage.title }}</h1>
      <p class="page-subtitle">{{ t.udidPage.subtitle }}</p>
    </div>

    <div class="card">
      <div class="filter-grid">
        <div class="form-group">
          <label class="label">{{ t.udidPage.iosVariant }}</label>
          <select v-model="variantId" class="input" @change="queryFromFirstPage">
            <option value="">{{ t.udidPage.allVariants }}</option>
            <option v-for="variant in iosVariants" :key="variant.id" :value="variant.id">
              {{ variant.product_name }} ({{ variant.display_name || variant.identifier }})
            </option>
          </select>
        </div>
        <div class="form-group">
          <label class="label">{{ t.udidPage.search }}</label>
          <input v-model="q" :placeholder="t.udidPage.searchPlaceholder" class="input">
        </div>
        <div class="form-group">
          <label class="label">{{ t.udidPage.startDate }}</label>
          <input v-model="from" type="date" class="input">
        </div>
        <div class="form-group">
          <label class="label">{{ t.udidPage.endDate }}</label>
          <input v-model="to" type="date" class="input">
        </div>
        <div class="form-group form-group-actions">
          <button @click="queryFromFirstPage()" class="btn-primary">{{ t.common.query }}</button>
          <button @click="exportCsv" class="btn-ghost">{{ t.common.export }}</button>
        </div>
      </div>

      <!-- Batch Actions -->
      <div v-if="selectedIds.length > 0 && appleConfigured" class="batch-actions">
        <span class="selected-count">{{ t.udidPage.selectedCount.replace('{count}', String(selectedIds.length)) }}</span>
        <button @click="batchRegister" :disabled="registering" class="btn-primary">
          {{ registering ? t.udidPage.registering : t.udidPage.batchRegister }}
        </button>
      </div>

      <Pagination :total="totalItems" v-model:page="currentPage" :page-size="pageSize" />

      <div v-if="loading" class="loading">{{ t.common.loading }}</div>

      <div v-else class="table-wrapper">
        <table class="table">
          <thead>
            <tr>
              <th v-if="appleConfigured" class="th-checkbox">
                <input type="checkbox" :checked="allSelected" @change="toggleSelectAll" :indeterminate="someSelected">
              </th>
              <th>UDID</th>
              <th>{{ t.udidPage.deviceName }}</th>
              <th>{{ t.udidPage.model }}</th>
              <th>{{ t.udidPage.osVersion }}</th>
              <th>{{ t.udidPage.appleStatus }}</th>
              <th>{{ t.udidPage.createdAt }}</th>
              <th v-if="appleConfigured">{{ t.common.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(d, i) in items" :key="d.id || i">
              <td v-if="appleConfigured" class="td-checkbox">
                <input type="checkbox" :checked="selectedIds.includes(d.id)" @change="toggleSelect(d.id)" :disabled="d.apple_registered">
              </td>
              <td><code class="udid-code">{{ d.udid }}</code></td>
              <td>{{ d.device_name || '-' }}</td>
              <td>{{ d.model || '-' }}</td>
              <td>{{ d.os_version || '-' }}</td>
              <td>
                <span v-if="d.apple_registered" class="status-dot status-on"><i></i>{{ t.udidPage.registered }}</span>
                <span v-else class="status-dot status-off"><i></i>{{ t.udidPage.notRegistered }}</span>
              </td>
              <td>{{ formatTs(d.created_at) }}</td>
              <td v-if="appleConfigured">
                <button
                  v-if="!d.apple_registered"
                  @click="registerDevice(d)"
                  :disabled="d._registering"
                  class="btn-link"
                >
                  {{ d._registering ? t.udidPage.registering : t.udidPage.registerToApple }}
                </button>
                <span v-else class="text-muted">-</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <Pagination :total="totalItems" v-model:page="currentPage" :page-size="pageSize" />

      <!-- Messages -->
      <div v-if="successMessage" class="success-banner">{{ successMessage }}</div>
      <div v-if="errorMessage" class="error-banner">{{ errorMessage }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import Pagination from '../components/Pagination.vue'

const { token, handleAuthError: onAuthError } = useAuth()

const { t } = useI18n()
const items = ref<any[]>([])
const iosVariants = ref<any[]>([])
const loading = ref(false)
const q = ref('')
const from = ref('')
const to = ref('')
const variantId = ref('')
const currentPage = ref(1)
const totalItems = ref(0)
const pageSize = 50

watch([variantId, q, from, to], () => { currentPage.value = 1 })
watch(currentPage, () => fetchList())

function queryFromFirstPage() {
  if (currentPage.value === 1) fetchList()
  else currentPage.value = 1
}

// Apple registration state
const appleConfigured = ref(false)
const selectedIds = ref<string[]>([])
const registering = ref(false)
const successMessage = ref('')
const errorMessage = ref('')

// Selection computed properties
const selectableItems = computed(() => items.value.filter(d => !d.apple_registered))
const allSelected = computed(() => selectableItems.value.length > 0 && selectableItems.value.every(d => selectedIds.value.includes(d.id)))
const someSelected = computed(() => selectedIds.value.length > 0 && !allSelected.value)

async function fetchIOSVariants() {
  try {
    const r = await fetch('/admin/api/ios_variants', { headers: { 'X-Auth-Token': token.value } })
    if (onAuthError(r)) return
    const j = await r.json()
    iosVariants.value = j?.data?.items || []
  } catch (e) {
    console.error(e)
  }
}

function params() {
  const p = new URLSearchParams()
  if (variantId.value) p.set('variant_id', variantId.value)
  if (q.value) p.set('q', q.value)
  if (from.value) p.set('from', from.value)
  if (to.value) p.set('to', to.value)
  p.set('limit', String(pageSize))
  p.set('offset', String((currentPage.value - 1) * pageSize))
  return p
}

async function fetchList() {
  loading.value = true
  try {
    const url = '/admin/api/ios_devices?' + params().toString()
    const r = await fetch(url, { headers: { 'X-Auth-Token': token.value } })

    if (onAuthError(r)) return

    const j = await r.json()
    items.value = j?.data?.items || []
    totalItems.value = j?.data?.total || 0
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

async function exportCsv() {
  const path = '/admin/exports/ios_devices.csv'
  const url = path + '?' + params().toString()
  try {
    const r = await fetch(url, { headers: { 'X-Auth-Token': token.value } })

    if (onAuthError(r)) return
    if (!r.ok) throw new Error('HTTP ' + r.status)

    const blob = await r.blob()
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'ios_devices.csv'
    document.body.appendChild(a)
    a.click()
    a.remove()
    setTimeout(() => URL.revokeObjectURL(a.href), 1000)
  } catch (e) {
    alert(t.value.exportPage.exportFailed + ': ' + e)
  }
}

function formatTs(v: string | number | Date | null) {
  if (!v) return ''
  try { return new Date(v).toLocaleString() } catch { return '' }
}

// Check if Apple API is configured
async function checkAppleStatus() {
  try {
    const r = await fetch('/admin/api/apple/status', { headers: { 'X-Auth-Token': token.value } })
    if (!r.ok) return
    const j = await r.json()
    appleConfigured.value = j?.data?.configured || false
  } catch (e) {
    console.error(e)
  }
}

// Toggle select all
function toggleSelectAll() {
  if (allSelected.value) {
    selectedIds.value = []
  } else {
    selectedIds.value = selectableItems.value.map(d => d.id)
  }
}

// Toggle single selection
function toggleSelect(id: string) {
  const idx = selectedIds.value.indexOf(id)
  if (idx >= 0) {
    selectedIds.value.splice(idx, 1)
  } else {
    selectedIds.value.push(id)
  }
}

// Register single device
async function registerDevice(device: any) {
  device._registering = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    const r = await fetch(`/admin/api/devices/${device.id}/register-apple`, {
      method: 'POST',
      headers: { 'X-Auth-Token': token.value }
    })

    if (onAuthError(r)) return

    const j = await r.json()
    if (j.ok) {
      device.apple_registered = true
      device.apple_device_id = j.data?.apple_device_id
      successMessage.value = t.value.udidPage.registerSuccess
      setTimeout(() => { successMessage.value = '' }, 3000)
    } else {
      errorMessage.value = j.error?.message || t.value.udidPage.registerFailed
    }
  } catch (e) {
    errorMessage.value = t.value.udidPage.registerFailed + ': ' + e
  } finally {
    device._registering = false
  }
}

// Batch register devices
async function batchRegister() {
  if (selectedIds.value.length === 0) return

  registering.value = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    const r = await fetch('/admin/api/devices/register-apple', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth-Token': token.value
      },
      body: JSON.stringify({ device_ids: selectedIds.value })
    })

    if (onAuthError(r)) return

    const j = await r.json()
    if (j.ok) {
      const data = j.data
      // Update local state
      if (data.results) {
        for (const result of data.results) {
          if (result.success) {
            const item = items.value.find(d => d.id === result.device_id)
            if (item) {
              item.apple_registered = true
              item.apple_device_id = result.apple_device_id
            }
          }
        }
      }
      selectedIds.value = []
      successMessage.value = t.value.udidPage.batchRegisterSuccess
        .replace('{success}', String(data.success_count || 0))
        .replace('{fail}', String(data.fail_count || 0))
      setTimeout(() => { successMessage.value = '' }, 5000)
    } else {
      errorMessage.value = j.error?.message || t.value.udidPage.registerFailed
    }
  } catch (e) {
    errorMessage.value = t.value.udidPage.registerFailed + ': ' + e
  } finally {
    registering.value = false
  }
}

onMounted(async () => {
  await checkAppleStatus()
  await fetchIOSVariants()
  await fetchList()
})
</script>

<style scoped>
.filter-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 16px;
}

.form-group-actions {
  display: flex;
  flex-direction: row;
  align-items: flex-end;
  gap: 8px;
}

.batch-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 16px;
  background: var(--primary-deep);
  border: 1px solid var(--border);
  border-radius: 8px;
  margin-bottom: 16px;
}

.selected-count {
  color: var(--text-secondary);
  font-size: 13px;
}

.table-wrapper { overflow-x: auto; }

.th-checkbox, .td-checkbox {
  width: 40px;
  text-align: center;
}

.udid-code {
  font-family: 'Monaco', 'Menlo', monospace;
  font-size: 12px;
  background: var(--bg-deepest);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--text-body);
}

/* Status dot — same pattern as AppsPage */
.status-dot {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
}

.status-dot i {
  display: block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
}

.status-on {
  color: var(--green);
}

.status-on i {
  background: var(--green);
  box-shadow: 0 0 6px var(--green);
}

.status-off {
  color: var(--text-dim);
}

.status-off i {
  background: var(--text-dim);
}

.btn-link {
  background: none;
  border: none;
  color: var(--primary);
  font-size: 13px;
  cursor: pointer;
  padding: 0;
}

.btn-link:hover:not(:disabled) {
  text-decoration: underline;
}

.btn-link:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.text-muted {
  color: var(--text-dim);
}

.loading { padding: 12px; color: var(--text-muted); }
</style>
