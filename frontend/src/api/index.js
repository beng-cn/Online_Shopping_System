import axios from 'axios'
import { ElMessage } from 'element-plus'

// 创建 Axios 实例，后端在 8080，Vite 代理已配置
const api = axios.create({
  baseURL: '/api',
  timeout: 15000,
})

// 请求拦截器：自动注入 JWT Token
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：统一错误处理
api.interceptors.response.use(
  (res) => {
    const data = res.data
    if (data.code !== 0) {
      ElMessage.error(data.message || '请求失败')
      return Promise.reject(new Error(data.message))
    }
    return data // 直接返回 { code, message, data }
  },
  (err) => {
    if (err.response?.status === 401) {
      localStorage.removeItem('token')
      localStorage.removeItem('role_id')
      window.location.href = '/login'
    }
    return Promise.reject(err)
  }
)

export default api
