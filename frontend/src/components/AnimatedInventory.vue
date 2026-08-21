<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { CreateAnimatedInventory, GetInstalledPacksDetailed, SetResourcePacksPath, SelectDirectory } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { progressStore, startPort, updateFromEvent, finish, clearProgress } from '../utils/progressStore'
import { parseBedrockCodes } from '../utils/formatCodes'

const props = defineProps({ active: Boolean })

const gifFile = ref(null)
const gifName = ref('')
const gifSize = ref('')
const showGifInfo = ref(false)
const isStaticImage = ref(false)

const overlayFile = ref(null)
const overlayName = ref('')
const showOverlayInfo = ref(false)

const includeOverlay = ref(true)
const frameDuration = ref('0.06')
const durationOptions = ['0.05', '0.06', '0.07', '0.08', '0.09']
const transparentFill = ref(true)
const fillColor = ref('#000000')

const selectedPacks = ref([])
const draftPacks = ref([])
const showDrawer = ref(false)
const installInfo = ref({ path: '', found: false, packs: [] })
const pickingDir = ref(false)

const showProgress = ref(false)
const progressStatus = ref('Starting...')
const progressWidth = ref(0)
const progressPercent = ref('0%')
const spinnerClass = ref('ai-spinner')
const progressDone = ref(false)
const startedPort = ref(false)

const gifDropZone = ref(null)
const overlayDropZone = ref(null)

function formatSize(size) {
  return (size / 1024 / 1024).toFixed(2) + ' MB'
}

function handleGif(selectedFile) {
  if (!selectedFile) return
  const name = selectedFile.name.toLowerCase()
  const isImage = /\.(png|jpe?g)$/i.test(name)
  if (!/\.gif$/i.test(name) && !isImage) {
    popup('Error', 'Must be a .gif, .png, .jpg, or .jpeg file.', 'error')
    return
  }
  gifFile.value = selectedFile
  gifName.value = selectedFile.name
  gifSize.value = formatSize(selectedFile.size)
  isStaticImage.value = isImage
  showGifInfo.value = true
}

function handleOverlay(selectedFile) {
  if (!selectedFile) return
  if (!/\.png$/i.test(selectedFile.name)) {
    popup('Error', 'Must be a .png file.', 'error')
    return
  }
  overlayFile.value = selectedFile
  overlayName.value = selectedFile.name
  showOverlayInfo.value = true
}

function onGifDrop(e) {
  e.preventDefault()
  gifDropZone.value?.classList.remove('drag-over')
  handleGif(e.dataTransfer.files[0])
}

function onGifDragOver(e) {
  e.preventDefault()
  gifDropZone.value?.classList.add('drag-over')
}

function onGifDragLeave() {
  gifDropZone.value?.classList.remove('drag-over')
}

function onGifChange(e) {
  handleGif(e.target.files[0])
}

function onOverlayDrop(e) {
  e.preventDefault()
  overlayDropZone.value?.classList.remove('drag-over')
  handleOverlay(e.dataTransfer.files[0])
}

function onOverlayDragOver(e) {
  e.preventDefault()
  overlayDropZone.value?.classList.add('drag-over')
}

function onOverlayDragLeave() {
  overlayDropZone.value?.classList.remove('drag-over')
}

function onOverlayChange(e) {
  handleOverlay(e.target.files[0])
}

function cancelGif() {
  gifFile.value = null
  showGifInfo.value = false
  isStaticImage.value = false
}

function cancelOverlay() {
  overlayFile.value = null
  showOverlayInfo.value = false
}

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

async function loadInstalledPacks() {
  try {
    installInfo.value = await GetInstalledPacksDetailed()
  } catch (err) {
    console.error('[AnimatedInventory] GetInstalledPacksDetailed error:', err)
    installInfo.value = { path: '', found: false, packs: [] }
  }
}

async function openInstallMenu() {
  draftPacks.value = [...selectedPacks.value]
  showDrawer.value = true
  await loadInstalledPacks()
}

function closeInstallMenu() {
  showDrawer.value = false
}

function togglePack(name) {
  const idx = draftPacks.value.indexOf(name)
  if (idx === -1) draftPacks.value.push(name)
  else draftPacks.value.splice(idx, 1)
}

function confirmMerge() {
  selectedPacks.value = [...draftPacks.value]
  showDrawer.value = false
}

