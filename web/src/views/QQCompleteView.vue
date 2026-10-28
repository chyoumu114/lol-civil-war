<template>
  <div class="complete-page">
    <a-card :bordered="false" class="complete-card">
      <a-result
        status="success"
        title="QQ 登录已升级"
        sub-title="QQ 快捷登录现在会直接进入，不再需要绑定邮箱和密码。请返回首页重新使用快捷登录。"
      >
        <template #extra>
          <a-button type="primary" @click="goHome">返回首页</a-button>
        </template>
      </a-result>
    </a-card>
  </div>
</template>

<script setup>
function goHome() {
  window.location.href = '/'
}
</script>

<style scoped>
.complete-page {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 32px 16px;
  background:
    radial-gradient(900px 480px at 15% 10%, rgba(22, 93, 255, 0.35), transparent 60%),
    radial-gradient(800px 500px at 90% 85%, rgba(114, 46, 209, 0.35), transparent 60%),
    #0b1020;
}
.complete-card {
  width: 100%;
  max-width: 480px;
  padding: 8px;
}
</style>
