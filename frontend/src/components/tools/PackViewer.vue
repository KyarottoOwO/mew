<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import * as THREE from 'three'
import { SkinView3d } from 'vue-skinview3d'
import { IdleAnimation, WalkingAnimation } from 'vue-skinview3d/animations'
import PackExporter from './PackExporter.vue'
import JsonUiStage from './JsonUiStage.vue'
import { GetPackListWithInfo, GetInstalledPacks, GetPackCacheList, GetPackPreviewInfo, GetPackArmorTextures, GetPackItemTextures, GetPackSkyTextures, GetPackSkySubpacks, GetPackSkySubpackTextures, GetPlayerSkinTexture, GetDefaultSkin, SaveDefaultSkin, GetCustomItems, SaveCustomItem, RemoveCustomItem, GetPackItemTextureNames, GetPackItemTexture, GetRemovedItems, SaveRemovedItem, RestoreRemovedItem, OpenFolder, DeleteInstalledPack, IsDebug, PackHasUiScreens, GetPackPreviewInfoFromCache, GetPackArmorTexturesFromCache, GetPackItemTexturesFromCache, GetPackSkyTexturesFromCache, GetPlayerSkinTextureFromCache, GetPackSkySubpacksFromCache, GetPackSkySubpackTexturesFromCache, GetPackItemTextureNamesFromCache, GetPackItemTextureFromCache, PackHasUiScreensFromCache, ExportCachePacks } from '../../../wailsjs/go/main/App'
import defaultSkinImg from '../../assets/default-skin.png'

import { parseBedrockCodes } from '../../utils/formatCodes'
const props = defineProps({ active: Boolean, openPackReq: { type: Object, default: null } })

const packList = ref([])
const packsPath = ref('')
const loading = ref(true)
const searchQuery = ref('')
const sortBy = ref('name')
const showServerPacks = ref(false)
const cachePackList = ref([])
const serverPacksPath = ref('')
const exportingServer = ref(false)

const showModal = ref(false)
const confirmDelete = ref(false)
const selectedPack = ref('')
const selectedPackInfo = ref({ name: '', description: '', iconURI: '' })
const selectedMaterial = ref('diamond')
const itemTextures = ref([])
const itemCache = new Map()

const materials = ['diamond', 'gold', 'iron', 'chain', 'cloth', 'netherite', 'naked']
const materialLabels = {
  diamond: 'Diamond', gold: 'Gold', iron: 'Iron',
  chain: 'Chainmail', cloth: 'Leather', netherite: 'Netherite', naked: 'Naked'
}
const skinModel = ref('auto-detect')
const animating = ref(false)

const customSkinURI = ref('')
const skinFileInput = ref(null)
const viewerRef = ref(null)
const modalContainer = ref(null)
const isDebug = ref(false)

const showMemPanel = ref(true)
const memLog = ref([])
const memTimer = ref(null)
const memLive = ref(null)

const SKY_DEBUG_KEY = 'mew-sky-debug-config'
const skyDebugFaces = reactive({
  0: { rotation: 0,   flipH: false },
  1: { rotation: 0,   flipH: false },
  2: { rotation: 0,   flipH: false },
  3: { rotation: 0,   flipH: false },
  4: { rotation: 90,  flipH: false },
  5: { rotation: 270, flipH: false },
})
const showSkyDebug = ref(false)

const customItemNames = ref([])
const removedItemNames = ref([])
const showItemPicker = ref(false)
const pickerNames = ref([])
const pickerTextures = reactive(new Map())
const pickerThumbs = reactive(new Map())
const pickerLoaded = reactive(new Set())
const pickerGridRef = ref(null)
const pickerLoading = ref(false)
const pickerSearch = ref('')
let pickerObserver = null

const hasScreens = ref(false)
// Which panel the pack modal shows: the 3d texture viewer or the JSON-UI renderer.
const viewerMode = ref('texture')

function toggleUiPanel() {
  if (viewerMode.value === 'ui') {
    viewerMode.value = 'texture'
    return
  }
  if (!selectedPack.value || !hasScreens.value) return
  viewerMode.value = 'ui'
}
let viewerInstance = null

const fullscreen = ref(false)
const fsViewerRef = ref(null)
const fsContainerRef = ref(null)
let fsViewerInstance = null

const showExportPopup = ref(false)

const IDLE_PAUSE_MS = 3000
const IDLE_GC_MS = 10000
const idleCleanups = new Set()
let fsIdleCleanup = null

// Cap the 3D renderer's pixel ratio to bound GPU/JS memory on HiDPI displays.
// skinview3d defaults to devicePixelRatio (up to 2-3x), which can ~4-9x memory usage.
const SKIN_PIXEL_RATIO = 1.5

function capPixelRatio(v) {
  if (!v || v.disposed) return
  try {
    if (v.pixelRatio !== SKIN_PIXEL_RATIO) v.pixelRatio = SKIN_PIXEL_RATIO
  } catch {}
}

let viewerTimer = null
let fsViewerTimer = null
let modelSetupAlive = true

const skySubpacks = ref([])
const skySlider = ref(0)

const SKY_FACE_META = [
  { key: 0, label: 'Left',     threeFace: '-X', bedrock: 'cubemap_0' },
  { key: 1, label: 'Front',    threeFace: '+Z', bedrock: 'cubemap_1' },
  { key: 2, label: 'Right',    threeFace: '+X', bedrock: 'cubemap_2' },
  { key: 3, label: 'Behind',   threeFace: '-Z', bedrock: 'cubemap_3' },
  { key: 4, label: 'Top',      threeFace: '+Y', bedrock: 'cubemap_4' },
  { key: 5, label: 'Bottom',   threeFace: '-Y', bedrock: 'cubemap_5' },
]

function loadSkyDebugConfig() {
  try {
    const raw = localStorage.getItem(SKY_DEBUG_KEY)
    if (!raw) return
    const cfg = JSON.parse(raw)
    for (const key of Object.keys(cfg)) {
      const i = parseInt(key)
      if (skyDebugFaces[i] && cfg[i]) {
        skyDebugFaces[i].rotation = cfg[i].rotation ?? skyDebugFaces[i].rotation
        skyDebugFaces[i].flipH = cfg[i].flipH ?? skyDebugFaces[i].flipH
      }
    }
  } catch {}
}

function saveSkyDebugConfig() {
  try {
    const cfg = {}
    for (const key of Object.keys(skyDebugFaces)) {
      cfg[key] = { rotation: skyDebugFaces[key].rotation, flipH: skyDebugFaces[key].flipH }
    }
    localStorage.setItem(SKY_DEBUG_KEY, JSON.stringify(cfg))
  } catch {}
}

function resetSkyDebugConfig() {
  skyDebugFaces[0].rotation = 0;   skyDebugFaces[0].flipH = false
  skyDebugFaces[1].rotation = 0;   skyDebugFaces[1].flipH = false
  skyDebugFaces[2].rotation = 0;   skyDebugFaces[2].flipH = false
  skyDebugFaces[3].rotation = 0;   skyDebugFaces[3].flipH = false
  skyDebugFaces[4].rotation = 90;  skyDebugFaces[4].flipH = false
  skyDebugFaces[5].rotation = 270; skyDebugFaces[5].flipH = false
  saveSkyDebugConfig()
  applySkyDebug()
}

function copySkyDebugConfig() {
  const cfg = {}
  for (const key of Object.keys(skyDebugFaces)) {
    cfg[key] = { rotation: skyDebugFaces[key].rotation, flipH: skyDebugFaces[key].flipH }
  }
  navigator.clipboard.writeText(JSON.stringify(cfg, null, 2)).catch(() => {})
}

function flipCanvasH(src) {
  if (!src) return null
  const c = document.createElement('canvas')
  const w = src.naturalWidth || src.width
  const h = src.naturalHeight || src.height
  c.width = w; c.height = h
  const ctx = c.getContext('2d')
  ctx.translate(w, 0)
  ctx.scale(-1, 1)
  ctx.drawImage(src, 0, 0)
  return c
}

let lastSkyTex = null
let currentSkyKey = null
let lastLoadedPack = null

function setLastSkyTex(tex) { lastSkyTex = tex }

const skyCache = new Map()
const skyPending = new Map()
const SKY_CACHE_CAP = 3
const ITEM_CACHE_CAP = 3
const SKY_FACE_CAP_MODAL = 512

function skyKey(key) {
  return key + (fullscreen.value ? ':fs' : ':m')
}

function skyCap() {
  return fullscreen.value ? null : SKY_FACE_CAP_MODAL
}

function skyBuild(key, tex) {
  return buildSkyCubemap(skyKey(key), tex, skyCap())
}

function pruneSkyCache() {
  if (skyCache.size <= SKY_CACHE_CAP) return
  const base = selectedPack.value + '#'
  const evictable = [...skyCache.keys()].filter(k => k !== 'default' && !k.startsWith(base))
  for (const k of evictable) {
    if (skyCache.size <= SKY_CACHE_CAP) break
    const t = skyCache.get(k)
    try { t.dispose() } catch {}
    skyCache.delete(k)
  }
}

function pruneItemCache() {
  if (itemCache.size <= ITEM_CACHE_CAP) return
  const cur = selectedPack.value
  const evictable = [...itemCache.keys()].filter(k => k !== cur)
  for (const k of evictable) {
    if (itemCache.size <= ITEM_CACHE_CAP) break
    itemCache.delete(k)
  }
}

async function buildSkyCubemap(key, tex, cap) {
  if (skyCache.has(key)) return skyCache.get(key)
  if (skyPending.has(key)) return skyPending.get(key)
  const pending = generateSkyCubemap(tex, cap).then(built => {
    skyPending.delete(key)
    if (skyCache.has(key)) {
      try { built.dispose() } catch {}
      return skyCache.get(key)
    }
    skyCache.set(key, built)
    pruneSkyCache()
    return built
  })
  skyPending.set(key, pending)
  return pending
}

function setSkyOnViewers(cubemap) {
  if (viewerInstance) viewerInstance.scene.background = cubemap
  if (fsViewerInstance) fsViewerInstance.scene.background = cubemap
}

function invalidateSkyCache(pack) {
  for (const k of [...skyCache.keys()]) {
    if (k.startsWith(pack + '#')) {
      const t = skyCache.get(k)
      try { t.dispose() } catch {}
      skyCache.delete(k)
    }
  }
  for (const k of [...skyPending.keys()]) {
    if (k.startsWith(pack + '#')) skyPending.delete(k)
  }
}

const textureCache = new Map()
const texturePending = new Map()
const TEXTURE_CACHE_CAP = 12

function pruneTextureCache() {
  if (textureCache.size <= TEXTURE_CACHE_CAP) return
  const cur = selectedPack.value
  const evictable = [...textureCache.keys()].filter(k => !cur || !k.startsWith(cur + '|'))
  for (const k of evictable) {
    if (textureCache.size <= TEXTURE_CACHE_CAP) break
    textureCache.delete(k)
  }
}

function invalidateTextureCache(pack) {
  const prefix = pack + '|'
  for (const k of [...textureCache.keys()]) {
    if (k.startsWith(prefix)) textureCache.delete(k)
  }
  for (const k of [...texturePending.keys()]) {
    if (k.startsWith(prefix)) texturePending.delete(k)
  }
}

function cachedTextureFetch(key, fetcher) {
  if (textureCache.has(key)) return Promise.resolve(textureCache.get(key))
  if (texturePending.has(key)) return texturePending.get(key)
  const p = Promise.resolve().then(fetcher).then(v => {
    texturePending.delete(key)
    if (textureCache.has(key)) return textureCache.get(key)
    textureCache.set(key, v)
    pruneTextureCache()
    return v
  })
  texturePending.set(key, p)
  return p
}


async function applySkyDebug() {
  if (!viewerInstance) return
  const cubemap = await skyBuild(selectedPack.value + '#current', lastSkyTex)
  currentSkyKey = skyKey(selectedPack.value + '#current')
  setSkyOnViewers(cubemap)
}

function loadImageToCanvas(uri) {
  return new Promise(resolve => {
    if (!uri) return resolve(null)
    const img = new Image(); img.crossOrigin = 'anonymous'
    img.onload = () => resolve(img)
    img.onerror = () => resolve(null)
    img.src = uri
  })
}

