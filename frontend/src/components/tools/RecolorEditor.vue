<script setup>
import { ref, reactive, computed, watch, onMounted, onBeforeUnmount, nextTick, markRaw } from 'vue'
import {
  GetImages, SaveImage, ExportPack, DeleteSession, GetSettings,
  ListPackTextures, GetPackTexture, GetPackThumb, SavePackTexture,
  GetPackRoot, OpenFolder
} from '../../../wailsjs/go/main/App'
import { applyHsvShift, applySepia, rgbToHsv, hsvToRgb } from '../../utils/hue'
import {
  floodSelect, selectSimilarColors, applyColorMask, brushStroke, gradientFill, resizeNearestNeighbor,
  loadImage, toThumb, hexToRgb, rgbToHex, canvasToDataURL, cloneImageData
} from '../../utils/image'
import { loadSession, saveSession, clearSession } from '../../utils/recolorSession'

const props = defineProps({
  source: { type: Object, required: true }, // { kind: 'installed'|'cache', dirName, basePath } | { kind: 'upload', folders, packName, sessionId }
  sidebarWidth: { type: Number, default: 64 },
  visible: { type: Boolean, default: true }
})
const emit = defineEmits(['close'])

const isInstalled = computed(() => props.source.kind === 'installed' || props.source.kind === 'cache')
const packBasePath = computed(() => props.source.basePath || '')
const sessionId = computed(() => props.source.sessionId || '')

const folders = ref([])
const textures = ref([])
let allTextures = [] // full pack listing (for global search)
const search = ref('')
const filteredTextures = computed(() => {
  const q = search.value.trim().toLowerCase()
  const src = q ? (allTextures.length ? allTextures : textures.value) : textures.value
  if (!q) return textures.value
  return src.filter(t =>
    (t.relPath || '').toLowerCase().includes(q) ||
    (t.filename || '').toLowerCase().includes(q)
  )
})
const thumbs = reactive(new Map())
const activeFolder = ref('')
const currentTex = ref(null)
const openTabs = ref([])
const tabStates = new Map()
const loading = ref(false)
const ready = ref(false)

const canvasEl = ref(null)
const overlayEl = ref(null)
const viewportEl = ref(null)
const working = ref({ w: 0, h: 0 })
const zoom = ref(1)
const dirty = ref(false)
const busy = ref(false)

const layers = ref([])
const activeLayerId = ref(null)
const layersOpen = ref(true)
let nextLayerId = 1

const BLEND_MODES = [
  { value: 'normal', label: 'Normal' },
  { value: 'multiply', label: 'Multiply' },
  { value: 'screen', label: 'Screen' },
  { value: 'overlay', label: 'Overlay' },
  { value: 'darken', label: 'Darken' },
  { value: 'lighten', label: 'Lighten' },
  { value: 'add', label: 'Add' },
  { value: 'difference', label: 'Difference' },
  { value: 'exclusion', label: 'Exclusion' },
  { value: 'color-dodge', label: 'Color Dodge' },
  { value: 'color-burn', label: 'Color Burn' },
  { value: 'soft-light', label: 'Soft Light' },
  { value: 'hard-light', label: 'Hard Light' }
]

const BLEND_COMPOSITE_OPS = {
  normal: 'source-over',
  multiply: 'multiply',
  screen: 'screen',
  overlay: 'overlay',
  darken: 'darken',
  lighten: 'lighten',
  add: 'lighter',
  difference: 'difference',
  exclusion: 'exclusion',
  'color-dodge': 'color-dodge',
  'color-burn': 'color-burn',
  'soft-light': 'soft-light',
  'hard-light': 'hard-light'
}

const tool = ref('brush')
const fg = ref('#ffffff')
const bg = ref('#000000')
const opacity = ref(100)
const brushSize = ref(4)
const tolerance = ref(20)
const gradientMode = ref('linear')

const selMask = ref(null)
const selRect = ref(null)
const selDrag = ref(null)
let selBase = null
let selAdditive = false
let selSubtractive = false

const hue = ref(0)
const sat = ref(0)
const bright = ref(0)
let hsvBase = null
const hsvOpen = ref(false)

let baseImage = null
let savedImage = null

let lastMask = null
let tintBuffer = null
let tintCanvas = null
let outlineSegs = []
const hover = ref(null)

let renderPending = false
function scheduleRender() {
  if (renderPending) return
  renderPending = true
  requestAnimationFrame(() => {
    renderPending = false
    renderOverlay()
  })
}

const resizing = ref(false)
const resizeW = ref(1)
const resizeH = ref(1)
const resizeLock = ref(true)
const resizeRatio = ref(1)

const undoStack = ref([])
const redoStack = ref([])
const editorClipboard = ref(null)
const floatPaste = ref(null) // { clip, src, x, y, w, h } while pasting from the editor clipboard
const FLOAT_HANDLE = 8 // handle hit area in screen px

const hasFloatPaste = computed(() => !!floatPaste.value)

const saveMenuOpen = ref(false)
const bottomH = ref(230)
let bottomDrag = null

function onBottomGripDown(e) {
  bottomDrag = { y: e.clientY, h: bottomH.value }
  window.addEventListener('pointermove', onBottomGripMove)
  window.addEventListener('pointerup', onBottomGripUp)
  e.preventDefault()
}

function onBottomGripMove(e) {
  if (!bottomDrag) return
  bottomH.value = Math.max(120, Math.min(window.innerHeight * 0.6, bottomDrag.h + (bottomDrag.y - e.clientY)))
}

function onBottomGripUp() {
  bottomDrag = null
  window.removeEventListener('pointermove', onBottomGripMove)
  window.removeEventListener('pointerup', onBottomGripUp)
}

const ALL_TAB = '__all__'

function folderLabel(f) {
  if (f === ALL_TAB) return 'All'
  return (f || 'root').replace('textures/', '')
}

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

async function loadFolders() {
  folders.value = []
  if (isInstalled.value) {
    loading.value = true
    try {
      const list = (await ListPackTextures(props.source.dirName, packBasePath.value)) || []
      const folderSet = new Set(list.map(t => t.folder || ''))
      folders.value = [ALL_TAB, ...[...folderSet].sort()]
      if (folders.value.length) await selectFolder(folders.value[0])
    } catch (e) {
      popup('Error', 'Failed to list textures: ' + e.toString(), 'error')
    }
    loading.value = false
  } else {
    folders.value = [ALL_TAB].concat(props.source.folders || [])
    if (folders.value.length) await selectFolder(folders.value[0])
  }
}

async function selectFolder(folder) {
  const isAll = folder === ALL_TAB
  const listAll = (list) => isAll ? list : list.filter(t => (t.folder || '') === folder)
  activeFolder.value = folder
  loading.value = true
  textures.value = []
  try {
    if (isInstalled.value) {
      const list = (await ListPackTextures(props.source.dirName, packBasePath.value)) || []
      allTextures = list
      textures.value = listAll(list)
      loadInstalledThumbs()
    } else {
      const raw = (await GetImages(sessionId.value, isAll ? '' : folder)) || []
      const mapped = raw.map(img => ({
        folder: isAll ? (img.relPath.includes('/') ? img.relPath.slice(0, img.relPath.lastIndexOf('/')) : '') : folder,
        filename: img.filename,
        relPath: img.relPath,
        size: img.size,
        dataURI: img.dataURI
      }))
      allTextures = mapped
      textures.value = listAll(mapped)
      loadUploadThumbs()
    }
  } catch (e) {
    popup('Error', 'Failed to load folder: ' + e.toString(), 'error')
  }
  loading.value = false
}

let thumbQueue = null
const thumbRequested = new Set()

function loadInstalledThumbs(list) {
  if (thumbQueue) thumbQueue.cancelled = true
  const targets = (list || textures.value).filter(t => !thumbs.has(t.relPath))
  thumbRequested.clear()
  for (const t of targets) thumbRequested.add(t.relPath)
  const queue = { cancelled: false, idx: 0 }
  thumbQueue = queue
  const CONCURRENCY = 16
  const worker = async () => {
    while (!queue.cancelled) {
      const i = queue.idx++
      if (i >= targets.length) return
      const tex = targets[i]
      try {
        const th = await GetPackThumb(props.source.dirName, tex.relPath, packBasePath.value)
        if (queue.cancelled) return
        if (th) thumbs.set(tex.relPath, th)
      } catch (e) { console.warn('[recolor] thumb failed:', tex.relPath, e) }
    }
  }
  for (let i = 0; i < CONCURRENCY; i++) worker()
}

function loadUploadThumbs(list) {
  if (thumbQueue) thumbQueue.cancelled = true
  const targets = (list || textures.value).filter(t => !thumbs.has(t.relPath))
  const queue = { cancelled: false, idx: 0 }
  thumbQueue = queue
  const worker = async () => {
    while (!queue.cancelled) {
      const i = queue.idx++
      if (i >= targets.length) return
      const tex = targets[i]
      try {
        const th = await toThumb(tex.dataURI)
        if (queue.cancelled) return
        if (th) thumbs.set(tex.relPath, th)
      } catch (e) { console.warn('[recolor] upload thumb failed:', tex.relPath, e) }
    }
  }
  for (let i = 0; i < 4; i++) worker()
}

function showThumb(tex) {
  if (tex.dataURI && !thumbs.has(tex.relPath)) return tex.dataURI
  return thumbs.get(tex.relPath) || (tex.dataURI || null)
}

watch(search, () => {
  if (search.value.trim()) {
    if (isInstalled.value) loadInstalledThumbs(filteredTextures.value)
    else loadUploadThumbs(filteredTextures.value)
  } else {
    if (isInstalled.value) loadInstalledThumbs()
    else loadUploadThumbs()
  }
})

watch(dirty, (v) => {
  if (!currentTex.value) return
  const tab = openTabs.value.find(t => t.relPath === currentTex.value.relPath)
  if (tab) tab.dirty = v
})

watch(() => props.source, async (_nVal, oVal) => {
  if (oVal) {
    if (dirty.value) await flushAutosave(true)
    saveSessionFor(oVal)
  }
  openTabs.value = []
  tabStates.clear()
  thumbs.clear()
  resetEditor()
  await loadFolders()
  restoreSession()
})

watch(() => props.visible, async (v) => {
  if (v) await reloadFolder()
})

async function reloadFolder() {
  if (!folders.value.length) return
  for (const t of textures.value) thumbs.delete(t.relPath)
  await selectFolder(activeFolder.value)
}

async function openTexture(tex) {
  const existing = openTabs.value.find(t => t.relPath === tex.relPath)
  if (existing) {
    await switchTab(tex.relPath)
    return
  }
  openTabs.value.push({ relPath: tex.relPath, tex, dirty: false })
  await switchTab(tex.relPath)
}

async function switchTab(relPath) {
  const tab = openTabs.value.find(t => t.relPath === relPath)
  if (!tab) return
  if (currentTex.value && currentTex.value.relPath === relPath) return
  // a background tab is only ever left behind once it is safely on disk
  if (dirty.value) await flushAutosave()
  captureActiveState()
  currentTex.value = tab.tex
  const st = tabStates.get(relPath)
  if (st) {
    ready.value = true
    restoreTabState(relPath)
    await nextTick()
    renderOverlay()
    return
  }
  ready.value = false
  dirty.value = false
  selMask.value = null
  selBase = null
  selAdditive = false
  selSubtractive = false
  undoStack.value = []
  redoStack.value = []
  hsvBase = null
  try {
    let uri = tab.tex.dataURI
    if (isInstalled.value && !uri) uri = await GetPackTexture(props.source.dirName, tab.tex.relPath, packBasePath.value)
    if (!uri) throw new Error('no image data')
    layers.value = []
    await renderImage(uri)
  } catch (e) {
    popup('Error', 'Failed to open texture: ' + e.toString(), 'error')
  }
}

function captureActiveState() {
  if (!currentTex.value || !ready.value) return
  if (hsvOpen.value) {
    cancelHsv()
    hsvOpen.value = false
  }
  if (floatPaste.value) commitFloatingPaste()
  const relPath = currentTex.value.relPath
  if (!openTabs.value.some(t => t.relPath === relPath)) return
  tabStates.set(relPath, {
    tex: currentTex.value,
    ...snapshotLayers(),
    undo: undoStack.value,
    redo: redoStack.value,
    selMask: selMask.value,
    selBase,
    selAdditive,
    selSubtractive,
    hsvBase,
    hue: hue.value,
    sat: sat.value,
    bright: bright.value,
    baseImage,
    savedImage,
    dirty: dirty.value,
    zoom: zoom.value,
    nextLayerId
  })
}

function restoreTabState(relPath) {
  const st = tabStates.get(relPath)
  if (!st) return
  undoStack.value = st.undo
  redoStack.value = st.redo
  selMask.value = st.selMask
  selBase = st.selBase
  selAdditive = st.selAdditive
  selSubtractive = st.selSubtractive
  hsvBase = null
  baseImage = st.baseImage
  savedImage = st.savedImage
  dirty.value = st.dirty
  zoom.value = st.zoom
  nextLayerId = st.nextLayerId
  rectsCache = null
  restoreSnapshot(st)
}

function resetEditor() {
  currentTex.value = null
  ready.value = false
  dirty.value = false
  selMask.value = null
  selBase = null
  selAdditive = false
  selSubtractive = false
  lastMask = null
  hsvBase = null
  hue.value = 0
  sat.value = 0
  bright.value = 0
  hsvOpen.value = false
  hover.value = null
  baseImage = null
  savedImage = null
  layers.value = []
  undoStack.value = []
  redoStack.value = []
  activeLayerId.value = null
  working.value = { w: 0, h: 0 }
  zoom.value = 1
  rectsCache = null
  renderOverlay()
}

async function closeTab(relPath) {
  const idx = openTabs.value.findIndex(t => t.relPath === relPath)
  if (idx === -1) return
  const isActive = !!(currentTex.value && currentTex.value.relPath === relPath)
  if (isActive) {
    // write pending edits before dropping the tab
    if (dirty.value && !(await flushAutosave(true))) return
    captureActiveState()
  }
  openTabs.value.splice(idx, 1)
  tabStates.delete(relPath)
  if (!isActive) return
  const next = openTabs.value[Math.min(idx, openTabs.value.length - 1)]
  if (next) switchTab(next.relPath)
  else resetEditor()
}

async function closeAllTabs() {
  if (currentTex.value && dirty.value && !(await flushAutosave(true))) return
  openTabs.value = []
  tabStates.clear()
  resetEditor()
}

function tabLabel(tab) {
  const f = tab.tex.filename || (tab.tex.relPath || '').split('/').pop() || ''
  return f.replace(/\.png$/i, '') || 'texture'
}

function saveSessionFor(source) {
  if (!source) return
  captureActiveState()
  if (!openTabs.value.length && !tabStates.size) {
    clearSession(source)
    return
  }
  saveSession(source, {
    openTabs: openTabs.value.map(t => ({ relPath: t.relPath, tex: t.tex, dirty: t.dirty })),
    tabStates,
    activeRelPath: currentTex.value ? currentTex.value.relPath : null,
    global: {
      tool: tool.value,
      fg: fg.value,
      bg: bg.value,
      opacity: opacity.value,
      brushSize: brushSize.value,
      tolerance: tolerance.value,
      gradientMode: gradientMode.value,
      layersOpen: layersOpen.value
    }
  })
}

function saveSessionToStore() {
  saveSessionFor(props.source)
}

function restoreSession() {
  const session = loadSession(props.source)
  if (!session) return
  openTabs.value = session.openTabs.map(t => ({ relPath: t.relPath, tex: t.tex, dirty: t.dirty }))
  session.tabStates.forEach((v, k) => tabStates.set(k, v))
  const g = session.global
  if (g) {
    tool.value = g.tool || 'brush'
    fg.value = g.fg || '#ffffff'
    bg.value = g.bg || '#000000'
    opacity.value = g.opacity ?? 100
    brushSize.value = g.brushSize ?? 4
    tolerance.value = g.tolerance ?? 20
    gradientMode.value = g.gradientMode || 'linear'
    layersOpen.value = g.layersOpen ?? true
  }
  if (session.activeRelPath) {
    const tab = openTabs.value.find(t => t.relPath === session.activeRelPath)
    if (tab) switchTab(session.activeRelPath)
  }
}

