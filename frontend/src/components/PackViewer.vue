<script setup>
import { ref, reactive, computed, onMounted, nextTick, watch } from 'vue'
import * as THREE from 'three'
import { SkinView3d } from 'vue-skinview3d'
import { IdleAnimation, WalkingAnimation } from 'vue-skinview3d/animations'
import { GetPackListWithInfo, GetInstalledPacks, GetPackPreviewInfo, GetPackArmorTextures, GetPackItemTextures, GetPackSkyTextures, GetPlayerSkinTexture, GetDefaultSkin, SaveDefaultSkin, GetCustomItems, SaveCustomItem, RemoveCustomItem, GetPackAllItemTextures, GetRemovedItems, SaveRemovedItem, RestoreRemovedItem, OpenFolder, IsDebug } from '../../wailsjs/go/main/App'
import defaultSkinImg from '../assets/default-skin.png'
import { parseBedrockCodes } from '../utils/formatCodes'

const packList = ref([])
const packsPath = ref('')
const loading = ref(true)
const searchQuery = ref('')
const sortBy = ref('name')

const showModal = ref(false)
const selectedPack = ref('')
const selectedPackInfo = ref({ name: '', description: '', iconURI: '' })
const selectedMaterial = ref('diamond')
const itemTextures = ref([])

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

const SKY_DEBUG_KEY = 'mew-sky-debug-config'
const skyDebugFaces = reactive({
  0: { rotation: 270, flipH: false },
  1: { rotation: 0,   flipH: false },
  2: { rotation: 90,  flipH: false },
  3: { rotation: 180, flipH: false },
  4: { rotation: 0,   flipH: false },
  5: { rotation: 0,   flipH: false },
})
const showSkyDebug = ref(false)

const customItemNames = ref([])
const removedItemNames = ref([])
const showItemPicker = ref(false)
const pickerItems = ref([])
const pickerLoading = ref(false)
const pickerSearch = ref('')

let viewerInstance = null

