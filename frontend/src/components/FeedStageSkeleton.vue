<script setup lang="ts">
import Skeleton from './Skeleton.vue'
</script>

<template>
  <div class="stage" aria-busy="true" aria-label="内容加载中">
    <div class="fill">
      <Skeleton width="100%" height="100%" radius="0" />
    </div>
    <div class="grad" />

    <div class="meta">
      <div class="author">
        <Skeleton circle width="34px" height="34px" />
      </div>
      <Skeleton block width="72%" height="16px" />
      <Skeleton block width="48%" height="13px" />
    </div>

    <div class="actions">
      <Skeleton v-for="n in 4" :key="n" width="40px" height="40px" radius="999px" />
    </div>
  </div>
</template>

<style scoped>
.stage {
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  background: #000;
}

.fill {
  position: absolute;
  inset: 0;
}

.fill :deep(.sk) {
  width: 100% !important;
  height: 100% !important;
}

.grad {
  position: absolute;
  inset: 0;
  background: linear-gradient(to top, rgba(0, 0, 0, 0.68), rgba(0, 0, 0, 0.12) 40%, rgba(0, 0, 0, 0) 70%);
  pointer-events: none;
}

.meta {
  position: absolute;
  left: 16px;
  bottom: 18px;
  width: min(620px, calc(100% - 96px));
  display: grid;
  gap: 10px;
}

.author {
  display: flex;
  align-items: center;
  gap: 10px;
}

.actions {
  position: absolute;
  right: 10px;
  bottom: 18px;
  display: grid;
  gap: 14px;
}

.actions :deep(.sk) {
  width: 40px !important;
  height: 40px !important;
  border-radius: 999px !important;
}

@media (max-width: 900px) {
  .meta {
    left: 12px;
    right: 84px;
    bottom: calc(14px + env(safe-area-inset-bottom, 0px));
    width: auto;
  }

  .actions {
    right: 6px;
    bottom: calc(18px + env(safe-area-inset-bottom, 0px));
    gap: 12px;
  }
}
</style>
