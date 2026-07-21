<script setup>
import { ref } from 'vue'

const emit = defineEmits(['close'])
</script>

<template>
  <div class="page active-page display-overlay">
    <div class="display-topbar">
      <button class="back-btn" @click="emit('close')">
        <i class="fa fa-arrow-left"></i> Back
      </button>
      <div class="display-title">Recolor Tool</div>
      <div></div>
    </div>
    <div class="display-content">
      <p class="text-neutral-400">Folder display will render here with texture grid and hue controls.</p>
    </div>
  </div>
</template>

<style scoped>
.display-overlay {
  position: fixed;
  top: 0;
  left: 64px;
  right: 0;
  bottom: 0;
  background: hsl(0, 0%, 2%);
  z-index: 20;
  flex-direction: column;
}

.display-topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid hsl(0, 0%, 10%);
  background: hsl(0, 0%, 3%);
}

.back-btn {
  background: none;
  border: 1px solid hsl(0, 0%, 20%);
  color: hsl(0, 0%, 60%);
  padding: 0.4rem 0.8rem;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.back-btn:hover {
  background: hsl(0, 0%, 10%);
}

.display-title {
  font-weight: 600;
  color: #A90B3C;
}

.display-content {
  flex: 1;
  padding: 1rem;
  overflow-y: auto;
}
</style>
