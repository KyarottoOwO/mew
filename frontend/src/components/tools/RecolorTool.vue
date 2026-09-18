<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { CheckPack, GetPackListWithInfo, GetPackCache, GetPackCacheList } from '../../../wailsjs/go/main/App'

const props = defineProps({
  active: { type: Boolean, default: false }
})

const emit = defineEmits(['openDisplay'])

const tab = ref('installed')
const packs = ref([])
const packsLoading = ref(false)
const packsLoaded = ref(false)

const cacheSources = ref([])
const cacheLoading = ref(false)
const cacheLoaded = ref(false)

const searchQuery = ref('')

function matchesQuery(p) {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return true
  return (p.name || '').toLowerCase().includes(q) ||
         (p.dirName || '').toLowerCase().includes(q) ||
         (p.description || '').toLowerCase().includes(q)
}

const filteredPacks = computed(() => packs.value.filter(matchesQuery))

const filteredCacheSources = computed(() =>
  cacheSources.value
    .map(src => ({ ...src, packs: src.packs.filter(matchesQuery) }))
    .filter(src => src.packs.length > 0)
)

const file = ref(null)
const showFileInfo = ref(false)
const fileName = ref('')
const fileSize = ref('')
const fileType = ref('')
const fileModified = ref('')

const dropZone = ref(null)

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let n = bytes
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024
    i++
  }
  return n.toFixed(i === 0 ? 0 : 1) + ' ' + units[i]
}

async function loadPacks(force = false) {
  if (packsLoaded.value && !force) return
  packsLoading.value = true
  try {
    packs.value = (await GetPackListWithInfo()) || []
    packsLoaded.value = true
  } catch (err) {
    popup('Error', err.toString(), 'error')
  }
  packsLoading.value = false
}

function pickInstalled(p) {
  emit('openDisplay', { kind: 'installed', dirName: p.dirName })
}

function pickCache(pack, basePath) {
  emit('openDisplay', { kind: 'cache', dirName: pack, basePath })
}

async function loadCache(force = false) {
  if (cacheLoaded.value && !force) return
  cacheLoading.value = true
  try {
    const srcs = (await GetPackCache()) || []
    const detailed = []
    for (const src of srcs) {
      let list = []
      try {
        list = (await GetPackCacheList(src.path)) || []
      } catch (e) {
        list = []
      }
      detailed.push({ ...src, packs: list })
    }
    cacheSources.value = detailed
    cacheLoaded.value = true
  } catch (err) {
    popup('Error', err.toString(), 'error')
  }
  cacheLoading.value = false
}

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
      emit('openDisplay', { kind: 'upload', folders: result.folders, packName: file.value.name })
    } else {
      popup('Error', result.errorMsg || 'Invalid pack', 'error')
    }
  } catch (err) {
    Swal.close()
    popup('Error', err.toString(), 'error')
  }
}

onMounted(() => { loadPacks(); loadCache() })

watch(() => props.active, (isActive) => {
  if (isActive) {
    loadPacks(true)
    loadCache(true)
  }
})
</script>

