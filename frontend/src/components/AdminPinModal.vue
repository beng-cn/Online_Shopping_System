<template>
  <el-dialog v-model="visible" title="管理员安全验证" :close-on-click-modal="false" width="400px">
    <p style="margin-bottom:16px;color:#666">请输入管理PIN码以访问后台</p>
    <el-input v-model="pin" type="password" placeholder="PIN码" show-password maxlength="20" @keyup.enter="verify" />
    <template #footer>
      <el-button @click="handleCancel">取消</el-button>
      <el-button type="primary" :loading="loading" @click="verify">验证</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref } from 'vue'
import axios from 'axios'
import { ElMessage } from 'element-plus'

const emit = defineEmits(['verified', 'cancel'])
const pin = ref('')
const loading = ref(false)
const visible = ref(true)

async function verify() {
  if (!pin.value) return ElMessage.warning('请输入PIN码')
  loading.value = true
  try {
    await axios.post('/api/admin/verify-pin', { pin: pin.value })
    ElMessage.success('验证成功')
    visible.value = false
    emit('verified')
  } catch (e) {
    // 错误已由拦截器显示
  } finally {
    loading.value = false
  }
}

function handleCancel() {
  visible.value = false
  emit('cancel')
}
</script>
