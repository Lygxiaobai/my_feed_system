<script setup lang="ts">
withDefaults(
  defineProps<{
    width?: string
    height?: string
    radius?: string
    circle?: boolean
    block?: boolean
  }>(),
  {
    width: '100%',
    height: '14px',
    radius: '8px',
  },
)
</script>

<template>
  <span
    class="sk"
    :class="{ circle, block }"
    :style="{ width, height, borderRadius: circle ? '999px' : radius }"
  />
</template>

<style scoped>
.sk {
  display: inline-block;
  flex-shrink: 0;
  vertical-align: middle;
  background: linear-gradient(
    90deg,
    rgba(255, 255, 255, 0.06) 0%,
    rgba(255, 255, 255, 0.14) 46%,
    rgba(255, 255, 255, 0.06) 100%
  );
  background-size: 200% 100%;
  animation: sk-shimmer 1.25s ease-in-out infinite;
}

.sk.block {
  display: block;
}

.sk.circle {
  border-radius: 999px;
}

@media (prefers-reduced-motion: reduce) {
  .sk {
    animation: none;
    background: rgba(255, 255, 255, 0.1);
  }
}

@keyframes sk-shimmer {
  0% {
    background-position: 100% 0;
  }
  100% {
    background-position: -100% 0;
  }
}
</style>
