<template>
  <div>
    <div class="page-header">
      <h1 class="page-title">{{ t.settings.title }}</h1>
      <p class="page-subtitle">{{ t.settings.subtitle }}</p>
    </div>

    <div v-if="loading" class="loading-state">
      {{ t.common.loading }}
    </div>

    <form v-else @submit.prevent="handleSave">
      <!-- General Settings -->
      <div class="card">
        <h2 class="card-title">{{ t.settings.generalTitle }}</h2>

        <div class="form-group">
          <label class="label required">{{ t.settings.primaryDomain }}</label>
          <input
            v-model="form.primaryDomain"
            type="text"
            class="input"
            :placeholder="t.settings.primaryDomainPlaceholder"
            required
          >
          <p class="help-text">{{ t.settings.primaryDomainHelp }}</p>
          <p v-if="errors.primaryDomain" class="error-text">{{ errors.primaryDomain }}</p>
        </div>

        <div class="form-group">
          <label class="label">{{ t.settings.secondaryDomains }}</label>
          <textarea
            v-model="secondaryDomainsText"
            class="input textarea"
            :placeholder="t.settings.secondaryDomainsPlaceholder"
            rows="4"
          ></textarea>
          <p class="help-text">{{ t.settings.secondaryDomainsHelp }}</p>
        </div>

        <div class="form-group">
          <label class="label required">{{ t.settings.organization }}</label>
          <input
            v-model="form.organization"
            type="text"
            class="input"
            :placeholder="t.settings.organizationPlaceholder"
            required
          >
          <p class="help-text">{{ t.settings.organizationHelp }}</p>
          <p v-if="errors.organization" class="error-text">{{ errors.organization }}</p>
        </div>
      </div>

      <!-- Storage Configuration -->
      <div class="card section-card">
        <h2 class="card-title">{{ t.storage.title }}</h2>
        <p class="card-subtitle">{{ t.storage.subtitle }}</p>

        <div class="form-group">
          <label class="label">{{ t.storage.storageType }}</label>
          <div class="radio-group">
            <label class="radio-label" :class="{ active: storageForm.storageType === 'local' }">
              <input type="radio" v-model="storageForm.storageType" value="local">
              <span class="radio-title">{{ t.storage.local }}</span>
              <span class="radio-desc">{{ t.storage.localDesc }}</span>
            </label>
            <label class="radio-label" :class="{ active: storageForm.storageType === 's3' }">
              <input type="radio" v-model="storageForm.storageType" value="s3">
              <span class="radio-title">{{ t.storage.s3 }}</span>
              <span class="radio-desc">{{ t.storage.s3Desc }}</span>
            </label>
          </div>
        </div>

        <div class="form-group">
          <label class="label">{{ t.storage.uploadDomain }}</label>
          <input
            v-model="storageForm.uploadDomain"
            type="text"
            class="input"
            :placeholder="t.storage.uploadDomainPlaceholder"
          >
          <p class="help-text">{{ t.storage.uploadDomainHelp }}</p>
        </div>

        <template v-if="storageForm.storageType === 's3'">
          <div class="form-group">
            <label class="label">{{ t.storage.s3Endpoint }}</label>
            <input v-model="storageForm.s3Endpoint" type="text" class="input" :placeholder="t.storage.s3EndpointPlaceholder">
            <p class="help-text">{{ t.storage.s3EndpointHelp }}</p>
          </div>

          <div class="form-group">
            <label class="label">{{ t.storage.s3Bucket }}</label>
            <input v-model="storageForm.s3Bucket" type="text" class="input" :placeholder="t.storage.s3BucketPlaceholder">
            <p class="help-text">{{ t.storage.s3BucketHelp }}</p>
          </div>

          <div class="form-group">
            <label class="label">{{ t.storage.s3AccessKey }}</label>
            <input v-model="storageForm.s3AccessKey" type="text" class="input" :placeholder="t.storage.s3AccessKeyPlaceholder">
          </div>

          <div class="form-group">
            <label class="label">{{ t.storage.s3SecretKey }}</label>
            <input v-model="storageForm.s3SecretKey" type="password" class="input" :placeholder="storageForm.s3SecretKeySet ? t.storage.s3SecretKeyConfigured : t.storage.s3SecretKeyPlaceholder">
          </div>

          <div class="form-group">
            <label class="label">{{ t.storage.s3PublicURL }}</label>
            <input v-model="storageForm.s3PublicURL" type="text" class="input" :placeholder="t.storage.s3PublicURLPlaceholder">
            <p class="help-text">{{ t.storage.s3PublicURLHelp }}</p>
          </div>
        </template>
      </div>

      <!-- Apple Developer Configuration -->
      <div class="card section-card">
        <h2 class="card-title">{{ t.apple.title }}</h2>
        <p class="card-subtitle">{{ t.apple.subtitle }}</p>

        <div class="apple-status" :class="appleStatus.connected ? 'connected' : (appleStatus.configured ? 'configured' : 'not-configured')">
          <span class="status-icon">
            <Icon v-if="appleStatus.connected" name="check-circle" :size="16" />
            <Icon v-else-if="appleStatus.configured" name="alert-triangle" :size="16" />
            <Icon v-else name="x-circle" :size="16" />
          </span>
          <span class="status-text">{{ appleStatus.message }}</span>
          <button type="button" @click="testAppleConnection" :disabled="testingConnection" class="btn-sm btn-test">
            {{ testingConnection ? t.apple.testing : t.apple.testConnection }}
          </button>
        </div>

        <div class="form-group">
          <label class="label">{{ t.apple.keyId }}</label>
          <input v-model="appleForm.keyId" type="text" class="input" :placeholder="t.apple.keyIdPlaceholder">
          <p class="help-text">{{ t.apple.keyIdHelp }}</p>
        </div>

        <div class="form-group">
          <label class="label">{{ t.apple.issuerId }}</label>
          <input v-model="appleForm.issuerId" type="text" class="input" :placeholder="t.apple.issuerIdPlaceholder">
          <p class="help-text">{{ t.apple.issuerIdHelp }}</p>
        </div>

        <div class="form-group">
          <label class="label">{{ t.apple.teamId }}</label>
          <input v-model="appleForm.teamId" type="text" class="input" :placeholder="t.apple.teamIdPlaceholder">
          <p class="help-text">{{ t.apple.teamIdHelp }}</p>
        </div>

        <div class="form-group">
          <label class="label">{{ t.apple.privateKey }}</label>
          <div class="file-input-wrapper">
            <input type="file" accept=".p8,.pem" @change="handlePrivateKeyFile" class="file-input">
            <span v-if="appleForm.privateKeySet" class="file-status configured">{{ t.apple.privateKeyConfigured }}</span>
            <span v-else-if="appleForm.privateKey" class="file-status selected">{{ t.apple.privateKeySelected }}</span>
          </div>
          <p class="help-text">{{ t.apple.privateKeyHelp }}</p>
        </div>
      </div>

      <!-- Single Save Button -->
      <div class="save-bar">
        <div v-if="lastUpdated" class="info-text">
          {{ t.settings.lastUpdated }}: {{ lastUpdated }}
        </div>
        <button type="submit" :disabled="saving" class="btn-primary btn-save">
          {{ saving ? t.settings.saving : t.settings.saveButton }}
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useI18n } from '../i18n'
import { useAuth } from '../composables/useAuth'
import { useToast } from '../composables/useToast'
import Icon from '../components/Icon.vue'

