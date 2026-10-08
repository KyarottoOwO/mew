<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import * as THREE from 'three'
import PackExporter from './PackExporter.vue'
import JsonUiStage from './JsonUiStage.vue'
import { GetSettings, SaveSettings, GetPackListWithInfo, GetInstalledPacks, GetPackPreviewInfo, GetPackItemTextures, GetPackSkyTextures, GetPackSkySubpacks, GetPackSkySubpackTextures, GetPackSkinThumbnails, SaveDefaultSkin, GetCustomItems, SaveCustomItem, RemoveCustomItem, GetPackItemTextureNames, GetPackItemTexture, GetRemovedItems, SaveRemovedItem, RestoreRemovedItem, OpenFolder, DeleteInstalledPack, IsDebug, PackHasUiScreens, RenderSkin, RenderSkinFrames, RenderSkinGIF, ListAnimations, RenderItem, RenderItemSpin, SaveRender } from '../../../wailsjs/go/main/App'

import { parseBedrockCodes } from '../../utils/formatCodes'
import { BACKGROUNDS, DEFAULT_BACKGROUND_ID, BACKGROUND_SETTING_KEY, normalizeBackgroundId, backgroundById, backgroundSwatchStyle as swatchStyle } from '../../utils/backgrounds'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
const props = defineProps({ active: Boolean, openPackReq: { type: Object, default: null } })

const packList = ref([])
const packsPath = ref('')
const loading = ref(true)
const searchQuery = ref('')
const sortBy = ref('name')

const showModal = ref(false)
const confirmDelete = ref(false)
const selectedPack = ref('')
const selectedPackInfo = ref({ name: '', description: '', iconURI: '' })
const selectedMaterial = ref('diamond')
const elytraOn = ref(false)
const equipmentOnly = ref(false)

// Each hand holds a tool of the selected tier, a flat item, or nothing.
const HAND_EMPTY = 'none'
const handTools = ['sword', 'pickaxe', 'axe', 'shovel', 'hoe']
const handToolLabels = { sword: 'Sword', pickaxe: 'Pickaxe', axe: 'Axe', shovel: 'Shovel', hoe: 'Hoe' }
const handFlat = ['bread', 'apple', 'golden_apple']
const handFlatLabels = { bread: 'Bread', apple: 'Apple', golden_apple: 'Golden Apple' }
const HELD_TIER = { cloth: 'stone', chain: 'stone', iron: 'iron', gold: 'gold', diamond: 'diamond', netherite: 'netherite', naked: 'diamond' }
const rightHand = ref(HAND_EMPTY)
const leftHand = ref(HAND_EMPTY)
const emptyAdjust = () => ({ offsetX: 0, offsetY: 0, offsetZ: 0, rotX: 0, rotY: 0, rotZ: 0, scale: 1 })
const rightAdjust = reactive(emptyAdjust())
const leftAdjust = reactive(emptyAdjust())
const rightAdjustOpen = ref(false)
const leftAdjustOpen = ref(false)

// Sliders for the per-hand item adjustment panel.
const ADJUST_SLIDERS = [
  { key: 'offsetX', label: 'X', min: -4, max: 4, step: 0.05 },
  { key: 'offsetY', label: 'Y', min: -4, max: 4, step: 0.05 },
  { key: 'offsetZ', label: 'Z', min: -4, max: 4, step: 0.05 },
  { key: 'rotX', label: 'RX', min: -180, max: 180, step: 1 },
  { key: 'rotY', label: 'RY', min: -180, max: 180, step: 1 },
  { key: 'rotZ', label: 'RZ', min: -180, max: 180, step: 1 },
  { key: 'scale', label: 'Size', min: 0.1, max: 3, step: 0.05 },
]

const itemTextures = ref([])
const itemCache = new Map()

const materials = ['naked', 'cloth', 'chain', 'iron', 'gold', 'diamond', 'netherite']
const materialLabels = {
  diamond: 'Diamond', gold: 'Gold', iron: 'Iron',
  chain: 'Chainmail', cloth: 'Leather', netherite: 'Netherite', naked: 'Naked'
}
const skinModel = ref('auto-detect')

// The view state, shared by the modal and fullscreen viewers: the sky camera
// and the player render use the same yaw/pitch/zoom.
const FOV = 35
const viewYaw = ref(0)
const viewPitch = ref(10)
const viewZoom = ref(1.5) // bedrock-skin-go's Margin: smaller is closer

// Animation: "" is a still; otherwise a motion or example animation name.
// 30 FPS is the backend's cap; the motion is sampled from a continuous curve,
// so the extra frames only smooth it out (the clip keeps its real duration).
const ANIM_FPS = 30
const animationName = ref('')
const animations = ref([])
const animFrames = ref([])
const animIndex = ref(0)
// animFramesFresh is whether the loaded frames match the current camera.
// animFramesFresh == false means the still path must keep drawing frames until
// the settled set arrives; cameraStamp changes with the camera so an in-flight
// load can tell it went stale.
let animFramesFresh = false
let cameraStamp = 0
let animTimer = null
let animToken = 0
let animDebounce = null

const modalPlayerSrc = ref('')
const fsPlayerSrc = ref('')
let renderToken = 0
let renderInFlight = false
let renderAgain = false
let destroyed = false
let vanillaRenderReadyHandler = null
let dragging = false
let dragMoved = false
let inertia = null

const customSkinURI = ref('')
const skinFileInput = ref(null)
// When true, the viewer draws the user's uploaded skin instead of the selected
// pack's own player skin. Set when they use "Change Skin" and remembered, so
// picking a skin visibly works even on packs that ship one.
const SKIN_OVERRIDE_KEY = 'mew.pv.skinOverride'
const skinOverride = ref(false)
try { skinOverride.value = localStorage.getItem(SKIN_OVERRIDE_KEY) === '1' } catch {}
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
const fsContainerRef = ref(null)
let fsViewerInstance = null

const showExportPopup = ref(false)

const IDLE_GC_MS = 10000

// Cap the 3D renderer's pixel ratio to bound GPU/JS memory on HiDPI displays.
const SKIN_PIXEL_RATIO = 1.5

// Cap the size used for animations. The whole clip is preloaded at this size and
// a rotating animation draws its stills at the same size, so sharpness never
// changes between turning the model and letting it loop. Bounds the cost of a
// long clip (idle is 80 frames).
const ANIM_MAX_SIZE = 512

const skySubpacks = ref([])
const skySlider = ref(0)

// Built-in backgrounds come from utils/backgrounds so the Settings tab offers
// the same list. A pack's own cubemap or subpack sky always takes precedence.
const BACKGROUND_PREFIX = 'bg:'
const backgroundId = ref(DEFAULT_BACKGROUND_ID)
// The choice is an app setting (Settings -> Background) so the settings page
// and this viewer always agree. Loaded fresh whenever a pack is opened.
async function loadBackgroundSetting() {
  try {
    const s = await GetSettings()
    backgroundId.value = normalizeBackgroundId(s && s[BACKGROUND_SETTING_KEY])
  } catch {
    backgroundId.value = DEFAULT_BACKGROUND_ID
  }
}
async function persistBackgroundSetting(id) {
  try {
    const s = (await GetSettings()) || {}
    s[BACKGROUND_SETTING_KEY] = id
    await SaveSettings(s)
  } catch {}
}
// True when the loaded pack ships a sky of its own (a base cubemap or
// subpacks). The viewer then shows that and ignores the background choice.
const packHasSky = ref(false)

