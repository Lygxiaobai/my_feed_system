<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as videoApi from '../api/video'

/**
 * 分享链接的落地页。
 *
 * 存在的意义是让分享文案里的 URL 可以直接点开，而不是只能靠粘贴口令。
 * 解析仍然走后端 resolveShare：口令的编解码规则只有服务端一份。
 */
const route = useRoute()
const router = useRouter()

const error = ref('')

onMounted(async () => {
  const code = String(route.params.code ?? '')
  try {
    const video = await videoApi.resolveShare(code)
    // 用 replace 而不是 push：回退时不该再落回这个中转页。
    await router.replace(`/video/${video.id}`)
  } catch (e) {
    error.value = e instanceof ApiError ? e.message : '口令无效或内容已下架'
  }
})
</script>

<template>
  <AppShell>
    <div class="wrap">
      <template v-if="error">
        <p class="msg">{{ error }}</p>
        <RouterLink class="back" to="/">返回首页</RouterLink>
      </template>
      <p v-else class="msg">正在打开…</p>
    </div>
  </AppShell>
</template>

<style scoped>
.wrap {
  display: grid;
  gap: 14px;
  justify-items: center;
  padding: 64px 16px;
}

.msg {
  margin: 0;
  color: rgba(var(--fg), 0.7);
}

.back {
  color: #fe2c55;
  font-weight: 700;
}
</style>
