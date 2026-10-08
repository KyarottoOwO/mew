<script setup>
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { Editor, initJsonUi } from '../../assets/jsonui/initJsonUi.js'

const props = defineProps({ active: Boolean })

let editor = null
let layers = []
let frame = null

const canvasRef = ref(null)

const screenInput = ref('')
const sizeSelect = ref('1280x720')
const scaleSelect = ref('0')
const contextSelect = ref('Desktop (Windows)')

const diagnostics = ref('Waiting for render…')
const diagnosticsClass = ref('ok')
const frameInfo = ref('—')
const boxesInfo = ref('—')
const emptyHint = ref('Load a pack or example to render JSON-UI')
const renderStatus = ref('')

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

const canvasScale = ref(1)

function calcCanvasScale() {
  const canvas = canvasRef.value
  const container = canvas?.closest('.jui-preview')
  if (!canvas || !container) return
  const maxW = container.clientWidth - 48
  const maxH = container.clientHeight - 48
  if (!frame) { canvasScale.value = 1; return }
  const ratio = Math.min(maxW / frame.width, maxH / frame.height, 4)
  canvasScale.value = Math.max(ratio, 0.1)
}

async function initEditor() {
  try {
    await initJsonUi()
    editor = new Editor()
    renderStatus.value = 'Engine ready'
    loadExample()
  } catch (e) {
    renderStatus.value = 'WASM init failed: ' + e
    console.error(e)
  }
}

async function loadExample() {
  if (!editor) return
  renderStatus.value = 'Loading example…'
  try {
    const resp = await fetch('/jsonui/examples-files.json')
    const meta = await resp.json()

    layers = []
    for (const [layerName, paths] of Object.entries(meta.layers)) {
      const idx = editor.add_layer('example ' + layerName)
      const files = new Map()
      for (const rel of paths) {
        const blob = await fetch('/jsonui/examples/' + layerName + '/' + rel).then(r => r.blob())
        const bytes = new Uint8Array(await blob.arrayBuffer())
        editor.stage_file(rel, bytes)
        files.set(rel, bytes)
      }
      editor.commit_files(idx)
      insertLayer(idx, { name: 'example ' + layerName, files })
    }
    layers.reverse()
    screenInput.value = meta.screen
    doRender()
  } catch (e) {
    renderStatus.value = 'Example load failed: ' + e
    console.error(e)
  }
}

async function handleFolderInput(fileList) {
  if (!fileList.length || !editor) return
  renderStatus.value = 'Loading folder…'
  const groups = folderGroups(fileList)
  for (const g of groups) await addLayer(g.name, g.entries)
  doRender()
}

async function handleZipInput(file) {
  if (!file || !editor) return
  renderStatus.value = 'Loading zip…'
  const idx = editor.add_layer(file.name)
  const bytes = new Uint8Array(await file.arrayBuffer())
  try {
    await editor.add_zip(idx, bytes)
    insertLayer(idx, { name: file.name, files: new Map() })
    doRender()
    renderStatus.value = `${layers.length} layer(s) loaded`
  } catch (err) {
    editor.remove_layer(idx)
    renderStatus.value = 'ZIP error: ' + err
    console.error(err)
  }
}

async function addLayer(name, entries) {
  const idx = editor.add_layer(name)
  const files = new Map()
  const eagerRe = /(^|\/)(ui\/.*\.json|texts\/.*|pack\.json)$/i
  const imgRe = /(^|\/)textures\/.*\.(png|tga|jpe?g)$/i
  for (const [path, file] of entries) {
    const ab = new Uint8Array(await file.arrayBuffer())
    if (eagerRe.test(path)) {
      editor.stage_file(path, ab)
    } else if (imgRe.test(path)) {
      editor.stage_pending(path)
      files.set(path, ab)
    }
  }
  editor.commit_files(idx)
  insertLayer(idx, { name, files })
}

function insertLayer(index, entry) {
  layers.splice(index, 0, entry)
}

function folderGroups(fileList) {
  const byRoot = new Map()
  for (const file of fileList) {
    const path = file.webkitRelativePath || file.name
    const root = path.split('/')[0]
    if (!byRoot.has(root)) byRoot.set(root, [])
    byRoot.get(root).push([path, file])
  }
  return [...byRoot].map(([name, entries]) => ({ name, entries }))
}

