<script setup>
import { OpenDiscordLink, OpenDonationLink } from '../../../wailsjs/go/main/App'
</script>

<template>
  <div class="page active-page contact-page">
    <div class="contact-content">
      <div class="contact-hero">
        <h2 class="contact-title">Contact</h2>
        <p class="contact-tagline">Get in touch with the MEW community</p>
      </div>

      <div class="contact-cards">
        <div class="contact-card">
          <div class="contact-card-icon discord-icon">
            <i class="fa-brands fa-discord"></i>
          </div>
          <div class="contact-card-info">
            <h3>Discord</h3>
            <p>Join the MEW community for help, suggestions, and updates.</p>
          </div>
          <button class="btn-contact" @click="OpenDiscordLink">
            <i class="fa-brands fa-discord"></i> Join Server
          </button>
        </div>

        <div class="contact-card">
          <div class="contact-card-icon donate-icon">
            <i class="fa fa-heart"></i>
          </div>
          <div class="contact-card-info">
            <h3>Support MEW</h3>
            <p>If MEW has been useful to you, consider supporting the development.</p>
          </div>
          <button class="btn-contact" @click="OpenDonationLink">
            <i class="fa fa-heart"></i> Donate
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.contact-page {
  justify-content: safe center;
  align-items: center;
}

.contact-content {
  max-width: 480px;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.contact-hero {
  text-align: center;
  margin-bottom: 2rem;
}

.contact-title {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--accent);
  margin: 0 0 0.4rem 0;
}

.contact-tagline {
  color: var(--text-dim);
  font-size: 0.85rem;
  margin: 0;
}

.contact-cards {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  width: 100%;
}

.contact-card {
  border: 1px solid var(--border-default);
  background: var(--bg-hover-1);
  border-radius: 8px;
  padding: 1.25rem;
  display: flex;
  align-items: center;
  gap: 1rem;
}

.contact-card-icon {
  width: 48px;
  height: 48px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 1.25rem;
  flex-shrink: 0;
}

.discord-icon {
  background: rgba(88, 101, 242, 0.15);
  color: #5865F2;
}

.donate-icon {
  background: rgba(232, 121, 168, 0.15);
  color: var(--accent);
}

.contact-card-info {
  flex: 1;
  min-width: 0;
}

.contact-card-info h3 {
  margin: 0 0 0.2rem 0;
  font-size: 0.9rem;
  color: var(--text-secondary);
}

.contact-card-info p {
  margin: 0;
  font-size: 0.75rem;
  color: var(--text-dim);
  line-height: 1.5;
}

.btn-contact {
  padding: 0.45rem 0.9rem;
  background: transparent;
  color: var(--accent);
  border: 1px solid var(--accent);
  border-radius: 4px;
  cursor: pointer;
  font-size: 0.8rem;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
  white-space: nowrap;
}

.btn-contact:hover {
  background: var(--accent-glow);
}
</style>
