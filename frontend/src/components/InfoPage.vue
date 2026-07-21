<template>
  <div class="page active-page info-page">
    <div class="info-content">
      <h2 class="info-title">About SRM</h2>

      <p class="info-text">
        SRM is a small, easy-to-use program made in Go that helps you manage Minecraft Bedrock texture packs.
        It can quickly convert Java texture packs to Bedrock and even lets you recolor textures with just a few clicks.
      </p>

      <p class="info-text">
        Everything runs on your own computer, so your files stay private. SRM does a lot of the boring work for you,
        making texture pack editing faster and easier.
      </p>

      <p class="info-text">
        If you like customizing your game or using packs from Java Edition, SRM is designed to make the whole process
        simple and stress-free. We're also adding new features over time, so more improvements are on the way!
      </p>
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
  color: #A90B3C;
}

.info-text {
  color: hsl(0, 0%, 55%);
  margin-bottom: 1rem;
  line-height: 1.6;
}
</style>
