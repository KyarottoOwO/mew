<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import Sidebar from './components/layout/Sidebar.vue'
import HomePage from './components/pages/HomePage.vue'
import PackPorter from './components/tools/PackPorter.vue'
import FolderPorter from './components/tools/FolderPorter.vue'
import RecolorTool from './components/tools/RecolorTool.vue'
import AnimatedInventory from './components/tools/AnimatedInventory.vue'
import RecolorEditor from './components/tools/RecolorEditor.vue'
import InfoPage from './components/pages/InfoPage.vue'
import SettingsPage from './components/pages/SettingsPage.vue'
import ContactPage from './components/pages/ContactPage.vue'
import RecentPacksPage from './components/pages/RecentPacksPage.vue'
import SkyConverter from './components/tools/SkyConverter.vue'
import PackViewer from './components/tools/PackViewer.vue'
import ManifestTool from './components/tools/ManifestTool.vue'
import ProgressNotification from './components/shared/ProgressNotification.vue'
import { GetSettings, SaveSettings, CheckForUpdate, OpenDownloadLink, SetDiscordActivity } from '../wailsjs/go/main/App'
import { updaterState, bindUpdaterEvents, unbindUpdaterEvents, downloadUpdate, installUpdate } from './utils/updater'

const currentPage = ref('home')
const recolorSource = ref(null)
const packViewRequest = ref(null)
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
  recolorSource.value = data
  currentPage.value = 'recoloreditor'
}

function closeDisplay() {
  const data = recolorSource.value
  const target = data && data.dirName
  recolorSource.value = null
  if (target && data.kind === 'installed') {
    packViewRequest.value = { name: target, ts: Date.now() }
    currentPage.value = 'packviewer'
  } else {
    currentPage.value = 'recolor'
  }
}

function dismissUpdate() {
  if (updaterState.state === 'downloading') return
  updateDismissed.value = true
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
  bindUpdaterEvents()
  syncSettingsToBackend()
  checkForUpdates()
  SetDiscordActivity('Browsing MEW', 'Minecraft Bedrock Texture Pack Manager')
  window.addEventListener('wheel', blockCtrlZoom, { passive: false })
})

onUnmounted(() => {
  unbindUpdaterEvents()
  window.removeEventListener('wheel', blockCtrlZoom)
})

function blockCtrlZoom(e) {
  if (e.ctrlKey || e.metaKey) e.preventDefault()
}
</script>

<template>
  <div class="app-layout">
    <div v-if="updateAvailable && !updateDismissed" class="update-banner">
      <i class="fa fa-download update-icon"></i>

      <template v-if="updaterState.state === 'done'">
        <span class="update-text">v{{ latestVersion }} downloaded</span>
        <button class="update-btn update-install" @click="installUpdate(updaterState.path)"><i class="fa fa-play"></i> Launch new version</button>
      </template>

      <template v-else-if="updaterState.state === 'error'">
        <span class="update-text">Update failed &mdash; {{ updaterState.error }}</span>
        <button class="update-btn" @click="downloadUpdate(downloadURL)"><i class="fa fa-rotate-right"></i> Retry</button>
        <button class="update-btn" @click="OpenDownloadLink(downloadURL)">View on GitHub</button>
      </template>

      <template v-else>
        <span class="update-text">A new version (<strong>v{{ latestVersion }}</strong>) is available!</span>
        <div v-if="updaterState.state === 'downloading'" class="update-progress">
          <div class="update-track"><div class="update-fill" :style="{ width: updaterState.percent + '%' }"></div></div>
          <span class="update-pct">{{ updaterState.total > 0 ? updaterState.percent + '%' : '...' }}</span>
        </div>
        <button v-else-if="downloadURL" class="update-btn" @click="downloadUpdate(downloadURL)"><i class="fa fa-download"></i> Download</button>
        <button v-else class="update-btn" @click="OpenDownloadLink('https://github.com/KyarottoOwO/mew/releases')">View on GitHub</button>
      </template>

      <button class="update-dismiss" @click="dismissUpdate"><i class="fa fa-xmark"></i></button>
    </div>

    <div class="app-main">
      <Sidebar :current-page="currentPage" @navigate="switchPage" @width-change="onSidebarWidthChange" />
      <HomePage v-if="currentPage === 'home'" :home-animated="homeAnimated" @navigate="switchPage" @animated="onHomeAnimated" />
      <PackPorter v-show="currentPage === 'packporter'" :active="currentPage === 'packporter'" />
      <FolderPorter v-show="currentPage === 'packFolderPorter'" :active="currentPage === 'packFolderPorter'" />
      <RecolorTool v-show="currentPage === 'recolor'" :active="currentPage === 'recolor'" @open-display="openDisplay" />
      <AnimatedInventory v-show="currentPage === 'animator'" :active="currentPage === 'animator'" />
      <template v-if="recolorSource">
        <RecolorEditor v-show="currentPage === 'recoloreditor'" :visible="currentPage === 'recoloreditor'" :source="recolorSource" :sidebar-width="sidebarWidth" @close="closeDisplay" />
      </template>
      <RecentPacksPage v-if="currentPage === 'recentpacks'" />
      <SkyConverter v-show="currentPage === 'skyconverter'" :active="currentPage === 'skyconverter'" />
      <PackViewer v-show="currentPage === 'packviewer'" :active="currentPage === 'packviewer'" :open-pack-req="packViewRequest" />
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
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-shrink: 0;
}

.update-btn:hover {
  background: var(--accent-glow);
}

.update-btn + .update-btn {
  margin-left: 0;
}

.update-install {
  background: var(--accent);
  color: #0e1420;
  border-color: var(--accent);
  font-weight: 600;
}

.update-install:hover {
  background: var(--accent-light);
  border-color: var(--accent-light);
  color: #0e1420;
}

.update-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.update-progress {
  margin-left: auto;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 180px;
  max-width: 320px;
  flex-shrink: 1;
}

.update-track {
  flex: 1;
  height: 6px;
  background: var(--bg-hover-3);
  border-radius: 999px;
  overflow: hidden;
}

.update-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 999px;
  transition: width 0.15s ease;
}

.update-pct {
  font-size: 0.72rem;
  color: var(--text-dim);
  width: 3ch;
  text-align: right;
  flex-shrink: 0;
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
