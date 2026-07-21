<template>
  <div class="page active-page info-page">
    <div class="info-content">
      <h2 class="info-title">About MEW</h2>

      <p class="info-text">
        MEW is a small, easy-to-use program made in Go that helps you manage Minecraft Bedrock texture packs.
        It can quickly convert Java texture packs to Bedrock and even lets you recolor textures with just a few clicks.
      </p>

      <p class="info-text">
        Everything runs on your own computer, so your files stay private. MEW does a lot of the boring work for you,
        making texture pack editing faster and easier.
      </p>

      <div class="tool-cards">
        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-box-open tool-icon"></i>
            <h3>Pack Porter</h3>
          </div>
          <p>
            Drag and drop a <code>.zip</code> Java texture pack onto the drop zone.
            MEW will automatically convert it to a Bedrock <code>.mcpack</code> file using SwimPorter.
            The converted pack is saved to the <code>Bedrock/</code> folder and the original stays in <code>Java/</code>.
          </p>
        </div>

        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-cloud-arrow-down tool-icon"></i>
            <h3>Folder Porter</h3>
          </div>
          <p>
            Paste a MediaFire folder link and MEW will download and convert every pack in it one by one.
            Each pack is downloaded, converted, and saved immediately so nothing is lost if you cancel halfway.
            Progress is shown with a spinner and per-pack status for each file.
          </p>
        </div>

        <div class="tool-card">
          <div class="tool-card-header">
            <i class="fa fa-palette tool-icon"></i>
            <h3>Recolor Tool</h3>
          </div>
          <p>
            Upload a <code>.mcpack</code> file to open the recolor editor. Browse texture folders using the
            tabs at the top, then click any texture to open the editor with Hue, Saturation, and Brightness sliders.
          </p>
          <p>
            <strong>Paint Mode</strong> &mdash; Toggle the paintbrush icon, adjust the Hue, Saturation, and
            Brightness sliders to pick your color, then click any texture to apply it. Each click paints
            from the original image, so changing the sliders and clicking again replaces the color
            instead of stacking on top of it.
          </p>
          <p>
            <strong>Export</strong> &mdash; When you're happy with your changes, click "Export Pack" to save a
            new <code>.mcpack</code> file with all your recolored textures.
          </p>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.info-page {
  justify-content: flex-start;
  padding: 1rem 1rem 1rem 1rem;
}

.info-content {
  padding-left: 1rem;
  max-width: 700px;
}

.info-title {
  font-size: 1.5rem;
  margin-bottom: 1rem;
  color: #e879a8;
}

.info-text {
  color: hsl(0, 0%, 55%);
  margin-bottom: 1rem;
  line-height: 1.6;
}

.tool-cards {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  margin-top: 1rem;
}

.tool-card {
  background: hsl(0, 0%, 5%);
  border: 1px solid hsl(0, 0%, 12%);
  border-radius: 6px;
  padding: 0.85rem 1rem;
}

.tool-card p {
  color: hsl(0, 0%, 55%);
  line-height: 1.6;
  margin: 0.4rem 0 0 0;
  font-size: 0.8rem;
}

.tool-card p:first-of-type {
  margin-top: 0.5rem;
}

.tool-card code {
  background: hsl(0, 0%, 12%);
  color: #e879a8;
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
  font-size: 0.75rem;
}

.tool-card strong {
  color: hsl(0, 0%, 75%);
}

.tool-card-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.tool-card-header h3 {
  margin: 0;
  font-size: 0.9rem;
  color: hsl(0, 0%, 80%);
}

.tool-icon {
  color: #e879a8;
  font-size: 1rem;
  width: 20px;
  text-align: center;
}
</style>
