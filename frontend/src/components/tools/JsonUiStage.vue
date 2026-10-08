<script setup>
import { ref, computed, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { EventsOn } from '../../../wailsjs/runtime/runtime'
import {
  PackHasUiScreens,
  GetPackAllUiResources,
  GetPackFile,
  GetVanillaUiFiles,
  GetVanillaFile,
  GetVanillaCacheStatus,
  ClearVanillaCache,
} from '../../../wailsjs/go/main/App'
import { Editor as MenuEditor, initJsonUi } from '../../assets/jsonui/initJsonUi.js'

const props = defineProps({ pack: { type: String, required: true } })
const emit = defineEmits(['close'])

function b64ToBytes(b64) {
  return Uint8Array.from(atob(b64), c => c.charCodeAt(0))
}

const editor = ref(null)
const editorReady = ref(false)
const layers = ref([])
const canvasRef = ref(null)
const screens = ref([])
const screen = ref('')
const screenFilter = ref('')
const sizeSelect = ref('1280x720')
const scaleSelect = ref('0')
const contextSelect = ref('Desktop (Windows)')
const diagnostics = ref('')
const diagnosticsClass = ref('ok')
const frameInfo = ref('—')
const boxesInfo = ref('—')
const renderStatus = ref('')
const error = ref('')
const loading = ref(true)

const useVanilla = ref(true)
// With the vanilla base loaded the picker would list thousands of vanilla
// controls, so by default only the references the pack itself defines are shown.
const onlyPackScreens = ref(true)
const packRefs = ref(new Set())
const vanillaStatus = ref('')
const vanillaDownloading = ref(false)
let vanillaProgressHandler = null

const filteredScreens = computed(() => {
  const q = screenFilter.value.trim().toLowerCase()
  let list = screens.value
  if (onlyPackScreens.value && packRefs.value.size) {
    list = list.filter(s => packRefs.value.has(s.reference))
  }
  if (!q) return list
  return list.filter(s => s.reference.toLowerCase().includes(q))
})

const jsonCommentRe = /\/\/[^\r\n]*/g

// Reads the depth-1 keys out of a ui json. JSON.parse is too strict here:
// Bedrock files may contain raw control characters inside strings, which is why
// the renderer accepts them and why we cannot lean on the parser.
function topLevelKeys(text) {
  const keys = []
  let depth = 0
  let inStr = false
  let esc = false
  let keyStart = -1
  for (let i = 0; i < text.length; i++) {
    const ch = text[i]
    if (inStr) {
      if (esc) { esc = false; continue }
      if (ch === '\\') { esc = true; continue }
      if (ch === '"') { inStr = false }
      continue
    }
    if (ch === '/' && text[i + 1] === '/') {
      while (i < text.length && text[i] !== '\n') i++
      continue
    }
    if (ch === '"') {
      if (depth === 1 && keyStart < 0) keyStart = i
      inStr = true
      continue
    }
    if (ch === '{' || ch === '[') { depth++; keyStart = -1; continue }
    if (ch === '}' || ch === ']') { depth--; keyStart = -1; continue }
    if (ch === ',' && depth === 1) { keyStart = -1; continue }
    if (ch === ':' && depth === 1 && keyStart >= 0) {
      // Strip the surrounding quotes the slice keeps.
      keys.push(text.slice(keyStart + 1, i).replace(/^"|"$/g, ''))
      keyStart = -1
    }
  }
  return keys
}

// References a pack defines itself, read from the namespace of each ui json it
// ships. Anything the pack does not declare is vanilla and hidden by default.
function collectPackRefs(files) {
  const refs = new Set()
  for (const f of files) {
    if (!f.path.startsWith('ui/') || !f.path.endsWith('.json')) continue
    if (f.path === 'ui/_ui_defs.json') continue
    const text = new TextDecoder().decode(f.bytes).replace(jsonCommentRe, '')
    const ns = /"namespace"\s*:\s*"([^"]+)"/.exec(text)?.[1]
    if (!ns) continue
    for (const raw of topLevelKeys(text)) {
      // Keys may inherit a vanilla control; the renderer reports the bare name.
      const key = raw.trim().split('@')[0].trim()
      if (!key || key === 'namespace' || key === 'ui_defs') continue
      if (key.startsWith('$') || key.startsWith('#')) continue
      refs.add(ns + '.' + key)
    }
  }
  return refs
}

