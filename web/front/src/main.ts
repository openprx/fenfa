import { computed, createApp, nextTick, onMounted, reactive, ref } from 'vue'
import QRCode from 'qrcode'
import { useI18n } from './i18n'
import './style.css'

type Platform = 'ios' | 'android' | 'macos' | 'windows' | 'linux'

interface ReleaseActions {
  download?: string
  release_page?: string
  ios_manifest?: string
  ios_install?: string
}

interface ReleaseItem {
  id: string
  version: string
  build?: number
  created_at: string
  download_count?: number
  channel?: string
  min_os?: string
  changelog?: string
  actions: ReleaseActions
}

interface VariantItem {
  id: string
  platform: Platform
  identifier: string
  display_name?: string
  arch?: string
  installer_type?: string
  min_os?: string
  latest_release?: ReleaseItem | null
  releases: ReleaseItem[]
}

interface ProductData {
  product: {
    id: string
    slug?: string
    name: string
    icon_url?: string
  }
  variants: VariantItem[]
}

function detectClientPlatform(): Platform | '' {
  const ua = navigator.userAgent.toLowerCase()
  if (/iphone|ipad|ipod/.test(ua)) return 'ios'
  if (ua.includes('android')) return 'android'
  if (ua.includes('mac os x') || ua.includes('macintosh')) return 'macos'
  if (ua.includes('windows')) return 'windows'
  if (ua.includes('linux')) return 'linux'
  return ''
}

function platformLabel(platform: string): string {
  const labels: Record<string, string> = {
    ios: 'iOS',
    android: 'Android',
    macos: 'macOS',
    windows: 'Windows',
    linux: 'Linux'
  }
  return labels[platform] || platform
}

