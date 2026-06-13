<template>
  <div class="user-manage">
    <!-- 顶部栏 -->
    <el-card shadow="never" class="header-card">
      <div class="header-bar">
        <h2>用户管理</h2>
        <el-button @click="fetchUsers">
          <el-icon><Refresh /></el-icon> 刷新
        </el-button>
      </div>
    </el-card>

    <!-- 搜索栏 -->
    <el-card shadow="never" class="filter-card">
      <el-form :inline="true" :model="searchForm">
        <el-form-item label="用户搜索">
          <el-input v-model="searchForm.keyword" placeholder="用户名/手机号" clearable @clear="handleSearch" @keyup.enter="handleSearch" style="width: 260px" />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleSearch">搜索</el-button>
          <el-button @click="resetSearch">重置</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 用户表格 -->
    <el-card shadow="never">
      <el-table v-loading="loading" :data="userList" border stripe style="width: 100%" empty-text="暂无用户数据">
        <el-table-column prop="id" label="ID" width="60" align="center" />
        <el-table-column prop="username" label="用户名" width="140" show-overflow-tooltip />
        <el-table-column prop="phone" label="手机号" width="140" align="center" />
        <el-table-column label="角色" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.role_id === 1 ? 'danger' : 'info'" size="small">
              {{ row.role_id === 1 ? '管理员' : '普通用户' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? '启用' : '禁用' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="注册时间" width="170" align="center" show-overflow-tooltip />
        <el-table-column label="操作" width="280" align="center" fixed="right">
          <template #default="{ row }">
            <el-button
              :type="row.status === 1 ? 'warning' : 'success'"
              size="small"
              link
              @click="handleToggleStatus(row)"
            >
              {{ row.status === 1 ? '禁用' : '启用' }}
            </el-button>
            <el-button type="info" size="small" link @click="openResetPassword(row)">重置密码</el-button>
            <el-button type="danger" size="small" link @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="pagination.page_num"
          v-model:page-size="pagination.page_size"
          :page-sizes="[10, 20, 50, 100]"
          :total="pagination.total"
          layout="total, sizes, prev, pager, next, jumper"
          background
          @size-change="handleSearch"
          @current-change="handleSearch"
        />
      </div>
    </el-card>

    <!-- 重置密码弹窗 -->
    <el-dialog
      v-model="resetPwdVisible"
      title="重置用户密码"
      width="420px"
      :close-on-click-modal="false"
      @closed="resetPwdForm.password = ''"
    >
      <p style="margin-bottom: 16px; color: #909399">
        正在为用户 <strong>{{ resetTarget?.username }}</strong> 重置密码
      </p>
      <el-form ref="resetPwdFormRef" :model="resetPwdForm" :rules="resetPwdRules" label-width="80px">
        <el-form-item label="新密码" prop="password">
          <el-input
            v-model="resetPwdForm.password"
            type="password"
            placeholder="请输入新密码（至少6位）"
            show-password
            maxlength="50"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="resetPwdVisible = false">取消</el-button>
        <el-button type="primary" :loading="resetPwdLoading" @click="handleResetPassword">确认重置</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import api from '../../api'
import { ElMessage, ElMessageBox } from 'element-plus'

// ==================== 数据状态 ====================
const loading = ref(false)
const userList = ref([])

const searchForm = reactive({
  keyword: ''
})

const pagination = reactive({
  page_num: 1,
  page_size: 10,
  total: 0
})

onMounted(() => {
  fetchUsers()
})

// 获取用户列表
async function fetchUsers() {
  loading.value = true
  try {
    const params = {
      page_num: pagination.page_num,
      page_size: pagination.page_size
    }
    if (searchForm.keyword) params.keyword = searchForm.keyword

    const res = await api.get('/admin/user/list', { params })
    const data = res.data
    userList.value = data.list || []
    pagination.total = data.total || 0
  } catch (e) {
    userList.value = []
  } finally {
    loading.value = false
  }
}

// 搜索
function handleSearch() {
  pagination.page_num = 1
  fetchUsers()
}

// 重置搜索
function resetSearch() {
  searchForm.keyword = ''
  pagination.page_num = 1
  fetchUsers()
}

// ==================== 启用/禁用用户 ====================
async function handleToggleStatus(row) {
  const newStatus = row.status === 1 ? 0 : 1
  const actionText = newStatus === 0 ? '禁用' : '启用'

  try {
    await ElMessageBox.confirm(
      `确定要${actionText}用户「${row.username}」吗？`,
      `${actionText}确认`,
      {
        confirmButtonText: `确定${actionText}`,
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await api.put(`/admin/user/${row.id}/status`, { status: newStatus })
    ElMessage.success(`用户已${actionText}`)
    fetchUsers()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  }
}

// ==================== 重置密码 ====================
const resetPwdVisible = ref(false)
const resetPwdLoading = ref(false)
const resetTarget = ref(null)
const resetPwdFormRef = ref(null)
const resetPwdForm = reactive({
  password: ''
})

const resetPwdRules = {
  password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { min: 6, max: 50, message: '密码长度在6到50位之间', trigger: 'blur' }
  ]
}

function openResetPassword(row) {
  resetTarget.value = row
  resetPwdForm.password = ''
  resetPwdVisible.value = true
}

async function handleResetPassword() {
  const valid = await resetPwdFormRef.value.validate().catch(() => false)
  if (!valid) return

  resetPwdLoading.value = true
  try {
    await api.put(`/admin/user/${resetTarget.value.id}/reset-password`, {
      password: resetPwdForm.password
    })
    ElMessage.success('密码重置成功')
    resetPwdVisible.value = false
  } catch (e) {
    // 错误已由拦截器处理
  } finally {
    resetPwdLoading.value = false
  }
}

// ==================== 删除用户 ====================
async function handleDelete(row) {
  try {
    await ElMessageBox.confirm(
      `确定要删除用户「${row.username}」吗？此操作不可恢复，所有关联数据将被清除。`,
      '删除确认',
      {
        confirmButtonText: '确定删除',
        cancelButtonText: '取消',
        type: 'warning'
      }
    )
    await api.delete(`/admin/user/${row.id}`)
    ElMessage.success('用户已删除')
    fetchUsers()
  } catch (e) {
    if (e === 'cancel' || e === 'close') return
    // 错误已由拦截器处理
  }
}
</script>

<style scoped>
.user-manage {
  max-width: 1400px;
  margin: 0 auto;
}

.header-card {
  margin-bottom: 16px;
}

.header-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-bar h2 {
  margin: 0;
  font-size: 20px;
}

.filter-card {
  margin-bottom: 16px;
}

.pagination-wrap {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