function renderImage(uri) {
  return new Promise((resolve) => {
    const img = new Image()
    img.crossOrigin = 'anonymous'
    img.onload = async () => {
      const canvas = canvasEl.value
      canvas.width = img.naturalWidth
      canvas.height = img.naturalHeight
      const ctx = canvas.getContext('2d')
      ctx.clearRect(0, 0, canvas.width, canvas.height)
      ctx.drawImage(img, 0, 0)
      working.value = { w: canvas.width, h: canvas.height }
      syncCanvasSizes()
      fitZoom()
      await nextTick()
      layers.value = [{
        id: nextLayerId++,
        name: 'Layer 1',
        visible: true,
        opacity: 100,
        blend: 'normal',
        image: markRaw(cloneImageData(ctx.getImageData(0, 0, canvas.width, canvas.height)))
      }]
      activeLayerId.value = layers.value[0].id
      commit()
      ready.value = true
      baseImage = cloneImageData(ctx.getImageData(0, 0, canvas.width, canvas.height))
      savedImage = null
      dirty.value = false
      hsvBase = null
      resolve()
    }
    img.onerror = () => { popup('Error', 'Failed to decode image', 'error'); resolve() }
    img.src = uri
  })
}

function syncCanvasSizes() {
  if (!overlayEl.value) return
  const octx = overlayEl.value.getContext('2d')
  octx.clearRect(0, 0, overlayEl.value.width, overlayEl.value.height)
  lastMask = null
}

function fitZoom() {
  const vp = viewportEl.value
  if (!vp) return
  const avail = Math.max(50, vp.clientWidth - 40)
  const availH = Math.max(50, vp.clientHeight - 40)
  zoom.value = Math.min(1, avail / working.value.w, availH / working.value.h)
  rectsCache = null
}

function zoomBy(f) {
  zoom.value = Math.max(0.05, Math.min(64, zoom.value * f))
  rectsCache = null
}

function centerImage() {
  if (!currentTex.value || !ready.value || !canvasEl.value) return
  const w = working.value.w
  const h = working.value.h
  if (!w || !h) return
  commit()
  const data = getImageData()
  const px = Math.floor(w / 2)
  const py = Math.floor(h / 2)
  const i = py * data.width + px
  data.data[i * 4] = 0
  data.data[i * 4 + 1] = 0
  data.data[i * 4 + 2] = 0
  data.data[i * 4 + 3] = 255
  putImageData(data)
  markDirty()
}

function canvasCtx() {
  return canvasEl.value.getContext('2d')
}

function activeLayer() {
  return layers.value.find(l => l.id === activeLayerId.value) || null
}

function dataToCanvas(img) {
  const c = document.createElement('canvas')
  c.width = img.width
  c.height = img.height
  c.getContext('2d').putImageData(img, 0, 0)
  return c
}

const layerCanvasCache = new WeakMap()

function bumpLayerVersion(layer) {
  if (layer) layer.__ver = (layer.__ver || 0) + 1
}

function getLayerCanvas(layer) {
  const ver = layer.__ver || 0
  const entry = layerCanvasCache.get(layer)
  if (entry && entry.ver === ver) return entry.canvas
  const c = dataToCanvas(layer.image)
  layerCanvasCache.set(layer, { ver, canvas: c })
  return c
}

function recomposite() {
  const c = canvasEl.value
  if (!c || !working.value.w) return
  const ctx = canvasCtx()
  ctx.clearRect(0, 0, c.width, c.height)
  ctx.globalCompositeOperation = 'source-over'
  for (const layer of layers.value) {
    if (!layer.visible || !layer.image) continue
    const op = BLEND_COMPOSITE_OPS[layer.blend] || 'source-over'
    ctx.globalAlpha = layer.opacity / 100
    ctx.globalCompositeOperation = op
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(getLayerCanvas(layer), 0, 0)
  }
  ctx.globalAlpha = 1
  ctx.globalCompositeOperation = 'source-over'
}

function getImageData() {
  const layer = activeLayer()
  if (layer) return layer.image
  const c = canvasEl.value
  return c.getContext('2d').getImageData(0, 0, c.width, c.height)
}

function putImageData(data) {
  const layer = activeLayer()
  if (layer) {
    layer.image = markRaw(data)
    bumpLayerVersion(layer)
    recomposite()
  } else {
    canvasCtx().putImageData(data, 0, 0)
  }
  renderOverlay()
}

function snapshotLayers() {
  return {
    activeId: activeLayerId.value,
    w: working.value.w,
    h: working.value.h,
    sel: selMask.value ? new Uint8Array(selMask.value) : null,
    layers: layers.value.map(l => ({
      id: l.id,
      name: l.name,
      visible: l.visible,
      opacity: l.opacity,
      blend: l.blend || 'normal',
      image: cloneImageData(l.image)
    }))
  }
}

function snapshotsEqual(a, b) {
  if (!a || !b) return false
  if (a.activeId !== b.activeId || a.w !== b.w || a.h !== b.h) return false
  if ((a.sel ? 1 : 0) !== (b.sel ? 1 : 0)) return false
  if (a.sel && b.sel) {
    const as = a.sel
    const bs = b.sel
    for (let i = 0; i < as.length; i++) if (as[i] !== bs[i]) return false
  }
  if (a.layers.length !== b.layers.length) return false
  for (let i = 0; i < a.layers.length; i++) {
    const la = a.layers[i]
    const lb = b.layers[i]
    if (la.id !== lb.id || la.name !== lb.name || la.visible !== lb.visible || la.opacity !== lb.opacity || la.blend !== lb.blend) return false
    if (!equalsImageData(la.image, lb.image)) return false
  }
  return true
}

function commit() {
  const snap = snapshotLayers()
  const top = undoStack.value[undoStack.value.length - 1]
  if (top && snapshotsEqual(top, snap)) return
  undoStack.value.push(snap)
  redoStack.value = []
}

function restoreSnapshot(snap) {
  layers.value = snap.layers.map(l => markRaw({
    id: l.id,
    name: l.name,
    visible: l.visible,
    opacity: l.opacity,
    blend: l.blend,
    image: l.image
  }))
  activeLayerId.value = snap.activeId
  selMask.value = snap.sel ? new Uint8Array(snap.sel) : null
  selBase = null
  selAdditive = false
  selSubtractive = false
  lastMask = null
  const canvas = canvasEl.value
  if (canvas && (canvas.width !== snap.w || canvas.height !== snap.h)) {
    canvas.width = snap.w
    canvas.height = snap.h
    working.value = { w: snap.w, h: snap.h }
    syncCanvasSizes()
  } else {
    working.value = { w: snap.w, h: snap.h }
  }
  const maxId = snap.layers.reduce((m, l) => Math.max(m, l.id), 0)
  nextLayerId = Math.max(nextLayerId, maxId + 1)
  recomposite()
  renderOverlay()
}

function undo() {
  if (!undoStack.value.length) return
  redoStack.value.push(snapshotLayers())
  const prev = undoStack.value.pop()
  restoreSnapshot(prev)
  hsvBase = null
  actualizeDirty()
}

function redo() {
  if (!redoStack.value.length) return
  undoStack.value.push(snapshotLayers())
  const next = redoStack.value.pop()
  restoreSnapshot(next)
  hsvBase = null
  actualizeDirty()
}

function currentImageData() {
  return canvasEl.value
    ? canvasCtx().getImageData(0, 0, canvasEl.value.width, canvasEl.value.height)
    : null
}

function equalsImageData(a, b) {
  if (!a || !b) return false
  if (a.width !== b.width || a.height !== b.height) return false
  const ad = a.data
  const bd = b.data
  for (let i = 0; i < ad.length; i++) if (ad[i] !== bd[i]) return false
  return true
}

function actualizeDirty() {
  const base = savedImage || baseImage
  if (!base) {
    dirty.value = false
    return
  }
  dirty.value = !equalsImageData(currentImageData(), base)
  if (dirty.value) scheduleAutosave()
}

// ---------- pointer math ----------
let rectsCache = null
let viewportObserver = null

function refreshRects() {
  const vp = viewportEl.value
  const can = canvasEl.value
  rectsCache = {
    vp: vp ? vp.getBoundingClientRect() : null,
    can: can ? can.getBoundingClientRect() : null
  }
}

function getRects() {
  if (!rectsCache) refreshRects()
  return rectsCache
}

function nativeCoords(e) {
  // Start a gesture with a fresh layout read; reuses the cached rect for the
  // duration of the gesture so rapid pointermove calls don't force reflow.
  const can = getRects().can
  const x = (e.clientX - can.left) / zoom.value
  const y = (e.clientY - can.top) / zoom.value
  return { x, y }
}

function snapBrushPoint(x, y) {
  if (brushSize.value !== 1) return { x, y }
  return { x: Math.floor(x), y: Math.floor(y) }
}

const pointer = ref({ down: false, tool: null, start: null, last: null, button: 0 })

function onPointerDown(e) {
  if (!currentTex.value || !canvasEl.value || !ready.value) return
  refreshRects()
  pointer.value.down = true
  pointer.value.button = e.button
  const { x, y } = nativeCoords(e)
  pointer.value.start = { x, y }
  pointer.value.last = { x, y }
  pointer.value.tool = tool.value
  if (floatPaste.value) {
    const fp = floatPaste.value
    const hx = FLOAT_HANDLE / zoom.value
    pointer.value.tool = 'float'
    let mode = 'outside'
    const shift = e.shiftKey
    const corners = [
      { cx: fp.x, cy: fp.y, dx: -1, dy: -1 },
      { cx: fp.x + fp.w, cy: fp.y, dx: 1, dy: -1 },
      { cx: fp.x, cy: fp.y + fp.h, dx: -1, dy: 1 },
      { cx: fp.x + fp.w, cy: fp.y + fp.h, dx: 1, dy: 1 }
    ]
    for (const c of corners) {
      if (Math.abs(x - c.cx) <= hx && Math.abs(y - c.cy) <= hx) {
        mode = 'resize'
        pointer.value.floatCorner = { dx: c.dx, dy: c.dy, shift }
        break
      }
    }
    if (mode !== 'resize' && x >= fp.x && x <= fp.x + fp.w && y >= fp.y && y <= fp.y + fp.h) {
      mode = 'move'
    }
    if (mode === 'outside') {
      commitFloatingPaste()
      pointer.value.down = false
      renderOverlay()
      e.preventDefault()
      return
    }
    pointer.value.floatMode = mode
    pointer.value.floatOrigin = { x: fp.x, y: fp.y, w: fp.w, h: fp.h }
    renderOverlay()
    e.preventDefault()
    canvasEl.value.setPointerCapture(e.pointerId)
    return
  }
  if (tool.value === 'brush') {
    commit()
    stamp({ x, y })
    pointer.value.moved = true
  } else if (tool.value === 'eraser') {
    commit()
    eraseStamp({ x, y })
    pointer.value.moved = true
  } else if (tool.value === 'picker') {
    pickColor(x, y, e.button)
    pointer.value.down = false
  } else if (tool.value === 'wand') {
    wandAt(x, y, e.altKey ? 'sub' : (e.ctrlKey ? 'add' : 'replace'), e.shiftKey)
    pointer.value.down = false
  } else if (tool.value === 'select') {
    commit()
    selBase = selMask.value
    selAdditive = e.ctrlKey
    selSubtractive = e.altKey
    selDrag.value = { x0: x, y0: y, x1: x, y1: y }
    previewRectMask()
  } else if (tool.value === 'bucket') {
    commit()
    bucketAt(x, y)
    pointer.value.down = false
    markDirty()
  } else if (tool.value === 'gradient') {
    commit()
    selDrag.value = { x0: x, y0: y, x1: x, y1: y }
  }
  renderOverlay()
  e.preventDefault()
  canvasEl.value.setPointerCapture(e.pointerId)
}

function onPointerMove(e) {
  const { x, y } = nativeCoords(e)
  hover.value = { x, y }
  if (floatPaste.value && pointer.value.down && pointer.value.tool === 'float') {
    const fp = floatPaste.value
    const o = pointer.value.floatOrigin
    const dx = x - pointer.value.start.x
    const dy = y - pointer.value.start.y
    if (pointer.value.floatMode === 'move') {
      fp.x = Math.max(-Math.round(fp.w / 2), Math.min(working.value.w - Math.round(fp.w / 2), Math.round(o.x + dx)))
      fp.y = Math.max(-Math.round(fp.h / 2), Math.min(working.value.h - Math.round(fp.h / 2), Math.round(o.y + dy)))
    } else if (pointer.value.floatMode === 'resize') {
      const c = pointer.value.floatCorner
      let nw = c.dx === -1 ? (o.x + o.w) - x : x - o.x
      let nh = c.dy === -1 ? (o.y + o.h) - y : y - o.y
      if (c.shift) {
        const ratio = fp.clip.w / fp.clip.h
        if (Math.abs(nw - o.w) >= Math.abs(nh - o.h)) nh = nw / ratio
        else nw = nh * ratio
      }
      fp.w = Math.max(1, Math.round(Math.abs(nw)))
      fp.h = Math.max(1, Math.round(Math.abs(nh)))
      fp.x = c.dx === -1 ? Math.round(o.x + o.w) - fp.w : Math.round(o.x)
      fp.y = c.dy === -1 ? Math.round(o.y + o.h) - fp.h : Math.round(o.y)
    }
    hover.value = null
    renderOverlay()
    return
  }
  if (!pointer.value.down) {
    renderOverlay()
    return
  }
  const last = pointer.value.last
  if (pointer.value.tool === 'brush' || pointer.value.tool === 'eraser') {
    const dist = Math.hypot(x - last.x, y - last.y)
    const steps = Math.max(1, Math.ceil(dist / Math.max(1, brushSize.value / 2)))
    const isErase = pointer.value.tool === 'eraser'
    const pts = []
    for (let i = 1; i <= steps; i++) {
      pts.push({
        x: last.x + (x - last.x) * i / steps,
        y: last.y + (y - last.y) * i / steps
      })
    }
    stampMany(pts, isErase)
    pointer.value.moved = true
  } else if (pointer.value.tool === 'gradient') {
    selDrag.value = { x0: pointer.value.start.x, y0: pointer.value.start.y, x1: x, y1: y }
    applyGradient(selDrag.value)
  } else if (pointer.value.tool === 'select') {
    selDrag.value = { x0: pointer.value.start.x, y0: pointer.value.start.y, x1: x, y1: y }
    pointer.value.moved = true
    previewRectMask()
  }
  pointer.value.last = { x, y }
  scheduleRender()
}

function onPointerUp() {
  if (!pointer.value.down) return
  if (pointer.value.tool === 'float') {
    pointer.value.down = false
    pointer.value.tool = null
    pointer.value.floatMode = null
    pointer.value.floatCorner = null
    pointer.value.floatOrigin = null
    pointer.value.moved = false
    hover.value = null
    renderOverlay()
    return
  }
  pointer.value.down = false
  hover.value = null
  if ((pointer.value.tool === 'brush' || pointer.value.tool === 'eraser') && pointer.value.moved) {
    markDirty()
  }
  if (pointer.value.tool === 'select' && selDrag.value) {
    if (pointer.value.moved) commitRectMask(selDrag.value)
    else stampRect(selDrag.value)
    selBase = null
  }
  pointer.value.moved = false
  selDrag.value = null
  renderOverlay()
}

function activeMask() {
  return selMask.value
}

function strokeColor() {
  return hexToRgb(pointer.value.button === 2 ? bg.value : fg.value).concat([Math.round(opacity.value * 2.55)])
}

function stamp(p) {
  const data = getImageData()
  const mask = activeMask()
  const color = strokeColor()
  const c = snapBrushPoint(p.x, p.y)
  brushStroke(data, mask, c.x, c.y, brushSize.value / 2, color)
  putImageData(data)
}

