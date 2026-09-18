<script setup>
import { ref, onMounted } from 'vue'
import { GetChangelog, CheckForUpdate, GetVersion, OpenDownloadLink, OpenDiscordLink, OpenDonationLink } from '../../../wailsjs/go/main/App'
import { updaterState, downloadUpdate, installUpdate } from '../../utils/updater'

const currentVersion = ref('')
const updateInfo = ref(null)
const changelog = ref([])

const githubURL = 'https://github.com/KyarottoOwO/mew'

function openGitHub(url) {
  OpenDownloadLink(url)
}

const quickFacts = [
  { icon: 'fa-shield-halved', label: 'Runs entirely offline', desc: 'Your files never leave your computer' },
  { icon: 'fa-box-open', label: 'Java \u2192 Bedrock', desc: 'One-click porting powered by SwimPorter' },
  { icon: 'fa-bolt', label: 'Fast & light', desc: 'A small Go app, no waiting around' },
  { icon: 'fa-chalkboard-user', label: 'Beginner friendly', desc: 'Wizard-style flows for every task' },
]

const tools = [
  {
    icon: 'fa-box-open',
    name: 'Pack Porter',
    body:
      'Upload a <code>.zip</code> or <code>.rar</code> Java texture pack, or paste a MediaFire link. ' +
      'MEW converts it to a Bedrock <code>.mcpack</code> in one step using SwimPorter.',
  },
  {
    icon: 'fa-folder-open',
    name: 'Multi-Pack Porter',
    body:
      'Paste a MediaFire folder link to download and convert every pack inside, or upload a ' +
      '<code>.zip</code> / <code>.rar</code> full of packs. Each pack is saved as soon as it\u2019s done, ' +
      'so nothing is lost if you cancel halfway.',
  },
  {
    icon: 'fa-palette',
    name: 'Pack Editor',
    body:
      'Open textures from any installed pack (edited in place) or from an uploaded <code>.mcpack</code>. ' +
      'Paint with the Brush, Eraser, Paint Bucket, Gradient, Magic Wand, Marquee, and eyedropper tools, ' +
      'shift colors with Hue / Saturation / Brightness, and resize with crisp nearest-neighbor scaling. ' +
      'The Paint.NET-style color picker gives you a color wheel, brightness slider, a shades panel for ' +
      'blending one color across many tones, RGB and HSV sliders with a hex box, and a 48-color palette ' +
      '&mdash; right-click a swatch to edit the other color.',
  },
  {
    icon: 'fa-video',
    name: 'Animated Inventory',
    body:
      'Turn a <code>.gif</code> into an animated inventory for Bedrock. Set the frame duration and ' +
      'empty-area fill, optionally layer an overlay <code>.png</code>, then make a <code>.mcpack</code> ' +
      'or merge it straight into a pack you already have installed.',
  },
  {
    icon: 'fa-cloud-sun',
    name: 'Sky Converter',
    body:
      'Turn images into custom Bedrock skies. <strong>Make a Sky</strong> builds the whole pack from a ' +
      '360\u00b0 image; <strong>Port a Sky</strong> slices a Java sky into the faces Bedrock needs. ' +
      'Pick 512px\u20132048px quality and export a <code>.mcpack</code> or merge it into an installed pack.',
  },
  {
    icon: 'fa-eye',
    name: 'Pack Viewer',
    body:
      'Browse installed resource packs and preview armor, tools, and items in 3D on a full player model. ' +
      'Custom skins, material tiers, idle and walk animations, and orbit camera controls behave like an ' +
      'in-game render.',
  },
  {
    icon: 'fa-file-lines',
    name: 'Manifest',
    body:
      'Build a Bedrock pack manifest from a ready-made template, regenerate UUIDs, name and describe your ' +
      'pack, and either create a new <code>.mcpack</code> or apply the manifest to any installed pack.',
  },
  {
    icon: 'fa-gear',
    name: 'Settings',
    body:
      'Auto-import ported packs straight into your Minecraft resource packs folder, choose a custom output ' +
      'directory, auto-open packs on export, and inject your own manifest description. Pack Editor sessions ' +
      'and Discord Rich Presence status also follow these preferences.',
  },
]

onMounted(async () => {
  try {
    currentVersion.value = (await GetVersion()) || ''
  } catch (e) {
    console.error('Failed to load version:', e)
  }
  try {
    const info = await CheckForUpdate()
    if (info && info.needsUpdate) updateInfo.value = info
  } catch (e) {
    console.error('Failed to check for updates:', e)
  }
  try {
    const entries = await GetChangelog()
    if (entries) changelog.value = entries
  } catch (e) {
    console.error('Failed to load changelog:', e)
  }
})
</script>

