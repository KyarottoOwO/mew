// Built-in viewer backgrounds, used when a pack ships no sky of its own. Each
// is a gradient cube built in the browser - nothing is bundled - shaped the way
// Minecraft's skies are: zenith straight up, horizon at eye level, nadir
// straight down. A pack's own cubemap or subpack sky always takes precedence
// over this choice.
export const BACKGROUNDS = [
  { id: 'day',    name: 'Overworld Day', zenith: '#3a7cc2', horizon: '#bcd9f2', nadir: '#6f8f6a' },
  { id: 'sunset', name: 'Sunset',        zenith: '#26315e', horizon: '#f0a05a', nadir: '#6b4a52' },
  { id: 'night',  name: 'Night',         zenith: '#04060e', horizon: '#1b2b52', nadir: '#080d18', stars: true },
  { id: 'end',    name: 'The End',       zenith: '#05030a', horizon: '#2a1f3d', nadir: '#120a1e', stars: true },
  { id: 'nether', name: 'Nether',        zenith: '#2a0d0d', horizon: '#7a2b12', nadir: '#3a1008' },
  { id: 'void',   name: 'Void',          zenith: '#050508', horizon: '#0a0a12', nadir: '#000000' },
  { id: 'slate',  name: 'Slate',         solid: '#262933' },
  { id: 'paper',  name: 'Paper',         solid: '#d7dbe4' },
]

export const DEFAULT_BACKGROUND_ID = BACKGROUNDS[0].id

// The app settings key the choice is stored under.
export const BACKGROUND_SETTING_KEY = 'viewerBackground'
export const DEFAULT_SKY_PACK_KEY = 'defaultSkyPack'
export const DEFAULT_SKY_SUBPACK_KEY = 'defaultSkySubpack'

// The background choice that shows the default sky pack picked in Settings
// rather than one of the built-in gradients.
export const SKY_PACK_BACKGROUND_ID = 'skypack'

export function normalizeBackgroundId(id) {
  if (id === SKY_PACK_BACKGROUND_ID) return id
  return BACKGROUNDS.some(b => b.id === id) ? id : DEFAULT_BACKGROUND_ID
}

// The sky pack's swatch: a sky-blue tile, drawn with a cloud icon on top.
export const SKY_PACK_SWATCH_STYLE = { background: 'linear-gradient(180deg, #4b8fd6, #a9d2f5)' }

export function backgroundById(id) {
  return BACKGROUNDS.find(b => b.id === id) || BACKGROUNDS[0]
}

// The swatch shown in a background menu: the preset's own gradient.
export function backgroundSwatchStyle(bg) {
  if (bg.solid) return { background: bg.solid }
  const horizon = bg.horizon || bg.zenith
  return { background: `linear-gradient(180deg, ${bg.zenith}, ${horizon} 55%, ${bg.nadir})` }
}
