<template>
  <div>
    <div class="page-header">
      <div class="page-header-row">
        <div>
          <h1 class="page-title">{{ t.apps.title }}</h1>
          <p class="page-subtitle">{{ t.apps.subtitle }}</p>
        </div>
        <button class="btn-create" @click="showCreateProduct = !showCreateProduct">
          <Icon :name="showCreateProduct ? 'x' : 'plus'" :size="16" />
          {{ showCreateProduct ? t.common.cancel : t.apps.createProduct }}
        </button>
      </div>
    </div>

    <!-- Collapsible Create Product Form -->
    <div v-if="showCreateProduct" class="card create-card">
      <div class="create-grid">
        <input v-model="newProduct.name" class="input" :placeholder="t.apps.productNamePlaceholder" />
        <input v-model="newProduct.slug" class="input" :placeholder="t.apps.productSlugPlaceholder" />
        <input v-model="newProduct.description" class="input" :placeholder="t.apps.productDescriptionPlaceholder" />
      </div>
      <div class="create-actions">
        <button class="btn-primary" :disabled="creatingProduct" @click="createProduct">
          {{ creatingProduct ? t.common.loading : t.apps.createProduct }}
        </button>
      </div>
    </div>

    <div class="card">
      <div class="search-bar">
        <input
          v-model="localQuery"
          :placeholder="t.apps.searchPlaceholder"
          class="input search-input"
        >
        <button
          @click="fetchProducts()"
          :disabled="loading"
          class="btn-ghost btn-refresh"
        >
          <Icon name="refresh-cw" :size="14" :class="{ spinning: loading }" />
          {{ loading ? t.common.loading : t.apps.refreshButton }}
        </button>
      </div>

      <div class="table-container">
        <table class="table">
          <thead>
            <tr>
              <th>{{ t.apps.name }}</th>
              <th>{{ t.apps.slug }}</th>
              <th>{{ t.apps.variantCount }}</th>
              <th>{{ t.apps.status }}</th>
              <th>{{ t.apps.createdAt }}</th>
              <th style="text-align:right">{{ t.apps.actions }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="product in products" :key="product.id">
              <tr :class="{ 'row-active': activeProductId === product.id }" @click="openProduct(product)" class="product-row">
                <td class="product-name">{{ product.name || t.apps.unnamed }}</td>
                <td class="mono">{{ product.slug || product.id }}</td>
                <td>{{ product.variant_count || 0 }}</td>
                <td>
                  <span v-if="product.published" class="status-dot status-on"><i></i>{{ t.apps.published }}</span>
                  <span v-else class="status-dot status-off"><i></i>{{ t.apps.unpublished }}</span>
                </td>
                <td class="product-date">{{ new Date(product.created_at).toLocaleString() }}</td>
                <td class="actions" @click.stop>
                  <button class="btn-text" @click="openDistribution(product)">
                    <Icon name="external-link" :size="13" />
                    {{ t.apps.viewDistribution }}
                  </button>
                  <button
                    v-if="product.published"
                    class="btn-text btn-text-warning"
                    :disabled="updatingProductId === product.id"
                    @click="updateProductPublished(product, false)"
                  >
                    {{ updatingProductId === product.id ? t.common.loading : t.apps.unpublish }}
                  </button>
                  <button
                    v-else
                    class="btn-text btn-text-success"
                    :disabled="updatingProductId === product.id"
                    @click="updateProductPublished(product, true)"
                  >
                    {{ updatingProductId === product.id ? t.common.loading : t.apps.publish }}
                  </button>
                  <button
                    class="btn-text btn-text-danger"
                    @click="deleteProduct(product)"
                  >
                    <Icon name="trash" :size="13" />
                    {{ t.common.delete }}
                  </button>
                </td>
              </tr>
              <!-- Inline Accordion: Variant Detail -->
              <tr v-if="activeProductId === product.id && activeProduct" class="detail-row">
                <td colspan="6" class="detail-cell">
                  <div class="detail-inner">
                    <div class="detail-header">
                      <div>
                        <h2 class="detail-title">{{ activeProduct.product.name }}</h2>
                        <p class="detail-subtitle">/{{ activeProduct.product.slug || activeProduct.product.id }}</p>
                      </div>
                      <button class="btn-ghost btn-ghost-sm" @click.stop="closeProduct">
                        <Icon name="x" :size="14" />
                        {{ t.common.cancel }}
                      </button>
                    </div>

                    <!-- Variants -->
                    <div class="variant-section">
                      <div v-if="activeProduct.variants.length === 0" class="empty-variants">{{ t.apps.noVariants }}</div>
                      <div v-else class="variant-grid">
                        <article v-for="variant in activeProduct.variants" :key="variant.id" class="variant-card">
                          <div class="variant-head">
                            <div>
                              <div class="variant-platform">{{ variant.platform }}</div>
                              <div class="variant-name">{{ variant.display_name || activeProduct.product.name }}</div>
                            </div>
                            <span v-if="variant.published" class="status-dot status-on"><i></i>{{ t.apps.published }}</span>
                            <span v-else class="status-dot status-off"><i></i>{{ t.apps.unpublished }}</span>
                          </div>

                          <div class="variant-meta">
                            <div v-if="variant.identifier"><span class="meta-label">{{ t.apps.identifier }}:</span> <span class="mono">{{ variant.identifier }}</span></div>
                            <div v-if="variant.arch"><span class="meta-label">Arch:</span> {{ variant.arch }}</div>
                            <div v-if="variant.installer_type"><span class="meta-label">Installer:</span> {{ variant.installer_type }}</div>
                            <div v-if="variant.min_os"><span class="meta-label">{{ t.upload.minOS }}:</span> {{ variant.min_os }}</div>
                          </div>

                          <div v-if="variant.releases?.length" class="release-summary">
                            <div class="release-title">{{ t.apps.latestRelease }}</div>
                            <div class="release-line">
                              <strong>{{ variant.releases[0].version }}</strong>
                              <span v-if="variant.releases[0].build">({{ t.upload.build }} {{ variant.releases[0].build }})</span>
                            </div>
                            <div class="release-line muted">{{ new Date(variant.releases[0].created_at).toLocaleString() }}</div>
                            <div v-if="variant.releases[0].changelog" class="release-changelog">{{ variant.releases[0].changelog }}</div>
                          </div>

                          <div class="variant-actions">
                            <button class="btn-accent" @click="openUploadModal(activeProduct.product, variant)">
                              <Icon name="upload" :size="13" />
                              {{ t.apps.uploadNewVersion }}
                            </button>
                            <div class="dropdown" @click.stop>
                              <button class="btn-ghost btn-ghost-sm" @click="toggleDropdown(variant.id)">{{ t.apps.more }}</button>
                              <div v-if="openDropdownId === variant.id" class="dropdown-menu">
                                <button class="dropdown-item" @click="router.push('/stats/' + variant.id); openDropdownId = ''">{{ t.apps.viewStats }}</button>
                                <button class="dropdown-item" @click="router.push('/events/' + variant.id); openDropdownId = ''">{{ t.apps.viewEvents }}</button>
                                <div class="dropdown-divider"></div>
                                <button
                                  v-if="variant.published"
                                  class="dropdown-item danger"
                                  :disabled="updatingVariantId === variant.id"
                                  @click="updateVariantPublished(variant, false)"
                                >
                                  {{ t.apps.unpublish }}
                                </button>
                                <button
                                  v-else
                                  class="dropdown-item"
                                  :disabled="updatingVariantId === variant.id"
                                  @click="updateVariantPublished(variant, true)"
                                >
                                  {{ t.apps.publish }}
                                </button>
                                <div class="dropdown-divider"></div>
                                <button
                                  class="dropdown-item danger"
                                  @click="deleteVariant(variant)"
                                >
                                  {{ t.apps.deleteVariant }}
                                </button>
                              </div>
                            </div>
                          </div>
                        </article>
                      </div>
                    </div>

                    <!-- Create Variant (collapsible) -->
                    <div class="create-variant">
                      <button v-if="!showCreateVariant" class="btn-dashed" @click="showCreateVariant = true">
                        <Icon name="plus" :size="14" />
                        {{ t.apps.addVariant }}
                      </button>
                      <template v-else>
                        <div class="section-header">
                          <h3>{{ t.apps.addVariant }}</h3>
                          <button class="btn-ghost btn-ghost-sm" @click="showCreateVariant = false">{{ t.common.cancel }}</button>
                        </div>
                        <div class="create-grid">
                          <select v-model="newVariant.platform" class="input">
                            <option value="ios">iOS</option>
                            <option value="android">Android</option>
                            <option value="macos">macOS</option>
                            <option value="windows">Windows</option>
                            <option value="linux">Linux</option>
                          </select>
                          <input v-model="newVariant.identifier" class="input" :placeholder="t.apps.variantIdentifier" />
                          <input v-model="newVariant.display_name" class="input" :placeholder="t.apps.variantDisplayName" />
                        </div>
                        <div class="create-actions">
                          <button class="btn-primary" :disabled="creatingVariant" @click="createVariant">
                            {{ creatingVariant ? t.common.loading : t.apps.createVariant }}
                          </button>
                        </div>
                      </template>
                    </div>
                  </div>
                </td>
              </tr>
            </template>
            <tr v-if="products.length === 0">
              <td colspan="6" class="empty-state">{{ loading ? t.common.loading : t.apps.noData }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <UploadModal
      :show="showUploadModal"
      :product="selectedProduct"
      :variant="selectedVariant"
      @close="showUploadModal = false"
      @success="handleUploadSuccess"
    />

    <DistributionModal
      :show="showDistributionModal"
      :product="selectedDistributionProduct"
      :token="token"
      @close="showDistributionModal = false"
    />
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import { useProducts } from '../composables/useProducts'
import { useToast } from '../composables/useToast'
import { useConfirmDialog } from '../composables/useConfirmDialog'
import Icon from '../components/Icon.vue'
import UploadModal from '../components/UploadModal.vue'
import DistributionModal from '../components/DistributionModal.vue'

interface ProductSummary {
  id: string
  name: string
  slug?: string
  published: boolean
  variant_count?: number
  created_at: string
}

interface ProductDetail {
  product: ProductSummary
  variants: any[]
}

const router = useRouter()
const { token, handleAuthError } = useAuth()
const { products, loadingProducts: loading, productQuery, fetchProducts } = useProducts()

const { t } = useI18n()
const toast = useToast()
const confirm = useConfirmDialog()
const localQuery = ref(productQuery.value)
const activeProduct = ref<ProductDetail | null>(null)
const activeProductId = ref('')
const updatingProductId = ref('')
const updatingVariantId = ref('')
const creatingProduct = ref(false)
const creatingVariant = ref(false)
const showUploadModal = ref(false)
const showDistributionModal = ref(false)
const showCreateProduct = ref(false)
const showCreateVariant = ref(false)
const openDropdownId = ref('')
const selectedProduct = ref<any>(null)
const selectedVariant = ref<any>(null)
const selectedDistributionProduct = ref<any>(null)

const newVariant = reactive({
  platform: 'ios',
  display_name: '',
  identifier: '',
})

const newProduct = reactive({
  name: '',
  slug: '',
  description: ''
})

// Close dropdown on outside click
function handleGlobalClick() {
  openDropdownId.value = ''
}
onMounted(() => document.addEventListener('click', handleGlobalClick))
onUnmounted(() => document.removeEventListener('click', handleGlobalClick))

function toggleDropdown(id: string) {
  openDropdownId.value = openDropdownId.value === id ? '' : id
}

watch(localQuery, (value) => {
  productQuery.value = value
})

async function fetchProductDetail(productId: string) {
  const res = await fetch(`/admin/api/products/${productId}`, {
    headers: { 'X-Auth-Token': token.value }
  })
  if (handleAuthError(res)) return null
  const payload = await res.json()
  if (!res.ok || !payload.ok) {
    throw new Error(payload.error?.message || t.value.apps.fetchFailed)
  }
  return payload.data as ProductDetail
}

function resetProductForm() {
  newProduct.name = ''
  newProduct.slug = ''
  newProduct.description = ''
}

async function openProduct(product: ProductSummary) {
  if (activeProductId.value === product.id) {
    closeProduct()
    return
  }

  try {
    activeProductId.value = product.id
    activeProduct.value = await fetchProductDetail(product.id)
    showCreateVariant.value = false
  } catch (error) {
    activeProductId.value = ''
    toast.error(String(error))
  }
}

function closeProduct() {
  activeProductId.value = ''
  activeProduct.value = null
  showCreateVariant.value = false
  resetVariantForm()
}

function resetVariantForm() {
  newVariant.platform = 'ios'
  newVariant.display_name = ''
  newVariant.identifier = ''
}

function openUploadModal(product: any, variant: any) {
  selectedProduct.value = product
  selectedVariant.value = variant
  showUploadModal.value = true
}

function openDistribution(product: ProductSummary) {
  selectedDistributionProduct.value = product
  showDistributionModal.value = true
}

async function createProduct() {
  if (!newProduct.name.trim()) {
    toast.error(t.value.apps.productNameRequired)
    return
  }

  creatingProduct.value = true
  try {
    const res = await fetch('/admin/api/products', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth-Token': token.value
      },
      body: JSON.stringify({
        name: newProduct.name,
        slug: newProduct.slug,
        description: newProduct.description
      })
    })

    if (handleAuthError(res)) return

    const payload = await res.json()
    if (!res.ok || !payload.ok) {
      toast.error(payload.error?.message || t.value.apps.createProductFailed)
      return
    }

    toast.success(t.value.apps.createProductSuccess)
    resetProductForm()
    showCreateProduct.value = false
    fetchProducts()
    await openProduct(payload.data.product)
  } catch (error) {
    toast.error(t.value.apps.createProductFailed + ': ' + error)
  } finally {
    creatingProduct.value = false
  }
}

