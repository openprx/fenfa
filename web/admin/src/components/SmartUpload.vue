<template>
  <div class="smart-upload">
    <div class="upload-header">
      <h3 class="upload-title">{{ t.smartUpload.title }}</h3>
      <p class="upload-subtitle">{{ t.smartUpload.subtitle }}</p>
    </div>

    <!-- Step 1: File Selection -->
    <div v-if="step === 1" class="upload-step">
      <div
        class="dropzone dropzone-large"
        :class="{ 'dragover': isDragging, 'has-file': selectedFile }"
        @drop.prevent="handleDrop"
        @dragover.prevent="isDragging = true"
        @dragleave.prevent="isDragging = false"
        @click="triggerFileInput"
      >
        <input
          ref="fileInput"
          type="file"
          accept=".apk,.ipa"
          @change="handleFileSelect"
          style="display: none"
        >

        <div v-if="!selectedFile" class="dropzone-placeholder">
          <div class="upload-icon"><Icon name="smartphone" :size="48" /></div>
          <div class="upload-text">
            <p class="primary-text primary-text-lg">{{ t.smartUpload.dragDropText }}</p>
            <p class="secondary-text">{{ t.smartUpload.orClickToSelect }}</p>
            <p class="file-type-text">{{ t.smartUpload.supportedFiles }}</p>
          </div>
        </div>

        <div v-else class="file-selected file-selected-lg">
          <div class="file-icon"><Icon name="package" :size="36" /></div>
          <div class="file-info">
            <div class="file-name file-name-lg">{{ selectedFile.name }}</div>
            <div class="file-size">{{ formatFileSize(selectedFile.size) }}</div>
          </div>
          <button @click.stop="clearFile" class="remove-file-btn remove-file-btn-lg">✕</button>
        </div>
      </div>

      <div v-if="selectedFile" class="step-actions">
        <button @click="parseAppInfo" :disabled="parsing" class="btn-primary btn-parse">
          <Icon name="search" :size="16" />
          {{ parsing ? t.smartUpload.parsing : t.smartUpload.parseButton }}
        </button>
      </div>

      <!-- Parse Progress -->
      <div v-if="parsing" class="progress-section">
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: parseProgress + '%' }"></div>
        </div>
        <div class="progress-text">{{ t.smartUpload.parsing }} {{ parseProgress }}%</div>
      </div>

      <div v-if="parseError" class="error-message">
        <Icon name="x-circle" :size="16" style="margin-right: 4px" /> {{ t.smartUpload.parseFailed }}: {{ parseError }}
      </div>
    </div>

    <!-- Step 2: Confirm Info -->
    <div v-if="step === 2 && appInfo" class="upload-step">
      <div class="app-preview-card">
        <div class="preview-header">
          <div class="app-icon-container">
            <img v-if="appInfo.icon_base64" :src="appInfo.icon_base64" class="app-icon" alt="App Icon">
            <div v-else class="app-icon-placeholder"><Icon name="smartphone" :size="40" /></div>
          </div>
          <div class="app-basic-info">
            <h3 class="app-name">{{ appInfo.app_name || t.smartUpload.unnamed }}</h3>
            <div class="app-meta">
              <span class="badge">{{ appInfo.platform }}</span>
              <span class="version-text">{{ appInfo.version || appInfo.version_name }} <span v-if="appInfo.build || appInfo.version_code">({{ appInfo.build || appInfo.version_code }})</span></span>
            </div>
          </div>
        </div>

        <div class="info-grid">
          <div class="info-item">
            <span class="info-label">{{ t.smartUpload.identifier }}:</span>
            <span class="info-value">{{ appInfo.bundle_id || appInfo.package_name }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t.smartUpload.minOS }}:</span>
            <span class="info-value">{{ appInfo.min_os || appInfo.min_sdk }}</span>
          </div>
          <div class="info-item">
            <span class="info-label">{{ t.smartUpload.fileSize }}:</span>
            <span class="info-value">{{ formatFileSize(appInfo.file_size) }}</span>
          </div>
        </div>
      </div>

      <div class="optional-fields">
        <h4 class="section-title">{{ t.smartUpload.optionalInfo }}</h4>

        <div class="form-row">
          <div class="form-group">
            <label class="label">{{ t.smartUpload.channel }}</label>
            <input v-model="form.channel" :placeholder="t.smartUpload.channelPlaceholder" class="input">
          </div>
        </div>

        <div class="form-group">
          <label class="label">{{ t.smartUpload.changelog }}</label>
          <textarea v-model="form.changelog" :placeholder="t.smartUpload.changelogPlaceholder" class="input textarea" rows="3"></textarea>
        </div>
      </div>

      <div class="step-actions">
        <button @click="step = 1" class="btn-secondary">{{ t.common.back }}</button>
        <button @click="handleUpload" :disabled="uploading" class="btn-primary">
          {{ uploading ? t.smartUpload.uploading : t.smartUpload.uploadButton }}
        </button>
      </div>

      <!-- Upload Progress -->
      <div v-if="uploading" class="progress-section">
        <div class="progress-bar">
          <div class="progress-fill" :style="{ width: uploadProgress + '%' }"></div>
        </div>
        <div class="progress-text">{{ t.smartUpload.uploading }} {{ uploadProgress }}%</div>
      </div>

      <!-- Upload Result -->
      <div v-if="uploadResult" class="result-section" :class="uploadResult.ok ? 'success' : 'error'">
        <div v-if="uploadResult.ok">
          <div class="result-title"><Icon name="check-circle" :size="16" style="margin-right: 4px" /> {{ t.smartUpload.uploadSuccess }}</div>
          <div class="result-content">
            <div><strong>{{ t.smartUpload.downloadPage }}:</strong> <a :href="uploadResult.data.urls.page" target="_blank">{{ uploadResult.data.urls.page }}</a></div>
          </div>
        </div>
        <div v-else class="error-message">
          <Icon name="x-circle" :size="16" style="margin-right: 4px" /> {{ t.smartUpload.uploadFailed }}: {{ uploadResult.error?.message || t.smartUpload.unknownError }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from '../i18n'
import Icon from './Icon.vue'

const props = defineProps<{
  token: string
  onAuthError: (res: Response) => boolean
  variant: { id: string; platform: string; identifier: string } | null
}>()

// Dynamic upload URL (may point to a different domain)
const uploadBaseURL = ref('')

async function fetchUploadConfig() {
  try {
    const res = await fetch('/admin/api/upload-config', {
      headers: { 'X-Auth-Token': props.token }
    })
    if (res.ok) {
      const data = await res.json()
      if (data.ok && data.data?.upload_domain) {
        uploadBaseURL.value = data.data.upload_domain
      }
    }
  } catch (_) { /* fallback to same origin */ }
}

onMounted(fetchUploadConfig)

const emit = defineEmits<{
  success: []
}>()

const { t } = useI18n()

const step = ref(1)
const fileInput = ref<HTMLInputElement | null>(null)
const selectedFile = ref<File | null>(null)
const isDragging = ref(false)
const parsing = ref(false)
const parseProgress = ref(0)
const parseError = ref('')
const appInfo = ref<any>(null)
const uploading = ref(false)
const uploadProgress = ref(0)
const uploadResult = ref<any>(null)

const form = reactive({
  channel: 'internal',
  changelog: ''
})

function triggerFileInput() {
  fileInput.value?.click()
}

function handleFileSelect(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (file) {
    selectedFile.value = file
    parseError.value = ''
  }
}

function handleDrop(event: DragEvent) {
  isDragging.value = false
  const file = event.dataTransfer?.files?.[0]
  if (file) {
    const ext = file.name.split('.').pop()?.toLowerCase()
    if (ext === 'apk' || ext === 'ipa') {
      selectedFile.value = file
      parseError.value = ''
    } else {
      parseError.value = t.value.smartUpload.invalidFileType
    }
  }
}

function clearFile() {
  selectedFile.value = null
  appInfo.value = null
  step.value = 1
  parseProgress.value = 0
  parseError.value = ''
  if (fileInput.value) {
    fileInput.value.value = ''
  }
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

async function parseAppInfo() {
  if (!selectedFile.value) return
  if (!props.variant) {
    parseError.value = t.value.upload.selectVariantFirst
    return
  }

  parsing.value = true
  parseProgress.value = 0
  parseError.value = ''

  try {
    const fd = new FormData()
    fd.set('app_file', selectedFile.value)

    const xhr = new XMLHttpRequest()

    xhr.upload.addEventListener('progress', (e) => {
      if (e.lengthComputable) {
        parseProgress.value = Math.round((e.loaded / e.total) * 100)
      }
    })

    const result = await new Promise<any>((resolve, reject) => {
      xhr.addEventListener('load', () => {
        if (xhr.status === 401 || xhr.status === 403) {
          const fakeRes = new Response(null, { status: xhr.status })
          props.onAuthError(fakeRes)
          reject(new Error('AUTH_ERROR'))
          return
        }
        if (xhr.status === 200) {
          resolve(JSON.parse(xhr.responseText))
        } else {
          reject(new Error(`HTTP ${xhr.status}`))
        }
      })
      xhr.addEventListener('error', () => reject(new Error('Network error')))
      xhr.open('POST', uploadBaseURL.value + '/admin/api/parse-app')
      xhr.setRequestHeader('X-Auth-Token', props.token)
      xhr.send(fd)
    })

    if (result.ok) {
      if (result.data?.platform !== props.variant.platform) {
        parseError.value = t.value.smartUpload.platformMismatch
        return
      }
      appInfo.value = result.data
      step.value = 2
    } else {
      parseError.value = result.error?.message || t.value.smartUpload.unknownError
    }
  } catch (error: any) {
    if (error.message === 'AUTH_ERROR') return
    parseError.value = error.message
  } finally {
    parsing.value = false
  }
}

async function handleUpload() {
  if (!selectedFile.value || !appInfo.value || !props.variant) return

  const fd = new FormData()
  fd.set('variant_id', props.variant.id)
  if (appInfo.value.version || appInfo.value.version_name) {
    fd.set('version', appInfo.value.version || appInfo.value.version_name)
  }
  if (appInfo.value.build || appInfo.value.version_code) {
    fd.set('build', String(appInfo.value.build || appInfo.value.version_code))
  }
  if (appInfo.value.min_os || appInfo.value.min_sdk) {
    fd.set('min_os', appInfo.value.min_os || appInfo.value.min_sdk)
  }
  if (form.channel) {
    fd.set('channel', form.channel)
  }
  if (form.changelog) {
    fd.set('changelog', form.changelog)
  }
  if (appInfo.value.icon_base64) {
    fd.set('icon_base64', appInfo.value.icon_base64)
  }
  fd.set('app_file', selectedFile.value)

  uploading.value = true
  uploadProgress.value = 0
  uploadResult.value = null

  try {
    const xhr = new XMLHttpRequest()

    xhr.upload.addEventListener('progress', (e) => {
      if (e.lengthComputable) {
        uploadProgress.value = Math.round((e.loaded / e.total) * 100)
      }
    })

    const response = await new Promise<any>((resolve, reject) => {
      xhr.addEventListener('load', () => {
        if (xhr.status === 200 || xhr.status === 201) {
          resolve(JSON.parse(xhr.responseText))
        } else if (xhr.status === 401 || xhr.status === 403) {
          reject(new Error('AUTH_ERROR'))
        } else {
          reject(new Error(`HTTP ${xhr.status}`))
        }
      })
      xhr.addEventListener('error', () => reject(new Error('Network error')))
      xhr.open('POST', uploadBaseURL.value + '/upload')
      xhr.setRequestHeader('X-Auth-Token', props.token)
      xhr.send(fd)
    })

    uploadResult.value = response
    if (response.ok) {
      setTimeout(() => {
        emit('success')
        resetForm()
      }, 2000)
    }
  } catch (error: any) {
    if (error.message === 'AUTH_ERROR') {
      const fakeRes = new Response(null, { status: 401 })
      props.onAuthError(fakeRes)
      return
    }
    uploadResult.value = {
      ok: false,
      error: { message: error.message }
    }
  } finally {
    uploading.value = false
  }
}

function resetForm() {
  step.value = 1
  selectedFile.value = null
  appInfo.value = null
  form.channel = 'internal'
  form.changelog = ''
  parseProgress.value = 0
  uploadProgress.value = 0
  uploadResult.value = null
  parseError.value = ''
}
</script>

<style scoped>
/* Component-specific layout only — shared patterns from common.css */
.smart-upload {
  max-width: 800px;
  margin: 0 auto;
}

.upload-header {
  text-align: center;
  margin-bottom: 32px;
}

.upload-title {
  font-size: 24px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.upload-subtitle {
  font-size: 14px;
  color: var(--text-muted);
}

.upload-step {
  background: var(--bg-card);
  border-radius: 12px;
  padding: 32px;
}

.dropzone-large {
  padding: 48px 32px;
  margin-bottom: 24px;
}

.primary-text-lg {
  font-size: 18px;
}

.upload-icon {
  color: var(--text-muted);
}

.upload-text {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.file-selected-lg {
  padding: 16px;
  gap: 20px;
}

.file-icon {
  color: var(--text-muted);
}

.file-info {
  flex: 1;
  text-align: left;
}

.file-name-lg {
  font-size: 16px;
}

.file-size {
  font-size: 14px;
}

.remove-file-btn-lg {
  width: 32px;
  height: 32px;
  font-size: 18px;
}

.app-preview-card {
  background: var(--bg-deepest);
  border-radius: 12px;
  padding: 24px;
  margin-bottom: 24px;
}

.preview-header {
  display: flex;
  gap: 20px;
  margin-bottom: 24px;
  padding-bottom: 24px;
  border-bottom: 1px solid var(--border);
}

.app-icon-container {
  flex-shrink: 0;
}

.app-icon {
  width: 80px;
  height: 80px;
  border-radius: 16px;
  object-fit: cover;
}

.app-icon-placeholder {
  width: 80px;
  height: 80px;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--purple), var(--purple-light));
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 40px;
}

.app-basic-info {
  flex: 1;
}

.app-name {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-bright);
  margin: 0 0 12px 0;
}

.app-meta {
  display: flex;
  align-items: center;
  gap: 12px;
}

.version-text {
  font-size: 14px;
  color: var(--text-muted);
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.info-label {
  font-size: 12px;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.info-value {
  font-size: 14px;
  color: var(--text-bright);
  font-family: monospace;
}

.optional-fields {
  margin-bottom: 24px;
}

.section-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 16px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  margin-bottom: 16px;
}

.textarea {
  resize: vertical;
  min-height: 80px;
}

.step-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.btn-parse {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 14px;
  font-size: 16px;
}

@media (max-width: 640px) {
  .form-row {
    grid-template-columns: 1fr;
  }

  .info-grid {
    grid-template-columns: 1fr;
  }
}
</style>
