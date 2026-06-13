<template>
  <div class="login-page">
    <el-card class="login-card" shadow="always">
      <h2 style="text-align:center;margin-bottom:24px">用户登录</h2>

      <el-form ref="formRef" :model="form" :rules="rules" label-width="0" size="large">
        <el-form-item prop="username">
          <el-input v-model="form.username" placeholder="用户名" prefix-icon="User" />
        </el-form-item>
        <el-form-item prop="password">
          <el-input v-model="form.password" type="password" placeholder="密码" prefix-icon="Lock" show-password />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" style="width:100%" :loading="loading" @click="handleLogin">登 录</el-button>
        </el-form-item>
      </el-form>

      <div style="text-align:center">
        <el-link type="primary" @click="showForgot = true">忘记密码？</el-link>
      </div>
    </el-card>

    <!-- 找回密码弹窗 -->
    <el-dialog v-model="showForgot" title="找回密码" width="400px">
      <el-form :model="forgotForm" :rules="forgotRules" ref="forgotRef" label-width="80px">
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="forgotForm.phone" placeholder="请输入注册时的手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="新密码" prop="new_password">
          <el-input v-model="forgotForm.new_password" type="password" placeholder="请输入新密码" show-password />
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
import { ref, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from '../stores/user'
import api from '../api'
import { ElMessage } from 'element-plus'

const router = useRouter()
const userStore = useUserStore()

const form = reactive({ username: '', password: '' })
const rules = {
  username: [{ required: true, message: '请输入用户名', trigger: 'blur' }],
  password: [{ required: true, min: 6, message: '密码至少6位', trigger: 'blur' }],
}
const loading = ref(false)

async function handleLogin() {
  loading.value = true
  try {
    const res = await api.post('/user/login', form)
    userStore.setLogin(res.data)
    ElMessage.success('登录成功')
    router.push(userStore.isAdmin ? '/admin/dashboard' : '/')
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    loading.value = false
  }
}

// 找回密码
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
.login-card { width: 400px; }
</style>
