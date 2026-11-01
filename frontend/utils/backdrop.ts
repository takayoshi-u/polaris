export function createBackdropDismissHandler(onDismiss: () => void) {
  let isDownOnBackdrop = false
  return {
    onMousedown(e: { target: any; currentTarget: any }) {
      isDownOnBackdrop = (e.target === e.currentTarget)
    },
    onMouseup(e: { target: any; currentTarget: any }) {
      if (isDownOnBackdrop && e.target === e.currentTarget) {
        onDismiss()
      }
      isDownOnBackdrop = false
    }
  }
}
