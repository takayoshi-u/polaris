import { createBackdropDismissHandler } from '~/utils/backdrop'

export default defineNuxtPlugin((nuxtApp) => {
  nuxtApp.vueApp.directive('backdrop-dismiss', {
    mounted(el: HTMLElement & { _onMouseDown?: any; _onMouseUp?: any; _backdropBinding?: any }, binding) {
      el._backdropBinding = binding
      const handler = createBackdropDismissHandler(() => {
        const val = el._backdropBinding ? el._backdropBinding.value : binding.value
        if (typeof val === 'function') {
          val()
        }
      })
      el._onMouseDown = (e: MouseEvent) => handler.onMousedown(e)
      el._onMouseUp = (e: MouseEvent) => handler.onMouseup(e)
      el.addEventListener('mousedown', el._onMouseDown)
      el.addEventListener('mouseup', el._onMouseUp)
    },
    updated(el: HTMLElement & { _backdropBinding?: any }, binding) {
      el._backdropBinding = binding
    },
    unmounted(el: HTMLElement & { _onMouseDown?: any; _onMouseUp?: any; _backdropBinding?: any }) {
      if (el._onMouseDown) el.removeEventListener('mousedown', el._onMouseDown)
      if (el._onMouseUp) el.removeEventListener('mouseup', el._onMouseUp)
      delete el._onMouseDown
      delete el._onMouseUp
      delete el._backdropBinding
    }
  })
})