<template>
  <div class="page active-page info-page">
    <div class="info-content">
      <div class="info-hero">
        <img src="/logo.png" alt="MEW" class="info-logo" />
        <h2 class="info-title">MEW</h2>
        <p class="info-tagline">Minecraft Bedrock Texture Pack Manager</p>
        <span class="info-version" v-if="currentVersion">v{{ currentVersion }}</span>
      </div>

      <p class="info-intro">
        MEW is a small, easy-to-use program made in Go that handles the busy work of Minecraft texture
        packs. Port Java packs to Bedrock, recolor textures, animate inventories, craft custom skies,
        and preview the result &mdash; all on your own computer, fully offline.
      </p>

      <div v-if="updateInfo" class="info-update">
        <div class="info-update-icon"><i class="fa fa-arrow-up"></i></div>

        <div class="info-update-text">
          <template v-if="updaterState.state === 'done'">
            <strong>v{{ updateInfo.latestVersion }} downloaded</strong>
            <span>{{ updaterState.path }}</span>
          </template>
          <template v-else-if="updaterState.state === 'error'">
            <strong>Update failed</strong>
            <span>{{ updaterState.error }}</span>
          </template>
          <template v-else>
            <strong>v{{ updateInfo.latestVersion }} is available</strong>
            <span v-if="updaterState.state !== 'downloading'">
              You&rsquo;re on v{{ currentVersion }}. Grab the new build when you&rsquo;re ready.
            </span>
            <span v-else>Downloading to your Downloads folder&hellip;</span>
          </template>
        </div>

        <template v-if="updaterState.state === 'done'">
          <button class="info-update-btn info-update-solid" @click="installUpdate(updaterState.path)"><i class="fa fa-play"></i> Launch new version</button>
        </template>
        <template v-else-if="updaterState.state === 'error'">
          <button class="info-update-btn" @click="downloadUpdate(updateInfo.downloadUrl)"><i class="fa fa-rotate-right"></i> Retry</button>
          <button class="info-update-btn" @click="openGitHub(updateInfo.downloadUrl)">GitHub</button>
        </template>
        <template v-else-if="updaterState.state === 'downloading'">
          <div class="info-update-bar">
            <div class="info-update-fill" :style="{ width: updaterState.percent + '%' }"></div>
          </div>
          <span class="info-update-pct">{{ updaterState.total > 0 ? updaterState.percent + '%' : '...' }}</span>
        </template>
        <button
          v-else-if="updateInfo.downloadUrl"
          class="info-update-btn"
          @click="downloadUpdate(updateInfo.downloadUrl)"
        >
          <i class="fa fa-download"></i> Download
        </button>
        <button v-else class="info-update-btn" @click="openGitHub(githubURL)">View on GitHub</button>
      </div>

      <div class="info-facts">
        <div v-for="fact in quickFacts" :key="fact.label" class="info-fact">
          <i :class="'fa ' + fact.icon"></i>
          <div>
            <strong>{{ fact.label }}</strong>
            <span>{{ fact.desc }}</span>
          </div>
        </div>
      </div>

      <section class="info-features">
        <h3 class="info-section-title">What&rsquo;s inside</h3>
        <div v-for="tool in tools" :key="tool.name" class="tool-card">
          <div class="tool-card-header">
            <span class="tool-icon-wrap"><i :class="'fa ' + tool.icon"></i></span>
            <h3>{{ tool.name }}</h3>
          </div>
          <p v-html="tool.body"></p>
        </div>
      </section>

      <section class="info-links">
        <button class="info-link-btn" @click="openGitHub(githubURL)">
          <i class="fa-brands fa-github"></i> GitHub
        </button>
        <button class="info-link-btn" @click="OpenDiscordLink">
          <i class="fa-brands fa-discord"></i> Discord
        </button>
        <button class="info-link-btn" @click="OpenDonationLink">
          <i class="fa fa-heart"></i> Donate
        </button>
      </section>

      <section v-if="changelog.length > 0" class="changelog-section">
        <h3 class="info-section-title">Changelog</h3>
        <div v-for="entry in changelog" :key="entry.tag" class="changelog-entry">
          <div class="changelog-header">
            <span class="changelog-tag">v{{ entry.tag }}</span>
            <span class="changelog-date">{{ entry.date }}</span>
          </div>
          <p class="changelog-body">{{ entry.body }}</p>
          <a @click.prevent="openGitHub(entry.url)" class="changelog-link">View on GitHub</a>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.info-page {
  justify-content: flex-start;
  align-items: center;
  padding: 2.5rem 1rem;
  overflow-y: auto;
}

.info-content {
  max-width: 640px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.info-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 1.25rem;
}

.info-logo {
  width: 72px;
  height: 72px;
  border-radius: 14px;
  margin-bottom: 0.85rem;
  object-fit: contain;
  box-shadow: 0 0 0 1px var(--border-default), 0 8px 24px rgba(0, 0, 0, 0.35);
}