const { token, handleAuthError: onAuthError } = useAuth()

const { t } = useI18n()
const toast = useToast()

const loading = ref(true)
const saving = ref(false)
const lastUpdated = ref('')

const form = reactive({
  primaryDomain: '',
  organization: ''
})

const secondaryDomainsText = ref('')

const errors = reactive({
  primaryDomain: '',
  organization: ''
})

const storageForm = reactive({
  storageType: 'local',
  uploadDomain: '',
  s3Endpoint: '',
  s3Bucket: '',
  s3AccessKey: '',
  s3SecretKey: '',
  s3SecretKeySet: false,
  s3PublicURL: ''
})

const appleForm = reactive({
  keyId: '',
  issuerId: '',
  teamId: '',
  privateKey: '',
  privateKeySet: false
})

const appleStatus = reactive({
  configured: false,
  connected: false,
  message: ''
})

const testingConnection = ref(false)

function isValidDomain(domain: string): boolean {
  if (!domain) return false
  return domain.startsWith('http://') || domain.startsWith('https://')
}

async function loadSettings() {
  loading.value = true
  try {
    const res = await fetch('/admin/api/settings', {
      headers: { 'X-Auth-Token': token.value }
    })
    if (onAuthError(res)) return
    if (!res.ok) {
      toast.error(t.value.settings.loadFailed)
      return
    }

    const data = await res.json()
    if (data.ok && data.data) {
      form.primaryDomain = data.data.primary_domain || ''
      form.organization = data.data.organization || ''

      if (data.data.secondary_domains && Array.isArray(data.data.secondary_domains)) {
        secondaryDomainsText.value = data.data.secondary_domains.join('\n')
      }

      if (data.data.updated_at) {
        lastUpdated.value = new Date(data.data.updated_at).toLocaleString()
      }

      storageForm.storageType = data.data.storage_type || 'local'
      storageForm.uploadDomain = data.data.upload_domain || ''
      storageForm.s3Endpoint = data.data.s3_endpoint || ''
      storageForm.s3Bucket = data.data.s3_bucket || ''
      storageForm.s3PublicURL = data.data.s3_public_url || ''
      storageForm.s3SecretKeySet = data.data.s3_configured || false

      if (data.data.apple_configured !== undefined) {
        appleStatus.configured = data.data.apple_configured
        appleForm.keyId = data.data.apple_key_id || ''
        appleForm.teamId = data.data.apple_team_id || ''
        appleForm.privateKeySet = data.data.apple_configured
      }
    }
  } catch (e) {
    toast.error(t.value.settings.loadFailed + ': ' + e)
  } finally {
    loading.value = false
  }

  await checkAppleStatus()
}