async function chooseResourcePacksDir() {
  if (pickingDir.value) return
  pickingDir.value = true
  try {
    const dir = await SelectDirectory()
    if (dir) {
      try {
        await SetResourcePacksPath(dir)
        await loadInstalledPacks()
      } catch (err) {
        popup('Error', err.toString(), 'error')
      }
    }
  } catch (err) {
    console.error('[AnimatedInventory] SelectDirectory error:', err)
  } finally {
    pickingDir.value = false
  }
}

function resetProgress() {
  progressStatus.value = 'Starting...'
  progressWidth.value = 0
  progressPercent.value = '0%'
  spinnerClass.value = 'ai-spinner'
  progressDone.value = false
}

let progressHandler = null
const isMounted = ref(false)

onMounted(() => {
  isMounted.value = true
  progressHandler = EventsOn('progress', (data) => {
    if (!isMounted.value || !startedPort.value) return
    const t = parseInt(data.total) || 0
    const c = parseInt(data.completed) || 0

    if (data.icon === 'success' || data.icon === 'error') {
      finish(data)
      startedPort.value = false
    } else {
      updateFromEvent(data)
    }

    if (!props.active) return

    if (data.icon === 'success') {
      spinnerClass.value = 'ai-spinner done'
      progressWidth.value = 100
      progressPercent.value = '100%'
      progressStatus.value = (data.title || 'Done') + ': ' + (data.message || '')
      progressDone.value = true
      if (selectedPacks.value.length) loadInstalledPacks()
      setTimeout(() => {
        showProgress.value = false
        gifFile.value = null
        showGifInfo.value = false
        overlayFile.value = null
        showOverlayInfo.value = false
      }, 1500)
    } else if (data.icon === 'error') {
      spinnerClass.value = 'ai-spinner fail'
      progressStatus.value = 'Error: ' + (data.message || 'Unknown error')
      setTimeout(() => {
        showProgress.value = false
      }, 1500)
    } else if (data.icon === 'info') {
      progressStatus.value = (data.title || '') + ': ' + (data.message || '')
      if (t > 0) {
        const pct = Math.round((c / t) * 100)
        progressWidth.value = pct
        progressPercent.value = pct + '%'
      }
    }
  })
})

onUnmounted(() => {
  isMounted.value = false
  if (progressHandler) progressHandler()
})

watch(() => props.active, (active) => {
  if (!active) return
  if (startedPort.value && progressStore.active && progressStore.source === 'animator') {
    showProgress.value = true
    progressStatus.value = (progressStore.title || '') + (progressStore.message ? ': ' + progressStore.message : '')
    progressWidth.value = progressStore.percent
    progressPercent.value = progressStore.percent + '%'
    spinnerClass.value = 'ai-spinner'
    progressDone.value = false
  } else {
    showProgress.value = false
    resetProgress()
  }
})

async function createPack() {
  if (!gifFile.value) {
    popup('Error', 'Please choose a .gif file first.', 'error')
    return
  }
  resetProgress()
  showProgress.value = true
  startedPort.value = true
  startPort('animator', 'Starting...')

  try {
    const gifBuffer = await gifFile.value.arrayBuffer()
    const gifBytes = Array.from(new Uint8Array(gifBuffer))

    let overlayBytes = []
    if (includeOverlay.value && overlayFile.value) {
      const overlayBuffer = await overlayFile.value.arrayBuffer()
      overlayBytes = Array.from(new Uint8Array(overlayBuffer))
    }

    await CreateAnimatedInventory(
      gifBytes,
      gifFile.value.name,
      frameDuration.value,
      overlayBytes,
      includeOverlay.value,
      transparentFill.value,
      selectedPacks.value,
      fillColor.value
    )
  } catch (err) {
    console.error('[AnimatedInventory] CreateAnimatedInventory error:', err)
    popup('Error', err.toString(), 'error')
    startedPort.value = false
    clearProgress()
    showProgress.value = false
  }
}
</script>

