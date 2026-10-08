import init, { Editor } from './jsonui_editor.js'

// The wasm-bindgen module is a process-wide singleton, and several components
// use it at once (the standalone renderer stays mounted while a pack panel
// opens). Initialising it twice concurrently trips wasm-bindgen's re-entrancy
// guard ("recursive use of an object detected"), so the first call starts the
// load and everyone else awaits that same promise.
let pending = null

export function initJsonUi() {
  if (!pending) {
    pending = init(new URL('./jsonui_editor_bg.wasm', import.meta.url).href).catch((err) => {
      // Allow a later retry rather than caching the failure forever.
      pending = null
      throw err
    })
  }
  return pending
}

export { Editor }