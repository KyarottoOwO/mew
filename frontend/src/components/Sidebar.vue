<script setup>
import { ref, computed, onMounted } from 'vue'

defineProps({
  currentPage: String
})

const emit = defineEmits(['navigate', 'width-change'])

const navItems = [
  { id: 'home', icon: 'fa-house', label: 'Home' },
  { id: 'packporter', icon: 'fa-box-open', label: 'Pack Porter' },
  { id: 'packFolderPorter', icon: 'fa-folder-open', label: 'Folder Porter' },
  { id: 'recolor', icon: 'fa-palette', label: 'Recolor Tool' },
  { id: 'info', icon: 'fa-circle-info', label: 'Info' },
]

const COLLAPSED_WIDTH = 64
const EXPANDED_WIDTH = 220

const isExpanded = ref(false)

const sidebarWidth = computed(() => isExpanded.value ? EXPANDED_WIDTH : COLLAPSED_WIDTH)

function toggleSidebar() {
  isExpanded.value = !isExpanded.value
  emit('width-change', isExpanded.value ? EXPANDED_WIDTH : COLLAPSED_WIDTH)
}

const tooltipVisible = ref(false)
const tooltipText = ref('')
const tooltipStyle = ref({})

function showTooltip(e, text) {
  if (isExpanded.value) return
  tooltipText.value = text
  const rect = e.currentTarget.getBoundingClientRect()
  tooltipStyle.value = {
    left: (rect.right + 10) + 'px',
    top: (rect.top + rect.height / 2) + 'px',
  }
  tooltipVisible.value = true
}

function hideTooltip() {
  tooltipVisible.value = false
}
</script>

<template>
  <aside
    class="sidebar"
    :class="{ expanded: isExpanded }"
    :style="{ width: sidebarWidth + 'px', minWidth: sidebarWidth + 'px' }"
  >
    <div class="sidebar-logo">
      <img src="/logo.png" alt="MEW" />
    </div>

    <ul>
      <li v-for="item in navItems" :key="item.id"
          :class="{ active: currentPage === item.id }"
          @click="emit('navigate', item.id)"
          @mouseenter="showTooltip($event, item.label)"
          @mouseleave="hideTooltip">
        <i :class="'fa ' + item.icon"></i>
        <span class="nav-label" v-if="isExpanded">{{ item.label }}</span>
      </li>
    </ul>

    <div class="sidebar-bottom">
      <span class="version">v1.0</span>
      <button class="toggle-btn" @click="toggleSidebar" :title="isExpanded ? 'Collapse sidebar' : 'Expand sidebar'">
        <i class="fa" :class="isExpanded ? 'fa-chevron-left' : 'fa-chevron-right'"></i>
      </button>
    </div>

    <div class="tooltip" v-if="tooltipVisible" :style="tooltipStyle">{{ tooltipText }}</div>
  </aside>
</template>

<style scoped>
.sidebar {
  position: relative;
  background: hsl(0, 0%, 3%);
  border-right: 1px solid hsl(0, 0%, 8%);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 12px 0;
  z-index: 10;
  overflow: hidden;
}

.sidebar-logo {
  width: 40px;
  height: 40px;
  margin-bottom: 16px;
  flex-shrink: 0;
}

.sidebar.expanded .sidebar-logo {
  width: 44px;
  height: 44px;
}

.sidebar-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  border-radius: 8px;
}

.sidebar ul {
  list-style: none;
  padding: 0 6px;
  margin: 0;
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.sidebar li {
  position: relative;
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  border-radius: 8px;
  border: 1px solid transparent;
  color: hsl(0, 0%, 40%);
  gap: 10px;
}

.sidebar.expanded li {
  justify-content: flex-start;
  padding-left: 12px;
}

.sidebar li i {
  font-size: 18px;
  width: 24px;
  text-align: center;
  flex-shrink: 0;
}

.sidebar li:hover {
  background: hsl(0, 0%, 8%);
  color: hsl(0, 0%, 70%);
}

.sidebar li.active {
  background: hsla(330, 60%, 65%, 0.08);
  color: hsl(330, 60%, 70%);
  border-color: hsl(330, 60%, 70%);
}

.sidebar li.active i {
  color: #e879a8;
}

.nav-label {
  font-size: 13px;
  font-weight: 500;
  white-space: nowrap;
}

.tooltip {
  position: fixed;
  transform: translateY(-50%);
  background: hsl(0, 0%, 10%);
  border: 1px solid hsl(0, 0%, 18%);
  color: hsl(0, 0%, 85%);
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 500;
  white-space: nowrap;
  z-index: 100;
  pointer-events: none;
  box-shadow: 0 4px 12px rgba(0,0,0,0.4);
}

.sidebar-bottom {
  margin-top: auto;
  padding: 12px 0 4px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.version {
  font-size: 10px;
  color: hsl(0, 0%, 22%);
  letter-spacing: 0.5px;
  user-select: none;
}

.toggle-btn {
  width: 32px;
  height: 32px;
  border-radius: 6px;
  border: 1px solid hsl(0, 0%, 20%);
  background: hsl(0, 0%, 5%);
  color: hsl(0, 0%, 40%);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
}

.toggle-btn:hover {
  background: hsl(0, 0%, 10%);
  color: hsl(0, 0%, 70%);
  border-color: hsl(0, 0%, 30%);
}
</style>