async function generateSkyCubemap(skyTex, cap) {
  function makeCanvas(s, draw) {
    const c = document.createElement('canvas')
    c.width = s; c.height = s
    draw(c.getContext('2d'), s)
    return c
  }

  function skyGradient(ctx, s, topColor, bottomColor) {
    const g = ctx.createLinearGradient(0, 0, 0, s)
    g.addColorStop(0, topColor)
    g.addColorStop(1, bottomColor)
    ctx.fillStyle = g
    ctx.fillRect(0, 0, s, s)
  }

  function fallbackFace(s, topColor, bottomColor) {
    return makeCanvas(s, (ctx, sz) => {
      skyGradient(ctx, sz, topColor, bottomColor)
      for (let i = 0; i < 30; i++) {
        ctx.fillStyle = `rgba(255,255,255,${0.2 + Math.random() * 0.6})`
        ctx.fillRect(Math.random() * sz, Math.random() * sz, 1 + Math.random() * 2, 1 + Math.random() * 2)
      }
    })
  }

  // Three.js CubeTexture order: +X, -X, +Y, -Y, +Z, -Z
  // Bedrock: 0=left, 1=north(behind player), 2=right, 3=behind camera, 4=top, 5=bottom
  function rotateCanvas(src, degrees) {
    if (!src) return null
    const c = document.createElement('canvas')
    const w = src.naturalWidth || src.width
    const h = src.naturalHeight || src.height
    const rad = degrees * Math.PI / 180
    const swap = degrees === 90 || degrees === 270
    c.width = swap ? h : w
    c.height = swap ? w : h
    const ctx = c.getContext('2d')
    ctx.translate(c.width / 2, c.height / 2)
    ctx.rotate(rad)
    ctx.drawImage(src, -w / 2, -h / 2)
    return c
  }

  const [c0, c1, c2, c3, c4, c5] = await Promise.all([
    loadImageToCanvas(skyTex?.cubemap0),
    loadImageToCanvas(skyTex?.cubemap1),
    loadImageToCanvas(skyTex?.cubemap2),
    loadImageToCanvas(skyTex?.cubemap3),
    loadImageToCanvas(skyTex?.cubemap4),
    loadImageToCanvas(skyTex?.cubemap5),
  ])

  const rawFaces = [c0, c1, c2, c3, c4, c5]
  const xfaces = rawFaces.map((face, i) => {
    let r = face
    const cfg = skyDebugFaces[i]
    if (cfg.rotation) r = rotateCanvas(r, cfg.rotation)
    if (cfg.flipH) r = flipCanvasH(r)
    return r
  })

  const faceMap = [
    xfaces[2], // +X = right = cubemap_2
    xfaces[0], // -X = left = cubemap_0
    xfaces[4], // +Y = top = cubemap_4
    xfaces[5], // -Y = bottom = cubemap_5
    xfaces[1], // +Z = front = cubemap_1
    xfaces[3], // -Z = behind = cubemap_3
  ]

  const faceDims = faceMap.map(img => img ? (img.naturalWidth || img.width) : 0)
  const native = Math.max(...faceDims, 128)
  const size = (cap && Number.isFinite(cap)) ? Math.min(native, cap) : native

  const fallbacks = [
    fallbackFace(size, '#0e1e3d', '#3a7cc2'),
    fallbackFace(size, '#0c1a35', '#3a7cc2'),
    fallbackFace(size, '#070d1f', '#1a4a8a'),
    fallbackFace(size, '#7ec8e3', '#dceefb'),
    fallbackFace(size, '#10203f', '#2e6db3'),
    fallbackFace(size, '#0b1630', '#2e6db3'),
  ]

  const cubeFaces = faceMap.map((img, i) => {
    if (!img) return fallbacks[i]
    const c = document.createElement('canvas')
    c.width = size
    c.height = size
    const ctx = c.getContext('2d')
    ctx.drawImage(img, 0, 0, size, size)
    return c
  })

  const cubeTexture = new THREE.CubeTexture(cubeFaces)
  cubeTexture.needsUpdate = true
  return cubeTexture
}

const currentAnimation = computed(() => {
  return animating.value ? new WalkingAnimation() : new IdleAnimation()
})

const skinOptions = computed(() => ({
  model: skinModel.value,
  ears: false
}))

const filteredPickerItems = computed(() => {
  const q = pickerSearch.value.trim().toLowerCase()
  if (!q) return pickerNames.value
  return pickerNames.value.filter(name => name.toLowerCase().includes(q))
})

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

let gcTimer = null
let lastActivity = Date.now()
let idleGCArmed = false
let gcWatchdog = null
let gcIdleEvents = null

function markActivity() {
  lastActivity = Date.now()
}

function armIdleGC() {
  idleGCArmed = true
}

function scheduleIdleGC(delayMs, reason) {
  if (typeof gc !== 'function') return
  if (gcTimer) return
  gcTimer = setTimeout(() => {
    gcTimer = null
    try {
      gc()
      if (isDebug.value) console.log('[gc] collected' + (reason ? ' (' + reason + ')' : ''))
    } catch {}
    recordMem(reason ? 'gc ' + reason : 'gc')
  }, delayMs == null ? 4000 : delayMs)
}

function startGCWatchdog() {
  stopGCWatchdog()
  gcWatchdog = setInterval(() => {
    if (!idleGCArmed) return
    if (Date.now() - lastActivity < IDLE_GC_MS) return
    idleGCArmed = false
    scheduleIdleGC(null, 'idle')
  }, 2000)
  gcIdleEvents = ['pointerdown', 'pointermove', 'wheel', 'touchstart', 'touchmove', 'keydown']
  for (const evt of gcIdleEvents) window.addEventListener(evt, markActivity, { passive: true })
}

function stopGCWatchdog() {
  if (gcWatchdog) { clearInterval(gcWatchdog); gcWatchdog = null }
  if (gcIdleEvents) {
    for (const evt of gcIdleEvents) window.removeEventListener(evt, markActivity)
    gcIdleEvents = null
  }
}

function measureMem() {
  let heap = null
  let heapTotal = null
  let heapLimit = null
  try {
    const m = performance.memory
    if (m) {
      heap = m.usedJSHeapSize
      heapTotal = m.totalJSHeapSize
      heapLimit = m.jsHeapSizeLimit
    }
  } catch {}

  let nodeCount = 0
  try { nodeCount = document.querySelectorAll('*').length } catch {}

  let canvasBytes = 0
  let canvasCount = 0
  for (const c of document.querySelectorAll('canvas')) {
    if (c.width > 1 && c.height > 1) {
      canvasCount++
      canvasBytes += c.width * c.height * 4
    }
  }

  let imgBytes = 0
  let imgCount = 0
  for (const img of document.querySelectorAll('.pv-item-img, .pv-picker-item-img, .pv-fs-3d canvas, .pv-thumb, .pack-card img, .pv-pack-icon')) {
    const w = img.naturalWidth || img.width
    const h = img.naturalHeight || img.height
    if (w > 1 && h > 1) {
      imgCount++
      imgBytes += w * h * 4
    }
  }

  let allImgBytes = 0
  let allImgCount = 0
  for (const img of document.querySelectorAll('img')) {
    const w = img.naturalWidth || img.width
    const h = img.naturalHeight || img.height
    if (w > 1 && h > 1) {
      allImgCount++
      allImgBytes += w * h * 4
    }
  }

  return { heap, canvasBytes, canvasCount, imgBytes, imgCount, allImgBytes, allImgCount }
}

function cachePinStats() {
  let itemTexBytes = 0
  const it = itemTextures.value || []
  for (const itm of it) {
    if (itm && itm.dataURI) itemTexBytes += itm.dataURI.length
  }
  let skyBytes = 0
  for (const t of skyCache.values()) {
    if (t && t.image) {
      for (const c of t.image) {
        if (c) skyBytes += (c.naturalWidth || c.width || 0) * (c.naturalHeight || c.height || 0) * 4
      }
    }
  }
  let itemBytes = 0
  for (const arr of itemCache.values()) {
    if (Array.isArray(arr)) {
      for (const itm of arr) {
        if (itm && itm.dataURI) itemBytes += itm.dataURI.length
      }
    }
  }
  return {
    skyCount: skyCache.size,
    skyBytes,
    itemCacheCount: itemCache.size,
    itemCacheBytes: itemBytes,
    itemTexCount: it.length,
    itemTexBytes,
  }
}

function recordMem(label) {
  if (!isDebug.value) return
  const s = measureMem()
  const p = cachePinStats()
  const entry = {
    t: new Date().toLocaleTimeString(),
    label,
    heap: s.heap,
    canvasBytes: s.canvasBytes,
    canvasCount: s.canvasCount,
    imgBytes: s.imgBytes,
    imgCount: s.imgCount,
    skyCount: p.skyCount,
    skyBytes: p.skyBytes,
    itemCacheCount: p.itemCacheCount,
    itemCacheBytes: p.itemCacheBytes,
    itemTexCount: p.itemTexCount,
    itemTexBytes: p.itemTexBytes,
    allImgCount: s.allImgCount,
    allImgBytes: s.allImgBytes,
  }
  const log = memLog.value
  log.push(entry)
  if (log.length > 60) log.shift()
  memLog.value = log
}

async function memDump() {
  const s = measureMem()
  const p = cachePinStats()
  const logRows = (memLog.value || []).slice(-30).map(e =>
    e.t + ' ' + e.label +
    ' heap ' + formatSize(e.heap) +
    ' sky ' + e.skyCount + ' (' + formatSize(e.skyBytes) + ')' +
    ' itemC ' + e.itemCacheCount + ' (' + formatSize(e.itemCacheBytes) + ')' +
    ' allI ' + e.allImgCount + ' (' + formatSize(e.allImgBytes) + ')'
  )
  const uaLines = []
  try {
    if (typeof performance.measureUserAgentSpecificMemory === 'function') {
      const r = await performance.measureUserAgentSpecificMemory()
      uaLines.push('UA memory total: ' + formatSize(r.bytes))
      const byType = {}
      for (const b of r.breakdown || []) {
        if (b.bytes > 0) byType[b.type || 'unknown'] = (byType[b.type] || 0) + b.bytes
      }
      for (const k of Object.keys(byType).sort((a, b) => byType[b] - byType[a])) {
        uaLines.push('  ' + k + ': ' + formatSize(byType[k]))
      }
    }
  } catch {}
  const lines = [
    '=== MEMORY DUMP ' + new Date().toISOString() + ' ===',
    'heap: ' + formatSize(s.heap) + (s.heap != null ? ' (' + Math.round(s.heap / 1048576) + ' MB)' : ''),
    'canvases: ' + s.canvasCount + ' / ' + formatSize(s.canvasBytes),
    'decoded imgs: ' + s.imgCount + ' / ' + formatSize(s.imgBytes),
    'ALL imgs in DOM: ' + (s.allImgCount != null ? s.allImgCount + ' / ' + formatSize(s.allImgBytes) : 'n/a'),
    'itemTextures (grid): ' + p.itemTexCount + ' / ' + formatSize(p.itemTexBytes),
    'itemCache: ' + p.itemCacheCount + ' pack(s) / ' + formatSize(p.itemCacheBytes),
    'skyCache: ' + p.skyCount + ' cubemap(s) / ' + formatSize(p.skyBytes),
    'pickerTextures: ' + pickerTextures.size,
    'pickerNames: ' + pickerNames.value.length,
    'packList: ' + (packList.value || []).length,
    'packsPath: ' + (packsPath.value || ''),
    ...uaLines,
    '--- recent log ---',
    ...(logRows.length ? logRows : ['(empty)']),
    '=== END DUMP ===',
  ]
  const text = lines.join('\n')
  try { await navigator.clipboard.writeText(text) } catch {}
  console.log(text)
  recordMem('MEM DUMP')
}

function resetMem() {
  if (!confirm('Reset memory? This clears all caches/viewers and reloads the app.')) return
  if (viewerTimer) { clearTimeout(viewerTimer); viewerTimer = null }
  if (fsViewerTimer) { clearTimeout(fsViewerTimer); fsViewerTimer = null }
  clearIdlePause()
  disposeViewerResources(viewerInstance)
  disposeViewerResources(fsViewerInstance)
  viewerInstance = null
  fsViewerInstance = null
  itemCache.clear()
  itemTextures.value = []
  clearSkyCache()
  pickerTextures.clear()
  pickerLoaded.clear()
  window.location.reload()
}

