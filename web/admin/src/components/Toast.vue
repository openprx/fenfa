<template>
  <Teleport to="body">
    <div class="toast-container">
      <transition-group name="toast">
        <div
          v-for="toast in toasts"
          :key="toast.id"
          :class="['toast', `toast-${toast.type}`]"
        >
          <div class="toast-icon">
            <Icon v-if="toast.type === 'success'" name="check" :size="14" />
            <Icon v-else-if="toast.type === 'error'" name="x" :size="14" />
            <Icon v-else-if="toast.type === 'warning'" name="alert-triangle" :size="14" />
            <Icon v-else name="info" :size="14" />
          </div>
          <div class="toast-content">
            <div class="toast-title" v-if="toast.title">{{ toast.title }}</div>
            <div class="toast-message">{{ toast.message }}</div>
          </div>
          <button class="toast-close" @click="removeToast(toast.id)">×</button>
        </div>
      </transition-group>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import Icon from './Icon.vue'

export interface Toast {
  id: number
  type: 'success' | 'error' | 'warning' | 'info'
  title?: string
  message: string
  duration?: number
}

const toasts = ref<Toast[]>([])
let nextId = 1

function addToast(toast: Omit<Toast, 'id'>) {
  const id = nextId++
  const newToast: Toast = { id, ...toast }
  toasts.value.push(newToast)

  const duration = toast.duration || 3000
  if (duration > 0) {
    setTimeout(() => {
      removeToast(id)
    }, duration)
  }

  return id
}

function removeToast(id: number) {
  const index = toasts.value.findIndex(t => t.id === id)
  if (index > -1) {
    toasts.value.splice(index, 1)
  }
}

defineExpose({
  addToast,
  removeToast,
  success: (message: string, title?: string) => addToast({ type: 'success', message, title }),
  error: (message: string, title?: string) => addToast({ type: 'error', message, title }),
  warning: (message: string, title?: string) => addToast({ type: 'warning', message, title }),
  info: (message: string, title?: string) => addToast({ type: 'info', message, title })
})
</script>

<style scoped>
.toast-container {
  position: fixed;
  top: 20px;
  right: 20px;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 12px;
  pointer-events: none;
}

.toast {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  min-width: 320px;
  max-width: 420px;
  padding: 16px;
  background: var(--bg-card);
  border-radius: 12px;
  box-shadow: var(--toast-shadow);
  pointer-events: auto;
  backdrop-filter: blur(10px);
}

.toast-icon {
  flex-shrink: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  font-size: 14px;
  font-weight: bold;
}

.toast-success .toast-icon {
  background: var(--green-deep);
  color: var(--green);
}

.toast-error .toast-icon {
  background: var(--red-deep);
  color: var(--red);
}

.toast-warning .toast-icon {
  background: var(--orange-deep);
  color: var(--orange);
}

.toast-info .toast-icon {
  background: var(--blue-deep);
  color: var(--blue-light);
}

.toast-content {
  flex: 1;
  min-width: 0;
}

.toast-title {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-bright);
  margin-bottom: 4px;
}

.toast-message {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.toast-close {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 20px;
  line-height: 1;
  cursor: pointer;
  padding: 0;
  transition: color 0.2s;
}

.toast-close:hover {
  color: var(--text-bright);
}

/* Animations */
.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100px);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100px) scale(0.9);
}
</style>
