<template>
  <span
    v-if="rank && rank.tier"
    class="rank-tag"
    :class="[tierClass, { compact }]"
    :title="label"
  >
    <span class="rank-text">{{ label }}</span>
  </span>
  <span v-else class="rank-tag rank-empty">未设置段位</span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  rank: { type: Object, default: null },
  compact: { type: Boolean, default: false },
})

const tier = computed(() => String(props.rank?.tier || '').toUpperCase())
const label = computed(() => props.rank?.label || '')

const tierClass = computed(() => {
  const map = {
    IRON: 'tier-iron',
    BRONZE: 'tier-bronze',
    SILVER: 'tier-silver',
    GOLD: 'tier-gold',
    PLATINUM: 'tier-platinum',
    EMERALD: 'tier-emerald',
    DIAMOND: 'tier-diamond',
    MASTER: 'tier-master',
    GRANDMASTER: 'tier-grandmaster',
    CHALLENGER: 'tier-challenger',
  }
  return map[tier.value] || 'tier-unknown'
})
</script>

<style scoped>
.rank-tag {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  max-width: 100%;
  padding: 3px 7px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 6px;
  font-size: 10px;
  font-weight: 700;
  line-height: 1.2;
  letter-spacing: 0.2px;
  color: #f5f8ff;
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.45);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  position: relative;
  isolation: isolate;
  background-image: linear-gradient(135deg, var(--rank-dark), var(--rank-light));
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18), 0 2px 8px var(--rank-glow);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}
.rank-tag::after {
  content: '';
  position: absolute;
  inset: 0;
  border-radius: inherit;
  background: linear-gradient(115deg, transparent 15%, rgba(255, 255, 255, 0.16) 45%, transparent 65%);
  transform: translateX(-120%);
  animation: rank-sheen 5.5s ease-in-out infinite;
  pointer-events: none;
  z-index: -1;
}
.rank-tag:hover {
  transform: translateY(-1px);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.24), 0 4px 14px var(--rank-glow);
}
.rank-text {
  overflow: hidden;
  text-overflow: ellipsis;
}
.rank-empty {
  background: #151d27;
  color: #7d8a9b;
  border-color: #283443;
  box-shadow: none;
  font-weight: 500;
}
.rank-empty::after {
  display: none;
}

/* 普通段位：深色底 + 分层高光 */
.tier-iron {
  --rank-dark: #2c3440;
  --rank-light: #596274;
  --rank-glow: rgba(148, 163, 184, 0.22);
  border-color: rgba(203, 213, 225, 0.25);
}
.tier-bronze {
  --rank-dark: #4a2a17;
  --rank-light: #a85d2a;
  --rank-glow: rgba(217, 119, 6, 0.35);
  border-color: rgba(251, 146, 60, 0.35);
}
.tier-silver {
  --rank-dark: #374151;
  --rank-light: #9ca3af;
  --rank-glow: rgba(226, 232, 240, 0.25);
  border-color: rgba(241, 245, 249, 0.35);
}
.tier-gold {
  --rank-dark: #6b4a06;
  --rank-light: #d4a017;
  --rank-glow: rgba(250, 204, 21, 0.35);
  border-color: rgba(253, 224, 71, 0.4);
}
.tier-platinum {
  /* 比原来更偏蓝 */
  --rank-dark: #0c4a6e;
  --rank-light: #38bdf8;
  --rank-glow: rgba(56, 189, 248, 0.35);
  border-color: rgba(125, 211, 252, 0.45);
}
.tier-emerald {
  /* 比原来更深一点的绿 */
  --rank-dark: #052e16;
  --rank-light: #15803d;
  --rank-glow: rgba(34, 197, 94, 0.3);
  border-color: rgba(74, 222, 128, 0.38);
}
.tier-diamond {
  --rank-dark: #1e3a8a;
  --rank-light: #60a5fa;
  --rank-glow: rgba(96, 165, 250, 0.4);
  border-color: rgba(147, 197, 253, 0.45);
  background-image: linear-gradient(135deg, #1e3a8a 0%, #4f46e5 50%, #38bdf8 100%);
}

/* 大师：固定紫色，不再按 LP 变色 */
.tier-master {
  --rank-dark: #3b0764;
  --rank-light: #9333ea;
  --rank-glow: rgba(168, 85, 247, 0.5);
  border-color: rgba(216, 180, 254, 0.55);
  background-image:
    radial-gradient(circle at 25% 20%, rgba(233, 213, 255, 0.3), transparent 45%),
    linear-gradient(135deg, #3b0764 0%, #7e22ce 52%, #a855f7 100%);
}

/* 宗师：绚丽红色 */
.tier-grandmaster {
  --rank-dark: #6b0f1a;
  --rank-light: #ef4444;
  --rank-glow: rgba(239, 68, 68, 0.55);
  border-color: rgba(254, 202, 202, 0.6);
  background-image:
    radial-gradient(circle at 22% 18%, rgba(255, 210, 150, 0.35), transparent 42%),
    linear-gradient(135deg, #6b0f1a 0%, #dc2626 52%, #f97316 100%);
  animation: rank-red-glow 3.5s ease-in-out infinite;
}

/* 王者：沿用彩色流动背景 */
.tier-challenger {
  --rank-glow: rgba(255, 120, 220, 0.5);
  border-color: rgba(255, 255, 255, 0.55);
  background-image: linear-gradient(
    110deg,
    #ff2d55 0%,
    #ff8a00 18%,
    #ffe600 34%,
    #2bff88 52%,
    #00c8ff 68%,
    #7a5cff 84%,
    #ff2d55 100%
  );
  background-size: 260% 100%;
  animation: rank-rainbow 3.2s linear infinite;
  text-shadow: 0 1px 3px rgba(0, 0, 0, 0.65);
}
.tier-challenger::after {
  animation: rank-sheen 2.4s ease-in-out infinite;
}

.compact {
  padding: 2px 5px;
  font-size: 9px;
}

@keyframes rank-rainbow {
  0% { background-position: 0% 50%; }
  100% { background-position: 260% 50%; }
}
@keyframes rank-red-glow {
  0%, 100% { box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.2), 0 2px 8px var(--rank-glow); }
  50% { box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.32), 0 2px 16px rgba(239, 68, 68, 0.75); }
}
@keyframes rank-sheen {
  0%, 72% { transform: translateX(-120%); }
  100% { transform: translateX(120%); }
}
</style>
