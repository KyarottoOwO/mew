<script setup>
import { ref, onMounted } from 'vue'
import { GetAllRecentPacks } from '../../../wailsjs/go/main/App'
import { EventsOn } from '../../../wailsjs/runtime/runtime'

import { parseBedrockCodes } from '../../utils/formatCodes'
const recentPacks = ref([])
const isMounted = ref(false)

async function loadPacks() {
  try {
    const packs = await GetAllRecentPacks()
    if (isMounted.value) recentPacks.value = packs || []
  } catch (e) {
    console.error('Failed to load recent packs:', e)
  }
}

function formatTimestamp(ts) {
  if (!ts) return ''
  const d = new Date(ts.replace(' ', 'T'))
  const now = new Date()
  const diff = now - d
  const dayMs = 86400000

  const options = { hour: 'numeric', minute: '2-digit', hour12: true }
  const timeStr = d.toLocaleTimeString('en-US', options)

  if (diff < dayMs && d.getDate() === now.getDate()) {
    return `Today ${timeStr}`
  }
  const yesterday = new Date(now)
  yesterday.setDate(yesterday.getDate() - 1)
  if (d.getDate() === yesterday.getDate() && d.getMonth() === yesterday.getMonth() && d.getFullYear() === yesterday.getFullYear()) {
    return `Yesterday ${timeStr}`
  }
  if (diff < 7 * dayMs) {
    const dayName = d.toLocaleDateString('en-US', { weekday: 'short' })
    return `${dayName} ${timeStr}`
  }
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' })
}

onMounted(() => {
  isMounted.value = true
  loadPacks()
  EventsOn('recent-packs-changed', () => {
    if (isMounted.value) loadPacks()
  })
})
</script>

<template>
  <div class="page active-page recentpacks-page">
    <div class="rp-content">
      <div class="rp-hero">
        <i class="fa fa-clock rp-hero-icon"></i>
        <h2 class="rp-title">Recent Packs</h2>
      </div>
      <p class="rp-desc">Recently ported packs.</p>

      <div v-if="recentPacks.length === 0" class="rp-empty">
        <i class="fa fa-box-open"></i>
        <p>No packs ported yet.</p>
      </div>

      <div v-else class="rp-list">
        <div v-for="(pack, i) in recentPacks" :key="i" class="rp-entry">
          <div class="rp-header">
            <i class="fa fa-box-open rp-icon"></i>
            <span class="rp-name" v-html="parseBedrockCodes(pack.name)"></span>
            <span class="rp-date">{{ formatTimestamp(pack.timestamp) }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.recentpacks-page {
  justify-content: flex-start;
  align-items: center;
  padding: 2rem 1rem;
  overflow-y: auto;
}

.rp-content {
  max-width: 560px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.rp-title {
  font-size: 1.125rem;
  font-weight: 600;
  color: var(--accent);
}

.rp-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 0.25rem;
}

.rp-hero-icon {
  font-size: 2rem;
  color: var(--accent);
  margin-bottom: 0.5rem;
}

.rp-desc {
  font-size: 0.875rem;
  color: var(--text-desc);
  margin-bottom: 1.5rem;
}

.rp-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2rem;
  color: var(--text-dim);
  font-size: 0.85rem;
}

.rp-empty i {
  font-size: 2rem;
  color: var(--text-faint);
}

.rp-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.rp-entry {
  background: var(--bg-hover-1);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  padding: 0.85rem 1rem;
}

.rp-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.3rem;
}

.rp-icon {
  color: var(--accent);
  font-size: 0.85rem;
  width: 18px;
  text-align: center;
  flex-shrink: 0;
}

.rp-name {
  font-weight: 600;
  font-size: 0.9rem;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: pre-line;
}

.rp-date {
  font-size: 0.75rem;
  color: var(--text-dim);
  margin-left: auto;
  flex-shrink: 0;
}


</style>