function backgroundKey() { return BACKGROUND_PREFIX + backgroundId.value }
function currentBackgroundPreset() {
  return backgroundById(backgroundId.value)
}
function hasCubemapFaces(tex) {
  return !!(tex && (tex.cubemap0 || tex.cubemap1 || tex.cubemap2 || tex.cubemap3 || tex.cubemap4 || tex.cubemap5))
}

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
  if (viewerInstance) { viewerInstance.scene.background = cubemap; renderSky(viewerInstance) }
  if (fsViewerInstance) { fsViewerInstance.scene.background = cubemap; renderSky(fsViewerInstance) }
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

// One face of a preset background: a vertical gradient from the zenith through
// the horizon to the nadir on the sides, flat zenith on the top and flat nadir
// on the bottom. `stars` sprinkles points, as vanilla's night skies have.
function backgroundFace(size, preset, kind) {
  const zenith = preset.zenith || preset.solid || '#3a7cc2'
  const horizon = preset.horizon || preset.solid || '#bcd9f2'
  const nadir = preset.nadir || preset.solid || '#6f8f6a'
  const c = document.createElement('canvas')
  c.width = size; c.height = size
  const ctx = c.getContext('2d')
  if (kind === 'top') {
    ctx.fillStyle = zenith
  } else if (kind === 'bottom') {
    ctx.fillStyle = nadir
  } else {
    const g = ctx.createLinearGradient(0, 0, 0, size)
    g.addColorStop(0, zenith)
    g.addColorStop(0.55, horizon)
    g.addColorStop(1, nadir)
    ctx.fillStyle = g
  }
  ctx.fillRect(0, 0, size, size)
  if (preset.stars && kind !== 'bottom') {
    for (let i = 0; i < 140; i++) {
      ctx.globalAlpha = 0.25 + Math.random() * 0.7
      ctx.fillStyle = '#ffffff'
      ctx.beginPath()
      ctx.arc(Math.random() * size, Math.random() * size, 0.4 + Math.random() * 1.3, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.globalAlpha = 1
  }
  return c
}

// A whole preset background as a cube, in three.js face order (+X, -X, +Y, -Y,
// +Z, -Z). Built at 512 so stars stay crisp when stretched over the cube.
function buildBackgroundCubemap(preset, cap) {
  const size = (cap && Number.isFinite(cap)) ? Math.min(512, cap) : 512
  const faces = [
    backgroundFace(size, preset, 'side'),
    backgroundFace(size, preset, 'side'),
    backgroundFace(size, preset, 'top'),
    backgroundFace(size, preset, 'bottom'),
    backgroundFace(size, preset, 'side'),
    backgroundFace(size, preset, 'side'),
  ]
  const cube = new THREE.CubeTexture(faces)
  cube.needsUpdate = true
  return cube
}

async function generateSkyCubemap(skyTex, cap) {
  // The background choice is carried as a pseudo-texture, so the same build
  // path and cache serve it as serve a pack's cubemap.
  if (skyTex && skyTex.background) return buildBackgroundCubemap(skyTex.background, cap)

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

  // Faces the pack does not supply fall back to the chosen background, so a
  // partial cubemap still reads as a sky.
  const bg = currentBackgroundPreset()
  const fallbacks = [
    backgroundFace(size, bg, 'side'),   // +X
    backgroundFace(size, bg, 'side'),   // -X
    backgroundFace(size, bg, 'top'),    // +Y
    backgroundFace(size, bg, 'bottom'), // -Y
    backgroundFace(size, bg, 'side'),   // +Z
    backgroundFace(size, bg, 'side'),   // -Z
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

// ---------------------------------------------------------------------------
// Viewer: a three.js sky layer behind a bedrock-skin-go player image. The
// backend draws the player (skin, armor, elytra, held items) exactly as the
// game does; three.js only turns the pack's cubemap with the same camera.
// ---------------------------------------------------------------------------

function clamp(v, lo, hi) { return Math.min(hi, Math.max(lo, v)) }

function animationLabel(name) {
  return (name || '')
    .replace(/^animation\.player\./, '')
    .replace(/_/g, ' ')
    .replace(/\b\w/g, c => c.toUpperCase())
}

async function loadAnimations() {
  try { animations.value = (await ListAnimations()) || [] } catch { animations.value = [] }
}

function handItemName(value) {
  if (!value || value === HAND_EMPTY) return ''
  if (handTools.includes(value)) return (HELD_TIER[selectedMaterial.value] || 'diamond') + '_' + value
  return value
}

function handAdjust(adjust) {
  return {
    Offset: [Number(adjust.offsetX) || 0, Number(adjust.offsetY) || 0, Number(adjust.offsetZ) || 0],
    Rotation: [Number(adjust.rotX) || 0, Number(adjust.rotY) || 0, Number(adjust.rotZ) || 0],
    Scale: adjust.scale === '' || adjust.scale == null ? 1 : Number(adjust.scale),
  }
}

function handRequest(value, adjust) {
  return { item: handItemName(value), adjust: handAdjust(adjust) }
}

function currentRequest(size, animation = '', frame = 0) {
  return {
    pack: selectedPack.value || '',
    model: skinModel.value === 'slim' ? 'slim' : skinModel.value === 'default' ? 'wide' : 'auto',
    material: selectedMaterial.value === 'naked' ? 'none' : selectedMaterial.value,
    elytra: elytraOn.value,
    right: handRequest(rightHand.value, rightAdjust),
    left: handRequest(leftHand.value, leftAdjust),
    view: 'body',
    angle: 'front',
    camera: { yaw: viewYaw.value, pitch: viewPitch.value, fov: FOV, margin: viewZoom.value },
    size,
    hideSkin: equipmentOnly.value,
    overrideSkin: skinOverride.value,
    animation,
    frame,
    fps: ANIM_FPS,
  }
}

function activeContainer() {
  return fullscreen.value ? fsContainerRef.value : modalContainer.value
}

function playerSize() {
  const el = activeContainer()
  let min = 400
  if (el) {
    const r = el.getBoundingClientRect()
    if (r.width > 0 && r.height > 0) min = Math.min(r.width, r.height)
  }
  const dpr = Math.min(window.devicePixelRatio || 1, SKIN_PIXEL_RATIO)
  return Math.round(clamp(min * dpr, 64, 1024))
}

// animationSize is the render size for animated views. Still poses render at the
// full playerSize; animations use the same capped size for both the stills drawn
// while the camera moves and the preloaded loop, so quality is consistent.
function animationSize() {
  return Math.min(playerSize(), ANIM_MAX_SIZE)
}

// Decode a data URI before showing it, so swapping frames never flashes.
const decodedSrc = new Set()
function decodeSrc(uri) {
  if (!uri || decodedSrc.has(uri)) return Promise.resolve()
  return new Promise(resolve => {
    const img = new Image()
    img.onload = () => {
      decodedSrc.add(uri)
      if (decodedSrc.size > 400) decodedSrc.clear()
      resolve()
    }
    img.onerror = () => resolve()
    img.src = uri
  })
}

async function setPlayerSrc(uri) {
  if (!uri) return
  await decodeSrc(uri)
  if (destroyed) return
  modalPlayerSrc.value = uri
  fsPlayerSrc.value = uri
}

async function renderPlayerStill() {
  // Never drop resolution while turning the model. A still pose renders at the
  // full playerSize; an animation uses the same size as its preloaded loop so
  // the image does not get softer when the clip starts looping.
  const size = animationName.value ? animationSize() : playerSize()
  const token = ++renderToken
  // While the camera moves - or its frame set is still loading - an animation
  // is drawn one frame at a time from its prepared set: a single cheap render
  // keeps the model turning with the hand and still animating, and because the
  // frame is framed by the animation's shared camera, whole-body motion keeps
  // its place.
  const req = currentRequest(size)
  if (animationName.value) {
    req.animation = animationName.value
    req.frame = animIndex.value
  }
  try {
    const uri = await RenderSkin(req)
    if (token !== renderToken || destroyed) return
    setPlayerSrc(uri)
  } catch (e) {
    if (isDebug.value) console.error('RenderSkin failed:', e)
  }
}

// pumpPlayerStill renders one still now, coalescing calls that arrive while a
// render is already in flight.
function pumpPlayerStill() {
  if (renderInFlight) { renderAgain = true; return }
  renderInFlight = true
  ;(async () => {
    do {
      renderAgain = false
      await renderPlayerStill()
    } while (renderAgain && !destroyed)
    renderInFlight = false
  })()
}

function schedulePlayerRender() {
  // A settled camera with current frames reloads the whole set and loops it
  // locally; anything else takes the cheap single-still path.
  if (animationName.value && !dragging && !dragMoved) { scheduleAnimationFrames(); return }
  pumpPlayerStill()
}

function refreshPlayer() {
  // An appearance change makes the loaded frames stale too: the still path
  // draws the new look until the matching set arrives.
  if (animationName.value) animFramesFresh = false
  schedulePlayerRender()
}

// Animation frames are fetched once per camera/pose change, then cycled
// locally at ANIM_FPS.
function showAnimationFrame() {
  // While the camera moves - or its frame set is stale - the still path owns
  // the image; the loop must not overwrite it with frames from the old camera.
  if (dragging || dragMoved || !animFramesFresh) return
  const frames = animFrames.value
  if (!frames.length) return
  setPlayerSrc(frames[animIndex.value % frames.length])
}

let animFetching = false
let animPending = false

async function loadAnimationFramesNow() {
  const name = animationName.value
  if (!name) return
  if (animFetching) { animPending = true; return }
  animFetching = true
  const token = ++animToken
  const stamp = cameraStamp
  try {
    const frames = await RenderSkinFrames(currentRequest(animationSize(), name))
    if (token !== animToken || destroyed) return
    if (!frames || !frames.length) return
    animFrames.value = frames
    if (animIndex.value >= frames.length) animIndex.value = 0
    // Warm the browser cache so the first swap has no blank flash.
    frames.forEach(decodeSrc)
    // These frames only match the current camera if it did not move while they
    // rendered; otherwise the next settle loads them again. Until then the
    // still path keeps the animation going.
    if (stamp === cameraStamp) {
      animFramesFresh = true
      showAnimationFrame()
    }
  } catch (e) {
    if (isDebug.value) console.error('RenderSkinFrames failed:', e)
  } finally {
    animFetching = false
    if (animPending) { animPending = false; loadAnimationFramesNow() }
  }
}

// Reload frames for the current camera once the rotation settles. The current
// frames stay in memory and the still path keeps drawing them until the
// matching set arrives, so the animation never pauses between the drag and the
// new frames.
function scheduleAnimationFrames() {
  if (animDebounce) clearTimeout(animDebounce)
  animDebounce = setTimeout(loadAnimationFramesNow, 120)
}

function startAnimationLoop() {
  stopAnimationLoop()
  if (!animationName.value) return
  animTimer = setInterval(() => {
    const len = animFrames.value.length
    if (len) animIndex.value = (animIndex.value + 1) % len
    // While following the camera (or waiting for its frame set), draw one still
    // per tick: the animation keeps playing even if the hand is briefly still,
    // and never freezes waiting for the settled set.
    if (dragging || dragMoved || !animFramesFresh) { pumpPlayerStill(); return }
    if (len) showAnimationFrame()
  }, Math.round(1000 / ANIM_FPS))
}

function stopAnimationLoop() {
  if (animTimer) { clearInterval(animTimer); animTimer = null }
}

async function setAnimation(name) {
  animationName.value = name || ''
  animIndex.value = 0
  animFramesFresh = false
  if (!animationName.value) {
    stopAnimationLoop()
    animFrames.value = []
    schedulePlayerRender()
    return
  }
  await loadAnimationFramesNow()
  startAnimationLoop()
}

async function savePNG() {
  try {
    const uri = await RenderSkin(currentRequest(1024))
    await SaveRender(uri, renderFileName('png'))
  } catch (e) { console.error('Save PNG failed:', e) }
}

async function saveGIF() {
  try {
    const name = animationName.value || 'walk'
    const uri = await RenderSkinGIF(currentRequest(512, name))
    await SaveRender(uri, renderFileName('gif'))
  } catch (e) { console.error('Save GIF failed:', e) }
}

function renderFileName(ext) {
  const anim = animationName.value ? animationName.value.split('.').pop() : 'pose'
  const pack = (selectedPack.value || 'mew').replace(/[^A-Za-z0-9._-]/g, '_')
  return pack + '-' + selectedMaterial.value + '-' + anim + '.' + ext
}

// --- sky layer -------------------------------------------------------------

function createSkyView(container) {
  const renderer = new THREE.WebGLRenderer({ antialias: false })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, SKIN_PIXEL_RATIO))
  const el = renderer.domElement
  el.style.position = 'absolute'
  el.style.inset = '0'
  el.style.width = '100%'
  el.style.height = '100%'
  el.style.display = 'block'
  container.appendChild(el)
  const scene = new THREE.Scene()
  const camera = new THREE.PerspectiveCamera(FOV, 1, 0.1, 100)
  camera.position.set(0, 0, 0)
  const v = { renderer, scene, camera, container, canvas: el, disposed: false }
  setupSkyInteraction(v)
  return v
}

function resizeSkyView(v, w, h) {
  if (!v || v.disposed || w <= 0 || h <= 0) return
  v.renderer.setPixelRatio(Math.min(window.devicePixelRatio || 1, SKIN_PIXEL_RATIO))
  v.renderer.setSize(w, h, false)
  v.camera.aspect = w / h
  v.camera.updateProjectionMatrix()
}

function renderSky(v) {
  if (!v || v.disposed) return
  const yaw = viewYaw.value * Math.PI / 180
  const pitch = viewPitch.value * Math.PI / 180
  v.camera.lookAt(
    -Math.sin(yaw) * Math.cos(pitch),
    -Math.sin(pitch),
    -Math.cos(yaw) * Math.cos(pitch)
  )
  v.renderer.render(v.scene, v.camera)
}

function renderAllSkies() {
  renderSky(viewerInstance)
  renderSky(fsViewerInstance)
}

function ensureSkyView(which) {
  const container = which === 'fs' ? fsContainerRef.value : modalContainer.value
  if (!container) return null
  let v = which === 'fs' ? fsViewerInstance : viewerInstance
  if (!v || v.disposed || v.container !== container) {
    if (v) disposeViewerResources(v)
    v = createSkyView(container)
    if (which === 'fs') fsViewerInstance = v
    else viewerInstance = v
  }
  const r = container.getBoundingClientRect()
  resizeSkyView(v, r.width, r.height)
  return v
}

function disposeViewerResources(v) {
  if (!v) return
  v.disposed = true
  try { v.scene.background = null } catch {}
  try { v.renderer.dispose() } catch {}
  try {
    const gl = v.renderer && v.renderer.getContext && v.renderer.getContext()
    const lose = gl && gl.getExtension('WEBGL_lose_context')
    if (lose) lose.loseContext()
  } catch {}
  try { v.canvas.remove() } catch {}
}

function setupSkyInteraction(v) {
  const el = v.canvas
  el.style.touchAction = 'none'
  el.style.cursor = 'grab'
  const pointers = new Map()
  let lastX = 0, lastY = 0, lastDist = 0, velX = 0, velY = 0

  const pinchDist = () => {
    const [a, b] = [...pointers.values()]
    return Math.hypot(a.x - b.x, a.y - b.y)
  }

  el.addEventListener('pointerdown', (e) => {
    cancelInertia()
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    try { el.setPointerCapture(e.pointerId) } catch {}
    el.style.cursor = 'grabbing'
    if (pointers.size === 1) {
      dragging = true; dragMoved = false
      lastX = e.clientX; lastY = e.clientY
      velX = 0; velY = 0
    } else if (pointers.size === 2) {
      lastDist = pinchDist()
    }
  })

  el.addEventListener('pointermove', (e) => {
    if (!pointers.has(e.pointerId)) return
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY })
    if (pointers.size >= 2) {
      const d = pinchDist()
      if (lastDist > 0 && d > 0) viewZoom.value = clamp(viewZoom.value * (lastDist / d), 0.6, 3)
      lastDist = d
      cameraChanged()
      schedulePlayerRender()
      return
    }
    const dx = e.clientX - lastX
    const dy = e.clientY - lastY
    if (!dragMoved && Math.abs(dx) + Math.abs(dy) < 2) return
    cameraChanged()
    dragging = true
    viewYaw.value += dx * 0.3
    viewPitch.value = clamp(viewPitch.value + dy * 0.3, -60, 60)
    velX = dx * 0.3
    velY = dy * 0.3
    lastX = e.clientX; lastY = e.clientY
    renderAllSkies()
    schedulePlayerRender()
  })

  const release = (e) => {
    if (!pointers.has(e.pointerId)) return
    pointers.delete(e.pointerId)
    try { el.releasePointerCapture(e.pointerId) } catch {}
    if (pointers.size === 0) {
      el.style.cursor = 'grab'
      dragging = false
      if (dragMoved && (Math.abs(velX) > 0.6 || Math.abs(velY) > 0.6)) startInertia(velX, velY)
      else finishDrag()
    } else if (pointers.size === 1) {
      lastDist = 0
    }
  }
  el.addEventListener('pointerup', release)
  el.addEventListener('pointercancel', release)

  el.addEventListener('wheel', (e) => {
    e.preventDefault()
    viewZoom.value = clamp(viewZoom.value + e.deltaY * 0.0015, 0.6, 3)
    markCameraMoving()
    schedulePlayerRender()
  }, { passive: false })
}

