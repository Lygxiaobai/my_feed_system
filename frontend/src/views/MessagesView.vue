<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import MessagePanel from '../components/MessagePanel.vue'
import { useAuthStore } from '../stores/auth'

const auth = useAuthStore()
const router = useRouter()
const blocked = computed(() => !auth.isLoggedIn)
</script>

<template>
  <AppShell>
    <div v-if="blocked" class="card">
      <p class="title">消息</p>
      <p class="subtle">登录后才能和好友私聊。</p>
      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button class="primary" type="button" @click="router.push('/account')">去登录</button>
      </div>
    </div>
    <MessagePanel v-else variant="page" />
  </AppShell>
</template>
