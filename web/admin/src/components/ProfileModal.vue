<template>
  <Teleport to="body">
    <transition name="dialog">
      <div v-if="show" class="dialog-overlay" @click="$emit('close')">
        <div class="dialog-container" @click.stop>
          <div class="dialog-header">
            <h3 class="dialog-title">{{ t.profile.title }} · {{ app?.name || app?.id }}</h3>
          </div>
          <div class="dialog-body">
            <div v-if="loading" class="loading">{{ t.common.loading }}</div>
            <div v-else-if="!profile" class="empty-state">
              {{ t.profile.noProfile }}
            </div>
            <div v-else class="profile-content">
              <!-- Basic Info -->
              <div class="section">
                <h4 class="section-title">{{ t.profile.basicInfo }}</h4>
                <div class="info-grid">
                  <div class="info-item">
                    <span class="label">{{ t.profile.profileName }}</span>
                    <span class="value">{{ profile.name || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.profileType }}</span>
                    <span class="value">
                      <span class="badge" :class="profileTypeClass">{{ profileTypeLabel }}</span>
                    </span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.uuid }}</span>
                    <span class="value mono">{{ profile.uuid || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.platform }}</span>
                    <span class="value">{{ profile.platform || '-' }}</span>
                  </div>
                </div>
              </div>

              <!-- Team Info -->
              <div class="section">
                <h4 class="section-title">{{ t.profile.teamInfo }}</h4>
                <div class="info-grid">
                  <div class="info-item">
                    <span class="label">{{ t.profile.teamName }}</span>
                    <span class="value">{{ profile.team_name || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.teamId }}</span>
                    <span class="value mono">{{ profile.team_id || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.appIdName }}</span>
                    <span class="value">{{ profile.app_id_name || '-' }}</span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.bundleId }}</span>
                    <span class="value mono">{{ profile.bundle_id || '-' }}</span>
                  </div>
                </div>
              </div>

              <!-- Validity -->
              <div class="section">
                <h4 class="section-title">{{ t.profile.validity }}</h4>
                <div class="info-grid">
                  <div class="info-item">
                    <span class="label">{{ t.profile.creationDate }}</span>
                    <span class="value">{{ formatDate(profile.creation_date) }}</span>
                  </div>
                  <div class="info-item">
                    <span class="label">{{ t.profile.expirationDate }}</span>
                    <span class="value" :class="{ 'text-danger': isExpired, 'text-warning': isExpiringSoon }">
                      {{ formatDate(profile.expiration_date) }}
                      <span v-if="isExpired" class="status-tag expired">{{ t.profile.expired }}</span>
                      <span v-else-if="isExpiringSoon" class="status-tag expiring">{{ t.profile.expiringSoon }}</span>
                    </span>
                  </div>
                </div>
              </div>

              <!-- Devices -->
              <div v-if="profile.provisioned_devices && profile.provisioned_devices.length > 0" class="section">
                <h4 class="section-title">
                  {{ t.profile.devices }}
                  <span class="count-badge">{{ profile.device_count || profile.provisioned_devices.length }}</span>
                </h4>
                <div class="device-list">
                  <div v-for="(udid, idx) in profile.provisioned_devices" :key="idx" class="device-item">
                    <span class="device-index">{{ idx + 1 }}</span>
                    <span class="device-udid">{{ udid }}</span>
                  </div>
                </div>
              </div>
              <div v-else-if="profile.provisions_all_devices" class="section">
                <h4 class="section-title">{{ t.profile.devices }}</h4>
                <div class="enterprise-badge">{{ t.profile.enterpriseAllDevices }}</div>
              </div>

              <!-- Certificates -->
              <div v-if="profile.certificates && profile.certificates.length > 0" class="section">
                <h4 class="section-title">
                  {{ t.profile.certificates }}
                  <span class="count-badge">{{ profile.certificates.length }}</span>
                </h4>
                <div class="cert-list">
                  <div v-for="(cert, idx) in profile.certificates" :key="idx" class="cert-item">
                    <div class="cert-name">{{ cert.name }}</div>
                    <div class="cert-details">
                      <span class="cert-sha1">SHA1: {{ cert.sha1 }}</span>
                      <span class="cert-expiry">{{ t.profile.validUntil }}: {{ formatDate(cert.expiry_date) }}</span>
                    </div>
                  </div>
                </div>
              </div>

              <!-- Entitlements -->
              <div v-if="profile.entitlements && Object.keys(profile.entitlements).length > 0" class="section">
                <h4 class="section-title">{{ t.profile.entitlements }}</h4>
                <div class="entitlements-box">
                  <pre>{{ JSON.stringify(profile.entitlements, null, 2) }}</pre>
                </div>
              </div>
            </div>
          </div>
          <div class="dialog-footer">
            <button class="btn btn-secondary" @click="$emit('close')">{{ t.common.cancel }}</button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, computed } from 'vue'
