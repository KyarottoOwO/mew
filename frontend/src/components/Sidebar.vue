<script setup>
defineProps({
  currentPage: String
})

const emit = defineEmits(['navigate'])

const navItems = [
  { id: 'home', icon: 'fa-house', label: 'Home' },
  { id: 'packporter', icon: 'fa-box-open', label: 'Pack Porter' },
  { id: 'packFolderPorter', icon: 'fa-box-open', label: 'Folder Porter' },
  { id: 'recolor', icon: 'fa-palette', label: 'Recolor Tool' },
  { id: 'info', icon: 'fa-circle-info', label: 'Info' },
]
</script>

<template>
  <aside class="sidebar">
    <div class="sidebar-logo">
      <img src="/logo.png" alt="MEW" />
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
  border-radius: 6px;
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
  background: hsl(330, 60%, 25%);
  color: hsl(330, 60%, 65%);
}

.sidebar li.active i {
  color: #e879a8;
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
