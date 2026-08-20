<script setup lang="ts">
import Skeleton from './Skeleton.vue'

withDefaults(
  defineProps<{
    count?: number
  }>(),
  { count: 6 },
)
</script>

<template>
  <div class="list" aria-busy="true" aria-label="用户列表加载中">
    <div v-for="n in count" :key="n" class="user-row">
      <Skeleton circle width="40px" height="40px" />
      <div class="meta">
        <Skeleton :width="n % 2 ? '96px' : '128px'" height="14px" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.list {
  display: grid;
  gap: 10px;
}

.user-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 4px;
}

.meta {
  display: grid;
  gap: 6px;
}
</style>
