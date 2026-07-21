<script setup>
defineProps({
  currentPage: String
})

const emit = defineEmits(['navigate'])

const navItems = [
  { id: 'home', icon: 'fa-house', label: 'Home' },
  { id: 'packporter', icon: 'fa-box-open', label: 'Pack Porter' },
  { id: 'packFolderPorter', icon: 'fa-box-open', label: 'Pack Folder Porter' },
  { id: 'recolor', icon: 'fa-palette', label: 'Recolor Tool' },
  { id: 'info', icon: 'fa-circle-info', label: 'Info' },
]
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-logo">
      <img src="data:image/png;base64,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" alt="SRM Logo">
    </div>
    <div class="sidebar-bar"></div>
    <ul>
      <li v-for="item in navItems" :key="item.id"
          :class="{ active: currentPage === item.id }"
          @click="emit('navigate', item.id)">
        <i :class="'fa ' + item.icon"></i>
        <span>{{ item.label }}</span>
      </li>
      <li class="disabled">
        <i class="fa fa-clock"></i>
        <span>Coming Soon</span>
      </li>
    </ul>
  </aside>
</template>

<style scoped>
.sidebar {
  width: 64px;
  min-width: 64px;
  background: hsl(0, 0%, 3%);
  border-right: 1px solid hsl(0, 0%, 8%);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  z-index: 10;
}

.sidebar-logo {
  width: 40px;
  height: 40px;
  margin-bottom: 8px;
}

.sidebar-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.sidebar-bar {
  width: 32px;
  height: 1px;
  background: hsl(0, 0%, 15%);
  margin: 8px 0;
}

.sidebar ul {
  list-style: none;
  padding: 0;
  margin: 0;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.sidebar li {
  width: 48px;
  height: 48px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-radius: 8px;
  transition: all 0.2s;
  color: hsl(0, 0%, 40%);
  font-size: 10px;
  gap: 2px;
}

.sidebar li i {
  font-size: 16px;
}

.sidebar li:hover {
  background: hsl(0, 0%, 8%);
  color: hsl(0, 0%, 70%);
}

.sidebar li.active {
  background: hsl(340, 85%, 25%);
  color: hsl(340, 85%, 55%);
}

.sidebar li.active i {
  color: #A90B3C;
}

.sidebar li.disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.sidebar li span {
  font-size: 8px;
  text-align: center;
  line-height: 1;
}
</style>
