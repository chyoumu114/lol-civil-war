<template>
  <div class="home">
    <div class="home-inner">
      <div class="home-brand">CIVIL WAR TOOLS</div>
      <h1 class="home-title">LOL 内战工具</h1>
      <p class="home-sub">为 QQ 群内战而生 · 报名 / 战力平衡 / 对局记录 / 排行榜</p>

      <router-link to="/civil-war" class="app-card">
        <div class="app-icon"><icon-trophy /></div>
        <div class="app-info">
          <div class="app-name">内战组织</div>
          <div class="app-desc">5v5 自定义对局报名、自动平衡阵容、录像战绩与个人排行</div>
        </div>
        <icon-right class="app-arrow" />
      </router-link>

      <div class="home-tips">
        <span><icon-check-circle /> 数据按 QQ 群隔离</span>
        <span><icon-check-circle /> 支持 PC 与手机</span>
        <span><icon-check-circle /> 邮箱 / QQ 登录</span>
      </div>

      <div class="home-foot">与 Riot Games 无关联 · League of Legends 是 Riot Games, Inc. 的商标</div>
    </div>
  </div>
</template>

<script setup>
// 首页只有一个应用入口：内战组织。
</script>

<style scoped>
.home {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  background:
    radial-gradient(900px 480px at 15% 10%, rgba(22, 93, 255, 0.35), transparent 60%),
    radial-gradient(800px 500px at 90% 85%, rgba(114, 46, 209, 0.35), transparent 60%),
    #0b1020;
  color: #fff;
  overflow: hidden;
}
.home::after {
  content: '';
  position: absolute;
  inset: 0;
  background-image: linear-gradient(rgba(255, 255, 255, 0.03) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255, 255, 255, 0.03) 1px, transparent 1px);
  background-size: 48px 48px;
  mask-image: radial-gradient(circle at center, #000 30%, transparent 75%);
}
.home-inner {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 560px;
  text-align: center;
}
.home-brand {
  display: inline-block;
  padding: 4px 12px;
  border: 1px solid rgba(255, 255, 255, 0.18);
  border-radius: 999px;
  font-size: 11px;
  letter-spacing: 3px;
  color: rgba(255, 255, 255, 0.72);
}
.home-title {
  margin: 18px 0 8px;
  font-size: 42px;
  font-weight: 800;
  letter-spacing: 1px;
}
.home-sub {
  margin: 0 0 32px;
  color: rgba(255, 255, 255, 0.62);
  font-size: 14px;
}
.app-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px;
  border-radius: 18px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.14);
  backdrop-filter: blur(16px);
  text-align: left;
  transition: all 0.2s;
}
.app-card:hover {
  transform: translateY(-2px);
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.28);
}
.app-icon {
  display: grid;
  place-items: center;
  width: 54px;
  height: 54px;
  border-radius: 14px;
  background: linear-gradient(135deg, #165dff, #722ed1);
  font-size: 26px;
  flex: 0 0 auto;
}
.app-info {
  flex: 1;
  min-width: 0;
}
.app-name {
  font-size: 18px;
  font-weight: 700;
}
.app-desc {
  margin-top: 4px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.62);
  line-height: 1.5;
}
.app-arrow {
  color: rgba(255, 255, 255, 0.5);
  font-size: 18px;
}
.home-tips {
  display: flex;
  justify-content: center;
  flex-wrap: wrap;
  gap: 20px;
  margin-top: 28px;
  color: rgba(255, 255, 255, 0.6);
  font-size: 12px;
}
.home-tips span {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.home-foot {
  margin-top: 48px;
  color: rgba(255, 255, 255, 0.32);
  font-size: 11px;
}
@media (max-width: 480px) {
  .home-title { font-size: 32px; }
  .home-sub { font-size: 12px; }
}
</style>