const App = {
  setup() {
    const { t, getLanguage } = useI18n()
    const productData = ref<ProductData | null>(null)
    const loading = ref(true)
    const error = ref('')
    const installing = ref(false)
    const udidBound = reactive<Record<string, boolean>>({})
    const selectedReleaseByVariant = reactive<Record<string, ReleaseItem | null>>({})
    const clientPlatform = detectClientPlatform()
    const pathParts = location.pathname.split('/').filter(Boolean)
    const routeType = 'product'
    const resourceID = pathParts[1] || ''

    const sortedVariants = computed(() => {
      const items = [...(productData.value?.variants || [])]
      return items.sort((left, right) => {
        const leftScore = left.platform === clientPlatform ? 1 : 0
        const rightScore = right.platform === clientPlatform ? 1 : 0
        if (leftScore !== rightScore) return rightScore - leftScore
        return left.platform.localeCompare(right.platform)
      })
    })

    const recommendedVariant = computed(() => {
      return sortedVariants.value.find((variant) => variant.platform === clientPlatform) || sortedVariants.value[0] || null
    })

    function getSelectedRelease(variant: VariantItem): ReleaseItem | null {
      return selectedReleaseByVariant[variant.id] || variant.latest_release || variant.releases[0] || null
    }

    function selectRelease(variantID: string, release: ReleaseItem) {
      selectedReleaseByVariant[variantID] = release
    }

    function isIOSVariant(variant: VariantItem): boolean {
      return variant.platform === 'ios'
    }

    function isRecommended(variant: VariantItem): boolean {
      return !!clientPlatform && variant.platform === clientPlatform
    }

    function getVariantAction(variant: VariantItem, release: ReleaseItem | null) {
      if (!release) return { type: 'disabled', href: '#', label: t.value.actions.unavailable }
      if (variant.platform === 'ios') {
        if (udidBound[variant.id]) {
          return {
            type: 'install',
            href: release.actions?.ios_install || '',
            label: installing.value ? t.value.actions.installing : t.value.actions.install
          }
        }
        return {
          type: 'bind',
          href: `/udid/profile.mobileconfig?variant=${encodeURIComponent(variant.id)}`,
          label: t.value.actions.bindDevice
        }
      }
      return {
        type: 'download',
        href: release.actions?.download || '',
        label: t.value.actions.download
      }
    }

    function getCookie(name: string): string | null {
      const match = document.cookie.match(new RegExp('(^| )' + name + '=([^;]+)'))
      return match ? match[2] : null
    }

    async function fetchUDIDStatuses() {
      const variants = productData.value?.variants || []
      const iosVariants = variants.filter((variant) => variant.platform === 'ios')
      if (iosVariants.length === 0) return

      const urlParams = new URLSearchParams(location.search)
      if (urlParams.get('udid_bound') === '1') {
        const boundVariantID = urlParams.get('variant')
        if (boundVariantID) {
          udidBound[boundVariantID] = true
        } else {
          iosVariants.forEach((variant) => {
            udidBound[variant.id] = true
          })
        }
        urlParams.delete('udid_bound')
        urlParams.delete('variant')
        const nextURL = urlParams.toString() ? `${location.pathname}?${urlParams}` : location.pathname
        history.replaceState(null, '', nextURL)
        return
      }

      const nonce = getCookie('udid_nonce')
      await Promise.all(iosVariants.map(async (variant) => {
        try {
          const params = new URLSearchParams()
          if (nonce) params.set('nonce', nonce)
          params.set('variant', variant.id)
          const res = await fetch(`/udid/status?${params}`)
          const payload = await res.json()
          udidBound[variant.id] = !!payload?.data?.bound
        } catch {
          udidBound[variant.id] = false
        }
      }))
    }

    async function fetchData() {
      const selectedReleaseID = new URLSearchParams(location.search).get('r')
      const endpoint = `/api/products/${resourceID}`
      const res = await fetch(endpoint + (selectedReleaseID ? `?r=${encodeURIComponent(selectedReleaseID)}` : ''))
      const payload = await res.json()

      if (!res.ok || !payload.ok) {
        loading.value = false
        error.value = payload.error?.message || t.value.error.loadFailed
        return
      }

      productData.value = payload.data as ProductData

      for (const variant of productData.value.variants) {
        const preferred = variant.releases.find((release) => release.id === selectedReleaseID) || variant.latest_release || variant.releases[0] || null
        selectedReleaseByVariant[variant.id] = preferred
      }

      loading.value = false
      await nextTick()
      await renderQRCode()
      await fetchUDIDStatuses()
    }

    async function renderQRCode() {
      try {
        const canvas = document.getElementById('qrCanvas') as HTMLCanvasElement | null
        if (!canvas) return
        await QRCode.toCanvas(canvas, location.href, { width: 148, margin: 1 })
      } catch (err) {
        console.warn('qr failed', err)
      }
    }

    function sendBeaconJSON(url: string, payload: any) {
      try {
        if (navigator.sendBeacon) {
          const blob = new Blob([JSON.stringify(payload)], { type: 'application/json' })
          navigator.sendBeacon(url, blob)
          return
        }
      } catch {}
      fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      }).catch(() => {})
    }

    async function logVisit() {
      const variant = recommendedVariant.value
      const release = variant ? getSelectedRelease(variant) : null
      if (!variant || !release) return

      try {
        const mod: any = await import('ua-parser-js')
        const UAParser = mod.UAParser || mod.default
        const parser = new UAParser(navigator.userAgent)
        sendBeaconJSON('/events', {
          type: 'visit',
          variant_id: variant.id,
          release_id: release.id,
          extra: {
            product_id: productData.value?.product.id || '',
            variant_id: variant.id,
            ua: parser.getResult(),
            client_platform: clientPlatform,
            path: location.pathname
          }
        })
      } catch {}
    }

    function trackAction(variant: VariantItem, release: ReleaseItem | null, href: string) {
      if (!release || !href) return
      sendBeaconJSON('/events', {
        type: 'click',
        variant_id: variant.id,
        release_id: release.id,
        extra: {
          product_id: productData.value?.product.id || '',
          variant_id: variant.id,
          href,
          path: location.pathname
        }
      })
    }

    function handlePrimaryAction(variant: VariantItem) {
      const release = getSelectedRelease(variant)
      const action = getVariantAction(variant, release)
      if (!release || !action.href) return
      if (action.type === 'install') {
        installing.value = true
        trackAction(variant, release, action.href)
        location.href = action.href
        setTimeout(() => { installing.value = false }, 10000)
        return
      }
      if (action.type === 'bind') {
        trackAction(variant, release, action.href)
        location.href = action.href
        return
      }
      trackAction(variant, release, action.href)
      location.href = action.href
    }

    function formatTime(dateStr: string) {
      const date = new Date(dateStr)
      const now = new Date()
      const diff = now.getTime() - date.getTime()
      const minutes = Math.floor(diff / (1000 * 60))
      const hours = Math.floor(diff / (1000 * 60 * 60))
      const days = Math.floor(diff / (1000 * 60 * 60 * 24))

      if (minutes < 1) return t.value.time.justNow
      if (minutes < 60) return `${minutes} ${t.value.time.minutesAgo}`
      if (hours < 24) return `${hours} ${t.value.time.hoursAgo}`
      if (days < 7) return `${days} ${t.value.time.daysAgo}`
      if (days < 30) return `${Math.floor(days / 7)} ${t.value.time.weeksAgo}`
      if (days < 365) return `${Math.floor(days / 30)} ${t.value.time.monthsAgo}`
      return date.toLocaleDateString()
    }

    onMounted(async () => {
      await fetchData()
      await logVisit()
    })

    return {
      clientPlatform,
      error,
      formatTime,
      getLanguage,
      getSelectedRelease,
      getVariantAction,
      handlePrimaryAction,
      isIOSVariant,
      isRecommended,
      loading,
      platformLabel,
      productData,
      recommendedVariant,
      routeType,
      selectRelease,
      sortedVariants,
      t,
      udidBound
    }
  },
  template: /* html */ `
  <div class="page-shell">
    <div v-if="loading" class="state-panel">
      <div class="spinner"></div>
      <p>{{ t.loading }}</p>
    </div>

    <div v-else-if="error" class="state-panel state-error">
      <h1>{{ t.error.title }}</h1>
      <p>{{ error }}</p>
    </div>

    <div v-else-if="productData" class="product-page">
      <section class="hero-card">
        <div class="hero-copy">
          <div class="hero-kicker">{{ routeType === 'product' ? t.hero.productCenter : t.hero.legacyPage }}</div>
          <h1 class="hero-title">{{ productData.product.name }}</h1>
          <p class="hero-subtitle">{{ t.hero.subtitle }}</p>
          <div class="hero-tags">
            <span v-for="variant in sortedVariants" :key="variant.id" class="hero-tag">
              {{ platformLabel(variant.platform) }}
            </span>
          </div>
          <div v-if="recommendedVariant" class="hero-callout">
            <span class="callout-label">{{ t.hero.recommended }}</span>
            <strong>{{ platformLabel(recommendedVariant.platform) }}</strong>
            <span v-if="recommendedVariant.latest_release">
              · {{ recommendedVariant.latest_release.version }}
            </span>
          </div>
        </div>

        <div class="hero-side">
          <div class="product-icon">
            <img v-if="productData.product.icon_url" :src="productData.product.icon_url" alt="Product Icon" class="product-icon-image" />
            <div v-else class="product-icon-fallback">{{ productData.product.name.slice(0, 1).toUpperCase() }}</div>
          </div>
          <div class="qr-card">
            <p class="qr-title">{{ t.actions.scan }}</p>
            <canvas id="qrCanvas" width="148" height="148"></canvas>
          </div>
        </div>
      </section>

      <section class="variant-grid">
        <article v-for="variant in sortedVariants" :key="variant.id" class="variant-card" :class="{ recommended: isRecommended(variant) }">
          <header class="variant-header">
            <div>
              <div class="variant-title-row">
                <h2 class="variant-title">{{ platformLabel(variant.platform) }}</h2>
                <span v-if="isRecommended(variant)" class="recommended-badge">{{ t.hero.recommended }}</span>
              </div>
              <p class="variant-name">{{ variant.display_name || productData.product.name }}</p>
            </div>
            <div class="variant-meta-badges">
              <span v-if="variant.arch" class="meta-pill">{{ variant.arch }}</span>
              <span v-if="variant.installer_type" class="meta-pill">{{ variant.installer_type }}</span>
            </div>
          </header>

          <div class="variant-details">
            <div class="detail-item">
              <span class="detail-label">{{ t.variant.identifier }}</span>
              <span class="detail-value code-text">{{ variant.identifier }}</span>
            </div>
            <div v-if="getSelectedRelease(variant)?.version" class="detail-item">
              <span class="detail-label">{{ t.variant.version }}</span>
              <span class="detail-value">
                {{ getSelectedRelease(variant)?.version }}
                <template v-if="getSelectedRelease(variant)?.build">
                  ({{ t.variant.build }} {{ getSelectedRelease(variant)?.build }})
                </template>
              </span>
            </div>
            <div v-if="getSelectedRelease(variant)?.min_os || variant.min_os" class="detail-item">
              <span class="detail-label">{{ t.variant.minOS }}</span>
              <span class="detail-value">{{ getSelectedRelease(variant)?.min_os || variant.min_os }}</span>
            </div>
            <div v-if="getSelectedRelease(variant)?.created_at" class="detail-item">
              <span class="detail-label">{{ t.variant.updatedAt }}</span>
              <span class="detail-value">{{ formatTime(getSelectedRelease(variant)?.created_at || '') }}</span>
            </div>
          </div>

          <div class="variant-actions">
            <button
              class="primary-action"
              :class="getVariantAction(variant, getSelectedRelease(variant)).type"
              :disabled="getVariantAction(variant, getSelectedRelease(variant)).type === 'disabled'"
              @click="handlePrimaryAction(variant)"
            >
              {{ getVariantAction(variant, getSelectedRelease(variant)).label }}
            </button>
            <p v-if="isIOSVariant(variant) && !udidBound[variant.id]" class="action-hint">
              {{ t.variant.iosHint }}
            </p>
          </div>

          <div v-if="getSelectedRelease(variant)?.changelog" class="changelog-box">
            <div class="changelog-title">{{ getLanguage() === 'zh' ? '更新说明' : 'Changelog' }}</div>
            <p>{{ getSelectedRelease(variant)?.changelog }}</p>
          </div>

          <div v-if="variant.releases.length > 0" class="release-list">
            <div class="release-list-title">{{ t.variant.allVersions }}</div>
            <button
              v-for="release in variant.releases"
              :key="release.id"
              class="release-item"
              :class="{ active: getSelectedRelease(variant)?.id === release.id }"
              @click="selectRelease(variant.id, release)"
            >
              <span class="release-version">{{ release.version }}</span>
              <span class="release-date">{{ formatTime(release.created_at) }}</span>
            </button>
          </div>
        </article>
      </section>
    </div>
  </div>`
}

createApp(App).mount('#app')
