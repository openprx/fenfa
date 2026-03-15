<template>
  <div v-if="totalPages > 1" class="pagination">
    <span class="pagination-info">
      {{ t.pagination.total.replace('{total}', String(total)) }} · {{ t.pagination.page.replace('{page}', String(page)).replace('{totalPages}', String(totalPages)) }}
    </span>

    <div class="pagination-buttons">
      <button
        class="pagination-btn"
        :disabled="page <= 1"
        @click="$emit('update:page', page - 1)"
      >
        {{ t.pagination.prev }}
      </button>

      <template v-for="p in visiblePages" :key="p">
        <span v-if="p === '...'" class="pagination-ellipsis">...</span>
        <button
          v-else
          :class="['pagination-btn', 'pagination-num', { active: p === page }]"
          @click="$emit('update:page', p)"
        >
          {{ p }}
        </button>
      </template>

      <button
        class="pagination-btn"
        :disabled="page >= totalPages"
        @click="$emit('update:page', page + 1)"
      >
        {{ t.pagination.next }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  total: number
  page: number
  pageSize?: number
}>(), {
  pageSize: 50
})

defineEmits<{
  'update:page': [page: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const visiblePages = computed(() => {
  const tp = totalPages.value
  if (tp <= 7) {
    return Array.from({ length: tp }, (_, i) => i + 1)
  }

  const pages: (number | string)[] = []
  const current = props.page

  pages.push(1)

  if (current > 4) {
    pages.push('...')
  }

  const start = Math.max(2, current - 2)
  const end = Math.min(tp - 1, current + 2)
  for (let i = start; i <= end; i++) {
    pages.push(i)
  }

  if (current < tp - 3) {
    pages.push('...')
  }

  if (tp > 1) {
    pages.push(tp)
  }

  return pages
})
</script>

<style scoped>
.pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 0;
  gap: 16px;
  flex-wrap: wrap;
}

.pagination-info {
  font-size: 13px;
  color: var(--text-muted);
  white-space: nowrap;
}

.pagination-buttons {
  display: flex;
  align-items: center;
  gap: 4px;
}

.pagination-btn {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg-card);
  color: var(--text-bright);
  font-size: 13px;
  cursor: pointer;
  transition: all 0.2s;
  white-space: nowrap;
}

.pagination-btn:hover:not(:disabled):not(.active) {
  border-color: var(--blue);
  color: var(--blue);
}

.pagination-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.pagination-btn.pagination-num {
  min-width: 34px;
  text-align: center;
  padding: 6px 8px;
}

.pagination-btn.active {
  background: var(--blue);
  border-color: var(--blue);
  color: #fff;
  cursor: default;
}

.pagination-ellipsis {
  padding: 6px 4px;
  color: var(--text-muted);
  font-size: 13px;
  user-select: none;
}
</style>
