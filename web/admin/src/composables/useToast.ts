import { inject, provide, type InjectionKey, type Ref } from 'vue'

export interface ToastInstance {
  success: (message: string, title?: string) => void
  error: (message: string, title?: string) => void
  warning: (message: string, title?: string) => void
  info: (message: string, title?: string) => void
}

const TOAST_KEY: InjectionKey<Ref<ToastInstance | undefined>> = Symbol('toast')

export function provideToast(toast: Ref<ToastInstance | undefined>) {
  provide(TOAST_KEY, toast)
}

export function useToast(): ToastInstance {
  const toast = inject(TOAST_KEY)
  const noop = () => {}
  return {
    success: (msg, title) => toast?.value?.success(msg, title) ?? noop(),
    error: (msg, title) => toast?.value?.error(msg, title) ?? noop(),
    warning: (msg, title) => toast?.value?.warning(msg, title) ?? noop(),
    info: (msg, title) => toast?.value?.info(msg, title) ?? noop(),
  }
}