let memTimerHandle = null
function startMemSampler() {
  stopMemSampler()
  const tick = () => {
    const s = measureMem()
    const p = cachePinStats()
    memLive.value = {
      t: new Date().toLocaleTimeString(),
      heap: s.heap,
      canvasBytes: s.canvasBytes,
      canvasCount: s.canvasCount,
      imgBytes: s.imgBytes,
      imgCount: s.imgCount,
      skyCount: p.skyCount,
      skyBytes: p.skyBytes,
      itemCacheCount: p.itemCacheCount,
      itemCacheBytes: p.itemCacheBytes,
      itemTexCount: p.itemTexCount,
      itemTexBytes: p.itemTexBytes,
      allImgCount: s.allImgCount,
      allImgBytes: s.allImgBytes,
      modal: !!viewerInstance,
      fs: !!fsViewerInstance,
      packs: (packList.value || []).length,
    }
    memTimerHandle = setTimeout(tick, 1000)
  }
  tick()
}
function stopMemSampler() {
  if (memTimerHandle) { clearTimeout(memTimerHandle); memTimerHandle = null }
}

watch(showMemPanel, (on) => {
  if (on && isDebug.value) startMemSampler()
  else stopMemSampler()
})

const filteredPacks = computed(() => {
  const list = showServerPacks.value ? cachePackList.value : packList.value
  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    return list.filter(p => {
      const name = (p.name || '').toLowerCase()
      const desc = (p.description || '').toLowerCase()
      const dir = (p.dirName || '').toLowerCase()
      return name.includes(q) || desc.includes(q) || dir.includes(q)
    })
  }
  const sorted = [...list]
  switch (sortBy.value) {
    case 'name':
      sorted.sort((a, b) => (a.name || '').localeCompare(b.name || ''))
      break
    case 'date':
      sorted.sort((a, b) => (b.modTime || '').localeCompare(a.modTime || ''))
      break
    case 'size':
      sorted.sort((a, b) => (b.size || 0) - (a.size || 0))
      break
  }
  return sorted
})

const ARMOR_LAYER1_BONES = [
  { name: 'helmet',        size: [8, 8, 8],   pos: [0, 4, 0],  inflate: 1, uv: [0, 0] },
  { name: 'chestplate',    size: [8, 12, 4],  pos: [0, 0, 0],  inflate: 1, uv: [16, 16] },
  { name: 'rightArmArmor', size: [4, 12, 4],  pos: [-1, -4, 0], inflate: 1.01, uv: [40, 16], flipX: true },
  { name: 'leftArmArmor',  size: [4, 12, 4],  pos: [1, -4, 0],  inflate: 1.01, uv: [40, 16], mirror: true },
  { name: 'rightLeg',      size: [4, 12, 4],  pos: [0, -6, 0],  inflate: 1, uv: [0, 16], flipX: true },
  { name: 'leftLeg',       size: [4, 12, 4],  pos: [0, -6, 0],  inflate: 1, uv: [0, 16], mirror: true },
]

const ARMOR_LAYER2_BONES = [
  { name: 'TopOfLeggings', size: [8, 4, 4.1], pos: [0, -4, 0], inflate: 0.5, uv: [16, 23] },
  { name: 'rightLegOverlay', size: [4, 12, 4], pos: [0, -6, 0], inflate: 0.5, uv: [0, 16], flipX: true },
  { name: 'leftLegOverlay',  size: [4, 12, 4], pos: [0, -6, 0], inflate: 0.5, uv: [0, 16], mirror: true },
]

const ARMOR_ATTACH_MAP = {
  helmet: 'head',
  chestplate: 'body',
  rightArmArmor: 'rightArm',
  leftArmArmor: 'leftArm',
  rightLeg: 'rightLeg',
  leftLeg: 'leftLeg',
  rightLegOverlay: 'rightLeg',
  leftLegOverlay: 'leftLeg',
  TopOfLeggings: 'body',
}

const FACE_ORDER = ['right', 'left', 'top', 'bottom', 'front', 'back']

function boxUV(u, v, W, H, D, flipX) {
  const leftU = flipX ? u + D + W : u
  const rightU = flipX ? u : u + D + W
  return {
    right:  { u: rightU,    v: v + D,     w: D, h: H },
    left:   { u: leftU,     v: v + D,     w: D, h: H },
    top:    { u: u + D,     v: v,         w: W, h: D },
    bottom: { u: u + D + W, v: v,         w: W, h: D },
    front:  { u: u + D,     v: v + D,     w: W, h: H },
    back:   { u: u + D + W + D, v: v + D, w: W, h: H },
  }
}

function safeCrop(img, face) {
  const canvas = document.createElement('canvas')
  const iw = img.naturalWidth || img.width
  const ih = img.naturalHeight || img.height
  const sx = Math.max(0, Math.min(face.u, iw))
  const sy = Math.max(0, Math.min(face.v, ih))
  const sw = Math.max(0, Math.min(face.w, iw - sx))
  const sh = Math.max(0, Math.min(face.h, ih - sy))
  if (sw <= 0 || sh <= 0) { canvas.width = 1; canvas.height = 1; return canvas }
  canvas.width = sw; canvas.height = sh
  const ctx = canvas.getContext('2d')
  ctx.imageSmoothingEnabled = false
  ctx.drawImage(img, sx, sy, sw, sh, 0, 0, sw, sh)
  return canvas
}

function buildMeshFromBone(img, bone, isArmor, renderOrder) {
  const [W, H, D] = bone.size
  const inflate = bone.inflate || 0
  const finalSize = [W + inflate * 2, H + inflate * 2, D + inflate * 2]
  const iw = img.naturalWidth || img.width
  const ih = img.naturalHeight || img.height
  const sx = iw / 64
  const isModern = ih > 32 && ih > iw / 2
  const sy = isModern ? ih / 64 : ih / 32
  const u = bone.uv[0], v = bone.uv[1]
  const faces = boxUV(u * sx, v * sy, W * sx, H * sy, D * sx, !!bone.flipX)
  const materials = []
  for (const fname of FACE_ORDER) {
    let canvas = safeCrop(img, faces[fname])
    if (bone.mirror) {
      const flipped = document.createElement('canvas')
      flipped.width = canvas.width; flipped.height = canvas.height
      const fctx = flipped.getContext('2d')
      fctx.translate(canvas.width, 0); fctx.scale(-1, 1)
      fctx.drawImage(canvas, 0, 0); canvas = flipped
    }
    const tex = new THREE.CanvasTexture(canvas)
    tex.magFilter = THREE.NearestFilter; tex.minFilter = THREE.NearestFilter
    const opts = { map: tex, side: THREE.FrontSide }
    if (isArmor) {
      opts.side = THREE.DoubleSide
      opts.alphaTest = 0.1
      opts.depthWrite = true
    } else {
      opts.alphaTest = 0.1
    }
    materials.push(new THREE.MeshStandardMaterial(opts))
  }
  const geo = new THREE.BoxGeometry(finalSize[0], finalSize[1], finalSize[2])
  const mesh = new THREE.Mesh(geo, materials)
  mesh.position.set(bone.pos[0], bone.pos[1], bone.pos[2])
  mesh.renderOrder = renderOrder
  return mesh
}

function disposeMesh(mesh) {
  if (!mesh) return
  if (mesh.geometry) mesh.geometry.dispose()
  if (Array.isArray(mesh.material)) {
    mesh.material.forEach(m => { if (m.map) m.map.dispose(); m.dispose() })
  } else if (mesh.material) { mesh.material.dispose() }
}

function disposeViewerResources(v) {
  if (!v) return
  removeArmorMeshes(v)
  v.scene.background = null
  if (!v.disposed) {
    try { v.dispose() } catch {}
  }
  try {
    const gl = v.renderer && v.renderer.getContext && v.renderer.getContext()
    const lose = gl && gl.getExtension('WEBGL_lose_context')
    if (lose) lose.loseContext()
  } catch {}
  skinLoadKeys.delete(v)
}

// Don't re-decode the same skin repeatedly: navigating between packs (and
// other triggers) would otherwise re-load an identical skin URI just because a
// different pack was selected, pinning a fresh decoded image each time.
const skinLoadKeys = new Map()
function loadSkinOnce(v, uri, model) {
  if (!v || v.disposed) return false
  const key = (uri || '') + '\u0001' + (model || '')
  if (skinLoadKeys.get(v) === key) return false
  skinLoadKeys.set(v, key)
  return true
}

function clearIdlePause() {
  for (const c of idleCleanups) { try { c() } catch {} }
  idleCleanups.clear()
  if (fsIdleCleanup) { try { fsIdleCleanup() } catch {}; fsIdleCleanup = null }
  setViewerPaused(viewerInstance, false)
  setViewerPaused(fsViewerInstance, false)
}

function setViewerPaused(v, paused) {
  if (!v || v.disposed) return
  v.renderPaused = paused
}

function resumeViewers() {
  setViewerPaused(viewerInstance, false)
  setViewerPaused(fsViewerInstance, false)
}

function setupIdlePause(v, cleanupSet) {
  if (!v || v.disposed) return
  let timer = null
  const resume = () => {
    setViewerPaused(v, false)
    clearTimeout(timer)
    timer = setTimeout(() => setViewerPaused(v, true), IDLE_PAUSE_MS)
  }
  const evts = ['pointerdown', 'pointermove', 'wheel', 'touchstart', 'touchmove']
  for (const evt of evts) v.canvas.addEventListener(evt, resume, { passive: true })
  setViewerPaused(v, true)
  const cleanup = () => {
    clearTimeout(timer)
    for (const evt of evts) v.canvas.removeEventListener(evt, resume)
  }
  if (cleanupSet) cleanupSet.add(cleanup)
  return cleanup
}

function clearSkyCache() {
  for (const t of skyCache.values()) {
    try { t.dispose() } catch {}
  }
  skyCache.clear()
  skyPending.clear()
}

function loadImage(uri) {
  return new Promise(resolve => {
    if (!uri) return resolve(null)
    const img = new Image(); img.crossOrigin = 'anonymous'
    img.onload = () => resolve(img)
    img.onerror = () => resolve(null)
    img.src = uri
  })
}

// Pack grid shows icons at 56px; decoding the full-res pack_icon for every card
// wastes a lot of memory, so we downscale once and drop the original data URI.
const PACK_ICON_THUMB = 64
async function thumbifyPackIcons(list) {
  for (let i = 0; i < list.length; i++) {
    const p = list[i]
    if (p && p.iconURI) {
      try { p.iconURI = await toThumb(p.iconURI, PACK_ICON_THUMB) } catch {}
    }
    if ((i & 7) === 7) await new Promise(r => setTimeout(r, 0))
  }
}

async function toThumb(dataURI, size = 64) {
  try {
    const img = await loadImage(dataURI)
    if (!img) return dataURI
    const c = document.createElement('canvas')
    c.width = size; c.height = size
    const ctx = c.getContext('2d')
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(img, 0, 0, size, size)
    return c.toDataURL('image/png')
  } catch {
    return dataURI
  }
}

function buildArmor(v, tex1, tex2) {
  if (!v || v.disposed) return
  const skin = v.playerObject.skin
  const allBones = [
    ...(tex1 ? ARMOR_LAYER1_BONES.map(b => [b, tex1]) : []),
    ...(tex2 ? ARMOR_LAYER2_BONES.map(b => [b, tex2]) : []),
  ]
  for (const [bone, tex] of allBones) {
    const mesh = buildMeshFromBone(tex, bone, true, 10)
    mesh.name = bone.name + '_armor'
    const targetPart = skin[ARMOR_ATTACH_MAP[bone.name]]
    if (targetPart) targetPart.add(mesh)
  }
}

function removeArmorMeshes(v) {
  if (!v || v.disposed) return
  const skin = v.playerObject.skin
  for (const partName of Object.keys(skin)) {
    const part = skin[partName]
    if (!part || !part.children) continue
    const toRemove = part.children.filter(c => c.name && c.name.endsWith('_armor'))
    for (const c of toRemove) { part.remove(c); disposeMesh(c) }
  }
}

function getViewerSize() {
  if (modalContainer.value) {
    return { w: modalContainer.value.clientWidth, h: 400 }
  }
  return { w: 560, h: 400 }
}

