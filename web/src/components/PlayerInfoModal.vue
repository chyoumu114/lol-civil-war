<template>
  <a-modal
    :visible="visible"
    title="玩家资料"
    :width="460"
    :footer="false"
    unmount-on-close
    @cancel="close"
  >
    <div v-if="player" class="player-profile">
      <div class="profile-head">
        <a-avatar :size="64" :image-url="player.avatar || ''">
          {{ (player.nickname || '?').slice(0, 1) }}
        </a-avatar>
        <div class="profile-name">
          <div class="name-line">
            <b>{{ player.nickname || '—' }}</b>
            <a-tag v-if="player.isOrganizer" size="small" color="gold">组织者</a-tag>
          </div>
          <div class="profile-qq">QQ {{ player.qqNumber || '未绑定' }}</div>
        </div>
      </div>

      <div class="profile-row">
        <span class="profile-label">主选段位</span>
        <span class="profile-value">{{ player.rankLabel || '未设置' }}</span>
      </div>
      <div class="profile-row">
        <span class="profile-label">位置</span>
        <span class="profile-value positions">
          <template v-if="player.positions?.length">
            <PositionIcon v-for="code in player.positions" :key="code" :code="code" :size="22" show-label />
          </template>
          <span v-else class="muted">未设置</span>
        </span>
      </div>
      <div v-if="player.note" class="profile-row">
        <span class="profile-label">备注</span>
        <span class="profile-value">{{ player.note }}</span>
      </div>
    </div>
  </a-modal>
</template>

<script setup>
import PositionIcon from './PositionIcon.vue'

defineProps({
  visible: { type: Boolean, default: false },
  player: { type: Object, default: null },
})
const emit = defineEmits(['update:visible'])

function close() {
  emit('update:visible', false)
}
</script>

<style scoped>
.player-profile {
  padding: 4px 2px;
}
.profile-head {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 18px;
}
.name-line {
  display: flex;
  align-items: center;
  gap: 8px;
}
.profile-name b {
  font-size: 20px;
}
.profile-qq {
  margin-top: 4px;
  color: var(--app-muted);
  font-size: 13px;
  letter-spacing: 0.5px;
}
.profile-row {
  display: flex;
  gap: 16px;
  padding: 10px 0;
  border-top: 1px solid var(--app-border);
  font-size: 13px;
}
.profile-label {
  width: 72px;
  flex: 0 0 auto;
  color: var(--app-muted);
}
.profile-value {
  flex: 1;
  min-width: 0;
  color: var(--app-text);
}
.profile-value.positions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}
</style>