function eraseIn(data, x, y) {
  if (brushSize.value === 1) {
    x = Math.floor(x)
    y = Math.floor(y)
  }
  const w = data.width
  const h = data.height
  const d = data.data
  const mask = activeMask()
  const r = Math.max(0.5, brushSize.value / 2)
  const strength = opacity.value / 100
  for (let yy = Math.floor(y - r); yy <= Math.ceil(y + r); yy++) {
    for (let xx = Math.floor(x - r); xx <= Math.ceil(x + r); xx++) {
      if (xx < 0 || yy < 0 || xx >= w || yy >= h) continue
      const dx = xx - x
      const dy = yy - y
      if (dx * dx + dy * dy > r * r) continue
      const i = yy * w + xx
      if (mask && !mask[i]) continue
      d[i * 4 + 3] = Math.round(d[i * 4 + 3] * (1 - strength))
    }
  }
}

function eraseStamp(p) {
  const data = getImageData()
  eraseIn(data, p.x, p.y)
  putImageData(data)
}

function stampMany(pts, isErase) {
  const data = getImageData()
  const mask = activeMask()
  const color = strokeColor()
  for (const p of pts) {
    const c = snapBrushPoint(p.x, p.y)
    if (isErase) eraseIn(data, c.x, c.y)
    else brushStroke(data, mask, c.x, c.y, brushSize.value / 2, color)
  }
  const layer = activeLayer()
  if (layer) {
    layer.image = markRaw(data)
    bumpLayerVersion(layer)
    recomposite()
  } else {
    canvasCtx().putImageData(data, 0, 0)
  }
}

function bucketAt(x, y) {
  const data = getImageData()
  const color = hexToRgb(fg.value).concat([Math.round(opacity.value * 2.55)])
  const mask = floodSelect(data, Math.floor(x), Math.floor(y), tolerance.value, false)
  const existing = selMask.value
  if (existing) {
    for (let i = 0; i < mask.length; i++) mask[i] = existing[i] ? mask[i] : 0
  }
  applyColorMask(data, mask, color)
  putImageData(data)
}

function wandAt(x, y, mode, useGlobal) {
  const data = getImageData()
  const mask = useGlobal
    ? selectSimilarColors(data, Math.floor(x), Math.floor(y), tolerance.value, false)
    : floodSelect(data, Math.floor(x), Math.floor(y), tolerance.value, false)
  commit()
  if (mode === 'add' && selMask.value) selMask.value = unionMask(selMask.value, mask)
  else if (mode === 'sub' && selMask.value) selMask.value = subtractMask(selMask.value, mask)
  else selMask.value = mask
  selBase = null
  renderOverlay()
}

function applyGradient(g) {
  const data = getImageData()
  const color0 = hexToRgb(fg.value).concat([Math.round(opacity.value * 2.55)])
  const color1 = hexToRgb(bg.value).concat([Math.round(opacity.value * 2.55)])
  gradientFill(data, activeMask(), g.x0, g.y0, g.x1, g.y1, color0, color1, gradientMode.value)
  putImageData(data)
  markDirty()
}

function rectMask(g) {
  const w = canvasEl.value.width
  const h = canvasEl.value.height
  let x0 = Math.max(0, Math.floor(Math.min(g.x0, g.x1)))
  let y0 = Math.max(0, Math.floor(Math.min(g.y0, g.y1)))
  let x1 = Math.min(w, Math.ceil(Math.max(g.x0, g.x1)))
  let y1 = Math.min(h, Math.ceil(Math.max(g.y0, g.y1)))
  if (x1 === x0) x1 = Math.min(w, x0 + 1)
  if (y1 === y0) y1 = Math.min(h, y0 + 1)
  const mask = new Uint8Array(w * h)
  for (let y = y0; y < y1; y++) {
    for (let x = x0; x < x1; x++) {
      mask[y * w + x] = 1
    }
  }
  return mask
}

function unionMask(a, b) {
  const next = new Uint8Array(a.length)
  for (let i = 0; i < next.length; i++) next[i] = a[i] || b[i]
  return next
}

function subtractMask(a, b) {
  const next = new Uint8Array(a.length)
  for (let i = 0; i < next.length; i++) next[i] = a[i] && !b[i]
  return next
}

function selectAll() {
  const c = canvasEl.value
  if (!c) return
  commit()
  selMask.value = new Uint8Array(c.width * c.height).fill(1)
  selBase = null
  renderOverlay()
}

function previewRectMask() {
  if (!selDrag.value) return
  const rect = rectMask(selDrag.value)
  if (selBase && selAdditive) selMask.value = unionMask(selBase, rect)
  else if (selBase && selSubtractive) selMask.value = subtractMask(selBase, rect)
  else selMask.value = rect
}

function commitRectMask(g) {
  const rect = rectMask(g)
  if (selBase && selAdditive) selMask.value = unionMask(selBase, rect)
  else if (selBase && selSubtractive) selMask.value = subtractMask(selBase, rect)
  else selMask.value = rect
}

function stampRect(g) {
  const x = Math.floor(g.x0)
  const y = Math.floor(g.y0)
  const w = canvasEl.value.width
  const h = canvasEl.value.height
  if (x < 0 || y < 0 || x >= w || y >= h) return
  const rect = new Uint8Array(w * h)
  rect[y * w + x] = 1
  if (selBase && selAdditive) selMask.value = unionMask(selBase, rect)
  else if (selBase && selSubtractive) selMask.value = subtractMask(selBase, rect)
  else selMask.value = rect
  renderOverlay()
}

function clearSelection() {
  commit()
  selMask.value = null
  selBase = null
  selAdditive = false
  selSubtractive = false
  renderOverlay()
}

function pickColor(x, y, btn) {
  const c = canvasEl.value
  const px = Math.floor(x)
  const py = Math.floor(y)
  if (px < 0 || py < 0 || px >= c.width || py >= c.height) return
  const d = c.getContext('2d').getImageData(px, py, 1, 1).data
  const hex = '#' + Array.from(d.slice(0, 3)).map(v => v.toString(16).padStart(2, '0')).join('')
  if (btn === 2) bg.value = hex
  else fg.value = hex
}

function swapColors() {
  const t = fg.value
  fg.value = bg.value
  bg.value = t
}

const pickTarget = ref('fg')
const pickerOpen = ref(false)
const moreOpen = ref(false)
const pickHsv = ref({ h: 0, s: 0, v: 100 })
const pickRgb = ref({ r: 255, g: 255, b: 255 })
const pickHex = ref('#FFFFFF')
const hexDraft = ref('FFFFFF')
const wheelEl = ref(null)
const wheelCanvasEl = ref(null)
let wheelDragging = false
const brightBarEl = ref(null)
const brightCanvasEl = ref(null)
let brightDragging = false

const PALETTE_COLORS = buildPalette()

function buildPalette() {
  const colors = []
  const hues = [0, 40, 80, 120, 160, 200, 240, 300]
  const shades = [
    [50, 96], [90, 96], [100, 88], [100, 70], [100, 50], [100, 28]
  ]
  for (const [s, v] of shades) {
    for (const h of hues) {
      colors.push(rgbToHex(hsvToRgb(h, s, v)))
    }
  }
  return colors
}

function switchSlot(slot) {
  pickTarget.value = slot
  setPickerColor(slot === 'fg' ? fg.value : bg.value)
}

function onPickSwap() {
  swapColors()
  setPickerColor(pickTarget.value === 'fg' ? fg.value : bg.value)
}

const wheelMarkerStyle = computed(() => {
  const ang = (pickHsv.value.h * Math.PI) / 180
  const rad = (pickHsv.value.s / 100) * 50
  return {
    left: (50 + rad * Math.cos(ang)) + '%',
    top: (50 + rad * Math.sin(ang)) + '%'
  }
})

function drawColorWheel() {
  const c = wheelCanvasEl.value
  if (!c) return
  const size = c.width
  const ctx = c.getContext('2d')
  const img = ctx.createImageData(size, size)
  const d = img.data
  const cx = size / 2
  const cy = size / 2
  const r = size / 2
  for (let y = 0; y < size; y++) {
    for (let x = 0; x < size; x++) {
      const dx = x - cx
      const dy = y - cy
      const idx = (y * size + x) * 4
      if (dx * dx + dy * dy > r * r) continue
      const hue = (Math.atan2(dy, dx) * 180 / Math.PI + 360) % 360
      const sat = Math.min(100, Math.sqrt(dx * dx + dy * dy) / r * 100)
      const [rr, gg, bb] = hsvToRgb(hue, sat, 100)
      d[idx] = rr
      d[idx + 1] = gg
      d[idx + 2] = bb
      d[idx + 3] = 255
    }
  }
  ctx.putImageData(img, 0, 0)
}

function drawBrightBar() {
  const c = brightCanvasEl.value
  if (!c) return
  const w = c.width
  const h = c.height
  const ctx = c.getContext('2d')
  const img = ctx.createImageData(w, h)
  const d = img.data
  const { h: hue, s: sat } = pickHsv.value
  for (let x = 0; x < w; x++) {
    const v = Math.round(x / (w - 1) * 100)
    const [r, g, b] = hsvToRgb(hue, sat, v)
    for (let y = 0; y < h; y++) {
      const idx = (y * w + x) * 4
      d[idx] = r
      d[idx + 1] = g
      d[idx + 2] = b
      d[idx + 3] = 255
    }
  }
  ctx.putImageData(img, 0, 0)
}

function setPickerColor(hex) {
  const [r, g, b] = hexToRgb(hex)
  const [h, s, v] = rgbToHsv(r, g, b)
  pickRgb.value = { r, g, b }
  pickHsv.value = { h: Math.round(h), s: Math.round(s), v: Math.round(v) }
  pickHex.value = rgbToHex([r, g, b])
  hexDraft.value = pickHex.value.slice(1)
  writePickerColor()
}

function writePickerColor() {
  if (!pickerOpen.value) return
  if (pickTarget.value === 'fg') fg.value = pickHex.value
  else bg.value = pickHex.value
}

function openColorPicker(target) {
  pickTarget.value = target
  moreOpen.value = false
  pickerOpen.value = true
  setPickerColor(target === 'fg' ? fg.value : bg.value)
  nextTick(() => { drawColorWheel(); drawShades(); drawBrightBar() })
}

function closeColorPicker() {
  pickerOpen.value = false
  wheelDragging = false
  shadesDragging = false
  brightDragging = false
}

function syncFromHsv() {
  const [r, g, b] = hsvToRgb(pickHsv.value.h, pickHsv.value.s, pickHsv.value.v)
  pickRgb.value = { r, g, b }
  pickHex.value = rgbToHex([r, g, b])
  hexDraft.value = pickHex.value.slice(1)
  writePickerColor()
}

function onHueInput(e) {
  pickHsv.value.h = Math.max(0, Math.min(360, Number(e.target.value) || 0))
  syncFromHsv()
}

function onSatInput(e) {
  pickHsv.value.s = Math.max(0, Math.min(100, Number(e.target.value) || 0))
  syncFromHsv()
}

function onValueInput(e) {
  pickHsv.value.v = Math.max(0, Math.min(100, Number(e.target.value) || 0))
  syncFromHsv()
}

function onRgbInput(e, ch) {
  pickRgb.value[ch] = Math.max(0, Math.min(255, Number(e.target.value) || 0))
  const { r, g, b } = pickRgb.value
  const [h, s, v] = rgbToHsv(r, g, b)
  pickHsv.value = { h: Math.round(h), s: Math.round(s), v: Math.round(v) }
  pickHex.value = rgbToHex([r, g, b])
  hexDraft.value = pickHex.value.slice(1)
  writePickerColor()
}

function onHexInput(e) {
  const raw = e.target.value.toUpperCase().replace(/[^0-9A-F]/g, '').slice(0, 6)
  hexDraft.value = raw
  if (raw.length === 6) setPickerColor('#' + raw)
}

function toggleMore() {
  moreOpen.value = !moreOpen.value
}

function onPalettePick(hex) {
  setPickerColor(hex)
}

function onPalettePickOther(hex) {
  pickTarget.value = pickTarget.value === 'fg' ? 'bg' : 'fg'
  setPickerColor(hex)
}

function onWheelPointerDown(e) {
  wheelDragging = true
  wheelEl.value?.setPointerCapture(e.pointerId)
  updateWheelFromPointer(e)
  e.preventDefault()
}

function onWheelPointerMove(e) {
  if (!wheelDragging) return
  updateWheelFromPointer(e)
}

function onWheelPointerUp(e) {
  if (wheelDragging) wheelEl.value?.releasePointerCapture?.(e.pointerId)
  wheelDragging = false
}

function updateWheelFromPointer(e) {
  const el = wheelEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const cx = rect.left + rect.width / 2
  const cy = rect.top + rect.height / 2
  let dx = e.clientX - cx
  let dy = e.clientY - cy
  const dist = Math.hypot(dx, dy)
  const max = rect.width / 2
  if (dist > max) {
    dx = dx / dist * max
    dy = dy / dist * max
  }
  const hue = (Math.atan2(dy, dx) * 180 / Math.PI + 360) % 360
  const sat = Math.min(100, Math.round(dist / max * 100))
  pickHsv.value.h = Math.round(hue)
  pickHsv.value.s = sat
  syncFromHsv()
}

function onBrightPointerDown(e) {
  brightDragging = true
  brightBarEl.value?.setPointerCapture(e.pointerId)
  updateBrightFromPointer(e)
  e.preventDefault()
}

function onBrightPointerMove(e) {
  if (!brightDragging) return
  updateBrightFromPointer(e)
}

function onBrightPointerUp(e) {
  if (brightDragging) brightBarEl.value?.releasePointerCapture?.(e.pointerId)
  brightDragging = false
}

function updateBrightFromPointer(e) {
  const el = brightBarEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const x = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  pickHsv.value.v = Math.round(x * 100)
  syncFromHsv()
}

const shadesEl = ref(null)
const shadesCanvasEl = ref(null)
let shadesDragging = false

function drawShades() {
  const c = shadesCanvasEl.value
  if (!c) return
  const hue = pickHsv.value.h
  const w = c.width
  const h = c.height
  const ctx = c.getContext('2d')
  const img = ctx.createImageData(w, h)
  const d = img.data
  for (let y = 0; y < h; y++) {
    const v = Math.round(100 - (y / (h - 1)) * 100)
    for (let x = 0; x < w; x++) {
      const s = Math.round((x / (w - 1)) * 100)
      const [r, g, b] = hsvToRgb(hue, s, v)
      const idx = (y * w + x) * 4
      d[idx] = r
      d[idx + 1] = g
      d[idx + 2] = b
      d[idx + 3] = 255
    }
  }
  ctx.putImageData(img, 0, 0)
}

function onShadesPointerDown(e) {
  shadesDragging = true
  shadesEl.value?.setPointerCapture(e.pointerId)
  updateShadesFromPointer(e)
  e.preventDefault()
}

function onShadesPointerMove(e) {
  if (!shadesDragging) return
  updateShadesFromPointer(e)
}

function onShadesPointerUp(e) {
  if (shadesDragging) shadesEl.value?.releasePointerCapture?.(e.pointerId)
  shadesDragging = false
}

function updateShadesFromPointer(e) {
  const el = shadesEl.value
  if (!el) return
  const rect = el.getBoundingClientRect()
  const x = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width))
  const y = Math.max(0, Math.min(1, (e.clientY - rect.top) / rect.height))
  pickHsv.value.s = Math.round(x * 100)
  pickHsv.value.v = Math.round(100 - y * 100)
  syncFromHsv()
}

watch([() => pickHsv.value.h, () => pickHsv.value.s], () => {
  if (pickerOpen.value) { drawShades(); drawBrightBar() }
})

