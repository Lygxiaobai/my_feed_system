<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'

const router = useRouter()

onMounted(async () => {
  await router.replace('/account')
})
</script>

<template>
  <AppShell>
    <div class="card">
      <p class="title">注册</p>
      <p class="subtle">请使用邮箱验证码登录，未注册的邮箱会自动创建账号。</p>
    </div>
  </AppShell>
</template>
