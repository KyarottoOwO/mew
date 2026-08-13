<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { PortFolder, PortLocalArchive, CancelPortFolder } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { progressStore, startPort, updateFromEvent, finish, clearProgress } from '../utils/progressStore'

const props = defineProps({ active: Boolean })
const mode = ref('url')
const url = ref('')
const showProgress = ref(false)
const progressList = ref([])
const progressStatus = ref('Connecting...')
const progressPercent = ref('0%')
const progressCount = ref('0 / 0 packs')
const progressWidth = ref(0)
const spinnerClass = ref('fp-spinner')
const totalPacks = ref(0)
const startedPort = ref(false)

const dropZone = ref(null)
const archiveFile = ref(null)
const archiveFileName = ref('')
const archiveFileSize = ref('')

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

function fpAddEntry(name, state) {
  const existing = progressList.value.find(e => e.name === name)
  if (existing) {
    existing.state = state
    return
  }
  progressList.value.push({ name, state })
}

function fpUpdateBar(completed, total) {
  const pct = total > 0 ? Math.round((completed / total) * 100) : 0
  progressWidth.value = pct
  progressPercent.value = total > 0 ? pct + '%' : ''
  progressCount.value = total > 0 ? completed + ' / ' + total + ' packs' : 'Processing...'
}

function resetProgress() {
  progressList.value = []
  progressWidth.value = 0
  progressPercent.value = '0%'
  progressCount.value = '0 / 0 packs'
  progressStatus.value = 'Connecting...'
  spinnerClass.value = 'fp-spinner'
  totalPacks.value = 0
}

let progressHandler = null
const isMounted = ref(false)

onMounted(() => {
  isMounted.value = true
  console.log('[FolderPorter] mounted, registering progress listener')
  progressHandler = EventsOn('progress', (data) => {
    if (!isMounted.value || !startedPort.value) {
      console.log('[FolderPorter] progress event received but not started or unmounted, ignoring')
      return
    }
    console.log('[FolderPorter] progress event:', data.title, data.message, data.icon)
    const t = parseInt(data.total) || 0
    const c = parseInt(data.completed) || 0

    if (data.icon === 'success' || data.icon === 'error' || data.icon === 'warning') {
      finish(data)
      startedPort.value = false
    } else {
      updateFromEvent(data)
    }

    if (!props.active) return

    if (t > 0) totalPacks.value = t

    progressStatus.value = (data.title || '') + ': ' + (data.message || '')

    if (data.icon === 'success') {
      spinnerClass.value = 'fp-spinner done'
      progressWidth.value = 100
      progressPercent.value = '100%'
      fpUpdateBar(totalPacks.value, totalPacks.value)
      setTimeout(() => {
        popup('All Done!', data.message, 'success')
        showProgress.value = false
        url.value = ''
        archiveFile.value = null
        archiveFileName.value = ''
        archiveFileSize.value = ''
      }, 600)
    } else if (data.icon === 'error') {
      spinnerClass.value = 'fp-spinner fail'
      setTimeout(() => {
        popup('Error', data.message, 'error')
        showProgress.value = false
      }, 400)
    } else if (data.icon === 'warning') {
      spinnerClass.value = 'fp-spinner fail'
      fpUpdateBar(totalPacks.value, totalPacks.value)
      setTimeout(() => {
        popup('Cancelled', data.message, 'warning')
        showProgress.value = false
      }, 400)
    } else if (data.icon === 'info') {
      if (t > 0) fpUpdateBar(c, t)
    } else if (data.icon === 'done' || data.icon === 'fail') {
      if (t > 0) fpUpdateBar(c, t)
      if (data.fileName) {
        fpAddEntry(data.fileName, data.icon === 'done' ? 'done' : 'fail')
      }
    }
  })
})

onUnmounted(() => {
  isMounted.value = false
  console.log('[FolderPorter] unmounted')
})