function renderOverlay() {
  const octx = overlayEl.value && overlayEl.value.getContext('2d')
  if (!octx) return
  const w = working.value.w
  const h = working.value.h
  const mask = selMask.value
  if (mask !== lastMask) {
    lastMask = mask
    tintBuffer = new Uint8ClampedArray(w * h * 4)
    tintCanvas = null
    outlineSegs = []
    if (mask) {
      for (let i = 0; i < mask.length; i++) {
        if (!mask[i]) continue
        const idx = i * 4
        tintBuffer[idx] = 60
        tintBuffer[idx + 1] = 140
        tintBuffer[idx + 2] = 255
        tintBuffer[idx + 3] = 16
        const px = i % w
        const py = (i / w) | 0
        if (py === 0 || !mask[i - w]) outlineSegs.push([px, py, px + 1, py])
        if (py === h - 1 || !mask[i + w]) outlineSegs.push([px, py + 1, px + 1, py + 1])
        if (px === 0 || !mask[i - 1]) outlineSegs.push([px, py, px, py + 1])
        if (px === w - 1 || !mask[i + 1]) outlineSegs.push([px + 1, py, px + 1, py + 1])
      }
    }
  }
  const dpr = window.devicePixelRatio || 1
  const cssW = Math.floor(w * zoom.value * dpr)
  const cssH = Math.floor(h * zoom.value * dpr)
  if (overlayEl.value.width !== cssW) overlayEl.value.width = cssW
  if (overlayEl.value.height !== cssH) overlayEl.value.height = cssH
  octx.setTransform(1, 0, 0, 1, 0, 0)
  octx.clearRect(0, 0, cssW, cssH)
  if (mask && tintBuffer) {
    if (!tintCanvas || tintCanvas.width !== w || tintCanvas.height !== h) {
      tintCanvas = new OffscreenCanvas(w, h)
      tintCanvas.getContext('2d').putImageData(new ImageData(tintBuffer, w, h), 0, 0)
    }
    octx.imageSmoothingEnabled = false
    octx.setTransform(zoom.value * dpr, 0, 0, zoom.value * dpr, 0, 0)
    octx.drawImage(tintCanvas, 0, 0)
    if (outlineSegs.length) {
      octx.lineWidth = 1 / zoom.value
      octx.strokeStyle = '#4da6ff'
      octx.setLineDash([3 / zoom.value, 3 / zoom.value])
      octx.lineDashOffset = 0
      octx.beginPath()
      for (let s = 0; s < outlineSegs.length; s++) {
        const seg = outlineSegs[s]
        octx.moveTo(seg[0], seg[1])
        octx.lineTo(seg[2], seg[3])
      }
      octx.stroke()
      octx.setLineDash([])
    }
    octx.setTransform(1, 0, 0, 1, 0, 0)
  }
  if (working.value.w && hover.value && (tool.value === 'brush' || tool.value === 'eraser' || tool.value === 'select')) {
    octx.setTransform(zoom.value * dpr, 0, 0, zoom.value * dpr, 0, 0)
    octx.lineWidth = 1 / zoom.value
    if (brushSize.value > 1) {
      const r = Math.max(1, brushSize.value / 2 - 1)
      octx.strokeStyle = 'rgba(0,0,0,0.55)'
      octx.beginPath()
      octx.arc(hover.value.x, hover.value.y, r, 0, Math.PI * 2)
      octx.stroke()
      octx.strokeStyle = 'rgba(255,255,255,0.9)'
      octx.beginPath()
      octx.arc(hover.value.x + 1 / zoom.value, hover.value.y + 1 / zoom.value, r, 0, Math.PI * 2)
      octx.stroke()
    } else {
      const len = 4 / zoom.value
      const o = 1 / zoom.value
      octx.beginPath()
      octx.moveTo(hover.value.x - len, hover.value.y)
      octx.lineTo(hover.value.x + len, hover.value.y)
      octx.moveTo(hover.value.x, hover.value.y - len)
      octx.lineTo(hover.value.x, hover.value.y + len)
      octx.strokeStyle = 'rgba(0,0,0,0.55)'
      octx.stroke()
      octx.beginPath()
      octx.moveTo(hover.value.x - len + o, hover.value.y + o)
      octx.lineTo(hover.value.x + len + o, hover.value.y + o)
      octx.moveTo(hover.value.x + o, hover.value.y - len + o)
      octx.lineTo(hover.value.x + o, hover.value.y + len + o)
      octx.strokeStyle = 'rgba(255,255,255,0.9)'
      octx.stroke()
    }
    octx.setTransform(1, 0, 0, 1, 0, 0)
  }
  if (selDrag.value && tool.value !== 'select') {
    octx.setTransform(zoom.value * dpr, 0, 0, zoom.value * dpr, 0, 0)
    const g = selDrag.value
    octx.strokeStyle = '#4da6ff'
    octx.lineWidth = 1 / zoom.value
    octx.setLineDash([3 / zoom.value, 3 / zoom.value])
    octx.lineDashOffset = 0
    octx.beginPath()
    octx.moveTo(g.x0 + 0.5, g.y0 + 0.5)
    octx.lineTo(g.x1 + 0.5, g.y1 + 0.5)
    octx.stroke()
    octx.setLineDash([])
    octx.setTransform(1, 0, 0, 1, 0, 0)
  }
  if (floatPaste.value && working.value.w) {
    const fp = floatPaste.value
    const sz = FLOAT_HANDLE / 2 / zoom.value
    const lw = 1 / zoom.value
    octx.setTransform(zoom.value * dpr, 0, 0, zoom.value * dpr, 0, 0)
    octx.imageSmoothingEnabled = false
    octx.globalAlpha = 0.8
    octx.drawImage(fp.src, fp.x, fp.y, fp.w, fp.h)
    octx.globalAlpha = 1
    octx.lineWidth = lw
    octx.strokeStyle = 'rgba(255,255,255,0.95)'
    octx.setLineDash([3 / zoom.value, 3 / zoom.value])
    octx.strokeRect(fp.x, fp.y, fp.w, fp.h)
    octx.strokeStyle = 'rgba(0,0,0,0.55)'
    octx.strokeRect(fp.x + lw, fp.y + lw, fp.w, fp.h)
    octx.setLineDash([])
    const hs = [
      [fp.x, fp.y], [fp.x + fp.w, fp.y],
      [fp.x, fp.y + fp.h], [fp.x + fp.w, fp.y + fp.h]
    ]
    for (const [hx, hy] of hs) {
      octx.fillStyle = 'rgba(255,255,255,0.95)'
      octx.fillRect(hx - sz, hy - sz, sz * 2, sz * 2)
      octx.lineWidth = lw
      octx.strokeStyle = 'rgba(0,0,0,0.55)'
      octx.strokeRect(hx - sz, hy - sz, sz * 2, sz * 2)
    }
    octx.setTransform(1, 0, 0, 1, 0, 0)
  }
}

// ---------- layers ----------
function makeLayer(name) {
  const base = working.value
  const off = document.createElement('canvas')
  off.width = base.w
  off.height = base.h
  const data = off.getContext('2d').getImageData(0, 0, base.w, base.h)
  return {
    id: nextLayerId++,
    name,
    visible: true,
    opacity: 100,
    blend: 'normal',
    image: markRaw(data)
  }
}

function addLayer() {
  if (!layers.value.length) return
  commit()
  const layer = makeLayer('Layer ' + (layers.value.length + 1))
  layers.value.push(layer)
  activeLayerId.value = layer.id
  recomposite()
  renderOverlay()
}

function duplicateLayer() {
  const layer = activeLayer()
  if (!layer) return
  commit()
  const copy = {
    id: nextLayerId++,
    name: layer.name + ' copy',
    visible: true,
    opacity: layer.opacity,
    blend: layer.blend || 'normal',
    image: markRaw(cloneImageData(layer.image))
  }
  const idx = layers.value.indexOf(layer)
  layers.value.splice(idx + 1, 0, copy)
  activeLayerId.value = copy.id
  recomposite()
  renderOverlay()
}

function moveLayer(dir) {
  const idx = layers.value.findIndex(l => l.id === activeLayerId.value)
  if (idx === -1) return
  const to = idx + dir
  if (to < 0 || to >= layers.value.length) return
  commit()
  const [l] = layers.value.splice(idx, 1)
  layers.value.splice(to, 0, l)
  recomposite()
  renderOverlay()
  scrollActiveLayerIntoView()
}

function scrollActiveLayerIntoView() {
  nextTick(() => {
    const list = document.querySelector('.re-layer-list')
    const active = list?.querySelector('.re-layer.active')
    active?.scrollIntoView({ block: 'nearest' })
  })
}

function mergeDown() {
  const idx = layers.value.findIndex(l => l.id === activeLayerId.value)
  if (idx <= 0 || idx === -1) return
  const top = layers.value[idx]
  const bot = layers.value[idx - 1]
  commit()
  const w = working.value.w
  const h = working.value.h
  const tmp = document.createElement('canvas')
  tmp.width = w
  tmp.height = h
  const tc = tmp.getContext('2d')
  tc.imageSmoothingEnabled = false
  tc.globalCompositeOperation = 'source-over'
  if (bot.visible && bot.image) {
    tc.globalAlpha = bot.opacity / 100
    tc.globalCompositeOperation = BLEND_COMPOSITE_OPS[bot.blend] || 'source-over'
    tc.drawImage(getLayerCanvas(bot), 0, 0)
  }
  if (top.visible && top.image) {
    tc.globalAlpha = top.opacity / 100
    tc.globalCompositeOperation = BLEND_COMPOSITE_OPS[top.blend] || 'source-over'
    tc.drawImage(getLayerCanvas(top), 0, 0)
  }
  tc.globalAlpha = 1
  tc.globalCompositeOperation = 'source-over'
  bot.image = markRaw(tc.getImageData(0, 0, w, h))
  bumpLayerVersion(bot)
  bot.opacity = 100
  bot.blend = 'normal'
  layers.value.splice(idx, 1)
  activeLayerId.value = bot.id
  recomposite()
  renderOverlay()
}

function flatten() {
  if (layers.value.length <= 1) return
  commit()
  const w = working.value.w
  const h = working.value.h
  const c = document.createElement('canvas')
  c.width = w
  c.height = h
  const ctx = c.getContext('2d')
  ctx.clearRect(0, 0, w, h)
  ctx.globalCompositeOperation = 'source-over'
  for (const layer of layers.value) {
    if (!layer.visible || !layer.image) continue
    ctx.globalAlpha = layer.opacity / 100
    ctx.globalCompositeOperation = BLEND_COMPOSITE_OPS[layer.blend] || 'source-over'
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(getLayerCanvas(layer), 0, 0)
  }
  ctx.globalAlpha = 1
  ctx.globalCompositeOperation = 'source-over'
  const flat = {
    id: nextLayerId++,
    name: 'Flattened',
    visible: true,
    opacity: 100,
    blend: 'normal',
    image: markRaw(ctx.getImageData(0, 0, w, h))
  }
  layers.value = [flat]
  activeLayerId.value = flat.id
  recomposite()
  renderOverlay()
}

function deleteLayer() {
  if (layers.value.length <= 1) return
  const idx = layers.value.findIndex(l => l.id === activeLayerId.value)
  if (idx === -1) return
  commit()
  layers.value.splice(idx, 1)
  activeLayerId.value = layers.value[Math.max(0, Math.min(idx, layers.value.length - 1))].id
  recomposite()
  renderOverlay()
}

function toggleLayer(id) {
  const layer = layers.value.find(l => l.id === id)
  if (!layer) return
  layer.visible = !layer.visible
  recomposite()
  renderOverlay()
}

function selectLayer(id) {
  activeLayerId.value = id
  renderOverlay()
  scrollActiveLayerIntoView()
}

function setLayerOpacity(id, val) {
  const layer = layers.value.find(l => l.id === id)
  if (!layer) return
  layer.opacity = val
  recomposite()
  renderOverlay()
}

function setLayerBlend(id, val) {
  const layer = layers.value.find(l => l.id === id)
  if (!layer) return
  layer.blend = val
  recomposite()
  renderOverlay()
}

// ---------- HSV tool ----------
function shiftWithMask(src, mask) {
  const out = applyHsvShift(cloneImageData(src), hue.value, sat.value, bright.value)
  if (mask) {
    for (let i = 0; i < mask.length; i++) {
      if (!mask[i]) out.data.set(src.data.subarray(i * 4, i * 4 + 4), i * 4)
    }
  }
  return out
}

function applyHsv() {
  const zero = hue.value === 0 && sat.value === 0 && bright.value === 0
  if (hsvBase) {
    putImageData(hsvBase)
    hsvBase = null
  }
  if (zero) return
  const src = getImageData()
  if (!src) return
  commit()
  putImageData(shiftWithMask(src, activeMask()))
  actualizeDirty()
}

function previewHsv() {
  if (!currentTex.value || !hsvOpen.value) return
  const zero = hue.value === 0 && sat.value === 0 && bright.value === 0
  if (zero) {
    if (hsvBase) {
      putImageData(hsvBase)
      hsvBase = null
    }
    return
  }
  if (!hsvBase) {
    hsvBase = getImageData()
  }
  if (!hsvBase) return
  putImageData(shiftWithMask(hsvBase, activeMask()))
}

watch([hue, sat, bright], previewHsv)

function cancelHsv() {
  if (hsvBase) {
    putImageData(hsvBase)
    hsvBase = null
    actualizeDirty()
  }
}

function hsvReset() {
  cancelHsv()
  hue.value = 0
  sat.value = 0
  bright.value = 0
}

function openHsv() {
  if (!currentTex.value || !ready.value) return
  cancelHsv()
  saveMenuOpen.value = false
  hsvOpen.value = true
  previewHsv()
}

function hsvExitApply() {
  applyHsv()
  hsvOpen.value = false
}

function hsvExitCancel() {
  cancelHsv()
  hsvOpen.value = false
}

function applySepiaFilter() {
  if (!currentTex.value || !ready.value) return
  const src = getImageData()
  if (!src) return
  commit()
  const out = applySepia(cloneImageData(src))
  const mask = activeMask()
  if (mask) {
    for (let i = 0; i < mask.length; i++) {
      if (!mask[i]) out.data.set(src.data.subarray(i * 4, i * 4 + 4), i * 4)
    }
  }
  putImageData(out)
  actualizeDirty()
}

// ---------- resize ----------
function openResize() {
  resizeW.value = working.value.w
  resizeH.value = working.value.h
  resizeRatio.value = working.value.w / working.value.h
  resizing.value = true
}

async function applyResize() {
  const w = Math.max(1, Math.round(resizeW.value))
  const h = Math.max(1, Math.round(resizeH.value))
  if (w === working.value.w && h === working.value.h) {
    resizing.value = false
    return
  }
  commit()
  for (const layer of layers.value) {
    if (!layer.image) continue
    const canvas = resizeNearestNeighbor(layer.image, w, h)
    layer.image = markRaw(canvas.getContext('2d').getImageData(0, 0, w, h))
    bumpLayerVersion(layer)
  }
  canvasEl.value.width = w
  canvasEl.value.height = h
  recomposite()
  working.value = { w, h }
  syncCanvasSizes()
  fitZoom()
  markDirty()
  selMask.value = null
  lastMask = null
  hsvBase = null
  resizing.value = false
}

// ---------- save ----------
function saveBase64() {
  return canvasToDataURL(canvasEl.value).split(',')[1]
}

async function refreshThumb(relPath) {
  const c = canvasEl.value
  if (!c) return
  try {
    const th = await toThumb(canvasToDataURL(c))
    if (th) thumbs.set(relPath, th)
  } catch (_) {}
}

async function writeTexture(relPath, filename, base64) {
  if (isInstalled.value) {
    await SavePackTexture(props.source.dirName, relPath, base64, packBasePath.value)
  } else {
    await SaveImage(sessionId.value, { imageName: filename, imagePath: '', relPath, imageData: base64, done: false })
  }
}

// ---------- autosave ----------
// Edits are written to the working copy shortly after the user stops editing,
// so switching tools, closing the window or crashing cannot lose work.
const AUTOSAVE_DELAY = 1500
const autosaveOn = ref(true)
let autosaveTimer = null
let autosaveBusy = false

function loadAutosaveSetting() {
  GetSettings()
    .then(s => { autosaveOn.value = s?.recolorAutosave !== false })
    .catch(() => { autosaveOn.value = true })
}

function scheduleAutosave() {
  if (!autosaveOn.value) return
  if (!currentTex.value || !dirty.value) return
  if (autosaveTimer) clearTimeout(autosaveTimer)
  autosaveTimer = setTimeout(() => {
    autosaveTimer = null
    flushAutosave()
  }, AUTOSAVE_DELAY)
}

function cancelAutosave() {
  if (autosaveTimer) {
    clearTimeout(autosaveTimer)
    autosaveTimer = null
  }
}

