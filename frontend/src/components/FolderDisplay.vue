<script setup>
import { ref, computed, onMounted, watch, nextTick } from 'vue'
import { GetImages, SaveImage, ExportPack, DeleteTemp } from '../../wailsjs/go/main/App'
import { applyHsvShift, upscaleNearestNeighbor } from '../utils/hue'

const props = defineProps({
  checkResult: Array,
  packName: String,
  sidebarWidth: {
    type: Number,
    default: 64
  }
})
const emit = defineEmits(['close'])

const folders = ref(props.checkResult || [])
const selectedFolder = ref('')
const images = ref([])
const loading = ref(false)
const selectedImage = ref(null)
const hue = ref(0)
const sat = ref(0)
const bright = ref(0)
const modifiedImages = ref({})
const previewCanvas = ref(null)
const originalImage = ref(null)
const saving = ref(false)
const originalDataURIs = ref({})

const paintMode = ref(false)
const hoveredImage = ref(null)
const searchQuery = ref('')
const allImages = ref([])

async function loadAllFolders() {
  const collected = []
  const seen = new Set()
  for (const folder of folders.value) {
    try {
      const raw = await GetImages(folder)
      for (const img of raw) {
        if (seen.has(img.relPath)) continue
        seen.add(img.relPath)
        originalDataURIs.value[img.relPath] = img.dataURI
        const scaled = await upscaleNearestNeighbor(img.dataURI, 8)
        collected.push({ ...img, dataURI: scaled, folder })
      }
    } catch (_) {}
  }
  allImages.value = collected
}

const filteredImages = computed(() => {
  if (!searchQuery.value) return images.value
  const q = searchQuery.value.toLowerCase()
  return allImages.value.filter(img => img.filename.toLowerCase().includes(q) || img.relPath.toLowerCase().includes(q))
})

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

async function loadFolder(folder) {
  selectedFolder.value = folder
  selectedImage.value = null
  loading.value = true
  try {
    const raw = await GetImages(folder)
    for (const img of raw) {
      originalDataURIs.value[img.relPath] = img.dataURI
    }
    const upscaled = await Promise.all(raw.map(async (img) => {
      const scaled = await upscaleNearestNeighbor(img.dataURI, 8)
      return { ...img, dataURI: scaled }
    }))
    images.value = upscaled
    if (allImages.value.length === 0) {
      await loadAllFolders()
    }
  } catch (err) {
    images.value = []
    popup('Error', 'Failed to load images: ' + err.toString(), 'error')
  }
  loading.value = false
}

async function selectImage(img) {
  if (searchQuery.value && img.folder && img.folder !== selectedFolder.value) {
    await loadFolder(img.folder)
    const found = images.value.find(i => i.relPath === img.relPath)
    if (!found) return
    img = found
  }
  if (paintMode.value) {
    paintImage(img)
    return
  }
  selectedImage.value = img
  hue.value = 0
  sat.value = 0
  bright.value = 0
  originalImage.value = null
  nextTick(() => drawPreview())
}

async function paintImage(img) {
  if (hue.value === 0 && sat.value === 0 && bright.value === 0) return
  const srcURI = originalDataURIs.value[img.relPath] || img.dataURI
  const canvas = document.createElement('canvas')
  const ctx = canvas.getContext('2d')
  const image = new Image()
  image.crossOrigin = 'anonymous'
  await new Promise((resolve) => {
    image.onload = resolve
    image.src = srcURI
  })
  canvas.width = image.naturalWidth
  canvas.height = image.naturalHeight
  ctx.drawImage(image, 0, 0)
  const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
  const shifted = applyHsvShift(imageData, hue.value, sat.value, bright.value)
  ctx.putImageData(shifted, 0, 0)
  const rawDataURL = canvas.toDataURL('image/png')
  const dataURL = await upscaleNearestNeighbor(rawDataURL, 8)
  modifiedImages.value[img.relPath] = {
    dataURI: dataURL,
    filename: img.filename,
    hue: hue.value,
    sat: sat.value,
    bright: bright.value
  }
  const idx = images.value.findIndex(i => i.relPath === img.relPath)
  if (idx !== -1) {
    images.value[idx] = { ...images.value[idx], dataURI: dataURL }
  }
  syncAllImages(img.relPath, dataURL)
}