const sizeOptions = [
  { label: '1920×1080', value: '1920x1080' },
  { label: '1280×720', value: '1280x720' },
  { label: '800×600', value: '800x600' },
  { label: '400×240', value: '400x240' },
]
const scaleOptions = [
  { label: 'Auto', value: '0' },
  { label: '1', value: '1' },
  { label: '2', value: '2' },
  { label: '3', value: '3' },
  { label: '4', value: '4' },
]
const contextOptions = [
  { label: 'Desktop (Windows)', value: 'Desktop (Windows)' },
  { label: 'Desktop (macOS)', value: 'Desktop (macOS)' },
  { label: 'Pocket (touch)', value: 'Pocket (touch)' },
  { label: 'Console', value: 'Console' },
  { label: 'Empty', value: 'Empty' },
]

// Prefer a real screen over a sub-control so the first render shows a menu
// rather than a single template fragment.
const wellKnownScreens = ['start_screen', 'pause_screen', 'hud_screen', 'inventory_screen', 'world_templates_screen']

function pickDefaultScreen(list) {
  const scoped = onlyPackScreens.value && packRefs.value.size
    ? list.filter(s => packRefs.value.has(s.reference))
    : list
  const pool = scoped.length ? scoped : list
  if (!pool.length) return ''
  if (pool.find(s => s.screen)) return pool.find(s => s.screen).reference
  for (const name of wellKnownScreens) {
    const hit = pool.find(s => s.reference === name || s.reference.endsWith('.' + name))
    if (hit) return hit.reference
  }
  return pool.find(s => /screen$/i.test(s.reference))?.reference ?? pool[0].reference
}

async function refreshVanillaStatus() {
  try {
    const st = await GetVanillaCacheStatus()
    if (!st) return
    // Wails marshals with the Go json tags, so these keys are lower camel case.
    vanillaStatus.value = st.ready
      ? `Vanilla cached (${st.cached} files, ${((st.sizeKb || 0) / 1024).toFixed(1)} MB)`
      : 'Vanilla not cached yet'
  } catch {
    vanillaStatus.value = 'Vanilla status unavailable'
  }
}

async function clearVanillaCache() {
  try {
    await ClearVanillaCache()
    await refreshVanillaStatus()
  } catch (e) {
    console.error('Failed to clear vanilla cache:', e)
  }
}

function disposeEditor() {
  if (editor.value) {
    try { editor.value.free() } catch {}
    editor.value = null
  }
  editorReady.value = false
  layers.value = []
}

