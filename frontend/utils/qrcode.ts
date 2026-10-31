import { renderSVG } from 'uqr'

export function generateQRCodeSVG(text: string): string {
  if (!text) return ''
  try {
    return renderSVG(text)
  } catch {
    return ''
  }
}
