import { createApp } from 'vue'
import ArcoVue from '@arco-design/web-vue'
import ArcoVueIcon from '@arco-design/web-vue/es/icon'
import zhCN from '@arco-design/web-vue/es/locale/lang/zh-cn'
import '@arco-design/web-vue/dist/arco.css'
import './styles.css'
import App from './App.vue'
import router from './router'

// Arco 深色主题：body[arco-theme='dark']
document.body.setAttribute('arco-theme', 'dark')

const app = createApp(App)
app.use(ArcoVue, { locale: zhCN })
app.use(ArcoVueIcon)
app.use(router)
app.mount('#app')