<template>
  <div class="page active-page animator-page">
    <div class="porter-card">
      <h2 class="card-title">Animated Inventory</h2>
      <p class="card-desc">Turn a .gif or image into an animated inventory for Minecraft Bedrock.</p>

      <div v-if="!showProgress">
        <label ref="gifDropZone" class="drop-zone"
               @drop="onGifDrop" @dragover="onGifDragOver" @dragleave="onGifDragLeave">
          <input ref="gifUpload" type="file" accept=".gif,.png,.jpg,.jpeg" class="hidden" @change="onGifChange" />

          <div v-if="!showGifInfo" class="drop-zone-inner">
            <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
                 stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
              <polyline points="17 8 12 3 7 8"></polyline>
              <line x1="12" x2="12" y1="3" y2="15"></line>
            </svg>
            <span class="drop-hint">Drop a .gif or image here</span>
          </div>

          <div v-else class="file-info-box">
            <div class="ai-row">
              <p class="ai-name">{{ gifName }}</p>
              <p class="ai-size">{{ gifSize }}</p>
            </div>
          </div>
        </label>

        <div class="ai-fields">
          <div class="ai-field">
            <label class="ai-checkbox">
              <input type="checkbox" v-model="includeOverlay" />
              <span>Include overlay PNG</span>
            </label>
            <label ref="overlayDropZone" class="drop-zone overlay-zone"
                   :class="{ disabled: !includeOverlay }"
                   @drop="onOverlayDrop" @dragover="onOverlayDragOver" @dragleave="onOverlayDragLeave">
              <input ref="overlayUpload" type="file" accept=".png" class="hidden" @change="onOverlayChange" />
              <div v-if="!showOverlayInfo" class="drop-zone-inner">
                <span class="drop-hint">Drop a .png overlay</span>
              </div>
              <div v-else class="file-info-box">
                <div class="ai-row">
                  <p class="ai-name">{{ overlayName }}</p>
                </div>
              </div>
            </label>
          </div>

          <div class="ai-field-row">
            <div v-if="!isStaticImage" class="ai-field">
              <label class="ai-label">Frame Duration</label>
              <select v-model="frameDuration" class="ai-select">
                <option v-for="opt in durationOptions" :key="opt" :value="opt">{{ opt }}s</option>
              </select>
            </div>

            <div class="ai-field">
              <label class="ai-label">Empty Area Fill</label>
              <div class="mode-tabs">
                <button :class="['mode-tab', transparentFill ? 'active' : '']" @click="transparentFill = true">
                  Transparent
                </button>
                <button :class="['mode-tab', !transparentFill ? 'active' : '']" @click="transparentFill = false">
                  Solid
                </button>
              </div>
              <div v-if="!transparentFill" class="ai-color-row">
                <input type="color" v-model="fillColor" class="ai-color" />
                <span class="ai-color-hex">{{ fillColor }}</span>
              </div>
            </div>
          </div>
        </div>

        <p v-if="!isStaticImage" class="card-desc mt-4">MCPE UI can only handle up to 40 frames max. Frame duration can be 0.05–0.09, any faster could cause crashes.</p>

        <div class="ai-actions">
          <button class="btn-cancel" @click="cancelGif">Clear</button>
          <button class="btn-main" :class="{ installed: selectedPacks.length }" @click="openInstallMenu">
            {{ selectedPacks.length ? 'Merge into ' + selectedPacks.length + ' pack' + (selectedPacks.length > 1 ? 's' : '') + ' ✓' : 'Merge into Pack' }}
          </button>
          <button class="btn-main" @click="createPack">Create Pack</button>
        </div>
        <p v-if="selectedPacks.length" class="ai-install-note">Will install into {{ selectedPacks.length }} selected pack(s) after creating.</p>
      </div>

      <div v-else class="progress-section">
        <div class="ai-progress-head">
          <div :class="spinnerClass"></div>
          <div class="ai-progress-status">{{ progressStatus }}</div>
        </div>

        <div class="progress-track">
          <div class="progress-bar" :style="{ width: progressWidth + '%' }"></div>
        </div>

        <div class="ai-progress-foot">
          <span>{{ progressPercent }}</span>
          <span v-if="progressDone" class="ai-complete">Complete</span>
        </div>

        <div class="ai-actions">
          <button class="btn-cancel" @click="showProgress = false">Close</button>
        </div>
      </div>
    </div>

    <div v-if="showDrawer" class="ai-drawer-overlay" @click.self="closeInstallMenu">
      <div class="ai-drawer">
        <div class="ai-drawer-head">
          <h3 class="ai-drawer-title">Merge into Pack</h3>
          <button class="ai-drawer-close" @click="closeInstallMenu">&times;</button>
        </div>

        <div v-if="!installInfo.found" class="ai-drawer-body">
          <p class="ai-modal-warn">Couldn't find Minecraft's resource packs folder.</p>
          <p class="ai-modal-path">{{ installInfo.path || 'No path detected' }}</p>
          <div class="ai-actions">
            <button class="btn-main" :disabled="pickingDir" @click="chooseResourcePacksDir">
              {{ pickingDir ? 'Opening...' : 'Choose Folder' }}
            </button>
          </div>
        </div>

        <div v-else class="ai-drawer-body">
          <p class="ai-drawer-path">{{ installInfo.path }}</p>
          <div class="ai-drawer-header">
            <span class="ai-drawer-count">{{ draftPacks.length }} selected</span>
            <button v-if="draftPacks.length" class="ai-unselect-all" @click="draftPacks = []">Unselect All</button>
          </div>
          <div v-if="installInfo.packs.length > 0" class="ai-pack-grid">
            <div
              v-for="p in installInfo.packs"
              :key="p.name"
              class="ai-pack-card"
              :class="{ selected: draftPacks.includes(p.name) }"
              @click="togglePack(p.name)"
            >
              <div class="ai-pack-card-icon">
                <img v-if="p.icon" :src="p.icon" alt="" />
                <div v-else class="ai-pack-card-placeholder"></div>
                <div v-if="draftPacks.includes(p.name)" class="ai-pack-card-check">&#10003;</div>
              </div>
              <div class="ai-pack-card-info">
                <div class="ai-pack-card-name" v-html="parseBedrockCodes(p.name)"></div>
                <div v-if="p.description" class="ai-pack-card-desc" v-html="parseBedrockCodes(p.description)"></div>
              </div>
            </div>
          </div>
          <p v-else class="ai-drawer-empty">No packs found.</p>
        </div>

        <div class="ai-drawer-foot">
          <button class="btn-cancel" @click="closeInstallMenu">Cancel</button>
          <button class="btn-main" @click="confirmMerge">Confirm</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.animator-page {
  justify-content: safe center;
  align-items: center;
}