import { useI18n } from '../i18n'

interface Certificate {
  name: string
  serial_number: string
  sha1: string
  creation_date: string
  expiry_date: string
}

interface Profile {
  uuid: string
  name: string
  team_id: string
  team_name: string
  app_id_name: string
  app_id_prefix: string
  bundle_id: string
  platform: string
  profile_type: string
  provisions_all_devices: boolean
  creation_date: string
  expiration_date: string
  certificates?: Certificate[]
  provisioned_devices?: string[]
  device_count?: number
  entitlements?: Record<string, unknown>
}

interface AppLite { id: string; name?: string; platform?: string }

const props = defineProps<{ show: boolean; app: AppLite | null; token: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const loading = ref(false)
const profile = ref<Profile | null>(null)

const profileTypeClass = computed(() => {
  if (!profile.value) return ''
  switch (profile.value.profile_type) {
    case 'development': return 'type-dev'
    case 'ad-hoc': return 'type-adhoc'
    case 'enterprise': return 'type-enterprise'
    case 'app-store': return 'type-appstore'
    default: return 'type-distribution'
  }
})

const profileTypeLabel = computed(() => {
  if (!profile.value) return ''
  switch (profile.value.profile_type) {
    case 'development': return t.value.profile.typeDevelopment
    case 'ad-hoc': return t.value.profile.typeAdHoc
    case 'enterprise': return t.value.profile.typeEnterprise
    case 'app-store': return t.value.profile.typeAppStore
    default: return t.value.profile.typeDistribution
  }
})

const isExpired = computed(() => {
  if (!profile.value?.expiration_date) return false
  return new Date(profile.value.expiration_date) < new Date()
})

const isExpiringSoon = computed(() => {
  if (!profile.value?.expiration_date || isExpired.value) return false
  const expDate = new Date(profile.value.expiration_date)
  const daysUntilExpiry = (expDate.getTime() - Date.now()) / (1000 * 60 * 60 * 24)
  return daysUntilExpiry <= 30
})

function formatDate(dateStr: string) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric'
  })
}

async function fetchProfile() {
  if (!props.app?.id) return
  loading.value = true
  profile.value = null
  try {
    const res = await fetch(`/admin/api/apps/${props.app.id}`, {
      headers: { 'X-Auth-Token': props.token }
    })
    const j = await res.json()
    if (res.ok && j.ok && j.data?.releases?.length > 0) {
      const latestRelease = j.data.releases[0]
      if (latestRelease.provisioning_profile) {
        profile.value = latestRelease.provisioning_profile
      }
    }
  } catch (e) {
    console.error('Failed to fetch profile:', e)
  } finally {
    loading.value = false
  }
}

watch(() => props.show, (v) => {
  if (v && props.app?.platform === 'ios') {
    fetchProfile()
  }
})

onMounted(() => {
  if (props.show && props.app?.platform === 'ios') {
    fetchProfile()
  }
})
</script>

