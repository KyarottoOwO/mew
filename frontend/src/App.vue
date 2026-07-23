<script setup>
import { ref } from 'vue'
import Sidebar from './components/Sidebar.vue'
import HomePage from './components/HomePage.vue'
import PackPorter from './components/PackPorter.vue'
import FolderPorter from './components/FolderPorter.vue'
import RecolorTool from './components/RecolorTool.vue'
import FolderDisplay from './components/FolderDisplay.vue'
import InfoPage from './components/InfoPage.vue'

const currentPage = ref('home')
const showRecolorPage = ref(false)
const checkResult = ref(null)
const packName = ref('')
const sidebarWidth = ref(64)

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
</script>

<template>
  <div class="app-layout">
    <Sidebar :current-page="currentPage" @navigate="switchPage" @width-change="onSidebarWidthChange" />
    <HomePage v-show="currentPage === 'home'" @navigate="switchPage" />
    <PackPorter v-show="currentPage === 'packporter'" />
    <FolderPorter v-show="currentPage === 'packFolderPorter'" />
    <RecolorTool v-show="currentPage === 'recolor'" @open-display="openDisplay" />
    <FolderDisplay v-if="showRecolorPage" :check-result="checkResult" :pack-name="packName" :sidebar-width="sidebarWidth" @close="closeDisplay" />
    <InfoPage v-show="currentPage === 'info'" />
  </div>
</template>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  background-color: hsl(0, 0%, 2%);
  color: white;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  overflow: hidden;
}

.app-layout {
  display: flex;
  height: 100vh;
  width: 100vw;
}

.app-layout > :not(.sidebar) {
  flex: 1;
  min-width: 0;
  min-height: 0;
}

.page {
  display: flex;
  flex-direction: column;
  height: 100%;
}
</style>
