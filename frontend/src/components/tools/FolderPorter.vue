<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue'
import { PortFolder, PortLocalArchive, CancelPortFolder } from '../../../wailsjs/go/main/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

import { progressStore, startPort, updateFromEvent, finish, clearProgress } from '../../utils/progressStore'
const props = defineProps({ active: Boolean })
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
  progressHandler = EventsOn('progress', (data) => {
    if (!isMounted.value || !startedPort.value) return
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
        clearArchive()
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

function handleArchiveFile(f) {
  if (startedPort.value) return
  const lower = f.name.toLowerCase()
  if (!lower.endsWith('.zip') && !lower.endsWith('.rar')) {
    popup('Error', 'Please provide a .zip or .rar file.', 'error')
    return
  }
  archiveFile.value = f
  archiveFileName.value = f.name
  archiveFileSize.value = (f.size / 1024 / 1024).toFixed(2) + ' MB'
  startArchivePort(f)
}

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
  e.target.value = ''
}

function clearArchive() {
  archiveFile.value = null
  archiveFileName.value = ''
  archiveFileSize.value = ''
}

function onUrlPaste() {
  setTimeout(() => {
    if (url.value && url.value.trim()) startUrlPort(url.value)
  }, 0)
}

function onUrlEnter(e) {
  startUrlPort(e.target.value)
}

async function startArchivePort(f) {
  resetProgress()
  showProgress.value = true
  startedPort.value = true
  startPort('packFolderPorter', 'Connecting...')
  try {
    const buffer = await f.arrayBuffer()
    const bytes = new Uint8Array(buffer)
    await PortLocalArchive(Array.from(bytes), f.name)
  } catch (err) {
    console.error('[FolderPorter] PortLocalArchive error:', err)
    popup('Error', err.toString(), 'error')
    startedPort.value = false
    clearProgress()
    showProgress.value = false
    clearArchive()
  }
}

async function startUrlPort(raw) {
  const trimmed = (raw || '').trim()
  if (!trimmed || startedPort.value) return
  url.value = trimmed
  resetProgress()
  showProgress.value = true
  startedPort.value = true
  startPort('packFolderPorter', 'Connecting...')
  try {
    await PortFolder(trimmed)
  } catch (err) {
    console.error('[FolderPorter] PortFolder error:', err)
    popup('Error', err.toString(), 'error')
    startedPort.value = false
    clearProgress()
    showProgress.value = false
  }
}

async function cancelPorter() {
  try {
    await CancelPortFolder()
  } catch (e) {
    console.error('[FolderPorter] CancelPortFolder error:', e)
  }
  startedPort.value = false
  showProgress.value = false
  url.value = ''
  clearArchive()
}
</script>

<template>
  <div class="page active-page folderporter-page">
    <div class="porter-card">
      <h2 class="card-title">Multi-Pack Porter</h2>
      <p class="card-desc">Drop an archive of packs or paste a MediaFire link &mdash; porting starts automatically, no buttons.</p>

      <div v-if="!showProgress">
        <label ref="dropZone" class="drop-zone"
               @drop="onDrop" @dragover="onDragOver" @dragleave="onDragLeave">
          <input type="file" accept=".zip,.rar" class="hidden" @change="onFileChange" />

          <div v-if="!archiveFile" class="drop-zone-inner">
            <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
                 stroke-linecap="round" stroke-linejoin="round" class="drop-icon">
              <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
              <polyline points="17 8 12 3 7 8"></polyline>
              <line x1="12" x2="12" y1="3" y2="15"></line>
            </svg>
            <span class="drop-hint">Drop a .zip or .rar of packs here</span>
            <span class="drop-sub">or click to browse</span>
          </div>

          <div v-else class="file-info-box">
            <div class="file-info-row">
              <span class="file-info-name">{{ archiveFileName }}</span>
              <span class="file-info-size">{{ archiveFileSize }}</span>
            </div>
          </div>
        </label>

        <div class="url-row">
          <i class="fa fa-link url-icon"></i>
          <input v-model="url" type="text"
                 placeholder="...or paste a MediaFire folder link"
                 class="url-input" @paste="onUrlPaste" @keyup.enter="onUrlEnter" />
        </div>
        <p class="card-hint">Pasting a link or pressing Enter starts porting it immediately.</p>
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
          <button class="btn-cancel text-xs" @click="cancelPorter"><i class="fa fa-ban"></i> Cancel</button>
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

.hidden { display: none; }

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
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.btn-cancel:hover {
  background: var(--bg-hover-5);
}
</style>