watch(() => props.active, (active) => {
  if (!active) return
  if (startedPort.value && progressStore.active && progressStore.source === 'packFolderPorter') {
    showProgress.value = true
    progressStatus.value = (progressStore.title || '') + (progressStore.message ? ': ' + progressStore.message : '')
    progressWidth.value = progressStore.percent
    progressPercent.value = progressStore.percent + '%'
    progressList.value = progressStore.packList.slice()
    totalPacks.value = progressStore.total || totalPacks.value
    spinnerClass.value = 'fp-spinner'
  } else {
    showProgress.value = false
    resetProgress()
  }
})

function onDrop(e) {
  e.preventDefault()
  dropZone.value?.classList.remove('drag-over')
  const droppedFile = e.dataTransfer.files[0]
  if (droppedFile) handleArchiveFile(droppedFile)
}

function onDragOver(e) {
  e.preventDefault()
  dropZone.value?.classList.add('drag-over')
}

function onDragLeave() {
  dropZone.value?.classList.remove('drag-over')
}

function onFileChange(e) {
  if (e.target.files[0]) handleArchiveFile(e.target.files[0])
}

function handleArchiveFile(f) {
  const lower = f.name.toLowerCase()
  if (!lower.endsWith('.zip') && !lower.endsWith('.rar')) {
    popup('Error', 'Please provide a .zip or .rar file.', 'error')
    return
  }
  archiveFile.value = f
  archiveFileName.value = f.name
  archiveFileSize.value = (f.size / 1024 / 1024).toFixed(2) + ' MB'
}

function clearArchive() {
  archiveFile.value = null
  archiveFileName.value = ''
  archiveFileSize.value = ''
}

async function confirmPort() {
  console.log('[FolderPorter] confirmPort called, mode:', mode.value)
  if (mode.value === 'url') {
    const trimmedUrl = url.value.trim()
    if (!trimmedUrl) {
      popup('Error', 'Please paste a link first.', 'error')
      return
    }
    showProgress.value = true
    resetProgress()
    startedPort.value = true
    startPort('packFolderPorter', 'Connecting...')
    console.log('[FolderPorter] calling PortFolder')
    try {
      await PortFolder(trimmedUrl)
      console.log('[FolderPorter] PortFolder resolved')
    } catch (err) {
      console.error('[FolderPorter] PortFolder error:', err)
      popup('Error', err.toString(), 'error')
      startedPort.value = false
      clearProgress()
      showProgress.value = false
    }
  } else {
    if (!archiveFile.value) {
      popup('Error', 'Please upload a file first.', 'error')
      return
    }
    showProgress.value = true
    resetProgress()
    startedPort.value = true
    startPort('packFolderPorter', 'Connecting...')
    console.log('[FolderPorter] calling PortLocalArchive')
    try {
      const buffer = await archiveFile.value.arrayBuffer()
      const bytes = new Uint8Array(buffer)
      await PortLocalArchive(Array.from(bytes), archiveFile.value.name)
      console.log('[FolderPorter] PortLocalArchive resolved')
    } catch (err) {
      console.error('[FolderPorter] PortLocalArchive error:', err)
      popup('Error', err.toString(), 'error')
      startedPort.value = false
      clearProgress()
      showProgress.value = false
    }
  }
}

async function cancelPorter() {
  console.log('[FolderPorter] cancelPorter called')
  try {
    await CancelPortFolder()
    console.log('[FolderPorter] CancelPortFolder resolved')
  } catch (e) {
    console.error('[FolderPorter] CancelPortFolder error:', e)
  }
  showProgress.value = false
  url.value = ''
  clearArchive()
}
</script>

