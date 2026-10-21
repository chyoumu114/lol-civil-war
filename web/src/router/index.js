import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior: () => ({ top: 0 }),
  routes: [
    { path: '/', name: 'home', component: HomeView },
    { path: '/civil-war', name: 'entry', component: () => import('../views/CivilWarEntryView.vue') },
    {
      path: '/civil-war/:group',
      component: () => import('../layouts/CivilWarLayout.vue'),
      children: [
        { path: '', name: 'events', component: () => import('../views/EventListView.vue') },
        { path: 'events/new', name: 'event-create', component: () => import('../views/EventCreateView.vue') },
        { path: 'events/:eventId', name: 'event-detail', component: () => import('../views/EventDetailView.vue') },
        {
          // 对局详情已改为组织详情内的弹窗；旧链接统一回到组织详情
          path: 'events/:eventId/matches/:gameNo',
          redirect: (to) => `/civil-war/${to.params.group}/events/${to.params.eventId}`,
        },
        { path: 'ranking', name: 'ranking', component: () => import('../views/RankingView.vue') },
        { path: 'players/:memberId', name: 'player', component: () => import('../views/PlayerView.vue') },
        { path: 'profile', name: 'profile', component: () => import('../views/ProfileView.vue') },
      ],
    },
    { path: '/analysis-of-power', name: 'analysis-of-power', component: () => import('../views/PowerAnalysisView.vue') },
    { path: '/champions', name: 'champions', component: () => import('../views/ChampionsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

// 不同页面设置浏览器标题，便于多标签识别
const PAGE_TITLES = {
  entry: '进入内战组织',
  events: '组织列表',
  'event-create': '发起组织',
  'event-detail': '组织详情',
  'match-record': '对局记录',
  ranking: '排行榜',
  player: '个人分析',
  profile: '个人资料',
  champions: '英雄资料库',
  'analysis-of-power': '战力计算器',
}

router.afterEach((to) => {
  const title = PAGE_TITLES[to.name]
  document.title = title ? `${title} · LOL 内战工具` : 'LOL 内战工具'
})

export default router
