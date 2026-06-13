<template>
  <div class="dashboard">
    <!-- 欢迎卡片 -->
    <el-card class="welcome-card" shadow="hover">
      <div class="welcome-content">
        <div class="welcome-text">
          <h2>欢迎回来，{{ userStore.username }}</h2>
          <p>今天是 {{ today }}，祝您工作愉快！</p>
        </div>
        <el-icon :size="64" color="#409EFF"><Avatar /></el-icon>
      </div>
    </el-card>

    <!-- 快捷功能入口 -->
    <el-row :gutter="20" class="quick-actions">
      <el-col :span="6">
        <el-card shadow="hover" class="action-card" @click="$router.push('/admin/products')">
          <el-icon :size="36" color="#67C23A"><Goods /></el-icon>
          <h3>商品管理</h3>
          <p>新增、编辑、删除商品</p>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="action-card" @click="$router.push('/admin/categories')">
          <el-icon :size="36" color="#409EFF"><Menu /></el-icon>
          <h3>分类管理</h3>
          <p>管理商品分类层级</p>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="action-card" @click="$router.push('/admin/flash')">
          <el-icon :size="36" color="#E6A23C"><Lightning /></el-icon>
          <h3>秒杀管理</h3>
          <p>创建和管理秒杀活动</p>
        </el-card>
      </el-col>
      <el-col :span="6">
        <el-card shadow="hover" class="action-card" @click="$router.push('/admin/users')">
          <el-icon :size="36" color="#F56C6C"><UserFilled /></el-icon>
          <h3>用户管理</h3>
          <p>管理用户和权限</p>
        </el-card>
      </el-col>
    </el-row>

    <!-- 设置管理员PIN码 -->
    <el-card class="pin-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <el-icon><Lock /></el-icon>
          <span>设置管理员PIN码</span>
        </div>
      </template>
      <el-form :model="pinForm" :rules="pinRules" ref="pinFormRef" label-width="100px" style="max-width: 500px">
        <el-form-item label="新PIN码" prop="pin">
          <el-input v-model="pinForm.pin" type="password" placeholder="请输入新PIN码" show-password maxlength="20" />
        </el-form-item>
        <el-form-item label="确认PIN码" prop="pinConfirm">
          <el-input v-model="pinForm.pinConfirm" type="password" placeholder="请再次输入PIN码" show-password maxlength="20" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :loading="pinLoading" @click="handleSetPin">保存PIN码</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 系统信息 -->
    <el-card class="info-card" shadow="hover">
      <template #header>
        <div class="card-header">
          <el-icon><InfoFilled /></el-icon>
          <span>系统信息</span>
        </div>
      </template>
      <el-descriptions :column="2" border>
        <el-descriptions-item label="系统版本">v1.0.0</el-descriptions-item>
        <el-descriptions-item label="前端框架">Vue 3 + Element Plus</el-descriptions-item>
        <el-descriptions-item label="后端框架">Go + Gin</el-descriptions-item>
        <el-descriptions-item label="数据库">MySQL + Redis</el-descriptions-item>
        <el-descriptions-item label="当前角色">管理员</el-descriptions-item>
        <el-descriptions-item label="登录时间">{{ loginTime }}</el-descriptions-item>
      </el-descriptions>
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useUserStore } from '../../stores/user'
import api from '../../api'
import { ElMessage } from 'element-plus'

const userStore = useUserStore()

// 今天日期
const today = new Date().toLocaleDateString('zh-CN', {
  year: 'numeric',
  month: 'long',
  day: 'numeric',
  weekday: 'long'
})

// 登录时间
const loginTime = new Date().toLocaleString('zh-CN')

// PIN码表单
const pinFormRef = ref(null)
const pinLoading = ref(false)
const pinForm = reactive({
  pin: '',
  pinConfirm: ''
})

// 验证PIN码一致性
const validatePinConfirm = (_rule, value, callback) => {
  if (value !== pinForm.pin) {
    callback(new Error('两次输入的PIN码不一致'))
  } else {
    callback()
  }
}

const pinRules = {
  pin: [
    { required: true, message: '请输入PIN码', trigger: 'blur' },
    { min: 4, max: 20, message: 'PIN码长度在4到20位之间', trigger: 'blur' }
  ],
  pinConfirm: [
    { required: true, message: '请再次输入PIN码', trigger: 'blur' },
    { validator: validatePinConfirm, trigger: 'blur' }
  ]
}

// 设置PIN码
async function handleSetPin() {
  const valid = await pinFormRef.value.validate().catch(() => false)
  if (!valid) return

  pinLoading.value = true
  try {
    await api.post('/admin/set-pin', { pin: pinForm.pin })
    ElMessage.success('PIN码设置成功')
    pinForm.pin = ''
    pinForm.pinConfirm = ''
    pinFormRef.value.resetFields()
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    pinLoading.value = false
  }
}
</script>

<style scoped>
.dashboard {
  max-width: 1200px;
  margin: 0 auto;
}

.welcome-card {
  margin-bottom: 24px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  color: #fff;
}

.welcome-card h2 {
  margin: 0 0 8px 0;
  font-size: 24px;
}

.welcome-card p {
  margin: 0;
  opacity: 0.9;
}

.welcome-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.quick-actions {
  margin-bottom: 24px;
}

.action-card {
  text-align: center;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;
}

.action-card:hover {
  transform: translateY(-4px);
}

.action-card h3 {
  margin: 12px 0 8px 0;
  font-size: 18px;
}

.action-card p {
  margin: 0;
  color: #909399;
  font-size: 13px;
}

.pin-card {
  margin-bottom: 24px;
}

.info-card {
  margin-bottom: 24px;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 16px;
  font-weight: 500;
}
</style>