const SKY_FACE_META = [
  { key: 0, label: 'Left',     threeFace: '-X', bedrock: 'cubemap_0' },
  { key: 1, label: 'Behind',   threeFace: '-Z', bedrock: 'cubemap_1' },
  { key: 2, label: 'Right',    threeFace: '+X', bedrock: 'cubemap_2' },
  { key: 3, label: 'Front',    threeFace: '+Z', bedrock: 'cubemap_3' },
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
  skyDebugFaces[0].rotation = 270; skyDebugFaces[0].flipH = false
  skyDebugFaces[1].rotation = 0;   skyDebugFaces[1].flipH = false
  skyDebugFaces[2].rotation = 90;  skyDebugFaces[2].flipH = false
  skyDebugFaces[3].rotation = 180; skyDebugFaces[3].flipH = false
  skyDebugFaces[4].rotation = 0;   skyDebugFaces[4].flipH = false
  skyDebugFaces[5].rotation = 0;   skyDebugFaces[5].flipH = false
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

function setLastSkyTex(tex) { lastSkyTex = tex }

async function applySkyDebug() {
  if (!viewerInstance) return
  if (viewerInstance.scene.background && viewerInstance.scene.background.dispose) {
    viewerInstance.scene.background.dispose()
  }
  const cubemap = await generateSkyCubemap(lastSkyTex)
  if (viewerInstance) viewerInstance.scene.background = cubemap
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

async function generateSkyCubemap(skyTex) {
  const size = 512

  function makeCanvas(draw) {
    const c = document.createElement('canvas')
    c.width = size; c.height = size
    draw(c.getContext('2d'), size)
    return c
  }

  function skyGradient(ctx, s, topColor, bottomColor) {
    const g = ctx.createLinearGradient(0, 0, 0, s)
    g.addColorStop(0, topColor)
    g.addColorStop(1, bottomColor)
    ctx.fillStyle = g
    ctx.fillRect(0, 0, s, s)
  }

  function fallbackFace(topColor, bottomColor) {
    return makeCanvas((ctx, s) => {
      skyGradient(ctx, s, topColor, bottomColor)
      for (let i = 0; i < 30; i++) {
        ctx.fillStyle = `rgba(255,255,255,${0.2 + Math.random() * 0.6})`
        ctx.fillRect(Math.random() * s, Math.random() * s, 1 + Math.random() * 2, 1 + Math.random() * 2)
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
    xfaces[3], // +Z = behind camera = cubemap_3
    xfaces[1], // -Z = behind player = cubemap_1
  ]

  const fallbacks = [
    fallbackFace('#0e1e3d', '#3a7cc2'),
    fallbackFace('#0c1a35', '#3a7cc2'),
    fallbackFace('#070d1f', '#1a4a8a'),
    fallbackFace('#7ec8e3', '#dceefb'),
    fallbackFace('#10203f', '#2e6db3'),
    fallbackFace('#0b1630', '#2e6db3'),
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
  if (!q) return pickerItems.value
  return pickerItems.value.filter(item => item.name.toLowerCase().includes(q))
})

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1048576) return (bytes / 1024).toFixed(1) + ' KB'
  return (bytes / 1048576).toFixed(1) + ' MB'
}

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

function loadImage(uri) {
  return new Promise(resolve => {
    if (!uri) return resolve(null)
    const img = new Image(); img.crossOrigin = 'anonymous'
    img.onload = () => resolve(img)
    img.onerror = () => resolve(null)
    img.src = uri
  })
}

function buildArmor(tex1, tex2) {
  if (!viewerInstance) return
  const skin = viewerInstance.playerObject.skin
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

function removeArmorMeshes() {
  if (!viewerInstance) return
  const skin = viewerInstance.playerObject.skin
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
  viewerInstance.camera.position.set(0, 8, 40)
  viewerInstance.controls.target.set(0, 2, 0)
  viewerInstance.controls.enableDamping = true
  viewerInstance.controls.dampingFactor = 0.08
  viewerInstance.controls.minDistance = 15
  viewerInstance.controls.maxDistance = 80
  viewerInstance.controls.update()
}

watch(viewerRef, async (newRef) => {
  if (!newRef) return
  const tryGetViewer = async () => {
    const v = newRef.viewer
    if (v) {
      viewerInstance = v
      configureViewer()
      try {
        const defSkin = await GetDefaultSkin()
        if (defSkin) customSkinURI.value = defSkin
      } catch {}
      const uri = customSkinURI.value || defaultSkinImg
      await viewerInstance.loadSkin(uri, { model: skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value })
      if (viewerInstance.scene.background && viewerInstance.scene.background.dispose) {
        viewerInstance.scene.background.dispose()
      }
      viewerInstance.scene.background = await generateSkyCubemap(null)
    } else {
      setTimeout(tryGetViewer, 50)
    }
  }
  tryGetViewer()
})

watch(() => customSkinURI.value, async (newUri) => {
  if (!viewerInstance || !showModal.value) return
  const uri = newUri || defaultSkinImg
  viewerInstance.loadSkin(uri, { model: skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value })
})

async function loadAllPacks() {
  try {
    const [list, info] = await Promise.all([GetPackListWithInfo(), GetInstalledPacks()])
    packsPath.value = info?.path || ''
    packList.value = (list || []).map(p => ({
      ...p,
      dirName: p.dirName || '',
    }))
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

async function openPack(packName) {
  selectedPack.value = packName
  selectedMaterial.value = 'diamond'
  showModal.value = true
  await nextTick()
  await loadCustomItems()
  await waitForViewer()
  await loadPackData(packName)
}

function waitForViewer() {
  if (viewerInstance) return Promise.resolve()
  return new Promise(resolve => {
    const check = setInterval(() => {
      if (viewerInstance) { clearInterval(check); resolve() }
    }, 50)
  })
}

function closeModal() {
  showModal.value = false
}

async function loadPackData(packName) {
  try {
    const [info, skyTex] = await Promise.all([
      GetPackPreviewInfo(packName),
      GetPackSkyTextures(packName),
    ])
    selectedPackInfo.value = info
    setLastSkyTex(skyTex)
    const cubemap = await generateSkyCubemap(skyTex)
    if (viewerInstance) {
      if (viewerInstance.scene.background && viewerInstance.scene.background.dispose) {
        viewerInstance.scene.background.dispose()
      }
      viewerInstance.scene.background = cubemap
    }
    await applySkin(packName)
    await applyArmor(packName, selectedMaterial.value)
    await loadItems(packName, selectedMaterial.value)
  } catch (e) {
    console.error('Failed to load pack data:', e)
  }
}

async function loadItems(packName, material) {
  const items = await GetPackItemTextures(packName, material)
  itemTextures.value = items || []
}

async function applySkin(packName) {
  if (!viewerInstance) return
  let skinURI = customSkinURI.value || await GetPlayerSkinTexture(packName) || defaultSkinImg
  await viewerInstance.loadSkin(skinURI, { model: skinModel.value === 'auto-detect' ? 'auto-detect' : skinModel.value })
}

async function applyArmor(packName, material) {
  selectedMaterial.value = material
  if (!viewerInstance) return
  removeArmorMeshes()
  try {
    const tex = await GetPackArmorTextures(packName, material)
    let img1 = null, img2 = null
    if (tex.layer1) img1 = await loadImage(tex.layer1)
    if (tex.layer2) img2 = await loadImage(tex.layer2)
    buildArmor(img1, img2)
    await loadItems(packName, material)
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
  pickerItems.value = []
  pickerSearch.value = ''
  try {
    const all = await GetPackAllItemTextures(selectedPack.value)
    pickerItems.value = all || []
  } catch (err) {
    console.error('Failed to load item textures:', err)
    pickerItems.value = []
  }
  pickerLoading.value = false
}

function closeItemPicker() {
  showItemPicker.value = false
  pickerItems.value = []
  pickerSearch.value = ''
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
    if (selectedPack.value) await loadItems(selectedPack.value, selectedMaterial.value)
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
    if (selectedPack.value) await loadItems(selectedPack.value, selectedMaterial.value)
  } catch (err) {
    console.error('Failed to remove item:', err)
  }
}

onMounted(async () => {
  try { isDebug.value = await IsDebug() } catch {}
  loadSkyDebugConfig()
  await loadAllPacks()
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
          <img v-if="pack.iconURI" :src="pack.iconURI" class="pv-card-icon" />
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
      <div class="pv-modal">
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
            <button class="pv-modal-folder" @click="openPackFolder" title="Open pack folder"><i class="fa fa-folder-open"></i></button>
            <button class="pv-modal-close" @click="closeModal"><i class="fa fa-xmark"></i></button>
          </div>
        </div>

        <div class="pv-viewer-wrap">
          <div ref="modalContainer" class="pv-3d">
            <SkinView3d
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
        </div>

        <div class="pv-materials">
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
        </div>
        <input ref="skinFileInput" type="file" accept="image/png" class="pv-hidden-input" @change="onSkinFileChange" />

        <div class="pv-items-section">
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

    <div v-if="showItemPicker" class="pv-picker-overlay" @click.self="closeItemPicker">
      <div class="pv-picker" @click.stop>
        <div class="pv-picker-head">
          <h3 class="pv-picker-title">Add Item to Grid</h3>
          <button class="pv-modal-close" @click="closeItemPicker"><i class="fa fa-xmark"></i></button>
        </div>
        <div v-if="!pickerLoading && pickerItems.length > 0" class="pv-picker-search">
          <i class="fa fa-search pv-picker-search-icon"></i>
          <input v-model="pickerSearch" class="pv-picker-search-input" placeholder="Search items..." />
        </div>
        <div v-if="pickerLoading" class="pv-picker-loading">
          <div class="pv-spinner"></div>
        </div>
        <div v-else-if="filteredPickerItems.length === 0" class="pv-picker-empty">
          <p>{{ pickerSearch ? 'No matching items found.' : 'No item textures found in this pack.' }}</p>
        </div>
        <div v-else class="pv-picker-grid">
          <div v-for="item in filteredPickerItems" :key="item.name"
            class="pv-picker-item"
            :class="{ used: itemTextures.some(i => i.name === item.name) }"
            @click="selectPickerItem(item.name)">
            <img :src="item.dataURI" class="pv-picker-item-img" />
            <span class="pv-picker-item-name">{{ formatName(item.name) }}</span>
            <div v-if="itemTextures.some(i => i.name === item.name)" class="pv-picker-item-check">
              <i class="fa fa-check"></i>
            </div>
          </div>
        </div>
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
  border-color: var(--accent-light);
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
</style>
