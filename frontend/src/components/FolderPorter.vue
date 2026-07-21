<script setup>
import { ref, onMounted, onUnmounted } from 'vue'
import { PortFolder, CancelPortFolder } from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

const url = ref('')
const showProgress = ref(false)
const progressList = ref([])
const progressStatus = ref('Connecting...')
const progressPercent = ref('0%')
const progressCount = ref('0 / 0 packs')
const progressWidth = ref(0)
const spinnerClass = ref('fp-spinner')
const totalPacks = ref(0)

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

function fpAddEntry(name, state) {
  const existing = progressList.value.find(e => e.name === name)
  if (existing) {
    existing.state = state
    return
  }
  progressList.value.push({ name, state })
}

function fpUpdateBar(completed, total) {
  const pct = total > 0 ? Math.round((completed / total) * 100) : 0
  progressWidth.value = pct
  progressPercent.value = total > 0 ? pct + '%' : ''
  progressCount.value = total > 0 ? completed + ' / ' + total + ' packs' : 'Processing...'
}

function resetProgress() {
  progressList.value = []
  progressWidth.value = 0
  progressPercent.value = '0%'
  progressCount.value = '0 / 0 packs'
  progressStatus.value = 'Connecting...'
  spinnerClass.value = 'fp-spinner'
}

let progressHandler = null

onMounted(() => {
  progressHandler = EventsOn('progress', (data) => {
    const t = parseInt(data.total) || 0
    const c = parseInt(data.completed) || 0

    if (t > 0) totalPacks.value = t

    progressStatus.value = (data.title || '') + ': ' + (data.message || '')

    if (data.icon === 'success') {
      spinnerClass.value = 'fp-spinner done'
      progressWidth.value = 100
      progressPercent.value = '100%'
      fpUpdateBar(totalPacks.value, totalPacks.value)
      setTimeout(() => {
        popup('All Done!', data.message, 'success')
        showProgress.value = false
        url.value = ''
      }, 600)
    } else if (data.icon === 'error') {
      spinnerClass.value = 'fp-spinner fail'
      setTimeout(() => {
        popup('Error', data.message, 'error')
        showProgress.value = false
      }, 400)
    } else if (data.icon === 'warning') {
      spinnerClass.value = 'fp-spinner fail'
      fpUpdateBar(totalPacks.value, totalPacks.value)
      setTimeout(() => {
        popup('Cancelled', data.message, 'warning')
        showProgress.value = false
      }, 400)
    } else if (data.icon === 'info') {
      if (t > 0) fpUpdateBar(c, t)
    } else if (data.icon === 'done' || data.icon === 'fail') {
      if (t > 0) fpUpdateBar(c, t)
      if (data.fileName) {
        fpAddEntry(data.fileName, data.icon === 'done' ? 'done' : 'fail')
      }
    }
  })
})

onUnmounted(() => {
  if (progressHandler) EventsOff('progress')
})

async function confirmPort() {
  const trimmedUrl = url.value.trim()
  if (!trimmedUrl) {
    popup('Error', 'Please paste a link first.', 'error')
    return
  }

  showProgress.value = true
  resetProgress()

  try {
    await PortFolder(trimmedUrl)
  } catch (err) {
    popup('Error', err.toString(), 'error')
    showProgress.value = false
  }
}

async function cancelPorter() {
  try {
    await CancelPortFolder()
  } catch (_) {}
  showProgress.value = false
  url.value = ''
}
</script>

