<script setup>
import { ref, computed, watch } from 'vue'
import { ApplyManifestToDir, ApplyManifestToPack, ApplyManifestToUpload, CheckPack, CreatePackFromManifest, GetInstalledManifestPacks, GetManifestFromUpload, GetPackCache, GetPackCacheList, GetPackManifest, GetPackManifestFromDir, NewManifestTemplate, RegenerateUuid, DeleteSession } from '../../../wailsjs/go/main/App'
import { ClipboardSetText } from '../../../wailsjs/runtime/runtime'

const props = defineProps({ active: Boolean })

const packs = ref([])
const loading = ref(false)
const saving = ref(false)
const sel = ref(null)
const sourceText = ref('')
const manifest = ref(null)
const isNew = ref(false)
const advanced = ref(false)
const rawJson = ref('')

const sourceTab = ref('installed')
const mtQuery = ref('')

const cacheSources = ref([])
const cacheLoading = ref(false)
const cacheLoaded = ref(false)

const uploads = ref([])
const uploadSeq = ref(0)
const uploadFile = ref(null)
const uploadInfo = ref('')
const showUploadInfo = ref(false)
const uploadChecking = ref(false)
const uploadDropZone = ref(null)

const formName = ref('')
const formDesc = ref('')
const formHeaderUuid = ref('')
const formVersion = ref('')
const formMinEngine = ref('')
const formModules = ref([])

const MODULE_TYPES = ['resources', 'data', 'client_data', 'interface', 'world_template', 'skin_pack', 'entity']

function clone(o) {
  return JSON.parse(JSON.stringify(o))
}

function arrayToText(arr) {
  if (!Array.isArray(arr)) return ''
  return arr.map(x => (typeof x === 'number' ? (Number.isInteger(x) ? x : Math.round(x)) : x)).join('.')
}

function parseVersion(str) {
  const parts = String(str || '').trim().split('.')
  const nums = parts.map(p => {
    const n = parseInt(p, 10)
    return isNaN(n) ? 0 : n
  })
  return nums.length ? nums : [1, 0, 0]
}

function newModuleRow() {
  return { type: 'resources', description: '', uuid: '', version: '1.0.0' }
}

async function newUuid() {
  try {
    return await RegenerateUuid()
  } catch {
    if (typeof crypto !== 'undefined' && crypto.randomUUID) return crypto.randomUUID()
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, c => {
      const r = Math.random() * 16 | 0
      return (c === 'x' ? r : (r & 0x3 | 0x8)).toString(16)
    })
  }
}

function popup(title, text, icon) {
  Swal.mixin({
    customClass: { popup: 'swal-custom-popup', confirmButton: 'custom-confirm-btn' },
    buttonsStyling: false
  }).fire({ title, text, icon, confirmButtonText: 'OK' })
}

function hydrateForm() {
  const m = manifest.value || {}
  const h = m.header && typeof m.header === 'object' ? m.header : {}
  formName.value = h.name || ''
  formDesc.value = h.description || ''
  formHeaderUuid.value = h.uuid || ''
  formVersion.value = arrayToText(h.version) || '1.0.0'
  formMinEngine.value = arrayToText(h.min_engine_version) || '1.12.1'
  const mods = Array.isArray(m.modules) ? m.modules : []
  formModules.value = mods.map(mod => ({
    type: (mod && mod.type) || 'resources',
    description: (mod && mod.description) || '',
    uuid: (mod && mod.uuid) || '',
    version: arrayToText(mod && mod.version) || '1.0.0'
  }))
  if (!formModules.value.length) formModules.value = [newModuleRow()]
}

function buildEditedManifest(m) {
  const withVal = m || {}
  if (withVal.format_version == null) withVal.format_version = 1
  const h = withVal.header && typeof withVal.header === 'object' ? withVal.header : {}
  withVal.header = h
  h.name = formName.value.trim()
  h.description = formDesc.value
  h.uuid = formHeaderUuid.value.trim()
  h.version = parseVersion(formVersion.value)
  h.min_engine_version = parseVersion(formMinEngine.value)
  const existing = Array.isArray(withVal.modules) ? withVal.modules : []
  const rows = formModules.value
  const keepBase = existing.length === rows.length
  withVal.modules = rows.map((r, i) => {
    const base = keepBase && existing[i] && typeof existing[i] === 'object' ? existing[i] : {}
    base.type = r.type || 'resources'
    if (r.description) base.description = r.description
    else if (keepBase) delete base.description
    base.uuid = r.uuid.trim()
    base.version = parseVersion(r.version)
    return base
  }).filter(mod => mod.uuid)
  return withVal
}

const previewJson = computed(() => {
  if (!manifest.value) return ''
  try {
    return JSON.stringify(buildEditedManifest(clone(manifest.value)), null, 2)
  } catch {
    return ''
  }
})