// Writes the current canvas if it differs from what is on disk. Never touches
// `busy` (autosave must not block the UI) and stays quiet unless it fails.
// Resolves to false only when the write actually failed, so callers can treat
// true as "safe to move on".
async function flushAutosave(force = false) {
  cancelAutosave()
  if (!force && !autosaveOn.value) return true
  if (autosaveBusy) return true // a write is already in flight
  if (!currentTex.value || !ready.value || !dirty.value) return true

  const relPath = currentTex.value.relPath
  const filename = currentTex.value.filename
  const snapshot = currentImageData()
  if (!snapshot) return true

  autosaveBusy = true
  try {
    await writeTexture(relPath, filename, saveBase64())
    // only treat the texture as clean if nothing changed while we were writing
    if (currentTex.value && currentTex.value.relPath === relPath) {
      if (equalsImageData(currentImageData(), snapshot)) {
        savedImage = snapshot
        dirty.value = false
      } else {
        scheduleAutosave()
      }
    }
    return true
  } catch (e) {
    popup('Autosave failed', 'Your last edit could not be written to disk: ' + e.toString(), 'error')
    return false
  } finally {
    autosaveBusy = false
  }
}

function markDirty() {
  dirty.value = true
  scheduleAutosave()
}

async function saveReplace() {
  if (!currentTex.value || busy.value) return
  busy.value = true
  try {
    await writeTexture(currentTex.value.relPath, currentTex.value.filename, saveBase64())
    cancelAutosave()
    dirty.value = false
    savedImage = currentImageData()
    await refreshThumb(currentTex.value.relPath)
    popup('Saved', `Replaced ${currentTex.value.filename}`, 'success')
  } catch (e) {
    popup('Error', 'Save failed: ' + e.toString(), 'error')
  }
  busy.value = false
}

async function saveAs() {
  if (!currentTex.value || busy.value) return
  const folder = currentTex.value.folder || ''
  const baseName = currentTex.value.filename.replace(/\.[^.]+$/, '')
  const { value: input } = await Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn', cancelButton: 'custom-cancel-btn' },
    buttonsStyling: false
  }).fire({
    title: 'Save As',
    html: `
      <div style="display:flex;flex-direction:column;gap:8px;text-align:left;">
        <label style="font-size:12px;color:var(--text-dim);">Folder (relative to pack, e.g. textures/blocks)</label>
        <input id="swal-folder" class="swal-input" style="padding:8px;border-radius:6px;border:1px solid var(--border-medium);background:var(--bg-input);color:var(--text-primary);" value="${folder}" />
        <label style="font-size:12px;color:var(--text-dim);">File name</label>
        <input id="swal-name" class="swal-input" style="padding:8px;border-radius:6px;border:1px solid var(--border-medium);background:var(--bg-input);color:var(--text-primary);" value="${baseName}-copy" />
      </div>`,
    icon: 'question',
    showCancelButton: true,
    confirmButtonText: 'Save',
    cancelButtonText: 'Cancel',
    preConfirm: () => {
      const folderVal = document.getElementById('swal-folder').value.trim()
      const nameVal = document.getElementById('swal-name').value.trim()
      return { folder: folderVal, name: nameVal }
    }
  })
  if (!input) return
  let name = input.name || (baseName + '-copy')
  name = name.replace(/\.[^.]+$/, '') + '.png'
  let relPath = name
  if (input.folder) relPath = input.folder.replace(/^\/+/, '').replace(/\/+$/, '') + '/' + name

  if (isInstalled.value) {
    try {
      const existing = await ListPackTextures(props.source.dirName)
      if (existing.some(t => t.relPath === relPath)) {
        const confirm = await Swal.mixin({
          customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn', cancelButton: 'custom-cancel-btn' },
          buttonsStyling: false
        }).fire({
          title: 'Overwrite?',
          text: `A texture already exists at ${relPath}. Replace it?`,
          icon: 'warning',
          showCancelButton: true,
          confirmButtonText: 'Overwrite',
          cancelButtonText: 'Cancel'
        })
        if (!confirm.isConfirmed) return
      }
    } catch (_) {}
  }

  busy.value = true
  try {
    await writeTexture(relPath, name, saveBase64())
    if (isInstalled.value) {
      await selectFolder(activeFolder.value)
      const found = textures.value.find(t => t.relPath === relPath)
      if (found) await openTexture(found)
      else await openTexture({ ...currentTex.value, relPath, filename: name, folder: relPath.split('/').slice(0, -1).join('/') || '' })
    }
    cancelAutosave()
    savedImage = currentImageData()
    dirty.value = false
    if (!isInstalled.value) await selectFolder(activeFolder.value)
    await refreshThumb(relPath)
    popup('Saved', `Saved as ${relPath}`, 'success')
  } catch (e) {
    popup('Error', 'Save As failed: ' + e.toString(), 'error')
  }
  busy.value = false
}

async function exportUploaded() {
  if (busy.value) return
  busy.value = true
  try {
    // make sure the export contains the pixels currently on screen
    await flushAutosave(true)
    const result = await ExportPack(sessionId.value, props.source.packName || '')
    // the work is now in the .mcpack, so the working copy can go
    if (sessionId.value) { try { await DeleteSession(sessionId.value) } catch (_) {} }
    clearSession(props.source)
    cancelAutosave()
    popup('Exported!', result, 'success')
    emit('close')
  } catch (e) {
    popup('Error', 'Export failed: ' + e.toString(), 'error')
  }
  busy.value = false
}

function dirtyTabCount() {
  return openTabs.value.filter(t => t.dirty).length
}

// Returns 'save', 'skip' or 'cancel'.
async function askAboutPending(count) {
  const choice = await Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn', cancelButton: 'custom-cancel-btn', denyButton: 'custom-cancel-btn' },
    buttonsStyling: false
  }).fire({
    title: 'Unsaved changes',
    text: `${count} texture${count === 1 ? ' has' : 's have'} edits that aren't written to disk yet.`,
    icon: 'warning',
    showCancelButton: true,
    showDenyButton: true,
    confirmButtonText: 'Save and close',
    denyButtonText: 'Close without saving',
    cancelButtonText: 'Cancel'
  })
  if (choice.isConfirmed) return 'save'
  if (choice.isDenied) return 'skip'
  return 'cancel'
}

async function handleClose() {
  const pending = dirtyTabCount()
  let save = true
  // with autosave on, every tab is already on disk by the time it is left
  if (pending > 0 && !autosaveOn.value) {
    const choice = await askAboutPending(pending)
    if (choice === 'cancel') return
    save = choice === 'save'
  }
  if (save && currentTex.value && dirty.value) await flushAutosave(true)
  cancelAutosave()
  // deliberately keep the working copy: leaving the editor must not destroy work
  emit('close')
}

async function openInExplorer() {
  if (!isInstalled.value || busy.value) return
  try {
    const root = await GetPackRoot(props.source.dirName, packBasePath.value)
    if (root) OpenFolder(root)
  } catch (e) {
    popup('Error', 'Failed to open folder: ' + e.toString(), 'error')
  }
}

function onPointerLeave() {
  hover.value = null
  renderOverlay()
  if (pointer.value.down) onPointerUp()
}

function onPointerCancel() {
  if (pointer.value.down) onPointerUp()
}

function onWheel(e) {
  const vp = viewportEl.value
  const canvas = canvasEl.value
  if (!vp || !canvas) return
  const unit = e.deltaMode === 1 ? 15 : 1
  if (e.ctrlKey) {
    e.preventDefault()
    const canRect = canvas.getBoundingClientRect()
    const mx = e.clientX - canRect.left
    const my = e.clientY - canRect.top
    const cx = (vp.scrollLeft + mx) / zoom.value
    const cy = (vp.scrollTop + my) / zoom.value
    const factor = e.deltaY < 0 ? 1.1 : 1 / 1.1
    const next = Math.max(0.05, Math.min(64, zoom.value * factor))
    if (next === zoom.value) return
    zoom.value = next
    vp.scrollLeft = cx * zoom.value - mx
    vp.scrollTop = cy * zoom.value - my
  } else if (e.shiftKey) {
    if (vp.scrollWidth <= vp.clientWidth) return
    e.preventDefault()
    vp.scrollLeft += e.deltaY * unit
  } else {
    if (vp.scrollHeight <= vp.clientHeight) return
    e.preventDefault()
    vp.scrollTop += e.deltaY * unit
  }
}

function formatBytes(n) {
  if (!n) return ''
  return (n / 1024).toFixed(1) + ' KB'
}

const canUndo = computed(() => undoStack.value.length > 0)
const canRedo = computed(() => redoStack.value.length > 0)
const hasSelection = computed(() => !!selMask.value)
const revLayers = computed(() => [...layers.value].reverse())
const activeIdx = computed(() => layers.value.findIndex(l => l.id === activeLayerId.value))
const canMoveUp = computed(() => activeIdx.value >= 0 && activeIdx.value < layers.value.length - 1)
const canMoveDown = computed(() => activeIdx.value > 0)
const canDelete = computed(() => layers.value.length > 1)
const canMergeDown = computed(() => activeIdx.value > 0)
const canFlatten = computed(() => layers.value.length > 1)

const tools = [
  { id: 'brush', icon: 'fa-paintbrush', label: 'Brush (B)' },
  { id: 'eraser', icon: 'fa-eraser', label: 'Eraser (E)' },
  { id: 'bucket', icon: 'fa-fill-drip', label: 'Paint Bucket (G)' },
  { id: 'gradient', icon: 'fa-blender', label: 'Gradient (L)' },
  { id: 'wand', icon: 'fa-wand-magic-sparkles', label: 'Magic Wand (W)' },
  { id: 'select', icon: 'fa-vector-square', label: 'Select (S)' },
  { id: 'picker', icon: 'fa-eye-dropper', label: 'Color Picker (I)' }
]

const adjustTools = [
  { id: 'resize', icon: 'fa-expand', label: 'Resize' },
  { id: 'hsv', icon: 'fa-palette', label: 'Adjust Color (Ctrl+Shift+U)' },
  { id: 'sepia', icon: 'fa-filter', label: 'Sepia (Ctrl+Shift+I)' }
]

function onAdjustTool(id) {
  if (id === 'resize') {
    tool.value = id
    openResize()
  } else if (id === 'hsv') {
    openHsv()
  } else if (id === 'sepia') {
    applySepiaFilter()
  }
}

function isFormFieldTarget(t) {
  if (!t) return false
  const tag = t.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || !!t.isContentEditable
}

function onKey(e) {
  // the editor stays mounted while another tool is open (v-show), so its
  // shortcuts must not hijack the page the user is actually on
  if (!props.visible) return
  if (pickerOpen.value && e.key === 'Escape') {
    e.preventDefault()
    closeColorPicker()
    return
  }
  if (pickerOpen.value) return
  if (hsvOpen.value && (e.key === 'Enter' || e.key === 'Escape')) {
    e.preventDefault()
    if (e.key === 'Enter') hsvExitApply()
    else hsvExitCancel()
    return
  }
  if (floatPaste.value) {
    if (e.key === 'Enter') {
      e.preventDefault()
      commitFloatingPaste()
      return
    }
    if (e.key === 'Escape') {
      e.preventDefault()
      clearFloatingPaste()
      return
    }
  }
  if (e.key === 'Escape') {
    clearSelection()
    saveMenuOpen.value = false
    return
  }
  if (e.key === 'Alt') {
    e.preventDefault()
    return
  }
  if (isFormFieldTarget(e.target)) return
  const mod = e.ctrlKey || e.metaKey
  const k = e.key.toLowerCase()
  if (mod && k === 'z') {
    e.preventDefault()
    if (e.shiftKey) redo()
    else undo()
    return
  }
  if (mod && k === 'y') {
    e.preventDefault()
    redo()
    return
  }
  if (mod && k === 's') {
    e.preventDefault()
    if (e.shiftKey) saveAs()
    else saveReplace()
    return
  }
  if (mod && k === 'a') {
    e.preventDefault()
    selectAll()
    return
  }
  if (mod && k === 'c') {
    e.preventDefault()
    copyPixels()
    return
  }
  if (mod && k === 'x') {
    e.preventDefault()
    cutPixels()
    return
  }
  if (mod && k === 'v') {
    if (editorClipboard.value) {
      e.preventDefault()
      pasteEditorClipboard()
    }
    return
  }
  if (e.key === 'Backspace' || e.key === 'Delete') {
    e.preventDefault()
    deleteSelection()
    return
  }
  if (mod && k === 'u' && e.shiftKey) {
    e.preventDefault()
    openHsv()
    return
  }
  if (mod && k === 'i' && e.shiftKey) {
    e.preventDefault()
    applySepiaFilter()
    return
  }
  const map = { b: 'brush', e: 'eraser', g: 'bucket', l: 'gradient', w: 'wand', s: 'select', i: 'picker' }
  if (k === 'c') {
    centerImage()
    return
  }
  if (k === 'x') {
    swapColors()
    return
  }
  if (map[k]) {
    tool.value = map[k]
    saveMenuOpen.value = false
  }
}

function onPaste(e) {
  if (!props.visible) return
  if (isFormFieldTarget(e.target)) return
  const items = e.clipboardData && e.clipboardData.items
  let hasImage = false
  if (items) {
    for (const it of items) {
      if (it.type && it.type.indexOf('image/') === 0) {
        hasImage = true
        const file = it.getAsFile()
        if (file) {
          e.preventDefault()
          pasteImageFile(file)
          return
        }
      }
    }
  }
  if (!hasImage && editorClipboard.value) {
    e.preventDefault()
    pasteEditorClipboard()
  }
}

async function pasteImageFile(file) {
  try {
    const url = URL.createObjectURL(file)
    const img = await loadImage(url)
    URL.revokeObjectURL(url)
    if (!img.naturalWidth || !img.naturalHeight) throw new Error('empty image')
    if (!currentTex.value || !ready.value) {
      pasteAsNewTab(img)
      return
    }
    const w = working.value.w
    const h = working.value.h
    if (img.naturalWidth > w || img.naturalHeight > h) {
      const r = await Swal.mixin({
        customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn', cancelButton: 'custom-cancel-btn' },
        buttonsStyling: false
      }).fire({
        title: 'Resize canvas to fit?',
        text: `Pasted image is ${img.naturalWidth}×${img.naturalHeight}, but the canvas is ${w}×${h}. Resize the canvas and paste?`,
        icon: 'question',
        showCancelButton: true,
        confirmButtonText: 'Resize & paste',
        cancelButtonText: 'Paste as-is'
      })
      if (r.isConfirmed) resizeCanvasTo(img.naturalWidth, img.naturalHeight)
    }
    commit()
    const cw = working.value.w
    const ch = working.value.h
    const layer = makeLayer('Pasted')
    const tmp = document.createElement('canvas')
    tmp.width = cw
    tmp.height = ch
    const tc = tmp.getContext('2d')
    tc.imageSmoothingEnabled = false
    tc.drawImage(img, 0, 0)
    layer.image = markRaw(tc.getImageData(0, 0, cw, ch))
    layers.value.push(layer)
    activeLayerId.value = layer.id
    recomposite()
    renderOverlay()
    markDirty()
  } catch (e) {
    popup('Error', 'Could not use pasted image: ' + e.toString(), 'error')
  }
}

function resizeCanvasTo(w2, h2) {
  w2 = Math.max(1, Math.round(w2))
  h2 = Math.max(1, Math.round(h2))
  if (w2 === working.value.w && h2 === working.value.h) return
  commit()
  for (const layer of layers.value) {
    if (!layer.image) continue
    const old = layer.image
    if (old.width > w2 || old.height > h2) {
      const canvas = resizeNearestNeighbor(old, w2, h2)
      layer.image = markRaw(canvas.getContext('2d').getImageData(0, 0, w2, h2))
    } else {
      const out = new ImageData(w2, h2)
      for (let y = 0; y < old.height; y++) {
        out.data.set(old.data.subarray(y * old.width * 4, (y + 1) * old.width * 4), (y * w2) * 4)
      }
      layer.image = markRaw(out)
    }
    bumpLayerVersion(layer)
  }
  canvasEl.value.width = w2
  canvasEl.value.height = h2
  recomposite()
  working.value = { w: w2, h: h2 }
  syncCanvasSizes()
  fitZoom()
  markDirty()
  selMask.value = null
  lastMask = null
  hsvBase = null
}

