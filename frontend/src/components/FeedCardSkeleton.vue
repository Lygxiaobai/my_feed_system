<script setup lang="ts">
import Skeleton from './Skeleton.vue'
</script>

<template>
  <div class="feed-card" aria-busy="true" aria-label="视频卡片加载中">
    <div class="cover">
      <Skeleton width="100%" height="100%" radius="0" />
    </div>
    <div class="content">
      <div class="top">
        <div class="copy">
          <Skeleton block width="58%" height="16px" />
          <Skeleton block width="78%" height="13px" />
        </div>
        <div class="pills">
          <Skeleton width="72px" height="28px" radius="999px" />
          <Skeleton width="56px" height="28px" radius="999px" />
          <Skeleton width="56px" height="28px" radius="999px" />
        </div>
      </div>
      <Skeleton block width="88%" height="13px" />
      <div class="pills">
        <Skeleton width="88px" height="28px" radius="999px" />
        <Skeleton width="120px" height="28px" radius="999px" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.feed-card {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 14px;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  border-radius: 16px;
  overflow: hidden;
}

.cover {
  background: rgba(0, 0, 0, 0.25);
  aspect-ratio: 16 / 9;
}

.cover :deep(.sk) {
  width: 100% !important;
  height: 100% !important;
}

.content {
  padding: 12px 12px 14px;
  display: grid;
  gap: 10px;
  min-width: 0;
}

.top {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.copy {
  flex: 1;
  min-width: 0;
  display: grid;
  gap: 8px;
}

.pills {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}

@media (max-width: 900px) {
  .feed-card {
    grid-template-columns: 1fr;
  }

  .top {
    flex-direction: column;
  }
}
</style>
