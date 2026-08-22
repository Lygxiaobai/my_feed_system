<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink } from 'vue-router'

import UserAvatar from './UserAvatar.vue'

// 头像必须走 UserAvatar，才能和资料页、通知用同一套颜色种子。

const props = withDefaults(
  defineProps<{
    username: string
    authorId: number
    createdAt: string
    size?: number
  }>(),
  { size: 36 },
)

const displayName = computed(() => props.username.trim() || '用户')
const profileTo = computed(() => (props.authorId > 0 ? `/u/${props.authorId}` : ''))
const when = computed(() => {
  const date = new Date(props.createdAt)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString()
})
</script>

<template>
  <div class="author">
    <RouterLink v-if="profileTo" class="face" :to="profileTo">
      <UserAvatar compact :username="displayName" :id="authorId" :size="size" />
    </RouterLink>
    <span v-else class="face">
      <UserAvatar compact :username="displayName" :id="authorId" :size="size" />
    </span>
    <div class="main">
      <div class="head">
        <RouterLink v-if="profileTo" class="name" :to="profileTo">{{ displayName }}</RouterLink>
        <span v-else class="name">{{ displayName }}</span>
        <time v-if="when" class="when" :datetime="createdAt">{{ when }}</time>
      </div>
      <slot />
    </div>
  </div>
</template>

<style scoped>
.author {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 10px;
  align-items: start;
}

.face {
  display: block;
  line-height: 0;
  text-decoration: none;
}

.name {
  font-weight: 700;
  font-size: 13px;
  color: var(--text);
  text-decoration: none;
}

.name:hover {
  color: rgba(var(--fg), 0.92);
}

.when {
  display: block;
  margin-top: 2px;
  font-size: 12px;
  color: var(--muted);
}

.main {
  min-width: 0;
}
</style>