function syncRawJson() {
  try {
    rawJson.value = previewJson.value
  } catch {
    rawJson.value = ''
  }
}

async function regenUuidsInObj(obj) {
  if (!obj) return obj
  if (obj.header && typeof obj.header === 'object') {
    obj.header.uuid = await newUuid()
  }
  if (Array.isArray(obj.modules)) {
    for (const mod of obj.modules) {
      if (mod && typeof mod === 'object') mod.uuid = await newUuid()
    }
  }
  return obj
}

function toggleAdvanced() {
  if (advanced.value) {
    try {
      const parsed = JSON.parse(rawJson.value)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('not an object')
      manifest.value = parsed
      hydrateForm()
      advanced.value = false
    } catch (e) {
      popup('Invalid JSON', 'Fix the JSON before switching back to the form.', 'error')
    }
  } else {
    syncRawJson()
    advanced.value = true
  }
}

async function loadPacks() {
  try {
    packs.value = (await GetInstalledManifestPacks()) || []
  } catch (e) {
    console.error('[ManifestTool] load packs error:', e)
    packs.value = []
  }
}

function selKey(it) {
  if (it.kind === 'upload') return 'upload::' + (it.uid || it.name)
  return it.kind + '::' + it.name + '::' + (it.basePath || '')
}

function isSel(it) {
  return sel.value && selKey(sel.value) === selKey(it)
}

function matchesQuery(p) {
  const q = mtQuery.value.trim().toLowerCase()
  if (!q) return true
  return (p.name || '').toLowerCase().includes(q) ||
         (p.dirName || '').toLowerCase().includes(q) ||
         (p.description || '').toLowerCase().includes(q)
}

const filteredInstalled = computed(() => packs.value.filter(matchesQuery))

const filteredCacheSources = computed(() =>
  cacheSources.value
    .map(src => ({ ...src, packs: src.packs.filter(matchesQuery) }))
    .filter(src => src.packs.length > 0)
)

async function loadCache(force = false) {
  if (cacheLoaded.value && !force) return
  cacheLoading.value = true
  try {
    const srcs = (await GetPackCache()) || []
    const detailed = []
    for (const src of srcs) {
      let list = []
      try {
        list = (await GetPackCacheList(src.path)) || []
      } catch (e) {
        list = []
      }
      detailed.push({ ...src, packs: list })
    }
    cacheSources.value = detailed
    cacheLoaded.value = true
  } catch (err) {
    console.error('[ManifestTool] GetPackCache error:', err)
  }
  cacheLoading.value = false
}

function onUploadChange(e) {
  handleUpload(e.target.files[0])
}

function onUploadDrop(e) {
  e.preventDefault()
  uploadDropZone.value?.classList.remove('drag-over')
  handleUpload(e.dataTransfer.files[0])
}

function onUploadDragOver(e) {
  e.preventDefault()
  uploadDropZone.value?.classList.add('drag-over')
}

function onUploadDragLeave() {
  uploadDropZone.value?.classList.remove('drag-over')
}

function handleUpload(selectedFile) {
  if (!selectedFile) return
  if (!/\.mcpack$/i.test(selectedFile.name)) {
    popup('Error', 'Must be a .mcpack file.', 'error')
    return
  }
  const kb = (selectedFile.size / 1024).toFixed(1)
  uploadInfo.value = kb + ' KB'
  uploadFile.value = selectedFile
  showUploadInfo.value = true
}

async function addUpload() {
  if (!uploadFile.value) {
    popup('Error', 'Upload a .mcpack first.', 'error')
    return
  }
  uploadChecking.value = true
  try {
    const buffer = await uploadFile.value.arrayBuffer()
    const bytes = Array.from(new Uint8Array(buffer))
    const result = await CheckPack(bytes, uploadFile.value.name)
    // we only needed the validation result, so don't keep a session dir around
    if (result.sessionId) { try { await DeleteSession(result.sessionId) } catch (_) {} }
    if (!result.valid) {
      popup('Error', result.errorMsg || 'Invalid pack', 'error')
      return
    }
    const u = { uid: (Date.now().toString(36) + '-' + uploadSeq.value++), name: uploadFile.value.name, file: uploadFile.value }
    uploads.value.push(u)
    uploadFile.value = null
    showUploadInfo.value = false
    uploadInfo.value = ''
    await openItem({ kind: 'upload', name: u.name, uid: u.uid, file: u.file, label: u.name + ' (uploaded .mcpack)' })
  } catch (err) {
    popup('Error', err.toString(), 'error')
  } finally {
    uploadChecking.value = false
  }
}

function removeUpload(i, ev) {
  if (ev) ev.stopPropagation()
  const u = uploads.value[i]
  uploads.value.splice(i, 1)
  if (u && sel.value && selKey(sel.value) === selKey({ kind: 'upload', name: u.name, uid: u.uid })) {
    sel.value = null
    manifest.value = null
    sourceText.value = ''
  }
}

