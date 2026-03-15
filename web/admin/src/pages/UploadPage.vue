<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t.upload.title }}</h1>
      <p class="page-subtitle">{{ t.upload.subtitle }}</p>
    </div>

    <div class="card selectors-card">
      <div class="selectors-grid">
        <div class="form-group">
          <label class="label">{{ t.apps.name }}</label>
          <select v-model="selectedProductId" class="input">
            <option value="">{{ t.upload.selectProduct }}</option>
            <option v-for="product in products" :key="product.id" :value="product.id">
              {{ product.name }}
            </option>
          </select>
        </div>

        <div class="form-group">
          <label class="label">{{ t.apps.platform }}</label>
          <select v-model="selectedVariantId" class="input" :disabled="!selectedProductId || loadingVariants">
            <option value="">{{ loadingVariants ? t.common.loading : t.upload.selectVariant }}</option>
            <option v-for="variant in variantOptions" :key="variant.id" :value="variant.id">
              {{ variant.platform }} · {{ variant.identifier }}
            </option>
          </select>
        </div>
      </div>
    </div>

    <!-- Not ready: prompt user -->
    <div v-if="!selectedVariant" class="card upload-prompt">
      <p v-if="!selectedProductId">{{ t.upload.selectProductFirst }}</p>
      <p v-else-if="loadingVariants">{{ t.common.loading }}</p>
      <p v-else-if="variantOptions.length === 0">{{ t.upload.noVariantsHint }}</p>
      <p v-else>{{ t.upload.selectVariantFirst }}</p>
    </div>

    <!-- Ready: show upload area -->
    <template v-else>
      <div v-if="isMobilePlatform" class="mode-selector">
        <button
          @click="uploadMode = 'smart'"
          class="mode-btn"
          :class="{ active: uploadMode === 'smart' }"
        >
          <div class="mode-title">{{ t.upload.smartMode }}</div>
          <div class="mode-desc">{{ t.upload.smartModeDesc }}</div>
        </button>
        <button
          @click="uploadMode = 'manual'"
          class="mode-btn"
          :class="{ active: uploadMode === 'manual' }"
        >
          <div class="mode-title">{{ t.upload.manualMode }}</div>
          <div class="mode-desc">{{ t.upload.manualModeDesc }}</div>
        </button>
      </div>

      <SmartUpload
        v-if="uploadMode === 'smart' && isMobilePlatform"
        :token="token"
        :on-auth-error="handleAuthError"
        :variant="selectedVariant"
        @success="fetchProducts"
      />

      <div v-else class="card">
        <div class="form-grid">
          <div class="form-group">
            <label class="label">{{ t.upload.versionRequired }}</label>
            <input v-model="form.version" :placeholder="t.upload.versionPlaceholder" class="input">
          </div>

          <div class="form-group">
            <label class="label">{{ t.upload.build }}</label>
            <input v-model="form.build" :placeholder="t.upload.buildPlaceholder" class="input">
          </div>

          <div class="form-group">
            <label class="label">{{ t.upload.channel }}</label>
            <input v-model="form.channel" :placeholder="t.upload.channelPlaceholder" class="input">
          </div>

          <div class="form-group">
            <label class="label">{{ t.upload.minOS }}</label>
            <input v-model="form.min_os" :placeholder="t.upload.minOSPlaceholder" class="input">
          </div>
        </div>

        <div class="form-group full-width">
          <label class="label">{{ t.upload.changelog }}</label>
          <textarea v-model="form.changelog" :placeholder="t.upload.changelogPlaceholder" class="input textarea" rows="3"></textarea>
        </div>

        <div class="form-group full-width">
          <label class="label">{{ t.upload.fileRequired }}</label>
          <input ref="fileInput" type="file" :accept="acceptTypes" class="input file-input">
        </div>

        <div class="upload-actions">
          <button @click="doUpload" :disabled="uploading" class="btn-primary btn-upload">
            {{ uploading ? t.upload.uploading : t.upload.uploadButton }}
          </button>
        </div>

        <div v-if="response" class="result-section" :class="response.ok ? 'success' : 'error'">
          <div v-if="response.ok">
            <div class="result-title">{{ t.upload.uploadSuccess }}</div>
            <div class="result-content">
              <div><strong>{{ t.upload.downloadPage }}:</strong> <a :href="response.data.urls.page" target="_blank">{{ response.data.urls.page }}</a></div>
              <div><strong>{{ t.upload.directDownload }}:</strong> <a :href="response.data.urls.download" target="_blank">{{ response.data.urls.download }}</a></div>
            </div>
          </div>
          <div v-else class="error-message">
            {{ t.upload.uploadFailed }}: {{ response.error?.message || t.upload.unknownError }}
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch, onMounted } from 'vue'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import { useProducts } from '../composables/useProducts'
import { useToast } from '../composables/useToast'
import { useProductVariantSelector } from '../composables/useProductVariantSelector'
import SmartUpload from '../components/SmartUpload.vue'

