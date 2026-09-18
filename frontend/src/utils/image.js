export function loadImage(src) {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('Failed to load image'))
    img.src = src
  })
}

export function toThumb(dataURI, size = 96) {
  return new Promise((resolve) => {
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = () => {
      const canvas = document.createElement('canvas')
      const scale = Math.min(1, size / Math.max(img.naturalWidth || 1, img.naturalHeight || 1))
      canvas.width = Math.max(1, Math.floor((img.naturalWidth || 1) * scale))
      canvas.height = Math.max(1, Math.floor((img.naturalHeight || 1) * scale))
      const ctx = canvas.getContext('2d')
      ctx.imageSmoothingEnabled = false
      ctx.drawImage(img, 0, 0, canvas.width, canvas.height)
      resolve(canvas.toDataURL('image/png'))
    }
    img.onerror = () => resolve(null)
    img.src = dataURI
  })
}

export function resizeNearestNeighbor(source, width, height) {
  const src = source instanceof HTMLCanvasElement
    ? source
    : (() => { const c = document.createElement('canvas'); c.width = source.width; c.height = source.height; c.getContext('2d').putImageData(source, 0, 0); return c })()
  const canvas = document.createElement('canvas')
  canvas.width = Math.max(1, Math.round(width))
  canvas.height = Math.max(1, Math.round(height))
  const ctx = canvas.getContext('2d')
  ctx.imageSmoothingEnabled = false
  ctx.drawImage(src, 0, 0, canvas.width, canvas.height)
  return canvas
}

function colorDist(data, i, j) {
  const dr = data[i] - data[j]
  const dg = data[i + 1] - data[j + 1]
  const db = data[i + 2] - data[j + 2]
  const da = data[i + 3] - data[j + 3]
  return dr * dr + dg * dg + db * db + da * da
}

// Magic wand / flood fill selection. Returns a Uint8Array mask (1 = selected).
export function floodSelect(imageData, x, y, tolerance, skipTransparent = true) {
  const w = imageData.width
  const h = imageData.height
  const data = imageData.data
  const maxDist = Math.pow((tolerance / 100) * 441.67, 2) // ~sqrt(3)*255 scaled
  const mask = new Uint8Array(w * h)
  const seed = y * w + x
  if (x < 0 || y < 0 || x >= w || y >= h) return mask
  const target = seed * 4
  if (skipTransparent && data[target + 3] === 0) return mask
  const stack = [seed]
  mask[seed] = 1
  while (stack.length) {
    const idx = stack.pop()
    const px = idx % w
    const py = (idx / w) | 0
    const neighbors = [
      px > 0 ? idx - 1 : -1,
      px < w - 1 ? idx + 1 : -1,
      py > 0 ? idx - w : -1,
      py < h - 1 ? idx + w : -1
    ]
    for (const n of neighbors) {
      if (n < 0 || mask[n]) continue
      const ni = n * 4
      if (skipTransparent && data[ni + 3] === 0) continue
      if (colorDist(data, ni, target) > maxDist) continue
      mask[n] = 1
      stack.push(n)
    }
  }
  return mask
}

// Select every pixel similar to the clicked one across the whole image (not just connected region).
export function selectSimilarColors(imageData, x, y, tolerance, skipTransparent = true) {
  const w = imageData.width
  const h = imageData.height
  const data = imageData.data
  const maxDist = Math.pow((tolerance / 100) * 441.67, 2)
  const mask = new Uint8Array(w * h)
  if (x < 0 || y < 0 || x >= w || y >= h) return mask
  const target = (y * w + x) * 4
  if (skipTransparent && data[target + 3] === 0) return mask
  for (let i = 0; i < w * h; i++) {
    const ni = i * 4
    if (skipTransparent && data[ni + 3] === 0) continue
    if (colorDist(data, ni, target) > maxDist) continue
    mask[i] = 1
  }
  return mask
}

// Paint bucket: flood-select then fill with color.
export function bucketFill(imageData, x, y, color, tolerance) {
  const mask = floodSelect(imageData, x, y, tolerance, false)
  applyColorMask(imageData, mask, color)
  return mask
}

export function applyColorMask(imageData, mask, [r, g, b, a]) {
  if (!mask) return
  const data = imageData.data
  for (let i = 0; i < mask.length; i++) {
    if (!mask[i]) continue
    const idx = i * 4
    if (a >= 255) {
      data[idx] = r
      data[idx + 1] = g
      data[idx + 2] = b
      data[idx + 3] = a
    } else {
      const srcA = a / 255
      const dstA = data[idx + 3] / 255
      const outA = srcA + dstA * (1 - srcA)
      if (outA > 0) {
        data[idx] = Math.round((r * srcA + data[idx] * dstA * (1 - srcA)) / outA)
        data[idx + 1] = Math.round((g * srcA + data[idx + 1] * dstA * (1 - srcA)) / outA)
        data[idx + 2] = Math.round((b * srcA + data[idx + 2] * dstA * (1 - srcA)) / outA)
      }
      data[idx + 3] = Math.round(outA * 255)
    }
  }
}