function configureViewer() {
  if (!viewerInstance) return
  capPixelRatio(viewerInstance)
  viewerInstance.camera.position.set(0, 8, 40)
  viewerInstance.controls.target.set(0, 2, 0)
  viewerInstance.controls.enableDamping = true
  viewerInstance.controls.dampingFactor = 0.08
  viewerInstance.controls.minDistance = 15
  viewerInstance.controls.maxDistance = 80
  viewerInstance.controls.update()
}

const fsIndex = ref(0)

function openFullscreen() {
  const idx = filteredPacks.value.findIndex(p => p.dirName === selectedPack.value)
  fsIndex.value = idx >= 0 ? idx : 0
  fullscreen.value = true
  nextTick(() => {
    measureFs()
    applyAppearanceFs()
    recordMem('fs open')
  })
}

async function closeFullscreen() {
  const wasFsKey = currentSkyKey
  const baseKey = wasFsKey && wasFsKey.startsWith(selectedPack.value + '#')
    ? wasFsKey.replace(/:fs$/, '')
    : (selectedPack.value ? selectedPack.value + '#base' : null)
  fullscreen.value = false
  if (baseKey && lastSkyTex) {
    try {
      const cubemap = await skyBuild(baseKey, lastSkyTex)
      currentSkyKey = skyKey(baseKey)
      if (currentSkyKey !== wasFsKey) setSkyOnViewers(cubemap)
    } catch {}
  }
  if (wasFsKey && wasFsKey.endsWith(':fs')) {
    const t = skyCache.get(wasFsKey)
    if (t) { try { t.dispose() } catch {}; skyCache.delete(wasFsKey) }
    skyPending.delete(wasFsKey)
  }
  recordMem('fs close')
}

function fsPrev() {
  navPack(-1)
}

function fsNext() {
  navPack(1)
}

let lastNavAt = 0

function navPack(delta) {
  if (filteredPacks.value.length === 0) return
  const now = Date.now()
  if (now - lastNavAt < 500) return
  lastNavAt = now
  doNavPack(delta)
}

async function doNavPack(delta) {
  const curIdx = filteredPacks.value.findIndex(p => p.dirName === selectedPack.value)
  const base = curIdx >= 0 ? curIdx : fsIndex.value
  const idx = (base + delta + filteredPacks.value.length) % filteredPacks.value.length
  const pack = filteredPacks.value[idx]
  if (!pack) return
  fsIndex.value = idx
  selectedPack.value = pack.dirName
  selectedMaterial.value = selectedMaterial.value
  showModal.value = true
  await nextTick()
  await loadPackData(pack.dirName)
  recordMem(fullscreen.value ? 'fs nav' : 'nav')
}

function onFsNavKey(e) {
  if (!fullscreen.value) return
  if (e.key === 'ArrowLeft') { e.preventDefault(); fsPrev() }
  else if (e.key === 'ArrowRight') { e.preventDefault(); fsNext() }
  else if (e.key === 'Escape') closeFullscreen()
}

watch(fullscreen, (val) => {
  if (val) window.addEventListener('keydown', onFsNavKey)
  else window.removeEventListener('keydown', onFsNavKey)
})

const fsSize = ref({ w: 0, h: 0 })

function measureFs() {
  if (fsContainerRef.value) {
    const r = fsContainerRef.value.getBoundingClientRect()
    fsSize.value = { w: Math.round(r.width), h: Math.round(r.height) }
  }
}

watch(fsViewerRef, (newRef) => {
  if (!newRef || !modelSetupAlive) return
  if (fsViewerTimer) { clearTimeout(fsViewerTimer); fsViewerTimer = null }
  const tryGetViewer = () => {
    if (!modelSetupAlive) return
    const v = newRef.viewer
    if (v) {
      if (v.disposed) return
      fsViewerTimer = null
      fsViewerInstance = v
      capPixelRatio(v)
      if (fsIdleCleanup) { fsIdleCleanup(); fsIdleCleanup = null }
      fsIdleCleanup = setupIdlePause(v, null)
      v.camera.position.set(0, 8, 40)
      v.controls.target.set(0, 2, 0)
      v.controls.enableDamping = true
      v.controls.dampingFactor = 0.08
      applyAppearanceFs()
    } else {
      fsViewerTimer = setTimeout(tryGetViewer, 50)
    }
  }
  tryGetViewer()
})

async function applyAppearanceFs() {
  if (!fsViewerInstance) return
  const uri = customSkinURI.value || defaultSkinImg
  const model = skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value
  if (loadSkinOnce(fsViewerInstance, uri, model)) {
    await fsViewerInstance.loadSkin(uri, { model })
  }
  const cached = currentSkyKey ? skyCache.get(currentSkyKey) : null
  const cubemap = cached || await skyBuild(selectedPack.value + '#base', lastSkyTex)
  currentSkyKey = skyKey(selectedPack.value + '#base')
  setSkyOnViewers(cubemap)
  await applyArmor(selectedPack.value, selectedMaterial.value)
  resumeViewers()
}

watch(viewerRef, (newRef) => {
  if (!newRef || !modelSetupAlive) return
  if (viewerTimer) { clearTimeout(viewerTimer); viewerTimer = null }
  const tryGetViewer = () => {
    if (!modelSetupAlive) return
    const v = newRef.viewer
    if (v) {
      if (v.disposed) return
      viewerTimer = null
      viewerInstance = v
      setupIdlePause(v, idleCleanups)
      configureViewer()
      const setupViewer = async () => {
        try {
          const defSkin = await GetDefaultSkin()
          if (defSkin) customSkinURI.value = defSkin
        } catch {}
        if (viewerInstance && !viewerInstance.disposed) {
          const uri = customSkinURI.value || defaultSkinImg
          await viewerInstance.loadSkin(uri, { model: skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value })
          setSkyOnViewers(await buildSkyCubemap('default', null))
          if (selectedPack.value) await applyArmor(selectedPack.value, selectedMaterial.value)
          resumeViewers()
        }
      }
      setupViewer()
    } else {
      viewerTimer = setTimeout(tryGetViewer, 50)
    }
  }
  tryGetViewer()
})

async function loadAllPacks() {
  try {
    const [list, info] = await Promise.all([GetPackListWithInfo(), GetInstalledPacks()])
    packsPath.value = info?.path || ''
    packList.value = (list || []).map(p => ({
      ...p,
      dirName: p.dirName || '',
    }))
    thumbifyPackIcons(packList.value)
  } catch (e) {
    console.error('Failed to load packs:', e)
  }
  try {
    const cacheSources = await GetPackCacheList()
    if (cacheSources && cacheSources.length > 0) {
      serverPacksPath.value = cacheSources[0].path || ''
      const cacheList = await GetPackCacheList(cacheSources[0].path)
      cachePackList.value = (cacheList || []).map(p => ({
        ...p,
        dirName: p.dirName || '',
      }))
      thumbifyPackIcons(cachePackList.value)
    }
  } catch (e) {
    console.error('Failed to load server packs:', e)
  }
  loading.value = false
}

function openPackFolder() {
  const basePath = showServerPacks.value ? serverPacksPath.value : packsPath.value
  if (!basePath || !selectedPack.value) return
  const sep = basePath.includes('\\') ? '\\' : '/'
  OpenFolder(basePath + sep + selectedPack.value)
}

async function deletePack() {
  if (!selectedPack.value) return
  if (!confirmDelete.value) {
    confirmDelete.value = true
    setTimeout(() => { confirmDelete.value = false }, 3000)
    return
  }
  const name = selectedPack.value
  confirmDelete.value = false
  try {
    await DeleteInstalledPack(name)
    invalidateTextureCache(name)
    closeModal()
    await loadAllPacks()
  } catch (e) {
    console.error('Failed to delete pack:', e)
  }
}

async function openPack(packName) {
  selectedPack.value = packName
  selectedMaterial.value = 'diamond'
  showModal.value = true
  await nextTick()
  await loadCustomItems()
  await waitForViewer()
  await loadPackData(packName)
}

async function openCachePack(packName) {
  closeModal()
  selectedPack.value = packName
  selectedMaterial.value = 'diamond'
  showModal.value = true
  await nextTick()
  await loadCustomItems()
  await waitForViewer()
  await loadCachePackData(packName)
}

async function loadCachePackData(packName) {
  try {
    hasScreens.value = false
    if (lastLoadedPack && lastLoadedPack !== packName) {
      invalidateSkyCache(lastLoadedPack)
      itemCache.delete(lastLoadedPack)
      invalidateTextureCache(lastLoadedPack)
    }
    lastLoadedPack = packName
    const basePath = serverPacksPath.value
    const [info, skyTex, packHasScreens] = await Promise.all([
      GetPackPreviewInfoFromCache(packName, basePath),
      cachedTextureFetch(packName + '|sky', () => GetPackSkyTexturesFromCache(packName, basePath)),
      PackHasUiScreensFromCache(packName, basePath),
    ])
    hasScreens.value = !!packHasScreens
    if (!hasScreens.value) viewerMode.value = 'texture'
    recordMem('stage:info')
    selectedPackInfo.value = info
    setLastSkyTex(skyTex)
    const cubemap = await skyBuild(packName + '#base', skyTex)
    recordMem('stage:sky')
    currentSkyKey = skyKey(packName + '#base')
    setSkyOnViewers(cubemap)
    textureCache.delete(packName + '|sky')
    await loadCacheSkin(packName, basePath)
    recordMem('stage:skin')
    await loadCacheArmor(packName, basePath, selectedMaterial.value)
    recordMem('stage:armor')
    await loadCacheItems(packName, basePath)
    recordMem('stage:items')
    skySubpacks.value = (await GetPackSkySubpacksFromCache(packName, basePath)) || []
    skySlider.value = 0
    resumeViewers()
    recordMem('load cache pack')
    armIdleGC()
    scheduleIdleGC(null, 'load cache pack')
  } catch (e) {
    console.error('Failed to load cache pack data:', e)
  }
}

async function loadCacheSkin(packName, basePath) {
  if (!viewerInstance) return
  const skinURI = customSkinURI.value || await cachedTextureFetch(packName + '|skin', () => GetPlayerSkinTextureFromCache(packName, basePath)) || defaultSkinImg
  const model = skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value
  if (loadSkinOnce(viewerInstance, skinURI, model)) {
    await viewerInstance.loadSkin(skinURI, { model })
  }
  resumeViewers()
  scheduleIdleGC(null, 'skin')
}

async function loadCacheArmor(packName, basePath, material) {
  selectedMaterial.value = material
  removeArmorMeshes(viewerInstance)
  removeArmorMeshes(fsViewerInstance)
  try {
    const tex = await cachedTextureFetch(packName + '|armor|' + material, () => GetPackArmorTexturesFromCache(packName, material, basePath))
    let img1 = null, img2 = null
    if (tex.layer1) img1 = await loadImage(tex.layer1)
    if (tex.layer2) img2 = await loadImage(tex.layer2)
    buildArmor(viewerInstance, img1, img2)
    buildArmor(fsViewerInstance, img1, img2)
    resumeViewers()
    recordMem('material ' + material)
    armIdleGC()
    scheduleIdleGC(null, 'armor')
  } catch (e) {
    console.error('Failed to apply armor:', e)
  }
}

async function loadCacheItems(packName, basePath) {
  if (itemCache.has(packName)) {
    itemTextures.value = itemCache.get(packName)
    return
  }
  const thmb = await cachedTextureFetch(packName + '|itemthumbs|' + selectedMaterial.value, async () => {
    const items = await GetPackItemTexturesFromCache(packName, selectedMaterial.value, basePath)
    return Promise.all((items || []).map(async itm => itm && itm.dataURI ? { ...itm, dataURI: await toThumb(itm.dataURI) } : itm))
  }) || []
  itemTextures.value = thmb
  itemCache.set(packName, thmb)
  pruneItemCache()
  scheduleIdleGC(null, 'items')
}

async function reloadCachePack(packName, basePath) {
  invalidateSkyCache(packName)
  itemCache.delete(packName)
  invalidateTextureCache(packName)
  await loadCachePackData(packName)
  if (fullscreen.value && fsViewerInstance) {
    const uri = customSkinURI.value || defaultSkinImg
    const model = skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value
    if (loadSkinOnce(fsViewerInstance, uri, model)) {
      await fsViewerInstance.loadSkin(uri, { model })
    }
    const cached = currentSkyKey ? skyCache.get(currentSkyKey) : null
    const cubemap = cached || await skyBuild(packName + '#base', lastSkyTex)
    currentSkyKey = skyKey(packName + '#base')
    setSkyOnViewers(cubemap)
    await loadCacheArmor(packName, basePath, selectedMaterial.value)
    resumeViewers()
  }
  recordMem('reload cache pack')
}

