<template>
  <div class="profile-page">
    <div class="back-bar">
      <el-button text @click="$router.back()">
        <el-icon><ArrowLeft /></el-icon> 返回上一级
      </el-button>
    </div>
    <el-card shadow="never" class="profile-card">
      <template #header>
        <h3>个人信息</h3>
      </template>

      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="80px"
        size="large"
        style="max-width: 500px"
        :disabled="!editing"
      >
        <el-form-item label="用户名">
          <el-input :model-value="info.username" disabled />
        </el-form-item>
        <el-form-item label="昵称" prop="nickname">
          <el-input v-model="form.nickname" placeholder="给自己起个名字" maxlength="50" />
        </el-form-item>
        <el-form-item label="邮箱" prop="email">
          <el-input v-model="form.email" placeholder="常用的邮箱地址" />
        </el-form-item>
        <el-form-item label="手机号" prop="phone">
          <el-input v-model="form.phone" placeholder="11位手机号" maxlength="11" />
        </el-form-item>
        <el-form-item label="角色">
          <el-tag :type="info.role_id === 1 ? 'danger' : 'info'">
            {{ info.role_id === 1 ? '管理员' : '普通用户' }}
          </el-tag>
        </el-form-item>
        <el-form-item label="注册时间">
          <span>{{ info.created_at || '-' }}</span>
        </el-form-item>

        <el-form-item>
          <template v-if="!editing">
            <el-button type="primary" @click="startEdit">修改资料</el-button>
          </template>
          <template v-else>
            <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
            <el-button @click="cancelEdit">取消</el-button>
          </template>
        </el-form-item>
      </el-form>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '../../api'
import { ElMessage } from 'element-plus'

const editing = ref(false)
const saving = ref(false)
const formRef = ref(null)

const info = reactive({
  username: '',
  role_id: 2,
  created_at: '',
})

const form = reactive({
  nickname: '',
  email: '',
  phone: '',
})

// 备份编辑前的值
let backup = {}

const rules = {
  email: [{ type: 'email', message: '邮箱格式不正确', trigger: 'blur' }],
  phone: [
    { pattern: /^(1[3-9]\d{9})?$/, message: '手机号格式不正确', trigger: 'blur' },
  ],
}

async function fetchInfo() {
  try {
    const res = await api.get('/auth/user/info')
    const d = res.data
    info.username = d.username
    info.role_id = d.role_id
    info.created_at = d.created_at
    form.nickname = d.nickname || ''
    form.email = d.email || ''
    form.phone = d.phone || ''
  } catch (e) {
    // 已由拦截器处理
  }
}

function startEdit() {
  backup = { ...form }
  editing.value = true
}

function cancelEdit() {
  Object.assign(form, backup)
  editing.value = false
}

async function handleSave() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  saving.value = true
  try {
    await api.put('/auth/user/info', {
      nickname: form.nickname,
      email: form.email,
      phone: form.phone,
    })
    ElMessage.success('资料已更新')
    editing.value = false
    // 刷新显示
    fetchInfo()
  } catch (e) {
    // 已由拦截器处理
  } finally {
    saving.value = false
  }
}

onMounted(fetchInfo)
</script>

<style scoped>
.back-bar { margin-bottom: 8px; }
.profile-page {
  max-width: 700px; margin: 0 auto;
}
.profile-card h3 { margin: 0; }
</style>
