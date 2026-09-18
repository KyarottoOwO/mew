import { reactive } from 'vue'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { DownloadUpdate, InstallUpdate, OpenFolder } from '../../wailsjs/go/main/App'

export const updaterState = reactive({
  state: 'idle',
  percent: 0,
  total: 0,
  downloaded: 0,
  path: '',
  error: '',
  busy: false
})

let onProgress = null

export function bindUpdaterEvents() {
  if (onProgress) return
  onProgress = EventsOn('updateProgress', (data) => {
    const total = parseInt(data && data.total) || 0
    const downloaded = parseInt(data && data.downloaded) || 0
    updaterState.total = total
    updaterState.downloaded = downloaded
    updaterState.percent = total > 0 ? Math.min(100, Math.round((downloaded / total) * 100)) : 0
  })
}

export function unbindUpdaterEvents() {
  if (onProgress) {
    onProgress()
    onProgress = null
  }
}

export async function downloadUpdate(url) {
  if (updaterState.busy) return
  updaterState.busy = true
  updaterState.state = 'downloading'
  updaterState.error = ''
  updaterState.path = ''
  updaterState.percent = 0
  updaterState.downloaded = 0
  updaterState.total = 0
  try {
    updaterState.path = await DownloadUpdate(url)
    updaterState.percent = 100
    updaterState.state = 'done'
  } catch (e) {
    updaterState.error = (e && e.message) || String(e) || 'Download failed'
    updaterState.state = 'error'
  } finally {
    updaterState.busy = false
  }
}

export function resetUpdater() {
  updaterState.state = 'idle'
  updaterState.percent = 0
  updaterState.downloaded = 0
  updaterState.total = 0
  updaterState.path = ''
  updaterState.error = ''
}

export async function installUpdate(path) {
  if (!path) return
  try {
    await InstallUpdate(path)
  } catch (e) {
    console.error('Failed to start installer:', e)
  }
}

export function openUpdateFolder(path) {
  if (path) OpenFolder(path)
}