async function exportSelectedServerPacks() {
  if (exportingServer.value || !showServerPacks.value) return
  const names = filteredPacks.value.map(p => p.dirName)
  if (!names.length) return
  exportingServer.value = true
  try {
    const written = await ExportCachePacks(names, serverPacksPath.value)
    popup('Exported', `Exported ${written.length} server pack(s) to ${written.length === 1 ? written[0] : 'the output folder'}`, 'success')
  } catch (err) {
    console.error('ExportCachePacks error:', err)
    popup('Export Failed', String(err?.toString ? err.toString() : err), 'error')
  } finally {
    exportingServer.value = false
  }
}

function waitForViewer(timeout = 5000) {
  if (viewerInstance) return Promise.resolve()
  return new Promise(resolve => {
    const start = Date.now()
    const check = setInterval(() => {
      if (viewerInstance || Date.now() - start > timeout) { clearInterval(check); resolve() }
    }, 50)
  })
}

function closeModal() {
  showModal.value = false
  confirmDelete.value = false
  fullscreen.value = false
  if (viewerTimer) { clearTimeout(viewerTimer); viewerTimer = null }
  if (fsViewerTimer) { clearTimeout(fsViewerTimer); fsViewerTimer = null }
  clearIdlePause()
  disposeViewerResources(viewerInstance)
  disposeViewerResources(fsViewerInstance)
  viewerInstance = null
  fsViewerInstance = null
  setLastSkyTex(null)
  currentSkyKey = null
  skySubpacks.value = []
  skySlider.value = 0
  itemTextures.value = []
  itemCache.clear()
  clearSkyCache()
  textureCache.clear()
  texturePending.clear()
  if (pickerObserver) pickerObserver.disconnect()
  pickerObserver = null
  pickerNames.value = []
  pickerTextures.clear()
  pickerThumbs.clear()
  pickerLoaded.clear()
  pickerSearch.value = ''
  showItemPicker.value = false
  viewerMode.value = 'texture'
}

async function loadPackData(packName) {
  try {
    hasScreens.value = false
    if (lastLoadedPack && lastLoadedPack !== packName) {
      invalidateSkyCache(lastLoadedPack)
      itemCache.delete(lastLoadedPack)
      invalidateTextureCache(lastLoadedPack)
    }
    lastLoadedPack = packName
    const [info, skyTex, packHasScreens] = await Promise.all([
      GetPackPreviewInfo(packName),
      cachedTextureFetch(packName + '|sky', () => GetPackSkyTextures(packName)),
      PackHasUiScreens(packName),
    ])
    hasScreens.value = !!packHasScreens
    // A pack without screens cannot use the renderer panel.
    if (!hasScreens.value) viewerMode.value = 'texture'
    recordMem('stage:info')
    selectedPackInfo.value = info
    setLastSkyTex(skyTex)
    const cubemap = await skyBuild(packName + '#base', skyTex)
    recordMem('stage:sky')
    currentSkyKey = skyKey(packName + '#base')
    setSkyOnViewers(cubemap)
    textureCache.delete(packName + '|sky')
    await applySkin(packName)
    recordMem('stage:skin')
    await applyArmor(packName, selectedMaterial.value)
    recordMem('stage:armor')
    await loadItems(packName)
    recordMem('stage:items')
    skySubpacks.value = (await GetPackSkySubpacks(packName)) || []
    skySlider.value = 0
    resumeViewers()
    recordMem('load pack')
    armIdleGC()
    scheduleIdleGC(null, 'load pack')
  } catch (e) {
    console.error('Failed to load pack data:', e)
  }
}

async function reloadPack() {
  const pack = selectedPack.value
  if (!pack) return
  invalidateSkyCache(pack)
  itemCache.delete(pack)
  invalidateTextureCache(pack)
  await loadPackData(pack)
  if (fullscreen.value && fsViewerInstance) await applyAppearanceFs()
  recordMem('reload pack')
}

async function applySkySubpack(idx) {
  if (!skySubpacks.value.length) return
  const sp = skySubpacks.value[idx]
  if (!sp) return
  const tex = await cachedTextureFetch(selectedPack.value + '|sub|' + sp.folderName, () => GetPackSkySubpackTextures(selectedPack.value, sp.folderName))
  if (isDebug.value) {
    console.log('[subpack]', selectedPack.value, sp.folderName, JSON.stringify(tex, (k, v) => v ? (typeof v === 'string' ? v.slice(0, 40) + '...' : v) : v))
  }
  if (!tex || (!tex.cubemap0 && !tex.cubemap1)) {
    if (isDebug.value) console.warn('[subpack] no cubemap textures returned for', sp.folderName)
    return
  }
  setLastSkyTex(tex)
  const cubemap = await skyBuild(selectedPack.value + '#' + idx, tex)
  currentSkyKey = skyKey(selectedPack.value + '#' + idx)
  setSkyOnViewers(cubemap)
  resumeViewers()
  recordMem('sky subpack')
}

async function loadItems(packName) {
  if (itemCache.has(packName)) {
    itemTextures.value = itemCache.get(packName)
    return
  }
  const thmb = await cachedTextureFetch(packName + '|itemthumbs|' + selectedMaterial.value, async () => {
    const items = await GetPackItemTextures(packName, selectedMaterial.value)
    return Promise.all((items || []).map(async itm => itm && itm.dataURI ? { ...itm, dataURI: await toThumb(itm.dataURI) } : itm))
  }) || []
  itemTextures.value = thmb
  itemCache.set(packName, thmb)
  pruneItemCache()
  scheduleIdleGC(null, 'items')
}

function reloadItems(packName) {
  itemCache.delete(packName)
  const prefix = packName + '|itemthumbs|'
  for (const k of [...textureCache.keys()]) { if (k.startsWith(prefix)) textureCache.delete(k) }
  for (const k of [...texturePending.keys()]) { if (k.startsWith(prefix)) texturePending.delete(k) }
  return loadItems(packName)
}

async function applySkin(packName) {
  if (!viewerInstance) return
  const skinURI = customSkinURI.value || await cachedTextureFetch(packName + '|skin', () => GetPlayerSkinTexture(packName)) || defaultSkinImg
  const model = skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value
  if (loadSkinOnce(viewerInstance, skinURI, model)) {
    await viewerInstance.loadSkin(skinURI, { model })
  }
  resumeViewers()
  scheduleIdleGC(null, 'skin')
}

async function applyArmor(packName, material) {
  selectedMaterial.value = material
  removeArmorMeshes(viewerInstance)
  removeArmorMeshes(fsViewerInstance)
  try {
    const tex = await cachedTextureFetch(packName + '|armor|' + material, () => GetPackArmorTextures(packName, material))
    let img1 = null, img2 = null
    if (tex.layer1) img1 = await loadImage(tex.layer1)
    if (tex.layer2) img2 = await loadImage(tex.layer2)
    buildArmor(viewerInstance, img1, img2)
    buildArmor(fsViewerInstance, img1, img2)
    resumeViewers()
    recordMem('material ' + material)
    armIdleGC()
    scheduleIdleGC(null, 'armor')
  } catch (e) {
    console.error('Failed to apply armor:', e)
  }
}

function formatName(name) {
  return name.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

function triggerSkinUpload() { skinFileInput.value?.click() }

async function onSkinFileChange(e) {
  const file = e.target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async () => {
    customSkinURI.value = reader.result
    try { await SaveDefaultSkin(reader.result) } catch (err) { console.error('Failed to save skin:', err) }
  }
  reader.readAsDataURL(file)
  e.target.value = ''
}

async function loadCustomItems() {
  try {
    customItemNames.value = await GetCustomItems() || []
  } catch {
    customItemNames.value = []
  }
  try {
    removedItemNames.value = await GetRemovedItems() || []
  } catch {
    removedItemNames.value = []
  }
}

async function openItemPicker() {
  showItemPicker.value = true
  pickerLoading.value = true
  pickerNames.value = []
  pickerTextures.clear()
  pickerThumbs.clear()
  pickerLoaded.clear()
  pickerSearch.value = ''
  try {
    const packName = selectedPack.value
    const names = showServerPacks.value
      ? await GetPackItemTextureNamesFromCache(packName, serverPacksPath.value)
      : await GetPackItemTextureNames(packName)
    pickerNames.value = names || []
  } catch (err) {
    console.error('Failed to load item names:', err)
    pickerNames.value = []
  }
  pickerLoading.value = false
  nextTick(setupPickerObserver)
  recordMem('picker open')
}

function setupPickerObserver() {
  if (pickerObserver) pickerObserver.disconnect()
  pickerObserver = null
  if (!pickerGridRef.value) return
  pickerObserver = new IntersectionObserver((entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) loadPickerTexture(entry.target.dataset.name)
    }
  }, { root: pickerGridRef.value, rootMargin: '200px' })
  for (const el of pickerGridRef.value.querySelectorAll('.pv-picker-item')) {
    const name = el.dataset.name
    if (name && (pickerLoaded.has(name) || pickerTextures.has(name))) {
      loadPickerTexture(name)
      continue
    }
    pickerObserver.observe(el)
  }
}

watch(filteredPickerItems, () => {
  nextTick(setupPickerObserver)
})

async function loadPickerTexture(name) {
  if (pickerLoaded.has(name) || pickerTextures.has(name)) return
  pickerLoaded.add(name)
  try {
    const packName = selectedPack.value
    const dataURI = await cachedTextureFetch(packName + '|picker|' + name, async () => {
      if (showServerPacks.value) {
        return GetPackItemTextureFromCache(packName, name, serverPacksPath.value)
      }
      return GetPackItemTexture(packName, name)
    })
    if (dataURI) pickerTextures.set(name, dataURI)
    loadPickerThumb(name, dataURI)
  } catch (err) {
    console.error('Failed to load item texture:', name, err)
  }
}

async function loadPickerThumb(name, dataURI) {
  if (!dataURI || pickerThumbs.has(name)) return
  try {
    pickerThumbs.set(name, await toThumb(dataURI, 36))
  } catch {}
}

function closeItemPicker() {
  showItemPicker.value = false
  if (pickerObserver) pickerObserver.disconnect()
  pickerObserver = null
  pickerNames.value = []
  pickerTextures.clear()
  pickerLoaded.clear()
  pickerSearch.value = ''
  recordMem('picker close')
  armIdleGC()
  scheduleIdleGC(null, 'picker close')
}

async function selectPickerItem(name) {
  const gridNames = itemTextures.value.map(i => i.name)
  if (gridNames.includes(name)) return
  try {
    if (removedItemNames.value.includes(name)) {
      await RestoreRemovedItem(name)
    } else {
      await SaveCustomItem(name)
    }
    await loadCustomItems()
    if (selectedPack.value) await reloadItems(selectedPack.value)
    recordMem('item added')
  } catch (err) {
    console.error('Failed to add item:', err)
  }
  closeItemPicker()
}

async function removeItem(name) {
  try {
    if (customItemNames.value.includes(name)) {
      await RemoveCustomItem(name)
    } else {
      await SaveRemovedItem(name)
    }
    await loadCustomItems()
    if (selectedPack.value) await reloadItems(selectedPack.value)
    recordMem('item removed')
  } catch (err) {
    console.error('Failed to remove item:', err)
  }
}

onMounted(async () => {
  try { isDebug.value = await IsDebug() } catch {}
  loadSkyDebugConfig()
  await loadAllPacks()
  window.addEventListener('resize', onFsResize)
  if (isDebug.value && showMemPanel.value) startMemSampler()
  recordMem('mounted')
  startGCWatchdog()
})

onUnmounted(() => {
  stopGCWatchdog()
  if (gcTimer) { clearTimeout(gcTimer); gcTimer = null }
  stopMemSampler()
  window.removeEventListener('resize', onFsResize)
  window.removeEventListener('keydown', onFsNavKey)
  if (viewerTimer) { clearTimeout(viewerTimer); viewerTimer = null }
  if (fsViewerTimer) { clearTimeout(fsViewerTimer); fsViewerTimer = null }
  modelSetupAlive = false
  clearIdlePause()
  disposeViewerResources(viewerInstance)
  disposeViewerResources(fsViewerInstance)
  clearSkyCache()
  viewerInstance = null
  fsViewerInstance = null
})

