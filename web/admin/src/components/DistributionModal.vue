<template>
  <Teleport to="body">
    <transition name="dialog">
      <div v-if="show" class="dialog-overlay" @click="$emit('close')">
        <div class="dialog-container" @click.stop>
          <div class="dialog-header">
            <h3 class="dialog-title">{{ t.apps.viewDistribution }} · {{ product?.name || product?.id }}</h3>
          </div>
          <div class="dialog-body">
            <div v-if="loading" class="loading">{{ t.common.loading }}</div>
            <div v-else>
              <div class="desc">{{ t.apps.distributionDesc }}</div>
              <div class="link-list">
                <div v-for="(d, idx) in domains" :key="idx" class="link-row">
                  <div class="domain-label">
                    <span class="badge" :class="idx === 0 ? 'primary' : 'secondary'">
                      {{ idx === 0 ? t.apps.primaryDomain : t.apps.secondaryDomain }}
                    </span>
                  </div>
                  <div class="url-text">{{ buildUrl(d) }}</div>
                  <div class="row-actions">
                    <button class="btn-sm" @click="copy(buildUrl(d))">{{ copiedUrl === buildUrl(d) ? t.apps.copied : t.apps.copyUrl }}</button>
                    <button class="btn-sm" @click="showQR(buildUrl(d))">{{ t.apps.qrCode }}</button>
                  </div>
                </div>
                <div v-if="qrUrl" class="qr-area">
                  <div class="qr-title">{{ t.apps.qrFor }}: <span class="qr-url">{{ qrUrl }}</span></div>
                  <canvas ref="qrCanvas"></canvas>
                  <div class="qr-hint">{{ t.apps.qrHint }}</div>
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
import { ref, watch, onMounted, nextTick, computed } from 'vue'
import { useI18n } from '../i18n'
import QRCode from 'qrcode'

interface ProductLite { id: string; name?: string; slug?: string }

const props = defineProps<{ show: boolean; product: ProductLite | null; token: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const settings = ref<{ primary_domain: string; secondary_domains: string[] } | null>(null)
const loading = ref(false)
const qrUrl = ref('')

const copiedUrl = ref('')

const domains = computed(() => {
  if (!settings.value) return [] as string[]
  const arr = [settings.value.primary_domain || '']
  const secs = Array.isArray(settings.value.secondary_domains) ? settings.value.secondary_domains : []
  return arr.concat(secs).filter(Boolean)
})

function buildUrl(domain: string) {
  const base = domain.replace(/\/$/, '')
  const key = props.product?.slug || props.product?.id || ''
  return `${base}/products/${key}`
}

const qrCanvas = ref<HTMLCanvasElement | null>(null)

async function fetchSettings() {
  loading.value = true
  try {
    const res = await fetch('/admin/api/settings', { headers: { 'X-Auth-Token': props.token } })
    const j = await res.json()
    if (res.ok && j.ok) {
      settings.value = j.data
    }
  } catch (e) {
    // ignore
  } finally {
    loading.value = false
  }
}

async function copy(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    copiedUrl.value = text
    setTimeout(() => { if (copiedUrl.value === text) copiedUrl.value = '' }, 1500)
  } catch {}
}

async function showQR(url: string) {
  qrUrl.value = url
  await nextTick()
  if (qrCanvas.value) {
    QRCode.toCanvas(qrCanvas.value, url, { width: 180, margin: 1 })
  }
}

watch(() => props.show, (v) => {
  if (v) {
    if (!settings.value) fetchSettings()
    qrUrl.value = ''
  }
})

onMounted(() => { if (props.show) fetchSettings() })
</script>

<style scoped>
.dialog-container { max-width: 800px; }
.dialog-body { padding: 20px 24px; }
.loading { color: var(--text-secondary); }
.desc { color: var(--text-muted); margin-bottom: 12px; font-size: 13px; }
.link-list { display: flex; flex-direction: column; gap: 10px; }
.link-row { display: grid; grid-template-columns: 120px 1fr auto; gap: 12px; align-items: center; background: var(--bg-deepest); border: 1px solid var(--divider); padding: 10px 12px; border-radius: 12px; }
.domain-label .badge { padding: 4px 8px; border-radius: 999px; font-size: 12px; }
.badge.primary { background: var(--green-deep); color: var(--green); border: 1px solid var(--green-dark); }
.badge.secondary { background: var(--blue-deep); color: var(--blue-light); border: 1px solid var(--blue); }
.url-text { font-family: monospace; font-size: 13px; color: var(--text-body); overflow: auto; }
.row-actions .btn-sm { margin-left: 8px; }
.qr-area { margin-top: 16px; display: flex; flex-direction: column; align-items: flex-start; gap: 8px; }
.qr-title { color: var(--text-body); font-size: 13px; }
.qr-url { font-family: monospace; color: var(--blue-pale); }
.qr-hint { color: var(--text-muted); font-size: 12px; }
.btn { display: inline-flex; align-items: center; gap: 6px; padding: 10px 20px; border-radius: 8px; font-size: 14px; font-weight: 500; border: none; cursor: pointer; transition: all 0.15s; }
</style>
