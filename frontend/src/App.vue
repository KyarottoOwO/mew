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

function switchPage(page) {
  currentPage.value = page
}
</script>

<template>
  <div class="app-layout">
    <Sidebar :current-page="currentPage" @navigate="switchPage" />
    <HomePage v-show="currentPage === 'home'" @navigate="switchPage" />
    <PackPorter v-show="currentPage === 'packporter'" />
    <FolderPorter v-show="currentPage === 'packFolderPorter'" />
    <RecolorTool v-show="currentPage === 'recolor'" @open-display="showRecolorPage = true" />
    <FolderDisplay v-if="showRecolorPage" @close="showRecolorPage = false" />
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
