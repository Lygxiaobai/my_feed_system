<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import NotificationPanel from '../components/NotificationPanel.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const blocked = computed(() => !auth.isLoggedIn)
</script>

<template>
  <AppShell>
    <div v-if="blocked" class="card">
      <p class="title">通知</p>
      <p class="subtle">登录后才能查看关注、点赞、评论和打赏。</p>
      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button class="primary" type="button" @click="router.push('/account')">去登录</button>
      </div>
    </div>
    <NotificationPanel v-else variant="page" />
  </AppShell>
</template>