.porter-card {
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  padding: 1.5rem;
  width: 100%;
  max-width: 520px;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: var(--accent);
}

.card-desc {
  font-size: 0.875rem;
  color: var(--text-desc);
  margin-bottom: 1rem;
}

.mt-4 {
  margin-top: 1rem;
}

.drop-zone {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px dashed var(--border-strong);
  border-radius: 8px;
  padding: 2rem;
  cursor: pointer;
  transition: all 0.2s;
  min-height: 100px;
}

.drop-zone:hover, .drop-zone.drag-over {
  border-color: var(--accent);
  background: var(--accent-glow);
}

.drop-zone.disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.overlay-zone {
  min-height: 60px;
  padding: 1rem;
}

.drop-zone-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.drop-hint {
  font-size: 0.8rem;
  color: var(--text-dim);
}

.file-info-box {
  width: 100%;
}

.ai-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  width: 100%;
}

.ai-name {
  color: var(--text-secondary);
  font-size: 0.875rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ai-size {
  flex-shrink: 0;
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 0.875rem;
  padding: 0.25rem 0.5rem;
}

.ai-fields {
  margin-top: 1rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.ai-field-row {
  display: flex;
  gap: 1rem;
  align-items: flex-start;
}

.ai-field {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.ai-label {
  font-size: 0.8rem;
  color: var(--text-desc);
}

.ai-unselect-all {
  background: none;
  border: none;
  color: #aaa;
  cursor: pointer;
  font-size: 0.8rem;
  padding: 0;
  text-decoration: underline;
}

.ai-unselect-all:hover {
  color: #ddd;
}

.ai-checkbox {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.875rem;
  color: var(--text-secondary);
  cursor: pointer;
  margin-bottom: 0.5rem;
}

.ai-checkbox input {
  accent-color: var(--accent);
}

.ai-select {
  width: 100%;
  background: var(--bg-input);
  border: 1px solid var(--border-focus);
  color: var(--text-secondary);
  font-size: 0.875rem;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  outline: none;
}

.ai-select:focus {
  border-color: var(--accent);
}

.mode-tabs {
  display: flex;
  gap: 0.25rem;
  background: var(--bg-input);
  border-radius: 6px;
  padding: 3px;
}

.mode-tab {
  flex: 1;
  padding: 0.4rem 0.5rem;
  background: transparent;
  color: var(--text-dim);
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
}

.mode-tab.active {
  background: var(--bg-hover-2);
  color: var(--accent);
}

.ai-color-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.ai-color {
  width: 36px;
  height: 28px;
  padding: 0;
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  background: var(--bg-input);
  cursor: pointer;
}

.ai-color-hex {
  font-size: 0.8rem;
  color: var(--text-dim);
}

.ai-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1.5rem;
}

.btn-cancel {
  padding: 0.5rem 1rem;
  background: var(--bg-hover-2);
  color: var(--text-muted);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-cancel:hover {
  background: var(--bg-hover-5);
}

.btn-main {
  padding: 0.5rem 1rem;
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-main:hover {
  background: var(--accent-glow);
}

.btn-main.installed {
  background: var(--accent);
  color: var(--bg-body);
}

.btn-main:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ai-install-note {
  margin-top: 0.75rem;
  font-size: 0.8rem;
  color: #22c55e;
}

.ai-drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 50;
  display: flex;
  justify-content: flex-end;
  background: rgba(0, 0, 0, 0.5);
}

.ai-drawer {
  width: 380px;
  max-width: 90vw;
  height: 100%;
  display: flex;
  flex-direction: column;
  background: var(--bg-surface);
  border-left: 1px solid var(--border-default);
  box-shadow: -4px 0 24px rgba(0, 0, 0, 0.4);
  animation: drawerSlideIn 0.2s ease-out;
}

@keyframes drawerSlideIn {
  from { transform: translateX(100%); }
  to { transform: translateX(0); }
}

.ai-drawer-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border-default);
}

.ai-drawer-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--accent);
  margin: 0;
}