function onFsResize() {
  if (fullscreen.value) measureFs()
}

watch(() => customSkinURI.value, async () => {
  if (fsViewerInstance) {
    const uri = customSkinURI.value || defaultSkinImg
    const model = skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value
    if (loadSkinOnce(fsViewerInstance, uri, model)) {
      await fsViewerInstance.loadSkin(uri, { model })
      await applyAppearanceFs()
    }
  }
})

watch(() => skinModel.value, async () => {
  if (fsViewerInstance) {
    const uri = customSkinURI.value || defaultSkinImg
    const model = skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value
    if (loadSkinOnce(fsViewerInstance, uri, model)) {
      await fsViewerInstance.loadSkin(uri, { model })
    }
  }
})

watch(() => props.active, (val) => {
  if (val) {
    loadAllPacks()
  } else {
    closeModal()
  }
})

watch(() => props.openPackReq, (req) => {
  if (req && req.name) openPack(req.name)
})
</script>

<template>
  <div class="pv-page page">
    <div v-if="!loading && !showServerPacks && packList.length === 0" class="pv-empty">
      <i class="fa fa-box-open pv-empty-icon"></i>
      <p>No installed packs found.</p>
      <p class="pv-empty-sub">Install a pack from the Pack Porter first.</p>
    </div>
    <div v-else-if="!loading && showServerPacks && cachePackList.length === 0" class="pv-empty">
      <i class="fa fa-server pv-empty-icon"></i>
      <p>No server packs found.</p>
      <p class="pv-empty-sub">Download a resource pack from a server to see it here.</p>
    </div>

    <template v-else>
      <div class="pv-toolbar">
        <div class="pv-search">
          <i class="fa fa-magnifying-glass pv-search-icon"></i>
          <input v-model="searchQuery" class="pv-search-input" placeholder="Search packs..." />
          <span v-if="searchQuery" class="pv-search-clear" @click="searchQuery = ''"><i class="fa fa-xmark"></i></span>
        </div>
        <div class="pv-toolbar-right">
          <button class="pv-sort-btn" :class="{ active: showServerPacks }" @click="showServerPacks = !showServerPacks" title="Toggle server packs from cache">
            <i class="fa fa-server"></i> Server Packs
          </button>
          <span class="pv-count">{{ filteredPacks.length }} pack{{ filteredPacks.length !== 1 ? 's' : '' }}</span>
          <div class="pv-sort">
            <button v-for="opt in [{v:'name',icon:'fa-arrow-down-a-z'},{v:'date',icon:'fa-clock'},{v:'size',icon:'fa-hard-drive'}]"
              :key="opt.v" class="pv-sort-btn" :class="{ active: sortBy === opt.v }"
              :title="opt.v === 'name' ? 'Name' : opt.v === 'date' ? 'Date Added' : 'Size'"
              @click="sortBy = opt.v"><i class="fa" :class="opt.icon"></i></button>
          </div>
        </div>
      </div>

      <div v-if="filteredPacks.length === 0" class="pv-empty" style="height:auto;padding:3rem">
        <i class="fa fa-magnifying-glass pv-empty-icon" style="font-size:2rem"></i>
        <p>No packs match your search.</p>
      </div>

      <div v-else class="pv-grid">
        <div v-for="pack in filteredPacks" :key="pack.dirName" class="pv-card" @click="showServerPacks ? openCachePack(pack.dirName) : openPack(pack.dirName)">
          <img v-if="pack.iconURI" :src="pack.iconURI" class="pv-card-icon" width="56" height="56" loading="lazy" decoding="async" />
          <div v-else class="pv-card-icon pv-card-placeholder">
            <i class="fa fa-box"></i>
          </div>
          <div class="pv-card-info">
            <span class="pv-card-name" v-html="parseBedrockCodes(pack.name || pack.dirName)"></span>
            <span v-if="pack.description" class="pv-card-desc" v-html="parseBedrockCodes(pack.description)"></span>
            <span class="pv-card-size">{{ formatSize(pack.size) }}</span>
          </div>
        </div>
      </div>
    </template>

    <div v-if="showModal" class="pv-modal-overlay" @click.self="closeModal">
      <div class="pv-modal" :class="{ 'pv-modal-wide': viewerMode === 'ui' }">
        <div class="pv-modal-head">
          <div class="pv-modal-pack">
            <img v-if="selectedPackInfo.iconURI" :src="selectedPackInfo.iconURI" class="pv-modal-icon" />
            <div>
              <h2 class="pv-modal-name" v-html="parseBedrockCodes(selectedPackInfo.name || selectedPack)"></h2>
              <p v-if="selectedPackInfo.description" class="pv-modal-desc" v-html="parseBedrockCodes(selectedPackInfo.description)"></p>
            </div>
          </div>
          <div class="pv-modal-actions">
            <button v-if="isDebug" class="pv-modal-skybtn" :class="{ active: showSkyDebug }" @click="showSkyDebug = !showSkyDebug" title="Sky debug"><i class="fa fa-cloud-sun"></i></button>
            <button v-if="hasScreens" class="pv-modal-menubtn" :class="{ active: viewerMode === 'ui' }" :title="viewerMode === 'ui' ? 'Back to texture viewer' : 'Open UI renderer'" @click="toggleUiPanel"><i class="fa fa-bars-staggered"></i></button>
            <button class="pv-modal-folder" @click="openFullscreen" title="Fullscreen preview"><i class="fa fa-expand"></i></button>
            <button class="pv-modal-folder" @click="openPackFolder" title="Open pack folder"><i class="fa fa-folder-open"></i></button>
            <button v-if="showServerPacks" class="pv-modal-folder" :class="{ 'pv-export-btn': true, 'pv-export-active': !exportingServer }" @click="exportSelectedServerPacks" :disabled="exportingServer" title="Export all visible server packs">
              <i :class="exportingServer ? 'fa fa-circle-notch fa-spin' : 'fa fa-file-export'"></i>
            </button>
            <button v-if="!showServerPacks" class="pv-modal-delete" :class="{ confirming: confirmDelete }" @click="deletePack" :title="confirmDelete ? 'Click again to delete' : 'Delete pack'">
              <i :class="confirmDelete ? 'fa fa-triangle-exclamation' : 'fa fa-trash'"></i>
            </button>
            <button class="pv-modal-close" @click="closeModal"><i class="fa fa-xmark"></i></button>
          </div>
        </div>

        <div v-show="viewerMode === 'texture'" class="pv-viewer-wrap">
          <div ref="modalContainer" class="pv-3d">
            <SkinView3d
              v-if="!fullscreen"
              ref="viewerRef"
              :width="getViewerSize().w"
              :height="getViewerSize().h"
              :skin-url="customSkinURI || defaultSkinImg"
              :skin-options="skinOptions"
              :animation="currentAnimation"
              :background="null"
              :fov="45"
              :zoom="0.9"
              :enable-rotate="true"
              :enable-zoom="true"
              :enable-pan="false"
              :global-light="3"
              :camera-light="1.2"
            />
          </div>

          <button class="pv-nav pv-nav-prev" @click.stop="navPack(-1)" title="Previous pack"><i class="fa fa-chevron-left"></i></button>
          <button class="pv-nav pv-nav-next" @click.stop="navPack(1)" title="Next pack"><i class="fa fa-chevron-right"></i></button>

          <div v-if="isDebug && showSkyDebug" class="pv-sky-debug">
            <div class="pv-sky-debug-head">
              <span class="pv-sky-debug-title"><i class="fa fa-cloud-sun"></i> Sky Cubemap</span>
              <div class="pv-sky-debug-actions">
                <button class="pv-sky-debug-btn" @click="copySkyDebugConfig" title="Copy config"><i class="fa fa-copy"></i></button>
                <button class="pv-sky-debug-btn" @click="resetSkyDebugConfig" title="Reset defaults"><i class="fa fa-rotate-left"></i></button>
              </div>
            </div>
            <div class="pv-sky-debug-faces">
              <div v-for="meta in SKY_FACE_META" :key="meta.key" class="pv-sky-debug-row">
                <span class="pv-sky-debug-label">{{ meta.bedrock }}<span class="pv-sky-debug-sub"> ({{ meta.threeFace }})</span></span>
                <select v-model.number="skyDebugFaces[meta.key].rotation" class="pv-sky-debug-select" @change="saveSkyDebugConfig(); applySkyDebug()">
                  <option :value="0">0°</option>
                  <option :value="90">90°</option>
                  <option :value="180">180°</option>
                  <option :value="270">270°</option>
                </select>
                <label class="pv-sky-debug-flip" title="Flip horizontal">
                  <input type="checkbox" v-model="skyDebugFaces[meta.key].flipH" @change="saveSkyDebugConfig(); applySkyDebug()" />
                  <i class="fa fa-left-right"></i>
                </label>
              </div>
            </div>
          </div>

          <div v-if="skySubpacks.length > 0" class="pv-skybar">
            <div class="pv-skybar-head">
              <div class="pv-skybar-label"><i class="fa fa-cloud-sun"></i> Subpacks</div>
              <div class="pv-skybar-current" v-html="parseBedrockCodes(skySubpacks[skySlider]?.name || '')"></div>
            </div>
            <input type="range" class="pv-skybar-slider" min="0" :max="skySubpacks.length - 1" step="1"
              v-model.number="skySlider" @input="skySlider = Number(skySlider); applySkySubpack(Number(skySlider))" />
          </div>
        </div>

        <JsonUiStage v-if="viewerMode === 'ui'" :pack="selectedPack" @close="viewerMode = 'texture'" />

        <div v-show="viewerMode === 'texture'" class="pv-materials">
          <button v-for="m in materials" :key="m"
            class="pv-mat-btn" :class="{ active: selectedMaterial === m }"
            @click="applyArmor(selectedPack, m)">
            {{ materialLabels[m] }}
          </button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: animating }" @click="animating = !animating">
            <i class="fa fa-walking"></i> Walk
          </button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'auto-detect' }" @click="skinModel = 'auto-detect'">
            <i class="fa fa-robot"></i> Auto
          </button>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'default' }" @click="skinModel = 'default'">
            <i class="fa fa-person"></i> Wide
          </button>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'slim' }" @click="skinModel = 'slim'">
            <i class="fa fa-person-dress"></i> Slim
          </button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn pv-skin-btn" @click="triggerSkinUpload">
            <i class="fa fa-user-pen"></i> Change Skin
          </button>
          <button class="pv-mat-btn pv-reload-btn" @click="showServerPacks ? reloadCachePack(selectedPack, serverPacksPath) : reloadPack()" title="Reload this pack from disk (sky, items, armor)">
            <i class="fa fa-rotate"></i> Reload
          </button>
          <span class="pv-mat-sep"></span>
          <button v-if="!showServerPacks" class="pv-mat-btn" @click="showExportPopup = true" title="Export this pack as a .mcpack file">
            <i class="fa fa-file-export"></i> Export .mcpack
          </button>
        </div>
        <input ref="skinFileInput" type="file" accept="image/png" class="pv-hidden-input" @change="onSkinFileChange" />

        <div v-show="viewerMode === 'texture'" class="pv-items-section">
          <div class="pv-items-grid">
            <div v-for="item in itemTextures" :key="item.name"
              class="pv-item-card">
              <div class="pv-item-wrap">
                <img :src="item.dataURI" class="pv-item-img" />
                <button class="pv-item-remove" @click.stop="removeItem(item.name)">
                  <i class="fa fa-xmark"></i>
                </button>
              </div>
              <span class="pv-item-label">{{ formatName(item.name) }}</span>
            </div>
            <div class="pv-item-card pv-item-add" @click="openItemPicker">
              <i class="fa fa-plus pv-item-add-icon"></i>
              <span class="pv-item-label">Add Item</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <PackExporter :show="showExportPopup" :active="showExportPopup" @close="showExportPopup = false" />

    <div v-if="fullscreen" class="pv-fs-overlay">
      <div ref="fsContainerRef" class="pv-fs-3d">
        <SkinView3d
          ref="fsViewerRef"
          :width="fsSize.w"
          :height="fsSize.h"
          :skin-url="customSkinURI || defaultSkinImg"
          :skin-options="skinOptions"
          :animation="currentAnimation"
          :background="null"
          :fov="45"
          :zoom="1.1"
          :enable-rotate="true"
          :enable-zoom="true"
          :enable-pan="false"
          :global-light="3"
          :camera-light="1.2"
        />
        <button class="pv-nav pv-nav-prev" @click.stop="navPack(-1)" title="Previous pack (←)"><i class="fa fa-chevron-left"></i></button>
        <button class="pv-nav pv-nav-next" @click.stop="navPack(1)" title="Next pack (→)"><i class="fa fa-chevron-right"></i></button>
        <div v-if="skySubpacks.length > 0" class="pv-skybar pv-fs-skybar">
          <div class="pv-skybar-head">
            <div class="pv-skybar-label"><i class="fa fa-cloud-sun"></i> Subpacks</div>
            <div class="pv-skybar-current" v-html="parseBedrockCodes(skySubpacks[skySlider]?.name || '')"></div>
          </div>
          <input type="range" class="pv-skybar-slider" min="0" :max="skySubpacks.length - 1" step="1"
            v-model.number="skySlider" @input="skySlider = Number(skySlider); applySkySubpack(Number(skySlider))" />
        </div>
      </div>
      <div class="pv-fs-bar">
        <div class="pv-fs-controls">
          <button v-for="m in materials" :key="m" class="pv-mat-btn" :class="{ active: selectedMaterial === m }"
            @click="applyArmor(selectedPack, m)">{{ materialLabels[m] }}</button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: animating }" @click="animating = !animating"><i class="fa fa-walking"></i> Walk</button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'auto-detect' }" @click="skinModel = 'auto-detect'"><i class="fa fa-robot"></i> Auto</button>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'default' }" @click="skinModel = 'default'"><i class="fa fa-person"></i> Wide</button>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'slim' }" @click="skinModel = 'slim'"><i class="fa fa-person-dress"></i> Slim</button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn pv-skin-btn" @click="triggerSkinUpload"><i class="fa fa-user-pen"></i> Change Skin</button>
          <button class="pv-mat-btn pv-reload-btn" @click="showServerPacks ? reloadCachePack(selectedPack, serverPacksPath) : reloadPack()" title="Reload this pack from disk (sky, items, armor)"><i class="fa fa-rotate"></i></button>
        </div>
        <button class="pv-fs-btn pv-fs-close" @click="closeFullscreen" title="Close (Esc)"><i class="fa fa-xmark"></i></button>
      </div>
    </div>

    <div v-if="showItemPicker" class="pv-picker-overlay" @click.self="closeItemPicker">
      <div class="pv-picker" @click.stop>
        <div class="pv-picker-head">
          <h3 class="pv-picker-title">Add Item to Grid</h3>
          <button class="pv-modal-close" @click="closeItemPicker"><i class="fa fa-xmark"></i></button>
        </div>
        <div v-if="!pickerLoading && pickerNames.length > 0" class="pv-picker-search">
          <i class="fa fa-search pv-picker-search-icon"></i>
          <input v-model="pickerSearch" class="pv-picker-search-input" placeholder="Search items..." />
        </div>
        <div v-if="pickerLoading" class="pv-picker-loading">
          <div class="pv-spinner"></div>
        </div>
        <div v-else-if="filteredPickerItems.length === 0" class="pv-picker-empty">
          <p>{{ pickerSearch ? 'No matching items found.' : 'No item textures found in this pack.' }}</p>
        </div>
        <div v-else ref="pickerGridRef" class="pv-picker-grid">
          <div v-for="item in filteredPickerItems" :key="item"
            class="pv-picker-item"
            :class="{ used: itemTextures.some(i => i.name === item) }"
            :data-name="item"
            @click="selectPickerItem(item)">
            <img :src="pickerThumbs.get(item) || pickerTextures.get(item)" class="pv-picker-item-img" />
            <span class="pv-picker-item-name">{{ formatName(item) }}</span>
            <div v-if="itemTextures.some(i => i.name === item)" class="pv-picker-item-check">
              <i class="fa fa-check"></i>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="isDebug && showMemPanel" class="pv-mem-panel">
      <div class="pv-mem-head">
        <button class="pv-mem-toggle" @click="showMemPanel = !showMemPanel" title="Toggle mem panel"><i class="fa fa-microchip"></i></button>
        <button class="pv-mem-toggle" @click="memDump" title="Copy memory dump to clipboard + console"><i class="fa fa-clipboard"></i></button>
        <button class="pv-mem-toggle" @click="resetMem" title="Reset memory: clear caches/viewers and reload"><i class="fa fa-refresh"></i></button>
        <button class="pv-mem-clear" @click="memLog = []" title="Clear log"><i class="fa fa-eraser"></i></button>
      </div>
      <div class="pv-mem-live" v-if="memLive">
        <span class="pv-mem-k">LIVE {{ memLive.t }}</span>
        <span class="pv-mem-v">heap {{ formatSize(memLive.heap) }}</span>
        <span class="pv-mem-v">canv {{ formatSize(memLive.canvasBytes) }} ({{ memLive.canvasCount }})</span>
        <span class="pv-mem-v">imgs {{ formatSize(memLive.imgBytes) }} ({{ memLive.imgCount }})</span>
        <span class="pv-mem-v">allI {{ memLive.allImgCount }} ({{ formatSize(memLive.allImgBytes) }})</span>
        <span class="pv-mem-v">sky {{ memLive.skyCount }} ({{ formatSize(memLive.skyBytes) }})</span>
        <span class="pv-mem-v">itemC {{ memLive.itemCacheCount }} ({{ formatSize(memLive.itemCacheBytes) }})</span>
        <span class="pv-mem-v">itemG {{ memLive.itemTexCount }} ({{ formatSize(memLive.itemTexBytes) }})</span>
        <span class="pv-mem-v">modal {{ memLive.modal ? 'on' : 'off' }} / fs {{ memLive.fs ? 'on' : 'off' }} / {{ memLive.packs }} packs</span>
      </div>
      <div v-if="memLog.length" class="pv-mem-last">
        <span class="pv-mem-k">{{ memLog[memLog.length - 1].label }}</span>
        <span class="pv-mem-v">heap {{ formatSize(memLog[memLog.length - 1].heap) }}</span>
        <span class="pv-mem-v">canvas {{ formatSize(memLog[memLog.length - 1].canvasBytes) }} ({{ memLog[memLog.length - 1].canvasCount }})</span>
        <span class="pv-mem-v">img {{ formatSize(memLog[memLog.length - 1].imgBytes) }} ({{ memLog[memLog.length - 1].imgCount }})</span>
        <span class="pv-mem-v">allI {{ memLog[memLog.length - 1].allImgCount }} ({{ formatSize(memLog[memLog.length - 1].allImgBytes) }})</span>
        <span class="pv-mem-v">sky {{ memLog[memLog.length - 1].skyCount }} ({{ formatSize(memLog[memLog.length - 1].skyBytes) }})</span>
        <span class="pv-mem-v">itemC {{ memLog[memLog.length - 1].itemCacheCount }} ({{ formatSize(memLog[memLog.length - 1].itemCacheBytes) }})</span>
      </div>
      <div v-for="(e, i) in [...memLog].reverse()" :key="i" class="pv-mem-row">
        <span class="pv-mem-k">{{ e.t }} {{ e.label }}</span>
        <span class="pv-mem-v">heap {{ formatSize(e.heap) }}</span>
        <span class="pv-mem-v">canv {{ formatSize(e.canvasBytes) }}</span>
        <span class="pv-mem-v">imgs {{ formatSize(e.imgBytes) }}</span>
        <span class="pv-mem-v">allI {{ e.allImgCount }} ({{ formatSize(e.allImgBytes) }})</span>
        <span class="pv-mem-v">sky {{ e.skyCount }} ({{ formatSize(e.skyBytes) }})</span>
        <span class="pv-mem-v">itemC {{ e.itemCacheCount }} ({{ formatSize(e.itemCacheBytes) }})</span>
      </div>
    </div>

    <div v-if="loading" class="pv-loading">
      <div class="pv-spinner"></div>
    </div>
  </div>
