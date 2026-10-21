<template>
  <div class="analysis-shell">
    <header class="analysis-header">
      <router-link to="/" class="analysis-brand">LOL 内战工具</router-link>
      <span class="analysis-divider">/</span>
      <span class="analysis-title">战力计算器</span>
    </header>

    <main class="page analysis-page">
      <h1 class="page-title">战力计算器</h1>
      <p class="page-subtitle">选择分路、大区与段位，实时查看战力拆解；用于调试战力平衡。</p>

      <div class="analysis-grid">
        <a-card :bordered="false" class="analysis-card">
          <div class="field-label">分路</div>
          <a-radio-group v-model="position" type="button" class="position-tabs">
            <a-radio v-for="item in positions" :key="item.code" :value="item.code">
              <span class="position-choice">
                <PositionIcon :code="item.code" :size="18" />
                {{ item.label }}
              </span>
            </a-radio>
          </a-radio-group>

          <div class="field-label rank-label">大区 / 段位</div>
          <RankPicker
            v-if="meta"
            v-model="rank"
            :regions="meta.regions"
            :tiers="meta.rankTiers"
          />
        </a-card>

        <a-card :bordered="false" class="analysis-card result-card">
          <div v-if="error" class="result-error">{{ error }}</div>
          <a-spin v-else :loading="loading" style="width: 100%">
            <template v-if="result">
              <div class="power-result">
                <div>
                  <div class="power-caption">{{ result.positionLabel }} · 战力</div>
                  <div class="power-number">{{ result.power }}</div>
                </div>
                <RankTag :rank="result.rank" />
              </div>

              <div class="formula">
                <span>{{ result.baseScore }}</span>
                <span class="operator">×</span>
                <span>{{ result.regionCoefficient }}</span>
                <span class="operator">×</span>
                <span>(1 + {{ result.positionInfluence }})</span>
                <span class="operator">=</span>
                <b>{{ result.power }}</b>
              </div>

              <div class="breakdown">
                <div class="breakdown-row">
                  <span>段位基础分</span>
                  <b>{{ result.baseScore }}</b>
                </div>
                <div class="breakdown-row">
                  <span>{{ result.region }} 大区系数</span>
                  <b>× {{ result.regionCoefficient }}</b>
                </div>
                <div class="breakdown-row">
                  <span>{{ result.positionLabel }} 位置影响力</span>
                  <b>+ {{ Math.round(result.positionInfluence * 100) }}%</b>
                </div>
              </div>
            </template>
            <div v-else class="result-empty">请选择分路、大区和段位</div>
          </a-spin>
        </a-card>
      </div>
    </main>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import PositionIcon from '../components/PositionIcon.vue'
import RankPicker from '../components/RankPicker.vue'
import RankTag from '../components/RankTag.vue'
import { api } from '../api'

const meta = ref(null)
const position = ref('TOP')
const rank = ref({ region: '', tier: 'GOLD', division: 4, lp: 0 })
const result = ref(null)
const loading = ref(false)
const error = ref('')
let timer = null

const positions = computed(() => meta.value?.positions || [])

async function calculate() {
  if (!meta.value || !rank.value.region || !rank.value.tier) return
  loading.value = true
  error.value = ''
  try {
    result.value = await api.calculatePower({
      position: position.value,
      region: rank.value.region,
      tier: rank.value.tier,
      division: rank.value.division,
      lp: rank.value.lp || 0,
    })
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function schedule() {
  clearTimeout(timer)
  timer = setTimeout(calculate, 120)
}

watch([position, rank], schedule, { deep: true })

onMounted(async () => {
  try {
    const data = await api.meta()
    meta.value = data
    rank.value = {
      region: data.regions?.[0] || '',
      tier: 'GOLD',
      division: 4,
      lp: 0,
    }
    position.value = data.positions?.[0]?.code || 'TOP'
  } catch (e) {
    error.value = e.message
  }
})

onBeforeUnmount(() => clearTimeout(timer))
</script>

<style scoped>
.analysis-shell {
  min-height: 100vh;
  background: var(--app-bg);
}
.analysis-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 18px 24px;
  border-bottom: 1px solid var(--app-border);
  color: var(--app-muted);
}
.analysis-brand {
  color: #ffcf66;
  font-weight: 800;
}
.analysis-divider {
  color: var(--app-border);
}
.analysis-title {
  font-size: 13px;
}
.analysis-page {
  padding-top: 28px;
}
.analysis-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}
.analysis-card {
  background: var(--app-panel);
  border: 1px solid var(--app-border);
  border-radius: 12px;
}
.field-label {
  margin-bottom: 8px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}
.rank-label {
  margin-top: 20px;
}
.position-tabs {
  display: flex;
  flex-wrap: wrap;
}
.position-choice {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.power-result {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--app-border);
}
.power-caption {
  color: var(--app-muted);
  font-size: 12px;
}
.power-number {
  margin-top: 2px;
  color: #ffcf66;
  font-size: 40px;
  font-weight: 800;
  line-height: 1;
}
.formula {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 7px;
  margin: 16px 0;
  padding: 10px 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.035);
  color: #dfe7f2;
  font-size: 13px;
}
.formula .operator {
  color: var(--app-muted);
}
.formula b {
  color: #ffcf66;
}
.breakdown {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.breakdown-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  color: var(--app-muted);
  font-size: 12px;
}
.breakdown-row b {
  color: #dfe7f2;
}
.result-empty,
.result-error {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 190px;
  color: var(--app-muted);
  font-size: 13px;
}
.result-error {
  color: #ff8a94;
}
@media (max-width: 768px) {
  .analysis-grid {
    grid-template-columns: 1fr;
  }
}
</style>