async function doRender() {
  if (!editor) return
  const reference = screenInput.value.trim()
  if (!reference) return

  const [w, h] = sizeSelect.value.split('x').map(Number)
  const guiScale = Number(scaleSelect.value) || null
  const ctxPreset = contextSelect.value
  const presets = JSON.parse(editor.context_presets())
  const context = presets[ctxPreset] || {}

  const view = { reference, size: [w, h], gui_scale: guiScale, context }
  try {
    editor.set_view(JSON.stringify(view))
  } catch (e) {
    renderStatus.value = 'View error: ' + e
    return
  }

  renderStatus.value = 'Rendering…'
  let result = JSON.parse(editor.render())

  for (let round = 0; round < 4 && result.wanted?.length; round++) {
    for (const [layerIdx, path] of result.wanted) {
      const layer = layers[layerIdx]
      if (!layer) continue
      let bytes = layer.files.get(path)
      if (!bytes) {
        for (const [full, b] of layer.files) {
          if (full.endsWith('/' + path)) { bytes = b; break }
        }
      }
      if (bytes) editor.supply(layerIdx, path, bytes)
    }
    result = JSON.parse(editor.render())
  }

  frame = result
  const diags = result.diagnostics || []
  diagnostics.value = diags.length ? diags.join('\n') : '(none)'
  diagnosticsClass.value = diags.length ? 'err' : 'ok'

  const n = result.boxes?.length ?? 0
  const vis = result.visible?.filter(Boolean).length ?? 0
  frameInfo.value = `Size: ${result.width}×${result.height}\nGUI scale: ${result.gui_scale}\nControls: ${n} (${vis} visible)\nAnimated: ${result.animated}`

  boxesInfo.value = result.boxes?.slice(0, 20).map(b =>
    `${b.name} [${b.type ?? '?'}] ${b.rect.map(v => v.toFixed(1)).join('×')}`
  ).join('\n') + (n > 20 ? `\n… and ${n - 20} more` : '')

  paintCanvas(result)
  renderStatus.value = `Rendered ${result.width}×${result.height} · ${n} controls`
}

function paintCanvas(result) {
  const canvas = canvasRef.value
  if (!canvas || !result) return
  try {
    const pixels = editor.paint(0)
    const imageData = new ImageData(
      new Uint8ClampedArray(pixels.buffer, pixels.byteOffset, pixels.byteLength),
      result.width, result.height
    )
    canvas.width = result.width
    canvas.height = result.height
    canvas.getContext('2d').putImageData(imageData, 0, 0)
    emptyHint.value = ''
  } catch (e) {
    emptyHint.value = 'Paint failed: ' + e
  }
}

watch(() => props.active, (val) => {
  if (val) {
    nextTick(calcCanvasScale)
    if (editor && layers.length) doRender()
  }
})

onMounted(() => {
  initEditor()
  window.addEventListener('resize', calcCanvasScale)
  nextTick(calcCanvasScale)
})

onUnmounted(() => {
  window.removeEventListener('resize', calcCanvasScale)
  if (editor) {
    editor.free()
    editor = null
  }
})
</script>

<template>
  <div class="jui-page page">
    <div class="jui-toolbar">
      <div class="jui-toolbar-left">
        <span class="jui-brand"><i class="fa fa-image"></i> JSON-UI</span>
        <div class="jui-field">
          <label>Screen</label>
          <input
            type="text"
            v-model="screenInput"
            class="jui-input"
            placeholder="namespace.control"
            @change="doRender"
          />
        </div>
        <div class="jui-field">
          <label>Size</label>
          <select v-model="sizeSelect" class="jui-select" @change="doRender">
            <option v-for="opt in sizeOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <div class="jui-field">
          <label>Scale</label>
          <select v-model="scaleSelect" class="jui-select" @change="doRender">
            <option v-for="opt in scaleOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
        <div class="jui-field">
          <label>Context</label>
          <select v-model="contextSelect" class="jui-select" @change="doRender">
            <option v-for="opt in contextOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
          </select>
        </div>
      </div>
      <div class="jui-toolbar-right">
        <span class="jui-status" :class="{ 'jui-status-ok': !renderStatus.includes('error') && !renderStatus.includes('failed') }">{{ renderStatus || 'Initializing…' }}</span>
        <button class="jui-btn" @click="loadExample" title="Load the bundled example pack"><i class="fa fa-flask"></i> Example</button>
        <button class="jui-btn" @click="$refs.folderInput?.click()" title="Open a folder of pack files"><i class="fa fa-folder-open"></i> Folder</button>
        <input ref="folderInput" type="file" webkitdirectory multiple class="jui-hidden" @change="e => handleFolderInput(e.target.files)" />
        <button class="jui-btn" @click="$refs.zipInput?.click()" title="Open a .mcpack/.zip/.mcaddon"><i class="fa fa-file-zipper"></i> ZIP</button>
        <input ref="zipInput" type="file" accept=".zip,.mcpack,.mcaddon" class="jui-hidden" @change="e => handleZipInput(e.target.files[0])" />
        <button class="jui-btn jui-btn-primary" :disabled="!editor" @click="doRender"><i class="fa fa-play"></i> Render</button>
      </div>
    </div>

    <div class="jui-body">
      <div class="jui-preview">
        <canvas
          ref="canvasRef"
          v-if="frame"
          :style="{ width: Math.round(frame.width * canvasScale) + 'px', height: Math.round(frame.height * canvasScale) + 'px', imageRendering: 'pixelated' }"
        />
        <div v-if="!frame" class="jui-empty-hint">{{ emptyHint }}</div>
      </div>

      <div class="jui-sidebar">
        <div class="jui-panel">
          <div class="jui-panel-head">Diagnostics</div>
          <pre class="jui-pre" :class="diagnosticsClass">{{ diagnostics }}</pre>
        </div>

        <div class="jui-panel">
          <div class="jui-panel-head">Frame Info</div>
          <pre class="jui-pre jui-pre-ok">{{ frameInfo }}</pre>
        </div>

        <div class="jui-panel">
          <div class="jui-panel-head">Controls</div>
          <pre class="jui-pre jui-pre-ok">{{ boxesInfo }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.jui-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: hidden;
}

