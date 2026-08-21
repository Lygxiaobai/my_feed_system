<script setup lang="ts">
import { computed } from 'vue'

import { useThemeStore } from '../stores/theme'
import type { ThemePreference } from '../theme'
import AppIcon, { type AppIconName } from './AppIcon.vue'

const props = defineProps<{
  compact?: boolean
}>()

const emit = defineEmits<{
  picked: [ThemePreference]
}>()

const theme = useThemeStore()

const options: { value: ThemePreference; label: string; icon: AppIconName }[] = [
  { value: 'system', label: '跟随系统', icon: 'desktop' },
  { value: 'light', label: '浅色', icon: 'sun' },
  { value: 'dark', label: '深色', icon: 'moon' },
]

const hint = computed(() => {
  const now = theme.resolved === 'dark' ? '深色' : '浅色'
  if (theme.preference === 'system') return `当前为${now}，跟随系统`
  return `已固定为${now}`
})

function pick(value: ThemePreference) {
  theme.setPreference(value)
  emit('picked', value)
}
</script>

<template>
  <div class="theme-picker" :class="{ compact: props.compact }">
    <div class="theme-options" role="radiogroup" aria-label="外观">
      <button
        v-for="option in options"
        :key="option.value"
        class="theme-opt"
        type="button"
        role="radio"
        :aria-checked="theme.preference === option.value"
        :class="{ on: theme.preference === option.value }"
        @click="pick(option.value)"
      >
        <AppIcon :name="option.icon" :size="props.compact ? 16 : 18" />
        <span>{{ option.label }}</span>
      </button>
    </div>
    <p v-if="!props.compact" class="theme-hint">{{ hint }}</p>
  </div>
</template>

<style scoped>
.theme-picker {
  display: grid;
  gap: 10px;
}

.theme-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.theme-picker.compact .theme-options {
  grid-template-columns: 1fr;
  gap: 4px;
}

.theme-opt {
  appearance: none;
  border: 1px solid var(--border);
  background: var(--fill);
  color: var(--text);
  border-radius: 12px;
  padding: 10px 8px;
  min-height: 40px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 13px;
}

.theme-picker.compact .theme-opt {
  justify-content: flex-start;
  padding: 8px 10px;
  border-color: transparent;
  background: transparent;
}

.theme-opt:hover,
.theme-picker.compact .theme-opt:hover {
  background: var(--fill-hover);
}

.theme-opt.on {
  border-color: rgba(254, 44, 85, 0.55);
  background: rgba(254, 44, 85, 0.14);
}

.theme-hint {
  margin: 0;
  font-size: 13px;
  color: var(--muted);
}
</style>
