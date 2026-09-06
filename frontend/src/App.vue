<script setup>
import { ref, onMounted } from 'vue'
import Sidebar from './components/layout/Sidebar.vue'
import HomePage from './components/pages/HomePage.vue'
import PackPorter from './components/tools/PackPorter.vue'
import FolderPorter from './components/tools/FolderPorter.vue'
import RecolorTool from './components/tools/RecolorTool.vue'
import AnimatedInventory from './components/tools/AnimatedInventory.vue'
import FolderDisplay from './components/shared/FolderDisplay.vue'
import InfoPage from './components/pages/InfoPage.vue'
import SettingsPage from './components/pages/SettingsPage.vue'
import ContactPage from './components/pages/ContactPage.vue'
import RecentPacksPage from './components/pages/RecentPacksPage.vue'
import SkyConverter from './components/tools/SkyConverter.vue'
import PackViewer from './components/tools/PackViewer.vue'
import ManifestTool from './components/tools/ManifestTool.vue'
import ProgressNotification from './components/shared/ProgressNotification.vue'
import { GetSettings, SaveSettings, CheckForUpdate, OpenDownloadLink, SetDiscordActivity } from '../wailsjs/go/main/App'

const currentPage = ref('home')
const showRecolorPage = ref(false)
const checkResult = ref(null)
const packName = ref('')
const sidebarWidth = ref(64)

const homeAnimated = ref(localStorage.getItem('mew_home_animated') === '1')
const updateAvailable = ref(false)
const latestVersion = ref('')
const downloadURL = ref('')
const updateDismissed = ref(false)

function onHomeAnimated() {
  homeAnimated.value = true
  localStorage.setItem('mew_home_animated', '1')
}

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
    await SaveSettings(s)
    const theme = s.theme || 'dark'
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
  SetDiscordActivity('Browsing MEW', 'Minecraft Bedrock Texture Pack Manager')
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
      <HomePage v-if="currentPage === 'home'" :home-animated="homeAnimated" @navigate="switchPage" @animated="onHomeAnimated" />
      <PackPorter v-show="currentPage === 'packporter'" :active="currentPage === 'packporter'" />
      <FolderPorter v-show="currentPage === 'packFolderPorter'" :active="currentPage === 'packFolderPorter'" />
      <RecolorTool v-show="currentPage === 'recolor'" @open-display="openDisplay" />
      <AnimatedInventory v-show="currentPage === 'animator'" :active="currentPage === 'animator'" />
      <FolderDisplay v-if="showRecolorPage" :check-result="checkResult" :pack-name="packName" :sidebar-width="sidebarWidth" @close="closeDisplay" />
      <RecentPacksPage v-if="currentPage === 'recentpacks'" />
      <SkyConverter v-show="currentPage === 'skyconverter'" :active="currentPage === 'skyconverter'" />
      <PackViewer v-show="currentPage === 'packviewer'" :active="currentPage === 'packviewer'" />
      <ManifestTool v-show="currentPage === 'manifest'" :active="currentPage === 'manifest'" />
      <InfoPage v-show="currentPage === 'info'" />
      <SettingsPage v-show="currentPage === 'settings'" />
      <ContactPage v-show="currentPage === 'contact'" />
    </div>

    <ProgressNotification :current-page="currentPage" @navigate="switchPage" />
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
  overflow-y: auto;
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
