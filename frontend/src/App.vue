<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import ShareInbox from './components/ShareInbox.vue'

const route = useRoute()
const isAdmin = computed(() => route.path === '/admin' || route.path.startsWith('/admin/'))
</script>

<template>
  <RouterView />
  <ShareInbox v-if="!isAdmin" />
</template>