function syncAllImages(relPath, dataURL) {
  const idx = allImages.value.findIndex(i => i.relPath === relPath)
  if (idx !== -1) {
    allImages.value[idx] = { ...allImages.value[idx], dataURI: dataURL }
  }
}

function restoreAllImages(relPath) {
  const orig = originalDataURIs.value[relPath]
  if (!orig) return
  const idx = allImages.value.findIndex(i => i.relPath === relPath)
  if (idx !== -1) {
    allImages.value[idx] = { ...allImages.value[idx], dataURI: orig }
  }
}

function closeEditor() {
  selectedImage.value = null
  hue.value = 0
  sat.value = 0
  bright.value = 0
  originalImage.value = null
}

function drawPreview() {
  if (!selectedImage.value || !previewCanvas.value) return
  const canvas = previewCanvas.value
  const ctx = canvas.getContext('2d')
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => {
    canvas.width = img.naturalWidth
    canvas.height = img.naturalHeight
    ctx.clearRect(0, 0, canvas.width, canvas.height)
    ctx.drawImage(img, 0, 0)

    if (hue.value !== 0 || sat.value !== 0 || bright.value !== 0) {
      const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height)
      const shifted = applyHsvShift(imageData, hue.value, sat.value, bright.value)
      ctx.putImageData(shifted, 0, 0)
    }

    if (!originalImage.value) {
      originalImage.value = selectedImage.value.dataURI
    }
  }
  img.src = selectedImage.value.dataURI
}

watch([hue, sat, bright], () => {
  drawPreview()
})

async function applyChanges() {
  if (!previewCanvas.value || !selectedImage.value) return
  const rawDataURL = previewCanvas.value.toDataURL('image/png')
  const dataURL = await upscaleNearestNeighbor(rawDataURL, 8)
  const relPath = selectedImage.value.relPath

  modifiedImages.value[relPath] = {
    dataURI: dataURL,
    filename: selectedImage.value.filename,
    hue: hue.value,
    sat: sat.value,
    bright: bright.value
  }

  const idx = images.value.findIndex(i => i.relPath === relPath)
  if (idx !== -1) {
    images.value[idx] = { ...images.value[idx], dataURI: dataURL }
  }
  syncAllImages(relPath, dataURL)

  closeEditor()
}

function resetSliders() {
  hue.value = 0
  sat.value = 0
  bright.value = 0
}

function undoImage(relPath) {
  if (modifiedImages.value[relPath]) {
    delete modifiedImages.value[relPath]
    const orig = originalDataURIs.value[relPath]
    if (orig) {
      const idx = images.value.findIndex(i => i.relPath === relPath)
      if (idx !== -1) {
        images.value[idx] = { ...images.value[idx], dataURI: orig }
      }
      restoreAllImages(relPath)
    }
  }
}

function undoAll() {
  for (const relPath of Object.keys(modifiedImages.value)) {
    const orig = originalDataURIs.value[relPath]
    if (orig) {
      const idx = images.value.findIndex(i => i.relPath === relPath)
      if (idx !== -1) {
        images.value[idx] = { ...images.value[idx], dataURI: orig }
      }
      restoreAllImages(relPath)
    }
  }
  modifiedImages.value = {}
}

function isModified(relPath) {
  return relPath in modifiedImages.value
}

async function exportPack() {
  saving.value = true
  try {
    const entries = Object.entries(modifiedImages.value)
    for (const [relPath, mod] of entries) {
      const base64 = mod.dataURI.split(',')[1]
      await SaveImage({
        imageName: mod.filename,
        imagePath: '',
        relPath: relPath,
        imageData: base64,
        done: false
      })
    }

    const result = await ExportPack(props.packName || '')
    await DeleteTemp()
    popup('Exported!', result, 'success')
    emit('close')
  } catch (err) {
    popup('Error', 'Export failed: ' + err.toString(), 'error')
  }
  saving.value = false
}

async function handleClose() {
  try { await DeleteTemp() } catch (_) {}
  emit('close')
}