async function openItem(item) {
  sel.value = item
  loading.value = true
  try {
    let m = null
    if (item.kind === 'cache') {
      m = await GetPackManifestFromDir(item.basePath + '/' + item.name)
    } else if (item.kind === 'upload') {
      const bytes = Array.from(new Uint8Array(await item.file.arrayBuffer()))
      m = await GetManifestFromUpload(bytes)
    } else {
      m = await GetPackManifest(item.name)
    }
    manifest.value = clone(m)
    isNew.value = false
    hydrateForm()
    sourceText.value = item.label || item.name
  } catch (e) {
    if (item.kind === 'installed') {
      const t = await NewManifestTemplate()
      manifest.value = t
      isNew.value = true
      hydrateForm()
      formName.value = item.displayName || item.name
      sourceText.value = (item.label || item.name) + ' (new manifest)'
    } else if (item.kind === 'cache') {
      const t = await NewManifestTemplate()
      manifest.value = t
      isNew.value = false
      hydrateForm()
      formName.value = item.displayName || item.name
      sourceText.value = (item.label || item.name) + ' (no manifest yet)'
    } else {
      popup('Read failed', (e && e.message) || String(e), 'error')
      manifest.value = null
      sel.value = null
    }
  }
  if (advanced.value) syncRawJson()
  loading.value = false
}

async function makeNew() {
  sel.value = { kind: 'installed', name: '', label: 'New pack', displayName: 'New Pack' }
  isNew.value = true
  const t = await NewManifestTemplate()
  manifest.value = t
  hydrateForm()
  sourceText.value = 'New pack (will be created from this manifest)'
  if (advanced.value) syncRawJson()
}

async function regenHeader() {
  formHeaderUuid.value = await newUuid()
}

async function regenModule(i) {
  formModules.value[i].uuid = await newUuid()
}

async function regenerateAll() {
  if (advanced.value) {
    try {
      const parsed = JSON.parse(rawJson.value)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('not an object')
      await regenUuidsInObj(parsed)
      rawJson.value = JSON.stringify(parsed, null, 2)
      popup('UUIDs regenerated', 'All header and module UUIDs were replaced.', 'success')
    } catch (e) {
      popup('Invalid JSON', 'Fix the JSON before regenerating UUIDs.', 'error')
    }
    return
  }
  formHeaderUuid.value = await newUuid()
  for (let i = 0; i < formModules.value.length; i++) {
    formModules.value[i].uuid = await newUuid()
  }
  popup('UUIDs regenerated', 'All header and module UUIDs were replaced.', 'success')
}

async function addModule() {
  const row = newModuleRow()
  row.uuid = await newUuid()
  formModules.value.push(row)
}

function removeModule(i) {
  if (formModules.value.length > 1) {
    formModules.value.splice(i, 1)
  } else {
    formModules.value[0] = newModuleRow()
  }
}