<template>
  <div class="page active-page recolor-page">
    <div class="porter-card">
      <h2 class="card-title">Pack Editor</h2>
      <p class="card-desc">Edit textures in an installed pack or upload a pack to recolor it.</p>

      <div class="mode-tabs">
        <button class="mode-tab" :class="{ active: tab === 'installed' }" @click="tab = 'installed'">
          Installed Pack
        </button>
        <button class="mode-tab" :class="{ active: tab === 'cache' }" @click="tab = 'cache'">
          Pack Cache
        </button>
        <button class="mode-tab" :class="{ active: tab === 'upload' }" @click="tab = 'upload'">
          Upload .mcpack
        </button>
      </div>

      <div v-if="tab === 'installed'" class="installed-section">
        <p class="card-desc">Pick a pack to edit its textures in place.</p>

        <div class="re-search-wrap">
          <i class="fa fa-search"></i>
          <input v-model="searchQuery" class="re-search" type="text" placeholder="Search packs..." />
          <button v-if="searchQuery" class="re-search-clear" @click="searchQuery = ''" title="Clear search">&times;</button>
        </div>

        <div v-if="packsLoading" class="pack-loading">
          <i class="fa fa-spinner fa-spin"></i> Loading packs...
        </div>
        <div v-else-if="packs.length === 0" class="pack-loading">
          No packs found in your resource packs folder.
        </div>
        <div v-else-if="filteredPacks.length === 0" class="pack-loading">
          No packs match "<strong>{{ searchQuery }}</strong>".
        </div>
        <div v-else class="pv-grid">
          <div v-for="p in filteredPacks" :key="p.dirName" class="pv-card" @click="pickInstalled(p)">
            <img v-if="p.iconURI" :src="p.iconURI" class="pv-card-icon" alt="" />
            <div v-else class="pv-card-icon pv-card-placeholder"><i class="fa fa-box"></i></div>
            <div class="pv-card-info">
              <div class="pv-card-name" v-html="p.name"></div>
              <div class="pv-card-size">{{ formatBytes(p.size) }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-else-if="tab === 'cache'" class="installed-section">
        <p class="card-desc">Packs found in your Minecraft pack cache. Edits write back to the cache in place. Change the cache path in Settings for custom clients.</p>

        <div class="re-search-wrap">
          <i class="fa fa-search"></i>
          <input v-model="searchQuery" class="re-search" type="text" placeholder="Search packs..." />
          <button v-if="searchQuery" class="re-search-clear" @click="searchQuery = ''" title="Clear search">&times;</button>
        </div>

        <div v-if="cacheLoading" class="pack-loading">
          <i class="fa fa-spinner fa-spin"></i> Scanning pack cache...
        </div>
        <div v-else-if="cacheSources.length === 0" class="pack-loading">
          No Minecraft pack cache found.
        </div>
        <div v-else class="cache-sources">
          <div v-for="src in filteredCacheSources" :key="src.path" class="cache-source">
            <div class="cache-source-head">
              <span class="cache-source-name">{{ src.name }}</span>
              <span class="cache-source-path" :title="src.path">{{ src.path }}</span>
            </div>
            <div v-if="searchQuery.trim() !== '' && src.packs.length === 0" class="pack-loading">
              No packs match "<strong>{{ searchQuery }}</strong>".
            </div>
            <div v-else-if="src.packs.length === 0" class="pack-loading">
              No packs found here.
            </div>
            <div v-else class="pv-grid">
              <div v-for="p in src.packs" :key="p.dirName" class="pv-card" @click="pickCache(p.dirName, src.path)">
                <img v-if="p.iconURI" :src="p.iconURI" class="pv-card-icon" alt="" />
                <div v-else class="pv-card-icon pv-card-placeholder"><i class="fa fa-box"></i></div>
                <div class="pv-card-info">
                  <div class="pv-card-name" :title="p.description" v-html="p.name"></div>
                  <div class="pv-card-size">from cache · {{ formatBytes(p.size) }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="upload-section">
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

        <p class="card-desc mt-4">After uploading click confirm to start editing your pack.</p>

        <div class="flex items-center gap-2 mt-6">
          <button class="btn-cancel" @click="cancelSelection">Cancel</button>
          <button class="btn-main" @click="confirmCheck">Confirm</button>
        </div>
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
  max-width: 560px;
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
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}

.mode-tab {
  flex: 1;
  padding: 0.5rem 1rem;
  background: var(--bg-input);
  color: var(--text-muted);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
  transition: all 0.15s;
}

.mode-tab:hover {
  color: var(--text-secondary);
  background: var(--bg-hover-2);
}

.mode-tab.active {
  background: transparent;
  color: var(--accent);
  border-color: var(--accent);
}

.installed-section {
  min-height: 120px;
}

.re-search-wrap {
  position: relative;
  margin-bottom: 1rem;
}

.re-search-wrap .fa-search {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-faint);
  font-size: 0.8rem;
  pointer-events: none;
}

.re-search {
  width: 100%;
  padding: 0.5rem 2.1rem;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  border-radius: 6px;
  color: var(--text-secondary);
  font-size: 0.875rem;
  outline: none;
  transition: border-color 0.15s;
}

.re-search::placeholder {
  color: var(--text-faint);
}

.re-search:focus {
  border-color: var(--accent);
}

.re-search-clear {
  position: absolute;
  right: 0.5rem;
  top: 50%;
  transform: translateY(-50%);
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 1.05rem;
  line-height: 1;
  cursor: pointer;
  padding: 0.25rem;
}

.re-search-clear:hover {
  color: var(--text-secondary);
}

.pack-loading {
  padding: 2rem 0;
  text-align: center;
  color: var(--text-dim);
  font-size: 0.875rem;
}

.pv-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
  gap: 0.75rem;
  max-height: 380px;
  overflow-y: auto;
}

.pv-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 1rem;
  border-radius: 12px;
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  cursor: pointer;
  transition: all 0.15s;
}

.pv-card:hover {
  border-color: var(--accent);
  background: var(--bg-hover-1);
  transform: translateY(-2px);
}

.pv-card-icon {
  width: 56px;
  height: 56px;
  border-radius: 10px;
  object-fit: cover;
  border: 1px solid var(--border-subtle);
}

.pv-card-placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-hover-1);
  color: var(--text-dim);
  font-size: 1.2rem;
}

.pv-card-info {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.15rem;
  min-width: 0;
  width: 100%;
}

.pv-card-name {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  width: 100%;
}

.pv-card-size {
  font-size: 0.6rem;
  color: var(--text-faint);
}

.cache-sources {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  max-height: 380px;
  overflow-y: auto;
}

.cache-source-head {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding-bottom: 0.35rem;
  border-bottom: 1px solid var(--border-subtle);
  margin-bottom: 0.5rem;
}

.cache-source-name {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-secondary);
  flex-shrink: 0;
}

.cache-source-path {
  font-size: 0.62rem;
  color: var(--text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cache-sources .pv-grid {
  max-height: none;
  overflow: visible;
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