function validateForm(): boolean {
  errors.primaryDomain = ''
  errors.organization = ''
  let isValid = true

  if (!form.primaryDomain) {
    errors.primaryDomain = t.value.settings.requiredField
    isValid = false
  } else if (!isValidDomain(form.primaryDomain)) {
    errors.primaryDomain = t.value.settings.invalidDomain
    isValid = false
  }

  if (!form.organization) {
    errors.organization = t.value.settings.requiredField
    isValid = false
  }

  return isValid
}

async function handleSave() {
  if (!validateForm()) return

  saving.value = true
  try {
    const secondaryDomains = secondaryDomainsText.value
      .split('\n')
      .map(line => line.trim())
      .filter(line => line.length > 0)

    const payload: Record<string, any> = {
      primary_domain: form.primaryDomain,
      secondary_domains: secondaryDomains,
      organization: form.organization,
      storage_type: storageForm.storageType,
      upload_domain: storageForm.uploadDomain,
    }

    if (storageForm.storageType === 's3') {
      payload.s3_endpoint = storageForm.s3Endpoint
      payload.s3_bucket = storageForm.s3Bucket
      payload.s3_public_url = storageForm.s3PublicURL
      if (storageForm.s3AccessKey) payload.s3_access_key = storageForm.s3AccessKey
      if (storageForm.s3SecretKey) payload.s3_secret_key = storageForm.s3SecretKey
    }

    if (appleForm.keyId) payload.apple_key_id = appleForm.keyId
    if (appleForm.issuerId) payload.apple_issuer_id = appleForm.issuerId
    if (appleForm.teamId) payload.apple_team_id = appleForm.teamId
    if (appleForm.privateKey) payload.apple_private_key = appleForm.privateKey

    const res = await fetch('/admin/api/settings', {
      method: 'PUT',
      headers: {
        'Content-Type': 'application/json',
        'X-Auth-Token': token.value
      },
      body: JSON.stringify(payload)
    })

    if (onAuthError(res)) return

    const data = await res.json()
    if (!res.ok || !data.ok) {
      toast.error(data.error?.message || t.value.settings.saveFailed)
      return
    }

    toast.success(t.value.settings.saveSuccess)

    if (data.data) {
      if (data.data.updated_at) {
        lastUpdated.value = new Date(data.data.updated_at).toLocaleString()
      }
      storageForm.storageType = data.data.storage_type || 'local'
      storageForm.uploadDomain = data.data.upload_domain || ''
      storageForm.s3Endpoint = data.data.s3_endpoint || ''
      storageForm.s3Bucket = data.data.s3_bucket || ''
      storageForm.s3PublicURL = data.data.s3_public_url || ''
      storageForm.s3SecretKeySet = data.data.s3_configured || false
      storageForm.s3SecretKey = ''
      storageForm.s3AccessKey = ''

      if (data.data.apple_configured !== undefined) {
        appleStatus.configured = data.data.apple_configured
        appleForm.keyId = data.data.apple_key_id || ''
        appleForm.teamId = data.data.apple_team_id || ''
        appleForm.privateKeySet = data.data.apple_configured
        appleForm.privateKey = ''
      }
    }

    await checkAppleStatus()
  } catch (e) {
    toast.error(t.value.settings.saveFailed + ': ' + e)
  } finally {
    saving.value = false
  }
}