onMounted(() => {
  if (folders.value.length > 0) {
    loadFolder(folders.value[0])
  }
})
</script>

<template>
  <div class="display-overlay" :style="{ left: sidebarWidth + 'px' }">
    <div class="display-topbar">
      <button class="back-btn" @click="handleClose">
        <i class="fa fa-arrow-left"></i> Back
      </button>
      <div class="display-title">{{ packName || 'Recolor Tool' }}</div>
      <div class="topbar-right">
        <button v-if="Object.keys(modifiedImages).length > 0" class="undo-all-btn" @click="undoAll" title="Undo all changes">
          <i class="fa fa-rotate-left"></i>
          Undo All
        </button>
        <button class="export-btn" @click="exportPack" :disabled="saving || Object.keys(modifiedImages).length === 0">
          <i class="fa fa-download"></i>
          {{ saving ? 'Exporting...' : 'Export Pack' }}
          <span v-if="Object.keys(modifiedImages).length > 0" class="mod-badge">{{ Object.keys(modifiedImages).length }}</span>
        </button>
      </div>
    </div>

    <div class="paint-bar">
      <button class="paint-toggle" :class="{ active: paintMode }" @click="paintMode = !paintMode" title="Toggle Paint Mode: pick a color then click textures to apply it">
        <i class="fa fa-paintbrush"></i>
        Paint Mode
      </button>
      <div class="search-bar">
        <i class="fa fa-search search-icon"></i>
        <input type="text" v-model="searchQuery" placeholder="Search all textures..." class="search-input" />
        <button v-if="searchQuery" class="search-clear" @click="searchQuery = ''">
          <i class="fa fa-xmark"></i>
        </button>
      </div>
    </div>

    <div class="toolbar-sliders" v-if="paintMode">
      <div class="slider-row">
        <div class="slider-group">
          <label>Hue <span class="slider-val">{{ hue }}°</span></label>
          <input type="range" min="-180" max="180" step="1" v-model.number="hue" class="slider hue-slider" />
        </div>
        <div class="slider-group">
          <label>Saturation <span class="slider-val">{{ sat > 0 ? '+' : '' }}{{ sat }}%</span></label>
          <input type="range" min="-100" max="100" step="1" v-model.number="sat" class="slider sat-slider" />
        </div>
        <div class="slider-group">
          <label>Brightness <span class="slider-val">{{ bright > 0 ? '+' : '' }}{{ bright }}%</span></label>
          <input type="range" min="-100" max="100" step="1" v-model.number="bright" class="slider bright-slider" />
        </div>
        <button class="reset-paint-btn" @click="resetSliders" title="Reset sliders">
          <i class="fa fa-rotate-left"></i>
        </button>
      </div>
    </div>

    <div class="folder-tabs" v-if="folders.length > 0">
      <button
        v-for="folder in folders" :key="folder"
        :class="['folder-tab', selectedFolder === folder ? 'active' : '']"
        @click="loadFolder(folder)">
        {{ folder.replace('textures/', '') }}
      </button>
    </div>

    <div class="display-content">
      <div v-if="loading" class="loading-text">
        <div class="fp-spinner"></div>
        Loading images...
      </div>

      <div v-else-if="images.length === 0 && !loading" class="empty-text">
        No images found in this folder.
      </div>

      <div v-else-if="filteredImages.length === 0 && searchQuery" class="empty-text">
        No textures match "{{ searchQuery }}".
      </div>

      <div v-else class="image-grid">
        <div
          v-for="img in filteredImages" :key="img.relPath"
          :class="['image-card', selectedImage?.relPath === img.relPath ? 'selected' : '', isModified(img.relPath) ? 'modified' : '']"
          @click="selectImage(img)"
          @mouseenter="hoveredImage = img.relPath"
          @mouseleave="hoveredImage = null">
          <div class="image-thumb">
            <img :src="img.dataURI" :alt="img.filename" :style="paintMode && hoveredImage === img.relPath && (hue !== 0 || sat !== 0 || bright !== 0) ? { filter: `hue-rotate(${hue}deg) saturate(${100 + sat}%) brightness(${100 + bright}%)` } : {}" />
          </div>
          <div class="image-info">
            <span class="image-name">{{ img.filename }}</span>
            <span v-if="searchQuery" class="image-folder">{{ img.folder ? img.folder.replace('textures/', '') : '' }}</span>
            <span class="image-size">{{ (img.size / 1024).toFixed(1) }}KB</span>
          </div>
          <div v-if="isModified(img.relPath)" class="modified-dot" @click.stop="undoImage(img.relPath)" title="Click to undo changes"></div>
        </div>
      </div>
    </div>

    <div v-if="selectedImage && !paintMode" class="editor-panel">
      <div class="editor-header">
        <span class="editor-filename">{{ selectedImage.filename }}</span>
        <div class="editor-actions">
          <button class="editor-btn reset-btn" @click="resetSliders">Reset</button>
          <button class="editor-btn apply-btn" @click="applyChanges">Apply</button>
          <button class="editor-btn close-btn" @click="closeEditor">Cancel</button>
        </div>
      </div>

      <div class="editor-body">
        <div class="editor-preview">
          <div class="preview-label">Preview</div>
          <div class="preview-images">
            <div class="preview-box">
              <div class="preview-sublabel">Original</div>
              <img :src="originalImage || selectedImage.dataURI" class="preview-img" />
            </div>
            <div class="preview-arrow"><i class="fa fa-arrow-right"></i></div>
            <div class="preview-box">
              <div class="preview-sublabel">Recolored</div>
              <canvas ref="previewCanvas" class="preview-canvas"></canvas>
            </div>
          </div>
        </div>

        <div class="editor-controls">
          <div class="slider-group">
            <label>Hue <span class="slider-val">{{ hue }}°</span></label>
            <input type="range" min="-180" max="180" step="1" v-model.number="hue" class="slider hue-slider" />
          </div>
          <div class="slider-group">
            <label>Saturation <span class="slider-val">{{ sat > 0 ? '+' : '' }}{{ sat }}%</span></label>
            <input type="range" min="-100" max="100" step="1" v-model.number="sat" class="slider sat-slider" />
          </div>
          <div class="slider-group">
            <label>Brightness <span class="slider-val">{{ bright > 0 ? '+' : '' }}{{ bright }}%</span></label>
            <input type="range" min="-100" max="100" step="1" v-model.number="bright" class="slider bright-slider" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.display-overlay {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  background: hsl(0, 0%, 2%);
  z-index: 20;
  display: flex;
  flex-direction: column;
}

