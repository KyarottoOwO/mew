<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { PortPack, PortPackFromURL } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const mode = ref('file')
const file = ref(null)
const fileName = ref('')
const fileSize = ref('')
const fileType = ref('')
const fileModified = ref('')
const showFileInfo = ref(false)
const url = ref('')

const fileUpload = ref(null)
const dropZone = ref(null)

const showProgress = ref(false)
const progressStatus = ref('Starting...')
const progressWidth = ref(0)
const progressPercent = ref('0%')
const spinnerClass = ref('pp-spinner')
const progressDone = ref(false)

function handleFile(selectedFile) {
  file.value = selectedFile
  if (selectedFile) {
    fileName.value = selectedFile.name
    fileSize.value = (selectedFile.size / 1024 / 1024).toFixed(2) + ' MB'
    fileType.value = selectedFile.type || 'application/zip'
    fileModified.value = 'modified ' + new Date(selectedFile.lastModified).toLocaleDateString()
    showFileInfo.value = true
  }
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
}

function cancelSelection() {
  file.value = null
  showFileInfo.value = false
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

let progressHandler = null

onMounted(() => {
  progressHandler = EventsOn('progress', (data) => {
    const t = parseInt(data.total) || 0
    const c = parseInt(data.completed) || 0

    if (data.icon === 'success') {
      spinnerClass.value = 'pp-spinner done'
      progressWidth.value = 100
      progressPercent.value = '100%'
      progressStatus.value = (data.title || 'Done') + ': ' + (data.message || '')
      progressDone.value = true
      setTimeout(() => {
        showProgress.value = false
        file.value = null
        showFileInfo.value = false
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
  if (progressHandler) EventsOff('progress')
})

async function confirmPort() {
  resetProgress()
  showProgress.value = true

  if (mode.value === 'url') {
    const trimmedUrl = url.value.trim()
    if (!trimmedUrl) {
      popup('Error', 'Please paste a link first.', 'error')
      showProgress.value = false
      return
    }
    try {
      await PortPackFromURL(trimmedUrl)
    } catch (err) {
      popup('Error', err.toString(), 'error')
      showProgress.value = false
    }
  } else {
    if (!file.value) {
      popup('Error', 'Please upload a file first.', 'error')
      showProgress.value = false
      return
    }
    try {
      const buffer = await file.value.arrayBuffer()
      const bytes = new Uint8Array(buffer)
      await PortPack(Array.from(bytes), file.value.name)
    } catch (err) {
      popup('Error', err.toString(), 'error')
      showProgress.value = false
    }
  }
}

function cancelUrl() {
  url.value = ''
}
</script>

<template>
  <div class="page active-page packporter-page">
    <div class="porter-card">
      <h2 class="card-title">Port Pack</h2>
      <p class="card-desc">Upload a .zip/.rar pack or paste a MediaFire link to port it.</p>

      <div v-if="!showProgress">
        <div class="mode-tabs">
          <button :class="['mode-tab', mode === 'file' ? 'active' : '']" @click="mode = 'file'">
            <i class="fa fa-upload"></i> Upload File
          </button>
          <button :class="['mode-tab', mode === 'url' ? 'active' : '']" @click="mode = 'url'">
            <i class="fa fa-link"></i> Paste Link
          </button>
        </div>

        <div v-if="mode === 'file'">
          <label ref="dropZone" class="drop-zone"
                 @drop="onDrop" @dragover="onDragOver" @dragleave="onDragLeave">
            <input ref="fileUpload" type="file" accept=".zip,.rar" class="hidden" @change="onFileChange" />

            <div v-if="!showFileInfo" class="drop-zone-inner">
              <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
                   stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                <polyline points="17 8 12 3 7 8"></polyline>
                <line x1="12" x2="12" y1="3" y2="15"></line>
              </svg>
              <span class="drop-hint">Drop .zip or .rar here</span>
            </div>

            <div v-else class="file-info-box">
              <div class="flex w-full items-center justify-between gap-4">
                <p class="max-w-xs truncate text-base" style="color: var(--text-secondary);">{{ fileName }}</p>
                <p class="w-fit flex-shrink-0 px-2 py-1 text-sm" style="background: var(--bg-input); color: var(--text-primary);">{{ fileSize }}</p>
              </div>
              <div class="mt-2 flex w-full flex-col items-start justify-between text-sm md:flex-row md:items-center gap-2" style="color: var(--text-dim);">
                <p class="px-1 py-0.5" style="background: var(--bg-input);">{{ fileType }}</p>
                <p>{{ fileModified }}</p>
              </div>
            </div>
          </label>

          <p class="card-desc mt-4">After uploading click confirm to start porting your pack.</p>

          <div class="flex items-center gap-2 mt-6">
            <button class="btn-cancel" @click="cancelSelection">Cancel</button>
            <button class="btn-main" @click="confirmPort">Confirm</button>
          </div>
        </div>

        <div v-if="mode === 'url'">
          <div class="mb-4">
            <input v-model="url" type="text" placeholder="https://www.mediafire.com/file/..."
                   class="url-input" />
          </div>
          <p class="card-desc">Paste a MediaFire link to a .zip or .rar pack.</p>

          <div class="flex items-center gap-2 mt-6">
            <button class="btn-cancel" @click="cancelUrl">Clear</button>
            <button class="btn-main" @click="confirmPort">Confirm</button>
          </div>
        </div>
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
  justify-content: center;
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

.mode-tabs {
  display: flex;
  gap: 0.25rem;
  margin-bottom: 1rem;
  background: var(--bg-input);
  border-radius: 6px;
  padding: 3px;
}

.mode-tab {
  flex: 1;
  padding: 0.4rem 0.75rem;
  background: transparent;
  color: var(--text-dim);
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.8rem;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
}

.mode-tab.active {
  background: var(--bg-hover-2);
  color: var(--accent);
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

.url-input {
  width: 100%;
  background: var(--bg-input);
  border: 1px solid var(--border-focus);
  color: var(--text-secondary);
  font-size: 0.875rem;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  outline: none;
}

.url-input:focus {
  border-color: var(--accent);
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
