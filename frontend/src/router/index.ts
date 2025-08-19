import { createRouter, createWebHistory } from 'vue-router'
import ConnectionListView from '../views/ConnectionListView.vue'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/',
      name: 'connections',
      component: ConnectionListView
    }
  ]
})

export default router