<template>
  <Teleport to="body">
    <transition name="dialog">
      <div v-if="isVisible" class="dialog-overlay" @click="handleCancel">
        <div class="dialog-container" @click.stop>
          <div class="dialog-header">
            <h3 class="dialog-title">{{ title }}</h3>
          </div>
          <div class="dialog-body">
            <p class="dialog-message">{{ message }}</p>
          </div>
          <div class="dialog-footer">
            <button class="btn btn-secondary" @click="handleCancel">
              {{ cancelText }}
            </button>
            <button :class="['btn', confirmButtonClass]" @click="handleConfirm">
              {{ confirmText }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref } from 'vue'

interface ConfirmOptions {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  type?: 'danger' | 'primary' | 'warning'
}

const isVisible = ref(false)
const title = ref('')
const message = ref('')
const confirmText = ref('OK')
const cancelText = ref('Cancel')
const confirmButtonClass = ref('btn-primary')
let resolvePromise: ((value: boolean) => void) | null = null

function show(options: ConfirmOptions): Promise<boolean> {
  title.value = options.title
  message.value = options.message
  confirmText.value = options.confirmText || 'OK'
  cancelText.value = options.cancelText || 'Cancel'

  if (options.type === 'danger') {
    confirmButtonClass.value = 'btn-danger'
  } else if (options.type === 'warning') {
    confirmButtonClass.value = 'btn-warning'
  } else {
    confirmButtonClass.value = 'btn-primary'
  }

  isVisible.value = true

  return new Promise((resolve) => {
    resolvePromise = resolve
  })
}

function handleConfirm() {
  isVisible.value = false
  if (resolvePromise) {
    resolvePromise(true)
    resolvePromise = null
  }
}

function handleCancel() {
  isVisible.value = false
  if (resolvePromise) {
    resolvePromise(false)
    resolvePromise = null
  }
}

defineExpose({
  show
})
</script>

<style scoped>
.dialog-container {
  max-width: 480px;
}

.dialog-message {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-secondary);
}

.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 10px 20px;
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  border: none;
  cursor: pointer;
  transition: all 0.15s;
}

.btn-danger {
  background: var(--red-dark);
  color: #fff;
}

.btn-danger:hover {
  background: var(--red);
}

.btn-warning {
  background: var(--orange-dark);
  color: #fff;
}

.btn-warning:hover {
  background: var(--orange);
}

/* Animations */
.dialog-enter-active,
.dialog-leave-active {
  transition: opacity 0.3s ease;
}

.dialog-enter-active .dialog-container,
.dialog-leave-active .dialog-container {
  transition: transform 0.3s ease, opacity 0.3s ease;
}

.dialog-enter-from,
.dialog-leave-to {
  opacity: 0;
}

.dialog-enter-from .dialog-container {
  transform: scale(0.9);
  opacity: 0;
}

.dialog-leave-to .dialog-container {
  transform: scale(0.95);
  opacity: 0;
}
</style>
