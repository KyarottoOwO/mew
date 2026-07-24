<template>
  <div class="page active-page info-page">
    <div class="info-content">
      <div class="info-hero">
        <img src="/logo.png" alt="MEW" class="info-logo" />
        <h2 class="info-title">MEW</h2>
        <p class="info-tagline">Minecraft Bedrock Texture Pack Manager</p>
      </div>

      <p class="info-text">
        MEW is a small, easy-to-use program made in Go that helps you manage Minecraft Bedrock texture packs.
        It can quickly convert Java texture packs to Bedrock and even lets you recolor textures with just a few clicks.
        Everything runs on your own computer, so your files stay private.
      </p>

      <div class="tool-cards">
        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-box-open tool-icon"></i>
            <h3>Pack Porter</h3>
          </div>
          <p>
            Upload a <code>.zip</code> or <code>.rar</code> Java texture pack, or paste a MediaFire link.
            MEW will automatically convert it to a Bedrock <code>.mcpack</code> file using SwimPorter.
          </p>
        </div>

        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-folder-open tool-icon"></i>
            <h3>Multi-Pack Porter</h3>
          </div>
          <p>
            Paste a MediaFire folder link to download and convert every pack in it one by one,
            or upload a <code>.zip</code> / <code>.rar</code> archive containing multiple packs.
            Each pack is converted and saved immediately so nothing is lost if you cancel halfway.
          </p>
        </div>

        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-palette tool-icon"></i>
            <h3>Recolor Tool</h3>
          </div>
          <p>
            Upload a <code>.mcpack</code> file to open the recolor editor. Browse texture folders using the
            tabs, then click any texture to open the editor with Hue, Saturation, and Brightness sliders.
          </p>
          <p>
            <strong>Paint Mode</strong> &mdash; Toggle the paintbrush icon, adjust the sliders to pick your
            color, then click any texture to apply it. Each click paints from the original image, so
            changing the sliders and clicking again replaces the color instead of stacking.
          </p>
          <p>
            <strong>Export</strong> &mdash; When you're happy with your changes, click "Export Pack" to save a
            new <code>.mcpack</code> file with all your recolored textures.
          </p>
        </div>

        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-gear tool-icon"></i>
            <h3>Settings</h3>
          </div>
          <p>
            Configure auto-import to directly inject ported packs into your Minecraft resource packs folder.
            Set a custom output directory, enable auto-open on export, and inject a custom manifest description.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.info-page {
  justify-content: flex-start;
  align-items: center;
  padding: 2rem 1rem;
  overflow-y: auto;
}

.info-content {
  max-width: 560px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.info-hero {
  display: flex;
  flex-direction: column;
  align-items: center;
  margin-bottom: 1.5rem;
}

.info-logo {
  width: 64px;
  height: 64px;
  border-radius: 12px;
  margin-bottom: 0.75rem;
  object-fit: contain;
}

.info-title {
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--accent);
  margin: 0;
}

.info-tagline {
  color: var(--text-dim);
  font-size: 0.85rem;
  margin-top: 0.3rem;
}

.info-text {
  color: var(--text-desc);
  line-height: 1.7;
  text-align: center;
  margin-bottom: 1.5rem;
  font-size: 0.85rem;
  max-width: 480px;
}

.tool-cards {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  width: 100%;
}

.tool-card {
  background: var(--bg-hover-1);
  border: 1px solid var(--border-default);
  border-radius: 8px;
  padding: 0.85rem 1rem;
}

.tool-card p {
  color: var(--text-desc);
  line-height: 1.6;
  margin: 0.35rem 0 0 0;
  font-size: 0.8rem;
}

.tool-card p:first-of-type {
  margin-top: 0.4rem;
}

.tool-card code {
  background: var(--bg-hover-3);
  color: var(--accent);
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  font-size: 0.75rem;
}

.tool-card strong {
  color: var(--text-secondary);
}

.tool-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.tool-card-header h3 {
  margin: 0;
  font-size: 0.9rem;
  color: var(--text-secondary);
}

.tool-icon {
  color: var(--accent);
  font-size: 1rem;
  width: 20px;
  text-align: center;
}
</style>
