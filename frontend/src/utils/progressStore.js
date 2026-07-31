import { reactive } from 'vue'

export const progressStore = reactive({
  source: null,
  active: false,
  done: false,
  title: '',
  message: '',
  icon: '',
  percent: 0,
  total: 0,
  completed: 0,
  fileName: '',
  packList: [],
  finishTimer: null
})

function resetStore() {
  if (progressStore.finishTimer) {
    clearTimeout(progressStore.finishTimer)
    progressStore.finishTimer = null
  }
  progressStore.source = null
  progressStore.active = false
  progressStore.done = false
  progressStore.title = ''
  progressStore.message = ''
  progressStore.icon = ''
  progressStore.percent = 0
  progressStore.total = 0
  progressStore.completed = 0
  progressStore.fileName = ''
  progressStore.packList = []
}

export function startPort(source, initialStatus = 'Starting...') {
  resetStore()
  progressStore.source = source
  progressStore.active = true
  progressStore.message = initialStatus
}

export function clearProgress() {
  resetStore()
}

export function updateFromEvent(data) {
  const t = parseInt(data.total) || 0
  const c = parseInt(data.completed) || 0
  if (data.title !== undefined) progressStore.title = data.title || ''
  if (data.message !== undefined) progressStore.message = data.message || ''
  if (data.icon !== undefined) progressStore.icon = data.icon || ''
  if (data.fileName !== undefined) progressStore.fileName = data.fileName || ''
  if (t > 0) progressStore.total = t
  if (c > 0) progressStore.completed = c
  if (t > 0) progressStore.percent = Math.round((c / t) * 100)

  if ((data.icon === 'done' || data.icon === 'fail') && data.fileName) {
    const existing = progressStore.packList.find(e => e.name === data.fileName)
    if (existing) {
      existing.state = data.icon === 'done' ? 'done' : 'fail'
    } else {
      progressStore.packList.push({ name: data.fileName, state: data.icon === 'done' ? 'done' : 'fail' })
    }
  }
}

export function finish(data) {
  if (data) {
    if (data.title !== undefined) progressStore.title = data.title || ''
    if (data.message !== undefined) progressStore.message = data.message || ''
    if (data.icon !== undefined) progressStore.icon = data.icon || ''
    if (data.fileName !== undefined) progressStore.fileName = data.fileName || ''
  }
  progressStore.done = true
  if (progressStore.icon === 'success') {
    progressStore.percent = 100
    progressStore.completed = progressStore.total
  }
  if (progressStore.finishTimer) clearTimeout(progressStore.finishTimer)
  progressStore.finishTimer = setTimeout(() => {
    resetStore()
  }, 4000)
}
