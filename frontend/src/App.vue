<template>
  <div id="app">
    <!-- 顶栏：登录后显示 -->
    <el-menu v-if="userStore.isLoggedIn" mode="horizontal" :ellipsis="false" class="top-menu">
      <el-menu-item index="shop" class="nav-shop" @click="$router.push('/')">
        <el-icon><ShoppingCart /></el-icon> 商城
      </el-menu-item>

      <el-menu-item v-if="userStore.isAdmin" index="admin" @click="goAdmin">
        <el-icon><Setting /></el-icon> 管理后台
      </el-menu-item>

      <div class="menu-right">
        <el-menu-item index="flash" class="nav-flash" @click="$router.push('/flash')">
          🔥 秒杀
        </el-menu-item>
        <el-menu-item index="cart" class="nav-cart" @click="$router.push('/cart')">
          <el-icon><ShoppingCartFull /></el-icon> 购物车
        </el-menu-item>
        <el-menu-item index="orders" class="nav-orders" @click="$router.push('/orders')">
          <el-icon><Tickets /></el-icon> 我的订单
        </el-menu-item>
        <el-sub-menu index="user" class="nav-user">
          <template #title>
            <el-icon><UserFilled /></el-icon> {{ userStore.username }}
          </template>
          <el-menu-item index="logout" @click="handleLogout">
            <span class="logout-text">退出登录</span>
          </el-menu-item>
        </el-sub-menu>
      </div>
    </el-menu>

    <!-- 未登录顶栏 -->
    <div v-else class="top-bar-guest">
      <span class="brand" @click="$router.push('/')">🛒 在线商城</span>
      <div>
        <el-button type="warning" @click="$router.push('/flash')">🔥 秒杀</el-button>
        <el-button type="primary" @click="$router.push('/login')">登录</el-button>
      </div>
    </div>

    <!-- 页面内容 -->
    <div class="page-container">
      <router-view />
    </div>

    <!-- 管理员 PIN 二次验证弹窗 -->
    <AdminPinModal v-if="showPinModal" @verified="onPinVerified" @cancel="showPinModal = false" />
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useUserStore } from './stores/user'
import AdminPinModal from './components/AdminPinModal.vue'

const router = useRouter()
const route = useRoute()
const userStore = useUserStore()
const showPinModal = ref(false)

function goAdmin() {
  if (!userStore.pinVerified) {
    showPinModal.value = true
    return
  }
  router.push('/admin/dashboard')
}

watch(() => route.query.needPin, (val) => {
  if (val === '1' && !userStore.pinVerified) {
    showPinModal.value = true
    router.replace({ query: {} })
  }
}, { immediate: true })

function onPinVerified() {
  showPinModal.value = false
  userStore.pinVerified = true
  router.push('/admin/dashboard')
}

function handleLogout() {
  userStore.logout()
  router.push('/login')
}
</script>

<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
#app { font-family: 'Helvetica Neue', Helvetica, 'PingFang SC', 'Microsoft YaHei', sans-serif; }
.top-menu { display: flex; }
.menu-right { margin-left: auto; display: flex; }
.top-bar-guest {
  display: flex; justify-content: space-between; align-items: center;
  padding: 0 24px; height: 56px; border-bottom: 1px solid #eee; background: #fff;
}
.top-bar-guest .brand { font-size: 20px; font-weight: bold; cursor: pointer; }
.page-container { padding: 20px; max-width: 1400px; margin: 0 auto; }

/* 导航栏橙色背景 */
.top-menu {
  background: #fa8c16 !important;
  border-bottom: 2px solid #d46b08 !important;
}
.top-menu .el-menu-item { color: #fff !important; }
.top-menu .el-menu-item:hover { background: #ffa940 !important; color: #fff !important; }
.top-menu .el-sub-menu .el-sub-menu__title { color: #fff !important; }
.top-menu .el-sub-menu .el-sub-menu__title:hover { background: #ffa940 !important; }

/* 导航菜单项黄色底板 */
.nav-shop { background: #df710b !important; color: #fff !important; font-weight: bold; margin: 4px 2px; border-radius: 4px; }
.nav-shop:hover { background: #ffa940 !important; color: #fff !important; }
.nav-flash { background: #df710b !important; color: #333 !important; margin: 4px 2px; border-radius: 4px; }
.nav-flash:hover { background: #df710b !important; color: #333 !important; }
.nav-cart { background: #df710b !important; color: #333 !important; margin: 4px 2px; border-radius: 4px; }
.nav-cart:hover { background: #df710b !important; color: #333 !important; }
.nav-orders { background: #df710b !important; color: #333 !important; margin: 4px 2px; border-radius: 4px; }
.nav-orders:hover { background: #df710b !important; color: #333 !important; }
.nav-user .el-sub-menu__title:hover { color: #fff !important; }
.logout-text { color: #ffccc7; }
</style>