async function save() {
  if (!manifest.value && !advanced.value) return
  const swal = Swal.mixin({
    customClass: {
      popup: 'swal-custom-popup',
      confirmButton: 'custom-confirm-btn',
      cancelButton: 'custom-cancel-btn'
    },
    buttonsStyling: false
  })
  let edited
  if (advanced.value) {
    try {
      const parsed = JSON.parse(rawJson.value)
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) throw new Error('not an object')
      edited = parsed
    } catch (e) {
      popup('Invalid JSON', 'The manifest.json text is not valid JSON.', 'error')
      return
    }
  } else {
    if (!formName.value.trim()) {
      popup('Missing name', 'Pack name is required.', 'error')
      return
    }
    if (!formHeaderUuid.value.trim()) {
      popup('Missing header UUID', 'Regenerate or enter a header UUID first.', 'error')
      return
    }
    edited = buildEditedManifest(clone(manifest.value))
  }

  if (isNew.value) {
    const name = (edited.header && edited.header.name) || ''
    if (!name.trim()) {
      popup('Missing name', 'Pack name is required.', 'error')
      return
    }
    const c = await swal.fire({
      title: 'Make a Pack?',
      text: `Create a new pack "${name}" from this manifest?`,
      icon: 'question',
      showCancelButton: true,
      confirmButtonText: 'Make a Pack',
      cancelButtonText: 'Cancel'
    })
    if (!c.isConfirmed) return
    saving.value = true
    try {
      const dirName = await CreatePackFromManifest(edited)
      manifest.value = clone(edited)
      isNew.value = false
      advanced.value = false
      hydrateForm()
      sel.value = { kind: 'installed', name: dirName, label: dirName, displayName: name }
      sourceText.value = dirName
      await loadPacks()
      swal.fire({ title: 'Pack created', text: `Created "${name}" in your resource packs folder.`, icon: 'success', confirmButtonText: 'OK' })
    } catch (err) {
      console.error('[ManifestTool] create pack error:', err)
      const msg = err && err.message ? err.message : String(err)
      swal.fire({ title: 'Failed to create pack', text: msg, icon: 'error', confirmButtonText: 'OK' })
    }
    saving.value = false
    return
  }

  if (!sel.value) {
    swal.fire({ title: 'No target pack', text: 'Select a pack from the list to save this manifest into.', icon: 'info', confirmButtonText: 'OK' })
    return
  }

  const targetLabel = sel.value.label || sel.value.name
  let applyMsg = ''
  if (sel.value.kind === 'cache') {
    applyMsg = `Overwrite manifest.json in the cache pack "${targetLabel}"? The old file will be backed up as manifest.json.bak.`
  } else if (sel.value.kind === 'upload') {
    applyMsg = `Apply the manifest to "${targetLabel}"? A new pack will be exported to your output folder.`
  } else {
    applyMsg = `Overwrite manifest.json in "${targetLabel}"? The old file will be backed up as manifest.json.bak.`
  }
  const c = await swal.fire({
    title: 'Apply manifest?',
    text: applyMsg,
    icon: 'question',
    showCancelButton: true,
    confirmButtonText: 'Apply',
    cancelButtonText: 'Cancel'
  })
  if (!c.isConfirmed) return
  saving.value = true
  try {
    if (sel.value.kind === 'cache') {
      await ApplyManifestToDir(sel.value.basePath + '/' + sel.value.name, edited)
    } else if (sel.value.kind === 'upload') {
      const outPath = await ApplyManifestToUpload(edited, sel.value.name)
      swal.fire({ title: 'Pack exported', text: 'Saved to ' + outPath, icon: 'success', confirmButtonText: 'OK' })
    } else {
      await ApplyManifestToPack(sel.value.name, edited)
    }
    manifest.value = clone(edited)
    isNew.value = false
    advanced.value = false
    hydrateForm()
    sourceText.value = sel.value.label || sel.value.name
    await loadPacks()
    if (sel.value.kind !== 'upload') {
      swal.fire({ title: 'Manifest applied', text: `Saved to ${targetLabel}`, icon: 'success', confirmButtonText: 'OK' })
    }
  } catch (err) {
    console.error('[ManifestTool] apply error:', err)
    const msg = err && err.message ? err.message : String(err)
    swal.fire({ title: 'Failed to apply', text: msg, icon: 'error', confirmButtonText: 'OK' })
  }
  saving.value = false
}

async function copyManifest() {
  const text = advanced.value ? rawJson.value : previewJson.value
  if (!text) {
    popup('Nothing to copy', 'Edit a manifest first.', 'info')
    return
  }
  try {
    await ClipboardSetText(text)
    popup('Copied', 'manifest.json content copied to clipboard.', 'success')
  } catch (e) {
    console.error('[ManifestTool] copy error:', e)
    popup('Copy failed', String(e), 'error')
  }
}

watch(() => props.active, (val) => {
  if (val) {
    loadPacks()
    loadCache()
  }
})
</script>

