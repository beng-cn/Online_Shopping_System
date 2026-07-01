<template>
  <div class="login-page">
    <el-card class="login-card" shadow="always">
      <!-- 登录/注册切换 -->
      <div class="tab-row">
        <span :class="['tab', { active: mode === 'login' }]" @click="switchMode('login')">登录</span>
        <span class="tab-divider">|</span>
        <span :class="['tab', { active: mode === 'register' }]" @click="switchMode('register')">注册</span>
      </div>

      <!-- 登录表单 -->
      <el-form v-if="mode === 'login'" ref="loginRef" :model="loginForm" :rules="loginRules" label-width="0" size="large">
        <el-form-item prop="username">
          <el-input v-model="loginForm.username" placeholder="用户名" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="loginForm.password" type="password" placeholder="密码" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" style="width:100%" :loading="loginLoading" @click="handleLogin">登 录</el-button>
        </el-form-item>
      </el-form>

      <!-- 注册表单 -->
      <el-form v-else ref="registerRef" :model="registerForm" :rules="registerRules" label-width="0" size="large">
        <el-form-item prop="username">
          <el-input v-model="registerForm.username" placeholder="用户名（3-20位）" maxlength="20" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="registerForm.password" type="password" placeholder="密码（至少6位）" show-password />
        </el-form-item>
        <el-form-item prop="confirmPassword">
          <el-input v-model="registerForm.confirmPassword" type="password" placeholder="确认密码" show-password />
        </el-form-item>
        <el-form-item prop="phone">
          <el-input v-model="registerForm.phone" placeholder="手机号（选填）" maxlength="11" />
        </el-form-item>
        <el-form-item prop="email">
          <el-input v-model="registerForm.email" placeholder="邮箱（选填）" />
        </el-form-item>
        <el-form-item>
          <el-button type="success" style="width:100%" :loading="registerLoading" @click="handleRegister">注 册</el-button>
        </el-form-item>
      </el-form>

      <div style="text-align:center">
        <el-link v-if="mode === 'login'" type="primary" @click="showForgot = true">忘记密码？</el-link>
      </div>
    </el-card>

    <!-- 找回密码弹窗 -->
    <el-dialog v-model="showForgot" title="找回密码" width="400px">
      <el-form :model="forgotForm" :rules="forgotRules" ref="forgotRef" label-width="80px">
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="forgotForm.phone" placeholder="注册时的手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="forgotForm.new_password" type="password" placeholder="新密码" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showForgot = false">取消</el-button>
        <el-button type="primary" :loading="forgotLoading" @click="handleForgot">重置密码</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import api from '../api'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()

const mode = ref('login')

function switchMode(m) {
  mode.value = m
}

// ===== 登录 =====
const loginForm = reactive({ username: '', password: '' })
const loginRules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, min: 6, message: '密码至少6位', trigger: 'blur' }],
}
const loginLoading = ref(false)

async function handleLogin() {
  loginLoading.value = true
  try {
    const res = await api.post('/user/login', loginForm)
    userStore.setLogin(res.data)
    ElMessage.success('登录成功')
    const targetPath = '/'
    // nextTick 确保 Vue 先完成导航栏的响应式更新，再执行路由跳转
    // 避免状态更新与路由切换在同一微任务中相互竞争
    await nextTick()
    try {
      await router.replace(targetPath)
    } catch (navErr) {
      // replace 被取消（如同路由跳转或守卫拦截），直接整页跳转
      console.warn('路由跳转被拦截，切换整页跳转:', navErr)
      window.location.href = targetPath
    }
    // 兜底：若路由确实变了但组件未刷新，强制跳转
    if (router.currentRoute.value.path === '/login') {
      window.location.href = targetPath
    }
  } catch (e) {
    // 已由拦截器处理
  } finally {
    loginLoading.value = false
  }
}

// ===== 注册 =====
const registerForm = reactive({
  username: '',
  password: '',
  confirmPassword: '',
  phone: '',
  email: '',
})
const validateConfirm = (rule, value, callback) => {
  if (value !== registerForm.password) {
    callback(new Error('两次密码不一致'))
  } else {
    callback()
  }
}
const registerRules = {
  username: [
    { required: true, min: 3, max: 20, message: '用户名3-20位', trigger: 'blur' },
  ],
  password: [
    { required: true, min: 6, message: '密码至少6位', trigger: 'blur' },
  ],
  confirmPassword: [
    { required: true, message: '请确认密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' },
  ],
  phone: [
    { pattern: /^1[3-9]\d{9}$|^$/, message: '手机号格式不正确', trigger: 'blur' },
  ],
  email: [
    { type: 'email', message: '邮箱格式不正确', trigger: 'blur' },
  ],
}
const registerLoading = ref(false)

async function handleRegister() {
  registerLoading.value = true
  try {
    await api.post('/user/register', {
      username: registerForm.username,
      password: registerForm.password,
      phone: registerForm.phone,
      email: registerForm.email,
    })
    ElMessage.success('注册成功，请登录')
    mode.value = 'login'
    loginForm.username = registerForm.username
    loginForm.password = ''
  } catch (e) {
    // 已由拦截器处理
  } finally {
    registerLoading.value = false
  }
}

// ===== 找回密码 =====
const showForgot = ref(false)
const forgotLoading = ref(false)
const forgotForm = reactive({ phone: '', new_password: '' })
const forgotRules = {
  phone: [{ required: true, len: 11, message: '请输入11位手机号', trigger: 'blur' }],
  new_password: [{ required: true, min: 6, message: '新密码至少6位', trigger: 'blur' }],
}

async function handleForgot() {
  forgotLoading.value = true
  try {
    await api.post('/user/forgot-password', forgotForm)
    ElMessage.success('密码重置成功，请用新密码登录')
    showForgot.value = false
  } catch (e) {
    // 已由拦截器处理
  } finally {
    forgotLoading.value = false
  }
}
</script>

<style scoped>
.login-page {
  display: flex; justify-content: center; align-items: center;
  min-height: 80vh; background: #f5f7fa;
}
.login-card { width: 420px; }
.tab-row {
  text-align: center; margin-bottom: 24px; font-size: 18px;
}
.tab { cursor: pointer; color: #909399; padding: 0 12px; }
.tab.active { color: #409eff; font-weight: bold; }
.tab-divider { color: #dcdfe6; margin: 0 4px; }
</style>