const { t } = useI18n()
const { token, handleAuthError } = useAuth()
const { products, fetchProducts } = useProducts()
const toast = useToast()

const {
  selectedProductId, selectedVariantId, loadingVariants,
  variantOptions, selectedVariant
} = useProductVariantSelector(products, token, handleAuthError)

const uploadMode = ref<'smart' | 'manual'>('smart')
const uploading = ref(false)
const response = ref<any>(null)
const uploadBaseURL = ref('')

const form = reactive({
  version: '',
  build: '',
  channel: '',
  min_os: '',
  changelog: ''
})

const fileInput = ref<HTMLInputElement | null>(null)

const isMobilePlatform = computed(() => {
  return selectedVariant.value && ['ios', 'android'].includes(selectedVariant.value.platform)
})

const acceptTypes = computed(() => {
  const platform = selectedVariant.value?.platform || ''
  const map: Record<string, string> = {
    ios: '.ipa',
    android: '.apk',
    macos: '.dmg,.pkg,.zip',
    windows: '.exe,.msi,.zip',
    linux: '.appimage,.deb,.rpm,.zip,.tar.gz'
  }
  return map[platform] || '*'
})

watch(selectedVariant, (variant) => {
  if (!variant) return
  if (['ios', 'android'].includes(variant.platform)) {
    uploadMode.value = 'smart'
  } else {
    uploadMode.value = 'manual'
  }
})

async function fetchUploadConfig() {
  try {
    const res = await fetch('/admin/api/upload-config', {
      headers: { 'X-Auth-Token': token.value }
    })
    if (res.ok) {
      const data = await res.json()
      if (data.ok && data.data?.upload_domain) {
        uploadBaseURL.value = data.data.upload_domain
      }
    }
  } catch {}
}

async function doUpload() {
  const file = fileInput.value?.files?.[0]
  if (!selectedVariant.value) {
    toast.error(t.value.upload.selectVariantFirst)
    return
  }
  if (!file) {
    toast.error(t.value.upload.pleaseSelectFile)
    return
  }
  if (!form.version) {
    toast.error(t.value.upload.pleaseEnterVersion)
    return
  }

  const fd = new FormData()
  fd.set('variant_id', selectedVariant.value.id)
  fd.set('version', form.version)
  if (form.build) fd.set('build', form.build)
  if (form.channel) fd.set('channel', form.channel)
  if (form.min_os) fd.set('min_os', form.min_os)
  if (form.changelog) fd.set('changelog', form.changelog)
  fd.set('app_file', file)

  uploading.value = true
  try {
    const res = await fetch(uploadBaseURL.value + '/upload', {
      method: 'POST',
      headers: { 'X-Auth-Token': token.value },
      body: fd
    })
    if (handleAuthError(res)) return
    const j = await res.json()
    response.value = j
    if (!j?.ok) {
      toast.error(t.value.upload.uploadFailed + ': ' + (j?.error?.message || res.status))
    } else {
      toast.success(t.value.upload.uploadSuccess)
      fetchProducts()
    }
  } catch (e) {
    toast.error(t.value.upload.uploadFailed + ': ' + e)
  } finally {
    uploading.value = false
  }
}

onMounted(() => {
  fetchUploadConfig()
})
</script>

<style scoped>
.selectors-card {
  margin-bottom: 20px;
}

.selectors-grid,
.form-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 20px;
}

.mode-selector {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 24px;
}

.mode-btn {
  background: var(--bg-card);
  border: 2px solid var(--border);
  border-radius: 12px;
  padding: 20px;
  cursor: pointer;
  text-align: left;
}

.mode-btn.active {
  border-color: var(--purple);
  box-shadow: 0 0 0 3px rgba(124, 58, 237, 0.12);
}

.mode-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-bright);
}

.mode-desc {
  margin-top: 6px;
  color: var(--text-muted);
  font-size: 13px;
}

.upload-prompt {
  text-align: center;
  color: var(--text-muted);
  padding: 48px 24px;
}

.full-width {
  margin-top: 20px;
}

.textarea {
  resize: vertical;
  font-family: inherit;
}

.file-input {
  padding: 10px;
}

.upload-actions {
  margin-top: 24px;
}

.btn-upload {
  padding: 14px 32px;
  font-size: 16px;
}

@media (max-width: 768px) {
  .mode-selector {
    grid-template-columns: 1fr;
  }
}
</style>
