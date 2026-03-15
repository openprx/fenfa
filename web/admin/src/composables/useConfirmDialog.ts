import { inject, provide, type InjectionKey, type Ref } from 'vue'

export interface ConfirmOptions {
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  type?: 'danger' | 'primary' | 'warning'
}

export interface ConfirmDialogInstance {
  show: (options: ConfirmOptions) => Promise<boolean>
}

const CONFIRM_KEY: InjectionKey<Ref<ConfirmDialogInstance | undefined>> = Symbol('confirmDialog')

export function provideConfirmDialog(dialog: Ref<ConfirmDialogInstance | undefined>) {
  provide(CONFIRM_KEY, dialog)
}

export function useConfirmDialog(): ConfirmDialogInstance {
  const dialog = inject(CONFIRM_KEY)
  return {
    show: (options) => dialog?.value?.show(options) ?? Promise.resolve(false),
  }
}