</template>

<style scoped>
.pv-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
  background: var(--bg-body);
}

.pv-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 0.5rem;
  color: var(--text-dim);
}

.pv-empty-icon { font-size: 3rem; margin-bottom: 0.5rem; }
.pv-empty-sub { font-size: 0.8rem; color: var(--text-version); }

.pv-toolbar {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 1rem;
  padding: 1rem 1.5rem 0.25rem;
  position: relative;
}

.pv-toolbar-right {
  position: absolute;
  right: 1.5rem;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.pv-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
  max-width: 340px;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 0.5rem 0.75rem;
  transition: border-color 0.2s, box-shadow 0.2s;
}

.pv-search:focus-within {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px var(--accent-glow);
}

.pv-search-icon {
  color: var(--text-dim);
  font-size: 0.75rem;
  flex-shrink: 0;
}

.pv-search-input {
  flex: 1;
  background: none;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-size: 0.8rem;
  min-width: 0;
}

.pv-search-input::placeholder {
  color: var(--text-faint);
}

.pv-search-clear {
  color: var(--text-dim);
  font-size: 0.7rem;
  cursor: pointer;
  padding: 0.15rem;
  border-radius: 4px;
  transition: color 0.15s;
  flex-shrink: 0;
}

.pv-search-clear:hover {
  color: var(--text-primary);
}

.pv-count {
  font-size: 0.7rem;
  color: var(--text-faint);
  white-space: nowrap;
}

.pv-sort {
  display: flex;
  background: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.pv-sort-btn {
  padding: 0.45rem 0.65rem;
  border: none;
  background: transparent;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 0.75rem;
  transition: all 0.15s;
  position: relative;
}

.pv-sort-btn:not(:last-child)::after {
  content: '';
  position: absolute;
  right: 0;
  top: 25%;
  height: 50%;
  width: 1px;
  background: var(--border-subtle);
}

.pv-sort-btn:hover {
  color: var(--text-secondary);
  background: var(--bg-hover-1);
}

.pv-sort-btn.active {
  color: var(--accent-light);
  background: var(--accent-active-bg);
}

.pv-sort-btn.active::after {
  background: transparent;
}

.pv-card-size {
  font-size: 0.6rem;
  color: var(--text-faint);
  margin-top: 0.1rem;
}

.pv-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.75rem;
  padding: 1.5rem;
}

.pv-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 1rem;
  border-radius: 12px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  cursor: pointer;
  transition: all 0.15s;
}

.pv-card:hover {
  border-color: var(--accent);
  background: var(--bg-hover-1);
  transform: translateY(-2px);
}

.pv-card-icon {
  width: 56px;
  height: 56px;
  border-radius: 10px;
  object-fit: cover;
  border: 1px solid var(--border-subtle);
}

.pv-card-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-hover-1);
  color: var(--text-dim);
  font-size: 1.2rem;
}

.pv-card-info {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.15rem;
  min-width: 0;
  width: 100%;
}

.pv-card-name {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
  text-align: center;
  white-space: pre-line;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
}

.pv-card-desc {
  font-size: 0.65rem;
  color: var(--text-dim);
  text-align: center;
  white-space: pre-line;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
}

.pv-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
}

.pv-modal {
  background: var(--bg-body);
  border: 1px solid var(--border-default);
  border-radius: 14px;
  width: 90vw;
  max-width: 600px;
  max-height: 90vh;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  padding: 1rem;
}

/* The UI renderer needs room and a definite height so its own panels can size
   against it; the default modal is a small scrolling card. */
.pv-modal-wide {
  max-width: min(1500px, 95vw);
  height: 90vh;
  overflow: hidden;
}

