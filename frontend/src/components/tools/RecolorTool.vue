<script setup>
import { ref } from 'vue'
import { CheckPack } from '../../../wailsjs/go/main/App'

const emit = defineEmits(['openDisplay'])
const file = ref(null)
const showFileInfo = ref(false)
const fileName = ref('')
const fileSize = ref('')
const fileType = ref('')
const fileModified = ref('')

const dropZone = ref(null)

function handleFile(selectedFile) {
  file.value = selectedFile
  if (selectedFile) {
    fileName.value = selectedFile.name
    fileSize.value = (selectedFile.size / 1024 / 1024).toFixed(2) + ' MB'
    fileType.value = selectedFile.type || 'application/x-mcpack'
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

async function confirmCheck() {
  if (!file.value) {
    popup('Error', 'Please upload a file first.', 'error')
    return
  }

  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({
    title: 'Checking',
    text: 'Loading pack...',
    icon: 'info',
    allowOutsideClick: false,
    allowEscapeKey: false,
    showConfirmButton: false,
    didOpen: () => Swal.showLoading()
  })

  try {
    const buffer = await file.value.arrayBuffer()
    const bytes = new Uint8Array(buffer)
    const result = await CheckPack(Array.from(bytes))

    Swal.close()

    if (result.valid) {
      emit('openDisplay', { folders: result.folders, packName: file.value.name })
    } else {
      popup('Error', result.errorMsg || 'Invalid pack', 'error')
    }
  } catch (err) {
    Swal.close()
    popup('Error', err.toString(), 'error')
  }
}
</script>

<template>
  <div class="page active-page recolor-page">
    <div class="porter-card">
      <h2 class="card-title">Recolor Pack</h2>
      <p class="card-desc">Drag and drop your pack here or click to upload.</p>

      <label ref="dropZone" class="drop-zone"
             @drop="onDrop" @dragover="onDragOver" @dragleave="onDragLeave">
        <input type="file" accept=".mcpack" class="hidden" @change="onFileChange" />

        <div v-if="!showFileInfo" class="drop-zone-inner">
          <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
               stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
            <polyline points="17 8 12 3 7 8"></polyline>
            <line x1="12" x2="12" y1="3" y2="15"></line>
          </svg>
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

      <p class="card-desc mt-4">After uploading click confirm to start recoloring your pack.</p>

      <div class="flex items-center gap-2 mt-6">
        <button class="btn-cancel" @click="cancelSelection">Cancel</button>
        <button class="btn-main" @click="confirmCheck">Confirm</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.recolor-page {
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
  min-height: 100px;
}

.drop-zone:hover, .drop-zone.drag-over {
  border-color: var(--accent);
  background: var(--accent-glow);
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

.btn-main {
  padding: 0.5rem 1rem;
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.hidden { display: none; }
</style>