<template>
  <div class="mt-page page">
    <div class="mt-toolbar">
      <div class="mt-toolbar-title">
        <i class="fa fa-file-lines"></i>
        <span>Manifest Manager</span>
        <span class="mt-count">{{ packs.length }} pack{{ packs.length === 1 ? '' : 's' }}</span>
      </div>
      <button class="mt-btn mt-btn-primary" :disabled="saving" @click="makeNew">
        <i class="fa fa-plus"></i> Make New Manifest
      </button>
    </div>

    <div class="mt-body">
      <div class="mt-list">
        <div class="mt-source-tabs">
          <button class="mt-source-tab" :class="{ active: sourceTab === 'installed' }" @click="sourceTab = 'installed'">
            Installed
          </button>
          <button class="mt-source-tab" :class="{ active: sourceTab === 'cache' }" @click="sourceTab = 'cache'">
            Pack Cache
          </button>
          <button class="mt-source-tab" :class="{ active: sourceTab === 'upload' }" @click="sourceTab = 'upload'">
            Upload
          </button>
        </div>

        <div v-if="sourceTab !== 'upload'" class="mt-list-filter">
          <i class="fa fa-magnifying-glass"></i>
          <input v-model="mtQuery" class="mt-list-search" type="text" placeholder="Search packs..." />
          <button v-if="mtQuery" class="mt-list-clear" @click="mtQuery = ''" title="Clear search">&times;</button>
        </div>

        <div class="mt-list-body">
          <template v-if="sourceTab === 'installed'">
            <div v-if="loading" class="mt-empty">Loading packs...</div>
            <div v-else-if="!filteredInstalled.length" class="mt-empty">
              {{ mtQuery ? 'No packs match "' + mtQuery + '".' : 'No installed packs found.' }}
            </div>
            <div v-else class="mt-items">
              <div
                v-for="p in filteredInstalled"
                :key="p.dirName"
                class="mt-item"
                :class="{ active: isSel({ kind: 'installed', name: p.dirName }) }"
                @click="openItem({ kind: 'installed', name: p.dirName, label: p.dirName, displayName: p.name })"
              >
                <img v-if="p.iconURI" :src="p.iconURI" class="mt-item-icon" alt="" />
                <div v-else class="mt-item-icon mt-item-ph"><i class="fa fa-box"></i></div>
                <div class="mt-item-info">
                  <span class="mt-item-name">{{ p.name }}</span>
                  <span v-if="p.description" class="mt-item-desc">{{ p.description }}</span>
                  <span class="mt-item-meta">
                    <span v-if="p.hasManifest" class="mt-badge">
                      {{ p.moduleType || 'manifest' }}<template v-if="p.packVersion"> · {{ p.packVersion }}</template>
                    </span>
                    <span v-else class="mt-badge mt-badge-warn">no manifest</span>
                  </span>
                </div>
              </div>
            </div>
          </template>

          <template v-else-if="sourceTab === 'cache'">
            <div v-if="cacheLoading" class="mt-empty">
              <i class="fa fa-spinner fa-spin"></i> Scanning pack cache...
            </div>
            <div v-else-if="filteredCacheSources.length === 0" class="mt-empty">
              {{ mtQuery ? 'No packs match "' + mtQuery + '".' : 'No Minecraft pack cache found.' }}
            </div>
            <div v-else class="mt-cache-sources">
              <div v-for="src in filteredCacheSources" :key="src.path" class="mt-cache-source">
                <div class="mt-cache-source-head">
                  <span class="mt-cache-source-name">{{ src.name }}</span>
                  <span class="mt-cache-source-path" :title="src.path">{{ src.path }}</span>
                </div>
                <div class="mt-items">
                  <div
                    v-for="p in src.packs"
                    :key="'c-' + src.path + '-' + p.dirName"
                    class="mt-item"
                    :class="{ active: isSel({ kind: 'cache', name: p.dirName, basePath: src.path }) }"
                    @click="openItem({ kind: 'cache', name: p.dirName, basePath: src.path, label: src.name + ' / ' + p.name, displayName: p.name })"
                  >
                    <img v-if="p.iconURI" :src="p.iconURI" class="mt-item-icon" alt="" />
                    <div v-else class="mt-item-icon mt-item-ph"><i class="fa fa-box"></i></div>
                    <div class="mt-item-info">
                      <span class="mt-item-name">{{ p.name }}</span>
                      <span v-if="p.description" class="mt-item-desc">{{ p.description }}</span>
                      <span class="mt-item-meta">
                        <span class="mt-badge">cache</span>
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </template>

          <template v-else>
            <p class="mt-list-hint">Upload an .mcpack to read and edit its manifest. Saving exports a new <code>&lt;name&gt;-updated.mcpack</code> to your output folder.</p>
            <label ref="uploadDropZone" class="mt-upload-zone"
                   @drop="onUploadDrop" @dragover="onUploadDragOver" @dragleave="onUploadDragLeave">
              <input type="file" accept=".mcpack" class="hidden" @change="onUploadChange" />
              <div v-if="!showUploadInfo" class="mt-upload-inner">
                <i class="fa fa-cloud-arrow-up"></i>
                <span class="mt-upload-hint">Drop a .mcpack here or click to upload</span>
              </div>
              <div v-else class="mt-upload-info">
                <span class="mt-upload-name">{{ uploadFile.name }}</span>
                <span class="mt-upload-size">{{ uploadInfo }}</span>
              </div>
            </label>
            <button class="mt-btn mt-btn-primary mt-upload-confirm" :disabled="uploadChecking || !uploadFile" @click="addUpload">
              {{ uploadChecking ? 'Checking...' : 'Open Uploaded Pack' }}
            </button>
            <div v-if="uploads.length" class="mt-items">
              <div
                v-for="(u, i) in uploads"
                :key="'u-' + u.uid"
                class="mt-item"
                :class="{ active: isSel({ kind: 'upload', name: u.name, uid: u.uid }) }"
                @click="openItem({ kind: 'upload', name: u.name, uid: u.uid, file: u.file, label: u.name + ' (uploaded .mcpack)' })"
              >
                <div class="mt-item-icon mt-item-ph"><i class="fa fa-box"></i></div>
                <div class="mt-item-info">
                  <span class="mt-item-name">{{ u.name }}</span>
                  <span class="mt-item-meta"><span class="mt-badge">upload</span></span>
                </div>
                <button class="mt-icon-btn mt-remove" title="Remove" @click="removeUpload(i, $event)"><i class="fa fa-trash"></i></button>
              </div>
            </div>
          </template>
        </div>
      </div>

      <div class="mt-editor">
        <div v-if="!manifest" class="mt-empty mt-editor-empty">
          <i class="fa fa-id-card"></i>
          <p>Select a pack to view or edit its manifest, or click "Make New Manifest".</p>
        </div>
        <div v-else>
          <div class="mt-editor-head">
            <div>
              <div class="mt-editor-title">Manifest</div>
              <div class="mt-editor-sub">{{ sourceText }}</div>
            </div>
            <div class="mt-editor-actions">
              <button class="mt-btn" :class="{ 'mt-btn-toggle-on': advanced }" @click="toggleAdvanced" :title="advanced ? 'Switch to form mode' : 'Switch to raw JSON editor'">
                <i class="fa" :class="advanced ? 'fa-sliders' : 'fa-code'"></i> Advanced
              </button>
              <button class="mt-btn" @click="regenerateAll" :disabled="advanced && !rawJson"><i class="fa fa-dice"></i> Regenerate UUIDs</button>
              <button class="mt-btn" @click="copyManifest"><i class="fa fa-copy"></i> Copy</button>
              <button class="mt-btn mt-btn-primary" :disabled="saving" @click="save">
                <i class="fa fa-floppy-disk"></i> {{ isNew ? 'Make a Pack' : (sel && sel.kind === 'upload' ? 'Export Pack' : 'Save to Pack') }}
              </button>
            </div>
          </div>

          <div v-if="!advanced" class="mt-form">
            <div class="mt-field">
              <label>Pack Name <em>*</em></label>
              <input v-model="formName" class="mt-input" placeholder="Pack name" />
            </div>
            <div class="mt-field">
              <label>Description</label>
              <textarea v-model="formDesc" class="mt-input mt-textarea" rows="2" placeholder="Pack description"></textarea>
            </div>
            <div class="mt-field-row">
              <div class="mt-field mt-field-uuid">
                <label>Header UUID</label>
                <div class="mt-input-group">
                  <input v-model="formHeaderUuid" class="mt-input" spellcheck="false" />
                  <button class="mt-icon-btn" title="Regenerate UUID" @click="regenHeader"><i class="fa fa-dice"></i></button>
                </div>
              </div>
              <div class="mt-field mt-field-sm">
                <label>Version</label>
                <input v-model="formVersion" class="mt-input" placeholder="1.0.0" />
              </div>
              <div class="mt-field mt-field-sm">
                <label>Min Engine</label>
                <input v-model="formMinEngine" class="mt-input" placeholder="1.12.1" />
              </div>
            </div>

            <div class="mt-modules">
              <div class="mt-modules-head">
                <span>Modules</span>
                <button class="mt-btn" @click="addModule"><i class="fa fa-plus"></i> Add</button>
              </div>
              <div v-for="(mod, i) in formModules" :key="i" class="mt-module">
                <div class="mt-field mt-field-sm">
                  <label>Type</label>
                  <select v-model="mod.type" class="mt-input">
                    <option v-for="t in MODULE_TYPES" :key="t" :value="t">{{ t }}</option>
                  </select>
                </div>
                <div class="mt-field mt-field-module-desc">
                  <label>Description</label>
                  <input v-model="mod.description" class="mt-input" placeholder="Module description" />
                </div>
                <div class="mt-field mt-field-module-uuid">
                  <label>UUID</label>
                  <div class="mt-input-group">
                    <input v-model="mod.uuid" class="mt-input" spellcheck="false" />
                    <button class="mt-icon-btn" title="Regenerate UUID" @click="regenModule(i)"><i class="fa fa-dice"></i></button>
                  </div>
                </div>
                <div class="mt-field mt-field-sm">
                  <label>Version</label>
                  <input v-model="mod.version" class="mt-input" />
                </div>
                <button class="mt-icon-btn mt-remove" title="Remove module" @click="removeModule(i)"><i class="fa fa-trash"></i></button>
              </div>
            </div>
          </div>

          <div v-else class="mt-json-editor-wrap">
            <div class="mt-json-editor-title">manifest.json <span>edit as JSON</span></div>
            <textarea v-model="rawJson" class="mt-json-editor" spellcheck="false"></textarea>
          </div>

          <div v-if="!advanced" class="mt-preview">
            <div class="mt-preview-title">manifest.json preview</div>
            <pre class="mt-pre">{{ previewJson }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mt-page {
  padding: 1.2rem 1.4rem;
  gap: 1rem;
  display: flex;
  flex-direction: column;
}

.mt-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-shrink: 0;
}

