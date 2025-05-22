import { createRouter, createWebHistory } from 'vue-router'
import ResourcesLayout from '@/views/ResourcesLayout.vue'
import ResourceList from '@/views/ResourceList.vue'

const routes = [
  {
    path: '/',
    redirect: '/resources'
  },
  {
    path: '/resources',
    component: ResourcesLayout,
    children: [
      {
        path: '',
        name: 'ResourceList',
        component: ResourceList
      },
      {
        path: ':group/:version/:resource',
        name: 'ResourceDetail',
        component: () => import('@/views/ResourceDetail.vue')
      }
    ]
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

export default router 