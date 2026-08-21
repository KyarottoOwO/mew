const BEDROCK_COLORS = {
  '0': '#000000',
  '1': '#0000AA',
  '2': '#00AA00',
  '3': '#00AAAA',
  '4': '#AA0000',
  '5': '#AA00AA',
  '6': '#FFAA00',
  '7': '#C6C6C6',
  '8': '#555555',
  '9': '#5555FF',
  'a': '#55FF55',
  'b': '#55FFFF',
  'c': '#FF5555',
  'd': '#FF55FF',
  'e': '#FFFF55',
  'f': '#FFFFFF',
  'g': '#DDD605',
  'h': '#E3D4D1',
  'i': '#CECACA',
  'j': '#443A3B',
  'm': '#971607',
  'n': '#B4684D',
  'p': '#DEB12D',
  'q': '#119F36',
  's': '#2CBAA8',
  't': '#21497B',
  'u': '#9A5CC6',
  'v': '#EB7114',
  'w': '#8CB3FF',
}

function escapeHtml(str) {
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
}

export function parseBedrockCodes(text) {
  if (!text) return ''

  let result = ''
  let currentColor = null
  let isBold = false
  let isItalic = false
  let i = 0

  while (i < text.length) {
    const ch = text[i]

    if (ch === '\u00A7' && i + 1 < text.length) {
      const code = text[i + 1].toLowerCase()

      if (BEDROCK_COLORS[code]) {
        if (currentColor || isBold || isItalic) result += '</span>'
        currentColor = BEDROCK_COLORS[code]
        isBold = false
        isItalic = false
        result += `<span style="color:${currentColor}">`
        i += 2
        continue
      }

      if (code === 'l') {
        if (currentColor || isBold || isItalic) result += '</span>'
        isBold = true
        const style = buildStyle(currentColor, isBold, isItalic)
        result += `<span style="${style}">`
        i += 2
        continue
      }

      if (code === 'o') {
        if (currentColor || isBold || isItalic) result += '</span>'
        isItalic = true
        const style = buildStyle(currentColor, isBold, isItalic)
        result += `<span style="${style}">`
        i += 2
        continue
      }

      if (code === 'k') {
        i += 2
        continue
      }

      if (code === 'r') {
        if (currentColor || isBold || isItalic) result += '</span>'
        currentColor = null
        isBold = false
        isItalic = false
        i += 2
        continue
      }

      result += escapeHtml(ch)
      i++
      continue
    }

    if (ch === '\n') {
      result += '<br>'
      i++
      continue
    }

    result += escapeHtml(ch)
    i++
  }

  if (currentColor || isBold || isItalic) result += '</span>'

  return result
}

function buildStyle(color, bold, italic) {
  const parts = []
  if (color) parts.push(`color:${color}`)
  if (bold) parts.push('font-weight:bold')
  if (italic) parts.push('font-style:italic')
  return parts.join(';')
}
