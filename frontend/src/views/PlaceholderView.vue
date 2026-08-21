<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { useAuthStore } from '../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const title = computed(() => (typeof route.meta.title === 'string' ? route.meta.title : '即将上线'))
const summary = computed(() =>
  typeof route.meta.summary === 'string' ? route.meta.summary : '这个功能还在做，入口先放在这里。',
)
const needAuth = computed(() => route.meta.auth === true)
const blocked = computed(() => needAuth.value && !auth.isLoggedIn)
</script>

<template>
  <AppShell>
    <div class="card">
      <p class="title">{{ title }}</p>
      <p v-if="blocked" class="subtle">登录后才能使用这个功能。</p>
      <p v-else class="subtle">{{ summary }}</p>
      <div class="row" style="margin-top: 14px; justify-content: flex-end">
        <button v-if="blocked" class="primary" type="button" @click="router.push('/account')">去登录</button>
        <button v-else type="button" @click="router.back()">返回</button>
      </div>
    </div>
  </AppShell>
</template>
