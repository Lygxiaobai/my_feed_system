<script setup lang="ts">
import Skeleton from './Skeleton.vue'

withDefaults(
  defineProps<{
    count?: number
  }>(),
  { count: 5 },
)
</script>

<template>
  <div class="list" aria-busy="true" aria-label="评论加载中">
    <div v-for="n in count" :key="n" class="comment">
      <Skeleton width="86px" height="13px" />
      <Skeleton block width="42%" height="12px" />
      <Skeleton block width="92%" height="13px" />
      <Skeleton block :width="n % 2 ? '68%' : '80%'" height="13px" />
    </div>
  </div>
</template>

<style scoped>
.list {
  display: grid;
  gap: 10px;
}

.comment {
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.05);
  border-radius: 14px;
  padding: 10px;
  display: grid;
  gap: 8px;
}
</style>
