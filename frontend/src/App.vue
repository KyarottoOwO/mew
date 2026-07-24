<script setup>
import { ref, onMounted } from 'vue'
import Sidebar from './components/Sidebar.vue'
import HomePage from './components/HomePage.vue'
import PackPorter from './components/PackPorter.vue'
import FolderPorter from './components/FolderPorter.vue'
import RecolorTool from './components/RecolorTool.vue'
import FolderDisplay from './components/FolderDisplay.vue'
import InfoPage from './components/InfoPage.vue'
import SettingsPage from './components/SettingsPage.vue'
import ContactPage from './components/ContactPage.vue'
import { GetSettings, SaveSettings, CheckForUpdate, OpenDownloadLink } from '../wailsjs/go/main/App'

const currentPage = ref('home')
const showRecolorPage = ref(false)
const checkResult = ref(null)
const packName = ref('')
const sidebarWidth = ref(64)

const updateAvailable = ref(false)
const latestVersion = ref('')
const downloadURL = ref('')
const updateDismissed = ref(false)

function switchPage(page) {
  currentPage.value = page
}

function onSidebarWidthChange(w) {
  sidebarWidth.value = w
}

function openDisplay(data) {
  checkResult.value = data.folders
  packName.value = data.packName
  showRecolorPage.value = true
}

function closeDisplay() {
  showRecolorPage.value = false
  checkResult.value = null
  packName.value = ''
}

function dismissUpdate() {
  updateDismissed.value = true
}

async function downloadUpdate() {
  try {
    await OpenDownloadLink(downloadURL.value)
  } catch (e) {
    console.error('Failed to open download link:', e)
  }
}

async function syncSettingsToBackend() {
  try {
    const s = await GetSettings()
    const ls = JSON.parse(localStorage.getItem('mew_settings') || '{}')
    const merged = { ...s, ...ls }
    await SaveSettings(merged)

    const theme = merged.theme || 'dark'
    document.documentElement.dataset.theme = theme
  } catch (e) {
    console.error('Failed to sync settings:', e)
  }
}

async function checkForUpdates() {
  try {
    const info = await CheckForUpdate()
    if (info.needsUpdate) {
      updateAvailable.value = true
      latestVersion.value = info.latestVersion
      downloadURL.value = info.downloadUrl
    }
  } catch (e) {
    console.error('Update check failed:', e)
  }
}

onMounted(() => {
  syncSettingsToBackend()
  checkForUpdates()
})
</script>

<template>
  <div class="app-layout">
    <div v-if="updateAvailable && !updateDismissed" class="update-banner">
      <i class="fa fa-download update-icon"></i>
      <span>A new version (<strong>v{{ latestVersion }}</strong>) is available!</span>
      <button class="update-btn" @click="downloadUpdate">Download</button>
      <button class="update-dismiss" @click="dismissUpdate"><i class="fa fa-xmark"></i></button>
    </div>

    <div class="app-main">
      <Sidebar :current-page="currentPage" @navigate="switchPage" @width-change="onSidebarWidthChange" />
      <HomePage v-show="currentPage === 'home'" @navigate="switchPage" />
      <PackPorter v-show="currentPage === 'packporter'" />
      <FolderPorter v-show="currentPage === 'packFolderPorter'" />
      <RecolorTool v-show="currentPage === 'recolor'" @open-display="openDisplay" />
      <FolderDisplay v-if="showRecolorPage" :check-result="checkResult" :pack-name="packName" :sidebar-width="sidebarWidth" @close="closeDisplay" />
      <InfoPage v-show="currentPage === 'info'" />
      <SettingsPage v-show="currentPage === 'settings'" />
      <ContactPage v-show="currentPage === 'contact'" />
    </div>
  </div>
</template>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  background-color: var(--bg-body);
  color: var(--text-primary);
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  overflow: hidden;
}

.app-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  width: 100vw;
}

.app-main {
  display: flex;
  flex: 1;
  min-height: 0;
}

.app-main > :not(.sidebar) {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.update-banner {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  padding: 0.5rem 1rem;
  background: var(--accent-glow2);
  border-bottom: 1px solid var(--accent-border);
  color: var(--text-secondary);
  font-size: 0.8rem;
  flex-shrink: 0;
  z-index: 20;
}

.update-icon {
  color: var(--accent);
}

.update-banner strong {
  color: var(--accent);
}

.update-btn {
  margin-left: auto;
  padding: 0.25rem 0.75rem;
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
}

.update-btn:hover {
  background: var(--accent-glow);
}

.update-dismiss {
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 0.85rem;
  padding: 0.2rem;
}

.update-dismiss:hover {
  color: var(--text-secondary);
}
</style>
