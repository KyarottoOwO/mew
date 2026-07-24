<script setup>
import { ref, onMounted } from 'vue'
import { GetRecentPacks, OpenFolder } from '../../wailsjs/go/main/App'

const emit = defineEmits(['navigate'])
const showSuite = ref(false)
const hasAnimated = ref(false)
const recentPacks = ref([])

async function loadRecentPacks() {
  try {
    const packs = await GetRecentPacks()
    recentPacks.value = packs || []
  } catch (e) {
    console.error('Failed to load recent packs:', e)
  }
}

async function openPackFolder(path) {
  try {
    await OpenFolder(path)
  } catch (e) {
    console.error('Failed to open folder:', e)
  }
}

onMounted(() => {
  setTimeout(() => {
    showSuite.value = true
  }, 2500)
  setTimeout(() => {
    hasAnimated.value = true
  }, 3500)
  loadRecentPacks()
})
</script>

<template>
  <div class="home-page">
    <h1 v-if="!showSuite" class="welcome-text" :class="{ 'fade-in': !hasAnimated }">Welcome to MEW</h1>

    <div v-if="showSuite" class="suite-content" :class="{ 'fade-in': !hasAnimated }">
      <h1 class="text-4xl font-bold mb-4" style="color: var(--accent);">Pack Tools Suite</h1>
      <p class="text-neutral-400 mb-10" style="color: var(--text-desc);">All the tools you need to handle Minecraft Bedrock texture packs in one place.</p>

      <div class="flex gap-8 justify-center mb-10">
        <div class="home-card" @click="emit('navigate', 'packporter')">
          <div class="home-card-header">Pack Porter</div>
          <h3 class="home-card-text">Turn Java Edition resource packs into Bedrock Edition fast and easy.</h3>
        </div>
        <div class="home-card" @click="emit('navigate', 'recolor')">
          <div class="home-card-header">Recolor Tool</div>
          <h3 class="home-card-text">Edit and recolor texture packs to your liking</h3>
        </div>
      </div>

      <div class="about-box">
        <h3 class="text-xl font-semibold mb-2" style="color: var(--accent);">About MEW</h3>
        <p class="text-sm mb-4" style="color: var(--text-desc);">MEW helps you work with resource packs faster by automating common tasks and keeping things simple.</p>
        <div class="flex justify-around">
          <div>
            <span class="font-bold" style="color: var(--accent);">Fast</span>
            <p class="text-sm" style="color: var(--text-desc);">Quick and smooth performance.</p>
          </div>
          <div>
            <span class="font-bold" style="color: var(--accent);">User-Friendly</span>
            <p class="text-sm" style="color: var(--text-desc);">Simple and accessible for all.</p>
          </div>
        </div>
      </div>

      <div v-if="recentPacks.length > 0" class="recent-box">
        <h3 class="recent-title">Recent Packs</h3>
        <div class="recent-list">
          <div v-for="(pack, i) in recentPacks" :key="i" class="recent-item" @click="openPackFolder(pack.path)">
            <div class="recent-info">
              <i class="fa fa-box-open recent-icon"></i>
              <span class="recent-name">{{ pack.name }}</span>
            </div>
            <i class="fa fa-folder-open recent-folder-icon" title="Open folder"></i>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.home-page {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 2rem;
}

.welcome-text {
  font-size: 2.25rem;
  font-weight: bold;
  color: var(--accent);
  animation: fade-in 1s ease-in;
}

.suite-content {
  text-align: center;
  transition: opacity 0.7s ease;
}

.home-card {
  border: 1px solid var(--border-medium);
  background: var(--bg-surface);
  padding: 1.5rem;
  width: 280px;
  cursor: pointer;
  transition: all 0.2s;
}

.home-card:hover {
  border-color: var(--accent);
  background: var(--bg-hover-3);
}

.home-card-header {
  font-weight: 600;
  margin-bottom: 0.5rem;
  color: var(--accent);
}

.home-card-text {
  font-size: 0.875rem;
  color: var(--text-desc);
}

.about-box {
  border: 1px solid var(--border-medium);
  padding: 1.5rem;
  max-width: 600px;
  margin: 0 auto;
}

.recent-box {
  border: 1px solid var(--border-medium);
  padding: 1rem 1.5rem;
  max-width: 600px;
  margin: 1.5rem auto 0;
  text-align: left;
}

.recent-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--accent);
  margin-bottom: 0.75rem;
}

.recent-list {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
}

.recent-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.4rem 0.5rem;
  border-radius: 4px;
  cursor: pointer;
  transition: background 0.15s;
}

.recent-item:hover {
  background: var(--bg-hover-1);
}

.recent-info {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
}

.recent-icon {
  color: var(--accent);
  font-size: 0.75rem;
  flex-shrink: 0;
}

.recent-name {
  font-size: 0.8rem;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.recent-folder-icon {
  color: var(--text-dim);
  font-size: 0.7rem;
  flex-shrink: 0;
  opacity: 0;
  transition: opacity 0.15s;
}

.recent-item:hover .recent-folder-icon {
  opacity: 1;
}

.fade-in {
  animation: fadeIn 1s ease-in forwards;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