.info-title {
  font-size: 1.9rem;
  font-weight: 700;
  color: var(--accent);
  margin: 0;
  letter-spacing: 0.02em;
}

.info-tagline {
  color: var(--text-dim);
  font-size: 0.85rem;
  margin: 0.3rem 0 0.6rem 0;
}

.info-version {
  font-size: 0.72rem;
  color: var(--text-dim);
  background: var(--bg-hover-1);
  border: 1px solid var(--border-default);
  padding: 0.15rem 0.6rem;
  border-radius: 999px;
  letter-spacing: 0.03em;
}

.info-intro {
  color: var(--text-desc);
  line-height: 1.7;
  text-align: center;
  margin: 0.4rem 0 1.25rem 0;
  font-size: 0.85rem;
  max-width: 500px;
}

.info-update {
  display: flex;
  align-items: center;
  gap: 0.7rem;
  width: 100%;
  max-width: 560px;
  background: var(--accent-active-bg);
  border: 1px solid var(--accent-light);
  border-radius: 8px;
  padding: 0.7rem 0.85rem;
  margin-bottom: 1.25rem;
}

.info-update-icon {
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: var(--accent-glow);
  color: var(--accent);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.8rem;
  flex-shrink: 0;
}

.info-update-text {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}

.info-update-text strong {
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.info-update-text span {
  font-size: 0.75rem;
  color: var(--text-dim);
}

.info-update-btn {
  padding: 0.4rem 0.85rem;
  background: var(--accent);
  color: #0e1420;
  border: none;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  font-weight: 600;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.info-update-btn:hover {
  filter: brightness(1.1);
}

.info-update-solid {
  background: var(--accent-light);
}

.info-update-bar {
  width: 140px;
  height: 8px;
  background: var(--bg-hover-3);
  border-radius: 999px;
  overflow: hidden;
  flex-shrink: 0;
}

.info-update-fill {
  height: 100%;
  background: var(--accent);
  border-radius: 999px;
  transition: width 0.15s ease;
}

.info-update-pct {
  font-size: 0.75rem;
  color: var(--text-dim);
  width: 3ch;
  text-align: right;
  flex-shrink: 0;
}

.info-facts {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 0.6rem;
  width: 100%;
  max-width: 560px;
  margin-bottom: 1.75rem;
}

.info-fact {
  display: flex;
  align-items: flex-start;
  gap: 0.65rem;
  background: var(--bg-hover-1);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  padding: 0.7rem 0.8rem;
}

.info-fact i {
  color: var(--accent);
  font-size: 0.95rem;
  width: 18px;
  text-align: center;
  margin-top: 0.15rem;
  flex-shrink: 0;
}

.info-fact div {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
  min-width: 0;
}

.info-fact strong {
  font-size: 0.78rem;
  color: var(--text-secondary);
}

.info-fact span {
  font-size: 0.72rem;
  color: var(--text-dim);
  line-height: 1.45;
}

.info-features {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.info-section-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--accent);
  margin: 0 0 0.25rem 0;
  text-align: center;
}

.tool-card {
  background: var(--bg-hover-1);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  padding: 0.85rem 1rem;
}

.tool-card-header {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.tool-card-header h3 {
  margin: 0;
  font-size: 0.9rem;
  color: var(--text-secondary);
}

.tool-icon-wrap {
  width: 28px;
  height: 28px;
  border-radius: 7px;
  background: var(--accent-glow);
  border: 1px solid var(--accent-light);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.tool-icon-wrap i {
  color: var(--accent);
  font-size: 0.8rem;
}

.tool-card p {
  color: var(--text-desc);
  line-height: 1.65;
  margin: 0.55rem 0 0 0;
  font-size: 0.8rem;
}

.tool-card code {
  background: var(--bg-hover-3);
  color: var(--accent);
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  font-size: 0.74rem;
}

.tool-card strong {
  color: var(--text-secondary);
}

.info-links {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.6rem;
  width: 100%;
  margin: 1.5rem 0 0.5rem 0;
}

.info-link-btn {
  padding: 0.5rem 1rem;
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.8rem;
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.info-link-btn:hover {
  background: var(--accent-glow);
}

.changelog-section {
  margin-top: 1.25rem;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.changelog-entry {
  background: var(--bg-hover-1);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  padding: 0.85rem 1rem;
}

.changelog-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.4rem;
}

.changelog-tag {
  font-weight: 600;
  font-size: 0.9rem;
  color: var(--accent);
}

.changelog-date {
  font-size: 0.75rem;
  color: var(--text-dim);
}

.changelog-body {
  font-size: 0.8rem;
  color: var(--text-desc);
  line-height: 1.6;
  margin: 0.25rem 0;
  white-space: pre-wrap;
}

.changelog-link {
  font-size: 0.75rem;
  color: var(--accent);
  text-decoration: none;
}

.changelog-link:hover {
  text-decoration: underline;
}
</style>