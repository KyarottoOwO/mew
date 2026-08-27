<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { CreateSkyPack, GetInstalledPacksDetailed, SetResourcePacksPath, SelectDirectory } from '../../../wailsjs/go/main/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

import { progressStore, startPort, updateFromEvent, finish, clearProgress } from '../../utils/progressStore'

import { parseBedrockCodes } from '../../utils/formatCodes'

const props = defineProps({ active: Boolean })

const panoFile = ref(null)
const panoName = ref('')
const panoSize = ref('')
const showPanoInfo = ref(false)
const faceSize = ref('1024')
const faceOptions = ['512', '1024', '2048']
const skyMode = ref('panorama')
const modeOptions = [
  { value: 'panorama', label: 'Make a Sky', desc: 'You take an image and turn it into a minecraft bedrock sky.' },
  { value: 'cross_a', label: 'Port a Sky', desc: 'You take a java sky and turn it into a minecraft bedrock sky.' }
]
function isCross() {
  return skyMode.value.startsWith('cross')
}

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

const panoDropZone = ref(null)

function formatSize(size) {
  return (size / 1024 / 1024).toFixed(2) + ' MB'
}

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

function handlePano(selectedFile) {
  if (!selectedFile) return
  if (!/\.(png|jpe?g)$/i.test(selectedFile.name)) {
    popup('Error', 'Must be a .png, .jpg, or .jpeg file.', 'error')
    return
  }
  panoFile.value = selectedFile
  panoName.value = selectedFile.name
  panoSize.value = formatSize(selectedFile.size)
  showPanoInfo.value = true
}

function onPanoDrop(e) {
  e.preventDefault()
  panoDropZone.value?.classList.remove('drag-over')
  handlePano(e.dataTransfer.files[0])
}

function onPanoDragOver(e) {
  e.preventDefault()
  panoDropZone.value?.classList.add('drag-over')
}

function onPanoDragLeave() {
  panoDropZone.value?.classList.remove('drag-over')
}

function onPanoChange(e) {
  handlePano(e.target.files[0])
}

async function loadInstalledPacks() {
  try {
    installInfo.value = await GetInstalledPacksDetailed()
  } catch (err) {
    console.error('[SkyConverter] GetInstalledPacksDetailed error:', err)
    installInfo.value = { path: '', found: false, packs: [] }
  }
}

async function openInstallMenu() {
  draftPacks.value = [...selectedPacks.value]
  showDrawer.value = true
  await loadInstalledPacks()
}

