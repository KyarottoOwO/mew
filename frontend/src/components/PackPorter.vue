<script setup>
import { ref } from 'vue'
import { PortPack } from '../../wailsjs/go/main/App'

const file = ref(null)
const fileName = ref('')
const fileSize = ref('')
const fileType = ref('')
const fileModified = ref('')
const showFileInfo = ref(false)

const fileUpload = ref(null)
const dropZone = ref(null)

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

async function confirmPort() {
  if (!file.value) {
    popup('Error', 'Please upload a file first.', 'error')
    return
  }

  popup('Porting', file.value.name + ' is being ported, please wait...', 'info')

  try {
    const buffer = await file.value.arrayBuffer()
    const bytes = new Uint8Array(buffer)
    const result = await PortPack(Array.from(bytes), file.value.name)
    popup('Done!', result, 'success')
  } catch (err) {
    popup('Error', err.toString(), 'error')
  }
}
</script>

<template>
  <div class="page active-page packporter-page">
    <div class="porter-card">
      <h2 class="card-title">Port Pack</h2>
      <p class="card-desc">Drag and drop your pack here or click to upload.</p>

      <label ref="dropZone" class="drop-zone"
             @drop="onDrop" @dragover="onDragOver" @dragleave="onDragLeave">
        <input ref="fileUpload" type="file" accept=".zip" class="hidden" @change="onFileChange" />

        <div v-if="!showFileInfo" class="drop-zone-inner">
          <svg stroke="currentColor" fill="none" stroke-width="2" viewBox="0 0 24 24"
               stroke-linecap="round" stroke-linejoin="round" class="h-4 w-4 text-neutral-300">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"></path>
            <polyline points="17 8 12 3 7 8"></polyline>
            <line x1="12" x2="12" y1="3" y2="15"></line>
          </svg>
        </div>

        <div v-else class="file-info-box">
          <div class="flex w-full items-center justify-between gap-4">
            <p class="max-w-xs truncate text-base text-neutral-300">{{ fileName }}</p>
            <p class="w-fit flex-shrink-0 bg-neutral-800 px-2 py-1 text-sm text-white">{{ fileSize }}</p>
          </div>
          <div class="mt-2 flex w-full flex-col items-start justify-between text-sm text-neutral-400 md:flex-row md:items-center gap-2">
            <p class="bg-neutral-800 px-1 py-0.5">{{ fileType }}</p>
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
  </div>
</template>

<style scoped>
.packporter-page {
  justify-content: center;
  align-items: center;
}

.porter-card {
  border: 1px solid hsl(0, 0%, 10%);
  background: hsl(0, 0%, 2%);
  padding: 1.5rem;
  width: 100%;
  max-width: 480px;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: #e879a8;
}

.card-desc {
  font-size: 0.875rem;
  color: hsl(0, 0%, 50%);
  margin-bottom: 1rem;
}

.drop-zone {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px dashed hsl(0, 0%, 20%);
  border-radius: 8px;
  padding: 2rem;
  cursor: pointer;
  transition: all 0.2s;
  min-height: 100px;
}

.drop-zone:hover, .drop-zone.drag-over {
  border-color: #e879a8;
  background: hsl(330, 60%, 5%);
}

.file-info-box {
  width: 100%;
}

.btn-cancel {
  padding: 0.5rem 1rem;
  background: hsl(0, 0%, 10%);
  color: hsl(0, 0%, 60%);
  border: 1px solid hsl(0, 0%, 20%);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-cancel:hover {
  background: hsl(0, 0%, 15%);
}

.btn-main {
  padding: 0.5rem 1rem;
  background: #e879a8;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-main:hover {
  background: #c41048;
}

.hidden { display: none; }
</style>
