<script setup>
import { ref, onMounted } from 'vue'

const emit = defineEmits(['navigate'])
const showSuite = ref(false)

onMounted(() => {
  setTimeout(() => {
    showSuite.value = true
  }, 2500)
})
</script>

<template>
  <div class="home-page">
    <h1 v-if="!showSuite" class="welcome-text fade-in">Welcome to SRM</h1>

    <div v-if="showSuite" class="suite-content fade-in">
      <h1 class="text-4xl font-bold mb-4" style="color: #A90B3C;">Pack Tools Suite</h1>
      <p class="text-neutral-400 mb-10">All the tools you need to handle Minecraft Bedrock texture packs in one place.</p>

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
        <h3 class="text-xl font-semibold mb-2">About SRM</h3>
        <p class="text-neutral-400 text-sm mb-4">SRM helps you work with resource packs faster by automating common tasks and keeping things simple.</p>
        <div class="flex justify-around">
          <div>
            <span class="font-bold" style="color: #A90B3C;">Fast</span>
            <p class="text-neutral-400 text-sm">Quick and smooth performance.</p>
          </div>
          <div>
            <span class="font-bold" style="color: #A90B3C;">User-Friendly</span>
            <p class="text-neutral-400 text-sm">Simple and accessible for all.</p>
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
  color: #A90B3C;
  animation: fade-in 1s ease-in;
}

.suite-content {
  text-align: center;
  transition: opacity 0.7s ease;
}

.home-card {
  border: 1px solid hsl(0, 0%, 15%);
  background: hsl(0, 0%, 4%);
  padding: 1.5rem;
  width: 280px;
  cursor: pointer;
  transition: all 0.2s;
}

.home-card:hover {
  border-color: #A90B3C;
  background: hsl(0, 0%, 6%);
}

.home-card-header {
  font-weight: 600;
  margin-bottom: 0.5rem;
  color: #A90B3C;
}

.home-card-text {
  font-size: 0.875rem;
  color: hsl(0, 0%, 50%);
}

.about-box {
  border: 1px solid hsl(0, 0%, 15%);
  padding: 1.5rem;
  max-width: 600px;
  margin: 0 auto;
}

.fade-in {
  animation: fadeIn 1s ease-in forwards;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}
</style>
