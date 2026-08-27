<script setup>
import { ref, computed, watch } from 'vue'
import { progressStore } from '../../utils/progressStore'
import { CancelPortFolder } from '../../../wailsjs/go/main/App'

const props = defineProps({ currentPage: String })
const emit = defineEmits(['navigate'])

const dismissed = ref(false)

watch(() => progressStore.active, (active) => {
  if (active) dismissed.value = false
})

watch(() => props.currentPage, (page) => {
  if (page === progressStore.source) dismissed.value = false
})

const show = computed(() => {
  if (dismissed.value || !progressStore.active) return false
  return progressStore.source !== props.currentPage
})

const status = computed(() => {
  const title = progressStore.title || ''
  const msg = progressStore.message || ''
  if (progressStore.done) {
    if (progressStore.icon === 'error') return 'Error: ' + msg
    if (progressStore.icon === 'warning') return msg
    return (title || 'Done') + (msg ? ': ' + msg : '')
  }
  return title + (msg ? ': ' + msg : '')
})

const spinnerClass = computed(() => {
  if (!progressStore.done) return 'pn-spinner'
  return progressStore.icon === 'error' || progressStore.icon === 'warning'
    ? 'pn-spinner fail'
    : 'pn-spinner done'
})

const showCancel = computed(() => progressStore.source === 'packFolderPorter' && !progressStore.done)

const title = computed(() => {
  switch (progressStore.source) {
    case 'packFolderPorter': return 'Multi-Pack Porter'
    case 'skyconverter': return 'Sky Converter'
    case 'animator': return 'Animated Inventory'
    default: return 'Pack Porter'
  }
})

function goToPage() {
  emit('navigate', progressStore.source)
}

async function cancel() {
  try {
    await CancelPortFolder()
  } catch (e) {
    console.error('Failed to cancel port:', e)
  }
}
</script>

<template>
  <transition name="pn">
    <div v-if="show" class="progress-notification" @click="goToPage">
      <div class="pn-head">
        <div :class="spinnerClass"></div>
        <span class="pn-title">{{ title }}</span>
        <button class="pn-close" @click.stop="dismissed = true"><i class="fa fa-xmark"></i></button>
      </div>

      <div class="pn-status">{{ status }}</div>

      <div class="pn-track">
        <div class="pn-bar" :style="{ width: progressStore.percent + '%' }"></div>
      </div>

      <div class="pn-foot">
        <span>{{ progressStore.percent }}%</span>
        <span v-if="progressStore.source === 'packFolderPorter' && progressStore.total > 0">
          {{ progressStore.completed }} / {{ progressStore.total }} packs
        </span>
      </div>

      <div v-if="progressStore.packList.length" class="pn-list">
        <div v-for="entry in progressStore.packList" :key="entry.name" class="pn-entry"
             :style="{ color: entry.state === 'done' ? '#22c55e' : entry.state === 'fail' ? '#ef4444' : '' }">
          <span v-if="entry.state === 'done'" class="pn-check">&#10003;</span>
          <span v-else-if="entry.state === 'fail'" class="pn-x">&#10007;</span>
          <span v-else class="pn-dash">&#8212;</span>
          <span class="pn-entry-name">{{ entry.name }}</span>
        </div>
      </div>

      <button v-if="showCancel" class="pn-cancel" @click.stop="cancel">Cancel</button>
    </div>
  </transition>
</template>

<style scoped>
.progress-notification {
  position: fixed;
  bottom: 1rem;
  right: 1rem;
  width: 320px;
  max-width: calc(100vw - 2rem);
  background: var(--bg-surface);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  padding: 0.85rem 1rem;
  box-shadow: 0 6px 24px rgba(0, 0, 0, 0.35);
  cursor: pointer;
  z-index: 1000;
}

.pn-head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.35rem;
}

.pn-title {
  font-weight: 600;
  font-size: 0.85rem;
  color: var(--accent);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pn-close {
  background: none;
  border: none;
  color: var(--text-dim);
  cursor: pointer;
  font-size: 0.8rem;
  padding: 0.15rem;
  flex-shrink: 0;
}

.pn-close:hover {
  color: var(--text-secondary);
}

.pn-status {
  font-size: 0.75rem;
  color: var(--text-secondary);
  margin-bottom: 0.5rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pn-track {
  width: 100%;
  background: var(--bg-hover-2);
  border-radius: 999px;
  height: 6px;
  overflow: hidden;
}

.pn-bar {
  height: 100%;
  background: var(--accent);
  border-radius: 999px;
  transition: width 0.3s ease;
}

.pn-foot {
  display: flex;
  justify-content: space-between;
  font-size: 0.7rem;
  color: var(--text-dim);
  margin-top: 0.35rem;
}

.pn-list {
  max-height: 110px;
  overflow-y: auto;
  border: 1px solid var(--border-default);
  border-radius: 4px;
  padding: 0.35rem 0.5rem;
  margin-top: 0.5rem;
}

.pn-entry {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.15rem 0;
  font-size: 0.7rem;
  color: var(--text-muted);
}

.pn-entry-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pn-check, .pn-x, .pn-dash {
  width: 12px;
  text-align: center;
  flex-shrink: 0;
}

.pn-cancel {
  margin-top: 0.6rem;
  padding: 0.35rem 0.75rem;
  background: var(--bg-hover-2);
  color: var(--text-muted);
  border: 1px solid var(--border-strong);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.75rem;
}

.pn-cancel:hover {
  background: var(--bg-hover-5);
}

.pn-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid var(--border-strong);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.pn-spinner.done {
  border-color: #22c55e;
  border-top-color: #22c55e;
  animation: none;
}

.pn-spinner.fail {
  border-color: #ef4444;
  border-top-color: #ef4444;
  animation: none;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.pn-enter-active, .pn-leave-active {
  transition: opacity 0.25s ease, transform 0.25s ease;
}

.pn-enter-from, .pn-leave-to {
  opacity: 0;
  transform: translateX(20px);
}
</style>
