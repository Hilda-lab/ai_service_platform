import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import ChatView from '../views/ChatView.vue'
import VisionView from '../views/VisionView.vue'
import SpeechView from '../views/SpeechView.vue'
import { getToken } from '../utils/auth'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', redirect: '/login' },
    { path: '/login', name: 'login', component: LoginView },
    { path: '/chat', name: 'chat', component: ChatView },
    { path: '/vision', name: 'vision', component: VisionView },
    { path: '/speech', name: 'speech', component: SpeechView },
  ],
})

router.beforeEach((to) => {
  const token = getToken()
  if ((to.path === '/chat' || to.path === '/vision' || to.path === '/speech') && !token) {
    return '/login'
  }
  if (to.path === '/login' && token) {
    return '/chat'
  }
  return true
})

export default router
