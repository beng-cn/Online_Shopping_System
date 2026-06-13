import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

// 用户状态管理（存储 JWT Token、用户信息、管理员 PIN 验证状态）
export const useUserStore = defineStore('user', () => {
  const token = ref(localStorage.getItem('token') || '')
  const roleId = ref(Number(localStorage.getItem('role_id')) || 0)
  const username = ref(localStorage.getItem('username') || '')
  const pinVerified = ref(false) // 管理员 PIN 二次验证（会话级别，不持久化）

  // 是否为管理员
  const isAdmin = computed(() => roleId.value === 1)

  // 是否已登录
  const isLoggedIn = computed(() => !!token.value)

  // 登录成功，存储用户信息
  function setLogin(data) {
    token.value = data.token
    roleId.value = data.user.role_id
    username.value = data.user.username
    localStorage.setItem('token', data.token)
    localStorage.setItem('role_id', data.user.role_id)
    localStorage.setItem('username', data.user.username)
  }

  // 退出登录
  function logout() {
    token.value = ''
    roleId.value = 0
    username.value = ''
    pinVerified.value = false
    localStorage.clear()
  }

  return { token, roleId, username, pinVerified, isAdmin, isLoggedIn, setLogin, logout }
})