async function load() {
  if (!props.pack) return
  const token = ++loadToken
  const stale = () => token !== loadToken
  error.value = ''
  renderStatus.value = ''
  diagnostics.value = ''
  frameInfo.value = '—'
  boxesInfo.value = '—'
  screens.value = []
  screen.value = ''
  loading.value = true
  disposeEditor()

  try {
    renderStatus.value = 'Initializing renderer…'
    await initJsonUi()
    if (stale()) return
    editor.value = new MenuEditor()
    editorReady.value = true

    renderStatus.value = 'Loading pack UI…'
    const layerFiles = []

    // Layer 0: vanilla definitions so packs that only ship overrides still
    // resolve their bases. Fetched on demand and cached on disk by the backend.
    if (useVanilla.value) {
      vanillaDownloading.value = true
      try {
        const files = (await GetVanillaUiFiles()) || []
        if (stale()) return
        layerFiles.push({ name: 'vanilla', files: files.map(f => ({ path: f.path, bytes: new TextEncoder().encode(f.content) })) })
      } catch (e) {
        vanillaStatus.value = 'Vanilla unavailable (offline?)'
        console.warn('vanilla ui unavailable:', e)
      }
      vanillaDownloading.value = false
    }

    // Layer 1: the pack itself, textures registered as pending.
    const resources = (await GetPackAllUiResources(props.pack)) || []
    if (stale()) return
    const eagerRe = /(^|\/)(ui\/.*\.json|texts\/.*|pack\.json)$/i
    const imgRe = /(^|\/)(textures\/.*\.(png|tga|jpe?g))$/i
    const staged = []
    const pending = []
    const inline = new Map()
    for (const res of resources) {
      if (eagerRe.test(res.path)) {
        staged.push({ path: res.path, bytes: new TextEncoder().encode(res.content) })
      } else if (imgRe.test(res.path)) {
        pending.push(res.path)
        if (res.content) inline.set(res.path, b64ToBytes(res.content))
      }
    }
    layerFiles.push({ name: props.pack, files: staged, pending, inline })
    packRefs.value = collectPackRefs(staged)

    const ed = editor.value
    for (const layer of layerFiles) {
      if (editor.value !== ed || stale()) return
      const idx = ed.add_layer(layer.name)
      for (const f of layer.files) ed.stage_file(f.path, f.bytes)
      for (const p of layer.pending || []) ed.stage_pending(p)
      ed.commit_files(idx)
    }
    // Byte store per layer index, used to answer the renderer's texture requests.
    // Only textures are kept: staged json has already been handed to wasm and
    // holding another few megabytes of it per pack is pure waste.
    layers.value = layerFiles.map((l, index) => {
      const store = new Map()
      for (const [path, bytes] of l.inline || []) store.set(path, { key: path, bytes })
      for (const f of l.files) {
        if (f.path.startsWith('textures/')) store.set(f.path, { key: f.path, bytes: f.bytes })
      }
      return {
        index,
        name: l.name,
        files: store,
        declared: new Set(l.pending || []),
        supplied: new Set(),
      }
    })

    refreshVanillaStatus()

    try {
      if (editor.value !== ed || stale()) return
      const screensData = JSON.parse(ed.screens())
      screens.value = Array.isArray(screensData) ? screensData : []
      if (screens.value.length) screen.value = pickDefaultScreen(screens.value)
    } catch (e) {
      console.warn('screens() failed:', e)
    }

    renderStatus.value = `${screens.value.length} screens · ${resources.length} pack files`
    if (stale()) return
    if (screen.value) await doRender()
  } catch (e) {
    if (stale()) return
    error.value = 'Load failed: ' + (e?.message || e)
    console.error('jsonui load failed:', e)
  } finally {
    if (!stale()) {
      loading.value = false
      vanillaDownloading.value = false
    }
  }
}

// The renderer matches json references (extensionless) against registered file
// paths, so a texture must be declared and supplied under its real file path.
// resolveTexture reports which layer the bytes came from.
async function resolveTexture(p) {
  try {
    const f = await GetPackFile(props.pack, p)
    if (f && f.path && f.content) return { key: f.path, bytes: b64ToBytes(f.content), source: 'pack' }
  } catch {}
  if (!useVanilla.value) return null
  try {
    const f = await GetVanillaFile(p)
    if (f && f.path && f.content) return { key: f.path, bytes: b64ToBytes(f.content), source: 'vanilla' }
  } catch {}
  return null
}