function pasteAsNewTab(img) {
  const c = document.createElement('canvas')
  c.width = img.naturalWidth
  c.height = img.naturalHeight
  const ctx = c.getContext('2d')
  ctx.imageSmoothingEnabled = false
  ctx.drawImage(img, 0, 0)
  const ts = Date.now()
  const name = 'pasted-' + ts + '.png'
  openTexture({
    folder: 'paste',
    filename: name,
    relPath: 'paste/' + name,
    size: 0,
    dataURI: canvasToDataURL(c)
  })
}

const dragDepth = ref(0)
const isDragging = ref(false)

function dragHasFiles(e) {
  return Array.from(e.dataTransfer?.types || []).includes('Files')
}

function onDragEnter(e) {
  if (!dragHasFiles(e)) return
  e.preventDefault()
  dragDepth.value++
  isDragging.value = true
}

function onDragOver(e) {
  if (!dragHasFiles(e)) return
  e.preventDefault()
}

function onDragLeave() {
  dragDepth.value = Math.max(0, dragDepth.value - 1)
  if (dragDepth.value === 0) isDragging.value = false
}

function onDrop(e) {
  e.preventDefault()
  dragDepth.value = 0
  isDragging.value = false
  const files = Array.from(e.dataTransfer?.files || []).filter(f => f.type.startsWith('image/'))
  if (!files.length) {
    popup('Error', 'No images found in the dropped files', 'error')
    return
  }
  for (const f of files) dropImageFile(f)
}

async function dropImageFile(file) {
  let folder = activeFolder.value
  if (!folder || folder === ALL_TAB) folder = 'paste'
  let name = (file.name || '').trim().replace(/[^a-zA-Z0-9._-]/g, '_')
  name = name.replace(/\.(tga|bmp)$/i, '.png')
  if (!/\.(png|jpg|jpeg|webp|gif)$/i.test(name)) name += '.png'
  const relPath = folder + '/' + name
  try {
    const url = URL.createObjectURL(file)
    const img = await loadImage(url)
    URL.revokeObjectURL(url)
    if (!img.naturalWidth || !img.naturalHeight) throw new Error('empty image')
    const c = document.createElement('canvas')
    c.width = img.naturalWidth
    c.height = img.naturalHeight
    const ctx = c.getContext('2d')
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(img, 0, 0)
    await openDroppedTexture({
      folder,
      filename: name,
      relPath,
      size: file.size || 0,
      dataURI: canvasToDataURL(c)
    })
  } catch (e) {
    popup('Error', 'Could not load dropped image: ' + e.toString(), 'error')
  }
}

async function openDroppedTexture(tex) {
  const existing = openTabs.value.find(t => t.relPath === tex.relPath)
  if (existing && existing.dirty) {
    const r = await Swal.mixin({
      customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn', cancelButton: 'custom-cancel-btn' },
      buttonsStyling: false
    }).fire({
      title: 'Replace image?',
      text: `A texture with this name is already open with unsaved changes. Replacing it will discard those edits.`,
      icon: 'warning',
      showCancelButton: true,
      confirmButtonText: 'Replace',
      cancelButtonText: 'Cancel'
    })
    if (!r.isConfirmed) return
  }
  if (existing) {
    existing.tex = tex
    existing.dirty = false
    tabStates.delete(tex.relPath)
    if (currentTex.value && currentTex.value.relPath === tex.relPath) {
      currentTex.value = tex
      ready.value = false
      dirty.value = false
      selMask.value = null
      selBase = null
      selAdditive = false
      selSubtractive = false
      undoStack.value = []
      redoStack.value = []
      hsvBase = null
      try {
        let uri = tex.dataURI
        if (isInstalled.value && !uri) uri = await GetPackTexture(props.source.dirName, tex.relPath, packBasePath.value)
        if (!uri) throw new Error('no image data')
        layers.value = []
        await renderImage(uri)
      } catch (e) {
        popup('Error', 'Failed to open texture: ' + e.toString(), 'error')
      }
      return
    }
  }
  await openTexture(tex)
}

async function copyPixels() {
  const c = canvasEl.value
  if (!c || !working.value.w || !ready.value) return
  const src = getImageData()
  const w = src.width
  const h = src.height
  const out = new ImageData(w, h)
  out.data.set(src.data)
  const mask = selMask.value
  if (mask) {
    for (let i = 0; i < mask.length; i++) {
      if (!mask[i]) out.data[i * 4 + 3] = 0
    }
  }
  editorClipboard.value = { w, h, data: out.data.slice() }
  try {
    if (navigator.clipboard && navigator.clipboard.write && typeof ClipboardItem !== 'undefined') {
      const tmp = document.createElement('canvas')
      tmp.width = w
      tmp.height = h
      tmp.getContext('2d').putImageData(out, 0, 0)
      const blob = await new Promise(res => tmp.toBlob(res, 'image/png'))
      if (blob) await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    }
  } catch (_) {}
}

function eraseSelectedPixels() {
  if (!canvasEl.value || !working.value.w || !ready.value) return
  const data = getImageData()
  commit()
  const d = data.data
  const mask = selMask.value
  if (mask) {
    for (let i = 0; i < mask.length; i++) {
      if (mask[i]) d[i * 4 + 3] = 0
    }
  } else {
    for (let i = 3; i < d.length; i += 4) d[i] = 0
  }
  putImageData(data)
  markDirty()
}

function cutPixels() {
  if (!canvasEl.value || !working.value.w || !ready.value) return
  copyPixels()
  eraseSelectedPixels()
}

function deleteSelection() {
  eraseSelectedPixels()
}

function clipboardToCanvas(clip) {
  const c = document.createElement('canvas')
  c.width = clip.w
  c.height = clip.h
  c.getContext('2d').putImageData(new ImageData(clip.data, clip.w, clip.h), 0, 0)
  return c
}

function pasteAsNewTabFromClip(clip) {
  const c = clipboardToCanvas(clip)
  const ts = Date.now()
  const name = 'pasted-' + ts + '.png'
  openTexture({
    folder: 'paste',
    filename: name,
    relPath: 'paste/' + name,
    size: 0,
    dataURI: canvasToDataURL(c)
  })
}

function pasteEditorClipboard() {
  const clip = editorClipboard.value
  if (!clip) return
  if (!currentTex.value || !ready.value) {
    pasteAsNewTabFromClip(clip)
    return
  }
  const w = working.value.w
  const h = working.value.h
  floatPaste.value = {
    clip,
    src: clipboardToCanvas(clip),
    x: Math.floor((w - clip.w) / 2),
    y: Math.floor((h - clip.h) / 2),
    w: clip.w,
    h: clip.h
  }
  renderOverlay()
}

function commitFloatingPaste() {
  const fp = floatPaste.value
  if (!fp) return
  if (currentTex.value && ready.value) {
    const w = working.value.w
    const h = working.value.h
    commit()
    const base = getImageData()
    const tmp = document.createElement('canvas')
    tmp.width = w
    tmp.height = h
    const tc = tmp.getContext('2d')
    tc.imageSmoothingEnabled = false
    tc.putImageData(base, 0, 0)
    const sx = Math.round(fp.x)
    const sy = Math.round(fp.y)
    tc.drawImage(fp.src, 0, 0, fp.clip.w, fp.clip.h, sx, sy, Math.max(1, Math.round(fp.w)), Math.max(1, Math.round(fp.h)))
    putImageData(tc.getImageData(0, 0, w, h))
    markDirty()
  }
  floatPaste.value = null
  renderOverlay()
}

function clearFloatingPaste() {
  floatPaste.value = null
  renderOverlay()
}

watch(brushSize, (v) => {
  brushSize.value = Math.max(1, Math.min(256, v))
  renderOverlay()
})

watch(zoom, () => nextTick(() => { rectsCache = null; renderOverlay() }))

function onViewportScroll() {
  rectsCache = null
  renderOverlay()
}

onMounted(() => {
  window.addEventListener('keydown', onKey)
  window.addEventListener('paste', onPaste)
  viewportEl.value?.addEventListener('scroll', onViewportScroll)
  viewportEl.value?.addEventListener('wheel', onWheel, { passive: false })
  if (viewportEl.value && typeof ResizeObserver !== 'undefined') {
    viewportObserver = new ResizeObserver(() => {
      rectsCache = null
      renderOverlay()
    })
    viewportObserver.observe(viewportEl.value)
  }
  loadAutosaveSetting()
  loadFolders().then(restoreSession)
})

onBeforeUnmount(() => {
  // best effort: the write may not land if the process is going down anyway,
  // but leaving via a normal unmount will save
  if (dirty.value) flushAutosave(true)
  saveSessionToStore()
  if (viewportObserver) {
    viewportObserver.disconnect()
    viewportObserver = null
  }
  if (bottomDrag) {
    window.removeEventListener('pointermove', onBottomGripMove)
    window.removeEventListener('pointerup', onBottomGripUp)
    bottomDrag = null
  }
  window.removeEventListener('keydown', onKey)
  window.removeEventListener('paste', onPaste)
  viewportEl.value?.removeEventListener('scroll', onViewportScroll)
  viewportEl.value?.removeEventListener('wheel', onWheel)
})
</script>

