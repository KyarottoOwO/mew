<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { PortPack, PortPackFromURL } from '../../../wailsjs/go/main/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

import { progressStore, startPort, updateFromEvent, finish, clearProgress } from '../../utils/progressStore'
const props = defineProps({ active: Boolean })
const file = ref(null)
const fileName = ref('')
const fileSize = ref('')
const fileType = ref('')
const fileModified = ref('')
const url = ref('')

const dropZone = ref(null)

const showProgress = ref(false)
const progressStatus = ref('Starting...')
const progressWidth = ref(0)
const progressPercent = ref('0%')
const spinnerClass = ref('pp-spinner')
const progressDone = ref(false)
const startedPort = ref(false)

function handleFile(selectedFile) {
  if (!selectedFile || startedPort.value) return
  file.value = selectedFile
  fileName.value = selectedFile.name
  fileSize.value = (selectedFile.size / 1024 / 1024).toFixed(2) + ' MB'
  fileType.value = selectedFile.type || 'application/zip'
  fileModified.value = 'modified ' + new Date(selectedFile.lastModified).toLocaleDateString()
  startFilePort(selectedFile)
}

function onDrop(e) {
  e.preventDefault()
  dropZone.value?.classList.remove('drag-over')
  const droppedFile = e.dataTransfer.files[0]
  if (droppedFile) handleFile(droppedFile)
}

function onDragOver(e) {
  e.preventDefault()
  dropZone.value?.classList.add('drag-over')
}

function onDragLeave() {
  dropZone.value?.classList.remove('drag-over')
}

function onFileChange(e) {
  if (e.target.files[0]) handleFile(e.target.files[0])
  e.target.value = ''
}

function onUrlPaste() {
  setTimeout(() => {
    if (url.value && url.value.trim()) startUrlPort(url.value)
  }, 0)
}

function onUrlEnter(e) {
  startUrlPort(e.target.value)
}

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

function resetProgress() {
  progressStatus.value = 'Starting...'
  progressWidth.value = 0
  progressPercent.value = '0%'
  spinnerClass.value = 'pp-spinner'
  progressDone.value = false
}

async function startFilePort(selectedFile) {
  resetProgress()
  showProgress.value = true
  startedPort.value = true
  startPort('packporter', 'Starting...')
  try {
    const buffer = await selectedFile.arrayBuffer()
    const bytes = new Uint8Array(buffer)
    await PortPack(Array.from(bytes), selectedFile.name)
  } catch (err) {
    console.error('[PackPorter] PortPack error:', err)
    popup('Error', err.toString(), 'error')
    startedPort.value = false
    clearProgress()
    showProgress.value = false
  }
}

async function startUrlPort(raw) {
  const trimmed = (raw || '').trim()
  if (!trimmed || startedPort.value) return
  url.value = trimmed
  resetProgress()
  showProgress.value = true
  startedPort.value = true
  startPort('packporter', 'Starting...')
  try {
    await PortPackFromURL(trimmed)
  } catch (err) {
    console.error('[PackPorter] PortPackFromURL error:', err)
    popup('Error', err.toString(), 'error')
    startedPort.value = false
    clearProgress()
    showProgress.value = false
  }
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
      spinnerClass.value = 'pp-spinner done'
      progressWidth.value = 100
      progressPercent.value = '100%'
      progressStatus.value = (data.title || 'Done') + ': ' + (data.message || '')
      progressDone.value = true
      setTimeout(() => {
        showProgress.value = false
        file.value = null
        fileName.value = ''
        fileSize.value = ''
        fileType.value = ''
        fileModified.value = ''
        url.value = ''
      }, 1500)
    } else if (data.icon === 'error') {
      spinnerClass.value = 'pp-spinner fail'
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
})

watch(() => props.active, (active) => {
  if (!active) return
  if (startedPort.value && progressStore.active && progressStore.source === 'packporter') {
    showProgress.value = true
    progressStatus.value = (progressStore.title || '') + (progressStore.message ? ': ' + progressStore.message : '')
    progressWidth.value = progressStore.percent
    progressPercent.value = progressStore.percent + '%'
    spinnerClass.value = 'pp-spinner'
    progressDone.value = false
  } else {
    showProgress.value = false
    resetProgress()
  }
})
</script>