.ai-drawer-close {
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 1.5rem;
  cursor: pointer;
  line-height: 1;
  padding: 0 0.25rem;
}

.ai-drawer-close:hover {
  color: var(--text-primary);
}

.ai-drawer-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1rem 1.25rem;
  overflow: hidden;
}

.ai-drawer-path {
  font-size: 0.75rem;
  color: var(--text-dim);
  word-break: break-all;
  background: var(--bg-input);
  padding: 0.35rem 0.5rem;
}

.ai-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.25rem;
}

.ai-drawer-count {
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.ai-drawer-empty {
  font-size: 0.875rem;
  color: var(--text-dim);
  text-align: center;
  padding: 2rem 0;
}

.ai-pack-grid {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding-right: 0.25rem;
}

.ai-pack-grid::-webkit-scrollbar {
  width: 6px;
}

.ai-pack-grid::-webkit-scrollbar-track {
  background: var(--scrollbar-track);
}

.ai-pack-grid::-webkit-scrollbar-thumb {
  background: var(--scrollbar-thumb);
  border-radius: 3px;
}

.ai-pack-card {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0.75rem;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s;
}

.ai-pack-card:hover {
  border-color: var(--border-strong);
  background: var(--bg-hover-1);
}

.ai-pack-card.selected {
  border-color: var(--accent);
  background: var(--accent-glow);
}

.ai-pack-card-icon {
  position: relative;
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  border-radius: 4px;
  overflow: hidden;
  background: var(--bg-hover-2);
}

.ai-pack-card-icon img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.ai-pack-card-placeholder {
  width: 100%;
  height: 100%;
  background: var(--bg-hover-3);
}

.ai-pack-card-check {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
  color: var(--accent);
  font-size: 1.1rem;
  font-weight: 700;
}

.ai-pack-card-info {
  flex: 1;
  min-width: 0;
}

.ai-pack-card-name {
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: pre-line;
}

.ai-pack-card-desc {
  font-size: 0.7rem;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: pre-line;
  margin-top: 0.15rem;
}

.ai-drawer-foot {
  display: flex;
  gap: 0.5rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border-default);
}

.hidden { display: none; }

.progress-section {
  padding-top: 0.5rem;
}

.ai-progress-head {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.ai-progress-status {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.progress-track {
  width: 100%;
  background: var(--bg-hover-2);
  border-radius: 999px;
  height: 8px;
  margin-bottom: 1rem;
  overflow: hidden;
}

.progress-bar {
  height: 100%;
  background: var(--accent);
  border-radius: 999px;
  transition: width 0.3s ease;
}

.ai-progress-foot {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-bottom: 0.25rem;
}

.ai-complete {
  color: #22c55e;
}

.ai-spinner {
  width: 20px;
  height: 20px;
  border: 3px solid var(--border-strong);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.ai-spinner.done {
  border-color: #22c55e;
  border-top-color: #22c55e;
}

.ai-spinner.fail {
  border-color: #ef4444;
  border-top-color: #ef4444;
  animation: none;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