<template>
  <div class="re-display" @dragenter="onDragEnter" @dragleave="onDragLeave" @dragover="onDragOver" @drop="onDrop">
    <div v-if="isDragging" class="re-drop-overlay">
      <div class="re-drop-card"><i class="fa fa-images"></i> Drop images here</div>
    </div>
    <div class="re-topbar">
      <button class="re-btn re-back" @click="handleClose"><i class="fa fa-arrow-left"></i> Back</button>
      <div class="re-title">
        <span class="re-packname">{{ isInstalled ? props.source.dirName : (props.source.packName || 'Uploaded Pack') }}</span>
        <span v-if="currentTex" class="re-filename">{{ currentTex.relPath }}</span>
      </div>
      <div class="re-top-actions">
        <button class="re-btn" :disabled="!canUndo" @click="undo" title="Undo (Ctrl+Z)"><i class="fa fa-rotate-left"></i></button>
        <button class="re-btn" :disabled="!canRedo" @click="redo" title="Redo (Ctrl+Y)"><i class="fa fa-rotate-right"></i></button>
        <button v-if="isInstalled" class="re-btn" @click="openInExplorer" title="Open pack folder in File Explorer"><i class="fa fa-folder-open"></i></button>
        <div class="re-savewrap">
          <button class="re-btn re-save" :disabled="!currentTex || busy" @click="saveMenuOpen = !saveMenuOpen">Save <i class="fa fa-caret-down"></i></button>
          <div v-if="saveMenuOpen" class="re-save-menu">
            <button @click="saveMenuOpen = false; saveReplace()"><i class="fa fa-pen"></i> Replace</button>
            <button @click="saveMenuOpen = false; saveAs()"><i class="fa fa-copy"></i> Save As...</button>
            <button v-if="!isInstalled" @click="saveMenuOpen = false; exportUploaded()"><i class="fa fa-file-export"></i> Export Pack...</button>
          </div>
        </div>
      </div>
    </div>

    <div class="re-tabs" v-if="openTabs.length">
      <button
        v-for="tab in openTabs" :key="tab.relPath"
        class="re-tab" :class="{ active: currentTex && currentTex.relPath === tab.relPath }"
        @click="switchTab(tab.relPath)"
      >
        <span class="re-tab-name">{{ tabLabel(tab) }}</span>
        <span v-if="tab.dirty" class="re-tab-dot" title="Unsaved changes"></span>
        <i class="fa fa-xmark re-tab-close" @click.stop="closeTab(tab.relPath)" title="Close"></i>
      </button>
      <button class="re-tab re-tab-closeall" @click="closeAllTabs" title="Close all tabs"><i class="fa fa-xmark"></i> Close all</button>
    </div>

    <div class="re-body">
      <div class="re-toolrail">
        <button
          v-for="t in tools" :key="t.id"
          class="re-tool-btn" :class="{ active: tool === t.id }"
          :title="t.label" @click="tool = t.id"
        ><i :class="'fa ' + t.icon"></i></button>
        <div class="re-rail-sep"></div>
        <button
          v-for="t in adjustTools" :key="t.id"
          class="re-tool-btn" :class="{ active: tool === t.id }"
          :title="t.label"
          @click="onAdjustTool(t.id)"
        ><i :class="'fa ' + t.icon"></i></button>

        <div class="re-rail-sep"></div>
        <button class="re-tool-btn" title="Mark center pixel (C)" @click="centerImage"><i class="fa fa-crosshairs"></i></button>

        <div class="re-colors">
          <div class="re-color-stack">
            <button class="re-color-input re-fg" :style="{ background: fg }" :title="'Foreground (primary)  ' + fg" @click="openColorPicker('fg')"></button>
            <button class="re-color-input re-bg" :style="{ background: bg }" :title="'Background (secondary)  ' + bg" @click="openColorPicker('bg')"></button>
          </div>
          <button class="re-tool-btn" title="Swap colors (X)" @click="swapColors"><i class="fa fa-arrows-up-down"></i></button>
        </div>

        <div v-if="hasSelection" class="re-selection-clear">
          <button class="re-tool-btn" title="Clear selection (Esc)" @click="clearSelection"><i class="fa fa-scissors"></i></button>
        </div>
      </div>

      <div class="re-main">
        <div class="re-optionsbar">
          <template v-if="tool === 'brush'">
            <label class="re-opt">
              <span>Brush</span>
              <input type="range" min="1" max="120" v-model.number="brushSize" class="re-range" />
              <b>{{ brushSize }}px</b>
            </label>
          </template>
          <template v-else-if="tool === 'eraser'">
            <label class="re-opt">
              <span>Eraser</span>
              <input type="range" min="1" max="120" v-model.number="brushSize" class="re-range" />
              <b>{{ brushSize }}px</b>
            </label>
          </template>
          <template v-else-if="tool === 'wand' || tool === 'bucket'">
            <label class="re-opt">
              <span>Tolerance</span>
              <input type="range" min="0" max="100" v-model.number="tolerance" class="re-range" />
              <b>{{ tolerance }}</b>
            </label>
          </template>
          <template v-else-if="tool === 'gradient'">
            <label class="re-opt">
              <span>Gradient</span>
              <select v-model="gradientMode" class="re-select">
                <option value="linear">Linear</option>
                <option value="radial">Radial</option>
              </select>
            </label>
          </template>
          <template v-else-if="tool === 'select' || tool === 'wand'">
            <span class="re-hint">Selection active — edits apply only inside it</span>
          </template>

          <label class="re-opt re-opacity">
            <span>Opacity</span>
            <input type="range" min="0" max="100" v-model.number="opacity" class="re-range" />
            <b>{{ opacity }}%</b>
          </label>

          <div class="re-zoom">
            <button class="re-btn" @click="zoomBy(1 / 1.25)" title="Zoom out"><i class="fa fa-minus"></i></button>
            <button class="re-btn" @click="fitZoom" title="Fit to window">{{ (zoom * 100).toFixed(0) }}%</button>
            <button class="re-btn" @click="zoomBy(1.25)" title="Zoom in"><i class="fa fa-plus"></i></button>
          </div>
        </div>

        <div class="re-work">
          <div class="re-layers" :class="{ open: layersOpen }">
            <div class="re-layer-head">
              <span>Layers</span>
              <button class="re-btn re-ghost" @click="layersOpen = !layersOpen" title="Toggle layers"><i class="fa" :class="layersOpen ? 'fa-chevron-left' : 'fa-chevron-right'"></i></button>
            </div>
            <template v-if="layersOpen && layers.length">
              <div class="re-layer-list">
                <div
                  v-for="layer in revLayers" :key="layer.id"
                  class="re-layer" :class="{ active: layer.id === activeLayerId }"
                  @click="selectLayer(layer.id)"
                >
                  <div class="re-layer-main">
                    <button class="re-layer-eye" :class="{ off: !layer.visible }" @click.stop="toggleLayer(layer.id)" title="Toggle visibility"><i class="fa" :class="layer.visible ? 'fa-eye' : 'fa-eye-slash'"></i></button>
                    <span class="re-layer-name">{{ layer.name }}</span>
                  </div>
                  <div class="re-layer-opts">
                    <select class="re-layer-blend" :value="layer.blend || 'normal'" @change.stop="setLayerBlend(layer.id, $event.target.value)" title="Blend mode">
                      <option v-for="b in BLEND_MODES" :key="b.value" :value="b.value">{{ b.label }}</option>
                    </select>
                    <input type="range" min="0" max="100" v-model.number="layer.opacity" class="re-layer-op" title="Opacity" @input.stop="setLayerOpacity(layer.id, layer.opacity)" />
                    <b class="re-layer-op-val">{{ layer.opacity }}%</b>
                  </div>
                </div>
              </div>
              <div class="re-layer-actions">
                <button class="re-btn re-ghost" @click="addLayer" title="New layer (+)"><i class="fa fa-plus"></i></button>
                <button class="re-btn re-ghost" @click="deleteLayer" title="Delete layer" :disabled="!canDelete"><i class="fa fa-minus"></i></button>
                <button class="re-btn re-ghost" @click="duplicateLayer" title="Duplicate layer"><i class="fa fa-copy"></i></button>
                <button class="re-btn re-ghost" @click="moveLayer(1)" title="Move layer up" :disabled="!canMoveUp"><i class="fa fa-chevron-up"></i></button>
                <button class="re-btn re-ghost" @click="moveLayer(-1)" title="Move layer down" :disabled="!canMoveDown"><i class="fa fa-chevron-down"></i></button>
                <button class="re-btn re-ghost" @click="mergeDown" title="Merge down onto layer below" :disabled="!canMergeDown"><i class="fa fa-compress"></i></button>
                <button class="re-btn re-ghost" @click="flatten" title="Flatten image" :disabled="!canFlatten"><i class="fa fa-layer-group"></i></button>
              </div>
            </template>
          </div>

          <div class="re-canvas-area">
            <div class="re-canvas-stage">
              <div ref="viewportEl" class="re-viewport">
                <div v-if="!currentTex" class="re-empty">
                  <i class="fa fa-image"></i>
                  <p>Select a texture on the right, or pick an open tab at the bottom.</p>
                </div>
                <div v-else class="re-canvas-wrap" :style="{ width: working.w * zoom + 'px', height: working.h * zoom + 'px' }">
                  <canvas
                    ref="canvasEl"
                    class="re-canvas"
                    :class="{ 're-cursor-hidden': tool === 'brush' || tool === 'eraser' }"
                    :style="{ width: working.w * zoom + 'px', height: working.h * zoom + 'px' }"
                    @pointerdown="onPointerDown"
                    @pointermove="onPointerMove"
                    @pointerup="onPointerUp"
                    @pointerleave="onPointerLeave"
                    @pointercancel="onPointerCancel"
                    @contextmenu.prevent
                  ></canvas>
                  <canvas ref="overlayEl" class="re-canvas re-overlay" :style="{ width: working.w * zoom + 'px', height: working.h * zoom + 'px' }"></canvas>
                </div>
              </div>
              <div v-if="dirty" class="re-dirty-badge" title="Unsaved changes">modified</div>
            </div>

            <div class="re-statusbar">
              <span v-if="working.w">{{ working.w }} × {{ working.h }}px</span>
              <span v-else>no image</span>
              <span v-if="currentTex && currentTex.size">{{ formatBytes(currentTex.size) }}</span>
              <span v-if="activeLayer()" class="re-status-layer">{{ activeLayer().name }}</span>
              <span v-if="hasSelection" class="re-status-sel">selection on</span>
              <span v-if="dirty" class="re-status-dirty">● modified</span>
              <span v-if="hasFloatPaste" class="re-status-float">pasted — drag to move, corners to resize, Enter confirms, Esc cancels</span>
            </div>
          </div>
        </div>

        <div class="re-bottom" :style="{ height: bottomH + 'px' }">
          <div class="re-bottom-grip" @pointerdown="onBottomGripDown" title="Drag to resize"></div>
          <div class="re-bottom-head">
            <span class="re-bottom-title">Textures</span>
            <div class="re-search">
              <i class="fa fa-search"></i>
              <input type="text" v-model="search" placeholder="Search textures..." spellcheck="false" />
              <button v-if="search" class="re-search-clear" @click="search = ''"><i class="fa fa-xmark"></i></button>
            </div>
          </div>
          <div class="re-foldertabs">
            <i class="fa fa-folder"></i>
            <select
              class="re-folder-select"
              :value="activeFolder"
              @change="selectFolder($event.target.value)"
            >
              <option v-for="f in folders" :key="f" :value="f">{{ folderLabel(f) }}</option>
            </select>
          </div>
          <div class="re-texturegrid" v-if="!loading">
            <template v-if="filteredTextures.length">
              <div
                v-for="t in filteredTextures" :key="t.relPath"
                class="re-tcell" :class="{ active: currentTex && currentTex.relPath === t.relPath }"
                @click="openTexture(t)"
              >
                <div class="re-tcell-wrap">
                  <img v-if="showThumb(t)" :src="showThumb(t)" class="re-tcell-img" alt="" />
                  <div v-else class="re-tcell-pl"></div>
                </div>
                <span class="re-tcell-name">{{ t.filename }}</span>
              </div>
            </template>
            <div v-else class="re-no-match">No textures match</div>
          </div>
          <div v-else class="re-loading"><div class="fp-spinner"></div></div>
          <div class="re-count">{{ filteredTextures.length }}<span v-if="search.trim() && allTextures.length"> / {{ allTextures.length }}</span><span v-else> / {{ textures.length }}</span></div>
        </div>
      </div>
    </div>

    <div v-if="resizing" class="re-modal-bg" @click.self="resizing = false">
      <div class="re-modal">
        <h3>Resize Image</h3>
        <div class="re-modal-row">
          <label>Width
            <input type="number" min="1" v-model.number="resizeW" class="re-input" @input="resizeLock && (resizeH = Math.round(resizeW / resizeRatio))" />
          </label>
          <label>Height
            <input type="number" min="1" v-model.number="resizeH" class="re-input" @input="resizeLock && (resizeW = Math.round(resizeH * resizeRatio))" />
          </label>
        </div>
        <label class="re-check">
          <input type="checkbox" v-model="resizeLock" /> Keep aspect ratio
        </label>
        <p class="re-modal-note">Scaling uses nearest-neighbor so pixels stay crisp.</p>
        <div class="re-modal-actions">
          <button class="re-btn" @click="resizing = false">Cancel</button>
          <button class="re-btn re-apply" @click="applyResize">Apply</button>
        </div>
      </div>
    </div>

    <div v-if="hsvOpen" class="re-modal-bg" @click.self="hsvExitCancel">
      <div class="re-modal re-hsv-pop">
        <h3>Adjust Color</h3>
        <div class="re-hsv-rows">
          <label class="re-hsv-row"><span>Hue</span><input type="range" min="-180" max="180" v-model.number="hue" class="re-range re-hue" /><input type="number" min="-180" max="180" v-model.number="hue" class="re-num" /><b>°</b></label>
          <label class="re-hsv-row"><span>Sat</span><input type="range" min="-100" max="100" v-model.number="sat" class="re-range" /><input type="number" min="-100" max="100" v-model.number="sat" class="re-num" /><b>%</b></label>
          <label class="re-hsv-row"><span>Bright</span><input type="range" min="-100" max="100" v-model.number="bright" class="re-range" /><input type="number" min="-100" max="100" v-model.number="bright" class="re-num" /><b>%</b></label>
        </div>
        <p class="re-modal-note">Live preview — applies this shift to the active layer (inside the selection if one exists). Numbers carry over as you switch textures.</p>
        <div class="re-modal-actions">
          <button class="re-btn" @click="hsvReset">Reset</button>
          <button class="re-btn" @click="hsvExitCancel">Cancel (Esc)</button>
          <button class="re-btn re-apply" @click="hsvExitApply"><i class="fa fa-check"></i> Apply (Enter)</button>
        </div>
      </div>
    </div>

    <div v-if="pickerOpen" class="re-modal-bg" @click.self="closeColorPicker">
      <div class="re-modal re-cpicker">
        <h3>Color Picker</h3>
        <div class="re-cp-slots">
          <button class="re-cp-slot" :class="{ active: pickTarget === 'fg' }" @click="switchSlot('fg')">
            <span class="re-cp-slot-chip" :style="{ background: fg }"></span>
            <span class="re-cp-slot-label">Foreground</span>
          </button>
          <button class="re-cp-swap" title="Swap colors (X)" @click="onPickSwap"><i class="fa fa-arrows-up-down"></i></button>
          <button class="re-cp-slot" :class="{ active: pickTarget === 'bg' }" @click="switchSlot('bg')">
            <span class="re-cp-slot-chip" :style="{ background: bg }"></span>
            <span class="re-cp-slot-label">Background</span>
          </button>
        </div>
        <div class="re-cp-wheel-wrap">
          <div
            ref="wheelEl"
            class="re-cwheel"
            @pointerdown="onWheelPointerDown"
            @pointermove="onWheelPointerMove"
            @pointerup="onWheelPointerUp"
            @pointercancel="onWheelPointerUp"
          >
            <canvas ref="wheelCanvasEl" class="re-cwheel-canvas" width="280" height="280"></canvas>
            <span class="re-cwheel-marker" :style="wheelMarkerStyle"></span>
          </div>
          <div
            ref="brightBarEl"
            class="re-bright"
            title="Brightness"
            @pointerdown="onBrightPointerDown"
            @pointermove="onBrightPointerMove"
            @pointerup="onBrightPointerUp"
            @pointercancel="onBrightPointerUp"
          >
            <canvas ref="brightCanvasEl" class="re-bright-canvas" width="720" height="24"></canvas>
            <span class="re-bright-marker" :style="{ left: pickHsv.v + '%' }"></span>
          </div>
        </div>
        <div class="re-cp-bottom">
          <label class="re-cp-hex"><span>Hex</span><i>#</i><input :value="hexDraft" @input="onHexInput" class="re-input" spellcheck="false" maxlength="6" /></label>
          <div class="re-cp-preview" :style="{ background: pickHex }" :title="pickHex"></div>
          <button class="re-btn re-more-btn" @click="toggleMore">{{ moreOpen ? 'Less' : 'More' }} <i class="fa" :class="moreOpen ? 'fa-chevron-up' : 'fa-chevron-down'"></i></button>
        </div>
        <div v-if="moreOpen" class="re-cp-more">
          <span class="re-cp-panel-label">RGB</span>
          <div class="re-cp-rows">
            <label class="re-hsv-row"><span>Red</span><input type="range" min="0" max="255" :value="pickRgb.r" @input="onRgbInput($event, 'r')" class="re-range" /><input type="number" min="0" max="255" :value="pickRgb.r" @input="onRgbInput($event, 'r')" class="re-num" /></label>
            <label class="re-hsv-row"><span>Green</span><input type="range" min="0" max="255" :value="pickRgb.g" @input="onRgbInput($event, 'g')" class="re-range" /><input type="number" min="0" max="255" :value="pickRgb.g" @input="onRgbInput($event, 'g')" class="re-num" /></label>
            <label class="re-hsv-row"><span>Blue</span><input type="range" min="0" max="255" :value="pickRgb.b" @input="onRgbInput($event, 'b')" class="re-range" /><input type="number" min="0" max="255" :value="pickRgb.b" @input="onRgbInput($event, 'b')" class="re-num" /></label>
          </div>
          <span class="re-cp-panel-label">HSV</span>
          <div class="re-cp-rows">
            <label class="re-hsv-row"><span>Hue</span><input type="range" min="0" max="360" :value="pickHsv.h" @input="onHueInput" class="re-range re-hue" /><input type="number" min="0" max="360" :value="pickHsv.h" @input="onHueInput" class="re-num" /><b>°</b></label>
            <label class="re-hsv-row"><span>Sat</span><input type="range" min="0" max="100" :value="pickHsv.s" @input="onSatInput" class="re-range" /><input type="number" min="0" max="100" :value="pickHsv.s" @input="onSatInput" class="re-num" /><b>%</b></label>
            <label class="re-hsv-row"><span>Value</span><input type="range" min="0" max="100" :value="pickHsv.v" @input="onValueInput" class="re-range" /><input type="number" min="0" max="100" :value="pickHsv.v" @input="onValueInput" class="re-num" /><b>%</b></label>
          </div>
        </div>
        <div class="re-cp-shades">
          <span class="re-cp-panel-label">Shades</span>
          <div
            ref="shadesEl"
            class="re-shades"
            title="Click or drag to pick a shade"
            @pointerdown="onShadesPointerDown"
            @pointermove="onShadesPointerMove"
            @pointerup="onShadesPointerUp"
            @pointercancel="onShadesPointerUp"
          >
            <canvas ref="shadesCanvasEl" class="re-shades-canvas" width="360" height="64"></canvas>
            <span class="re-shades-marker" :style="{ left: pickHsv.s + '%', top: (100 - pickHsv.v) + '%' }"></span>
          </div>
        </div>
        <div class="re-cp-palette" title="Left-click: set active color. Right-click: set the other color.">
          <button v-for="c in PALETTE_COLORS" :key="c" class="re-cp-pcell" :style="{ background: c }" :class="{ active: pickHex === c }" @click="onPalettePick(c)" @contextmenu.prevent="onPalettePickOther(c)"></button>
        </div>
        <div class="re-modal-actions">
          <button class="re-btn" @click="closeColorPicker"><i class="fa fa-xmark"></i> Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.re-display {
  flex: 1;
  min-width: 0;
  min-height: 0;
  height: 100%;
  position: relative;
  background: var(--bg-body);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.re-drop-overlay {
  position: absolute;
  inset: 0;
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
  background: color-mix(in srgb, var(--bg-body) 55%, transparent);
  pointer-events: none;
}

.re-drop-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 2rem 2.75rem;
  border: 2px dashed var(--accent);
  border-radius: 12px;
  background: var(--bg-surface);
  color: var(--accent-light);
  font-size: 1.05rem;
  font-weight: 600;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.45);
}

.re-drop-card i {
  font-size: 2.4rem;
  color: var(--accent);
}

.re-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  padding: 0.6rem 1rem;
  border-bottom: 1px solid var(--border-default);
  background: var(--bg-sidebar);
  flex-shrink: 0;
}

.re-title {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.re-packname {
  font-weight: 700;
  color: var(--accent);
  font-size: 0.9rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.re-filename {
  font-size: 0.7rem;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.re-top-actions, .re-zoom, .re-modal-actions, .re-re-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.re-btn {
  padding: 0.35rem 0.7rem;
  background: var(--bg-hover-2);
  color: var(--text-muted2);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.78rem;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  transition: all 0.15s;
}

.re-btn:hover:not(:disabled) {
  background: var(--bg-hover-4);
  color: var(--text-secondary);
}

.re-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.re-back {
  border-color: var(--border-strong);
}

.re-save {
  background: var(--accent-glow);
  color: var(--accent-light);
  border-color: var(--accent-border);
  font-weight: 700;
}

.re-save:hover {
  background: var(--accent);
  color: var(--bg-body);
}

.re-savewrap {
  position: relative;
}

.re-save-menu {
  position: absolute;
  top: calc(100% + 4px);
  right: 0;
  background: var(--bg-surface);
  border: 1px solid var(--border-medium);
  border-radius: 8px;
  padding: 0.3rem;
  min-width: 170px;
  z-index: 40;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
  display: flex;
  flex-direction: column;
}

.re-save-menu button {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.45rem 0.6rem;
  background: none;
  border: none;
  color: var(--text-secondary);
  font-size: 0.8rem;
  cursor: pointer;
  border-radius: 6px;
  text-align: left;
}

.re-save-menu button:hover {
  background: var(--bg-hover-3);
  color: var(--text-primary);
}

.re-body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.re-toolrail {
  width: 52px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.3rem;
  padding: 0.5rem 0;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-subtle);
  overflow-y: auto;
}

.re-tool-btn {
  width: 36px;
  height: 36px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  color: var(--text-dim);
  border: 1px solid transparent;
  border-radius: 7px;
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.12s;
}

.re-tool-btn:hover {
  background: var(--bg-hover-3);
  color: var(--text-secondary);
}

.re-tool-btn.active {
  background: var(--accent-active-bg);
  color: var(--accent);
  border-color: var(--accent-border);
}

.re-rail-sep {
  height: 1px;
  width: 70%;
  background: var(--border-subtle);
  margin: 0.25rem 0;
}

.re-colors {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  margin-top: 0.3rem;
}

.re-color-stack {
  position: relative;
  width: 32px;
  height: 44px;
}

.re-color-input {
  position: absolute;
  width: 26px;
  height: 26px;
  border: 2px solid var(--bg-surface);
  border-radius: 7px;
  padding: 0;
  cursor: pointer;
}

.re-fg {
  top: 0;
  left: 0;
  z-index: 2;
}

.re-bg {
  bottom: 0;
  right: 0;
  z-index: 1;
  border-color: var(--border-medium);
}

.re-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.re-work {
  flex: 1;
  min-height: 0;
  display: flex;
}

.re-canvas-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}

.re-canvas-stage {
  position: relative;
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
}

.re-layers {
  width: 176px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: var(--bg-sidebar);
  border-right: 1px solid var(--border-subtle);
  min-height: 0;
}

.re-layer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.4rem 0.6rem;
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-secondary);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.re-layer-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 0.3rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.re-layer {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  padding: 0.3rem 0.35rem;
  border-radius: 6px;
  border: 1px solid var(--border-light);
  background: var(--bg-hover-1);
  cursor: pointer;
  transition: all 0.12s;
}

