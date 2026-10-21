<template>
  <span v-if="text" class="rank-badge" :class="`rank-${tierClass}`">
    {{ text }}
  </span>
  <span v-else class="rank-badge rank-empty">未设置段位</span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  rank: { type: Object, default: null },
})

const TEXT = {
  IRON: '黑铁',
  BRONZE: '青铜',
  SILVER: '白银',
  GOLD: '黄金',
  PLATINUM: '铂金',
  EMERALD: '翡翠',
  DIAMOND: '钻石',
  MASTER: '大师',
  GRANDMASTER: '宗师',
  CHALLENGER: '王者',
}

const text = computed(() => {
  if (!props.rank || !props.rank.tier) return ''
  const label = props.rank.tierLabel || TEXT[props.rank.tier] || props.rank.tier
  const tail = props.rank.division ? props.rank.division : (props.rank.lp ? `${props.rank.lp}分` : '')
  return `${label}${tail}`
})

const tierClass = computed(() => (props.rank?.tier || 'empty').toLowerCase())
</script>

<style scoped>
.rank-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 8px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  color: #4e5969;
  background: #f2f3f5;
  white-space: nowrap;
}
.rank-iron { color: #6b7280; background: #f2f3f5; }
.rank-bronze { color: #9a5b23; background: #fdf0e6; }
.rank-silver { color: #5c6b7a; background: #eef2f6; }
.rank-gold { color: #9a7b16; background: #fdf6dd; }
.rank-platinum { color: #2f7c7a; background: #e6f7f5; }
.rank-emerald { color: #1f8a4c; background: #e8f7ee; }
.rank-diamond { color: #165dff; background: #e8f0ff; }
.rank-master { color: #722ed1; background: #f3e8ff; }
.rank-grandmaster { color: #d4380d; background: #fff0e8; }
.rank-challenger { color: #fff; background: linear-gradient(135deg, #f7ba1e, #f53f3f); }
.rank-empty { color: var(--app-muted); }
</style>