.mt-toolbar-title {
  display: flex;
  align-items: center;
  gap: 0.6rem;
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-primary);
}

.mt-toolbar-title .fa {
  color: var(--accent);
}

.mt-count {
  font-size: 0.75rem;
  font-weight: 500;
  color: var(--text-dim);
  border: 1px solid var(--border-subtle);
  border-radius: 999px;
  padding: 0.15rem 0.6rem;
}

.mt-body {
  display: flex;
  flex: 1;
  min-height: 0;
  gap: 1rem;
}

.mt-list {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  background: var(--bg-surface);
}

.mt-source-tabs {
  display: flex;
  gap: 0.35rem;
  padding: 0.6rem 0.6rem 0;
  flex-shrink: 0;
}

.mt-source-tab {
  flex: 1;
  padding: 0.4rem 0.5rem;
  background: var(--bg-input);
  color: var(--text-muted);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.72rem;
  font-weight: 600;
  text-align: center;
  transition: all 0.15s;
}

.mt-source-tab:hover {
  color: var(--text-secondary);
  background: var(--bg-hover-2);
}

.mt-source-tab.active {
  background: transparent;
  color: var(--accent);
  border-color: var(--accent);
}

.mt-list-filter {
  position: relative;
  display: flex;
  align-items: center;
  padding: 0.6rem 0.6rem 0.35rem;
  flex-shrink: 0;
}

