import test from 'node:test'
import assert from 'node:assert/strict'

export function createBackdropDismissHandler(onDismiss) {
  let isDownOnBackdrop = false
  return {
    onMousedown(e) {
      isDownOnBackdrop = (e.target === e.currentTarget)
    },
    onMouseup(e) {
      if (isDownOnBackdrop && e.target === e.currentTarget) {
        onDismiss()
      }
      isDownOnBackdrop = false
    }
  }
}

test('createBackdropDismissHandler dismisses when mousedown and mouseup are both on backdrop', () => {
  let dismissed = false
  const handler = createBackdropDismissHandler(() => {
    dismissed = true
  })

  const backdrop = { id: 'backdrop' }

  // Mousedown on backdrop
  handler.onMousedown({ target: backdrop, currentTarget: backdrop })
  assert.equal(dismissed, false)

  // Mouseup on backdrop
  handler.onMouseup({ target: backdrop, currentTarget: backdrop })
  assert.equal(dismissed, true)
})

test('createBackdropDismissHandler does NOT dismiss when dragging text inside modal onto backdrop', () => {
  let dismissed = false
  const handler = createBackdropDismissHandler(() => {
    dismissed = true
  })

  const backdrop = { id: 'backdrop' }
  const textInsideModal = { id: 'text-element' }

  // User starts selecting text inside the modal (mousedown inside)
  handler.onMousedown({ target: textInsideModal, currentTarget: backdrop })
  assert.equal(dismissed, false)

  // User releases mouse outside on backdrop (mouseup on backdrop)
  handler.onMouseup({ target: backdrop, currentTarget: backdrop })
  assert.equal(dismissed, false, 'Modal should NOT close when dragging text selection outside')
})

test('createBackdropDismissHandler does NOT dismiss when mousedown is on backdrop but mouseup is inside modal', () => {
  let dismissed = false
  const handler = createBackdropDismissHandler(() => {
    dismissed = true
  })

  const backdrop = { id: 'backdrop' }
  const modalPanel = { id: 'modal-panel' }

  // User mousedown on backdrop, but drags into modal
  handler.onMousedown({ target: backdrop, currentTarget: backdrop })
  assert.equal(dismissed, false)

  // Mouseup inside modal
  handler.onMouseup({ target: modalPanel, currentTarget: backdrop })
  assert.equal(dismissed, false, 'Modal should NOT close when mouseup lands inside modal')
})

test('createBackdropDismissHandler does NOT dismiss when mousedown and mouseup are both inside modal', () => {
  let dismissed = false
  const handler = createBackdropDismissHandler(() => {
    dismissed = true
  })

  const backdrop = { id: 'backdrop' }
  const buttonInside = { id: 'btn' }

  handler.onMousedown({ target: buttonInside, currentTarget: backdrop })
  assert.equal(dismissed, false)

  handler.onMouseup({ target: buttonInside, currentTarget: backdrop })
  assert.equal(dismissed, false)
})