.jui-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
  flex-wrap: wrap;
}

.jui-toolbar-left {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  flex: 1;
}

.jui-toolbar-right {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.jui-brand {
  color: var(--accent);
  font-weight: 700;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 6px;
  margin-right: 4px;
}

.jui-field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.jui-field label {
  font-size: 10px;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.jui-input {
  background: var(--bg-input);
  color: var(--text-primary);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  padding: 4px 8px;
  font-size: 12px;
  width: 200px;
  outline: none;
}

.jui-input:focus {
  border-color: var(--accent);
}

.jui-select {
  background: var(--bg-input);
  color: var(--text-primary);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  padding: 4px 8px;
  font-size: 12px;
  cursor: pointer;
  outline: none;
}

.jui-select:focus {
  border-color: var(--accent);
}

.jui-status {
  font-size: 11px;
  color: var(--text-dim);
  padding: 0 8px;
  white-space: nowrap;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
}

.jui-status-ok {
  color: var(--text-secondary);
}

.jui-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: var(--bg-input);
  color: var(--text-secondary);
  border: 1px solid var(--border-medium);
  border-radius: 6px;
  padding: 5px 10px;
  font-size: 12px;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.15s;
}

.jui-btn:hover {
  background: var(--bg-hover-2);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.jui-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.jui-btn-primary {
  background: var(--accent-glow);
  color: var(--accent-light);
  border-color: var(--accent-border);
}

.jui-btn-primary:hover {
  background: var(--accent);
  color: var(--bg-body);
}

.jui-hidden {
  display: none;
}

.jui-body {
  flex: 1;
  display: flex;
  overflow: hidden;
  min-height: 0;
}

.jui-preview {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-body);
  position: relative;
  overflow: auto;
  padding: 24px;
  background-image:
    linear-gradient(45deg, var(--bg-checker-a) 25%, transparent 25%),
    linear-gradient(-45deg, var(--bg-checker-a) 25%, transparent 25%),
    linear-gradient(45deg, transparent 75%, var(--bg-checker-a) 75%),
    linear-gradient(-45deg, transparent 75%, var(--bg-checker-a) 75%);
  background-size: 16px 16px;
  background-position: 0 0, 0 8px, 8px -8px, -8px 0;
}

.jui-preview canvas {
  border: 1px solid var(--border-medium);
}

.jui-empty-hint {
  position: absolute;
  color: var(--text-faint);
  font-size: 13px;
  pointer-events: none;
}

.jui-sidebar {
  width: 300px;
  background: var(--bg-surface);
  border-left: 1px solid var(--border-subtle);
  padding: 12px;
  overflow-y: auto;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.jui-panel {
  background: var(--bg-body);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  overflow: hidden;
}

.jui-panel-head {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-dim);
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-hover-1);
}

.jui-pre {
  font-size: 11px;
  color: var(--text-dim);
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.6;
  margin: 0;
  padding: 8px 12px;
  max-height: 160px;
  overflow-y: auto;
  font-family: 'Consolas', 'Courier New', monospace;
}

.jui-pre.ok {
  color: #3fb950;
}

.jui-pre.err {
  color: #f85149;
}
</style>
