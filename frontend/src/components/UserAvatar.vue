<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    username: string
    id?: number
    size?: number
    compact?: boolean
  }>(),
  { size: 40, compact: false },
)

function hashToHue(input: string) {
  let h = 0
  for (let i = 0; i < input.length; i += 1) {
    h = (h * 31 + input.charCodeAt(i)) >>> 0
  }
  return h % 360
}

const initial = computed(() => {
  const s = (props.username ?? '').trim()
  if (!s) return '?'
  return s.slice(0, 1).toUpperCase()
})

const sizePx = computed(() => `${props.size}px`)
const fontPx = computed(() => (props.compact ? `${Math.max(11, Math.round(props.size * 0.42))}px` : undefined))

const bg = computed(() => {
  const seed = typeof props.id === 'number' ? String(props.id) : props.username
  const hue = hashToHue(seed || '0')
  const h1 = hue
  const h2 = (hue + 40) % 360
  return `linear-gradient(135deg, hsl(${h1} 90% 55%), hsl(${h2} 90% 55%))`
})
</script>

<template>
  <div
    class="avatar"
    :class="{ compact }"
    :style="{ width: sizePx, height: sizePx, fontSize: fontPx, backgroundImage: bg }"
    aria-hidden="true"
  >
    {{ initial }}
  </div>
</template>

<style scoped>
.avatar {
  display: grid;
  place-items: center;
  border-radius: 999px;
  border: 1px solid var(--border);
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.25);
  color: rgba(255, 255, 255, 0.92);
  font-weight: 900;
  letter-spacing: 0.2px;
  user-select: none;
}

.avatar.compact {
  box-shadow: none;
}
</style>