<style scoped>
.dialog-container { max-width: 800px; max-height: 85vh; display: flex; flex-direction: column; }
.dialog-body { padding: 20px 24px; overflow-y: auto; flex: 1; }
.dialog-footer { flex-shrink: 0; }
.loading, .empty-state { color: var(--text-muted); text-align: center; padding: 40px 0; }

.profile-content { display: flex; flex-direction: column; gap: 20px; }

.section { background: var(--bg-deepest); border: 1px solid var(--divider); border-radius: 12px; padding: 16px; }
.section-title { margin: 0 0 12px; font-size: 14px; font-weight: 600; color: var(--text-body); display: flex; align-items: center; gap: 8px; }
.count-badge { background: var(--blue); color: #fff; font-size: 11px; padding: 2px 8px; border-radius: 999px; font-weight: 500; }

.info-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 12px; }
@media (max-width: 600px) { .info-grid { grid-template-columns: 1fr; } }

.info-item { display: flex; flex-direction: column; gap: 4px; }
.info-item .label { font-size: 12px; color: var(--text-dim); }
.info-item .value { font-size: 14px; color: var(--text-body); word-break: break-all; }
.info-item .value.mono { font-family: monospace; font-size: 13px; }

.badge { display: inline-block; padding: 4px 10px; border-radius: 999px; font-size: 12px; font-weight: 500; }
.type-dev { background: var(--blue-deep); color: var(--blue-light); border: 1px solid var(--blue); }
.type-adhoc { background: var(--orange-deep); color: var(--orange); border: 1px solid var(--orange-dark); }
.type-enterprise { background: var(--green-deep); color: var(--green); border: 1px solid var(--green-dark); }
.type-appstore { background: var(--purple-deep); color: var(--purple-light); border: 1px solid var(--purple); }
.type-distribution { background: var(--border); color: var(--text-muted); border: 1px solid var(--text-dim); }

.text-danger { color: var(--red); }
.text-warning { color: var(--orange); }

.status-tag { font-size: 11px; padding: 2px 6px; border-radius: 4px; margin-left: 8px; }
.status-tag.expired { background: var(--red-deep); color: var(--red-text); }
.status-tag.expiring { background: var(--orange-deep); color: var(--orange); }

.device-list { display: flex; flex-direction: column; gap: 6px; max-height: 200px; overflow-y: auto; }
.device-item { display: flex; align-items: center; gap: 10px; background: var(--bg-card); padding: 8px 12px; border-radius: 8px; }
.device-index { background: var(--border); color: var(--text-muted); font-size: 11px; padding: 2px 8px; border-radius: 999px; }
.device-udid { font-family: monospace; font-size: 12px; color: var(--text-secondary); }

.enterprise-badge { background: var(--green-deep); color: var(--green); padding: 12px 16px; border-radius: 8px; text-align: center; }

.cert-list { display: flex; flex-direction: column; gap: 10px; }
.cert-item { background: var(--bg-card); padding: 12px; border-radius: 8px; }
.cert-name { font-size: 14px; color: var(--text-body); font-weight: 500; margin-bottom: 6px; }
.cert-details { display: flex; flex-wrap: wrap; gap: 16px; }
.cert-sha1, .cert-expiry { font-size: 12px; color: var(--text-muted); font-family: monospace; }

.entitlements-box { background: var(--bg-card); border-radius: 8px; padding: 12px; overflow-x: auto; }
.entitlements-box pre { margin: 0; font-size: 12px; color: var(--text-muted); white-space: pre-wrap; word-break: break-all; }

.btn { padding: 10px 20px; border-radius: 8px; font-size: 14px; border: none; cursor: pointer; }

.dialog-enter-active, .dialog-leave-active { transition: opacity 0.2s ease; }
.dialog-enter-from, .dialog-leave-to { opacity: 0; }
</style>