async function createVariant() {
  if (!activeProduct.value?.product.id) {
    return
  }

  creatingVariant.value = true
  try {
    const res = await fetch(`/admin/api/products/${activeProduct.value.product.id}/variants`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth-Token': token.value
      },
      body: JSON.stringify({
        platform: newVariant.platform,
        display_name: newVariant.display_name,
        identifier: newVariant.identifier,
      })
    })

    if (handleAuthError(res)) return

    const payload = await res.json()
    if (!res.ok || !payload.ok) {
      toast.error(payload.error?.message || t.value.apps.createVariantFailed)
      return
    }

    toast.success(t.value.apps.createVariantSuccess)
    resetVariantForm()
    showCreateVariant.value = false
    fetchProducts()
    activeProduct.value = await fetchProductDetail(activeProduct.value.product.id)
  } catch (error) {
    toast.error(t.value.apps.createVariantFailed + ': ' + error)
  } finally {
    creatingVariant.value = false
  }
}

async function updateVariantPublished(variant: any, published: boolean) {
  const confirmed = await confirm.show({
    title: published ? t.value.apps.publish : t.value.apps.unpublish,
    message: published ? t.value.apps.confirmPublishVariant : t.value.apps.confirmUnpublishVariant,
    confirmText: published ? t.value.apps.publish : t.value.apps.unpublish,
    cancelText: t.value.common.cancel,
    type: published ? 'primary' : 'danger'
  })
  if (!confirmed) return

  openDropdownId.value = ''
  updatingVariantId.value = variant.id
  try {
    const res = await fetch(`/admin/api/variants/${variant.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth-Token': token.value
      },
      body: JSON.stringify({ published })
    })
    if (handleAuthError(res)) return
    const payload = await res.json()
    if (!res.ok || !payload.ok) {
      toast.error(payload.error?.message || t.value.apps.updateVariantFailed)
      return
    }

    toast.success(t.value.apps.updateVariantSuccess)
    fetchProducts()
    if (activeProduct.value) {
      activeProduct.value = await fetchProductDetail(activeProduct.value.product.id)
    }
  } catch (error) {
    toast.error(t.value.apps.updateVariantFailed + ': ' + error)
  } finally {
    updatingVariantId.value = ''
  }
}

async function handleUploadSuccess() {
  showUploadModal.value = false
  fetchProducts()
  if (activeProduct.value) {
    activeProduct.value = await fetchProductDetail(activeProduct.value.product.id)
  }
}

async function deleteProduct(product: ProductSummary) {
  const confirmed = await confirm.show({
    title: t.value.apps.deleteProduct,
    message: t.value.apps.deleteProductConfirm.replace('{name}', product.name),
    confirmText: t.value.common.delete,
    cancelText: t.value.common.cancel,
    type: 'danger'
  })
  if (!confirmed) return

  try {
    const res = await fetch(`/admin/api/products/${product.id}`, {
      method: 'DELETE',
      headers: { 'X-Auth-Token': token.value }
    })
    if (handleAuthError(res)) return
    const payload = await res.json()
    if (!res.ok || !payload.ok) {
      toast.error(payload.error?.message || t.value.apps.deleteProductFailed)
      return
    }
    toast.success(t.value.common.deleteSuccess)
    if (activeProductId.value === product.id) closeProduct()
    fetchProducts()
  } catch (error) {
    toast.error(String(error))
  }
}

async function deleteVariant(variant: any) {
  openDropdownId.value = ''
  const confirmed = await confirm.show({
    title: t.value.apps.deleteVariant,
    message: t.value.apps.deleteVariantConfirm.replace('{name}', variant.display_name || variant.identifier),
    confirmText: t.value.common.delete,
    cancelText: t.value.common.cancel,
    type: 'danger'
  })
  if (!confirmed) return

  try {
    const res = await fetch(`/admin/api/variants/${variant.id}`, {
      method: 'DELETE',
      headers: { 'X-Auth-Token': token.value }
    })
    if (handleAuthError(res)) return
    const payload = await res.json()
    if (!res.ok || !payload.ok) {
      toast.error(payload.error?.message || t.value.apps.deleteVariantFailed)
      return
    }
    toast.success(t.value.common.deleteSuccess)
    fetchProducts()
    if (activeProduct.value) {
      activeProduct.value = await fetchProductDetail(activeProduct.value.product.id)
    }
  } catch (error) {
    toast.error(String(error))
  }
}

async function updateProductPublished(product: ProductSummary, published: boolean) {
  const confirmed = await confirm.show({
    title: published ? t.value.apps.publish : t.value.apps.unpublish,
    message: published ? t.value.apps.confirmPublishProduct : t.value.apps.confirmUnpublishProduct,
    confirmText: published ? t.value.apps.publish : t.value.apps.unpublish,
    cancelText: t.value.common.cancel,
    type: published ? 'primary' : 'danger'
  })
  if (!confirmed) return

  updatingProductId.value = product.id
  try {
    const res = await fetch(`/admin/api/products/${product.id}`, {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth-Token': token.value
      },
      body: JSON.stringify({ published })
    })
    if (handleAuthError(res)) return
    const payload = await res.json()
    if (!res.ok || !payload.ok) {
      toast.error(payload.error?.message || t.value.apps.updateProductFailed)
      return
    }

    toast.success(t.value.apps.updateProductSuccess)
    fetchProducts()
    if (activeProduct.value?.product.id === product.id) {
      activeProduct.value = await fetchProductDetail(product.id)
    }
  } catch (error) {
    toast.error(t.value.apps.updateProductFailed + ': ' + error)
  } finally {
    updatingProductId.value = ''
  }
}
</script>

<style scoped>
.page-header-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
}

.create-card {
  margin-bottom: 20px;
}

.search-bar {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 16px;
  margin-bottom: 20px;
}

.search-input {
  padding: 12px 16px;
  font-size: 14px;
}

.btn-create {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--purple);
  color: #fff;
  border: none;
  border-radius: 8px;
  padding: 10px 20px;
  font-weight: 600;
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-create:hover {
  background: var(--purple-light);
  transform: translateY(-1px);
}

.btn-ghost {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border);
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-ghost:hover:not(:disabled) {
  background: var(--bg-hover);
  color: var(--text-bright);
  border-color: var(--border-hover);
}

.btn-ghost:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.btn-refresh {
  padding: 10px 20px;
  white-space: nowrap;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.spinning {
  animation: spin 1s linear infinite;
}

.table-container {
  overflow-x: auto;
}

.product-row {
  cursor: pointer;
}

.product-row:hover {
  background: var(--bg-hover) !important;
}

.row-active {
  background: var(--bg-hover) !important;
  border-left: 3px solid var(--purple);
}

.product-name {
  font-weight: 600;
}

.product-date,
.mono {
  color: var(--text-muted);
  font-family: monospace;
}

.actions {
  text-align: right;
  white-space: nowrap;
}

.actions .btn-text {
  margin-left: 4px;
}

/* Inline detail row */
.detail-row td {
  padding: 0 !important;
  border-top: none !important;
}

.detail-cell {
  background: var(--bg-deepest);
}

.detail-inner {
  padding: 24px;
}

.detail-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.detail-title {
  margin: 0;
  font-size: 18px;
}

.detail-subtitle {
  margin: 4px 0 0;
  color: var(--text-muted);
  font-family: monospace;
  font-size: 13px;
}

.variant-section {
  margin-top: 8px;
}

.create-variant {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}

.variant-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 16px;
}

.variant-card {
  border: 1px solid var(--border);
  border-radius: 14px;
  padding: 16px;
  background: var(--bg-card);
}

.variant-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.variant-platform {
  font-size: 12px;
  text-transform: uppercase;
  color: var(--text-dim);
}

.variant-name {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-bright);
}

.variant-meta,
.release-summary {
  display: grid;
  gap: 8px;
}

.meta-label,
.muted {
  color: var(--text-muted);
}

.release-summary {
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

.release-title {
  font-size: 12px;
  text-transform: uppercase;
  color: var(--text-dim);
}

.release-changelog {
  color: var(--text-body);
  white-space: pre-wrap;
}

.variant-actions {
  margin-top: 16px;
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

/* Dropdown menu */
.dropdown {
  position: relative;
}

.dropdown-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 4px;
  min-width: 160px;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: var(--modal-shadow);
  z-index: 100;
  padding: 4px 0;
}

.dropdown-item {
  display: block;
  width: 100%;
  text-align: left;
  padding: 8px 16px;
  background: none;
  border: none;
  color: var(--text-body);
  font-size: 13px;
  cursor: pointer;
}

.dropdown-item:hover {
  background: var(--bg-hover);
}

.dropdown-item.danger {
  color: var(--red);
}

.dropdown-divider {
  height: 1px;
  background: var(--border);
  margin: 4px 0;
}

.create-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 16px;
}

.create-actions {
  margin-top: 16px;
}

/* Text buttons for table row actions */
.btn-text {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 13px;
  padding: 4px 8px;
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-text:hover {
  color: var(--text-bright);
  background: var(--bg-hover);
}

.btn-text-success {
  color: var(--green);
}

.btn-text-success:hover {
  color: var(--green);
  background: var(--green-deep);
}

.btn-text-warning {
  color: var(--orange);
}

.btn-text-warning:hover {
  color: var(--orange);
  background: var(--orange-deep);
}

.btn-text-danger {
  color: var(--red);
}

.btn-text-danger:hover {
  color: var(--red);
  background: var(--red-deep);
}

.btn-text:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Accent button for variant upload */
.btn-accent {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: var(--purple);
  color: #fff;
  border: none;
  border-radius: 6px;
  padding: 6px 14px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-accent:hover {
  background: var(--purple-light);
}

/* Ghost small variant */
.btn-ghost-sm {
  padding: 6px 12px;
  font-size: 13px;
}

/* Dashed add button */
.btn-dashed {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: transparent;
  color: var(--text-muted);
  border: 1px dashed var(--border);
  border-radius: 8px;
  padding: 8px 16px;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-dashed:hover {
  color: var(--purple);
  border-color: var(--purple);
  background: var(--purple-deep);
}

.empty-state,
.empty-variants {
  text-align: center;
  padding: 32px;
  color: var(--text-dim);
}

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
</style>