function startInertia(vx, vy) {
  cancelInertia()
  const step = () => {
    vx *= 0.94; vy *= 0.94
    if (Math.abs(vx) < 0.08 && Math.abs(vy) < 0.08) { inertia = null; finishDrag(); return }
    viewYaw.value += vx
    viewPitch.value = clamp(viewPitch.value + vy, -60, 60)
    cameraChanged()
    renderAllSkies()
    schedulePlayerRender()
    inertia = requestAnimationFrame(step)
  }
  inertia = requestAnimationFrame(step)
}

function cancelInertia() {
  if (inertia) { cancelAnimationFrame(inertia); inertia = null }
}

// cameraChanged records that the camera moved: the loaded frames no longer
// match it, so the still path takes over until a settle reloads them. The stamp
// lets an in-flight frame load tell it went stale.
function cameraChanged() {
  dragMoved = true
  animFramesFresh = false
  cameraStamp++
}

// settleCamera marks the camera as briefly still-moving: while it is, the
// player is drawn one still per move; once it settles, the animation reloads
// its frames for the final angle.
let cameraSettleTimer = null
function settleCamera(delay) {
  if (cameraSettleTimer) clearTimeout(cameraSettleTimer)
  cameraSettleTimer = setTimeout(() => {
    if (dragging || inertia) return
    dragMoved = false
    if (!animationName.value) { schedulePlayerRender(); return }
    // Frames for the angle the user stopped at are loaded on settle; until
    // then the still path keeps animating, so there is no pause.
    if (animFramesFresh) return
    scheduleAnimationFrames()
  }, delay)
}