.pv-modal-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.pv-modal-pack {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.pv-modal-icon {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  object-fit: cover;
  border: 1px solid var(--border-default);
  flex-shrink: 0;
}

.pv-modal-name {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
  white-space: pre-line;
}

.pv-modal-desc {
  font-size: 0.7rem;
  color: var(--text-dim);
  margin-top: 1px;
  white-space: pre-line;
}

.pv-modal-actions {
  display: flex;
  gap: 0.35rem;
  flex-shrink: 0;
}

.pv-modal-folder,
.pv-modal-delete,
.pv-modal-close,
.pv-export-btn {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.15s;
}

.pv-modal-folder:hover,
.pv-modal-close:hover {
  background: var(--bg-hover-1);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.pv-export-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.pv-export-btn:not(:disabled):hover {
  background: var(--accent-active-bg);
  color: var(--accent-light);
  border-color: var(--accent);
}

.pv-modal-delete:hover {
  background: rgba(248, 113, 113, 0.15);
  color: #f87171;
  border-color: #f87171;
}

.pv-modal-delete.confirming {
  background: #f87171;
  color: #1a1a1a;
  border-color: #f87171;
}

.pv-3d {
  width: 100%;
  height: 400px;
  border-radius: 0;
  background: transparent;
}

.pv-3d canvas { display: block; }

.pv-materials {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  justify-content: center;
}

.pv-mat-btn {
  padding: 0.35rem 0.75rem;
  border-radius: 8px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 0.75rem;
  font-weight: 500;
  transition: all 0.15s;
}

.pv-mat-btn:hover {
  background: var(--bg-hover-1);
  border-color: var(--border-focus);
}

.pv-mat-btn.active {
  background: var(--accent-active-bg);
  color: var(--accent-light);
  border-color: var(--accent-light);
}

.pv-mat-sep {
  width: 1px;
  height: 1.2rem;
  background: var(--border-default);
  margin: 0 0.15rem;
}

.pv-skin-btn i { margin-right: 0.3rem; }

.pv-reload-btn {
  color: var(--accent-light);
}

.pv-reload-btn:hover {
  border-color: var(--accent-light);
}

.pv-hidden-input {
  position: absolute;
  width: 0;
  height: 0;
  opacity: 0;
  pointer-events: none;
}

.pv-items-section {
  width: 100%;
}

.pv-items-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(64px, 1fr));
  gap: 0.4rem;
}

.pv-item-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  padding: 0.4rem 0.2rem;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  background: var(--bg-body);
}

.pv-item-img {
  width: 36px;
  height: 36px;
  image-rendering: pixelated;
  object-fit: contain;
}

.pv-item-wrap {
  position: relative;
  width: 36px;
  height: 36px;
}

.pv-item-remove {
  position: absolute;
  top: -4px;
  right: -4px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: none;
  background: #e74c3c;
  color: #fff;
  font-size: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  opacity: 0;
  transition: opacity 0.15s;
  padding: 0;
  line-height: 1;
}

.pv-item-card:hover .pv-item-remove {
  opacity: 1;
}

.pv-item-add {
  cursor: pointer;
  border-style: dashed;
  border-color: var(--border-medium);
  transition: all 0.15s;
}

.pv-item-add:hover {
  border-color: var(--accent);
  background: var(--bg-hover-1);
}

.pv-item-add-icon {
  font-size: 1rem;
  color: var(--text-dim);
}

.pv-picker-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 60;
}

.pv-picker {
  background: var(--bg-body);
  border: 1px solid var(--border-default);
  border-radius: 14px;
  width: 90vw;
  max-width: 560px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.pv-picker-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.85rem 1rem;
  border-bottom: 1px solid var(--border-default);
}

.pv-picker-title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.pv-picker-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 3rem;
}

.pv-picker-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 3rem;
  color: var(--text-dim);
  font-size: 0.85rem;
}

.pv-picker-search {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border-default);
}

.pv-picker-search-icon {
  color: var(--text-dim);
  font-size: 0.75rem;
}

.pv-picker-search-input {
  flex: 1;
  background: none;
  border: none;
  outline: none;
  color: var(--text-primary);
  font-size: 0.8rem;
  padding: 0.25rem 0;
}

.pv-picker-search-input::placeholder {
  color: var(--text-dim);
}

.pv-picker-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(72px, 1fr));
  gap: 0.4rem;
  padding: 0.75rem;
  overflow-y: auto;
  max-height: 60vh;
}

.pv-picker-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.2rem;
  padding: 0.4rem 0.2rem;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  background: var(--bg-body);
  cursor: pointer;
  transition: all 0.15s;
  position: relative;
}

.pv-picker-item:hover:not(.used) {
  border-color: var(--accent);
  background: var(--bg-hover-1);
}

.pv-picker-item.used {
  opacity: 0.4;
  cursor: default;
}

.pv-picker-item-img {
  width: 36px;
  height: 36px;
  image-rendering: pixelated;
  object-fit: contain;
}

.pv-picker-item-name {
  font-size: 0.55rem;
  color: var(--text-dim);
  text-align: center;
  line-height: 1.1;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pv-picker-item-check {
  position: absolute;
  top: 2px;
  right: 2px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--accent);
  color: #fff;
  font-size: 7px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.pv-item-label {
  font-size: 0.55rem;
  color: var(--text-dim);
  text-align: center;
  line-height: 1.1;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pv-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.4);
  z-index: 10;
}

.pv-spinner {
  width: 32px;
  height: 32px;
  border: 3px solid var(--border-default);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin { to { transform: rotate(360deg); } }

.pv-viewer-wrap {
  position: relative;
  width: 100%;
  border-radius: 10px;
  overflow: hidden;
  background: #1a3a6a;
}

.pv-viewer-wrap .pv-3d {
  background: transparent;
  border-radius: 0;
}

.pv-modal-skybtn {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.15s;
}

.pv-modal-skybtn:hover {
  background: var(--bg-hover-1);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.pv-modal-skybtn.active {
  background: var(--accent-active-bg);
  color: var(--accent-light);
  border-color: var(--accent);
}

.pv-modal-menubtn {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  transition: all 0.15s;
  font-size: 14px;
}

.pv-modal-menubtn:hover {
  background: var(--bg-hover-2);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.pv-modal-menubtn.active {
  background: var(--accent-active-bg);
  color: var(--accent-light);
  border-color: var(--accent);
}

.pv-sky-debug {
  position: absolute;
  top: 8px;
  right: 8px;
  width: 220px;
  background: rgba(12, 12, 20, 0.92);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  padding: 8px;
  z-index: 20;
  backdrop-filter: blur(8px);
}

.pv-sky-debug-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}

.pv-sky-debug-title {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 4px;
}

.pv-sky-debug-title i {
  color: var(--accent-light);
}

.pv-sky-debug-actions {
  display: flex;
  gap: 3px;
}

.pv-sky-debug-btn {
  width: 22px;
  height: 22px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: transparent;
  color: var(--text-dim);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.6rem;
  transition: all 0.15s;
}

.pv-sky-debug-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.pv-sky-debug-faces {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.pv-sky-debug-row {
  display: flex;
  align-items: center;
  gap: 5px;
}

.pv-sky-debug-label {
  font-size: 0.6rem;
  color: var(--text-secondary);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: monospace;
}

.pv-sky-debug-sub {
  color: var(--text-faint);
}

.pv-sky-debug-select {
  width: 52px;
  padding: 2px 3px;
  border-radius: 4px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.06);
  color: var(--text-primary);
  font-size: 0.6rem;
  font-family: monospace;
  cursor: pointer;
  outline: none;
}

.pv-sky-debug-select:focus {
  border-color: var(--accent);
}

.pv-sky-debug-select option {
  background: #1a1a2e;
  color: var(--text-primary);
}

.pv-sky-debug-flip {
  display: flex;
  align-items: center;
  cursor: pointer;
  color: var(--text-dim);
  font-size: 0.6rem;
  transition: color 0.15s;
}

.pv-sky-debug-flip input {
  display: none;
}

.pv-sky-debug-flip:has(input:checked) {
  color: var(--accent-light);
}

.pv-skybar {
  position: absolute;
  left: 0.5rem;
  right: 0.5rem;
  bottom: 0.5rem;
  padding: 0.55rem 0.75rem 0.6rem;
  background: rgba(10, 10, 18, 0.78);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  box-shadow: 0 4px 16px rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(8px);
  z-index: 15;
}

.pv-skybar-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.4rem;
}

.pv-skybar-label {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.7rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  color: var(--text-secondary);
  text-transform: uppercase;
}

.pv-skybar-label i { color: var(--accent-light); }

.pv-skybar-slider {
  width: 100%;
  -webkit-appearance: none;
  appearance: none;
  height: 6px;
  border-radius: 999px;
  background: var(--bg-hover-3);
  outline: none;
  cursor: pointer;
  margin-bottom: 0.35rem;
}

.pv-skybar-slider::-webkit-slider-runnable-track {
  height: 6px;
  border-radius: 999px;
  background: var(--bg-hover-3);
}

.pv-skybar-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid #1a1a1a;
  cursor: pointer;
  margin-top: -5px;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.4);
}

.pv-skybar-slider::-moz-range-track {
  height: 6px;
  border-radius: 999px;
  background: var(--bg-hover-3);
}

.pv-skybar-slider::-moz-range-thumb {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--accent);
  border: 2px solid #1a1a1a;
  cursor: pointer;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.4);
}

.pv-skybar-current {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--accent-light);
  text-align: right;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pv-fs-overlay {
  position: fixed;
  inset: 0;
  background: #000;
  display: flex;
  flex-direction: column;
  z-index: 100;
}

.pv-fs-3d {
  flex: 1;
  min-height: 0;
  position: relative;
  overflow: hidden;
}

.pv-fs-3d canvas { display: block; }

.pv-fs-bar {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.6rem;
  padding: 0.6rem 0.9rem;
  background: rgba(8, 8, 16, 0.92);
  border-top: 1px solid var(--border-subtle);
  backdrop-filter: blur(6px);
  flex-wrap: wrap;
  z-index: 110;
}

.pv-nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 40px;
  height: 56px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  background: rgba(10, 10, 18, 0.55);
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.1rem;
  backdrop-filter: blur(6px);
  z-index: 120;
  transition: all 0.15s;
}

.pv-nav-prev {
  left: 0.6rem;
  border-radius: 10px;
}

.pv-nav-next {
  right: 0.6rem;
  border-radius: 10px;
}

.pv-nav:hover {
  background: rgba(10, 10, 18, 0.9);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.pv-nav:active {
  opacity: 0.8;
}

.pv-fs-controls {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.pv-fs-btn {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: rgba(10, 10, 18, 0.7);
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1rem;
  backdrop-filter: blur(6px);
  transition: all 0.15s;
}

.pv-fs-btn:hover {
  background: rgba(10, 10, 18, 0.9);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.pv-fs-btn.active {
  background: var(--accent-active-bg);
  color: var(--accent-light);
  border-color: var(--accent-light);
}

.pv-fs-close {
  position: absolute;
  right: 0.9rem;
  top: 50%;
  transform: translateY(-50%);
  background: rgba(231, 76, 60, 0.8);
  border-color: rgba(231, 76, 60, 0.9);
  color: #fff;
}

.pv-fs-close:hover {
  background: #e74c3c;
  color: #fff;
  border-color: #e74c3c;
}

.pv-fs-skybar {
  top: 0.75rem;
  right: 0.75rem;
  left: auto;
  bottom: auto;
  width: 300px;
  max-width: calc(100% - 1.5rem);
  border-radius: 12px;
  border: 1px solid var(--border-subtle);
}

.pv-mem-panel {
  position: fixed;
  right: 0.5rem;
  bottom: 0.5rem;
  width: 340px;
  max-height: 60vh;
  overflow-y: auto;
  z-index: 999;
  background: rgba(8, 8, 16, 0.92);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 0.4rem 0.5rem;
  backdrop-filter: blur(6px);
  font-size: 0.62rem;
  color: var(--text-secondary);
}

.pv-mem-head {
  display: flex;
  gap: 0.3rem;
  margin-bottom: 0.35rem;
}

.pv-mem-toggle,
.pv-mem-clear {
  padding: 0.2rem 0.45rem;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
  background: rgba(10, 10, 18, 0.7);
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 0.62rem;
}

.pv-mem-toggle:hover,
.pv-mem-clear:hover {
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.pv-mem-last {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  padding: 0.3rem;
  margin-bottom: 0.3rem;
  background: var(--accent-active-bg);
  border-radius: 6px;
}

.pv-mem-live {
  display: flex;
  flex-wrap: wrap;
  gap: 0.1rem 0.45rem;
  align-items: baseline;
  padding: 0.25rem 0.3rem;
  margin-bottom: 0.3rem;
  background: rgba(120, 200, 120, 0.12);
  border: 1px solid rgba(120, 200, 120, 0.25);
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
}

.pv-mem-live .pv-mem-tag {
  color: #7fdb8a;
  font-weight: 600;
}

.pv-mem-row {
  display: flex;
  gap: 0.4rem;
  padding: 0.15rem 0.2rem;
  border-bottom: 1px solid var(--border-subtle);
}

.pv-mem-k {
  color: var(--text-primary);
  white-space: nowrap;
}

.pv-mem-v {
  white-space: nowrap;
}
</style>