.re-layer.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}

.re-layer-main {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  min-width: 0;
}

.re-layer-opts {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  min-width: 0;
}

.re-layer-eye {
  flex-shrink: 0;
  width: 20px;
  height: 20px;
  border: none;
  background: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 0.72rem;
}

.re-layer-eye.off {
  color: var(--text-faint);
}

.re-layer-name {
  flex: 1;
  min-width: 0;
  font-size: 0.68rem;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.re-layer-blend {
  flex-shrink: 0;
  min-width: 0;
  max-width: 76px;
  font-size: 0.62rem;
  padding: 0.1rem 0.2rem;
  background: var(--bg-input);
  color: var(--text-secondary);
  border: 1px solid var(--border-medium);
  border-radius: 4px;
  outline: none;
  cursor: pointer;
}

.re-layer-op {
  flex: 1;
  min-width: 0;
  height: 3px;
  -webkit-appearance: none;
  appearance: none;
  border-radius: 2px;
  background: var(--bg-hover-4);
  outline: none;
  cursor: pointer;
}

.re-layer-op::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 9px;
  height: 9px;
  border-radius: 50%;
  background: var(--accent);
  border: 1px solid var(--text-primary);
  cursor: pointer;
}

.re-layer-op-val {
  width: 30px;
  text-align: right;
  font-size: 0.6rem;
  color: var(--text-dim);
  font-variant-numeric: tabular-nums;
}

.re-layer-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.2rem;
  padding: 0.4rem;
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.re-optionsbar {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.4rem 0.8rem;
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.re-opt {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.72rem;
  color: var(--text-dim);
}

.re-opt b {
  color: var(--text-secondary);
  min-width: 42px;
  font-variant-numeric: tabular-nums;
}

.re-range {
  -webkit-appearance: none;
  appearance: none;
  width: 130px;
  height: 5px;
  border-radius: 3px;
  background: var(--bg-hover-4);
  outline: none;
  cursor: pointer;
}

.re-range::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 13px;
  height: 13px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid var(--text-primary);
  cursor: pointer;
}

.re-hue {
  background: linear-gradient(to right, #ff0000, #ffff00, #00ff00, #00ffff, #0000ff, #ff00ff, #ff0000);
}

.re-select {
  padding: 0.25rem 0.5rem;
  background: var(--bg-input);
  color: var(--text-secondary);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  font-size: 0.75rem;
  outline: none;
}

.re-hint {
  font-size: 0.72rem;
  color: var(--text-dim);
}

.re-hsv-pop {
  width: 360px;
}

.re-hsv-rows {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.re-hsv-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.72rem;
  color: var(--text-dim);
}

.re-hsv-row span {
  width: 48px;
  flex-shrink: 0;
}

.re-hsv-pop .re-range {
  flex: 1;
  min-width: 0;
}

.re-num {
  width: 52px;
  padding: 0.2rem 0.3rem;
  font-size: 0.7rem;
  color: var(--text-secondary);
  background: var(--bg-input);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  text-align: center;
  outline: none;
  font-variant-numeric: tabular-nums;
}

.re-num:focus {
  border-color: var(--accent-border);
}

.re-cpicker {
  width: 400px;
}

.re-cp-slots {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.6rem;
}

.re-cp-slot {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex: 1;
  min-width: 0;
  padding: 0.35rem 0.5rem;
  background: var(--bg-hover-1);
  color: var(--text-muted);
  border: 1px solid var(--border-light);
  border-radius: 7px;
  cursor: pointer;
  font-size: 0.72rem;
  transition: all 0.12s;
}

.re-cp-slot:hover {
  background: var(--bg-hover-2);
  color: var(--text-secondary);
}

.re-cp-slot.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
  color: var(--text-primary);
}

.re-cp-slot-chip {
  width: 18px;
  height: 18px;
  border-radius: 4px;
  flex-shrink: 0;
  border: 1px solid rgba(0, 0, 0, 0.35);
  box-shadow: 0 0 0 1px var(--border-light);
}

.re-cp-slot-label {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.re-cp-swap {
  flex-shrink: 0;
  padding: 0.3rem 0.45rem;
  background: var(--bg-hover-1);
  color: var(--text-muted);
  border: 1px solid var(--border-light);
  border-radius: 7px;
  cursor: pointer;
  font-size: 0.75rem;
  transition: all 0.12s;
}

.re-cp-swap:hover {
  background: var(--bg-hover-2);
  color: var(--text-secondary);
}

.re-cp-wheel-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.7rem;
}

.re-cwheel {
  position: relative;
  width: 200px;
  height: 200px;
  flex-shrink: 0;
  border-radius: 50%;
  cursor: crosshair;
  touch-action: none;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.35);
}

.re-cwheel-canvas {
  width: 200px;
  height: 200px;
  border-radius: 50%;
  display: block;
}

.re-cwheel-marker {
  position: absolute;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid #fff;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.5);
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.re-bright {
  position: relative;
  width: 200px;
  height: 24px;
  border-radius: 6px;
  overflow: hidden;
  cursor: pointer;
  touch-action: none;
  box-shadow: inset 0 0 0 1px var(--border-medium);
}

.re-bright-canvas {
  width: 200px;
  height: 24px;
  display: block;
}

.re-bright-marker {
  position: absolute;
  top: 0;
  bottom: 0;
  width: 3px;
  background: rgba(255, 255, 255, 0.95);
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.5);
  transform: translateX(-50%);
  pointer-events: none;
}

.re-cp-shades {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  margin-bottom: 0.7rem;
}

.re-cp-panel-label {
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.4px;
}

.re-shades {
  position: relative;
  height: 64px;
  border-radius: 8px;
  overflow: hidden;
  cursor: pointer;
  touch-action: none;
  box-shadow: inset 0 0 0 1px var(--border-medium);
}

.re-shades-canvas {
  width: 100%;
  height: 64px;
  display: block;
}

.re-shades-marker {
  position: absolute;
  width: 12px;
  height: 12px;
  border-radius: 50%;
  border: 2px solid rgba(255, 255, 255, 0.95);
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.5);
  transform: translate(-50%, -50%);
  pointer-events: none;
}

.re-cp-rows {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin-bottom: 0.7rem;
}

.re-cp-bottom {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  margin-bottom: 0.7rem;
}

.re-cp-hex {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.72rem;
  color: var(--text-dim);
  flex: 1;
  min-width: 0;
}

.re-cp-hex i {
  font-style: normal;
  color: var(--text-muted);
}

.re-cp-hex .re-input {
  width: 92px;
  padding: 0.3rem 0.4rem;
  font-size: 0.78rem;
  letter-spacing: 1px;
}

.re-cp-preview {
  width: 36px;
  height: 36px;
  flex-shrink: 0;
  border-radius: 7px;
  border: 1px solid var(--border-medium);
  box-shadow: 0 0 0 1px var(--bg-surface);
}

.re-more-btn {
  flex-shrink: 0;
  padding: 0.3rem 0.55rem;
  font-size: 0.72rem;
}

.re-cp-more {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  margin-bottom: 0.7rem;
}

.re-cp-palette {
  display: grid;
  grid-template-columns: repeat(8, 1fr);
  gap: 0.3rem;
  margin-bottom: 0.8rem;
}

.re-cp-pcell {
  aspect-ratio: 1;
  border-radius: 4px;
  border: 1px solid rgba(0, 0, 0, 0.35);
  box-shadow: 0 0 0 1px var(--border-light);
  cursor: pointer;
  padding: 0;
  transition: transform 0.1s;
}

.re-cp-pcell:hover {
  transform: scale(1.12);
}

.re-cp-pcell.active {
  box-shadow: 0 0 0 2px var(--accent);
}

.re-num::-webkit-inner-spin-button,
.re-num::-webkit-outer-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.re-input {
  padding: 0.45rem 0.6rem;
  background: var(--bg-input);
  color: var(--text-primary);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  font-size: 0.85rem;
  outline: none;
  width: 100%;
  box-sizing: border-box;
}

.re-input::-webkit-inner-spin-button,
.re-input::-webkit-outer-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.re-zoom {
  margin-left: auto;
}

.re-apply {
  background: var(--accent-glow);
  color: var(--accent-light);
  border-color: var(--accent-border);
}

.re-viewport {
  flex: 1;
  min-height: 0;
  overflow: auto;
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: flex-start;
  padding: 20px;
  background: var(--bg-body);
}

.re-canvas-wrap {
  position: relative;
  flex-shrink: 0;
  margin: auto;
}

.re-canvas {
  display: block;
  image-rendering: pixelated;
  background: repeating-conic-gradient(var(--preview-checker-a) 0% 25%, var(--preview-checker-b) 0% 50%) 50% / 16px 16px;
  touch-action: none;
  cursor: crosshair;
  box-shadow: 0 0 0 1px var(--border-medium);
}

.re-canvas.re-cursor-hidden {
  cursor: none;
}

.re-overlay {
  position: absolute;
  top: 0;
  left: 0;
  pointer-events: none;
  background: none;
  box-shadow: none;
  image-rendering: pixelated;
}

.re-dirty-badge {
  position: absolute;
  right: 10px;
  bottom: 10px;
  font-size: 0.65rem;
  color: #ffd24a;
  background: rgba(255, 210, 74, 0.15);
  border: 1px solid rgba(255, 210, 74, 0.4);
  padding: 0.15rem 0.5rem;
  border-radius: 999px;
  pointer-events: none;
}

.re-empty {
  margin: auto;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.6rem;
  color: var(--text-dim);
  font-size: 0.85rem;
}

.re-empty .fa {
  font-size: 2.4rem;
}

.re-statusbar {
  display: flex;
  align-items: center;
  gap: 0.8rem;
  padding: 0.25rem 0.8rem;
  font-size: 0.68rem;
  color: var(--text-dim);
  background: var(--bg-sidebar);
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.re-status-dirty {
  color: #ffd24a;
}

.re-status-layer {
  color: var(--text-secondary);
  font-variant-numeric: tabular-nums;
}

.re-status-sel {
  color: #4da6ff;
}

.re-status-float {
  color: var(--accent-light);
  font-style: italic;
}

.re-ghost {
  border: none;
  background: none;
  padding: 0.2rem 0.4rem;
}

.re-search {
  position: relative;
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.4rem 0.5rem;
  flex-shrink: 0;
}

.re-search .fa {
  color: var(--text-dim);
  font-size: 0.7rem;
  pointer-events: none;
}

.re-search input {
  flex: 1;
  min-width: 0;
  padding: 0.3rem 0.5rem;
  font-size: 0.7rem;
  color: var(--text-secondary);
  background: var(--bg-input);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  outline: none;
}

.re-search input:focus {
  border-color: var(--accent-border);
}

.re-search-clear {
  padding: 0.2rem 0.35rem;
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 0.7rem;
}

.re-search-clear:hover {
  color: var(--text-primary);
}

.re-foldertabs {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.5rem;
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.re-foldertabs .fa {
  color: var(--text-dim);
  font-size: 0.72rem;
  flex-shrink: 0;
}

.re-folder-select {
  flex: 1;
  min-width: 0;
  padding: 0.3rem 0.5rem;
  font-size: 0.72rem;
  color: var(--text-secondary);
  background: var(--bg-input);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  outline: none;
  cursor: pointer;
  text-overflow: ellipsis;
}

.re-folder-select:focus {
  border-color: var(--accent-border);
}

.re-no-match {
  grid-column: 1 / -1;
  padding: 1rem;
  text-align: center;
  font-size: 0.7rem;
  color: var(--text-dim);
}

.re-count {
  padding: 0.25rem 0.5rem;
  font-size: 0.62rem;
  color: var(--text-muted);
  border-top: 1px solid var(--border-subtle);
  flex-shrink: 0;
  font-variant-numeric: tabular-nums;
}

.re-tabs {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.35rem 0.8rem;
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--border-subtle);
  overflow-x: auto;
  flex-shrink: 0;
}

.re-tab {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.25rem 0.55rem;
  font-size: 0.7rem;
  color: var(--text-dim);
  background: var(--bg-hover-1);
  border: 1px solid var(--border-light);
  border-radius: 999px;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  transition: all 0.12s;
}

.re-tab:hover {
  border-color: var(--border-focus);
  color: var(--text-secondary);
}

.re-tab.active {
  background: var(--accent-active-bg);
  color: var(--accent);
  border-color: var(--accent-border);
}

.re-tab-name {
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.re-tab-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #ffd24a;
  flex-shrink: 0;
}

.re-tab-close {
  font-size: 0.6rem;
  color: var(--text-muted);
  cursor: pointer;
}

.re-tab-close:hover {
  color: #e74c3c;
}

.re-tab-closeall {
  font-size: 0.65rem;
  color: var(--text-muted);
}

.re-tab-closeall:hover {
  color: #e74c3c;
  border-color: #e74c3c;
}

.re-bottom {
  flex-shrink: 0;
  border-top: 1px solid var(--border-subtle);
  background: var(--bg-surface);
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.re-bottom-grip {
  flex-shrink: 0;
  height: 8px;
  cursor: row-resize;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-sidebar);
  border-bottom: 1px solid var(--border-subtle);
  touch-action: none;
}

.re-bottom-grip::before {
  content: '';
  width: 46px;
  height: 3px;
  border-radius: 2px;
  background: var(--bg-hover-4);
}

.re-bottom-grip:hover::before {
  background: var(--accent);
}

.re-bottom-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.3rem 0.6rem;
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.re-bottom-title {
  font-size: 0.75rem;
  font-weight: 700;
  color: var(--text-secondary);
}

.re-texturegrid {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(84px, 1fr));
  gap: 0.4rem;
  padding: 0.5rem;
  align-content: start;
}

.re-tcell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.15rem;
  padding: 0.3rem 0.15rem;
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  background: var(--bg-body);
  cursor: pointer;
  transition: all 0.12s;
  min-width: 0;
}

.re-tcell:hover {
  border-color: var(--border-focus);
}

.re-tcell.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}

.re-tcell-wrap {
  position: relative;
  width: 48px;
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.re-tcell-img {
  width: 48px;
  height: 48px;
  image-rendering: pixelated;
  object-fit: contain;
}

.re-tcell-pl {
  width: 48px;
  height: 48px;
  background: var(--bg-hover-2);
  border-radius: 6px;
}

.re-tcell-name {
  font-size: 0.55rem;
  color: var(--text-muted);
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: center;
}

.re-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  flex: 1;
}

.fp-spinner {
  width: 22px;
  height: 22px;
  border: 3px solid var(--border-strong);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.re-modal-bg {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 50;
  display: flex;
  align-items: center;
  justify-content: center;
}

.re-modal {
  background: var(--bg-surface);
  border: 1px solid var(--border-medium);
  border-radius: 12px;
  padding: 1rem 1.2rem;
  width: 320px;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.re-modal h3 {
  font-size: 1rem;
  color: var(--text-primary);
}

.re-modal-row {
  display: flex;
  gap: 0.75rem;
}

.re-modal-row label {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  font-size: 0.72rem;
  color: var(--text-dim);
}

.re-check {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.78rem;
  color: var(--text-secondary);
  cursor: pointer;
}

.re-modal-note {
  font-size: 0.7rem;
  color: var(--text-dim);
}

.re-modal-actions {
  justify-content: flex-end;
}
</style>