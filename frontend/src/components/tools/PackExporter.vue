<script setup>
import { ref, watch, onMounted } from 'vue'
import { GetInstalledPacksDetailed, SetResourcePacksPath, SelectDirectory, ExportInstalledPacks, GetExportOutputDir, OpenFolder } from '../../../wailsjs/go/main/App'

import { parseBedrockCodes } from '../../utils/formatCodes'

const props = defineProps({ show: Boolean, active: Boolean })
const emit = defineEmits(['close'])

const selectedPacks = ref([])
const installInfo = ref({ path: '', found: false, packs: [] })
const pickingDir = ref(false)

const exporting = ref(false)
const done = ref(false)
const status = ref('')
const error = ref('')

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
    console.error('[PackExporter] GetInstalledPacksDetailed error:', err)
    installInfo.value = { path: '', found: false, packs: [] }
  }
}

function close() {
  emit('close')
}

function togglePack(p) {
  const i = selectedPacks.value.findIndex(s => s.name === p.name)
  if (i >= 0) selectedPacks.value.splice(i, 1)
  else selectedPacks.value.push(p)
  done.value = false
  status.value = ''
  error.value = ''
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
    console.error('[PackExporter] SelectDirectory error:', err)
  } finally {
    pickingDir.value = false
  }
}

async function exportPack() {
  if (!selectedPacks.value.length || exporting.value) return
  exporting.value = true
  done.value = false
  error.value = ''
  status.value = ''
  try {
    const names = selectedPacks.value.map(p => p.name)
    const out = await ExportInstalledPacks(names)
    status.value = out && out.length ? out.join(', ') : ''
    done.value = true
  } catch (err) {
    console.error('[PackExporter] ExportInstalledPacks error:', err)
    error.value = String(err?.toString ? err.toString() : err)
    status.value = ''
  } finally {
    exporting.value = false
  }
}

async function openOutputFolder() {
  try {
    const dir = await GetExportOutputDir()
    if (dir) await OpenFolder(dir)
  } catch (err) {
    console.error('[PackExporter] openOutputFolder error:', err)
  }
}

watch(() => props.show, (v) => {
  if (v) {
    selectedPacks.value = []
    done.value = false
    status.value = ''
    error.value = ''
    loadInstalledPacks()
  }
})

onMounted(() => {
  loadInstalledPacks()
})
</script>

<template>
  <div v-if="show" class="ai-popup-overlay" @click.self="close">
    <div class="ai-popup">
      <div class="ai-popup-head">
        <div class="ai-popup-title"><i class="fa fa-file-export"></i> Export Pack</div>
        <button class="ai-drawer-close" @click="close">&times;</button>
      </div>

      <div class="ai-popup-body">
        <div v-if="!installInfo.found">
          <p class="ai-modal-warn">Couldn't find Minecraft's resource packs folder.</p>
          <p class="ai-modal-path">{{ installInfo.path || 'No path detected' }}</p>
          <div class="ai-actions">
            <button class="btn-main" :disabled="pickingDir" @click="chooseResourcePacksDir">
              {{ pickingDir ? 'Opening...' : 'Choose Folder' }}
            </button>
          </div>
        </div>

        <template v-else>
          <p class="ai-drawer-path">{{ installInfo.path }}</p>
          <div class="ai-drawer-header">
            <span class="ai-drawer-count">{{ installInfo.packs.length }} pack(s)</span>
            <button v-if="selectedPacks.length" class="ai-unselect-all" @click="selectedPacks = []; done = false; status = ''; error = ''">Unselect All</button>
          </div>
          <div v-if="installInfo.packs.length > 0" class="ai-pack-grid ai-popup-grid">
            <div
              v-for="p in installInfo.packs"
              :key="p.name"
              class="ai-pack-card"
              :class="{ selected: selectedPacks.some(s => s.name === p.name) }"
              @click="togglePack(p)"
            >
              <div class="ai-pack-card-icon">
                <img v-if="p.icon" :src="p.icon" alt="" />
                <div v-else class="ai-pack-card-placeholder"></div>
                <div v-if="selectedPacks.some(s => s.name === p.name)" class="ai-pack-card-check">&#10003;</div>
              </div>
              <div class="ai-pack-card-info">
                <div class="ai-pack-card-name" v-html="parseBedrockCodes(p.name)"></div>
                <div v-if="p.description" class="ai-pack-card-desc" v-html="parseBedrockCodes(p.description)"></div>
              </div>
            </div>
          </div>
          <p v-else class="ai-drawer-empty">No packs found.</p>
        </template>
      </div>

      <div class="ai-popup-foot">
        <p v-if="selectedPacks.length" class="ai-popup-selected">
          <i class="fa fa-check-circle"></i> {{ selectedPacks.length }} selected
        </p>
        <button class="btn-cancel" @click="close">Close</button>
        <button class="btn-main" :class="{ installed: selectedPacks.length }" :disabled="!selectedPacks.length || exporting" @click="exportPack">
          <i class="fa" :class="exporting ? 'fa-circle-notch fa-spin' : 'fa-download'"></i> {{ exporting ? 'Exporting...' : 'Export .mcpack' }}
        </button>
        <button v-if="done && status" class="btn-main" @click="openOutputFolder"><i class="fa fa-folder-open"></i> Open Folder</button>
      </div>

      <p v-if="done && status" class="ai-install-note ai-popup-note"><i class="fa fa-circle-check"></i> Exported {{ selectedPacks.length }} pack(s) to <code style="color:var(--accent)">{{ status }}</code></p>
      <p v-if="error" class="ex-error ai-popup-note">{{ error }}</p>
    </div>
  </div>
</template>

<style scoped>
.ai-popup-overlay {
  position: fixed;
  inset: 0;
  z-index: 80;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
  animation: aiPopFade 0.15s ease-out;
}

@keyframes aiPopFade {
  from { opacity: 0; }
  to { opacity: 1; }
}

.ai-popup {
  width: 460px;
  max-width: calc(100vw - 2rem);
  max-height: 82vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 12px;
  box-shadow: 0 12px 48px rgba(0, 0, 0, 0.5);
  animation: aiPopZoom 0.18s ease-out;
}

@keyframes aiPopZoom {
  from { transform: scale(0.95); opacity: 0; }
  to { transform: scale(1); opacity: 1; }
}

.ai-popup-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border-default);
}

.ai-popup-title {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 1rem;
  font-weight: 600;
  color: var(--accent);
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

.ai-popup-body {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 1rem 1.25rem;
  overflow: hidden;
  min-height: 0;
}

.ai-modal-warn {
  font-size: 0.875rem;
  color: var(--text-secondary);
}

.ai-modal-path,
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
  min-height: 0;
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

.ai-popup-foot {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border-default);
}

.ai-popup-selected {
  flex: 1;
  min-width: 120px;
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.ai-popup-note {
  margin: 0 1.25rem 1rem;
  word-break: break-all;
}

.ai-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
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
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
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
  font-size: 0.8rem;
  color: #22c55e;
}

.ex-error {
  font-size: 0.8rem;
  color: #ef4444;
}
</style>
