<script setup lang="ts">
import Skeleton from './Skeleton.vue'

withDefaults(
  defineProps<{
    count?: number
  }>(),
  { count: 8 },
)
</script>

<template>
  <div class="video-grid" aria-busy="true" aria-label="视频加载中">
    <div v-for="n in count" :key="n" class="video-card">
      <div class="cover">
        <Skeleton width="100%" height="100%" radius="0" />
      </div>
      <div class="meta">
        <Skeleton block :width="n % 3 === 0 ? '64%' : '82%'" height="13px" />
        <Skeleton block width="54%" height="12px" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.video-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.video-card {
  border: 1px solid rgba(var(--fg), 0.12);
  background: rgba(var(--fg), 0.05);
  border-radius: 16px;
  overflow: hidden;
}

.cover {
  width: 100%;
  aspect-ratio: 9 / 12;
  background: rgba(0, 0, 0, 0.35);
}

.cover :deep(.sk) {
  width: 100% !important;
  height: 100% !important;
}

.meta {
  padding: 10px;
  display: grid;
  gap: 8px;
}

@media (max-width: 1100px) {
  .video-grid {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
}

@media (max-width: 800px) {
  .video-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