.mt-list-filter .fa-magnifying-glass {
  position: absolute;
  left: 1rem;
  font-size: 0.75rem;
  color: var(--text-faint);
}

.mt-list-search {
  width: 100%;
  padding: 0.4rem 1.9rem 0.4rem 1.6rem;
  background: var(--bg-input);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  color: var(--text-secondary);
  font-size: 0.8rem;
  outline: none;
  transition: border-color 0.15s;
}

.mt-list-search::placeholder {
  color: var(--text-faint);
}

.mt-list-search:focus {
  border-color: var(--accent);
}

.mt-list-clear {
  position: absolute;
  right: 0.95rem;
  background: none;
  border: none;
  color: var(--text-muted);
  font-size: 1rem;
  line-height: 1;
  cursor: pointer;
  padding: 0.25rem;
}

.mt-list-clear:hover {
  color: var(--text-secondary);
}

.mt-list-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.mt-list-hint {
  font-size: 0.75rem;
  color: var(--text-dim);
  line-height: 1.4;
  padding: 0.75rem 0.75rem 0.4rem;
}

.mt-list-hint code {
  color: var(--text-secondary);
  background: var(--bg-input);
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
}

.mt-cache-sources {
  display: flex;
  flex-direction: column;
}

.mt-cache-source-head {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 0.75rem 0.7rem 0.35rem;
  border-bottom: 1px solid var(--border-subtle);
}

.mt-cache-source-name {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--text-secondary);
  flex-shrink: 0;
}

.mt-cache-source-path {
  font-size: 0.62rem;
  color: var(--text-faint);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mt-upload-zone {
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2px dashed var(--border-medium);
  border-radius: 8px;
  margin: 0.75rem 0.6rem 0.5rem;
  padding: 1rem;
  cursor: pointer;
  transition: all 0.15s;
  background: var(--bg-input);
}

.mt-upload-zone:hover, .mt-upload-zone.drag-over {
  border-color: var(--accent);
  background: var(--accent-glow);
}

.mt-upload-inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
}

.mt-upload-inner .fa {
  font-size: 1.4rem;
  color: var(--text-dim);
}

.mt-upload-hint {
  font-size: 0.72rem;
  color: var(--text-dim);
  text-align: center;
}

.mt-upload-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  width: 100%;
}

.mt-upload-name {
  font-size: 0.78rem;
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mt-upload-size {
  font-size: 0.7rem;
  color: var(--text-dim);
  flex-shrink: 0;
}

.mt-upload-confirm {
  width: calc(100% - 1.2rem);
  margin: 0 0.6rem 0.75rem;
  justify-content: center;
  flex-shrink: 0;
}

.mt-items {
  display: flex;
  flex-direction: column;
}

.mt-item {
  display: flex;
  gap: 0.6rem;
  padding: 0.6rem 0.7rem;
  cursor: pointer;
  border-bottom: 1px solid var(--border-subtle);
  transition: background 0.12s;
}

.mt-item:last-child {
  border-bottom: none;
}

.mt-item:hover {
  background: var(--bg-hover-1);
}

.mt-item.active {
  background: var(--accent-active-bg);
  border-left: 3px solid var(--accent);
}

.mt-item-icon {
  width: 40px;
  height: 40px;
  flex-shrink: 0;
  object-fit: cover;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
}

.mt-item-ph {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--bg-hover-1);
  color: var(--text-dim);
}

.mt-item-info {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  min-width: 0;
}

