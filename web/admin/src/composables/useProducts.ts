import { ref } from 'vue'
import { useAuth } from './useAuth'

const products = ref<any[]>([])
const loadingProducts = ref(false)
const productQuery = ref('')

export function useProducts() {
  const { token, handleAuthError } = useAuth()

  async function fetchProducts() {
    if (!token.value) return
    loadingProducts.value = true
    try {
      const p = new URLSearchParams()
      if (productQuery.value) p.set('q', productQuery.value)
      const res = await fetch('/admin/api/products' + (p.toString() ? ('?' + p.toString()) : ''), {
        headers: { 'X-Auth-Token': token.value }
      })
      if (handleAuthError(res)) return
      const j = await res.json()
      products.value = j?.data?.items || []
    } catch (e) {
      console.error('Failed to fetch products:', e)
    } finally {
      loadingProducts.value = false
    }
  }

  function clearProducts() {
    products.value = []
    productQuery.value = ''
  }

  return {
    products,
    loadingProducts,
    productQuery,
    fetchProducts,
    clearProducts,
  }
}
