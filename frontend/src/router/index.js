import { createRouter, createWebHistory } from 'vue-router'
import { useUserStore } from '../stores/user'

const routes = [
  // === 公开页面 ===
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue') },
  { path: '/', name: 'Home', component: () => import('../views/shop/Home.vue') },
  { path: '/flash', name: 'FlashSale', component: () => import('../views/shop/FlashSale.vue') },
  { path: '/product/:id', name: 'ProductDetail', component: () => import('../views/shop/ProductDetail.vue') },

  // === 需登录 ===
  { path: '/profile', name: 'Profile', component: () => import('../views/shop/Profile.vue'), meta: { requiresAuth: true } },
  { path: '/cart', name: 'Cart', component: () => import('../views/shop/Cart.vue'), meta: { requiresAuth: true } },
  { path: '/orders', name: 'Orders', component: () => import('../views/shop/Orders.vue'), meta: { requiresAuth: true } },

  // === 管理员 ===
  { path: '/admin', redirect: '/admin/dashboard' },
  { path: '/admin/dashboard', name: 'AdminDashboard', component: () => import('../views/admin/Dashboard.vue'), meta: { requiresAdmin: true } },
  { path: '/admin/products', name: 'AdminProducts', component: () => import('../views/admin/ProductManage.vue'), meta: { requiresAdmin: true } },
  { path: '/admin/categories', name: 'AdminCategories', component: () => import('../views/admin/CategoryManage.vue'), meta: { requiresAdmin: true } },
  { path: '/admin/flash', name: 'AdminFlash', component: () => import('../views/admin/FlashManage.vue'), meta: { requiresAdmin: true } },
  { path: '/admin/users', name: 'AdminUsers', component: () => import('../views/admin/UserManage.vue'), meta: { requiresAdmin: true } },

  // 404
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// 全局前置守卫：鉴权 + PIN 验证
router.beforeEach((to, from, next) => {
  const userStore = useUserStore()

  // 管理员页面：需要 role_id=1
  if (to.meta.requiresAdmin) {
    if (!userStore.isAdmin) {
      return next('/')
    }
    // PIN 二次验证（初次访问管理后台时需要）
    if (!userStore.pinVerified) {
      return next({ path: '/admin/dashboard', query: { needPin: '1' } })
    }
  }

  // 需登录页面
  if (to.meta.requiresAuth && !userStore.isLoggedIn) {
    return next('/login')
  }

  // 已登录用户访问登录页 → 跳到首页
  if (to.path === '/login' && userStore.isLoggedIn) {
    return next('/')
  }

  next()
})

export default router