<template>
  <div class="page active-page packporter-page">
    <div class="porter-card">
      <h2 class="card-title">Port Pack</h2>
      <p class="card-desc">Drop a .zip/.rar pack or paste a MediaFire link &mdash; porting starts automatically, no buttons.</p>

      <div v-if="!showProgress">
        <label ref="dropZone" class="drop-zone"
               @drop="onDrop" @dragover="onDragOver" @dragleave="onDragLeave">
          <input type="file" accept=".zip,.rar" class="hidden" @change="onFileChange" />

          <div v-if="!file" class="drop-zone-inner">
            <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
                 stroke-linecap="round" stroke-linejoin="round" class="drop-icon">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
              <polyline points="17 8 12 3 7 8"></polyline>
              <line x1="12" x2="12" y1="3" y2="15"></line>
            </svg>
            <span class="drop-hint">Drop .zip or .rar here</span>
            <span class="drop-sub">or click to browse</span>
          </div>

          <div v-else class="file-info-box">
            <div class="file-info-row">
              <span class="file-info-name">{{ fileName }}</span>
              <span class="file-info-size">{{ fileSize }}</span>
            </div>
            <span class="file-info-sub">{{ fileType }} &middot; {{ fileModified }}</span>
          </div>
        </label>

        <div class="url-row">
          <i class="fa fa-link url-icon"></i>
          <input v-model="url" type="text"
                 placeholder="...or paste a MediaFire link"
                 class="url-input" @paste="onUrlPaste" @keyup.enter="onUrlEnter" />
        </div>
        <p class="card-hint">Pasting a link or pressing Enter starts porting it immediately.</p>
      </div>

      <div v-else class="progress-section">
        <div class="flex items-center gap-3 mb-3">
          <div :class="spinnerClass"></div>
          <div class="text-sm" style="color: var(--text-secondary);">{{ progressStatus }}</div>
        </div>

        <div class="progress-track">
          <div class="progress-bar" :style="{ width: progressWidth + '%' }"></div>
        </div>

        <div class="flex justify-between text-xs mb-3" style="color: var(--text-dim);">
          <span>{{ progressPercent }}</span>
          <span v-if="progressDone" style="color: #22c55e;">Complete</span>
        </div>

        <div class="flex justify-end mt-3">
          <button class="btn-cancel text-xs" @click="showProgress = false">Close</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.packporter-page {
  justify-content: safe center;
  align-items: center;
}

.porter-card {
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  padding: 1.5rem;
  width: 100%;
  max-width: 480px;
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

.drop-zone {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px dashed var(--border-strong);
  border-radius: 8px;
  padding: 2rem;
  cursor: pointer;
  transition: all 0.2s;
  min-height: 120px;
}

.drop-zone:hover, .drop-zone.drag-over {
  border-color: var(--accent);
  background: var(--accent-glow);
}

.drop-zone-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
}

.drop-icon {
  width: 28px;
  height: 28px;
  color: var(--accent);
}

.drop-hint {
  font-size: 0.8rem;
  color: var(--text-dim);
}

.drop-sub {
  font-size: 0.72rem;
  color: var(--text-dim);
  opacity: 0.7;
}

.file-info-box {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  pointer-events: none;
}

.file-info-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.file-info-name {
  font-size: 0.85rem;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-info-size {
  font-size: 0.75rem;
  background: var(--bg-input);
  color: var(--text-primary);
  padding: 0.15rem 0.5rem;
  border-radius: 4px;
  flex-shrink: 0;
}

.file-info-sub {
  font-size: 0.72rem;
  color: var(--text-dim);
}

.url-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 1rem;
  background: var(--bg-input);
  border: 1px solid var(--border-focus);
  border-radius: 6px;
  padding: 0 0.75rem;
}

.url-row:focus-within {
  border-color: var(--accent);
}

.url-icon {
  color: var(--text-dim);
  font-size: 0.8rem;
  flex-shrink: 0;
}

.url-input {
  flex: 1;
  background: transparent;
  border: none;
  color: var(--text-secondary);
  font-size: 0.875rem;
  padding: 0.6rem 0;
  outline: none;
  min-width: 0;
}

.card-hint {
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-top: 0.5rem;
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

.hidden { display: none; }

.progress-section {
  padding-top: 0.5rem;
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

.pp-spinner {
  width: 20px;
  height: 20px;
  border: 3px solid var(--border-strong);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.pp-spinner.done {
  border-color: #22c55e;
  border-top-color: #22c55e;
}

.pp-spinner.fail {
  border-color: #ef4444;
  border-top-color: #ef4444;
  animation: none;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>