.display-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid hsl(0, 0%, 10%);
  background: hsl(0, 0%, 3%);
  flex-shrink: 0;
}

.back-btn {
  background: none;
  border: 1px solid hsl(0, 0%, 20%);
  color: hsl(0, 0%, 60%);
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.back-btn:hover {
  background: hsl(0, 0%, 10%);
}

.display-title {
  font-weight: 600;
  color: #e879a8;
  max-width: 300px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.undo-all-btn {
  padding: 0.4rem 0.75rem;
  background: hsl(0, 0%, 10%);
  color: hsl(0, 0%, 55%);
  border: 1px solid hsl(0, 0%, 18%);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.8rem;
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.undo-all-btn:hover {
  background: hsl(0, 0%, 14%);
  color: hsl(0, 0%, 75%);
}

.export-btn {
  padding: 0.4rem 1rem;
  background: transparent;
  color: #e879a8;
  border: 1px solid #e879a8;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  position: relative;
}

.export-btn:hover:not(:disabled) {
  background: hsla(330, 60%, 65%, 0.1);
}

.export-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.mod-badge {
  background: hsl(330, 60%, 40%);
  color: white;
  font-size: 0.65rem;
  padding: 0.1rem 0.35rem;
  border-radius: 8px;
  font-weight: 700;
}

.paint-bar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.5rem 1rem;
  border-bottom: 1px solid hsl(0, 0%, 8%);
  background: hsl(0, 0%, 3%);
  flex-shrink: 0;
}

.paint-toggle {
  padding: 0.35rem 0.75rem;
  background: hsl(0, 0%, 10%);
  color: hsl(0, 0%, 55%);
  border: 1px solid hsl(0, 0%, 18%);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
  display: flex;
  align-items: center;
  gap: 0.35rem;
  transition: all 0.15s;
  flex-shrink: 0;
}

.paint-toggle:hover {
  background: hsl(0, 0%, 14%);
  color: hsl(0, 0%, 75%);
}

.paint-toggle.active {
  background: hsl(330, 60%, 25%);
  color: #e879a8;
  border-color: hsl(330, 60%, 40%);
}

.search-bar {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex: 1;
}

.search-icon {
  color: hsl(0, 0%, 30%);
  font-size: 0.8rem;
  flex-shrink: 0;
}

.search-input {
  flex: 1;
  background: hsl(0, 0%, 8%);
  border: 1px solid hsl(0, 0%, 14%);
  border-radius: 4px;
  padding: 0.35rem 0.6rem;
  color: hsl(0, 0%, 75%);
  font-size: 0.8rem;
  outline: none;
  min-width: 0;
}

.search-input::placeholder {
  color: hsl(0, 0%, 30%);
}

.search-input:focus {
  border-color: hsl(330, 60%, 35%);
}

.search-clear {
  background: none;
  border: none;
  color: hsl(0, 0%, 35%);
  cursor: pointer;
  font-size: 0.85rem;
  padding: 0.2rem;
  flex-shrink: 0;
}

.search-clear:hover {
  color: hsl(0, 0%, 65%);
}

.toolbar-sliders {
  padding: 0.5rem 1rem;
  border-bottom: 1px solid hsl(0, 0%, 8%);
  background: hsl(0, 0%, 3%);
  flex-shrink: 0;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 1rem;
}

.slider-row .slider-group {
  flex: 1;
}

.reset-paint-btn {
  width: 32px;
  height: 32px;
  border-radius: 4px;
  border: 1px solid hsl(0, 0%, 18%);
  background: hsl(0, 0%, 10%);
  color: hsl(0, 0%, 55%);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8rem;
  flex-shrink: 0;
}

.reset-paint-btn:hover {
  background: hsl(0, 0%, 15%);
  color: hsl(0, 0%, 80%);
}

.folder-tabs {
  display: flex;
  gap: 0.25rem;
  padding: 0.5rem 1rem;
  border-bottom: 1px solid hsl(0, 0%, 8%);
  overflow-x: auto;
  flex-shrink: 0;
  background: hsl(0, 0%, 3%);
}

.folder-tab {
  padding: 0.3rem 0.75rem;
  background: hsl(0, 0%, 8%);
  color: hsl(0, 0%, 50%);
  border: 1px solid hsl(0, 0%, 12%);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
  white-space: nowrap;
  transition: all 0.15s;
}

.folder-tab:hover {
  background: hsl(0, 0%, 12%);
  color: hsl(0, 0%, 70%);
}

.folder-tab.active {
  background: hsl(330, 60%, 20%);
  color: #e879a8;
  border-color: hsl(330, 60%, 35%);
}

.display-content {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
  min-height: 0;
}

.loading-text, .empty-text {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: hsl(0, 0%, 40%);
  gap: 0.75rem;
}

.fp-spinner {
  width: 24px;
  height: 24px;
  border: 3px solid hsl(0, 0%, 20%);
  border-top-color: #e879a8;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.image-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(120px, 1fr));
  gap: 0.75rem;
}

.image-card {
  border: 1px solid hsl(0, 0%, 12%);
  background: hsl(0, 0%, 4%);
  border-radius: 6px;
  overflow: hidden;
  cursor: pointer;
  transition: all 0.15s;
  position: relative;
}

.image-card:hover {
  border-color: hsl(0, 0%, 25%);
  background: hsl(0, 0%, 6%);
}

.image-card.selected {
  border-color: #e879a8;
  box-shadow: 0 0 0 1px #e879a8;
}

.image-card.modified {
  border-color: hsl(330, 40%, 30%);
}

.image-thumb {
  width: 100%;
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background: hsl(0, 0%, 6%);
  overflow: hidden;
}

.image-thumb img {
  max-width: 90%;
  max-height: 90%;
  image-rendering: pixelated;
  object-fit: contain;
}

.image-info {
  padding: 0.35rem 0.5rem;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.1rem 0.3rem;
}

.image-name {
  font-size: 0.65rem;
  color: hsl(0, 0%, 60%);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 70px;
}

.image-folder {
  font-size: 0.65rem;
  color: hsl(330, 50%, 60%);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.image-size {
  font-size: 0.6rem;
  color: hsl(0, 0%, 35%);
}

.modified-dot {
  position: absolute;
  top: 4px;
  right: 4px;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #e879a8;
  cursor: pointer;
}

.modified-dot:hover {
  background: #ff4488;
}

.editor-panel {
  border-top: 1px solid hsl(0, 0%, 12%);
  background: hsl(0, 0%, 3%);
  flex-shrink: 0;
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 1rem;
  border-bottom: 1px solid hsl(0, 0%, 8%);
}

.editor-filename {
  font-size: 0.8rem;
  color: hsl(0, 0%, 60%);
}

.editor-actions {
  display: flex;
  gap: 0.4rem;
}

.editor-btn {
  padding: 0.3rem 0.75rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
  border: none;
}

.reset-btn {
  background: hsl(0, 0%, 12%);
  color: hsl(0, 0%, 60%);
  border: 1px solid hsl(0, 0%, 20%);
}

.reset-btn:hover { background: hsl(0, 0%, 16%); }

.apply-btn {
  background: transparent;
  color: #e879a8;
  border: 1px solid #e879a8;
}

.apply-btn:hover { background: hsla(330, 60%, 65%, 0.1); }

.close-btn {
  background: hsl(0, 0%, 8%);
  color: hsl(0, 0%, 50%);
  border: 1px solid hsl(0, 0%, 15%);
}

.close-btn:hover { background: hsl(0, 0%, 12%); }

.editor-body {
  display: flex;
  gap: 1.5rem;
  padding: 0.75rem 1rem;
  align-items: flex-start;
}

.editor-preview {
  flex-shrink: 0;
}

.preview-label {
  font-size: 0.7rem;
  color: hsl(0, 0%, 40%);
  margin-bottom: 0.4rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.preview-images {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.preview-box {
  width: 120px;
  height: 120px;
  border: 1px solid hsl(0, 0%, 12%);
  border-radius: 4px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  background: hsl(0, 0%, 5%);
}

.preview-sublabel {
  font-size: 0.6rem;
  color: hsl(0, 0%, 35%);
  text-align: center;
  padding: 0.15rem 0;
  background: hsl(0, 0%, 3%);
  border-bottom: 1px solid hsl(0, 0%, 10%);
}

.preview-img, .preview-canvas {
  flex: 1;
  width: 100%;
  object-fit: contain;
  image-rendering: pixelated;
  background: repeating-conic-gradient(hsl(0, 0%, 15%) 0% 25%, hsl(0, 0%, 10%) 0% 50%) 50% / 16px 16px;
}

.preview-arrow {
  color: hsl(0, 0%, 30%);
  font-size: 0.875rem;
}

.editor-controls {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.slider-group {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.slider-group label {
  font-size: 0.75rem;
  color: hsl(0, 0%, 55%);
  display: flex;
  justify-content: space-between;
}

.slider-val {
  color: hsl(0, 0%, 75%);
  font-variant-numeric: tabular-nums;
}

.slider {
  -webkit-appearance: none;
  appearance: none;
  width: 100%;
  height: 6px;
  border-radius: 3px;
  outline: none;
  cursor: pointer;
}

.slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  appearance: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #e879a8;
  border: 2px solid hsl(0, 0%, 100%);
  cursor: pointer;
}

.hue-slider {
  background: linear-gradient(to right, #ff0000, #ffff00, #00ff00, #00ffff, #0000ff, #ff00ff, #ff0000);
}

.sat-slider {
  background: linear-gradient(to right, hsl(0, 0%, 50%), hsl(0, 100%, 50%));
}

.bright-slider {
  background: linear-gradient(to right, #000000, #ffffff);
}
</style>