// Brush / eraser stamp. color is [r,g,b,a]; center in pixel coords; radius in px.
export function brushStroke(imageData, mask, cx, cy, radius, color) {
  const w = imageData.width
  const h = imageData.height
  const data = imageData.data
  const r = Math.max(0.5, radius)
  const r2 = r * r
  for (let y = Math.floor(cy - r); y <= Math.ceil(cy + r); y++) {
    if (y < 0 || y >= h) continue
    for (let x = Math.floor(cx - r); x <= Math.ceil(cx + r); x++) {
      if (x < 0 || x >= w) continue
      const dx = x - cx
      const dy = y - cy
      if (dx * dx + dy * dy > r2) continue
      const i = y * w + x
      if (mask && !mask[i]) continue
      const idx = i * 4
      const [cr, cg, cb, ca] = color
      if (ca >= 255) {
        data[idx] = cr
        data[idx + 1] = cg
        data[idx + 2] = cb
        data[idx + 3] = ca
      } else {
        const srcA = ca / 255
        const dstA = data[idx + 3] / 255
        const outA = srcA + dstA * (1 - srcA)
        if (outA > 0) {
          data[idx] = Math.round((cr * srcA + data[idx] * dstA * (1 - srcA)) / outA)
          data[idx + 1] = Math.round((cg * srcA + data[idx + 1] * dstA * (1 - srcA)) / outA)
          data[idx + 2] = Math.round((cb * srcA + data[idx + 2] * dstA * (1 - srcA)) / outA)
        }
        data[idx + 3] = Math.round(outA * 255)
      }
    }
  }
}

// Linear or radial gradient between two colors, constrained to optional mask.
export function gradientFill(imageData, mask, x0, y0, x1, y1, color0, color1, mode) {
  const w = imageData.width
  const h = imageData.height
  const data = imageData.data
  const vx = x1 - x0
  const vy = y1 - y0
  const vlen2 = vx * vx + vy * vy || 1
  const maxR = Math.max(Math.abs(vx), Math.abs(vy)) || 1

  for (let y = 0; y < h; y++) {
    for (let x = 0; x < w; x++) {
      const i = y * w + x
      if (mask && !mask[i]) continue
      let t
      if (mode === 'radial') {
        const dx = x - x0
        const dy = y - y0
        t = Math.sqrt(dx * dx + dy * dy) / maxR
      } else {
        const dot = (x - x0) * vx + (y - y0) * vy
        t = dot / vlen2
      }
      t = Math.max(0, Math.min(1, t))
      const r = Math.round(color0[0] + (color1[0] - color0[0]) * t)
      const g = Math.round(color0[1] + (color1[1] - color0[1]) * t)
      const b = Math.round(color0[2] + (color1[2] - color0[2]) * t)
      const a = Math.round(color0[3] + (color1[3] - color0[3]) * t)
      const idx = i * 4
      if (a >= 255) {
        data[idx] = r
        data[idx + 1] = g
        data[idx + 2] = b
        data[idx + 3] = a
      } else {
        const srcA = a / 255
        const dstA = data[idx + 3] / 255
        const outA = srcA + dstA * (1 - srcA)
        if (outA > 0) {
          data[idx] = Math.round((r * srcA + data[idx] * dstA * (1 - srcA)) / outA)
          data[idx + 1] = Math.round((g * srcA + data[idx + 1] * dstA * (1 - srcA)) / outA)
          data[idx + 2] = Math.round((b * srcA + data[idx + 2] * dstA * (1 - srcA)) / outA)
        }
        data[idx + 3] = Math.round(outA * 255)
      }
    }
  }
}

export function applyColorToMask(imageData, mask, color) {
  applyColorMask(imageData, mask, color)
}

export function hexToRgb(hex) {
  let h = String(hex || '#ffffff').replace('#', '')
  if (h.length === 3) h = h.split('').map(c => c + c).join('')
  const num = parseInt(h, 16)
  if (isNaN(num)) return [255, 255, 255]
  return [(num >> 16) & 255, (num >> 8) & 255, num & 255]
}

export function rgbToHex([r, g, b]) {
  return '#' + [r, g, b].map(v => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')).join('')
}

export function canvasToDataURL(canvas) {
  return canvas.toDataURL('image/png')
}

export function cloneImageData(imageData) {
  return new ImageData(new Uint8ClampedArray(imageData.data), imageData.width, imageData.height)
}