const missingTextureRe = /texture `([^`]+)` is in no loaded layer/

// ── Canvas pan / zoom ────────────────────────────────────────────────────────
const canvasWrapRef = ref(null)
const frameSize = ref({ w: 0, h: 0 })
const zoom = ref(1)
const panX = ref(0)
const panY = ref(0)
const dragging = ref(false)
const userAdjustedView = ref(false)

const ZOOM_MIN = 0.05
const ZOOM_MAX = 16

// Guards against overlapping loads: switching packs or toggling vanilla mid
// load would otherwise leave two passes staging into the same editor.
let loadToken = 0

const canvasStyle = computed(() => ({
  width: frameSize.value.w + 'px',
  height: frameSize.value.h + 'px',
  transform: `translate(calc(-50% + ${panX.value}px), calc(-50% + ${panY.value}px)) scale(${zoom.value})`,
}))

function clampZoom(z) {
  return Math.min(Math.max(z, ZOOM_MIN), ZOOM_MAX)
}

// Zoom keeping the point under the cursor (or the view centre) anchored.
function zoomAt(factor, clientX, clientY) {
  const wrap = canvasWrapRef.value
  if (!wrap || !frameSize.value.w) return
  const rect = wrap.getBoundingClientRect()
  const sx = clientX !== undefined ? clientX - rect.left - rect.width / 2 : 0
  const sy = clientY !== undefined ? clientY - rect.top - rect.height / 2 : 0
  const next = clampZoom(zoom.value * factor)
  const k = next / zoom.value
  panX.value = panX.value + (sx - panX.value) * (1 - k)
  panY.value = panY.value + (sy - panY.value) * (1 - k)
  zoom.value = next
  userAdjustedView.value = true
}

function zoomBy(dir) {
  zoomAt(dir > 0 ? 1.25 : 1 / 1.25)
}

function onWheel(e) {
  if (!frameSize.value.w) return
  e.preventDefault()
  zoomAt(e.deltaY < 0 ? 1.15 : 1 / 1.15, e.clientX, e.clientY)
}

function fitToView() {
  const wrap = canvasWrapRef.value
  if (!wrap || !frameSize.value.w) return
  const pad = 28
  const z = Math.min(
    (wrap.clientWidth - pad) / frameSize.value.w,
    (wrap.clientHeight - pad) / frameSize.value.h,
  )
  zoom.value = clampZoom(z)
  panX.value = 0
  panY.value = 0
  userAdjustedView.value = false
}

function resetView() {
  zoom.value = 1
  panX.value = 0
  panY.value = 0
  userAdjustedView.value = true
}

let dragFrom = null
function onPointerDown(e) {
  if (!frameSize.value.w || e.button !== 0) return
  dragging.value = true
  dragFrom = { x: e.clientX, y: e.clientY, px: panX.value, py: panY.value }
  canvasWrapRef.value?.setPointerCapture?.(e.pointerId)
}

function onPointerMove(e) {
  if (!dragFrom) return
  panX.value = dragFrom.px + (e.clientX - dragFrom.x)
  panY.value = dragFrom.py + (e.clientY - dragFrom.y)
}

function onPointerUp(e) {
  if (!dragFrom) return
  dragFrom = null
  dragging.value = false
  canvasWrapRef.value?.releasePointerCapture?.(e.pointerId)
}

function onResize() {
  if (!userAdjustedView.value) fitToView()
}

async function doRender() {
  if (!editor.value || !screen.value) return
  // Capture the instance: a render awaits texture fetches, and the panel can be
  // closed or the pack switched meanwhile, which frees the wasm handle. Calling
  // into it afterwards traps with "memory access out of bounds".
  const ed = editor.value
  const live = () => editor.value === ed
  const reference = screen.value.trim()
  if (!reference) return

  const [w, h] = sizeSelect.value.split('x').map(Number)
  const guiScale = Number(scaleSelect.value) || null
  const presets = JSON.parse(ed.context_presets())
  const context = presets[contextSelect.value] || {}

  const view = { reference, size: [w, h], gui_scale: guiScale, context }
  try {
    ed.set_view(JSON.stringify(view))
  } catch (e) {
    renderStatus.value = 'View error: ' + e
    return
  }

  renderStatus.value = 'Rendering…'
  let result = JSON.parse(ed.render())

  // Textures arrive in two ways. The renderer's `wanted` list holds textures
  // that block layout; the rest only show up as "is in no loaded layer"
  // diagnostics. Both are handled: resolve each path to a real file, declare it
  // pending in the right layer, commit, then hand over the bytes.
  for (let round = 0; round < 5; round++) {
    const wanted = result.wanted || []
    const wantedPaths = wanted.map(w => w[1])
    const missingPaths = (result.diagnostics || [])
      .map(d => d.message?.match(missingTextureRe)?.[1])
      .filter(Boolean)
    const paths = [...new Set([...wantedPaths, ...missingPaths])]
    if (!paths.length) break

    const layerOf = new Map(wanted.map(w => [w[1], w[0]]))
    const packIndex = layers.value.length - 1
    const vanillaIndex = layers.value.findIndex(l => l.name === 'vanilla')
    const touched = new Set()

    for (const p of paths) {
      const hinted = layerOf.get(p)
      let resolved = null
      for (const li of (hinted !== undefined ? [hinted] : [packIndex, vanillaIndex])) {
        if (li < 0 || !layers.value[li]) continue
        const found = layers.value[li].files.get(p)
        if (found) { resolved = { ...found, li }; break }
      }
      if (!resolved) {
        const r = await resolveTexture(p)
        if (!live()) return
        if (r) {
          const li = r.source === 'vanilla' ? vanillaIndex : packIndex
          if (li < 0 || !layers.value[li]) continue
          resolved = { key: r.key, bytes: r.bytes, li }
          layers.value[li].files.set(p, resolved)
        }
      }
      if (!resolved) continue
      const layer = layers.value[resolved.li]
      if (!layer.declared.has(resolved.key)) {
        ed.stage_pending(resolved.key)
        layer.declared.add(resolved.key)
        touched.add(resolved.li)
      }
    }
    if (!live()) return

    // Commit only the layers that gained paths, then refill everything they had
    // already supplied so a re-commit cannot lose earlier textures.
    for (const li of touched) {
      const layer = layers.value[li]
      ed.commit_files(layer.index)
      layer.supplied.clear()
      for (const [, entry] of layer.files) {
        try {
          ed.supply(layer.index, entry.key, entry.bytes)
          layer.supplied.add(entry.key)
        } catch {}
      }
    }
    if (!live()) return

    for (const p of paths) {
      for (const li of [layerOf.get(p), packIndex, vanillaIndex]) {
        if (li === undefined || li < 0 || !layers.value[li]) continue
        const layer = layers.value[li]
        const entry = layer.files.get(p)
        if (!entry || layer.supplied.has(entry.key)) continue
        try {
          ed.supply(li, entry.key, entry.bytes)
          layer.supplied.add(entry.key)
        } catch {}
        break
      }
    }
    if (!live()) return

    result = JSON.parse(ed.render())
    if (!result.wanted?.length && !(result.diagnostics || []).some(d => missingTextureRe.test(d.message || ''))) break
  }

  const diags = result.diagnostics || []
  diagnostics.value = diags.length ? diags.map(d => `${d.severity}: ${d.message}`).join('\n') : '(none)'
  diagnosticsClass.value = diags.length ? 'err' : 'ok'

  const n = result.boxes?.length ?? 0
  const vis = result.visible?.filter(Boolean).length ?? 0
  frameInfo.value = `Size: ${result.width}×${result.height}\nGUI scale: ${result.gui_scale}\nControls: ${n} (${vis} visible)\nAnimated: ${result.animated}`
  boxesInfo.value = (result.boxes || []).slice(0, 40).map(b =>
    `${b.name} [${b.type ?? '?'}] ${b.rect.map(v => v.toFixed(1)).join('×')}`
  ).join('\n') + (n > 40 ? `\n… and ${n - 40} more` : '')

  await nextTick()
  const canvas = canvasRef.value
  if (canvas) {
    try {
      const pixels = ed.paint(0)
      const imageData = new ImageData(
        new Uint8ClampedArray(pixels.buffer, pixels.byteOffset, pixels.byteLength),
        result.width, result.height
      )
      canvas.width = result.width
      canvas.height = result.height
      canvas.getContext('2d').putImageData(imageData, 0, 0)
      const sizeChanged = frameSize.value.w !== result.width || frameSize.value.h !== result.height
      frameSize.value = { w: result.width, h: result.height }
      if (sizeChanged) userAdjustedView.value = false
      await nextTick()
      if (!userAdjustedView.value) fitToView()
    } catch (e) {
      renderStatus.value = 'Paint failed: ' + e
      return
    }
  }

  renderStatus.value = `Rendered ${result.width}×${result.height} · ${n} controls`
}

function toggleVanilla() {
  useVanilla.value = !useVanilla.value
  load()
}

function close() {
  disposeEditor()
  emit('close')
}

watch(() => props.pack, () => {
  frameSize.value = { w: 0, h: 0 }
  zoom.value = 1
  panX.value = 0
  panY.value = 0
  userAdjustedView.value = false
  load()
})

onMounted(() => {
  load()
  refreshVanillaStatus()
  window.addEventListener('resize', onResize)
  vanillaProgressHandler = EventsOn('jsonuiVanillaProgress', (data) => {
    const done = parseInt(data?.done) || 0
    const total = parseInt(data?.total) || 0
    if (total > 0) {
      vanillaDownloading.value = true
      vanillaStatus.value = `Downloading vanilla UI ${done}/${total}…`
    }
  })
})

onUnmounted(() => {
  if (vanillaProgressHandler) { vanillaProgressHandler(); vanillaProgressHandler = null }
  window.removeEventListener('resize', onResize)
  disposeEditor()
})
</script>

<template>
  <div class="jui-stage">
    <div class="jui-stage-head">
      <button class="jui-back" @click="close" title="Back to pack viewer">
        <i class="fa fa-arrow-left"></i>
      </button>
      <div class="jui-stage-titles">
        <h2 class="jui-stage-title"><i class="fa fa-bars-staggered"></i> UI Renderer</h2>
        <span class="jui-stage-pack">{{ pack }}</span>
      </div>
      <label class="jui-stage-packonly" title="Show only screens this pack defines">
        <input type="checkbox" v-model="onlyPackScreens" />
        <i class="fa fa-filter"></i>
        <span>Pack only</span>
      </label>
      <label class="jui-stage-vanilla" title="Render on top of vanilla Bedrock UI definitions">
        <input type="checkbox" :checked="useVanilla" @change="toggleVanilla" />
        <i class="fa fa-layer-group"></i>
        <span>Vanilla base</span>
      </label>
      <span class="jui-stage-vanilla-status">
        <i v-if="vanillaDownloading" class="fa fa-spinner fa-spin"></i>
        {{ vanillaStatus || 'Vanilla status…' }}
      </span>
      <button class="jui-stage-clear" title="Delete cached vanilla files" @click="clearVanillaCache">
        <i class="fa fa-trash-can"></i>
      </button>
      <span class="jui-stage-status">{{ renderStatus || 'Ready' }}</span>
    </div>

    <div v-if="error" class="jui-stage-error">{{ error }}</div>

    <template v-else>
      <div class="jui-stage-controls">
        <div class="jui-stage-field jui-stage-field-screen">
          <label>Screen</label>
          <input type="text" v-model="screenFilter" class="jui-stage-filter" placeholder="filter screens…" spellcheck="false" />
          <select v-model="screen" class="jui-stage-select" :disabled="loading" @change="doRender">
            <option v-for="s in filteredScreens" :key="s.reference" :value="s.reference">
              {{ s.reference }}{{ s.screen ? ' [screen]' : '' }}
            </option>
          </select>
          <span v-if="!filteredScreens.length" class="jui-stage-nomatch">
            {{ screens.length ? 'no screens match this filter' : 'no screens loaded' }}
          </span>
        </div>
        <div class="jui-stage-field">
          <label>Size</label>
          <select v-model="sizeSelect" class="jui-stage-select" @change="doRender">
            <option v-for="opt in sizeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <div class="jui-stage-field">
          <label>Scale</label>
          <select v-model="scaleSelect" class="jui-stage-select" @change="doRender">
            <option v-for="opt in scaleOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <div class="jui-stage-field">
          <label>Context</label>
          <select v-model="contextSelect" class="jui-stage-select" @change="doRender">
            <option v-for="opt in contextOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <button class="jui-stage-render" @click="doRender" :disabled="!screen || loading">
          <i class="fa fa-play"></i> Render
        </button>
        <span class="jui-stage-count">{{ filteredScreens.length }}<template v-if="filteredScreens.length !== screens.length"> / {{ screens.length }}</template> screens</span>
      </div>

      <div class="jui-stage-body">
        <div
          ref="canvasWrapRef"
          class="jui-stage-canvas-wrap"
          :class="{ dragging }"
          @wheel="onWheel"
          @pointerdown="onPointerDown"
          @pointermove="onPointerMove"
          @pointerup="onPointerUp"
          @pointercancel="onPointerUp"
          @dblclick="fitToView"
        >
          <div v-if="loading" class="jui-stage-loading">
            <i class="fa fa-spinner fa-spin"></i> {{ renderStatus || 'Loading…' }}
          </div>
          <canvas
            v-else-if="frameInfo !== '—'"
            ref="canvasRef"
            class="jui-stage-canvas"
            :style="canvasStyle"
          />
          <div v-else class="jui-stage-empty">Pick a screen and press Render</div>

          <div v-if="frameInfo !== '—'" class="jui-stage-zoombar" @pointerdown.stop @wheel.stop>
            <button title="Zoom out" @click="zoomBy(-1)"><i class="fa fa-minus"></i></button>
            <span class="jui-stage-zoomval">{{ Math.round(zoom * 100) }}%</span>
            <button title="Zoom in" @click="zoomBy(1)"><i class="fa fa-plus"></i></button>
            <button title="Fit to view" @click="fitToView"><i class="fa fa-compress"></i></button>
            <button title="Actual size" @click="resetView"><i class="fa fa-crosshairs"></i></button>
          </div>
        </div>

        <div class="jui-stage-dock">
          <div class="jui-stage-info">
            <span class="jui-stage-info-label">Diagnostics</span>
            <pre class="jui-stage-info-pre" :class="diagnosticsClass">{{ diagnostics }}</pre>
          </div>
          <div class="jui-stage-info">
            <span class="jui-stage-info-label">Frame</span>
            <pre class="jui-stage-info-pre ok">{{ frameInfo }}</pre>
          </div>
          <div class="jui-stage-info">
            <span class="jui-stage-info-label">Controls</span>
            <pre class="jui-stage-info-pre ok">{{ boxesInfo }}</pre>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.jui-stage {
  display: flex;
  flex-direction: column;
  width: 100%;
  min-width: 0;
  height: 100%;
  min-height: 14rem;
  overflow: hidden;
  background: var(--bg-body);
}

.jui-stage-head {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-surface);
  flex-shrink: 0;
}

.jui-back {
  background: none;
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0.25rem 0.55rem;
  font-size: 0.75rem;
  transition: all 0.15s;
  flex-shrink: 0;
}

.jui-back:hover {
  background: var(--bg-hover-2);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.jui-stage-titles {
  display: flex;
  flex-direction: column;
  min-width: 0;
  flex-shrink: 0;
}

.jui-stage-title {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin: 0;
}

.jui-stage-title .fa {
  color: var(--accent);
}

.jui-stage-pack {
  font-size: 0.65rem;
  color: var(--text-faint);
  font-family: 'Consolas', 'Courier New', monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 18rem;
}

.jui-stage-vanilla {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.7rem;
  color: var(--text-secondary);
  cursor: pointer;
  white-space: nowrap;
  margin-left: auto;
}

.jui-stage-packonly {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.7rem;
  color: var(--text-secondary);
  cursor: pointer;
  white-space: nowrap;
}

.jui-stage-packonly .fa {
  color: var(--accent);
}

.jui-stage-vanilla .fa {
  color: var(--accent);
}

.jui-stage-vanilla-status {
  font-size: 0.65rem;
  color: var(--text-dim);
  font-family: 'Consolas', 'Courier New', monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 16rem;
}

.jui-stage-clear {
  background: none;
  border: 1px solid var(--border-medium);
  border-radius: 5px;
  color: var(--text-dim);
  cursor: pointer;
  padding: 0.1rem 0.35rem;
  font-size: 0.65rem;
  line-height: 1;
  transition: all 0.15s;
  flex-shrink: 0;
}

.jui-stage-clear:hover {
  background: var(--bg-hover-2);
  color: var(--text-secondary);
  border-color: var(--border-focus);
}

.jui-stage-status {
  font-size: 0.68rem;
  color: var(--text-dim);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 20rem;
  flex-shrink: 0;
}

.jui-stage-error {
  padding: 1rem;
  color: #f85149;
  font-size: 0.8rem;
  border-bottom: 1px solid var(--border-subtle);
}

.jui-stage-controls {
  display: flex;
  align-items: flex-end;
  gap: 0.5rem;
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-body);
  flex-wrap: wrap;
  flex-shrink: 0;
}

.jui-stage-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.jui-stage-field label {
  font-size: 9px;
  color: var(--text-faint);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.jui-stage-field-screen {
  min-width: 0;
  flex: 1 1 18rem;
}

.jui-stage-filter {
  width: 100%;
  padding: 0.25rem 0.4rem;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 0.7rem;
  font-family: 'Consolas', 'Courier New', monospace;
  outline: none;
  transition: border-color 0.15s;
}

.jui-stage-filter:focus {
  border-color: var(--accent);
}

.jui-stage-filter::placeholder {
  color: var(--text-faint);
}

.jui-stage-select {
  background: var(--bg-input);
  color: var(--text-primary);
  border: 1px solid var(--border-medium);
  border-radius: 5px;
  padding: 3px 6px;
  font-size: 11px;
  cursor: pointer;
  outline: none;
  min-width: 90px;
}

.jui-stage-select:focus {
  border-color: var(--accent);
}

.jui-stage-nomatch {
  font-size: 0.65rem;
  color: var(--text-faint);
  font-style: italic;
}

.jui-stage-render {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: var(--accent-glow);
  color: var(--accent-light);
  border: 1px solid var(--accent-border);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 11px;
  cursor: pointer;
}

.jui-stage-render:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.jui-stage-count {
  font-size: 0.65rem;
  color: var(--text-faint);
  font-family: 'Consolas', 'Courier New', monospace;
  margin-left: auto;
}

.jui-stage-body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.jui-stage-canvas-wrap {
  flex: 1 1 auto;
  min-width: 0;
  /* Never let the canvas be squeezed out by the dock below it. */
  min-height: 12rem;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  touch-action: none;
  background:
    linear-gradient(45deg, var(--bg-hover-1) 25%, transparent 25%) -8px 0/16px 16px,
    linear-gradient(-45deg, var(--bg-hover-1) 25%, transparent 25%) -8px 0/16px 16px,
    linear-gradient(45deg, transparent 75%, var(--bg-hover-1) 75%) -8px 0/16px 16px,
    linear-gradient(-45deg, transparent 75%, var(--bg-hover-1) 75%) -8px 0/16px 16px;
}

.jui-stage-canvas-wrap.dragging {
  cursor: grabbing;
}

.jui-stage-canvas {
  position: absolute;
  left: 50%;
  top: 50%;
  transform-origin: center center;
  border: 1px solid var(--border-medium);
  image-rendering: pixelated;
  box-shadow: 0 4px 24px rgba(0, 0, 0, 0.5);
  pointer-events: none;
}

.jui-stage-zoombar {
  position: absolute;
  left: 0.75rem;
  bottom: 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.15rem;
  padding: 0.2rem 0.3rem;
  background: var(--bg-surface);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  box-shadow: 0 2px 10px rgba(0, 0, 0, 0.4);
}

.jui-stage-zoombar button {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0.15rem 0.35rem;
  font-size: 0.7rem;
  border-radius: 4px;
  line-height: 1;
}

.jui-stage-zoombar button:hover {
  background: var(--bg-hover-2);
  color: var(--text-primary);
}

.jui-stage-zoomval {
  font-size: 0.65rem;
  color: var(--text-dim);
  font-family: 'Consolas', 'Courier New', monospace;
  min-width: 3.2rem;
  text-align: center;
}

.jui-stage-dock {
  flex: 0 0 auto;
  /* Capped in both directions: a fixed baseline so the percentages below
     always resolve, and a ceiling so long diagnostics cannot take over. */
  height: 12rem;
  min-height: 6rem;
  max-height: 45%;
  display: grid;
  grid-template-columns: 1.4fr 0.8fr 1fr;
  grid-template-rows: minmax(0, 1fr);
  gap: 1px;
  background: var(--border-subtle);
  border-top: 1px solid var(--border-subtle);
  overflow: hidden;
}

.jui-stage-info {
  min-width: 0;
  min-height: 0;
  background: var(--bg-surface);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.jui-stage-info-label {
  display: block;
  font-size: 9px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-dim);
  padding: 5px 12px 3px;
  background: var(--bg-hover-1);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}

.jui-stage-info-pre {
  margin: 0;
  padding: 5px 12px 7px;
  font-size: 10px;
  color: var(--text-dim);
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  font-family: 'Consolas', 'Courier New', monospace;
}

.jui-stage-empty,
.jui-stage-loading {
  color: var(--text-faint);
  font-size: 0.85rem;
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.jui-stage-info-pre.ok {
  color: #3fb950;
}

.jui-stage-info-pre.err {
  color: #f85149;
}
</style>