<template>
  <div class="page active-page folderporter-page">
    <div class="porter-card">
      <h2 class="card-title">Multi-Pack Porter</h2>
      <p class="card-desc">Port multiple packs at once from a MediaFire link or a local archive.</p>

      <div v-if="!showProgress">
        <div class="mode-tabs">
          <button :class="['mode-tab', mode === 'url' ? 'active' : '']" @click="mode = 'url'">
            <i class="fa fa-link"></i> MediaFire Link
          </button>
          <button :class="['mode-tab', mode === 'file' ? 'active' : '']" @click="mode = 'file'">
            <i class="fa fa-upload"></i> Upload Archive
          </button>
        </div>

        <div v-if="mode === 'url'">
          <div class="mb-4">
            <input v-model="url" type="text" placeholder="https://www.mediafire.com/file/... or /folder/..."
                   class="url-input" />
          </div>
          <p class="card-desc">Paste a MediaFire link to a single pack or a folder of packs.</p>
        </div>

        <div v-if="mode === 'file'">
          <label ref="dropZone" class="drop-zone"
                 @drop="onDrop" @dragover="onDragOver" @dragleave="onDragLeave">
            <input type="file" accept=".zip,.rar" class="hidden" @change="onFileChange" />

            <div v-if="!archiveFile" class="drop-zone-inner">
              <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
                   stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4">
                <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
                <polyline points="17 8 12 3 7 8"></polyline>
                <line x1="12" x2="12" y1="3" y2="15"></line>
              </svg>
              <span class="drop-hint">Drop .zip or .rar containing packs</span>
            </div>

            <div v-else class="file-info-box">
              <div class="flex w-full items-center justify-between gap-4">
                <p class="max-w-xs truncate text-base" style="color: var(--text-secondary);">{{ archiveFileName }}</p>
                <p class="w-fit flex-shrink-0 px-2 py-1 text-sm" style="background: var(--bg-input); color: var(--text-primary);">{{ archiveFileSize }}</p>
              </div>
            </div>
          </label>
          <p class="card-desc mt-4">Upload a .zip or .rar that contains one or more pack .zip files inside.</p>
        </div>

        <div class="flex items-center gap-2 mt-6">
          <button class="btn-cancel" @click="cancelPorter">Cancel</button>
          <button class="btn-main" @click="confirmPort">Confirm</button>
        </div>
      </div>

      <div v-else>
        <div class="flex items-center gap-3 mb-3">
          <div :class="spinnerClass"></div>
          <div class="text-sm" style="color: var(--text-secondary);">{{ progressStatus }}</div>
        </div>

        <div class="progress-track">
          <div class="progress-bar" :style="{ width: progressWidth + '%' }"></div>
        </div>

        <div class="flex justify-between text-xs mb-3" style="color: var(--text-dim);">
          <span>{{ progressPercent }}</span>
          <span>{{ progressCount }}</span>
        </div>

        <div class="progress-list">
          <div v-for="entry in progressList" :key="entry.name"
               :class="['fp-entry', entry.state === 'active' ? 'active' : '']"
               :style="{ color: entry.state === 'done' ? '#22c55e' : entry.state === 'fail' ? '#ef4444' : '' }">
            <div class="fp-entry-icon">
              <div v-if="entry.state === 'active'" class="fp-mini-spinner"></div>
              <span v-else-if="entry.state === 'done'" class="fp-check">&#10003;</span>
              <span v-else-if="entry.state === 'fail'" class="fp-x">&#10007;</span>
              <span v-else class="fp-dash">&#8212;</span>
            </div>
            <span>{{ entry.name }}</span>
          </div>
        </div>

        <div class="flex justify-end mt-3">
          <button class="btn-cancel text-xs" @click="cancelPorter">Cancel</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.folderporter-page {
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

.progress-list {
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid var(--border-default);
  border-radius: 4px;
  padding: 0.5rem;
}

.fp-entry {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.25rem 0;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.fp-entry-icon {
  width: 16px;
  text-align: center;
  flex-shrink: 0;
}

.fp-mini-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid var(--border-medium);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.fp-spinner {
  width: 20px;
  height: 20px;
  border: 3px solid var(--border-strong);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.fp-spinner.done { border-color: #22c55e; border-top-color: #22c55e; }
.fp-spinner.fail { border-color: #ef4444; border-top-color: #ef4444; animation: none; }

@keyframes spin {
  to { transform: rotate(360deg); }
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
</style>