.mt-item-name {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mt-item-desc {
  font-size: 0.72rem;
  color: var(--text-dim);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mt-item-meta {
  display: flex;
  gap: 0.3rem;
}

.mt-badge {
  font-size: 0.66rem;
  font-weight: 600;
  color: var(--accent-light);
  background: var(--accent-glow);
  border: 1px solid var(--accent-border);
  border-radius: 999px;
  padding: 0.1rem 0.5rem;
  align-self: flex-start;
}

.mt-badge-warn {
  color: #ffb454;
  background: rgba(255, 180, 84, 0.12);
  border-color: rgba(255, 180, 84, 0.4);
}

.mt-editor {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  background: var(--bg-surface);
  padding: 1rem 1.2rem;
}

.mt-editor-empty {
  height: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 0.6rem;
  color: var(--text-dim);
  text-align: center;
}

.mt-editor-empty .fa {
  font-size: 2.4rem;
  color: var(--text-dim);
}

.mt-editor-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}

.mt-editor-title {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-primary);
}

.mt-editor-sub {
  font-size: 0.72rem;
  color: var(--text-dim);
}

.mt-editor-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.mt-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.85rem;
  border-radius: 8px;
  border: 1px solid var(--border-medium);
  background: var(--bg-input);
  color: var(--text-secondary);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s;
}

.mt-btn:hover {
  background: var(--bg-hover-2);
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.mt-btn:disabled {
  opacity: 0.5;
  cursor: default;
}

.mt-btn-primary {
  background: var(--accent-glow);
  color: var(--accent-light);
  border-color: var(--accent-border);
}

.mt-btn-primary:hover {
  background: var(--accent);
  color: var(--bg-body);
}

.mt-form {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
}

.mt-field {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  min-width: 0;
}

.mt-field label {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.mt-field label em {
  color: var(--accent);
  font-style: normal;
}

.mt-field-row {
  display: flex;
  gap: 0.8rem;
  flex-wrap: wrap;
}

.mt-field-row .mt-field-uuid {
  flex: 2;
  min-width: 220px;
}

.mt-field-row .mt-field-sm {
  flex: 1;
  min-width: 120px;
}

.mt-input {
  width: 100%;
  padding: 0.5rem 0.65rem;
  border-radius: 8px;
  border: 1px solid var(--border-medium);
  background: var(--bg-input);
  color: var(--text-primary);
  font-size: 0.85rem;
  outline: none;
  transition: border-color 0.15s;
}

.mt-input:focus {
  border-color: var(--border-focus);
}

.mt-textarea {
  resize: vertical;
}

.mt-input-group {
  display: flex;
  gap: 0.4rem;
}

.mt-icon-btn {
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  border: 1px solid var(--border-medium);
  background: var(--bg-input);
  color: var(--text-dim);
  cursor: pointer;
  transition: all 0.15s;
}

.mt-icon-btn:hover {
  color: var(--text-primary);
  border-color: var(--border-focus);
}

.mt-remove:hover {
  color: #ff6b6b;
  border-color: rgba(255, 107, 107, 0.5);
}

.mt-modules {
  margin-top: 0.4rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.mt-modules-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.82rem;
  font-weight: 700;
  color: var(--text-primary);
}

.mt-module {
  display: flex;
  gap: 0.6rem;
  align-items: flex-end;
  flex-wrap: wrap;
  padding: 0.7rem 0.7rem 0.6rem;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background: var(--bg-hover-1);
}

.mt-field-sm {
  flex: 1;
  min-width: 120px;
}

.mt-field-module-desc {
  flex: 2;
  min-width: 160px;
}

.mt-field-module-uuid {
  flex: 2;
  min-width: 200px;
}

.mt-remove {
  margin-bottom: 0;
}

.mt-preview {
  margin-top: 1rem;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  overflow: hidden;
}

.mt-preview-title {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.3px;
  padding: 0.5rem 0.8rem;
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-hover-1);
}

.mt-pre {
  margin: 0;
  padding: 0.8rem;
  font-size: 0.75rem;
  line-height: 1.45;
  color: var(--text-secondary);
  overflow-x: auto;
  max-height: 300px;
  overflow-y: auto;
  white-space: pre;
}

.mt-btn-toggle-on {
  background: var(--accent-glow);
  color: var(--accent-light);
  border-color: var(--accent-border);
}

.mt-json-editor-wrap {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.mt-json-editor-title {
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.3px;
}

.mt-json-editor-title span {
  color: var(--text-dim);
  text-transform: none;
  letter-spacing: 0;
  font-weight: 400;
}

.mt-json-editor {
  width: 100%;
  min-height: 420px;
  resize: vertical;
  padding: 0.8rem 1rem;
  border-radius: 10px;
  border: 1px solid var(--border-medium);
  background: var(--bg-input);
  color: var(--text-primary);
  font-family: 'Consolas', 'Courier New', monospace;
  font-size: 0.78rem;
  line-height: 1.5;
  outline: none;
  tab-size: 2;
  white-space: pre;
  overflow-x: auto;
}

.mt-json-editor:focus {
  border-color: var(--border-focus);
}

.hidden { display: none; }
</style>