function closeInstallMenu() { showDrawer.value = false }

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
    console.error('[SkyConverter] SelectDirectory error:', err)
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
        cancelAll()
      }, 1500)
    } else if (data.icon === 'error') {
      spinnerClass.value = 'ai-spinner fail'
      progressStatus.value = 'Error: ' + (data.message || 'Unknown error')
      setTimeout(() => { showProgress.value = false }, 1500)
    } else if (data.icon === 'info') {
      progressStatus.value = (data.title || '') + ': ' + (data.message || '')
      const t = parseInt(data.total) || 0
      const c = parseInt(data.completed) || 0
      if (t > 0) {
        progressWidth.value = Math.round((c / t) * 100)
        progressPercent.value = Math.round((c / t) * 100) + '%'
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
  if (startedPort.value && progressStore.active && progressStore.source === 'skyconverter') {
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

function cancelAll() {
  panoFile.value = null
  showPanoInfo.value = false
}

async function createPack() {
  resetProgress()
  showProgress.value = true
  startedPort.value = true
  startPort('skyconverter', 'Starting...')
  try {
    if (!panoFile.value) {
      popup('Error', 'Add a sky image first', 'error')
      startedPort.value = false
      clearProgress()
      showProgress.value = false
      return
    }
    const buf = await panoFile.value.arrayBuffer()
    const mode = isCross() ? 'cross' : skyMode.value
    await CreateSkyPack(Array.from(new Uint8Array(buf)), panoFile.value.name, parseInt(faceSize.value), selectedPacks.value, mode, null)
  } catch (err) {
    console.error('[SkyConverter] createPack error:', err)
    popup('Error', err.toString(), 'error')
    startedPort.value = false
    clearProgress()
    showProgress.value = false
  }
}
</script>

<template>
  <div class="page active-page skyconv-page">
    <div class="porter-card skyconv-card">
      <h2 class="card-title"><i class="fa fa-cloud-sun" style="margin-right:0.5rem"></i>Sky Converter</h2>

      <p class="card-desc">
        {{ modeOptions.find(m => m.value === skyMode)?.desc || 'Create a Bedrock sky from an image.' }}
      </p>

      <div v-if="!showProgress">
        <div class="ai-fields">
          <div class="ai-field">
            <label class="ai-label">Mode</label>
            <div class="mode-tabs">
              <button v-for="opt in modeOptions" :key="opt.value"
                      :class="['mode-tab', skyMode === opt.value ? 'active' : '']"
                      @click="skyMode = opt.value">{{ opt.label }}</button>
            </div>
          </div>
        </div>
        <label ref="panoDropZone" class="drop-zone sky-drop"
               @drop="onPanoDrop" @dragover="onPanoDragOver" @dragleave="onPanoDragLeave">
          <input type="file" accept=".png,.jpg,.jpeg" class="hidden" @change="onPanoChange" />
          <div v-if="!showPanoInfo" class="drop-zone-inner">
            <i class="fa fa-cloud-arrow-up drop-icon"></i>
            <span class="drop-hint">{{ isCross() ? 'Drop a java sky image here' : 'Drop an image here' }}</span>
            <span class="drop-sub">.png, .jpg, .jpeg</span>
          </div>
          <div v-else class="file-info-box">
            <div class="ai-row">
              <p class="ai-name">{{ panoName }}</p>
              <p class="ai-size">{{ panoSize }}</p>
            </div>
          </div>
        </label>

        <div v-if="skyMode === 'panorama'" class="ai-fields">
          <div class="ai-field">
            <label class="ai-label">Sky Quality</label>
            <div class="mode-tabs">
              <button v-for="opt in faceOptions" :key="opt"
                      :class="['mode-tab', faceSize === opt ? 'active' : '']"
                      @click="faceSize = opt">{{ opt }}px</button>
            </div>
          </div>
        </div>

        <div class="ai-actions">
          <button class="btn-cancel" @click="cancelAll">Clear</button>
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
            <div v-for="p in installInfo.packs" :key="p.name"
                 class="ai-pack-card" :class="{ selected: draftPacks.includes(p.name) }"
                 @click="togglePack(p.name)">
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
.skyconv-page {
  justify-content: safe center;
  align-items: center;
}

.skyconv-card {
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  padding: 1.5rem;
  width: 100%;
  max-width: 580px;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 0.75rem;
  color: var(--accent);
}

.card-desc {
  font-size: 0.875rem;
  color: var(--text-desc);
  margin-bottom: 1rem;
}

.drop-zone {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px dashed var(--border-strong);
  border-radius: 8px;
  padding: 1.5rem;
  cursor: pointer;
  transition: all 0.2s;
  background: var(--bg-surface);
  margin-bottom: 0.75rem;
}

.drop-zone.drag-over {
  border-color: var(--accent);
  background: var(--accent-glow);
}

.drop-zone-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.drop-icon {
  font-size: 1.8rem;
  color: var(--text-dim);
  margin-bottom: 0.25rem;
}

.drop-hint {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.drop-sub {
  font-size: 0.75rem;
  color: var(--text-dim);
}

.file-info-box {
  width: 100%;
}

.ai-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.ai-name {
  font-size: 0.875rem;
  color: var(--text-secondary);
  word-break: break-all;
}

.ai-size {
  font-size: 0.75rem;
  color: var(--text-dim);
  flex-shrink: 0;
  margin-left: 1rem;
}

.ai-fields {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
}

.ai-field {
  display: flex;
  flex-direction: column;
  gap: 0.375rem;
}

.ai-label {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--text-dim);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.mode-tabs {
  display: flex;
  gap: 0.375rem;
}

.mode-tab {
  flex: 1;
  padding: 0.4rem 0.75rem;
  border: 1px solid var(--border-default);
  background: var(--bg-surface);
  color: var(--text-dim);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  transition: all 0.15s;
}

.mode-tab:hover {
  background: var(--bg-hover-1);
  color: var(--text-secondary);
}

.mode-tab.active {
  border-color: var(--accent);
  color: var(--accent);
  background: var(--accent-glow);
}

.ai-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.75rem;
}

.btn-cancel {
  padding: 0.5rem 1rem;
  border: 1px solid var(--border-default);
  background: var(--bg-surface);
  color: var(--text-dim);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
}

.btn-cancel:hover {
  background: var(--bg-hover-1);
  color: var(--text-secondary);
}

.btn-main {
  padding: 0.5rem 1rem;
  border: 1px solid var(--accent);
  background: transparent;
  color: var(--accent);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  font-weight: 500;
  transition: all 0.15s;
}

.btn-main:hover {
  background: var(--accent-glow);
}

.btn-main.installed {
  background: var(--accent-glow2);
}

.ai-install-note {
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-top: 0.5rem;
  text-align: center;
}

.hidden {
  display: none;
}

.progress-section {
  padding: 1rem 0;
}

.ai-progress-head {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 1rem;
}

.ai-progress-status {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.progress-track {
  width: 100%;
  height: 6px;
  background: var(--bg-surface);
  border-radius: 3px;
  overflow: hidden;
  margin-bottom: 0.5rem;
}

.progress-bar {
  height: 100%;
  background: var(--accent);
  border-radius: 3px;
  transition: width 0.3s;
}

.ai-progress-foot {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-bottom: 0.75rem;
}

.ai-complete {
  color: #4ade80;
  font-weight: 500;
}

.ai-spinner {
  width: 24px;
  height: 24px;
  border: 3px solid var(--border-default);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.ai-spinner.done {
  border-color: #4ade80;
  border-top-color: #4ade80;
  animation: none;
}

.ai-spinner.fail {
  border-color: #f87171;
  border-top-color: #f87171;
  animation: none;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.ai-drawer-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: stretch;
  justify-content: flex-end;
  z-index: 100;
}

.ai-drawer {
  width: 380px;
  max-width: 90vw;
  background: var(--bg-body);
  border-left: 1px solid var(--border-default);
  display: flex;
  flex-direction: column;
  box-shadow: -4px 0 20px rgba(0, 0, 0, 0.4);
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
  color: var(--text-primary);
}

.ai-drawer-close {
  background: none;
  border: none;
  color: var(--text-dim);
  font-size: 1.4rem;
  cursor: pointer;
  padding: 0.2rem;
}

.ai-drawer-close:hover {
  color: var(--text-secondary);
}

.ai-drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 1rem 1.25rem;
}

.ai-modal-warn {
  font-size: 0.875rem;
  color: #f87171;
  margin-bottom: 0.5rem;
}

.ai-modal-path {
  font-size: 0.75rem;
  color: var(--text-dim);
  word-break: break-all;
  margin-bottom: 1rem;
}

.ai-drawer-path {
  font-size: 0.75rem;
  color: var(--text-dim);
  word-break: break-all;
  margin-bottom: 0.75rem;
}

.ai-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.75rem;
}

.ai-drawer-count {
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--accent);
}

.ai-unselect-all {
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 0.75rem;
}

.ai-unselect-all:hover {
  color: var(--text-secondary);
}

.ai-pack-grid {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.ai-pack-card {
  display: flex;
  gap: 0.75rem;
  padding: 0.625rem;
  border: 1px solid var(--border-default);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
  background: var(--bg-surface);
}

.ai-pack-card:hover {
  border-color: var(--border-focus);
}

.ai-pack-card.selected {
  border-color: var(--accent);
  background: var(--accent-glow);
}

.ai-pack-card-icon {
  width: 48px;
  height: 48px;
  border-radius: 6px;
  overflow: hidden;
  flex-shrink: 0;
  position: relative;
  background: var(--bg-hover-1);
}

.ai-pack-card-icon img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.ai-pack-card-placeholder {
  width: 100%;
  height: 100%;
  background: var(--bg-hover-2);
}

.ai-pack-card-check {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.5);
  color: var(--accent);
  font-size: 1.2rem;
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
  white-space: nowrap;
}

.ai-pack-card-desc {
  font-size: 0.7rem;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-top: 0.15rem;
}

.ai-drawer-empty {
  font-size: 0.875rem;
  color: var(--text-dim);
  text-align: center;
  padding: 2rem 0;
}

.ai-drawer-foot {
  display: flex;
  gap: 0.5rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border-default);
  justify-content: flex-end;
}
</style>