function markCameraMoving() {
  cameraChanged()
  settleCamera(150)
}

function finishDrag() {
  dragging = false
  settleCamera(150)
}

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
  stopAnimationLoop()
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
  let list = packList.value

  const q = searchQuery.value.trim().toLowerCase()
  if (q) {
    list = list.filter(p => {
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

// Packs that override the player skin get a small 3D render of it on their
// card, drawn by the backend. Fetched in batches so the grid fills in quickly.
async function loadSkinThumbs(list) {
  const BATCH = 16
  for (let i = 0; i < list.length; i += BATCH) {
    const batch = list.slice(i, i + BATCH)
    try {
      const thumbs = await GetPackSkinThumbnails(batch.map(p => p.dirName).filter(Boolean))
      for (const p of batch) {
        if (thumbs && thumbs[p.dirName]) p.skinThumb = thumbs[p.dirName]
      }
    } catch (e) {
      console.error('Failed to load skin thumbnails:', e)
      return
    }
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

const fsIndex = ref(0)

function currentSkyBase() {
  if (!currentSkyKey) return selectedPack.value ? selectedPack.value + '#base' : null
  return currentSkyKey.replace(/:m$/, '').replace(/:fs$/, '')
}

function openFullscreen() {
  const idx = filteredPacks.value.findIndex(p => p.dirName === selectedPack.value)
  fsIndex.value = idx >= 0 ? idx : 0
  fullscreen.value = true
  nextTick(async () => {
    measureFs()
    const v = ensureSkyView('fs')
    const base = currentSkyBase()
    if (base && lastSkyTex) {
      try {
        const cubemap = await skyBuild(base, lastSkyTex)
        currentSkyKey = skyKey(base)
        if (v) { v.scene.background = cubemap; renderSky(v) }
      } catch {}
    }
    watchSkySizes()
    if (animationName.value) { await loadAnimationFramesNow(); startAnimationLoop() }
    else schedulePlayerRender()
    recordMem('fs open')
  })
}

async function closeFullscreen() {
  cancelInertia()
  const wasFsKey = currentSkyKey
  const base = currentSkyBase()
  fullscreen.value = false
  if (base && lastSkyTex) {
    try {
      const cubemap = await skyBuild(base, lastSkyTex)
      currentSkyKey = skyKey(base)
      if (viewerInstance) { viewerInstance.scene.background = cubemap; renderSky(viewerInstance) }
    } catch {}
  }
  disposeViewerResources(fsViewerInstance)
  fsViewerInstance = null
  if (wasFsKey && wasFsKey.endsWith(':fs')) {
    const t = skyCache.get(wasFsKey)
    if (t) { try { t.dispose() } catch {}; skyCache.delete(wasFsKey) }
    skyPending.delete(wasFsKey)
  }
  watchSkySizes()
  schedulePlayerRender()
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

let skyResizeObserver = null
function watchSkySizes() {
  if (skyResizeObserver) skyResizeObserver.disconnect()
  skyResizeObserver = new ResizeObserver(() => {
    if (viewerInstance && modalContainer.value) {
      const r = modalContainer.value.getBoundingClientRect()
      resizeSkyView(viewerInstance, r.width, r.height)
      renderSky(viewerInstance)
    }
    if (fsViewerInstance && fsContainerRef.value) {
      const r = fsContainerRef.value.getBoundingClientRect()
      resizeSkyView(fsViewerInstance, r.width, r.height)
      renderSky(fsViewerInstance)
    }
    schedulePlayerRender()
  })
  if (modalContainer.value) skyResizeObserver.observe(modalContainer.value)
  if (fsContainerRef.value) skyResizeObserver.observe(fsContainerRef.value)
}

async function loadAllPacks() {
  try {
    const [list, info] = await Promise.all([GetPackListWithInfo(), GetInstalledPacks()])
    packsPath.value = info?.path || ''
    packList.value = (list || []).map(p => ({
      ...p,
      dirName: p.dirName || '',
      skinThumb: '',
    }))
    thumbifyPackIcons(packList.value)
    loadSkinThumbs(packList.value)
  } catch (e) {
    console.error('Failed to load packs:', e)
  }
  loading.value = false
}

function openPackFolder() {
  if (!packsPath.value || !selectedPack.value) return
  const sep = packsPath.value.includes('\\') ? '\\' : '/'
  OpenFolder(packsPath.value + sep + selectedPack.value)
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
  elytraOn.value = false
  equipmentOnly.value = false
  rightHand.value = HAND_EMPTY
  leftHand.value = HAND_EMPTY
  Object.assign(rightAdjust, emptyAdjust())
  Object.assign(leftAdjust, emptyAdjust())
  rightAdjustOpen.value = false
  leftAdjustOpen.value = false
  stopAnimationLoop()
  animationName.value = ''
  animFrames.value = []
  animFramesFresh = false
  showModal.value = true
  await nextTick()
  ensureSkyView('modal')
  watchSkySizes()
  await loadCustomItems()
  await loadPackData(packName)
}

function closeModal() {
  showModal.value = false
  confirmDelete.value = false
  fullscreen.value = false
  cancelInertia()
  stopAnimationLoop()
  if (animDebounce) { clearTimeout(animDebounce); animDebounce = null }
  if (skyResizeObserver) { skyResizeObserver.disconnect(); skyResizeObserver = null }
  disposeViewerResources(viewerInstance)
  disposeViewerResources(fsViewerInstance)
  viewerInstance = null
  fsViewerInstance = null
  animationName.value = ''
  animFrames.value = []
  animFramesFresh = false
  setLastSkyTex(null)
  currentSkyKey = null
  skySubpacks.value = []
  skySlider.value = 0
  packHasSky.value = false
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
    textureCache.delete(packName + '|sky')
    // Which sky? The pack's own wins: a base cubemap, or a subpack when the
    // base has none. Only a pack with no sky at all falls back to the user's
    // chosen background. The background lives in the app settings.
    await loadBackgroundSetting()
    skySubpacks.value = (await GetPackSkySubpacks(packName)) || []
    skySlider.value = 0
    packHasSky.value = hasCubemapFaces(skyTex) || skySubpacks.value.length > 0
    if (hasCubemapFaces(skyTex)) {
      setLastSkyTex(skyTex)
      const cubemap = await skyBuild(packName + '#base', skyTex)
      currentSkyKey = skyKey(packName + '#base')
      setSkyOnViewers(cubemap)
    } else if (skySubpacks.value.length > 0) {
      await applySkySubpack(0)
    } else {
      await applyBackground()
    }
    recordMem('stage:sky')
    await loadItems(packName)
    recordMem('stage:items')
    await applyPlayerAppearance()
    recordMem('load pack')
    armIdleGC()
    scheduleIdleGC(null, 'load pack')
  } catch (e) {
    console.error('Failed to load pack data:', e)
  }
}

async function applyPlayerAppearance() {
  if (animationName.value) {
    await loadAnimationFramesNow()
    startAnimationLoop()
  } else {
    renderAgain = false
    await renderPlayerStill()
  }
}

async function reloadPack() {
  const pack = selectedPack.value
  if (!pack) return
  invalidateSkyCache(pack)
  itemCache.delete(pack)
  invalidateTextureCache(pack)
  await loadPackData(pack)
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
  recordMem('sky subpack')
}

// Show the user's chosen background. A pack's own sky takes precedence, so
// this does nothing while one is loaded - the menu is disabled for that case.
async function applyBackground() {
  if (packHasSky.value) return
  const preset = currentBackgroundPreset()
  const key = backgroundKey()
  setLastSkyTex({ background: preset })
  const cubemap = await skyBuild(key, lastSkyTex)
  currentSkyKey = skyKey(key)
  setSkyOnViewers(cubemap)
  recordMem('background ' + preset.id)
}

function setBackground(id) {
  if (packHasSky.value) return
  backgroundId.value = normalizeBackgroundId(id)
  persistBackgroundSetting(backgroundId.value)
  return applyBackground()
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

function selectMaterial(material) {
  selectedMaterial.value = material
  if (material === 'naked') elytraOn.value = false
  recordMem('material ' + material)
  armIdleGC()
  scheduleIdleGC(null, 'armor')
  refreshPlayer()
}

function formatName(name) {
  return name.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
}

function triggerSkinUpload() { skinFileInput.value?.click() }

function setSkinOverride(on) {
  skinOverride.value = on
  try { localStorage.setItem(SKIN_OVERRIDE_KEY, on ? '1' : '0') } catch {}
  refreshPlayer()
}

async function onSkinFileChange(e) {
  const file = e.target.files?.[0]
  if (!file) return
  const reader = new FileReader()
  reader.onload = async () => {
    customSkinURI.value = reader.result
    try { await SaveDefaultSkin(reader.result) } catch (err) { console.error('Failed to save skin:', err) }
    setSkinOverride(true)
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
    const names = await GetPackItemTextureNames(selectedPack.value)
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
    const dataURI = await cachedTextureFetch(selectedPack.value + '|picker|' + name, () => GetPackItemTexture(selectedPack.value, name))
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

// --- item 3D dialog ---------------------------------------------------------
const showItem3D = ref(false)
const item3DName = ref('')
const item3DLoading = ref(false)
const item3DError = ref('')
const item3DFront = ref('')
const item3DIso = ref('')
const item3DSpin = ref('')

async function openItem3D(name) {
  item3DName.value = name
  item3DLoading.value = true
  item3DError.value = ''
  item3DFront.value = ''
  item3DIso.value = ''
  item3DSpin.value = ''
  showItem3D.value = true
  await nextTick()
  loadItem3D()
  recordMem('item 3d open')
}

async function loadItem3D() {
  const pack = selectedPack.value
  const item = item3DName.value
  if (!pack || !item) return
  try {
    const [front, iso, spin] = await Promise.all([
      RenderItem(pack, item, 'front', 256),
      RenderItem(pack, item, 'iso', 256),
      RenderItemSpin(pack, item, 256),
    ])
    item3DFront.value = front
    item3DIso.value = iso
    item3DSpin.value = spin
  } catch (e) {
    item3DError.value = String(e?.message || e)
  } finally {
    item3DLoading.value = false
  }
}

function closeItem3D() {
  showItem3D.value = false
  item3DName.value = ''
  item3DFront.value = ''
  item3DIso.value = ''
  item3DSpin.value = ''
  item3DError.value = ''
}

function itemFileName(name, label) {
  const pack = (selectedPack.value || 'mew').replace(/[^A-Za-z0-9._-]/g, '_')
  return pack + '-' + name.replace(/[^A-Za-z0-9._-]/g, '_') + '-' + label
}

async function saveItemPNG(uri, label) {
  try { await SaveRender(uri, itemFileName(item3DName.value, label + '.png')) }
  catch (e) { console.error('Save item PNG failed:', e) }
}

async function saveItemSpin() {
  try { await SaveRender(item3DSpin.value, itemFileName(item3DName.value, 'spin.gif')) }
  catch (e) { console.error('Save item spin failed:', e) }
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
  loadAnimations()
  await loadAllPacks()
  window.addEventListener('resize', onFsResize)
  // The backend prefetches the renderer's vanilla textures in the background.
  // When any arrive, redraw: a viewer that opened early may be showing a
  // placeholder for a texture that is now on disk.
  vanillaRenderReadyHandler = EventsOn('vanillaRenderReady', () => refreshPlayer())
  if (isDebug.value && showMemPanel.value) startMemSampler()
  recordMem('mounted')
  startGCWatchdog()
})

onUnmounted(() => {
  destroyed = true
  if (vanillaRenderReadyHandler) { vanillaRenderReadyHandler(); vanillaRenderReadyHandler = null }
  stopGCWatchdog()
  if (gcTimer) { clearTimeout(gcTimer); gcTimer = null }
  stopMemSampler()
  window.removeEventListener('resize', onFsResize)
  window.removeEventListener('keydown', onFsNavKey)
  cancelInertia()
  stopAnimationLoop()
  if (animDebounce) { clearTimeout(animDebounce); animDebounce = null }
  if (skyResizeObserver) { skyResizeObserver.disconnect(); skyResizeObserver = null }
  disposeViewerResources(viewerInstance)
  disposeViewerResources(fsViewerInstance)
  clearSkyCache()
  viewerInstance = null
  fsViewerInstance = null
})

function onFsResize() {
  if (fullscreen.value) measureFs()
}

// Any control that changes the player (hands, model, armor, elytra) kicks off a
// fresh render; the sky is untouched.
watch([rightHand, leftHand, elytraOn, equipmentOnly, skinModel, () => selectedMaterial.value], refreshPlayer)
watch([rightAdjust, leftAdjust], refreshPlayer, { deep: true })

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
    <div v-if="packList.length === 0 && !loading" class="pv-empty">
      <i class="fa fa-box-open pv-empty-icon"></i>
      <p>No installed packs found.</p>
      <p class="pv-empty-sub">Install a pack from the Pack Porter first.</p>
    </div>

    <template v-else>
      <div class="pv-toolbar">
        <div class="pv-search">
          <i class="fa fa-magnifying-glass pv-search-icon"></i>
          <input v-model="searchQuery" class="pv-search-input" placeholder="Search packs..." />
          <span v-if="searchQuery" class="pv-search-clear" @click="searchQuery = ''"><i class="fa fa-xmark"></i></span>
        </div>
        <div class="pv-toolbar-right">
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
        <div v-for="pack in filteredPacks" :key="pack.dirName" class="pv-card" @click="openPack(pack.dirName)">
          <div class="pv-card-art">
            <img v-if="pack.iconURI" :src="pack.iconURI" class="pv-card-icon" width="56" height="56" loading="lazy" decoding="async" />
            <div v-else class="pv-card-icon pv-card-placeholder">
              <i class="fa fa-box"></i>
            </div>
            <img v-if="pack.skinThumb" :src="pack.skinThumb" class="pv-card-skin" width="64" height="64" title="Pack preview: skin, diamond armor and sword" alt="Pack preview" decoding="async" />
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
            <button class="pv-modal-delete" :class="{ confirming: confirmDelete }" @click="deletePack" :title="confirmDelete ? 'Click again to delete' : 'Delete pack'">
              <i :class="confirmDelete ? 'fa fa-triangle-exclamation' : 'fa fa-trash'"></i>
            </button>
            <button class="pv-modal-close" @click="closeModal"><i class="fa fa-xmark"></i></button>
          </div>
        </div>

        <div v-show="viewerMode === 'texture'" class="pv-viewer-wrap">
          <div ref="modalContainer" class="pv-3d">
            <img v-if="modalPlayerSrc" :src="modalPlayerSrc" class="pv-player-img" width="512" height="512" alt="Player preview" />
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
          <div v-else class="pv-skybar pv-bgbar" :class="{ 'pv-bgbar-locked': packHasSky }">
            <div class="pv-skybar-head">
              <div class="pv-skybar-label"><i class="fa fa-image"></i> Background</div>
              <div v-if="packHasSky" class="pv-skybar-current"><i class="fa fa-lock"></i> Pack sky</div>
              <div v-else class="pv-skybar-current">{{ currentBackgroundPreset().name }}</div>
            </div>
            <div class="pv-bgbar-options">
              <button v-for="bg in BACKGROUNDS" :key="bg.id"
                class="pv-bg-swatch" :class="{ active: backgroundId === bg.id }"
                :style="swatchStyle(bg)" :disabled="packHasSky" :title="bg.name"
                @click="setBackground(bg.id)"></button>
            </div>
          </div>
        </div>

        <JsonUiStage v-if="viewerMode === 'ui'" :pack="selectedPack" @close="viewerMode = 'texture'" />

        <div v-show="viewerMode === 'texture'" class="pv-materials">
          <button v-for="m in materials" :key="m"
            class="pv-mat-btn" :class="{ active: selectedMaterial === m }"
            @click="selectMaterial(m)">
            {{ materialLabels[m] }}
          </button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: elytraOn }" @click="elytraOn = !elytraOn"
            title="Wings in place of the chestplate">
            <i class="fa fa-feather"></i> Elytra
          </button>
          <button class="pv-mat-btn" :class="{ active: equipmentOnly }" @click="equipmentOnly = !equipmentOnly"
            title="Hide the skin and show only armor, wings and items">
            <i class="fa fa-shirt"></i> Gear
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
          <label class="pv-anim">
            <i class="fa fa-film"></i>
            <select class="pv-anim-select" :value="animationName" @change="setAnimation($event.target.value)">
              <option value="">Still</option>
              <option v-for="a in animations" :key="a" :value="a">{{ animationLabel(a) }}</option>
            </select>
          </label>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" @click="savePNG" title="Save a high-resolution PNG">
            <i class="fa fa-image"></i> PNG
          </button>
          <button class="pv-mat-btn" :disabled="!animationName" @click="saveGIF" title="Save the current animation as a GIF">
            <i class="fa fa-film"></i> GIF
          </button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn pv-skin-btn" @click="triggerSkinUpload">
            <i class="fa fa-user-pen"></i> Change Skin
          </button>
          <button v-if="skinOverride" class="pv-mat-btn pv-reload-btn" @click="setSkinOverride(false)" title="Show the pack's own skin again">
            <i class="fa fa-rotate-left"></i> Pack Skin
          </button>
          <button class="pv-mat-btn pv-reload-btn" @click="reloadPack" title="Reload this pack from disk">
            <i class="fa fa-rotate"></i> Reload
          </button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" @click="showExportPopup = true" title="Export this pack as a .mcpack file">
            <i class="fa fa-file-export"></i> Export .mcpack
          </button>
        </div>
        <div v-show="viewerMode === 'texture'" class="pv-materials pv-held">
          <span class="pv-held-label"><i class="fa fa-hand"></i> Right</span>
          <select class="pv-hand-select" :value="rightHand" @change="rightHand = $event.target.value">
            <option :value="HAND_EMPTY">Empty</option>
            <option v-for="t in handTools" :key="t" :value="t">{{ handToolLabels[t] }}</option>
            <option v-for="f in handFlat" :key="f" :value="f">{{ handFlatLabels[f] }}</option>
          </select>
          <button class="pv-mat-btn pv-adjust-btn" :class="{ active: rightAdjustOpen }"
            @click="rightAdjustOpen = !rightAdjustOpen" title="Fine-tune where the item sits"><i class="fa fa-sliders"></i></button>
          <span class="pv-held-label"><i class="fa fa-hand"></i> Left</span>
          <select class="pv-hand-select" :value="leftHand" @change="leftHand = $event.target.value">
            <option :value="HAND_EMPTY">Empty</option>
            <option v-for="t in handTools" :key="t" :value="t">{{ handToolLabels[t] }}</option>
            <option v-for="f in handFlat" :key="f" :value="f">{{ handFlatLabels[f] }}</option>
          </select>
          <button class="pv-mat-btn pv-adjust-btn" :class="{ active: leftAdjustOpen }"
            @click="leftAdjustOpen = !leftAdjustOpen" title="Fine-tune where the item sits"><i class="fa fa-sliders"></i></button>
        </div>
        <div v-show="viewerMode === 'texture' && rightAdjustOpen" class="pv-adjust">
          <span class="pv-adjust-title">Right hand placement</span>
          <label v-for="s in ADJUST_SLIDERS" :key="'r' + s.key" class="pv-adjust-row">
            <span class="pv-adjust-label">{{ s.label }}</span>
            <input type="range" class="pv-adjust-range" :min="s.min" :max="s.max" :step="s.step" v-model.number="rightAdjust[s.key]" />
          </label>
          <button class="pv-mat-btn" @click="Object.assign(rightAdjust, emptyAdjust())">Reset</button>
        </div>
        <div v-show="viewerMode === 'texture' && leftAdjustOpen" class="pv-adjust">
          <span class="pv-adjust-title">Left hand placement</span>
          <label v-for="s in ADJUST_SLIDERS" :key="'l' + s.key" class="pv-adjust-row">
            <span class="pv-adjust-label">{{ s.label }}</span>
            <input type="range" class="pv-adjust-range" :min="s.min" :max="s.max" :step="s.step" v-model.number="leftAdjust[s.key]" />
          </label>
          <button class="pv-mat-btn" @click="Object.assign(leftAdjust, emptyAdjust())">Reset</button>
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
                <button class="pv-item-3d" @click.stop="openItem3D(item.name)" title="See this item in 3D">
                  <i class="fa fa-cube"></i>
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
        <img v-if="fsPlayerSrc" :src="fsPlayerSrc" class="pv-player-img" width="512" height="512" alt="Player preview" />
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
        <div v-else class="pv-skybar pv-fs-skybar pv-bgbar" :class="{ 'pv-bgbar-locked': packHasSky }">
          <div class="pv-skybar-head">
            <div class="pv-skybar-label"><i class="fa fa-image"></i> Background</div>
            <div v-if="packHasSky" class="pv-skybar-current"><i class="fa fa-lock"></i> Pack sky</div>
            <div v-else class="pv-skybar-current">{{ currentBackgroundPreset().name }}</div>
          </div>
          <div class="pv-bgbar-options">
            <button v-for="bg in BACKGROUNDS" :key="bg.id"
              class="pv-bg-swatch" :class="{ active: backgroundId === bg.id }"
              :style="swatchStyle(bg)" :disabled="packHasSky" :title="bg.name"
              @click="setBackground(bg.id)"></button>
          </div>
        </div>
      </div>
      <div class="pv-fs-bar">
        <div class="pv-fs-controls">
          <button v-for="m in materials" :key="m" class="pv-mat-btn" :class="{ active: selectedMaterial === m }"
            @click="selectMaterial(m)">{{ materialLabels[m] }}</button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: elytraOn }" @click="elytraOn = !elytraOn" title="Elytra"><i class="fa fa-feather"></i></button>
          <button class="pv-mat-btn" :class="{ active: equipmentOnly }" @click="equipmentOnly = !equipmentOnly" title="Gear only"><i class="fa fa-shirt"></i></button>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'auto-detect' }" @click="skinModel = 'auto-detect'"><i class="fa fa-robot"></i></button>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'default' }" @click="skinModel = 'default'"><i class="fa fa-person"></i></button>
          <button class="pv-mat-btn" :class="{ active: skinModel === 'slim' }" @click="skinModel = 'slim'"><i class="fa fa-person-dress"></i></button>
          <span class="pv-mat-sep"></span>
          <select class="pv-hand-select" :value="rightHand" @change="rightHand = $event.target.value" title="Right hand">
            <option :value="HAND_EMPTY">Right: empty</option>
            <option v-for="t in handTools" :key="'r' + t" :value="t">{{ handToolLabels[t] }}</option>
            <option v-for="f in handFlat" :key="'rf' + f" :value="f">{{ handFlatLabels[f] }}</option>
          </select>
          <select class="pv-hand-select" :value="leftHand" @change="leftHand = $event.target.value" title="Left hand">
            <option :value="HAND_EMPTY">Left: empty</option>
            <option v-for="t in handTools" :key="'l' + t" :value="t">{{ handToolLabels[t] }}</option>
            <option v-for="f in handFlat" :key="'lf' + f" :value="f">{{ handFlatLabels[f] }}</option>
          </select>
          <span class="pv-mat-sep"></span>
          <select class="pv-anim-select" :value="animationName" @change="setAnimation($event.target.value)">
            <option value="">Still</option>
            <option v-for="a in animations" :key="a" :value="a">{{ animationLabel(a) }}</option>
          </select>
          <span class="pv-mat-sep"></span>
          <button class="pv-mat-btn" @click="savePNG" title="Save PNG"><i class="fa fa-image"></i></button>
          <button class="pv-mat-btn" :disabled="!animationName" @click="saveGIF" title="Save GIF"><i class="fa fa-film"></i></button>
          <button class="pv-mat-btn pv-skin-btn" @click="triggerSkinUpload"><i class="fa fa-user-pen"></i></button>
          <button class="pv-mat-btn pv-reload-btn" @click="reloadPack" title="Reload this pack from disk"><i class="fa fa-rotate"></i></button>
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

    <div v-if="showItem3D" class="pv-picker-overlay" @click.self="closeItem3D">
      <div class="pv-item3d" @click.stop>
        <div class="pv-picker-head">
          <h3 class="pv-picker-title">3D: {{ formatName(item3DName) }}</h3>
          <button class="pv-modal-close" @click="closeItem3D"><i class="fa fa-xmark"></i></button>
        </div>
        <div v-if="item3DLoading" class="pv-picker-loading">
          <div class="pv-spinner"></div>
        </div>
        <div v-else-if="item3DError" class="pv-item3d-error">{{ item3DError }}</div>
        <div v-else class="pv-item3d-body">
          <div class="pv-item3d-cell">
            <div class="pv-item3d-frame"><img :src="item3DFront" class="pv-item3d-img" alt="Front view" /></div>
            <span class="pv-item3d-cap">Front</span>
            <button class="pv-mat-btn" @click="saveItemPNG(item3DFront, 'front')"><i class="fa fa-image"></i> Save PNG</button>
          </div>
          <div class="pv-item3d-cell">
            <div class="pv-item3d-frame"><img :src="item3DIso" class="pv-item3d-img" alt="Isometric view" /></div>
            <span class="pv-item3d-cap">Iso</span>
            <button class="pv-mat-btn" @click="saveItemPNG(item3DIso, 'iso')"><i class="fa fa-image"></i> Save PNG</button>
          </div>
          <div class="pv-item3d-cell">
            <div class="pv-item3d-frame"><img :src="item3DSpin" class="pv-item3d-img" alt="Spinning view" /></div>
            <span class="pv-item3d-cap">Spin</span>
            <button class="pv-mat-btn" @click="saveItemSpin"><i class="fa fa-film"></i> Save GIF</button>
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

.pv-card-art {
  position: relative;
  display: flex;
}

.pv-card-skin {
  position: absolute;
  right: -38px;
  bottom: -8px;
  width: 64px;
  height: 64px;
  filter: drop-shadow(0 2px 3px rgba(0, 0, 0, 0.35));
  pointer-events: none;
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
.pv-modal-close {
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
  position: relative;
  width: 100%;
  height: 400px;
  border-radius: 0;
  background: transparent;
  overflow: hidden;
}

.pv-3d canvas { display: block; }

/* The player is a transparent bedrock-skin-go render laid over the sky
   canvas and centred in the square that fits the viewer. object-fit keeps the
   displayed size fixed however many pixels a still or frame was rendered at,
   so releasing a drag does not change the apparent zoom. */
.pv-player-img {
  position: absolute;
  inset: 0;
  margin: auto;
  z-index: 1;
  width: 100%;
  height: 100%;
  object-fit: contain;
  pointer-events: none;
  image-rendering: auto;
}

.pv-anim {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  color: var(--text-dim);
}

.pv-anim-select,
.pv-hand-select {
  padding: 0.35rem 0.5rem;
  border-radius: 8px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  color: var(--text-secondary);
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
  max-width: 160px;
}

.pv-hand-select {
  max-width: 130px;
}

.pv-adjust-btn {
  padding: 0.35rem 0.55rem;
}

.pv-adjust {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 0.4rem 0.8rem;
  margin-top: 0.4rem;
  padding: 0.5rem 0.75rem;
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
  background: var(--bg-hover-1);
}

.pv-adjust-title {
  width: 100%;
  text-align: center;
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-dim);
}

.pv-adjust-row {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.7rem;
  color: var(--text-dim);
}

.pv-adjust-label {
  min-width: 28px;
  text-align: right;
}

.pv-adjust-range {
  width: 110px;
}

.pv-mat-btn:disabled {
  opacity: 0.45;
  cursor: default;
}

.pv-materials {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
  justify-content: center;
}

.pv-held {
  align-items: center;
  margin-top: 0.4rem;
}

.pv-held-label {
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-right: 0.25rem;
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

.pv-item-3d {
  position: absolute;
  top: -4px;
  left: -4px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: none;
  background: #2980b9;
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

.pv-item-card:hover .pv-item-remove,
.pv-item-card:hover .pv-item-3d {
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

.pv-item3d {
  background: var(--bg-body);
  border: 1px solid var(--border-default);
  border-radius: 14px;
  width: 90vw;
  max-width: 560px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.pv-item3d-body {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  justify-content: center;
  padding: 1rem;
  overflow: auto;
}

.pv-item3d-cell {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
}

.pv-item3d-frame {
  width: 160px;
  height: 160px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-hover-1);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.pv-item3d-img {
  max-width: 100%;
  max-height: 100%;
  image-rendering: pixelated;
}

.pv-item3d-cap {
  font-size: 0.75rem;
  color: var(--text-dim);
}

.pv-item3d-error {
  color: #f44;
  padding: 1rem;
  text-align: center;
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

.pv-bgbar-options {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.pv-bg-swatch {
  width: 30px;
  height: 30px;
  padding: 0;
  border-radius: 8px;
  border: 2px solid var(--border-subtle);
  cursor: pointer;
  transition: transform 0.12s, border-color 0.12s, box-shadow 0.12s;
}

.pv-bg-swatch:hover:not(:disabled) {
  transform: translateY(-1px);
  border-color: var(--accent-light);
}

.pv-bg-swatch.active {
  border-color: var(--accent);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--accent) 35%, transparent);
}

.pv-bg-swatch:disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.pv-bgbar-locked .pv-skybar-current i { margin-right: 0.25rem; }

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
