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
      <Skeleton v-for="n in 4" :key="n" width="70px" height="62px" radius="16px" />
    </div>
  </div>
</template>

<style scoped>
.stage {
  width: min(980px, calc(100vw - 28px));
  height: min(100%, calc(var(--app-height, 100dvh) - var(--topbar-h, 56px) - 52px - 36px - var(--bottom-nav-h, 0px)));
  position: relative;
  border-radius: 18px;
  overflow: hidden;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(0, 0, 0, 0.35);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.55);
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
  right: 12px;
  bottom: 18px;
  display: grid;
  gap: 12px;
}

@media (max-width: 900px) {
  .stage {
    width: 100%;
    height: 100%;
    border-radius: 0;
    border: none;
    box-shadow: none;
  }

  .meta {
    left: 12px;
    right: 84px;
    bottom: calc(14px + env(safe-area-inset-bottom, 0px));
    width: auto;
  }

  .actions {
    right: 8px;
    bottom: calc(18px + env(safe-area-inset-bottom, 0px));
    gap: 10px;
  }

  .actions :deep(.sk) {
    width: 64px !important;
    height: 56px !important;
    border-radius: 14px !important;
  }
}
</style>
