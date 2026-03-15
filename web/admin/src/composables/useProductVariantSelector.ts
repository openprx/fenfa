import { ref, computed, watch, type Ref } from 'vue'

interface ProductSummary {
  id: string
  name: string
  variants?: any[]
}

export function useProductVariantSelector(
  products: Ref<ProductSummary[]>,
  token: Ref<string>,
  onAuthError: (res: Response) => boolean
) {
  const selectedProductId = ref('')
  const selectedVariantId = ref('')
  const productDetails = ref<Record<string, any>>({})
  const loadingVariants = ref(false)

  const selectedProduct = computed(() => {
    if (!selectedProductId.value) return null
    return productDetails.value[selectedProductId.value] || products.value.find(p => p.id === selectedProductId.value) || null
  })

  const variantOptions = computed(() => selectedProduct.value?.variants || [])

  const selectedVariant = computed(() =>
    variantOptions.value.find((v: any) => v.id === selectedVariantId.value) || null
  )

  function autoSelectVariant() {
    const variants = variantOptions.value
    if (variants.length === 1) {
      selectedVariantId.value = variants[0].id
    }
  }

  async function fetchProductDetail(productId: string) {
    loadingVariants.value = true
    try {
      const res = await fetch(`/admin/api/products/${productId}`, {
        headers: { 'X-Auth-Token': token.value }
      })
      if (onAuthError(res)) return
      const payload = await res.json()
      if (res.ok && payload.ok && payload.data) {
        productDetails.value[productId] = payload.data
        autoSelectVariant()
      }
    } finally {
      loadingVariants.value = false
    }
  }

  let skipWatch = false

  watch(selectedProductId, (value) => {
    selectedVariantId.value = ''
    if (!value) return
    if (skipWatch) {
      skipWatch = false
      return
    }
    if (productDetails.value[value]?.variants) {
      autoSelectVariant()
      return
    }
    fetchProductDetail(value)
  })

  async function initFromVariantId(variantId: string) {
    try {
      const res = await fetch(`/admin/api/variants/${variantId}/stats`, {
        headers: { 'X-Auth-Token': token.value }
      })
      if (!res.ok) return
      const payload = await res.json()
      if (payload.ok && payload.data?.variant?.product_id) {
        const productId = payload.data.variant.product_id
        skipWatch = true
        selectedProductId.value = productId
        await fetchProductDetail(productId)
        selectedVariantId.value = variantId
        return payload.data
      }
    } catch {}
    return null
  }

  return {
    selectedProductId,
    selectedVariantId,
    productDetails,
    loadingVariants,
    selectedProduct,
    variantOptions,
    selectedVariant,
    initFromVariantId,
  }
}