<template>
  <div class="page active-page folderporter-page">
    <div class="porter-card">
      <h2 class="card-title">Pack Folder Porter</h2>
      <p class="card-desc">Paste a MediaFire link to a zip containing multiple packs. All packs inside will be ported at once.</p>

      <div v-if="!showProgress">
        <div class="mb-4">
          <input v-model="url" type="text" placeholder="https://www.mediafire.com/file/..."
                 class="url-input" />
        </div>
        <div class="flex items-center gap-2 mt-6">
          <button class="btn-cancel" @click="cancelPorter">Cancel</button>
          <button class="btn-main" @click="confirmPort">Confirm</button>
        </div>
      </div>

      <div v-else>
        <div class="flex items-center gap-3 mb-3">
          <div :class="spinnerClass"></div>
          <div class="text-sm text-neutral-300">{{ progressStatus }}</div>
        </div>

        <div class="progress-track">
          <div class="progress-bar" :style="{ width: progressWidth + '%' }"></div>
        </div>

        <div class="flex justify-between text-xs text-neutral-500 mb-3">
          <span>{{ progressPercent }}</span>
          <span>{{ progressCount }}</span>
        </div>

        <div class="progress-list">
          <div v-for="entry in progressList" :key="entry.name"
               :class="['fp-entry', entry.state === 'active' ? 'active' : '']"
               :style="{ color: entry.state === 'done' ? '#22c55e' : entry.state === 'fail' ? '#ef4444' : '' }">
            <div class="fp-entry-icon">
              <div v-if="entry.state === 'active'" class="fp-mini-spinner"></div>
              <span v-else-if="entry.state === 'done'" class="fp-check">&#10003;</span>
              <span v-else-if="entry.state === 'fail'" class="fp-x">&#10007;</span>
              <span v-else class="fp-dash">&#8212;</span>
            </div>
            <span>{{ entry.name }}</span>
          </div>
        </div>

        <div class="flex justify-end mt-3">
          <button class="btn-cancel text-xs" @click="cancelPorter">Cancel</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.folderporter-page {
  justify-content: center;
  align-items: center;
}

.porter-card {
  border: 1px solid hsl(0, 0%, 10%);
  background: hsl(0, 0%, 2%);
  padding: 1.5rem;
  width: 100%;
  max-width: 480px;
}

.card-title {
  font-size: 1.125rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: #e879a8;
}

.card-desc {
  font-size: 0.875rem;
  color: hsl(0, 0%, 50%);
  margin-bottom: 1rem;
}

.url-input {
  width: 100%;
  background: hsl(0, 0%, 5%);
  border: 1px solid hsl(0, 0%, 25%);
  color: hsl(0, 0%, 80%);
  font-size: 0.875rem;
  padding: 0.5rem 0.75rem;
  border-radius: 4px;
  outline: none;
}

.url-input:focus {
  border-color: #e879a8;
}

.progress-track {
  width: 100%;
  background: hsl(0, 0%, 10%);
  border-radius: 999px;
  height: 8px;
  margin-bottom: 1rem;
  overflow: hidden;
}

.progress-bar {
  height: 100%;
  background: #e879a8;
  border-radius: 999px;
  transition: width 0.3s ease;
}

.progress-list {
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid hsl(0, 0%, 10%);
  border-radius: 4px;
  padding: 0.5rem;
}

.fp-entry {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.25rem 0;
  font-size: 0.75rem;
  color: hsl(0, 0%, 60%);
}

.fp-entry-icon {
  width: 16px;
  text-align: center;
  flex-shrink: 0;
}

.fp-mini-spinner {
  width: 12px;
  height: 12px;
  border: 2px solid hsl(0, 0%, 30%);
  border-top-color: #e879a8;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

.fp-spinner {
  width: 20px;
  height: 20px;
  border: 3px solid hsl(0, 0%, 20%);
  border-top-color: #e879a8;
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
  flex-shrink: 0;
}

.fp-spinner.done { border-color: #22c55e; border-top-color: #22c55e; }
.fp-spinner.fail { border-color: #ef4444; border-top-color: #ef4444; animation: none; }

@keyframes spin {
  to { transform: rotate(360deg); }
}

.btn-cancel {
  padding: 0.5rem 1rem;
  background: hsl(0, 0%, 10%);
  color: hsl(0, 0%, 60%);
  border: 1px solid hsl(0, 0%, 20%);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}

.btn-main {
  padding: 0.5rem 1rem;
  background: #e879a8;
  color: white;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.875rem;
}
</style>
