<script setup>
import { ref, onMounted } from 'vue'
import { GetSettings, SaveSettings, SelectDirectory, DetectMinecraftPaths, ClearCache } from '../../wailsjs/go/main/App'

const autoImport = ref(false)
const autoOpenFolder = ref(false)
const deleteOriginals = ref(false)
const deleteMcpack = ref(false)
const customOutputDir = ref('')
const manifestDescription = ref('')
const resourcePacksPath = ref('')
const theme = ref('dark')

const detectedPaths = ref([])
const selectedMinecraftPath = ref('')
const isCustomPath = ref(false)
const customMinecraftPath = ref('')

const emit = defineEmits(['settings-changed'])

async function loadSettings() {
  try {
    const s = await GetSettings()
    autoImport.value = s.autoImport || false
    autoOpenFolder.value = s.autoOpenFolder || false
    deleteOriginals.value = s.deleteOriginals || false
    deleteMcpack.value = s.deleteMcpack || false
    customOutputDir.value = s.customOutputDir || ''
    manifestDescription.value = s.manifestDescription || ''
    resourcePacksPath.value = s.resourcePacksPath || ''
    theme.value = s.theme || 'dark'

    detectedPaths.value = await DetectMinecraftPaths()

    if (resourcePacksPath.value) {
      const match = detectedPaths.value.find(p => p.path === resourcePacksPath.value)
      if (match) {
        selectedMinecraftPath.value = resourcePacksPath.value
        isCustomPath.value = false
      } else {
        isCustomPath.value = true
        customMinecraftPath.value = resourcePacksPath.value
      }
    } else {
      if (detectedPaths.value.length > 0) {
        selectedMinecraftPath.value = detectedPaths.value[0].path
      }
      isCustomPath.value = false
    }
  } catch (e) {
    console.error('Failed to load settings:', e)
  }
}

async function persistSettings() {
  try {
    let finalPath = ''
    if (isCustomPath.value) {
      finalPath = customMinecraftPath.value
    } else {
      finalPath = selectedMinecraftPath.value
    }

    const data = {
      autoImport: autoImport.value,
      autoOpenFolder: autoOpenFolder.value,
      deleteOriginals: deleteOriginals.value,
      deleteMcpack: deleteMcpack.value,
      customOutputDir: customOutputDir.value,
      manifestDescription: manifestDescription.value,
      resourcePacksPath: finalPath,
      theme: theme.value,
    }
    await SaveSettings(data)

    document.documentElement.dataset.theme = theme.value
    emit('settings-changed')
  } catch (e) {
    console.error('Failed to save settings:', e)
  }
}

function onThemeChange() {
  persistSettings()
}

function onMinecraftPathChange() {
  if (selectedMinecraftPath.value === '__custom__') {
    isCustomPath.value = true
  } else {
    isCustomPath.value = false
    customMinecraftPath.value = ''
  }
  persistSettings()
}

async function pickMinecraftDir() {
  try {
    const dir = await SelectDirectory()
    if (dir) {
      customMinecraftPath.value = dir
      persistSettings()
    }
  } catch (e) {
    console.error('Directory picker failed:', e)
  }
}

async function pickOutputDir() {
  try {
    const dir = await SelectDirectory()
    if (dir) {
      customOutputDir.value = dir
      persistSettings()
    }
  } catch (e) {
    console.error('Directory picker failed:', e)
  }
}

function clearOutputDir() {
  customOutputDir.value = ''
  persistSettings()
}

async function clearCache() {
  const result = await Swal.fire({
    title: 'Clear Cache?',
    text: 'This will delete recent packs history and temporary files. Settings will not be affected.',
    icon: 'warning',
    showCancelButton: true,
    confirmButtonText: 'Clear',
    cancelButtonText: 'Cancel',
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn', cancelButton: 'custom-cancel-btn' },
    buttonsStyling: false,
  })
  if (!result.isConfirmed) return
  try {
    await ClearCache()
    await loadSettings()
    Swal.fire({ title: 'Cleared', text: 'Cache cleared successfully.', icon: 'success', timer: 1500, showConfirmButton: false, customClass: { popup: 'swal-custom-popup' }, buttonsStyling: false })
  } catch (e) {
    console.error('Failed to clear cache:', e)
  }
}

onMounted(loadSettings)
</script>

