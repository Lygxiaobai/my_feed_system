<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'

import { track } from '../analytics/track'
import AppShell from '../components/AppShell.vue'
import { ApiError } from '../api/client'
import * as accountApi from '../api/account'
import { useAuthStore } from '../stores/auth'
import { useSocialStore } from '../stores/social'
import { useToastStore } from '../stores/toast'

const router = useRouter()
const auth = useAuthStore()
const social = useSocialStore()
const toast = useToastStore()

const busy = ref(false)
const form = reactive({ username: '', password: '' })

onMounted(async () => {
  if (auth.isLoggedIn) {
    await router.replace('/account')
  }
})

async function onLogin() {
  if (busy.value) return
  const username = form.username.trim()
  const password = form.password.trim()
  if (!username || !password) {
    toast.error('请输入用户名和密码')
    return
  }
  busy.value = true
  try {
    const res = await accountApi.login(username, password)
    auth.setToken(res.token)
    track('login')
    toast.success('登录成功')
    await social.refreshMine()
    await router.replace('/account')
  } catch (e) {
    toast.error(e instanceof ApiError ? e.message : String(e))
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AppShell>
    <div class="login-wrap">
      <div class="card login-card">
        <p class="title">账号密码登录</p>
        <p class="subtle">已有用户名和密码的账号从这里登录。新用户请用邮箱验证码。</p>
        <div class="grid" style="margin-top: 12px">
          <div>
            <label>用户名</label>
            <input v-model.trim="form.username" autocomplete="username" />
          </div>
          <div>
            <label>密码</label>
            <input
              v-model.trim="form.password"
              type="password"
              autocomplete="current-password"
              @keydown.enter="onLogin"
            />
          </div>
          <button class="primary" type="button" :disabled="busy" @click="onLogin">登录</button>
        </div>
        <div class="links">
          <button class="linkish" type="button" @click="router.push('/account')">返回邮箱登录</button>
          <button class="linkish" type="button" @click="router.push('/account/change-password')">修改密码</button>
        </div>
      </div>
    </div>
  </AppShell>
</template>

<style scoped>
.login-wrap {
  display: grid;
  justify-items: center;
  align-content: start;
  padding: clamp(56px, 14vh, 160px) 16px 40px;
}

.login-card {
  width: min(420px, 100%);
}

.links {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-top: 14px;
}

.linkish {
  border: 0;
  background: transparent;
  color: rgba(255, 255, 255, 0.62);
  padding: 0;
  cursor: pointer;
}

.linkish:hover {
  color: rgba(255, 255, 255, 0.9);
}
</style>