async function checkAppleStatus() {
  try {
    const res = await fetch('/admin/api/apple/status', {
      headers: { 'X-Auth-Token': token.value }
    })
    if (!res.ok) return
    const data = await res.json()
    if (data.ok && data.data) {
      appleStatus.configured = data.data.configured
      appleStatus.connected = data.data.connected
      appleStatus.message = data.data.message
    }
  } catch {
    appleStatus.message = 'Failed to check status'
  }
}

function handlePrivateKeyFile(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = (e) => {
    appleForm.privateKey = e.target?.result as string
  }
  reader.readAsText(file)
}

async function testAppleConnection() {
  testingConnection.value = true
  try {
    const res = await fetch('/admin/api/apple/status', {
      headers: { 'X-Auth-Token': token.value }
    })
    if (onAuthError(res)) return
    const data = await res.json()
    if (data.ok && data.data) {
      appleStatus.configured = data.data.configured
      appleStatus.connected = data.data.connected
      appleStatus.message = data.data.message
      if (data.data.connected) {
        toast.success(t.value.apple.connectionSuccess)
      } else {
        toast.error(data.data.message || t.value.apple.connectionFailed)
      }
    }
  } catch (e) {
    toast.error(t.value.apple.connectionFailed + ': ' + e)
  } finally {
    testingConnection.value = false
  }
}

onMounted(() => {
  loadSettings()
})
</script>

<style scoped>
.loading-state {
  text-align: center;
  padding: 48px;
  color: var(--text-muted);
}

.card {
  background: var(--bg-card);
  border-radius: 8px;
  padding: 24px;
  border: 1px solid var(--border);
}

.section-card {
  margin-top: 20px;
}

.card-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.card-subtitle {
  font-size: 14px;
  color: var(--text-muted);
  margin-bottom: 20px;
}

.form-group {
  margin-bottom: 24px;
}

.label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-bright);
  margin-bottom: 8px;
}

.label.required::after {
  content: ' *';
  color: var(--red);
}

.input {
  width: 100%;
  padding: 10px 12px;
  background: var(--bg-deepest);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-bright);
  font-size: 14px;
}

.input:focus {
  outline: none;
  border-color: var(--blue);
}

.textarea {
  resize: vertical;
  font-family: inherit;
}

.help-text {
  margin-top: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.error-text {
  margin-top: 6px;
  font-size: 12px;
  color: var(--red);
}

.info-text {
  font-size: 13px;
  color: var(--text-muted);
}

/* Radio Group */
.radio-group {
  display: flex;
  gap: 12px;
}

.radio-label {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 16px;
  background: var(--bg-deepest);
  border: 2px solid var(--border);
  border-radius: 8px;
  cursor: pointer;
  transition: border-color 0.2s;
}

.radio-label:hover {
  border-color: var(--border-hover);
}

.radio-label.active {
  border-color: var(--blue);
  background: var(--bg-card);
}

.radio-label input[type="radio"] {
  display: none;
}

.radio-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-bright);
  margin-bottom: 4px;
}

.radio-desc {
  font-size: 12px;
  color: var(--text-muted);
}

/* Apple Status */
.apple-status {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  border-radius: 6px;
  margin-bottom: 24px;
}

.apple-status.connected {
  background: var(--green-deep);
  border: 1px solid var(--green-dark);
}

.apple-status.configured {
  background: var(--orange-deep);
  border: 1px solid var(--orange-dark);
}

.apple-status.not-configured {
  background: var(--bg-card);
  border: 1px solid var(--border);
}

.status-text {
  flex: 1;
  font-size: 14px;
  color: var(--text-bright);
}

.btn-test {
  flex-shrink: 0;
}

.file-input-wrapper {
  display: flex;
  align-items: center;
  gap: 12px;
}

.file-input {
  flex: 1;
  padding: 8px 12px;
  background: var(--bg-deepest);
  border: 1px solid var(--border);
  border-radius: 6px;
  color: var(--text-bright);
  font-size: 14px;
}

.file-input::file-selector-button {
  padding: 6px 12px;
  background: var(--border);
  border: none;
  border-radius: 4px;
  color: var(--text-bright);
  font-size: 12px;
  cursor: pointer;
  margin-right: 8px;
}

.file-status {
  font-size: 13px;
  padding: 4px 8px;
  border-radius: 4px;
}

.file-status.configured {
  background: var(--green-deep);
  color: var(--green);
}

.file-status.selected {
  background: var(--blue-deep);
  color: var(--blue);
}

/* Save Bar */
.save-bar {
  margin-top: 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.btn-save {
  padding: 12px 32px;
  font-size: 16px;
}
</style>