<template>
  <div class="page active-page settings-page">
    <div class="settings-card">
      <h2 class="card-title"><i class="fa fa-gear"></i> Settings</h2>

      <div class="setting-group">
        <div class="setting-row setting-row-block">
          <div class="setting-info">
            <span class="setting-label">Theme</span>
            <span class="setting-desc">Choose between dark and light appearance</span>
          </div>
          <select v-model="theme" class="select-input" @change="onThemeChange">
            <option value="dark">Dark</option>
            <option value="light">Light</option>
          </select>
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <span class="setting-label">Auto-Import to Bedrock</span>
            <span class="setting-desc">Automatically import ported packs into Minecraft's resource_packs folder</span>
          </div>
          <label class="toggle">
            <input type="checkbox" v-model="autoImport" @change="persistSettings" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div v-if="autoImport" class="setting-row setting-row-block">
          <div class="setting-info">
            <span class="setting-label">Minecraft Install Path</span>
            <span class="setting-desc">Where to import packs. Detected paths are shown first.</span>
          </div>

          <select v-if="!isCustomPath" v-model="selectedMinecraftPath" class="select-input" @change="onMinecraftPathChange">
            <option v-for="p in detectedPaths" :key="p.path" :value="p.path">{{ p.name }}</option>
            <option value="__custom__">Custom path...</option>
          </select>

          <div v-if="isCustomPath" class="dir-picker">
            <span class="dir-path" v-if="customMinecraftPath">{{ customMinecraftPath }}</span>
            <span class="dir-path dir-default" v-else>No path selected</span>
            <div class="dir-buttons">
              <button class="btn-sm btn-main" @click="pickMinecraftDir"><i class="fa fa-folder-open"></i> Browse</button>
              <button class="btn-sm btn-cancel" @click="isCustomPath = false; selectedMinecraftPath = detectedPaths.length > 0 ? detectedPaths[0].path : ''; persistSettings()">Back</button>
            </div>
          </div>
        </div>

        <div v-if="autoImport" class="setting-row">
          <div class="setting-info">
            <span class="setting-label">Delete .mcpack</span>
            <span class="setting-desc">Delete the .mcpack file after importing so it doesn't show up in the output</span>
          </div>
          <label class="toggle">
            <input type="checkbox" v-model="deleteMcpack" @change="persistSettings" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <span class="setting-label">Open Folder After Export</span>
            <span class="setting-desc">Automatically open the output folder when porting is complete</span>
          </div>
          <label class="toggle">
            <input type="checkbox" v-model="autoOpenFolder" @change="persistSettings" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <span class="setting-label">Delete Originals</span>
            <span class="setting-desc">Output packs directly instead of creating Java/ and Bedrock/ folders</span>
          </div>
          <label class="toggle">
            <input type="checkbox" v-model="deleteOriginals" @change="persistSettings" />
            <span class="toggle-slider"></span>
          </label>
        </div>

        <div class="setting-row setting-row-block">
          <div class="setting-info">
            <span class="setting-label">Custom Output Directory</span>
            <span class="setting-desc">Where Bedrock/ and Java/ folders are created. Default: app directory</span>
          </div>
          <div class="dir-picker">
            <span class="dir-path" v-if="customOutputDir">{{ customOutputDir }}</span>
            <span class="dir-path dir-default" v-else>Default (app directory)</span>
            <div class="dir-buttons">
              <button class="btn-sm btn-main" @click="pickOutputDir"><i class="fa fa-folder-open"></i> Browse</button>
              <button v-if="customOutputDir" class="btn-sm btn-cancel" @click="clearOutputDir">Clear</button>
            </div>
          </div>
        </div>

        <div class="setting-row setting-row-block">
          <div class="setting-info">
            <span class="setting-label">Manifest Description</span>
            <span class="setting-desc">Custom description injected into all ported pack manifests</span>
          </div>
          <input
            v-model="manifestDescription"
            type="text"
            placeholder="Leave empty to skip"
            class="text-input"
            @change="persistSettings"
          />
        </div>

        <div class="setting-row">
          <div class="setting-info">
            <span class="setting-label">Clear Cache</span>
            <span class="setting-desc">Delete recent packs history and temporary files</span>
          </div>
          <button class="btn-sm btn-cancel" @click="clearCache" style="flex-shrink: 0;">
            <i class="fa fa-trash-can"></i> Clear Cache
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-page {
  justify-content: safe center;
  align-items: center;
}

.settings-card {
  border: 1px solid var(--border-default);
  background: var(--bg-body);
  padding: 1.5rem;
  width: 100%;
  max-width: 520px;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 1.25rem;
  color: var(--accent);
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.setting-group {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.setting-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 0;
  border-bottom: 1px solid var(--border-subtle);
}

.setting-row:last-child {
  border-bottom: none;
}

.setting-row-block {
  flex-direction: column;
  align-items: flex-start;
  gap: 0.6rem;
}

.setting-info {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.setting-label {
  font-size: 0.875rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.setting-desc {
  font-size: 0.75rem;
  color: var(--text-dim);
}

.toggle {
  position: relative;
  display: inline-block;
  width: 40px;
  height: 22px;
  flex-shrink: 0;
}

.toggle input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  cursor: pointer;
  top: 0; left: 0; right: 0; bottom: 0;
  background: var(--bg-hover-5);
  border: 1px solid var(--border-focus);
  border-radius: 22px;
}

.toggle-slider::before {
  content: '';
  position: absolute;
  width: 16px;
  height: 16px;
  left: 2px;
  bottom: 2px;
  background: var(--text-dim);
  border-radius: 50%;
}

.toggle input:checked + .toggle-slider {
  background: var(--accent-glow2);
  border-color: var(--accent);
}

.toggle input:checked + .toggle-slider::before {
  transform: translateX(18px);
  background: var(--accent);
}

.select-input {
  width: 100%;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  color: var(--text-secondary);
  font-size: 0.8rem;
  padding: 0.45rem 0.65rem;
  border-radius: 4px;
  outline: none;
}

.select-input:focus {
  border-color: var(--accent);
}

.select-input option {
  background: var(--bg-input);
  color: var(--text-secondary);
}

.dir-picker {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
}

.dir-path {
  font-size: 0.8rem;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 260px;
}

.dir-default {
  color: var(--text-dim);
  font-style: italic;
}

.dir-buttons {
  display: flex;
  gap: 0.4rem;
  flex-shrink: 0;
}

.btn-sm {
  padding: 0.3rem 0.6rem;
  font-size: 0.75rem;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 0.3rem;
}

.btn-main {
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
}

.btn-main:hover {
  background: var(--accent-glow);
}

.btn-cancel {
  background: var(--bg-hover-2);
  color: var(--text-muted);
  border: 1px solid var(--border-strong);
}

.btn-cancel:hover {
  background: var(--bg-hover-5);
}

.text-input {
  width: 100%;
  background: var(--bg-input);
  border: 1px solid var(--border-strong);
  color: var(--text-secondary);
  font-size: 0.8rem;
  padding: 0.45rem 0.65rem;
  border-radius: 4px;
  outline: none;
}

.text-input:focus {
  border-color: var(--accent);
}

.text-input::placeholder {
  color: var(--text-faint);
}
</style>
